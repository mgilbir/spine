package docx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/xlsx"
)

// newDocxChart builds a chart of the given kind with a small fixed dataset so
// the round-trip assertions have known values.
func newDocxChart(kind chart.Kind) *chart.Chart {
	switch kind {
	case chart.KindColumn:
		c := chart.NewColumn().SetTitle("Col").SetCategories([]string{"Q1", "Q2", "Q3"})
		c.AddSeries("North", []float64{10, 20, 30})
		return c
	case chart.KindBar:
		c := chart.NewBar().SetTitle("Bar").SetCategories([]string{"Q1", "Q2"})
		c.AddSeries("S", []float64{4, 5})
		return c
	case chart.KindLine:
		c := chart.NewLine().SetTitle("Line").SetCategories([]string{"a", "b", "c"})
		c.AddSeries("L", []float64{1, 2, 3})
		return c
	case chart.KindPie:
		c := chart.NewPie().SetTitle("Pie").SetCategories([]string{"x", "y"})
		c.AddSeries("P", []float64{60, 40})
		return c
	case chart.KindDoughnut:
		c := chart.NewDoughnut().SetTitle("Doughnut").SetCategories([]string{"x", "y"})
		c.AddSeries("D", []float64{70, 30})
		return c
	case chart.KindRadar:
		c := chart.NewRadar().SetTitle("Radar").SetCategories([]string{"p", "q", "r"})
		c.AddSeries("R1", []float64{1, 2, 3})
		c.AddSeries("R2", []float64{3, 2, 1})
		return c
	case chart.KindArea:
		c := chart.NewArea().SetTitle("Area").SetCategories([]string{"m", "n"})
		c.AddSeries("A", []float64{7, 8})
		return c
	case chart.KindScatter:
		c := chart.NewScatter().SetTitle("Scatter")
		c.AddXYSeries("XY", []float64{1, 2, 3}, []float64{9, 8, 7})
		return c
	default:
		return nil
	}
}

// TestAddChartRoundTrip creates a document with a chart of each type on the
// Create path, saves it, reopens it, and reads the chart back via Charts(),
// verifying the type, title, categories, and series survive.
func TestAddChartRoundTrip(t *testing.T) {
	kinds := []chart.Kind{
		chart.KindColumn, chart.KindBar, chart.KindLine,
		chart.KindPie, chart.KindDoughnut, chart.KindRadar,
		chart.KindArea, chart.KindScatter,
	}
	for _, kind := range kinds {
		t.Run(kind.String()+"_"+kindSuffix(kind), func(t *testing.T) {
			c := newDocxChart(kind)
			doc := Create()
			doc.AddParagraphWithText("intro")
			if err := doc.AddChart(c, 5029200, 2743200); err != nil {
				t.Fatalf("AddChart: %v", err)
			}

			data, err := doc.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}

			rd, err := OpenReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			charts := rd.Charts()
			if len(charts) != 1 {
				t.Fatalf("Charts() = %d, want 1", len(charts))
			}
			got := charts[0]
			if got.Kind() != kind {
				t.Errorf("Kind = %v, want %v", got.Kind(), kind)
			}
			if got.Title() != c.Title() {
				t.Errorf("Title = %q, want %q", got.Title(), c.Title())
			}
			if strings.Join(got.Categories(), ",") != strings.Join(c.Categories(), ",") {
				t.Errorf("Categories = %v, want %v", got.Categories(), c.Categories())
			}
			gotSeries := got.SeriesList()
			wantSeries := c.SeriesList()
			if len(gotSeries) != len(wantSeries) {
				t.Fatalf("series count = %d, want %d", len(gotSeries), len(wantSeries))
			}
			for i := range wantSeries {
				if gotSeries[i].Name != wantSeries[i].Name {
					t.Errorf("series[%d] name = %q, want %q", i, gotSeries[i].Name, wantSeries[i].Name)
				}
				if !floatsEqual(gotSeries[i].Values, wantSeries[i].Values) {
					t.Errorf("series[%d] values = %v, want %v", i, gotSeries[i].Values, wantSeries[i].Values)
				}
			}
		})
	}
}

