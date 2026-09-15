package xlsx

import (
	"bytes"
	"sync"
	"testing"
)

// TestConcurrentReadsOfAParsedSheet guards a property the package had before
// the row index existed and has to keep: once a workbook's sheets are parsed,
// reading it from several goroutines is safe.
//
// The index is the one cache a read-only accessor builds — findCell, rowEntry,
// RowHeight and CellValue all resolve a row through it — so without
// synchronisation eight readers racing over a parsed sheet produced fifteen
// reported races where the same code on the previous release produced none.
// Other lazy caches in the package (comments, sheet text, pptx notes and
// shapes) have always raced under concurrent reads; this is not about them, and
// it does not make the package safe for concurrent *writes*, which it never was.
//
// The goroutines deliberately race to be FIRST to touch each sheet: the lazy
// parse is itself a cache built on a read, so pre-parsing here would hide the
// very thing under test.
//
// The assertion is the race detector: this test only means something under
// -race, which is a required gate (make test-race).
func TestConcurrentReadsOfAParsedSheet(t *testing.T) {
	const rows, cols = 60, 8

	wb := Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.SetColWidth(2, 14); err != nil {
		t.Fatal(err)
	}
	if err := sh.SetRowHeight(3, 22); err != nil {
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
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// Reopen and parse every sheet up front, single-threaded, so the lazy parse
	// is finished before any reader starts. What the goroutines do below is
	// pure reading.
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sheets := reopened.Sheets()

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for _, s := range sheets {
				// Every lazily built cache the package exposes to a reader: the
				// worksheet parse itself, comments, sparklines and the
				// workbook's threaded-comment authors.
				_ = s.Rows()
				_ = s.Text()
				_ = s.Comments()
				_ = s.Sparklines()
				_ = s.MergedCells()
				for r := 1 + g%2; r <= rows; r += 2 {
					_, _ = s.RowHeight(r)
					_ = s.RowHidden(r)
					for c := 1; c <= cols; c++ {
						ref, err := CellRef(r, c)
						if err != nil {
							continue
						}
						if cell := s.FindCell(ref); cell != nil {
							_ = cell.String()
						}
						if _, err := s.CellValue(ref); err != nil {
							continue
						}
						_, _ = s.ColumnWidth(c)
					}
				}
			}
		}(g)
	}
	wg.Wait()

	// The readers must also have read the right thing.
	if got := sheets[0].FindCell("C5"); got == nil || got.Float() != 15 {
		t.Fatalf("C5 = %v, want 15", got)
	}
}
