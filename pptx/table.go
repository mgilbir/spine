package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
)

// Table represents a table shape.
type Table struct {
	BaseShape
	rows        []*TableRow
	colWidths   []dml.EMU
	firstRow    bool // highlight first row
	lastRow     bool // highlight last row
	firstCol    bool // highlight first column
	lastCol     bool // highlight last column
	bandRow     bool // banded rows
	bandCol     bool // banded columns
}

// NewTable creates a new table with the specified number of rows and columns.
func NewTable(rows, cols int) *Table {
	t := &Table{
		rows:      make([]*TableRow, rows),
		colWidths: make([]dml.EMU, cols),
		bandRow:   true,
	}

	// Initialize rows
	for i := range t.rows {
		t.rows[i] = newTableRow(cols)
	}

	// Default column widths
	defaultWidth := dml.Inches(2)
	for i := range t.colWidths {
		t.colWidths[i] = defaultWidth
	}

	return t
}

// ShapeType returns ShapeTypeTable.
func (t *Table) ShapeType() ShapeType {
	return ShapeTypeTable
}

// Rows returns all rows in the table.
func (t *Table) Rows() []*TableRow {
	return t.rows
}

// RowCount returns the number of rows.
func (t *Table) RowCount() int {
	return len(t.rows)
}

// ColCount returns the number of columns.
func (t *Table) ColCount() int {
	return len(t.colWidths)
}

// Row returns the row at the specified index.
func (t *Table) Row(index int) *TableRow {
	if index < 0 || index >= len(t.rows) {
		return nil
	}
	return t.rows[index]
}

// Cell returns the cell at the specified row and column.
func (t *Table) Cell(row, col int) *TableCell {
	r := t.Row(row)
	if r == nil {
		return nil
	}
	return r.Cell(col)
}

// SetColWidth sets the width of a column.
func (t *Table) SetColWidth(col int, width dml.EMU) {
	if col >= 0 && col < len(t.colWidths) {
		t.colWidths[col] = width
	}
}

// ColWidth returns the width of a column.
func (t *Table) ColWidth(col int) dml.EMU {
	if col >= 0 && col < len(t.colWidths) {
		return t.colWidths[col]
	}
	return 0
}

// AddRow adds a new row to the table.
func (t *Table) AddRow() *TableRow {
	row := newTableRow(len(t.colWidths))
	t.rows = append(t.rows, row)
	return row
}

// InsertRow inserts a new row at the specified index.
func (t *Table) InsertRow(index int) *TableRow {
	row := newTableRow(len(t.colWidths))
	if index < 0 {
		index = 0
	}
	if index >= len(t.rows) {
		t.rows = append(t.rows, row)
	} else {
		t.rows = append(t.rows[:index], append([]*TableRow{row}, t.rows[index:]...)...)
	}
	return row
}

// DeleteRow removes the row at the specified index.
func (t *Table) DeleteRow(index int) {
	if index >= 0 && index < len(t.rows) {
		t.rows = append(t.rows[:index], t.rows[index+1:]...)
	}
}

// AddColumn adds a new column to the table.
func (t *Table) AddColumn(width dml.EMU) {
	t.colWidths = append(t.colWidths, width)
	for _, row := range t.rows {
		row.cells = append(row.cells, NewTableCell())
	}
}

// DeleteColumn removes the column at the specified index.
func (t *Table) DeleteColumn(index int) {
	if index >= 0 && index < len(t.colWidths) {
		t.colWidths = append(t.colWidths[:index], t.colWidths[index+1:]...)
		for _, row := range t.rows {
			if index < len(row.cells) {
				row.cells = append(row.cells[:index], row.cells[index+1:]...)
			}
		}
	}
}

// FirstRow returns whether the first row is highlighted.
func (t *Table) FirstRow() bool {
	return t.firstRow
}

// SetFirstRow sets whether the first row is highlighted.
func (t *Table) SetFirstRow(value bool) {
	t.firstRow = value
}

// LastRow returns whether the last row is highlighted.
func (t *Table) LastRow() bool {
	return t.lastRow
}

// SetLastRow sets whether the last row is highlighted.
func (t *Table) SetLastRow(value bool) {
	t.lastRow = value
}

// FirstCol returns whether the first column is highlighted.
func (t *Table) FirstCol() bool {
	return t.firstCol
}

// SetFirstCol sets whether the first column is highlighted.
func (t *Table) SetFirstCol(value bool) {
	t.firstCol = value
}

// LastCol returns whether the last column is highlighted.
func (t *Table) LastCol() bool {
	return t.lastCol
}

// SetLastCol sets whether the last column is highlighted.
func (t *Table) SetLastCol(value bool) {
	t.lastCol = value
}

// BandedRows returns whether rows are banded.
func (t *Table) BandedRows() bool {
	return t.bandRow
}

