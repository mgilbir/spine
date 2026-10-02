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

// TextLayout shares font, text, glyph and measurement budgets across paragraphs.
// It is private to one preparation and must not be used concurrently.
type TextLayout struct {
	page   Page
	budget prepareBudget
	lines  int
}

// TextLine contains owned glyph placements, measured in 1000 units per em.
// Face is a private font clone; the caller must keep it immutable.
type TextLine struct {
	Text   string
	Glyphs []shape.Glyph
	Face   *shape.Face
	Width  style.Unit
}

func NewTextLayout(limits Limits) (*TextLayout, error) {
	l, err := ResolveLimits(limits)
	if err != nil {
		return nil, err
	}
	return &TextLayout{page: Page{limits: l}}, nil
}

// PlainLines wraps a single horizontal ASCII paragraph using Forme's paragraph
// breaker. Spaces are preserved. Tabs, hard breaks, discretionary characters,
// bidi text and overlong words require a richer format-specific profile.
// Every candidate measurement and final shaping shares one remaining budget.
func (t *TextLayout) PlainLines(ctx context.Context, source *shape.Face, text string, size, width style.Unit) ([]TextLine, error) {
	return t.PlainLinesWithFeatures(ctx, source, text, size, width, shape.Features{})
}

// PlainLinesWithFeatures uses the same feature settings for candidate widths and
// final glyph placement. Adapters must resolve their format's feature defaults.
func (t *TextLayout) PlainLinesWithFeatures(ctx context.Context, source *shape.Face, text string, size, width style.Unit, features shape.Features) ([]TextLine, error) {
	if t == nil || ctx == nil || size <= 0 || width <= 0 || !utf8.ValidString(text) {
		return nil, fmt.Errorf("%w: paragraph input", ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l := t.page.limits
	if len(text) > l.MaxRunBytes || len(text) > l.MaxTextBytes-t.budget.textBytes {
		return nil, fmt.Errorf("%w: paragraph text", ErrLimit)
	}
	for _, c := range text {
		if c < 32 || c > 126 {
			return nil, fmt.Errorf("%w: plain paragraph character", ErrUnsupported)
		}
	}
	face, err := t.page.fontFace(source, &t.budget)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return []TextLine{{Face: face}}, nil
	}
	remaining := l.MaxShapeWork - t.budget.shapeWork
	remainingGlyphs := l.MaxGlyphs - t.budget.glyphs
	if remaining <= 0 || remainingGlyphs <= 0 {
		return nil, fmt.Errorf("%w: paragraph budget", ErrLimit)
	}
	var result []TextLine
	var glyphCount int
	work, err := face.WithShapingLimits(ctx, shape.RunLimits{MaxInputBytes: l.MaxRunBytes, MaxGlyphs: remainingGlyphs, MaxWork: remaining}, func(measure *shape.Face) error {
		br := paragraph.NewBreaker(nil)
		pieces, _ := paragraph.SplitAtBreaks(text, paragraph.WhiteSpace{PreserveBreaks: true, Wrap: true, BreakSpaces: true}, paragraph.WordBreak{}, paragraph.LineBreak{}, paragraph.Hyphens{}, paragraph.WritingSystemOther)
		items := make([]paragraph.Item, len(pieces))
		byteOffset := 0
		for i, piece := range pieces {
			if err := ctx.Err(); err != nil {
				return err
			}
			if piece.ZeroWidth || piece.Tab || piece.Segment {
				return fmt.Errorf("%w: paragraph break", ErrUnsupported)
			}
			end := byteOffset + len(piece.Text)
			if end > len(text) || text[byteOffset:end] != piece.Text {
				return fmt.Errorf("%w: paragraph preprocessing", ErrUnsupported)
			}
			how := paragraph.Shaping{MergeBefore: text[:byteOffset], MergeAfter: text[end:], MergeGroup: text, ContextKerns: true, Off: features}
			items[i] = paragraph.Item{Text: piece.Text, Face: measure, Size: size, Width: br.MeasureSpacedInContext(measure, piece.Text, size, paragraph.TextSpacing{}, how), BreakBefore: piece.BreakBefore, Space: piece.Space, MergePre: how.MergeBefore, MergePost: how.MergeAfter, MergeGroup: text, ContextKerns: true, Off: features}
			byteOffset = end
		}
		lines := br.Lines(items)
		for index, offset := 0, 0; index < len(items); {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(result) >= l.MaxOperations-t.lines {
				return fmt.Errorf("%w: paragraph lines", ErrLimit)
			}
			line, next, nextByte, floats, forced, hyphenated := lines.BreakOneLine(index, offset, width, 0)
			if !paragraph.CursorAdvanced(index, offset, next, nextByte) || len(floats) != 0 || forced || hyphenated {
				return fmt.Errorf("%w: paragraph breaking", ErrUnsupported)
			}
			var joined strings.Builder
			for _, item := range line {
				joined.WriteString(item.Text)
			}
			value := joined.String()
			glyphs, missing := measure.ShapeGlyphsMerged(value, "", "", "", "", false, features)
			if missing != 0 {
				return fmt.Errorf("%w: paragraph missing glyph", ErrUnsupported)
			}
			if len(glyphs) > remainingGlyphs-glyphCount {
				return fmt.Errorf("%w: paragraph glyphs", ErrLimit)
			}
			glyphCount += len(glyphs)
			advance := 0.0
			for _, glyph := range glyphs {
				advance += glyph.XAdvance
			}
			pixels := advance * size.Px() / 1000
			actual, ok := style.FromPx(pixels)
			if !ok || math.IsNaN(pixels) || math.IsInf(pixels, 0) || pixels < 0 {
				return fmt.Errorf("%w: paragraph advance", ErrInvalid)
			}
			if actual > width {
				return fmt.Errorf("%w: paragraph overflow", ErrUnsupported)
			}
			result = append(result, TextLine{Text: value, Glyphs: glyphs, Face: face, Width: actual})
			index, offset = next, nextByte
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, shape.ErrRunLimit) {
			return nil, fmt.Errorf("%w: paragraph shaping: %w", ErrLimit, err)
		}
		return nil, err
	}
	t.budget.textBytes += len(text)
	t.budget.glyphs += glyphCount
	t.budget.shapeWork += work
	t.lines += len(result)
	return result, nil
}
