# Charts

A format-agnostic `chart` package builds DrawingML charts (column, bar, line,
pie, doughnut, radar, scatter, area, combination, bubble, stock, surface,
pie-of-pie, and 3D column/bar/line/pie/area charts) and serializes a
schema-valid `chart.xml` with cached values, plus a matching embedded workbook
whose cells are exactly the ones the chart's `c:f` references point at. Data
that cannot be expressed in a valid part — a chart with no series, a series
with no values, a stock chart without its three or four price lines — is
rejected by `MarshalChartXML` with an error rather than written out. Series carry
optional solid colors (`Series.SetColor`), and the chart renders value data
labels chart-wide (`Chart.SetDataLabels`); combination charts (`NewCombo`) mix
per-series types (`Series.SetType`) across a primary and secondary value axis
(`Series.SetSecondaryAxis`). All three formats wire it in with symmetric
`AddChart` / `Charts()` methods: `xlsx` references the host workbook's cells
(`Sheet.AddChart` / `Sheet.Charts` / `Workbook.Charts`); `docx`
(`Document.AddChart` / `Paragraph.AddChart` / `Document.Charts`) and `pptx`
(`Slide.AddChart` / `Slide.Charts` / `Presentation.Charts`) embed the data
workbook.

## The chart package

The `chart` package builds a DrawingML `chart.xml` part (`c:chartSpace`) that is
independent of any one document format. Pick a chart type, set categories, add
series, and serialize:

```go
package main

import (
	"os"

	"github.com/mgilbir/spine/chart"
)

func main() {
	c := chart.NewColumn().
		SetTitle("Quarterly Sales").
		SetCategories([]string{"Q1", "Q2", "Q3", "Q4"}).
		SetAxisTitles("Quarter", "USD").
		SetLegend(chart.LegendRight).
		SetDataLabels(true) // render each point's value on the chart
	c.AddSeries("North", []float64{10, 20, 30, 40}).SetColor("#1F77B4")
	c.AddSeries("South", []float64{5, 15, 25, 35}).SetColor("#FF7F0E")

	// chart.xml with cached values so it renders without a live data source.
	xmlBytes, _ := c.MarshalChartXML()
	_ = os.WriteFile("chart1.xml", xmlBytes, 0o644)

	// The workbook docx/pptx charts embed so Office can edit the data. The
	// returned layout's cell ranges line up with the chart's c:f references.
	wbBytes, layout, _ := c.EmbeddedWorkbook()
	_ = wbBytes
	_ = layout

	// Read a chart.xml back into the model.
	parsed, _ := chart.Parse(xmlBytes)
	_ = parsed
}
```

Supported types: `NewColumn`, `NewBar` (horizontal), `NewLine`, `NewPie`,
`NewDoughnut`, `NewRadar`, `NewScatter` (via `AddXYSeries`), `NewArea`,
`NewCombo` (combination), `NewBubble` (x/y/size points, via `AddBubbleSeries`),
`NewStock` (high-low-close), `NewSurface` (filled contour), `NewOfPie`
(pie-of-pie), and the 3D variants `NewColumn3D`, `NewBar3D`, `NewLine3D`,
`NewPie3D`, and `NewArea3D`. Cached values (`c:numCache` / `c:strCache`) are
populated from the supplied data; `c:f` references are built against a
configurable `DataRef` sheet (default `Sheet1`).

Formatting is opt-in and symmetric with the constructors:

- `SetDataLabels(true)` emits `c:dLbls` (showVal) so each point's value renders
  on the chart. It round-trips through `Charts()` (`Chart.DataLabels()`).
- `Series.SetColor("#1F77B4")` gives a series a solid RGB fill
  (`c:spPr` / `a:solidFill` / `a:srgbClr`); the leading `#` is optional and the
  value is recovered on read as `Series.Color`.
