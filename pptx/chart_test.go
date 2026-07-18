package pptx

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/xlsx"
)

// buildChart returns a configured chart of the given kind with two categories
// and one or two series, for the round-trip tests.
func newCategoryChart(newFn func() *chart.Chart, title string) *chart.Chart {
	c := newFn().
		SetTitle(title).
		SetCategories([]string{"Q1", "Q2", "Q3"})
	c.AddSeries("Alpha", []float64{1, 2, 3})
	c.AddSeries("Beta", []float64{4, 5, 6})
	return c
}

func TestAddChartRoundTripByKind(t *testing.T) {
	cases := []struct {
		name string
		kind chart.Kind
		make func() *chart.Chart
	}{
		{"column", chart.KindColumn, chart.NewColumn},
		{"bar", chart.KindBar, chart.NewBar},
		{"line", chart.KindLine, chart.NewLine},
		{"area", chart.KindArea, chart.NewArea},
		{"pie", chart.KindPie, chart.NewPie},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Create()
			s := p.AddSlide()
			c := newCategoryChart(tc.make, "Title "+tc.name)
			if err := s.AddChart(c, 100, 200, 300, 400); err != nil {
				t.Fatalf("AddChart: %v", err)
			}

			data, err := p.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}

			p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			charts := p2.Charts()
			if len(charts) != 1 {
				t.Fatalf("Charts() = %d, want 1", len(charts))
			}
			got := charts[0]
			if got.Kind() != tc.kind {
				t.Errorf("kind = %v, want %v", got.Kind(), tc.kind)
			}
			if got.Title() != "Title "+tc.name {
				t.Errorf("title = %q, want %q", got.Title(), "Title "+tc.name)
			}
			wantCats := []string{"Q1", "Q2", "Q3"}
			if !equalStrings(got.Categories(), wantCats) {
				t.Errorf("categories = %v, want %v", got.Categories(), wantCats)
			}
			series := got.SeriesList()
			// A pie chart plots only its first series.
			wantSeries := 2
			if tc.kind == chart.KindPie {
				wantSeries = 1
			}
			if len(series) != wantSeries {
				t.Fatalf("series = %d, want %d", len(series), wantSeries)
			}
			if series[0].Name != "Alpha" {
				t.Errorf("series[0].Name = %q, want Alpha", series[0].Name)
			}
			if !equalFloats(series[0].Values, []float64{1, 2, 3}) {
				t.Errorf("series[0].Values = %v, want [1 2 3]", series[0].Values)
			}
		})
	}
}

func TestAddChartScatterRoundTrip(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	c := chart.NewScatter().SetTitle("XY")
	c.AddXYSeries("pts", []float64{1, 2, 3}, []float64{10, 20, 30})
	if err := s.AddChart(c, 0, 0, 1000, 1000); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	charts := p2.Charts()
	if len(charts) != 1 {
		t.Fatalf("Charts() = %d, want 1", len(charts))
	}
	got := charts[0]
	if got.Kind() != chart.KindScatter {
		t.Errorf("kind = %v, want scatter", got.Kind())
	}
	series := got.SeriesList()
	if len(series) != 1 {
		t.Fatalf("series = %d, want 1", len(series))
	}
	if !equalFloats(series[0].XValues, []float64{1, 2, 3}) {
		t.Errorf("XValues = %v, want [1 2 3]", series[0].XValues)
	}
	if !equalFloats(series[0].Values, []float64{10, 20, 30}) {
		t.Errorf("Values = %v, want [10 20 30]", series[0].Values)
	}
}

func TestAddChartPackageStructure(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	c := newCategoryChart(chart.NewColumn, "Struct")
	if err := s.AddChart(c, 1, 2, 3, 4); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	parts := unzipParts(t, data)
	for _, name := range []string{
		"ppt/charts/chart1.xml",
		"ppt/charts/_rels/chart1.xml.rels",
		"ppt/embeddings/Microsoft_Excel_Worksheet.xlsx",
	} {
		if _, ok := parts[name]; !ok {
			t.Errorf("missing part %s", name)
		}
	}
	ct := string(parts["[Content_Types].xml"])
	if !bytes.Contains([]byte(ct), []byte("drawingml.chart+xml")) {
		t.Error("chart content-type override missing")
	}
	if !bytes.Contains([]byte(ct), []byte("spreadsheetml.sheet\"")) {
		t.Error("embedded workbook content-type override missing")
	}
	slide := string(parts["ppt/slides/slide1.xml"])
	if !bytes.Contains([]byte(slide), []byte("graphicFrame")) {
		t.Error("slide graphicFrame missing")
	}
	if !bytes.Contains([]byte(slide), []byte("c:chart")) {
		t.Error("slide c:chart reference missing")
	}
	chartXML := string(parts["ppt/charts/chart1.xml"])
	if !bytes.Contains([]byte(chartXML), []byte("externalData")) {
		t.Error("chart externalData reference missing")
	}
	rels := string(parts["ppt/charts/_rels/chart1.xml.rels"])
	if !bytes.Contains([]byte(rels), []byte("relationships/package")) {
		t.Error("chart->workbook package relationship missing")
	}
}

