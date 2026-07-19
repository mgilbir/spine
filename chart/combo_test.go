package chart

import (
	"bytes"
	"testing"
)

// TestComboRoundTrip builds a combination chart with a column series on the
// primary axis and a line series on the secondary axis, and checks the emitted
// XML carries both chart-type groups and the secondary axis pair, then that it
// reads back as KindCombo with each series' plot type, axis, color, and values
// recovered in the original order.
func TestComboRoundTrip(t *testing.T) {
	cats := []string{"Q1", "Q2", "Q3", "Q4"}
	rev := []float64{100, 120, 140, 160}
	margin := []float64{12, 15, 14, 18}

	c := NewCombo().SetTitle("Rev vs Margin").SetCategories(cats).
		SetAxisTitles("Quarter", "Revenue").SetDataLabels(true)
	c.AddSeries("Revenue", rev).SetType(KindColumn).SetColor("#4472C4")
	c.AddSeries("Margin %", margin).SetType(KindLine).SetSecondaryAxis(true).SetColor("ED7D31")

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{"<c:barChart>", "<c:lineChart>", `<c:crosses val="max"/>`, `<c:delete val="1"/>`} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing %q\n%s", want, xmlBytes)
		}
	}
	// A secondary axis means two category axes and two value axes.
	if n := bytes.Count(xmlBytes, []byte("<c:catAx>")); n != 2 {
		t.Errorf("catAx count: got %d want 2", n)
	}
	if n := bytes.Count(xmlBytes, []byte("<c:valAx>")); n != 2 {
		t.Errorf("valAx count: got %d want 2", n)
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind() != KindCombo {
		t.Fatalf("kind: got %v want combo", got.Kind())
	}
	if got.Title() != "Rev vs Margin" {
		t.Errorf("title: got %q", got.Title())
	}
	if !stringsEqual(got.Categories(), cats) {
		t.Errorf("categories: got %v", got.Categories())
	}
	if !got.DataLabels() {
		t.Error("data labels: got false want true")
	}
	gs := got.SeriesList()
	if len(gs) != 2 {
		t.Fatalf("series count: got %d want 2", len(gs))
	}
	// Series 0: column, primary axis.
	if gs[0].Name != "Revenue" || gs[0].PlotType != KindColumn || gs[0].SecondaryAxis {
		t.Errorf("series 0: name=%q type=%v secondary=%v", gs[0].Name, gs[0].PlotType, gs[0].SecondaryAxis)
	}
	if !floatsEqual(gs[0].Values, rev) || gs[0].Color != "4472C4" {
		t.Errorf("series 0 data: values=%v color=%q", gs[0].Values, gs[0].Color)
	}
	// Series 1: line, secondary axis.
	if gs[1].Name != "Margin %" || gs[1].PlotType != KindLine || !gs[1].SecondaryAxis {
		t.Errorf("series 1: name=%q type=%v secondary=%v", gs[1].Name, gs[1].PlotType, gs[1].SecondaryAxis)
	}
	if !floatsEqual(gs[1].Values, margin) || gs[1].Color != "ED7D31" {
		t.Errorf("series 1 data: values=%v color=%q", gs[1].Values, gs[1].Color)
	}
}

// TestComboNoSecondaryAxis builds a combo with a column and an area series both
// on the primary axis and checks it emits a single axis pair (no secondary) and
// round-trips as a combo with both series on the primary axis.
func TestComboNoSecondaryAxis(t *testing.T) {
	cats := []string{"Jan", "Feb", "Mar"}
	c := NewCombo().SetCategories(cats)
	c.AddSeries("Bars", []float64{5, 6, 7}).SetType(KindColumn)
	c.AddSeries("Band", []float64{1, 2, 3}).SetType(KindArea)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{"<c:barChart>", "<c:areaChart>"} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
	if bytes.Contains(xmlBytes, []byte(`<c:crosses val="max"/>`)) {
		t.Error("no series is on the secondary axis, but a max-crossing axis was emitted")
	}
	if n := bytes.Count(xmlBytes, []byte("<c:catAx>")); n != 1 {
		t.Errorf("catAx count: got %d want 1", n)
	}
	if n := bytes.Count(xmlBytes, []byte("<c:valAx>")); n != 1 {
		t.Errorf("valAx count: got %d want 1", n)
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind() != KindCombo {
		t.Fatalf("kind: got %v want combo", got.Kind())
	}
	gs := got.SeriesList()
	if len(gs) != 2 {
		t.Fatalf("series count: got %d want 2", len(gs))
	}
	if gs[0].PlotType != KindColumn || gs[0].SecondaryAxis {
		t.Errorf("series 0: type=%v secondary=%v", gs[0].PlotType, gs[0].SecondaryAxis)
	}
	if gs[1].PlotType != KindArea || gs[1].SecondaryAxis {
		t.Errorf("series 1: type=%v secondary=%v", gs[1].PlotType, gs[1].SecondaryAxis)
	}
}

// TestComboSharedTypeSecondary covers a combo where both series are the same
// type (line) but split across the primary and secondary axes — two lineChart
// groups — which must still round-trip as a combo.
func TestComboSharedTypeSecondary(t *testing.T) {
	c := NewCombo().SetCategories([]string{"a", "b"})
	c.AddSeries("primary", []float64{1, 2}).SetType(KindLine)
	c.AddSeries("secondary", []float64{300, 400}).SetType(KindLine).SetSecondaryAxis(true)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	if n := bytes.Count(xmlBytes, []byte("<c:lineChart>")); n != 2 {
		t.Errorf("lineChart group count: got %d want 2", n)
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind() != KindCombo {
		t.Fatalf("kind: got %v want combo", got.Kind())
	}
	gs := got.SeriesList()
	if len(gs) != 2 || gs[0].SecondaryAxis || !gs[1].SecondaryAxis {
		t.Errorf("axis assignment: %+v", gs)
	}
}

// TestComboInvalidMemberType checks that a combo series with a non-combinable
// plot type (pie) is rejected at marshal time.
func TestComboInvalidMemberType(t *testing.T) {
	c := NewCombo().SetCategories([]string{"a", "b"})
	c.AddSeries("bad", []float64{1, 2}).SetType(KindPie)
	if _, err := c.MarshalChartXML(); err == nil {
		t.Fatal("expected an error for a pie series in a combo chart, got nil")
	}
}
