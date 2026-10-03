package pptx

import (
	"context"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/pptx/internal/presetgeom"
	"github.com/mgilbir/spine/render"
)

// PrepareRender prepares this static slide, including unsaved edits, for PNG or
// SVG output. The supported profile is solid or theme-referenced solid
// backgrounds; rectangles, rounded rectangles and ellipses with solid fills and
// solid outlines; uncropped embedded PNG/JPEG pictures; and plain horizontal
// European-script paragraphs in non-placeholder shapes, drawn with supplied
// fonts and styles inherited from list styles, document defaults and the
// theme; tables; straight connectors; and the master's and layout's own
// shapes beneath the slide's. Colors may be RGB, system or theme scheme colors
// with luminance transforms. Other theme styles, transformations, effects and
// other content fail explicitly. Hidden slides can be selected.
// Preparation does not synchronize or save source parts. Caller edits must not
// race with preparation; returned pages can be rendered concurrently.
// See docs/rendering.md for capability and resource contracts.
func (s *Slide) PrepareRender(ctx context.Context, opts render.Options) (*render.Page, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.presentation == nil {
		return nil, fmt.Errorf("%w: detached slide", render.ErrInvalid)
	}
	resolved, err := core.ResolveLimits(opts.Limits)
	if err != nil {
		return nil, err
	}
	opts.Limits = resolved
	textLayout, err := core.NewTextLayout(resolved)
	if err == nil && opts.Warn != nil {
		textLayout.AllowOverflow()
	}
	if err != nil {
		return nil, err
	}
	fonts := newSlideRenderFonts(opts)
	budget, err := core.NewSourceBudget(opts.MaxSourceBytes, opts.MaxLayoutNodes)
	if err != nil {
		return nil, err
	}
	// In best-effort mode soft reports a problem and lets preparation go on.
	lenient := opts.Warn != nil
	soft := func(err error) error {
		if err == nil || !lenient || ctx.Err() != nil {
			return err
		}
		opts.Warn(err)
		return nil
	}
	slideProfile := &renderProfile{lenient: lenient}
	// Check original bytes before lazy parsing can discard unknown markup. The
	// check is deliberately conservative for source features subsequently removed.
	if s.presentation.reader != nil {
		if file := s.presentation.reader.GetFile(s.partName); file != nil {
			stream, e := file.Open()
			if e != nil {
				return nil, e
			}
			e = budget.CheckReader(ctx, stream, slideProfile.slide)
			closeErr := stream.Close()
			if e != nil {
				return nil, fmt.Errorf("pptx: %s: %w", s.partName, e)
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
	}
	model := s.sx()
	if model == nil || model.CSld == nil {
		return nil, fmt.Errorf("%w: missing slide data", render.ErrInvalid)
	}
	for _, note := range slideProfile.notes {
		opts.Warn(fmt.Errorf("pptx: %s: %w", s.partName, note))
	}
	// Animation and transitions do not change a static slide; best effort
	// draws it without them.
	if len(model.AlternateContent) > 0 {
		if err = soft(fmt.Errorf("%w: slide alternate content", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	if (model.Timing != nil || model.Transition != nil) && !lenient {
		return nil, fmt.Errorf("%w: slide animation", render.ErrUnsupported)
	}
	if err = soft(renderModelExtensions(model.ExtLst, "p:sld")); err != nil {
		return nil, err
	}
	layers := []*oxml.CommonSlideData{}
	// parts names each layer's part, whose relationships its background uses.
	var parts []string
	if s.layout != nil {
		if s.layout.master != nil && s.layout.master.masterXML != nil {
			m := s.layout.master.masterXML
			// Header/footer flags only select footer placeholders, which this
			// profile rejects on the master and the slide.
			if len(m.AlternateContent) > 0 {
				if err = soft(fmt.Errorf("%w: master alternate content", render.ErrUnsupported)); err != nil {
					return nil, err
				}
			}
			if err = soft(renderModelExtensions(m.ExtLst, "p:sldMaster")); err != nil {
				return nil, err
			}
			layers, parts = append(layers, m.CSld), append(parts, s.layout.master.partName)
		}
		if s.layout.layoutXML != nil {
			m := s.layout.layoutXML
			if len(m.AlternateContent) > 0 {
				if err = soft(fmt.Errorf("%w: layout alternate content", render.ErrUnsupported)); err != nil {
					return nil, err
				}
			}
			if err = soft(renderModelExtensions(m.ExtLst, "p:sldLayout")); err != nil {
				return nil, err
			}
			layers, parts = append(layers, m.CSld), append(parts, s.layout.partName)
		}
	}
	masterProfile, layoutProfile := &renderProfile{lenient: lenient}, &renderProfile{lenient: lenient}
	if s.layout != nil {
		if s.layout.layoutXML != nil && len(s.layout.layoutXML.SourceXML) > 0 {
			if err = budget.CheckXML(ctx, s.layout.layoutXML.SourceXML, layoutProfile.inherited); err != nil {
				return nil, fmt.Errorf("pptx: layout: %w", err)
			}
		}
		if s.layout.master != nil && s.layout.master.masterXML != nil && len(s.layout.master.masterXML.SourceXML) > 0 {
			if err = budget.CheckXML(ctx, s.layout.master.masterXML.SourceXML, masterProfile.inherited); err != nil {
				return nil, fmt.Errorf("pptx: master: %w", err)
			}
		}
	}
	for _, note := range append(layoutProfile.notes, masterProfile.notes...) {
		opts.Warn(fmt.Errorf("pptx: layout or master: %w", note))
	}
	// A slide hiding background graphics hides its layout's and master's
	// shapes; a layout hiding them hides its master's.
	var inherited []renderInherited
	shown := map[*oxml.CommonSlideData]bool{}
	if l := s.layout; l != nil && (model.ShowMasterSp == nil || *model.ShowMasterSp) {
		if m := l.master; m != nil && m.masterXML != nil && (l.layoutXML == nil || l.layoutXML.ShowMasterSp == nil || *l.layoutXML.ShowMasterSp) {
			inherited = append(inherited, renderInherited{data: m.masterXML.CSld, part: m.partName, shapeErrs: masterProfile.shapeErrs})
			shown[m.masterXML.CSld] = true
		}
		if l.layoutXML != nil {
			inherited = append(inherited, renderInherited{data: l.layoutXML.CSld, part: l.partName, shapeErrs: layoutProfile.shapeErrs})
			shown[l.layoutXML.CSld] = true
		}
	}
	// A hidden layer still supplies its background but draws no shapes.
	for _, layer := range layers {
		if err = soft(renderTreeBase(layer, budget, shown[layer])); err != nil {
			return nil, err
		}
	}
	layers, parts = append(layers, model.CSld), append(parts, s.partName)
	w, h := s.presentation.slideDimensions()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%w: slide dimensions", render.ErrInvalid)
	}
	for _, v := range []dml.EMU{w, h} {
		if _, ok := style.FromPx(float64(v) / float64(dml.EMUsPerPixel)); !ok {
			return nil, fmt.Errorf("%w: slide coordinate", render.ErrLimit)
		}
	}
	colors := &renderColors{ctx: ctx, slide: s, budget: budget}
	styles := &renderTextStyles{ctx: ctx, slide: s, budget: budget, colors: colors, masterErrs: masterProfile.styleErrs, warn: opts.Warn}
	// The nearest defined background wins: slide, then layout, then master.
	background := renderPaint{color: renderWhite}
	px := float64(dml.EMUsPerPixel)
	var (
		bgImage      image.Image
		bgImageBytes int64
	)
	for i := len(layers) - 1; i >= 0; i-- {
		if layers[i] != nil && layers[i].Bg != nil {
			if bp := layers[i].Bg.BgPr; bp != nil && bp.BlipFill != nil {
				if bgImage, bgImageBytes, err = s.renderBackgroundImage(ctx, parts[i], bp, resolved, colors); err != nil {
					if err = soft(fmt.Errorf("pptx: background drawn white: %w", err)); err != nil {
						return nil, err
					}
				}
				break
			}
			if background, err = colors.background(layers[i].Bg, float64(w)/px, float64(h)/px); err != nil {
				if err = soft(fmt.Errorf("pptx: background drawn white: %w", err)); err != nil {
					return nil, err
				}
				background = renderPaint{color: renderWhite}
			}
			break
		}
	}
	if err = soft(renderTreeBase(model.CSld, budget, true)); err != nil {
		return nil, err
	}
	shapes := s.shapeList()
	if len(shapes) > budget.Nodes || len(shapes) >= resolved.MaxOperations {
		return nil, fmt.Errorf("%w: slide shapes", render.ErrLimit)
	}
	budget.Nodes -= len(shapes)
	// A gradient background lies over white, which shows through its alpha.
	ops := []layout.Op{layout.FillRect{Rect: layout.Rect{W: renderUnit(w), H: renderUnit(h)}, Color: background.color}}
	if background.gradient != nil {
		ops[0] = layout.FillRect{Rect: layout.Rect{W: renderUnit(w), H: renderUnit(h)}, Color: renderWhite}
		bgOps, err := background.fillOps(0, 0, float64(w)/px, float64(h)/px, nil)
		if err != nil {
			return nil, err
		}
		ops = append(ops, bgOps...)
	}
	imageCount := 0
	imagePixels, imageBytes := int64(0), int64(0)
	if bgImage != nil {
		// A picture background is stretched over the slide.
		ops = append(ops, layout.DrawImage{Rect: layout.Rect{W: renderUnit(w), H: renderUnit(h)}, Image: bgImage})
		imageCount, imageBytes = 1, bgImageBytes
		imagePixels = int64(bgImage.Bounds().Dx()) * int64(bgImage.Bounds().Dy())
	}
	// A slide often repeats one image; decode and charge it once.
	decoded := map[renderImageKey]image.Image{}
	// drawShape paints one shape. sp is its parsed p:sp, if any; index is its
	// position among the slide's own shapes, or -1 for an inherited shape;
	// picture resolves a picture's image bytes.
	var (
		drawShapeRef renderDraw
		connect      renderConnect
	)
	drawShape := func(sh Shape, sp *oxml.Shape, picProps *dml.SpPr, index int, picture func(*Picture) ([]byte, renderImageKey)) ([]layout.Op, error) {
		switch v := sh.(type) {
		case *AutoShape:
			if v == nil {
				return nil, fmt.Errorf("%w: nil shape", render.ErrInvalid)
			}
		case *TextBox:
			if v == nil {
				return nil, fmt.Errorf("%w: nil text box", render.ErrInvalid)
			}
		case *Picture:
			if v == nil {
				return nil, fmt.Errorf("%w: nil picture", render.ErrInvalid)
			}
		case *Table:
			if v == nil || index < 0 {
				return nil, fmt.Errorf("%w: nil or inherited table", render.ErrUnsupported)
			}
		case *Connector:
			if v == nil || index < 0 {
				return nil, fmt.Errorf("%w: nil or inherited connector", render.ErrUnsupported)
			}
		case *PlaceholderShape:
			if v == nil || index < 0 {
				return nil, fmt.Errorf("%w: nil or inherited placeholder", render.ErrUnsupported)
			}
		case *GroupShape:
			if v == nil {
				return nil, fmt.Errorf("%w: nil group", render.ErrInvalid)
			}
		default:
			return nil, fmt.Errorf("%w: shape (%T)", render.ErrUnsupported, sh)
		}
		if lenient {
			// Details of this shape drawn approximately are reported under
			// its name; a group's children report under their own.
			prev := colors.approx
			seen := map[string]bool{}
			colors.approx = func(err error) {
				if ctx.Err() == nil && !seen[err.Error()] {
					seen[err.Error()] = true
					opts.Warn(fmt.Errorf("pptx: shape %q: drawn approximately: %w", sh.Name(), err))
				}
			}
			defer func() { colors.approx = prev }()
		}
		x, y := sh.Position()
		sw, shh := sh.Size()
		if sw < 0 || shh < 0 {
			return nil, fmt.Errorf("%w: shape extent", render.ErrInvalid)
		}
		for _, v := range []dml.EMU{x, y, sw, shh} {
			if _, ok := style.FromPx(float64(v) / float64(dml.EMUsPerPixel)); !ok {
				return nil, fmt.Errorf("%w: shape coordinate", render.ErrLimit)
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// A parsed or inherited picture, such as a picture placeholder, may
		// inherit its geometry, which pictures here do not; an API picture
		// writes its own.
		if _, ok := sh.(*Picture); ok && (index < 0 || picProps != nil) && (picProps == nil || picProps.Xfrm == nil || picProps.Xfrm.Off == nil || picProps.Xfrm.Ext == nil) {
			return nil, fmt.Errorf("%w: picture without its own geometry", render.ErrUnsupported)
		}
		var (
			props *dml.SpPr
			st    *dml.Style
		)
		if sp != nil {
			props, st = sp.SpPr, sp.Style
		}
		switch v := sh.(type) {
		case *TextBox:
			// A text box is written as a rectangle unless its source says
			// otherwise.
			preset := "rect"
			if props != nil && props.PrstGeom != nil {
				preset = props.PrstGeom.Prst
			}
			box := &AutoShape{BaseShape: v.BaseShape, textFrame: v.textFrame, spPr: v.spPr, presetGeometry: preset}
			drawn, geometry, err := renderAutoShape(box, props, st, colors, resolved)
			if err != nil || box.textFrame == nil {
				return drawn, err
			}
			text, err := renderShapeText(ctx, sp, box, geometry, textLayout, fonts, styles, nil)
			if err == nil && geometry.turned && len(text) > 0 {
				err = colors.approximate(fmt.Errorf("%w: text of a rotated or flipped shape drawn upright", render.ErrUnsupported))
			}
			if err = soft(renderTextLeftOut(sh, err)); err != nil {
				return nil, err
			}
			return append(drawn, text...), nil
		case *AutoShape:
			drawn, geometry, err := renderAutoShape(v, props, st, colors, resolved)
			if err != nil || v.textFrame == nil {
				return drawn, err
			}
			text, err := renderShapeText(ctx, sp, v, geometry, textLayout, fonts, styles, nil)
			if err == nil && geometry.turned && len(text) > 0 {
				err = colors.approximate(fmt.Errorf("%w: text of a rotated or flipped shape drawn upright", render.ErrUnsupported))
			}
			if err = soft(renderTextLeftOut(sh, err)); err != nil {
				return nil, err
			}
			return append(drawn, text...), nil
		case *GroupShape:
			grp := s.renderSourceGroup(index)
			if grp == nil || v.isDirty() {
				return nil, fmt.Errorf("%w: new or edited group; save and reopen to preview it", render.ErrUnsupported)
			}
			return renderGroup(grp, renderIdentity, drawShapeRef, connect, s.renderPartPicture(s.partName), 0, opts.Warn, colors, resolved.MaxPathSegments)
		case *PlaceholderShape:
			return s.renderPlaceholderShape(ctx, v, sp, colors, resolved, textLayout, fonts, styles, layoutProfile.shapeErrs, masterProfile.shapeErrs, masterProfile.styleErrs, soft)
		case *Table:
			return s.renderTable(ctx, index, v, colors, textLayout, fonts, styles)
		case *Connector:
			return s.renderConnector(index, v, colors, resolved)
		case *Picture:
			if picProps != nil {
				if err := renderPictureProperties(picProps, colors); err != nil {
					return nil, err
				}
			}
			// An SVG picture draws its raster fallback, as Office versions
			// without SVG support show it.
			if v.isMedia {
				return nil, fmt.Errorf("%w: media picture", render.ErrUnsupported)
			}
			imgData, key := picture(v)
			if len(imgData) == 0 {
				return nil, fmt.Errorf("%w: missing picture data", render.ErrInvalid)
			}
			if imageCount >= resolved.MaxImages {
				return nil, fmt.Errorf("%w: slide image budget", render.ErrLimit)
			}
			imageCount++
			img := decoded[key]
			if img == nil {
				if int64(len(imgData)) > resolved.MaxImageBytes-imageBytes || imagePixels >= resolved.MaxImagePixels {
					return nil, fmt.Errorf("%w: slide image budget", render.ErrLimit)
				}
				imageBytes += int64(len(imgData))
				decodeLimits := resolved
				decodeLimits.MaxImagePixels = resolved.MaxImagePixels - imagePixels
				var err error
				if img, err = core.DecodeImage(ctx, imgData, decodeLimits); err != nil {
					return nil, err
				}
				imagePixels += int64(img.Bounds().Dx()) * int64(img.Bounds().Dy())
				decoded[key] = img
			}
			// A negative crop extends the picture past its image, which is
			// then laid over the extended box and clipped to the picture's.
			extended := v.cropLeft < 0 || v.cropTop < 0 || v.cropRight < 0 || v.cropBottom < 0
			if !extended {
				if img, err = renderCrop(img, v.cropLeft, v.cropTop, v.cropRight, v.cropBottom); err != nil {
					return nil, err
				}
			}
			if v.blipEffects {
				if err := colors.approximate(fmt.Errorf("%w: picture color effects left out", render.ErrUnsupported)); err != nil {
					return nil, err
				}
			}
			if v.opacity != nil && *v.opacity < 1 {
				img = renderFade(img, math.Max(0, *v.opacity))
			}
			x, y := v.Position()
			width, height := v.Size()
			if picProps != nil && picProps.Xfrm != nil && picProps.Xfrm.Off != nil && picProps.Xfrm.Ext != nil {
				// A placeholder picture may take its geometry from its layout.
				x, y = dml.EMU(picProps.Xfrm.Off.X), dml.EMU(picProps.Xfrm.Off.Y)
				width, height = dml.EMU(picProps.Xfrm.Ext.Cx), dml.EMU(picProps.Xfrm.Ext.Cy)
			}
			rect := layout.Rect{X: renderUnit(x), Y: renderUnit(y), W: renderUnit(width), H: renderUnit(height)}
			var outline []layout.Op
			if picProps != nil {
				if xf := picProps.Xfrm; xf != nil && (xf.FlipH || xf.FlipV || xf.Rot != 0) {
					if img, rect, err = renderOrientImage(img, rect, xf.FlipH, xf.FlipV, xf.Rot, colors); err != nil {
						return nil, err
					}
				}
				// A picture's outline runs around its box.
				line, placeholder, err := renderStyledLine(nil, picProps.Ln, colors)
				if err != nil {
					return nil, err
				}
				if line != nil {
					g := renderGeometry{box: [4]dml.EMU{x, y, width, height}}
					if outline, err = g.stroke(line, placeholder, colors, resolved.MaxPathSegments); err != nil {
						return nil, err
					}
				}
			}
			if extended {
				sx, sy := 1-v.cropLeft-v.cropRight, 1-v.cropTop-v.cropBottom
				if sx <= 0 || sy <= 0 {
					return nil, fmt.Errorf("%w: empty picture crop", render.ErrInvalid)
				}
				w, h := rect.W.Px()/sx, rect.H.Px()/sy
				ix, okX := style.FromPx(rect.X.Px() - v.cropLeft*w)
				iy, okY := style.FromPx(rect.Y.Px() - v.cropTop*h)
				iw, okW := style.FromPx(w)
				ih, okH := style.FromPx(h)
				if !okX || !okY || !okW || !okH {
					return nil, fmt.Errorf("%w: picture crop", render.ErrLimit)
				}
				full := layout.Rect{X: ix, Y: iy, W: iw, H: ih}
				img = renderDownscale(img, w*renderMaxImageScale, h*renderMaxImageScale)
				clipped := layout.DrawImage{Rect: full, Image: img, Clip: layout.Clip{Active: true, Rect: rect}}
				return append([]layout.Op{clipped}, outline...), nil
			}
			img = renderDownscale(img, rect.W.Px()*renderMaxImageScale, rect.H.Px()*renderMaxImageScale)
			return append([]layout.Op{layout.DrawImage{Rect: rect, Image: img}}, outline...), nil
		}
		return nil, fmt.Errorf("%w: shape %T", render.ErrUnsupported, sh)
	}
	drawShapeRef = drawShape
	// Connectors on layouts and masters, and in groups, are drawn from their
	// parsed form.
	connect = func(c *oxml.ConnectionShape) ([]layout.Op, error) {
		if lenient {
			name := ""
			if c.NvCxnSpPr != nil && c.NvCxnSpPr.CNvPr != nil {
				name = c.NvCxnSpPr.CNvPr.Name
			}
			prev := colors.approx
			colors.approx = func(err error) {
				if ctx.Err() == nil {
					opts.Warn(fmt.Errorf("pptx: connector %q: drawn approximately: %w", name, err))
				}
			}
			defer func() { colors.approx = prev }()
		}
		return renderConnectorSource(c, colors, resolved)
	}
	// Master shapes, then layout shapes, then the slide's own.
	for _, layer := range inherited {
		drawn, err := s.renderLayer(layer, budget, drawShape, connect, opts.Warn, colors, resolved.MaxPathSegments)
		if err != nil {
			return nil, err
		}
		ops = append(ops, drawn...)
	}
	slidePicture := func(v *Picture) ([]byte, renderImageKey) {
		data := v.Data()
		if len(data) == 0 {
			return nil, renderImageKey{}
		}
		return data, renderPictureKey(v, data)
	}
	for i, sh := range shapes {
		if s.renderSourceHidden(i) {
			continue
		}
		var drawn []layout.Op
		if err = slideProfile.shapeErrs[renderRefKey(s, i)].any; err == nil {
			drawn, err = drawShape(sh, s.renderSourceShape(i), s.renderPictureProps(i), i, slidePicture)
		}
		if err != nil {
			err = fmt.Errorf("pptx: slide %d shape %d (%s): %w", s.index, i, sh.Name(), err)
			if err = soft(err); err != nil {
				return nil, err
			}
			continue
		}
		ops = append(ops, drawn...)
	}
	return render.Prepare(ctx, w, h, ops, opts.Limits)
}

// renderHidden reports whether a parsed shape is hidden, which PowerPoint
// does not show.
func renderHidden(v any) bool {
	var c *dml.CNvPr
	switch s := v.(type) {
	case *oxml.Shape:
		if s != nil && s.NvSpPr != nil {
			c = s.NvSpPr.CNvPr
		}
	case *oxml.Picture:
		if s != nil && s.NvPicPr != nil {
			c = s.NvPicPr.CNvPr
		}
	case *oxml.GroupShape:
		if s != nil && s.NvGrpSpPr != nil {
			c = s.NvGrpSpPr.CNvPr
		}
	case *oxml.ConnectionShape:
		if s != nil && s.NvCxnSpPr != nil {
			c = s.NvCxnSpPr.CNvPr
		}
	case *oxml.GraphicFrame:
		if s != nil && s.NvGraphicFramePr != nil {
			c = s.NvGraphicFramePr.CNvPr
		}
	}
	return c != nil && c.Hidden
}

// renderSourceHidden reports whether a slide shape's parsed source is hidden.
func (s *Slide) renderSourceHidden(index int) bool {
	if s.sxModel == nil || s.sxModel.CSld == nil || s.sxModel.CSld.SpTree == nil || index >= len(s.shapeRefs) {
		return false
	}
	t, ref := s.sxModel.CSld.SpTree, s.shapeRefs[index]
	if ref.Index < 0 {
		return false
	}
	switch {
	case ref.Kind == oxml.ChildSp && ref.Index < len(t.Sp):
		return renderHidden(t.Sp[ref.Index])
	case ref.Kind == oxml.ChildPic && ref.Index < len(t.Pic):
		return renderHidden(t.Pic[ref.Index])
	case ref.Kind == oxml.ChildGrpSp && ref.Index < len(t.GrpSp):
		return renderHidden(t.GrpSp[ref.Index])
	case ref.Kind == oxml.ChildCxnSp && ref.Index < len(t.CxnSp):
		return renderHidden(t.CxnSp[ref.Index])
	case ref.Kind == oxml.ChildGraphicFrame && ref.Index < len(t.GraphicFrame):
		return renderHidden(t.GraphicFrame[ref.Index])
	}
	return false
}

// renderImageKey identifies a picture's image without hashing it: the media
// part it references, or else the identity of image bytes set through the API.
type renderImageKey struct {
	part string
	data *byte
	size int
}

// It resolves the part as Picture.Data does.
func renderPictureKey(v *Picture, data []byte) renderImageKey {
	if len(v.imageData) == 0 && v.slide != nil && v.relID != "" {
		if name := v.slide.relTargetPart(v.relID); name != "" {
			return renderImageKey{part: name}
		}
	}
	return renderImageKey{data: &data[0], size: len(data)}
}

// renderEffects reports whether an effect list applies any effect. PowerPoint
// writes an empty list where the schema requires effect properties, such as in
// a background.
func renderEffects(e *dml.EffectLst) bool {
	return e != nil && *e != (dml.EffectLst{})
}

// renderCrop keeps the part of an image a picture's source rectangle
// selects; crop fractions are of the image's width and height.
func renderCrop(img image.Image, l, t, r, b float64) (image.Image, error) {
	if l == 0 && t == 0 && r == 0 && b == 0 {
		return img, nil
	}
	if l < 0 || t < 0 || r < 0 || b < 0 || l+r >= 1 || t+b >= 1 {
		return nil, fmt.Errorf("%w: extended or empty picture crop", render.ErrUnsupported)
	}
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return nil, fmt.Errorf("%w: picture crop", render.ErrUnsupported)
	}
	bounds := img.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	rect := image.Rect(bounds.Min.X+int(math.Round(l*w)), bounds.Min.Y+int(math.Round(t*h)), bounds.Max.X-int(math.Round(r*w)), bounds.Max.Y-int(math.Round(b*h)))
	if rect.Empty() {
		return nil, fmt.Errorf("%w: empty picture crop", render.ErrUnsupported)
	}
	return sub.SubImage(rect), nil
}

// renderStyleFill resolves a style's fill reference to a solid or gradient
// fill and the color its phClr names; it returns nils for none.
func renderStyleFill(r *dml.FillRef, colors *renderColors) (*dml.SolidFill, *dml.GradFill, *style.RGBA, error) {
	if r == nil || r.Idx == 0 {
		return nil, nil, nil, nil
	}
	c, err := colors.color(renderColorOf(r.SrgbClr, r.SchemeClr, r.SysClr, r.PrstClr, r.ScrgbClr != nil, r.HslClr != nil), nil)
	if err != nil {
		return nil, nil, nil, err
	}
	theme, err := colors.loadTheme()
	if err != nil {
		return nil, nil, nil, err
	}
	var format *dml.FmtScheme
	if theme.ThemeElements != nil {
		format = theme.ThemeElements.FmtScheme
	}
	var (
		entry dml.StyleFill
		found bool
	)
	switch {
	case format == nil:
	case r.Idx <= 999:
		entry, found = format.FillStyleLst.Entry(int(r.Idx) - 1)
	case r.Idx >= 1001:
		entry, found = format.BgFillStyleLst.Entry(int(r.Idx - 1001))
	}
	switch {
	case !found:
		return nil, nil, nil, fmt.Errorf("%w: fill style %d", render.ErrInvalid, r.Idx)
	case entry.NoFill != nil:
		return nil, nil, nil, nil
	case entry.SolidFill == nil && entry.GradFill == nil:
		return nil, nil, nil, fmt.Errorf("%w: theme pattern or picture fill", render.ErrUnsupported)
	}
	return entry.SolidFill, entry.GradFill, &c, nil
}

// renderTextLeftOut marks a text failure whose shape is still drawn.
func renderTextLeftOut(sh Shape, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("pptx: shape %q: text left out: %w", sh.Name(), err)
}

// renderRefKey names a slide shape's spTree element for the source check's
// per-shape records.
func renderRefKey(s *Slide, i int) renderShapeKey {
	if i >= len(s.shapeRefs) {
		return renderShapeKey{}
	}
	ref := s.shapeRefs[i]
	name := map[oxml.ChildKind]string{oxml.ChildSp: "sp", oxml.ChildPic: "pic", oxml.ChildGraphicFrame: "graphicFrame", oxml.ChildGrpSp: "grpSp", oxml.ChildCxnSp: "cxnSp"}[ref.Kind]
	return renderShapeKey{name: name, occurrence: ref.Index + 1}
}

func renderUnit(v dml.EMU) style.Unit {
	u, _ := style.FromPx(float64(v) / float64(dml.EMUsPerPixel))
	return u
}
func renderTreeBase(c *oxml.CommonSlideData, b *core.SourceBudget, drawn bool) error {
	if c == nil || c.SpTree == nil {
		return fmt.Errorf("%w: shape tree", render.ErrInvalid)
	}
	if len(c.Controls) > 0 || len(c.CustDataLst) > 0 {
		return fmt.Errorf("%w: slide controls/customer data", render.ErrUnsupported)
	}
	if err := renderModelExtensions(c.ExtLst, "p:cSld"); err != nil {
		return err
	}
	t := c.SpTree
	if len(t.AltContent) > 0 || len(t.RawXML) > 0 {
		return fmt.Errorf("%w: raw/alternate drawing content", render.ErrUnsupported)
	}
	if t.GrpSpPr != nil {
		g := t.GrpSpPr
		if !renderRootTransform(g.Xfrm) || g.NoFill != nil || g.SolidFill != nil || g.GradFill != nil || g.BlipFill != nil || g.PattFill != nil || renderEffects(g.EffectLst) || g.EffectDag != nil || g.Scene3d != nil || g.ExtLst != nil {
			return fmt.Errorf("%w: root group properties", render.ErrUnsupported)
		}
	}
	if !drawn {
		return nil
	}
	for _, gf := range t.GraphicFrame {
		if gf == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil || gf.Graphic.GraphicData.URI != oxml.TableGraphicDataURI {
			return fmt.Errorf("%w: chart, diagram or embedded object", render.ErrUnsupported)
		}
	}
	return nil
}

// st is the shape's style reference, if any: its fill applies when the shape
// sets none, and its line beneath the shape's own.
func renderAutoShape(v *AutoShape, source *dml.SpPr, st *dml.Style, colors *renderColors, limits render.Limits) ([]layout.Op, renderGeometry, error) {
	var g renderGeometry
	p := &v.spPr
	if source != nil {
		if source.Xfrm == nil || source.Xfrm.Off == nil || source.Xfrm.Ext == nil {
			return nil, g, fmt.Errorf("%w: inherited/missing shape geometry", render.ErrUnsupported)
		}
		if renderEffects(source.EffectLst) || source.EffectDag != nil || source.Scene3d != nil || source.Sp3d != nil || renderEffects(v.spPr.EffectLst) || v.spPr.EffectDag != nil || v.spPr.Scene3d != nil || v.spPr.Sp3d != nil {
			if err := colors.approximate(fmt.Errorf("%w: shape effects left out", render.ErrUnsupported)); err != nil {
				return nil, g, err
			}
		}
		copyProps := *source
		if source.Ln != nil {
			line := *source.Ln
			copyProps.Ln = &line
		}
		applyShapeStyle(&copyProps, &v.spPr)
		p = &copyProps
	}
	if p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil {
		return nil, g, fmt.Errorf("%w: shape picture, pattern or group fill", render.ErrUnsupported)
	}
	if err := renderDMLExtensions(p.ExtLst, "p:spPr"); err != nil {
		return nil, g, err
	}
	if source == nil && (renderEffects(p.EffectLst) || p.EffectDag != nil || p.Scene3d != nil || p.Sp3d != nil) {
		if err := colors.approximate(fmt.Errorf("%w: shape effects left out", render.ErrUnsupported)); err != nil {
			return nil, g, err
		}
	}
	x, y := v.Position()
	w, h := v.Size()
	if w < 0 || h < 0 {
		return nil, g, fmt.Errorf("%w: shape extent", render.ErrInvalid)
	}
	px := float64(dml.EMUsPerPixel)
	xf := renderShapeTransform{cx: (float64(x) + float64(w)/2) / px, cy: (float64(y) + float64(h)/2) / px}
	if p.Xfrm != nil {
		xf.flipH, xf.flipV, xf.rot = p.Xfrm.FlipH, p.Xfrm.FlipV, float64(p.Xfrm.Rot)/60000
	}
	// PowerPoint turns text with the shape's rotation and a vertical flip.
	g.turned = math.Mod(xf.rot, 360) != 0 || xf.flipV
	if v.presetGeometry == "line" {
		// A line runs corner to corner, its flips choosing the corners.
		line, placeholder, err := renderStyledLine(st, p.Ln, colors)
		if err != nil || line == nil {
			return nil, g, err
		}
		x0, y0, x1, y1 := float64(x), float64(y), float64(x+w), float64(y+h)
		if p.Xfrm != nil && p.Xfrm.FlipH {
			x0, x1 = x1, x0
		}
		if p.Xfrm != nil && p.Xfrm.FlipV {
			y0, y1 = y1, y0
		}
		ops, err := renderLineStroke(line, placeholder, colors, x0/px, y0/px, x1/px, y1/px, limits.MaxPathSegments)
		if err != nil {
			return nil, g, err
		}
		turn := xf
		turn.flipH, turn.flipV = false, false
		ops, err = turn.ops(ops, colors, limits.MaxPathSegments)
		return ops, g, err
	}
	// Without a style reference, an absent fill, an absent outline, or an
	// outline without a fill is none.
	paint, filled, err := renderShapePaint(p, st, colors, w, h)
	if err != nil {
		return nil, g, err
	}
	line, linePlaceholder, err := renderStyledLine(st, p.Ln, colors)
	if err != nil {
		return nil, g, err
	}
	if p.CustGeom != nil {
		turned := g.turned
		ops, g, err := renderCustomShape(p.CustGeom, x, y, w, h, paint, filled, line, linePlaceholder, colors, limits)
		if err != nil {
			return nil, g, err
		}
		g.turned = turned
		ops, err = xf.ops(ops, colors, limits.MaxPathSegments)
		return ops, g, err
	}
	// Presets other than the three drawn exactly evaluate the standard's
	// definitions, with the shape's own adjustments over their defaults.
	if def, ok := presetgeom.Lookup(v.presetGeometry); ok && v.presetGeometry != "rect" && v.presetGeometry != "roundRect" && v.presetGeometry != "ellipse" {
		if p.PrstGeom != nil && p.PrstGeom.Prst != v.presetGeometry {
			return nil, g, fmt.Errorf("%w: preset geometry mismatch", render.ErrInvalid)
		}
		cg := *def
		if p.PrstGeom != nil && p.PrstGeom.AvLst != nil {
			var adjust dml.AvLst
			if def.AvLst != nil {
				adjust.Gd = append(adjust.Gd, def.AvLst.Gd...)
			}
			adjust.Gd = append(adjust.Gd, p.PrstGeom.AvLst.Gd...)
			cg.AvLst = &adjust
		}
		turned := g.turned
		ops, g, err := renderCustomShape(&cg, x, y, w, h, paint, filled, line, linePlaceholder, colors, limits)
		if err != nil {
			return nil, g, err
		}
		g.turned = turned
		ops, err = xf.ops(ops, colors, limits.MaxPathSegments)
		return ops, g, err
	}
	turned := g.turned
	g, err = renderPresetGeometry(v.presetGeometry, p.PrstGeom, x, y, w, h)
	if err != nil {
		return nil, g, err
	}
	g.turned = turned
	var ops []layout.Op
	if filled {
		if ops, err = g.fill(paint); err != nil {
			return nil, g, err
		}
	}
	if l := line; l != nil && (l.SolidFill != nil || l.GradFill != nil || l.PattFill != nil) {
		if l.NoFill != nil {
			return nil, g, fmt.Errorf("%w: ambiguous shape stroke", render.ErrInvalid)
		}
		outline, err := g.stroke(l, linePlaceholder, colors, limits.MaxPathSegments)
		if err != nil {
			return nil, g, err
		}
		ops = append(ops, outline...)
	}
	ops, err = xf.ops(ops, colors, limits.MaxPathSegments)
	return ops, g, err
}
// renderShapePaint resolves a shape's fill, its own or its style's; filled is
// false for none.
func renderShapePaint(p *dml.SpPr, st *dml.Style, colors *renderColors, w, h dml.EMU) (paint renderPaint, filled bool, err error) {
	if p.NoFill != nil && (p.SolidFill != nil || p.GradFill != nil) || p.SolidFill != nil && p.GradFill != nil {
		return paint, false, fmt.Errorf("%w: ambiguous shape fill", render.ErrInvalid)
	}
	fill, grad := p.SolidFill, p.GradFill
	var placeholder *style.RGBA
	if st != nil && p.NoFill == nil && fill == nil && grad == nil {
		if fill, grad, placeholder, err = renderStyleFill(st.FillRef, colors); err != nil {
			return paint, false, err
		}
	}
	if p.NoFill != nil || (fill == nil && grad == nil) {
		return paint, false, nil
	}
	px := float64(dml.EMUsPerPixel)
	paint, err = colors.fillPaint(fill, grad, placeholder, float64(w)/px, float64(h)/px)
	return paint, err == nil, err
}

// renderCustomShape draws a custom geometry: each path filled unless its fill
// is none, then outlined unless its stroke is off. An outline of more than
// one segment is drawn approximately, with round joins.
func renderCustomShape(cg *dml.CustGeom, x, y, w, h dml.EMU, paint renderPaint, filled bool, line *dml.Ln, placeholder *style.RGBA, colors *renderColors, limits render.Limits) ([]layout.Op, renderGeometry, error) {
	paths, insets, err := renderCustomGeometry(cg, x, y, w, h, limits.MaxPathSegments)
	g := renderGeometry{box: [4]dml.EMU{x, y, w, h}, text: insets}
	if err != nil {
		return nil, g, err
	}
	px := float64(dml.EMUsPerPixel)
	box := [4]float64{float64(x) / px, float64(y) / px, float64(w) / px, float64(h) / px}
	var ops []layout.Op
	for _, cp := range paths {
		if !filled || !cp.fill {
			continue
		}
		if cp.shade {
			if err := colors.approximate(fmt.Errorf("%w: lightened or darkened path fill drawn plain", render.ErrUnsupported)); err != nil {
				return nil, g, err
			}
		}
		drawn, err := renderCustomFill(cp, paint, box)
		if err != nil {
			return nil, g, err
		}
		ops = append(ops, drawn...)
	}
	if line == nil {
		return ops, g, nil
	}
	for _, cp := range paths {
		if !cp.stroke {
			continue
		}
		for _, sub := range cp.subpaths {
			if len(sub.pts) > 2 || sub.closed {
				if err := colors.approximate(fmt.Errorf("%w: custom outline drawn with round joins", render.ErrUnsupported)); err != nil {
					return nil, g, err
				}
				break
			}
		}
		drawn, err := renderCustomStroke(cp, line, placeholder, colors, limits.MaxPathSegments)
		if err != nil {
			return nil, g, err
		}
		ops = append(ops, drawn...)
	}
	return ops, g, nil
}

func slideRenderXML(el xml.StartElement) error {
	var attrs string
	switch el.Name.Space {
	case nsP:
		switch el.Name.Local {
		case "sld":
			attrs = "show showMasterSp showMasterPhAnim"
		case "cSld":
			attrs = "name"
		case "cNvPr":
			// Hidden shapes are not drawn.
			attrs = "id name descr title hidden"
		case "cNvSpPr":
			attrs = "txBox"
		case "ph":
			// Identifies a placeholder; prompts are not painted.
			attrs = "type orient sz idx hasCustomPrompt"
		case "hf":
			attrs = "sldNum hdr ftr dt"
		case "bgRef":
			attrs = "idx"
		case "xfrm":
			attrs = "rot flipH flipV"
		case "nvPr":
			// Marks a shape the user drew on a layout; painting is the same.
			attrs = "userDrawn"
		case "cNvPicPr":
			// An editor resize preference.
			attrs = "preferRelativeResize"
		case "spPr", "grpSpPr", "bg":
			// Black-and-white modes apply only to black-and-white output.
			attrs = "bwMode"
		case "blipFill":
			// Rotation with the shape and the stored DPI do not change an
			// unrotated stretched picture.
			attrs = "rotWithShape dpi"
		case "grpSp", "cNvGrpSpPr", "nvGrpSpPr", "spTree", "nvSpPr", "nvPicPr", "pic", "sp", "bgPr", "clrMapOvr", "txBody", "graphicFrame", "nvGraphicFramePr", "cNvGraphicFramePr", "cxnSp", "nvCxnSpPr", "cNvCxnSpPr", "style":
		default:
			return fmt.Errorf("%w: XML %s", render.ErrUnsupported, el.Name.Local)
		}
	case nsA:
		switch el.Name.Local {
		case "bodyPr":
			// rtlCol orders columns; the single-column profile has one.
			// The resolver checks the values of the rest.
			attrs = "wrap anchor lIns tIns rIns bIns rtlCol rot spcFirstLastPara vertOverflow horzOverflow vert numCol spcCol fromWordArt anchorCtr forceAA upright compatLnSpc"
		case "pPr", "defPPr", "lvl1pPr", "lvl2pPr", "lvl3pPr", "lvl4pPr", "lvl5pPr", "lvl6pPr", "lvl7pPr", "lvl8pPr", "lvl9pPr":
			// East Asian breaking, hanging punctuation, font alignment within
			// a uniformly sized line, and tab sizes for text without tabs do
			// not change this profile's text; the resolver checks the rest.
			attrs = "marL marR lvl indent algn defTabSz rtl eaLnBrk fontAlgn latinLnBrk hangingPunct"
		case "rPr", "defRPr", "endParaRPr":
			// Language, proofing, smart-tag, bookmark and East Asian attributes
			// do not change the painting of horizontal ASCII text; the resolver
			// checks the rest.
			attrs = "kumimoji lang altLang sz b i u strike kern cap spc normalizeH baseline noProof dirty err smtClean smtId bmk"
		case "latin", "ea", "cs", "sym":
			// ASCII text uses the Latin font; the other slots are not consulted.
			attrs = "typeface panose pitchFamily charset"
		case "tab":
			attrs = "pos algn"
		case "fld":
			// A field is drawn with the text it was saved with.
			attrs = "id type"
		case "br":
		case "buChar":
			attrs = "char"
		case "buFont":
			attrs = "typeface panose pitchFamily charset"
		case "buSzPct", "buSzPts":
			attrs = "val"
		case "buAutoNum":
			attrs = "type startAt"
		case "buBlip":
		case "buFontTx", "buClr", "buClrTx", "buSzTx":
		case "lnRef", "fillRef", "effectRef":
			attrs = "idx"
		case "fontRef":
			attrs = "idx"
		case "stCxn", "endCxn":
			// Bindings move a connector only when its shapes move.
			attrs = "id idx"
		case "grpSpLocks":
			attrs = "noGrp noUngrp noSelect noRot noChangeAspect noMove noResize"
		case "spLocks":
			attrs = "noGrp noSelect noRot noChangeAspect noMove noResize noEditPoints noAdjustHandles noChangeArrowheads noChangeShapeType noTextEdit"
		case "cxnSpLocks":
			attrs = "noGrp noSelect noRot noChangeAspect noMove noResize noEditPoints noAdjustHandles noChangeArrowheads noChangeShapeType"
		case "graphicFrameLocks":
			// Editor locks do not change painting.
			attrs = "noGrp noDrilldown noSelect noChangeAspect noMove noResize"
		case "graphicData":
			attrs = "uri"
		case "tblPr":
			attrs = "rtl firstRow firstCol lastRow lastCol bandRow bandCol"
		case "gridCol":
			attrs = "w"
		case "tr":
			attrs = "h"
		case "tc":
			attrs = "rowSpan gridSpan hMerge vMerge id"
		case "tcPr":
			attrs = "marL marR marT marB vert anchor anchorCtr horzOverflow"
		case "lnL", "lnR", "lnT", "lnB":
			attrs = "w cap cmpd algn"
		case "graphic", "tbl", "tblGrid", "txBody", "tableStyleId":
		case "gd":
			attrs = "name fmla"
		case "prstDash":
			attrs = "val"
		case "miter":
			attrs = "lim"
		case "headEnd", "tailEnd":
			// Line ends apply to open paths; the drawn presets are closed.
			attrs = "type w len"
		case "round", "bevel":
		case "tabLst", "uLnTx", "uFillTx":
		case "spcPct", "spcPts":
			attrs = "val"
		case "normAutofit":
			attrs = "fontScale lnSpcReduction"
		case "p", "r", "t", "lstStyle", "noAutofit", "spAutoFit", "buNone", "lnSpc", "spcBef", "spcAft":
		case "xfrm":
			attrs = "rot flipH flipV"
		case "off", "chOff":
			attrs = "x y"
		case "ext", "chExt":
			attrs = "cx cy"
		case "prstGeom":
			attrs = "prst"
		case "srgbClr", "schemeClr", "prstClr", "tint", "shade", "alpha", "alphaMod", "alphaOff", "hue", "hueMod", "hueOff", "sat", "satMod", "satOff",
			"lum", "lumMod", "lumOff", "red", "redMod", "redOff", "green", "greenMod", "greenOff", "blue", "blueMod", "blueOff":
			attrs = "val"
		case "comp", "inv", "gray", "gamma", "invGamma":
		case "sysClr":
			attrs = "val lastClr"
		case "picLocks":
			attrs = "noGrp noSelect noRot noChangeAspect noMove noResize noEditPoints noAdjustHandles noChangeArrowheads noChangeShapeType noCrop"
		case "blip":
			attrs = "cstate embed"
		case "alphaModFix":
			attrs = "amt"
		case "blipFill":
			attrs = "rotWithShape dpi"
		case "tile":
			attrs = "tx ty sx sy flip algn"
		case "fillRect", "srcRect", "fillToRect", "tileRect":
			attrs = "l t r b"
		case "gradFill":
			attrs = "flip rotWithShape"
		case "gs":
			attrs = "pos"
		case "lin":
			attrs = "ang scaled"
		case "path":
			// A gradient's path shade, or a geometry path.
			attrs = "path w h fill stroke extrusionOk"
		case "pt", "pos":
			attrs = "x y"
		case "arcTo":
			attrs = "wR hR stAng swAng"
		case "rect":
			attrs = "l t r b"
		case "ahXY":
			attrs = "gdRefX minX maxX gdRefY minY maxY"
		case "ahPolar":
			attrs = "gdRefR minR maxR gdRefAng minAng maxAng"
		case "cxn":
			attrs = "ang"
		case "prstTxWarp":
			attrs = "prst"
		case "custGeom", "gdLst", "ahLst", "cxnLst", "pathLst", "moveTo", "lnTo", "quadBezTo", "cubicBezTo", "close":
		case "gsLst":
		case "ln":
			attrs = "w cap cmpd algn"
		case "overrideClrMapping":
			attrs = "bg1 tx1 bg2 tx2 accent1 accent2 accent3 accent4 accent5 accent6 hlink folHlink"
		case "avLst", "noFill", "solidFill", "stretch", "masterClrMapping", "highlight", "effectLst":
		default:
			return fmt.Errorf("%w: XML %s", render.ErrUnsupported, el.Name.Local)
		}
	default:
		return fmt.Errorf("%w: XML namespace %s", render.ErrUnsupported, el.Name.Space)
	}
	for _, a := range el.Attr {
		if el.Name.Local == "xfrm" && (a.Name.Local == "rot" || a.Name.Local == "flipH" || a.Name.Local == "flipV") && a.Value != "0" && a.Value != "false" {
			return fmt.Errorf("%w: picture/shape transform", render.ErrUnsupported)
		}
		if el.Name.Local == "fillRect" && a.Value != "0" {
			return fmt.Errorf("%w: picture fill rectangle", render.ErrUnsupported)
		}
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		if a.Name.Space != "" && (el.Name.Local != "blip" || a.Name.Space != nsR || a.Name.Local != "embed") {
			return fmt.Errorf("%w: XML attribute namespace", render.ErrUnsupported)
		}
		if !strings.Contains(" "+attrs+" ", " "+a.Name.Local+" ") {
			return fmt.Errorf("%w: XML %s/@%s", render.ErrUnsupported, el.Name.Local, a.Name.Local)
		}
	}
	return nil
}

func renderRootTransform(x *dml.GrpXfrm) bool {
	if x == nil {
		return true
	}
	if x.Rot != 0 || x.FlipH || x.FlipV {
		return false
	}
	for _, v := range []*dml.OffXML{x.Off, x.ChOff} {
		if v != nil && (v.X != 0 || v.Y != 0) {
			return false
		}
	}
	for _, v := range []*dml.ExtXML{x.Ext, x.ChExt} {
		if v != nil && (v.Cx != 0 || v.Cy != 0) {
			return false
		}
	}
	return true
}

func (s *Slide) renderPictureProps(index int) *dml.SpPr {
	if index >= len(s.shapeRefs) {
		return nil
	}
	ref := s.shapeRefs[index]
	t := s.sxModel.CSld.SpTree
	if ref.Kind != oxml.ChildPic || ref.Index < 0 || ref.Index >= len(t.Pic) {
		return nil
	}
	pic := t.Pic[ref.Index]
	props := pic.SpPr
	if props != nil && props.Xfrm != nil && props.Xfrm.Off != nil && props.Xfrm.Ext != nil {
		return props
	}
	// A picture placeholder without geometry takes its layout's, or else its
	// master's, placeholder geometry.
	if pic.NvPicPr == nil || pic.NvPicPr.NvPr == nil || pic.NvPicPr.NvPr.Ph == nil {
		return props
	}
	ph := pic.NvPicPr.NvPr.Ph
	var trees []*oxml.ShapeTree
	if l := s.layout; l != nil {
		if l.layoutXML != nil && l.layoutXML.CSld != nil {
			trees = append(trees, l.layoutXML.CSld.SpTree)
		}
		if l.master != nil && l.master.masterXML != nil && l.master.masterXML.CSld != nil {
			trees = append(trees, l.master.masterXML.CSld.SpTree)
		}
	}
	for k, tree := range trees {
		i, err := renderMatch(tree, ph, k == 0, func(err error) error { return err })
		if err != nil || i < 0 {
			continue
		}
		if src := tree.Sp[i].SpPr; src != nil && src.Xfrm != nil && src.Xfrm.Off != nil && src.Xfrm.Ext != nil {
			out := dml.SpPr{}
			if props != nil {
				out = *props
			}
			out.Xfrm = src.Xfrm
			return &out
		}
	}
	return props
}
// renderBackgroundImage decodes a picture background from its part, cropped
// by its source rectangle. A tiled picture is drawn stretched, approximately.
func (s *Slide) renderBackgroundImage(ctx context.Context, part string, bp *oxml.BackgroundProps, limits render.Limits, colors *renderColors) (image.Image, int64, error) {
	f := bp.BlipFill
	if bp.SolidFill != nil || bp.GradFill != nil || bp.PattFill != nil || bp.NoFill != nil || renderEffects(bp.EffectLst) || bp.ExtLst != nil || f.Blip == nil || f.Blip.Embed == "" || f.Blip.Link != "" || len(f.Blip.Effects) > 0 {
		return nil, 0, fmt.Errorf("%w: background picture", render.ErrUnsupported)
	}
	if f.Tile != nil || f.Stretch == nil {
		if err := colors.approximate(fmt.Errorf("%w: tiled background picture drawn stretched", render.ErrUnsupported)); err != nil {
			return nil, 0, err
		}
	}
	data, _ := s.renderPartPicture(part)(&Picture{relID: f.Blip.Embed})
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("%w: missing background picture", render.ErrInvalid)
	}
	img, err := core.DecodeImage(ctx, data, limits)
	if err != nil {
		return nil, 0, err
	}
	if r := f.SrcRect; r != nil {
		if img, err = renderCrop(img, float64(r.L.Int32())/100000, float64(r.T.Int32())/100000, float64(r.R.Int32())/100000, float64(r.B.Int32())/100000); err != nil {
			return nil, 0, err
		}
	}
	return img, int64(len(data)), nil
}

// renderMaxImageScale is how many image pixels per drawn CSS pixel a picture
// keeps: enough for 384 DPI output.
const renderMaxImageScale = 4

// renderDownscale averages an image down to at most w by h pixels; a
// picture drawn smaller than its pixels needs no more.
func renderDownscale(img image.Image, w, h float64) image.Image {
	b := img.Bounds()
	tw, th := int(math.Ceil(w)), int(math.Ceil(h))
	if tw < 1 || th < 1 || (b.Dx() <= tw && b.Dy() <= th) {
		return img
	}
	tw, th = min(tw, b.Dx()), min(th, b.Dy())
	out := image.NewNRGBA(image.Rect(0, 0, tw, th))
	for y := 0; y < th; y++ {
		y0, y1 := y*b.Dy()/th, max((y+1)*b.Dy()/th, y*b.Dy()/th+1)
		for x := 0; x < tw; x++ {
			x0, x1 := x*b.Dx()/tw, max((x+1)*b.Dx()/tw, x*b.Dx()/tw+1)
			// Average premultiplied channels, as compositing would.
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, ca := img.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					r, g, bl, a, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), a+uint64(ca), n+1
				}
			}
			out.Set(x, y, color.RGBA64{R: uint16(r / n), G: uint16(g / n), B: uint16(bl / n), A: uint16(a / n)})
		}
	}
	return out
}

// renderFade multiplies an image's opacity.
func renderFade(img image.Image, alpha float64) image.Image {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			c.A = uint8(math.Round(float64(c.A) * alpha))
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}

// renderOrientImage applies a picture's flips and rotation to its pixels:
// quarter turns exactly, turning the drawn box about its centre, and other
// angles, approximately, not at all.
func renderOrientImage(img image.Image, rect layout.Rect, flipH, flipV bool, rot int32, colors *renderColors) (image.Image, layout.Rect, error) {
	turns := 0
	switch r := ((rot % 21600000) + 21600000) % 21600000; r {
	case 0, 5400000, 10800000, 16200000:
		turns = int(r / 5400000)
	default:
		if err := colors.approximate(fmt.Errorf("%w: picture drawn unrotated", render.ErrUnsupported)); err != nil {
			return nil, rect, err
		}
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	ow, oh := w, h
	if turns%2 == 1 {
		ow, oh = h, w
		cx, cy := rect.X+rect.W/2, rect.Y+rect.H/2
		rect = layout.Rect{X: cx - rect.H/2, Y: cy - rect.W/2, W: rect.H, H: rect.W}
	}
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for sy := 0; sy < h; sy++ {
		for sx := 0; sx < w; sx++ {
			// Flip in the picture's own frame, then turn clockwise.
			fx, fy := sx, sy
			if flipH {
				fx = w - 1 - sx
			}
			if flipV {
				fy = h - 1 - sy
			}
			dx, dy := fx, fy
			switch turns {
			case 1:
				dx, dy = h-1-fy, fx
			case 2:
				dx, dy = w-1-fx, h-1-fy
			case 3:
				dx, dy = fy, w-1-fx
			}
			out.Set(dx, dy, img.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return out, rect, nil
}

func renderPictureProperties(p *dml.SpPr, colors *renderColors) error {
	if p.CustGeom != nil || p.SolidFill != nil || p.GradFill != nil || p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil {
		return fmt.Errorf("%w: picture shape properties", render.ErrUnsupported)
	}
	if err := renderDMLExtensions(p.ExtLst, "p:spPr"); err != nil {
		return err
	}
	if renderEffects(p.EffectLst) || p.EffectDag != nil || p.Scene3d != nil || p.Sp3d != nil {
		if err := colors.approximate(fmt.Errorf("%w: picture effects left out", render.ErrUnsupported)); err != nil {
			return err
		}
	}
	if p.PrstGeom != nil && (p.PrstGeom.Prst != "rect" || (p.PrstGeom.AvLst != nil && len(p.PrstGeom.AvLst.Gd) > 0)) {
		return fmt.Errorf("%w: picture geometry", render.ErrUnsupported)
	}
	return nil
}

// renderProfile checks original slide, layout and master XML. It skips the
// subtree of an extension whose URI renderExtensions lists for its owner; the
// streaming checker reports no end tags, so a skipped subtree ends at the next
// node that is not below it.
type renderProfile struct {
	skipDepth int
	// lenient records problems instead of failing; see slide.
	lenient bool
	notes   []error
	// shapeErrs holds the first unsupported node of each master or layout
	// shape, by element and occurrence; a shape fails only when drawn.
	shapeErrs map[renderShapeKey]renderShapeErrs
	current   renderShapeKey
	// styleErrs holds the first unsupported node in each of a master's text
	// styles, which fails only slides that resolve text through that style.
	// which fails only slides that resolve text through it.
	styleErrs map[string]error
}

func (r *renderProfile) skipped(node core.XMLNode) bool {
	if r.skipDepth == 0 {
		return false
	}
	if len(node.Path) > r.skipDepth || (node.Text && len(node.Path) == r.skipDepth) {
		return true
	}
	r.skipDepth = 0
	return false
}

// extension checks an extension list or extension element and reports whether
// it consumed the node.
func (r *renderProfile) extension(node core.XMLNode) (bool, error) {
	if node.Text || len(node.Path) < 2 || node.Name.Space != nsP && node.Name.Space != nsA {
		return false, nil
	}
	parent := node.Path[len(node.Path)-2]
	switch {
	case node.Name.Local == "extLst":
		owner, ok := renderExtensions[renderXMLKey(parent)]
		if !ok || owner.list != node.Name.Space {
			return true, fmt.Errorf("%w: XML placement %s in %s", render.ErrUnsupported, node.Name.Local, parent.Local)
		}
		if node.Occurrence > 1 {
			return true, fmt.Errorf("%w: repeated XML %s", render.ErrInvalid, node.Name.Local)
		}
		// mod only records that an editor changed the list.
		return true, renderExtensionAttrs(node.StartElement, "mod")
	case node.Name.Local == "ext" && parent.Local == "extLst" && parent.Space == node.Name.Space:
		if len(node.Path) < 3 {
			return true, fmt.Errorf("%w: extension owner", render.ErrInvalid)
		}
		owner := renderExtensions[renderXMLKey(node.Path[len(node.Path)-3])]
		var uri string
		for _, a := range node.Attr {
			if a.Name.Space == "" && a.Name.Local == "uri" {
				uri = a.Value
			}
		}
		if !renderExtensionAllowed(owner, uri) {
			return true, fmt.Errorf("%w: extension %s", render.ErrUnsupported, uri)
		}
		if err := renderExtensionAttrs(node.StartElement, "uri"); err != nil {
			return true, err
		}
		r.skipDepth = len(node.Path)
		return true, nil
	}
	return false, nil
}

func renderExtensionAttrs(el xml.StartElement, allowed string) error {
	for _, a := range el.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		if a.Name.Space != "" || a.Name.Local != allowed {
			return fmt.Errorf("%w: XML %s/@%s", render.ErrUnsupported, el.Name.Local, a.Name.Local)
		}
	}
	return nil
}

// renderExtensionOwner names the namespace of an owner's extension list and the
// extension URIs it may carry.
type renderExtensionOwner struct {
	list string
	uris []string
}

// renderExtensions lists extensions that hold only identity, editor-guide,
// accessibility or image-storage metadata. None can change painted output;
// every other extension fails explicitly.
var renderExtensions = map[string]renderExtensionOwner{
	"p:cNvPr":     {list: nsA, uris: []string{xmlb.ExtURICreationId, xmlb.ExtURIDecorative}},
	// A picture with an SVG keeps a raster fallback, which is drawn.
	"a:blip": {list: nsA, uris: []string{xmlb.ExtURIUseLocalDpi, xmlb.ExtURISvgBlip}},
	// Hidden fills, lines and effects are kept for older editors and never
	// shown; the text box flag and shadow obscuring paint nothing here.
	"p:spPr": {list: nsA, uris: []string{xmlb.ExtURIHiddenFill, xmlb.ExtURIHiddenLine, xmlb.ExtURIHiddenEffects, xmlb.ExtURIShadowObscured, renderExtURIWrappingTextBox}},
	"p:cSld":      {list: nsP, uris: []string{xmlb.ExtURIPMLCreationId}},
	"p:sld":       {list: nsP, uris: []string{xmlb.ExtURISldGuideLst}},
	"p:sldLayout": {list: nsP, uris: []string{xmlb.ExtURISldGuideLstLayout}},
	"p:sldMaster": {list: nsP, uris: []string{xmlb.ExtURISldGuideLstMaster}},
	"p:nvPr":      {list: nsP, uris: []string{xmlb.ExtURIPMLModId, renderExtURIDesignElement}},
	"a:gridCol":   {list: nsA, uris: []string{xmlb.ExtURIColId}},
	"a:tr":        {list: nsA, uris: []string{xmlb.ExtURIRowId}},
}

const (
	// renderExtURIWrappingTextBox flags a shape Office for Mac made as a
	// text box.
	renderExtURIWrappingTextBox = "{C572A759-6A51-4108-AA02-DFA0A04FC94B}"
	// renderExtURIDesignElement marks a shape PowerPoint Designer added.
	renderExtURIDesignElement = "{386F3935-93C4-4BCD-93E2-E3B085C9AB24}"
)

// renderDMLExtensions applies the extension profile to a parsed DrawingML
// list.
func renderDMLExtensions(l *dml.ExtLst, owner string) error {
	if l == nil {
		return nil
	}
	for _, e := range l.Ext {
		if e == nil || !renderExtensionAllowed(renderExtensions[owner], e.URI) {
			return fmt.Errorf("%w: shape property extension", render.ErrUnsupported)
		}
	}
	return nil
}

func renderExtensionAllowed(owner renderExtensionOwner, uri string) bool {
	for _, allowed := range owner.uris {
		if strings.EqualFold(uri, allowed) {
			return true
		}
	}
	return false
}

// renderModelExtensions applies the source extension profile to a parsed list,
// which also covers lists created or edited through the API.
func renderModelExtensions(l *oxml.ExtensionList, owner string) error {
	if l == nil {
		return nil
	}
	for _, e := range l.Ext {
		if !renderExtensionAllowed(renderExtensions[owner], e.URI) {
			return fmt.Errorf("%w: extension %s", render.ErrUnsupported, e.URI)
		}
	}
	return nil
}

// check applies the profile to one node.
func (r *renderProfile) check(node core.XMLNode) error {
	if r.skipped(node) {
		return nil
	}
	if done, err := r.extension(node); done {
		return err
	}
	return slideRenderNode(node)
}

// slide checks slide XML. In best-effort mode a problem inside a shape is
// recorded against that shape, which is then left out, and a problem
// elsewhere is noted as a warning; only a wrong root fails.
func (r *renderProfile) slide(node core.XMLNode) error {
	if !r.lenient || len(node.Path) == 1 {
		return r.check(node)
	}
	if r.inShape(node) {
		if r.approximated(node) {
			return nil
		}
		r.record(node, r.check(node))
		return nil
	}
	r.note(r.check(node))
	return nil
}

// approximated skips, in best-effort mode, a shape's effect subtree; the
// renderers report the shape as drawn without it.
func (r *renderProfile) approximated(node core.XMLNode) bool {
	n := len(node.Path)
	if !r.lenient || node.Text || n < 2 || r.skipped(node) {
		return false
	}
	parent := node.Path[n-2]
	effect := parent == (xml.Name{Space: nsA, Local: "effectLst"}) ||
		(parent == (xml.Name{Space: nsA, Local: "blip"}) && node.Name.Space == nsA && node.Name.Local != "extLst" && node.Name.Local != "alphaModFix") ||
		node.Name == (xml.Name{Space: nsA, Local: "buBlip"}) ||
		(parent.Local == "rPr" || parent.Local == "endParaRPr" || parent.Local == "defRPr") && (node.Name.Local == "hlinkClick" || node.Name.Local == "hlinkMouseOver") ||
		(parent == (xml.Name{Space: nsP, Local: "spPr"}) && node.Name.Space == nsA && (node.Name.Local == "effectDag" || node.Name.Local == "scene3d" || node.Name.Local == "sp3d"))
	if effect {
		r.skipDepth = n
	}
	return effect
}

// inShape reports whether a node is a shape tree child or inside one.
func (r *renderProfile) inShape(node core.XMLNode) bool {
	return len(node.Path) >= 4 && node.Path[2] == (xml.Name{Space: nsP, Local: "spTree"}) && (len(node.Path) != 4 || !node.Text)
}

// record keeps the first problem of the shape a node belongs to.
func (r *renderProfile) record(node core.XMLNode, err error) {
	if len(node.Path) == 4 {
		r.current = renderShapeKey{name: node.Name.Local, occurrence: node.Occurrence}
	}
	if err == nil {
		return
	}
	if r.shapeErrs == nil {
		r.shapeErrs = map[renderShapeKey]renderShapeErrs{}
	}
	e := r.shapeErrs[r.current]
	if e.any == nil {
		e.any = err
	}
	// A placeholder's paragraphs are its prompt; only the rest is inherited.
	inText := len(node.Path) >= 6 && node.Path[4] == (xml.Name{Space: nsP, Local: "txBody"}) && node.Path[5] == (xml.Name{Space: nsA, Local: "p"})
	if e.inherited == nil && !inText {
		e.inherited = err
	}
	r.shapeErrs[r.current] = e
}

// note keeps a best-effort problem outside any shape, once per message.
func (r *renderProfile) note(err error) {
	if err == nil || len(r.notes) >= 32 {
		return
	}
	for _, n := range r.notes {
		if n.Error() == err.Error() {
			return
		}
	}
	r.notes = append(r.notes, err)
}

func slideRenderNode(node core.XMLNode) error {
	if node.Text {
		// Run text, and a table style id, which the table renderer reports.
		if n := len(node.Path); n > 0 && (node.Path[n-1] == (xml.Name{Space: nsA, Local: "t"}) || node.Path[n-1] == (xml.Name{Space: nsA, Local: "tableStyleId"})) {
			return nil
		}
		return fmt.Errorf("%w: unexpected XML text", render.ErrUnsupported)
	}
	if len(node.Path) == 1 && (node.Name.Space != nsP || node.Name.Local != "sld") {
		return fmt.Errorf("%w: slide XML root", render.ErrInvalid)
	}
	if len(node.Path) > 1 {
		parents := renderXMLParents[renderXMLKey(node.Name)]
		parent := renderXMLKey(node.Path[len(node.Path)-2])
		if !strings.Contains(" "+parents+" ", " "+parent+" ") {
			return fmt.Errorf("%w: XML placement %s in %s", render.ErrUnsupported, node.Name.Local, node.Path[len(node.Path)-2].Local)
		}
	}
	// A field's own paragraph properties are admitted only empty.
	if n := len(node.Path); n >= 3 && node.Path[n-2] == (xml.Name{Space: nsA, Local: "fld"}) && node.Name == (xml.Name{Space: nsA, Local: "pPr"}) && len(node.Attr) > 0 ||
		n >= 3 && node.Path[n-2] == (xml.Name{Space: nsA, Local: "pPr"}) && node.Path[n-3] == (xml.Name{Space: nsA, Local: "fld"}) {
		return fmt.Errorf("%w: field paragraph properties", render.ErrUnsupported)
	}
	repeated := (node.Name.Space == nsP && (node.Name.Local == "sp" || node.Name.Local == "pic" || node.Name.Local == "graphicFrame" || node.Name.Local == "cxnSp" || node.Name.Local == "grpSp")) ||
		(node.Name.Space == nsA && (node.Name.Local == "p" || node.Name.Local == "r" || node.Name.Local == "br" || node.Name.Local == "fld" || node.Name.Local == "tab" || node.Name.Local == "gd" || node.Name.Local == "gs" || node.Name.Local == "path" || node.Name.Local == "moveTo" || node.Name.Local == "lnTo" ||
			node.Name.Local == "arcTo" || node.Name.Local == "quadBezTo" || node.Name.Local == "cubicBezTo" || node.Name.Local == "close" || node.Name.Local == "pt" || node.Name.Local == "cxn" ||
			node.Name.Local == "ahXY" || node.Name.Local == "ahPolar" || node.Name.Local == "gridCol" || node.Name.Local == "tr" || node.Name.Local == "tc"))
	if node.Occurrence > 1 && !repeated {
		return fmt.Errorf("%w: repeated XML %s", render.ErrInvalid, node.Name.Local)
	}
	el := node.StartElement
	// Shape, connector and picture renderers read rotation and flips and
	// reject what they cannot draw.
	if n := len(node.Path); n >= 3 && el.Name == (xml.Name{Space: nsA, Local: "xfrm"}) && (node.Path[n-3] == (xml.Name{Space: nsP, Local: "cxnSp"}) || node.Path[n-3] == (xml.Name{Space: nsP, Local: "sp"}) || node.Path[n-3] == (xml.Name{Space: nsP, Local: "pic"}) || node.Path[n-3] == (xml.Name{Space: nsP, Local: "grpSp"})) {
		var attrs []xml.Attr
		for _, a := range el.Attr {
			if a.Name.Space != "" || (a.Name.Local != "flipH" && a.Name.Local != "flipV" && a.Name.Local != "rot") {
				attrs = append(attrs, a)
			}
		}
		el.Attr = attrs
	}
	return slideRenderXML(el)
}
func (r *renderProfile) inherited(node core.XMLNode) error {
	if r.skipped(node) {
		return nil
	}
	// Best effort draws a static page without animation or transitions.
	if r.lenient && len(node.Path) == 2 && node.Name.Space == nsP && (node.Name.Local == "timing" || node.Name.Local == "transition") {
		r.skipDepth = 2
		return nil
	}
	if r.inShape(node) && r.approximated(node) {
		return nil
	}
	if len(node.Path) == 1 {
		if node.Name.Space != nsP || (node.Name.Local != "sldMaster" && node.Name.Local != "sldLayout") {
			return fmt.Errorf("%w: inherited XML root", render.ErrUnsupported)
		}
		for _, a := range node.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				continue
			}
			if a.Name.Space != "" || !strings.Contains(" preserve showMasterSp showMasterPhAnim type userDrawn matchingName ", " "+a.Name.Local+" ") {
				return fmt.Errorf("%w: inherited root attribute", render.ErrUnsupported)
			}
		}
		return nil
	}
	rootChild := node.Path[1]
	if rootChild.Space == nsP && (rootChild.Local == "txStyles" || rootChild.Local == "sldLayoutIdLst" || rootChild.Local == "clrMap" || rootChild.Local == "clrMapOvr") {
		if len(node.Path) == 2 && node.Occurrence > 1 {
			return fmt.Errorf("%w: repeated inherited metadata", render.ErrInvalid)
		}
		// Non-placeholder text inherits the other-text style; title and body
		// styles serve placeholders, which this profile rejects. None of these
		// subtrees dereference relationships.
		if len(node.Path) == 3 && node.Path[2].Local == "otherStyle" && node.Occurrence > 1 {
			return fmt.Errorf("%w: repeated other-text style", render.ErrInvalid)
		}
		if len(node.Path) > 3 && node.Path[2].Space == nsP {
			if name := node.Path[2].Local; name == "titleStyle" || name == "bodyStyle" || name == "otherStyle" {
				if r.styleErrs == nil {
					r.styleErrs = map[string]error{}
				}
				if r.styleErrs[name] == nil {
					r.styleErrs[name] = slideRenderNode(node)
				}
			}
		}
		return nil
	}
	// A master or layout shape is checked like slide content, but its first
	// problem is recorded rather than returned: placeholders are never drawn,
	// and a hidden layer's shapes are not drawn either.
	if r.inShape(node) {
		r.record(node, r.check(node))
		return nil
	}
	return r.slide(node)
}

func renderXMLKey(n xml.Name) string {
	if n.Space == nsP {
		return "p:" + n.Local
	}
	if n.Space == nsA {
		return "a:" + n.Local
	}
	return "?" + n.Local
}

// Text property elements share their content models across slide text, shape
// list styles and the inherited other-text and default text styles.
const (
	renderListParents      = "a:lstStyle p:titleStyle p:bodyStyle p:otherStyle p:defaultTextStyle"
	renderParagraphParents = "a:pPr a:defPPr a:lvl1pPr a:lvl2pPr a:lvl3pPr a:lvl4pPr a:lvl5pPr a:lvl6pPr a:lvl7pPr a:lvl8pPr a:lvl9pPr"
	renderRunParents       = "a:rPr a:defRPr a:endParaRPr"
	renderLineParents      = "a:ln a:lnL a:lnR a:lnT a:lnB"
	renderStyleRefs        = "a:lnRef a:fillRef a:effectRef a:fontRef"
)

// renderColorTransforms are the EG_ColorTransform elements, all evaluated.
var renderColorTransforms = []string{"tint", "shade", "comp", "inv", "gray", "alpha", "alphaMod", "alphaOff", "hue", "hueMod", "hueOff", "sat", "satMod", "satOff",
	"lum", "lumMod", "lumOff", "red", "redMod", "redOff", "green", "greenMod", "greenOff", "blue", "blueMod", "blueOff", "gamma", "invGamma"}

func init() {
	for _, name := range renderColorTransforms {
		renderXMLParents["a:"+name] = "a:srgbClr a:schemeClr a:sysClr a:prstClr"
	}
	renderXMLParents["a:prstClr"] = renderXMLParents["a:srgbClr"]
}

var renderXMLParents = map[string]string{
	"p:txBody": "p:sp", "a:bodyPr": "p:txBody a:txBody", "a:lstStyle": "p:txBody a:txBody", "a:noAutofit": "a:bodyPr", "a:spAutoFit": "a:bodyPr", "a:normAutofit": "a:bodyPr",
	"a:p": "p:txBody a:txBody", "a:pPr": "a:p a:fld", "a:r": "a:p", "a:br": "a:p", "a:fld": "a:p", "a:rPr": "a:r a:br a:fld", "a:t": "a:r a:fld", "a:endParaRPr": "a:p",
	"a:defPPr": renderListParents, "a:lvl1pPr": renderListParents, "a:lvl2pPr": renderListParents, "a:lvl3pPr": renderListParents,
	"a:lvl4pPr": renderListParents, "a:lvl5pPr": renderListParents, "a:lvl6pPr": renderListParents, "a:lvl7pPr": renderListParents,
	"a:lvl8pPr": renderListParents, "a:lvl9pPr": renderListParents,
	"a:buNone": renderParagraphParents, "a:buChar": renderParagraphParents, "a:buFont": renderParagraphParents, "a:buFontTx": renderParagraphParents,
	"a:buClr": renderParagraphParents, "a:buClrTx": renderParagraphParents, "a:buSzPct": renderParagraphParents, "a:buSzPts": renderParagraphParents, "a:buSzTx": renderParagraphParents, "a:buAutoNum": renderParagraphParents, "a:buBlip": renderParagraphParents, "a:lnSpc": renderParagraphParents, "a:spcBef": renderParagraphParents, "a:spcAft": renderParagraphParents,
	"a:tabLst": renderParagraphParents, "a:defRPr": renderParagraphParents, "a:tab": "a:tabLst",
	"a:spcPct": "a:lnSpc a:spcBef a:spcAft", "a:spcPts": "a:lnSpc a:spcBef a:spcAft",
	"a:latin": renderRunParents, "a:ea": renderRunParents, "a:cs": renderRunParents, "a:sym": renderRunParents,
	"a:uLnTx": renderRunParents, "a:uFillTx": renderRunParents,
	"p:cSld": "p:sld p:sldMaster p:sldLayout", "p:spTree": "p:cSld", "p:bg": "p:cSld", "p:bgPr": "p:bg",
	"p:clrMapOvr": "p:sld p:sldLayout", "a:masterClrMapping": "p:clrMapOvr", "a:overrideClrMapping": "p:clrMapOvr", "p:bgRef": "p:bg", "p:hf": "p:sldMaster p:sldLayout",
	"p:nvGrpSpPr": "p:spTree p:grpSp", "p:grpSp": "p:spTree p:grpSp", "a:grpSpLocks": "p:cNvGrpSpPr", "p:grpSpPr": "p:spTree p:grpSp", "p:sp": "p:spTree p:grpSp", "p:pic": "p:spTree p:grpSp",
	"p:nvSpPr": "p:sp", "p:ph": "p:nvPr", "a:spLocks": "p:cNvSpPr", "p:cxnSp": "p:spTree p:grpSp", "p:nvCxnSpPr": "p:cxnSp", "p:cNvCxnSpPr": "p:nvCxnSpPr", "a:stCxn": "p:cNvCxnSpPr", "a:endCxn": "p:cNvCxnSpPr", "a:cxnSpLocks": "p:cNvCxnSpPr",
	"p:style": "p:cxnSp p:sp", "a:lnRef": "p:style", "a:fillRef": "p:style", "a:effectRef": "p:style", "a:fontRef": "p:style", "p:nvPicPr": "p:pic", "p:cNvPr": "p:nvSpPr p:nvPicPr p:nvGrpSpPr p:nvGraphicFramePr p:nvCxnSpPr",
	"p:graphicFrame": "p:spTree", "p:nvGraphicFramePr": "p:graphicFrame", "p:cNvGraphicFramePr": "p:nvGraphicFramePr", "a:graphicFrameLocks": "p:cNvGraphicFramePr",
	"p:xfrm": "p:graphicFrame", "a:graphic": "p:graphicFrame", "a:graphicData": "a:graphic", "a:tbl": "a:graphicData", "a:tblPr": "a:tbl", "a:tableStyleId": "a:tblPr", "a:tblGrid": "a:tbl",
	"a:gridCol": "a:tblGrid", "a:tr": "a:tbl", "a:tc": "a:tr", "a:txBody": "a:tc", "a:tcPr": "a:tc",
	"a:lnL": "a:tcPr", "a:lnR": "a:tcPr", "a:lnT": "a:tcPr", "a:lnB": "a:tcPr",
	"p:cNvSpPr": "p:nvSpPr", "p:cNvPicPr": "p:nvPicPr", "p:cNvGrpSpPr": "p:nvGrpSpPr",
	"p:nvPr": "p:nvSpPr p:nvPicPr p:nvGrpSpPr p:nvGraphicFramePr p:nvCxnSpPr", "p:spPr": "p:sp p:pic p:cxnSp", "p:blipFill": "p:pic",
	"a:xfrm": "p:spPr p:grpSpPr", "a:off": "a:xfrm p:xfrm", "a:ext": "a:xfrm p:xfrm", "a:chOff": "a:xfrm", "a:chExt": "a:xfrm",
	"a:prstGeom": "p:spPr", "a:avLst": "a:prstGeom a:custGeom a:prstTxWarp", "a:gd": "a:avLst a:gdLst", "a:prstTxWarp": "a:bodyPr",
	"a:custGeom": "p:spPr", "a:gdLst": "a:custGeom", "a:ahLst": "a:custGeom", "a:ahXY": "a:ahLst", "a:ahPolar": "a:ahLst", "a:cxnLst": "a:custGeom", "a:cxn": "a:cxnLst",
	"a:pos": "a:ahXY a:ahPolar a:cxn", "a:rect": "a:custGeom", "a:pathLst": "a:custGeom", "a:moveTo": "a:path", "a:lnTo": "a:path", "a:arcTo": "a:path",
	"a:quadBezTo": "a:path", "a:cubicBezTo": "a:path", "a:close": "a:path", "a:pt": "a:moveTo a:lnTo a:quadBezTo a:cubicBezTo",
	// Effect lists are admitted empty; no effect element is.
	"a:effectLst": "p:spPr p:bgPr " + renderRunParents,
	"a:prstDash":  renderLineParents, "a:round": renderLineParents, "a:bevel": renderLineParents, "a:miter": renderLineParents,
	"a:headEnd": renderLineParents, "a:tailEnd": renderLineParents, "a:noFill": "p:spPr p:bgPr a:tcPr " + renderLineParents + " " + renderRunParents,
	"a:solidFill": "p:spPr p:bgPr a:tcPr " + renderLineParents + " " + renderRunParents, "a:srgbClr": "a:solidFill a:gs p:bgRef a:highlight a:buClr " + renderStyleRefs, "a:schemeClr": "a:solidFill a:gs p:bgRef a:highlight a:buClr " + renderStyleRefs, "a:sysClr": "a:solidFill a:gs p:bgRef a:highlight a:buClr " + renderStyleRefs,
	"a:highlight": renderRunParents,
	"a:ln": "p:spPr " + renderRunParents,
	"a:gradFill": "p:spPr p:bgPr " + renderLineParents + " " + renderRunParents, "a:gsLst": "a:gradFill", "a:gs": "a:gsLst",
	"a:lin": "a:gradFill", "a:path": "a:gradFill a:pathLst", "a:fillToRect": "a:path", "a:tileRect": "a:gradFill",
	"a:picLocks": "p:cNvPicPr", "a:blip": "p:blipFill a:blipFill", "a:alphaModFix": "a:blip", "a:srcRect": "p:blipFill a:blipFill",
	"a:blipFill": "p:bgPr", "a:tile": "a:blipFill p:blipFill", "a:stretch": "p:blipFill a:blipFill", "a:fillRect": "a:stretch",
}
