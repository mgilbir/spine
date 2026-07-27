package chart

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	dmlchart "github.com/mgilbir/spine/common/dml/chart"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// InjectExternalData inserts a c:externalData element (referencing the embedded
// workbook relationship) into a serialized chart part at its schema position:
// after c:chart (and any c:spPr / c:txPr that follows it) and before
// c:printSettings, c:userShapes, or the chartSpace-level c:extLst.
//
// The shared chart serialization is data-source-agnostic and does not emit it,
// so format integrations that embed the chart's data (docx and pptx charts,
// which have no host worksheet) add it: without it Office cannot open the
// embedded workbook behind "Edit Data", leaving the workbook orphaned. relID is
// the chart part's relationship to its embedded workbook. If the chartSpace
// close tag is absent the input is returned unchanged.
//
// This is the one implementation of that placement; docx and pptx both call it
// rather than keeping private copies that would drift apart the moment
// buildChartSpace grows a trailing element.
func InjectExternalData(chartXML []byte, relID string) []byte {
	const closeTag = "</c:chartSpace>"
	end := bytes.LastIndex(chartXML, []byte(closeTag))
	if end < 0 {
		return chartXML
	}
	// Elements that must follow c:externalData. Searching from the end of
	// c:chart keeps the scan at chartSpace level: c:spPr and c:txPr carry only
	// a:-prefixed children, so a c:extLst found there is the chartSpace's own.
	idx := end
	if from := bytes.LastIndex(chartXML[:end], []byte("</c:chart>")); from >= 0 {
		for _, tag := range [][]byte{[]byte("<c:printSettings"), []byte("<c:userShapes"), []byte("<c:extLst")} {
			if at := bytes.Index(chartXML[from:end], tag); at >= 0 && from+at < idx {
				idx = from + at
			}
		}
	}
	ext := `<c:externalData r:id="` + relID + `"><c:autoUpdate val="0"/></c:externalData>`
	out := make([]byte, 0, len(chartXML)+len(ext))
	out = append(out, chartXML[:idx]...)
	out = append(out, ext...)
	out = append(out, chartXML[idx:]...)
	return out
}

// Axis identifiers. Each chart.xml is an independent part, so fixed IDs are
// fine; they only need to be distinct within one chart.
const (
	catAxisID uint32 = 111111111
	valAxisID uint32 = 222222222
	xAxisID   uint32 = 111111111
	yAxisID   uint32 = 222222222
	// Secondary axis pair for combination charts (series moved to the
	// right-hand value axis).
	secCatAxisID uint32 = 333333333
	secValAxisID uint32 = 444444444
	// serAxisID is the series (depth) axis of 3D and surface charts.
	serAxisID uint32 = 555555555
)

// MarshalChartXML serializes the chart to a DrawingML chart.xml part
// (c:chartSpace). Numeric and string caches are populated from the chart's
// data so it renders without a live data source; c:f formula references point
// at the DataRef sheet.
func (c *Chart) MarshalChartXML() ([]byte, error) {
	cs, err := c.buildChartSpace()
	if err != nil {
		return nil, err
	}

	b := xmlb.NewBuilder()
	b.RegisterNamespace(xmlb.NSDrawingMLChart, xmlb.PrefixDrawingMLChart)
	b.RegisterNamespace(xmlb.NSDrawingML, xmlb.PrefixDrawingML)
	b.RegisterNamespace(xmlb.NSOfficeDocumentRels, xmlb.PrefixRelationships)
	// Producers self-close empty elements (<c:layout/>) with no space.
	b.SetCollapseEmptyElements(true)
	b.SetSelfClosingSpace(false)
	b.WriteHeader()

	nsDecls := []xmlb.NSDecl{
		{Prefix: xmlb.PrefixDrawingMLChart, URI: xmlb.NSDrawingMLChart},
		{Prefix: xmlb.PrefixDrawingML, URI: xmlb.NSDrawingML},
		{Prefix: xmlb.PrefixRelationships, URI: xmlb.NSOfficeDocumentRels},
	}
	b.MarshalRoot(xmlb.NSDrawingMLChart, "chartSpace", cs, nsDecls)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("chart: marshal chartSpace: %w", err)
	}
	return b.Bytes(), nil
}

