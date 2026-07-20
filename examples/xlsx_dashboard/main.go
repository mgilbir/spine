// Example: Build a "sales dashboard" workbook.
//
// This program is a guided tour of the newest spine XLSX analytics features. It
// builds a small three-sheet workbook that turns raw sales records into an
// at-a-glance dashboard using, in one place:
//
//   - a flat source-data sheet (regions × months of revenue records)
//   - a native PIVOT TABLE built from that range, cross-tabulating region
//     (row axis) against month (column axis) with two aggregated value fields
//     (Sum of Revenue on the value axis using Sheet.AddPivotTable)
//   - per-row SPARKLINES: a tiny line chart drawn inside a single cell for each
//     region, tracing its six-month revenue trend (Sheet.AddSparklineGroup)
//   - a second, COLUMN sparkline group summarizing the monthly company totals
//   - a full-size line CHART of every region's monthly revenue
//   - supporting formatting: a named title style, number formats, a totals
//     column/row driven by live formulas, and frozen panes
//
// The two data shapes are deliberate and mirror real spreadsheets. The pivot
// needs "tidy" long-format records (one row per Region×Month observation) so it
// can place fields on axes and aggregate them. Sparklines instead need the
// wide "matrix" layout (one row per region, the six months side by side) so a
// single row of cells feeds one in-cell chart. The dashboard therefore carries
// both: a Data sheet of long records and a Dashboard sheet of the wide matrix,
// both derived from the same underlying numbers.
//
// After writing the file it reopens it and reads back the pivot table's field
// layout and the sparkline groups, proving the whole thing survives a real
// save/open cycle, then prints a short success summary.
//
// Run with:
//
//	go run ./examples/xlsx_dashboard
//
// It writes sales_dashboard.xlsx in the working directory (or to the path given
// as the first argument).
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/xlsx"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// The dashboard's data: monthly revenue (USD) for four sales regions over the
// first half of the year. All figures are hard-coded so the example is fully
// deterministic — the same bytes come out on every run.
var (
	regions = []string{"North", "South", "East", "West"}
	months  = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}

	// revenue[i] is region i's revenue across the six months.
	revenue = [][]float64{
		{15200, 16100, 17850, 19200, 21050, 22800}, // North
		{32100, 30400, 31800, 29500, 33200, 35600}, // South
		{8050, 9500, 9100, 11200, 12500, 13800},    // East
		{21400, 23100, 20800, 24600, 26300, 27900}, // West
	}
)

func main() {
	wb := xlsx.Create()
	wb.Properties.Title = "Regional Sales Dashboard"
	wb.Properties.Creator = "Spine Library"

	// A named title style reused for every sheet heading.
	styles := wb.Styles()
	_, err := styles.AddNamedStyle(xlsx.NamedStyle{
		Name: "Heading",
		Style: xlsx.CellStyle{
			Font:      &xlsx.FontStyle{Name: "Calibri", Size: 15, Bold: true, Color: "1F4E79"},
			Alignment: &xlsx.AlignmentStyle{Vertical: "center"},
		},
	})
	must(err)

	buildDataSheet(wb)
	dashboard := buildDashboard(wb)
	buildPivot(wb)

	// Land the reader on the dashboard when the file opens.
	for i, s := range wb.Sheets() {
		if s.Name() == dashboard.Name() {
			must(wb.SetActiveSheet(i))
			break
		}
	}

	// ── SAVE ─────────────────────────────────────────────────────────────
	outputPath := "sales_dashboard.xlsx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		must(os.MkdirAll(dir, 0o755))
	}
	must(wb.Save(outputPath))
	fmt.Printf("Wrote %s\n", outputPath)

	// ── REOPEN and verify the round-trip ─────────────────────────────────
	verify(outputPath)
}

// buildDataSheet writes the long-format source records the pivot reads from:
// one row per Region×Month observation, with the header row naming the three
// fields (Region, Month, Revenue). This is the "tidy" shape pivot tables expect.
func buildDataSheet(wb *xlsx.Workbook) {
	data := wb.AddSheet("Data")
	data.SetTabColor("808080")

	must(data.SetCellValue("A1", "Region"))
	must(data.SetCellValue("B1", "Month"))
	must(data.SetCellValue("C1", "Revenue"))

	moneyStyle := xlsx.CellStyle{Format: "#,##0", Alignment: &xlsx.AlignmentStyle{Horizontal: "right"}}

	// Emit region×month rows in a stable order so the file is deterministic.
	r := 2
	for i, region := range regions {
		for j, month := range months {
			must(data.SetCellValue(fmt.Sprintf("A%d", r), region))
			must(data.SetCellValue(fmt.Sprintf("B%d", r), month))
			must(data.SetCellValue(fmt.Sprintf("C%d", r), revenue[i][j]))
			cell, err := data.Cell(fmt.Sprintf("C%d", r))
			must(err)
			must(cell.SetStyle(moneyStyle))
			r++
		}
	}

	must(data.SetColWidth(1, 12))
	must(data.SetColWidth(2, 10))
	must(data.SetColWidth(3, 12))
}

