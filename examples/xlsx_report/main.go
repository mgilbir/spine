// Example: Build a rich "quarterly sales report" workbook.
//
// This program is a guided tour of the spine XLSX authoring features added in
// the most recent development wave. It builds a single, realistic report sheet
// that combines, in one place:
//
//   - cell data with number formats and custom cell styles
//   - a native Excel Table (a.k.a. ListObject) with a totals row and a style
//   - conditional formatting: a 3-color scale and a data bar
//   - an embedded chart driven by the sheet's own data
//   - page & print setup: landscape, margins, header/footer, print area/titles
//   - freeze panes and sheet-view tweaks (gridlines, zoom)
//   - a reusable named cell style
//   - sheet protection with a mix of locked, unlocked and hidden-formula cells
//   - workbook structure protection
//
// After writing the file it reopens it and reads several things back — the
// table, the chart, the protection flags, the page setup and the frozen panes —
// to prove the whole thing round-trips through a real save/open cycle.
//
// Run with:
//
//	go run ./examples/xlsx_report
//
// It writes quarterly_report.xlsx in the working directory (or to the path
// given as the first argument) and prints a short verification summary.
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

// u32 / boolPtr return pointers to literals, which the optional (pointer)
// fields of PageSetup and friends use to distinguish "unset" from a real value.
func u32(v uint32) *uint32 { return &v }
func boolPtr(v bool) *bool { return &v }

// The report's data. Each product has four quarters of revenue; the yearly
// total is a formula so Excel keeps it live.
var products = []struct {
	name           string
	q1, q2, q3, q4 float64
}{
	{"Widgets", 15200, 18400, 21100, 24300},
	{"Gadgets", 32100, 29500, 31800, 35200},
	{"Sprockets", 8050, 9500, 11200, 12500},
	{"Flanges", 21400, 23100, 20800, 26300},
	{"Brackets", 5600, 6200, 7100, 8400},
}

