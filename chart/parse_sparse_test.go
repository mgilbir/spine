package chart

import (
	"math"
	"testing"
)

// chartWithSparseCache is a minimal column chart whose value numCache declares
// ptCount=3 but carries points only at idx 0 and idx 2 — the shape Excel emits
// when the middle cell is blank. The idx-2 value must land at position 2 (under
// category "c"), leaving position 1 (under "b") a blank placeholder, rather than
// collapsing to [v0 v2] and misaligning the series against its categories.
const chartWithSparseCache = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
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
          <c:cat><c:strRef><c:f>Sheet1!$A$2:$A$4</c:f><c:strCache><c:ptCount val="3"/><c:pt idx="0"><c:v>a</c:v></c:pt><c:pt idx="1"><c:v>b</c:v></c:pt><c:pt idx="2"><c:v>c</c:v></c:pt></c:strCache></c:strRef></c:cat>
          <c:val><c:numRef><c:f>Sheet1!$B$2:$B$4</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="3"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="2"><c:v>30</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
        <c:axId val="1"/>
        <c:axId val="2"/>
      </c:barChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

// TestParseSparseNumCache pins the sparse-cache alignment fix (C250): a numCache
// with ptCount=3 and points only at idx 0 and 2 parses to three values with the
// idx-2 value at position 2 and a blank (NaN) placeholder at position 1.
func TestParseSparseNumCache(t *testing.T) {
	c, err := Parse([]byte(chartWithSparseCache))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !stringsEqual(c.Categories(), []string{"a", "b", "c"}) {
		t.Fatalf("Categories = %v, want [a b c]", c.Categories())
	}
	series := c.SeriesList()
	if len(series) != 1 {
		t.Fatalf("len(Series) = %d, want 1", len(series))
	}
	vals := series[0].Values
	if len(vals) != 3 {
		t.Fatalf("len(Values) = %d, want 3 (aligned with categories): %v", len(vals), vals)
	}
	if vals[0] != 10 {
		t.Errorf("Values[0] = %v, want 10", vals[0])
	}
	if !math.IsNaN(vals[1]) {
		t.Errorf("Values[1] = %v, want NaN placeholder for the blank cell", vals[1])
	}
	if vals[2] != 30 {
		t.Errorf("Values[2] = %v, want 30 (idx-2 value under category \"c\", not shifted)", vals[2])
	}
}