// buildChartSpace assembles the internal ChartSpace model from the chart.
func (c *Chart) buildChartSpace() (*dmlchart.ChartSpace, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	plot := &dmlchart.PlotArea{Layout: &dmlchart.Layout{}}
	if err := c.buildPlotType(plot); err != nil {
		return nil, err
	}
	if c.usesAxes() {
		c.buildAxes(plot)
	}

	ch := &dmlchart.Chart{
		AutoTitleDeleted: boolElem(c.title == ""),
		PlotArea:         plot,
		PlotVisOnly:      boolElem(true),
		DispBlanksAs:     &dmlchart.DispBlanksAs{Val: "gap"},
	}
	if c.is3D() {
		ch.View3D = c.view3D()
	}
	if c.title != "" {
		ch.Title = &dmlchart.Title{
			Tx:      &dmlchart.ChartText{Rich: richText(c.title)},
			Overlay: boolElem(false),
		}
	}
	if c.showLegend {
		ch.Legend = &dmlchart.Legend{
			LegendPos: &dmlchart.LegendPos{Val: string(c.legendPos)},
			Overlay:   boolElem(false),
		}
	}

	return &dmlchart.ChartSpace{
		Date1904:       boolElem(false),
		Lang:           &dmlchart.String{Val: "en-US"},
		RoundedCorners: boolElem(false),
		Style:          &dmlchart.Style{Val: 2},
		Chart:          ch,
	}, nil
}

// validate reports the schema-level constraints on the chart's data that
// MarshalChartXML cannot honor by construction. They are checked up front so a
// caller learns about them instead of shipping a part Office reports as
// damaged: a chart needs at least one series, every series needs at least one
// data point (an empty series serializes as a ptCount="0" cache with a
// reference to no cells), and CT_StockChart requires three or four c:ser —
// high/low/close, optionally preceded by open.
func (c *Chart) validate() error {
	if len(c.series) == 0 {
		return fmt.Errorf("chart: at least one series is required")
	}
	if c.kind == KindStock && (len(c.series) < 3 || len(c.series) > 4) {
		return fmt.Errorf("chart: a stock chart needs 3 or 4 series (high/low/close, optionally preceded by open); got %d", len(c.series))
	}
	for i, s := range c.series {
		if c.seriesLen(i) == 0 && len(s.XValues) == 0 && len(s.Sizes) == 0 {
			return fmt.Errorf("chart: series %d (%q) has no values", i, s.Name)
		}
	}
	return nil
}

