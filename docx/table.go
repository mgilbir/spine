package docx

import (
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Table represents a table in a Word document.
type Table struct {
	tbl *oxml.CT_Tbl
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
	t.tbl.Tr = append(t.tbl.Tr, tr)
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

func (t *Table) ensureTblPr() {
	if t.tbl.TblPr == nil {
		t.tbl.TblPr = &oxml.CT_TblPr{}
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
	tc := &oxml.CT_Tc{
		P: []*oxml.CT_P{{}},
	}
	tr.tr.Tc = append(tr.tr.Tc, tc)
	return &TableCell{row: tr, tc: tc}
}

// TableCell represents a cell in a table.
type TableCell struct {
	row *TableRow
	tc  *oxml.CT_Tc
}

// Paragraphs returns all paragraphs in the cell.
func (tc *TableCell) Paragraphs() []*Paragraph {
	result := make([]*Paragraph, len(tc.tc.P))
	for i, p := range tc.tc.P {
		result[i] = &Paragraph{p: p}
	}
	return result
}

// AddParagraph adds a new paragraph to the cell.
func (tc *TableCell) AddParagraph() *Paragraph {
	p := &oxml.CT_P{}
	tc.tc.P = append(tc.tc.P, p)
	return &Paragraph{p: p}
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
