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

func (f *slideRenderFonts) resolve(ctx context.Context, family string, bold, italic bool) (*shape.Face, error) {
	if f.opts.Fonts == nil {
		return nil, fmt.Errorf("%w: explicit font resolver required", render.ErrUnsupported)
	}
	if len(family) == 0 || len(family) > 1024 || strings.HasPrefix(family, "+") {
		return nil, fmt.Errorf("%w: explicit font family required", render.ErrUnsupported)
	}
	req := render.FontRequest{Family: family, Bold: bold, Italic: italic}
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
	anchor  enum.TextAnchor
	grows   bool
}

// renderBodyFrame applies DrawingML body defaults. A non-placeholder body
// inherits nothing: absent attributes take their schema defaults (top anchor,
// square wrap, 0.1"/0.05" insets, no autofit).
func renderBodyFrame(bp *dml.BodyPr) (renderFrame, error) {
	f := renderFrame{margins: TextMargins{Left: 91440, Top: 45720, Right: 91440, Bottom: 45720}, anchor: enum.TextAnchorTop}
	if bp == nil {
		return f, nil
	}
	switch enum.TextAnchor(bp.Anchor) {
	case "", enum.TextAnchorTop:
	case enum.TextAnchorMiddle, enum.TextAnchorBottom:
		f.anchor = enum.TextAnchor(bp.Anchor)
	default:
		return f, fmt.Errorf("%w: justified or distributed text anchoring", render.ErrUnsupported)
	}
	if bp.Wrap != "" && bp.Wrap != "square" {
		return f, fmt.Errorf("%w: text requires square wrapping", render.ErrUnsupported)
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

func (s *Slide) renderShapeText(ctx context.Context, index int, v *AutoShape, g renderGeometry, breaker *core.TextLayout, fonts *slideRenderFonts, styles *renderTextStyles) ([]layout.Op, error) {
	source := s.renderSourceShape(index)
	if source != nil && source.NvSpPr != nil && source.NvSpPr.NvPr != nil && source.NvSpPr.NvPr.Ph != nil {
		return nil, fmt.Errorf("%w: inherited placeholder text", render.ErrUnsupported)
	}
	saved := renderTextBody(source, v.textFrame)
	if saved == nil {
		return nil, nil
	}
	if len(saved.P) > fonts.nodes {
		return nil, fmt.Errorf("%w: text paragraphs", render.ErrLimit)
	}
	fonts.nodes -= len(saved.P)
	frame, err := renderBodyFrame(saved.BodyPr)
	if err != nil {
		return nil, err
	}
	if err = styles.load(); err != nil {
		return nil, err
	}
	// Text lays out in the preset's text rectangle.
	x, y := g.box[0]+g.text[0], g.box[1]+g.text[1]
	w, h := g.box[2]-g.text[0]-g.text[2], g.box[3]-g.text[1]-g.text[3]
	if w < 0 || h < 0 {
		return nil, fmt.Errorf("%w: text rectangle", render.ErrInvalid)
	}
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
	content := w - m.Left - m.Right
	contentTop := float64(y)/float64(dml.EMUsPerPixel) + float64(m.Top)/float64(dml.EMUsPerPixel)
	bottom := float64(y)/float64(dml.EMUsPerPixel) + float64(h-m.Bottom)/float64(dml.EMUsPerPixel)
	// Lay every paragraph out first: anchoring needs the text height.
	blocks := make([]renderBlock, 0, len(saved.P))
	height := 0.0
	for pi, p := range saved.P {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if p == nil {
			return nil, fmt.Errorf("%w: nil paragraph", render.ErrInvalid)
		}
		if len(p.Br) > 0 || len(p.Fld) > 0 {
			return nil, fmt.Errorf("%w: hard break or field", render.ErrUnsupported)
		}
		if len(p.R) > fonts.nodes {
			return nil, fmt.Errorf("%w: text runs", render.ErrLimit)
		}
		fonts.nodes -= len(p.R)
		para, layers, err := styles.paragraph(saved, p)
		if err != nil {
			return nil, err
		}
		// Whether PowerPoint adds space before a body's first paragraph
		// depends on spcFirstLastPara semantics this profile does not claim.
		if pi == 0 && para.before != 0 {
			return nil, fmt.Errorf("%w: space before the first paragraph", render.ErrUnsupported)
		}
		if para.marL > content || para.marR > content-para.marL || para.marL+para.marR == content {
			return nil, fmt.Errorf("%w: paragraph margins", render.ErrUnsupported)
		}
		left, ok := style.FromPx(float64(x+m.Left+para.marL) / float64(dml.EMUsPerPixel))
		if !ok {
			return nil, render.ErrLimit
		}
		width := renderUnit(content - para.marL - para.marR)
		// Runs may differ in paint but must shape alike, so the paragraph wraps
		// as one string and each line is cut back into runs to paint them.
		var (
			runs []renderRunStyle
			ends []int // byte offset after each run
			text strings.Builder
		)
		for _, r := range p.R {
			if r == nil {
				return nil, fmt.Errorf("%w: nil run", render.ErrInvalid)
			}
			rs, err := styles.run(layers, r.RPr)
			if err != nil {
				return nil, err
			}
			if len(runs) > 0 && rs.renderShaping != runs[0].renderShaping {
				return nil, fmt.Errorf("%w: mixed paragraph fonts or sizes", render.ErrUnsupported)
			}
			if len(r.T) > fonts.opts.Limits.MaxRunBytes-text.Len() {
				return nil, fmt.Errorf("%w: paragraph text", render.ErrLimit)
			}
			text.WriteString(r.T)
			runs = append(runs, rs)
			ends = append(ends, text.Len())
		}
		// The end-of-paragraph mark sizes an empty paragraph; a paragraph with
		// runs takes its line box from them, as LibreOffice's import does.
		var shaping renderShaping
		if len(runs) == 0 {
			end, err := styles.run(layers, p.EndParaRPr)
			if err != nil {
				return nil, err
			}
			shaping = end.renderShaping
		} else {
			shaping = runs[0].renderShaping
		}
		lines, metrics, err := renderParagraphLines(ctx, breaker, fonts, shaping, text.String(), width, para.lineSpacing)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, renderBlock{para: para, runs: runs, ends: ends, text: text.String(), lines: lines, metrics: metrics, left: left, width: width})
		height += float64(para.before+para.after)/float64(dml.EMUsPerPixel) + float64(len(lines))*metrics.lineHeight
	}
	// The text block spans its paragraphs' spacing and full line heights.
	top := contentTop
	switch frame.anchor {
	case enum.TextAnchorMiddle:
		top += (bottom - contentTop - height) / 2
	case enum.TextAnchorBottom:
		top = bottom - height
	}
	if top < contentTop && !frame.grows {
		return nil, fmt.Errorf("%w: text exceeds frame", render.ErrUnsupported)
	}
	var ops []layout.Op
	for _, b := range blocks {
		para, runs, ends, lines, metrics, left, width := b.para, b.runs, b.ends, b.lines, b.metrics, b.left, b.width
		top += float64(para.before) / float64(dml.EMUsPerPixel)
		start := 0
		for _, line := range lines {
			if top+metrics.ascent+metrics.descent > bottom && !frame.grows {
				return nil, fmt.Errorf("%w: text exceeds frame", render.ErrUnsupported)
			}
			if !strings.HasPrefix(b.text[start:], line.Text) {
				return nil, fmt.Errorf("%w: paragraph line text", render.ErrUnsupported)
			}
			if len(line.Glyphs) > 0 {
				xp := left.Px()
				if para.align == enum.TextAlignCenter {
					xp += (width.Px() - line.Width.Px()) / 2
				}
				if para.align == enum.TextAlignRight {
					xp += width.Px() - line.Width.Px()
				}
				drawn, err := renderLineRuns(line, start, ends, runs, xp, top, metrics)
				if err != nil {
					return nil, err
				}
				if len(drawn) > fonts.opts.Limits.MaxOperations-fonts.ops {
					return nil, fmt.Errorf("%w: text drawing operations", render.ErrLimit)
				}
				fonts.ops += len(drawn)
				ops = append(ops, drawn...)
			}
			start += len(line.Text)
			top += metrics.lineHeight
		}
		if start != len(b.text) {
			return nil, fmt.Errorf("%w: paragraph line text", render.ErrUnsupported)
		}
		top += float64(para.after) / float64(dml.EMUsPerPixel)
	}
	return ops, nil
}

// renderBlock is one laid-out paragraph awaiting vertical placement.
type renderBlock struct {
	para    renderParaStyle
	runs    []renderRunStyle
	ends    []int
	text    string
	lines   []core.TextLine
	metrics renderLineMetrics
	left    style.Unit
	width   style.Unit
}

// renderLineMetrics are one style's line box in CSS pixels.
type renderLineMetrics struct {
	size                        style.Unit
	ascent, descent, lineHeight float64
}

// renderParagraphLines wraps one uniformly styled paragraph. Empty text yields
// one empty line carrying the style's metrics.
func renderParagraphLines(ctx context.Context, breaker *core.TextLayout, fonts *slideRenderFonts, run renderShaping, text string, width style.Unit, lineSpacing int32) ([]core.TextLine, renderLineMetrics, error) {
	var m renderLineMetrics
	// ST_TextFontSize is 1 to 4000 points.
	if run.size < 100 || run.size > 400000 {
		return nil, m, fmt.Errorf("%w: font size", render.ErrUnsupported)
	}
	face, err := fonts.resolve(ctx, run.font, run.bold, run.italic)
	if err != nil {
		return nil, m, err
	}
	var ok bool
	if m.size, ok = style.FromPx(float64(run.size) / 100 * 4 / 3); !ok {
		return nil, m, render.ErrLimit
	}
	// kern is the smallest size PowerPoint kerns; absent or zero is off.
	features := shape.Features{NoKerning: run.kern == 0 || run.size < run.kern}
	// DrawingML's Latin font serves Latin, Greek and Cyrillic text alike.
	lines, err := breaker.Lines(ctx, face, text, m.size, width, features, core.RepertoireEuropean)
	if err != nil {
		return nil, m, err
	}
	if len(lines) == 0 {
		return nil, m, fmt.Errorf("%w: paragraph lines", render.ErrInvalid)
	}
	metrics := lines[0].Face.Descriptor()
	em := float64(lines[0].Face.UnitsPerEm())
	m.ascent = float64(metrics.Ascent) * m.size.Px() / em
	m.descent = -float64(metrics.Descent) * m.size.Px() / em
	m.lineHeight = (m.ascent + m.descent + float64(metrics.LineGap)*m.size.Px()/em) * float64(lineSpacing) / 100000
	if m.ascent <= 0 || m.descent < 0 || m.lineHeight <= 0 {
		return nil, m, fmt.Errorf("%w: font line metrics", render.ErrUnsupported)
	}
	return lines, m, nil
}

// renderLineRuns paints one wrapped line run by run: highlights first, merging
// touching spans of one color so they show no seam, then each run's glyphs in
// its color. start is the line's byte offset in the paragraph text and ends the
// offset after each run. PowerPoint shapes runs apart, so a glyph standing for
// characters of two runs, such as a ligature, fails.
func renderLineRuns(line core.TextLine, start int, ends []int, runs []renderRunStyle, x, top float64, m renderLineMetrics) ([]layout.Op, error) {
	runAt := func(offset int) int {
		i := 0
		for i < len(ends)-1 && offset >= ends[i] {
			i++
		}
		return i
	}
	yu, yok := style.FromPx(top + m.ascent)
	ty, tok := style.FromPx(top)
	hu, hok := style.FromPx(m.ascent + m.descent)
	if !yok || !tok || !hok {
		return nil, render.ErrLimit
	}
	var (
		highlights, glyphOps []layout.Op
		span                 renderHighlight
		spanFrom, spanTo     float64
	)
	flush := func() error {
		if span.on && spanTo > spanFrom {
			xu, xok := style.FromPx(x + spanFrom*m.size.Px()/1000)
			wu, wok := style.FromPx((spanTo - spanFrom) * m.size.Px() / 1000)
			if !xok || !wok {
				return render.ErrLimit
			}
			highlights = append(highlights, layout.FillRect{Rect: layout.Rect{X: xu, Y: ty, W: wu, H: hu}, Color: span.color})
		}
		span = renderHighlight{}
		return nil
	}
	pen := 0.0 // in 1000 units per em
	for i := 0; i < len(line.Glyphs); {
		first := line.Glyphs[i].Cluster
		if first < 0 || first >= len(line.Text) {
			return nil, fmt.Errorf("%w: glyph cluster", render.ErrInvalid)
		}
		r := runAt(start + first)
		j := i
		advance := 0.0
		for j < len(line.Glyphs) && runAt(start+line.Glyphs[j].Cluster) == r {
			advance += line.Glyphs[j].XAdvance
			j++
		}
		// The segment's characters run to the next segment's first cluster.
		segmentEnd := len(line.Text)
		if j < len(line.Glyphs) {
			segmentEnd = line.Glyphs[j].Cluster
		}
		if segmentEnd <= first || runAt(start+segmentEnd-1) != r {
			return nil, fmt.Errorf("%w: glyph spanning runs", render.ErrUnsupported)
		}
		if h := runs[r].highlight; h != span {
			if err := flush(); err != nil {
				return nil, err
			}
			span, spanFrom = h, pen
		}
		spanTo = pen + advance
		glyphs := make([]shape.Glyph, j-i)
		copy(glyphs, line.Glyphs[i:j])
		for k := range glyphs {
			glyphs[k].Cluster -= first
		}
		xu, xok := style.FromPx(x + pen*m.size.Px()/1000)
		if !xok {
			return nil, render.ErrLimit
		}
		glyphOps = append(glyphOps, layout.DrawGlyphs{At: layout.Point{X: xu, Y: yu}, Text: line.Text[first:segmentEnd], Glyphs: glyphs, Face: line.Face, Size: m.size, Color: runs[r].color})
		pen += advance
		i = j
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return append(highlights, glyphOps...), nil
}
