package chart

import (
	"bytes"
	"testing"
)

// TestRoundTripDoughnut builds a doughnut chart, checks it emits a
// c:doughnutChart with a hole size, and that it reads back as KindDoughnut with
// its single series, categories, and title intact.
func TestRoundTripDoughnut(t *testing.T) {
	cats := []string{"A", "B", "C"}
	vals := []float64{18, 2, 66}
	c := NewDoughnut().SetTitle("Ring").SetCategories(cats).SetLegend(LegendBottom)
	c.AddSeries("Cases", vals)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{"<c:doughnutChart>", "<c:holeSize val=\"50\"/>"} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing %q\n%s", want, xmlBytes)
		}
	}
	// A doughnut has no axes.
	if bytes.Contains(xmlBytes, []byte("<c:valAx>")) || bytes.Contains(xmlBytes, []byte("<c:catAx>")) {
		t.Error("doughnut chart should have no axes")
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind() != KindDoughnut {
		t.Errorf("kind: got %v want doughnut", got.Kind())
	}
	if got.Title() != "Ring" {
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

// TestRoundTripRadar builds a radar chart with two series and checks it emits a
// c:radarChart with axes and reads back as KindRadar with everything intact.
func TestRoundTripRadar(t *testing.T) {
	cats := []string{"Speed", "Power", "Range", "Agility"}
	s1 := []float64{3, 4, 2, 5}
	s2 := []float64{5, 2, 4, 3}
	c := NewRadar().SetTitle("Stats").SetCategories(cats)
	c.AddSeries("Alpha", s1)
	c.AddSeries("Beta", s2)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{"<c:radarChart>", "<c:radarStyle val=\"marker\"/>", "<c:catAx>", "<c:valAx>"} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing %q\n%s", want, xmlBytes)
		}
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind() != KindRadar {
		t.Errorf("kind: got %v want radar", got.Kind())
	}
	if got.Title() != "Stats" {
		t.Errorf("title: got %q", got.Title())
	}
	if !stringsEqual(got.Categories(), cats) {
		t.Errorf("categories: got %v", got.Categories())
	}
	gs := got.SeriesList()
	if len(gs) != 2 {
		t.Fatalf("series count: got %d want 2", len(gs))
	}
	if gs[0].Name != "Alpha" || !floatsEqual(gs[0].Values, s1) {
		t.Errorf("series 0: got %q %v", gs[0].Name, gs[0].Values)
	}
	if gs[1].Name != "Beta" || !floatsEqual(gs[1].Values, s2) {
		t.Errorf("series 1: got %q %v", gs[1].Name, gs[1].Values)
	}
}

// TestDataLabels checks the SetDataLabels toggle emits c:dLbls with showVal on
// the chart-type group and round-trips through Parse.
func TestDataLabels(t *testing.T) {
	c := NewColumn().SetCategories([]string{"Jan", "Feb"}).SetDataLabels(true)
	c.AddSeries("Rev", []float64{100, 250})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{"<c:dLbls>", "<c:showVal val=\"1\"/>", "<c:showLegendKey val=\"0\"/>"} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing %q\n%s", want, xmlBytes)
		}
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !got.DataLabels() {
		t.Error("expected data labels on after parse")
	}

	// The default (off) emits no dLbls.
	off := NewColumn().SetCategories([]string{"Jan"})
	off.AddSeries("Rev", []float64{1})
	offXML, err := off.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	if bytes.Contains(offXML, []byte("<c:dLbls>")) {
		t.Error("expected no dLbls when data labels are off")
	}
	gotOff, _ := Parse(offXML)
	if gotOff.DataLabels() {
		t.Error("expected data labels off after parse")
	}
}

// TestSeriesColor checks Series.SetColor emits a solid srgbClr fill and that the
// color round-trips through Parse, including hex normalization.
func TestSeriesColor(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b"})
	c.AddSeries("One", []float64{1, 2}).SetColor("#ff0000")
	c.AddSeries("Two", []float64{3, 4}).SetColor("1F77B4")

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{
		"<c:spPr><a:solidFill><a:srgbClr val=\"FF0000\"/></a:solidFill></c:spPr>",
		"<a:srgbClr val=\"1F77B4\"/>",
	} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing %q\n%s", want, xmlBytes)
		}
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	gs := got.SeriesList()
	if len(gs) != 2 {
		t.Fatalf("series count: got %d", len(gs))
	}
	if gs[0].Color != "FF0000" {
		t.Errorf("series 0 color: got %q want FF0000", gs[0].Color)
	}
	if gs[1].Color != "1F77B4" {
		t.Errorf("series 1 color: got %q want 1F77B4", gs[1].Color)
	}
}

// TestDoughnutAndRadarInStringer checks the Kind stringer names the new types.
func TestNewKindStrings(t *testing.T) {
	if KindDoughnut.String() != "doughnutChart" {
		t.Errorf("doughnut string: got %q", KindDoughnut.String())
	}
	if KindRadar.String() != "radarChart" {
		t.Errorf("radar string: got %q", KindRadar.String())
	}
}
