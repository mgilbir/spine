package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// buildCorePackage zips the given part map into an xlsx package. Callers supply
// every part verbatim so a finding can be reproduced against an exact producer
// shape.
func buildCorePackage(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range parts {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

const coreRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`

// ---------------------------------------------------------------------------
// C366 — DeleteSheet on an opaque sheet leaves its workbook relationship
// ---------------------------------------------------------------------------

// TestDeleteChartsheetDropsItsRelationship guards C366: deleting a chartsheet
// removes its part, so the workbook .rels must not keep the chartsheet-typed
// relationship pointing at the now-absent part (Excel repairs such a package).
func TestDeleteChartsheetDropsItsRelationship(t *testing.T) {
	src := buildChartsheetWorkbook(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if err := wb.DeleteSheet(1); err != nil {
		t.Fatalf("DeleteSheet: %v", err)
	}
	// The output-set validation must see the reference as dangling before the
	// save, not after Excel opens the file.
	if rep := wb.Validate(); rep.HasErrors() {
		t.Fatalf("Validate after DeleteSheet reported errors: %v", rep.Errors())
	}

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if zipHasPart(t, out, "xl/chartsheets/sheet1.xml") {
		t.Fatalf("deleted chartsheet part is still present in the saved package")
	}
	rels, err := opc.UnmarshalRelationships(readZipPart(t, out, "xl/_rels/workbook.xml.rels"))
	if err != nil {
		t.Fatalf("UnmarshalRelationships: %v", err)
	}
	for _, rel := range rels {
		if strings.Contains(rel.Target, "chartsheets/") {
			t.Fatalf("workbook .rels still targets the deleted chartsheet: %+v", rel)
		}
	}
	// And the sheet list must no longer reference it either.
	wbXML := string(readZipPart(t, out, "xl/workbook.xml"))
	if strings.Contains(wbXML, `name="Chart"`) {
		t.Errorf("workbook.xml still lists the deleted chartsheet:\n%s", wbXML)
	}
}

// ---------------------------------------------------------------------------
// C367 — the 1904 date system is parsed but never honored
// ---------------------------------------------------------------------------

// buildDatePackage returns a one-sheet package whose workbookPr carries the
// given date1904 attribute text (empty for none) and whose A1 holds serial.
func buildDatePackage(t *testing.T, date1904Attr, serial string) []byte {
	t.Helper()
	pr := ""
	if date1904Attr != "" {
		pr = `<workbookPr date1904="` + date1904Attr + `"/>`
	}
	return buildCorePackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels": coreRootRels,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + pr + `<sheets><sheet name="Data" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>` + serial + `</v></c></row></sheetData></worksheet>`,
	})
}

// TestDate1904ReadAndWrite guards C367: on a workbook declaring the 1904 date
// system the serial↔time conversions must use the 1904-01-01 epoch (and no
// fictitious leap day) in both directions.
func TestDate1904ReadAndWrite(t *testing.T) {
	src := buildDatePackage(t, "1", "100")
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet: %v", err)
	}
	c, err := sh.Cell("A1")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	want := time.Date(1904, 4, 10, 0, 0, 0, 0, time.UTC)
	if got := c.Time(); !got.Equal(want) {
		t.Errorf("1904 serial 100 read as %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}

	// The two systems differ by exactly 1462 days (1904-01-01 is serial 1462
	// in the 1900 system), so 2020-01-01 is 43831-1462 = 42369.
	c.SetTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if got := c.String(); got != "42369" {
		t.Errorf("1904 SetTime(2020-01-01) stored %q, want \"42369\"", got)
	}
	// Round-trip through the same workbook.
	if got := c.Time(); !got.Equal(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("1904 round-trip = %s, want 2020-01-01", got.Format(time.RFC3339))
	}
}

// TestDate1900Unaffected pins the default (1900) system, including the
// fictitious-leap-day serials 59/60/61, against the C367 change.
func TestDate1900Unaffected(t *testing.T) {
	src := buildDatePackage(t, "", "100")
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	c, err := sh.Cell("A1")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	want := time.Date(1900, 4, 9, 0, 0, 0, 0, time.UTC)
	if got := c.Time(); !got.Equal(want) {
		t.Errorf("1900 serial 100 read as %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	c.SetTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if got := c.String(); got != "43831" {
		t.Errorf("1900 SetTime(2020-01-01) stored %q, want \"43831\"", got)
	}
}

// TestDate1904LenientBoolForms covers the lexical forms Excel and third-party
// producers write for date1904.
func TestDate1904LenientBoolForms(t *testing.T) {
	for _, form := range []string{"1", "true", "on"} {
		src := buildDatePackage(t, form, "0")
		wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
		if err != nil {
			t.Fatalf("OpenReader(%s): %v", form, err)
		}
		sh, _ := wb.Sheet(0)
		c, err := sh.Cell("A1")
		if err != nil {
			t.Fatalf("Cell: %v", err)
		}
		want := time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
		if got := c.Time(); !got.Equal(want) {
			t.Errorf("date1904=%q: serial 0 read as %s, want %s", form, got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	}
	for _, form := range []string{"0", "false"} {
		src := buildDatePackage(t, form, "1")
		wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
		if err != nil {
			t.Fatalf("OpenReader(%s): %v", form, err)
		}
		sh, _ := wb.Sheet(0)
		c, err := sh.Cell("A1")
		if err != nil {
			t.Fatalf("Cell: %v", err)
		}
		want := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		if got := c.Time(); !got.Equal(want) {
			t.Errorf("date1904=%q: serial 1 read as %s, want %s", form, got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	}
}

// ---------------------------------------------------------------------------
// C383 — editColumn ignored every <cols> group but the first (re-opens C127)
// ---------------------------------------------------------------------------

// buildMultiColsPackage returns a package whose sheet carries two <cols>
// groups; column 6 is covered only by the second.
func buildMultiColsPackage(t *testing.T) []byte {
	t.Helper()
	return buildCorePackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels": coreRootRels,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cols><col min="1" max="3" width="10" customWidth="1"/></cols><cols><col min="5" max="8" width="20" customWidth="1"/></cols><sheetData/></worksheet>`,
	})
}

