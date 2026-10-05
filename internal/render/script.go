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

// IsComplex reports whether a character belongs to the right-to-left scripts
// a format sets in its complex-script font and this package lays out:
// Hebrew, Arabic, Syriac, Thaana and N'Ko, with the Arabic and Hebrew
// presentation forms, and the Arabic comma, semicolon, question mark, tatweel
// and digits that go with them.
func IsComplex(c rune) bool {
	switch {
	case c < 0x590:
		return false
	case c >= 0x590 && c <= 0x7FF, // Hebrew to N'Ko
		c >= 0x870 && c <= 0x8FF,   // Arabic extended-B and -A
		c >= 0xFB1D && c <= 0xFB4F, // Hebrew presentation forms
		c >= 0xFB50 && c <= 0xFDCF, // Arabic presentation forms-A
		c >= 0xFDF0 && c <= 0xFDFF,
		c >= 0xFE70 && c <= 0xFEFE: // Arabic presentation forms-B
		return !unicode.Is(unicode.Cf, c)
	}
	return false
}

// IsFormat reports whether a character is one of the invisible format
// characters set among right-to-left text: the zero width joiner and
// non-joiner, and the left-to-right, right-to-left and Arabic letter marks.
// They take no room and take part in shaping and in the bidirectional
// algorithm.
func IsFormat(c rune) bool {
	switch c {
	case 0x200C, 0x200D, 0x200E, 0x200F, 0x061C:
		return true
	}
	return false
}