- `SetGrouping` arranges a bar, column, line, or area chart's series:
  `GroupingClustered` (the bar default), `GroupingStandard` (the line/area
  default), `GroupingStacked`, or `GroupingPercentStacked`. A stacked bar chart
  also emits the full overlap Office uses. It round-trips through `Charts()`
  (`Chart.Grouping()`).

### Blank data points

A spreadsheet cell can be empty, and a chart plots that as a gap rather than as
zero. `chart.Blank()` is the value that marks one, and `chart.IsBlank(v)` is the
predicate every consumer of `Series.Values` should use before treating a value
as a number (it is a NaN, so `==` never matches it):

```go
c.AddSeries("North", []float64{10, chart.Blank(), 30})
```

A blank is written the way Excel writes one: it counts towards the cache's
`ptCount` but emits no `c:pt`, and the corresponding cell of the data sheet (or
the embedded workbook) is left empty. Reading a chart back turns each gap in a
cache into a blank at the same index, so a series always stays aligned with its
categories.

### Which series a chart plots

Every chart type plots all of its series except the single-series pie family:
`NewPie`, `NewPie3D`, and `NewOfPie` render the first series only. `NewDoughnut`
plots every series, one concentric ring each. The data of an unplotted series is
still written to the data sheet, so nothing is lost and the chart can be
re-pointed at it in Office.

`NewStock` requires three or four series — high, low, close, optionally preceded
by open — and reports an error otherwise, as CT_StockChart admits no other
count.

### What `Charts()` recovers, and what it does not

A `*chart.Chart` is this package's builder model, not a faithful `c:chartSpace`.
Reading recovers the type, title, legend, axis titles, data labels, grouping,
number format, categories, and each series' name, values, and color. Everything
else is re-emitted at the builder's defaults if the chart is marshaled again:
per-series markers and smoothing, doughnut hole size, gap width and overlap,
explicit axis identifiers, scaling, tick and gridline settings, `dispBlanksAs`,
per-point formatting, trendlines, error bars, data tables, 3D view angles, and
extension lists. Plan a read-modify-re-embed pipeline accordingly — it rebuilds
the part rather than editing it.

A plot area can also hold chart-type groups the model has no place for (a
`barChart` and a `scatterChart` side by side, say). Those are dropped, and each
one is reported by `Chart.ParseNotes()` so a caller can tell "this is a column
chart" from "it had two groups and I kept one".

A **combination chart** mixes series types on a shared category axis. Give each
series a plot type with `Series.SetType` (`KindColumn`, `KindLine`, or
`KindArea`) and, optionally, move it to the right-hand secondary value axis with
`Series.SetSecondaryAxis(true)`:

```go
c := chart.NewCombo().SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
c.AddSeries("Revenue", []float64{100, 120, 140, 160}).SetType(chart.KindColumn)
c.AddSeries("Margin %", []float64{12, 15, 14, 18}).
    SetType(chart.KindLine).SetSecondaryAxis(true)
```

`Charts()` reads a combo back as `KindCombo` with each series' `PlotType` and
`SecondaryAxis` recovered.

All types (including combination charts) flow through every format's `AddChart`
and are read back by `Charts()`. `AddChart` copies the chart before pointing its
references at the host's data sheet, so the value you pass keeps its own
`DataRef` and can be added to several sheets, workbooks, or documents; later
edits to it do not change what an earlier host saves.

This package is the shared core. Each format wires it in with an `AddChart`
method and a `Charts()` reader (xlsx references the host sheet; docx and pptx
embed the workbook from `EmbeddedWorkbook`).

## Charts in spreadsheets

In `xlsx`, `Sheet.AddChart` anchors a chart on a sheet and `Sheet.Charts` /
`Workbook.Charts` read the charts back:

