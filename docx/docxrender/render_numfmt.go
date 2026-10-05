package docxrender

import (
	"strconv"
	"strings"
)

// Number formats (w:numFmt). A counter value is formatted by the level's
// format; a format the profile does not draw as Word does is drawn as decimal
// and reported as approximated by the caller.

// wordFormatNumber formats counter value v (0 to wordMaxListValue) in the
// named format. exact is false when the text is not what Word draws: the
// format is not known, the value is outside what the format can express, or
// Word's text depends on the document's language, which the profile does not
// model (English is drawn).
func wordFormatNumber(format string, v int) (text string, exact bool) {
	dec := strconv.Itoa(v)
	switch format {
	case "decimal", "decimalHalfWidth":
		return dec, true
	case "decimalZero":
		if v < 10 {
			return "0" + dec, true
		}
		return dec, true
	case "none":
		return "", true
	case "upperRoman", "lowerRoman":
		if v < 1 || v > 3999 {
			return dec, false
		}
		s := wordRoman(v)
		if format == "lowerRoman" {
			s = strings.ToLower(s)
		}
		return s, true
	case "upperLetter", "lowerLetter":
		if v < 1 {
			return dec, false
		}
		// a to z, then aa to zz, then aaa: the letter repeated.
		base := 'A'
		if format == "lowerLetter" {
			base = 'a'
		}
		return strings.Repeat(string(base+rune((v-1)%26)), (v-1)/26+1), true
	case "chicago":
		if v < 1 {
			return dec, false
		}
		return strings.Repeat([]string{"*", "†", "‡", "§"}[(v-1)%4], (v-1)/4+1), true
	case "decimalEnclosedParen":
		return "(" + dec + ")", true
	case "decimalEnclosedFullstop":
		return dec + ".", true
	case "decimalEnclosedCircle":
		if v < 1 || v > 20 {
			return dec, false
		}
		return string(rune(0x2460 + v - 1)), true
	case "decimalFullWidth", "decimalFullWidth2":
		var sb strings.Builder
		for _, d := range dec {
			sb.WriteRune(0xFF10 + d - '0')
		}
		return sb.String(), true
	case "ordinal":
		return dec + wordOrdinalSuffix(v), false
	case "cardinalText":
		return wordEnglishWords(v, false), false
	case "ordinalText":
		return wordEnglishWords(v, true), false
	}
	return dec, false
}

func wordRoman(v int) string {
	var sb strings.Builder
	for _, r := range [...]struct {
		n int
		s string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}} {
		for v >= r.n {
			sb.WriteString(r.s)
			v -= r.n
		}
	}
	return sb.String()
}

func wordOrdinalSuffix(v int) string {
	if v%100 >= 11 && v%100 <= 13 {
		return "th"
	}
	switch v % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	}
	return "th"
}

var (
	wordOnes = [...]string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
		"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	wordOnesOrd = [...]string{"zeroth", "first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth",
		"eleventh", "twelfth", "thirteenth", "fourteenth", "fifteenth", "sixteenth", "seventeenth", "eighteenth", "nineteenth"}
	wordTens    = [...]string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	wordTensOrd = [...]string{"", "", "twentieth", "thirtieth", "fortieth", "fiftieth", "sixtieth", "seventieth", "eightieth", "ninetieth"}
)

// wordEnglishWords spells v (below one million) in English with a capital
// first letter.
func wordEnglishWords(v int, ordinal bool) string {
	var parts []string
	rest := v
	if rest >= 1000 {
		parts = append(parts, wordUnder1000(rest/1000, false)+" thousand")
		rest %= 1000
		if rest == 0 && ordinal {
			parts[0] += "th"
		}
	}
	if rest > 0 || v == 0 {
		parts = append(parts, wordUnder1000(rest, ordinal))
	}
	s := strings.Join(parts, " ")
	return strings.ToUpper(s[:1]) + s[1:]
}

func wordUnder1000(v int, ordinal bool) string {
	var parts []string
	if v >= 100 {
		parts = append(parts, wordOnes[v/100]+" hundred")
		v %= 100
		if v == 0 {
			if ordinal {
				parts[0] += "th"
			}
			return parts[0]
		}
	}
	switch {
	case v < 20 && ordinal:
		parts = append(parts, wordOnesOrd[v])
	case v < 20:
		parts = append(parts, wordOnes[v])
	case v%10 == 0 && ordinal:
		parts = append(parts, wordTensOrd[v/10])
	case v%10 == 0:
		parts = append(parts, wordTens[v/10])
	case ordinal:
		parts = append(parts, wordTens[v/10]+"-"+wordOnesOrd[v%10])
	default:
		parts = append(parts, wordTens[v/10]+"-"+wordOnes[v%10])
	}
	return strings.Join(parts, " and ")
}

// wordNumberText formats n in a w:numFmt number format as page numbers and
// note marks use it, where an absent format is decimal.
func wordNumberText(n int, format string) (text string, exact bool) {
	if format == "" {
		format = "decimal"
	}
	return wordFormatNumber(format, n)
}
