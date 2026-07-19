package chart

import "testing"

// chartWithSignedAxIDs is a minimal but valid c:chartSpace whose axId and
// crossAx elements carry negative signed int32 values. Microsoft PowerPoint and
// Excel routinely emit axis identifiers this way even though the schema types
// c:axId/@val as xsd:unsignedInt, so a real corpus is full of them. Before the
// UnsignedInt.Val widening to int64, encoding/xml rejected the whole part with
// "strconv.ParseUint: parsing \"-2042759216\": invalid syntax", and Charts()
// silently dropped every such chart.
const chartWithSignedAxIDs = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
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
          <c:tx><c:strRef><c:f>Sheet1!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>North</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:cat><c:strRef><c:f>Sheet1!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt></c:strCache></c:strRef></c:cat>
          <c:val><c:numRef><c:f>Sheet1!$B$2:$B$3</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
        <c:axId val="-2042759216"/>
        <c:axId val="-2042956128"/>
      </c:barChart>
      <c:catAx>
        <c:axId val="-2042759216"/>
        <c:scaling><c:orientation val="minMax"/></c:scaling>
        <c:delete val="0"/>
        <c:axPos val="b"/>
        <c:crossAx val="-2042956128"/>
      </c:catAx>
      <c:valAx>
        <c:axId val="-2042956128"/>
        <c:scaling><c:orientation val="minMax"/></c:scaling>
        <c:delete val="0"/>
        <c:axPos val="l"/>
        <c:crossAx val="-2042759216"/>
      </c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

// TestParseSignedAxisIDs pins the fix for the most common wild-corpus chart
// parse failure: negative signed axis identifiers. The whole part must parse
// and the series data must be recovered.
func TestParseSignedAxisIDs(t *testing.T) {
	c, err := Parse([]byte(chartWithSignedAxIDs))
	if err != nil {
		t.Fatalf("Parse rejected a chart with negative axId values: %v", err)
	}
	if c.Kind() != KindColumn {
		t.Errorf("Kind = %v, want %v", c.Kind(), KindColumn)
	}
	if !stringsEqual(c.Categories(), []string{"Q1", "Q2"}) {
		t.Errorf("Categories = %v, want [Q1 Q2]", c.Categories())
	}
	series := c.SeriesList()
	if len(series) != 1 {
		t.Fatalf("len(Series) = %d, want 1", len(series))
	}
	if series[0].Name != "North" {
		t.Errorf("series name = %q, want North", series[0].Name)
	}
	if !floatsEqual(series[0].Values, []float64{10, 20}) {
		t.Errorf("series values = %v, want [10 20]", series[0].Values)
	}
}
