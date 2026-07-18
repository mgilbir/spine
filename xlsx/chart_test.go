package xlsx

import (
	"archive/zip"
	"bytes"
	"math"
	"os"
	"testing"

	"github.com/mgilbir/spine/chart"
)

func chartFloatsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-9 {
			return false
		}
	}
	return true
}

func chartStringsEqual(a, b []string) bool {
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

// TestAddChartCreatePathRoundTrip builds each supported chart type on a
// freshly created workbook, saves, reopens, and verifies Sheet.Charts and
// Workbook.Charts recover the type, title, categories, and series. It confirms
// AddChart persists on the Create path (the xlsx comments API once had a
// save-new gap).
func TestAddChartCreatePathRoundTrip(t *testing.T) {
	cats := []string{"Q1", "Q2", "Q3", "Q4"}
	s1 := []float64{10, 20, 30, 40}
	s2 := []float64{5, 15, 25, 35}

	cases := []struct {
		name    string
		build   func() *chart.Chart
		scatter bool
	}{
		{"column", func() *chart.Chart {
			c := chart.NewColumn().SetTitle("Col").SetCategories(cats)
			c.AddSeries("North", s1)
			c.AddSeries("South", s2)
			return c
		}, false},
		{"bar", func() *chart.Chart {
			c := chart.NewBar().SetTitle("Bar").SetCategories(cats)
			c.AddSeries("North", s1)
			return c
		}, false},
		{"line", func() *chart.Chart {
			c := chart.NewLine().SetTitle("Line").SetCategories(cats)
			c.AddSeries("North", s1)
			c.AddSeries("South", s2)
			return c
		}, false},
		{"pie", func() *chart.Chart {
			c := chart.NewPie().SetTitle("Pie").SetCategories(cats)
			c.AddSeries("Share", s1)
			return c
		}, false},
		{"scatter", func() *chart.Chart {
			c := chart.NewScatter().SetTitle("Scatter")
			c.AddXYSeries("A", s1, s2)
			return c
		}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wb := Create()
			sheet := wb.AddSheet("Data")
			if err := sheet.SetCellValue("A1", "hello"); err != nil {
				t.Fatalf("SetCellValue: %v", err)
			}
			c := tc.build()
			if err := sheet.AddChart("E2", c); err != nil {
				t.Fatalf("AddChart: %v", err)
			}

			// Create-path persistence: the chart is visible on the model before save.
			if got := len(sheet.Charts()); got != 1 {
				t.Fatalf("Charts() before save: got %d want 1", got)
			}

			data, err := wb.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}

			wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			sheet2, err := wb2.SheetByName("Data")
			if err != nil {
				t.Fatalf("SheetByName: %v", err)
			}

			got := sheet2.Charts()
			if len(got) != 1 {
				t.Fatalf("Charts() after reopen: got %d want 1", len(got))
			}
			if all := wb2.Charts(); len(all) != 1 {
				t.Errorf("Workbook.Charts: got %d want 1", len(all))
			}
			gc := got[0]
			if gc.Kind() != c.Kind() {
				t.Errorf("kind: got %v want %v", gc.Kind(), c.Kind())
			}
			if gc.Title() != c.Title() {
				t.Errorf("title: got %q want %q", gc.Title(), c.Title())
			}
			if !tc.scatter {
				if !chartStringsEqual(gc.Categories(), cats) {
					t.Errorf("categories: got %v want %v", gc.Categories(), cats)
				}
			}
			gs := gc.SeriesList()
			ws := c.SeriesList()
			if len(gs) != len(ws) {
				t.Fatalf("series count: got %d want %d", len(gs), len(ws))
			}
			for i := range ws {
				if gs[i].Name != ws[i].Name {
					t.Errorf("series %d name: got %q want %q", i, gs[i].Name, ws[i].Name)
				}
				if !chartFloatsEqual(gs[i].Values, ws[i].Values) {
					t.Errorf("series %d values: got %v want %v", i, gs[i].Values, ws[i].Values)
				}
				if tc.scatter && !chartFloatsEqual(gs[i].XValues, ws[i].XValues) {
					t.Errorf("series %d x: got %v want %v", i, gs[i].XValues, ws[i].XValues)
				}
			}
		})
	}
}

