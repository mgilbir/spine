package docxrender

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The renderer hands layout generated HTML and CSS. Nothing from the document
// reaches the markup except text nodes, and text is escaped here: families are
// replaced by synthetic names (wordFonts), colours are formatted from integers,
// and every length is formatted from a float64 clamped to wordMaxLength. No
// attribute value, selector, URL or style text is ever copied from the source.

// wordEscape appends s as HTML text. It escapes markup characters, maps
// control characters other than tab to a space, and replaces invalid UTF-8.
func wordEscape(sb *strings.Builder, s string) {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	for _, c := range s {
		switch {
		case c == '&':
			sb.WriteString("&amp;")
		case c == '<':
			sb.WriteString("&lt;")
		case c == '>':
			sb.WriteString("&gt;")
		case c == '\t':
			sb.WriteByte('\t')
		case c == 0 || c == 0xFFFE || c == 0xFFFF:
			// Not valid in XML text; drop.
		case unicode.IsControl(c) || c == ' ' || c == ' ':
			sb.WriteByte(' ')
		default:
			sb.WriteRune(c)
		}
	}
}

// wordCSS builds a style attribute value.
type wordCSS struct{ s string }

// px formats a length in CSS pixels. Non-finite values and values beyond the
// profile's length bound are clamped so a hostile document cannot write an
// unparseable number.
func wordPx(v float64) string {
	if math.IsNaN(v) {
		v = 0
	}
	v = math.Max(-wordMaxLength, math.Min(wordMaxLength, v))
	return strconv.FormatFloat(v, 'f', 3, 64) + "px"
}

func (c *wordCSS) add(prop, value string) {
	c.s += prop + ":" + value + ";"
}

func (c *wordCSS) px(prop string, v float64) { c.add(prop, wordPx(v)) }

func (c *wordCSS) rgb(prop string, col wordRGB) {
	c.add(prop, "rgb("+strconv.Itoa(int(col.r))+","+strconv.Itoa(int(col.g))+","+strconv.Itoa(int(col.b))+")")
}

func (c *wordCSS) String() string { return c.s }
