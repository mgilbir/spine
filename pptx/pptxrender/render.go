package pptxrender

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/mgilbir/spine/pptx"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/pptx/internal/presetgeom"
	"github.com/mgilbir/spine/render"
)

// prepare implements PrepareSlide.
func (s *renderSlide) prepare(ctx context.Context, opts render.Options) (*render.Page, error) {
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
	rootAnimated, err := renderRootAlternates(model.AlternateContent)
	if err = soft(err); err != nil {
		return nil, err
	}
	if (model.Timing != nil || model.Transition != nil || rootAnimated) && !lenient {
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
			// Root alternate content holds transitions; they are not drawn.
			if _, err = renderRootAlternates(m.AlternateContent); err != nil {
				if err = soft(err); err != nil {
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
			// Root alternate content holds transitions; they are not drawn.
			if _, err = renderRootAlternates(m.AlternateContent); err != nil {
				if err = soft(err); err != nil {
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
	colors := &renderColors{ctx: ctx, slide: s, budget: budget, limits: resolved}
	styles := &renderTextStyles{ctx: ctx, slide: s, budget: budget, colors: colors, masterErrs: masterProfile.styleErrs, warn: opts.Warn}
	// The nearest defined background wins: slide, then layout, then master.
	background := renderPaint{color: renderWhite}
	px := float64(dml.EMUsPerPixel)
	var (
		bgImage      image.Image
		bgImageBytes int64
	)
	// Best effort reports a background drawn approximately.
	if lenient {
		colors.approx = func(err error) {
			if ctx.Err() == nil {
				opts.Warn(fmt.Errorf("pptx: background: %w: %w", render.ErrApproximated, err))
			}
		}
	}
	for i := len(layers) - 1; i >= 0; i-- {
		if layers[i] != nil && layers[i].Bg != nil {
			if bp := layers[i].Bg.BgPr; bp != nil && bp.BlipFill != nil {
				if bgImage, bgImageBytes, err = s.renderBackgroundImage(ctx, parts[i], bp, float64(w)/px, float64(h)/px, resolved, colors); err != nil {
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
	colors.approx = nil
	if err = soft(renderTreeBase(model.CSld, budget, true)); err != nil {
		return nil, err
	}
	shapes := s.shapeList()
	if len(shapes) > budget.Nodes || len(shapes) >= resolved.MaxOperations {
		return nil, fmt.Errorf("%w: slide shapes", render.ErrLimit)
	}
	budget.Nodes -= len(shapes)
	// A gradient or pattern background lies over white, which shows through
	// its alpha.
	ops := []layout.Op{layout.FillRect{Rect: layout.Rect{W: renderUnit(w), H: renderUnit(h)}, Color: background.color}}
	if background.gradient != nil || background.image != nil {
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
	// A slide often repeats one image; decode and charge it once. Every
	// drawing of it counts toward the image budget.
	decoded := map[renderImageKey]image.Image{}
	loadImage := func(data []byte, key renderImageKey) (image.Image, error) {
		if len(data) == 0 {
			return nil, fmt.Errorf("%w: missing picture data", render.ErrInvalid)
		}
		if imageCount >= resolved.MaxImages {
			return nil, fmt.Errorf("%w: slide image budget", render.ErrLimit)
		}
		imageCount++
		if img := decoded[key]; img != nil {
			return img, nil
		}
		if int64(len(data)) > resolved.MaxImageBytes-imageBytes || imagePixels >= resolved.MaxImagePixels {
			return nil, fmt.Errorf("%w: slide image budget", render.ErrLimit)
		}
		imageBytes += int64(len(data))
		decodeLimits := resolved
		decodeLimits.MaxImagePixels = resolved.MaxImagePixels - imagePixels
		img, err := core.DecodeImage(ctx, data, decodeLimits)
		if err != nil {
			return nil, err
		}
		imagePixels += int64(img.Bounds().Dx()) * int64(img.Bounds().Dy())
		decoded[key] = img
		return img, nil
	}
	// drawChartPart draws a chart part over a frame: its Vega specification
	// drawn by the caller's chart renderer, as a picture under the slide's
	// image budget.
	drawChartPart := func(name, part string, x, y, cw, ch dml.EMU) ([]layout.Op, error) {
		if opts.Charts == nil {
			return nil, fmt.Errorf("%w: chart without a chart renderer", render.ErrUnsupported)
		}
		if cw <= 0 || ch <= 0 {
			return nil, fmt.Errorf("%w: chart extent", render.ErrInvalid)
		}
		src := s.presentation.otherParts[part]
		if src == nil || len(src.Data) == 0 {
			return nil, fmt.Errorf("%w: missing chart part", render.ErrInvalid)
		}
		if err := budget.CheckXML(ctx, src.Data, func(core.XMLNode) error { return nil }); err != nil {
			return nil, err
		}
		if lenient {
			prev := colors.approx
			seen := map[string]bool{}
			colors.approx = func(err error) {
				if ctx.Err() == nil && !seen[err.Error()] {
					seen[err.Error()] = true
					opts.Warn(fmt.Errorf("pptx: chart %q: %w: %w", name, render.ErrApproximated, err))
				}
			}
			defer func() { colors.approx = prev }()
		}
		plan, err := renderChartPart(src.Data, colors)
		if err != nil {
			return nil, err
		}
		pw, ph := float64(cw)/px, float64(ch)/px
		spec, err := plan.spec(pw, ph)
		if err != nil {
			return nil, err
		}
		// The chart is asked for at a scale the slide's image budget holds,
		// before the renderer allocates it: a frame may be any size.
		if imageCount >= resolved.MaxImages {
			return nil, fmt.Errorf("%w: slide image budget", render.ErrLimit)
		}
		scale := renderChartScale
		if area, room := pw*ph, float64(resolved.MaxImagePixels-imagePixels); area*scale*scale > room {
			scale = math.Sqrt(room / area)
			if !(scale >= renderMinChartScale) {
				return nil, fmt.Errorf("%w: chart frame too large for the slide image budget", render.ErrLimit)
			}
		}
		img, err := opts.Charts(ctx, spec, scale)
		if err != nil {
			return nil, fmt.Errorf("%w: chart renderer: %w", render.ErrUnsupported, err)
		}
		if img == nil || img.Bounds().Empty() {
			return nil, fmt.Errorf("%w: chart renderer returned no image", render.ErrInvalid)
		}
		pixels := int64(img.Bounds().Dx()) * int64(img.Bounds().Dy())
		if imageCount >= resolved.MaxImages || pixels > resolved.MaxImagePixels-imagePixels {
			return nil, fmt.Errorf("%w: slide image budget", render.ErrLimit)
		}
		imageCount++
		imagePixels += pixels
		img = renderDownscale(img, pw*renderMaxImageScale, ph*renderMaxImageScale)
		return []layout.Op{layout.DrawImage{Rect: layout.Rect{X: renderUnit(x), Y: renderUnit(y), W: renderUnit(cw), H: renderUnit(ch)}, Image: img}}, nil
	}
	drawChart := func(gf *oxml.GraphicFrame, part string) ([]layout.Op, error) {
		if gf.Xfrm == nil || gf.Xfrm.Off == nil || gf.Xfrm.Ext == nil {
			return nil, fmt.Errorf("%w: chart frame geometry", render.ErrUnsupported)
		}
		if gf.Xfrm.Rot != 0 || gf.Xfrm.FlipH || gf.Xfrm.FlipV {
			return nil, fmt.Errorf("%w: turned chart frame", render.ErrUnsupported)
		}
		return drawChartPart(renderFrameName(gf), part, dml.EMU(gf.Xfrm.Off.X), dml.EMU(gf.Xfrm.Off.Y), dml.EMU(gf.Xfrm.Ext.Cx), dml.EMU(gf.Xfrm.Ext.Cy))
	}
	// framesFor paints the graphic frames of one part: its tables, and its
	// charts, whose parts its relationships name.
	framesFor := func(part string) renderFrameDraw {
		return func(gf *oxml.GraphicFrame) ([]layout.Op, error) {
			if gf == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil {
				return nil, fmt.Errorf("%w: graphic frame", render.ErrInvalid)
			}
			if id := chartRelIDOf(gf); id != "" {
				return drawChart(gf, s.renderPartTarget(part, id))
			}
			if gf.Graphic.GraphicData.URI != oxml.TableGraphicDataURI {
				return nil, fmt.Errorf("%w: diagram or embedded object", render.ErrUnsupported)
			}
			if lenient {
				prev := colors.approx
				seen := map[string]bool{}
				name := renderFrameName(gf)
				colors.approx = func(err error) {
					if ctx.Err() == nil && !seen[err.Error()] {
						seen[err.Error()] = true
						opts.Warn(fmt.Errorf("pptx: table %q: %w: %w", name, render.ErrApproximated, err))
					}
				}
				defer func() { colors.approx = prev }()
			}
			return renderTableFrame(ctx, gf, colors, textLayout, fonts, styles)
		}
	}
	// drawShape paints one shape. sp is its parsed p:sp, if any; index is its
	// position among the slide's own shapes, or -1 for an inherited shape;
	// picture resolves a picture's image bytes.
	var (
		drawShapeRef renderDraw
		connect      renderConnect
	)
	drawShape := func(sh pptx.Shape, sp *oxml.Shape, picProps *dml.SpPr, index int, picture renderPictureSource) ([]layout.Op, error) {
		switch v := sh.(type) {
		case *pptx.AutoShape:
			if v == nil {
				return nil, fmt.Errorf("%w: nil shape", render.ErrInvalid)
			}
		case *pptx.TextBox:
			if v == nil {
				return nil, fmt.Errorf("%w: nil text box", render.ErrInvalid)
			}
		case *pptx.Picture:
			if v == nil {
				return nil, fmt.Errorf("%w: nil picture", render.ErrInvalid)
			}
		case *pptx.Table:
			if v == nil || index < 0 {
				return nil, fmt.Errorf("%w: nil or inherited table", render.ErrUnsupported)
			}
		case *pptx.Connector:
			if v == nil || index < 0 {
				return nil, fmt.Errorf("%w: nil or inherited connector", render.ErrUnsupported)
			}
		case *pptx.PlaceholderShape:
			// A grouped placeholder comes with its parsed, mapped source.
			if v == nil || (index < 0 && sp == nil) {
				return nil, fmt.Errorf("%w: nil or inherited placeholder", render.ErrUnsupported)
			}
		case *pptx.GroupShape:
			if v == nil {
				return nil, fmt.Errorf("%w: nil group", render.ErrInvalid)
			}
		case *pptx.ChartFrame:
			// A chart added since the slide was read.
			if v == nil || index < 0 {
				return nil, fmt.Errorf("%w: nil or inherited chart", render.ErrUnsupported)
			}
			x, y := v.Position()
			cw, ch := v.Size()
			return drawChartPart(v.Name(), shapeState(v).Part, x, y, cw, ch)
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
					opts.Warn(fmt.Errorf("pptx: shape %q: %w: %w", sh.Name(), render.ErrApproximated, err))
				}
			}
			defer func() { colors.approx = prev }()
		}
		// A shape's picture fill comes from its own part.
		prevPicture := colors.picture
		colors.picture = func(embed string) (image.Image, []byte, error) {
			if picture == nil {
				return nil, nil, fmt.Errorf("%w: picture fill without its part", render.ErrUnsupported)
			}
			data, key := picture(embed, nil)
			img, err := loadImage(data, key)
			return img, data, err
		}
		defer func() { colors.picture = prevPicture }()
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
		if _, ok := sh.(*pptx.Picture); ok && (index < 0 || picProps != nil) && (picProps == nil || picProps.Xfrm == nil || picProps.Xfrm.Off == nil || picProps.Xfrm.Ext == nil) {
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
		case *pptx.TextBox:
			// A text box is written as a rectangle unless its source says
			// otherwise.
			preset := "rect"
			if props != nil && props.PrstGeom != nil {
				preset = props.PrstGeom.Prst
			}
			box := boxOf(v, preset)
			drawn, geometry, err := renderAutoShape(box, props, st, colors, resolved)
			if err != nil || box.textFrame == nil {
				return drawn, err
			}
			text, err := renderShapeText(ctx, sp, box, geometry, textLayout, fonts, styles, nil)
			if err = soft(renderTextLeftOut(sh, err)); err != nil {
				return nil, err
			}
			return append(drawn, text...), nil
		case *pptx.AutoShape:
			box := boxOf(v, "")
			drawn, geometry, err := renderAutoShape(box, props, st, colors, resolved)
			if err != nil || box.textFrame == nil {
				return drawn, err
			}
			text, err := renderShapeText(ctx, sp, box, geometry, textLayout, fonts, styles, nil)
			if err = soft(renderTextLeftOut(sh, err)); err != nil {
				return nil, err
			}
			return append(drawn, text...), nil
		case *pptx.GroupShape:
			grp := s.renderSourceGroup(index)
			if grp == nil || shapeState(v).Dirty {
				return nil, fmt.Errorf("%w: new or edited group; save and reopen to preview it", render.ErrUnsupported)
			}
			return renderGroup(grp, renderIdentity, drawShapeRef, connect, framesFor(s.partName), s.renderPartPicture(s.partName), false, 0, opts.Warn, colors, resolved.MaxPathSegments, renderGroupFill{})
		case *pptx.PlaceholderShape:
			return s.renderPlaceholderShape(ctx, v, sp, colors, resolved, textLayout, fonts, styles, layoutProfile.shapeErrs, masterProfile.shapeErrs, masterProfile.styleErrs, soft)
		case *pptx.Table:
			return s.renderTable(ctx, index, v, colors, textLayout, fonts, styles)
		case *pptx.Connector:
			return s.renderConnector(index, v, colors, resolved)
		case *pptx.Picture:
			pst := shapeState(v)
			if picProps != nil {
				if err := renderPictureProperties(picProps, colors); err != nil {
					return nil, err
				}
			}
			// An SVG picture draws its raster fallback, as Office versions
			// without SVG support show it.
			if pst.Media {
				return nil, fmt.Errorf("%w: media picture", render.ErrUnsupported)
			}
			img, err := loadImage(picture(pst.RelID, v))
			if err != nil {
				return nil, err
			}
			// A negative crop extends the picture past its image, which is
			// then laid over the extended box and clipped to the picture's.
			extended := pst.Crop[0] < 0 || pst.Crop[1] < 0 || pst.Crop[2] < 0 || pst.Crop[3] < 0
			if !extended {
				if img, err = renderCrop(img, pst.Crop[0], pst.Crop[1], pst.Crop[2], pst.Crop[3]); err != nil {
					return nil, err
				}
			}
			if len(pst.Effects) > 0 {
				drawnWidth, _ := v.Size()
				if picProps != nil && picProps.Xfrm != nil && picProps.Xfrm.Ext != nil {
					drawnWidth = dml.EMU(picProps.Xfrm.Ext.Cx)
				}
				if img, err = colors.blipEffects(img, pst.Effects, float64(drawnWidth)/float64(max(1, img.Bounds().Dx()))); err != nil {
					return nil, err
				}
			} else if pst.Opacity != nil && *pst.Opacity < 1 {
				// A picture given an opacity through the API.
				img = renderFade(img, math.Max(0, *pst.Opacity))
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
				sx, sy := 1-pst.Crop[0]-pst.Crop[2], 1-pst.Crop[1]-pst.Crop[3]
				if sx <= 0 || sy <= 0 {
					return nil, fmt.Errorf("%w: empty picture crop", render.ErrInvalid)
				}
				w, h := rect.W.Px()/sx, rect.H.Px()/sy
				ix, okX := style.FromPx(rect.X.Px() - pst.Crop[0]*w)
				iy, okY := style.FromPx(rect.Y.Px() - pst.Crop[1]*h)
				iw, okW := style.FromPx(w)
				ih, okH := style.FromPx(h)
				if !okX || !okY || !okW || !okH {
					return nil, fmt.Errorf("%w: picture crop", render.ErrLimit)
				}
				full := layout.Rect{X: ix, Y: iy, W: iw, H: ih}
				img = renderDownscale(img, w*renderMaxImageScale, h*renderMaxImageScale)
				clipped := layout.DrawImage{Rect: full, Image: img, Clip: layout.Clip{Active: true, Rect: rect}}
				return renderPictureEffects(append([]layout.Op{clipped}, outline...), picProps, colors, resolved)
			}
			img = renderDownscale(img, rect.W.Px()*renderMaxImageScale, rect.H.Px()*renderMaxImageScale)
			return renderPictureEffects(append([]layout.Op{layout.DrawImage{Rect: rect, Image: img}}, outline...), picProps, colors, resolved)
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
					opts.Warn(fmt.Errorf("pptx: connector %q: %w: %w", name, render.ErrApproximated, err))
				}
			}
			defer func() { colors.approx = prev }()
		}
		return renderConnectorSource(c, colors, resolved)
	}
	// Master shapes, then layout shapes, then the slide's own.
	for _, layer := range inherited {
		drawn, err := s.renderLayer(layer, budget, drawShape, connect, framesFor, opts.Warn, colors, resolved.MaxPathSegments)
		if err != nil {
			return nil, err
		}
		ops = append(ops, drawn...)
	}
	// A picture holds its image; a picture fill names it by relationship.
	partPicture := s.renderPartPicture(s.partName)
	slidePicture := func(relID string, v *pptx.Picture) ([]byte, renderImageKey) {
		if v != nil {
			if data := v.Data(); len(data) > 0 {
				return data, renderPictureKey(v, data)
			}
		}
		return partPicture(relID, v)
	}
	// Alternate content draws its fallback, and a chart frame its chart,
	// where it stands among the shapes: after the last shape before it.
	type renderExtra struct{ pos, alt, frame int }
	extrasAfter := map[int][]renderExtra{}
	if t := model.CSld.SpTree; t != nil && (len(t.AltContent) > 0 || len(t.GraphicFrame) > 0) {
		pos := map[oxml.ChildRef]int{}
		for i, ref := range t.ChildOrder() {
			pos[ref] = i
		}
		place := func(ref oxml.ChildRef, e renderExtra) {
			at, ok := pos[ref]
			if !ok {
				at = len(pos)
			}
			e.pos = at
			after := -1
			for i := range shapes {
				if i < len(s.shapeRefs) {
					if p, ok := pos[s.shapeRefs[i]]; ok && p < at {
						after = i
					}
				}
			}
			extrasAfter[after] = append(extrasAfter[after], e)
		}
		for ai := range t.AltContent {
			place(oxml.ChildRef{Kind: oxml.ChildAltContent, Index: ai}, renderExtra{alt: ai, frame: -1})
		}
		for fi, gf := range t.GraphicFrame {
			if chartRelIDOf(gf) != "" && !renderHiddenChild(nil, nil, nil, nil, t.GraphicFrame, oxml.ChildRef{Kind: oxml.ChildGraphicFrame, Index: fi}) {
				place(oxml.ChildRef{Kind: oxml.ChildGraphicFrame, Index: fi}, renderExtra{alt: -1, frame: fi})
			}
		}
		for _, list := range extrasAfter {
			sort.SliceStable(list, func(i, j int) bool { return list[i].pos < list[j].pos })
		}
	}
	drawAlternates := func(after int) error {
		for _, e := range extrasAfter[after] {
			var (
				drawn []layout.Op
				err   error
			)
			if e.frame >= 0 {
				gf := model.CSld.SpTree.GraphicFrame[e.frame]
				drawn, err = drawChart(gf, s.relTargetPart(chartRelIDOf(gf)))
				if err != nil {
					err = fmt.Errorf("pptx: slide %d chart %q: %w", s.index, renderFrameName(gf), err)
				}
			} else if drawn, err = s.renderAlternate(e.alt, slideProfile.shapeErrs, budget, drawShape, connect, framesFor, opts.Warn, colors, resolved.MaxPathSegments); err != nil {
				err = fmt.Errorf("pptx: slide %d alternate content %d: %w", s.index, e.alt, err)
			}
			if err != nil {
				if err = soft(err); err != nil {
					return err
				}
				continue
			}
			ops = append(ops, drawn...)
		}
		return nil
	}
	if err = drawAlternates(-1); err != nil {
		return nil, err
	}
	for i, sh := range shapes {
		if i > 0 {
			if err = drawAlternates(i - 1); err != nil {
				return nil, err
			}
		}
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
	if len(shapes) > 0 {
		if err = drawAlternates(len(shapes) - 1); err != nil {
			return nil, err
		}
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
func (s *renderSlide) renderSourceHidden(index int) bool {
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
func renderPictureKey(v *pptx.Picture, data []byte) renderImageKey {
	if st := shapeState(v); !st.HasImageData && st.ImagePart != "" {
		return renderImageKey{part: st.ImagePart}
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
func renderTextLeftOut(sh pptx.Shape, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("pptx: shape %q: text left out: %w", sh.Name(), err)
}

// renderRefKey names a slide shape's spTree element for the source check's
// per-shape records.
func renderRefKey(s *renderSlide, i int) renderShapeKey {
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
	if len(t.RawXML) > 0 {
		return fmt.Errorf("%w: raw drawing content", render.ErrUnsupported)
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
	// Tables and charts are drawn, or reported, frame by frame.
	for _, gf := range t.GraphicFrame {
		if gf == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil || (gf.Graphic.GraphicData.URI != oxml.TableGraphicDataURI && chartRelIDOf(gf) == "") {
			return fmt.Errorf("%w: diagram or embedded object", render.ErrUnsupported)
		}
	}
	return nil
}

// st is the shape's style reference, if any: its fill applies when the shape
// sets none, and its line beneath the shape's own.
func renderAutoShape(v *renderBox, source *dml.SpPr, st *dml.Style, colors *renderColors, limits render.Limits) ([]layout.Op, renderGeometry, error) {
	ops, g, err := renderAutoShapeOps(v, source, st, colors, limits)
	if err != nil {
		return nil, g, err
	}
	// The shape's own effects win over inherited ones, and both over its
	// style's.
	own, dag, threeD := v.spPr.EffectLst, v.spPr.EffectDag != nil, v.spPr.Scene3d != nil || v.spPr.Sp3d != nil
	if source != nil {
		if !renderEffects(own) {
			own = source.EffectLst
		}
		dag, threeD = dag || source.EffectDag != nil, threeD || source.Scene3d != nil || source.Sp3d != nil
	}
	ops, err = renderShapeEffects(ops, own, dag, threeD, st, colors, limits.MaxOperations)
	return ops, g, err
}

// renderAutoShapeOps draws a shape without its effects.
func renderAutoShapeOps(v *renderBox, source *dml.SpPr, st *dml.Style, colors *renderColors, limits render.Limits) ([]layout.Op, renderGeometry, error) {
	var g renderGeometry
	p := &v.spPr
	if source != nil {
		if source.Xfrm == nil || source.Xfrm.Off == nil || source.Xfrm.Ext == nil {
			return nil, g, fmt.Errorf("%w: inherited/missing shape geometry", render.ErrUnsupported)
		}
		copyProps := *source
		if source.Ln != nil {
			line := *source.Ln
			copyProps.Ln = &line
		}
		applyShapeStyle(&copyProps, &v.spPr)
		p = &copyProps
	}
	if p.GrpFill != nil {
		return nil, g, fmt.Errorf("%w: shape group fill", render.ErrUnsupported)
	}
	if err := renderDMLExtensions(p.ExtLst, "p:spPr"); err != nil {
		return nil, g, err
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
	g.textTurn = xf.rot
	if xf.flipV {
		g.textTurn += 180
	}
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
		turn := g.textTurn
		ops, g, err := renderCustomShape(p.CustGeom, x, y, w, h, paint, filled, line, linePlaceholder, colors, limits)
		if err != nil {
			return nil, g, err
		}
		g.textTurn = turn
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
		turn := g.textTurn
		ops, g, err := renderCustomShape(&cg, x, y, w, h, paint, filled, line, linePlaceholder, colors, limits)
		if err != nil {
			return nil, g, err
		}
		g.textTurn = turn
		ops, err = xf.ops(ops, colors, limits.MaxPathSegments)
		return ops, g, err
	}
	turn := g.textTurn
	g, err = renderPresetGeometry(v.presetGeometry, p.PrstGeom, x, y, w, h)
	if err != nil {
		return nil, g, err
	}
	g.textTurn = turn
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

// pictureFill resolves a picture fill over a box w by h pixels: stretched
// over its fill rectangle, or tiled. Its fixed opacity is drawn, and its
// other color effects left out, as pictures draw them.
func (c *renderColors) pictureFill(f *dml.BlipFillXML, w, h float64) (renderPaint, error) {
	// The source check admits the blip's local-DPI and SVG extensions; an
	// SVG picture draws its raster fallback.
	if f.Blip == nil || f.Blip.Embed == "" || f.Blip.Link != "" || c.picture == nil {
		return renderPaint{}, fmt.Errorf("%w: linked picture fill", render.ErrUnsupported)
	}
	if f.Tile != nil && f.Stretch != nil {
		return renderPaint{}, fmt.Errorf("%w: picture fill both tiled and stretched", render.ErrInvalid)
	}
	if f.Tile == nil && f.Stretch == nil {
		if err := c.approximate(fmt.Errorf("%w: picture fill without a fill mode drawn stretched", render.ErrUnsupported)); err != nil {
			return renderPaint{}, err
		}
	}
	img, data, err := c.picture(f.Blip.Embed)
	if err != nil {
		return renderPaint{}, err
	}
	if r := f.SrcRect; r != nil {
		l, t, rr, b := float64(r.L.Int32())/100000, float64(r.T.Int32())/100000, float64(r.R.Int32())/100000, float64(r.B.Int32())/100000
		if l < 0 || t < 0 || rr < 0 || b < 0 {
			// A negative source inset extends the picture with transparency.
			if err := c.approximate(fmt.Errorf("%w: picture fill extended past its image drawn unextended", render.ErrUnsupported)); err != nil {
				return renderPaint{}, err
			}
			l, t, rr, b = math.Max(l, 0), math.Max(t, 0), math.Max(rr, 0), math.Max(b, 0)
		}
		if img, err = renderCrop(img, l, t, rr, b); err != nil {
			return renderPaint{}, err
		}
	}
	if img, err = c.blipEffects(img, f.Blip.Effects, w*float64(dml.EMUsPerPixel)/float64(max(1, img.Bounds().Dx()))); err != nil {
		return renderPaint{}, err
	}
	paint := renderPaint{image: img}
	if f.Tile != nil {
		// A tiled fill ignores the fill rectangle; its tiles cover the box.
		tw, th := renderTileSize(img, data, f.Dpi, f.Tile)
		paint.image, err = renderTileImage(c, img, w, h, tw, th, f.Tile)
		return paint, err
	}
	if f.Stretch != nil && f.Stretch.FillRect != nil {
		r := f.Stretch.FillRect
		paint.fill = [4]float64{float64(r.L.Int32()) / 100000, float64(r.T.Int32()) / 100000, float64(r.R.Int32()) / 100000, float64(r.B.Int32()) / 100000}
	}
	return paint, nil
}

// renderShapePaint resolves a shape's fill, its own or its style's; filled is
// false for none.
func renderShapePaint(p *dml.SpPr, st *dml.Style, colors *renderColors, w, h dml.EMU) (paint renderPaint, filled bool, err error) {
	set := 0
	for _, f := range []bool{p.NoFill != nil, p.SolidFill != nil, p.GradFill != nil, p.BlipFill != nil, p.PattFill != nil} {
		if f {
			set++
		}
	}
	if set > 1 {
		return paint, false, fmt.Errorf("%w: ambiguous shape fill", render.ErrInvalid)
	}
	if p.PattFill != nil {
		px := float64(dml.EMUsPerPixel)
		paint, err = colors.patternPaint(p.PattFill, nil, float64(w)/px, float64(h)/px)
		return paint, err == nil, err
	}
	if p.BlipFill != nil {
		px := float64(dml.EMUsPerPixel)
		paint, err = colors.pictureFill(p.BlipFill, float64(w)/px, float64(h)/px)
		return paint, err == nil, err
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
			// Font alignment within a uniformly sized line, and tab sizes for
			// text without tabs, do not change this profile's text; the
			// resolver reads East Asian breaking and hanging punctuation and
			// checks the rest.
			attrs = "marL marR lvl indent algn defTabSz rtl eaLnBrk fontAlgn latinLnBrk hangingPunct"
		case "rPr", "defRPr", "endParaRPr":
			// Proofing, smart-tag, bookmark and East Asian vertical attributes
			// do not change the painting of horizontal text; the resolver
			// reads the language, which picks an East Asian font, and checks
			// the rest.
			attrs = "kumimoji lang altLang sz b i u strike kern cap spc normalizeH baseline noProof dirty err smtClean smtId bmk"
		case "latin", "ea", "cs", "sym":
			// European text uses the Latin font, East Asian text the East
			// Asian font and right-to-left text the complex-script font; the
			// symbol slot is not consulted.
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
		case "pattFill":
			attrs = "prst"
		case "fgClr", "bgClr":
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
		case "avLst", "noFill", "solidFill", "grpFill", "stretch", "masterClrMapping", "highlight", "effectLst", "clrFrom", "clrTo":
		default:
			return fmt.Errorf("%w: XML %s", render.ErrUnsupported, el.Name.Local)
		}
	case oxml.ChartGraphicDataURI:
		// A chart frame names its chart part, which the chart renderer reads.
		if el.Name.Local != "chart" {
			return fmt.Errorf("%w: XML %s", render.ErrUnsupported, el.Name.Local)
		}
		for _, a := range el.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				continue
			}
			if a.Name.Space != nsR || a.Name.Local != "id" {
				return fmt.Errorf("%w: XML chart/@%s", render.ErrUnsupported, a.Name.Local)
			}
		}
		return nil
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

func (s *renderSlide) renderPictureProps(index int) *dml.SpPr {
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
// by its source rectangle, to stretch over a slide w by h pixels: stretched
// itself, or tiled.
func (s *renderSlide) renderBackgroundImage(ctx context.Context, part string, bp *oxml.BackgroundProps, w, h float64, limits render.Limits, colors *renderColors) (image.Image, int64, error) {
	f := bp.BlipFill
	if bp.SolidFill != nil || bp.GradFill != nil || bp.PattFill != nil || bp.NoFill != nil || renderEffects(bp.EffectLst) || bp.ExtLst != nil || f.Blip == nil || f.Blip.Embed == "" || f.Blip.Link != "" || len(f.Blip.Effects) > 0 {
		return nil, 0, fmt.Errorf("%w: background picture", render.ErrUnsupported)
	}
	if f.Tile != nil && f.Stretch != nil {
		return nil, 0, fmt.Errorf("%w: background picture both tiled and stretched", render.ErrInvalid)
	}
	if f.Tile == nil && f.Stretch == nil {
		if err := colors.approximate(fmt.Errorf("%w: background picture without a fill mode drawn stretched", render.ErrUnsupported)); err != nil {
			return nil, 0, err
		}
	}
	data, _ := s.renderPartPicture(part)(f.Blip.Embed, nil)
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
	if f.Tile != nil {
		var dpi *int32
		if f.Dpi != nil {
			v := int32(*f.Dpi)
			dpi = &v
		}
		tw, th := renderTileSize(img, data, dpi, f.Tile)
		if img, err = renderTileImage(colors, img, w, h, tw, th, f.Tile); err != nil {
			return nil, 0, err
		}
	}
	return img, int64(len(data)), nil
}

// renderPictureEffects draws a picture's own effects with it.
func renderPictureEffects(ops []layout.Op, p *dml.SpPr, colors *renderColors, limits render.Limits) ([]layout.Op, error) {
	if p == nil {
		return ops, nil
	}
	return renderShapeEffects(ops, p.EffectLst, p.EffectDag != nil, p.Scene3d != nil || p.Sp3d != nil, nil, colors, limits.MaxOperations)
}

// renderAlternate draws an alternate content's fallback shapes, which the
// source check verified as if they stood in the shape tree.
func (s *renderSlide) renderAlternate(index int, shapeErrs map[renderShapeKey]renderShapeErrs, budget *core.SourceBudget, draw renderDraw, connect renderConnect, frames func(string) renderFrameDraw, warn func(error), colors *renderColors, maxSegments int) ([]layout.Op, error) {
	if err := shapeErrs[renderShapeKey{name: "AlternateContent", occurrence: index + 1}].any; err != nil {
		return nil, err
	}
	return s.renderAlternateIn(s.sxModel.CSld.SpTree.AltContent[index], s.partName, budget, draw, connect, frames, warn, colors, maxSegments)
}

// renderAlternateIn draws an alternate content's fallback in a part: a
// slide's, or a layout's or master's, whose placeholders are prompts.
func (s *renderSlide) renderAlternateIn(ac *oxml.AlternateContent, part string, budget *core.SourceBudget, draw renderDraw, connect renderConnect, frames func(string) renderFrameDraw, warn func(error), colors *renderColors, maxSegments int) ([]layout.Op, error) {
	if ac == nil || !ac.HasFallback || len(ac.Fallback) == 0 {
		return nil, nil
	}
	// The fallback's prefixes are declared on the slide root; declare the
	// presentation, drawing and relationship ones around it. The checked
	// parse rejects any other prefix the fallback leaves unbound.
	var src bytes.Buffer
	src.WriteString(`<p:spTree xmlns:p="` + nsP + `" xmlns:a="` + nsA + `" xmlns:r="` + nsR + `">`)
	src.Write(ac.Fallback)
	src.WriteString(`</p:spTree>`)
	var tree oxml.ShapeTree
	if err := xmlb.Unmarshal(src.Bytes(), &tree); err != nil {
		return nil, fmt.Errorf("%w: alternate content fallback: %w", render.ErrInvalid, err)
	}
	return s.renderLayer(renderInherited{data: &oxml.CommonSlideData{SpTree: &tree}, part: part}, budget, draw, connect, frames, warn, colors, maxSegments)
}

// alternate checks a shape tree's markup-compatibility alternate content:
// only its fallback is drawn, as a reader without the choices' extensions
// shows it, so choices are not checked and fallback content is checked as
// if it stood in the shape tree. A problem is recorded against the
// alternate content, or in strict mode fails.
func (r *renderProfile) alternate(node core.XMLNode) (bool, error) {
	return r.alternateIn(node, false)
}

// alternateIn is alternate; with recorded set, as for a layout's or
// master's content, which fails only when drawn, a problem is always
// recorded against the alternate content.
func (r *renderProfile) alternateIn(node core.XMLNode, recorded bool) (bool, error) {
	n := len(node.Path)
	mc := func(local string) xml.Name { return xml.Name{Space: xmlb.NSMarkupCompatibility, Local: local} }
	if n < 4 || node.Path[2] != (xml.Name{Space: nsP, Local: "spTree"}) || node.Path[3] != mc("AlternateContent") {
		return false, nil
	}
	fail := func(err error) error {
		if err == nil || (!r.lenient && !recorded) {
			return err
		}
		if r.shapeErrs == nil {
			r.shapeErrs = map[renderShapeKey]renderShapeErrs{}
		}
		if e := r.shapeErrs[r.current]; e.any == nil {
			r.shapeErrs[r.current] = renderShapeErrs{any: err, inherited: err}
		}
		return nil
	}
	switch {
	case n == 4:
		if !node.Text {
			r.current = renderShapeKey{name: "AlternateContent", occurrence: node.Occurrence}
		}
		return true, nil
	case node.Path[4] == mc("Choice"):
		return true, nil
	case node.Path[4] != mc("Fallback"):
		return true, fail(fmt.Errorf("%w: XML %s in alternate content", render.ErrUnsupported, node.Path[4].Local))
	case n == 5:
		return true, nil
	}
	v := node
	v.Path = append(append([]xml.Name{}, node.Path[:3]...), node.Path[5:]...)
	return true, fail(r.check(v))
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
	// Each source row is read once and summed into the target columns it
	// spans; channels are averaged premultiplied, as compositing would.
	read := renderRowReader(img)
	row := make([]uint32, 4*b.Dx())
	acc := make([]uint64, 4*tw)
	spans := make([][2]int, tw)
	for x := range spans {
		spans[x] = [2]int{x * b.Dx() / tw, max((x+1)*b.Dx()/tw, x*b.Dx()/tw+1)}
	}
	for y := 0; y < th; y++ {
		y0, y1 := y*b.Dy()/th, max((y+1)*b.Dy()/th, y*b.Dy()/th+1)
		clear(acc)
		for sy := y0; sy < y1; sy++ {
			read(sy, row)
			for x, sp := range spans {
				a := acc[4*x : 4*x+4]
				for sx := sp[0]; sx < sp[1]; sx++ {
					a[0] += uint64(row[4*sx])
					a[1] += uint64(row[4*sx+1])
					a[2] += uint64(row[4*sx+2])
					a[3] += uint64(row[4*sx+3])
				}
			}
		}
		p := out.Pix[y*out.Stride:]
		for x, sp := range spans {
			n := uint64((sp[1] - sp[0]) * (y1 - y0))
			a := acc[4*x : 4*x+4]
			renderUnpremultiply(p[4*x:4*x+4], uint32(a[0]/n), uint32(a[1]/n), uint32(a[2]/n), uint32(a[3]/n))
		}
	}
	return out
}

// renderFade multiplies an image's opacity.
func renderFade(img image.Image, alpha float64) image.Image {
	out := renderNRGBA(img)
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = uint8(math.Round(float64(out.Pix[i]) * alpha))
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
	src := renderNRGBA(img)
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
			copy(out.Pix[out.PixOffset(dx, dy):out.PixOffset(dx, dy)+4], src.Pix[src.PixOffset(sx, sy):])
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
	"p:cNvPr": {list: nsA, uris: []string{xmlb.ExtURICreationId, xmlb.ExtURIDecorative}},
	// A picture with an SVG keeps a raster fallback, which is drawn.
	"a:blip": {list: nsA, uris: []string{xmlb.ExtURIUseLocalDpi, xmlb.ExtURISvgBlip}},
	// Hidden fills, lines and effects are kept for older editors and never
	// shown; the text box flag and shadow obscuring paint nothing here.
	"p:spPr":      {list: nsA, uris: []string{xmlb.ExtURIHiddenFill, xmlb.ExtURIHiddenLine, xmlb.ExtURIHiddenEffects, xmlb.ExtURIShadowObscured, renderExtURIWrappingTextBox}},
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
	if r.rootAlternate(node) {
		return nil
	}
	if handled, err := r.alternate(node); handled {
		return err
	}
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

// rootAlternate skips alternate content at a part's root, whose fallback
// renderRootAlternates checks: it holds transitions and animation, which a
// static page does not show.
func (r *renderProfile) rootAlternate(node core.XMLNode) bool {
	if r.skipped(node) {
		return true
	}
	if len(node.Path) == 2 && !node.Text && node.Name == (xml.Name{Space: xmlb.NSMarkupCompatibility, Local: "AlternateContent"}) {
		r.skipDepth = 2
		return true
	}
	return false
}

// renderRootAlternates checks a part's root alternate content: a reader
// without the choices' extensions reads the fallbacks, and fallbacks of
// transitions and animation only, or none, change nothing on a static page.
// animated reports such content; other fallback content is unsupported.
func renderRootAlternates(acs []*oxml.AlternateContent) (animated bool, err error) {
	for _, ac := range acs {
		if ac == nil {
			continue
		}
		animated = true
		if !ac.HasFallback || len(ac.Fallback) == 0 {
			continue
		}
		var src bytes.Buffer
		src.WriteString(`<f xmlns:p="` + nsP + `" xmlns:a="` + nsA + `" xmlns:r="` + nsR + `">`)
		src.Write(ac.Fallback)
		src.WriteString(`</f>`)
		d := xml.NewDecoder(&src)
		depth := 0
		for {
			tok, e := d.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				return animated, fmt.Errorf("%w: root alternate content fallback: %w", render.ErrInvalid, e)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				depth++
				if depth == 2 && (t.Name.Space != nsP || (t.Name.Local != "transition" && t.Name.Local != "timing")) {
					return animated, fmt.Errorf("%w: root alternate content holding %s", render.ErrUnsupported, t.Name.Local)
				}
			case xml.EndElement:
				depth--
			}
		}
	}
	return animated, nil
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
	// A duotone's two colors may be of one kind.
	if n := len(node.Path); n >= 2 && node.Path[n-2] == (xml.Name{Space: nsA, Local: "duotone"}) && node.Occurrence == 2 {
		repeated = true
	}
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
	// A picture effect's attributes are its own, apart from those of color
	// transforms that share its name.
	if n := len(node.Path); n >= 2 && node.Path[n-2] == (xml.Name{Space: nsA, Local: "blip"}) && el.Name.Space == nsA {
		attrs := map[string]string{"alphaModFix": "amt", "alphaBiLevel": "thresh", "biLevel": "thresh", "alphaRepl": "a", "lum": "bright contrast", "tint": "hue amt",
			"hsl": "hue sat lum", "clrChange": "useA", "fillOverlay": "blend", "blur": "rad grow", "alphaCeiling": "", "alphaFloor": "", "alphaInv": "", "clrRepl": "",
			"duotone": "", "grayscl": "", "extLst": ""}
		allowed, ok := attrs[el.Name.Local]
		if !ok {
			return fmt.Errorf("%w: picture effect %s", render.ErrUnsupported, el.Name.Local)
		}
		for _, a := range el.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				continue
			}
			if a.Name.Space != "" || !strings.Contains(" "+allowed+" ", " "+a.Name.Local+" ") {
				return fmt.Errorf("%w: XML %s/@%s", render.ErrUnsupported, el.Name.Local, a.Name.Local)
			}
		}
		if el.Name.Local != "extLst" {
			return nil
		}
	}
	// A shape's picture fill draws its fill rectangle.
	if n := len(node.Path); n >= 4 && el.Name == (xml.Name{Space: nsA, Local: "fillRect"}) && node.Path[n-4] == (xml.Name{Space: nsP, Local: "spPr"}) {
		for _, a := range el.Attr {
			if a.Name.Space != "" || !strings.Contains(" l t r b ", " "+a.Name.Local+" ") {
				return fmt.Errorf("%w: XML fillRect/@%s", render.ErrUnsupported, a.Name.Local)
			}
		}
		el.Attr = nil
	}
	return slideRenderXML(el)
}
func (r *renderProfile) inherited(node core.XMLNode) error {
	if r.rootAlternate(node) {
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
	// and a hidden layer's shapes are not drawn either. Alternate content
	// is checked as the slide's, its problems recorded likewise.
	if handled, err := r.alternateIn(node, true); handled {
		return err
	}
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
	if n.Space == oxml.ChartGraphicDataURI {
		return "c:" + n.Local
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
	// Brightness and tint are also picture effects.
	renderXMLParents["a:lum"] += " a:blip"
	renderXMLParents["a:tint"] += " a:blip"
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
	"p:graphicFrame": "p:spTree p:grpSp", "p:nvGraphicFramePr": "p:graphicFrame", "p:cNvGraphicFramePr": "p:nvGraphicFramePr", "a:graphicFrameLocks": "p:cNvGraphicFramePr",
	"p:xfrm": "p:graphicFrame", "a:graphic": "p:graphicFrame", "a:graphicData": "a:graphic", "a:tbl": "a:graphicData", "c:chart": "a:graphicData", "a:tblPr": "a:tbl", "a:tableStyleId": "a:tblPr", "a:tblGrid": "a:tbl",
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
	"a:headEnd": renderLineParents, "a:tailEnd": renderLineParents, "a:noFill": "p:spPr p:grpSpPr p:bgPr a:tcPr " + renderLineParents + " " + renderRunParents,
	"a:solidFill": "p:spPr p:grpSpPr p:bgPr a:tcPr a:fillOverlay " + renderLineParents + " " + renderRunParents, "a:srgbClr": "a:solidFill a:gs a:fgClr a:bgClr a:duotone a:clrRepl a:alphaInv a:clrFrom a:clrTo p:bgRef a:highlight a:buClr " + renderStyleRefs, "a:schemeClr": "a:solidFill a:gs a:fgClr a:bgClr a:duotone a:clrRepl a:alphaInv a:clrFrom a:clrTo p:bgRef a:highlight a:buClr " + renderStyleRefs, "a:sysClr": "a:solidFill a:gs a:fgClr a:bgClr a:duotone a:clrRepl a:alphaInv a:clrFrom a:clrTo p:bgRef a:highlight a:buClr " + renderStyleRefs,
	"a:highlight": renderRunParents,
	"a:ln":        "p:spPr " + renderRunParents, "a:grpFill": "p:spPr p:grpSpPr",
	"a:gradFill": "p:spPr p:grpSpPr p:bgPr a:fillOverlay " + renderLineParents + " " + renderRunParents, "a:pattFill": "p:spPr p:bgPr " + renderLineParents + " " + renderRunParents, "a:fgClr": "a:pattFill", "a:bgClr": "a:pattFill", "a:gsLst": "a:gradFill", "a:gs": "a:gsLst",
	"a:lin": "a:gradFill", "a:path": "a:gradFill a:pathLst", "a:fillToRect": "a:path", "a:tileRect": "a:gradFill",
	"a:picLocks": "p:cNvPicPr", "a:blip": "p:blipFill a:blipFill", "a:alphaModFix": "a:blip", "a:alphaBiLevel": "a:blip", "a:alphaCeiling": "a:blip", "a:alphaFloor": "a:blip", "a:alphaInv": "a:blip", "a:alphaRepl": "a:blip",
	"a:biLevel": "a:blip", "a:blur": "a:blip", "a:clrChange": "a:blip", "a:clrFrom": "a:clrChange", "a:clrTo": "a:clrChange", "a:clrRepl": "a:blip", "a:duotone": "a:blip",
	"a:fillOverlay": "a:blip", "a:grayscl": "a:blip", "a:hsl": "a:blip", "a:srcRect": "p:blipFill a:blipFill",
	"a:blipFill": "p:bgPr p:spPr", "a:tile": "a:blipFill p:blipFill", "a:stretch": "p:blipFill a:blipFill", "a:fillRect": "a:stretch",
}
