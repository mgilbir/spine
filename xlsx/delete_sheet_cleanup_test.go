package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

const deleteFixtureSheetXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`

// buildDeleteCleanupXlsx assembles a three-sheet workbook where sheet2 has
// its own .rels part, the workbook view marks the third sheet active, and
// defined names are scoped to each sheet.
func buildDeleteCleanupXlsx(t *testing.T) []byte {
	t.Helper()
	return buildFixtureXlsxParts(t, []struct{ name, data string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/worksheets/sheet3.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView activeTab="2"/></bookViews><sheets><sheet name="One" sheetId="1" r:id="rId1"/><sheet name="Two" sheetId="2" r:id="rId2"/><sheet name="Three" sheetId="3" r:id="rId3"/></sheets><definedNames><definedName name="Global">One!$A$1</definedName><definedName name="OnOne" localSheetId="0">One!$A$1</definedName><definedName name="OnTwo" localSheetId="1">Two!$A$1</definedName><definedName name="OnThree" localSheetId="2">Three!$A$1</definedName></definedNames></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet3.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", deleteFixtureSheetXML},
		{"xl/worksheets/sheet2.xml", deleteFixtureSheetXML},
		{"xl/worksheets/sheet3.xml", deleteFixtureSheetXML},
		{"xl/worksheets/_rels/sheet2.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/></Relationships>`},
	})
}

// C75: deleting a middle sheet must not leave its content-type override or
// its own .rels part behind, and position-indexed workbook state (activeTab,
// definedName localSheetId) must be adjusted.
func TestDeleteSheetCleansUpPackage(t *testing.T) {
	data := buildDeleteCleanupXlsx(t)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.DeleteSheet(1); err != nil { // delete "Two"
		t.Fatal(err)
	}
	saved, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	if zipHasPart(t, saved, "xl/worksheets/sheet2.xml") {
		t.Error("deleted sheet part still present")
	}
	if zipHasPart(t, saved, "xl/worksheets/_rels/sheet2.xml.rels") {
		t.Error("deleted sheet's .rels part still present")
	}
	ct := string(readZipPart(t, saved, "[Content_Types].xml"))
	if strings.Contains(ct, "/xl/worksheets/sheet2.xml") {
		t.Errorf("orphan content-type override for the deleted sheet:\n%s", ct)
	}
	wbRels := string(readZipPart(t, saved, "xl/_rels/workbook.xml.rels"))
	if strings.Contains(wbRels, "worksheets/sheet2.xml") {
		t.Errorf("orphan workbook relationship to the deleted sheet:\n%s", wbRels)
	}

	// activeTab pointed at "Three" (index 2); after the delete it is index 1.
	wbXML := string(readZipPart(t, saved, "xl/workbook.xml"))
	if !strings.Contains(wbXML, `activeTab="1"`) {
		t.Errorf("activeTab not shifted after delete:\n%s", wbXML)
	}

	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("saved workbook does not reopen: %v", err)
	}
	if got := reopened.ActiveSheet().Name(); got != "Three" {
		t.Errorf("active sheet after delete = %q, want Three", got)
	}

	names := reopened.DefinedNames()
	byName := map[string]DefinedName{}
	for _, dn := range names {
		byName[dn.Name] = dn
	}
	if _, ok := byName["OnTwo"]; ok {
		t.Error("defined name scoped to the deleted sheet survived")
	}
	if dn, ok := byName["OnOne"]; !ok || dn.SheetIndex != 0 {
		t.Errorf("OnOne = %+v, want localSheetId 0", byName["OnOne"])
	}
	if dn, ok := byName["OnThree"]; !ok || dn.SheetIndex != 1 {
		t.Errorf("OnThree = %+v, want localSheetId shifted to 1", byName["OnThree"])
	}
	if dn, ok := byName["Global"]; !ok || dn.SheetIndex != -1 {
		t.Errorf("Global = %+v, want workbook scope", byName["Global"])
	}
}

// C75: activeTab pointing AT the deleted last sheet must clamp into range.
func TestDeleteSheetClampsActiveTab(t *testing.T) {
	data := buildDeleteCleanupXlsx(t)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.DeleteSheet(2); err != nil { // delete "Three", the active one
		t.Fatal(err)
	}
	bv := wb.workbook.BookViews.WorkbookView[0]
	if bv.ActiveTab == nil || *bv.ActiveTab != 1 {
		t.Errorf("ActiveTab = %v, want 1 (clamped)", bv.ActiveTab)
	}
}
