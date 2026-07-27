package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

const chartsheetPartXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<chartsheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetViews><sheetView workbookViewId="0"/></sheetViews><pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/></chartsheet>`

// buildChartsheetWorkbook returns the bytes of a minimal, valid xlsx package
// containing one worksheet (Data) and one chartsheet (Chart).
func buildChartsheetWorkbook(t *testing.T) []byte {
	t.Helper()
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/chartsheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.chartsheet+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/><sheet name="Chart" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chartsheet" Target="chartsheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>hello</t></is></c></row></sheetData></worksheet>`,
		"xl/chartsheets/sheet1.xml": chartsheetPartXML,
	}

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

// TestChartsheetEditWorksheetPreservesChartsheet guards C241: editing a cell on a
// worksheet must not make the save emit a second, worksheet-typed relationship
// for the chartsheet's part, nor alter its content-type override or bytes.
func TestChartsheetEditWorksheetPreservesChartsheet(t *testing.T) {
	src := buildChartsheetWorkbook(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if wb.SheetCount() != 2 {
		t.Fatalf("SheetCount = %d, want 2", wb.SheetCount())
	}
	// The chartsheet still appears in the sheet list, in order.
	if s, _ := wb.Sheet(0); s.Name() != "Data" {
		t.Errorf("sheet 0 = %q, want Data", s.Name())
	}
	if s, _ := wb.Sheet(1); s.Name() != "Chart" {
		t.Errorf("sheet 1 = %q, want Chart", s.Name())
	}

	data, err := wb.SheetByName("Data")
	if err != nil {
		t.Fatalf("SheetByName(Data): %v", err)
	}
	if err := data.SetCellValue("A1", "world"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// Exactly one chartsheet relationship, and no worksheet-typed relationship
	// pointing at the chartsheet's part.
	rels, err := opc.UnmarshalRelationships(readZipPart(t, out, "xl/_rels/workbook.xml.rels"))
	if err != nil {
		t.Fatalf("UnmarshalRelationships: %v", err)
	}
	chartsheetRels := 0
	for _, rel := range rels {
		if rel.Target == "chartsheets/sheet1.xml" {
			if rel.Type != opc.RelTypeChartsheet {
				t.Errorf("relationship %q to chartsheet has type %q, want chartsheet", rel.ID, rel.Type)
			}
			chartsheetRels++
		}
	}
	if chartsheetRels != 1 {
		t.Fatalf("found %d relationships targeting the chartsheet, want exactly 1", chartsheetRels)
	}

	// Content-type override intact.
	ct := string(readZipPart(t, out, "[Content_Types].xml"))
	if !strings.Contains(ct, `PartName="/xl/chartsheets/sheet1.xml" ContentType="`+opc.ContentTypeChartsheet+`"`) {
		t.Errorf("chartsheet content-type override missing from [Content_Types].xml:\n%s", ct)
	}

	// Chartsheet part bytes unchanged.
	if got := string(readZipPart(t, out, "xl/chartsheets/sheet1.xml")); got != chartsheetPartXML {
		t.Errorf("chartsheet bytes changed:\n got: %s\nwant: %s", got, chartsheetPartXML)
	}
}

// TestChartsheetCellWriteRefused guards C241 test (b): a cell write on the
// chartsheet handle returns an error and does not replace its bytes with a
// <worksheet> root.
func TestChartsheetCellWriteRefused(t *testing.T) {
	src := buildChartsheetWorkbook(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	chart, err := wb.SheetByName("Chart")
	if err != nil {
		t.Fatalf("SheetByName(Chart): %v", err)
	}

	if _, err := chart.Cell("A1"); !errors.Is(err, ErrNotWorksheet) {
		t.Fatalf("Cell on chartsheet: err = %v, want ErrNotWorksheet", err)
	}
	if err := chart.SetCellValue("A1", "x"); !errors.Is(err, ErrNotWorksheet) {
		t.Fatalf("SetCellValue on chartsheet: err = %v, want ErrNotWorksheet", err)
	}

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	got := string(readZipPart(t, out, "xl/chartsheets/sheet1.xml"))
	if strings.Contains(got, "<worksheet") {
		t.Fatalf("chartsheet was overwritten with a worksheet root:\n%s", got)
	}
	if got != chartsheetPartXML {
		t.Errorf("chartsheet bytes changed:\n got: %s\nwant: %s", got, chartsheetPartXML)
	}
}
