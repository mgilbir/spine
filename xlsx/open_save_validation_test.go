package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
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
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/book.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.ws()+xml"/></Types>`},
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
//
// Each pass is a separate edit session, and an edit records its write time in
// dcterms:modified (see modified.go), which two passes either side of a second
// boundary would legitimately disagree about — the flake TestFurnitureDeterministic
// showed in pptx for three audits. Pinning Properties.Modified (an explicit
// assignment the save respects) keeps the subject of this test the zip entry
// order rather than the clock; the stamp's own determinism is pinned by
// TestEditedWorkbookSaveIsStillIdempotent.
func TestXlsxSaveIsDeterministic(t *testing.T) {
	pinned := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	build := func() []byte {
		wb, err := Open("testdata/minimal.xlsx")
		if err != nil {
			t.Fatal(err)
		}
		defer wb.Close() //nolint:errcheck
		wb.Properties.Modified = pinned
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

// buildBookXlsxWithout rebuilds the buildBookXlsx fixture, dropping the named
// entry.
func buildBookXlsxWithout(t *testing.T, drop string) []byte {
	t.Helper()
	full := buildBookXlsx(t)
	zr, err := zip.NewReader(bytes.NewReader(full), int64(len(full)))
	if err != nil {
		t.Fatal(err)
	}
	var files []struct{ name, data string }
	for _, f := range zr.File {
		if f.Name == drop {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(rc); err != nil {
			t.Fatal(err)
		}
		_ = rc.Close()
		files = append(files, struct{ name, data string }{f.Name, b.String()})
	}
	return buildFixtureXlsxParts(t, files)
}

// C60: a worksheet referenced from workbook.xml but absent from the package
// must fail Open. Previously an empty sheet model was materialized, and the
// first save wrote a fabricated near-empty sheet part in its place.
func TestOpenErrorsOnMissingReferencedSheetPart(t *testing.T) {
	data := buildBookXlsxWithout(t, "xl/worksheets/sheet1.xml")
	_, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Fatal("Open succeeded on a workbook whose referenced sheet part is missing")
	}
	if !strings.Contains(err.Error(), "/xl/worksheets/sheet1.xml") {
		t.Errorf("error does not name the missing part: %v", err)
	}
}

// C60: a sheet whose r:id has no matching relationship must fail Open for the
// same reason (nothing to load, so saving would fabricate an empty sheet).
func TestOpenErrorsOnDanglingSheetRelationship(t *testing.T) {
	data := buildBookXlsxWithout(t, "xl/_rels/book.xml.rels")
	_, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Fatal("Open succeeded on a workbook whose sheet r:id resolves to no relationship")
	}
	if !strings.Contains(err.Error(), "rId1") {
		t.Errorf("error does not name the dangling relationship: %v", err)
	}
}

// Raw .rels preservation: when a save rebuilds the workbook relationship set
// but ends up with the same set that was parsed, the source bytes — BOM,
// non-canonical prolog, producer attribute order — must be written verbatim
// instead of regenerated in canonical form.
func TestWorkbookRelsPreservedVerbatimOnUnchangedSet(t *testing.T) {
	rawRels := "\xef\xbb\xbf" + `<?xml version="1.0" encoding="utf-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml" Id="rId1" />` +
		`</Relationships>`
	full := buildBookXlsx(t)
	zr, err := zip.NewReader(bytes.NewReader(full), int64(len(full)))
	if err != nil {
		t.Fatal(err)
	}
	var files []struct{ name, data string }
	for _, f := range zr.File {
		data := ""
		if f.Name == "xl/_rels/book.xml.rels" {
			data = rawRels
		} else {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if _, err := b.ReadFrom(rc); err != nil {
				t.Fatal(err)
			}
			_ = rc.Close()
			data = b.String()
		}
		files = append(files, struct{ name, data string }{f.Name, data})
	}
	fixture := buildFixtureXlsxParts(t, files)

	wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	// Dirty the sheet so the save takes the rels-rebuild path.
	if err := sheet.SetCellValue("B1", 7); err != nil {
		t.Fatal(err)
	}
	saved, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	got := string(readZipPart(t, saved, "xl/_rels/book.xml.rels"))
	if got != rawRels {
		t.Errorf("workbook .rels regenerated despite unchanged set:\ngot  %q\nwant %q", got, rawRels)
	}
}
