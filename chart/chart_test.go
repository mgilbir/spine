package chart

import (
	"bytes"
	"math"
	"testing"
)

func floatsEqual(a, b []float64) bool {
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

func stringsEqual(a, b []string) bool {
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

// TestRoundTripCategoryCharts builds each category chart type with two series
// and four categories, marshals it, parses it back, and checks the type,
// title, categories, and series values survive.
func TestRoundTripCategoryCharts(t *testing.T) {
	cats := []string{"Q1", "Q2", "Q3", "Q4"}
	s1 := []float64{10, 20, 30, 40}
	s2 := []float64{5, 15, 25, 2.5}

	builders := map[string]func() *Chart{
		"column": NewColumn,
		"bar":    NewBar,
		"line":   NewLine,
		"area":   NewArea,
	}
	for name, mk := range builders {
		t.Run(name, func(t *testing.T) {
			c := mk().SetTitle("Sales").SetCategories(cats).SetAxisTitles("Quarter", "USD")
			c.AddSeries("North", s1)
			c.AddSeries("South", s2)

			xmlBytes, err := c.MarshalChartXML()
			if err != nil {
				t.Fatalf("MarshalChartXML: %v", err)
			}
			if !bytes.Contains(xmlBytes, []byte("c:chartSpace")) {
				t.Fatalf("output is not a chartSpace: %s", xmlBytes)
			}

			got, err := Parse(xmlBytes)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got.Kind() != c.Kind() {
				t.Errorf("kind: got %v want %v", got.Kind(), c.Kind())
			}
			if got.Title() != "Sales" {
				t.Errorf("title: got %q", got.Title())
			}
			if !stringsEqual(got.Categories(), cats) {
				t.Errorf("categories: got %v want %v", got.Categories(), cats)
			}
			gs := got.SeriesList()
			if len(gs) != 2 {
				t.Fatalf("series count: got %d want 2", len(gs))
			}
			if gs[0].Name != "North" || !floatsEqual(gs[0].Values, s1) {
				t.Errorf("series 0: got %q %v", gs[0].Name, gs[0].Values)
			}
			if gs[1].Name != "South" || !floatsEqual(gs[1].Values, s2) {
				t.Errorf("series 1: got %q %v", gs[1].Name, gs[1].Values)
			}
			ct, vt := got.AxisTitles()
			if ct != "Quarter" || vt != "USD" {
				t.Errorf("axis titles: got %q/%q", ct, vt)
			}
			if pos, on := got.LegendPos(); !on || pos != LegendRight {
				t.Errorf("legend: on=%v pos=%v", on, pos)
			}
		})
	}
}

func TestRoundTripPie(t *testing.T) {
	cats := []string{"A", "B", "C"}
	vals := []float64{18, 2, 66}
	c := NewPie().SetTitle("Share").SetCategories(cats).SetLegend(LegendBottom)
	c.AddSeries("Cases", vals)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind() != KindPie {
		t.Errorf("kind: got %v", got.Kind())
	}
	if got.Title() != "Share" {
		t.Errorf("title: got %q", got.Title())
	}
	if !stringsEqual(got.Categories(), cats) {
		t.Errorf("categories: got %v", got.Categories())
	}
	gs := got.SeriesList()
	if len(gs) != 1 || gs[0].Name != "Cases" || !floatsEqual(gs[0].Values, vals) {
		t.Errorf("series: %+v", gs)
	}
	if pos, on := got.LegendPos(); !on || pos != LegendBottom {
		t.Errorf("legend: on=%v pos=%v", on, pos)
	}
}

func TestRoundTripScatter(t *testing.T) {
	x := []float64{1, 2, 3, 4}
	y1 := []float64{4, 5, 6, 7}
	y2 := []float64{1.5, 2.5, 3.5, 4.5}
	c := NewScatter().SetTitle("XY").SetAxisTitles("X", "Y")
	c.AddXYSeries("A", x, y1)
	c.AddXYSeries("B", x, y2)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind() != KindScatter {
		t.Fatalf("kind: got %v", got.Kind())
	}
	gs := got.SeriesList()
	if len(gs) != 2 {
		t.Fatalf("series count: got %d", len(gs))
	}
	if gs[0].Name != "A" || !floatsEqual(gs[0].XValues, x) || !floatsEqual(gs[0].Values, y1) {
		t.Errorf("series 0: name=%q x=%v y=%v", gs[0].Name, gs[0].XValues, gs[0].Values)
	}
	if gs[1].Name != "B" || !floatsEqual(gs[1].XValues, x) || !floatsEqual(gs[1].Values, y2) {
		t.Errorf("series 1: name=%q x=%v y=%v", gs[1].Name, gs[1].XValues, gs[1].Values)
	}
	ct, vt := got.AxisTitles()
	if ct != "X" || vt != "Y" {
		t.Errorf("axis titles: got %q/%q", ct, vt)
	}
}

// TestCachedValues checks that the emitted XML carries the input values in
// numeric/string caches so it renders without a live data source.
func TestCachedValues(t *testing.T) {
	c := NewColumn().SetCategories([]string{"Jan", "Feb"})
	c.AddSeries("Rev", []float64{100, 250})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{
		"<c:numCache>", "<c:strCache>",
		"<c:v>Jan</c:v>", "<c:v>Feb</c:v>",
		"<c:v>100</c:v>", "<c:v>250</c:v>",
		"<c:v>Rev</c:v>",
	} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing %q\n%s", want, xmlBytes)
		}
	}
}