// buildPlotType appends the chart-type group (barChart, lineChart, ...) with
// its series to the plot area.
func (c *Chart) buildPlotType(plot *dmlchart.PlotArea) error {
	dl := c.layout()
	switch c.kind {
	case KindColumn, KindBar:
		dir := "col"
		if c.kind == KindBar {
			dir = "bar"
		}
		bc := c.newBarChart(dir, axIDs(catAxisID, valAxisID))
		for i, s := range c.series {
			bc.Ser = append(bc.Ser, c.barSer(i, s, dl))
		}
		plot.BarChart = append(plot.BarChart, bc)
	case KindLine:
		lc := c.newLineChart(axIDs(catAxisID, valAxisID))
		for i, s := range c.series {
			lc.Ser = append(lc.Ser, c.lineSer(i, s, dl))
		}
		plot.LineChart = append(plot.LineChart, lc)
	case KindArea:
		ac := c.newAreaChart(axIDs(catAxisID, valAxisID))
		for i, s := range c.series {
			ac.Ser = append(ac.Ser, c.areaSer(i, s, dl))
		}
		plot.AreaChart = append(plot.AreaChart, ac)
	case KindCombo:
		return c.buildComboPlot(plot, dl)
	case KindPie:
		pc := &dmlchart.PieChart{
			VaryColors: boolElem(true),
			DLbls:      c.groupDataLabels(),
		}
		// A pie chart plots a single series; use the first.
		pc.Ser = append(pc.Ser, c.pieSer(0, c.series[0], dl))
		plot.PieChart = append(plot.PieChart, pc)
	case KindPie3D:
		pc := &dmlchart.Pie3DChart{
			VaryColors: boolElem(true),
			DLbls:      c.groupDataLabels(),
		}
		// Like a pie, a 3D pie plots a single series; use the first.
		pc.Ser = append(pc.Ser, c.pieSer(0, c.series[0], dl))
		plot.Pie3DChart = append(plot.Pie3DChart, pc)
	case KindDoughnut:
		dc := &dmlchart.DoughnutChart{
			VaryColors: boolElem(true),
			DLbls:      c.groupDataLabels(),
			HoleSize:   &dmlchart.HoleSize{Val: defaultHoleSize},
		}
		// Unlike a pie, a doughnut plots every series: each renders as its own
		// concentric ring, which is what Office does and what the embedded
		// workbook's columns already describe (C561).
		for i, s := range c.series {
			dc.Ser = append(dc.Ser, c.pieSer(i, s, dl))
		}
		plot.DoughnutChart = append(plot.DoughnutChart, dc)
	case KindOfPie:
		oc := &dmlchart.OfPieChart{
			OfPieType:     &dmlchart.OfPieType{Val: "pie"},
			VaryColors:    boolElem(true),
			DLbls:         c.groupDataLabels(),
			GapWidth:      &dmlchart.GapAmount{Val: 100},
			SplitType:     &dmlchart.SplitType{Val: "auto"},
			SecondPieSize: &dmlchart.SecondPieSize{Val: 75},
			SerLines:      []*dmlchart.ChartLines{{}},
		}
		// Like a pie, a pie-of-pie plots a single series; use the first.
		oc.Ser = append(oc.Ser, c.pieSer(0, c.series[0], dl))
		plot.OfPieChart = append(plot.OfPieChart, oc)
	case KindRadar:
		rc := &dmlchart.RadarChart{
			RadarStyle: &dmlchart.RadarStyle{Val: "marker"},
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			AxId:       axIDs(catAxisID, valAxisID),
		}
		for i, s := range c.series {
			rc.Ser = append(rc.Ser, &dmlchart.RadarSer{
				Idx:    uintElem(uint32(i)),
				Order:  uintElem(uint32(i)),
				Tx:     serName(s.Name, dl.Series[i].NameRef),
				SpPr:   seriesSpPr(s.Color),
				Marker: &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "none"}},
				Cat:    c.catSource(dl),
				Val:    numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
			})
		}
		plot.RadarChart = append(plot.RadarChart, rc)
	case KindScatter:
		sc := &dmlchart.ScatterChart{
			ScatterStyle: &dmlchart.ScatterStyle{Val: "lineMarker"},
			VaryColors:   boolElem(false),
			DLbls:        c.groupDataLabels(),
			AxId:         axIDs(xAxisID, yAxisID),
		}
		for i, s := range c.series {
			sc.Ser = append(sc.Ser, &dmlchart.ScatterSer{
				Idx:    uintElem(uint32(i)),
				Order:  uintElem(uint32(i)),
				Tx:     serName(s.Name, dl.Series[i].NameRef),
				SpPr:   seriesSpPr(s.Color),
				Marker: &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "circle"}},
				XVal:   axNumSource(s.XValues, dl.Series[i].XValuesRef, "General"),
				YVal:   numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
				Smooth: boolElem(false),
			})
		}
		plot.ScatterChart = append(plot.ScatterChart, sc)
	case KindBubble:
		bc := &dmlchart.BubbleChart{
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			AxId:       axIDs(xAxisID, yAxisID),
		}
		for i, s := range c.series {
			bc.Ser = append(bc.Ser, &dmlchart.BubbleSer{
				Idx:              uintElem(uint32(i)),
				Order:            uintElem(uint32(i)),
				Tx:               serName(s.Name, dl.Series[i].NameRef),
				SpPr:             seriesSpPr(s.Color),
				InvertIfNegative: boolElem(false),
				XVal:             axNumSource(s.XValues, dl.Series[i].XValuesRef, "General"),
				YVal:             numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
				BubbleSize:       numSource(s.Sizes, dl.Series[i].SizesRef, "General"),
				Bubble3D:         boolElem(false),
			})
		}
		plot.BubbleChart = append(plot.BubbleChart, bc)
	case KindStock:
		stc := &dmlchart.StockChart{
			DLbls:      c.groupDataLabels(),
			HiLowLines: &dmlchart.ChartLines{},
			AxId:       axIDs(catAxisID, valAxisID),
		}
		for i, s := range c.series {
			stc.Ser = append(stc.Ser, c.stockSer(i, s, dl))
		}
		plot.StockChart = append(plot.StockChart, stc)
	case KindSurface:
		sc := &dmlchart.SurfaceChart{
			Wireframe: boolElem(false),
			AxId:      axIDs(catAxisID, valAxisID, serAxisID),
		}
		for i, s := range c.series {
			sc.Ser = append(sc.Ser, &dmlchart.SurfaceSer{
				Idx:   uintElem(uint32(i)),
				Order: uintElem(uint32(i)),
				Tx:    serName(s.Name, dl.Series[i].NameRef),
				SpPr:  seriesSpPr(s.Color),
				Cat:   c.catSource(dl),
				Val:   numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
			})
		}
		plot.SurfaceChart = append(plot.SurfaceChart, sc)
	case KindColumn3D, KindBar3D:
		dir := "col"
		if c.kind == KindBar3D {
			dir = "bar"
		}
		bc := &dmlchart.Bar3DChart{
			BarDir:     &dmlchart.BarDir{Val: dir},
			Grouping:   &dmlchart.BarGrouping{Val: string(c.barGrouping())},
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			GapWidth:   &dmlchart.GapAmount{Val: 150},
			GapDepth:   &dmlchart.GapAmount{Val: 150},
			Shape:      &dmlchart.BarShape{Val: "box"},
			AxId:       axIDs(catAxisID, valAxisID, serAxisID),
		}
		for i, s := range c.series {
			bc.Ser = append(bc.Ser, c.barSer(i, s, dl))
		}
		plot.Bar3DChart = append(plot.Bar3DChart, bc)
	case KindLine3D:
		lc := &dmlchart.Line3DChart{
			Grouping:   &dmlchart.Grouping{Val: string(c.lineGrouping())},
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			GapDepth:   &dmlchart.GapAmount{Val: 150},
			AxId:       axIDs(catAxisID, valAxisID, serAxisID),
		}
		for i, s := range c.series {
			lc.Ser = append(lc.Ser, c.lineSer(i, s, dl))
		}
		plot.Line3DChart = append(plot.Line3DChart, lc)
	case KindArea3D:
		ac := &dmlchart.Area3DChart{
			Grouping:   &dmlchart.Grouping{Val: string(c.lineGrouping())},
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			GapDepth:   &dmlchart.GapAmount{Val: 150},
			AxId:       axIDs(catAxisID, valAxisID, serAxisID),
		}
		for i, s := range c.series {
			ac.Ser = append(ac.Ser, c.areaSer(i, s, dl))
		}
		plot.Area3DChart = append(plot.Area3DChart, ac)
	default:
		return fmt.Errorf("chart: unsupported kind %v", c.kind)
	}
	return nil
}