// buildDashboard writes the wide region×month matrix and decorates it with
// sparklines and a chart. The matrix (one row per region) is what the per-row
// sparklines and the chart both read from.
func buildDashboard(wb *xlsx.Workbook) *xlsx.Sheet {
	sheet := wb.AddSheet("Dashboard")
	sheet.SetTabColor("1F4E79")

	// Title.
	must(sheet.SetCellValue("A1", "Regional Sales Dashboard — H1 FY2026"))
	must(sheet.MergeCells("A1", "I1"))
	must(sheet.SetRowHeight(1, 26))
	title, err := sheet.Cell("A1")
	must(err)
	must(title.SetNamedStyle("Heading"))

	// The matrix header: Region, then the six month columns, then Total and a
	// Trend column that will hold the per-row sparkline.
	//
	// Column map (1-based): A=Region, B..G=Jan..Jun, H=Total, I=Trend.
	const (
		headerRow = 3
		firstData = 4 // first region row
		trendCol  = 9 // column I
		monthCol0 = 2 // column B = first month
	)
	headerStyle := xlsx.CellStyle{
		Font:      &xlsx.FontStyle{Bold: true, Color: "FFFFFF"},
		Fill:      &xlsx.FillStyle{Pattern: "solid", FgColor: "4472C4"},
		Alignment: &xlsx.AlignmentStyle{Horizontal: "center"},
	}
	must(sheet.SetCellValue("A3", "Region"))
	for j, month := range months {
		ref, _ := xlsx.CellRef(headerRow, monthCol0+j)
		must(sheet.SetCellValue(ref, month))
	}
	must(sheet.SetCellValue("H3", "Total"))
	must(sheet.SetCellValue("I3", "Trend"))
	for c := 1; c <= trendCol; c++ {
		ref, _ := xlsx.CellRef(headerRow, c)
		cell, err := sheet.Cell(ref)
		must(err)
		must(cell.SetStyle(headerStyle))
	}

	// One row per region: the six monthly values plus a live SUM total.
	moneyStyle := xlsx.CellStyle{Format: "#,##0", Alignment: &xlsx.AlignmentStyle{Horizontal: "right"}}
	for i, region := range regions {
		row := firstData + i
		must(sheet.SetCellValue(fmt.Sprintf("A%d", row), region))
		for j := range months {
			ref, _ := xlsx.CellRef(row, monthCol0+j)
			must(sheet.SetCellValue(ref, revenue[i][j]))
			cell, err := sheet.Cell(ref)
			must(err)
			must(cell.SetStyle(moneyStyle))
		}
		total, err := sheet.Cell(fmt.Sprintf("H%d", row))
		must(err)
		total.SetFormula(fmt.Sprintf("SUM(B%d:G%d)", row, row))
		must(total.SetStyle(xlsx.CellStyle{
			Format:    "#,##0",
			Font:      &xlsx.FontStyle{Bold: true},
			Alignment: &xlsx.AlignmentStyle{Horizontal: "right"},
		}))
	}

	// A totals-by-month row beneath the matrix: each month's company-wide sum.
	// It doubles as the source for the column sparkline group below.
	totalsRow := firstData + len(regions)
	must(sheet.SetCellValue(fmt.Sprintf("A%d", totalsRow), "All regions"))
	labelCell, err := sheet.Cell(fmt.Sprintf("A%d", totalsRow))
	must(err)
	must(labelCell.SetStyle(xlsx.CellStyle{Font: &xlsx.FontStyle{Bold: true}}))
	for j := range months {
		colLetter := string(rune('A' + monthCol0 - 1 + j))
		cell, err := sheet.Cell(fmt.Sprintf("%s%d", colLetter, totalsRow))
		must(err)
		cell.SetFormula(fmt.Sprintf("SUM(%s%d:%s%d)", colLetter, firstData, colLetter, totalsRow-1))
		must(cell.SetStyle(xlsx.CellStyle{
			Format:    "#,##0",
			Font:      &xlsx.FontStyle{Bold: true},
			Alignment: &xlsx.AlignmentStyle{Horizontal: "right"},
		}))
	}

	// ── SPARKLINES (per region) ──────────────────────────────────────────
	//
	// A sparkline group holds one (data range, location cell) mapping per
	// sparkline. Here each region row's six month cells (B..G) feed a tiny
	// line drawn in that row's Trend cell (column I). Data ranges are
	// sheet-qualified so the sparklines keep pointing at the right cells even
	// if the group is moved. One AddSparklineGroup call renders all four.
	var rowSparks []xlsx.SparklineData
	for i := range regions {
		row := firstData + i
		rowSparks = append(rowSparks, xlsx.SparklineData{
			DataRange:    fmt.Sprintf("Dashboard!B%d:G%d", row, row),
			LocationCell: fmt.Sprintf("I%d", row),
		})
	}
	_, err = sheet.AddSparklineGroup(xlsx.SparklineOptions{
		Type:        xlsx.SparklineLine,
		SeriesColor: "1F4E79",
		Data:        rowSparks,
	})
	must(err)

	// ── SPARKLINES (monthly totals, as columns) ──────────────────────────
	//
	// A second group, this time column-style, drawn in the Trend cell of the
	// "All regions" totals row: six little bars, one per month, showing the
	// shape of company-wide revenue over the half.
	_, err = sheet.AddSparklineGroup(xlsx.SparklineOptions{
		Type:        xlsx.SparklineColumn,
		SeriesColor: "4472C4",
		Data: []xlsx.SparklineData{{
			DataRange:    fmt.Sprintf("Dashboard!B%d:G%d", totalsRow, totalsRow),
			LocationCell: fmt.Sprintf("I%d", totalsRow),
		}},
	})
	must(err)

	// ── CHART ────────────────────────────────────────────────────────────
	//
	// A full-size line chart: months along the category axis, one line per
	// region. It complements the sparklines — the same trends, drawn large.
	line := chart.NewLine().
		SetTitle("Monthly Revenue by Region").
		SetCategories(months).
		SetAxisTitles("Month", "Revenue (USD)").
		SetLegend(chart.LegendBottom)
	for i, region := range regions {
		line.AddSeries(region, revenue[i])
	}
	must(sheet.AddChart(fmt.Sprintf("A%d:I%d", totalsRow+2, totalsRow+20), line))

	// Freeze the title and header rows, and the Region column, so they stay
	// visible while scrolling.
	must(sheet.FreezePanes(fmt.Sprintf("B%d", firstData)))
	sheet.SetShowGridLines(false)

	// Column widths.
	must(sheet.SetColWidth(1, 12))
	for c := monthCol0; c <= 8; c++ {
		must(sheet.SetColWidth(c, 9))
	}
	must(sheet.SetColWidth(trendCol, 12))
	return sheet
}

