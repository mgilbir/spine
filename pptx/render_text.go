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

// renderShapeText lays out and paints a shape's text. ph is the inheritance
// of a placeholder, and nil for any other shape.
func renderShapeText(ctx context.Context, source *oxml.Shape, v *AutoShape, g renderGeometry, breaker *core.TextLayout, fonts *slideRenderFonts, styles *renderTextStyles, ph *renderPlaceholder) ([]layout.Op, error) {
	if ph == nil && source != nil && source.NvSpPr != nil && source.NvSpPr.NvPr != nil && source.NvSpPr.NvPr.Ph != nil {
		return nil, fmt.Errorf("%w: placeholder text without its inheritance", render.ErrUnsupported)
	}
	saved := renderTextBody(source, v.textFrame)
	if saved == nil || !renderHasText(saved) {
		// A body without characters paints nothing.
		return nil, nil
	}
	if len(saved.P) > fonts.nodes {
		return nil, fmt.Errorf("%w: text paragraphs", render.ErrLimit)
	}
	fonts.nodes -= len(saved.P)
	bodyPr := saved.BodyPr
	if ph != nil {
		bodyPr = ph.bodyPr(saved.BodyPr)
	}
	frame, err := renderBodyFrame(bodyPr)
	if err != nil {
		return nil, err
	}
	if err = styles.load(); err != nil {
		return nil, err
	}
	chain := styles.shapeChain()
	if ph != nil {
		chain = ph.chain(styles)
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
	blocks, height, err := renderLayoutParagraphs(ctx, saved, x+m.Left, content, breaker, fonts, styles, chain)
	if err != nil {
		return nil, err
	}
	return renderPlaceParagraphs(blocks, height, contentTop, bottom, frame.anchor, frame.grows, fonts)
}

// renderLayoutParagraphs wraps a text body's paragraphs in a content box of
// the given left edge and width, returning them with their total height.
func renderLayoutParagraphs(ctx context.Context, saved *dml.TxBody, left0, content dml.EMU, breaker *core.TextLayout, fonts *slideRenderFonts, styles *renderTextStyles, chain renderListChain) ([]renderBlock, float64, error) {
	blocks := make([]renderBlock, 0, len(saved.P))
	height := 0.0
	for pi, p := range saved.P {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		if p == nil {
			return nil, 0, fmt.Errorf("%w: nil paragraph", render.ErrInvalid)
		}
		if len(p.Br) > 0 || len(p.Fld) > 0 {
			return nil, 0, fmt.Errorf("%w: hard break or field", render.ErrUnsupported)
		}
		if len(p.R) > fonts.nodes {
			return nil, 0, fmt.Errorf("%w: text runs", render.ErrLimit)
		}
		fonts.nodes -= len(p.R)
		para, layers, err := styles.paragraph(saved, p, chain)
		if err != nil {
			return nil, 0, err
		}
		// Whether PowerPoint adds space before a body's first paragraph
		// depends on spcFirstLastPara semantics this profile does not claim.
		if pi == 0 && para.before != 0 {
			return nil, 0, fmt.Errorf("%w: space before the first paragraph", render.ErrUnsupported)
		}
		if para.marL > content || para.marR > content-para.marL || para.marL+para.marR == content {
			return nil, 0, fmt.Errorf("%w: paragraph margins", render.ErrUnsupported)
		}
		left, ok := style.FromPx(float64(left0+para.marL) / float64(dml.EMUsPerPixel))
		if !ok {
			return nil, 0, render.ErrLimit
		}
		width := renderUnit(content - para.marL - para.marR)
		// Consecutive runs that shape alike form one span; runs differing only
		// in paint are cut apart again by glyph cluster when painted.
		var (
			runs   []renderRunStyle
			ends   []int // byte offset after each run
			spans  []renderShaping
			starts []int // byte offset where each span starts
			texts  []string
			text   strings.Builder
		)
		for _, r := range p.R {
			if r == nil {
				return nil, 0, fmt.Errorf("%w: nil run", render.ErrInvalid)
			}
			rs, err := styles.run(layers, r.RPr)
			if err != nil {
				return nil, 0, err
			}
			if rs.eastAsian && !renderASCII(r.T) {
				return nil, 0, fmt.Errorf("%w: non-ASCII text in an East Asian language", render.ErrUnsupported)
			}
			if len(r.T) > fonts.opts.Limits.MaxRunBytes-text.Len() {
				return nil, 0, fmt.Errorf("%w: paragraph text", render.ErrLimit)
			}
			if n := len(spans); n > 0 && spans[n-1] == rs.renderShaping {
				texts[n-1] += r.T
			} else {
				spans, starts, texts = append(spans, rs.renderShaping), append(starts, text.Len()), append(texts, r.T)
			}
			text.WriteString(r.T)
			runs = append(runs, rs)
			ends = append(ends, text.Len())
		}
		// The end-of-paragraph mark sizes an empty paragraph; a paragraph with
		// runs takes its line boxes from them, as LibreOffice's import does.
		if len(runs) == 0 {
			end, err := styles.run(layers, p.EndParaRPr)
			if err != nil {
				return nil, 0, err
			}
			spans, starts, texts = []renderShaping{end.renderShaping}, []int{0}, []string{""}
		}
		lines, err := renderParagraphLines(ctx, breaker, fonts, spans, texts, width, para.lineSpacing)
		if err != nil {
			return nil, 0, err
		}
		blocks = append(blocks, renderBlock{para: para, runs: runs, ends: ends, starts: starts, text: text.String(), lines: lines, left: left, width: width})
		height += float64(para.before+para.after) / float64(dml.EMUsPerPixel)
		for _, line := range lines {
			height += line.height
		}
	}
	return blocks, height, nil
}

// renderPlaceParagraphs anchors laid-out paragraphs between contentTop and
// bottom and paints them. A frame that grows may hold text past its bottom.
func renderPlaceParagraphs(blocks []renderBlock, height, contentTop, bottom float64, anchor enum.TextAnchor, grows bool, fonts *slideRenderFonts) ([]layout.Op, error) {
	// The text block spans its paragraphs' spacing and full line heights.
	top := contentTop
	switch anchor {
	case enum.TextAnchorMiddle:
		top += (bottom - contentTop - height) / 2
	case enum.TextAnchorBottom:
		top = bottom - height
	}
	if top < contentTop && !grows {
		return nil, fmt.Errorf("%w: text exceeds frame", render.ErrUnsupported)
	}
	var ops []layout.Op
	for _, b := range blocks {
		para, left, width := b.para, b.left, b.width
		top += float64(para.before) / float64(dml.EMUsPerPixel)
		covered := 0
		for _, line := range b.lines {
			// A line that draws nothing may hang below the frame unseen.
			if top+line.ascent+line.descent > bottom && !grows && renderLineDraws(line) {
				return nil, fmt.Errorf("%w: text exceeds frame", render.ErrUnsupported)
			}
			xp := left.Px()
			if para.align == enum.TextAlignCenter {
				xp += (width.Px() - line.Width.Px()) / 2
			}
			if para.align == enum.TextAlignRight {
				xp += width.Px() - line.Width.Px()
			}
			for _, sg := range line.Segments {
				start := b.starts[sg.Span] + sg.Offset
				if start != covered || !strings.HasPrefix(b.text[start:], sg.Text) {
					return nil, fmt.Errorf("%w: paragraph line text", render.ErrUnsupported)
				}
				covered += len(sg.Text)
				if len(sg.Glyphs) == 0 {
					continue
				}
				drawn, err := renderSegmentRuns(sg, start, b.ends, b.runs, xp+sg.X.Px(), top+line.ascent)
				if err != nil {
					return nil, err
				}
				if len(drawn) > fonts.opts.Limits.MaxOperations-fonts.ops {
					return nil, fmt.Errorf("%w: text drawing operations", render.ErrLimit)
				}
				fonts.ops += len(drawn)
				ops = append(ops, drawn...)
			}
			top += line.height
		}
		if covered != len(b.text) {
			return nil, fmt.Errorf("%w: paragraph line text", render.ErrUnsupported)
		}
		top += float64(para.after) / float64(dml.EMUsPerPixel)
	}
	return ops, nil
}

// renderBlock is one laid-out paragraph awaiting vertical placement.
type renderBlock struct {
	para   renderParaStyle
	runs   []renderRunStyle
	ends   []int
	starts []int
	text   string
	lines  []renderLine
	left   style.Unit
	width  style.Unit
}

// renderLine is a wrapped line with its box: the largest ascent, descent and
// line gap of its segments, the gaps scaled by percentage line spacing.
type renderLine struct {
	core.RichLine
	ascent, descent, height float64
}

// renderParagraphLines wraps a paragraph's spans. An empty paragraph has one
// empty span, whose face sizes its line.
func renderParagraphLines(ctx context.Context, breaker *core.TextLayout, fonts *slideRenderFonts, shapings []renderShaping, texts []string, width style.Unit, lineSpacing int32) ([]renderLine, error) {
	spans := make([]core.Span, len(shapings))
	for i, run := range shapings {
		// ST_TextFontSize is 1 to 4000 points.
		if run.size < 100 || run.size > 400000 {
			return nil, fmt.Errorf("%w: font size", render.ErrUnsupported)
		}
		face, err := fonts.resolve(ctx, run.font, run.bold, run.italic)
		if err != nil {
			return nil, err
		}
		size, ok := style.FromPx(float64(run.size) / 100 * 4 / 3)
		if !ok {
			return nil, render.ErrLimit
		}
		// kern is the smallest size PowerPoint kerns; absent or zero is off.
		spans[i] = core.Span{Face: face, Size: size, Text: texts[i], Features: shape.Features{NoKerning: run.kern == 0 || run.size < run.kern}}
	}
	// DrawingML's Latin font serves Latin, Greek and Cyrillic text alike.
	wrapped, err := breaker.RichLines(ctx, spans, width, core.RepertoireEuropean)
	if err != nil {
		return nil, err
	}
	lines := make([]renderLine, len(wrapped))
	for i, w := range wrapped {
		line := renderLine{RichLine: w}
		gap := 0.0
		for _, sg := range w.Segments {
			a, d, g := renderFaceMetrics(sg.Face, sg.Size)
			line.ascent, line.descent, gap = max(line.ascent, a), max(line.descent, d), max(gap, g)
		}
		line.height = (line.ascent + line.descent + gap) * float64(lineSpacing) / 100000
		if line.ascent <= 0 || line.descent < 0 || line.height <= 0 {
			return nil, fmt.Errorf("%w: font line metrics", render.ErrUnsupported)
		}
		lines[i] = line
	}
	return lines, nil
}

// renderFaceMetrics returns a face's hhea ascent, descent and line gap at a
// size, in CSS pixels.
func renderFaceMetrics(face *shape.Face, size style.Unit) (ascent, descent, gap float64) {
	metrics := face.Descriptor()
	em := float64(face.UnitsPerEm())
	return float64(metrics.Ascent) * size.Px() / em, -float64(metrics.Descent) * size.Px() / em, float64(metrics.LineGap) * size.Px() / em
}

// renderSegmentRuns paints one line segment run by run: highlights first,
// merging touching spans of one color so they show no seam, then each run's
// glyphs in its color. start is the segment's byte offset in the paragraph
// text and ends the offset after each run. A highlight spans the segment
// face's ascent to descent about the shared baseline. PowerPoint shapes runs
// apart, so a glyph standing for characters of two runs, such as a ligature,
// fails.
func renderSegmentRuns(sg core.RichSegment, start int, ends []int, runs []renderRunStyle, x, baseline float64) ([]layout.Op, error) {
	runAt := func(offset int) int {
		i := 0
		for i < len(ends)-1 && offset >= ends[i] {
			i++
		}
		return i
	}
	ascent, descent, _ := renderFaceMetrics(sg.Face, sg.Size)
	yu, yok := style.FromPx(baseline)
	ty, tok := style.FromPx(baseline - ascent)
	hu, hok := style.FromPx(ascent + descent)
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
			xu, xok := style.FromPx(x + spanFrom*sg.Size.Px()/1000)
			wu, wok := style.FromPx((spanTo - spanFrom) * sg.Size.Px() / 1000)
			if !xok || !wok {
				return render.ErrLimit
			}
			highlights = append(highlights, layout.FillRect{Rect: layout.Rect{X: xu, Y: ty, W: wu, H: hu}, Color: span.color})
		}
		span = renderHighlight{}
		return nil
	}
	pen := 0.0 // in 1000 units per em
	for i := 0; i < len(sg.Glyphs); {
		first := sg.Glyphs[i].Cluster
		if first < 0 || first >= len(sg.Text) {
			return nil, fmt.Errorf("%w: glyph cluster", render.ErrInvalid)
		}
		r := runAt(start + first)
		j := i
		advance := 0.0
		for j < len(sg.Glyphs) && runAt(start+sg.Glyphs[j].Cluster) == r {
			advance += sg.Glyphs[j].XAdvance
			j++
		}
		// The piece's characters run to the next piece's first cluster.
		pieceEnd := len(sg.Text)
		if j < len(sg.Glyphs) {
			pieceEnd = sg.Glyphs[j].Cluster
		}
		if pieceEnd <= first || runAt(start+pieceEnd-1) != r {
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
		copy(glyphs, sg.Glyphs[i:j])
		for k := range glyphs {
			glyphs[k].Cluster -= first
		}
		xu, xok := style.FromPx(x + pen*sg.Size.Px()/1000)
		if !xok {
			return nil, render.ErrLimit
		}
		glyphOps = append(glyphOps, layout.DrawGlyphs{At: layout.Point{X: xu, Y: yu}, Text: sg.Text[first:pieceEnd], Glyphs: glyphs, Face: sg.Face, Size: sg.Size, Color: runs[r].color})
		pen += advance
		i = j
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return append(highlights, glyphOps...), nil
}

func renderASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func renderHasText(body *dml.TxBody) bool {
	for _, p := range body.P {
		if p == nil {
			continue
		}
		if len(p.Br) > 0 || len(p.Fld) > 0 {
			return true
		}
		for _, r := range p.R {
			if r != nil && r.T != "" {
				return true
			}
		}
	}
	return false
}

func renderLineDraws(line renderLine) bool {
	for _, sg := range line.Segments {
		if len(sg.Glyphs) > 0 {
			return true
		}
	}
	return false
}