func TestAddChartComboRoundTrip(t *testing.T) {
	c := chart.NewCombo().SetCategories([]string{"Q1", "Q2", "Q3"})
	c.AddSeries("Revenue", []float64{10, 20, 30}).SetType(chart.KindColumn)
	c.AddSeries("Margin", []float64{1, 2, 3}).SetType(chart.KindLine).SetSecondaryAxis(true)

	doc := Create()
	doc.AddParagraphWithText("intro")
	if err := doc.AddChart(c, 5029200, 2743200); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	rd, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	charts := rd.Charts()
	if len(charts) != 1 {
		t.Fatalf("Charts() = %d, want 1", len(charts))
	}
	got := charts[0]
	if got.Kind() != chart.KindCombo {
		t.Fatalf("Kind = %v, want combo", got.Kind())
	}
	gs := got.SeriesList()
	if len(gs) != 2 {
		t.Fatalf("series count = %d, want 2", len(gs))
	}
	if gs[0].PlotType != chart.KindColumn || gs[0].SecondaryAxis {
		t.Errorf("series 0: type=%v secondary=%v", gs[0].PlotType, gs[0].SecondaryAxis)
	}
	if gs[1].PlotType != chart.KindLine || !gs[1].SecondaryAxis {
		t.Errorf("series 1: type=%v secondary=%v", gs[1].PlotType, gs[1].SecondaryAxis)
	}
}

func kindSuffix(k chart.Kind) string {
	switch k {
	case chart.KindColumn:
		return "column"
	case chart.KindBar:
		return "bar"
	default:
		return k.String()
	}
}

func floatsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestChartEmbeddedWorkbookOpens verifies the embedded workbook a docx chart
// carries opens via the xlsx package with data matching the chart.
func TestChartEmbeddedWorkbookOpens(t *testing.T) {
	c := chart.NewColumn().SetCategories([]string{"Q1", "Q2", "Q3"})
	c.AddSeries("North", []float64{10, 20, 30})

	doc := Create()
	if err := doc.AddChart(c, 5029200, 2743200); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// Pull the embedded workbook part out of the package.
	wbBytes := []byte(mustZipEntry(t, data, "word/embeddings/Microsoft_Excel_Worksheet1.xlsx"))
	wb, err := xlsx.OpenReader(bytes.NewReader(wbBytes), int64(len(wbBytes)))
	if err != nil {
		t.Fatalf("open embedded workbook: %v", err)
	}
	sh, err := wb.SheetByName("Sheet1")
	if err != nil {
		t.Fatalf("SheetByName: %v", err)
	}
	cell := func(ref string) string {
		v, err := sh.GetCellValue(ref)
		if err != nil {
			t.Fatalf("GetCellValue(%s): %v", ref, err)
		}
		return v
	}
	if got := cell("A2"); got != "Q1" {
		t.Errorf("A2 = %q, want Q1", got)
	}
	if got := cell("B1"); got != "North" {
		t.Errorf("B1 = %q, want North", got)
	}
	if got := cell("B2"); got != "10" {
		t.Errorf("B2 = %q, want 10", got)
	}
	if got := cell("B4"); got != "30" {
		t.Errorf("B4 = %q, want 30", got)
	}
}

// TestChartContentTypesAndWiring verifies the package produced on the Create
// path declares the chart and embedded-workbook content types and wires the
// inline drawing, the document relationship, and the chart→workbook rel.
func TestChartContentTypesAndWiring(t *testing.T) {
	c := chart.NewColumn().SetCategories([]string{"a", "b"})
	c.AddSeries("s", []float64{1, 2})
	doc := Create()
	if err := doc.AddChart(c, 100, 100); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	ct := mustZipEntry(t, data, "[Content_Types].xml")
	if !strings.Contains(ct, "drawingml.chart+xml") {
		t.Error("content types missing chart override")
	}
	if !strings.Contains(ct, "spreadsheetml.sheet") {
		t.Error("content types missing embedded-workbook type")
	}

	docXML := mustZipEntry(t, data, "word/document.xml")
	if !strings.Contains(docXML, "<c:chart") {
		t.Error("document.xml missing c:chart element")
	}
	if !strings.Contains(docXML, "drawingml/2006/chart") {
		t.Error("document.xml missing chart graphicData uri")
	}

	docRels := mustZipEntry(t, data, "word/_rels/document.xml.rels")
	if !strings.Contains(docRels, "relationships/chart") || !strings.Contains(docRels, "charts/chart1.xml") {
		t.Errorf("document.xml.rels missing chart relationship:\n%s", docRels)
	}

	chartRels := mustZipEntry(t, data, "word/charts/_rels/chart1.xml.rels")
	if !strings.Contains(chartRels, "relationships/package") {
		t.Error("chart rels missing package relationship type")
	}
	if !strings.Contains(chartRels, "../embeddings/Microsoft_Excel_Worksheet1.xlsx") {
		t.Errorf("chart rels missing embedded-workbook target:\n%s", chartRels)
	}
}

