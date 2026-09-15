package oxml

import (
	"errors"
	"strconv"
	"strings"
)

// Worksheet grid limits (Excel 2007+): 1,048,576 rows by 16,384 columns (XFD).
const (
	MaxRow = 1048576
	MaxCol = 16384
)

// ErrInvalidRef is returned for text that is not a cell reference inside the
// grid. The xlsx package maps it onto its own ErrInvalidCell.
var ErrInvalidRef = errors.New("oxml: invalid cell reference")

// ColumnLetters converts a 1-based column number to column letters. It returns
// "" for a non-positive column, which callers must treat as invalid.
func ColumnLetters(col int) string {
	if col < 1 {
		return ""
	}
	// Built back-to-front in a fixed buffer. The obvious loop prepends with
	// result = string(rune(...)) + result, which allocates once per letter and
	// again for each concatenation — and this runs per cell on the marshal
	// path. MaxCol is 16384 ("XFD"), so three letters is the most there can be.
	var buf [4]byte
	i := len(buf)
	for col > 0 {
		col--
		i--
		buf[i] = byte('A' + col%26)
		col /= 26
	}
	return string(buf[i:])
}

// refIsCanonical reports whether ref is spelled exactly as CellRefString would
// spell the position it denotes, without building that string to find out.
//
// Formatting the canonical form and comparing cost an allocation per cell on
// the parse path, for a question answerable by looking at the text: the column
// encoding is bijective, so an all-uppercase letter run that parses is the only
// spelling of its column, and a decimal row without a leading zero is the only
// spelling of its number. Anything else — lower case, a padded row, trailing
// junk — is not canonical.
//
// It assumes nothing about ref; callers pair it with ParseRefString, which
// decides whether ref denotes a position at all.
func refIsCanonical(ref string) bool {
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i == len(ref) || ref[i] == '0' {
		return false
	}
	for j := i; j < len(ref); j++ {
		if ref[j] < '0' || ref[j] > '9' {
			return false
		}
	}
	return true
}

// CellRefString builds a cell reference from 1-based row and column numbers. It
// returns "" for coordinates outside the worksheet grid rather than an invalid
// reference such as "5" (column 0).
func CellRefString(row, col int) string {
	if row < 1 || row > MaxRow || col < 1 || col > MaxCol {
		return ""
	}
	// Plain concatenation rather than fmt.Sprintf: this is on the hot path of
	// every range walk, and the formatted form cost an interface boxing plus a
	// reflection-driven format pass per cell.
	return ColumnLetters(col) + strconv.Itoa(row)
}

// ParseRefString parses a cell reference like "A1" into 1-based row and column
// numbers. It rejects references outside the worksheet grid and guards against
// integer overflow from pathologically long column strings.
//
// This is the one implementation; xlsx.ParseCellRef wraps it. Keeping a second
// copy next to the model would be the C383 hazard — two carves of the same
// rule, drifting apart.
func ParseRefString(ref string) (row, col int, err error) {
	if ref == "" {
		return 0, 0, ErrInvalidRef
	}

	// Split into column letters and row number. Accept any mix of upper- and
	// lower-case letters ("Aa1", "aB3") the way Excel does, rather than
	// requiring the prefix to be uniformly one case; the prefix is upper-cased
	// below before it is decoded into a column number.
	i := 0
	for i < len(ref) && ((ref[i] >= 'A' && ref[i] <= 'Z') || (ref[i] >= 'a' && ref[i] <= 'z')) {
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, ErrInvalidRef
	}

	colStr := strings.ToUpper(ref[:i])
	rowStr := ref[i:]

	// Parse column letters to number, rejecting anything past the last column
	// as soon as it overflows the grid (which also prevents int overflow).
	col = 0
	for _, c := range colStr {
		col = col*26 + int(c-'A'+1)
		if col > MaxCol {
			return 0, 0, ErrInvalidRef
		}
	}

	// Parse row number. strconv.Atoi accepts a leading sign, so "A+5" would
	// otherwise silently address A5 and the caller would write to a cell it
	// never named (C547); require the row to be digits only.
	for i := 0; i < len(rowStr); i++ {
		if rowStr[i] < '0' || rowStr[i] > '9' {
			return 0, 0, ErrInvalidRef
		}
	}
	row, err = strconv.Atoi(rowStr)
	if err != nil || row < 1 || row > MaxRow {
		return 0, 0, ErrInvalidRef
	}

	return row, col, nil
}
