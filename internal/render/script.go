package render

import "unicode"

// IsEastAsian reports whether a character belongs to the East Asian scripts
// a format sets in its East Asian font: ideographs, kana, hangul and
// bopomofo, the CJK symbols and punctuation, enclosed and compatibility
// forms, and the halfwidth and fullwidth forms. ASCII and the symbols of
// ambiguous East Asian width are not: which font draws those depends on the
// language and on the format.
func IsEastAsian(c rune) bool {
	switch {
	case c < 0x1100:
		return false
	case c >= 0x1100 && c <= 0x11FF, // hangul jamo
		c >= 0x2E80 && c <= 0x2FDF, // CJK and Kangxi radicals
		c >= 0x2FF0 && c <= 0x2FFF, // ideographic description characters
		c >= 0x3000 && c <= 0x303F, // CJK symbols and punctuation
		c >= 0x3099 && c <= 0x309A, // combining marks of kana
		c >= 0x3200 && c <= 0x33FF, // enclosed CJK letters, CJK compatibility
		c >= 0xA960 && c <= 0xA97F, // hangul jamo extended-A
		c >= 0xD7B0 && c <= 0xD7FF, // hangul jamo extended-B
		c >= 0xFE30 && c <= 0xFE4F, // CJK compatibility forms
		c >= 0xFE50 && c <= 0xFE6F, // small form variants
		c >= 0xFF00 && c <= 0xFFEF: // halfwidth and fullwidth forms
		return true
	}
	return unicode.In(c, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Bopomofo)
}
