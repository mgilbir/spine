package render

import (
	"sort"
	"unicode/utf8"

	"github.com/mgilbir/forme/bidi"
)

// rtlPrefix is the right-to-left override that states a run's direction to the
// shaper, which applies the bidirectional algorithm to the string it is
// given, and answers for that string and not for the paragraph the run came
// from: a lone bracket would face the wrong way. The override is a
// default-ignorable character, so shaping drops it before choosing a glyph;
// what comes back is the run's glyphs in the order they are drawn, with the
// characters that mirror drawn mirrored. It is how forme's own layout hands a
// resolved right-to-left run to a backend.
const rtlPrefix = "\u202e"

// bidiLayout holds a paragraph's embedding levels, resolved once, for lines
// that are cut and ordered after the paragraph is broken.
type bidiLayout struct {
	para *bidi.Paragraph
	// offsets are the byte offsets of the paragraph's characters, and its
	// length after them.
	offsets []int
}

// newBidiLayout resolves a paragraph's text in a base direction. It returns
// nil for text that runs left to right throughout, which the algorithm leaves
// as it is.
func newBidiLayout(text string, rtl bool) *bidiLayout {
	if !rtl && !bidi.NeedsAlgorithm(text) {
		return nil
	}
	runes := make([]rune, 0, len(text))
	offsets := make([]int, 0, len(text)+1)
	for i, r := range text {
		runes, offsets = append(runes, r), append(offsets, i)
	}
	offsets = append(offsets, len(text))
	dir := bidi.LeftToRight
	if rtl {
		dir = bidi.RightToLeft
	}
	return &bidiLayout{para: bidi.Resolve(runes, dir), offsets: offsets}
}

// index is the character at a byte offset of the paragraph's text.
func (b *bidiLayout) index(offset int) int {
	return sort.SearchInts(b.offsets, offset)
}

// arrange cuts a line's segments, in logical order, where the embedding level
// changes, and returns the pieces in the order they are drawn, left to right.
// starts are the byte offsets where the paragraph's spans begin in its text.
// The level of white space ending the line is the paragraph's, as rule L1
// has it.
func (b *bidiLayout) arrange(segments []RichSegment, starts []int) []RichSegment {
	first, last := segments[0], segments[len(segments)-1]
	from := b.index(starts[first.Span] + first.Offset)
	to := b.index(starts[last.Span] + last.Offset + len(last.Text))
	levels := b.para.LineLevels(from, to)
	var (
		out    []RichSegment
		levelV []int
	)
	k := 0
	for _, sg := range segments {
		for off := 0; off < len(sg.Text); {
			level, end := levels[k], off
			for end < len(sg.Text) && levels[k] == level {
				_, n := utf8.DecodeRuneInString(sg.Text[end:])
				end, k = end+n, k+1
			}
			out = append(out, RichSegment{Span: sg.Span, Offset: sg.Offset + off, Text: sg.Text[off:end], Level: level})
			levelV = append(levelV, level)
			off = end
		}
	}
	ordered := make([]RichSegment, len(out))
	for i, at := range bidi.VisualOrder(levelV) {
		ordered[i] = out[at]
	}
	return ordered
}
