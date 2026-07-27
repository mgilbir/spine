// Package chart provides a small, format-agnostic API for building and reading
// DrawingML charts as a chart.xml part (c:chartSpace) that the xlsx, docx, and
// pptx integrations embed.
//
// The package wraps the verbose internal model in common/dml/chart with an
// ergonomic builder: pick a chart type, set categories, add series, and
// serialize. Cached values (c:numCache / c:strCache) are populated from the
// supplied data so the chart renders standalone, without a live data source.
// Parse recovers a Chart from an existing chart.xml.
//
// This package is the shared core; each format wires it in through symmetric
// methods over the same *Chart value: AddChart to embed one (Slide.AddChart in
// pptx, Document.AddChart / Paragraph.AddChart in docx, Sheet.AddChart in xlsx)
// and Charts() to read them back (Slide.Charts / Presentation.Charts,
// Document.Charts, Sheet.Charts / Workbook.Charts).
//
// DataRef and EmbeddedWorkbook are how those integrations supply the chart's
// data location. The c:f formula references point at a sheet named by DataRef
// (default "Sheet1"), which an integration sets to match its host. docx and
// pptx charts have no host worksheet, so they embed the minimal SpreadsheetML
// package EmbeddedWorkbook returns (letting Office edit the data); xlsx charts
// reference the host workbook's cells directly and need no embedded copy.
package chart

import "fmt"

// Kind identifies a chart type.
type Kind int

const (
	// KindColumn is a vertical bar chart (c:barChart, barDir=col).
	KindColumn Kind = iota
	// KindBar is a horizontal bar chart (c:barChart, barDir=bar).
	KindBar
	// KindLine is a line chart (c:lineChart).
	KindLine
	// KindPie is a pie chart (c:pieChart).
	KindPie
	// KindScatter is an XY scatter chart (c:scatterChart).
	KindScatter
	// KindArea is an area chart (c:areaChart).
	KindArea
	// KindDoughnut is a doughnut chart (c:doughnutChart): a pie with a hole.
	KindDoughnut
	// KindRadar is a radar (spider) chart (c:radarChart).
	KindRadar
	// KindCombo is a combination chart: series of mixed types (column, line,
	// area) sharing one category axis, optionally split across a primary and a
	// secondary value axis. It has no single c: element of its own — each
	// series renders in the chart-type group named by its Series.PlotType.
	KindCombo
	// KindBubble is a bubble chart (c:bubbleChart): each point carries an X, a Y,
	// and a size. Add series with AddBubbleSeries.
	KindBubble
	// KindStock is a stock (high-low-close) chart (c:stockChart). Add one series
	// per price line (e.g. high, low, close) with AddSeries.
	KindStock
	// KindSurface is a surface chart (c:surfaceChart): a filled topological
	// contour over a category and a series axis.
	KindSurface
	// KindOfPie is a pie-of-pie / bar-of-pie chart (c:ofPieChart). Like a pie it
	// plots a single series (its first).
	KindOfPie
	// KindColumn3D is a 3D vertical (column) bar chart (c:bar3DChart, barDir=col).
	KindColumn3D
	// KindBar3D is a 3D horizontal bar chart (c:bar3DChart, barDir=bar).
	KindBar3D
	// KindLine3D is a 3D line chart (c:line3DChart).
	KindLine3D
	// KindPie3D is a 3D pie chart (c:pie3DChart). Like a pie it plots a single
	// series (its first).
	KindPie3D
	// KindArea3D is a 3D area chart (c:area3DChart).
	KindArea3D
)