// colEntriesCovering returns every <col> entry across every group that spans col.
func colEntriesCovering(s *Sheet, col uint32) []string {
	var out []string
	for gi := range s.ws().Cols {
		for _, e := range s.ws().Cols[gi].Col {
			if e.Min <= col && col <= e.Max {
				desc := fmt.Sprintf("min=%d max=%d", e.Min, e.Max)
				if e.Width != nil {
					desc += fmt.Sprintf(" width=%g", *e.Width)
				}
				if e.Hidden != nil && *e.Hidden {
					desc += " hidden"
				}
				out = append(out, desc)
			}
		}
	}
	return out
}

// TestSetColumnHiddenCarvesEveryColsGroup guards C383: hiding a column covered
// only by a later <cols> group must carve that group's entry (inheriting its
// width) instead of appending a bare overlapping entry to the first group.
func TestSetColumnHiddenCarvesEveryColsGroup(t *testing.T) {
	src := buildMultiColsPackage(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	if err := sh.SetColumnHidden(6, true); err != nil {
		t.Fatalf("SetColumnHidden: %v", err)
	}
	covering := colEntriesCovering(sh, 6)
	if len(covering) != 1 {
		t.Fatalf("column 6 is covered by %d <col> entries (%v), want exactly 1 — overlapping entries are rejected by Excel (C127/C383)", len(covering), covering)
	}
	if !strings.Contains(covering[0], "width=20") {
		t.Errorf("carved entry lost the spanned range's width: %s", covering[0])
	}
	if !strings.Contains(covering[0], "hidden") {
		t.Errorf("carved entry is not hidden: %s", covering[0])
	}
	if !sh.ColumnHidden(6) {
		t.Errorf("ColumnHidden(6) = false after SetColumnHidden(6, true)")
	}
	// The untouched neighbours keep their range and properties.
	if got, ok := sh.ColumnWidth(7); !ok || got != 20 {
		t.Errorf("ColumnWidth(7) = (%v, %v), want (20, true)", got, ok)
	}
	if got, ok := sh.ColumnWidth(2); !ok || got != 10 {
		t.Errorf("ColumnWidth(2) = (%v, %v), want (10, true)", got, ok)
	}
}

// TestSetColWidthCarvesEveryColsGroup pins the already-fixed twin so the
// unified helper does not regress C127.
func TestSetColWidthCarvesEveryColsGroup(t *testing.T) {
	src := buildMultiColsPackage(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	if err := sh.SetColWidth(6, 33); err != nil {
		t.Fatalf("SetColWidth: %v", err)
	}
	covering := colEntriesCovering(sh, 6)
	if len(covering) != 1 {
		t.Fatalf("column 6 is covered by %d <col> entries (%v), want exactly 1", len(covering), covering)
	}
	if !strings.Contains(covering[0], "width=33") {
		t.Errorf("carved entry = %s, want width 33", covering[0])
	}
}

// ---------------------------------------------------------------------------
// C424 — DeleteSheet leaves references to the deleted sheet name behind
// ---------------------------------------------------------------------------

func buildTwoSheetRefPackage(t *testing.T) []byte {
	t.Helper()
	return buildCorePackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels": coreRootRels,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/><sheet name="Report" sheetId="2" r:id="rId2"/></sheets><definedNames><definedName name="Total">Data!$A$1</definedName><definedName name="Kept">Report!$B$2</definedName></definedNames></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>5</v></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><f>Data!A1+1</f><v>6</v></c></row><row r="2"><c r="B2"><f>SUM(Report!A1:A2)</f><v>0</v></c></row></sheetData></worksheet>`,
	})
}

