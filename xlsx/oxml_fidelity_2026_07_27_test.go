package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Regression tests for the 2026-07-27 audit's xlsx-oxml findings (C368, C369,
// C429, C430, C431, C551–C556).

// ---------------------------------------------------------------------------
// C368 — implicit cell references
// ---------------------------------------------------------------------------

// A cell may legally omit r, taking the column after its predecessor. Keying
// such a cell to column 0 sorted it ahead of every explicit cell and emitted a
// schema-invalid r="", moving values into the wrong columns on a dirty save.
func TestRowKeepsImplicitCellRefsInPlace(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData><row r="1"><c r="A1"><v>11</v></c><c><v>22</v></c><c><v>33</v></c></row></sheetData>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	if strings.Contains(out, `r=""`) {
		t.Errorf("implicit cell ref emitted as schema-invalid r=\"\":\n%s", out)
	}
	want := `<row r="1"><c r="A1"><v>11</v></c><c><v>22</v></c><c><v>33</v></c></row>`
	if !strings.Contains(out, want) {
		t.Errorf("cell order/values changed on re-marshal:\ngot  %s\nwant containing %s", out, want)
	}
}

// The ordering fix must still put genuinely out-of-order explicit cells back
// into ascending column order, which OOXML requires.
func TestRowSortsExplicitCellsAscending(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData><row r="1"><c r="C1"><v>3</v></c><c r="A1"><v>1</v></c><c r="B1"><v>2</v></c></row></sheetData>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	want := `<c r="A1"><v>1</v></c><c r="B1"><v>2</v></c><c r="C1"><v>3</v></c>`
	if out := string(data); !strings.Contains(out, want) {
		t.Errorf("cells not sorted ascending:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// C369 — charset transcoding vs offset-based raw capture
// ---------------------------------------------------------------------------

// buildWorkbookPartXLSX crafts a minimal package with a caller-supplied
// workbook.xml, which xlsx always regenerates on save.
func buildWorkbookPartXLSX(t *testing.T, workbook []byte) []byte {
	t.Helper()

	ct := `<?xml version="1.0" encoding="utf-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml" /><Default Extension="xml" ContentType="application/xml" /><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml" /><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml" /></Types>`
	rootRels := `<?xml version="1.0" encoding="utf-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="/xl/workbook.xml" Id="rId1" /></Relationships>`
	wbRels := `<?xml version="1.0" encoding="utf-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="/xl/worksheets/sheet1.xml" Id="rId2" /></Relationships>`
	sheet := `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>1</v></c></row></sheetData></worksheet>`

	order := []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/worksheets/sheet1.xml"}
	files := map[string][]byte{
		"[Content_Types].xml":        []byte(ct),
		"_rels/.rels":                []byte(rootRels),
		"xl/workbook.xml":            workbook,
		"xl/_rels/workbook.xml.rels": []byte(wbRels),
		"xl/worksheets/sheet1.xml":   []byte(sheet),
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range order {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := fw.Write(files[name]); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// partOf returns the named part's bytes from a saved package.
func partOf(t *testing.T, pkg []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("zip read: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		var b bytes.Buffer
		_, readErr := b.ReadFrom(rc)
		if err := rc.Close(); err != nil {
			t.Fatalf("close %s: %v", name, err)
		}
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		return b.Bytes()
	}
	t.Fatalf("part %s not found", name)
	return nil
}

// A workbook.xml declaring a single-byte code page is transcoded to UTF-8 on
// read, so the decoder's input offsets index the transcoded stream. Slicing
// the original bytes at those offsets spliced misaligned fragments into the
// always-regenerated workbook.xml, producing XML that no longer parses.
func TestTranscodedWorkbookRegeneratesWellFormed(t *testing.T) {
	// 0xE9 is é in Windows-1252; each one costs a byte of offset skew.
	workbook := []byte("<?xml version=\"1.0\" encoding=\"windows-1252\"?>" +
		"<workbook xmlns=\"http://schemas.openxmlformats.org/spreadsheetml/2006/main\"" +
		" xmlns:r=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships\">" +
		"<fileVersion appName=\"Caf\xe9\xe9\xe9\xe9\xe9\xe9\xe9\xe9\xe9\xe9\"/>" +
		"<workbookProtection lockStructure=\"1\"/>" +
		"<sheets><sheet name=\"Sheet1\" sheetId=\"1\" r:id=\"rId2\"/></sheets>" +
		"</workbook>")
	fixture := buildWorkbookPartXLSX(t, workbook)

	wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	got := partOf(t, out, "xl/workbook.xml")
	var reparsed oxml.CT_Workbook
	if err := xmlb.Unmarshal(got, &reparsed); err != nil {
		t.Fatalf("regenerated workbook.xml is malformed: %v\n%s", err, got)
	}
	if n := len(reparsed.Sheets.Sheet); n != 1 {
		t.Errorf("regenerated workbook.xml has %d sheets, want 1:\n%s", n, got)
	}
	if reparsed.WorkbookProtection == nil {
		t.Errorf("regenerated workbook.xml lost workbookProtection:\n%s", got)
	}
	// The content is UTF-8 now, so the declaration must say so.
	if bytes.Contains(bytes.ToLower(got), []byte("windows-1252")) {
		t.Errorf("regenerated UTF-8 workbook.xml still declares windows-1252:\n%s", got)
	}
	if !bytes.Contains(got, []byte(`encoding="UTF-8"`)) {
		t.Errorf("regenerated workbook.xml lost its encoding declaration:\n%s", got)
	}
	// The transcoded text must survive as UTF-8, not as raw code-page bytes.
	if !bytes.Contains(got, []byte("Caféééééééééé")) {
		t.Errorf("regenerated workbook.xml lost the transcoded appName:\n%s", got)
	}
}

// A UTF-8 workbook keeps using offset-based raw capture: the gate must not
// disable verbatim preservation for ordinary files.
func TestUTF8WorkbookKeepsRawCapture(t *testing.T) {
	workbook := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<workbookProtection lockStructure="1" />` +
		`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId2"/></sheets>` +
		`</workbook>`)
	fixture := buildWorkbookPartXLSX(t, workbook)

	wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := partOf(t, out, "xl/workbook.xml"); !bytes.Equal(got, workbook) {
		t.Errorf("zero-mod save of a UTF-8 workbook is not byte-identical:\ngot  %s\nwant %s", got, workbook)
	}
}

// ---------------------------------------------------------------------------
// C429 — inline xmlns on workbookView
// ---------------------------------------------------------------------------

// A workbookView that declares its own extension namespace opened fine and
// then made every save fail, because the bespoke attribute capture skipped the
// declaration while still capturing the attribute that needed it.
func TestBookViewInlineNamespaceSaves(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<bookViews><workbookView xmlns:xr2="http://schemas.microsoft.com/office/spreadsheetml/2014/revision2" ` +
		`xr2:uid="{00000000-0000-0000-0000-000000000000}" windowWidth="12345"/></bookViews>` +
		`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xmlb.UnmarshalWithSource([]byte(src), &wb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		`xmlns:xr2="http://schemas.microsoft.com/office/spreadsheetml/2014/revision2"`,
		`xr2:uid="{00000000-0000-0000-0000-000000000000}"`,
		`windowWidth="12345"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("workbookView lost %s:\n%s", want, out)
		}
	}
}

// The captured list must also keep a producer's non-XSD attribute ordering
// (Apache POI writes workbookView attributes alphabetically).
func TestBookViewKeepsSourceAttributeOrder(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<bookViews><workbookView activeTab="1" windowHeight="500" windowWidth="1000"/></bookViews>` +
		`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xmlb.UnmarshalWithSource([]byte(src), &wb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	want := `<workbookView activeTab="1" windowHeight="500" windowWidth="1000"/>`
	if out := string(data); !strings.Contains(out, want) {
		t.Errorf("workbookView attribute order not preserved:\ngot  %s\nwant containing %s", out, want)
	}
}

// ---------------------------------------------------------------------------
// C430 — styles model gaps
// ---------------------------------------------------------------------------

func TestStylesPreserveBorderEdgesAndExtLst(t *testing.T) {
	const src = `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<borders count="1"><border><start style="thin"/><end style="thin"/>` +
		`<vertical style="dashed"/><horizontal style="dotted"/></border></borders>` +
		`<cellXfs count="1"><xf numFmtId="0"><extLst><ext uri="{XF}"><a/></ext></extLst></xf></cellXfs>` +
		`<cellStyles count="1"><cellStyle name="Normal" xfId="0"><extLst><ext uri="{CS}"><b/></ext></extLst></cellStyle></cellStyles>` +
		`<dxfs count="1"><dxf><border><vertical style="thin"/><horizontal style="thin"/></border>` +
		`<extLst><ext uri="{DXF}"><c/></ext></extLst></dxf></dxfs>` +
		`</styleSheet>`

	var ss oxml.CT_Stylesheet
	if err := xml.Unmarshal([]byte(src), &ss); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalStylesheetXML(&ss)
	if err != nil {
		t.Fatalf("marshalStylesheetXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		`<start style="thin"/>`, `<end style="thin"/>`,
		`<vertical style="dashed"/>`, `<horizontal style="dotted"/>`,
		`uri="{XF}"`, `uri="{CS}"`, `uri="{DXF}"`,
		`<dxf><border><vertical style="thin"/><horizontal style="thin"/></border>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("styles regeneration dropped %s:\n%s", want, out)
		}
	}
}

// Excel writes theme tints in E-notation; regenerating styles.xml must not
// reprint them in decimal form.
func TestStylesPreserveENotationTint(t *testing.T) {
	const src = `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<fonts count="1"><font><color theme="1" tint="-4.9989318521683403E-2"/></font></fonts>` +
		`</styleSheet>`

	var ss oxml.CT_Stylesheet
	if err := xml.Unmarshal([]byte(src), &ss); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalStylesheetXML(&ss)
	if err != nil {
		t.Fatalf("marshalStylesheetXML: %v", err)
	}
	if out := string(data); !strings.Contains(out, `tint="-4.9989318521683403E-2"`) {
		t.Errorf("E-notation tint reprinted in decimal form:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// C431 — worksheet model gaps
// ---------------------------------------------------------------------------

func TestWorksheetPreservesFilterSortAndValidationDetails(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetPr syncHorizontal="1" syncRef="B2"/>` +
		`<sheetViews><sheetView workbookViewId="0"/><extLst><ext uri="{SV}"><a/></ext></extLst></sheetViews>` +
		`<sheetData/>` +
		`<autoFilter ref="A1:B2"><extLst><ext uri="{AF}"><b/></ext></extLst></autoFilter>` +
		`<sortState ref="A1:B2"><sortCondition ref="A1:A2" dxfId="3" iconSet="3Arrows" iconId="1"/>` +
		`<extLst><ext uri="{SS}"><c/></ext></extLst></sortState>` +
		`<conditionalFormatting sqref="A1"><cfRule type="expression" priority="1">` +
		`<formula>TRUE</formula></cfRule><extLst><ext uri="{CF}"><d/></ext></extLst></conditionalFormatting>` +
		`<dataValidations count="1" xWindow="120" yWindow="240">` +
		`<dataValidation type="list" sqref="A1"><formula1>"a,b"</formula1></dataValidation></dataValidations>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		`syncRef="B2"`,
		`uri="{SV}"`, `uri="{AF}"`, `uri="{SS}"`, `uri="{CF}"`,
		`dxfId="3"`, `iconSet="3Arrows"`, `iconId="1"`,
		`xWindow="120"`, `yWindow="240"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("worksheet regeneration dropped %s:\n%s", want, out)
		}
	}
}

// The cell and row extension lists and the row's customFormat flag are the
// same class: modeled nowhere, so dropped by any sheet regeneration.
func TestWorksheetPreservesRowAndCellExtensions(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData><row r="1" s="3" customFormat="1">` +
		`<c r="A1"><v>1</v><extLst><ext uri="{CELL}"><a/></ext></extLst></c>` +
		`<extLst><ext uri="{ROW}"><b/></ext></extLst></row></sheetData>` +
		`<conditionalFormatting sqref="A1"><cfRule type="cellIs" priority="1" operator="greaterThan">` +
		`<formula>0</formula></cfRule></conditionalFormatting>` +
		`<extLst><ext uri="{WS}"><c/></ext></extLst></worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{`customFormat="1"`, `uri="{CELL}"`, `uri="{ROW}"`, `uri="{WS}"`} {
		if !strings.Contains(out, want) {
			t.Errorf("worksheet regeneration dropped %s:\n%s", want, out)
		}
	}
}

// The conditional-format value objects carry gte and their own extLst.
func TestCfvoPreservesGteAndExtLst(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/>` +
		`<conditionalFormatting sqref="A1:A9"><cfRule type="iconSet" priority="1"><iconSet>` +
		`<cfvo type="percent" val="0" gte="0"><extLst><ext uri="{CFVO}"><a/></ext></extLst></cfvo>` +
		`<cfvo type="percent" val="50"/></iconSet></cfRule></conditionalFormatting>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{`gte="0"`, `uri="{CFVO}"`} {
		if !strings.Contains(out, want) {
			t.Errorf("cfvo regeneration dropped %s:\n%s", want, out)
		}
	}
}

func TestPageSetupPreservesCustomPaperSize(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/><pageSetup paperHeight="297mm" paperWidth="210mm" orientation="portrait"/>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{`paperHeight="297mm"`, `paperWidth="210mm"`} {
		if !strings.Contains(out, want) {
			t.Errorf("pageSetup regeneration dropped %s:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// C551 — expanded-empty definedName
// ---------------------------------------------------------------------------

func TestEmptyDefinedNameKeepsExpandedForm(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>` +
		`<definedNames><definedName name="Empty"></definedName></definedNames>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xmlb.UnmarshalWithSource([]byte(src), &wb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	if out := string(data); !strings.Contains(out, `<definedName name="Empty"></definedName>`) {
		t.Errorf("expanded-empty definedName collapsed to self-closing:\n%s", out)
	}
}

// A source that self-closed it must stay self-closing.
func TestSelfClosedDefinedNameStaysSelfClosing(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>` +
		`<definedNames><definedName name="Empty"/></definedNames>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xmlb.UnmarshalWithSource([]byte(src), &wb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	if out := string(data); !strings.Contains(out, `<definedName name="Empty"/>`) {
		t.Errorf("self-closed definedName expanded:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// C552 — one attribute-parse policy
// ---------------------------------------------------------------------------

// An unparsable f/@si used to be fabricated as si="0", silently moving the
// formula into shared-formula group 0 alongside unrelated cells.
func TestUnparsableSharedFormulaIndexDoesNotFabricateGroupZero(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData>` +
		`<row r="1"><c r="A1"><f t="shared" ref="A1:A2" si="0">B1</f><v>1</v></c></row>` +
		`<row r="2"><c r="A2"><f t="shared" si="abc"/><v>2</v></c></row>` +
		`</sheetData></worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	bad := ws.SheetData.Row[1].C[0].F
	if bad.Si != nil {
		t.Errorf("si=\"abc\" parsed as %d, want unset", *bad.Si)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	if out := string(data); strings.Contains(out, `<f t="shared" si="0"/>`) {
		t.Errorf("unparsable si merged into shared-formula group 0:\n%s", out)
	}
}

// A non-numeric paperSize must not become paperSize="0" (a real paper size).
func TestUnparsablePaperSizeIsSkipped(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/><pageSetup paperSize="A4" orientation="portrait"/></worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ps := ws.PageSetup.PaperSize; ps != nil {
		t.Errorf("paperSize=\"A4\" parsed as %d, want unset", *ps)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	if out := string(data); strings.Contains(out, `paperSize="0"`) {
		t.Errorf("unparsable paperSize fabricated as 0:\n%s", out)
	}
}

// An unparsable sst count must not fail the whole Open.
func TestUnparsableSharedStringCountDoesNotFailOpen(t *testing.T) {
	const src = `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`count="lots" uniqueCount="2"><si><t>a</t></si><si><t>b</t></si></sst>`

	var sst oxml.CT_Sst
	if err := xmlb.Unmarshal([]byte(src), &sst); err != nil {
		t.Fatalf("unparsable sst count failed the parse: %v", err)
	}
	if sst.Count != nil {
		t.Errorf("count=\"lots\" parsed as %d, want unset", *sst.Count)
	}
	if sst.UniqueCount == nil || *sst.UniqueCount != 2 {
		t.Errorf("uniqueCount not parsed alongside the bad count: %v", sst.UniqueCount)
	}
	if len(sst.Si) != 2 {
		t.Errorf("got %d shared strings, want 2", len(sst.Si))
	}
}

// ---------------------------------------------------------------------------
// C553 — duplicated singleton children
// ---------------------------------------------------------------------------

func TestDuplicateWorksheetSingletonEmittedOnce(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/>` +
		`<mergeCells count="1"><mergeCell ref="A1:B1"/></mergeCells>` +
		`<mergeCells count="1"><mergeCell ref="C1:D1"/></mergeCells>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	if n := strings.Count(out, "<mergeCells "); n != 1 {
		t.Errorf("got %d mergeCells elements, want 1:\n%s", n, out)
	}
}

func TestDuplicateWorkbookSingletonEmittedOnce(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<calcPr calcId="1"/><calcPr calcId="2"/>` +
		`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xmlb.UnmarshalWithSource([]byte(src), &wb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	out := string(data)
	if n := strings.Count(out, "<calcPr "); n != 1 {
		t.Errorf("got %d calcPr elements, want 1:\n%s", n, out)
	}
}

// Repeatable children must still get one entry per occurrence.
func TestRepeatableWorksheetChildrenAllEmitted(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<cols><col min="1" max="1" width="5"/></cols>` +
		`<cols><col min="2" max="2" width="6"/></cols>` +
		`<sheetData/>` +
		`<conditionalFormatting sqref="A1"><cfRule type="expression" priority="1"><formula>1</formula></cfRule></conditionalFormatting>` +
		`<conditionalFormatting sqref="B1"><cfRule type="expression" priority="2"><formula>2</formula></cfRule></conditionalFormatting>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	if n := strings.Count(out, "<cols>"); n != 2 {
		t.Errorf("got %d cols elements, want 2:\n%s", n, out)
	}
	if n := strings.Count(out, "<conditionalFormatting "); n != 2 {
		t.Errorf("got %d conditionalFormatting elements, want 2:\n%s", n, out)
	}
}

// ---------------------------------------------------------------------------
// C555 — whitespace around a removed element
// ---------------------------------------------------------------------------

func TestUnprotectDoesNotDoubleWhitespace(t *testing.T) {
	workbook := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<workbook xmlns=\"http://schemas.openxmlformats.org/spreadsheetml/2006/main\"" +
		" xmlns:r=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships\">\n" +
		"  <workbookProtection lockStructure=\"1\"/>\n" +
		"  <sheets>\n    <sheet name=\"Sheet1\" sheetId=\"1\" r:id=\"rId2\"/>\n  </sheets>\n" +
		"</workbook>")
	fixture := buildWorkbookPartXLSX(t, workbook)

	wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	wb.Unprotect()
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	got := string(partOf(t, out, "xl/workbook.xml"))
	if strings.Contains(got, "workbookProtection") {
		t.Fatalf("Unprotect left the element in place:\n%s", got)
	}
	if strings.Contains(got, "\n  \n  ") {
		t.Errorf("removed element left its whitespace neighbours doubled:\n%q", got)
	}
	if !strings.Contains(got, ">\n  <sheets>") {
		t.Errorf("whitespace around the removed element is wrong:\n%q", got)
	}
}

// ---------------------------------------------------------------------------
// C556 — scenario user attribute
// ---------------------------------------------------------------------------

func TestAuthoredScenarioOmitsEmptyUser(t *testing.T) {
	sc := &oxml.CT_Scenarios{
		Dirty: true,
		Scenario: []oxml.CT_Scenario{{
			Name:       "Best case",
			InputCells: []oxml.CT_InputCells{{R: "A1", Val: "1"}},
		}},
	}
	b := xmlb.NewSpreadsheetMLBuilder()
	b.StartElement(nsSML, "worksheet")
	sc.MarshalToBuilder(b, nsSML, "scenarios")
	b.EndElement(nsSML, "worksheet")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if out := string(b.Bytes()); strings.Contains(out, `user=""`) {
		t.Errorf("scenario emitted an empty user attribute:\n%s", out)
	}
}
