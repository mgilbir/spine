package xlsx

import (
	"bytes"
	"testing"
)

// C13: deleting a middle sheet then adding one must not reuse a sheetId.
func TestAddSheet_UniqueSheetIDAfterDelete(t *testing.T) {
	wb := Create()
	wb.AddSheet("A")
	wb.AddSheet("B")
	wb.AddSheet("C")
	if err := wb.DeleteSheet(0); err != nil { // delete "A"
		t.Fatal(err)
	}
	wb.AddSheet("D")

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

// C71: sheet-name validation and AddSheet sanitization.
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

	wb := Create()
	s1 := wb.AddSheet("Data")
	s2 := wb.AddSheet("Data") // duplicate -> renamed
	if s2.Name() == s1.Name() {
		t.Errorf("duplicate sheet name not disambiguated: both %q", s1.Name())
	}
	s3 := wb.AddSheet(`a[b]c*?`) // forbidden chars stripped
	if err := ValidateSheetName(s3.Name()); err != nil {
		t.Errorf("AddSheet produced an invalid name %q: %v", s3.Name(), err)
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
