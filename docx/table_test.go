package docx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddTable(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(3, 4)

	rows := tbl.Rows()
	if len(rows) != 3 {
		t.Fatalf("Rows() = %d, want 3", len(rows))
	}
	for i, row := range rows {
		cells := row.Cells()
		if len(cells) != 4 {
			t.Fatalf("Row %d Cells() = %d, want 4", i, len(cells))
		}
	}
}

func TestTableSetBorders(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(2, 2)
	tbl.SetBorders(TableBorders{
		Top:     &Border{Style: "single", Width: 1, Color: "000000"},
		Bottom:  &Border{Style: "single", Width: 1, Color: "000000"},
		InsideH: &Border{Style: "single", Width: 0.5, Color: "CCCCCC"},
	})

	if tbl.tbl.TblPr == nil || tbl.tbl.TblPr.TblBorders == nil {
		t.Fatal("expected TblBorders to be set")
	}
	tb := tbl.tbl.TblPr.TblBorders
	if tb.Top == nil {
		t.Fatal("expected Top border")
	}
	if tb.Top.Val != "single" {
		t.Errorf("Top.Val = %s, want single", tb.Top.Val)
	}
	// 1 point * 8 = 8 eighths of a point
	if tb.Top.Sz != "8" {
		t.Errorf("Top.Sz = %s, want 8", tb.Top.Sz)
	}
	if tb.Top.Color != "000000" {
		t.Errorf("Top.Color = %s, want 000000", tb.Top.Color)
	}
	if tb.Left != nil {
		t.Error("expected Left border to be nil")
	}
	if tb.InsideH == nil {
		t.Fatal("expected InsideH border")
	}
	// 0.5 points * 8 = 4
	if tb.InsideH.Sz != "4" {
		t.Errorf("InsideH.Sz = %s, want 4", tb.InsideH.Sz)
	}
}

func TestTableSetWidth(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	tbl.SetWidth(500)

	if tbl.tbl.TblPr.TblW == nil {
		t.Fatal("expected TblW to be set")
	}
	// 500 points * 20 = 10000 twips
	if tbl.tbl.TblPr.TblW.W != "10000" {
		t.Errorf("TblW.W = %s, want 10000", tbl.tbl.TblPr.TblW.W)
	}
	if tbl.tbl.TblPr.TblW.Type != "dxa" {
		t.Errorf("TblW.Type = %s, want dxa", tbl.tbl.TblPr.TblW.Type)
	}
}

func TestTableSetCellMargins(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	tbl.SetCellMargins(5, 10, 5, 10)

	if tbl.tbl.TblPr.TblCellMar == nil {
		t.Fatal("expected TblCellMar to be set")
	}
	cm := tbl.tbl.TblPr.TblCellMar
	// 5 points * 20 = 100 twips
	if cm.Top.W != "100" {
		t.Errorf("Top.W = %s, want 100", cm.Top.W)
	}
	// 10 points * 20 = 200 twips
	if cm.Right.W != "200" {
		t.Errorf("Right.W = %s, want 200", cm.Right.W)
	}
}

func TestRowSetHeight(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	row := tbl.Rows()[0]
	row.SetHeight(24)

	if row.tr.TrPr == nil || row.tr.TrPr.TrHeight == nil {
		t.Fatal("expected TrHeight to be set")
	}
	// 24 points * 20 = 480 twips
	if row.tr.TrPr.TrHeight.Val != "480" {
		t.Errorf("TrHeight.Val = %s, want 480", row.tr.TrPr.TrHeight.Val)
	}
	if row.tr.TrPr.TrHeight.HRule != "atLeast" {
		t.Errorf("TrHeight.HRule = %s, want atLeast", row.tr.TrPr.TrHeight.HRule)
	}
}

func TestRowSetHeaderRow(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(2, 2)
	row := tbl.Rows()[0]

	row.SetHeaderRow(true)
	if row.tr.TrPr == nil || row.tr.TrPr.TblHeader == nil {
		t.Fatal("expected TblHeader to be set")
	}

	row.SetHeaderRow(false)
	if row.tr.TrPr.TblHeader != nil {
		t.Error("expected TblHeader to be nil")
	}
}

