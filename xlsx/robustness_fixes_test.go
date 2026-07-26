package xlsx

import "testing"

// TestParseCellRefMixedCase verifies ParseCellRef accepts column prefixes with
// mixed letter case, matching Excel's leniency.
func TestParseCellRefMixedCase(t *testing.T) {
	cases := []struct {
		ref      string
		row, col int
	}{
		{"Aa1", 1, 27},  // "AA" -> column 27
		{"aB3", 3, 28},  // "AB" -> column 28
		{"A1", 1, 1},    // baseline upper
		{"a1", 1, 1},    // baseline lower
		{"zZ100", 100, 702},
	}
	for _, c := range cases {
		row, col, err := ParseCellRef(c.ref)
		if err != nil {
			t.Errorf("ParseCellRef(%q) returned error: %v", c.ref, err)
			continue
		}
		if row != c.row || col != c.col {
			t.Errorf("ParseCellRef(%q) = (row %d, col %d), want (row %d, col %d)", c.ref, row, col, c.row, c.col)
		}
	}
}

// TestSheetsReturnsCopy verifies that mutating the slice returned by Sheets()
// does not affect the workbook's internal sheet order or per-sheet index.
func TestSheetsReturnsCopy(t *testing.T) {
	wb := Create()
	a := wb.AddSheet("Alpha")
	b := wb.AddSheet("Beta")
	c := wb.AddSheet("Gamma")
	_ = a
	_ = b
	_ = c

	got := wb.Sheets()
	if len(got) != 3 {
		t.Fatalf("Sheets() len = %d, want 3", len(got))
	}

	// Reverse and truncate the returned slice.
	got[0], got[2] = got[2], got[0]
	got = got[:1]
	_ = got

	// The workbook must be unaffected.
	after := wb.Sheets()
	if len(after) != 3 {
		t.Fatalf("workbook sheet count changed to %d, want 3", len(after))
	}
	wantOrder := []string{"Alpha", "Beta", "Gamma"}
	for i, name := range wantOrder {
		if after[i].Name() != name {
			t.Errorf("sheet[%d] = %q, want %q", i, after[i].Name(), name)
		}
		if after[i].index != i {
			t.Errorf("sheet[%d].index = %d, want %d", i, after[i].index, i)
		}
	}
}