// TestAddChartOpenedPath adds a chart to a workbook opened from an existing
// package (not Create) and verifies it round-trips, alongside the file's
// original parts.
func TestAddChartOpenedPath(t *testing.T) {
	// Build a plain workbook, then reopen it so the second AddChart runs on the
	// opened (round-trip) save path.
	base := Create()
	if err := base.AddSheet("One").SetCellValue("A1", "x"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	seed, err := base.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}

	wb, err := OpenReader(bytes.NewReader(seed), int64(len(seed)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sheet, err := wb.SheetByName("One")
	if err != nil {
		t.Fatalf("SheetByName: %v", err)
	}
	c := chart.NewColumn().SetTitle("Opened").SetCategories([]string{"a", "b"})
	c.AddSeries("S", []float64{7, 8})
	if err := sheet.AddChart("D2", c); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2, _ := wb2.SheetByName("One")
	got := s2.Charts()
	if len(got) != 1 {
		t.Fatalf("Charts after reopen: got %d want 1", len(got))
	}
	if got[0].Title() != "Opened" {
		t.Errorf("title: got %q", got[0].Title())
	}
	if v, _ := s2.GetCellValue("A1"); v != "x" {
		t.Errorf("original cell A1: got %q want x", v)
	}
}

// TestAddChartParts checks the package layout a chart produces: the chart part,
// the drawing part with a graphicFrame chart reference, the worksheet <drawing>
// element, and the chart/drawing content-type overrides. It also verifies the
// data lands on a dedicated hidden sheet and the host cell is untouched.
func TestAddChartParts(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sales")
	if err := sheet.SetCellValue("A1", "keepme"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	c := chart.NewColumn().SetTitle("T").SetCategories([]string{"a", "b"})
	c.AddSeries("S1", []float64{1, 2})
	if err := sheet.AddChart("E2:L20", c); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	parts := unzipParts(t, data)
	for _, want := range []string{"xl/charts/chart1.xml", "xl/drawings/drawing1.xml", "xl/drawings/_rels/drawing1.xml.rels"} {
		if _, ok := parts[want]; !ok {
			t.Errorf("missing part %s", want)
		}
	}
	ct := string(parts["[Content_Types].xml"])
	if !bytes.Contains([]byte(ct), []byte("drawingml.chart+xml")) {
		t.Error("content types missing chart override")
	}
	if !bytes.Contains([]byte(ct), []byte("officedocument.drawing+xml")) {
		t.Error("content types missing drawing override")
	}
	if !bytes.Contains(parts["xl/worksheets/sheet1.xml"], []byte("<drawing")) {
		t.Error("worksheet missing <drawing> element")
	}
	drawing := parts["xl/drawings/drawing1.xml"]
	if !bytes.Contains(drawing, []byte("graphicFrame")) || !bytes.Contains(drawing, []byte("c:chart")) {
		t.Error("drawing missing chart graphicFrame")
	}
	if !bytes.Contains(parts["xl/drawings/_rels/drawing1.xml.rels"], []byte("relationships/chart")) {
		t.Error("drawing rels missing chart relationship")
	}

	// Host cell preserved; data placed on a hidden ChartData sheet.
	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s2, _ := wb2.SheetByName("Sales")
	if v, _ := s2.GetCellValue("A1"); v != "keepme" {
		t.Errorf("host cell A1: got %q want keepme", v)
	}
	ds, err := wb2.SheetByName("ChartData1")
	if err != nil {
		t.Fatalf("ChartData1 sheet missing: %v", err)
	}
	if ds.state != "hidden" {
		t.Errorf("data sheet state: got %q want hidden", ds.state)
	}
	if v, _ := ds.GetCellValue("A2"); v != "a" {
		t.Errorf("data sheet A2: got %q want a", v)
	}
}

// TestChartAndImageCoexist puts an image and a chart on the same sheet and
// verifies both survive a save/reopen and share one drawing part.
func TestChartAndImageCoexist(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Mix")
	if err := sheet.AddImage("A1", testPNG(t, 20, 10), ImageOptions{WidthPx: 100, HeightPx: 50}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	c := chart.NewLine().SetTitle("Trend").SetCategories([]string{"a", "b", "c"})
	c.AddSeries("S", []float64{3, 1, 2})
	if err := sheet.AddChart("E2", c); err != nil {
		t.Fatalf("AddChart: %v", err)
	}

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// One drawing part holds both the pic and the chart graphicFrame.
	parts := unzipParts(t, data)
	drawing := parts["xl/drawings/drawing1.xml"]
	if !bytes.Contains(drawing, []byte("<xdr:pic>")) {
		t.Error("drawing missing image pic")
	}
	if !bytes.Contains(drawing, []byte("graphicFrame")) {
		t.Error("drawing missing chart graphicFrame")
	}
	if _, ok := parts["xl/drawings/drawing2.xml"]; ok {
		t.Error("expected a single shared drawing part, found a second")
	}

	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s2, _ := wb2.SheetByName("Mix")
	if got := len(s2.Images()); got != 1 {
		t.Errorf("Images: got %d want 1", got)
	}
	if got := len(s2.Charts()); got != 1 {
		t.Errorf("Charts: got %d want 1", got)
	}
}

// TestChartFixtureByteIdentical opens a real chart-bearing workbook and checks
// a zero-modification save reproduces the source bytes: the chart and drawing
// parts are preserved verbatim when no chart is added or modified.
func TestChartFixtureByteIdentical(t *testing.T) {
	const path = "testdata/external/excelize_test.xlsx"
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	// Guard against a stub/empty symlink target masquerading as the fixture.
	if !bytes.Contains(mustPart(t, orig, "[Content_Types].xml"), []byte("drawingml.chart+xml")) {
		t.Fatalf("fixture %s is not chart-bearing; refusing to pass vacuously", path)
	}

	wb, err := OpenReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	origParts := unzipParts(t, orig)
	outParts := unzipParts(t, out)
	for _, name := range []string{"xl/charts/chart1.xml", "xl/charts/chart2.xml", "xl/drawings/drawing1.xml"} {
		if !bytes.Equal(origParts[name], outParts[name]) {
			t.Errorf("part %s not byte-identical after zero-mod save", name)
		}
	}
}

func mustPart(t *testing.T, zipBytes []byte, name string) []byte {
	t.Helper()
	p := unzipParts(t, zipBytes)[name]
	if p == nil {
		t.Fatalf("part %s not found", name)
	}
	return p
}

func unzipParts(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		out[f.Name] = buf.Bytes()
	}
	return out
}
