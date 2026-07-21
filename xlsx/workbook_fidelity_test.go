package xlsx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

// Fixtures for workbook-level fidelity tests (C12, C198, C199). Unlike
// buildMutatorTestXlsx, this builder accepts extra parts, content-type
// overrides and workbook relationships (calcChain, styles, ...).

// buildFidelityTestXlsx assembles a one-sheet workbook plus the given extra
// parts. extraParts maps zip entry names (no leading slash) to their bytes;
// extraOverrides and extraWbRels are appended verbatim to [Content_Types].xml
// and xl/_rels/workbook.xml.rels respectively.
func buildFidelityTestXlsx(t *testing.T, sheetXML string, extraParts map[string]string, extraOverrides, extraWbRels string) []byte {
	t.Helper()

	files := []struct{ name, data string }{
		{"[Content_Types].xml", fmt.Sprintf(mutatorTestContentTypesFmt,
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.ws()+xml"/>`+extraOverrides)},
		{"_rels/.rels", mutatorTestRootRels},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			extraWbRels + `</Relationships>`},
		{"xl/worksheets/sheet1.xml", sheetXML},
	}
	for name, data := range extraParts {
		files = append(files, struct{ name, data string }{name, data})
	}

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

// zipHasPart reports whether the zipped xlsx contains the named entry.
func zipHasPart(t *testing.T, data []byte, name string) bool {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// C12: workbook-level mutators on an opened workbook lacking the element
// ---------------------------------------------------------------------------

// TestWorkbookMutatorsInsertMissingChildren is the P3 scenario: on an opened
// workbook whose workbook.xml has only <sheets>, AddDefinedName and
// SetActiveSheet must emit definedNames/bookViews once, in schema order, and
// the result must survive a reopen.
func TestWorkbookMutatorsInsertMissingChildren(t *testing.T) {
	data := buildMutatorTestXlsx(t, mutatorTestSheetBare)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := wb.AddDefinedName("MyName", "Sheet1!$A$1"); err != nil {
		t.Fatalf("AddDefinedName: %v", err)
	}
	if err := wb.SetActiveSheet(0); err != nil {
		t.Fatalf("SetActiveSheet: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	wbXML := string(readZipPart(t, out, "xl/workbook.xml"))

	// Present exactly once, in schema order: bookViews < sheets < definedNames.
	prev := -1
	for _, tag := range []string{"<bookViews>", "<sheets>", "<definedNames>"} {
		i := strings.Index(wbXML, tag)
		if i < 0 {
			t.Fatalf("workbook.xml is missing %s:\n%s", tag, wbXML)
		}
		if strings.Count(wbXML, tag) != 1 {
			t.Errorf("workbook.xml has duplicate %s:\n%s", tag, wbXML)
		}
		if i < prev {
			t.Errorf("element %s out of schema order:\n%s", tag, wbXML)
		}
		prev = i
	}

	// The mutations must survive a reopen.
	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	names := reopened.DefinedNames()
	if len(names) != 1 || names[0].Name != "MyName" || names[0].Value != "Sheet1!$A$1" {
		t.Errorf("defined names after reopen = %+v", names)
	}
	if got := reopened.ActiveSheet(); got == nil || got.Name() != "Sheet1" {
		t.Errorf("active sheet after reopen = %v", got)
	}
}

// TestWorkbookMutatorsMultiCycle runs mutate→save→reopen→mutate→save and
// verifies elements stay single and schema-ordered across cycles.
func TestWorkbookMutatorsMultiCycle(t *testing.T) {
	data := buildMutatorTestXlsx(t, mutatorTestSheetBare)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := wb.AddDefinedName("First", "Sheet1!$A$1"); err != nil {
		t.Fatalf("AddDefinedName: %v", err)
	}
	out1, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save 1: %v", err)
	}

	wb2, err := OpenReader(bytes.NewReader(out1), int64(len(out1)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if err := wb2.AddDefinedName("Second", "Sheet1!$B$1"); err != nil {
		t.Fatalf("AddDefinedName 2: %v", err)
	}
	if err := wb2.SetActiveSheet(0); err != nil {
		t.Fatalf("SetActiveSheet: %v", err)
	}
	out2, err := wb2.SaveBytes()
	if err != nil {
		t.Fatalf("save 2: %v", err)
	}
	wbXML := string(readZipPart(t, out2, "xl/workbook.xml"))

	for _, tag := range []string{"<bookViews>", "<sheets>", "<definedNames>"} {
		if strings.Count(wbXML, tag) != 1 {
			t.Errorf("after two cycles, %s count = %d, want 1:\n%s", tag, strings.Count(wbXML, tag), wbXML)
		}
	}
	bv, sh, dn := strings.Index(wbXML, "<bookViews>"), strings.Index(wbXML, "<sheets>"), strings.Index(wbXML, "<definedNames>")
	if bv > sh || sh > dn {
		t.Errorf("children not in schema order after two cycles:\n%s", wbXML)
	}
	for _, want := range []string{`name="First"`, `name="Second"`} {
		if !strings.Contains(wbXML, want) {
			t.Errorf("workbook.xml is missing %s:\n%s", want, wbXML)
		}
	}
}

// TestWorkbookMutatorWithExistingElementNoDuplicate verifies the ranked
// insertion is a no-op when the element already exists in the original file.
func TestWorkbookMutatorWithExistingElementNoDuplicate(t *testing.T) {
	const wbWithViews = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView tabRatio="500"/></bookViews><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`

	data := buildFidelityTestXlsx(t, mutatorTestSheetBare, nil, "", "")
	data = replaceZipEntry(t, data, "xl/workbook.xml", wbWithViews)

	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := wb.SetActiveSheet(0); err != nil {
		t.Fatalf("SetActiveSheet: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	wbXML := string(readZipPart(t, out, "xl/workbook.xml"))

	if n := strings.Count(wbXML, "<bookViews>"); n != 1 {
		t.Errorf("bookViews count = %d, want 1:\n%s", n, wbXML)
	}
	for _, want := range []string{`tabRatio="500"`, `activeTab="0"`} {
		if !strings.Contains(wbXML, want) {
			t.Errorf("workbook.xml is missing %s:\n%s", want, wbXML)
		}
	}
}

// replaceZipEntry rewrites one entry of a zipped xlsx.
func replaceZipEntry(t *testing.T, data []byte, name, newData string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if f.Name == name {
			if _, err := w.Write([]byte(newData)); err != nil {
				t.Fatalf("write %s: %v", f.Name, err)
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		if _, err := io.Copy(w, rc); err != nil {
			t.Fatalf("copy %s: %v", f.Name, err)
		}
		if err := rc.Close(); err != nil {
			t.Fatalf("close %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// C198: calcChain must be dropped when sheet data changed
// ---------------------------------------------------------------------------

const calcChainFixtureSheet = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
	`<row r="1"><c r="A1"><v>1</v></c><c r="B1"><f>A1*2</f><v>2</v></c></row>` +
	`</sheetData></worksheet>`

const calcChainFixturePart = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<calcChain xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><c r="B1" i="1"/></calcChain>`

const calcChainFixtureOverride = `<Override PartName="/xl/calcChain.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.calcChain+xml"/>`

const calcChainFixtureRel = `<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/calcChain" Target="calcChain.xml"/>`

func buildCalcChainFixture(t *testing.T) []byte {
	t.Helper()
	return buildFidelityTestXlsx(t, calcChainFixtureSheet,
		map[string]string{"xl/calcChain.xml": calcChainFixturePart},
		calcChainFixtureOverride, calcChainFixtureRel)
}

// TestDirtySheetDropsCalcChain is the P16 scenario: removing the only formula
// must drop calcChain.xml, its content-type override and its workbook
// relationship, so Excel rebuilds the chain instead of choking on a stale one.
func TestDirtySheetDropsCalcChain(t *testing.T) {
	data := buildCalcChainFixture(t)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := wb.Sheets()[0].SetCellValue("B1", "plain"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if zipHasPart(t, out, "xl/calcChain.xml") {
		t.Error("stale xl/calcChain.xml still present after formula removal")
	}
	ct := string(readZipPart(t, out, "[Content_Types].xml"))
	if strings.Contains(ct, "calcChain") {
		t.Errorf("dangling calcChain content-type override:\n%s", ct)
	}
	rels := string(readZipPart(t, out, "xl/_rels/workbook.xml.rels"))
	if strings.Contains(rels, "calcChain") {
		t.Errorf("dangling calcChain workbook relationship:\n%s", rels)
	}

	// Multi-save: a second save must stay consistent.
	out2, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if zipHasPart(t, out2, "xl/calcChain.xml") {
		t.Error("calcChain reappeared on second save")
	}
}

// TestUntouchedWorkbookKeepsCalcChain verifies the zero-modification round
// trip keeps calcChain (and its override and relationship) byte-identical.
func TestUntouchedWorkbookKeepsCalcChain(t *testing.T) {
	data := buildCalcChainFixture(t)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if got := readZipPart(t, out, "xl/calcChain.xml"); !bytes.Equal(got, []byte(calcChainFixturePart)) {
		t.Errorf("untouched calcChain.xml changed:\nwant: %s\ngot:  %s", calcChainFixturePart, got)
	}
	ct := string(readZipPart(t, out, "[Content_Types].xml"))
	if !strings.Contains(ct, calcChainFixtureOverride) {
		t.Errorf("calcChain content-type override lost on untouched save:\n%s", ct)
	}
	rels := string(readZipPart(t, out, "xl/_rels/workbook.xml.rels"))
	if !strings.Contains(rels, "calcChain.xml") {
		t.Errorf("calcChain relationship lost on untouched save:\n%s", rels)
	}
}

// TestStylesOnlyDirtyKeepsCalcChain: dirtying styles without touching any
// sheet data must not drop the calculation chain.
func TestStylesOnlyDirtyKeepsCalcChain(t *testing.T) {
	data := buildCalcChainFixture(t)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Force the styles-dirty path without touching sheet data.
	if _, err := wb.Styles().NewCellStyle(CellStyle{Font: &FontStyle{Bold: true}}); err != nil {
		t.Fatalf("NewCellStyle: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !zipHasPart(t, out, "xl/calcChain.xml") {
		t.Error("calcChain dropped by a styles-only change")
	}
}

// ---------------------------------------------------------------------------
// C199: styles.xml regeneration must keep mc:Ignorable and x14ac:knownFonts
// ---------------------------------------------------------------------------

const stylesFixture = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="x14ac x16r2" xmlns:x14ac="http://schemas.microsoft.com/office/spreadsheetml/2009/9/ac" xmlns:x16r2="http://schemas.microsoft.com/office/spreadsheetml/2015/02/main"><fonts count="1" x14ac:knownFonts="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs></styleSheet>`

const stylesFixtureOverride = `<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`

const stylesFixtureRel = `<Relationship Id="rId8" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`

// TestDirtyStylesKeepRootAttrsAndKnownFonts is the P8 scenario: after one
// cell style dirties the stylesheet, the regenerated styles.xml must keep
// mc:Ignorable and emit knownFonts with the x14ac prefix.
func TestDirtyStylesKeepRootAttrsAndKnownFonts(t *testing.T) {
	data := buildFidelityTestXlsx(t, mutatorTestSheetBare,
		map[string]string{"xl/styles.xml": stylesFixture},
		stylesFixtureOverride, stylesFixtureRel)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cell, err := wb.Sheets()[0].Cell("A1")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	if err := cell.SetStyle(CellStyle{Font: &FontStyle{Bold: true}}); err != nil {
		t.Fatalf("SetStyle: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	styles := string(readZipPart(t, out, "xl/styles.xml"))

	if !strings.Contains(styles, `mc:Ignorable="x14ac x16r2"`) {
		t.Errorf("mc:Ignorable dropped from regenerated styles.xml:\n%s", styles)
	}
	if !strings.Contains(styles, `x14ac:knownFonts="1"`) {
		t.Errorf("x14ac:knownFonts not emitted with its prefix:\n%s", styles)
	}
	if strings.Contains(styles, ` knownFonts=`) {
		t.Errorf("bare knownFonts attribute emitted (not a legal sml attribute):\n%s", styles)
	}
	// The xmlns declarations must survive in their original order too.
	if !strings.Contains(styles, `xmlns:x14ac="http://schemas.microsoft.com/office/spreadsheetml/2009/9/ac"`) {
		t.Errorf("x14ac namespace declaration dropped:\n%s", styles)
	}
}

// TestUntouchedStylesStayByteIdentical guards the other side of C199: opening
// and saving without touching styles must not perturb styles.xml at all.
func TestUntouchedStylesStayByteIdentical(t *testing.T) {
	data := buildFidelityTestXlsx(t, mutatorTestSheetBare,
		map[string]string{"xl/styles.xml": stylesFixture},
		stylesFixtureOverride, stylesFixtureRel)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := readZipPart(t, out, "xl/styles.xml"); !bytes.Equal(got, []byte(stylesFixture)) {
		t.Errorf("untouched styles.xml changed:\nwant: %s\ngot:  %s", stylesFixture, got)
	}
}

// The reserved xml: prefix is never declared, so prefix resolution against
// the root xmlns declarations cannot find it. A workbook root carrying
// xml:space="preserve" was re-emitted as the invalid bare space="preserve";
// the same held for xml:-prefixed attributes on unknown preserved children.
func TestXMLSpaceAttrKeepsReservedPrefix(t *testing.T) {
	const wbWithXMLSpace = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<workbook xml:space="preserve" xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets><oleSize xml:space="preserve" ref="A1:B2"/></workbook>`

	data := buildFidelityTestXlsx(t, mutatorTestSheetBare, nil, "", "")
	data = replaceZipEntry(t, data, "xl/workbook.xml", wbWithXMLSpace)

	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := string(readZipPart(t, out, "xl/workbook.xml"))
	if !strings.Contains(got, `<workbook xml:space="preserve"`) {
		t.Errorf("workbook root lost the xml: prefix:\n%s", got)
	}
	if !strings.Contains(got, `<oleSize xml:space="preserve" ref="A1:B2"/>`) {
		t.Errorf("preserved unknown child lost the xml: prefix:\n%s", got)
	}
	if strings.Contains(got, ` space="preserve"`) {
		t.Errorf("invalid unprefixed space attribute emitted:\n%s", got)
	}
}
