package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
)

func TestNewTable(t *testing.T) {
	table := NewTable(3, 4)
	if table == nil {
		t.Fatal("NewTable() returned nil")
	}

	if table.RowCount() != 3 {
		t.Errorf("RowCount() = %d, want 3", table.RowCount())
	}
	if table.ColCount() != 4 {
		t.Errorf("ColCount() = %d, want 4", table.ColCount())
	}

	if table.ShapeType() != ShapeTypeTable {
		t.Errorf("ShapeType() = %v, want ShapeTypeTable", table.ShapeType())
	}
}

func TestTable_Rows(t *testing.T) {
	table := NewTable(3, 2)
	rows := table.Rows()

	if len(rows) != 3 {
		t.Errorf("Rows() returned %d rows, want 3", len(rows))
	}
}

func TestTable_Row(t *testing.T) {
	table := NewTable(3, 2)

	row := table.Row(1)
	if row == nil {
		t.Fatal("Row(1) returned nil")
	}

	// Out of range
	row = table.Row(-1)
	if row != nil {
		t.Error("Row(-1) should return nil")
	}

	row = table.Row(10)
	if row != nil {
		t.Error("Row(10) should return nil")
	}
}

func TestTable_Cell(t *testing.T) {
	table := NewTable(3, 4)

	cell := table.Cell(1, 2)
	if cell == nil {
		t.Fatal("Cell(1, 2) returned nil")
	}

	// Out of range
	cell = table.Cell(-1, 0)
	if cell != nil {
		t.Error("Cell(-1, 0) should return nil")
	}

	cell = table.Cell(0, 10)
	if cell != nil {
		t.Error("Cell(0, 10) should return nil")
	}
}

func TestTable_ColWidth(t *testing.T) {
	table := NewTable(2, 3)

	// Default width
	width := table.ColWidth(0)
	if width != dml.Inches(2) {
		t.Errorf("Default ColWidth = %d, want %d", width, dml.Inches(2))
	}

	// Set width
	table.SetColWidth(1, dml.Inches(3))
	if table.ColWidth(1) != dml.Inches(3) {
		t.Errorf("After SetColWidth, ColWidth = %d, want %d", table.ColWidth(1), dml.Inches(3))
	}

	// Out of range
	if table.ColWidth(10) != 0 {
		t.Error("ColWidth(10) should return 0")
	}
}

func TestTable_AddRow(t *testing.T) {
	table := NewTable(2, 3)

	row := table.AddRow()
	if row == nil {
		t.Fatal("AddRow() returned nil")
	}

	if table.RowCount() != 3 {
		t.Errorf("After AddRow, RowCount() = %d, want 3", table.RowCount())
	}

	// New row should have correct number of cells
	if len(row.Cells()) != 3 {
		t.Errorf("New row has %d cells, want 3", len(row.Cells()))
	}
}

func TestTable_InsertRow(t *testing.T) {
	table := NewTable(2, 2)

	row := table.InsertRow(1)
	if row == nil {
		t.Fatal("InsertRow() returned nil")
	}

	if table.RowCount() != 3 {
		t.Errorf("After InsertRow, RowCount() = %d, want 3", table.RowCount())
	}
}

func TestTable_DeleteRow(t *testing.T) {
	table := NewTable(3, 2)

	table.DeleteRow(1)

	if table.RowCount() != 2 {
		t.Errorf("After DeleteRow, RowCount() = %d, want 2", table.RowCount())
	}
}

func TestTable_AddColumn(t *testing.T) {
	table := NewTable(2, 2)

	table.AddColumn(dml.Inches(1.5))

	if table.ColCount() != 3 {
		t.Errorf("After AddColumn, ColCount() = %d, want 3", table.ColCount())
	}

	// Each row should have new column
	for i := 0; i < table.RowCount(); i++ {
		if len(table.Row(i).Cells()) != 3 {
			t.Errorf("Row %d has %d cells, want 3", i, len(table.Row(i).Cells()))
		}
	}
}

func TestTable_DeleteColumn(t *testing.T) {
	table := NewTable(2, 3)

	table.DeleteColumn(1)

	if table.ColCount() != 2 {
		t.Errorf("After DeleteColumn, ColCount() = %d, want 2", table.ColCount())
	}

	// Each row should have fewer cells
	for i := 0; i < table.RowCount(); i++ {
		if len(table.Row(i).Cells()) != 2 {
			t.Errorf("Row %d has %d cells, want 2", i, len(table.Row(i).Cells()))
		}
	}
}

func TestTable_StyleOptions(t *testing.T) {
	table := NewTable(3, 3)

	// Test FirstRow
	if table.FirstRow() {
		t.Error("Default FirstRow() should be false")
	}
	table.SetFirstRow(true)
	if !table.FirstRow() {
		t.Error("After SetFirstRow(true), FirstRow() should be true")
	}

	// Test LastRow
	table.SetLastRow(true)
	if !table.LastRow() {
		t.Error("After SetLastRow(true), LastRow() should be true")
	}

	// Test FirstCol
	table.SetFirstCol(true)
	if !table.FirstCol() {
		t.Error("After SetFirstCol(true), FirstCol() should be true")
	}

	// Test LastCol
	table.SetLastCol(true)
	if !table.LastCol() {
		t.Error("After SetLastCol(true), LastCol() should be true")
	}

	// Test BandedRows (default true)
	if !table.BandedRows() {
		t.Error("Default BandedRows() should be true")
	}
	table.SetBandedRows(false)
	if table.BandedRows() {
		t.Error("After SetBandedRows(false), BandedRows() should be false")
	}

	// Test BandedCols
	if table.BandedCols() {
		t.Error("Default BandedCols() should be false")
	}
	table.SetBandedCols(true)
	if !table.BandedCols() {
		t.Error("After SetBandedCols(true), BandedCols() should be true")
	}
}