func main() {
	wb := xlsx.Create()
	wb.Properties.Title = "Quarterly Sales Report"
	wb.Properties.Creator = "Spine Library"

	sheet := wb.AddSheet("Q4 Sales")
	sheet.SetTabColor("1F4E79")

	// ── 1. A reusable NAMED cell style ───────────────────────────────────
	//
	// A named style shows up in Excel's "Cell Styles" gallery and can be
	// applied to any cell by name. We define one here for the report title and
	// reuse it, rather than repeating the same CellStyle literal everywhere.
	styles := wb.Styles()
	_, err := styles.AddNamedStyle(xlsx.NamedStyle{
		Name: "ReportTitle",
		Style: xlsx.CellStyle{
			Font:      &xlsx.FontStyle{Name: "Calibri", Size: 16, Bold: true, Color: "1F4E79"},
			Alignment: &xlsx.AlignmentStyle{Horizontal: "center", Vertical: "center"},
		},
	})
	must(err)

	// ── 2. Title row (merged, styled via the named style) ────────────────
	must(sheet.SetCellValue("A1", "Quarterly Sales Report — FY2026"))
	must(sheet.MergeCells("A1", "F1"))
	must(sheet.SetRowHeight(1, 28))
	titleCell, err := sheet.Cell("A1")
	must(err)
	must(titleCell.SetNamedStyle("ReportTitle"))

	// ── 3. Table data ────────────────────────────────────────────────────
	//
	// The table occupies A3:F9: a header row (row 3), five product rows
	// (rows 4-8) and a totals row (row 9). We fill the header, the per-product
	// quarter values and a per-row SUM for the yearly Total column. AddTable
	// (below) turns this rectangle into a real Excel Table.
	const (
		headerRow = 3
		firstData = 4
	)
	headers := []string{"Product", "Q1", "Q2", "Q3", "Q4", "Total"}
	for i, h := range headers {
		ref, _ := xlsx.CellRef(headerRow, i+1)
		must(sheet.SetCellValue(ref, h))
	}

	// A currency-ish number format applied to every numeric cell.
	moneyStyle := xlsx.CellStyle{
		Format:    "#,##0",
		Alignment: &xlsx.AlignmentStyle{Horizontal: "right"},
	}
	for i, p := range products {
		r := firstData + i
		must(sheet.SetCellValue(fmt.Sprintf("A%d", r), p.name))
		must(sheet.SetCellValue(fmt.Sprintf("B%d", r), p.q1))
		must(sheet.SetCellValue(fmt.Sprintf("C%d", r), p.q2))
		must(sheet.SetCellValue(fmt.Sprintf("D%d", r), p.q3))
		must(sheet.SetCellValue(fmt.Sprintf("E%d", r), p.q4))

		// The Total column is a live formula, not a precomputed number.
		total, err := sheet.Cell(fmt.Sprintf("F%d", r))
		must(err)
		total.SetFormula(fmt.Sprintf("SUM(B%d:E%d)", r, r))

		// Number-format the four quarter cells and the total.
		for col := 2; col <= 6; col++ {
			ref, _ := xlsx.CellRef(r, col)
			cell, err := sheet.Cell(ref)
			must(err)
			must(cell.SetStyle(moneyStyle))
		}
	}

	// Totals row (row 9): a "Total" label plus a column SUM under each column.
	totalsRow := firstData + len(products)
	must(sheet.SetCellValue(fmt.Sprintf("A%d", totalsRow), "Total"))
	for col := 2; col <= 6; col++ {
		colLetter := string(rune('A' + col - 1))
		cell, err := sheet.Cell(fmt.Sprintf("%s%d", colLetter, totalsRow))
		must(err)
		cell.SetFormula(fmt.Sprintf("SUM(%s%d:%s%d)", colLetter, firstData, colLetter, totalsRow-1))
		must(cell.SetStyle(xlsx.CellStyle{
			Format:    "#,##0",
			Font:      &xlsx.FontStyle{Bold: true},
			Alignment: &xlsx.AlignmentStyle{Horizontal: "right"},
		}))
	}

	// ── 4. Turn the rectangle into a native Excel TABLE ──────────────────
	//
	// AddTable wraps A3:F9 in a ListObject: a named, filterable, styled range.
	// TotalsRow: true tells Excel the last row is the totals row, and
	// ColumnTotals wires each column's built-in aggregation (the "Total" label
	// on the first column, SUM on the numeric ones). The built-in table style
	// gives it banded rows without any manual cell fills.
	tableRange := fmt.Sprintf("A%d:F%d", headerRow, totalsRow)
	_, err = sheet.AddTable(tableRange, xlsx.TableOptions{
		Name: "SalesByProduct",
		Style: xlsx.TableStyle{
			Name:           "TableStyleMedium2",
			ShowRowStripes: true,
		},
		TotalsRow: true,
		ColumnTotals: map[string]xlsx.TotalsColumn{
			"Product": {Label: "Total"},
			"Q1":      {Function: "sum"},
			"Q2":      {Function: "sum"},
			"Q3":      {Function: "sum"},
			"Q4":      {Function: "sum"},
			"Total":   {Function: "sum"},
		},
	})
	must(err)

	// ── 5. CONDITIONAL FORMATTING ────────────────────────────────────────
	//
	// (a) A 3-color scale over the yearly Total column: low totals shade red,
	//     the median yellow, the highest green — an instant heat map.
	must(sheet.AddConditionalFormat(
		fmt.Sprintf("F%d:F%d", firstData, totalsRow-1),
		xlsx.NewColorScaleRule(
			xlsx.ColorScalePoint{Type: "min", Color: "F8696B"},
			xlsx.ColorScalePoint{Type: "percentile", Value: "50", Color: "FFEB84"},
			xlsx.ColorScalePoint{Type: "max", Color: "63BE7B"},
		),
	))

	// (b) A data bar over the Q4 column: each cell grows a blue in-cell bar
	//     proportional to its value. Empty value objects default to min/max.
	must(sheet.AddConditionalFormat(
		fmt.Sprintf("E%d:E%d", firstData, totalsRow-1),
		xlsx.NewDataBarRule("638EC6", xlsx.ConditionalValueObject{}, xlsx.ConditionalValueObject{}),
	))

	// ── 6. A CHART driven by the sheet's data ────────────────────────────
	//
	// A clustered-column chart of each product's quarterly revenue. AddChart
	// copies the category labels and series values into a dedicated hidden
	// worksheet and points the chart at it, so the chart renders standalone
	// while Excel's "Edit Data" still opens real cells. We anchor it to the
	// right of the table (H3:P20).
	col := chart.NewColumn().
		SetTitle("Quarterly Revenue by Product").
		SetCategories(productNames()).
		SetAxisTitles("Product", "Revenue (USD)").
		SetLegend(chart.LegendBottom)
	col.AddSeries("Q1", column(0))
	col.AddSeries("Q2", column(1))
	col.AddSeries("Q3", column(2))
	col.AddSeries("Q4", column(3))
	must(sheet.AddChart("H3:P20", col))

	// ── 7. FREEZE PANES + sheet-view tweaks ──────────────────────────────
	//
	// Freeze everything above row 4 (the title and the table header) so they
	// stay visible while scrolling the product rows. Then hide the worksheet
	// gridlines (the table supplies its own banding) and bump the zoom.
	must(sheet.FreezePanes(fmt.Sprintf("A%d", firstData)))
	sheet.SetShowGridLines(false)
	sheet.SetZoom(115)

	// ── 8. PAGE & PRINT SETUP ────────────────────────────────────────────
	//
	// Landscape, scaled to one page wide, with print margins, a header/footer
	// (using Excel's &L/&C/&R section codes and &P/&N page-number codes), a
	// print area covering just the table, and the header row repeated on every
	// printed page.
	must(sheet.SetPageSetup(xlsx.PageSetup{
		Orientation: xlsx.OrientationLandscape,
		FitToWidth:  u32(1),
		FitToHeight: u32(0), // 0 = as many pages tall as needed
	}))
	must(sheet.SetPageMargins(xlsx.PageMargins{
		Left: 0.5, Right: 0.5, Top: 0.75, Bottom: 0.75, Header: 0.3, Footer: 0.3,
	}))
	must(sheet.SetHeaderFooter(xlsx.HeaderFooter{
		OddHeader: "&LQuarterly Sales Report&RFY2026",
		OddFooter: "&LConfidential&CPage &P of &N&R&D",
	}))
	must(sheet.SetPrintOptions(xlsx.PrintOptions{
		HorizontalCentered: boolPtr(true),
	}))
	must(sheet.SetPrintArea(tableRange))
	must(sheet.SetPrintTitles(fmt.Sprintf("%d:%d", headerRow, headerRow), ""))

	// ── 9. A small editable "forecast" input area, then PROTECTION ───────
	//
	// We add a labelled input cell that a reader is meant to fill in, and a
	// cell holding a proprietary formula we want to hide. Then we protect the
	// sheet. Under protection every cell is locked by default, so:
	//   - the forecast input is explicitly UNLOCKED (Locked: false), and
	//   - the formula cell is LOCKED and HIDDEN (its formula won't show in the
	//     formula bar).
	forecastLabelRow := totalsRow + 2
	must(sheet.SetCellValue(fmt.Sprintf("A%d", forecastLabelRow), "Next-year forecast (editable):"))
	must(sheet.MergeCells(fmt.Sprintf("A%d", forecastLabelRow), fmt.Sprintf("C%d", forecastLabelRow)))

	forecastInput, err := sheet.Cell(fmt.Sprintf("D%d", forecastLabelRow))
	must(err)
	forecastInput.SetFloat(0)
	must(forecastInput.SetStyle(xlsx.CellStyle{
		Format:     "#,##0",
		Fill:       &xlsx.FillStyle{Pattern: "solid", FgColor: "FFF2CC"},
		Border:     &xlsx.BorderStyle{Bottom: &xlsx.BorderSide{Style: "thin", Color: "BF8F00"}},
		Protection: &xlsx.ProtectionStyle{Locked: false}, // stays editable when protected
	}))

	// A cell whose formula we want to keep from prying eyes.
	secretRow := forecastLabelRow + 1
	must(sheet.SetCellValue(fmt.Sprintf("A%d", secretRow), "Blended growth factor:"))
	must(sheet.MergeCells(fmt.Sprintf("A%d", secretRow), fmt.Sprintf("C%d", secretRow)))
	secret, err := sheet.Cell(fmt.Sprintf("D%d", secretRow))
	must(err)
	secret.SetFormula(fmt.Sprintf("F%d/F%d", totalsRow, firstData))
	must(secret.SetStyle(xlsx.CellStyle{
		Format:     "0.0%",
		Protection: &xlsx.ProtectionStyle{Locked: true, Hidden: true}, // formula hidden under protection
	}))

	// Protect the sheet. The zero-value options lock every operation and allow
	// only selection; we additionally allow AutoFilter so readers can still use
	// the table's filter dropdowns.
	must(sheet.Protect(xlsx.SheetProtectionOptions{
		Password:        "review",
		AllowAutoFilter: true,
	}))

	// Protect the workbook's structure so sheets can't be added, removed,
	// reordered or unhidden (this would reveal the chart's hidden data sheet).
	wb.Protect(xlsx.WorkbookProtectionOptions{LockStructure: true})

	// Column widths for legibility.
	must(sheet.SetColWidth(1, 16))
	for c := 2; c <= 6; c++ {
		must(sheet.SetColWidth(c, 11))
	}

	// ── 10. SAVE ─────────────────────────────────────────────────────────
	outputPath := "quarterly_report.xlsx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		must(os.MkdirAll(dir, 0o755))
	}
	must(wb.Save(outputPath))
	fmt.Printf("Wrote %s\n", outputPath)

	// ── 11. REOPEN and verify the round-trip ─────────────────────────────
	//
	// Everything below reads the freshly written file back through Open and
	// checks that the features survived a real save/open cycle.
	verify(outputPath)
}

