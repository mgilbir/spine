package xlsx

import (
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

func mergeCellRef(ref string) oxml.CT_MergeCell { return oxml.CT_MergeCell{Ref: ref} }

// TestMergeCellsDownASheetSkipsTheScan guards the linearity. Each new range
// sits below every existing one, so the bounding box rules an overlap out and
// the per-merge comparison — which re-parsed every stored reference — must
// never run.
func TestMergeCellsDownASheetSkipsTheScan(t *testing.T) {
	const merges = 500

	wb := Create()
	sh, err := wb.AddSheet("S")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= merges; i++ {
		a, err := CellRef(i, 1)
		if err != nil {
			t.Fatal(err)
		}
		b, err := CellRef(i, 2)
		if err != nil {
			t.Fatal(err)
		}
		if err := sh.MergeCells(a, b); err != nil {
			t.Fatalf("merge %d: %v", i, err)
		}
	}
	if sh.mergeScans != 0 {
		t.Errorf("overlap scan ran %d times for %d non-overlapping merges laid out down the sheet (want 0)",
			sh.mergeScans, merges)
	}
	// Rebuilding the box re-parses every stored reference, so a rebuild per
	// merge is the same quadratic cost wearing a different hat.
	if sh.mergeBoxRebuilds > 2 {
		t.Errorf("merge box rebuilt %d times across %d merges (want <= 2)",
			sh.mergeBoxRebuilds, merges)
	}
	if got := len(sh.MergedCells()); got != merges {
		t.Errorf("sheet holds %d merges, want %d", got, merges)
	}
}

// TestMergeCellsStillRejectsOverlap is the correctness half: the box must only
// ever skip the comparison when an overlap is impossible. An overlap inside the
// box, and one that exactly repeats an existing range, must both be refused.
func TestMergeCellsStillRejectsOverlap(t *testing.T) {
	wb := Create()
	sh, err := wb.AddSheet("S")
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.MergeCells("A1", "C3"); err != nil {
		t.Fatal(err)
	}
	if err := sh.MergeCells("E5", "F6"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ a, b, why string }{
		{"B2", "D4", "overlaps A1:C3"},
		{"A1", "C3", "duplicates A1:C3"},
		{"C3", "C3", "single cell inside A1:C3"},
		{"F6", "G7", "overlaps E5:F6"},
	} {
		if err := sh.MergeCells(tc.a, tc.b); err == nil {
			t.Errorf("MergeCells(%s,%s) was accepted but %s", tc.a, tc.b, tc.why)
		}
	}
	// A range inside the bounding box but overlapping nothing is still legal.
	if err := sh.MergeCells("A5", "A6"); err != nil {
		t.Errorf("MergeCells(A5,A6) rejected though it overlaps nothing: %v", err)
	}
}

// TestMergeBoundsSeesUnmergeAndDirectChange covers the staleness directions. A
// box that is too small would accept an overlapping merge, which Excel then
// refuses to open, so a merge added without maintaining the box must still be
// seen.
func TestMergeBoundsSeesUnmergeAndDirectChange(t *testing.T) {
	wb := Create()
	sh, err := wb.AddSheet("S")
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.MergeCells("A1", "B2"); err != nil {
		t.Fatal(err)
	}

	// Add a merge behind the box's back, the way a path that does not maintain
	// it would.
	ws := sh.ws()
	ws.MergeCells.MergeCell = append(ws.MergeCells.MergeCell,
		mergeCellRef("D4:E5"))

	if err := sh.MergeCells("E5", "F6"); err == nil {
		t.Error("MergeCells(E5,F6) accepted though it overlaps the D4:E5 merge added directly")
	}

	// After unmerging, the freed area must become available again.
	if err := sh.UnmergeCells("A1", "B2"); err != nil {
		t.Fatal(err)
	}
	if err := sh.MergeCells("A1", "B2"); err != nil {
		t.Errorf("re-merging an unmerged range failed: %v", err)
	}
}
