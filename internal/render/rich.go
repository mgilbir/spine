package render

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/mgilbir/forme/paragraph"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
)

// Span is one uniformly styled run of a rich paragraph.
type Span struct {
	Face     *shape.Face
	Size     style.Unit
	Text     string
	Features shape.Features
	// Letter is added after every character, and may be negative.
	Letter style.Unit
	// TabStop, when positive, is the distance between tab stops, measured
	// from the start of the line: a tab advances to the next. Without it a
	// tab is unsupported.
	TabStop style.Unit
	// BreakWord lets a word too wide for a line break between characters,
	// as a last resort, rather than overflow.
	BreakWord bool
	// Tabs are explicit stops, from the start of the line and ascending,
	// which a tab in this span takes before the evenly spaced ones past
	// them. Lines break with every tab measured to the evenly spaced stops.
	Tabs []TabStop
}

// TabStop is an explicit tab stop.
type TabStop struct {
	At    style.Unit
	Align TabAlign
}

// TabAlign is how the text after a tab, up to the next tab or the line's
// end, sits at its stop.
type TabAlign int

const (
	// TabLeft starts the text at the stop.
	TabLeft TabAlign = iota
	// TabRight ends the text at the stop.
	TabRight
	// TabCenter centres the text on the stop.
	TabCenter
	// TabDecimal puts the text's first full stop at the stop, or ends the
	// text there when it has none.
	TabDecimal
)

// RichSegment is the part of one span on one line, shaped on its own.
type RichSegment struct {
	// Span indexes the paragraph's spans; Offset is where Text starts in the
	// span's text.
	Span   int
	Offset int
	Text   string
	Glyphs []shape.Glyph
	// Face is a private font clone; the caller must keep it immutable.
	Face *shape.Face
	Size style.Unit
	// X is the segment's pen offset from the start of the line.
	X, Width style.Unit
	// Level is the segment's embedding level: odd levels run right to left.
	// Glyphs of such a segment are in the order they are drawn, left to right,
	// so their clusters fall as they go; Text is in logical order. Segments of
	// a line are in the order they are drawn. Level is zero in a paragraph
	// that runs left to right throughout.
	Level int
}

// RTL reports whether the segment's text runs right to left.
func (s RichSegment) RTL() bool { return s.Level&1 == 1 }

// RichLine is one wrapped line of a rich paragraph.
type RichLine struct {
	Segments []RichSegment
	// Width is the line's measure, less Hang.
	Width style.Unit
	// Hang is the width of an East Asian stop or comma that ends the line
	// past its measure, which Width leaves out; see RichOptions.
	Hang style.Unit
	// Overflow marks a line wider than the wrapping width, which only a
	// layout allowing overflow returns.
	Overflow bool
	// TabsMoved marks a line where an explicit stop moved a tab from where
	// line breaking measured it.
	TabsMoved bool
	// TabsApprox marks a right-to-left line with a tab placed by a rule not
	// confirmed against PowerPoint: one at a centred or decimal stop, or past
	// the explicit stops. Left and right stops are exact.
	TabsApprox bool
}

// RichLines wraps a horizontal left-to-right paragraph whose spans may differ
// in face, size and features. Break opportunities come from the whole
// paragraph text; shaping context stays within a span, as formats that shape
// runs apart require, so no glyph or contextual form crosses a span boundary.
// An empty paragraph yields one line holding an empty segment of its first
// span, which carries that span's face. Budgets are shared with Lines; the
// remaining shaping work is divided evenly between the paragraph's faces.
func (t *TextLayout) RichLines(ctx context.Context, spans []Span, width style.Unit, repertoire Repertoire) ([]RichLine, error) {
	return t.RichLinesWith(ctx, spans, width, repertoire, RichOptions{})
}

// RichOptions are a rich paragraph's choices beyond its spans.
type RichOptions struct {
	// HangPunct lets an East Asian stop or comma, such as the ideographic
	// full stop, that would not otherwise fit at the end of a line hang past
	// its width, as PowerPoint's hanging punctuation does. It does not apply
	// to a paragraph that runs right to left or holds right-to-left text.
	HangPunct bool
	// RTL sets the paragraph's base direction right to left. Its lines start
	// at the right, and text of either direction is ordered by the Unicode
	// bidirectional algorithm: breaks are found in logical order, and each
	// line's segments are cut by embedding level and drawn in visual order.
	// Joining forms of Arabic follow the neighbours within a span. Tabs of a
	// right-to-left paragraph are measured from the line's start, its right
	// end, and a stop aligns the text after it physically: a left stop puts
	// the text's left edge at the stop and a right stop its right edge. Tabs
	// are unsupported in a left-to-right paragraph with right-to-left text.
	RTL bool
}

