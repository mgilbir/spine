package chart

import (
	"strings"
	"testing"
)

// TestCategoryChartWithoutCategoriesHasFormula pins C433: a category chart
// whose categories were never set must still give every c:numRef a c:f child —
// CT_NumRef requires it, and without one the references and the data sheet
// describe different things ("Edit Data" opens nothing).
func TestCategoryChartWithoutCategoriesHasFormula(t *testing.T) {
	c := NewPie()
	c.AddSeries("Cases", []float64{18, 2, 66})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	s := string(xmlBytes)
	if strings.Contains(s, "<c:numRef><c:numCache>") {
		t.Errorf("emitted a c:numRef with no c:f child:\n%s", s)
	}
	if !strings.Contains(s, "<c:f>Sheet1!$B$2:$B$4</c:f>") {
		t.Errorf("value reference missing or wrong:\n%s", s)
	}
	// And the reference points at the cells the data actually lands in.
	var b2 bool
	for _, dc := range c.DataCells() {
		if dc.Col == 2 && dc.Row == 2 && dc.Number == 18 {
			b2 = true
		}
	}
	if !b2 {
		t.Errorf("value 18 is not written to B2: %+v", c.DataCells())
	}
}

// TestNoRefEmitsLiteralCache checks the fallback for a source that genuinely
// has no cells to point at: a scatter series with X values but no Y. The cache
// must be emitted as c:numLit, never as a c:numRef missing its formula.
func TestNoRefEmitsLiteralCache(t *testing.T) {
	c := NewScatter()
	c.AddXYSeries("S", []float64{1, 2, 3}, nil)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	s := string(xmlBytes)
	if strings.Contains(s, "<c:numRef><c:numCache>") {
		t.Errorf("emitted a c:numRef with no c:f child:\n%s", s)
	}
	if !strings.Contains(s, "<c:numLit>") {
		t.Errorf("expected a literal cache for the empty Y source:\n%s", s)
	}
}

// TestSeriesRefSpansItsOwnPoints pins C434: the value reference, the cache's
// ptCount, and the cells written must all describe the same points. Sizing the
// reference from the category count instead made a 5-value series claim a
// 3-row range, and Excel drops the tail on refresh.
func TestSeriesRefSpansItsOwnPoints(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b", "c"})
	c.AddSeries("S", []float64{1, 2, 3, 4, 5})

	layout := c.Layout()
	if got := layout.Series[0].ValuesRef; got != "Sheet1!$B$2:$B$6" {
		t.Errorf("values ref = %q, want Sheet1!$B$2:$B$6 (5 points)", got)
	}

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	s := string(xmlBytes)
	if !strings.Contains(s, "<c:f>Sheet1!$B$2:$B$6</c:f>") {
		t.Errorf("chart.xml value reference does not span all five points:\n%s", s)
	}
	numCache := between(s, "<c:numCache>", "</c:numCache>")
	if !strings.Contains(numCache, `<c:ptCount val="5"/>`) {
		t.Errorf("numCache ptCount disagrees with the reference: %s", numCache)
	}

	rows := map[int]bool{}
	for _, dc := range c.DataCells() {
		if dc.Col == 2 && !dc.IsText {
			rows[dc.Row] = true
		}
	}
	if len(rows) != 5 {
		t.Errorf("data cells for the series = %d, want 5", len(rows))
	}
}

// TestShortSeriesRefSpansItsOwnPoints is the other half of C434: a series
// shorter than the category list must not claim rows it has no values for.
func TestShortSeriesRefSpansItsOwnPoints(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b", "c", "d"})
	c.AddSeries("S", []float64{1, 2})

	if got := c.Layout().Series[0].ValuesRef; got != "Sheet1!$B$2:$B$3" {
		t.Errorf("values ref = %q, want Sheet1!$B$2:$B$3 (2 points)", got)
	}
}

// TestAmbiguousSheetNamesAreQuoted pins C558: a sheet name that lexes as a cell
// reference or a boolean literal must be quoted, or Excel reads `A1!$B$1` as a
// cell rather than as a sheet reference.
func TestAmbiguousSheetNamesAreQuoted(t *testing.T) {
	cases := map[string]string{
		"A1":     "'A1'!$B$2:$B$3",
		"XFD1":   "'XFD1'!$B$2:$B$3",
		"R1C1":   "'R1C1'!$B$2:$B$3",
		"R":      "'R'!$B$2:$B$3",
		"C":      "'C'!$B$2:$B$3",
		"TRUE":   "'TRUE'!$B$2:$B$3",
		"Data":   "Data!$B$2:$B$3",
		"Sheet1": "Sheet1!$B$2:$B$3",
	}
	for name, want := range cases {
		c := NewColumn().SetCategories([]string{"a", "b"}).SetDataRef(name)
		c.AddSeries("S", []float64{1, 2})
		if got := c.Layout().Series[0].ValuesRef; got != want {
			t.Errorf("DataRef %q: values ref = %q, want %q", name, got, want)
		}
		xmlBytes, err := c.MarshalChartXML()
		if err != nil {
			t.Fatalf("DataRef %q: MarshalChartXML: %v", name, err)
		}
		if !strings.Contains(string(xmlBytes), "<c:f>"+want+"</c:f>") {
			t.Errorf("DataRef %q: chart.xml missing %q", name, want)
		}
	}
}

// TestCacheAndSheetAgreeOnNumberText pins C559: the numeric cache and the data
// sheet must render the same value the same way. Formatting the cache with 'g'
// put scientific notation in the cache ("1.234567e+06") while the sheet held
// "1234567" — and Office never writes E-notation in a numCache.
func TestCacheAndSheetAgreeOnNumberText(t *testing.T) {
	values := []float64{1234567, 0.0000125, 1e21}
	c := NewColumn().SetCategories([]string{"a", "b", "c"})
	c.AddSeries("S", values)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	cacheXML := string(xmlBytes)
	sheetXML := string(mustMarshalSheet(t, c))
	for _, want := range []string{"1234567", "0.0000125", "1000000000000000000000"} {
		if !strings.Contains(cacheXML, "<c:v>"+want+"</c:v>") {
			t.Errorf("numeric cache missing %q:\n%s", want, cacheXML)
		}
		if !strings.Contains(sheetXML, "<v>"+want+"</v>") {
			t.Errorf("data sheet missing %q:\n%s", want, sheetXML)
		}
	}
	if strings.Contains(cacheXML, "e+") || strings.Contains(cacheXML, "e-") {
		t.Errorf("numeric cache uses exponent notation:\n%s", cacheXML)
	}
}
