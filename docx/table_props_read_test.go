package docx

import (
	"bytes"
	"testing"
)

// Table.Shading and TableCell.Borders read properties the authoring API cannot
// write at the table level, so they only ever see markup a producer wrote. Both
// were unexercised, and both sit next to a same-named accessor on the other
// type (TableCell.Shading, Table.Borders) that reads a *different* element —
// the classic wrong-receiver bug, which nothing here would have noticed.
//
// The fixture below gives the table and the cell deliberately different
// shading fills and border styles, so an accessor reading the neighbouring
// element cannot match by accident.
const tablePropsBody = `<w:body><w:tbl>` +
	`<w:tblPr>` +
	`<w:tblBorders>` +
	`<w:top w:val="double" w:sz="24" w:color="AA0001"/>` +
	`<w:bottom w:val="double" w:sz="24" w:color="AA0002"/>` +
	`<w:left w:val="double" w:sz="24" w:color="AA0003"/>` +
	`<w:right w:val="double" w:sz="24" w:color="AA0004"/>` +
	`<w:insideH w:val="dotted" w:sz="8" w:color="AA0005"/>` +
	`<w:insideV w:val="dashed" w:sz="8" w:color="AA0006"/>` +
	`</w:tblBorders>` +
	`<w:shd w:val="clear" w:color="auto" w:fill="TBLFIL"/>` +
	`</w:tblPr>` +
	`<w:tblGrid><w:gridCol w:w="2000"/></w:tblGrid>` +
	`<w:tr><w:tc>` +
	`<w:tcPr>` +
	`<w:tcBorders>` +
	`<w:top w:val="single" w:sz="4" w:color="BB0001"/>` +
	`<w:bottom w:val="thick" w:sz="12" w:color="BB0002"/>` +
	`<w:left w:val="dotted" w:sz="6" w:color="BB0003"/>` +
	`<w:right w:val="dashed" w:sz="16" w:color="BB0004"/>` +
	`</w:tcBorders>` +
	`<w:shd w:val="clear" w:color="auto" w:fill="CELFIL"/>` +
	`</w:tcPr>` +
	`<w:p/></w:tc></w:tr>` +
	// A second row whose cell declares no tcPr at all, for the absent case.
	`<w:tr><w:tc><w:p/></w:tc></w:tr>` +
	`</w:tbl></w:body>`

func openTablePropsFixture(t *testing.T) *Table {
	t.Helper()
	fixture := fixtureWithDocument(t, fixtureWNS, tablePropsBody)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	tables := doc.Tables()
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
	return tables[0]
}

// TestTableShading_ReadsTheTablesOwnFill: w:shd carries both a w:val (the
// pattern) and a w:fill (the colour). Shading returns the fill, and it must be
// the *table's* fill, not the first cell's.
func TestTableShading(t *testing.T) {
	tbl := openTablePropsFixture(t)

	if got := tbl.Shading(); got != "TBLFIL" {
		t.Errorf("Table.Shading() = %q, want %q (the cell's fill is %q, the pattern is %q)",
			got, "TBLFIL", "CELFIL", "clear")
	}
	cells := tbl.Rows()[0].Cells()
	if len(cells) != 1 {
		t.Fatalf("got %d cells in the first row, want 1", len(cells))
	}
	if got := cells[0].Shading(); got != "CELFIL" {
		t.Errorf("TableCell.Shading() = %q, want %q", got, "CELFIL")
	}

	// A table with no w:shd reports "" rather than a stale or fabricated value.
	plain := Create()
	bare := plain.AddTable(1, 1)
	if got := bare.Shading(); got != "" {
		t.Errorf("Table.Shading() on a table with no w:shd = %q, want \"\"", got)
	}
	if got := bare.Rows()[0].Cells()[0].Shading(); got != "" {
		t.Errorf("TableCell.Shading() on a cell with no w:shd = %q, want \"\"", got)
	}
}