// --- chart-type group and series builders (shared by the single-type and
// combination paths) ---

func (c *Chart) newBarChart(dir string, axID []*dmlchart.UnsignedInt) *dmlchart.BarChart {
	bc := &dmlchart.BarChart{
		BarDir:     &dmlchart.BarDir{Val: dir},
		Grouping:   &dmlchart.BarGrouping{Val: string(c.barGrouping())},
		VaryColors: boolElem(false),
		DLbls:      c.groupDataLabels(),
		GapWidth:   &dmlchart.GapAmount{Val: 150},
		AxId:       axID,
	}
	if g := c.barGrouping(); g == GroupingStacked || g == GroupingPercentStacked {
		// Stacked bars sit on top of each other rather than side by side;
		// without full overlap Office renders them as separated slivers.
		bc.Overlap = &dmlchart.Overlap{Val: 100}
	}
	return bc
}

func (c *Chart) newLineChart(axID []*dmlchart.UnsignedInt) *dmlchart.LineChart {
	return &dmlchart.LineChart{
		Grouping:   &dmlchart.Grouping{Val: string(c.lineGrouping())},
		VaryColors: boolElem(false),
		DLbls:      c.groupDataLabels(),
		Marker:     boolElem(true),
		AxId:       axID,
	}
}

func (c *Chart) newAreaChart(axID []*dmlchart.UnsignedInt) *dmlchart.AreaChart {
	return &dmlchart.AreaChart{
		Grouping:   &dmlchart.Grouping{Val: string(c.lineGrouping())},
		VaryColors: boolElem(false),
		DLbls:      c.groupDataLabels(),
		AxId:       axID,
	}
}

func (c *Chart) barSer(i int, s *Series, dl DataLayout) *dmlchart.BarSer {
	return &dmlchart.BarSer{
		Idx:              uintElem(uint32(i)),
		Order:            uintElem(uint32(i)),
		Tx:               serName(s.Name, dl.Series[i].NameRef),
		SpPr:             seriesSpPr(s.Color),
		InvertIfNegative: boolElem(false),
		Cat:              c.catSource(dl),
		Val:              numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
	}
}

func (c *Chart) lineSer(i int, s *Series, dl DataLayout) *dmlchart.LineSer {
	return &dmlchart.LineSer{
		Idx:    uintElem(uint32(i)),
		Order:  uintElem(uint32(i)),
		Tx:     serName(s.Name, dl.Series[i].NameRef),
		SpPr:   seriesSpPr(s.Color),
		Marker: &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "none"}},
		Cat:    c.catSource(dl),
		Val:    numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
		Smooth: boolElem(false),
	}
}

