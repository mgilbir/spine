package chart_test

// The advertised workflow is "read a chart with Charts(), re-embed it with
// AddChart". These tests run it end to end for a chart with a blank cell — the
// shape Excel writes when a value is missing — through both host models: the
// xlsx host worksheet and the pptx embedded workbook. Before the blank sentinel
// was handled on the write paths, the first produced a #NUM! error cell in the
// data sheet and the second an invalid <v>NaN</v> in the embedded workbook,
// with <c:v>NaN</c:v> in the chart cache either way (C384).

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// excelChartWithBlank is the chart.xml Excel writes for a three-category column
// chart whose middle cell is empty: the numCache declares ptCount=3 but carries
// c:pt only at idx 0 and 2.
const excelChartWithBlank = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <c:chart>
    <c:plotArea>
      <c:layout/>
      <c:barChart>
        <c:barDir val="col"/>
        <c:grouping val="clustered"/>
        <c:ser>
          <c:idx val="0"/>
          <c:order val="0"/>
          <c:tx><c:strRef><c:f>Sheet1!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>North</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:cat><c:strRef><c:f>Sheet1!$A$2:$A$4</c:f><c:strCache><c:ptCount val="3"/><c:pt idx="0"><c:v>a</c:v></c:pt><c:pt idx="1"><c:v>b</c:v></c:pt><c:pt idx="2"><c:v>c</c:v></c:pt></c:strCache></c:strRef></c:cat>
          <c:val><c:numRef><c:f>Sheet1!$B$2:$B$4</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="3"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="2"><c:v>30</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
        <c:axId val="1"/>
        <c:axId val="2"/>
      </c:barChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

// zipPart returns the bytes of one part of a zip archive.
func zipPart(t *testing.T, archive []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		var buf bytes.Buffer
		_, readErr := buf.ReadFrom(rc)
		if closeErr := rc.Close(); closeErr != nil {
			t.Fatalf("close %s: %v", name, closeErr)
		}
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		return buf.Bytes()
	}
	t.Fatalf("part %q not found in archive", name)
	return nil
}

// zipNames lists the part names of a zip archive.
func zipNames(t *testing.T, archive []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

// TestBlankChartReembedIntoWorkbook runs parse → AddChart → save → reopen for an
// Excel-authored chart with one blank point, and checks the emitted chart part
// carries no NaN and the host data sheet leaves the blank cell empty rather
// than filling it with a #NUM! error.
func TestBlankChartReembedIntoWorkbook(t *testing.T) {
	c, err := chart.Parse([]byte(excelChartWithBlank))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if vals := c.SeriesList()[0].Values; len(vals) != 3 || !chart.IsBlank(vals[1]) {
		t.Fatalf("parsed values = %v, want a blank at index 1", vals)
	}

	wb := xlsx.Create()
	sheet := addSheetT(wb, "Host")
	if err := sheet.AddChart(c, "E2"); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	chartXML := zipPart(t, data, "xl/charts/chart1.xml")
	if bytes.Contains(chartXML, []byte("NaN")) {
		t.Errorf("emitted chart.xml carries NaN:\n%s", chartXML)
	}

	wb2, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	ds, err := wb2.SheetByName("ChartData1")
	if err != nil {
		t.Fatalf("SheetByName(ChartData1): %v", err)
	}
	for ref, want := range map[string]string{"B2": "10", "B3": "", "B4": "30"} {
		got, err := ds.GetCellValue(ref)
		if err != nil {
			t.Fatalf("GetCellValue(%s): %v", ref, err)
		}
		if got != want {
			t.Errorf("data sheet %s = %q, want %q", ref, got, want)
		}
	}

	// And the chart still reads back with the blank in place.
	s2, err := wb2.SheetByName("Host")
	if err != nil {
		t.Fatalf("SheetByName(Host): %v", err)
	}
	charts := s2.Charts()
	if len(charts) != 1 {
		t.Fatalf("Charts() = %d, want 1", len(charts))
	}
	vals := charts[0].SeriesList()[0].Values
	if len(vals) != 3 || vals[0] != 10 || !chart.IsBlank(vals[1]) || vals[2] != 30 {
		t.Errorf("re-read values = %v, want [10 blank 30]", vals)
	}
}

// TestBlankChartReembedIntoPresentation runs the same loop through pptx, whose
// chart carries its data in an embedded workbook: that workbook must be a valid
// .xlsx (opened here with the library's own reader) with the blank cell empty.
func TestBlankChartReembedIntoPresentation(t *testing.T) {
	c, err := chart.Parse([]byte(excelChartWithBlank))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	p := pptx.Create()
	slide := p.AddSlide()
	if err := slide.AddChart(c, 100, 200, 300, 400); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	chartXML := zipPart(t, data, "ppt/charts/chart1.xml")
	if bytes.Contains(chartXML, []byte("NaN")) {
		t.Errorf("emitted chart.xml carries NaN:\n%s", chartXML)
	}

	embedName := ""
	for _, name := range zipNames(t, data) {
		if strings.HasPrefix(name, "ppt/embeddings/") && strings.HasSuffix(name, ".xlsx") {
			embedName = name
		}
	}
	if embedName == "" {
		t.Fatalf("no embedded workbook in the package: %v", zipNames(t, data))
	}
	embed := zipPart(t, data, embedName)
	if bytes.Contains(embed, []byte("NaN")) {
		t.Error("embedded workbook bytes carry NaN")
	}

	wb, err := xlsx.OpenReader(bytes.NewReader(embed), int64(len(embed)))
	if err != nil {
		t.Fatalf("embedded workbook is not a readable .xlsx: %v", err)
	}
	sheet, err := wb.SheetByName("Sheet1")
	if err != nil {
		t.Fatalf("SheetByName(Sheet1): %v", err)
	}
	for ref, want := range map[string]string{"B2": "10", "B3": "", "B4": "30"} {
		got, err := sheet.GetCellValue(ref)
		if err != nil {
			t.Fatalf("GetCellValue(%s): %v", ref, err)
		}
		if got != want {
			t.Errorf("embedded workbook %s = %q, want %q", ref, got, want)
		}
	}
}
