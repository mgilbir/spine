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
		if cell.R == "C3" {
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
		C: []*oxml.CT_Cell{{R: "A9", T: "inlineStr", Is: &oxml.CT_Rst{T: &nine}}},
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
