package xlsx

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// dupRowSheetBody is a worksheet whose sheetData carries row 3 twice, with a
// different cell in each. Wild files contain these; the API cannot create one.
const dupRowSheetBody = `<sheetData>` +
	`<row r="3"><c r="A3" t="inlineStr"><is><t>first</t></is></c></row>` +
	`<row r="3"><c r="B3" t="inlineStr"><is><t>second</t></is></c></row>` +
	`<row r="5"><c r="A5" t="inlineStr"><is><t>five</t></is></c></row>` +
	`</sheetData>`

func openSheetBody(t *testing.T, body string) *Sheet {
	t.Helper()
	data := buildXLSXWithWorksheet(t, body)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("sheet: %v", err)
	}
	return sh
}

// TestFindCellSearchesEveryDuplicateRow pins the behaviour the row index must
// not quietly change: findCell looked in every row carrying the wanted number,
// not just the first, so a reference living in the second of two duplicate rows
// is still found. Sheet.Cell, by contrast, has always stopped at the first.
func TestFindCellSearchesEveryDuplicateRow(t *testing.T) {
	sh := openSheetBody(t, dupRowSheetBody)

	if got := sh.FindCell("A3"); got == nil || got.String() != "first" {
		t.Fatalf("A3 (first duplicate row): got %v, want \"first\"", got)
	}
	// B3 exists only in the SECOND row numbered 3. An index that trusted its
	// single first-match hit would report this cell as absent.
	got := sh.FindCell("B3")
	if got == nil {
		t.Fatal("B3 lives in the second row numbered 3 and was not found")
	}
	if got.String() != "second" {
		t.Fatalf("B3: got %q, want %q", got.String(), "second")
	}
	if sh.FindCell("C3") != nil {
		t.Error("C3 does not exist but was found")
	}
}

// TestCellTakesFirstDuplicateRow pins the other half: the materializing
// accessor resolves to the first row with the number, so writing through it
// lands in the same row the pre-index linear scan chose.
func TestCellTakesFirstDuplicateRow(t *testing.T) {
	sh := openSheetBody(t, dupRowSheetBody)

	c, err := sh.Cell("C3")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	c.SetValue("written")

	ws := sh.ws()
	if n := len(ws.SheetData.Row); n != 3 {
		t.Fatalf("row count changed: got %d, want 3 (no row should have been appended)", n)
	}
	// The new cell must be in the first row numbered 3, next to A3.
	first := ws.SheetData.Row[0]
	var found bool
	for _, cell := range first.C {
		if cell.Ref() == "C3" {
			found = true
		}
	}
	if !found {
		t.Error("C3 was not created in the first row numbered 3")
	}
}

// TestRowNumberDerivedFromCellsIsIndexed covers C73: a row may legally omit its
// r attribute, in which case its number comes from its cell references. The
// index has to derive the number the same way the linear scan did, or such a
// row becomes unreachable.
//
// The companion property — that a row with no derivable number at all stays
// unaddressable — is asserted below but is not independently falsifiable
// through the public API: such a row can only ever be indexed under 0, and no
// reference parses to row 0, so indexing it anyway changes no answer.
func TestRowNumberDerivedFromCellsIsIndexed(t *testing.T) {
	sh := openSheetBody(t, `<sheetData>`+
		`<row><c r="A7" t="inlineStr"><is><t>seven</t></is></c></row>`+
		`<row><c t="inlineStr"><is><t>nowhere</t></is></c></row>`+
		`</sheetData>`)

	// Row 7 carries no r attribute but is addressable through its cell ref.
	if got := sh.FindCell("A7"); got == nil || got.String() != "seven" {
		t.Fatalf("A7: got %v, want \"seven\"", got)
	}
	// The second row has no derivable number. Addressing any row must not
	// resolve to it.
	if got := sh.FindCell("A1"); got != nil {
		t.Errorf("A1: got %v, want nil", got)
	}
}

