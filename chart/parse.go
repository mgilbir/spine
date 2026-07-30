package chart

import (
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	dmlchart "github.com/mgilbir/spine/common/dml/chart"
)

// Parse reads a DrawingML chart.xml part (c:chartSpace) into a Chart: its type,
// title, legend, axis titles, data labels, grouping, number format, categories,
// and series names, values and colors (recovered from the cached values,
// falling back to literal data). It is what each format's Charts() reader
// builds on.
//
// Parse is lossy by design: a Chart is the builder's model, not a faithful
// c:chartSpace. What a round-trip through Parse and MarshalChartXML does not
// carry over is re-emitted at the builder's defaults — per-series markers and
// smoothing, doughnut hole size, gap width and overlap, explicit axis
// identifiers, scaling, tick and gridline settings, dispBlanksAs, per-point
// formatting (c:dPt), trendlines, error bars, data tables, 3D view angles, and
// any extension list. Anything the plot area holds that the model cannot
// represent at all is reported by ParseNotes rather than dropped silently.
func Parse(chartXML []byte) (*Chart, error) {
	var cs dmlchart.ChartSpace
	if err := xml.Unmarshal(chartXML, &cs); err != nil {
		return nil, fmt.Errorf("chart: parse chartSpace: %w", err)
	}
	if cs.Chart == nil || cs.Chart.PlotArea == nil {
		return nil, fmt.Errorf("chart: chartSpace has no plotArea")
	}
	pa := cs.Chart.PlotArea

	var c *Chart
	// selected names the plot-area group (or, for a combination chart, groups)
	// the Chart was built from; every other group present is reported by
	// droppedGroupNotes.
	var selected string
	switch {
	case len(pa.BarChart)+len(pa.LineChart)+len(pa.AreaChart) > 1:
		// More than one category-type group means a combination chart (mixed
		// series types, or a secondary axis with a repeated type).
		c, selected = parseCombo(pa), "combo"
	case len(pa.BarChart) > 0:
		c, selected = parseBar(pa.BarChart[0]), "barChart"
	case len(pa.LineChart) > 0:
		c, selected = parseLine(pa.LineChart[0]), "lineChart"
	case len(pa.PieChart) > 0:
		c, selected = parsePie(pa.PieChart[0]), "pieChart"
	case len(pa.DoughnutChart) > 0:
		c, selected = parseDoughnut(pa.DoughnutChart[0]), "doughnutChart"
	case len(pa.RadarChart) > 0:
		c, selected = parseRadar(pa.RadarChart[0]), "radarChart"
	case len(pa.AreaChart) > 0:
		c, selected = parseArea(pa.AreaChart[0]), "areaChart"
	case len(pa.ScatterChart) > 0:
		c, selected = parseScatter(pa.ScatterChart[0]), "scatterChart"
	case len(pa.BubbleChart) > 0:
		c, selected = parseBubble(pa.BubbleChart[0]), "bubbleChart"
	case len(pa.StockChart) > 0:
		c, selected = parseStock(pa.StockChart[0]), "stockChart"
	case len(pa.SurfaceChart) > 0:
		c, selected = parseSurface(pa.SurfaceChart[0]), "surfaceChart"
	case len(pa.OfPieChart) > 0:
		c, selected = parseOfPie(pa.OfPieChart[0]), "ofPieChart"
	case len(pa.Bar3DChart) > 0:
		c, selected = parseBar3D(pa.Bar3DChart[0]), "bar3DChart"
	case len(pa.Line3DChart) > 0:
		c, selected = parseLine3D(pa.Line3DChart[0]), "line3DChart"
	case len(pa.Pie3DChart) > 0:
		c, selected = parsePie3D(pa.Pie3DChart[0]), "pie3DChart"
	case len(pa.Area3DChart) > 0:
		c, selected = parseArea3D(pa.Area3DChart[0]), "area3DChart"
	default:
		return nil, fmt.Errorf("chart: unsupported or empty chart type")
	}

	if cs.Chart.Title != nil {
		c.title = titleText(cs.Chart.Title)
	}
	if cs.Chart.Legend != nil {
		c.showLegend = true
		if lp := cs.Chart.Legend.LegendPos; lp != nil && lp.Val != "" {
			c.legendPos = LegendPosition(lp.Val)
		}
	} else {
		c.showLegend = false
	}
	c.catAxisName, c.valAxisName = axisTitles(pa)
	if ref := firstDataRef(pa); ref != "" {
		if sheet := sheetOf(ref); sheet != "" {
			c.DataRef = sheet
		}
	}
	if code := firstFormatCode(pa); code != "" {
		// The cached formatCode is the chart's number format; without it a
		// re-marshaled chart silently reverts every value to "General" (C560).
		c.NumberFormat = code
	}
	c.parseNotes = append(c.parseNotes, droppedGroupNotes(pa, selected, c.kind)...)
	return c, nil
}

