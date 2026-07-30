package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

// UngroupColumns is the only column-range mutator in the package that was never
// executed. It rewrites the sheet's <cols> entries in bulk: the range carve
// splits every straddling entry into up-to-three pieces and rebuilds the list,
// so the risks are the classic bulk-rewrite ones — the wrong columns moved
// (off-by-one at either boundary), the untouched neighbours losing the
// properties they carried through the split (width, hidden), the clamp at zero
// failing (uint8 underflow to 255, which Excel rejects), and the change not
// surviving serialization.
//
// The assertions below therefore never look only at the columns that were
// ungrouped: each case pins the level of every column across the boundaries and
// re-reads them after a save/reopen.

// columnLevels reads the outline level of columns 1..n as a comparable vector.
func columnLevels(s *Sheet, n int) []uint8 {
	out := make([]uint8, n)
	for c := 1; c <= n; c++ {
		out[c-1] = s.ColumnOutlineLevel(c)
	}
	return out
}

func levelsEqual(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// saveReopenSheetBytes round-trips a workbook through the package writer and
// reader, returning the first sheet of the reopened copy together with the
// saved package bytes (page_setup_test.go's saveReopenSheet returns the sheet
// only, and these tests also assert on the serialized part).
func saveReopenSheetBytes(t *testing.T, w *Workbook) (*Sheet, []byte) {
	t.Helper()
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = re.Close() })
	return re.Sheets()[0], out
}

// Ungrouping an inner sub-range lowers exactly that range by one level and
// leaves the columns on both sides of each boundary alone, both in memory and
// after a save/reopen.
func TestUngroupColumnsAffectsOnlyItsRange(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "S")

	if err := s.GroupColumns(2, 8); err != nil {
		t.Fatalf("GroupColumns: %v", err)
	}
	if err := s.GroupColumns(2, 8); err != nil { // level 2 across B..H
		t.Fatalf("GroupColumns: %v", err)
	}
	if err := s.UngroupColumns(4, 6); err != nil {
		t.Fatalf("UngroupColumns: %v", err)
	}

	want := []uint8{0, 2, 2, 1, 1, 1, 2, 2, 0, 0}
	if got := columnLevels(s, 10); !levelsEqual(got, want) {
		t.Errorf("levels after UngroupColumns(4,6) = %v, want %v", got, want)
	}
	re, _ := saveReopenSheetBytes(t, w)
	if got := columnLevels(re, 10); !levelsEqual(got, want) {
		t.Errorf("levels after save/reopen = %v, want %v", got, want)
	}
}

