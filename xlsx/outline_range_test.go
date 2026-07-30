package xlsx

import (
	"reflect"
	"testing"
	"time"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// colsFixtures are pre-existing <cols> shapes the range carve has to handle:
// no entries, one wide entry, entries straddling the range on each side, several
// groups covering the same columns, and unsorted/overlapping entries.
func colsFixtures() map[string][]oxml.CT_Cols {
	w := func(v float64) *float64 { return &v }
	lvl := func(v uint8) *uint8 { return &v }
	yes := func() *bool { b := true; return &b }
	return map[string][]oxml.CT_Cols{
		"empty":     nil,
		"one group": {{}},
		"one wide entry": {{Col: []oxml.CT_Col{
			{Min: 1, Max: 20, Width: w(12), CustomWidth: yes()},
		}}},
		"straddles both ends": {{Col: []oxml.CT_Col{
			{Min: 2, Max: 9, Width: w(7)},
		}}},
		"exact fit": {{Col: []oxml.CT_Col{
			{Min: 3, Max: 6, OutlineLevel: lvl(2)},
		}}},
		"disjoint neighbours": {{Col: []oxml.CT_Col{
			{Min: 1, Max: 2, Width: w(3)},
			{Min: 9, Max: 12, Width: w(4)},
		}}},
		"already single columns": {{Col: []oxml.CT_Col{
			{Min: 3, Max: 3, OutlineLevel: lvl(1)},
			{Min: 4, Max: 4, OutlineLevel: lvl(7)},
		}}},
		"two groups overlapping": {
			{Col: []oxml.CT_Col{{Min: 2, Max: 8, Width: w(5)}}},
			{Col: []oxml.CT_Col{{Min: 4, Max: 10, Width: w(6)}}},
		},
		"unsorted entries": {{Col: []oxml.CT_Col{
			{Min: 7, Max: 9, Width: w(2)},
			{Min: 1, Max: 3, Width: w(8)},
			{Min: 5, Max: 5, Hidden: yes()},
		}}},
	}
}

func cloneCols(in []oxml.CT_Cols) []oxml.CT_Cols {
	if in == nil {
		return nil
	}
	out := make([]oxml.CT_Cols, len(in))
	for i := range in {
		out[i] = in[i]
		if in[i].Col != nil {
			out[i].Col = append([]oxml.CT_Col(nil), in[i].Col...)
		}
	}
	return out
}

// TestEditColumnRangeMatchesPerColumn is the correctness guard for the linear
// column carve: for every <cols> shape and every range, editColumnRange must
// produce exactly what calling editColumn once per column produced. The
// per-column loop was O(cols^2) — it rebuilt every group on each call — so the
// range form replaced it, and this pins the two together.
func TestEditColumnRangeMatchesPerColumn(t *testing.T) {
	ranges := [][2]int{{1, 1}, {1, 5}, {3, 6}, {4, 12}, {2, 20}, {6, 8}, {11, 14}}
	// A representative mutation: bump the outline level, exactly what
	// GroupColumns does.
	bump := func(c *oxml.CT_Col) {
		level := outlineLevelOf(c.OutlineLevel)
		if level < maxOutlineLevel {
			level++
		}
		setOutlineLevel(&c.OutlineLevel, level)
	}
	for name, fixture := range colsFixtures() {
		for _, r := range ranges {
			start, end := r[0], r[1]
			t.Run(name, func(t *testing.T) {
				wantSheet := addSheetT(Create(), "S")
				wantSheet.ensureWorksheet()
				wantSheet.ws().Cols = cloneCols(fixture)
				for col := start; col <= end; col++ {
					if err := wantSheet.editColumn(col, bump); err != nil {
						t.Fatalf("editColumn(%d): %v", col, err)
					}
				}

				gotSheet := addSheetT(Create(), "S")
				gotSheet.ensureWorksheet()
				gotSheet.ws().Cols = cloneCols(fixture)
				if err := gotSheet.editColumnRange(start, end, bump); err != nil {
					t.Fatalf("editColumnRange(%d,%d): %v", start, end, err)
				}

				if !reflect.DeepEqual(gotSheet.ws().Cols, wantSheet.ws().Cols) {
					t.Fatalf("range carve of [%d,%d] over %q differs\n got: %s\nwant: %s",
						start, end, name, formatCols(gotSheet.ws().Cols), formatCols(wantSheet.ws().Cols))
				}
			})
		}
	}
}

// formatCols renders <cols> groups compactly for failure messages.
func formatCols(groups []oxml.CT_Cols) string {
	out := "["
	for gi := range groups {
		if gi > 0 {
			out += " | "
		}
		for i, c := range groups[gi].Col {
			if i > 0 {
				out += " "
			}
			out += FormatCellRef(1, int(c.Min)) + ".." + FormatCellRef(1, int(c.Max))
			if c.OutlineLevel != nil {
				out += "@" + string(rune('0'+*c.OutlineLevel))
			}
			if c.Width != nil {
				out += "w"
			}
			if c.Hidden != nil && *c.Hidden {
				out += "h"
			}
		}
	}
	return out + "]"
}

// TestEditRowRangeMatchesPerRow is the row-side counterpart: the one-pass row
// editor must match editRow called per row, including on a sheet whose rows are
// unsorted, sparse, duplicated or missing their r attribute.
func TestEditRowRangeMatchesPerRow(t *testing.T) {
	rowNo := func(n uint32) *uint32 { return &n }
	fixtures := map[string][]oxml.CT_Row{
		"empty":  nil,
		"sparse": {{R: rowNo(2)}, {R: rowNo(7)}},
		"unsorted": {
			{R: rowNo(5)}, {R: rowNo(1)}, {R: rowNo(3)},
		},
		"duplicate row numbers": {{R: rowNo(3)}, {R: rowNo(3)}},
		"derived from cells": {
			{C: []*oxml.CT_Cell{{R: "A4"}}}, // no r attribute
			{R: rowNo(6)},
		},
	}
	bump := func(r *oxml.CT_Row) {
		level := outlineLevelOf(r.OutlineLevel)
		if level < maxOutlineLevel {
			level++
		}
		setOutlineLevel(&r.OutlineLevel, level)
	}
	cloneRows := func(in []oxml.CT_Row) []oxml.CT_Row {
		if in == nil {
			return nil
		}
		out := make([]oxml.CT_Row, len(in))
		for i := range in {
			out[i] = in[i]
			if in[i].R != nil {
				out[i].R = rowNo(*in[i].R)
			}
			if in[i].C != nil {
				out[i].C = make([]*oxml.CT_Cell, len(in[i].C))
				for j, c := range in[i].C {
					cc := *c
					out[i].C[j] = &cc
				}
			}
		}
		return out
	}
	for name, fixture := range fixtures {
		for _, r := range [][2]int{{1, 1}, {1, 4}, {2, 8}, {3, 3}, {5, 9}} {
			start, end := r[0], r[1]
			t.Run(name, func(t *testing.T) {
				want := addSheetT(Create(), "S")
				want.ensureWorksheet()
				want.ws().SheetData.Row = cloneRows(fixture)
				for row := start; row <= end; row++ {
					if err := want.editRow(row, bump); err != nil {
						t.Fatalf("editRow(%d): %v", row, err)
					}
				}

				got := addSheetT(Create(), "S")
				got.ensureWorksheet()
				got.ws().SheetData.Row = cloneRows(fixture)
				if err := got.editRowRange(start, end, bump); err != nil {
					t.Fatalf("editRowRange(%d,%d): %v", start, end, err)
				}

				gotRows, wantRows := got.ws().SheetData.Row, want.ws().SheetData.Row
				if len(gotRows) != len(wantRows) {
					t.Fatalf("range edit of [%d,%d] over %q produced %d rows, want %d",
						start, end, name, len(gotRows), len(wantRows))
				}
				for i := range gotRows {
					gn, _ := rowNumberOf(&gotRows[i])
					wn, _ := rowNumberOf(&wantRows[i])
					if gn != wn || outlineLevelOf(gotRows[i].OutlineLevel) != outlineLevelOf(wantRows[i].OutlineLevel) {
						t.Fatalf("row %d of [%d,%d] over %q: got r=%d level=%d, want r=%d level=%d",
							i, start, end, name, gn, outlineLevelOf(gotRows[i].OutlineLevel),
							wn, outlineLevelOf(wantRows[i].OutlineLevel))
					}
				}
			})
		}
	}
}

// TestGroupRowsScalesLinearly and TestGroupColumnsScalesLinearly guard the same
// quadratic shape AddTable had, on the row- and column-definition substrate:
// GroupRows re-scanned SheetData.Row per row and GroupColumns rebuilt every
// <cols> group per column, so grouping a tall or wide range cost O(n^2)
// (~225ms for 8000 rows, ~960ms for 8000 columns before the fix).
func TestGroupRowsScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}
	assertLinearInSpan(t, "GroupRows", 4000, 64000, func(n int) error {
		return addSheetT(Create(), "S").GroupRows(1, n)
	})
}

