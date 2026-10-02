package pptx

import (
	"context"
	"encoding/xml"
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
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
			layers = append(layers, m.CSld)
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
			layers = append(layers, m.CSld)
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
	layers = append(layers, model.CSld)
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
	for i := len(layers) - 1; i >= 0; i-- {
		if layers[i] != nil && layers[i].Bg != nil {
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
	// A slide often repeats one image; decode and charge it once.
	decoded := map[renderImageKey]image.Image{}
	// drawShape paints one shape. sp is its parsed p:sp, if any; index is its
	// position among the slide's own shapes, or -1 for an inherited shape;
	// picture resolves a picture's image bytes.
	var drawShapeRef renderDraw
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
			if err = soft(renderTextLeftOut(sh, err)); err != nil {
				return nil, err
			}
			return append(drawn, text...), nil
		case *GroupShape:
			grp := s.renderSourceGroup(index)
			if grp == nil || v.isDirty() {
				return nil, fmt.Errorf("%w: new or edited group; save and reopen to preview it", render.ErrUnsupported)
			}
			return renderGroup(grp, renderIdentity, drawShapeRef, s.renderPartPicture(s.partName), 0, opts.Warn)
		case *PlaceholderShape:
			return s.renderPlaceholderShape(ctx, v, sp, colors, resolved, textLayout, fonts, styles, layoutProfile.shapeErrs, masterProfile.shapeErrs, masterProfile.styleErrs, soft)
		case *Table:
			return s.renderTable(ctx, index, v, colors, textLayout, fonts, styles)
		case *Connector:
			return s.renderConnector(index, v, colors, resolved)
		case *Picture:
			if picProps != nil {
				if err := renderPictureProperties(picProps); err != nil {
					return nil, err
				}
			}
			if v.isMedia || len(v.svgData) > 0 || v.svgRelID != "" {
				return nil, fmt.Errorf("%w: media or SVG picture", render.ErrUnsupported)
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
			if img, err = renderCrop(img, v.cropLeft, v.cropTop, v.cropRight, v.cropBottom); err != nil {
				return nil, err
			}
			x, y := v.Position()
			width, height := v.Size()
			return []layout.Op{layout.DrawImage{Rect: layout.Rect{X: renderUnit(x), Y: renderUnit(y), W: renderUnit(width), H: renderUnit(height)}, Image: img}}, nil
		}
		return nil, fmt.Errorf("%w: shape %T", render.ErrUnsupported, sh)
	}
	drawShapeRef = drawShape
	// Master shapes, then layout shapes, then the slide's own.
	for _, layer := range inherited {
		drawn, err := s.renderLayer(layer, budget, drawShape, opts.Warn)
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
	c, err := colors.color(renderColorOf(r.SrgbClr, r.SchemeClr, r.SysClr, r.ScrgbClr != nil, r.HslClr != nil, r.PrstClr != nil), nil)
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
	if p.CustGeom != nil || p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil || p.ExtLst != nil {
		return nil, g, fmt.Errorf("%w: shape fill or geometry", render.ErrUnsupported)
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
	if v.presetGeometry == "line" {
		// A line runs corner to corner, its flips choosing the corners.
		if p.Xfrm != nil && p.Xfrm.Rot != 0 {
			return nil, g, fmt.Errorf("%w: rotated line", render.ErrUnsupported)
		}
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
		px := float64(dml.EMUsPerPixel)
		ops, err := renderLineStroke(line, placeholder, colors, x0/px, y0/px, x1/px, y1/px, limits.MaxPathSegments)
		return ops, g, err
	}
	if p.Xfrm != nil && (p.Xfrm.Rot != 0 || p.Xfrm.FlipH || p.Xfrm.FlipV) {
		return nil, g, fmt.Errorf("%w: shape transformation", render.ErrUnsupported)
	}
	g, err := renderPresetGeometry(v.presetGeometry, p.PrstGeom, x, y, w, h)
	if err != nil {
		return nil, g, err
	}
	if p.NoFill != nil && (p.SolidFill != nil || p.GradFill != nil) || p.SolidFill != nil && p.GradFill != nil {
		return nil, g, fmt.Errorf("%w: ambiguous shape fill", render.ErrInvalid)
	}
	// Without a style reference (checked by the caller), an absent fill, an
	// absent outline, or an outline without a fill is none.
	var ops []layout.Op
	fill, grad := p.SolidFill, p.GradFill
	var fillPlaceholder *style.RGBA
	if st != nil && p.NoFill == nil && fill == nil && grad == nil {
		var err error
		if fill, grad, fillPlaceholder, err = renderStyleFill(st.FillRef, colors); err != nil {
			return nil, g, err
		}
	}
	if p.NoFill == nil && (fill != nil || grad != nil) {
		paint, err := colors.fillPaint(fill, grad, fillPlaceholder, float64(w)/float64(dml.EMUsPerPixel), float64(h)/float64(dml.EMUsPerPixel))
		if err != nil {
			return nil, g, err
		}
		if ops, err = g.fill(paint); err != nil {
			return nil, g, err
		}
	}
	line, linePlaceholder, err := renderStyledLine(st, p.Ln, colors)
	if err != nil {
		return nil, g, err
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
			attrs = "id name descr title"
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
		case "srgbClr", "schemeClr", "tint", "shade", "alpha", "alphaMod", "alphaOff", "hue", "hueMod", "hueOff", "sat", "satMod", "satOff",
			"lum", "lumMod", "lumOff", "red", "redMod", "redOff", "green", "greenMod", "greenOff", "blue", "blueMod", "blueOff":
			attrs = "val"
		case "comp", "inv", "gray", "gamma", "invGamma":
		case "sysClr":
			attrs = "val lastClr"
		case "picLocks":
			attrs = "noGrp noSelect noRot noChangeAspect noMove noResize noEditPoints noAdjustHandles noChangeArrowheads noChangeShapeType noCrop"
		case "blip":
			attrs = "cstate embed"
		case "fillRect", "srcRect", "fillToRect", "tileRect":
			attrs = "l t r b"
		case "gradFill":
			attrs = "flip rotWithShape"
		case "gs":
			attrs = "pos"
		case "lin":
			attrs = "ang scaled"
		case "path":
			attrs = "path"
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
	if ref.Kind == oxml.ChildPic && ref.Index >= 0 && ref.Index < len(t.Pic) {
		return t.Pic[ref.Index].SpPr
	}
	return nil
}
func renderPictureProperties(p *dml.SpPr) error {
	if p.CustGeom != nil || p.SolidFill != nil || p.GradFill != nil || p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil || renderEffects(p.EffectLst) || p.EffectDag != nil || p.Scene3d != nil || p.Sp3d != nil || p.ExtLst != nil {
		return fmt.Errorf("%w: picture shape properties", render.ErrUnsupported)
	}
	if p.Xfrm != nil && (p.Xfrm.Rot != 0 || p.Xfrm.FlipH || p.Xfrm.FlipV) {
		return fmt.Errorf("%w: picture transformation", render.ErrUnsupported)
	}
	if p.Ln != nil && (p.Ln.NoFill == nil || p.Ln.SolidFill != nil || p.Ln.GradFill != nil || p.Ln.PattFill != nil) {
		return fmt.Errorf("%w: picture outline", render.ErrUnsupported)
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
	"a:blip":      {list: nsA, uris: []string{xmlb.ExtURIUseLocalDpi}},
	"p:cSld":      {list: nsP, uris: []string{xmlb.ExtURIPMLCreationId}},
	"p:sld":       {list: nsP, uris: []string{xmlb.ExtURISldGuideLst}},
	"p:sldLayout": {list: nsP, uris: []string{xmlb.ExtURISldGuideLstLayout}},
	"p:sldMaster": {list: nsP, uris: []string{xmlb.ExtURISldGuideLstMaster}},
	"p:nvPr":      {list: nsP, uris: []string{xmlb.ExtURIPMLModId}},
	"a:gridCol":   {list: nsA, uris: []string{xmlb.ExtURIColId}},
	"a:tr":        {list: nsA, uris: []string{xmlb.ExtURIRowId}},
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
		(node.Name.Space == nsA && (node.Name.Local == "p" || node.Name.Local == "r" || node.Name.Local == "br" || node.Name.Local == "fld" || node.Name.Local == "tab" || node.Name.Local == "gd" || node.Name.Local == "gs" || node.Name.Local == "gridCol" || node.Name.Local == "tr" || node.Name.Local == "tc"))
	if node.Occurrence > 1 && !repeated {
		return fmt.Errorf("%w: repeated XML %s", render.ErrInvalid, node.Name.Local)
	}
	el := node.StartElement
	// A connector's or line's direction is its flips; the shape renderers
	// read them and reject flipped shapes they cannot draw.
	if n := len(node.Path); n >= 3 && el.Name == (xml.Name{Space: nsA, Local: "xfrm"}) && (node.Path[n-3] == (xml.Name{Space: nsP, Local: "cxnSp"}) || node.Path[n-3] == (xml.Name{Space: nsP, Local: "sp"})) {
		var attrs []xml.Attr
		for _, a := range el.Attr {
			if a.Name.Space != "" || (a.Name.Local != "flipH" && a.Name.Local != "flipV") {
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
		renderXMLParents["a:"+name] = "a:srgbClr a:schemeClr a:sysClr"
	}
}

var renderXMLParents = map[string]string{
	"p:txBody": "p:sp", "a:bodyPr": "p:txBody a:txBody", "a:lstStyle": "p:txBody a:txBody", "a:noAutofit": "a:bodyPr", "a:spAutoFit": "a:bodyPr", "a:normAutofit": "a:bodyPr",
	"a:p": "p:txBody a:txBody", "a:pPr": "a:p a:fld", "a:r": "a:p", "a:br": "a:p", "a:fld": "a:p", "a:rPr": "a:r a:br a:fld", "a:t": "a:r a:fld", "a:endParaRPr": "a:p",
	"a:defPPr": renderListParents, "a:lvl1pPr": renderListParents, "a:lvl2pPr": renderListParents, "a:lvl3pPr": renderListParents,
	"a:lvl4pPr": renderListParents, "a:lvl5pPr": renderListParents, "a:lvl6pPr": renderListParents, "a:lvl7pPr": renderListParents,
	"a:lvl8pPr": renderListParents, "a:lvl9pPr": renderListParents,
	"a:buNone": renderParagraphParents, "a:buChar": renderParagraphParents, "a:buFont": renderParagraphParents, "a:buFontTx": renderParagraphParents,
	"a:buClr": renderParagraphParents, "a:buClrTx": renderParagraphParents, "a:buSzPct": renderParagraphParents, "a:buSzPts": renderParagraphParents, "a:buSzTx": renderParagraphParents, "a:lnSpc": renderParagraphParents, "a:spcBef": renderParagraphParents, "a:spcAft": renderParagraphParents,
	"a:tabLst": renderParagraphParents, "a:defRPr": renderParagraphParents, "a:tab": "a:tabLst",
	"a:spcPct": "a:lnSpc a:spcBef a:spcAft", "a:spcPts": "a:lnSpc a:spcBef a:spcAft",
	"a:latin": renderRunParents, "a:ea": renderRunParents, "a:cs": renderRunParents, "a:sym": renderRunParents,
	"a:uLnTx": renderRunParents, "a:uFillTx": renderRunParents,
	"p:cSld": "p:sld p:sldMaster p:sldLayout", "p:spTree": "p:cSld", "p:bg": "p:cSld", "p:bgPr": "p:bg",
	"p:clrMapOvr": "p:sld p:sldLayout", "a:masterClrMapping": "p:clrMapOvr", "a:overrideClrMapping": "p:clrMapOvr", "p:bgRef": "p:bg", "p:hf": "p:sldMaster p:sldLayout",
	"p:nvGrpSpPr": "p:spTree p:grpSp", "p:grpSp": "p:spTree p:grpSp", "a:grpSpLocks": "p:cNvGrpSpPr", "p:grpSpPr": "p:spTree p:grpSp", "p:sp": "p:spTree p:grpSp", "p:pic": "p:spTree p:grpSp",
	"p:nvSpPr": "p:sp", "p:ph": "p:nvPr", "a:spLocks": "p:cNvSpPr", "p:cxnSp": "p:spTree", "p:nvCxnSpPr": "p:cxnSp", "p:cNvCxnSpPr": "p:nvCxnSpPr", "a:stCxn": "p:cNvCxnSpPr", "a:endCxn": "p:cNvCxnSpPr", "a:cxnSpLocks": "p:cNvCxnSpPr",
	"p:style": "p:cxnSp p:sp", "a:lnRef": "p:style", "a:fillRef": "p:style", "a:effectRef": "p:style", "a:fontRef": "p:style", "p:nvPicPr": "p:pic", "p:cNvPr": "p:nvSpPr p:nvPicPr p:nvGrpSpPr p:nvGraphicFramePr p:nvCxnSpPr",
	"p:graphicFrame": "p:spTree", "p:nvGraphicFramePr": "p:graphicFrame", "p:cNvGraphicFramePr": "p:nvGraphicFramePr", "a:graphicFrameLocks": "p:cNvGraphicFramePr",
	"p:xfrm": "p:graphicFrame", "a:graphic": "p:graphicFrame", "a:graphicData": "a:graphic", "a:tbl": "a:graphicData", "a:tblPr": "a:tbl", "a:tableStyleId": "a:tblPr", "a:tblGrid": "a:tbl",
	"a:gridCol": "a:tblGrid", "a:tr": "a:tbl", "a:tc": "a:tr", "a:txBody": "a:tc", "a:tcPr": "a:tc",
	"a:lnL": "a:tcPr", "a:lnR": "a:tcPr", "a:lnT": "a:tcPr", "a:lnB": "a:tcPr",
	"p:cNvSpPr": "p:nvSpPr", "p:cNvPicPr": "p:nvPicPr", "p:cNvGrpSpPr": "p:nvGrpSpPr",
	"p:nvPr": "p:nvSpPr p:nvPicPr p:nvGrpSpPr p:nvGraphicFramePr p:nvCxnSpPr", "p:spPr": "p:sp p:pic p:cxnSp", "p:blipFill": "p:pic",
	"a:xfrm": "p:spPr p:grpSpPr", "a:off": "a:xfrm p:xfrm", "a:ext": "a:xfrm p:xfrm", "a:chOff": "a:xfrm", "a:chExt": "a:xfrm",
	"a:prstGeom": "p:spPr", "a:avLst": "a:prstGeom", "a:gd": "a:avLst",
	// Effect lists are admitted empty; no effect element is.
	"a:effectLst": "p:spPr p:bgPr " + renderRunParents,
	"a:prstDash":  renderLineParents, "a:round": renderLineParents, "a:bevel": renderLineParents, "a:miter": renderLineParents,
	"a:headEnd": renderLineParents, "a:tailEnd": renderLineParents, "a:noFill": "p:spPr p:bgPr a:tcPr " + renderLineParents + " " + renderRunParents,
	"a:solidFill": "p:spPr p:bgPr a:tcPr " + renderLineParents + " " + renderRunParents, "a:srgbClr": "a:solidFill a:gs p:bgRef a:highlight a:buClr " + renderStyleRefs, "a:schemeClr": "a:solidFill a:gs p:bgRef a:highlight a:buClr " + renderStyleRefs, "a:sysClr": "a:solidFill a:gs p:bgRef a:highlight a:buClr " + renderStyleRefs,
	"a:highlight": renderRunParents,
	"a:ln": "p:spPr " + renderRunParents,
	"a:gradFill": "p:spPr p:bgPr " + renderLineParents + " " + renderRunParents, "a:gsLst": "a:gradFill", "a:gs": "a:gsLst",
	"a:lin": "a:gradFill", "a:path": "a:gradFill", "a:fillToRect": "a:path", "a:tileRect": "a:gradFill",
	"a:picLocks": "p:cNvPicPr", "a:blip": "p:blipFill", "a:srcRect": "p:blipFill", "a:stretch": "p:blipFill", "a:fillRect": "a:stretch",
}