func TestAddChartEmbeddedWorkbookOpens(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	c := chart.NewColumn().SetCategories([]string{"Jan", "Feb"})
	c.AddSeries("Sales", []float64{42, 99})
	if err := s.AddChart(c, 0, 0, 100, 100); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	wbBytes := unzipParts(t, data)["ppt/embeddings/Microsoft_Excel_Worksheet.xlsx"]
	if len(wbBytes) == 0 {
		t.Fatal("embedded workbook part is empty")
	}
	wb, err := xlsx.OpenReader(bytes.NewReader(wbBytes), int64(len(wbBytes)))
	if err != nil {
		t.Fatalf("open embedded workbook: %v", err)
	}
	sheets := wb.Sheets()
	if len(sheets) == 0 {
		t.Fatal("embedded workbook has no sheets")
	}
	sh := sheets[0]
	if sh.Name() != "Sheet1" {
		t.Errorf("sheet name = %q, want Sheet1", sh.Name())
	}
	checks := map[string]string{"A2": "Jan", "A3": "Feb", "B1": "Sales", "B2": "42", "B3": "99"}
	for ref, want := range checks {
		cell, err := sh.Cell(ref)
		if err != nil || cell == nil {
			t.Errorf("cell %s: %v", ref, err)
			continue
		}
		if cell.String() != want {
			t.Errorf("cell %s = %q, want %q", ref, cell.String(), want)
		}
	}
}

func TestAddChartCoexistsWithShapes(t *testing.T) {
	p := Create()
	s := p.AddSlide()

	tb := s.AddTextBox()
	tb.SetName("Existing")
	tb.SetText("keep me")

	c := newCategoryChart(chart.NewLine, "Coexist")
	if err := s.AddChart(c, 0, 0, 100, 100); err != nil {
		t.Fatalf("AddChart: %v", err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	// The text box survives (charts are not materialized as shapes, so the
	// only shape is the text box).
	shapes := p2.Slides()[0].Shapes()
	foundTB := false
	for _, sh := range shapes {
		if box, ok := sh.(*TextBox); ok && box.Text() == "keep me" {
			foundTB = true
		}
	}
	if !foundTB {
		t.Error("text box did not survive AddChart")
	}
	if len(p2.Charts()) != 1 {
		t.Errorf("Charts() = %d, want 1", len(p2.Charts()))
	}
}

// TestAddChartToOpenedDeckWithShapes exercises the surgical-append path: a chart
// added to a slide loaded from a file must not clobber its existing shapes.
func TestAddChartToOpenedDeckWithShapes(t *testing.T) {
	// Build a deck with a shape, save, reopen, then add a chart.
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	tb.SetText("original")
	base, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	p2, err := OpenReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s2 := p2.Slides()[0]
	c := newCategoryChart(chart.NewColumn, "Added")
	if err := s2.AddChart(c, 0, 0, 500, 500); err != nil {
		t.Fatalf("AddChart on opened deck: %v", err)
	}
	out, err := p2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	p3, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	shapes := p3.Slides()[0].Shapes()
	foundTB := false
	for _, sh := range shapes {
		if box, ok := sh.(*TextBox); ok && box.Text() == "original" {
			foundTB = true
		}
	}
	if !foundTB {
		t.Error("existing text box was clobbered by AddChart on an opened deck")
	}
	if len(p3.Charts()) != 1 {
		t.Errorf("Charts() = %d, want 1", len(p3.Charts()))
	}
}

// TestChartFixtureByteIdentityAndRead confirms that opening a real chart-bearing
// deck and saving it back preserves the chart parts byte-for-byte, and that
// Charts() reads the chart from it.
func TestChartFixtureByteIdentityAndRead(t *testing.T) {
	const path = "testdata/external/big_data.pptx"
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Skip("fixture not present:", path)
	}
	p, err := OpenReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if got := len(p.Charts()); got != 1 {
		t.Errorf("Charts() = %d, want 1", got)
	}
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	origParts := unzipParts(t, orig)
	outParts := unzipParts(t, out)
	for _, name := range []string{"ppt/charts/chart1.xml", "ppt/charts/_rels/chart1.xml.rels"} {
		if !bytes.Equal(origParts[name], outParts[name]) {
			t.Errorf("part %s changed on zero-mod round trip (%d -> %d bytes)",
				name, len(origParts[name]), len(outParts[name]))
		}
	}
}

// --- helpers ---

func unzipParts(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	parts := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			_ = rc.Close()
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		parts[f.Name] = buf.Bytes()
	}
	return parts
}

func equalStrings(a, b []string) bool {
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

func equalFloats(a, b []float64) bool {
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
