package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

// C78: a sheet part whose XML fails to parse must be an Open error. Silently
// keeping an empty sheet model means one later innocent mutation replaces the
// original part with a fabricated near-empty sheet, destroying the data.
func TestOpenErrorsOnCorruptSheetPart(t *testing.T) {
	corrupt := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>` // truncated
	data := buildMutatorTestXlsx(t, corrupt)

	_, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Fatal("Open succeeded on a workbook with a corrupt sheet part")
	}
	if !strings.Contains(err.Error(), "/xl/worksheets/sheet1.xml") {
		t.Errorf("error does not name the failing part: %v", err)
	}
}

// C78: valid workbooks must be unaffected by the corrupt-sheet check.
func TestOpenValidWorkbookStillSucceeds(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("open valid workbook: %v", err)
	}
	defer wb.Close() //nolint:errcheck
	if wb.SheetCount() == 0 {
		t.Fatal("no sheets loaded")
	}
}

// C130: saving a workbook with zero sheets must fail; Excel refuses such files.
func TestSaveZeroSheetWorkbookErrors(t *testing.T) {
	wb := Create()
	if _, err := wb.SaveBytes(); !errors.Is(err, ErrNoSheets) {
		t.Fatalf("SaveBytes on empty workbook: got %v, want ErrNoSheets", err)
	}

	// Deleting every sheet from an opened workbook must also refuse to save.
	opened, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close() //nolint:errcheck
	for opened.SheetCount() > 0 {
		if err := opened.DeleteSheet(0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := opened.SaveBytes(); !errors.Is(err, ErrNoSheets) {
		t.Fatalf("SaveBytes after deleting all sheets: got %v, want ErrNoSheets", err)
	}
}

// buildFixtureXlsxParts zips the given entries (in order) into an in-memory
// package.
func buildFixtureXlsxParts(t *testing.T, files []struct{ name, data string }) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatalf("create %s: %v", f.name, err)
		}
		if _, err := w.Write([]byte(f.data)); err != nil {
			t.Fatalf("write %s: %v", f.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// buildBookXlsx assembles a workbook whose main part is /xl/book.xml instead
// of the conventional /xl/workbook.xml.
func buildBookXlsx(t *testing.T) []byte {
	t.Helper()
	return buildFixtureXlsxParts(t, []struct{ name, data string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/book.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/book.xml"/></Relationships>`},
		{"xl/book.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/book.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>1</v></c></row></sheetData></worksheet>`},
	})
}

// C231: with the main workbook part at a non-standard name (/xl/book.xml),
// save wrote the regenerated workbook to a hardcoded /xl/workbook.xml while
// the stale original part and the rels pointing at it were preserved — the
// package forked into a stale original plus an orphan workbook.xml.
func TestNonStandardMainWorkbookPartName(t *testing.T) {
	data := buildBookXlsx(t)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetCellValue("B1", 42); err != nil {
		t.Fatal(err)
	}
	saved, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if !zipHasPart(t, saved, "xl/book.xml") {
		t.Fatal("xl/book.xml missing from saved package")
	}
	if zipHasPart(t, saved, "xl/workbook.xml") {
		t.Fatal("orphan xl/workbook.xml written alongside xl/book.xml")
	}
	rootRels := string(readZipPart(t, saved, "_rels/.rels"))
	if !strings.Contains(rootRels, `Target="xl/book.xml"`) {
		t.Fatalf("root relationship no longer points at xl/book.xml:\n%s", rootRels)
	}

	// The edit must be visible after reopening.
	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2, err := reopened.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.GetCellValue("B1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "42" {
		t.Fatalf("edit lost after reopen: B1 = %q, want 42", got)
	}
}

// Wave-2 finding: preserved parts were written in map-iteration order, so the
// zip entry order (and therefore whole-archive bytes) differed between runs.
func TestXlsxSaveIsDeterministic(t *testing.T) {
	build := func() []byte {
		wb, err := Open("testdata/minimal.xlsx")
		if err != nil {
			t.Fatal(err)
		}
		defer wb.Close() //nolint:errcheck
		sheet, err := wb.Sheet(0)
		if err != nil {
			t.Fatal(err)
		}
		if err := sheet.SetCellValue("Z9", "determinism probe"); err != nil {
			t.Fatal(err)
		}
		saved, err := wb.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		return saved
	}
	first := build()
	for i := 0; i < 4; i++ {
		if !bytes.Equal(first, build()) {
			t.Fatalf("save %d produced different archive bytes", i+2)
		}
	}
}