// SetBandedRows sets whether rows are banded.
func (t *Table) SetBandedRows(value bool) {
	t.bandRow = value
}

// BandedCols returns whether columns are banded.
func (t *Table) BandedCols() bool {
	return t.bandCol
}

// SetBandedCols sets whether columns are banded.
func (t *Table) SetBandedCols(value bool) {
	t.bandCol = value
}

// TableRow represents a row in a table.
type TableRow struct {
	cells  []*TableCell
	height dml.EMU
}

// newTableRow creates a new table row with the specified number of cells.
func newTableRow(cols int) *TableRow {
	row := &TableRow{
		cells:  make([]*TableCell, cols),
		height: dml.Inches(0.5),
	}
	for i := range row.cells {
		row.cells[i] = NewTableCell()
	}
	return row
}

// Cells returns all cells in the row.
func (r *TableRow) Cells() []*TableCell {
	return r.cells
}

// Cell returns the cell at the specified index.
func (r *TableRow) Cell(index int) *TableCell {
	if index < 0 || index >= len(r.cells) {
		return nil
	}
	return r.cells[index]
}

// Height returns the row height.
func (r *TableRow) Height() dml.EMU {
	return r.height
}

// SetHeight sets the row height.
func (r *TableRow) SetHeight(height dml.EMU) {
	r.height = height
}

// TableCell represents a cell in a table.
type TableCell struct {
	textFrame   *TextFrame
	fill        *dml.Color
	borderLeft  *TableBorder
	borderRight *TableBorder
	borderTop   *TableBorder
	borderBottom *TableBorder
	vertAlign   enum.VerticalAlign
	rowSpan     int
	colSpan     int
	hMerge      bool // merged with cell to the left
	vMerge      bool // merged with cell above
}

// NewTableCell creates a new table cell.
func NewTableCell() *TableCell {
	return &TableCell{
		textFrame: NewTextFrame(),
		vertAlign: enum.VerticalAlignTop,
		rowSpan:   1,
		colSpan:   1,
	}
}

// TextFrame returns the text frame for the cell.
func (c *TableCell) TextFrame() *TextFrame {
	return c.textFrame
}

// SetText sets the text content of the cell.
func (c *TableCell) SetText(text string) {
	c.textFrame.SetText(text)
}

// Text returns the text content of the cell.
func (c *TableCell) Text() string {
	return c.textFrame.Text()
}

// Fill returns the fill color of the cell.
func (c *TableCell) Fill() *dml.Color {
	return c.fill
}

// SetFill sets the fill color of the cell.
func (c *TableCell) SetFill(color dml.Color) {
	c.fill = &color
}

// ClearFill removes the fill color.
func (c *TableCell) ClearFill() {
	c.fill = nil
}

// VerticalAlign returns the vertical alignment.
func (c *TableCell) VerticalAlign() enum.VerticalAlign {
	return c.vertAlign
}

// SetVerticalAlign sets the vertical alignment.
func (c *TableCell) SetVerticalAlign(align enum.VerticalAlign) {
	c.vertAlign = align
}

// RowSpan returns the number of rows this cell spans.
func (c *TableCell) RowSpan() int {
	return c.rowSpan
}

// SetRowSpan sets the number of rows this cell spans.
func (c *TableCell) SetRowSpan(span int) {
	if span < 1 {
		span = 1
	}
	c.rowSpan = span
}

// ColSpan returns the number of columns this cell spans.
func (c *TableCell) ColSpan() int {
	return c.colSpan
}

// SetColSpan sets the number of columns this cell spans.
func (c *TableCell) SetColSpan(span int) {
	if span < 1 {
		span = 1
	}
	c.colSpan = span
}

// TableBorder represents a border on a table cell.
type TableBorder struct {
	Width dml.EMU
	Color dml.Color
	Style BorderStyle
}

// BorderStyle specifies the style of a border.
type BorderStyle string

const (
	BorderStyleSingle BorderStyle = "single"
	BorderStyleDouble BorderStyle = "double"
	BorderStyleDashed BorderStyle = "dashed"
	BorderStyleDotted BorderStyle = "dotted"
	BorderStyleNone   BorderStyle = "none"
)

// SetBorderLeft sets the left border.
func (c *TableCell) SetBorderLeft(border *TableBorder) {
	c.borderLeft = border
}

// SetBorderRight sets the right border.
func (c *TableCell) SetBorderRight(border *TableBorder) {
	c.borderRight = border
}

// SetBorderTop sets the top border.
func (c *TableCell) SetBorderTop(border *TableBorder) {
	c.borderTop = border
}

// SetBorderBottom sets the bottom border.
func (c *TableCell) SetBorderBottom(border *TableBorder) {
	c.borderBottom = border
}

// SetBorders sets all borders to the same value.
func (c *TableCell) SetBorders(border *TableBorder) {
	c.borderLeft = border
	c.borderRight = border
	c.borderTop = border
	c.borderBottom = border
}
