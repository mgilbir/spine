// Package sheetref formats the worksheet part of an A1-style formula
// reference.
//
// Excel's formula grammar resolves an unquoted sheet name only when the name
// cannot be read as something else. A name that lexes as a cell reference
// ("A1", "XFD1048576", "R1C1", a bare "R" or "C") or as a boolean literal
// ("TRUE", "FALSE") is ambiguous unquoted: `A1!$B$1` is not a reference to
// cell B1 of a sheet called A1. Excel itself always writes those names quoted
// (`'A1'!$B$1`).
//
// The rules are shared by every producer of `<c:f>` chart references and of
// print-title / print-area definitions, so they live here rather than in one
// format package.
package sheetref

import "strings"

// maxColumn is the last spreadsheet column ("XFD", 16384) and maxRow the last
// row (1048576). A letter/digit pair beyond those is not a cell reference and
// so is not ambiguous.
const (
	maxColumn = 16384
	maxRow    = 1048576
)

// QuoteName returns name as it must appear before the '!' of an A1-style
// formula reference: bare when it is unambiguous, otherwise wrapped in single
// quotes with embedded quotes doubled. An empty name is quoted.
//
// A name is left bare only when every character is a letter, digit, '_' or
// '.', it does not start with a digit, and it does not lex as a cell reference
// or a boolean literal.
func QuoteName(name string) string {
	if !NeedsQuoting(name) {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// NeedsQuoting reports whether name must be quoted to appear unambiguously
// before the '!' of an A1-style formula reference.
func NeedsQuoting(name string) bool {
	if name == "" {
		return true
	}
	for i, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '_':
		case r >= '0' && r <= '9':
			if i == 0 {
				return true
			}
		case r == '.':
			if i == 0 {
				return true
			}
		default:
			return true
		}
	}
	return LooksLikeReference(name)
}

// LooksLikeReference reports whether name lexes as something other than a sheet
// name in a formula: an A1-style cell reference, an R1C1-style reference, or a
// boolean literal. Comparison is case-insensitive, as Excel's grammar is.
func LooksLikeReference(name string) bool {
	up := strings.ToUpper(name)
	if up == "TRUE" || up == "FALSE" {
		return true
	}
	return isA1Ref(up) || isR1C1Ref(up)
}

// isA1Ref reports whether an upper-cased name is an A1-style cell reference:
// one to three column letters within "XFD" followed by a row number within
// 1..1048576, and nothing else.
func isA1Ref(up string) bool {
	i := 0
	col := 0
	for i < len(up) && up[i] >= 'A' && up[i] <= 'Z' {
		col = col*26 + int(up[i]-'A') + 1
		i++
		if col > maxColumn {
			return false
		}
	}
	if i == 0 || i >= len(up) {
		return false
	}
	row := 0
	for ; i < len(up); i++ {
		if up[i] < '0' || up[i] > '9' {
			return false
		}
		row = row*10 + int(up[i]-'0')
		if row > maxRow {
			return false
		}
	}
	return row >= 1
}

// isR1C1Ref reports whether an upper-cased name is an R1C1-style reference:
// "R", "C", "RC", or either letter followed by digits ("R1", "C12", "R1C1").
// Excel accepts R1C1 references in formulas regardless of the workbook's
// reference style, so those names are ambiguous unquoted too.
func isR1C1Ref(up string) bool {
	rest, ok := digitsAfter(up, 'R')
	if ok {
		if rest == "" {
			return true
		}
		tail, okC := digitsAfter(rest, 'C')
		return okC && tail == ""
	}
	rest, ok = digitsAfter(up, 'C')
	return ok && rest == ""
}

// digitsAfter matches a leading letter followed by zero or more digits and
// returns the remainder. ok is false when the string does not start with the
// letter.
func digitsAfter(s string, letter byte) (rest string, ok bool) {
	if s == "" || s[0] != letter {
		return s, false
	}
	i := 1
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[i:], true
}