// TestChartChildOrder places a chart in a paragraph between two text runs and
// checks the inline drawing keeps its position in the run order.
func TestChartChildOrder(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddRun().SetText("before ")
	c := chart.NewColumn().SetCategories([]string{"a"})
	c.AddSeries("s", []float64{1})
	if err := p.AddChart(c, 100, 100); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	p.AddRun().SetText(" after")

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	docXML := mustZipEntry(t, data, "word/document.xml")
	iBefore := strings.Index(docXML, "before ")
	iChart := strings.Index(docXML, "<c:chart")
	iAfter := strings.Index(docXML, " after")
	if iBefore < 0 || iChart < 0 || iAfter < 0 {
		t.Fatalf("missing expected content: before=%d chart=%d after=%d", iBefore, iChart, iAfter)
	}
	if iBefore >= iChart || iChart >= iAfter {
		t.Errorf("child order wrong: before=%d chart=%d after=%d\n%s", iBefore, iChart, iAfter, docXML)
	}
}

// TestChartByteIdentical opens a real chart-bearing docx and saves it with no
// modifications, asserting every part round-trips byte-for-byte (the chart and
// embedded-workbook parts are preserved raw when unmodified).
func TestChartByteIdentical(t *testing.T) {
	const path = "testdata/chart.docx"
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("chart fixture missing (expected tracked at %s): %v", path, err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// The fixture must actually carry a chart, or the test proves nothing.
	if len(d.Charts()) == 0 {
		t.Fatal("fixture has no charts")
	}
	tmp := filepath.Join(t.TempDir(), "out.docx")
	if err := d.Save(tmp); err != nil {
		_ = d.Close()
		t.Fatalf("Save: %v", err)
	}
	_ = d.Close()

	missing, extra, changed := testutil.CompareZipFiles(t, path, tmp)
	for _, n := range missing {
		t.Errorf("MISSING part: %s", n)
	}
	for _, n := range extra {
		t.Errorf("EXTRA part: %s", n)
	}
	for _, n := range changed {
		t.Errorf("CHANGED part: %s", n)
	}
}

// TestAddChartEmbedNameAvoidsOrphanCollision opens a package carrying an orphan
// embedded workbook (/word/embeddings/Microsoft_Excel_Worksheet1.xlsx) but no
// chart1.xml, then adds a chart. nextChartNumber picked N by scanning only
// chart part names, so it would have chosen N=1 and derived an embed name
// colliding with the preserved orphan — writeChartParts then hit
// ErrDuplicatePart and Save failed. The number scan must also cover embedding
// part names.
func TestAddChartEmbedNameAvoidsOrphanCollision(t *testing.T) {
	ct := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`</Types>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": ct,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		// Orphan embedded workbook occupying N=1 with no matching chart1.xml.
		"word/embeddings/Microsoft_Excel_Worksheet1.xlsx": "orphan-workbook-bytes",
	})
	doc := openFixture(t, fixture)

	if err := doc.AddChart(newDocxChart(chart.KindColumn), 3000000, 2000000); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	// The chart's embed name must not reuse the orphan's number.
	if got := doc.chartParts[0].embedName; strings.EqualFold(got, "/word/embeddings/Microsoft_Excel_Worksheet1.xlsx") {
		t.Fatalf("embed name %q collides with the preserved orphan", got)
	}

	saved := saveDoc(t, doc)
	// The orphan must survive and the new embedding must be written alongside it.
	if _, ok := zipEntry(t, saved, "word/embeddings/Microsoft_Excel_Worksheet1.xlsx"); !ok {
		t.Fatal("orphan embedding lost")
	}
	newEmbed := strings.TrimPrefix(doc.chartParts[0].embedName, "/")
	if _, ok := zipEntry(t, saved, newEmbed); !ok {
		t.Fatalf("new embedding %q missing from saved package", newEmbed)
	}
}
