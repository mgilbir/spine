package xlsx

import (
	"fmt"
	"testing"
	"time"
)

// buildTableGridSheet returns a sheet holding a small seeded block of cells,
// matching what FuzzXlsxAddTable sets up before it calls AddTable.
func buildTableGridSheet(tb testing.TB) *Sheet {
	tb.Helper()
	w := Create()
	s, err := w.AddSheet("Sheet1")
	if err != nil {
		tb.Fatalf("AddSheet: %v", err)
	}
	for r := 1; r <= 4; r++ {
		for c := 1; c <= 4; c++ {
			if err := s.SetCellValue(FormatCellRef(r, c), "v"); err != nil {
				tb.Fatalf("SetCellValue: %v", err)
			}
		}
	}
	return s
}

// addTableCols times a single AddTable spanning nCols columns.
func addTableCols(tb testing.TB, nCols int) time.Duration {
	tb.Helper()
	s := buildTableGridSheet(tb)
	rng := "A1:" + FormatCellRef(8, nCols)
	start := time.Now()
	if _, err := s.AddTable(rng, TableOptions{Name: "T"}); err != nil {
		tb.Fatalf("AddTable(%s): %v", rng, err)
	}
	return time.Since(start)
}

// BenchmarkAddTableFullGrid measures AddTable over Excel's full column span,
// the shape FuzzXlsxAddTable drove into a quadratic header write-back.
func BenchmarkAddTableFullGrid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := buildTableGridSheet(b)
		if _, err := s.AddTable("A1:XFD1048576", TableOptions{Name: "T"}); err != nil {
			b.Fatalf("AddTable: %v", err)
		}
	}
}

// BenchmarkAddTableFullGridTotals measures the same span with a totals row, so
// the totals write-back loop is covered too.
func BenchmarkAddTableFullGridTotals(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := buildTableGridSheet(b)
		_, err := s.AddTable("A1:XFD1048576", TableOptions{
			Name:         "T",
			TotalsRow:    true,
			ColumnTotals: map[string]TotalsColumn{"v": {Function: "sum", Label: "Total"}},
		})
		if err != nil {
			b.Fatalf("AddTable: %v", err)
		}
	}
}

// TestAddTableScalesLinearly guards the fix for the quadratic header/totals
// write-back that FuzzXlsxAddTable found: AddTable used to walk the header row
// once per column and linear-scan the (growing) row on every step, so a
// full-width table cost O(cols^2) string comparisons — ~850ms for one call,
// enough garbage churn to get a fuzz worker OOM-killed.
//
// The assertion is on the per-column cost, not on absolute time, so it does not
// depend on how fast the machine is: widening the range must not make each
// column more expensive. Quadratic work multiplies the per-column cost by the
// span ratio (16x here); the bound is 4x.
func TestAddTableScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}
	assertLinearInSpan(t, "AddTable", 1024, 16384, func(n int) error {
		s := buildTableGridSheet(t)
		_, err := s.AddTable("A1:"+FormatCellRef(8, n), TableOptions{Name: "T"})
		return err
	})
}

// TestAddTableFullGridIsFast is a coarse absolute-time backstop for the same
// regression. The pre-fix implementation took ~850ms for this call; anything in
// that region means the per-column linear scan is back.
func TestAddTableFullGridIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 3; i++ {
		if d := addTableCols(t, 16384); d < best {
			best = d
		}
	}
	t.Logf("AddTable over the full 16384-column span: %v", best)
	if best > 200*time.Millisecond {
		t.Errorf("AddTable over the full column span took %v; want well under 200ms", best)
	}
}

// TestAddTableFullGridHeaders checks that the linear header write-back produces
// exactly the same cells the per-column scan did: every header cell carries its
// resolved column name, and the names stay unique.
func TestAddTableFullGridHeaders(t *testing.T) {
	s := buildTableGridSheet(t)
	tbl, err := s.AddTable("A1:XFD8", TableOptions{Name: "T"})
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	cols := tbl.Columns()
	if len(cols) != 16384 {
		t.Fatalf("Columns() = %d, want 16384", len(cols))
	}
	seen := make(map[string]int, len(cols))
	for i, c := range cols {
		if prev, dup := seen[c.Name]; dup {
			t.Fatalf("duplicate column name %q at %d and %d", c.Name, prev, i)
		}
		seen[c.Name] = i
		ref := FormatCellRef(1, i+1)
		got, err := s.CellValue(ref)
		if err != nil {
			t.Fatalf("CellValue(%s): %v", ref, err)
		}
		if got != c.Name {
			t.Fatalf("header %s = %q, want %q", ref, got, c.Name)
		}
	}
	// The seeded "v" headers must dedup exactly as before: v, v2, v3, v4.
	for i, want := range []string{"v", "v2", "v3", "v4"} {
		if cols[i].Name != want {
			t.Errorf("column %d name = %q, want %q", i, cols[i].Name, want)
		}
	}
	if cols[4].Name != "Column5" {
		t.Errorf("column 4 name = %q, want %q", cols[4].Name, "Column5")
	}
}

// TestAddTableDuplicateHeadersDedup covers the suffix search in
// resolveTableColumnNames: a wide range whose header cells all carry the same
// text must still produce unique names, and must not do it in quadratic time.
func TestAddTableDuplicateHeadersDedup(t *testing.T) {
	w := Create()
	s, err := w.AddSheet("Sheet1")
	if err != nil {
		t.Fatalf("AddSheet: %v", err)
	}
	const n = 4096
	for c := 1; c <= n; c++ {
		if err := s.SetCellValue(FormatCellRef(1, c), "dup"); err != nil {
			t.Fatalf("SetCellValue: %v", err)
		}
	}
	start := time.Now()
	tbl, err := s.AddTable("A1:"+FormatCellRef(4, n), TableOptions{Name: "T"})
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	t.Logf("AddTable over %d identically-named headers: %v", n, time.Since(start))
	cols := tbl.Columns()
	if len(cols) != n {
		t.Fatalf("Columns() = %d, want %d", len(cols), n)
	}
	for i, c := range cols {
		want := "dup"
		if i > 0 {
			want = fmt.Sprintf("dup%d", i+1)
		}
		if c.Name != want {
			t.Fatalf("column %d name = %q, want %q", i, c.Name, want)
		}
	}
}