func (c *Chart) areaSer(i int, s *Series, dl DataLayout) *dmlchart.AreaSer {
	return &dmlchart.AreaSer{
		Idx:   uintElem(uint32(i)),
		Order: uintElem(uint32(i)),
		Tx:    serName(s.Name, dl.Series[i].NameRef),
		SpPr:  seriesSpPr(s.Color),
		Cat:   c.catSource(dl),
		Val:   numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
	}
}

// pieSer builds a CT_PieSer, shared by the pie, 3D pie, doughnut, and
// pie-of-pie charts (all four use c:ser with the same shape).
func (c *Chart) pieSer(i int, s *Series, dl DataLayout) *dmlchart.PieSer {
	return &dmlchart.PieSer{
		Idx:   uintElem(uint32(i)),
		Order: uintElem(uint32(i)),
		Tx:    serName(s.Name, dl.Series[i].NameRef),
		SpPr:  seriesSpPr(s.Color),
		Cat:   c.catSource(dl),
		Val:   numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
	}
}

// stockSer builds a CT_LineSer for a stock chart. Unlike a line series it
// carries no connecting line marker of its own: the high-low lines join the
// points, so the series is just an index, name, and values.
func (c *Chart) stockSer(i int, s *Series, dl DataLayout) *dmlchart.LineSer {
	return &dmlchart.LineSer{
		Idx:   uintElem(uint32(i)),
		Order: uintElem(uint32(i)),
		Tx:    serName(s.Name, dl.Series[i].NameRef),
		SpPr:  seriesSpPr(s.Color),
		Cat:   c.catSource(dl),
		Val:   numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
	}
}

// view3D builds the c:view3D perspective element for the 3D chart kinds. The
// pie family uses a steeper rotation with no right-angle axes; the bar/line/area
// family uses Office's default oblique projection.
func (c *Chart) view3D() *dmlchart.View3D {
	if c.kind == KindPie3D {
		return &dmlchart.View3D{
			RotX:   &dmlchart.RotX{Val: 30},
			RotY:   &dmlchart.RotY{Val: 0},
			RAngAx: boolElem(false),
		}
	}
	return &dmlchart.View3D{
		RotX:         &dmlchart.RotX{Val: 15},
		RotY:         &dmlchart.RotY{Val: 20},
		DepthPercent: &dmlchart.DepthPercent{Val: 100},
		RAngAx:       boolElem(true),
	}
}

// comboGroupKey identifies one chart-type group in a combination chart: a plot
// type on one of the two value axes. Series sharing a key render together.
type comboGroupKey struct {
	kind      Kind
	secondary bool
}

// buildComboPlot builds the chart-type groups for a combination chart. Series
// are grouped by (plot type, value axis) preserving their original index, and
// each group emits the matching bar/line/area element bound to the primary or
// secondary axis pair. Groups appear in the order their key is first used.
func (c *Chart) buildComboPlot(plot *dmlchart.PlotArea, dl DataLayout) error {
	var order []comboGroupKey
	members := map[comboGroupKey][]int{}
	for i, s := range c.series {
		// A combo series with no explicit type (the zero value, KindColumn)
		// renders as a column.
		kind := s.PlotType
		if !kind.isComboMember() {
			return fmt.Errorf("chart: combo series %q has type %v; only column, bar, line, and area combine", s.Name, kind)
		}
		key := comboGroupKey{kind: kind, secondary: s.SecondaryAxis}
		if _, ok := members[key]; !ok {
			order = append(order, key)
		}
		members[key] = append(members[key], i)
	}

	for _, key := range order {
		axID := axIDs(catAxisID, valAxisID)
		if key.secondary {
			axID = axIDs(secCatAxisID, secValAxisID)
		}
		switch key.kind {
		case KindColumn, KindBar:
			// A bar-direction group is legal in a plot area alongside line and
			// area groups, and Parse recovers one as KindBar; rejecting it here
			// made such a chart readable but impossible to re-embed (C557).
			dir := "col"
			if key.kind == KindBar {
				dir = "bar"
			}
			bc := c.newBarChart(dir, axID)
			for _, i := range members[key] {
				bc.Ser = append(bc.Ser, c.barSer(i, c.series[i], dl))
			}
			plot.BarChart = append(plot.BarChart, bc)
		case KindLine:
			lc := c.newLineChart(axID)
			for _, i := range members[key] {
				lc.Ser = append(lc.Ser, c.lineSer(i, c.series[i], dl))
			}
			plot.LineChart = append(plot.LineChart, lc)
		case KindArea:
			ac := c.newAreaChart(axID)
			for _, i := range members[key] {
				ac.Ser = append(ac.Ser, c.areaSer(i, c.series[i], dl))
			}
			plot.AreaChart = append(plot.AreaChart, ac)
		}
	}
	return nil
}