// TestRowLookupSurvivesMarshalRowSort is the regression test for the one
// mutation the index cannot see by length or model identity: marshalSheetData
// sorts SheetData.Row in place, and prunedRows hands it the durable slice
// whenever there is nothing to prune. Cells must still resolve afterwards.
func TestRowLookupSurvivesMarshalRowSort(t *testing.T) {
	wb := Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	// Write rows in descending order so the slice order and the row numbering
	// disagree, making the marshal-time sort an actual permutation.
	for r := 5; r >= 1; r-- {
		if err := sh.SetCellValue(fmt.Sprintf("A%d", r), fmt.Sprintf("row-%d", r)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wb.SaveBytes(); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// After the in-place sort, every cell must still be reachable and hold its
	// own value — a stale index would return a neighbouring row's cell.
	for r := 1; r <= 5; r++ {
		ref := fmt.Sprintf("A%d", r)
		got := sh.FindCell(ref)
		if got == nil {
			t.Fatalf("%s missing after save", ref)
		}
		if want := fmt.Sprintf("row-%d", r); got.String() != want {
			t.Errorf("%s: got %q, want %q", ref, got.String(), want)
		}
	}

	// Writing after the sort must also land in the right row.
	if err := sh.SetCellValue("B3", "added-after-sort"); err != nil {
		t.Fatal(err)
	}
	if got := sh.FindCell("B3"); got == nil || got.String() != "added-after-sort" {
		t.Fatalf("B3 after sort: got %v", got)
	}
	if got := sh.FindCell("A3"); got == nil || got.String() != "row-3" {
		t.Fatalf("A3 after post-sort write: got %v", got)
	}
}

// TestRowIndexSelfHealsAfterDirectAppend checks the length guard: a row added
// to the model without going through appendRow must still be found, at the cost
// of a rebuild rather than a wrong answer.
func TestRowIndexSelfHealsAfterDirectAppend(t *testing.T) {
	wb := Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.SetCellValue("A1", "one"); err != nil {
		t.Fatal(err)
	}
	if sh.rowIdx == nil {
		t.Fatal("index should be warm after a write")
	}

	// Append behind the index's back, the way a future call site that forgets
	// to maintain it would.
	ws := sh.ws()
	r := uint32(9)
	nine := "nine"
	ws.SheetData.Row = append(ws.SheetData.Row, oxml.CT_Row{
		R: &r,
		C: []*oxml.CT_Cell{cellAt("A9", func(c *oxml.CT_Cell) { c.T = "inlineStr"; c.Is = &oxml.CT_Rst{T: &nine} })},
	})

	if got := sh.FindCell("A9"); got == nil || got.String() != "nine" {
		t.Fatalf("A9 after an unmaintained append: got %v, want \"nine\"", got)
	}
	if got := sh.FindCell("A1"); got == nil || got.String() != "one" {
		t.Fatalf("A1 after an unmaintained append: got %v", got)
	}
}

// TestAppendOnlyBuildKeepsIndexWarm is the guard for #314 itself. Filling a
// sheet row by row must not rebuild the index per row: that is precisely the
// per-row O(rows) walk the index removed, and it would come back as a silent
// slowdown with every value still correct. Asserted as a count rather than a
// duration so it cannot go flaky.
func TestAppendOnlyBuildKeepsIndexWarm(t *testing.T) {
	const rows, cols = 500, 5

	wb := Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	for r := 1; r <= rows; r++ {
		for c := 1; c <= cols; c++ {
			ref, err := CellRef(r, c)
			if err != nil {
				t.Fatal(err)
			}
			if err := sh.SetCellValue(ref, r*c); err != nil {
				t.Fatal(err)
			}
		}
	}

	// One rebuild is expected: the first lookup on the empty sheet. Anything
	// proportional to the row count means maintenance is broken.
	if sh.rowIdxRebuilds > 2 {
		t.Errorf("index rebuilt %d times while appending %d rows; "+
			"append-only writes must keep it warm (want <= 2)", sh.rowIdxRebuilds, rows)
	}
	if got := len(sh.rowIdx.byNumber); got != rows {
		t.Errorf("index holds %d rows, want %d", got, rows)
	}
	// The data must actually be right, not merely cheap to write.
	for _, probe := range []struct{ r, c int }{{1, 1}, {rows / 2, 3}, {rows, cols}} {
		ref, err := CellRef(probe.r, probe.c)
		if err != nil {
			t.Fatal(err)
		}
		cell := sh.FindCell(ref)
		if cell == nil {
			t.Fatalf("%s missing", ref)
		}
		if want := float64(probe.r * probe.c); cell.Float() != want {
			t.Errorf("%s: got %v, want %v", ref, cell.Float(), want)
		}
	}
}

func BenchmarkSheetCellAppendRows(b *testing.B) {
	for _, rows := range []int{1000, 4000, 16000} {
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				wb := Create()
				sh, err := wb.AddSheet("Data")
				if err != nil {
					b.Fatal(err)
				}
				for r := 1; r <= rows; r++ {
					for c := 1; c <= 10; c++ {
						ref, err := CellRef(r, c)
						if err != nil {
							b.Fatal(err)
						}
						if err := sh.SetCellValue(ref, r); err != nil {
							b.Fatal(err)
						}
					}
				}
			}
		})
	}
}

