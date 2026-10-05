package render

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
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
	// overflow lets rich lines run past their width; see AllowOverflow.
	overflow bool
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

// AllowOverflow lets RichLines return a line wider than its width, such as
// an overlong word, marked as overflowing, instead of failing.
func (t *TextLayout) AllowOverflow() { t.overflow = true }

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
	return t.Lines(ctx, source, text, size, width, features, RepertoireASCII)
}

// Repertoire selects the characters a plain paragraph may contain. A format
// whose font choice depends on the character class, as Word's ASCII and high
// ANSI fonts do, must keep to the class its single font serves.
type Repertoire int

const (
	// RepertoireASCII is printable ASCII.
	RepertoireASCII Repertoire = iota
	// RepertoireEuropean adds the Latin, Greek and Cyrillic scripts, combining
	// diacritics, Latin-1, general punctuation, currency and letterlike
	// symbols, arrows, mathematical operators and geometric shapes. Each is
	// left-to-right or neutral and none is an East Asian script; controls,
	// format characters, separators and the soft hyphen are left out. Some of
	// the symbols have ambiguous East Asian width, which formats may draw with
	// an East Asian font in East Asian text; callers must exclude that case.
	RepertoireEuropean
	// RepertoireEastAsian adds the East Asian scripts to RepertoireEuropean:
	// ideographs, kana, hangul and bopomofo, the punctuation and symbols
	// set with them, and the halfwidth and fullwidth forms. See IsEastAsian.
	RepertoireEastAsian
	// RepertoireBidi adds the right-to-left scripts to RepertoireEastAsian:
	// Hebrew, Arabic, Syriac, Thaana and N'Ko with their presentation forms,
	// and the joiners and direction marks set among them. See IsComplex and
	// IsFormat.
	RepertoireBidi
)

// Allows reports whether the repertoire includes a character.
func (r Repertoire) Allows(c rune) bool { return r.allows(c) }

func (r Repertoire) allows(c rune) bool {
	if c >= 32 && c <= 126 {
		return true
	}
	if r != RepertoireEuropean && r != RepertoireEastAsian && r != RepertoireBidi {
		return false
	}
	if r != RepertoireEuropean && IsEastAsian(c) && unicode.IsGraphic(c) {
		return true
	}
	if r == RepertoireBidi && ((IsComplex(c) && unicode.IsGraphic(c)) || IsFormat(c)) {
		return true
	}
	switch {
	case c == 0xAD: // soft hyphen, a discretionary break
		return false
	case c >= 0xA0 && c <= 0xFF, // Latin-1
		c >= 0x300 && c <= 0x36F,   // combining diacritics
		c >= 0x2010 && c <= 0x2027, // dashes, quotes, bullets, ellipsis
		c >= 0x2030 && c <= 0x205E, // per mille, primes, guillemets
		c >= 0x20A0 && c <= 0x20C0, // currency
		c >= 0x2100 && c <= 0x214F, // letterlike symbols
		c >= 0x2190 && c <= 0x22FF, // arrows, mathematical operators
		c >= 0x25A0 && c <= 0x25FF: // geometric shapes
		return true
	case c >= 0xFF00: // halfwidth and fullwidth forms
		return false
	}
	return unicode.In(c, unicode.Latin, unicode.Greek, unicode.Cyrillic)
}

// Lines wraps a plain paragraph drawn from a repertoire; see PlainLines.
func (t *TextLayout) Lines(ctx context.Context, source *shape.Face, text string, size, width style.Unit, features shape.Features, repertoire Repertoire) ([]TextLine, error) {
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
		if !repertoire.allows(c) {
			return nil, fmt.Errorf("%w: plain paragraph character", ErrUnsupported)
		}
	}
	face, err := t.page.fontFace(source, &t.budget)
	if err != nil {
		return nil, err
	}
	// A face that tracks its text by size is shaped at this one.
	features = face.FeaturesAt(features, size.Px())
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
