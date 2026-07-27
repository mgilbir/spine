package xlsx

import (
	"bytes"
	"testing"
)

// C13: deleting a middle sheet then adding one must not reuse a sheetId.
func TestAddSheet_UniqueSheetIDAfterDelete(t *testing.T) {
	wb := Create()
	addSheetT(wb, "A")
	addSheetT(wb, "B")
	addSheetT(wb, "C")
	if err := wb.DeleteSheet(0); err != nil { // delete "A"
		t.Fatal(err)
	}
	addSheetT(wb, "D")

	seen := map[uint32]bool{}
	for _, s := range wb.workbook.Sheets.Sheet {
		if seen[s.SheetId] {
			t.Fatalf("duplicate sheetId %d in %v", s.SheetId, sheetIDs(wb))
		}
		seen[s.SheetId] = true
	}
}

func sheetIDs(wb *Workbook) []uint32 {
	var ids []uint32
	for _, s := range wb.workbook.Sheets.Sheet {
		ids = append(ids, s.SheetId)
	}
	return ids
}

// C15: Close() then Save() must still preserve parts (round-trip path), not
// silently fall back to the from-scratch writer.
func TestCloseThenSavePreservesParts(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	sheetsBefore := wb.SheetCount()

	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after Close: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen after Close+Save: %v", err)
	}
	if got := reopened.SheetCount(); got != sheetsBefore {
		t.Errorf("sheet count after Close+Save = %d, want %d (preserved parts dropped)", got, sheetsBefore)
	}
}

// C71: sheet-name validation. The sanitization half of this test moved to
// TestAddSheetRejectsInvalidAndDuplicateNames and
// TestUniqueSheetNameCoercesExplicitly: C440 replaced AddSheet's silent
// coercion with an error plus an explicit UniqueSheetName opt-in, so asserting
// that AddSheet renames the caller's sheet would now pin the defect.
func TestSheetNameValidation(t *testing.T) {
	if err := ValidateSheetName(""); err == nil {
		t.Error("empty name should be invalid")
	}
	if err := ValidateSheetName("has/slash"); err == nil {
		t.Error("forbidden character should be invalid")
	}
	if err := ValidateSheetName("0123456789012345678901234567890123"); err == nil {
		t.Error(">31 chars should be invalid")
	}
	if err := ValidateSheetName("Good Name"); err != nil {
		t.Errorf("valid name rejected: %v", err)
	}

	// A sheet added under a legal, free name keeps exactly that name.
	wb := Create()
	s1 := addSheetT(wb, "Data")
	if s1.Name() != "Data" {
		t.Errorf("AddSheet(%q) produced a sheet named %q", "Data", s1.Name())
	}
	if _, err := wb.SheetByName("Data"); err != nil {
		t.Errorf("SheetByName(%q) after AddSheet(%q): %v", "Data", "Data", err)
	}
}

// C72: merely reading Styles() on a workbook that already has a stylesheet must
// not dirty styles (which would force styles.xml to be regenerated); a mutating
// call must dirty it.
func TestStylesReadDoesNotDirty(t *testing.T) {
	wb := Create()
	wb.stylesheet = defaultStylesheet() // simulate an existing styles part
	wb.stylesDirty = false

	_ = wb.Styles() // read-only access
	if wb.stylesDirty {
		t.Error("reading Styles() marked styles dirty (breaks byte-identical round-trip)")
	}

	// A mutating call flips the flag.
	if _, err := wb.Styles().NewCellStyle(CellStyle{Format: "0.00"}); err != nil {
		t.Fatal(err)
	}
	if !wb.stylesDirty {
		t.Error("mutating a style did not mark styles dirty")
	}
}