// allSeriesAreBars reports whether every series of a combination chart plots as
// a horizontal bar, which flips the axis orientation.
func (c *Chart) allSeriesAreBars() bool {
	for _, s := range c.series {
		if s.PlotType != KindBar {
			return false
		}
	}
	return len(c.series) > 0
}

// hasSecondaryAxis reports whether any series is assigned to the secondary
// value axis (only meaningful for a combination chart).
func (c *Chart) hasSecondaryAxis() bool {
	for _, s := range c.series {
		if s.SecondaryAxis {
			return true
		}
	}
	return false
}

// buildAxes appends the axis definitions to the plot area.
//
// Emission order note: the axes are written c:valAx before c:catAx, which is
// the reverse of what Office produces. CT_PlotArea's content model is a
// repeatable choice over the axis elements, so both orders are schema-valid and
// Excel, Word and PowerPoint all accept this one; TestChartXMLShape pins it.
// This is deliberate — do not "fix" fixtures or assertions to Office's order
// without changing the serializer first.
func (c *Chart) buildAxes(plot *dmlchart.PlotArea) {
	if c.kind == KindCombo {
		c.buildComboAxes(plot)
		return
	}
	if c.usesTwoValueAxes() {
		// Scatter and bubble use two value axes (X and Y).
		plot.ValAx = []*dmlchart.ValAx{
			c.valAx(xAxisID, yAxisID, "b", c.catAxisName),
			c.valAx(yAxisID, xAxisID, "l", c.valAxisName),
		}
		return
	}
	catPos, valPos := "b", "l"
	if c.kind == KindBar || c.kind == KindBar3D {
		// A horizontal bar chart swaps axis orientation.
		catPos, valPos = "l", "b"
	}
	plot.CatAx = []*dmlchart.CatAx{c.catAx(catPos)}
	plot.ValAx = []*dmlchart.ValAx{c.valAx(valAxisID, catAxisID, valPos, c.valAxisName)}
	if c.needsSerAx() {
		// 3D and surface charts plot series across a third (depth) axis.
		plot.SerAx = []*dmlchart.SerAx{c.serAx(serAxisID, valAxisID)}
	}
}

func (c *Chart) catAx(pos string) *dmlchart.CatAx {
	ax := c.catAxWith(catAxisID, valAxisID, pos)
	if c.catAxisName != "" {
		ax.Title = axisTitle(c.catAxisName)
	}
	return ax
}

// catAxWith builds a category axis with the given id, crossing axis, and
// position. Unlike catAx it carries no title (used for the primary combo axis,
// which titles itself, and the hidden secondary category axis).
func (c *Chart) catAxWith(id, crossID uint32, pos string) *dmlchart.CatAx {
	return &dmlchart.CatAx{
		AxId:          uintElem(id),
		Scaling:       &dmlchart.Scaling{Orientation: &dmlchart.Orientation{Val: "minMax"}},
		Delete:        boolElem(false),
		AxPos:         &dmlchart.AxPos{Val: pos},
		NumFmt:        &dmlchart.NumFmt{FormatCode: "General", SourceLinked: boolPtr(true)},
		MajorTickMark: &dmlchart.TickMark{Val: "out"},
		MinorTickMark: &dmlchart.TickMark{Val: "none"},
		TickLblPos:    &dmlchart.TickLblPos{Val: "nextTo"},
		CrossAx:       uintElem(crossID),
		Crosses:       &dmlchart.Crosses{Val: "autoZero"},
		Auto:          boolElem(true),
		LblAlgn:       &dmlchart.LblAlgn{Val: "ctr"},
		LblOffset:     &dmlchart.LblOffset{Val: 100},
		NoMultiLvlLbl: boolElem(false),
	}
}

