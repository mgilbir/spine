package pptx

import (
	"bytes"
	"testing"
)

// C310: SetColSpan/SetRowSpan must produce a valid merged grid — every grid
// cell the span covers has to carry an hMerge/vMerge continuation flag, so the
// row still emits one a:tc per grid column. A bare SetColSpan(2) previously
// emitted gridSpan="2" plus an ordinary neighbor, giving the row one more grid
// column than the table has.
func TestTable_SetColSpanEmitsMergeCell(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	tbl := slide.AddTable(1, 2)
	tbl.Cell(0, 0).SetColSpan(2)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	xml := zipPart(t, data, "ppt/slides/slide1.xml")
	if !bytes.Contains(xml, []byte(`gridSpan="2"`)) {
		t.Errorf("slide XML missing gridSpan=\"2\" on the master cell")
	}
	if !bytes.Contains(xml, []byte(`hMerge="1"`)) {
		t.Errorf("slide XML missing hMerge=\"1\" continuation cell (invalid grid: row has more columns than the table)")
	}
	if n := bytes.Count(xml, []byte("</a:tc>")); n != 2 {
		t.Errorf("emitted %d a:tc cells, want 2 (one master with gridSpan, one covered with hMerge)", n)
	}
}

// The rowSpan setter must likewise mark the cell below as a vMerge continuation.
func TestTable_SetRowSpanEmitsMergeCell(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	tbl := slide.AddTable(2, 1)
	tbl.Cell(0, 0).SetRowSpan(2)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	xml := zipPart(t, data, "ppt/slides/slide1.xml")
	if !bytes.Contains(xml, []byte(`rowSpan="2"`)) {
		t.Errorf("slide XML missing rowSpan=\"2\" on the master cell")
	}
	if !bytes.Contains(xml, []byte(`vMerge="1"`)) {
		t.Errorf("slide XML missing vMerge=\"1\" continuation cell (invalid grid)")
	}
}

// The merge must also survive on a table loaded from a file (the in-place
// patch path), where SetColSpan is applied to a materialized cell.
func TestTable_SetColSpanOnLoadedTable(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	slide.AddTable(1, 2)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	loaded := openBytes(t, data)
	var tbl *Table
	for _, sh := range loaded.Slides()[0].Shapes() {
		if tt, ok := sh.(*Table); ok {
			tbl = tt
			break
		}
	}
	if tbl == nil {
		t.Fatal("no table on reopened slide")
	}
	tbl.Cell(0, 0).SetColSpan(2)

	data2, err := loaded.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := zipPart(t, data2, "ppt/slides/slide1.xml")
	if !bytes.Contains(xml, []byte(`gridSpan="2"`)) || !bytes.Contains(xml, []byte(`hMerge="1"`)) {
		t.Errorf("loaded table: missing gridSpan/hMerge after SetColSpan (invalid grid)")
	}
}

// An ordinary cell on a loaded table must report span 1, matching a cell
// created via NewTableCell. The parsed a:tc omits rowSpan/gridSpan for ordinary
// cells, so materialization must normalize the absent 0 to 1.
func TestTable_LoadedOrdinaryCellReportsSpanOne(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	slide.AddTable(2, 2)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	loaded := openBytes(t, data)
	var tbl *Table
	for _, sh := range loaded.Slides()[0].Shapes() {
		if tt, ok := sh.(*Table); ok {
			tbl = tt
			break
		}
	}
	if tbl == nil {
		t.Fatal("no table on reopened slide")
	}

	cell := tbl.Cell(0, 0)
	if got := cell.RowSpan(); got != 1 {
		t.Errorf("loaded ordinary cell RowSpan() = %d, want 1", got)
	}
	if got := cell.ColSpan(); got != 1 {
		t.Errorf("loaded ordinary cell ColSpan() = %d, want 1", got)
	}
}