// plotGroupCounts lists the chart-type groups a plot area holds, by element
// name, in schema order. It is the basis for reporting the groups Parse could
// not represent (C563).
func plotGroupCounts(pa *dmlchart.PlotArea) []struct {
	name  string
	count int
} {
	return []struct {
		name  string
		count int
	}{
		{"areaChart", len(pa.AreaChart)},
		{"area3DChart", len(pa.Area3DChart)},
		{"barChart", len(pa.BarChart)},
		{"bar3DChart", len(pa.Bar3DChart)},
		{"bubbleChart", len(pa.BubbleChart)},
		{"doughnutChart", len(pa.DoughnutChart)},
		{"lineChart", len(pa.LineChart)},
		{"line3DChart", len(pa.Line3DChart)},
		{"ofPieChart", len(pa.OfPieChart)},
		{"pieChart", len(pa.PieChart)},
		{"pie3DChart", len(pa.Pie3DChart)},
		{"radarChart", len(pa.RadarChart)},
		{"scatterChart", len(pa.ScatterChart)},
		{"stockChart", len(pa.StockChart)},
		{"surfaceChart", len(pa.SurfaceChart)},
		{"surface3DChart", len(pa.Surface3DChart)},
	}
}

// droppedGroupNotes describes the chart-type groups Parse did not represent.
// A plot area may legally hold several groups (one barChart plus one
// scatterChart, say); the model carries one kind, and a combination chart
// covers only the bar/line/area triple. The rest are dropped — reporting them
// lets a caller tell "this is a column chart" from "it had four groups and I
// kept the bars". selected names the group the Chart was built from, or "combo"
// when every bar, line, and area group was consumed.
func droppedGroupNotes(pa *dmlchart.PlotArea, selected string, kind Kind) []string {
	var notes []string
	for _, g := range plotGroupCounts(pa) {
		dropped := g.count
		switch {
		case selected == "combo":
			if g.name == "barChart" || g.name == "lineChart" || g.name == "areaChart" {
				dropped = 0
			}
		case g.name == selected:
			// Only the first element of the selected group is represented.
			dropped--
		}
		if dropped > 0 {
			notes = append(notes, fmt.Sprintf(
				"chart: plot area holds %d c:%s group(s) this model does not represent; only the %s data was read",
				dropped, g.name, kind.String()))
		}
	}
	return notes
}

