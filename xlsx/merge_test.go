package xlsx

import (
	"bytes"
	"testing"
)

func setCell(t *testing.T, s *Sheet, ref string, val interface{}) {
	t.Helper()
	if err := s.SetCellValue(ref, val); err != nil {
		t.Fatalf("SetCellValue(%s): %v", ref, err)
	}
}

func TestCopySheetFrom(t *testing.T) {
	src := Create()
	s := src.AddSheet("Data")
	setCell(t, s, "A1", "Name")
	setCell(t, s, "B1", "Score")
	setCell(t, s, "A2", "Alice")
	setCell(t, s, "B2", 42)
	setCell(t, s, "B3", 8)
	if c, err := s.Cell("B4"); err == nil {
		c.SetFormula("SUM(B2:B3)")
	} else {
		t.Fatalf("Cell B4: %v", err)
	}
	// A styled cell to exercise style index remapping.
	if c, err := s.Cell("A1"); err == nil {
		if err := c.SetStyle(CellStyle{Format: "0.00"}); err != nil {
			t.Fatalf("SetStyle: %v", err)
		}
	}
	if err := s.MergeCells("A1", "B1"); err != nil {
		t.Fatalf("MergeCells: %v", err)
	}
	if err := s.SetColWidth(1, 20); err != nil {
		t.Fatalf("SetColWidth: %v", err)
	}

	dst := Create()
	setCell(t, dst.AddSheet("Existing"), "A1", "keep")

	newSheet, err := dst.CopySheetFrom(src, "Data")
	if err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	if newSheet.Name() != "Data" {
		t.Fatalf("new sheet name = %q, want Data", newSheet.Name())
	}
	if got := dst.SheetCount(); got != 2 {
		t.Fatalf("SheetCount = %d, want 2", got)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after copy: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}

	got, err := re.SheetByName("Data")
	if err != nil {
		t.Fatalf("SheetByName after reopen: %v", err)
	}
	assertCell(t, got, "A1", "Name")
	assertCell(t, got, "A2", "Alice")
	assertCell(t, got, "B2", "42")
	// The original "Existing" sheet is untouched.
	existing, err := re.SheetByName("Existing")
	if err != nil {
		t.Fatalf("SheetByName Existing: %v", err)
	}
	assertCell(t, existing, "A1", "keep")

	// The merged range came across.
	if got.worksheet.MergeCells == nil || len(got.worksheet.MergeCells.MergeCell) == 0 {
		t.Fatalf("merged range not copied")
	}
}

func assertCell(t *testing.T, s *Sheet, ref, want string) {
	t.Helper()
	got, err := s.GetCellValue(ref)
	if err != nil {
		t.Fatalf("GetCellValue(%s): %v", ref, err)
	}
	if got != want {
		t.Errorf("cell %s = %q, want %q", ref, got, want)
	}
}

func TestCopySheetFromDuplicateName(t *testing.T) {
	src := Create()
	setCell(t, src.AddSheet("Report"), "A1", "x")

	dst := Create()
	setCell(t, dst.AddSheet("Report"), "A1", "orig")

	newSheet, err := dst.CopySheetFrom(src, "Report")
	if err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	if newSheet.Name() == "Report" {
		t.Fatalf("expected a de-duplicated sheet name, got %q", newSheet.Name())
	}
	if got := dst.SheetCount(); got != 2 {
		t.Fatalf("SheetCount = %d, want 2", got)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate: %v", r)
	}
}

func TestCopySheetFromMissing(t *testing.T) {
	src := Create()
	setCell(t, src.AddSheet("A"), "A1", "x")
	dst := Create()
	setCell(t, dst.AddSheet("B"), "A1", "y")
	if _, err := dst.CopySheetFrom(src, "Nope"); err != ErrSheetNotFound {
		t.Fatalf("err = %v, want ErrSheetNotFound", err)
	}
}

func TestCopySheetFromNil(t *testing.T) {
	dst := Create()
	setCell(t, dst.AddSheet("A"), "A1", "x")
	if _, err := dst.CopySheetFrom(nil, "A"); err != ErrNilWorkbook {
		t.Fatalf("err = %v, want ErrNilWorkbook", err)
	}
}