// buildComboAxes appends a combination chart's axes: a bottom category axis and
// a left primary value axis always, plus — when any series is on the secondary
// axis — a right-hand secondary value axis and a hidden secondary category axis
// it crosses. The secondary value axis crosses at the maximum so it sits on the
// right.
func (c *Chart) buildComboAxes(plot *dmlchart.PlotArea) {
	catPos, valPos := "b", "l"
	if c.allSeriesAreBars() {
		// A combo of horizontal-bar groups only has the orientation of a bar
		// chart: categories run up the left, values along the bottom.
		catPos, valPos = "l", "b"
	}
	catPrimary := c.catAxWith(catAxisID, valAxisID, catPos)
	if c.catAxisName != "" {
		catPrimary.Title = axisTitle(c.catAxisName)
	}
	plot.CatAx = []*dmlchart.CatAx{catPrimary}
	plot.ValAx = []*dmlchart.ValAx{c.valAx(valAxisID, catAxisID, valPos, c.valAxisName)}

	if !c.hasSecondaryAxis() {
		return
	}
	secVal := c.valAx(secValAxisID, secCatAxisID, "r", "")
	secVal.Crosses = &dmlchart.Crosses{Val: "max"}
	secVal.MajorGridlines = nil // avoid overlaying a second gridline set
	plot.ValAx = append(plot.ValAx, secVal)

	secCat := c.catAxWith(secCatAxisID, secValAxisID, "b")
	secCat.Delete = boolElem(true) // the secondary category axis is not drawn
	plot.CatAx = append(plot.CatAx, secCat)
}

func (c *Chart) valAx(id, crossID uint32, pos, title string) *dmlchart.ValAx {
	ax := &dmlchart.ValAx{
		AxId:           uintElem(id),
		Scaling:        &dmlchart.Scaling{Orientation: &dmlchart.Orientation{Val: "minMax"}},
		Delete:         boolElem(false),
		AxPos:          &dmlchart.AxPos{Val: pos},
		MajorGridlines: &dmlchart.ChartLines{},
		NumFmt:         &dmlchart.NumFmt{FormatCode: "General", SourceLinked: boolPtr(true)},
		MajorTickMark:  &dmlchart.TickMark{Val: "out"},
		MinorTickMark:  &dmlchart.TickMark{Val: "none"},
		TickLblPos:     &dmlchart.TickLblPos{Val: "nextTo"},
		CrossAx:        uintElem(crossID),
		Crosses:        &dmlchart.Crosses{Val: "autoZero"},
		CrossBetween:   &dmlchart.CrossBetween{Val: "between"},
	}
	if title != "" {
		ax.Title = axisTitle(title)
	}
	return ax
}

// serAx builds a series (depth) axis for 3D and surface charts. It carries no
// title and crosses the value axis identified by crossID.
func (c *Chart) serAx(id, crossID uint32) *dmlchart.SerAx {
	return &dmlchart.SerAx{
		AxId:          uintElem(id),
		Scaling:       &dmlchart.Scaling{Orientation: &dmlchart.Orientation{Val: "minMax"}},
		Delete:        boolElem(false),
		AxPos:         &dmlchart.AxPos{Val: "b"},
		MajorTickMark: &dmlchart.TickMark{Val: "out"},
		MinorTickMark: &dmlchart.TickMark{Val: "none"},
		TickLblPos:    &dmlchart.TickLblPos{Val: "nextTo"},
		CrossAx:       uintElem(crossID),
	}
}

// catSource builds the shared category data source: a strRef when the layout
// gives the categories a home in the data sheet, otherwise a literal cache
// (c:strLit). CT_StrRef requires its c:f child, so a reference with no formula
// would be schema-invalid (C433).
func (c *Chart) catSource(dl DataLayout) *dmlchart.AxDataSource {
	if len(c.categories) == 0 {
		return nil
	}
	if dl.CategoriesRef == "" {
		return &dmlchart.AxDataSource{StrLit: strData(c.categories)}
	}
	return &dmlchart.AxDataSource{
		StrRef: &dmlchart.StrRef{
			F:        dl.CategoriesRef,
			StrCache: strData(c.categories),
		},
	}
}

// numSource builds a numeric data source for c:val / c:yVal: a numRef when the
// values have a home in the data sheet, otherwise a literal (c:numLit). CT_NumRef
// requires its c:f child, so emitting a reference with no formula produces a
// part Office reports as damaged (C433).
func numSource(values []float64, ref, formatCode string) *dmlchart.NumDataSource {
	if ref == "" {
		return &dmlchart.NumDataSource{NumLit: numData(values, formatCode)}
	}
	return &dmlchart.NumDataSource{
		NumRef: &dmlchart.NumRef{
			F:        ref,
			NumCache: numData(values, formatCode),
		},
	}
}

// axNumSource builds a numeric axis data source for c:xVal, with the same
// ref-or-literal rule as numSource.
func axNumSource(values []float64, ref, formatCode string) *dmlchart.AxDataSource {
	if ref == "" {
		return &dmlchart.AxDataSource{NumLit: numData(values, formatCode)}
	}
	return &dmlchart.AxDataSource{
		NumRef: &dmlchart.NumRef{
			F:        ref,
			NumCache: numData(values, formatCode),
		},
	}
}

