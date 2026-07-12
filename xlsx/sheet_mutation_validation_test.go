package xlsx

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// C71: SetName must validate like AddSheet instead of accepting forbidden
// characters, overlong names, and duplicates verbatim.
func TestSetNameValidates(t *testing.T) {
	wb := Create()
	s1 := wb.AddSheet("Data")
	s2 := wb.AddSheet("Other")

	if err := s2.SetName("bad/name"); err == nil {
		t.Error("forbidden character accepted")
	}
	if err := s2.SetName("0123456789012345678901234567890123"); err == nil {
		t.Error(">31 char name accepted")
	}
	if err := s2.SetName("data"); err == nil {
		t.Error("case-insensitive duplicate accepted")
	}
	if s2.Name() != "Other" {
		t.Errorf("rejected rename mutated the sheet name to %q", s2.Name())
	}

	if err := s2.SetName("Renamed"); err != nil {
		t.Fatalf("valid rename rejected: %v", err)
	}
	if s2.Name() != "Renamed" {
		t.Errorf("Name() = %q, want Renamed", s2.Name())
	}
	if got := wb.workbook.Sheets.Sheet[1].Name; got != "Renamed" {
		t.Errorf("workbook model name = %q, want Renamed", got)
	}
	// Renaming a sheet to its own current name is not a duplicate.
	if err := s1.SetName("Data"); err != nil {
		t.Errorf("self-rename rejected: %v", err)
	}
}

// C126: "A01" must address the same cell as "A1" instead of creating a
// phantom second cell beside it.
func TestCellRefCanonicalized(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	c1, err := sheet.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	c1.SetValue("first")

	c2, err := sheet.Cell("A01")
	if err != nil {
		t.Fatal(err)
	}
	if c2.Ref() != "A1" {
		t.Errorf("Cell(A01).Ref() = %q, want A1", c2.Ref())
	}
	c2.SetValue("second")

	if got, _ := sheet.GetCellValue("a01"); got != "second" {
		t.Errorf("GetCellValue(a01) = %q, want second", got)
	}
	if n := len(sheet.worksheet.SheetData.Row[0].C); n != 1 {
		t.Fatalf("row 1 has %d cells, want 1 (phantom cell created)", n)
	}
}

// C127: SetColWidth on a sheet with a ranged <col min max> entry must split
// the range instead of appending an overlapping entry.
func TestSetColWidthSplitsRangedEntry(t *testing.T) {
	sheetXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cols><col min="1" max="5" width="20" customWidth="1"/></cols><sheetData/></worksheet>`
	data := buildMutatorTestXlsx(t, sheetXML)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetColWidth(3, 42); err != nil {
		t.Fatal(err)
	}

	cols := sheet.worksheet.Cols[0].Col
	covered := map[uint32][]int{}
	for i, c := range cols {
		if c.Min > c.Max {
			t.Errorf("entry %d has min %d > max %d", i, c.Min, c.Max)
		}
		for n := c.Min; n <= c.Max; n++ {
			covered[n] = append(covered[n], i)
		}
	}
	for n, entries := range covered {
		if len(entries) > 1 {
			t.Errorf("column %d covered by %d entries (overlap): %+v", n, len(entries), cols)
		}
	}

	widthOf := func(n uint32) float64 {
		for _, c := range cols {
			if c.Min <= n && n <= c.Max {
				if c.Width == nil {
					t.Fatalf("column %d has no width: %+v", n, cols)
				}
				return *c.Width
			}
		}
		t.Fatalf("column %d not covered: %+v", n, cols)
		return 0
	}
	if w := widthOf(3); w != 42 {
		t.Errorf("column 3 width = %v, want 42", w)
	}
	for _, n := range []uint32{1, 2, 4, 5} {
		if w := widthOf(n); w != 20 {
			t.Errorf("column %d width = %v, want 20 (range properties lost)", n, w)
		}
	}
}

// C128: MergeCells must validate references, normalize their order, and
// reject duplicate or overlapping merges.
func TestMergeCellsValidation(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	if err := sheet.MergeCells("foo", "bar"); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("garbage refs: got %v, want ErrInvalidRange", err)
	}
	if sheet.worksheet != nil && sheet.worksheet.MergeCells != nil {
		t.Fatal("invalid merge was recorded")
	}

	// Reversed corners are normalized to top-left:bottom-right.
	if err := sheet.MergeCells("B2", "A1"); err != nil {
		t.Fatal(err)
	}
	if got := sheet.worksheet.MergeCells.MergeCell[0].Ref; got != "A1:B2" {
		t.Errorf("merge ref = %q, want A1:B2", got)
	}

	if err := sheet.MergeCells("A1", "B2"); err == nil {
		t.Error("duplicate merge accepted")
	}
	if err := sheet.MergeCells("B2", "C3"); err == nil {
		t.Error("overlapping merge accepted")
	}
	if err := sheet.MergeCells("C3", "D4"); err != nil {
		t.Errorf("non-overlapping merge rejected: %v", err)
	}
	if n := len(sheet.worksheet.MergeCells.MergeCell); n != 2 {
		t.Errorf("merge count = %d, want 2", n)
	}

	// Unmerge accepts either corner order.
	if err := sheet.UnmergeCells("D4", "C3"); err != nil {
		t.Fatal(err)
	}
	if n := len(sheet.worksheet.MergeCells.MergeCell); n != 1 {
		t.Errorf("merge count after unmerge = %d, want 1", n)
	}
}

// C128: merges parsed from an existing file constrain new merges.
func TestMergeCellsOverlapAgainstParsedMerges(t *testing.T) {
	sheetXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/><mergeCells count="1"><mergeCell ref="A1:C3"/></mergeCells></worksheet>`
	data := buildMutatorTestXlsx(t, sheetXML)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := sheet.MergeCells("B2", "D4"); err == nil {
		t.Error("merge overlapping a parsed merge accepted")
	} else if !strings.Contains(err.Error(), "A1:C3") {
		t.Errorf("error does not name the conflicting merge: %v", err)
	}
	if err := sheet.MergeCells("D4", "E5"); err != nil {
		t.Errorf("non-overlapping merge rejected: %v", err)
	}
}
