package xlsx

// C564: a chart hosted on a chartsheet was invisible to Sheet.Charts and
// Workbook.Charts, though the docs promise "every chart across all of the
// workbook's sheets". A chartsheet is opaque — its part is preserved verbatim
// and never parsed as a worksheet — so the drawing reference has to be read
// from the raw part.

import (
	"archive/zip"
	"bytes"
	"testing"
)

// chartsheetWithChartParts is a minimal xlsx package holding one worksheet and
// one chartsheet whose drawing anchors a chart part.
func chartsheetWithChartParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/chartsheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.chartsheet+xml"/><Override PartName="/xl/drawings/drawing1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/><Override PartName="/xl/charts/chart1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/><sheet name="Chart" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chartsheet" Target="chartsheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>hello</t></is></c></row></sheetData></worksheet>`,
		"xl/chartsheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<chartsheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetViews><sheetView workbookViewId="0"/></sheetViews><pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/><drawing r:id="rId1"/></chartsheet>`,
		"xl/chartsheets/_rels/sheet1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/></Relationships>`,
		"xl/drawings/drawing1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><xdr:absoluteAnchor><xdr:pos x="0" y="0"/><xdr:ext cx="9144000" cy="6858000"/><xdr:graphicFrame macro=""><xdr:nvGraphicFramePr><xdr:cNvPr id="2" name="Chart 1"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr><xdr:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></xdr:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rId1"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:absoluteAnchor></xdr:wsDr>`,
		"xl/drawings/_rels/drawing1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/></Relationships>`,
		"xl/charts/chart1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><c:chart><c:title><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Sheet Chart</a:t></a:r></a:p></c:rich></c:tx></c:title><c:plotArea><c:layout/><c:barChart><c:barDir val="col"/><c:grouping val="clustered"/><c:ser><c:idx val="0"/><c:order val="0"/><c:tx><c:strRef><c:f>Data!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>North</c:v></c:pt></c:strCache></c:strRef></c:tx><c:cat><c:strRef><c:f>Data!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt></c:strCache></c:strRef></c:cat><c:val><c:numRef><c:f>Data!$B$2:$B$3</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser><c:axId val="1"/><c:axId val="2"/></c:barChart></c:plotArea></c:chart></c:chartSpace>`,
	}
}

func buildZip(t *testing.T, parts map[string]string) []byte {
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

// TestChartsheetChartsAreVisible pins C564: the chart anchored on a chartsheet
// must be reported by both Sheet.Charts and Workbook.Charts.
func TestChartsheetChartsAreVisible(t *testing.T) {
	src := buildZip(t, chartsheetWithChartParts())
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}

	cs, err := wb.SheetByName("Chart")
	if err != nil {
		t.Fatalf("SheetByName(Chart): %v", err)
	}
	charts := cs.Charts()
	if len(charts) != 1 {
		t.Fatalf("chartsheet Charts() = %d, want 1", len(charts))
	}
	if got := charts[0].Title(); got != "Sheet Chart" {
		t.Errorf("title = %q, want Sheet Chart", got)
	}
	if got := charts[0].SeriesList(); len(got) != 1 || got[0].Name != "North" {
		t.Errorf("series = %+v", got)
	}

	all := wb.Charts()
	if len(all) != 1 {
		t.Fatalf("Workbook.Charts() = %d, want 1", len(all))
	}
}

// TestChartsheetChartsFoundWithoutWorksheetModel is the same check with the
// accident removed. A chartsheet's chart is reachable through the worksheet
// model only because CT_Worksheet decodes <chartsheet> by local name; the sheet
// is documented as never parsed as a worksheet, so this forces the state that
// contract describes — no model — and requires the charts to still be found.
func TestChartsheetChartsFoundWithoutWorksheetModel(t *testing.T) {
	src := buildZip(t, chartsheetWithChartParts())
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	cs, err := wb.SheetByName("Chart")
	if err != nil {
		t.Fatalf("SheetByName(Chart): %v", err)
	}
	// As if the opaque part had never been decoded into a worksheet model.
	cs.wsModel = nil
	cs.wsParsed = true

	charts := cs.Charts()
	if len(charts) != 1 {
		t.Fatalf("chartsheet Charts() = %d, want 1 (the drawing reference must be read from the preserved part)", len(charts))
	}
	if got := charts[0].Title(); got != "Sheet Chart" {
		t.Errorf("title = %q, want Sheet Chart", got)
	}
}

// TestChartsheetChartReadDoesNotDirtySheet checks reading the chartsheet's
// charts leaves its preserved bytes untouched: an opaque sheet must never be
// parsed or regenerated as a worksheet.
func TestChartsheetChartReadDoesNotDirtySheet(t *testing.T) {
	parts := chartsheetWithChartParts()
	src := buildZip(t, parts)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if n := len(wb.Charts()); n != 1 {
		t.Fatalf("Charts() = %d, want 1", n)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := string(readZipPart(t, out, "xl/chartsheets/sheet1.xml")); got != parts["xl/chartsheets/sheet1.xml"] {
		t.Errorf("chartsheet bytes changed after reading its charts:\n got: %s\nwant: %s", got, parts["xl/chartsheets/sheet1.xml"])
	}
}
