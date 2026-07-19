// Package chart provides a small, format-agnostic API for building
// DrawingML charts and serializing them to a chart.xml part
// (c:chartSpace) that any of the xlsx, docx, or pptx integrations can embed.
//
// The package wraps the verbose internal model in common/dml/chart with an
// ergonomic builder: pick a chart type, set categories, add series, and
// serialize. Cached values (c:numCache / c:strCache) are populated from the
// supplied data so the chart renders standalone, without a live data source.
//
// The c:f formula references point at a conventional data location (a sheet
// named by DataRef, default "Sheet1"). Format integrations (Phase B) supply
// the real host or embedded-workbook location by setting DataRef and, for
// docx/pptx, embedding the workbook returned by EmbeddedWorkbook.
//
// This package delivers the reusable core only. Wiring it into each format's
// Open/Save path (an AddChart method per format, a Charts() reader) is Phase B.
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
)

// String returns the chart kind's element name (without the c: prefix).
func (k Kind) String() string {
	switch k {
	case KindColumn, KindBar:
		return "barChart"
	case KindLine:
		return "lineChart"
	case KindPie:
		return "pieChart"
	case KindScatter:
		return "scatterChart"
	case KindArea:
		return "areaChart"
	case KindDoughnut:
		return "doughnutChart"
	case KindRadar:
		return "radarChart"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

// defaultHoleSize is the doughnut hole diameter as a percentage of the outer
// radius (CT_HoleSize, 1-90). 50 matches Office's default for a new doughnut.
const defaultHoleSize = 50

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
	XValues []float64 // scatter only

	// Color is an optional solid fill for the series, as a 6-digit hex RGB
	// string ("FF0000"). Empty leaves the series to the theme's automatic
	// color. Set it with SetColor.
	Color string
}

// SetColor sets the series' solid fill to the given RGB color and returns the
// series for chaining. hexRGB is a 6-digit hex string, with or without a
// leading '#' ("#1F77B4" or "1f77b4"); it is normalized to upper-case. An empty
// string clears the color, restoring the automatic (theme) color.
func (s *Series) SetColor(hexRGB string) *Series {
	s.Color = normalizeHexColor(hexRGB)
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

// NewPie returns a pie chart.
func NewPie() *Chart { return newChart(KindPie) }

// NewScatter returns an XY scatter chart.
func NewScatter() *Chart { return newChart(KindScatter) }

// NewArea returns an area chart.
func NewArea() *Chart { return newChart(KindArea) }

// NewDoughnut returns a doughnut chart: a pie chart with a hole. Like a pie it
// plots a single series (its first).
func NewDoughnut() *Chart { return newChart(KindDoughnut) }

// NewRadar returns a radar (spider) chart.
func NewRadar() *Chart { return newChart(KindRadar) }

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

// usesAxes reports whether the chart type uses axes. Pie and doughnut charts
// have none; every other kind (including radar) does.
func (c *Chart) usesAxes() bool { return c.kind != KindPie && c.kind != KindDoughnut }

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