// TestFillingWideRowKeepsOneCursor is the column-side counterpart of
// TestAppendOnlyBuildKeepsIndexWarm. Filling a row must build one column map,
// not one per cell: rebuilding per cell is the O(cols) walk that made a wide
// row quadratic, and it would return every correct value while doing so.
func TestFillingWideRowKeepsOneCursor(t *testing.T) {
	const cols = 800

	wb := Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	for c := 1; c <= cols; c++ {
		ref, err := CellRef(1, c)
		if err != nil {
			t.Fatal(err)
		}
		if err := sh.SetCellValue(ref, c); err != nil {
			t.Fatal(err)
		}
	}

	if sh.cellCursorRebuilds > 2 {
		t.Errorf("cell cursor rebuilt %d times while filling one row of %d cells (want <= 2)",
			sh.cellCursorRebuilds, cols)
	}
	for _, probe := range []int{1, cols / 2, cols} {
		ref, err := CellRef(1, probe)
		if err != nil {
			t.Fatal(err)
		}
		cell := sh.FindCell(ref)
		if cell == nil {
			t.Fatalf("%s missing", ref)
		}
		if cell.Float() != float64(probe) {
			t.Errorf("%s = %v, want %v", ref, cell.Float(), probe)
		}
	}
}

// TestCellCursorSeesCellsAppendedElsewhere guards the direction that corrupts:
// a cached cursor that had not seen a cell another cursor appended would append
// a second <c> for the same reference, and Excel rejects a duplicate cell.
func TestCellCursorSeesCellsAppendedElsewhere(t *testing.T) {
	wb := Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	// Warm the sheet's cached cursor on row 1.
	if err := sh.SetCellValue("A1", "a"); err != nil {
		t.Fatal(err)
	}

	// Append B1 through a different cursor, the way the table and chart paths do.
	other := sh.newRowCells(1)
	c, err := other.cell(2)
	if err != nil {
		t.Fatal(err)
	}
	c.SetValue("b")

	// The cached cursor must notice and reuse that cell rather than add another.
	got, err := sh.Cell("B1")
	if err != nil {
		t.Fatal(err)
	}
	got.SetValue("b2")

	ws := sh.ws()
	i, ok := sh.lookupRow(ws, 1)
	if !ok {
		t.Fatal("row 1 missing")
	}
	var b1 int
	for _, cell := range ws.SheetData.Row[i].C {
		if cell.Ref() == "B1" {
			b1++
		}
	}
	if b1 != 1 {
		t.Fatalf("row 1 holds %d cells named B1, want 1", b1)
	}
	if v := sh.FindCell("B1"); v == nil || v.String() != "b2" {
		t.Fatalf("B1 = %v, want \"b2\"", v)
	}
}

// TestCellCursorSurvivesMarshalRowSort exercises the cached cursor across the
// in-place row sort that marshalling performs. The rows are written in
// descending order so the sort genuinely permutes them, and the cell written
// afterwards goes to the SAME row the cursor already holds — the one case where
// a stale cursor is reused rather than replaced. A cursor still pointing at its
// old position would append the cell to whichever row now sits there.
func TestCellCursorSurvivesMarshalRowSort(t *testing.T) {
	wb := Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	for r := 5; r >= 1; r-- {
		if err := sh.SetCellValue(fmt.Sprintf("A%d", r), fmt.Sprintf("row-%d", r)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wb.SaveBytes(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Row 1 is the row the cursor was left on, and it moved from last to first.
	if err := sh.SetCellValue("B1", "after-sort"); err != nil {
		t.Fatal(err)
	}

	ws := sh.ws()
	for r := 1; r <= 5; r++ {
		i, ok := sh.lookupRow(ws, uint32(r))
		if !ok {
			t.Fatalf("row %d missing", r)
		}
		for _, c := range ws.SheetData.Row[i].C {
			wantRow := fmt.Sprintf("%d", r)
			if len(c.Ref()) < 2 || c.Ref()[1:] != wantRow {
				t.Errorf("row %d holds cell %q, which belongs to another row", r, c.Ref())
			}
		}
	}
	if got := sh.FindCell("B1"); got == nil || got.String() != "after-sort" {
		t.Fatalf("B1 = %v, want \"after-sort\"", got)
	}
	if got := sh.FindCell("A5"); got == nil || got.String() != "row-5" {
		t.Fatalf("A5 = %v, want \"row-5\"", got)
	}
}
