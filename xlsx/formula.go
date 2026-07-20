package xlsx

import (
	"strconv"
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// materializeSharedGroup converts every follower of the shared-formula group
// anchored at master into a plain-formula cell. Overwriting or replacing a
// shared-formula master with the group bookkeeping left in place would strand
// followers as `<f t="shared" si="N"/>` stubs with no master anywhere —
// spec-invalid per ECMA-376 §18.3.1.40 and an Excel repair prompt (C176).
//
// Followers that carry their own cached formula text keep it; followers with
// an empty stub get the master's formula translated by their (row, col)
// offset from the master, which is exactly the value Excel would have
// displayed for them.
func (s *Sheet) materializeSharedGroup(master *oxml.CT_Cell) {
	f := master.F
	if f == nil || f.T != "shared" || f.Ref == "" || f.Si == nil {
		return
	}
	if s == nil || s.worksheet == nil {
		return
	}

	mRow, mCol, mErr := ParseCellRef(master.R)
	for i := range s.worksheet.SheetData.Row {
		for _, cell := range s.worksheet.SheetData.Row[i].C {
			if cell == master {
				continue
			}
			cf := cell.F
			if cf == nil || cf.T != "shared" || cf.Si == nil || *cf.Si != *f.Si {
				continue
			}
			value := cf.Value
			if value == "" {
				// Empty stub: derive the follower's formula from the master's.
				value = f.Value
				if mErr == nil {
					if row, col, err := ParseCellRef(cell.R); err == nil {
						value = translateFormula(f.Value, row-mRow, col-mCol)
					}
				}
			}
			cell.F = &oxml.CT_CellFormula{Value: value}
		}
	}
}

// nextSharedFormulaSi returns a shared-formula index not currently used by any
// cell on the sheet: one past the highest existing si. Shared indices only need
// to be unique within a worksheet, so a fresh maximum is always safe.
func (s *Sheet) nextSharedFormulaSi() uint32 {
	var next uint32
	if s == nil || s.worksheet == nil {
		return 0
	}
	seen := false
	for i := range s.worksheet.SheetData.Row {
		for _, cell := range s.worksheet.SheetData.Row[i].C {
			if cell == nil || cell.F == nil || cell.F.Si == nil {
				continue
			}
			if !seen || *cell.F.Si >= next {
				next = *cell.F.Si + 1
				seen = true
			}
		}
	}
	return next
}

// translateFormula rewrites the relative A1-style cell references in formula,
// shifting them by dRow rows and dCol columns. Absolute markers anchor an
// axis: $A$1 never moves, $A1 shifts only its row, A$1 only its column. Text
// inside double-quoted string literals and quoted sheet names ('Sheet 1'!A1)
// is left untouched, as are function calls (LOG10(...)), unquoted sheet names
// (Sheet1!A1 shifts only the reference) and defined names. A reference that
// would be shifted off the worksheet grid becomes #REF!, matching Excel.
func translateFormula(formula string, dRow, dCol int) string {
	if dRow == 0 && dCol == 0 {
		return formula
	}
	var b strings.Builder
	b.Grow(len(formula) + 8)
	n := len(formula)
	for i := 0; i < n; {
		switch c := formula[i]; {
		case c == '"' || c == '\'':
			// String literal ("" escapes a quote) or quoted sheet name
			// ('' escapes an apostrophe): copy verbatim.
			j := i + 1
			for j < n {
				if formula[j] == c {
					if j+1 < n && formula[j+1] == c {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			b.WriteString(formula[i:j])
			i = j
		case c == '$' || isFormulaAlpha(c) || c == '_':
			// Consume a maximal identifier-like token so that ref-shaped
			// substrings of longer names (MyName1, LOG10) are not shifted.
			j := i
			for j < n && isFormulaTokenChar(formula[j]) {
				j++
			}
			tok := formula[i:j]
			if j < n && (formula[j] == '(' || formula[j] == '!') {
				// Function call or unquoted sheet name: not a cell reference.
				b.WriteString(tok)
			} else {
				b.WriteString(translateRefToken(tok, dRow, dCol))
			}
			i = j
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// isFormulaAlpha reports whether c is an ASCII letter.
func isFormulaAlpha(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// isFormulaTokenChar reports whether c can appear in an identifier-like
// formula token (cell reference, defined name, or function name).
func isFormulaTokenChar(c byte) bool {
	return isFormulaAlpha(c) || (c >= '0' && c <= '9') || c == '$' || c == '_' || c == '.'
}

// translateRefToken shifts tok by (dRow, dCol) if it is a valid A1-style cell
// reference; any other token (defined name, TRUE, XFE1, ...) is returned
// unchanged. A shifted reference that leaves the worksheet grid becomes #REF!.
func translateRefToken(tok string, dRow, dCol int) string {
	i := 0
	colAbs := i < len(tok) && tok[i] == '$'
	if colAbs {
		i++
	}
	colStart := i
	for i < len(tok) && isFormulaAlpha(tok[i]) {
		i++
	}
	colLetters := tok[colStart:i]
	if len(colLetters) == 0 || len(colLetters) > 3 {
		return tok
	}
	rowAbs := i < len(tok) && tok[i] == '$'
	if rowAbs {
		i++
	}
	rowDigits := tok[i:]
	// Row must be all digits with no leading zero (A01 is not a reference).
	if rowDigits == "" || rowDigits[0] == '0' {
		return tok
	}
	row, err := strconv.Atoi(rowDigits)
	if err != nil || row < 1 || row > MaxRow {
		return tok
	}
	col := 0
	for k := 0; k < len(colLetters); k++ {
		c := colLetters[k]
		if c >= 'a' {
			c -= 'a' - 'A'
		}
		col = col*26 + int(c-'A'+1)
		if col > MaxCol {
			return tok
		}
	}

	if !rowAbs {
		row += dRow
	}
	if !colAbs {
		col += dCol
	}
	if row < 1 || row > MaxRow || col < 1 || col > MaxCol {
		return "#REF!"
	}

	var b strings.Builder
	if colAbs {
		b.WriteByte('$')
	}
	b.WriteString(columnLetters(col))
	if rowAbs {
		b.WriteByte('$')
	}
	b.WriteString(strconv.Itoa(row))
	return b.String()
}
