package chart

import (
	"strings"
	"testing"
)

// TestDoughnutPlotsEveryRing pins C561 for the doughnut: multiple rings are
// legitimate in Office, and the embedded workbook already carries a column per
// series, so dropping every series after the first left the chart referencing
// less data than it shipped.
func TestDoughnutPlotsEveryRing(t *testing.T) {
	c := NewDoughnut().SetCategories([]string{"a", "b"})
	c.AddSeries("Inner", []float64{1, 2})
	c.AddSeries("Outer", []float64{3, 4})

	out, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	s := string(out)
	if n := strings.Count(s, "<c:ser>"); n != 2 {
		t.Errorf("doughnut emitted %d series, want 2:\n%s", n, s)
	}
	if !strings.Contains(s, "<c:f>Sheet1!$C$2:$C$3</c:f>") {
		t.Errorf("second ring does not reference its own column:\n%s", s)
	}

	got, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	gs := got.SeriesList()
	if len(gs) != 2 || gs[0].Name != "Inner" || gs[1].Name != "Outer" {
		t.Errorf("read back %d series: %+v", len(gs), gs)
	}
}

// TestPieFamilyPlotsFirstSeriesOnly pins the documented other half of C561: pie,
// 3D pie, and pie-of-pie plot one series, and that is deliberate — the
// remaining data stays in the workbook so it can be re-pointed.
func TestPieFamilyPlotsFirstSeriesOnly(t *testing.T) {
	for name, mk := range map[string]func() *Chart{"pie": NewPie, "pie3d": NewPie3D, "ofpie": NewOfPie} {
		c := mk().SetCategories([]string{"a", "b"})
		c.AddSeries("First", []float64{1, 2})
		c.AddSeries("Second", []float64{3, 4})
		out, err := c.MarshalChartXML()
		if err != nil {
			t.Fatalf("%s: MarshalChartXML: %v", name, err)
		}
		if n := strings.Count(string(out), "<c:ser>"); n != 1 {
			t.Errorf("%s emitted %d series, want 1", name, n)
		}
		// The second series' data is still written, so nothing is lost.
		var secondCol bool
		for _, dc := range c.DataCells() {
			if dc.Col == 3 {
				secondCol = true
			}
		}
		if !secondCol {
			t.Errorf("%s: the unplotted series' data was dropped from the workbook", name)
		}
	}
}

// TestStockSeriesCountValidated pins the CT_StockChart cardinality: three or
// four c:ser. A two-series stock chart used to serialize happily into a part
// Office reports as damaged.
func TestStockSeriesCountValidated(t *testing.T) {
	mk := func(n int) *Chart {
		c := NewStock().SetCategories([]string{"a", "b"})
		for i := 0; i < n; i++ {
			c.AddSeries("S", []float64{1, 2})
		}
		return c
	}
	for _, n := range []int{1, 2, 5} {
		if _, err := mk(n).MarshalChartXML(); err == nil {
			t.Errorf("stock chart with %d series: expected an error", n)
		}
	}
	for _, n := range []int{3, 4} {
		if _, err := mk(n).MarshalChartXML(); err != nil {
			t.Errorf("stock chart with %d series: %v", n, err)
		}
	}
}

// TestEmptySeriesRejected checks a series with no data points is refused rather
// than emitted as a ptCount="0" cache referencing no cells.
func TestEmptySeriesRejected(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b"})
	c.AddSeries("S", nil)
	if _, err := c.MarshalChartXML(); err == nil {
		t.Fatal("expected an error for a series with no values")
	}
	c2 := NewColumn().SetCategories([]string{"a", "b"})
	c2.AddSeries("Good", []float64{1, 2})
	c2.AddSeries("Empty", []float64{})
	if _, err := c2.MarshalChartXML(); err == nil {
		t.Fatal("expected an error for a chart with one empty series")
	}
}

// TestMarshalEmitsNothingAfterChart pins the invariant InjectExternalData's
// placement rests on: the serializer writes no chartSpace-level element after
// c:chart. If buildChartSpace ever grows one, this fails and points at the
// single injection point rather than at two copies of the same assumption.
func TestMarshalEmitsNothingAfterChart(t *testing.T) {
	c := NewColumn().SetTitle("T").SetCategories([]string{"a", "b"})
	c.AddSeries("S", []float64{1, 2})
	out, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	s := string(out)
	i := strings.LastIndex(s, "</c:chart>")
	if i < 0 {
		t.Fatalf("no c:chart in output:\n%s", s)
	}
	if tail := s[i+len("</c:chart>"):]; tail != "</c:chartSpace>" {
		t.Fatalf("chartSpace has trailing elements %q; InjectExternalData's insertion point must be revisited", tail)
	}
}

// TestInjectExternalDataPlacement checks the injected element lands after
// c:chart (and any spPr/txPr) but before the elements that must follow it, so
// the one shared implementation stays schema-correct if the serializer grows
// trailing content.
func TestInjectExternalDataPlacement(t *testing.T) {
	base := `<c:chartSpace><c:chart><c:plotArea/></c:chart></c:chartSpace>`
	got := string(InjectExternalData([]byte(base), "rId1"))
	want := `<c:chartSpace><c:chart><c:plotArea/></c:chart><c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData></c:chartSpace>`
	if got != want {
		t.Errorf("plain chartSpace:\n got %s\nwant %s", got, want)
	}

	withTail := `<c:chartSpace><c:chart><c:plotArea><c:extLst/></c:plotArea></c:chart><c:spPr/><c:printSettings/></c:chartSpace>`
	got = string(InjectExternalData([]byte(withTail), "rId7"))
	ext := strings.Index(got, "<c:externalData")
	spPr := strings.Index(got, "<c:spPr/>")
	printSettings := strings.Index(got, "<c:printSettings/>")
	if ext < 0 || spPr >= ext || ext >= printSettings {
		t.Errorf("externalData misplaced relative to spPr/printSettings:\n%s", got)
	}

	// A chartSpace-level extLst must also follow externalData, while the one
	// nested in the plot area must not be mistaken for it.
	withExtLst := `<c:chartSpace><c:chart><c:plotArea><c:extLst/></c:plotArea></c:chart><c:extLst/></c:chartSpace>`
	got = string(InjectExternalData([]byte(withExtLst), "rId2"))
	if strings.Index(got, "<c:externalData") > strings.LastIndex(got, "<c:extLst/>") {
		t.Errorf("externalData emitted after the chartSpace extLst:\n%s", got)
	}

	// No close tag: the input is returned unchanged.
	if got := string(InjectExternalData([]byte("<c:chartSpace>"), "rId1")); got != "<c:chartSpace>" {
		t.Errorf("truncated input was modified: %s", got)
	}
}

// TestAxisOrderIsDeliberate documents that the plot area writes c:valAx before
// c:catAx. The schema's sequence is a repeatable choice over the axis elements,
// so both orders validate and Office accepts this one; TestChartXMLShape pins
// it. It is recorded here so nobody "fixes" the fixtures to Office's order
// without deciding to change the serializer first.
func TestAxisOrderIsDeliberate(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b"})
	c.AddSeries("S", []float64{1, 2})
	out, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	s := string(out)
	val, cat := strings.Index(s, "<c:valAx>"), strings.Index(s, "<c:catAx>")
	if val < 0 || cat < 0 || val > cat {
		t.Fatalf("expected valAx before catAx (deliberate, see the comment in buildAxes):\n%s", s)
	}
}
