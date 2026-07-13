package docx

import (
	"fmt"
	"math"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Border represents a border definition for tables and cells.
type Border struct {
	Style string  // "single", "double", "dotted", "dashed", "thick", "none"
	Width float64 // points (mapped to eighths-of-a-point internally)
	Color string  // hex color (e.g., "000000")
}

// TableBorders defines borders for a table.
type TableBorders struct {
	Top, Bottom, Left, Right *Border
	InsideH, InsideV         *Border
}

// CellBorders defines borders for a table cell.
type CellBorders struct {
	Top, Bottom, Left, Right *Border
}

// Table represents a table in a Word document.
type Table struct {
	// document is the owning document, propagated to rows, cells, and cell
	// paragraphs so document-scoped operations (e.g. adding an image to a run
	// in a table cell) can register parts and relationships.
	document *Document
	tbl      *oxml.CT_Tbl
}

// Rows returns all rows in the table.
func (t *Table) Rows() []*TableRow {
	rows := make([]*TableRow, len(t.tbl.Tr))
	for i, tr := range t.tbl.Tr {
		rows[i] = &TableRow{table: t, tr: tr}
	}
	return rows
}

// AddRow adds a new row to the table.
func (t *Table) AddRow() *TableRow {
	tr := &oxml.CT_Tr{}
	t.tbl.AppendRow(tr)
	return &TableRow{table: t, tr: tr}
}

// Style returns the table style name.
func (t *Table) Style() string {
	if t.tbl.TblPr != nil && t.tbl.TblPr.TblStyle != nil {
		return t.tbl.TblPr.TblStyle.Val
	}
	return ""
}

// SetStyle sets the table style.
func (t *Table) SetStyle(style string) {
	t.ensureTblPr()
	t.tbl.TblPr.TblStyle = &oxml.CT_String{Val: style}
}

// SetBorders sets the borders on the table.
func (t *Table) SetBorders(b TableBorders) {
	t.ensureTblPr()
	tb := &oxml.CT_TblBorders{}
	tb.Top = borderToOxml(b.Top)
	tb.Bottom = borderToOxml(b.Bottom)
	tb.Left = borderToOxml(b.Left)
	tb.Right = borderToOxml(b.Right)
	tb.InsideH = borderToOxml(b.InsideH)
	tb.InsideV = borderToOxml(b.InsideV)
	t.tbl.TblPr.TblBorders = tb
}

// SetWidth sets the table width in points.
func (t *Table) SetWidth(points float64) {
	t.ensureTblPr()
	twips := int(math.Round(points * 20))
	t.tbl.TblPr.TblW = &oxml.CT_TblWidth{
		W:    fmt.Sprintf("%d", twips),
		Type: "dxa",
	}
}

// SetCellMargins sets the default cell margins for the table in points.
func (t *Table) SetCellMargins(top, right, bottom, left float64) {
	t.ensureTblPr()
	t.tbl.TblPr.TblCellMar = &oxml.CT_TblCellMar{
		Top:    twipWidth(top),
		Right:  twipWidth(right),
		Bottom: twipWidth(bottom),
		Left:   twipWidth(left),
	}
}

func twipWidth(points float64) *oxml.CT_TblWidth {
	return &oxml.CT_TblWidth{
		W:    fmt.Sprintf("%d", int(math.Round(points*20))),
		Type: "dxa",
	}
}

func (t *Table) ensureTblPr() {
	if t.tbl.TblPr == nil {
		t.tbl.TblPr = &oxml.CT_TblPr{}
	}
}

func borderToOxml(b *Border) *oxml.CT_Border {
	if b == nil {
		return nil
	}
	// Border size is in eighths of a point
	sz := int(math.Round(b.Width * 8))
	return &oxml.CT_Border{
		Val:   b.Style,
		Sz:    fmt.Sprintf("%d", sz),
		Color: b.Color,
		Space: "0",
	}
}

// TableRow represents a row in a table.
type TableRow struct {
	table *Table
	tr    *oxml.CT_Tr
}

// Cells returns all cells in the row.
func (tr *TableRow) Cells() []*TableCell {
	cells := make([]*TableCell, len(tr.tr.Tc))
	for i, tc := range tr.tr.Tc {
		cells[i] = &TableCell{row: tr, tc: tc}
	}
	return cells
}

// AddCell adds a new cell to the row.
func (tr *TableRow) AddCell() *TableCell {
	tc := &oxml.CT_Tc{}
	tc.AppendP(&oxml.CT_P{})
	tr.tr.AppendCell(tc)
	return &TableCell{row: tr, tc: tc}
}

// SetHeight sets the row height in points.
func (tr *TableRow) SetHeight(points float64) {
	tr.ensureTrPr()
	twips := int(math.Round(points * 20))
	tr.tr.TrPr.TrHeight = &oxml.CT_Height{
		Val:   fmt.Sprintf("%d", twips),
		HRule: "atLeast",
	}
}

// SetHeaderRow marks this row as a header row that repeats on page breaks.
func (tr *TableRow) SetHeaderRow(header bool) {
	tr.ensureTrPr()
	if header {
		tr.tr.TrPr.TblHeader = &oxml.CT_OnOff{}
	} else {
		tr.tr.TrPr.TblHeader = nil
	}
}

func (tr *TableRow) ensureTrPr() {
	if tr.tr.TrPr == nil {
		tr.tr.TrPr = &oxml.CT_TrPr{}
	}
}

// TableCell represents a cell in a table.
type TableCell struct {
	row *TableRow
	tc  *oxml.CT_Tc
}

// document returns the owning document (nil for a detached table).
func (tc *TableCell) document() *Document {
	if tc.row == nil || tc.row.table == nil {
		return nil
	}
	return tc.row.table.document
}

// Paragraphs returns all paragraphs in the cell.
func (tc *TableCell) Paragraphs() []*Paragraph {
	result := make([]*Paragraph, len(tc.tc.P))
	for i, p := range tc.tc.P {
		result[i] = &Paragraph{document: tc.document(), p: p}
	}
	return result
}

// AddParagraph adds a new paragraph to the cell. The paragraph carries the
// document backref, so runs created in it can add images end-to-end.
func (tc *TableCell) AddParagraph() *Paragraph {
	p := &oxml.CT_P{}
	tc.tc.AppendP(p)
	return &Paragraph{document: tc.document(), p: p}
}

// Text returns the text content of the cell.
func (tc *TableCell) Text() string {
	text := ""
	for _, p := range tc.Paragraphs() {
		if text != "" {
			text += "\n"
		}
		text += p.Text()
	}
	return text
}

// SetShading sets the background color of the cell.
func (tc *TableCell) SetShading(hexColor string) {
	tc.ensureTcPr()
	tc.tc.TcPr.Shd = &oxml.CT_Shd{
		Val:  "clear",
		Fill: hexColor,
	}
}

// SetVerticalAlignment sets the vertical alignment of the cell content.
// Valid values: "top", "center", "bottom".
func (tc *TableCell) SetVerticalAlignment(align string) {
	tc.ensureTcPr()
	tc.tc.TcPr.VAlign = &oxml.CT_String{Val: align}
}

// SetWidth sets the cell width in points.
func (tc *TableCell) SetWidth(points float64) {
	tc.ensureTcPr()
	twips := int(math.Round(points * 20))
	tc.tc.TcPr.TcW = &oxml.CT_TblWidth{
		W:    fmt.Sprintf("%d", twips),
		Type: "dxa",
	}
}

// SetBorders sets the borders on the cell.
func (tc *TableCell) SetBorders(b CellBorders) {
	tc.ensureTcPr()
	cb := &oxml.CT_TcBorders{}
	cb.Top = borderToOxml(b.Top)
	cb.Bottom = borderToOxml(b.Bottom)
	cb.Left = borderToOxml(b.Left)
	cb.Right = borderToOxml(b.Right)
	tc.tc.TcPr.TcBorders = cb
}

// SetGridSpan sets horizontal cell merging (column span).
func (tc *TableCell) SetGridSpan(span int) {
	tc.ensureTcPr()
	tc.tc.TcPr.GridSpan = &oxml.CT_DecimalNumber{Val: span}
}

func (tc *TableCell) ensureTcPr() {
	if tc.tc.TcPr == nil {
		tc.tc.TcPr = &oxml.CT_TcPr{}
	}
}
