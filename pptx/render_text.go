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

func (s *Slide) renderTextSource(index int) *oxml.Shape {
	if index >= len(s.shapeRefs) {
		return nil
	}
	ref := s.shapeRefs[index]
	if ref.Kind != oxml.ChildSp || ref.Index < 0 || ref.Index >= len(s.sxModel.CSld.SpTree.Sp) {
		return nil
	}
	return s.sxModel.CSld.SpTree.Sp[ref.Index]
}

func (s *Slide) renderShapeText(ctx context.Context, index int, v *AutoShape, breaker *core.TextLayout, fonts *slideRenderFonts) ([]layout.Op, error) {
	tf := v.textFrame
	if len(tf.paragraphs) > fonts.nodes {
		return nil, fmt.Errorf("%w: text paragraphs", render.ErrLimit)
	}
	fonts.nodes -= len(tf.paragraphs)
	if v.presetGeometry != "rect" || tf.anchor != enum.TextAnchorTop || tf.wrap != enum.TextWrappingSquare || tf.autofit != AutofitNone {
		return nil, fmt.Errorf("%w: text requires a top-anchored wrapping rectangle without autofit", render.ErrUnsupported)
	}
	source := s.renderTextSource(index)
	var body *dml.TxBody
	if source != nil {
		if source.NvSpPr != nil && source.NvSpPr.NvPr != nil && source.NvSpPr.NvPr.Ph != nil {
			return nil, fmt.Errorf("%w: inherited placeholder text", render.ErrUnsupported)
		}
		body = source.TxBody
	}
	if body != nil && !tf.marginsDirty {
		bp := body.BodyPr
		if bp == nil || bp.LIns == nil || bp.TIns == nil || bp.RIns == nil || bp.BIns == nil {
			return nil, fmt.Errorf("%w: explicit text insets required", render.ErrUnsupported)
		}
	}
	x, y := v.Position()
	w, h := v.Size()
	m := tf.margins
	for _, inset := range []dml.EMU{m.Left, m.Right, m.Top, m.Bottom} {
		if inset < 0 {
			return nil, fmt.Errorf("%w: text inset", render.ErrInvalid)
		}
	}
	// Subtract in sequence, so hostile EMU values cannot overflow a sum.
	if m.Left > w || m.Top > h || m.Right > w-m.Left || m.Bottom > h-m.Top {
		return nil, fmt.Errorf("%w: text content box", render.ErrInvalid)
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
			if top+ascent+descent > bottom {
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