// String returns the chart kind's element name (without the c: prefix).
func (k Kind) String() string {
	switch k {
	case KindColumn, KindBar:
		return "barChart"
	case KindColumn3D, KindBar3D:
		return "bar3DChart"
	case KindLine:
		return "lineChart"
	case KindLine3D:
		return "line3DChart"
	case KindPie:
		return "pieChart"
	case KindPie3D:
		return "pie3DChart"
	case KindOfPie:
		return "ofPieChart"
	case KindScatter:
		return "scatterChart"
	case KindBubble:
		return "bubbleChart"
	case KindArea:
		return "areaChart"
	case KindArea3D:
		return "area3DChart"
	case KindDoughnut:
		return "doughnutChart"
	case KindRadar:
		return "radarChart"
	case KindStock:
		return "stockChart"
	case KindSurface:
		return "surfaceChart"
	case KindCombo:
		return "combo"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

// isComboMember reports whether a kind can be a series' plot type inside a
// combination chart. Only the category chart types that share a value axis
// combine: column, bar, line, and area.
func (k Kind) isComboMember() bool {
	return k == KindColumn || k == KindBar || k == KindLine || k == KindArea
}

// defaultHoleSize is the doughnut hole diameter as a percentage of the outer
// radius (CT_HoleSize, 1-90). 50 matches Office's default for a new doughnut.
const defaultHoleSize = 50

// Grouping is how a bar, line, or area chart arranges the series of one
// chart-type group (ST_BarGrouping / ST_Grouping).
type Grouping string

const (
	// GroupingClustered draws each series side by side. It is the default for
	// bar and column charts; line and area charts have no clustered form and
	// fall back to GroupingStandard.
	GroupingClustered Grouping = "clustered"
	// GroupingStandard draws each series independently against the value axis.
	// It is the default for line and area charts.
	GroupingStandard Grouping = "standard"
	// GroupingStacked stacks the series of a category on top of one another.
	GroupingStacked Grouping = "stacked"
	// GroupingPercentStacked stacks the series of a category and scales each
	// stack to 100%.
	GroupingPercentStacked Grouping = "percentStacked"
)

// LegendPosition is a legend placement (CT_LegendPos values).
type LegendPosition string

const (
	LegendRight    LegendPosition = "r"
	LegendLeft     LegendPosition = "l"
	LegendTop      LegendPosition = "t"
	LegendBottom   LegendPosition = "b"
	LegendTopRight LegendPosition = "tr"
)

// Series is one data series in a chart. For category charts (column, bar,
// line, pie, area) only Values is used; the category labels are shared on the
// Chart. For scatter charts XValues holds the per-series X coordinates.
type Series struct {
	Name    string
	Values  []float64
	XValues []float64 // scatter and bubble only
	Sizes   []float64 // bubble only (c:bubbleSize)

	// Color is an optional solid fill for the series, as a 6-digit hex RGB
	// string ("FF0000"). Empty leaves the series to the theme's automatic
	// color. Set it with SetColor.
	Color string

	// PlotType selects how the series renders in a combination chart
	// (KindColumn, KindBar, KindLine, or KindArea). It is consulted only for combo
	// charts (NewCombo); other chart kinds render every series the chart's own
	// way and ignore it. The zero value is KindColumn. Set it with SetType.
	PlotType Kind

	// SecondaryAxis places the series on the chart's secondary (right-hand)
	// value axis in a combination chart. It is consulted only for combo charts.
	// Set it with SetSecondaryAxis.
	SecondaryAxis bool
}

// SetColor sets the series' solid fill to the given RGB color and returns the
// series for chaining. hexRGB is a 6-digit hex string, with or without a
// leading '#' ("#1F77B4" or "1f77b4"); it is normalized to upper-case. An empty
// string clears the color, restoring the automatic (theme) color.
func (s *Series) SetColor(hexRGB string) *Series {
	s.Color = normalizeHexColor(hexRGB)
	return s
}

// SetType sets the series' plot type for a combination chart (NewCombo) and
// returns the series for chaining. Only KindColumn, KindBar, KindLine, and
// KindArea combine; MarshalChartXML reports an error if a combo series carries
// any other type. It has no effect on single-type charts.
func (s *Series) SetType(kind Kind) *Series {
	s.PlotType = kind
	return s
}

// SetSecondaryAxis places the series on the secondary (right-hand) value axis of
// a combination chart and returns the series for chaining. It has no effect on
// single-type charts.
func (s *Series) SetSecondaryAxis(secondary bool) *Series {
	s.SecondaryAxis = secondary
	return s
}

// Chart is a format-agnostic chart definition. Build one with a New*
// constructor, configure it with the setter methods, then call
// MarshalChartXML to produce a chart.xml part.
type Chart struct {
	kind       Kind
	title      string
	categories []string
	series     []*Series

	showLegend  bool
	legendPos   LegendPosition
	catAxisName string
	valAxisName string

	// dataLabels emits c:dLbls (showVal) so each point's value renders on the
	// chart. Set with SetDataLabels.
	dataLabels bool

	// grouping is the bar/line/area chart-type group's arrangement. Empty means
	// the per-kind default (clustered for bars, standard for lines and areas).
	// Set with SetGrouping; recovered by Parse.
	grouping Grouping

	// parseNotes records what Parse could not represent in this model. It is nil
	// for a chart built with a New* constructor. Read with ParseNotes.
	parseNotes []string

	// DataRef is the sheet name that c:f formula references are built
	// against (e.g. "Sheet1" -> "Sheet1!$B$2:$B$5"). Format integrations set
	// this to the host sheet (xlsx) or embedded-workbook sheet (docx/pptx).
	DataRef string

	// NumberFormat is the format code recorded in numeric caches and value
	// axes (default "General").
	NumberFormat string
}

// newChart returns a Chart of the given kind with sensible defaults: a legend
// on the right, the conventional "Sheet1" data reference, and a "General"
// number format. Axes and a plot area are synthesized at marshal time.
func newChart(k Kind) *Chart {
	return &Chart{
		kind:         k,
		showLegend:   true,
		legendPos:    LegendRight,
		DataRef:      "Sheet1",
		NumberFormat: "General",
	}
}

// NewColumn returns a vertical (column) bar chart.
func NewColumn() *Chart { return newChart(KindColumn) }

// NewBar returns a horizontal bar chart.
func NewBar() *Chart { return newChart(KindBar) }

// NewLine returns a line chart.
func NewLine() *Chart { return newChart(KindLine) }

// NewPie returns a pie chart. A pie plots a single series (its first): the
// remaining series are kept in the chart's data — the embedded workbook and the
// xlsx data sheet hold every column, so the extra series stay editable — but
// only the first is rendered. Use NewDoughnut for a multi-series ring chart.
func NewPie() *Chart { return newChart(KindPie) }

// NewScatter returns an XY scatter chart.
func NewScatter() *Chart { return newChart(KindScatter) }

// NewArea returns an area chart.
func NewArea() *Chart { return newChart(KindArea) }

// NewDoughnut returns a doughnut chart: a pie chart with a hole. Unlike a pie
// it plots every series, each as a concentric ring from the inside out.
func NewDoughnut() *Chart { return newChart(KindDoughnut) }

// NewRadar returns a radar (spider) chart.
func NewRadar() *Chart { return newChart(KindRadar) }

// NewCombo returns a combination chart. Add series as usual, then give each one
// a plot type with Series.SetType (KindColumn, KindBar, KindLine, or KindArea) and,
// optionally, move it to the secondary value axis with Series.SetSecondaryAxis.
// Series without an explicit type render as columns. All series share the
// category axis and, unless moved, the primary value axis.
func NewCombo() *Chart { return newChart(KindCombo) }

// NewBubble returns a bubble chart. Add series with AddBubbleSeries, supplying
// each point's X, Y, and size.
func NewBubble() *Chart { return newChart(KindBubble) }

// NewStock returns a stock (high-low-close) chart. Add one series per price
// line with AddSeries; a high-low line joins the points in each category.
//
// CT_StockChart takes three or four series in a fixed order — high, low, close,
// optionally preceded by open — so MarshalChartXML reports an error for a stock
// chart with any other number of series rather than emitting a part Office
// reports as damaged.
func NewStock() *Chart { return newChart(KindStock) }

// NewSurface returns a surface chart: a filled topological contour. Add series
// with AddSeries; each series is one row of the surface.
func NewSurface() *Chart { return newChart(KindSurface) }

// NewOfPie returns a pie-of-pie chart: a pie whose smaller slices are broken
// out into a second, linked pie. Like a pie it plots a single series (its
// first).
func NewOfPie() *Chart { return newChart(KindOfPie) }

// NewColumn3D returns a 3D vertical (column) bar chart.
func NewColumn3D() *Chart { return newChart(KindColumn3D) }

// NewBar3D returns a 3D horizontal bar chart.
func NewBar3D() *Chart { return newChart(KindBar3D) }

// NewLine3D returns a 3D line chart.
func NewLine3D() *Chart { return newChart(KindLine3D) }

// NewPie3D returns a 3D pie chart. Like a pie it plots a single series (its
// first).
func NewPie3D() *Chart { return newChart(KindPie3D) }

// NewArea3D returns a 3D area chart.
func NewArea3D() *Chart { return newChart(KindArea3D) }

// Kind returns the chart's type.
func (c *Chart) Kind() Kind { return c.kind }

// Title returns the chart title.
func (c *Chart) Title() string { return c.title }

// Categories returns the category labels.
func (c *Chart) Categories() []string { return c.categories }

// SeriesList returns the chart's series.
func (c *Chart) SeriesList() []*Series { return c.series }

// LegendPos reports the legend position and whether a legend is shown.
func (c *Chart) LegendPos() (LegendPosition, bool) { return c.legendPos, c.showLegend }

// AxisTitles returns the category and value axis titles.
func (c *Chart) AxisTitles() (category, value string) { return c.catAxisName, c.valAxisName }

// DataLabels reports whether value data labels are shown on the chart.
func (c *Chart) DataLabels() bool { return c.dataLabels }

// Grouping returns how the chart arranges the series of its chart-type group.
// The zero value ("") means the per-kind default: clustered for bar and column
// charts, standard for line and area charts. It is recovered by Parse, so a
// stacked chart read back from a file reports GroupingStacked.
func (c *Chart) Grouping() Grouping { return c.grouping }

// SetGrouping sets how the series of a bar, column, line, or area chart are
// arranged — clustered (the bar default), standard (the line/area default),
// stacked, or percentStacked — and returns the chart for chaining. Chart kinds
// with no grouping in their schema (pie family, scatter, bubble, radar, stock,
// surface) ignore it. A stacked bar chart also emits the full overlap Office
// uses, so the bars sit on one another rather than beside each other.
//
// GroupingClustered has no line/area equivalent and is emitted as
// GroupingStandard there.
func (c *Chart) SetGrouping(g Grouping) *Chart {
	c.grouping = g
	return c
}

// ParseNotes returns the diagnostics Parse recorded while reading an existing
// chart.xml: one line per piece of the source part this model cannot represent,
// such as a plot area holding chart-type groups beyond the one the Chart kind
// stands for (C563). It is empty for a chart built with a New* constructor, and
// for a parsed chart whose plot area was fully represented.
//
// The notes describe read-time loss only. They do not affect marshaling: a
// chart with notes still serializes, it just does not carry everything its
// source did.
func (c *Chart) ParseNotes() []string { return c.parseNotes }

// Clone returns a deep copy of the chart: the copy's series are new values, so
// mutating either chart (or its series) leaves the other alone.
//
// Each format's AddChart points the chart's references at its own host sheet,
// so it clones first — a single *Chart added to two sheets, or to a document
// and a presentation, would otherwise end up with the last host's sheet name in
// every copy of its c:f references (C562).
func (c *Chart) Clone() *Chart {
	if c == nil {
		return nil
	}
	cp := *c
	cp.categories = append([]string(nil), c.categories...)
	cp.parseNotes = append([]string(nil), c.parseNotes...)
	cp.series = make([]*Series, len(c.series))
	for i, s := range c.series {
		s2 := *s
		s2.Values = append([]float64(nil), s.Values...)
		s2.XValues = append([]float64(nil), s.XValues...)
		s2.Sizes = append([]float64(nil), s.Sizes...)
		cp.series[i] = &s2
	}
	return &cp
}

// SetTitle sets the chart title. An empty string clears it.
func (c *Chart) SetTitle(title string) *Chart {
	c.title = title
	return c
}

// SetCategories sets the shared category labels for category charts. It has no
// effect on scatter charts, whose X values are supplied per series.
func (c *Chart) SetCategories(labels []string) *Chart {
	c.categories = append([]string(nil), labels...)
	return c
}

// AddSeries appends a named series of values and returns it. For scatter
// charts use AddXYSeries instead.
//
// A series must carry at least one value: MarshalChartXML rejects an empty one
// rather than emitting a cache declaring no points. Use Blank for a gap in the
// data — a blank point is cached and written as an empty cell, not as zero.
func (c *Chart) AddSeries(name string, values []float64) *Series {
	s := &Series{Name: name, Values: append([]float64(nil), values...)}
	c.series = append(c.series, s)
	return s
}

// AddXYSeries appends a scatter series with paired X and Y coordinates and
// returns it. x and y should have the same length; extra values are ignored.
func (c *Chart) AddXYSeries(name string, x, y []float64) *Series {
	s := &Series{
		Name:    name,
		XValues: append([]float64(nil), x...),
		Values:  append([]float64(nil), y...),
	}
	c.series = append(c.series, s)
	return s
}

// AddBubbleSeries appends a bubble series with paired X, Y, and size values and
// returns it. x, y, and sizes should have the same length; missing trailing
// values are treated as blanks. Use it with NewBubble.
func (c *Chart) AddBubbleSeries(name string, x, y, sizes []float64) *Series {
	s := &Series{
		Name:    name,
		XValues: append([]float64(nil), x...),
		Values:  append([]float64(nil), y...),
		Sizes:   append([]float64(nil), sizes...),
	}
	c.series = append(c.series, s)
	return s
}

// SetLegend shows the legend at the given position.
func (c *Chart) SetLegend(pos LegendPosition) *Chart {
	c.showLegend = true
	c.legendPos = pos
	return c
}

// HideLegend removes the legend.
func (c *Chart) HideLegend() *Chart {
	c.showLegend = false
	return c
}

// SetAxisTitles sets the category (horizontal) and value (vertical) axis
// titles. Empty strings leave the corresponding axis untitled. Pie charts have
// no axes and ignore this.
func (c *Chart) SetAxisTitles(category, value string) *Chart {
	c.catAxisName = category
	c.valAxisName = value
	return c
}

// SetDataRef sets the sheet name that c:f formula references are built
// against. It returns the chart for chaining.
func (c *Chart) SetDataRef(sheet string) *Chart {
	c.DataRef = sheet
	return c
}

// SetDataLabels toggles value data labels on the chart: when on, each data
// point's value is rendered next to it (c:dLbls with showVal). It returns the
// chart for chaining.
func (c *Chart) SetDataLabels(show bool) *Chart {
	c.dataLabels = show
	return c
}

// usesAxes reports whether the chart type uses axes. The pie family (pie,
// doughnut, pie-of-pie, and 3D pie) has none; every other kind does.
func (c *Chart) usesAxes() bool {
	switch c.kind {
	case KindPie, KindDoughnut, KindOfPie, KindPie3D:
		return false
	}
	return true
}

// needsSerAx reports whether the chart type carries a series (depth) axis: the
// 3D bar/line/area charts and the surface chart, which plot series across a
// third axis in addition to the category and value axes.
func (c *Chart) needsSerAx() bool {
	switch c.kind {
	case KindColumn3D, KindBar3D, KindLine3D, KindArea3D, KindSurface:
		return true
	}
	return false
}

// is3D reports whether the chart type renders with a 3D perspective and so
// carries a c:view3D element. Surface charts are excluded: c:surfaceChart is a
// flat top-down contour.
func (c *Chart) is3D() bool {
	switch c.kind {
	case KindColumn3D, KindBar3D, KindLine3D, KindArea3D, KindPie3D:
		return true
	}
	return false
}

// usesTwoValueAxes reports whether the chart plots against two value axes (X
// and Y) rather than a category and a value axis: scatter and bubble charts.
func (c *Chart) usesTwoValueAxes() bool {
	return c.kind == KindScatter || c.kind == KindBubble
}

func (c *Chart) sheet() string {
	if c.DataRef == "" {
		return "Sheet1"
	}
	return c.DataRef
}

func (c *Chart) numberFormat() string {
	if c.NumberFormat == "" {
		return "General"
	}
	return c.NumberFormat
}

// barGrouping resolves the chart's grouping to an ST_BarGrouping value, which
// defaults to clustered.
func (c *Chart) barGrouping() Grouping {
	switch c.grouping {
	case GroupingStandard, GroupingStacked, GroupingPercentStacked, GroupingClustered:
		return c.grouping
	}
	return GroupingClustered
}

// lineGrouping resolves the chart's grouping to an ST_Grouping value, which has
// no clustered form and defaults to standard.
func (c *Chart) lineGrouping() Grouping {
	switch c.grouping {
	case GroupingStacked, GroupingPercentStacked, GroupingStandard:
		return c.grouping
	}
	return GroupingStandard
}