```go
package main

import (
	"bytes"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/xlsx"
)

func main() {
	wb := xlsx.Create()
	sheet := wb.AddSheet("Sales")
	_ = sheet.SetCellValue("A1", "Region")

	c := chart.NewColumn().
		SetTitle("Quarterly Sales").
		SetCategories([]string{"Q1", "Q2", "Q3", "Q4"}).
		SetLegend(chart.LegendRight)
	c.AddSeries("North", []float64{10, 20, 30, 40})
	c.AddSeries("South", []float64{5, 15, 25, 35})

	// Anchor the chart at E2 (a single cell places a default-sized chart; a
	// range like "E2:L20" sizes it to that block).
	_ = sheet.AddChart("E2", c)

	data, _ := wb.SaveBytes()

	wb2, _ := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
	s2, _ := wb2.SheetByName("Sales")
	for _, got := range s2.Charts() {
		_ = got.Title()      // "Quarterly Sales"
		_ = got.Categories() // ["Q1" "Q2" "Q3" "Q4"]
		_ = got.SeriesList() // series names + values
	}
}
```

An xlsx chart references cells in the host workbook rather than an embedded
workbook. `AddChart` writes the chart's data (categories and each series) into a
dedicated hidden worksheet — one per chart — and points the chart's `c:f`
references at it, so Excel's "Edit Data" opens real cells while the cached
values keep the chart rendering standalone; the sheet's own cells are never
touched. Charts and images coexist in one drawing part per sheet. `AddChart`
persists on both the `Create` and `Open` save paths, and a zero-modification
open→save of a chart-bearing workbook stays byte-identical (the chart and
drawing parts are preserved verbatim unless a chart is added or modified).

`Sheet.Charts` covers chartsheets as well as worksheets, so `Workbook.Charts`
really does span every sheet of an opened workbook. One gap remains: a sheet
copied in with `Workbook.Merge` brings no drawing across, so charts on the
source sheet do not come with it and are not reported.

## Charts in Word documents

`docx` inserts a chart inline in the text flow, like an inline image. The
chart's data is written to an embedded workbook (`word/embeddings/…xlsx`) that
the chart part references, so Office can open and edit the values — a docx has
no host worksheet. Sizes are in EMUs (914400 per inch):

```go
doc := docx.Create()
doc.AddParagraphWithText("Quarterly revenue:")

c := chart.NewColumn().
	SetTitle("Revenue by Quarter").
	SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
c.AddSeries("North", []float64{10, 20, 30, 40})

// Appends a paragraph holding the chart (~5.5in x 3in). Use
// Paragraph.AddChart to place a chart inline among other runs.
if err := doc.AddChart(c, 5029200, 2743200); err != nil {
	log.Fatal(err)
}
_ = doc.Save("charted.docx")

// Read charts back from an opened document.
opened, _ := docx.Open("charted.docx")
for _, ch := range opened.Charts() {
	fmt.Println(ch.Title(), ch.Categories())
}
```

A zero-modification open→save of a chart-bearing document is byte-identical:
the chart and embedded-workbook parts round-trip verbatim, and are regenerated
only when a chart is added.

## Charts in PowerPoint

`Slide.AddChart` places a chart on a slide. Because a presentation has no host
workbook, the chart's data is embedded as a small `.xlsx` package that Office
can open to edit the data; the chart part, the embedded workbook, the wiring
relationships, and the content-type overrides are all created for you. Position
and size are given in EMUs.

```go
package main

import (
	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/pptx"
)

func main() {
	p := pptx.Create()
	slide := p.AddSlide()

	c := chart.NewColumn().
		SetTitle("Quarterly Revenue").
		SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
	c.AddSeries("2024", []float64{10, 20, 15, 25})
	c.AddSeries("2025", []float64{12, 18, 22, 30})

	// x, y, width, height in EMUs (914400 EMU = 1 inch).
	if err := slide.AddChart(c, 914400, 1828800, 5486400, 3657600); err != nil {
		panic(err)
	}
	_ = p.Save("charts.pptx")
}
```

Read charts back with `Slide.Charts()` or `Presentation.Charts()`, which return
the parsed `*chart.Chart` definitions (type, title, categories, and series).
A chart added this way coexists with the slide's existing shapes, and opening
and re-saving a chart-bearing deck without changes preserves the chart and
embedding parts byte-for-byte.
