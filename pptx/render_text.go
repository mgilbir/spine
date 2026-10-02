package pptx

import (
	"context"
	"fmt"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

type slideRenderFonts struct {
	opts  render.Options
	faces map[render.FontRequest]*shape.Face
	nodes int
	ops   int
}

func newSlideRenderFonts(opts render.Options) *slideRenderFonts {
	nodes := opts.MaxLayoutNodes
	if nodes == 0 {
		nodes = 100000
	}
	return &slideRenderFonts{opts: opts, faces: make(map[render.FontRequest]*shape.Face), nodes: nodes}
}

func (f *slideRenderFonts) resolve(ctx context.Context, r *Run) (*shape.Face, error) {
	if f.opts.Fonts == nil {
		return nil, fmt.Errorf("%w: explicit font resolver required", render.ErrUnsupported)
	}
	if len(r.fontName) == 0 || len(r.fontName) > 1024 || strings.HasPrefix(r.fontName, "+") {
		return nil, fmt.Errorf("%w: explicit font family required", render.ErrUnsupported)
	}
	req := render.FontRequest{Family: r.fontName, Bold: r.bold, Italic: r.italic}
	if face := f.faces[req]; face != nil {
		return face, nil
	}
	if len(f.faces) >= f.opts.Limits.MaxFonts {
		return nil, fmt.Errorf("%w: font requests", render.ErrLimit)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	face, err := f.opts.Fonts(ctx, req)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if face == nil {
		return nil, fmt.Errorf("%w: unresolved font", render.ErrUnsupported)
	}
	f.faces[req] = face
	return face, nil
}

// renderSourceShape returns the parsed p:sp behind a shape, or nil for a shape
// added through the API, which a save writes from the domain model.
func (s *Slide) renderSourceShape(index int) *oxml.Shape {
	if index >= len(s.shapeRefs) {
		return nil
	}
	ref := s.shapeRefs[index]
	if ref.Kind != oxml.ChildSp || ref.Index < 0 || ref.Index >= len(s.sxModel.CSld.SpTree.Sp) {
		return nil
	}
	return s.sxModel.CSld.SpTree.Sp[ref.Index]
}

// renderTextBody returns the text body a save writes for a shape, without
// mutating the source: the parsed body for an untouched frame, the parsed body
// with the frame's edits flushed onto a copy, or the body of a new shape.
func renderTextBody(source *oxml.Shape, tf *TextFrame) *dml.TxBody {
	if source == nil {
		if tf == nil {
			return nil
		}
		return textFrameToOxml(tf)
	}
	if !tf.isDirty() {
		return source.TxBody
	}
	body := renderCopyTxBody(source.TxBody)
	updateTxBody(&body, tf)
	return body
}

// renderCopyTxBody copies the nodes updateTxBody may assign to: the body,
// body properties, paragraphs, paragraph properties, runs and run properties.
// Deeper nodes are shared; the flush replaces rather than edits them.
func renderCopyTxBody(src *dml.TxBody) *dml.TxBody {
	if src == nil {
		return nil
	}
	body := *src
	if src.BodyPr != nil {
		bp := *src.BodyPr
		body.BodyPr = &bp
	}
	body.P = make([]*dml.P, len(src.P))
	for i, p := range src.P {
		if p == nil {
			continue
		}
		cp := *p
		if p.PPr != nil {
			pp := *p.PPr
			cp.PPr = &pp
		}
		cp.R = make([]*dml.R, len(p.R))
		for j, r := range p.R {
			if r == nil {
				continue
			}
			cr := *r
			if r.RPr != nil {
				rp := *r.RPr
				cr.RPr = &rp
			}
			cp.R[j] = &cr
		}
		body.P[i] = &cp
	}
	return &body
}

// renderFrame is a text body's content inset and whether PowerPoint grows the
// shape, or shrinks the text, to keep the text inside it.
type renderFrame struct {
	margins TextMargins
	grows   bool
}

// renderBodyFrame applies DrawingML body defaults. A non-placeholder body
// inherits nothing: absent attributes take their schema defaults (top anchor,
// square wrap, 0.1"/0.05" insets, no autofit).
func renderBodyFrame(bp *dml.BodyPr) (renderFrame, error) {
	f := renderFrame{margins: TextMargins{Left: 91440, Top: 45720, Right: 91440, Bottom: 45720}}
	if bp == nil {
		return f, nil
	}
	if (bp.Anchor != "" && bp.Anchor != "t") || (bp.Wrap != "" && bp.Wrap != "square") {
		return f, fmt.Errorf("%w: text requires top anchoring and square wrapping", render.ErrUnsupported)
	}
	if (bp.Rot != nil && *bp.Rot != 0) || (bp.Vert != "" && bp.Vert != "horz") || bp.NumCol > 1 || (bp.VertOverflow != "" && bp.VertOverflow != "overflow") || (bp.HorzOverflow != "" && bp.HorzOverflow != "overflow") ||
		renderTrue(bp.UpRight) || renderTrue(bp.AnchorCtr) || renderTrue(bp.FromWordArt) || renderTrue(bp.CompatLnSpc) || bp.PrstTxWarp != nil || bp.Scene3d != nil || bp.Sp3d != nil || bp.FlatTx != nil || bp.ExtLst != nil {
		return f, fmt.Errorf("%w: text body property", render.ErrUnsupported)
	}
	for _, inset := range []struct {
		v   *int64
		out *dml.EMU
	}{{bp.LIns, &f.margins.Left}, {bp.TIns, &f.margins.Top}, {bp.RIns, &f.margins.Right}, {bp.BIns, &f.margins.Bottom}} {
		if inset.v != nil {
			*inset.out = dml.EMU(*inset.v)
		}
	}
	autofits := 0
	for _, set := range []bool{bp.NoAutofit != nil, bp.NormAutofit != nil, bp.SpAutoFit != nil} {
		if set {
			autofits++
		}
	}
	if autofits > 1 {
		return f, fmt.Errorf("%w: autofit choice", render.ErrInvalid)
	}
	if n := bp.NormAutofit; n != nil && ((n.FontScale.Int32() != 0 && n.FontScale.Int32() != 100000) || n.LnSpcReduction.Int32() != 0) {
		return f, fmt.Errorf("%w: scaled autofit text", render.ErrUnsupported)
	}
	// PowerPoint stores the extent and scale it fitted with. Its text fits
	// that extent, so measurement differences below it are not overflow.
	f.grows = bp.SpAutoFit != nil || bp.NormAutofit != nil
	return f, nil
}

func renderTrue(v *bool) bool { return v != nil && *v }

func (s *Slide) renderShapeText(ctx context.Context, index int, v *AutoShape, breaker *core.TextLayout, fonts *slideRenderFonts) ([]layout.Op, error) {
	tf := v.textFrame
	if len(tf.paragraphs) > fonts.nodes {
		return nil, fmt.Errorf("%w: text paragraphs", render.ErrLimit)
	}
	fonts.nodes -= len(tf.paragraphs)
	source := s.renderSourceShape(index)
	var body *dml.TxBody
	if source != nil {
		if source.NvSpPr != nil && source.NvSpPr.NvPr != nil && source.NvSpPr.NvPr.Ph != nil {
			return nil, fmt.Errorf("%w: inherited placeholder text", render.ErrUnsupported)
		}
		body = source.TxBody
	}
	saved := renderTextBody(source, tf)
	if saved == nil {
		return nil, nil
	}
	if v.presetGeometry != "rect" {
		return nil, fmt.Errorf("%w: text requires a rectangle", render.ErrUnsupported)
	}
	frame, err := renderBodyFrame(saved.BodyPr)
	if err != nil {
		return nil, err
	}
	x, y := v.Position()
	w, h := v.Size()
	m := frame.margins
	for _, inset := range []dml.EMU{m.Left, m.Right, m.Top, m.Bottom} {
		if inset < 0 {
			return nil, fmt.Errorf("%w: text inset", render.ErrInvalid)
		}
	}
	// Subtract in sequence, so hostile EMU values cannot overflow a sum.
	if m.Left > w || m.Top > h || m.Right > w-m.Left || m.Bottom > h-m.Top {
		// DrawingML permits insets wider than the shape; this profile cannot
		// lay text out in such a box.
		return nil, fmt.Errorf("%w: text content box", render.ErrUnsupported)
	}
	left, ok := style.FromPx(float64(x)/float64(dml.EMUsPerPixel) + float64(m.Left)/float64(dml.EMUsPerPixel))
	if !ok {
		return nil, render.ErrLimit
	}
	top := float64(y)/float64(dml.EMUsPerPixel) + float64(m.Top)/float64(dml.EMUsPerPixel)
	width := renderUnit(w - m.Left - m.Right)
	bottom := float64(y)/float64(dml.EMUsPerPixel) + float64(h-m.Bottom)/float64(dml.EMUsPerPixel)
	var ops []layout.Op
	for pi, p := range tf.paragraphs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if p == nil || len(p.runs) == 0 || p.level != 0 || p.bulletType != BulletNone || p.marL != nil || p.indent != nil || len(p.tabStops) != 0 || p.lineSpacing <= 0 || p.spaceBefore < 0 || p.spaceAfter < 0 {
			return nil, fmt.Errorf("%w: plain paragraph requires explicit spacing and no bullet", render.ErrUnsupported)
		}
		if len(p.runs) > fonts.nodes {
			return nil, fmt.Errorf("%w: text runs", render.ErrLimit)
		}
		fonts.nodes -= len(p.runs)
		if p.alignment != enum.TextAlignLeft && p.alignment != enum.TextAlignCenter && p.alignment != enum.TextAlignRight {
			return nil, fmt.Errorf("%w: paragraph alignment", render.ErrUnsupported)
		}
		var originalParagraph *dml.PPr
		if !tf.contentDirty && body != nil && pi < len(body.P) && body.P[pi] != nil {
			originalParagraph = body.P[pi].PPr
		}
		if (!p.isSet(paraPropSpaceBefore) && (originalParagraph == nil || originalParagraph.SpcBef == nil)) || (!p.isSet(paraPropSpaceAfter) && (originalParagraph == nil || originalParagraph.SpcAft == nil)) {
			return nil, fmt.Errorf("%w: explicit paragraph before/after spacing required", render.ErrUnsupported)
		}
		if body != nil && pi < len(body.P) && body.P[pi] != nil && body.P[pi].PPr != nil {
			pp := body.P[pi].PPr
			if (pp.SpcBef != nil && pp.SpcBef.SpcPct != nil) || (pp.SpcAft != nil && pp.SpcAft.SpcPct != nil) {
				return nil, fmt.Errorf("%w: percentage paragraph spacing", render.ErrUnsupported)
			}
			sp := pp.LnSpc
			if sp != nil && sp.SpcPts != nil {
				return nil, fmt.Errorf("%w: fixed paragraph spacing", render.ErrUnsupported)
			}
		}
		first := p.runs[0]
		var text strings.Builder
		for ri, r := range p.runs {
			if r == nil || r.fontSize < 12 || r.fontSize > 4096 || r.color == nil || r.color.Type != dml.ColorTypeRGB || r.color.Tint != 0 || r.highlight != nil || r.baseline != 0 || r.hyperlink != nil || r.underline != enum.UnderlineNone || r.strike != enum.StrikeNone {
				return nil, fmt.Errorf("%w: explicit plain run style required", render.ErrUnsupported)
			}
			var original *dml.RPr
			if !tf.contentDirty && body != nil && pi < len(body.P) && body.P[pi] != nil && ri < len(body.P[pi].R) && body.P[pi].R[ri] != nil {
				original = body.P[pi].R[ri].RPr
			}
			if (!r.isSet(runPropBold) && (original == nil || original.B == nil)) || (!r.isSet(runPropItalic) && (original == nil || original.I == nil)) {
				return nil, fmt.Errorf("%w: explicit bold and italic required", render.ErrUnsupported)
			}
			if r.fontName != first.fontName || r.fontSize != first.fontSize || r.bold != first.bold || r.italic != first.italic || *r.color != *first.color {
				return nil, fmt.Errorf("%w: mixed paragraph styles", render.ErrUnsupported)
			}
			if len(r.text) > fonts.opts.Limits.MaxRunBytes-text.Len() {
				return nil, fmt.Errorf("%w: paragraph text", render.ErrLimit)
			}
			text.WriteString(r.text)
		}
		face, err := fonts.resolve(ctx, first)
		if err != nil {
			return nil, err
		}
		size, ok := style.FromPx(first.fontSize * 4 / 3)
		if !ok {
			return nil, render.ErrLimit
		}
		lines, err := breaker.PlainLines(ctx, face, text.String(), size, width)
		if err != nil {
			return nil, err
		}
		metrics := lines[0].Face.Descriptor()
		em := float64(lines[0].Face.UnitsPerEm())
		ascent := float64(metrics.Ascent) * size.Px() / em
		descent := -float64(metrics.Descent) * size.Px() / em
		lineHeight := (ascent + descent + float64(metrics.LineGap)*size.Px()/em) * float64(p.lineSpacing) / 100000
		if ascent <= 0 || descent < 0 || lineHeight <= 0 {
			return nil, fmt.Errorf("%w: font line metrics", render.ErrUnsupported)
		}
		top += float64(p.spaceBefore) / float64(dml.EMUsPerPixel)
		if len(lines) > fonts.opts.Limits.MaxOperations-fonts.ops {
			return nil, fmt.Errorf("%w: text drawing operations", render.ErrLimit)
		}
		fonts.ops += len(lines)
		for _, line := range lines {
			if top+ascent+descent > bottom && !frame.grows {
				return nil, fmt.Errorf("%w: text exceeds frame", render.ErrUnsupported)
			}
			xp := left.Px()
			if p.alignment == enum.TextAlignCenter {
				xp += (width.Px() - line.Width.Px()) / 2
			}
			if p.alignment == enum.TextAlignRight {
				xp += width.Px() - line.Width.Px()
			}
			xu, xok := style.FromPx(xp)
			yu, yok := style.FromPx(top + ascent)
			if !xok || !yok {
				return nil, render.ErrLimit
			}
			var fill dml.SpPr
			dml.NewSolidFill(*first.color).ApplyToSpPr(&fill)
			color, err := renderSolid(fill.SolidFill)
			if err != nil {
				return nil, err
			}
			ops = append(ops, layout.DrawGlyphs{At: layout.Point{X: xu, Y: yu}, Text: line.Text, Glyphs: line.Glyphs, Face: line.Face, Size: size, Color: color})
			top += lineHeight
		}
		top += float64(p.spaceAfter) / float64(dml.EMUsPerPixel)
	}
	return ops, nil
}