// serName builds a series-name source: a strRef with a one-point cache, or the
// literal c:v form when the name has no cell to point at.
func serName(name, ref string) *dmlchart.SerTx {
	if ref == "" {
		return &dmlchart.SerTx{V: name}
	}
	return &dmlchart.SerTx{
		StrRef: &dmlchart.StrRef{
			F:        ref,
			StrCache: strData([]string{name}),
		},
	}
}

// numData builds a c:numCache / c:numLit body from float values. A blank point
// (see Blank) contributes to ptCount but emits no c:pt, which is exactly how
// Excel caches an empty cell — writing the NaN sentinel out as a number instead
// put a literal <c:v>NaN</c:v> in the cache (C384).
func numData(values []float64, formatCode string) *dmlchart.NumData {
	nd := &dmlchart.NumData{
		FormatCode: formatCode,
		PtCount:    uintElem(uint32(len(values))),
	}
	for i, v := range values {
		if IsBlank(v) {
			continue
		}
		nd.Pt = append(nd.Pt, &dmlchart.NumVal{
			Idx: uint32(i),
			V:   formatFloat(v),
		})
	}
	return nd
}

// strData builds a c:strCache body from string values.
func strData(values []string) *dmlchart.StrData {
	sd := &dmlchart.StrData{PtCount: uintElem(uint32(len(values)))}
	for i, v := range values {
		sd.Pt = append(sd.Pt, &dmlchart.StrVal{Idx: uint32(i), V: v})
	}
	return sd
}

// richText builds a minimal rich-text body (a:p > a:r > a:t) for a title.
func richText(text string) *dml.TxBody {
	return &dml.TxBody{
		BodyPr:   &dml.BodyPr{},
		LstStyle: &dml.LstStyle{},
		P: []*dml.P{{
			R: []*dml.R{{T: text}},
		}},
	}
}

// axisTitle builds a CT_Title carrying rich text for an axis.
func axisTitle(text string) *dmlchart.Title {
	return &dmlchart.Title{
		Tx:      &dmlchart.ChartText{Rich: richText(text)},
		Overlay: boolElem(false),
	}
}

func axIDs(ids ...uint32) []*dmlchart.UnsignedInt {
	out := make([]*dmlchart.UnsignedInt, len(ids))
	for i, id := range ids {
		out[i] = uintElem(id)
	}
	return out
}

// groupDataLabels returns a c:dLbls that turns on value labels for every point
// in a chart-type group, or nil when the chart has data labels disabled. The
// show flags are emitted explicitly (only showVal true) so a reader does not
// fall back to its own defaults for the omitted ones.
func (c *Chart) groupDataLabels() *dmlchart.DataLabels {
	if !c.dataLabels {
		return nil
	}
	return &dmlchart.DataLabels{
		ShowLegendKey:  boolElem(false),
		ShowVal:        boolElem(true),
		ShowCatName:    boolElem(false),
		ShowSerName:    boolElem(false),
		ShowPercent:    boolElem(false),
		ShowBubbleSize: boolElem(false),
	}
}

// seriesSpPr returns a shape-properties element carrying a solid RGB fill for a
// series, or nil when hex is empty (leaving the series its automatic color).
func seriesSpPr(hex string) *dml.SpPr {
	if hex == "" {
		return nil
	}
	return &dml.SpPr{
		SolidFill: &dml.SolidFill{SrgbClr: &dml.SrgbClr{Val: hex}},
	}
}

// normalizeHexColor trims a leading '#' from an RGB hex string and upper-cases
// it. It does not validate length: callers pass a 6-digit RGB value.
func normalizeHexColor(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	return strings.ToUpper(hex)
}

func boolElem(v bool) *dmlchart.Boolean { return &dmlchart.Boolean{Val: v} }
func uintElem(v uint32) *dmlchart.UnsignedInt {
	return &dmlchart.UnsignedInt{Val: int64(v)}
}
func boolPtr(v bool) *bool { return &v }

// formatFloat renders a float as a chart or worksheet value: the shortest
// decimal that round-trips, never in exponent form (integers as "18",
// fractions as "2.5", large values as "1234567" rather than "1.234567e+06").
//
// It is the one number formatter for both the caches in chart.xml and the cells
// of the data sheet those caches mirror. Formatting the cache with 'g' and the
// sheet with 'f' made the two disagree textually for the same value, and Office
// never writes E-notation in a numCache (C559).
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
