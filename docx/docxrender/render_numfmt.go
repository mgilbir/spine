package docxrender

import (
	"strconv"
	"strings"
)

// wordNumberText formats n in a w:numFmt number format (the formats page
// numbers and note marks use). exact is false when the format is not one the
// profile draws and decimal was used instead, or when n is outside what the
// format can show.
func wordNumberText(n int, format string) (text string, exact bool) {
	switch format {
	case "", "decimal":
		return strconv.Itoa(n), true
	case "decimalZero":
		if n >= 0 && n < 10 {
			return "0" + strconv.Itoa(n), true
		}
		return strconv.Itoa(n), true
	case "upperRoman", "lowerRoman":
		s, ok := wordRoman(n)
		if !ok {
			return strconv.Itoa(n), false
		}
		if format == "lowerRoman" {
			s = strings.ToLower(s)
		}
		return s, true
	case "upperLetter", "lowerLetter":
		if n < 1 {
			return strconv.Itoa(n), false
		}
		// a, b, ... z, aa, bb, ...: the letter repeats once per 26 numbers.
		c := rune('A' + (n-1)%26)
		if format == "lowerLetter" {
			c = rune('a' + (n-1)%26)
		}
		return strings.Repeat(string(c), 1+(n-1)/26), true
	case "chicago":
		if n < 1 {
			return strconv.Itoa(n), false
		}
		marks := []string{"*", "†", "‡", "§"}
		return strings.Repeat(marks[(n-1)%4], 1+(n-1)/4), true
	}
	return strconv.Itoa(n), false
}

// wordRoman is n in upper-case Roman numerals, for 1 to 3999.
func wordRoman(n int) (string, bool) {
	if n < 1 || n > 3999 {
		return "", false
	}
	var sb strings.Builder
	for _, d := range []struct {
		v int
		s string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}} {
		for n >= d.v {
			sb.WriteString(d.s)
			n -= d.v
		}
	}
	return sb.String(), true
}