func TestTableRow_Cells(t *testing.T) {
	table := NewTable(2, 3)
	row := table.Row(0)

	cells := row.Cells()
	if len(cells) != 3 {
		t.Errorf("Cells() returned %d cells, want 3", len(cells))
	}
}

func TestTableRow_Cell(t *testing.T) {
	table := NewTable(2, 3)
	row := table.Row(0)

	cell := row.Cell(1)
	if cell == nil {
		t.Fatal("Cell(1) returned nil")
	}

	// Out of range
	cell = row.Cell(10)
	if cell != nil {
		t.Error("Cell(10) should return nil")
	}
}

func TestTableRow_Height(t *testing.T) {
	table := NewTable(2, 2)
	row := table.Row(0)

	// Default height
	if row.Height() != dml.Inches(0.5) {
		t.Errorf("Default Height() = %d, want %d", row.Height(), dml.Inches(0.5))
	}

	row.SetHeight(dml.Inches(1))
	if row.Height() != dml.Inches(1) {
		t.Errorf("After SetHeight, Height() = %d, want %d", row.Height(), dml.Inches(1))
	}
}

func TestNewTableCell(t *testing.T) {
	cell := NewTableCell()
	if cell == nil {
		t.Fatal("NewTableCell() returned nil")
	}

	if cell.TextFrame() == nil {
		t.Error("TextFrame() is nil")
	}
}

func TestTableCell_Text(t *testing.T) {
	cell := NewTableCell()
	cell.SetText("Hello")

	if cell.Text() != "Hello" {
		t.Errorf("Text() = %q, want %q", cell.Text(), "Hello")
	}
}

func TestTableCell_Fill(t *testing.T) {
	cell := NewTableCell()

	if cell.Fill() != nil {
		t.Error("Default Fill() should be nil")
	}

	cell.SetFill(dml.ColorBlue)
	if cell.Fill() == nil {
		t.Fatal("After SetFill, Fill() should not be nil")
	}

	cell.ClearFill()
	if cell.Fill() != nil {
		t.Error("After ClearFill, Fill() should be nil")
	}
}

func TestTableCell_VerticalAlign(t *testing.T) {
	cell := NewTableCell()

	if cell.VerticalAlign() != enum.VerticalAlignTop {
		t.Errorf("Default VerticalAlign() = %v, want VerticalAlignTop", cell.VerticalAlign())
	}

	cell.SetVerticalAlign(enum.VerticalAlignMiddle)
	if cell.VerticalAlign() != enum.VerticalAlignMiddle {
		t.Errorf("After SetVerticalAlign, VerticalAlign() = %v, want VerticalAlignMiddle", cell.VerticalAlign())
	}
}

func TestTableCell_Span(t *testing.T) {
	cell := NewTableCell()

	// Default spans
	if cell.RowSpan() != 1 {
		t.Errorf("Default RowSpan() = %d, want 1", cell.RowSpan())
	}
	if cell.ColSpan() != 1 {
		t.Errorf("Default ColSpan() = %d, want 1", cell.ColSpan())
	}

	cell.SetRowSpan(2)
	if cell.RowSpan() != 2 {
		t.Errorf("After SetRowSpan(2), RowSpan() = %d, want 2", cell.RowSpan())
	}

	cell.SetColSpan(3)
	if cell.ColSpan() != 3 {
		t.Errorf("After SetColSpan(3), ColSpan() = %d, want 3", cell.ColSpan())
	}

	// Test clamping
	cell.SetRowSpan(0)
	if cell.RowSpan() != 1 {
		t.Errorf("SetRowSpan(0) should clamp to 1, got %d", cell.RowSpan())
	}
}

func TestTableCell_Borders(t *testing.T) {
	cell := NewTableCell()

	border := &TableBorder{
		Width: dml.Points(1),
		Color: dml.ColorBlack,
		Style: BorderStyleSingle,
	}

	cell.SetBorderLeft(border)
	cell.SetBorderRight(border)
	cell.SetBorderTop(border)
	cell.SetBorderBottom(border)

	// Test SetBorders (sets all at once)
	newBorder := &TableBorder{
		Width: dml.Points(2),
		Color: dml.ColorRed,
		Style: BorderStyleDouble,
	}
	cell.SetBorders(newBorder)
}

func TestBorderStyle_Values(t *testing.T) {
	tests := []struct {
		style BorderStyle
		want  string
	}{
		{BorderStyleSingle, "single"},
		{BorderStyleDouble, "double"},
		{BorderStyleDashed, "dashed"},
		{BorderStyleDotted, "dotted"},
		{BorderStyleNone, "none"},
	}

	for _, tt := range tests {
		if string(tt.style) != tt.want {
			t.Errorf("BorderStyle = %q, want %q", tt.style, tt.want)
		}
	}
}
