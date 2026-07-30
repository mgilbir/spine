package docx

import (
	"strconv"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// --- Vertical cell merge (w:vMerge) ---

// VerticalMerge names a table cell's vertical-merge role (w:vMerge). A merged
// column is expressed as a "restart" cell at the top followed by "continue"
// cells that fold their content upward into it.
type VerticalMerge string

const (
	// VerticalMergeRestart begins a vertically merged region (w:vMerge="restart").
	VerticalMergeRestart VerticalMerge = "restart"
	// VerticalMergeContinue continues the region begun by the cell above
	// (a bare <w:vMerge/>).
	VerticalMergeContinue VerticalMerge = "continue"
)

// VerticalMerge returns the cell's vertical-merge role (w:vMerge), or "" when
// the cell is not part of a vertical merge.
func (tc *TableCell) VerticalMerge() VerticalMerge {
	if tc.tc.TcPr == nil || tc.tc.TcPr.VMerge == nil {
		return ""
	}
	if tc.tc.TcPr.VMerge.Val == "restart" {
		return VerticalMergeRestart
	}
	// A bare <w:vMerge/> (or an explicit w:val="continue") continues the merge.
	return VerticalMergeContinue
}

// SetVerticalMerge sets the cell's vertical-merge role (w:vMerge).
// VerticalMergeRestart marks the top cell of a merged column; VerticalMergeContinue
// marks a cell that merges upward into it and is emitted as a bare <w:vMerge/>,
// the form Word writes for a continued cell.
func (tc *TableCell) SetVerticalMerge(m VerticalMerge) {
	tc.ensureTcPr()
	if m == VerticalMergeRestart {
		tc.tc.TcPr.VMerge = &oxml.CT_String{Val: "restart"}
		return
	}
	tc.tc.TcPr.VMerge = &oxml.CT_String{}
}

// ClearVerticalMerge removes the cell's w:vMerge element.
func (tc *TableCell) ClearVerticalMerge() {
	tc.touch()
	if tc.tc.TcPr != nil {
		tc.tc.TcPr.VMerge = nil
	}
}

// --- Table look (w:tblLook) ---

// TableLook selects which conditional-formatting parts of the table's style are
// applied (w:tblLook): the special first/last row and column formatting and
// whether row/column banding is suppressed.
type TableLook struct {
	FirstRow, LastRow       bool
	FirstColumn, LastColumn bool
	// NoHBand / NoVBand suppress horizontal / vertical banding.
	NoHBand, NoVBand bool
}

// tblLook bit masks (ECMA-376 §17.4.56, w:tblLook@w:val).
const (
	tblLookFirstRow    = 0x0020
	tblLookLastRow     = 0x0040
	tblLookFirstColumn = 0x0080
	tblLookLastColumn  = 0x0100
	tblLookNoHBand     = 0x0200
	tblLookNoVBand     = 0x0400
)

// tblLookBit resolves one flag from the explicit boolean attribute when set,
// falling back to the corresponding bit of the w:val hex bitmask (older files
// carry only w:val).
func tblLookBit(attr, val string, bit int) bool {
	if attr != "" {
		return isOnOffTrue(attr)
	}
	if n, err := strconv.ParseInt(val, 16, 32); err == nil {
		return n&int64(bit) != 0
	}
	return false
}

func bit01(on bool) string {
	if on {
		return "1"
	}
	return "0"
}

// TableLook returns the table's conditional-formatting selection (w:tblLook) and
// whether the element is present.
func (t *Table) TableLook() (TableLook, bool) {
	if t.tbl.TblPr == nil || t.tbl.TblPr.TblLook == nil {
		return TableLook{}, false
	}
	tl := t.tbl.TblPr.TblLook
	return TableLook{
		FirstRow:    tblLookBit(tl.FirstRow, tl.Val, tblLookFirstRow),
		LastRow:     tblLookBit(tl.LastRow, tl.Val, tblLookLastRow),
		FirstColumn: tblLookBit(tl.FirstColumn, tl.Val, tblLookFirstColumn),
		LastColumn:  tblLookBit(tl.LastColumn, tl.Val, tblLookLastColumn),
		NoHBand:     tblLookBit(tl.NoHBand, tl.Val, tblLookNoHBand),
		NoVBand:     tblLookBit(tl.NoVBand, tl.Val, tblLookNoVBand),
	}, true
}

// SetTableLook sets the table's conditional-formatting selection (w:tblLook),
// writing both the explicit boolean attributes and the equivalent w:val
// bitmask that Word emits.
func (t *Table) SetTableLook(look TableLook) {
	t.ensureTblPr()
	val := 0
	for _, f := range []struct {
		on  bool
		bit int
	}{
		{look.FirstRow, tblLookFirstRow},
		{look.LastRow, tblLookLastRow},
		{look.FirstColumn, tblLookFirstColumn},
		{look.LastColumn, tblLookLastColumn},
		{look.NoHBand, tblLookNoHBand},
		{look.NoVBand, tblLookNoVBand},
	} {
		if f.on {
			val |= f.bit
		}
	}
	t.tbl.TblPr.TblLook = &oxml.CT_TblLook{
		Val:         hex4(val),
		FirstRow:    bit01(look.FirstRow),
		LastRow:     bit01(look.LastRow),
		FirstColumn: bit01(look.FirstColumn),
		LastColumn:  bit01(look.LastColumn),
		NoHBand:     bit01(look.NoHBand),
		NoVBand:     bit01(look.NoVBand),
	}
}

// hex4 renders v as an upper-case 4-digit hex string (the w:tblLook@w:val form
// Word writes, e.g. "04A0").
func hex4(v int) string {
	s := strconv.FormatInt(int64(v), 16)
	for len(s) < 4 {
		s = "0" + s
	}
	return strings.ToUpper(s)
}

// --- Table layout (w:tblLayout) ---

// TableLayout names a table's layout algorithm (w:tblLayout@w:type).
type TableLayout string

const (
	// TableLayoutFixed uses the grid's column widths verbatim.
	TableLayoutFixed TableLayout = "fixed"
	// TableLayoutAutofit sizes columns to their content.
	TableLayoutAutofit TableLayout = "autofit"
)

// Layout returns the table's layout algorithm (w:tblLayout), or "" when unset.
func (t *Table) Layout() TableLayout {
	if t.tbl.TblPr == nil || t.tbl.TblPr.TblLayout == nil {
		return ""
	}
	return TableLayout(t.tbl.TblPr.TblLayout.Type)
}

// SetLayout sets the table's layout algorithm (w:tblLayout). Passing "" removes
// the element.
func (t *Table) SetLayout(layout TableLayout) {
	if layout == "" {
		if t.tbl.TblPr != nil {
			t.tbl.TblPr.TblLayout = nil
		}
		return
	}
	t.ensureTblPr()
	t.tbl.TblPr.TblLayout = &oxml.CT_TblLayout{Type: string(layout)}
}

// --- Table indent (w:tblInd) ---

// Indent returns the table's indentation from the leading margin in points
// (w:tblInd) and whether the element is present.
func (t *Table) Indent() (float64, bool) {
	if t.tbl.TblPr == nil || t.tbl.TblPr.TblInd == nil {
		return 0, false
	}
	return twipsToPoints(t.tbl.TblPr.TblInd.W), true
}

// SetIndent sets the table's indentation from the leading margin in points
// (w:tblInd).
func (t *Table) SetIndent(points float64) {
	t.ensureTblPr()
	t.tbl.TblPr.TblInd = twipWidth(points)
}

// --- Table alignment (w:jc) ---

// Alignment returns the table's horizontal alignment within its column
// (w:tblPr/w:jc). Only left, center, and right apply to tables; anything else
// reports AlignmentLeft.
func (t *Table) Alignment() Alignment {
	if t.tbl.TblPr != nil && t.tbl.TblPr.Jc != nil {
		switch t.tbl.TblPr.Jc.Val {
		case "center":
			return AlignmentCenter
		case "right", "end":
			return AlignmentRight
		}
	}
	return AlignmentLeft
}

// SetAlignment sets the table's horizontal alignment (w:tblPr/w:jc). Tables
// support left, center, and right; AlignmentJustify is treated as left.
func (t *Table) SetAlignment(align Alignment) {
	t.ensureTblPr()
	val := "left"
	switch align {
	case AlignmentCenter:
		val = "center"
	case AlignmentRight:
		val = "right"
	}
	t.tbl.TblPr.Jc = &oxml.CT_Jc{Val: val}
}

// --- Getters for existing table/cell setters ---

// Borders returns the table's borders (w:tblBorders) and whether the element is
// present.
func (t *Table) Borders() (TableBorders, bool) {
	if t.tbl.TblPr == nil || t.tbl.TblPr.TblBorders == nil {
		return TableBorders{}, false
	}
	b := t.tbl.TblPr.TblBorders
	return TableBorders{
		Top:     oxmlToBorder(b.Top),
		Bottom:  oxmlToBorder(b.Bottom),
		Left:    oxmlToBorder(b.Left),
		Right:   oxmlToBorder(b.Right),
		InsideH: oxmlToBorder(b.InsideH),
		InsideV: oxmlToBorder(b.InsideV),
	}, true
}

// Width returns the table's width in points (w:tblW, type "dxa") and whether the
// element is present. Non-dxa widths (percent/auto) report their raw value
// divided by 20.
func (t *Table) Width() (float64, bool) {
	if t.tbl.TblPr == nil || t.tbl.TblPr.TblW == nil {
		return 0, false
	}
	return twipsToPoints(t.tbl.TblPr.TblW.W), true
}

// Shading returns the table's background fill color (w:tblPr/w:shd@w:fill), or
// "" when unset.
func (t *Table) Shading() string {
	if t.tbl.TblPr == nil || t.tbl.TblPr.Shd == nil {
		return ""
	}
	return t.tbl.TblPr.Shd.Fill
}

// Borders returns the cell's borders (w:tcBorders) and whether the element is
// present.
func (tc *TableCell) Borders() (CellBorders, bool) {
	if tc.tc.TcPr == nil || tc.tc.TcPr.TcBorders == nil {
		return CellBorders{}, false
	}
	b := tc.tc.TcPr.TcBorders
	return CellBorders{
		Top:    oxmlToBorder(b.Top),
		Bottom: oxmlToBorder(b.Bottom),
		Left:   oxmlToBorder(b.Left),
		Right:  oxmlToBorder(b.Right),
	}, true
}

// Width returns the cell's width in points (w:tcW, type "dxa") and whether the
// element is present.
func (tc *TableCell) Width() (float64, bool) {
	if tc.tc.TcPr == nil || tc.tc.TcPr.TcW == nil {
		return 0, false
	}
	return twipsToPoints(tc.tc.TcPr.TcW.W), true
}

// Shading returns the cell's background fill color (w:tcPr/w:shd@w:fill), or ""
// when unset.
func (tc *TableCell) Shading() string {
	if tc.tc.TcPr == nil || tc.tc.TcPr.Shd == nil {
		return ""
	}
	return tc.tc.TcPr.Shd.Fill
}

// VerticalAlignment returns the cell's vertical content alignment
// (w:tcPr/w:vAlign): "top", "center", or "bottom", or "" when unset.
func (tc *TableCell) VerticalAlignment() string {
	if tc.tc.TcPr == nil || tc.tc.TcPr.VAlign == nil {
		return ""
	}
	return tc.tc.TcPr.VAlign.Val
}

// GridSpan returns the number of grid columns the cell spans (w:gridSpan);
// 1 when unset.
func (tc *TableCell) GridSpan() int {
	if tc.tc.TcPr == nil || tc.tc.TcPr.GridSpan == nil {
		return 1
	}
	return tc.tc.TcPr.GridSpan.Val
}