// TestGroupRowsIsFast and TestGroupColumnsIsFast are the guards that actually
// decide whether the quadratic came back. The per-item ratio checks above are a
// coarse smoke test: they measure wall clock on whatever machine happens to run
// them, and the column span is capped at 8x by MaxCol, so the gap between "mild
// cache effects" and "quadratic" is only 8x wide. On a shared CI runner that gap
// closes — GroupColumns measured 5.4x there against a 2.3-3.2x local range.
// An absolute bound has no such problem: the pre-fix cost was ~225ms for 8000
// rows and ~960ms for 8000 columns, against ~2ms and ~1ms now, so a bound two
// orders of magnitude above the fixed cost still catches a regression outright
// while being immune to scheduler noise.
func TestGroupRowsIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}
	assertFasterThan(t, "GroupRows(1, 64000)", 400*time.Millisecond, func() error {
		return addSheetT(Create(), "S").GroupRows(1, 64000)
	})
}

func TestGroupColumnsIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}
	assertFasterThan(t, "GroupColumns(1, 16000)", 200*time.Millisecond, func() error {
		return addSheetT(Create(), "S").GroupColumns(1, 16000)
	})
}

// assertFasterThan runs op three times and asserts the best run beats limit.
// Taking the best discards scheduler noise without hiding a real regression:
// quadratic work is slower than the bound on every run, not just the unlucky
// ones.
func assertFasterThan(t *testing.T, what string, limit time.Duration, op func() error) {
	t.Helper()
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 3; i++ {
		start := time.Now()
		if err := op(); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if d := time.Since(start); d < best {
			best = d
		}
	}
	t.Logf("%s: %v (limit %v)", what, best, limit)
	if best > limit {
		t.Errorf("%s took %v, want <= %v: the linear-time path has regressed", what, best, limit)
	}
}

func TestGroupColumnsScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}
	// MaxCol is 16384, so the widest honest span is a 8x step from 2000.
	assertLinearInSpan(t, "GroupColumns", 2000, 16000, func(n int) error {
		return addSheetT(Create(), "S").GroupColumns(1, n)
	})
}

// assertLinearInSpan runs an operation over a small and a large span and
// compares the per-item cost rather than the total. Linear work keeps the
// per-item cost flat whatever the span; quadratic work multiplies it by the span
// ratio. Comparing per-item costs makes the check independent of machine speed
// and of the fixed overhead that dominates the small case, which a total-time
// ratio is very sensitive to.
func assertLinearInSpan(t *testing.T, what string, small, large int, run func(int) error) {
	t.Helper()
	best := func(n int) time.Duration {
		out := time.Duration(1<<63 - 1)
		for i := 0; i < 3; i++ {
			start := time.Now()
			if err := run(n); err != nil {
				t.Fatalf("%s(1,%d): %v", what, n, err)
			}
			if d := time.Since(start); d < out {
				out = d
			}
		}
		return out
	}
	if err := run(small); err != nil { // warm up
		t.Fatalf("%s warm-up: %v", what, err)
	}
	ds, dl := best(small), best(large)
	perSmall := float64(ds) / float64(small)
	perLarge := float64(dl) / float64(large)
	t.Logf("%s: %d items in %v (%.0f ns/item); %d items in %v (%.0f ns/item); %dx the span",
		what, small, ds, perSmall, large, dl, perLarge, large/small)
	if perSmall <= 0 {
		return
	}
	// Quadratic work multiplies the per-item cost by the span ratio (8x for
	// columns, 16x for rows). 4x looked like ample headroom on a quiet machine
	// but measured 5.4x on a shared CI runner against a 2.3-3.2x local range,
	// so it failed on noise rather than on a regression. 6x keeps this below
	// the 8x quadratic signature while surviving a loaded runner; the absolute
	// bounds in TestGroup*IsFast are what decisively catch the regression.
	if ratio := perLarge / perSmall; ratio > 6 {
		t.Errorf("%s scales super-linearly: per-item cost grew %.1fx over a %dx wider span (%.0f -> %.0f ns/item); want <= 6x",
			what, ratio, large/small, perSmall, perLarge)
	}
}