// TestFormulaReferences checks that c:f references follow the layout
// convention and honor a custom DataRef.
func TestFormulaReferences(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b", "c"}).SetDataRef("Data")
	c.AddSeries("S1", []float64{1, 2, 3})
	c.AddSeries("S2", []float64{4, 5, 6})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{
		"<c:f>Data!$A$2:$A$4</c:f>", // categories
		"<c:f>Data!$B$1</c:f>",      // series 1 name
		"<c:f>Data!$B$2:$B$4</c:f>", // series 1 values
		"<c:f>Data!$C$1</c:f>",      // series 2 name
		"<c:f>Data!$C$2:$C$4</c:f>", // series 2 values
	} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing reference %q", want)
		}
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.DataRef != "Data" {
		t.Errorf("DataRef: got %q want Data", got.DataRef)
	}
}

func TestNoSeriesError(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a"})
	if _, err := c.MarshalChartXML(); err == nil {
		t.Fatal("expected error for chart with no series")
	}
}

// TestChartXMLShape checks that the emitted chartSpace has the element
// sequence a real DrawingML chart uses, in order. It mirrors the structure of
// a genuine Office-authored barChart (chartSpace > chart > plotArea >
// barChart+axes, then legend/plotVisOnly/dispBlanksAs).
func TestChartXMLShape(t *testing.T) {
	c := NewColumn().SetTitle("T").SetCategories([]string{"a", "b"})
	c.AddSeries("S", []float64{1, 2})
	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	s := string(xmlBytes)

	// Ordered top-level shape of a real chart part.
	sequence := []string{
		"<c:chartSpace",
		"<c:date1904 val=\"0\"/>",
		"<c:lang val=\"en-US\"/>",
		"<c:roundedCorners val=\"0\"/>",
		"<c:style val=\"2\"/>",
		"<c:chart>",
		"<c:title>",
		"<c:autoTitleDeleted val=\"0\"/>",
		"<c:plotArea>",
		"<c:layout/>",
		"<c:barChart>",
		"<c:barDir val=\"col\"/>",
		"<c:grouping val=\"clustered\"/>",
		"<c:ser>",
		"<c:cat>",
		"<c:val>",
		"</c:barChart>",
		"<c:valAx>",
		"<c:catAx>",
		"</c:plotArea>",
		"<c:legend>",
		"<c:plotVisOnly val=\"1\"/>",
		"<c:dispBlanksAs val=\"gap\"/>",
		"</c:chart>",
		"</c:chartSpace>",
	}
	pos := 0
	for _, tok := range sequence {
		idx := indexFrom(s, tok, pos)
		if idx < 0 {
			t.Fatalf("element %q missing or out of order after offset %d", tok, pos)
		}
		pos = idx + len(tok)
	}
}

func indexFrom(s, sub string, from int) int {
	if from > len(s) {
		return -1
	}
	i := bytes.Index([]byte(s[from:]), []byte(sub))
	if i < 0 {
		return -1
	}
	return from + i
}

func TestHideLegend(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a"}).HideLegend()
	c.AddSeries("S", []float64{1})
	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	if bytes.Contains(xmlBytes, []byte("<c:legend>")) {
		t.Error("expected no legend element")
	}
	got, _ := Parse(xmlBytes)
	if _, on := got.LegendPos(); on {
		t.Error("expected legend off after parse")
	}
}
