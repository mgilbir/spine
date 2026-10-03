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
}

// RichLine is one wrapped line of a rich paragraph.
type RichLine struct {
	Segments []RichSegment
	Width    style.Unit
	// Overflow marks a line wider than the wrapping width, which only a
	// layout allowing overflow returns.
	Overflow bool
	// TabsMoved marks a line where an explicit stop moved a tab from where
	// line breaking measured it.
	TabsMoved bool
}

// RichLines wraps a horizontal left-to-right paragraph whose spans may differ
// in face, size and features. Break opportunities come from the whole
// paragraph text; shaping context stays within a span, as formats that shape
// runs apart require, so no glyph or contextual form crosses a span boundary.
// An empty paragraph yields one line holding an empty segment of its first
// span, which carries that span's face. Budgets are shared with Lines; the
// remaining shaping work is divided evenly between the paragraph's faces.
func (t *TextLayout) RichLines(ctx context.Context, spans []Span, width style.Unit, repertoire Repertoire) ([]RichLine, error) {
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
			return t.richBreak(ctx, spans, faceOf, faces, measures, width, remainingGlyphs, &result, &glyphCount)
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

// richItem locates a breaker item in the paragraph's spans.
type richItem struct {
	span, offset int
}

func (t *TextLayout) richBreak(ctx context.Context, spans []Span, faceOf []int, faces, measures []*shape.Face, width style.Unit, remainingGlyphs int, result *[]RichLine, glyphCount *int) error {
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
			how := paragraph.Shaping{MergeBefore: s.Text[:from], MergeAfter: s.Text[to:], MergeGroup: s.Text, ContextKerns: true, Off: s.Features}
			if piece.Tab {
				// A tab's advance is resolved where it falls on a line.
				if part != "\t" || s.TabStop <= 0 {
					return fmt.Errorf("%w: paragraph tab", ErrUnsupported)
				}
				items = append(items, paragraph.Item{Text: part, Face: measure, Size: s.Size, Tab: true, TabStop: s.TabStop, BreakBefore: piece.BreakBefore && at == byteOffset})
				where = append(where, richItem{span: span, offset: from})
				at += len(part)
				continue
			}
			items = append(items, paragraph.Item{Text: part, Face: measure, Size: s.Size, Width: br.MeasureSpacedInContext(measure, part, s.Size, paragraph.TextSpacing{Letter: s.Letter}, how), BreakBefore: piece.BreakBefore && at == byteOffset, Space: piece.Space, MergePre: how.MergeBefore, MergePost: how.MergeAfter, MergeGroup: s.Text, ContextKerns: true, Off: s.Features})
			where = append(where, richItem{span: span, offset: from})
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
		// Without forced or hyphenated breaks a line starts and ends on item
		// boundaries, so line items match paragraph items one for one; parts
		// of one span shape as one segment.
		if offset != 0 || nextByte != 0 || next != index+len(line) {
			return fmt.Errorf("%w: paragraph line mapping", ErrUnsupported)
		}
		var segments []RichSegment
		for k, item := range line {
			w := where[index+k]
			if item.Text != items[index+k].Text {
				return fmt.Errorf("%w: paragraph line mapping", ErrUnsupported)
			}
			// A tab is a segment of its own, which draws nothing.
			if n := len(segments); n > 0 && segments[n-1].Span == w.span && !item.Tab && segments[n-1].Text != "\t" {
				segments[n-1].Text += item.Text
				continue
			}
			segments = append(segments, RichSegment{Span: w.span, Offset: w.offset, Text: item.Text})
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
			glyphs, missing := measures[faceOf[sg.Span]].ShapeGlyphsMerged(sg.Text, "", "", "", "", false, s.Features)
			if missing != 0 {
				return fmt.Errorf("%w: paragraph missing glyph", ErrUnsupported)
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
		pen, moved := 0.0, false
		for k := range segments {
			sg := &segments[k]
			at, ok := style.FromPx(pen)
			if !ok {
				return fmt.Errorf("%w: paragraph advance", ErrInvalid)
			}
			sg.X = at
			if sg.Text == "\t" {
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
		actual, ok := style.FromPx(pen)
		if !ok {
			return fmt.Errorf("%w: paragraph advance", ErrInvalid)
		}
		if actual > width && !t.overflow {
			return fmt.Errorf("%w: paragraph overflow", ErrUnsupported)
		}
		*result = append(*result, RichLine{Segments: segments, Width: actual, Overflow: actual > width, TabsMoved: moved})
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
