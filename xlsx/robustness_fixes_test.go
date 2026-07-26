package xlsx

import "testing"

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
