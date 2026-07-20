package chart

import (
	"bytes"
	"testing"
)

// TestRoundTripCategoryKindsExtra builds each newly wired category-style chart
// type (3D bars/line/area, pie-of-pie, 3D pie, stock, surface), marshals it,
// checks the distinguishing c: element is emitted, then parses it back and
// verifies the kind, title, categories, and series survive.
func TestRoundTripCategoryKindsExtra(t *testing.T) {
	cats := []string{"Q1", "Q2", "Q3", "Q4"}
	s1 := []float64{10, 20, 30, 40}
	s2 := []float64{5, 15, 25, 2.5}

	cases := []struct {
		name     string
		make     func() *Chart
		kind     Kind
		want     []string // substrings the output must contain, in any order
		single   bool     // true for the pie-family kinds that plot one series
		hasSerAx bool
	}{
		{"column3d", NewColumn3D, KindColumn3D, []string{"<c:bar3DChart>", "<c:barDir val=\"col\"/>", "<c:view3D>", "<c:serAx>"}, false, true},
		{"bar3d", NewBar3D, KindBar3D, []string{"<c:bar3DChart>", "<c:barDir val=\"bar\"/>", "<c:serAx>"}, false, true},
		{"line3d", NewLine3D, KindLine3D, []string{"<c:line3DChart>", "<c:view3D>", "<c:serAx>"}, false, true},
		{"area3d", NewArea3D, KindArea3D, []string{"<c:area3DChart>", "<c:view3D>", "<c:serAx>"}, false, true},
		{"pie3d", NewPie3D, KindPie3D, []string{"<c:pie3DChart>", "<c:view3D>"}, true, false},
		{"ofpie", NewOfPie, KindOfPie, []string{"<c:ofPieChart>", "<c:ofPieType val=\"pie\"/>"}, true, false},
		{"stock", NewStock, KindStock, []string{"<c:stockChart>", "<c:hiLowLines/>", "<c:catAx>", "<c:valAx>"}, false, false},
		{"surface", NewSurface, KindSurface, []string{"<c:surfaceChart>", "<c:serAx>"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.make().SetTitle("T " + tc.name).SetCategories(cats)
			c.AddSeries("North", s1)
			c.AddSeries("South", s2)

			xmlBytes, err := c.MarshalChartXML()
			if err != nil {
				t.Fatalf("MarshalChartXML: %v", err)
			}
			for _, want := range tc.want {
				if !bytes.Contains(xmlBytes, []byte(want)) {
					t.Errorf("output missing %q\n%s", want, xmlBytes)
				}
			}
			if tc.hasSerAx != bytes.Contains(xmlBytes, []byte("<c:serAx>")) {
				t.Errorf("serAx presence: got %v want %v", !tc.hasSerAx, tc.hasSerAx)
			}

			got, err := Parse(xmlBytes)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got.Kind() != tc.kind {
				t.Errorf("kind: got %v want %v", got.Kind(), tc.kind)
			}
			if got.Title() != "T "+tc.name {
				t.Errorf("title: got %q", got.Title())
			}
			if !stringsEqual(got.Categories(), cats) {
				t.Errorf("categories: got %v want %v", got.Categories(), cats)
			}
			gs := got.SeriesList()
			wantN := 2
			if tc.single {
				wantN = 1
			}
			if len(gs) != wantN {
				t.Fatalf("series count: got %d want %d", len(gs), wantN)
			}
			if gs[0].Name != "North" || !floatsEqual(gs[0].Values, s1) {
				t.Errorf("series 0: got %q %v", gs[0].Name, gs[0].Values)
			}
		})
	}
}

