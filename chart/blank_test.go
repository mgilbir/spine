package chart

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// TestBlankSentinel checks the documented predicate: NaN (what Blank returns
// and what Parse fills gaps with) and the infinities are blanks; ordinary
// numbers, including zero and negatives, are not.
func TestBlankSentinel(t *testing.T) {
	if !IsBlank(Blank()) {
		t.Error("IsBlank(Blank()) = false, want true")
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if !IsBlank(v) {
			t.Errorf("IsBlank(%v) = false, want true", v)
		}
	}
	for _, v := range []float64{0, -1, 1e308, 2.5} {
		if IsBlank(v) {
			t.Errorf("IsBlank(%v) = true, want false", v)
		}
	}
}

// TestBlankPointOmittedFromCache pins C384 at the cache: a blank value must
// leave its c:pt out (Excel's own convention for an empty cell) while ptCount
// still counts it, instead of serializing the sentinel as <c:v>NaN</c:v>.
func TestBlankPointOmittedFromCache(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b", "c"})
	c.AddSeries("S", []float64{10, Blank(), 30})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	if bytes.Contains(xmlBytes, []byte("NaN")) {
		t.Errorf("chart.xml carries the blank sentinel literally:\n%s", xmlBytes)
	}
	if !bytes.Contains(xmlBytes, []byte(`<c:ptCount val="3"/>`)) {
		t.Errorf("ptCount must still count the blank position:\n%s", xmlBytes)
	}
	numCache := between(string(xmlBytes), "<c:numCache>", "</c:numCache>")
	if numCache == "" {
		t.Fatalf("no numCache in output:\n%s", xmlBytes)
	}
	for _, want := range []string{`<c:pt idx="0"><c:v>10</c:v></c:pt>`, `<c:pt idx="2"><c:v>30</c:v></c:pt>`} {
		if !strings.Contains(numCache, want) {
			t.Errorf("numCache missing %q: %s", want, numCache)
		}
	}
	if strings.Contains(numCache, `<c:pt idx="1"`) {
		t.Errorf("blank point emitted a value at idx 1: %s", numCache)
	}
}

// between returns the text between the first open and the following close
// marker, or "" when either is missing.
func between(s, open, close string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// TestBlankPointHasNoDataCell pins C384 at the data layer: DataCells is the one
// source the embedded workbook and the xlsx host sheet write from, so a blank
// must produce no cell at all rather than a NaN number cell (which degrades to
// a #NUM! error cell in the host sheet).
func TestBlankPointHasNoDataCell(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b", "c"})
	c.AddSeries("S", []float64{10, Blank(), 30})

	for _, dc := range c.DataCells() {
		if !dc.IsText && IsBlank(dc.Number) {
			t.Fatalf("DataCells emitted a blank cell at col %d row %d", dc.Col, dc.Row)
		}
		if !dc.IsText && dc.Row == 3 {
			t.Fatalf("DataCells emitted a value for the blank row: %+v", dc)
		}
	}

	sheetXML := string(mustMarshalSheet(t, c))
	if strings.Contains(sheetXML, "NaN") {
		t.Errorf("embedded worksheet carries NaN:\n%s", sheetXML)
	}
	if !strings.Contains(sheetXML, `<c r="B2"><v>10</v></c>`) || !strings.Contains(sheetXML, `<c r="B4"><v>30</v></c>`) {
		t.Errorf("embedded worksheet missing the present values:\n%s", sheetXML)
	}
	if strings.Contains(sheetXML, `r="B3"`) {
		t.Errorf("embedded worksheet wrote a cell for the blank point:\n%s", sheetXML)
	}
}

// TestBlankRoundTripsThroughParse checks the sentinel survives the whole loop:
// a chart built with a blank marshals to an omitted point, and Parse turns that
// omission back into a blank at the same index.
func TestBlankRoundTripsThroughParse(t *testing.T) {
	c := NewLine().SetCategories([]string{"a", "b", "c", "d"})
	c.AddSeries("S", []float64{1, Blank(), 3, Blank()})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	vals := got.SeriesList()[0].Values
	if len(vals) != 4 {
		t.Fatalf("Values = %v, want 4 aligned with the categories", vals)
	}
	if vals[0] != 1 || vals[2] != 3 {
		t.Errorf("present values shifted: %v", vals)
	}
	if !IsBlank(vals[1]) || !IsBlank(vals[3]) {
		t.Errorf("blank positions not recovered: %v", vals)
	}
}

// TestNonFiniteValueIsBlank checks the write paths never emit ±Inf, which has
// no SpreadsheetML representation either.
func TestNonFiniteValueIsBlank(t *testing.T) {
	c := NewColumn().SetCategories([]string{"a", "b"})
	c.AddSeries("S", []float64{math.Inf(1), 5})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	if bytes.Contains(xmlBytes, []byte("Inf")) {
		t.Errorf("chart.xml carries an infinity:\n%s", xmlBytes)
	}
	if strings.Contains(string(mustMarshalSheet(t, c)), "Inf") {
		t.Error("embedded worksheet carries an infinity")
	}
}

// mustMarshalSheet marshals a chart's embedded worksheet, failing the test when
// the Builder refuses to write it.
func mustMarshalSheet(t *testing.T, c *Chart) []byte {
	t.Helper()
	data, err := c.marshalEmbeddedSheet()
	if err != nil {
		t.Fatalf("marshalEmbeddedSheet: %v", err)
	}
	return data
}
