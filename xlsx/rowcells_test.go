package xlsx

import (
	"fmt"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// assertNoDuplicateCells checks the invariant a row cursor must never break: no
// row may hold two cells with the same reference. A cursor caches a row's cells
// by column, so a stale cursor — one used after something else created a cell in
// the same row — would append a second <c> for a reference that already exists.
func assertNoDuplicateCells(t *testing.T, s *Sheet, what string) {
	t.Helper()
	if s.ws() == nil {
		return
	}
	for i := range s.ws().SheetData.Row {
		row := &s.ws().SheetData.Row[i]
		seen := make(map[string]bool, len(row.C))
		for _, c := range row.C {
			if c == nil {
				continue
			}
			if seen[c.R] {
				rn, _ := rowNumberOf(row)
				t.Fatalf("%s: row %d holds two cells with reference %q", what, rn, c.R)
			}
			seen[c.R] = true
		}
	}
}

// TestRowCursorNoDuplicateCells runs every path that was converted to a row
// cursor and checks the invariant after each.
func TestRowCursorNoDuplicateCells(t *testing.T) {
	w := Create()
	s := addSheetT(w, "S")
	for r := 1; r <= 6; r++ {
		for c := 1; c <= 6; c++ {
			if err := s.SetCellValue(FormatCellRef(r, c), "v"); err != nil {
				t.Fatal(err)
			}
		}
	}

	if _, err := s.AddTable("A1:D5", TableOptions{
		Name:         "SalesTable",
		TotalsRow:    true,
		ColumnTotals: map[string]TotalsColumn{"v": {Function: "sum", Label: "Total"}},
	}); err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateCells(t, s, "AddTable with totals")

	// A second table over a partly overlapping range: its header cursor must see
	// the cells the first table's write-back created.
	if _, err := s.AddTable("C1:F4", TableOptions{Name: "OtherTable"}); err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateCells(t, s, "overlapping AddTable")

	cell, err := s.Cell("H1")
	if err != nil {
		t.Fatal(err)
	}
	if err := cell.SetSharedFormula("A1+1", "H1:K4"); err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateCells(t, s, "SetSharedFormula")

	// Refilling an overlapping shared-formula range must reuse the followers the
	// first fill created rather than append duplicates beside them.
	cell2, err := s.Cell("J3")
	if err != nil {
		t.Fatal(err)
	}
	if err := cell2.SetSharedFormula("A1+2", "J3:M6"); err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateCells(t, s, "overlapping SetSharedFormula")

	src := Create()
	ss := addSheetT(src, "Src")
	for r := 1; r <= 8; r++ {
		for c := 1; c <= 8; c++ {
			if err := ss.SetCellValue(FormatCellRef(r, c), r*10+c); err != nil {
				t.Fatal(err)
			}
		}
	}
	dst, err := w.CopySheetFrom(src, "Src")
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateCells(t, dst, "CopySheetFrom")

	// The copy must carry every source cell, not just the ones a cursor happened
	// to index.
	for r := 1; r <= 8; r++ {
		for c := 1; c <= 8; c++ {
			ref := FormatCellRef(r, c)
			got, err := dst.CellValue(ref)
			if err != nil {
				t.Fatal(err)
			}
			if want := fmt.Sprint(r*10 + c); got != want {
				t.Fatalf("CopySheetFrom: %s = %q, want %q", ref, got, want)
			}
		}
	}
}

// TestRowCursorMatchesSheetCell pins the cursor to Sheet.Cell on the awkward
// rows: duplicate references, a cell whose r attribute names a different row,
// a row with no r attribute, and a row that does not exist yet.
func TestRowCursorMatchesSheetCell(t *testing.T) {
	rowNo := func(n uint32) *uint32 { return &n }
	build := func() *Sheet {
		s := addSheetT(Create(), "S")
		s.ensureWorksheet()
		s.ws().SheetData.Row = []oxml.CT_Row{
			// Duplicate reference: the first must win, as the linear scan did.
			{R: rowNo(1), C: []*oxml.CT_Cell{{R: "A1", T: "inlineStr", Is: &oxml.CT_Rst{T: strPtr("first")}}, {R: "A1", T: "inlineStr", Is: &oxml.CT_Rst{T: strPtr("second")}}}},
			// A stray cell naming another row stays unaddressable through row 2.
			{R: rowNo(2), C: []*oxml.CT_Cell{{R: "B2"}, {R: "C9"}}},
			// No r attribute: the row number is derived from its first cell.
			{C: []*oxml.CT_Cell{{R: "D5"}}},
		}
		return s
	}
	type probe struct{ row, col int }
	probes := []probe{
		{1, 1}, {1, 2}, // existing duplicate, then a new cell in the same row
		{2, 2}, {2, 3}, // existing, then the column the stray C9 occupies
		{5, 4}, {5, 1}, // the r-less row
		{9, 3}, // the row the stray cell names but does not constitute
		{7, 7}, // a row that does not exist at all
	}

	for _, p := range probes {
		viaCell := build()
		wantCell, err := viaCell.Cell(FormatCellRef(p.row, p.col))
		if err != nil {
			t.Fatalf("Sheet.Cell(%d,%d): %v", p.row, p.col, err)
		}

		viaCursor := build()
		gotCell, err := viaCursor.newRowCells(p.row).cell(p.col)
		if err != nil {
			t.Fatalf("cursor cell(%d,%d): %v", p.row, p.col, err)
		}

		if gotCell.cell.R != wantCell.cell.R || gotCell.String() != wantCell.String() {
			t.Errorf("cell(%d,%d): cursor gave %q=%q, Sheet.Cell gave %q=%q",
				p.row, p.col, gotCell.cell.R, gotCell.String(), wantCell.cell.R, wantCell.String())
		}
		// The resulting model must match too: same rows, same cells per row.
		gotRows, wantRows := viaCursor.ws().SheetData.Row, viaCell.ws().SheetData.Row
		if len(gotRows) != len(wantRows) {
			t.Fatalf("cell(%d,%d): cursor left %d rows, Sheet.Cell left %d", p.row, p.col, len(gotRows), len(wantRows))
		}
		for i := range gotRows {
			if len(gotRows[i].C) != len(wantRows[i].C) {
				t.Fatalf("cell(%d,%d): row %d has %d cells via cursor, %d via Sheet.Cell",
					p.row, p.col, i, len(gotRows[i].C), len(wantRows[i].C))
			}
			for j := range gotRows[i].C {
				if gotRows[i].C[j].R != wantRows[i].C[j].R {
					t.Fatalf("cell(%d,%d): row %d cell %d is %q via cursor, %q via Sheet.Cell",
						p.row, p.col, i, j, gotRows[i].C[j].R, wantRows[i].C[j].R)
				}
			}
		}
	}
}

// TestRowCursorFindDoesNotCreate keeps the read-only half read-only: probing a
// range with find must not spawn phantom rows or cells (C425).
func TestRowCursorFindDoesNotCreate(t *testing.T) {
	s := addSheetT(Create(), "S")
	if err := s.SetCellValue("B2", "x"); err != nil {
		t.Fatal(err)
	}
	before := len(s.ws().SheetData.Row)
	for row := 1; row <= 20; row++ {
		cursor := s.newRowCells(row)
		for col := 1; col <= 20; col++ {
			if c := cursor.find(col); c != nil && (row != 2 || col != 2) {
				t.Fatalf("find(%d,%d) returned a cell that was never set", row, col)
			}
			if got := cursor.value(col); got != "" && (row != 2 || col != 2) {
				t.Fatalf("value(%d,%d) = %q, want empty", row, col, got)
			}
		}
	}
	if after := len(s.ws().SheetData.Row); after != before {
		t.Errorf("read-only probing created rows: %d -> %d", before, after)
	}
	if got, _ := s.CellValue("B2"); got != "x" {
		t.Errorf("B2 = %q, want %q", got, "x")
	}
}

func strPtr(s string) *string { return &s }