// TestTableCellBorders reads all four cell edges. Every edge in the fixture has
// a distinct style, width and colour, so an accessor that returned the wrong
// edge, or the table's w:tblBorders instead of the cell's w:tcBorders, fails.
func TestTableCellBorders(t *testing.T) {
	tbl := openTablePropsFixture(t)
	cell := tbl.Rows()[0].Cells()[0]

	got, ok := cell.Borders()
	if !ok {
		t.Fatal("TableCell.Borders() reports no w:tcBorders on a cell that declares one")
	}
	for _, c := range []struct {
		edge    string
		b       *Border
		style   string
		widthPt float64
		color   string
	}{
		// w:sz is in eighths of a point.
		{"Top", got.Top, "single", 0.5, "BB0001"},
		{"Bottom", got.Bottom, "thick", 1.5, "BB0002"},
		{"Left", got.Left, "dotted", 0.75, "BB0003"},
		{"Right", got.Right, "dashed", 2, "BB0004"},
	} {
		if c.b == nil {
			t.Errorf("Borders().%s is nil", c.edge)
			continue
		}
		if c.b.Style != c.style {
			t.Errorf("Borders().%s.Style = %q, want %q", c.edge, c.b.Style, c.style)
		}
		if c.b.Color != c.color {
			t.Errorf("Borders().%s.Color = %q, want %q", c.edge, c.b.Color, c.color)
		}
		if c.b.Width != c.widthPt {
			t.Errorf("Borders().%s.Width = %v pt, want %v pt (w:sz is eighths of a point)", c.edge, c.b.Width, c.widthPt)
		}
	}

	// The table's own borders are a different element with different values.
	tb, ok := tbl.Borders()
	if !ok {
		t.Fatal("Table.Borders() reports no w:tblBorders")
	}
	if tb.Top == nil || tb.Top.Color != "AA0001" {
		t.Errorf("Table.Borders().Top = %+v, want colour AA0001 — the table and cell borders are being confused", tb.Top)
	}
	if tb.InsideH == nil || tb.InsideH.Color != "AA0005" {
		t.Errorf("Table.Borders().InsideH = %+v, want colour AA0005", tb.InsideH)
	}

	// A cell with no w:tcPr reports absent rather than a zero-valued struct.
	bare := tbl.Rows()[1].Cells()[0]
	if _, ok := bare.Borders(); ok {
		t.Error("TableCell.Borders() reports present on a cell with no w:tcPr")
	}
}

// TestTableCellBorders_RoundTrip closes the loop through the writer: what
// SetBorders writes must be what Borders reads back.
func TestTableCellBorders_RoundTrip(t *testing.T) {
	want := CellBorders{
		Top:    &Border{Style: "single", Width: 0.5, Color: "111111"},
		Bottom: &Border{Style: "double", Width: 1.5, Color: "222222"},
		Left:   &Border{Style: "dotted", Width: 0.75, Color: "333333"},
		Right:  &Border{Style: "dashed", Width: 2, Color: "444444"},
	}
	doc := Create()
	doc.AddTable(1, 1).Rows()[0].Cells()[0].SetBorders(want)

	cell := saveAndReopen(t, doc).Tables()[0].Rows()[0].Cells()[0]
	got, ok := cell.Borders()
	if !ok {
		t.Fatal("the cell borders did not survive a save/reopen")
	}
	for _, c := range []struct {
		edge     string
		got, exp *Border
	}{
		{"Top", got.Top, want.Top},
		{"Bottom", got.Bottom, want.Bottom},
		{"Left", got.Left, want.Left},
		{"Right", got.Right, want.Right},
	} {
		if c.got == nil {
			t.Errorf("%s border is missing after the round trip", c.edge)
			continue
		}
		if *c.got != *c.exp {
			t.Errorf("%s border = %+v, want %+v", c.edge, *c.got, *c.exp)
		}
	}
}
