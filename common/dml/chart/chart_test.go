package chart

import (
	"encoding/xml"
	"testing"
)

func TestChartSpace_RoundTrip(t *testing.T) {
	cs := &ChartSpace{
		Lang:           &String{Val: "en-US"},
		RoundedCorners: &Boolean{Val: false},
		Style:          &Style{Val: 2},
		Chart: &Chart{
			AutoTitleDeleted: &Boolean{Val: false},
			PlotVisOnly:     &Boolean{Val: true},
		},
	}
	out, err := xml.Marshal(cs)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var cs2 ChartSpace
	if err := xml.Unmarshal(out, &cs2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestBarChart_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		grouping string
	}{
		{"clustered column", "col", "clustered"},
		{"stacked bar", "bar", "stacked"},
		{"percent stacked", "col", "percentStacked"},
		{"standard", "bar", "standard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &BarChart{
				BarDir:     &BarDir{Val: tt.dir},
				Grouping:   &BarGrouping{Val: tt.grouping},
				VaryColors: &Boolean{Val: false},
				GapWidth:   &GapAmount{Val: 150},
			}
			out, err := xml.Marshal(bc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var bc2 BarChart
			if err := xml.Unmarshal(out, &bc2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestLineChart_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		grouping string
	}{
		{"standard", "standard"},
		{"stacked", "stacked"},
		{"percent stacked", "percentStacked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := &LineChart{
				Grouping:   &Grouping{Val: tt.grouping},
				VaryColors: &Boolean{Val: false},
				Marker:     &Boolean{Val: true},
				Smooth:     &Boolean{Val: false},
			}
			out, err := xml.Marshal(lc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var lc2 LineChart
			if err := xml.Unmarshal(out, &lc2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestPieChart_RoundTrip(t *testing.T) {
	pc := &PieChart{
		VaryColors:    &Boolean{Val: true},
		FirstSliceAng: &UnsignedInt{Val: 0},
	}
	out, err := xml.Marshal(pc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var pc2 PieChart
	if err := xml.Unmarshal(out, &pc2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestDoughnutChart_RoundTrip(t *testing.T) {
	dc := &DoughnutChart{
		VaryColors:    &Boolean{Val: true},
		FirstSliceAng: &UnsignedInt{Val: 0},
		HoleSize:      &HoleSize{Val: 50},
	}
	out, err := xml.Marshal(dc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var dc2 DoughnutChart
	if err := xml.Unmarshal(out, &dc2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestScatterChart_RoundTrip(t *testing.T) {
	styles := []string{"none", "line", "lineMarker", "marker", "smooth", "smoothMarker"}
	for _, style := range styles {
		t.Run(style, func(t *testing.T) {
			sc := &ScatterChart{
				ScatterStyle: &ScatterStyle{Val: style},
				VaryColors:   &Boolean{Val: false},
			}
			out, err := xml.Marshal(sc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var sc2 ScatterChart
			if err := xml.Unmarshal(out, &sc2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestBubbleChart_RoundTrip(t *testing.T) {
	bc := &BubbleChart{
		VaryColors:     &Boolean{Val: false},
		Bubble3D:       &Boolean{Val: true},
		BubbleScale:    &BubbleScale{Val: 100},
		ShowNegBubbles: &Boolean{Val: false},
		SizeRepresents: &SizeRepresents{Val: "area"},
	}
	out, err := xml.Marshal(bc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var bc2 BubbleChart
	if err := xml.Unmarshal(out, &bc2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestRadarChart_RoundTrip(t *testing.T) {
	styles := []string{"standard", "marker", "filled"}
	for _, style := range styles {
		t.Run(style, func(t *testing.T) {
			rc := &RadarChart{
				RadarStyle: &RadarStyle{Val: style},
				VaryColors: &Boolean{Val: false},
			}
			out, err := xml.Marshal(rc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var rc2 RadarChart
			if err := xml.Unmarshal(out, &rc2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestAreaChart_RoundTrip(t *testing.T) {
	ac := &AreaChart{
		Grouping:   &Grouping{Val: "standard"},
		VaryColors: &Boolean{Val: false},
	}
	out, err := xml.Marshal(ac)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ac2 AreaChart
	if err := xml.Unmarshal(out, &ac2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestSurfaceChart_RoundTrip(t *testing.T) {
	sc := &SurfaceChart{Wireframe: &Boolean{Val: false}}
	out, err := xml.Marshal(sc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var sc2 SurfaceChart
	if err := xml.Unmarshal(out, &sc2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestStockChart_RoundTrip(t *testing.T) {
	sc := &StockChart{}
	out, err := xml.Marshal(sc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var sc2 StockChart
	if err := xml.Unmarshal(out, &sc2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestValAx_RoundTrip(t *testing.T) {
	va := &ValAx{
		AxId:    &UnsignedInt{Val: 1},
		Scaling: &Scaling{Orientation: &Orientation{Val: "minMax"}},
		Delete:  &Boolean{Val: false},
		AxPos:   &AxPos{Val: "l"},
		CrossAx: &UnsignedInt{Val: 2},
		Crosses: &Crosses{Val: "autoZero"},
	}
	out, err := xml.Marshal(va)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var va2 ValAx
	if err := xml.Unmarshal(out, &va2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestCatAx_RoundTrip(t *testing.T) {
	ca := &CatAx{
		AxId:    &UnsignedInt{Val: 2},
		Scaling: &Scaling{Orientation: &Orientation{Val: "minMax"}},
		Delete:  &Boolean{Val: false},
		AxPos:   &AxPos{Val: "b"},
		CrossAx: &UnsignedInt{Val: 1},
		Auto:    &Boolean{Val: true},
		LblAlgn: &LblAlgn{Val: "ctr"},
		LblOffset: &LblOffset{Val: 100},
	}
	out, err := xml.Marshal(ca)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ca2 CatAx
	if err := xml.Unmarshal(out, &ca2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestDateAx_RoundTrip(t *testing.T) {
	da := &DateAx{
		AxId:         &UnsignedInt{Val: 3},
		AxPos:        &AxPos{Val: "b"},
		BaseTimeUnit: &TimeUnit{Val: "days"},
	}
	out, err := xml.Marshal(da)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var da2 DateAx
	if err := xml.Unmarshal(out, &da2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestNumRef_RoundTrip(t *testing.T) {
	nr := &NumRef{
		F: "Sheet1!$B$2:$B$5",
		NumCache: &NumData{
			FormatCode: "General",
			PtCount:    &UnsignedInt{Val: 4},
			Pt: []*NumVal{
				{Idx: 0, V: "10"},
				{Idx: 1, V: "20"},
				{Idx: 2, V: "30"},
				{Idx: 3, V: "40"},
			},
		},
	}
	out, err := xml.Marshal(nr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var nr2 NumRef
	if err := xml.Unmarshal(out, &nr2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if nr2.F != "Sheet1!$B$2:$B$5" {
		t.Errorf("F = %q, want %q", nr2.F, "Sheet1!$B$2:$B$5")
	}
}

func TestStrRef_RoundTrip(t *testing.T) {
	sr := &StrRef{
		F: "Sheet1!$A$2:$A$5",
		StrCache: &StrData{
			PtCount: &UnsignedInt{Val: 4},
			Pt: []*StrVal{
				{Idx: 0, V: "Q1"},
				{Idx: 1, V: "Q2"},
				{Idx: 2, V: "Q3"},
				{Idx: 3, V: "Q4"},
			},
		},
	}
	out, err := xml.Marshal(sr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var sr2 StrRef
	if err := xml.Unmarshal(out, &sr2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestDataLabels_RoundTrip(t *testing.T) {
	dl := &DataLabels{
		ShowVal:       &Boolean{Val: true},
		ShowCatName:   &Boolean{Val: false},
		ShowSerName:   &Boolean{Val: false},
		ShowPercent:   &Boolean{Val: false},
		ShowLegendKey: &Boolean{Val: false},
	}
	out, err := xml.Marshal(dl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var dl2 DataLabels
	if err := xml.Unmarshal(out, &dl2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestLegend_RoundTrip(t *testing.T) {
	positions := []string{"b", "l", "r", "t", "tr"}
	for _, pos := range positions {
		t.Run(pos, func(t *testing.T) {
			l := &Legend{
				LegendPos: &LegendPos{Val: pos},
				Overlay:   &Boolean{Val: false},
			}
			out, err := xml.Marshal(l)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var l2 Legend
			if err := xml.Unmarshal(out, &l2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestView3D_RoundTrip(t *testing.T) {
	v := &View3D{
		RotX:         &RotX{Val: 15},
		RotY:         &RotY{Val: 20},
		RAngAx:       &Boolean{Val: true},
		Perspective:  &Perspective{Val: 30},
		DepthPercent: &DepthPercent{Val: 100},
	}
	out, err := xml.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var v2 View3D
	if err := xml.Unmarshal(out, &v2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestTrendline_RoundTrip(t *testing.T) {
	types := []string{"exp", "linear", "log", "movingAvg", "poly", "power"}
	for _, ttype := range types {
		t.Run(ttype, func(t *testing.T) {
			tl := &Trendline{
				TrendlineType: &TrendlineType{Val: ttype},
				DispRSqr:      &Boolean{Val: true},
				DispEq:        &Boolean{Val: true},
			}
			out, err := xml.Marshal(tl)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tl2 Trendline
			if err := xml.Unmarshal(out, &tl2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestMarker_RoundTrip(t *testing.T) {
	styles := []string{"circle", "dash", "diamond", "dot", "none", "plus", "square", "star", "triangle", "x"}
	for _, style := range styles {
		t.Run(style, func(t *testing.T) {
			m := &Marker{
				Symbol: &MarkerStyle{Val: style},
				Size:   &MarkerSize{Val: 5},
			}
			out, err := xml.Marshal(m)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var m2 Marker
			if err := xml.Unmarshal(out, &m2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestNumFmt_RoundTrip(t *testing.T) {
	tests := []struct {
		code   string
		linked bool
	}{
		{"General", true},
		{"0.00", false},
		{"#,##0", true},
		{"0%", false},
		{"$#,##0.00", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			linked := tt.linked
			nf := &NumFmt{FormatCode: tt.code, SourceLinked: &linked}
			out, err := xml.Marshal(nf)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var nf2 NumFmt
			if err := xml.Unmarshal(out, &nf2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if nf2.FormatCode != tt.code {
				t.Errorf("FormatCode = %q, want %q", nf2.FormatCode, tt.code)
			}
		})
	}
}

func TestPageMargins_RoundTrip(t *testing.T) {
	pm := &PageMargins{L: 0.7, R: 0.7, T: 0.75, B: 0.75, Header: 0.3, Footer: 0.3}
	out, err := xml.Marshal(pm)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var pm2 PageMargins
	if err := xml.Unmarshal(out, &pm2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestErrBars_RoundTrip(t *testing.T) {
	eb := &ErrBars{
		ErrDir:     &ErrDir{Val: "y"},
		ErrBarType: &ErrBarType{Val: "both"},
		ErrValType: &ErrValType{Val: "fixedVal"},
		NoEndCap:   &Boolean{Val: false},
		Val:        &Double{Val: 5.0},
	}
	out, err := xml.Marshal(eb)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var eb2 ErrBars
	if err := xml.Unmarshal(out, &eb2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestManualLayout_RoundTrip(t *testing.T) {
	ml := &ManualLayout{
		LayoutTarget: &LayoutTarget{Val: "inner"},
		XMode:        &LayoutMode{Val: "edge"},
		YMode:        &LayoutMode{Val: "edge"},
		X:            &Double{Val: 0.1},
		Y:            &Double{Val: 0.1},
		W:            &Double{Val: 0.8},
		H:            &Double{Val: 0.8},
	}
	out, err := xml.Marshal(ml)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ml2 ManualLayout
	if err := xml.Unmarshal(out, &ml2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestProtection_RoundTrip(t *testing.T) {
	p := &Protection{
		ChartObject:   &Boolean{Val: false},
		Data:          &Boolean{Val: false},
		Formatting:    &Boolean{Val: false},
		Selection:     &Boolean{Val: false},
		UserInterface: &Boolean{Val: false},
	}
	out, err := xml.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var p2 Protection
	if err := xml.Unmarshal(out, &p2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestScaling_RoundTrip(t *testing.T) {
	s := &Scaling{
		Orientation: &Orientation{Val: "minMax"},
		Max:         &Double{Val: 100.0},
		Min:         &Double{Val: 0.0},
	}
	out, err := xml.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var s2 Scaling
	if err := xml.Unmarshal(out, &s2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}