// TestRoundTripBubble builds a bubble chart with two x/y/size series, checks it
// emits a c:bubbleChart with bubbleSize sources and two value axes, then reads
// it back as KindBubble with X, Y, and sizes intact.
func TestRoundTripBubble(t *testing.T) {
	x := []float64{1, 2, 3}
	y1 := []float64{4, 5, 6}
	sz1 := []float64{10, 20, 30}
	y2 := []float64{7, 8, 9}
	sz2 := []float64{5, 15, 25}

	c := NewBubble().SetTitle("Bubbles").SetAxisTitles("X", "Y")
	c.AddBubbleSeries("A", x, y1, sz1)
	c.AddBubbleSeries("B", x, y2, sz2)

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, want := range []string{"<c:bubbleChart>", "<c:bubbleSize>", "<c:xVal>", "<c:yVal>"} {
		if !bytes.Contains(xmlBytes, []byte(want)) {
			t.Errorf("output missing %q\n%s", want, xmlBytes)
		}
	}
	// A bubble chart uses two value axes and no category axis.
	if bytes.Contains(xmlBytes, []byte("<c:catAx>")) {
		t.Error("bubble chart should have no category axis")
	}
	if bytes.Count(xmlBytes, []byte("<c:valAx>")) != 2 {
		t.Errorf("bubble chart should have two value axes: %s", xmlBytes)
	}

	got, err := Parse(xmlBytes)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Kind() != KindBubble {
		t.Errorf("kind: got %v want bubble", got.Kind())
	}
	gs := got.SeriesList()
	if len(gs) != 2 {
		t.Fatalf("series count: got %d want 2", len(gs))
	}
	if gs[0].Name != "A" || !floatsEqual(gs[0].XValues, x) || !floatsEqual(gs[0].Values, y1) || !floatsEqual(gs[0].Sizes, sz1) {
		t.Errorf("series 0: name=%q x=%v y=%v size=%v", gs[0].Name, gs[0].XValues, gs[0].Values, gs[0].Sizes)
	}
	if gs[1].Name != "B" || !floatsEqual(gs[1].XValues, x) || !floatsEqual(gs[1].Values, y2) || !floatsEqual(gs[1].Sizes, sz2) {
		t.Errorf("series 1: name=%q x=%v y=%v size=%v", gs[1].Name, gs[1].XValues, gs[1].Values, gs[1].Sizes)
	}
	ct, vt := got.AxisTitles()
	if ct != "X" || vt != "Y" {
		t.Errorf("axis titles: got %q/%q", ct, vt)
	}
}

// TestBubbleEmbeddedLayout checks the bubble chart's data layout places X in
// column A and each series' Y and size in adjacent columns, and that the
// emitted references match.
func TestBubbleEmbeddedLayout(t *testing.T) {
	c := NewBubble()
	c.AddBubbleSeries("A", []float64{1, 2}, []float64{3, 4}, []float64{5, 6})
	c.AddBubbleSeries("B", []float64{1, 2}, []float64{7, 8}, []float64{9, 10})

	layout := c.Layout()
	want := []struct{ x, y, size, name string }{
		{"Sheet1!$A$2:$A$3", "Sheet1!$B$2:$B$3", "Sheet1!$C$2:$C$3", "Sheet1!$B$1"},
		{"Sheet1!$A$2:$A$3", "Sheet1!$D$2:$D$3", "Sheet1!$E$2:$E$3", "Sheet1!$D$1"},
	}
	for i, w := range want {
		sl := layout.Series[i]
		if sl.XValuesRef != w.x || sl.ValuesRef != w.y || sl.SizesRef != w.size || sl.NameRef != w.name {
			t.Errorf("series %d layout: got x=%q y=%q size=%q name=%q", i, sl.XValuesRef, sl.ValuesRef, sl.SizesRef, sl.NameRef)
		}
	}

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	for _, ref := range []string{"Sheet1!$C$2:$C$3", "Sheet1!$E$2:$E$3"} {
		if !bytes.Contains(xmlBytes, []byte("<c:f>"+ref+"</c:f>")) {
			t.Errorf("chart.xml missing size ref %q", ref)
		}
	}
}

// TestNewKindStringsExtra checks the Kind stringer names the newly wired types.
func TestNewKindStringsExtra(t *testing.T) {
	cases := map[Kind]string{
		KindBubble:   "bubbleChart",
		KindStock:    "stockChart",
		KindSurface:  "surfaceChart",
		KindOfPie:    "ofPieChart",
		KindColumn3D: "bar3DChart",
		KindBar3D:    "bar3DChart",
		KindLine3D:   "line3DChart",
		KindPie3D:    "pie3DChart",
		KindArea3D:   "area3DChart",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(k), got, want)
		}
	}
}