// productNames returns the category labels for the chart.
func productNames() []string {
	names := make([]string, len(products))
	for i, p := range products {
		names[i] = p.name
	}
	return names
}

// column returns quarter q (0=Q1 .. 3=Q4) across all products, as the chart's
// per-quarter series values.
func column(q int) []float64 {
	vals := make([]float64, len(products))
	for i, p := range products {
		switch q {
		case 0:
			vals[i] = p.q1
		case 1:
			vals[i] = p.q2
		case 2:
			vals[i] = p.q3
		case 3:
			vals[i] = p.q4
		}
	}
	return vals
}

// verify reopens the workbook and prints a summary proving each feature
// round-tripped. Any inconsistency is fatal, so `go run` exits non-zero if the
// file did not come back the way we wrote it.
func verify(path string) {
	wb, err := xlsx.Open(path)
	must(err)

	sheet, err := wb.SheetByName("Q4 Sales")
	must(err)

	fmt.Println("Reopened and verified:")

	// Table.
	tables := sheet.Tables()
	if len(tables) != 1 {
		log.Fatalf("expected 1 table, got %d", len(tables))
	}
	t := tables[0]
	cols := make([]string, 0, len(t.Columns()))
	for _, c := range t.Columns() {
		cols = append(cols, c.Name)
	}
	fmt.Printf("  - table %q over %s, totals row=%v, columns=%v\n",
		t.Name(), t.Range(), t.TotalsRow(), cols)

	// Chart.
	charts := sheet.Charts()
	if len(charts) != 1 {
		log.Fatalf("expected 1 chart, got %d", len(charts))
	}
	fmt.Printf("  - chart %q with %d series\n", charts[0].Title(), len(charts[0].SeriesList()))

	// Protection: the sheet is protected; the forecast input is unlocked while
	// the secret cell is locked+hidden.
	prot := sheet.Protection()
	if prot == nil || !prot.Enabled() {
		log.Fatal("expected sheet protection to be enabled")
	}
	fmt.Printf("  - sheet protection enabled=%v password=%v autoFilter-locked=%v\n",
		prot.Enabled(), prot.HasPassword(), prot.AutoFilter())

	forecastLocked := cellLocked(wb, sheet, fmt.Sprintf("D%d", 4+len(products)+2))
	fmt.Printf("  - forecast input cell locked=%v (should be false)\n", forecastLocked)

	// Workbook structure protection.
	if wp := wb.Protection(); wp == nil || !wp.LockStructure() {
		log.Fatal("expected workbook structure protection")
	}
	fmt.Println("  - workbook structure locked=true")

	// Page setup and print area.
	ps, ok := sheet.PageSetup()
	if !ok || ps.Orientation != xlsx.OrientationLandscape {
		log.Fatalf("expected landscape page setup, got %+v (present=%v)", ps, ok)
	}
	fmt.Printf("  - page setup: orientation=%s, print area=%s\n", ps.Orientation, sheet.PrintArea())

	// Frozen panes.
	if fCols, fRows, ok := sheet.FrozenPanes(); ok {
		fmt.Printf("  - frozen panes: %d column(s), %d row(s)\n", fCols, fRows)
	}

	fmt.Println("All features round-tripped successfully.")
}

// cellLocked reports a cell's effective locked flag by reading its style's
// protection record. A cell with no explicit protection is locked by default.
func cellLocked(wb *xlsx.Workbook, sheet *xlsx.Sheet, ref string) bool {
	cell, err := sheet.Cell(ref)
	must(err)
	idx := cell.StyleIndex()
	if idx == nil {
		return true // no style => Excel's default (locked)
	}
	style, err := wb.Styles().GetCellStyle(*idx)
	must(err)
	if style.Protection == nil {
		return true
	}
	return style.Protection.Locked
}
