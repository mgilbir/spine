package xlsx

import (
	"archive/zip"
	"bytes"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
)

// refCells expands an absolute c:f reference (e.g. "ChartData1!$B$2:$B$4") into
// the sheet name and the ordered A1 cell references it spans.
func refCells(t *testing.T, ref string) (sheet string, cells []string) {
	t.Helper()
	bang := strings.Index(ref, "!")
	if bang < 0 {
		t.Fatalf("ref %q has no sheet", ref)
	}
	sheet = strings.Trim(ref[:bang], "'")
	rng := strings.ReplaceAll(ref[bang+1:], "$", "")
	start, end, isRange := strings.Cut(rng, ":")
	sr, sc, err := ParseCellRef(start)
	if err != nil {
		t.Fatalf("ParseCellRef(%q): %v", start, err)
	}
	if !isRange {
		return sheet, []string{FormatCellRef(sr, sc)}
	}
	er, ec, err := ParseCellRef(end)
	if err != nil {
		t.Fatalf("ParseCellRef(%q): %v", end, err)
	}
	for r := sr; r <= er; r++ {
		for c := sc; c <= ec; c++ {
			cells = append(cells, FormatCellRef(r, c))
		}
	}
	return sheet, cells
}

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
		{"doughnut", func() *chart.Chart {
			c := chart.NewDoughnut().SetTitle("Doughnut").SetCategories(cats)
			c.AddSeries("Share", s1)
			return c
		}, false},
		{"radar", func() *chart.Chart {
			c := chart.NewRadar().SetTitle("Radar").SetCategories(cats).SetDataLabels(true)
			c.AddSeries("North", s1).SetColor("#FF0000")
			c.AddSeries("South", s2)
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

// TestAddChartComboRoundTrip adds a combination chart (column + secondary-axis
// line) to a sheet and verifies each series' plot type and axis survive a
// save/reopen.
func TestAddChartComboRoundTrip(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Data")
	c := chart.NewCombo().SetCategories([]string{"Q1", "Q2", "Q3"})
	c.AddSeries("Revenue", []float64{10, 20, 30}).SetType(chart.KindColumn)
	c.AddSeries("Margin", []float64{1, 2, 3}).SetType(chart.KindLine).SetSecondaryAxis(true)
	if err := sheet.AddChart("E2", c); err != nil {
		t.Fatalf("AddChart: %v", err)
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
		t.Fatalf("Charts(): got %d want 1", len(got))
	}
	gc := got[0]
	if gc.Kind() != chart.KindCombo {
		t.Fatalf("kind: got %v want combo", gc.Kind())
	}
	gs := gc.SeriesList()
	if len(gs) != 2 {
		t.Fatalf("series count: got %d want 2", len(gs))
	}
	if gs[0].PlotType != chart.KindColumn || gs[0].SecondaryAxis {
		t.Errorf("series 0: type=%v secondary=%v", gs[0].PlotType, gs[0].SecondaryAxis)
	}
	if gs[1].PlotType != chart.KindLine || !gs[1].SecondaryAxis {
		t.Errorf("series 1: type=%v secondary=%v", gs[1].PlotType, gs[1].SecondaryAxis)
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

// TestAddChartToSheetWithExistingChart adds a second chart to an opened sheet
// that already carries a chart (from the source package) and verifies BOTH
// charts survive. Previously the sheet's single <drawing> was repointed at a
// fresh part holding only the new chart, orphaning the original (C249).
func TestAddChartToSheetWithExistingChart(t *testing.T) {
	// Seed a workbook whose sheet already has one chart, then reopen so the
	// second AddChart runs on the round-trip save path against a preserved
	// drawing part.
	seed := Create()
	sheet := seed.AddSheet("Data")
	c1 := chart.NewColumn().SetTitle("First").SetCategories([]string{"a", "b"})
	c1.AddSeries("S1", []float64{1, 2})
	if err := sheet.AddChart("B2", c1); err != nil {
		t.Fatalf("seed AddChart: %v", err)
	}
	seedData, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}

	wb, err := OpenReader(bytes.NewReader(seedData), int64(len(seedData)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s, err := wb.SheetByName("Data")
	if err != nil {
		t.Fatalf("SheetByName: %v", err)
	}
	if got := len(s.Charts()); got != 1 {
		t.Fatalf("opened sheet Charts(): got %d want 1", got)
	}
	c2 := chart.NewLine().SetTitle("Second").SetCategories([]string{"a", "b", "c"})
	c2.AddSeries("S2", []float64{4, 5, 6})
	if err := s.AddChart("H2", c2); err != nil {
		t.Fatalf("second AddChart: %v", err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// The single drawing part must now reference two chart parts (both anchors
	// preserved), and both chart parts must be present.
	parts := unzipParts(t, data)
	if _, ok := parts["xl/drawings/drawing2.xml"]; ok {
		t.Error("expected a single merged drawing part, found a second")
	}
	drawing := parts["xl/drawings/drawing1.xml"]
	if n := bytes.Count(drawing, []byte("<c:chart ")); n != 2 {
		t.Errorf("drawing chart reference count: got %d want 2\n%s", n, drawing)
	}
	if _, ok := parts["xl/charts/chart1.xml"]; !ok {
		t.Error("missing xl/charts/chart1.xml")
	}
	if _, ok := parts["xl/charts/chart2.xml"]; !ok {
		t.Error("missing xl/charts/chart2.xml (second chart lost)")
	}

	// Reopen: both charts recovered, with their titles.
	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2, _ := wb2.SheetByName("Data")
	got := s2.Charts()
	if len(got) != 2 {
		t.Fatalf("Charts after reopen: got %d want 2", len(got))
	}
	titles := map[string]bool{}
	for _, gc := range got {
		titles[gc.Title()] = true
	}
	if !titles["First"] || !titles["Second"] {
		t.Errorf("recovered chart titles = %v, want First and Second", titles)
	}
}

// TestAddBubbleChartDataLayout builds a multi-series bubble chart with distinct
// X/Y/size vectors, adds it to a sheet, and verifies the data sheet cells each
// series' c:xVal/c:yVal/c:bubbleSize reference actually hold that series' values.
// The data-sheet writer previously had only category/scatter layouts, so bubble
// references pointed at empty or foreign cells (C248).
func TestAddBubbleChartDataLayout(t *testing.T) {
	type ser struct {
		name        string
		x, y, sizes []float64
	}
	sers := []ser{
		{"Alpha", []float64{1, 2, 3}, []float64{10, 20, 30}, []float64{4, 5, 6}},
		{"Beta", []float64{1, 2, 3}, []float64{40, 50, 60}, []float64{7, 8, 9}},
	}
	c := chart.NewBubble().SetTitle("Bubbles")
	for _, s := range sers {
		c.AddBubbleSeries(s.name, s.x, s.y, s.sizes)
	}

	wb := Create()
	sheet := wb.AddSheet("Host")
	if err := sheet.AddChart("E2", c); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}

	// The chart's layout holds the exact c:f references serialized into
	// chart.xml (MarshalChartXML builds both from the same layout); assert the
	// cells they point at carry each series' values.
	layout := c.Layout()
	if len(layout.Series) != len(sers) {
		t.Fatalf("layout series = %d, want %d", len(layout.Series), len(sers))
	}
	check := func(ref string, want []float64) {
		sheetName, cells := refCells(t, ref)
		ds, err := wb2.SheetByName(sheetName)
		if err != nil {
			t.Fatalf("data sheet %q: %v", sheetName, err)
		}
		if len(cells) != len(want) {
			t.Fatalf("ref %q spans %d cells, want %d", ref, len(cells), len(want))
		}
		for i, cellRef := range cells {
			got, err := ds.GetCellValue(cellRef)
			if err != nil {
				t.Fatalf("GetCellValue(%s): %v", cellRef, err)
			}
			wantStr := strconv.FormatFloat(want[i], 'f', -1, 64)
			if got != wantStr {
				t.Errorf("ref %q cell %s = %q, want %q", ref, cellRef, got, wantStr)
			}
		}
	}
	for i, s := range sers {
		sl := layout.Series[i]
		check(sl.XValuesRef, s.x)
		check(sl.ValuesRef, s.y)
		check(sl.SizesRef, s.sizes)
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