// TestDeleteSheetRewritesReferences guards C424: after a sheet is deleted,
// defined-name values and formulas that named it must be rewritten to #REF!
// (Excel's own behavior) rather than left dangling.
func TestDeleteSheetRewritesReferences(t *testing.T) {
	src := buildTwoSheetRefPackage(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if err := wb.DeleteSheet(0); err != nil {
		t.Fatalf("DeleteSheet: %v", err)
	}
	names := wb.DefinedNames()
	byName := map[string]string{}
	for _, dn := range names {
		byName[dn.Name] = dn.Value
	}
	if got := byName["Total"]; got != "#REF!$A$1" {
		t.Errorf("defined name Total = %q, want %q", got, "#REF!$A$1")
	}
	if got := byName["Kept"]; got != "Report!$B$2" {
		t.Errorf("defined name Kept = %q (must be untouched)", got)
	}

	report, err := wb.SheetByName("Report")
	if err != nil {
		t.Fatalf("SheetByName: %v", err)
	}
	a1 := report.findCell("A1")
	if a1 == nil {
		t.Fatal("Report!A1 missing")
	}
	if got := a1.Formula(); got != "#REF!A1+1" {
		t.Errorf("Report!A1 formula = %q, want %q", got, "#REF!A1+1")
	}
	b2 := report.findCell("B2")
	if b2 == nil {
		t.Fatal("Report!B2 missing")
	}
	if got := b2.Formula(); got != "SUM(Report!A1:A2)" {
		t.Errorf("Report!B2 formula = %q (self-reference must be untouched)", got)
	}
}

// TestDeleteSheetLeavesUnrelatedSheetsClean pins that a delete with no
// references to the removed sheet does not dirty (and therefore rewrite) the
// surviving worksheet parts.
func TestDeleteSheetLeavesUnrelatedSheetsClean(t *testing.T) {
	src := buildChartsheetWorkbook(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if err := wb.DeleteSheet(1); err != nil {
		t.Fatalf("DeleteSheet: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	const want = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>hello</t></is></c></row></sheetData></worksheet>`
	if got := string(readZipPart(t, out, "xl/worksheets/sheet1.xml")); got != want {
		t.Errorf("untouched worksheet was rewritten:\n got: %s\nwant: %s", got, want)
	}
}

// TestRewriteDeletedSheetRefs table-drives the reference rewriter.
func TestRewriteDeletedSheetRefs(t *testing.T) {
	cases := []struct {
		expr, sheet, want string
	}{
		{"Data!$A$1", "Data", "#REF!$A$1"},
		{"Data!A1+Report!A1", "Data", "#REF!A1+Report!A1"},
		{"'My Sheet'!$A$1", "My Sheet", "#REF!$A$1"},
		{"SUM('My Sheet'!A1:B2)", "My Sheet", "SUM(#REF!A1:B2)"},
		{"MyData!A1", "Data", "MyData!A1"},
		{"DataX!A1", "Data", "DataX!A1"},
		{`"Data!A1"`, "Data", `"Data!A1"`},
		{"data!a1", "Data", "#REF!a1"},
		{"Report!A1", "Data", "Report!A1"},
		{"A1", "Data", "A1"},
		{"", "Data", ""},
		{"'It''s'!A1", "It's", "#REF!A1"},
	}
	for _, tc := range cases {
		if got := rewriteDeletedSheetRefs(tc.expr, tc.sheet); got != tc.want {
			t.Errorf("rewriteDeletedSheetRefs(%q, %q) = %q, want %q", tc.expr, tc.sheet, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// C425 — Cell() is a mutating accessor that leaves phantom cells behind
// ---------------------------------------------------------------------------

// TestPhantomCellsNotSerialized guards C425: probing an absent cell with Cell()
// must not add an empty <c>/<row> to the saved sheet nor inflate <dimension>.
func TestPhantomCellsNotSerialized(t *testing.T) {
	src := buildDatePackage(t, "", "1")
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	if _, err := sh.Cell("Z999"); err != nil { // read-only probe
		t.Fatalf("Cell: %v", err)
	}
	if err := sh.SetCellValue("A1", 7); err != nil { // unrelated edit dirties the sheet
		t.Fatalf("SetCellValue: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheetXML := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if strings.Contains(sheetXML, `r="Z999"`) || strings.Contains(sheetXML, `r="999"`) {
		t.Errorf("phantom cell serialized:\n%s", sheetXML)
	}

	// A styled-but-valueless cell is real content and must survive.
	c, err := sh.Cell("C3")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	c.SetStyleIndex(0)
	out, err = wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheetXML = string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheetXML, `r="C3"`) {
		t.Errorf("styled empty cell was pruned:\n%s", sheetXML)
	}
}

// TestPhantomCellHandleSurvivesSave pins that pruning at save is a view, not a
// model mutation: a handle to a then-empty cell still addresses the model.
func TestPhantomCellHandleSurvivesSave(t *testing.T) {
	src := buildDatePackage(t, "", "1")
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	probe, err := sh.Cell("Z999")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	if err := sh.SetCellValue("A1", 7); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	if _, err := wb.SaveBytes(); err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	probe.SetString("late")
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheetXML := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheetXML, `r="Z999"`) || !strings.Contains(sheetXML, "late") {
		t.Errorf("write through a pre-save cell handle was lost:\n%s", sheetXML)
	}
}

// TestPhantomCellsNotCounted guards the C425 read-side symptom: a probe must
// not inflate Rows()/Cols().
func TestPhantomCellsNotCounted(t *testing.T) {
	src := buildDatePackage(t, "", "1")
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	rowsBefore, colsBefore := sh.Rows(), sh.Cols()
	if _, err := sh.Cell("Z999"); err != nil {
		t.Fatalf("Cell: %v", err)
	}
	if got := sh.Rows(); got != rowsBefore {
		t.Errorf("Rows() = %d after probing Z999, want %d", got, rowsBefore)
	}
	if got := sh.Cols(); got != colsBefore {
		t.Errorf("Cols() = %d after probing Z999, want %d", got, colsBefore)
	}
}

// TestFindCellIsReadOnly covers the public read-only accessor added for C425.
func TestFindCellIsReadOnly(t *testing.T) {
	src := buildDatePackage(t, "", "1")
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	if got := sh.FindCell("A1"); got == nil {
		t.Fatal("FindCell(A1) = nil, want the existing cell")
	}
	if got := sh.FindCell("Z999"); got != nil {
		t.Errorf("FindCell(Z999) = %v, want nil", got)
	}
	if got := sh.FindCell("not a ref"); got != nil {
		t.Errorf("FindCell(invalid) = %v, want nil", got)
	}
	if sh.Rows() != 1 {
		t.Errorf("FindCell mutated the model: Rows() = %d", sh.Rows())
	}
}

// ---------------------------------------------------------------------------
// C426 — defined-name legality and uniqueness never validated
// ---------------------------------------------------------------------------

func TestAddDefinedNameRejectsIllegalNames(t *testing.T) {
	cases := []struct{ name, why string }{
		{"A1", "collides with an A1-style cell reference"},
		{"a1", "collides with an A1-style cell reference"},
		{"XFD1048576", "collides with the last cell reference"},
		{"R1C1", "collides with an R1C1-style reference"},
		{"R", "reserved R1C1 shorthand"},
		{"c", "reserved R1C1 shorthand"},
		{"my name", "contains a space"},
		{"1name", "starts with a digit"},
		{"", "empty"},
		{"a-b", "contains an illegal character"},
		{strings.Repeat("n", 256), "longer than 255 characters"},
	}
	for _, tc := range cases {
		wb := Create()
		wb.AddSheet("Sheet1")
		if err := wb.AddDefinedName(tc.name, "Sheet1!$A$1"); err == nil {
			t.Errorf("AddDefinedName(%q) succeeded; want rejection (%s)", tc.name, tc.why)
		}
	}
}

func TestAddDefinedNameAcceptsLegalNames(t *testing.T) {
	for _, name := range []string{"Total", "_hidden", "a_b.c", "AA1B", "Sales2024", "\\odd", "ABCD"} {
		wb := Create()
		wb.AddSheet("Sheet1")
		if err := wb.AddDefinedName(name, "Sheet1!$A$1"); err != nil {
			t.Errorf("AddDefinedName(%q) = %v, want success", name, err)
		}
	}
}

func TestAddDefinedNameRejectsDuplicates(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")
	wb.AddSheet("Sheet2")
	if err := wb.AddDefinedName("Total", "Sheet1!$A$1"); err != nil {
		t.Fatalf("first AddDefinedName: %v", err)
	}
	if err := wb.AddDefinedName("Total", "Sheet1!$A$2"); err == nil {
		t.Error("duplicate workbook-scoped AddDefinedName succeeded; want rejection")
	}
	// Case-insensitive: Excel treats names case-insensitively.
	if err := wb.AddDefinedName("TOTAL", "Sheet1!$A$3"); err == nil {
		t.Error("case-variant duplicate AddDefinedName succeeded; want rejection")
	}
	// A different scope is a different name.
	if err := wb.AddDefinedNameScoped("Total", "Sheet1!$A$1", 0); err != nil {
		t.Errorf("sheet-scoped Total = %v, want success", err)
	}
	if err := wb.AddDefinedNameScoped("Total", "Sheet1!$A$1", 0); err == nil {
		t.Error("duplicate sheet-scoped AddDefinedName succeeded; want rejection")
	}
	if err := wb.AddDefinedNameFull(DefinedName{Name: "Total", Value: "Sheet2!$A$1", SheetIndex: 1}); err != nil {
		t.Errorf("AddDefinedNameFull on a fresh scope = %v, want success", err)
	}
	if err := wb.AddDefinedNameFull(DefinedName{Name: "Total", Value: "Sheet2!$A$2", SheetIndex: 1}); err == nil {
		t.Error("duplicate AddDefinedNameFull succeeded; want rejection")
	}
}

// TestReservedDefinedNamesStillAllowed pins that the built-in _xlnm.* names the
// print-area API writes are not caught by the C426 syntax check.
func TestReservedDefinedNamesStillAllowed(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("Sheet1")
	if err := sh.SetPrintArea("A1:D20"); err != nil {
		t.Fatalf("SetPrintArea: %v", err)
	}
	if got := sh.PrintArea(); got == "" {
		t.Error("PrintArea() = \"\" after SetPrintArea")
	}
	if err := wb.AddDefinedName("_xlnm.Criteria", "Sheet1!$A$1"); err != nil {
		t.Errorf("AddDefinedName(_xlnm.Criteria) = %v, want success", err)
	}
}

// ---------------------------------------------------------------------------
// C544 — ApplyNamedStyle dirties styles before its dedup check, and can panic
// ---------------------------------------------------------------------------

// TestApplyNamedStyleReuseDoesNotDirty guards C544: a pure-reuse call must not
// regenerate styles.xml.
func TestApplyNamedStyleReuseDoesNotDirty(t *testing.T) {
	src := buildNamedStylePackage(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sm := wb.Styles()
	if _, err := sm.ApplyNamedStyle("Normal"); err != nil {
		t.Fatalf("ApplyNamedStyle: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	got := readZipPart(t, out, "xl/styles.xml")
	want := readZipPart(t, src, "xl/styles.xml")
	if !bytes.Equal(got, want) {
		t.Errorf("styles.xml regenerated by a reuse-only ApplyNamedStyle:\n got: %s\nwant: %s", got, want)
	}
}

// TestApplyNamedStyleMalformedCellStyleXfs guards C544's second leg: a
// cellStyles xfId pointing past (or lacking) cellStyleXfs must error, not panic.
func TestApplyNamedStyleMalformedCellStyleXfs(t *testing.T) {
	src := buildCorePackage(t, corePackageWithStyles(t, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs><cellStyles count="1"><cellStyle name="Bogus" xfId="9" builtinId="0"/></cellStyles></styleSheet>`))
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if _, err := wb.Styles().ApplyNamedStyle("Bogus"); err == nil {
		t.Error("ApplyNamedStyle on an out-of-range xfId succeeded; want an error")
	}
}

// buildNamedStylePackage returns a package with a Normal named style.
func buildNamedStylePackage(t *testing.T) []byte {
	t.Helper()
	return buildCorePackage(t, corePackageWithStyles(t, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`))
}

// corePackageWithStyles returns the part map of a one-sheet package carrying
// the given styles.xml body.
func corePackageWithStyles(t *testing.T, styles string) map[string]string {
	t.Helper()
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels": coreRootRels,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>1</v></c></row></sheetData></worksheet>`,
		"xl/styles.xml": styles,
	}
}

// ---------------------------------------------------------------------------
// C545 — SetName over-dirties the worksheet part
// ---------------------------------------------------------------------------

// TestSetNameKeepsSheetBytes guards C545: renaming a sheet must not rewrite its
// worksheet part, whether or not the model was materialized first.
func TestSetNameKeepsSheetBytes(t *testing.T) {
	src := buildCalcChainPackage(t)
	for _, readFirst := range []bool{false, true} {
		wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
		if err != nil {
			t.Fatalf("OpenReader: %v", err)
		}
		sh, _ := wb.Sheet(0)
		if readFirst {
			if _, err := sh.GetCellValue("A1"); err != nil {
				t.Fatalf("GetCellValue: %v", err)
			}
		}
		if err := sh.SetName("Renamed"); err != nil {
			t.Fatalf("SetName: %v", err)
		}
		out, err := wb.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes: %v", err)
		}
		got := readZipPart(t, out, "xl/worksheets/sheet1.xml")
		want := readZipPart(t, src, "xl/worksheets/sheet1.xml")
		if !bytes.Equal(got, want) {
			t.Errorf("readFirst=%v: worksheet part rewritten by SetName:\n got: %s\nwant: %s", readFirst, got, want)
		}
		if !zipHasPart(t, out, "xl/calcChain.xml") {
			t.Errorf("readFirst=%v: SetName dropped calcChain.xml", readFirst)
		}
		if !strings.Contains(string(readZipPart(t, out, "xl/workbook.xml")), `name="Renamed"`) {
			t.Errorf("readFirst=%v: rename did not reach workbook.xml", readFirst)
		}
	}
}

func buildCalcChainPackage(t *testing.T) []byte {
	t.Helper()
	return buildCorePackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/calcChain.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.calcChain+xml"/></Types>`,
		"_rels/.rels": coreRootRels,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/calcChain" Target="calcChain.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><f>1+1</f><v>2</v></c></row></sheetData></worksheet>`,
		"xl/calcChain.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<calcChain xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><c r="A1" i="1"/></calcChain>`,
	})
}

// ---------------------------------------------------------------------------
// C546 — SetRowHeight missing the grid bound its siblings enforce
// ---------------------------------------------------------------------------

func TestSetRowHeightRejectsOutOfGridRow(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("Sheet1")
	if err := sh.SetRowHeight(MaxRow+1, 20); !errors.Is(err, ErrInvalidCell) {
		t.Errorf("SetRowHeight(MaxRow+1) = %v, want ErrInvalidCell", err)
	}
	if err := sh.SetRowHeight(2_000_000, 20); !errors.Is(err, ErrInvalidCell) {
		t.Errorf("SetRowHeight(2000000) = %v, want ErrInvalidCell", err)
	}
	if err := sh.SetRowHeight(0, 20); !errors.Is(err, ErrInvalidCell) {
		t.Errorf("SetRowHeight(0) = %v, want ErrInvalidCell", err)
	}
	if err := sh.SetRowHeight(MaxRow, 20); err != nil {
		t.Errorf("SetRowHeight(MaxRow) = %v, want success", err)
	}
}

// ---------------------------------------------------------------------------
// C547 — ParseCellRef accepts a '+'-prefixed row
// ---------------------------------------------------------------------------

func TestParseCellRefRejectsSignedRows(t *testing.T) {
	for _, ref := range []string{"A+5", "A-5", "A 5", "A+0005", "A5 "} {
		if row, col, err := ParseCellRef(ref); err == nil {
			t.Errorf("ParseCellRef(%q) = (%d, %d, nil); want an error", ref, row, col)
		}
	}
	if row, col, err := ParseCellRef("A5"); err != nil || row != 5 || col != 1 {
		t.Errorf("ParseCellRef(\"A5\") = (%d, %d, %v), want (5, 1, nil)", row, col, err)
	}
	if row, col, err := ParseCellRef("A05"); err != nil || row != 5 || col != 1 {
		t.Errorf("ParseCellRef(\"A05\") = (%d, %d, %v), want (5, 1, nil)", row, col, err)
	}
}

// ---------------------------------------------------------------------------
// C548 — error cells read back as nil; t="d" cells misclassified
// ---------------------------------------------------------------------------

func TestErrorCellValue(t *testing.T) {
	src := buildTypedCellPackage(t, `<c r="A1" t="e"><v>#DIV/0!</v></c>`)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	c := sh.FindCell("A1")
	if c == nil {
		t.Fatal("A1 missing")
	}
	if c.Type() != CellTypeError {
		t.Fatalf("Type() = %v, want CellTypeError", c.Type())
	}
	v := c.Value()
	if v == nil {
		t.Fatal("Value() = nil for an error cell; it is indistinguishable from empty")
	}
	ce, ok := v.(CellError)
	if !ok {
		t.Fatalf("Value() = %T(%v), want CellError", v, v)
	}
	if string(ce) != "#DIV/0!" {
		t.Errorf("CellError = %q, want %q", ce, "#DIV/0!")
	}
	var asErr error = ce
	if asErr.Error() != "#DIV/0!" {
		t.Errorf("CellError.Error() = %q", asErr.Error())
	}
}

func TestErrorFormulaCellValue(t *testing.T) {
	src := buildTypedCellPackage(t, `<c r="A1" t="e"><f>1/0</f><v>#DIV/0!</v></c>`)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	c := sh.FindCell("A1")
	if c == nil {
		t.Fatal("A1 missing")
	}
	ce, ok := c.Value().(CellError)
	if !ok {
		t.Fatalf("Value() = %T(%v), want CellError", c.Value(), c.Value())
	}
	if string(ce) != "#DIV/0!" {
		t.Errorf("CellError = %q", ce)
	}
}

func TestISODateCellValue(t *testing.T) {
	src := buildTypedCellPackage(t, `<c r="A1" t="d"><v>2020-03-04T05:06:07</v></c><c r="B1" t="d"><v>2021-12-25</v></c>`)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, _ := wb.Sheet(0)
	a1 := sh.FindCell("A1")
	if a1 == nil {
		t.Fatal("A1 missing")
	}
	if a1.Type() != CellTypeDate {
		t.Fatalf("Type() = %v for t=\"d\", want CellTypeDate", a1.Type())
	}
	want := time.Date(2020, 3, 4, 5, 6, 7, 0, time.UTC)
	got, ok := a1.Value().(time.Time)
	if !ok {
		t.Fatalf("Value() = %T(%v), want time.Time", a1.Value(), a1.Value())
	}
	if !got.Equal(want) {
		t.Errorf("Value() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	b1 := sh.FindCell("B1")
	if b1 == nil {
		t.Fatal("B1 missing")
	}
	if got := b1.Time(); !got.Equal(time.Date(2021, 12, 25, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("B1.Time() = %s, want 2021-12-25", got.Format(time.RFC3339))
	}
	// The raw lexical form stays reachable.
	if got := a1.String(); got != "2020-03-04T05:06:07" {
		t.Errorf("String() = %q, want the raw ISO value", got)
	}
}

func buildTypedCellPackage(t *testing.T, cells string) []byte {
	t.Helper()
	return buildCorePackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels": coreRootRels,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1">` + cells + `</row></sheetData></worksheet>`,
	})
}

// ---------------------------------------------------------------------------
// C549 — CopySheetFrom rough edges
// ---------------------------------------------------------------------------

// TestCopySheetRangedColumnsStayRanged guards C549(a): a whole-sheet <col>
// range must not explode into 16384 single-column entries.
func TestCopySheetRangedColumnsStayRanged(t *testing.T) {
	srcWB := Create()
	src := srcWB.AddSheet("Wide")
	src.ensureWS()
	w := 12.5
	custom := true
	src.ws().Cols = append(src.ws().Cols, oxml.CT_Cols{Col: []oxml.CT_Col{{Min: 1, Max: 16384, Width: &w, CustomWidth: &custom}}})
	src.ws().EnsureChildOrder("cols")
	if err := src.SetCellValue("A1", "x"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}

	dstWB := Create()
	dstWB.AddSheet("Sheet1")
	dst, err := dstWB.CopySheetFrom(srcWB, "Wide")
	if err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	total := 0
	for gi := range dst.ws().Cols {
		total += len(dst.ws().Cols[gi].Col)
	}
	if total > 8 {
		t.Fatalf("copied %d <col> entries for one ranged source entry, want a handful", total)
	}
	if got, ok := dst.ColumnWidth(1); !ok || got != 12.5 {
		t.Errorf("ColumnWidth(1) = (%v, %v), want (12.5, true)", got, ok)
	}
	if got, ok := dst.ColumnWidth(16384); !ok || got != 12.5 {
		t.Errorf("ColumnWidth(16384) = (%v, %v), want (12.5, true)", got, ok)
	}
}

// TestCopySheetSingleCellMerge guards C549(b).
func TestCopySheetSingleCellMerge(t *testing.T) {
	srcWB := Create()
	src := srcWB.AddSheet("Src")
	if err := src.SetCellValue("A1", "x"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	src.ensureWS()
	src.ws().MergeCells = &oxml.CT_MergeCells{MergeCell: []oxml.CT_MergeCell{{Ref: "A1"}, {Ref: "B2:C3"}}}
	src.ws().EnsureChildOrder("mergeCells")

	dstWB := Create()
	dstWB.AddSheet("Sheet1")
	dst, err := dstWB.CopySheetFrom(srcWB, "Src")
	if err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	if dst.ws().MergeCells == nil {
		t.Fatal("no merges copied")
	}
	got := map[string]bool{}
	for _, mc := range dst.ws().MergeCells.MergeCell {
		got[mc.Ref] = true
	}
	// The single-cell merge survives; MergeCells normalizes it to the
	// degenerate "A1:A1" range form, which Excel reads identically.
	if !got["A1"] && !got["A1:A1"] {
		t.Errorf("single-cell merge dropped; copied merges = %v", got)
	}
	if !got["B2:C3"] {
		t.Errorf("ranged merge dropped; copied merges = %v", got)
	}
}

// TestCopySheetPreservesThemeColor guards C549(c): a theme-coloured font must
// keep its colour through the style remap.
func TestCopySheetPreservesThemeColor(t *testing.T) {
	srcWB := Create()
	src := srcWB.AddSheet("Src")
	if err := src.SetCellValue("A1", "x"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	idx := addThemeColorFontXf(srcWB.Styles())
	c, err := src.Cell("A1")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	c.SetStyleIndex(idx)

	dstWB := Create()
	dstWB.AddSheet("Sheet1")
	dst, err := dstWB.CopySheetFrom(srcWB, "Src")
	if err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	dc := dst.FindCell("A1")
	if dc == nil {
		t.Fatal("A1 not copied")
	}
	si := dc.StyleIndex()
	if si == nil {
		t.Fatal("copied cell has no style index")
	}
	if !xfHasThemeFont(dstWB.Styles(), *si) {
		t.Errorf("theme colour lost in the copied style (xf %d)", *si)
	}
}

// TestCopySheetErrorRollsBack guards C549(d): a mid-copy failure must not leave
// a half-populated sheet in the destination workbook.
func TestCopySheetErrorRollsBack(t *testing.T) {
	srcWB := Create()
	src := srcWB.AddSheet("Src")
	if err := src.SetCellValue("A1", "x"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	// Inject a cell whose reference lies outside the grid: dst.Cell rejects it.
	src.ensureWS()
	src.ws().SheetData.Row[0].C = append(src.ws().SheetData.Row[0].C, &oxml.CT_Cell{R: "ZZZZ1"})

	dstWB := Create()
	dstWB.AddSheet("Sheet1")
	before := dstWB.SheetCount()
	if _, err := dstWB.CopySheetFrom(srcWB, "Src"); err == nil {
		t.Fatal("CopySheetFrom succeeded on an out-of-grid cell reference; want an error")
	}
	if got := dstWB.SheetCount(); got != before {
		t.Errorf("SheetCount = %d after a failed copy, want %d (half-populated sheet left behind)", got, before)
	}
	if _, err := dstWB.SheetByName("Src"); err == nil {
		t.Error("the half-populated sheet is still reachable by name")
	}
}

// ---------------------------------------------------------------------------
// C550 — NewCellStyle emits a dangling custom numFmtId
// ---------------------------------------------------------------------------

func TestNewCellStyleRejectsUnregisteredNumFmtID(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")
	sm := wb.Styles()
	if _, err := sm.NewCellStyle(CellStyle{NumberFormatID: 200}); err == nil {
		t.Error("NewCellStyle with an unregistered custom numFmtId succeeded; want an error")
	}
	// Registering it first makes the same call legal.
	id := sm.AddNumberFormat(`0.000"kg"`)
	if _, err := sm.NewCellStyle(CellStyle{NumberFormatID: int(id)}); err != nil {
		t.Errorf("NewCellStyle with a registered custom numFmtId = %v, want success", err)
	}
	// Built-in ids stay legal without registration.
	if _, err := sm.NewCellStyle(CellStyle{NumberFormatID: NumberFormatDate}); err != nil {
		t.Errorf("NewCellStyle with a built-in numFmtId = %v, want success", err)
	}
}

// ---------------------------------------------------------------------------
// C14 docs — the opaque-sheet refusal surface must be coherent in core
// ---------------------------------------------------------------------------

func TestOpaqueSheetCoreMutatorsRefuse(t *testing.T) {
	src := buildChartsheetWorkbook(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.SheetByName("Chart")
	if err != nil {
		t.Fatalf("SheetByName: %v", err)
	}
	if _, err := sh.Cell("A1"); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("Cell = %v, want ErrNotWorksheet", err)
	}
	if _, err := sh.GetCellValue("A1"); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("GetCellValue = %v, want ErrNotWorksheet", err)
	}
	if err := sh.SetColWidth(1, 20); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("SetColWidth = %v, want ErrNotWorksheet", err)
	}
	if err := sh.SetRowHeight(1, 20); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("SetRowHeight = %v, want ErrNotWorksheet", err)
	}
	if err := sh.MergeCells("A1", "B2"); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("MergeCells = %v, want ErrNotWorksheet", err)
	}
	if err := sh.FreezePanes("B2"); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("FreezePanes = %v, want ErrNotWorksheet", err)
	}
	if err := sh.SetAutoFilter("A1:B2"); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("SetAutoFilter = %v, want ErrNotWorksheet", err)
	}
	if err := sh.AddDataValidation(DataValidation{Range: "A1", Type: "list"}); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("AddDataValidation = %v, want ErrNotWorksheet", err)
	}
	if err := sh.SetColumnHidden(1, true); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("SetColumnHidden = %v, want ErrNotWorksheet", err)
	}
	// The refusals must leave the chartsheet's bytes untouched.
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := string(readZipPart(t, out, "xl/chartsheets/sheet1.xml")); got != chartsheetPartXML {
		t.Errorf("chartsheet bytes changed:\n%s", got)
	}
}

// addThemeColorFontXf appends a cellXfs record whose font carries a theme
// colour (theme index 4 with a tint) rather than an explicit rgb, and returns
// its index. Theme and indexed colours are exactly what the CellStyle
// round-trip used to drop (C549c).
func addThemeColorFontXf(sm *StyleManager) uint32 {
	ss := sm.stylesheet
	theme := uint32(4)
	fontID := sm.findOrAddFont(oxml.CT_Font{
		Name:  &oxml.CT_FontName{Val: "Calibri"},
		Sz:    &oxml.CT_FontSize{Val: 11},
		Color: &oxml.CT_Color{Theme: &theme, Tint: &oxml.FloatLex{Val: -0.25}},
	})
	zero := uint32(0)
	yes := true
	if ss.CellXfs == nil {
		ss.CellXfs = &oxml.CT_CellXfs{}
	}
	ss.CellXfs.Xf = append(ss.CellXfs.Xf, oxml.CT_Xf{
		NumFmtId: &zero, FontId: &fontID, FillId: &zero, BorderId: &zero,
		XfId: &zero, ApplyFont: &yes,
	})
	count := uint32(len(ss.CellXfs.Xf))
	ss.CellXfs.Count = &count
	sm.markModified()
	return count - 1
}

// xfHasThemeFont reports whether the cellXfs record at idx resolves to a font
// whose colour is a theme colour.
func xfHasThemeFont(sm *StyleManager, idx uint32) bool {
	ss := sm.stylesheet
	if ss.CellXfs == nil || int(idx) >= len(ss.CellXfs.Xf) {
		return false
	}
	xf := &ss.CellXfs.Xf[idx]
	if xf.FontId == nil || ss.Fonts == nil || int(*xf.FontId) >= len(ss.Fonts.Font) {
		return false
	}
	f := &ss.Fonts.Font[*xf.FontId]
	return f.Color != nil && f.Color.Theme != nil && *f.Color.Theme == 4
}
