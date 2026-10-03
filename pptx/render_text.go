package pptx

import (
	"math"
	"context"
	"fmt"
	"strconv"
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
	// noWrap lays each line out at its natural width.
	noWrap bool
	// fontScale and lnSpcReduction are normal autofit's scaling.
	fontScale, lnSpcReduction int32
}

// renderBodyFrame applies DrawingML body defaults. A non-placeholder body
// inherits nothing: absent attributes take their schema defaults (top anchor,
// square wrap, 0.1"/0.05" insets, no autofit). Properties this profile does
// not draw are approximated through colors in best-effort mode.
func renderBodyFrame(bp *dml.BodyPr, colors *renderColors) (renderFrame, error) {
	f := renderFrame{margins: TextMargins{Left: 91440, Top: 45720, Right: 91440, Bottom: 45720}, anchor: enum.TextAnchorTop}
	if bp == nil {
		return f, nil
	}
	switch enum.TextAnchor(bp.Anchor) {
	case "", enum.TextAnchorTop:
	case enum.TextAnchorMiddle, enum.TextAnchorBottom:
		f.anchor = enum.TextAnchor(bp.Anchor)
	default:
		if err := colors.approximate(fmt.Errorf("%w: justified or distributed text anchoring drawn top", render.ErrUnsupported)); err != nil {
			return f, err
		}
	}
	switch bp.Wrap {
	case "", "square":
	case "none":
		f.noWrap = true
	default:
		return f, fmt.Errorf("%w: text wrapping", render.ErrInvalid)
	}
	if bp.ExtLst != nil {
		return f, fmt.Errorf("%w: text body extension", render.ErrUnsupported)
	}
	if (bp.Rot != nil && *bp.Rot != 0) || (bp.Vert != "" && bp.Vert != "horz") || renderTrue(bp.UpRight) {
		if err := colors.approximate(fmt.Errorf("%w: rotated or vertical text drawn horizontally", render.ErrUnsupported)); err != nil {
			return f, err
		}
	}
	if bp.NumCol > 1 {
		if err := colors.approximate(fmt.Errorf("%w: text columns drawn as one", render.ErrUnsupported)); err != nil {
			return f, err
		}
	}
	if (bp.VertOverflow != "" && bp.VertOverflow != "overflow") || (bp.HorzOverflow != "" && bp.HorzOverflow != "overflow") {
		if err := colors.approximate(fmt.Errorf("%w: clipped text drawn whole", render.ErrUnsupported)); err != nil {
			return f, err
		}
	}
	if renderTrue(bp.AnchorCtr) {
		if err := colors.approximate(fmt.Errorf("%w: horizontally centered text block drawn in place", render.ErrUnsupported)); err != nil {
			return f, err
		}
	}
	if renderTrue(bp.FromWordArt) || renderTrue(bp.CompatLnSpc) || bp.PrstTxWarp != nil || bp.Scene3d != nil || bp.Sp3d != nil || bp.FlatTx != nil {
		if err := colors.approximate(fmt.Errorf("%w: text warp, 3-D or compatible line spacing left out", render.ErrUnsupported)); err != nil {
			return f, err
		}
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
	if n := bp.NormAutofit; n != nil {
		// PowerPoint stores the scale it fitted with; text is laid out at it.
		f.fontScale, f.lnSpcReduction = n.FontScale.Int32(), n.LnSpcReduction.Int32()
		if f.fontScale < 0 || f.fontScale > 100000 || f.lnSpcReduction < 0 || f.lnSpcReduction > 100000 {
			return f, fmt.Errorf("%w: autofit scale", render.ErrInvalid)
		}
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
	frame, err := renderBodyFrame(bodyPr, styles.colors)
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
	var st *dml.Style
	if ph != nil {
		st = ph.styleRef()
	} else if source != nil {
		st = source.Style
	}
	if st != nil {
		if ref, err := renderFontRef(st.FontRef); err != nil {
			return nil, err
		} else if ref != nil {
			chain.inherited = append([]*dml.LstStyle{ref}, chain.inherited...)
		}
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
		// lay text out in such a box, and best effort drops the insets on the
		// axis they overfill.
		if err = styles.colors.approximate(fmt.Errorf("%w: text insets larger than the shape left out", render.ErrUnsupported)); err != nil {
			return nil, err
		}
		if m.Left > w || m.Right > w-m.Left {
			m.Left, m.Right = 0, 0
		}
		if m.Top > h || m.Bottom > h-m.Top {
			m.Top, m.Bottom = 0, 0
		}
	}
	content := w - m.Left - m.Right
	contentTop := float64(y)/float64(dml.EMUsPerPixel) + float64(m.Top)/float64(dml.EMUsPerPixel)
	bottom := float64(y)/float64(dml.EMUsPerPixel) + float64(h-m.Bottom)/float64(dml.EMUsPerPixel)
	// Lay every paragraph out first: anchoring needs the text height.
	styles.fontScale, styles.lnSpcReduction = frame.fontScale, frame.lnSpcReduction
	defer func() { styles.fontScale, styles.lnSpcReduction = 0, 0 }()
	blocks, height, err := renderLayoutParagraphs(ctx, saved, x+m.Left, content, frame.noWrap, breaker, fonts, styles, chain)
	if err != nil {
		return nil, err
	}
	return renderPlaceParagraphs(blocks, height, contentTop, bottom, frame.anchor, frame.grows, fonts, styles.colors)
}

// renderLayoutParagraphs wraps a text body's paragraphs in a content box of
// the given left edge and width, returning them with their total height.
// Without wrapping, each piece of a paragraph is one line however wide.
func renderLayoutParagraphs(ctx context.Context, saved *dml.TxBody, left0, content dml.EMU, noWrap bool, breaker *core.TextLayout, fonts *slideRenderFonts, styles *renderTextStyles, chain renderListChain) ([]renderBlock, float64, error) {
	blocks := make([]renderBlock, 0, len(saved.P))
	height := 0.0
	// numbers counts numbered paragraphs per level: a paragraph resets the
	// counts of deeper levels, and one without numbering its own level's.
	var numbers [9]struct {
		scheme string
		next   int32
	}
	for pi, p := range saved.P {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		if p == nil {
			return nil, 0, fmt.Errorf("%w: nil paragraph", render.ErrInvalid)
		}
		children := p.Children()
		if len(children) > fonts.nodes {
			return nil, 0, fmt.Errorf("%w: text runs", render.ErrLimit)
		}
		fonts.nodes -= len(children)
		para, layers, err := styles.paragraph(saved, p, chain)
		if err != nil {
			return nil, 0, err
		}
		// Whether PowerPoint adds space before a body's first paragraph
		// depends on spcFirstLastPara semantics this profile does not claim;
		// best effort adds it.
		if pi == 0 && (para.before != 0 || para.beforePct != 0) {
			if err := styles.colors.approximate(fmt.Errorf("%w: space before the first paragraph", render.ErrUnsupported)); err != nil {
				return nil, 0, err
			}
		}
		level := 0
		if p.PPr != nil && p.PPr.Lvl != nil {
			level = int(min(max(*p.PPr.Lvl, 0), 8))
		}
		for l := level + 1; l < len(numbers); l++ {
			numbers[l].scheme = ""
		}
		if b := &para.bullet; b.autoNum != "" && renderHasParagraphText(p) {
			n := &numbers[level]
			if n.scheme != b.autoNum {
				n.scheme, n.next = b.autoNum, b.startAt
			}
			var ok bool
			if b.char, ok = renderAutoNumber(b.autoNum, n.next); !ok {
				if err := styles.colors.approximate(fmt.Errorf("%w: numbering %s drawn as arabic numerals", render.ErrUnsupported, b.autoNum)); err != nil {
					return nil, 0, err
				}
				b.char, _ = renderAutoNumber("arabicPeriod", n.next)
			}
			n.next++
		} else if renderHasParagraphText(p) {
			numbers[level].scheme = ""
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
		// in paint are cut apart again by glyph cluster when painted. A field
		// is a run showing its saved text. A line break ends a piece of the
		// paragraph, which wraps on its own; spans never cross pieces.
		var (
			runs   []renderRunStyle
			ends   []int // byte offset after each run
			spans  []renderShaping
			starts []int // byte offset where each span starts
			texts  []string
			pieces []int // span index where each piece after the first starts
			text   strings.Builder
		)
		// mark sizes an empty piece by the properties of the break or
		// paragraph end that closes it.
		mark := func(rPr *dml.RPr) error {
			first := 0
			if n := len(pieces); n > 0 {
				first = pieces[n-1]
			}
			if len(spans) > first {
				return nil
			}
			end, err := styles.run(layers, rPr)
			if err != nil {
				return err
			}
			spans, starts, texts = append(spans, end.renderShaping), append(starts, text.Len()), append(texts, "")
			return nil
		}
		for _, c := range children {
			var (
				rPr *dml.RPr
				t   string
			)
			switch {
			case c.R != nil:
				rPr, t = c.R.RPr, c.R.T
			case c.Fld != nil:
				rPr, t = c.Fld.RPr, c.Fld.T
			case c.Br != nil:
				if err := mark(c.Br.RPr); err != nil {
					return nil, 0, err
				}
				pieces = append(pieces, len(spans))
				continue
			default:
				return nil, 0, fmt.Errorf("%w: nil run", render.ErrInvalid)
			}
			rs, err := styles.run(layers, rPr)
			if err != nil {
				return nil, 0, err
			}
			if rs.caps {
				t = strings.ToUpper(t)
			}
			// Best effort leaves out characters this profile or the run's font
			// cannot draw.
			if styles.colors.approx != nil && t != "" {
				if face, err := fonts.resolve(ctx, rs.font, rs.bold, rs.italic); err == nil {
					kept := strings.Map(func(c rune) rune {
						if _, ok := face.GlyphID(c); (ok && core.RepertoireEuropean.Allows(c)) || c == '\t' {
							return c
						}
						return -1
					}, t)
					if kept != t {
						styles.colors.approx(fmt.Errorf("%w: characters the font or profile lacks left out", render.ErrUnsupported))
						t = kept
					}
				}
			}
			if rs.eastAsian && !renderASCII(t) {
				return nil, 0, fmt.Errorf("%w: non-ASCII text in an East Asian language", render.ErrUnsupported)
			}
			if len(t) > fonts.opts.Limits.MaxRunBytes-text.Len() {
				return nil, 0, fmt.Errorf("%w: paragraph text", render.ErrLimit)
			}
			if n := len(spans); n > 0 && spans[n-1] == rs.renderShaping && (len(pieces) == 0 || pieces[len(pieces)-1] < n) {
				texts[n-1] += t
			} else {
				spans, starts, texts = append(spans, rs.renderShaping), append(starts, text.Len()), append(texts, t)
			}
			text.WriteString(t)
			runs = append(runs, rs)
			ends = append(ends, text.Len())
		}
		// The end-of-paragraph mark sizes an empty paragraph or last piece; a
		// piece with runs takes its line boxes from them, as LibreOffice's
		// import does.
		if err := mark(p.EndParaRPr); err != nil {
			return nil, 0, err
		}
		if para.customTabs && strings.Contains(text.String(), "\t") {
			if err := styles.colors.approximate(fmt.Errorf("%w: explicit tab stops placed at the default spacing", render.ErrUnsupported)); err != nil {
				return nil, 0, err
			}
		}
		wrap := width
		if noWrap {
			wrap = renderUnit(dml.EMU(1) << 30)
		}
		var lines []renderLine
		bounds := append(append([]int{0}, pieces...), len(spans))
		for k := 0; k+1 < len(bounds); k++ {
			a, b := bounds[k], bounds[k+1]
			piece, err := renderParagraphLines(ctx, breaker, fonts, spans[a:b], texts[a:b], wrap, para)
			if err != nil {
				return nil, 0, err
			}
			for _, l := range piece {
				for i := range l.Segments {
					l.Segments[i].Span += a
				}
			}
			lines = append(lines, piece...)
		}
		for _, l := range lines {
			if l.Overflow {
				if err = styles.colors.approximate(fmt.Errorf("%w: text wider than its box", render.ErrUnsupported)); err != nil {
					return nil, 0, err
				}
				break
			}
		}
		// Percentage spacing is of the first and last line heights.
		px := float64(dml.EMUsPerPixel)
		para.before += dml.EMU(math.Round(float64(para.beforePct) / 100000 * lines[0].height * px))
		para.after += dml.EMU(math.Round(float64(para.afterPct) / 100000 * lines[len(lines)-1].height * px))
		block := renderBlock{para: para, runs: runs, ends: ends, starts: starts, text: text.String(), lines: lines, left: left, width: width}
		if text.Len() > 0 && para.bullet.char != "" {
			if block.bullet, err = renderLayoutBullet(ctx, breaker, fonts, para, runs[0], lines[0], left0, width, styles.colors.approximate); err != nil {
				return nil, 0, err
			}
		} else if text.Len() > 0 && para.indent != 0 {
			// Without a bullet the first line would start apart from the rest;
			// best effort starts it with the rest.
			if err := styles.colors.approximate(fmt.Errorf("%w: first-line indent left out", render.ErrUnsupported)); err != nil {
				return nil, 0, err
			}
		}
		blocks = append(blocks, block)
		height += float64(para.before+para.after) / float64(dml.EMUsPerPixel)
		for _, line := range lines {
			height += line.height
		}
	}
	return blocks, height, nil
}

// renderPlaceParagraphs anchors laid-out paragraphs between contentTop and
// bottom and paints them. A frame that grows may hold text past its bottom.
// Text past a fixed frame fails, and best effort draws it, as PowerPoint
// shows it.
func renderPlaceParagraphs(blocks []renderBlock, height, contentTop, bottom float64, anchor enum.TextAnchor, grows bool, fonts *slideRenderFonts, colors *renderColors) ([]layout.Op, error) {
	overflow := func() error {
		if err := colors.approximate(fmt.Errorf("%w: text exceeds frame", render.ErrUnsupported)); err != nil {
			return err
		}
		grows = true
		return nil
	}
	// The text block spans its paragraphs' spacing and full line heights.
	top := contentTop
	switch anchor {
	case enum.TextAnchorMiddle:
		top += (bottom - contentTop - height) / 2
	case enum.TextAnchorBottom:
		top = bottom - height
	}
	if top < contentTop && !grows {
		if err := overflow(); err != nil {
			return nil, err
		}
	}
	var ops []layout.Op
	for _, b := range blocks {
		para, left, width := b.para, b.left, b.width
		top += float64(para.before) / float64(dml.EMUsPerPixel)
		covered := 0
		for _, line := range b.lines {
			// A line that draws nothing may hang below the frame unseen.
			if top+line.ascent+line.descent > bottom && !grows && renderLineDraws(line) {
				if err := overflow(); err != nil {
					return nil, err
				}
			}
			xp := left.Px()
			if para.align == enum.TextAlignCenter {
				xp += (width.Px() - line.Width.Px()) / 2
			}
			if para.align == enum.TextAlignRight {
				xp += width.Px() - line.Width.Px()
			}
			if b.bullet != nil && covered == 0 {
				bx, bxok := style.FromPx(b.bullet.x)
				by, byok := style.FromPx(top + line.ascent)
				if !bxok || !byok {
					return nil, render.ErrLimit
				}
				if fonts.opts.Limits.MaxOperations-fonts.ops < 1 {
					return nil, fmt.Errorf("%w: text drawing operations", render.ErrLimit)
				}
				fonts.ops++
				sg := b.bullet.seg
				ops = append(ops, layout.DrawGlyphs{At: layout.Point{X: bx, Y: by}, Text: sg.Text, Glyphs: sg.Glyphs, Face: sg.Face, Size: sg.Size, Color: b.bullet.color})
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
	bullet *renderBulletGlyph
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
func renderParagraphLines(ctx context.Context, breaker *core.TextLayout, fonts *slideRenderFonts, shapings []renderShaping, texts []string, width style.Unit, para renderParaStyle) ([]renderLine, error) {
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
		pts := float64(run.size) / 100
		if run.baseline != 0 {
			pts *= renderEscapedSize
		}
		size, ok := style.FromPx(pts * 4 / 3)
		letter, okL := style.FromPx(float64(run.spacing) / 100 * 4 / 3)
		if !ok || !okL {
			return nil, render.ErrLimit
		}
		// kern is the smallest size PowerPoint kerns; absent or zero is off.
		spans[i] = core.Span{Face: face, Size: size, Text: texts[i], Features: shape.Features{NoKerning: run.kern == 0 || run.size < run.kern}, TabStop: renderUnit(para.tabSize), Letter: letter}
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
		line.height = (line.ascent + line.descent + gap) * float64(para.lineSpacing) / 100000
		if para.lineFixed > 0 {
			// An exact height keeps the glyphs' ascent-to-descent proportion,
			// approximately as PowerPoint places them.
			fixed := float64(para.lineFixed) / float64(dml.EMUsPerPixel)
			share := line.ascent / (line.ascent + line.descent)
			line.height, line.ascent, line.descent = fixed, fixed*share, fixed*(1-share)
		}
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
		at := yu
		if shift := runs[r].baseline; shift != 0 {
			// The shift is a share of the run's own, unreduced size.
			y, ok := style.FromPx(baseline - float64(shift)/100000*float64(runs[r].size)/100*4/3)
			if !ok {
				return nil, render.ErrLimit
			}
			at = y
		}
		glyphOps = append(glyphOps, layout.DrawGlyphs{At: layout.Point{X: xu, Y: at}, Text: sg.Text[first:pieceEnd], Glyphs: glyphs, Face: sg.Face, Size: sg.Size, Color: runs[r].color})
		// Underlines sit a tenth of an em below the baseline and strikes
		// three tenths above it, a twentieth of an em thick.
		em := sg.Size.Px()
		thick := math.Max(em/20, 1)
		for _, line := range []struct {
			n int
			y float64
		}{{runs[r].underline, baseline + em/10}, {runs[r].strike, baseline - em*3/10}} {
			for k := 0; k < line.n; k++ {
				lx, okX := style.FromPx(x + pen*em/1000)
				ly, okY := style.FromPx(line.y + float64(k)*thick*2 - thick/2)
				lw, okW := style.FromPx(advance * em / 1000)
				lh, okH := style.FromPx(thick)
				if !okX || !okY || !okW || !okH {
					return nil, render.ErrLimit
				}
				glyphOps = append(glyphOps, layout.FillRect{Rect: layout.Rect{X: lx, Y: ly, W: lw, H: lh}, Color: runs[r].color})
			}
		}
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
		for _, c := range p.Children() {
			if c.Br != nil || c.Fld != nil || (c.R != nil && c.R.T != "") {
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

// renderBulletGlyph is a laid-out bullet, drawn on its paragraph's first
// baseline at x pixels.
type renderBulletGlyph struct {
	seg   core.RichSegment
	x     float64
	color style.RGBA
}

// renderLayoutBullet shapes a paragraph's bullet. It hangs in the first line's
// negative indent, which must hold it, so the text of every line starts at
// the left margin; a bullet past it would push the first line to a tab stop,
// whose placement this profile does not claim. The bullet may not raise its
// line. Best effort, through approx, draws such bullets where they would
// hang, over the text or past the line.
func renderLayoutBullet(ctx context.Context, breaker *core.TextLayout, fonts *slideRenderFonts, para renderParaStyle, first renderRunStyle, line renderLine, left0 dml.EMU, width style.Unit, approx func(error) error) (*renderBulletGlyph, error) {
	b := para.bullet
	shaping := renderShaping{font: first.font, bold: first.bold, italic: first.italic}
	if b.font != "" {
		shaping.font, shaping.bold, shaping.italic = b.font, false, false
	}
	if b.points > 0 {
		shaping.size = b.points
	} else {
		shaping.size = int32(int64(first.size) * int64(b.size) / 100000)
	}
	lines, err := renderParagraphLines(ctx, breaker, fonts, []renderShaping{shaping}, []string{b.char}, width, para)
	if err != nil {
		return nil, err
	}
	if len(lines) != 1 || len(lines[0].Segments) != 1 {
		return nil, fmt.Errorf("%w: bullet layout", render.ErrUnsupported)
	}
	bullet := lines[0]
	if bullet.Width.Px() > -float64(para.indent)/float64(dml.EMUsPerPixel) {
		if err := approx(fmt.Errorf("%w: bullet wider than its hanging indent", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	if bullet.ascent > line.ascent || bullet.descent > line.descent {
		if err := approx(fmt.Errorf("%w: bullet taller than its line", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	color := first.color
	if b.ownColor {
		color = b.color
	}
	return &renderBulletGlyph{seg: bullet.Segments[0], x: float64(left0+para.marL+para.indent) / float64(dml.EMUsPerPixel), color: color}, nil
}

// renderFontRef turns a style's font reference into a list style below the
// shape's own: every level takes the theme font and the reference's color.
func renderFontRef(r *dml.FontRef) (*dml.LstStyle, error) {
	if r == nil {
		return nil, nil
	}
	var def dml.RPr
	switch r.Idx {
	case "major":
		def.Latin = &dml.TextFont{Typeface: "+mj-lt"}
	case "minor":
		def.Latin = &dml.TextFont{Typeface: "+mn-lt"}
	case "none":
	default:
		return nil, fmt.Errorf("%w: font reference %q", render.ErrInvalid, r.Idx)
	}
	if r.ScrgbClr != nil || r.SrgbClr != nil || r.HslClr != nil || r.SysClr != nil || r.SchemeClr != nil || r.PrstClr != nil {
		def.SolidFill = &dml.SolidFill{ScRgbClr: r.ScrgbClr, SrgbClr: r.SrgbClr, HslClr: r.HslClr, SysClr: r.SysClr, SchemeClr: r.SchemeClr, PrstClr: r.PrstClr}
	}
	if def.Latin == nil && def.SolidFill == nil {
		return nil, nil
	}
	level := &dml.PPr{DefRPr: &def}
	return &dml.LstStyle{DefPPr: level, Lvl1pPr: level, Lvl2pPr: level, Lvl3pPr: level, Lvl4pPr: level, Lvl5pPr: level, Lvl6pPr: level, Lvl7pPr: level, Lvl8pPr: level, Lvl9pPr: level}, nil
}

// renderEscapedSize scales raised and lowered text, as LibreOffice's import
// does.
const renderEscapedSize = 0.58

// renderHasParagraphText reports whether a paragraph shows any characters.
func renderHasParagraphText(p *dml.P) bool {
	return renderHasText(&dml.TxBody{P: []*dml.P{p}})
}

// renderAutoNumber formats a paragraph number in an ST_TextAutonumberScheme
// of Latin letters, Roman or Arabic numerals.
func renderAutoNumber(scheme string, n int32) (string, bool) {
	if n < 1 {
		return "", false
	}
	var digits, rest string
	switch {
	case strings.HasPrefix(scheme, "arabic"):
		digits, rest = strconv.Itoa(int(n)), strings.TrimPrefix(scheme, "arabic")
	case strings.HasPrefix(scheme, "alphaLc"), strings.HasPrefix(scheme, "alphaUc"):
		// a … z, aa … zz, aaa …: the letter repeats.
		letter := string(rune('a' + (n-1)%26))
		digits = strings.Repeat(letter, int((n-1)/26+1))
		if strings.HasPrefix(scheme, "alphaUc") {
			digits = strings.ToUpper(digits)
		}
		rest = scheme[len("alphaLc"):]
	case strings.HasPrefix(scheme, "romanLc"), strings.HasPrefix(scheme, "romanUc"):
		if n > 3999 {
			return "", false
		}
		digits = renderRoman(int(n))
		if strings.HasPrefix(scheme, "romanLc") {
			digits = strings.ToLower(digits)
		}
		rest = scheme[len("romanLc"):]
	default:
		return "", false
	}
	switch rest {
	case "Period":
		return digits + ".", true
	case "ParenR":
		return digits + ")", true
	case "ParenBoth":
		return "(" + digits + ")", true
	case "Plain":
		return digits, true
	}
	return "", false
}

func renderRoman(n int) string {
	var b strings.Builder
	for _, d := range []struct {
		v int
		s string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}} {
		for n >= d.v {
			b.WriteString(d.s)
			n -= d.v
		}
	}
	return b.String()
}