// buildPivot cross-tabulates the long-format Data records into a pivot table
// anchored on a dedicated sheet: regions down the rows, months across the
// columns, revenue summed and averaged in the body.
func buildPivot(wb *xlsx.Workbook) {
	pivotSheet := wb.AddSheet("Pivot")
	pivotSheet.SetTabColor("70AD47")

	must(pivotSheet.SetCellValue("A1", "Revenue Pivot (region × month)"))
	head, err := pivotSheet.Cell("A1")
	must(err)
	must(head.SetNamedStyle("Heading"))

	// Source is the whole Data table: header row plus 24 observation rows.
	sourceRange := fmt.Sprintf("Data!A1:C%d", 1+len(regions)*len(months))
	_, err = pivotSheet.AddPivotTable(sourceRange, "A3", xlsx.PivotOptions{
		Name:         "RevenueByRegionMonth",
		RowFields:    []string{"Region"},
		ColumnFields: []string{"Month"},
		ValueFields: []xlsx.PivotValueField{
			{Field: "Revenue", Aggregation: xlsx.PivotSum, Name: "Sum of Revenue"},
			{Field: "Revenue", Aggregation: xlsx.PivotAverage, Name: "Avg Revenue"},
		},
	})
	must(err)
}

// verify reopens the workbook and prints a summary proving the pivot table and
// the sparkline groups round-tripped through a real save/open cycle. Any
// inconsistency is fatal, so `go run` exits non-zero if the file did not come
// back the way we wrote it.
func verify(path string) {
	wb, err := xlsx.Open(path)
	must(err)

	fmt.Println("Reopened and verified:")

	// ── Pivot table field layout ─────────────────────────────────────────
	pivots := wb.PivotTables()
	if len(pivots) != 1 {
		log.Fatalf("expected 1 pivot table, got %d", len(pivots))
	}
	p := pivots[0]
	fmt.Printf("  - pivot %q at %s, source %s!%s\n",
		p.Name(), p.Location(), p.SourceSheet(), p.SourceRange())
	fmt.Printf("      row fields:    %v\n", p.RowFields())
	fmt.Printf("      column fields: %v\n", p.ColumnFields())
	for _, v := range p.ValueFields() {
		fmt.Printf("      value field:   %-14s = %s(%s)\n", v.Name, v.Aggregation, v.Field)
	}

	// ── Sparkline groups ─────────────────────────────────────────────────
	dashboard, err := wb.SheetByName("Dashboard")
	must(err)
	groups := dashboard.Sparklines()
	if len(groups) != 2 {
		log.Fatalf("expected 2 sparkline groups, got %d", len(groups))
	}
	for _, g := range groups {
		sparks := g.Sparklines()
		first := "(none)"
		if len(sparks) > 0 {
			first = fmt.Sprintf("%s → %s", sparks[0].DataRange, sparks[0].LocationCell)
		}
		fmt.Printf("  - %-6s sparkline group: %d sparkline(s), color %s, e.g. %s\n",
			g.Type(), len(sparks), g.SeriesColor(), first)
	}

	// ── Chart ────────────────────────────────────────────────────────────
	charts := dashboard.Charts()
	if len(charts) != 1 {
		log.Fatalf("expected 1 chart, got %d", len(charts))
	}
	fmt.Printf("  - chart %q with %d series\n", charts[0].Title(), len(charts[0].SeriesList()))

	fmt.Println("All features round-tripped successfully.")
}
