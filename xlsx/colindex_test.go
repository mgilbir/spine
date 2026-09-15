package xlsx

import (
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// colRange builds a <col> entry spanning [min,max] with an explicit width.
func colRange(min, max uint32, width float64) oxml.CT_Col {
	return oxml.CT_Col{Min: min, Max: max, Width: &width, CustomWidth: boolPtrTest(true)}
}

func boolPtrTest(b bool) *bool { return &b }

// colEntries flattens every <col> entry across all groups, in order.
func colEntries(s *Sheet) []struct{ Min, Max uint32 } {
	var out []struct{ Min, Max uint32 }
	ws := s.ws()
	if ws == nil {
		return out
	}
	for gi := range ws.Cols {
		for ei := range ws.Cols[gi].Col {
			e := ws.Cols[gi].Col[ei]
			out = append(out, struct{ Min, Max uint32 }{e.Min, e.Max})
		}
	}
	return out
}

// TestSetColWidthLaysOutColumnsWithoutRebuilding is the guard for the coverage
// set. Laying out widths column by column must not rebuild it per column: that
// rebuild is the O(cols^2) walk the set removed, and it would come back as a
// silent slowdown with every width still correct.
func TestSetColWidthLaysOutColumnsWithoutRebuilding(t *testing.T) {
	const cols = 500

	wb := Create()
	sh, err := wb.AddSheet("S")
	if err != nil {
		t.Fatal(err)
	}
	for c := 1; c <= cols; c++ {
		if err := sh.SetColWidth(c, float64(c%20)+5); err != nil {
			t.Fatal(err)
		}
	}

	// No column here is covered by an earlier entry, so the carve path — which
	// rebuilds every <cols> group and is the quadratic cost — must never run.
	if sh.colCarves != 0 {
		t.Errorf("carve path ran %d times while laying out %d fresh columns (want 0); "+
			"every carve rebuilds all <col> entries", sh.colCarves, cols)
	}
	if sh.colCovRebuilds > 2 {
		t.Errorf("coverage rebuilt %d times while setting %d column widths (want <= 2)",
			sh.colCovRebuilds, cols)
	}
	// Every column must have its own entry, and the widths must be right.
	entries := colEntries(sh)
	if len(entries) != cols {
		t.Fatalf("got %d <col> entries, want %d", len(entries), cols)
	}
	for _, probe := range []int{1, cols / 2, cols} {
		w, ok := sh.ColumnWidth(probe)
		if !ok {
			t.Errorf("column %d has no width", probe)
			continue
		}
		if want := float64(probe%20) + 5; w != want {
			t.Errorf("column %d width = %v, want %v", probe, w, want)
		}
	}
}

// TestSetColWidthCarvesCoveringRange checks the fast path does not steal a
// column that an existing spanning entry covers: that column has to be carved
// out of the range, not appended as an overlapping second entry (Excel rejects
// overlapping <col> entries, C127/C383).
func TestSetColWidthCarvesCoveringRange(t *testing.T) {
	sh := openSheetBody(t, `<cols><col min="1" max="10" width="7" hidden="1"/></cols><sheetData/>`)

	if err := sh.SetColWidth(5, 33); err != nil {
		t.Fatal(err)
	}

	entries := colEntries(sh)
	want := []struct{ Min, Max uint32 }{{1, 4}, {5, 5}, {6, 10}}
	if len(entries) != len(want) {
		t.Fatalf("got entries %v, want %v", entries, want)
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Fatalf("entry %d = %v, want %v (full: %v)", i, entries[i], want[i], entries)
		}
	}
	if w, ok := sh.ColumnWidth(5); !ok || w != 33 {
		t.Errorf("column 5 width = %v (ok=%v), want 33", w, ok)
	}
	// The carved column keeps the covering range's other properties.
	if !sh.ColumnHidden(5) {
		t.Error("column 5 lost the covering range's hidden flag")
	}
	// The remainder keeps the original width.
	if w, ok := sh.ColumnWidth(3); !ok || w != 7 {
		t.Errorf("column 3 width = %v (ok=%v), want 7", w, ok)
	}
}

// TestColCoverageSelfHealsAfterDirectAppend is the safety direction that
// matters: if the cache wrongly believed a column were uncovered it would
// append an entry overlapping an existing one. A <col> added to the model
// without going through editColumn must still be seen.
func TestColCoverageSelfHealsAfterDirectAppend(t *testing.T) {
	wb := Create()
	sh, err := wb.AddSheet("S")
	if err != nil {
		t.Fatal(err)
	}
	// Warm the cache.
	if err := sh.SetColWidth(1, 10); err != nil {
		t.Fatal(err)
	}

	// Add a spanning entry behind the cache's back.
	ws := sh.ws()
	ws.Cols[0].Col = append(ws.Cols[0].Col, colRange(4, 8, 21))

	if err := sh.SetColWidth(6, 44); err != nil {
		t.Fatal(err)
	}

	// Column 6 must have been carved out of [4,8], not appended alongside it.
	entries := colEntries(sh)
	seen := 0
	for _, e := range entries {
		if e.Min <= 6 && 6 <= e.Max {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("column 6 is covered by %d entries, want exactly 1 (entries: %v)", seen, entries)
	}
	if w, ok := sh.ColumnWidth(6); !ok || w != 44 {
		t.Errorf("column 6 width = %v (ok=%v), want 44", w, ok)
	}
	if w, ok := sh.ColumnWidth(5); !ok || w != 21 {
		t.Errorf("column 5 width = %v (ok=%v), want 21 (remainder of the range)", w, ok)
	}
}