// wideColsSheetXML is a worksheet whose columns are described by ONE wide
// <col> entry, the shape Excel actually writes and the shape the range carve
// has to split into a left piece, per-column middles and a right piece.
// Building it through the package's own setters would not do: they create
// single-column entries, so a carve that drops the left or right piece stays
// invisible.
const wideColsSheetXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<cols><col min="1" max="10" width="14.5" customWidth="1" outlineLevel="3"/></cols>` +
	`<sheetData/></worksheet>`

// Ungrouping the middle of a single wide <col> entry must split it into three
// pieces that each keep the properties the original carried: the columns on
// both sides keep their level AND their width, and only the middle drops a
// level. A carve that forgets the left or the right piece silently resets those
// columns to the sheet default.
func TestUngroupColumnsSplitsWideEntryKeepingProperties(t *testing.T) {
	data := buildMutatorTestXlsx(t, wideColsSheetXML)
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = w.Close() }()
	s := w.Sheets()[0]

	if err := s.UngroupColumns(4, 6); err != nil {
		t.Fatalf("UngroupColumns: %v", err)
	}

	// Columns 1-3 (left piece), 4-6 (carved middle), 7-10 (right piece).
	want := []uint8{3, 3, 3, 2, 2, 2, 3, 3, 3, 3}
	if got := columnLevels(s, 10); !levelsEqual(got, want) {
		t.Errorf("levels after carving a wide entry = %v, want %v", got, want)
	}

	re, _ := saveReopenSheetBytes(t, w)
	if got := columnLevels(re, 10); !levelsEqual(got, want) {
		t.Errorf("levels after save/reopen = %v, want %v", got, want)
	}
	for c := 1; c <= 10; c++ {
		width, ok := re.ColumnWidth(c)
		if !ok || width != 14.5 {
			t.Errorf("column %d width after the carve = (%v, %v), want (14.5, true) — a split piece lost the entry's properties",
				c, width, ok)
		}
	}
	// Column 11 was outside the original entry and must stay untouched.
	if _, ok := re.ColumnWidth(11); ok {
		t.Error("the carve invented a <col> entry past the original entry's max")
	}
}

// The same carve applied at the very start and the very end of a wide entry:
// ungrouping a prefix leaves no left piece and ungrouping a suffix leaves no
// right piece, the two boundary cases the three-way split gets wrong on its own.
func TestUngroupColumnsAtWideEntryBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, end int
		want       []uint8
	}{
		{"prefix", 1, 3, []uint8{2, 2, 2, 3, 3, 3, 3, 3, 3, 3}},
		{"suffix", 8, 10, []uint8{3, 3, 3, 3, 3, 3, 3, 2, 2, 2}},
		{"whole entry", 1, 10, []uint8{2, 2, 2, 2, 2, 2, 2, 2, 2, 2}},
		{"past the end", 8, 14, []uint8{3, 3, 3, 3, 3, 3, 3, 2, 2, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := buildMutatorTestXlsx(t, wideColsSheetXML)
			w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer func() { _ = w.Close() }()
			s := w.Sheets()[0]
			if err := s.UngroupColumns(tc.start, tc.end); err != nil {
				t.Fatalf("UngroupColumns(%d,%d): %v", tc.start, tc.end, err)
			}
			re, _ := saveReopenSheetBytes(t, w)
			if got := columnLevels(re, 10); !levelsEqual(got, tc.want) {
				t.Errorf("levels = %v, want %v", got, tc.want)
			}
			for c := 1; c <= 10; c++ {
				if width, ok := re.ColumnWidth(c); !ok || width != 14.5 {
					t.Errorf("column %d width = (%v, %v), want (14.5, true)", c, width, ok)
				}
			}
		})
	}
}

// A per-column property carried by a single column inside a wide entry must not
// spread to its neighbours when the entry is split.
func TestUngroupColumnsDoesNotSpreadPerColumnProperties(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "S")

	for c := 1; c <= 6; c++ {
		if err := s.SetColWidth(c, 14.5); err != nil {
			t.Fatalf("SetColWidth(%d): %v", c, err)
		}
	}
	if err := s.SetColumnHidden(5, true); err != nil {
		t.Fatalf("SetColumnHidden: %v", err)
	}
	if err := s.GroupColumns(1, 6); err != nil {
		t.Fatalf("GroupColumns: %v", err)
	}
	if err := s.UngroupColumns(2, 4); err != nil {
		t.Fatalf("UngroupColumns: %v", err)
	}

	re, _ := saveReopenSheetBytes(t, w)
	for c := 1; c <= 6; c++ {
		width, ok := re.ColumnWidth(c)
		if !ok || width != 14.5 {
			t.Errorf("column %d width after ungroup = (%v, %v), want (14.5, true)", c, width, ok)
		}
	}
	if !re.ColumnHidden(5) {
		t.Error("column 5 lost its hidden flag through the ungroup carve")
	}
	if re.ColumnHidden(4) || re.ColumnHidden(6) {
		t.Error("ungroup spread the hidden flag onto a neighbouring column")
	}
}

// Ungrouping past zero clamps instead of wrapping (the levels are uint8, so a
// missing guard underflows to 255) and clears the attribute entirely, so an
// ungrouped column serializes with no outlineLevel at all.
func TestUngroupColumnsClampsAtZeroAndClearsTheAttribute(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "S")

	if err := s.GroupColumns(2, 4); err != nil {
		t.Fatalf("GroupColumns: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := s.UngroupColumns(2, 4); err != nil {
			t.Fatalf("UngroupColumns: %v", err)
		}
	}
	for c := 1; c <= 5; c++ {
		if got := s.ColumnOutlineLevel(c); got != 0 {
			t.Errorf("column %d level after ungrouping past zero = %d, want 0", c, got)
		}
	}

	re, out := saveReopenSheetBytes(t, w)
	for c := 1; c <= 5; c++ {
		if got := re.ColumnOutlineLevel(c); got != 0 {
			t.Errorf("column %d level after save/reopen = %d, want 0", c, got)
		}
	}
	if sheetXML := string(readZipPart(t, out, "xl/worksheets/sheet1.xml")); strings.Contains(sheetXML, "outlineLevel") {
		t.Errorf("a fully ungrouped column still serializes outlineLevel:\n%s", sheetXML)
	}
}

// Ungrouping a range that was never grouped is a no-op at level 0 rather than
// an error or an underflow, and it must not invent levels for columns outside
// the range.
func TestUngroupColumnsOnUngroupedRange(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "S")

	if err := s.GroupColumns(6, 8); err != nil {
		t.Fatalf("GroupColumns: %v", err)
	}
	if err := s.UngroupColumns(1, 4); err != nil {
		t.Fatalf("UngroupColumns: %v", err)
	}
	want := []uint8{0, 0, 0, 0, 0, 1, 1, 1, 0}
	if got := columnLevels(s, 9); !levelsEqual(got, want) {
		t.Errorf("levels = %v, want %v", got, want)
	}
}

// Invalid ranges are rejected before anything is written.
func TestUngroupColumnsRejectsInvalidRanges(t *testing.T) {
	cases := [][2]int{{0, 3}, {-1, 2}, {5, 4}}
	for _, c := range cases {
		w := Create()
		s := addSheetT(w, "S")
		if err := s.GroupColumns(1, 3); err != nil {
			t.Fatalf("GroupColumns: %v", err)
		}
		before := columnLevels(s, 5)
		if err := s.UngroupColumns(c[0], c[1]); err != ErrInvalidRange {
			t.Errorf("UngroupColumns(%d,%d) error = %v, want ErrInvalidRange", c[0], c[1], err)
		}
		if got := columnLevels(s, 5); !levelsEqual(got, before) {
			t.Errorf("UngroupColumns(%d,%d) changed levels %v -> %v despite the error",
				c[0], c[1], before, got)
		}
		_ = w.Close()
	}
}

// GroupColumns and UngroupColumns are inverses over a level that is neither
// clamp: grouping then ungrouping the same range returns every column to where
// it started, including the ones the carve split off.
func TestGroupUngroupColumnsRoundTrip(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "S")

	if err := s.GroupColumns(1, 12); err != nil {
		t.Fatalf("GroupColumns: %v", err)
	}
	before := columnLevels(s, 14)
	if err := s.GroupColumns(3, 7); err != nil {
		t.Fatalf("GroupColumns: %v", err)
	}
	if err := s.UngroupColumns(3, 7); err != nil {
		t.Fatalf("UngroupColumns: %v", err)
	}
	if got := columnLevels(s, 14); !levelsEqual(got, before) {
		t.Errorf("group/ungroup of the same range is not an identity: %v -> %v", before, got)
	}
}

// FuzzXlsxColumnOutline drives an arbitrary sequence of column grouping
// operations — the bulk <cols> rewrite — and then requires the workbook to save
// and reopen with the same outline levels it reported in memory.
//
// The invariants are the ones a corrupt carve breaks: every level stays inside
// Excel's 0..7 range (an unguarded decrement underflows uint8 to 255 and an
// unguarded increment runs past 7), the levels survive serialization, and the
// package stays readable. Levels are read back through the public accessor, so
// a rebuilt <cols> list that drops, duplicates or mis-bounds an entry shows up
// as a level mismatch on some column.
func FuzzXlsxColumnOutline(f *testing.F) {
	f.Add([]byte{0, 2, 8, 1, 4, 6})
	f.Add([]byte{0, 1, 16, 0, 1, 16, 1, 1, 16})
	f.Add([]byte{1, 3, 3})
	f.Add([]byte{0, 5, 2, 1, 2, 5, 2, 4, 4})
	f.Add([]byte{2, 1, 1, 0, 1, 1})
	f.Add([]byte{})

	const maxCol = 24
	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) > 120 {
			ops = ops[:120]
		}
		w := Create()
		defer func() { _ = w.Close() }()
		s := addSheetT(w, "S")

		for i := 0; i+2 < len(ops); i += 3 {
			start := int(ops[i+1])%maxCol + 1
			end := int(ops[i+2])%maxCol + 1
			if end < start {
				start, end = end, start
			}
			var err error
			switch ops[i] % 4 {
			case 0:
				err = s.GroupColumns(start, end)
			case 1:
				err = s.UngroupColumns(start, end)
			case 2:
				err = s.SetColumnOutlineLevel(start, ops[i+2]%9) // 8 is out of range
			default:
				err = s.SetColWidth(start, float64(ops[i+2]%40)+1)
			}
			// Rejections are fine (out-of-range level, bad range); a rejected
			// call must simply not have corrupted anything, which the
			// invariants below check.
			_ = err
		}

		inMemory := columnLevels(s, maxCol)
		for c, lvl := range inMemory {
			if lvl > maxOutlineLevel {
				t.Fatalf("column %d outline level %d exceeds the maximum %d", c+1, lvl, maxOutlineLevel)
			}
		}

		out, err := w.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes after column outline edits: %v", err)
		}
		re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
		if err != nil {
			t.Fatalf("reopen after column outline edits: %v", err)
		}
		defer func() { _ = re.Close() }()
		if got := columnLevels(re.Sheets()[0], maxCol); !levelsEqual(got, inMemory) {
			t.Fatalf("outline levels changed across save/reopen:\n in memory %v\n reopened  %v", inMemory, got)
		}
	})
}