func parseBar(bc *dmlchart.BarChart) *Chart {
	kind := KindColumn
	if bc.BarDir != nil && bc.BarDir.Val == "bar" {
		kind = KindBar
	}
	c := newChart(kind)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(bc.DLbls)
	c.grouping = barGroupingOf(bc.Grouping)
	for i, s := range bc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseLine(lc *dmlchart.LineChart) *Chart {
	c := newChart(KindLine)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(lc.DLbls)
	c.grouping = groupingOf(lc.Grouping)
	for i, s := range lc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseArea(ac *dmlchart.AreaChart) *Chart {
	c := newChart(KindArea)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(ac.DLbls)
	c.grouping = groupingOf(ac.Grouping)
	for i, s := range ac.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parsePie(pc *dmlchart.PieChart) *Chart {
	c := newChart(KindPie)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(pc.DLbls)
	for i, s := range pc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseDoughnut(dc *dmlchart.DoughnutChart) *Chart {
	c := newChart(KindDoughnut)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(dc.DLbls)
	for i, s := range dc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseRadar(rc *dmlchart.RadarChart) *Chart {
	c := newChart(KindRadar)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(rc.DLbls)
	for i, s := range rc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseScatter(sc *dmlchart.ScatterChart) *Chart {
	c := newChart(KindScatter)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(sc.DLbls)
	for _, s := range sc.Ser {
		c.series = append(c.series, &Series{
			Name:    seriesName(s.Tx),
			XValues: numbersFromAx(s.XVal),
			Values:  numbersFrom(s.YVal),
			Color:   seriesColor(s.SpPr),
		})
	}
	return c
}

// addCatSer appends one category-style series (name, values, color) recovered
// from the common c:ser fields, taking the shared categories from the first
// series. It backs the parsers for the chart kinds whose series share the
// cat/val shape (3D bar/line/area, pie-of-pie, 3D pie, stock, surface).
func addCatSer(c *Chart, i int, tx *dmlchart.SerTx, cat *dmlchart.AxDataSource, val *dmlchart.NumDataSource, spPr *dml.SpPr) {
	if i == 0 {
		c.categories = categoriesFrom(cat)
	}
	c.series = append(c.series, &Series{
		Name:   seriesName(tx),
		Values: numbersFrom(val),
		Color:  seriesColor(spPr),
	})
}

func parseBar3D(bc *dmlchart.Bar3DChart) *Chart {
	kind := KindColumn3D
	if bc.BarDir != nil && bc.BarDir.Val == "bar" {
		kind = KindBar3D
	}
	c := newChart(kind)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(bc.DLbls)
	c.grouping = barGroupingOf(bc.Grouping)
	for i, s := range bc.Ser {
		addCatSer(c, i, s.Tx, s.Cat, s.Val, s.SpPr)
	}
	return c
}

func parseLine3D(lc *dmlchart.Line3DChart) *Chart {
	c := newChart(KindLine3D)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(lc.DLbls)
	c.grouping = groupingOf(lc.Grouping)
	for i, s := range lc.Ser {
		addCatSer(c, i, s.Tx, s.Cat, s.Val, s.SpPr)
	}
	return c
}

func parseArea3D(ac *dmlchart.Area3DChart) *Chart {
	c := newChart(KindArea3D)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(ac.DLbls)
	c.grouping = groupingOf(ac.Grouping)
	for i, s := range ac.Ser {
		addCatSer(c, i, s.Tx, s.Cat, s.Val, s.SpPr)
	}
	return c
}

func parsePie3D(pc *dmlchart.Pie3DChart) *Chart {
	c := newChart(KindPie3D)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(pc.DLbls)
	for i, s := range pc.Ser {
		addCatSer(c, i, s.Tx, s.Cat, s.Val, s.SpPr)
	}
	return c
}

func parseOfPie(oc *dmlchart.OfPieChart) *Chart {
	c := newChart(KindOfPie)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(oc.DLbls)
	for i, s := range oc.Ser {
		addCatSer(c, i, s.Tx, s.Cat, s.Val, s.SpPr)
	}
	return c
}

func parseStock(stc *dmlchart.StockChart) *Chart {
	c := newChart(KindStock)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(stc.DLbls)
	for i, s := range stc.Ser {
		addCatSer(c, i, s.Tx, s.Cat, s.Val, s.SpPr)
	}
	return c
}

func parseSurface(sc *dmlchart.SurfaceChart) *Chart {
	c := newChart(KindSurface)
	c.showLegend = false
	for i, s := range sc.Ser {
		addCatSer(c, i, s.Tx, s.Cat, s.Val, s.SpPr)
	}
	return c
}

func parseBubble(bc *dmlchart.BubbleChart) *Chart {
	c := newChart(KindBubble)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(bc.DLbls)
	for _, s := range bc.Ser {
		c.series = append(c.series, &Series{
			Name:    seriesName(s.Tx),
			XValues: numbersFromAx(s.XVal),
			Values:  numbersFrom(s.YVal),
			Sizes:   numbersFrom(s.BubbleSize),
			Color:   seriesColor(s.SpPr),
		})
	}
	return c
}

// parseCombo reconstructs a combination chart: it recovers each series with its
// plot type (column/line/area) and whether it sits on the secondary value axis,
// then restores the original series order from the c:idx values. The secondary
// value axis is identified as the one positioned on the right (axPos="r") or
// crossing at the maximum.
func parseCombo(pa *dmlchart.PlotArea) *Chart {
	c := newChart(KindCombo)
	c.showLegend = false

	// Identify value-axis ids and, among them, the secondary axis.
	valAxIDs := map[int64]bool{}
	var secValID int64
	hasSec := false
	for _, ax := range pa.ValAx {
		if ax == nil || ax.AxId == nil {
			continue
		}
		valAxIDs[ax.AxId.Val] = true
		if (ax.AxPos != nil && ax.AxPos.Val == "r") || (ax.Crosses != nil && ax.Crosses.Val == "max") {
			secValID = ax.AxId.Val
			hasSec = true
		}
	}
	// onSecondary reports whether a group's axId pair binds to the secondary
	// value axis (matching the value-axis id in the pair against secValID).
	onSecondary := func(axIDs []*dmlchart.UnsignedInt) bool {
		if !hasSec {
			return false
		}
		for _, a := range axIDs {
			if a != nil && valAxIDs[a.Val] {
				return a.Val == secValID
			}
		}
		return false
	}

	type indexed struct {
		idx int
		s   *Series
	}
	var entries []indexed
	var cats []string

	add := func(order int, idxElem *dmlchart.UnsignedInt, kind Kind, secondary bool, s *Series) {
		s.PlotType = kind
		s.SecondaryAxis = secondary
		entries = append(entries, indexed{idx: serIdx(idxElem, order), s: s})
	}

	for _, bc := range pa.BarChart {
		kind := KindColumn
		if bc.BarDir != nil && bc.BarDir.Val == "bar" {
			kind = KindBar
		}
		if dLblsShowVal(bc.DLbls) {
			c.dataLabels = true
		}
		sec := onSecondary(bc.AxId)
		for _, s := range bc.Ser {
			if cats == nil {
				cats = categoriesFrom(s.Cat)
			}
			add(len(entries), s.Idx, kind, sec, &Series{
				Name:   seriesName(s.Tx),
				Values: numbersFrom(s.Val),
				Color:  seriesColor(s.SpPr),
			})
		}
	}
	for _, lc := range pa.LineChart {
		if dLblsShowVal(lc.DLbls) {
			c.dataLabels = true
		}
		sec := onSecondary(lc.AxId)
		for _, s := range lc.Ser {
			if cats == nil {
				cats = categoriesFrom(s.Cat)
			}
			add(len(entries), s.Idx, KindLine, sec, &Series{
				Name:   seriesName(s.Tx),
				Values: numbersFrom(s.Val),
				Color:  seriesColor(s.SpPr),
			})
		}
	}
	for _, ac := range pa.AreaChart {
		if dLblsShowVal(ac.DLbls) {
			c.dataLabels = true
		}
		sec := onSecondary(ac.AxId)
		for _, s := range ac.Ser {
			if cats == nil {
				cats = categoriesFrom(s.Cat)
			}
			add(len(entries), s.Idx, KindArea, sec, &Series{
				Name:   seriesName(s.Tx),
				Values: numbersFrom(s.Val),
				Color:  seriesColor(s.SpPr),
			})
		}
	}

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].idx < entries[j].idx })
	c.categories = cats
	for _, e := range entries {
		c.series = append(c.series, e.s)
	}
	return c
}

// serIdx returns a series' c:idx value, or the fallback (its appearance order)
// when the element is absent.
func serIdx(idx *dmlchart.UnsignedInt, fallback int) int {
	if idx == nil {
		return fallback
	}
	return int(idx.Val)
}

// dLblsShowVal reports whether a chart-type group's data labels turn on value
// display (c:dLbls/c:showVal val="1").
func dLblsShowVal(d *dmlchart.DataLabels) bool {
	return d != nil && d.ShowVal != nil && d.ShowVal.Val
}

// seriesColor recovers a series' solid RGB fill from its shape properties, or
// "" when it has no solid srgbClr fill.
func seriesColor(spPr *dml.SpPr) string {
	if spPr != nil && spPr.SolidFill != nil && spPr.SolidFill.SrgbClr != nil {
		return spPr.SolidFill.SrgbClr.Val
	}
	return ""
}

// seriesName recovers a series name from its tx (cached strRef or literal v).
func seriesName(tx *dmlchart.SerTx) string {
	if tx == nil {
		return ""
	}
	if tx.V != "" {
		return tx.V
	}
	if tx.StrRef != nil && tx.StrRef.StrCache != nil {
		if pts := tx.StrRef.StrCache.Pt; len(pts) > 0 {
			return pts[0].V
		}
	}
	return ""
}

// categoriesFrom recovers category labels from a c:cat source, preferring the
// string cache/literal, falling back to numeric points formatted as strings.
func categoriesFrom(src *dmlchart.AxDataSource) []string {
	if src == nil {
		return nil
	}
	switch {
	case src.StrRef != nil && src.StrRef.StrCache != nil:
		return strPoints(src.StrRef.StrCache)
	case src.StrLit != nil:
		return strPoints(src.StrLit)
	case src.NumRef != nil && src.NumRef.NumCache != nil:
		return numPointsAsStrings(src.NumRef.NumCache)
	case src.NumLit != nil:
		return numPointsAsStrings(src.NumLit)
	}
	return nil
}

func strPoints(sd *dmlchart.StrData) []string {
	if sd == nil {
		return nil
	}
	n := cacheLen(sd.PtCount, maxStrPtIdx(sd.Pt))
	out := make([]string, n)
	for _, p := range sd.Pt {
		if idx := int(p.Idx); idx >= 0 && idx < n {
			out[idx] = p.V
		}
	}
	return out
}

func numPointsAsStrings(nd *dmlchart.NumData) []string {
	if nd == nil {
		return nil
	}
	n := cacheLen(nd.PtCount, maxNumPtIdx(nd.Pt))
	out := make([]string, n)
	for _, p := range nd.Pt {
		if idx := int(p.Idx); idx >= 0 && idx < n {
			out[idx] = p.V
		}
	}
	return out
}

// numbersFrom recovers numeric values from a c:val / c:yVal source.
func numbersFrom(src *dmlchart.NumDataSource) []float64 {
	if src == nil {
		return nil
	}
	switch {
	case src.NumRef != nil && src.NumRef.NumCache != nil:
		return numPoints(src.NumRef.NumCache)
	case src.NumLit != nil:
		return numPoints(src.NumLit)
	}
	return nil
}

// numbersFromAx recovers numeric values from a c:xVal (AxDataSource) source.
func numbersFromAx(src *dmlchart.AxDataSource) []float64 {
	if src == nil {
		return nil
	}
	switch {
	case src.NumRef != nil && src.NumRef.NumCache != nil:
		return numPoints(src.NumRef.NumCache)
	case src.NumLit != nil:
		return numPoints(src.NumLit)
	}
	return nil
}

func numPoints(nd *dmlchart.NumData) []float64 {
	if nd == nil {
		return nil
	}
	n := cacheLen(nd.PtCount, maxNumPtIdx(nd.Pt))
	// Blank data points are omitted from the cache, so any position no c:pt
	// lands on stays a NaN placeholder — keeping every present value aligned
	// with its category rather than shifting to fill the gaps (C250).
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	for _, p := range nd.Pt {
		idx := int(p.Idx)
		if idx < 0 || idx >= n {
			continue
		}
		f, _ := strconv.ParseFloat(p.V, 64)
		out[idx] = f
	}
	return out
}

// maxCachePoints clamps the length a single c:numCache / c:strCache is trusted
// to declare.
//
// ptCount and a c:pt's idx are length fields in attacker-controlled bytes, and
// both were used directly as an allocation size: a 374-byte chart part carrying
// `<c:ptCount val="50000000"/>` allocated 400 MB — a 1,070,000x amplification,
// the C360 class, found by FuzzParseChartXML. Clamping to a limit the format
// itself justifies is the same remedy boundedCap applies in opc/cfb.go.
//
// 32768 is just above the 32,000 data points Excel allows in one series of a
// two-dimensional chart (4,000 for a 3-D one), so no cache a producer can write
// is truncated. A cache claiming more describes a series the application that
// defines the format would refuse to render. The clamp is stable across a round
// trip: re-serializing writes the clamped count back, so the fixed point holds.
const maxCachePoints = 1 << 15

// cacheLen returns the logical length of a numeric or string cache: its declared
// ptCount, or (when ptCount is absent or smaller than a sparsely-indexed point)
// one past the largest point index, clamped to maxCachePoints. Excel omits
// <c:pt> for blank cells, so the point slice is shorter than — and may skip
// indices within — the cache; sizing from ptCount preserves each point's
// position.
func cacheLen(ptCount *dmlchart.UnsignedInt, maxIdx int) int {
	n := 0
	if ptCount != nil && ptCount.Val > 0 {
		n = boundedCachePoints(uint64(ptCount.Val))
	}
	if maxIdx >= 0 && maxIdx+1 > n {
		n = boundedCachePoints(uint64(maxIdx) + 1)
	}
	return n
}

// boundedCachePoints clamps a point count taken from parsed bytes. want is
// widened to uint64 so a value past MaxInt32 cannot overflow into a negative
// (panicking) length on a 32-bit build.
func boundedCachePoints(want uint64) int {
	if want > maxCachePoints {
		return maxCachePoints
	}
	return int(want)
}

// maxNumPtIdx returns the largest idx among numeric points, or -1 when empty.
func maxNumPtIdx(pts []*dmlchart.NumVal) int {
	max := -1
	for _, p := range pts {
		if int(p.Idx) > max {
			max = int(p.Idx)
		}
	}
	return max
}

// maxStrPtIdx returns the largest idx among string points, or -1 when empty.
func maxStrPtIdx(pts []*dmlchart.StrVal) int {
	max := -1
	for _, p := range pts {
		if int(p.Idx) > max {
			max = int(p.Idx)
		}
	}
	return max
}

// titleText extracts the plain text of a chart/axis title.
func titleText(t *dmlchart.Title) string {
	if t == nil || t.Tx == nil {
		return ""
	}
	if t.Tx.Rich != nil {
		return richTextString(t.Tx.Rich)
	}
	if t.Tx.StrRef != nil && t.Tx.StrRef.StrCache != nil {
		if pts := t.Tx.StrRef.StrCache.Pt; len(pts) > 0 {
			return pts[0].V
		}
	}
	return ""
}

func richTextString(tb *dml.TxBody) string {
	var s string
	for _, p := range tb.P {
		for _, r := range p.R {
			s += r.T
		}
	}
	return s
}

// axisTitles returns the (category, value) axis titles from the plot area.
func axisTitles(pa *dmlchart.PlotArea) (cat, val string) {
	if len(pa.CatAx) > 0 && pa.CatAx[0].Title != nil {
		cat = titleText(pa.CatAx[0].Title)
	}
	if len(pa.ValAx) > 0 {
		// For scatter the first valAx is the X (category-like) axis.
		if len(pa.CatAx) == 0 && len(pa.ValAx) >= 2 {
			cat = titleText(pa.ValAx[0].Title)
			val = titleText(pa.ValAx[1].Title)
			return cat, val
		}
		if pa.ValAx[0].Title != nil {
			val = titleText(pa.ValAx[0].Title)
		}
	}
	return cat, val
}

// eachSeriesData calls fn with every series' category-like source (c:cat, or
// c:xVal for scatter and bubble) and its value source (c:val / c:yVal), in
// plot-area order, until fn returns false. It is the one traversal of the plot
// area's series: recovering the DataRef sheet and the cached number format both
// walk it, so neither can fall out of step with the chart types the package
// supports.
func eachSeriesData(pa *dmlchart.PlotArea, fn func(cat *dmlchart.AxDataSource, val *dmlchart.NumDataSource) bool) {
	for _, g := range pa.BarChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.LineChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.PieChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.DoughnutChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.RadarChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.AreaChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.ScatterChart {
		for _, s := range g.Ser {
			if !fn(s.XVal, s.YVal) {
				return
			}
		}
	}
	for _, g := range pa.BubbleChart {
		for _, s := range g.Ser {
			if !fn(s.XVal, s.YVal) {
				return
			}
		}
	}
	for _, g := range pa.Bar3DChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.Line3DChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.Area3DChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.Pie3DChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.OfPieChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.StockChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
	for _, g := range pa.SurfaceChart {
		for _, s := range g.Ser {
			if !fn(s.Cat, s.Val) {
				return
			}
		}
	}
}

// firstDataRef returns the first formula reference found on a series' data —
// the category (or scatter/bubble X) source, falling back to the value source —
// used to recover the DataRef sheet name.
//
// The value fallback is not cosmetic. A scatter series whose X source is
// categorical (dates, labels) parses to no XValues, so re-serializing writes
// c:xVal as a literal with no c:f at all; if only the X source were consulted,
// reading such a chart back and saving it again lost the sheet name and every
// reference silently moved to the default "Sheet1". The value source carries a
// c:f in exactly that case, and it names the same sheet.
func firstDataRef(pa *dmlchart.PlotArea) string {
	ref := ""
	eachSeriesData(pa, func(cat *dmlchart.AxDataSource, val *dmlchart.NumDataSource) bool {
		switch {
		case cat == nil:
		case cat.StrRef != nil && cat.StrRef.F != "":
			ref = cat.StrRef.F
		case cat.NumRef != nil && cat.NumRef.F != "":
			ref = cat.NumRef.F
		case cat.MultiLvlStrRef != nil && cat.MultiLvlStrRef.F != "":
			ref = cat.MultiLvlStrRef.F
		}
		if ref == "" && val != nil && val.NumRef != nil {
			ref = val.NumRef.F
		}
		return ref == ""
	})
	return ref
}

// firstFormatCode returns the number format cached with the first series that
// declares one. Parse restores it as the chart's NumberFormat: without it a
// currency or percentage chart read back and re-marshaled silently reverts
// every value to "General" (C560).
func firstFormatCode(pa *dmlchart.PlotArea) string {
	code := ""
	eachSeriesData(pa, func(_ *dmlchart.AxDataSource, val *dmlchart.NumDataSource) bool {
		switch {
		case val == nil:
		case val.NumRef != nil && val.NumRef.NumCache != nil:
			code = val.NumRef.NumCache.FormatCode
		case val.NumLit != nil:
			code = val.NumLit.FormatCode
		}
		return code == ""
	})
	return code
}

// groupingOf recovers an ST_Grouping value (line and area charts).
func groupingOf(g *dmlchart.Grouping) Grouping {
	if g == nil {
		return ""
	}
	return groupingValue(g.Val)
}

// barGroupingOf recovers an ST_BarGrouping value (bar, column, and 3D bar
// charts).
func barGroupingOf(g *dmlchart.BarGrouping) Grouping {
	if g == nil {
		return ""
	}
	return groupingValue(g.Val)
}

// groupingValue maps a schema grouping token to a Grouping, ignoring anything
// outside the enumeration so an unknown token falls back to the kind's default
// rather than being written back verbatim.
func groupingValue(val string) Grouping {
	switch Grouping(val) {
	case GroupingClustered, GroupingStandard, GroupingStacked, GroupingPercentStacked:
		return Grouping(val)
	}
	return ""
}

// sheetOf extracts the sheet name from a formula reference like
// "Sheet1!$A$2:$A$5" or "'My Sheet'!$A$1".
//
// The name it returns is the sheet's own name, not its quoted lexical form:
// DataRef holds an unquoted name (SetDataRef takes one) and quoteSheet puts the
// quoting back on the way out. Two things follow, and both were wrong until a
// fuzz round-trip made them visible on real files.
//
// A quoted name escapes an apostrophe by doubling it, so a sheet named
// "Vue d'ensemble" is referenced as
//
//	'Vue d''ensemble'!$B$1
//
// Returning that doubled form let quoteSheet double it again on every save: one
// save already pointed the chart at a sheet that does not exist, and the
// apostrophe run then grew without bound across the saves after that.
//
// Excel also writes a multi-area (union) reference as
//
//	('Sheet'!$Z$2,'Sheet'!$AD$2)
//
// where all areas name the same sheet, but the leading parenthesis left the
// name as "('Sheet'", which quoteSheet then re-quoted into an equally invalid —
// and equally growing — reference.
func sheetOf(ref string) string {
	ref = strings.TrimPrefix(ref, "(")
	bang := -1
	inQuote := false
	for i := 0; i < len(ref); i++ {
		switch ref[i] {
		case '\'':
			inQuote = !inQuote
		case '!':
			if !inQuote {
				bang = i
			}
		}
		if bang >= 0 {
			break
		}
	}
	if bang <= 0 {
		return ""
	}
	name := ref[:bang]
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name
}
