package xlsx

import (
	"strings"
	"testing"
)

func TestSheetAndWorkbookText(t *testing.T) {
	wb := Create()

	s1 := addSheetT(wb, "Sheet1")
	if err := s1.SetCellValue("A1", "Name"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	mustSet(t, s1, "B1", "Score")
	mustSet(t, s1, "A2", "Alice")
	mustSet(t, s1, "B2", 42)
	mustSet(t, s1, "A3", "Bob")
	mustSet(t, s1, "C3", "gap") // leaves B3 empty to exercise column gaps

	s1.AddComment("A1", "Reviewer", "Header cell")

	want1 := "Name\tScore\nAlice\t42\nBob\t\tgap\nHeader cell"
	if got := s1.Text(); got != want1 {
		t.Fatalf("Sheet.Text() =\n%q\nwant\n%q", got, want1)
	}

	s2 := addSheetT(wb, "Sheet2")
	mustSet(t, s2, "A1", "Second")

	want := want1 + "\n\n" + "Second"
	if got := wb.Text(); got != want {
		t.Fatalf("Workbook.Text() =\n%q\nwant\n%q", got, want)
	}
}

func TestWorkbookTextEmpty(t *testing.T) {
	wb := Create()
	if got := wb.Text(); got != "" {
		t.Errorf("empty workbook Text() = %q, want \"\"", got)
	}
}

func TestSheetTextSharedString(t *testing.T) {
	wb := Create()
	s := addSheetT(wb, "Sheet1")
	mustSet(t, s, "A1", "shared")
	mustSet(t, s, "A2", "shared")
	if got := s.Text(); !strings.Contains(got, "shared\nshared") {
		t.Errorf("Text() did not resolve shared strings; got %q", got)
	}
}

func mustSet(t *testing.T, s *Sheet, ref string, v interface{}) {
	t.Helper()
	if err := s.SetCellValue(ref, v); err != nil {
		t.Fatalf("SetCellValue(%s): %v", ref, err)
	}
}
