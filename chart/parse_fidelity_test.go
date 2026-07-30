package chart

import (
	"strings"
	"testing"
)

// stackedBarChartXML is a stacked column chart with a currency format code in
// its value cache — the two properties Parse must recover for a read-modify-
// re-embed round trip not to change how the data is presented (C560).
const stackedBarChartXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <c:chart>
    <c:plotArea>
      <c:layout/>
      <c:barChart>
        <c:barDir val="col"/>
        <c:grouping val="stacked"/>
        <c:ser>
          <c:idx val="0"/>
          <c:order val="0"/>
          <c:tx><c:strRef><c:f>Sheet1!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>North</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:cat><c:strRef><c:f>Sheet1!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt></c:strCache></c:strRef></c:cat>
          <c:val><c:numRef><c:f>Sheet1!$B$2:$B$3</c:f><c:numCache><c:formatCode>&quot;$&quot;#,##0.00</c:formatCode><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
        <c:axId val="1"/>
        <c:axId val="2"/>
      </c:barChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

// TestParseRecoversGroupingAndFormat pins C560's two data-presentation
// properties: a stacked chart read back and re-marshaled must still be stacked,
// and its cached number format must survive rather than reverting to "General".
func TestParseRecoversGroupingAndFormat(t *testing.T) {
	c, err := Parse([]byte(stackedBarChartXML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Grouping() != GroupingStacked {
		t.Errorf("Grouping() = %q, want %q", c.Grouping(), GroupingStacked)
	}
	if c.NumberFormat != `"$"#,##0.00` {
		t.Errorf("NumberFormat = %q, want the cached format code", c.NumberFormat)
	}

	out, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `<c:grouping val="stacked"/>`) {
		t.Errorf("re-marshaled chart is no longer stacked:\n%s", s)
	}
	if !strings.Contains(s, `<c:overlap val="100"/>`) {
		t.Errorf("stacked bars need full overlap:\n%s", s)
	}
	if !strings.Contains(s, `<c:formatCode>&#34;$&#34;#,##0.00</c:formatCode>`) &&
		!strings.Contains(s, `<c:formatCode>"$"#,##0.00</c:formatCode>`) {
		t.Errorf("re-marshaled chart lost the number format:\n%s", s)
	}
}

// TestGroupingRoundTrips covers the whole enumeration for both the bar family
// (ST_BarGrouping) and the line/area family (ST_Grouping), including the
// clustered value that has no line/area equivalent.
func TestGroupingRoundTrips(t *testing.T) {
	cases := []struct {
		make    func() *Chart
		set     Grouping
		wantXML string
		wantGot Grouping
	}{
		{NewColumn, GroupingStacked, `<c:grouping val="stacked"/>`, GroupingStacked},
		{NewColumn, GroupingPercentStacked, `<c:grouping val="percentStacked"/>`, GroupingPercentStacked},
		{NewBar, GroupingClustered, `<c:grouping val="clustered"/>`, GroupingClustered},
		{NewLine, GroupingStacked, `<c:grouping val="stacked"/>`, GroupingStacked},
		{NewArea, GroupingPercentStacked, `<c:grouping val="percentStacked"/>`, GroupingPercentStacked},
		// A line chart has no clustered form: it falls back to standard.
		{NewLine, GroupingClustered, `<c:grouping val="standard"/>`, GroupingStandard},
	}
	for _, tc := range cases {
		c := tc.make().SetCategories([]string{"a", "b"}).SetGrouping(tc.set)
		c.AddSeries("S", []float64{1, 2})
		out, err := c.MarshalChartXML()
		if err != nil {
			t.Fatalf("MarshalChartXML: %v", err)
		}
		if !strings.Contains(string(out), tc.wantXML) {
			t.Errorf("grouping %q: output missing %s", tc.set, tc.wantXML)
		}
		got, err := Parse(out)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if got.Grouping() != tc.wantGot {
			t.Errorf("grouping %q: read back %q, want %q", tc.set, got.Grouping(), tc.wantGot)
		}
	}
}

// barLineComboXML is a plot area holding a horizontal-bar group and a line
// group — a combination Excel can produce and Parse recovers as KindCombo with
// a KindBar series.
const barLineComboXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <c:chart>
    <c:plotArea>
      <c:layout/>
      <c:barChart>
        <c:barDir val="bar"/>
        <c:grouping val="clustered"/>
        <c:ser>
          <c:idx val="0"/>
          <c:order val="0"/>
          <c:tx><c:strRef><c:f>Sheet1!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Bars</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:cat><c:strRef><c:f>Sheet1!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt></c:strCache></c:strRef></c:cat>
          <c:val><c:numRef><c:f>Sheet1!$B$2:$B$3</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
        <c:axId val="1"/>
        <c:axId val="2"/>
      </c:barChart>
      <c:lineChart>
        <c:grouping val="standard"/>
        <c:ser>
          <c:idx val="1"/>
          <c:order val="1"/>
          <c:tx><c:strRef><c:f>Sheet1!$C$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Line</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:cat><c:strRef><c:f>Sheet1!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt></c:strCache></c:strRef></c:cat>
          <c:val><c:numRef><c:f>Sheet1!$C$2:$C$3</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
        <c:axId val="1"/>
        <c:axId val="2"/>
      </c:lineChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

// TestBarLineComboReMarshals pins C557: a combo carrying a horizontal-bar group
// parsed fine but could never be written back — buildComboPlot rejected
// KindBar, so re-embedding such a chart with AddChart always failed.
func TestBarLineComboReMarshals(t *testing.T) {
	c, err := Parse([]byte(barLineComboXML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Kind() != KindCombo {
		t.Fatalf("Kind() = %v, want combo", c.Kind())
	}
	series := c.SeriesList()
	if len(series) != 2 {
		t.Fatalf("series = %d, want 2", len(series))
	}
	if series[0].PlotType != KindBar {
		t.Errorf("series 0 plot type = %v, want bar", series[0].PlotType)
	}

	out, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("re-marshaling a parsed bar+line combo failed: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `<c:barDir val="bar"/>`) {
		t.Errorf("bar direction lost on re-marshal:\n%s", s)
	}
	if !strings.Contains(s, "<c:lineChart>") {
		t.Errorf("line group lost on re-marshal:\n%s", s)
	}

	back, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse of re-marshaled combo: %v", err)
	}
	if bs := back.SeriesList(); len(bs) != 2 || bs[0].PlotType != KindBar || bs[1].PlotType != KindLine {
		t.Errorf("round-tripped combo series: %+v", bs)
	}
}

// mixedPlotAreaXML holds a bar group and a scatter group. Only the bar data can
// be represented, and the loss must be reported rather than silent (C563).
const mixedPlotAreaXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <c:chart>
    <c:plotArea>
      <c:layout/>
      <c:barChart>
        <c:barDir val="col"/>
        <c:grouping val="clustered"/>
        <c:ser>
          <c:idx val="0"/>
          <c:order val="0"/>
          <c:tx><c:strRef><c:f>Sheet1!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Bars</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:val><c:numRef><c:f>Sheet1!$B$2:$B$3</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
        <c:axId val="1"/>
        <c:axId val="2"/>
      </c:barChart>
      <c:scatterChart>
        <c:scatterStyle val="lineMarker"/>
        <c:ser>
          <c:idx val="1"/>
          <c:order val="1"/>
          <c:tx><c:strRef><c:f>Sheet1!$D$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Points</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:xVal><c:numRef><c:f>Sheet1!$C$2:$C$3</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt></c:numCache></c:numRef></c:xVal>
          <c:yVal><c:numRef><c:f>Sheet1!$D$2:$D$3</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>3</c:v></c:pt><c:pt idx="1"><c:v>4</c:v></c:pt></c:numCache></c:numRef></c:yVal>
        </c:ser>
        <c:axId val="3"/>
        <c:axId val="4"/>
      </c:scatterChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

// TestDroppedGroupsAreReported pins C563: the scatter group the model cannot
// carry alongside the bars must show up in ParseNotes, so a caller can tell
// "this is a column chart" from "it had two groups and I kept one".
func TestDroppedGroupsAreReported(t *testing.T) {
	c, err := Parse([]byte(mixedPlotAreaXML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Kind() != KindColumn {
		t.Fatalf("Kind() = %v, want column", c.Kind())
	}
	notes := c.ParseNotes()
	if len(notes) != 1 {
		t.Fatalf("ParseNotes() = %v, want one note about the dropped scatter group", notes)
	}
	if !strings.Contains(notes[0], "scatterChart") {
		t.Errorf("note does not name the dropped group: %q", notes[0])
	}
}

// TestNoNotesForRepresentableChart checks the notes stay empty when nothing was
// dropped — including for a combination chart, whose bar, line, and area groups
// are all consumed.
func TestNoNotesForRepresentableChart(t *testing.T) {
	c := NewCombo().SetCategories([]string{"a", "b"})
	c.AddSeries("Bars", []float64{1, 2}).SetType(KindColumn)
	c.AddSeries("Line", []float64{3, 4}).SetType(KindLine)
	out, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	got, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if notes := got.ParseNotes(); len(notes) != 0 {
		t.Errorf("ParseNotes() = %v, want none", notes)
	}
	if notes := c.ParseNotes(); len(notes) != 0 {
		t.Errorf("a built chart has notes: %v", notes)
	}
}

// TestRepeatedGroupOfSameTypeReported covers the other half of C563: several
// groups of the same non-combo type, of which only index 0 is kept.
func TestRepeatedGroupOfSameTypeReported(t *testing.T) {
	doubled := strings.Replace(mixedPlotAreaXML,
		`<c:scatterChart>`,
		`<c:pieChart><c:ser><c:idx val="9"/></c:ser></c:pieChart><c:scatterChart>`, 1)
	c, err := Parse([]byte(doubled))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	notes := c.ParseNotes()
	if len(notes) != 2 {
		t.Fatalf("ParseNotes() = %v, want notes for both the pie and the scatter group", notes)
	}
}

// --- data-reference recovery (found by FuzzParseChartXML on corpus charts) ---

// TestSheetOfDecodesTheQuotedForm pins the two reference forms whose sheet name
// was returned in its *quoted* lexical shape. DataRef holds an unquoted name —
// SetDataRef takes one and quoteSheet re-quotes it on the way out — so leaving
// the escaping on meant every save re-escaped it: one save already pointed the
// chart at a sheet that does not exist, and the apostrophe run then doubled on
// each save after that.
func TestSheetOfDecodesTheQuotedForm(t *testing.T) {
	cases := map[string]string{
		`Sheet1!$A$1`:                                "Sheet1",
		`'My Sheet'!$A$2:$A$5`:                       "My Sheet",
		`'Vue d''ensemble FC'!$B$1`:                  "Vue d'ensemble FC",
		`'O''Brien''s'!$B$1`:                         "O'Brien's",
		`('Ship By Ship'!$Z$2,'Ship By Ship'!$AD$2)`: "Ship By Ship",
		`(Data!$Z$2,Data!$AD$2)`:                     "Data",
		`$A$1`:                                       "",
	}
	for ref, want := range cases {
		if got := sheetOf(ref); got != want {
			t.Errorf("sheetOf(%q) = %q, want %q", ref, got, want)
		}
	}
}

// TestQuotedSheetNameSurvivesTwoSaves is the end-to-end form of the same
// defect: the reference a second save writes must equal the one the first
// wrote. Before the fix the apostrophe run grew on every pass.
func TestQuotedSheetNameSurvivesTwoSaves(t *testing.T) {
	src := strings.ReplaceAll(stackedBarChartXML, `Sheet1!`, `'Vue d''ensemble FC'!`)
	c, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.DataRef != "Vue d'ensemble FC" {
		t.Fatalf("DataRef = %q, want the sheet's own name", c.DataRef)
	}
	first, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	if !strings.Contains(string(first), `<c:f>'Vue d''ensemble FC'!$B$1</c:f>`) {
		t.Errorf("first save did not reproduce the sheet reference:\n%s", first)
	}
	again, err := Parse(first)
	if err != nil {
		t.Fatalf("Parse of our own output: %v", err)
	}
	second, err := again.MarshalChartXML()
	if err != nil {
		t.Fatalf("second MarshalChartXML: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("the reference changed on the second save:\n%s\n%s", first, second)
	}
}

// TestScatterKeepsItsSheetAcrossTwoSaves pins the value-source fallback in
// firstDataRef. A scatter series whose X source is categorical parses to no
// XValues, so the next save writes c:xVal as a literal with no c:f; recovering
// the sheet from the X source alone therefore lost it on the second read and
// every reference silently moved to the default "Sheet1".
func TestScatterKeepsItsSheetAcrossTwoSaves(t *testing.T) {
	const src = `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<c:chart><c:plotArea><c:layout/><c:scatterChart><c:scatterStyle val="lineMarker"/>` +
		`<c:ser><c:idx val="0"/><c:order val="0"/>` +
		`<c:tx><c:strRef><c:f>'Through Egypt'!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Trip</c:v></c:pt></c:strCache></c:strRef></c:tx>` +
		`<c:xVal><c:strRef><c:f>'Through Egypt'!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Jan</c:v></c:pt><c:pt idx="1"><c:v>Feb</c:v></c:pt></c:strCache></c:strRef></c:xVal>` +
		`<c:yVal><c:numRef><c:f>'Through Egypt'!$B$2:$B$3</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>1.5</c:v></c:pt><c:pt idx="1"><c:v>2.5</c:v></c:pt></c:numCache></c:numRef></c:yVal>` +
		`</c:ser><c:axId val="1"/><c:axId val="2"/></c:scatterChart></c:plotArea></c:chart></c:chartSpace>`

	c, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	first, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	again, err := Parse(first)
	if err != nil {
		t.Fatalf("Parse of our own output: %v", err)
	}
	if again.DataRef != "Through Egypt" {
		t.Fatalf("DataRef after a save/read cycle = %q, want %q", again.DataRef, "Through Egypt")
	}
	second, err := again.MarshalChartXML()
	if err != nil {
		t.Fatalf("second MarshalChartXML: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("the chart's references changed on the second save:\n%s\n%s", first, second)
	}
}

// TestCachePointCountIsClamped pins the allocation amplification
// FuzzParseChartXML found: a cache's ptCount is a length field in the file's
// own bytes, and sizing the value slice from it let a 374-byte part drive a
// 400 MB allocation. The clamp keeps every count a producer can write exact and
// truncates the rest.
func TestCachePointCountIsClamped(t *testing.T) {
	build := func(ptCount string) []byte {
		return []byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
			`<c:chart><c:plotArea><c:layout/><c:barChart><c:barDir val="col"/>` +
			`<c:ser><c:idx val="0"/><c:order val="0"/>` +
			`<c:val><c:numLit><c:ptCount val="` + ptCount + `"/><c:pt idx="0"><c:v>1</c:v></c:pt></c:numLit></c:val>` +
			`</c:ser><c:axId val="1"/><c:axId val="2"/></c:barChart></c:plotArea></c:chart></c:chartSpace>`)
	}
	for _, tc := range []struct {
		ptCount string
		want    int
	}{
		{"3", 3},
		{"32000", 32000}, // Excel's own per-series limit stays exact
		{"50000000", maxCachePoints},
		{"4294967295", maxCachePoints},
	} {
		c, err := Parse(build(tc.ptCount))
		if err != nil {
			t.Fatalf("ptCount=%s: Parse: %v", tc.ptCount, err)
		}
		if got := len(c.SeriesList()[0].Values); got != tc.want {
			t.Errorf("ptCount=%s: %d values, want %d", tc.ptCount, got, tc.want)
		}
	}
	// A sparse point index is a length field too.
	c, err := Parse([]byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<c:chart><c:plotArea><c:layout/><c:barChart><c:barDir val="col"/>` +
		`<c:ser><c:idx val="0"/><c:order val="0"/>` +
		`<c:val><c:numLit><c:pt idx="2000000000"><c:v>1</c:v></c:pt></c:numLit></c:val>` +
		`</c:ser><c:axId val="1"/><c:axId val="2"/></c:barChart></c:plotArea></c:chart></c:chartSpace>`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := len(c.SeriesList()[0].Values); got != maxCachePoints {
		t.Errorf("sparse idx: %d values, want the clamp %d", got, maxCachePoints)
	}
}