// RichLinesWith is RichLines with options.
func (t *TextLayout) RichLinesWith(ctx context.Context, spans []Span, width style.Unit, repertoire Repertoire, opts RichOptions) ([]RichLine, error) {
	if t == nil || ctx == nil || width <= 0 || len(spans) == 0 {
		return nil, fmt.Errorf("%w: paragraph input", ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l := t.page.limits
	total := 0
	for _, s := range spans {
		if s.Size <= 0 || !utf8.ValidString(s.Text) {
			return nil, fmt.Errorf("%w: paragraph input", ErrInvalid)
		}
		for i, stop := range s.Tabs {
			if stop.Align < TabLeft || stop.Align > TabDecimal || (i > 0 && stop.At <= s.Tabs[i-1].At) {
				return nil, fmt.Errorf("%w: tab stops", ErrInvalid)
			}
		}
		if len(s.Text) > l.MaxRunBytes-total {
			return nil, fmt.Errorf("%w: paragraph text", ErrLimit)
		}
		total += len(s.Text)
		for _, c := range s.Text {
			if !repertoire.allows(c) && (c != '\t' || s.TabStop <= 0) {
				return nil, fmt.Errorf("%w: plain paragraph character", ErrUnsupported)
			}
		}
	}
	if total > l.MaxTextBytes-t.budget.textBytes {
		return nil, fmt.Errorf("%w: paragraph text", ErrLimit)
	}
	// A face that tracks its text by size is measured and shaped at each
	// span's, the same both times.
	spans = append([]Span(nil), spans...)
	for i := range spans {
		spans[i].Features = spans[i].Face.FeaturesAt(spans[i].Features, spans[i].Size.Px())
	}
	// Private faces, one per distinct source.
	var sources []*shape.Face
	faceOf := make([]int, len(spans))
	for i, s := range spans {
		faceOf[i] = -1
		for j, src := range sources {
			if src == s.Face {
				faceOf[i] = j
			}
		}
		if faceOf[i] < 0 {
			faceOf[i] = len(sources)
			sources = append(sources, s.Face)
		}
	}
	faces := make([]*shape.Face, len(sources))
	for i, src := range sources {
		f, err := t.page.fontFace(src, &t.budget)
		if err != nil {
			return nil, err
		}
		faces[i] = f
	}
	if total == 0 {
		return []RichLine{{Segments: []RichSegment{{Face: faces[faceOf[0]], Size: spans[0].Size}}}}, nil
	}
	remaining := l.MaxShapeWork - t.budget.shapeWork
	remainingGlyphs := l.MaxGlyphs - t.budget.glyphs
	share := remaining / int64(len(faces))
	if share <= 0 || remainingGlyphs <= 0 {
		return nil, fmt.Errorf("%w: paragraph budget", ErrLimit)
	}
	var (
		result     []RichLine
		glyphCount int
		work       int64
	)
	measures := make([]*shape.Face, len(faces))
	// Each face shapes inside its own limited scope; the scopes nest so all
	// are live while the paragraph breaks.
	var within func(i int) error
	within = func(i int) error {
		if i == len(faces) {
			return t.richBreak(ctx, spans, faceOf, faces, measures, width, remainingGlyphs, opts, &result, &glyphCount)
		}
		used, err := faces[i].WithShapingLimits(ctx, shape.RunLimits{MaxInputBytes: l.MaxRunBytes, MaxGlyphs: remainingGlyphs, MaxWork: share}, func(measure *shape.Face) error {
			measures[i] = measure
			return within(i + 1)
		})
		work += used
		return err
	}
	if err := within(0); err != nil {
		if errors.Is(err, shape.ErrRunLimit) {
			return nil, fmt.Errorf("%w: paragraph shaping: %w", ErrLimit, err)
		}
		return nil, err
	}
	t.budget.textBytes += total
	t.budget.glyphs += glyphCount
	t.budget.shapeWork += work
	t.lines += len(result)
	return result, nil
}

// hangsAlone reports whether a piece is one East Asian stop or comma.
func hangsAlone(piece string) bool {
	r, n := utf8.DecodeRuneInString(piece)
	return n == len(piece) && IsEastAsian(r) && paragraph.HangsAsStopOrComma(r)
}

// richItem locates a breaker item in the paragraph's spans.
type richItem struct {
	span, offset int
}

func (t *TextLayout) richBreak(ctx context.Context, spans []Span, faceOf []int, faces, measures []*shape.Face, width style.Unit, remainingGlyphs int, opts RichOptions, result *[]RichLine, glyphCount *int) error {
	l := t.page.limits
	var whole strings.Builder
	starts := make([]int, len(spans))
	for i, s := range spans {
		starts[i] = whole.Len()
		whole.WriteString(s.Text)
	}
	text := whole.String()
	br := paragraph.NewBreaker(nil)
	pieces, _ := paragraph.SplitAtBreaks(text, paragraph.WhiteSpace{PreserveBreaks: true, Wrap: true, BreakSpaces: true}, paragraph.WordBreak{}, paragraph.LineBreak{}, paragraph.Hyphens{}, paragraph.WritingSystemOther)
	var (
		items []paragraph.Item
		where []richItem
	)
	bd := newBidiLayout(text, opts.RTL)
	span, byteOffset := 0, 0
	for _, piece := range pieces {
		if err := ctx.Err(); err != nil {
			return err
		}
		if piece.ZeroWidth || piece.Segment {
			return fmt.Errorf("%w: paragraph break", ErrUnsupported)
		}
		end := byteOffset + len(piece.Text)
		if end > len(text) || text[byteOffset:end] != piece.Text {
			return fmt.Errorf("%w: paragraph preprocessing", ErrUnsupported)
		}
		// A piece crossing span boundaries becomes one item per span; only
		// the first may begin a line.
		for at := byteOffset; at < end; {
			for span < len(spans)-1 && at >= starts[span]+len(spans[span].Text) {
				span++
			}
			s := spans[span]
			from := at - starts[span]
			to := min(end-starts[span], len(s.Text))
			part := s.Text[from:to]
			measure := measures[faceOf[span]]
			if piece.Tab {
				// A tab's advance is resolved where it falls on a line.
				if part != "\t" || s.TabStop <= 0 || (bd != nil && !opts.RTL) {
					return fmt.Errorf("%w: paragraph tab", ErrUnsupported)
				}
				items = append(items, paragraph.Item{Text: part, Face: measure, Size: s.Size, Tab: true, TabStop: s.TabStop, BreakBefore: piece.BreakBefore && at == byteOffset})
				where = append(where, richItem{span: span, offset: from})
				at += len(part)
				continue
			}
			// An East Asian stop or comma ending the piece may hang, so it
			// is an item of its own, not to begin a line.
			cut := len(part)
			hangs := opts.HangPunct && bd == nil && !piece.Space
			if hangs {
				if r, n := utf8.DecodeLastRuneInString(part); n < len(part) && IsEastAsian(r) && paragraph.HangsAsStopOrComma(r) {
					cut -= n
				}
			}
			for k, ends := range []int{cut, len(part)} {
				if k == 1 && cut == len(part) {
					break
				}
				a, b := 0, ends
				if k == 1 {
					a = cut
				}
				how := paragraph.Shaping{MergeBefore: s.Text[:from+a], MergeAfter: s.Text[from+b:], MergeGroup: s.Text, ContextKerns: true, Off: s.Features}
				word := part[a:b]
				items = append(items, paragraph.Item{Text: word, Face: measure, Size: s.Size, Width: br.MeasureSpacedInContext(measure, word, s.Size, paragraph.TextSpacing{Letter: s.Letter}, how), BreakBefore: piece.BreakBefore && at == byteOffset && k == 0, Space: piece.Space, BreakWord: s.BreakWord, MayHangEnd: hangs && (k == 1 || (cut == len(part) && hangsAlone(word))), MergePre: how.MergeBefore, MergePost: how.MergeAfter, MergeGroup: s.Text, ContextKerns: true, Off: s.Features})
				where = append(where, richItem{span: span, offset: from + a})
			}
			at += len(part)
		}
		byteOffset = end
	}
	if byteOffset != len(text) {
		return fmt.Errorf("%w: paragraph preprocessing", ErrUnsupported)
	}
	lines := br.Lines(items)
	for index, offset := 0, 0; index < len(items); {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(*result) >= l.MaxOperations-t.lines {
			return fmt.Errorf("%w: paragraph lines", ErrLimit)
		}
		line, next, nextByte, floats, forced, hyphenated := lines.BreakOneLine(index, offset, width, 0)
		if !paragraph.CursorAdvanced(index, offset, next, nextByte) || len(floats) != 0 || forced || hyphenated {
			return fmt.Errorf("%w: paragraph breaking", ErrUnsupported)
		}
		// Without forced or hyphenated breaks line items match paragraph
		// items one for one, except that a word broken between characters
		// ends one line inside its item, at nextByte, and starts the next at
		// offset. Parts of one span shape as one segment.
		end := next
		if nextByte != 0 {
			end++
		}
		if end-index != len(line) {
			return fmt.Errorf("%w: paragraph line mapping", ErrUnsupported)
		}
		var segments []RichSegment
		for k, item := range line {
			i := index + k
			w := where[i]
			from, to := 0, len(items[i].Text)
			if k == 0 {
				from = offset
			}
			if i == next && nextByte != 0 {
				to = nextByte
			}
			if from > to || to > len(items[i].Text) || item.Text != items[i].Text[from:to] {
				return fmt.Errorf("%w: paragraph line mapping", ErrUnsupported)
			}
			// A tab is a segment of its own, which draws nothing.
			if n := len(segments); n > 0 && segments[n-1].Span == w.span && !item.Tab && segments[n-1].Text != "\t" {
				segments[n-1].Text += item.Text
				continue
			}
			segments = append(segments, RichSegment{Span: w.span, Offset: w.offset + from, Text: item.Text})
		}
		if bd != nil && len(segments) > 0 {
			segments = bd.arrange(segments, starts)
		}
		// Text segments are shaped first: a tab aligned at its stop needs
		// the width of the text after it.
		for k := range segments {
			sg := &segments[k]
			s := spans[sg.Span]
			sg.Face, sg.Size = faces[faceOf[sg.Span]], s.Size
			if sg.Text == "\t" {
				continue
			}
			// A right-to-left segment is stated so, and its glyphs come back
			// in drawing order; its override owns no glyph, so clusters are
			// put back to the segment's own text.
			shaped, prefix := sg.Text, 0
			if sg.RTL() {
				shaped, prefix = rtlPrefix+sg.Text, len(rtlPrefix)
			}
			glyphs, missing := measures[faceOf[sg.Span]].ShapeGlyphsMerged(shaped, "", "", "", "", false, s.Features)
			if missing != 0 {
				return fmt.Errorf("%w: paragraph missing glyph", ErrUnsupported)
			}
			if prefix > 0 {
				for i := range glyphs {
					glyphs[i].Cluster = max(glyphs[i].Cluster-prefix, 0)
				}
			}
			if len(glyphs) > remainingGlyphs-*glyphCount {
				return fmt.Errorf("%w: paragraph glyphs", ErrLimit)
			}
			*glyphCount += len(glyphs)
			if s.Letter != 0 && s.Size > 0 {
				// Letter spacing follows each character: the last glyph of
				// each cluster carries it, in the glyphs' 1000 units per em.
				extra := s.Letter.Px() * 1000 / s.Size.Px()
				glyphs = append([]shape.Glyph(nil), glyphs...)
				for i := range glyphs {
					if i+1 == len(glyphs) || glyphs[i+1].Cluster != glyphs[i].Cluster {
						glyphs[i].XAdvance += extra
					}
				}
			}
			advance := 0.0
			for _, glyph := range glyphs {
				advance += glyph.XAdvance
			}
			pixels := advance * s.Size.Px() / 1000
			w, okW := style.FromPx(pixels)
			if !okW || math.IsNaN(pixels) || math.IsInf(pixels, 0) || pixels < 0 {
				return fmt.Errorf("%w: paragraph advance", ErrInvalid)
			}
			sg.Glyphs, sg.Width = glyphs, w
		}
		pen, moved, approx := 0.0, false, false
		if bd != nil {
			var err error
			if moved, approx, err = rtlTabWidths(segments, spans); err != nil {
				return err
			}
		}
		for k := range segments {
			sg := &segments[k]
			at, ok := style.FromPx(pen)
			if !ok {
				return fmt.Errorf("%w: paragraph advance", ErrInvalid)
			}
			sg.X = at
			if sg.Text == "\t" && bd == nil {
				s := spans[sg.Span]
				even := paragraph.TabAdvance(at, s.TabStop, 0)
				w := tabAdvance(segments[k+1:], s.Tabs, pen, even.Px())
				if sg.Width, ok = style.FromPx(w); !ok {
					return fmt.Errorf("%w: paragraph advance", ErrInvalid)
				}
				moved = moved || sg.Width != even
			}
			pen += sg.Width.Px()
		}
		// A stop or comma that does not fit hangs outside the line's measure.
		// The breaker lets one overflow its line, and takes the hang back
		// when more text follows it, so the overflow is what says it hangs.
		hang := 0.0
		if n := len(line); n > 0 && line[n-1].MayHangEnd && pen > width.Px() {
			sg := segments[len(segments)-1]
			_, size := utf8.DecodeLastRuneInString(sg.Text)
			for _, g := range sg.Glyphs {
				if g.Cluster >= len(sg.Text)-size {
					hang += g.XAdvance * sg.Size.Px() / 1000
				}
			}
		}
		actual, ok := style.FromPx(pen - hang)
		hung, okH := style.FromPx(hang)
		if !ok || !okH {
			return fmt.Errorf("%w: paragraph advance", ErrInvalid)
		}
		if actual > width && !t.overflow {
			return fmt.Errorf("%w: paragraph overflow", ErrUnsupported)
		}
		*result = append(*result, RichLine{Segments: segments, Width: actual, Hang: hung, Overflow: actual > width, TabsMoved: moved, TabsApprox: approx})
		index, offset = next, nextByte
	}
	return nil
}

// tabAdvance is how far a tab at pen advances, in pixels: to the first
// explicit stop past pen, with the text after the tab aligned there, or
// else even, the distance to the next evenly spaced stop. Aligned text that
// would start before the tab leaves it no advance.
func tabAdvance(after []RichSegment, stops []TabStop, pen, even float64) float64 {
	var stop *TabStop
	for i := range stops {
		if stops[i].At.Px() > pen {
			stop = &stops[i]
			break
		}
	}
	if stop == nil {
		return even
	}
	// lead is how much of the text after the tab comes before the point
	// aligned at the stop.
	lead, whole := 0.0, 0.0
	point := false
	for _, sg := range after {
		if sg.Text == "\t" {
			break
		}
		em := sg.Size.Px() / 1000
		for _, g := range sg.Glyphs {
			if !point && g.Cluster >= 0 && g.Cluster < len(sg.Text) && sg.Text[g.Cluster] == '.' {
				lead, point = whole, true
			}
			whole += g.XAdvance * em
		}
	}
	switch stop.Align {
	case TabRight:
		lead = whole
	case TabCenter:
		lead = whole / 2
	case TabDecimal:
		if !point {
			lead = whole
		}
	default:
		lead = 0
	}
	return math.Max(0, stop.At.Px()-lead-pen)
}

// rtlTabWidths resolves the tabs of a right-to-left line, whose segments are
// in the order they are drawn. Positions are measured from the line's start,
// at its right end, going left, and the text after a tab, which lies to its
// left up to the next tab, sits at the stop by its physical alignment: its
// left edge at a left stop, its right edge at a right one. The line is placed
// afterwards, by the paragraph's alignment. approx marks a tab that no
// confirmed rule places.
func rtlTabWidths(segments []RichSegment, spans []Span) (moved, approx bool, err error) {
	pen := 0.0
	for k := len(segments) - 1; k >= 0; k-- {
		sg := &segments[k]
		if sg.Text == "\t" {
			s := spans[sg.Span]
			at, ok := style.FromPx(pen)
			if !ok {
				return false, false, fmt.Errorf("%w: paragraph advance", ErrInvalid)
			}
			even := paragraph.TabAdvance(at, s.TabStop, 0)
			w, exact := rtlTabAdvance(segments[:k], s.Tabs, pen, even.Px())
			if sg.Width, ok = style.FromPx(w); !ok {
				return false, false, fmt.Errorf("%w: paragraph advance", ErrInvalid)
			}
			moved = moved || sg.Width != even
			approx = approx || !exact
		}
		pen += sg.Width.Px()
	}
	return moved, approx, nil
}

// rtlTabAdvance is how far a tab at pen, measured from the line's start,
// advances. before are the segments to the tab's left, whose text follows it
// in reading order. exact is false when no confirmed rule gave the answer.
func rtlTabAdvance(before []RichSegment, stops []TabStop, pen, even float64) (advance float64, exact bool) {
	var stop *TabStop
	for i := range stops {
		if stops[i].At.Px() > pen {
			stop = &stops[i]
			break
		}
	}
	if stop == nil {
		return even, false
	}
	whole := 0.0
	for k := len(before) - 1; k >= 0 && before[k].Text != "\t"; k-- {
		whole += before[k].Width.Px()
	}
	at := stop.At.Px()
	switch stop.Align {
	case TabLeft:
		return math.Max(0, at-whole-pen), true
	case TabRight:
		return math.Max(0, at-pen), true
	case TabCenter:
		return math.Max(0, at-whole/2-pen), false
	}
	return math.Max(0, at-pen), false
}