func TestCellSetShading(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	cell := tbl.Rows()[0].Cells()[0]
	cell.SetShading("4472C4")

	if cell.tc.TcPr == nil || cell.tc.TcPr.Shd == nil {
		t.Fatal("expected Shd to be set")
	}
	if cell.tc.TcPr.Shd.Fill != "4472C4" {
		t.Errorf("Fill = %s, want 4472C4", cell.tc.TcPr.Shd.Fill)
	}
	if cell.tc.TcPr.Shd.Val != "clear" {
		t.Errorf("Val = %s, want clear", cell.tc.TcPr.Shd.Val)
	}
}

func TestCellSetVerticalAlignment(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	cell := tbl.Rows()[0].Cells()[0]
	cell.SetVerticalAlignment("center")

	if cell.tc.TcPr == nil || cell.tc.TcPr.VAlign == nil {
		t.Fatal("expected VAlign to be set")
	}
	if cell.tc.TcPr.VAlign.Val != "center" {
		t.Errorf("VAlign.Val = %s, want center", cell.tc.TcPr.VAlign.Val)
	}
}

func TestCellSetWidth(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	cell := tbl.Rows()[0].Cells()[0]
	cell.SetWidth(100)

	if cell.tc.TcPr == nil || cell.tc.TcPr.TcW == nil {
		t.Fatal("expected TcW to be set")
	}
	// 100 points * 20 = 2000 twips
	if cell.tc.TcPr.TcW.W != "2000" {
		t.Errorf("TcW.W = %s, want 2000", cell.tc.TcPr.TcW.W)
	}
}

func TestCellSetBorders(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	cell := tbl.Rows()[0].Cells()[0]
	cell.SetBorders(CellBorders{
		Bottom: &Border{Style: "thick", Width: 2, Color: "FF0000"},
	})

	if cell.tc.TcPr == nil || cell.tc.TcPr.TcBorders == nil {
		t.Fatal("expected TcBorders to be set")
	}
	cb := cell.tc.TcPr.TcBorders
	if cb.Bottom == nil {
		t.Fatal("expected Bottom border")
	}
	if cb.Bottom.Val != "thick" {
		t.Errorf("Bottom.Val = %s, want thick", cb.Bottom.Val)
	}
	// 2 points * 8 = 16 eighths of a point
	if cb.Bottom.Sz != "16" {
		t.Errorf("Bottom.Sz = %s, want 16", cb.Bottom.Sz)
	}
}

func TestCellSetGridSpan(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 3)
	cell := tbl.Rows()[0].Cells()[0]
	cell.SetGridSpan(3)

	if cell.tc.TcPr == nil || cell.tc.TcPr.GridSpan == nil {
		t.Fatal("expected GridSpan to be set")
	}
	if cell.tc.TcPr.GridSpan.Val != 3 {
		t.Errorf("GridSpan.Val = %d, want 3", cell.tc.TcPr.GridSpan.Val)
	}
}

func TestTableSaveAndReopen(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(2, 3)
	tbl.SetBorders(TableBorders{
		Top:    &Border{Style: "single", Width: 1, Color: "000000"},
		Bottom: &Border{Style: "single", Width: 1, Color: "000000"},
		Left:   &Border{Style: "single", Width: 1, Color: "000000"},
		Right:  &Border{Style: "single", Width: 1, Color: "000000"},
	})

	// Set cell content
	tbl.Rows()[0].Cells()[0].Paragraphs()[0].SetText("A1")
	tbl.Rows()[0].Cells()[1].Paragraphs()[0].SetText("B1")
	tbl.Rows()[1].Cells()[0].Paragraphs()[0].SetText("A2")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "table.docx")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Reopen and verify
	doc2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer doc2.Close() //nolint:errcheck

	tables := doc2.Tables()
	if len(tables) < 1 {
		t.Fatalf("expected at least 1 table, got %d", len(tables))
	}
}
