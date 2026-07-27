// Example: Create a formatted Excel spreadsheet
//
// This example demonstrates cell styling, freeze panes, auto-filter,
// data validation, named ranges, merged cells, column widths, and
// tab colors using the spine XLSX library.
//
// Run with: go run ./examples/create_spreadsheet
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/xlsx"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	wb := xlsx.Create()
	wb.Properties.Title = "Spine Demo Spreadsheet"
	wb.Properties.Creator = "Spine Library"

	// ── Sheet 1: Sales Data ──────────────────────────────────────────

	sales, err := wb.AddSheet("Sales Data")
	must(err)
	sales.SetTabColor("4472C4") // blue tab

	// Title row (merged)
	must(sales.SetCellValue("A1", "Quarterly Sales Report"))
	must(sales.MergeCells("A1", "G1"))
	titleCell, _ := sales.Cell("A1")
	must(titleCell.SetStyle(xlsx.CellStyle{
		Font:      &xlsx.FontStyle{Name: "Calibri", Size: 14, Bold: true, Color: "1F4E79"},
		Alignment: &xlsx.AlignmentStyle{Horizontal: "center"},
	}))
	must(sales.SetRowHeight(1, 30))

	// Header row
	headers := []string{"Product", "Q1", "Q2", "Q3", "Q4", "Total", "Status"}
	headerStyle := xlsx.CellStyle{
		Font:      &xlsx.FontStyle{Name: "Calibri", Size: 11, Bold: true, Color: "FFFFFF"},
		Fill:      &xlsx.FillStyle{Pattern: "solid", FgColor: "4472C4"},
		Alignment: &xlsx.AlignmentStyle{Horizontal: "center"},
		Border: &xlsx.BorderStyle{
			Bottom: &xlsx.BorderSide{Style: "thin", Color: "2F5496"},
		},
	}
	for i, h := range headers {
		ref, _ := xlsx.CellRef(2, i+1)
		must(sales.SetCellValue(ref, h))
		cell, _ := sales.Cell(ref)
		must(cell.SetStyle(headerStyle))
	}
	must(sales.SetRowHeight(2, 20))

	// Column widths
	must(sales.SetColWidth(1, 15)) // Product
	for i := 2; i <= 6; i++ {
		must(sales.SetColWidth(i, 12)) // Q1-Q4, Total
	}
	must(sales.SetColWidth(7, 12)) // Status

	// Data rows
	data := []struct {
		product            string
		q1, q2, q3, q4    float64
		status             string
	}{
		{"Widgets", 15200, 18400, 21100, 24300, "Active"},
		{"Gadgets", 32100, 29500, 31800, 35200, "Active"},
		{"Sprockets", 8050, 9500, 11200, 12500, "Pending"},
		{"Flanges", 21400, 23100, 20800, 26300, "Active"},
		{"Brackets", 5600, 6200, 7100, 8400, "Inactive"},
	}

	currencyStyle := xlsx.CellStyle{
		Format:    "#,##0",
		Alignment: &xlsx.AlignmentStyle{Horizontal: "right"},
	}

	for i, row := range data {
		r := i + 3 // data starts at row 3
		must(sales.SetCellValue(fmt.Sprintf("A%d", r), row.product))
		must(sales.SetCellValue(fmt.Sprintf("B%d", r), row.q1))
		must(sales.SetCellValue(fmt.Sprintf("C%d", r), row.q2))
		must(sales.SetCellValue(fmt.Sprintf("D%d", r), row.q3))
		must(sales.SetCellValue(fmt.Sprintf("E%d", r), row.q4))
		must(sales.SetCellValue(fmt.Sprintf("G%d", r), row.status))

		// Currency formatting on number cells
		for col := 2; col <= 5; col++ {
			ref, _ := xlsx.CellRef(r, col)
			cell, _ := sales.Cell(ref)
			must(cell.SetStyle(currencyStyle))
		}

		// Total formula
		cell, _ := sales.Cell(fmt.Sprintf("F%d", r))
		cell.SetFormula(fmt.Sprintf("SUM(B%d:E%d)", r, r))
		must(cell.SetStyle(xlsx.CellStyle{
			Format: "#,##0",
			Font:   &xlsx.FontStyle{Bold: true},
			Border: &xlsx.BorderStyle{
				Left: &xlsx.BorderSide{Style: "thin", Color: "4472C4"},
			},
			Alignment: &xlsx.AlignmentStyle{Horizontal: "right"},
		}))
	}

	// Grand total row
	totalRow := len(data) + 3
	must(sales.SetCellValue(fmt.Sprintf("A%d", totalRow), "Grand Total"))
	totalStyle := xlsx.CellStyle{
		Font:   &xlsx.FontStyle{Bold: true, Size: 11},
		Format: "#,##0",
		Fill:   &xlsx.FillStyle{Pattern: "solid", FgColor: "D6E4F0"},
		Border: &xlsx.BorderStyle{
			Top:    &xlsx.BorderSide{Style: "double", Color: "4472C4"},
			Bottom: &xlsx.BorderSide{Style: "double", Color: "4472C4"},
		},
		Alignment: &xlsx.AlignmentStyle{Horizontal: "right"},
	}
	for col := 2; col <= 6; col++ {
		ref, _ := xlsx.CellRef(totalRow, col)
		cell, _ := sales.Cell(ref)
		colLetter := string(rune('A' + col - 1))
		cell.SetFormula(fmt.Sprintf("SUM(%s3:%s%d)", colLetter, colLetter, totalRow-1))
		must(cell.SetStyle(totalStyle))
	}
	labelCell, _ := sales.Cell(fmt.Sprintf("A%d", totalRow))
	must(labelCell.SetStyle(xlsx.CellStyle{
		Font: &xlsx.FontStyle{Bold: true, Size: 11},
		Fill: &xlsx.FillStyle{Pattern: "solid", FgColor: "D6E4F0"},
		Border: &xlsx.BorderStyle{
			Top:    &xlsx.BorderSide{Style: "double", Color: "4472C4"},
			Bottom: &xlsx.BorderSide{Style: "double", Color: "4472C4"},
		},
	}))

	// Freeze panes: freeze title + header rows
	must(sales.FreezePanes("A3"))

	// Auto-filter on header row
	must(sales.SetAutoFilter(fmt.Sprintf("A2:G%d", totalRow-1)))

	// Data validation on Status column
	must(sales.AddDataValidation(xlsx.DataValidation{
		Range:        fmt.Sprintf("G3:G%d", totalRow-1),
		Type:         "list",
		Formula1:     `"Active,Inactive,Pending"`,
		AllowBlank:   true,
		ErrorTitle:   "Invalid Status",
		ErrorMessage: "Please select Active, Inactive, or Pending.",
	}))

	// Named range for grand total
	must(wb.AddDefinedName("GrandTotal",
		fmt.Sprintf("'Sales Data'!$F$%d", totalRow)))

	// ── Sheet 2: Summary ─────────────────────────────────────────────

	summary, err := wb.AddSheet("Summary")
	must(err)
	summary.SetTabColor("70AD47") // green tab
	summary.SetZoom(120)
	summary.SetShowGridLines(false)

	must(summary.SetCellValue("A1", "Summary"))
	cell, _ := summary.Cell("A1")
	must(cell.SetStyle(xlsx.CellStyle{
		Font: &xlsx.FontStyle{Size: 16, Bold: true, Color: "1F4E79"},
	}))

	must(summary.SetCellValue("A3", "Total Products"))
	must(summary.SetCellValue("B3", len(data)))
	must(summary.SetCellValue("A4", "Quarters"))
	must(summary.SetCellValue("B4", 4))
	must(summary.SetCellValue("A5", "Grand Total"))
	totalRef, _ := summary.Cell("B5")
	totalRef.SetFormula("GrandTotal")

	must(summary.SetColWidth(1, 18))
	must(summary.SetColWidth(2, 14))

	// Set active sheet to Sales Data
	must(wb.SetActiveSheet(0))

	// ── Save ─────────────────────────────────────────────────────────

	outputPath := "output.xlsx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}

	if err := wb.Save(outputPath); err != nil {
		log.Fatalf("Failed to save workbook: %v", err)
	}

	fmt.Printf("Spreadsheet saved to: %s\n", outputPath)
	fmt.Printf("Sheets: %d\n", wb.SheetCount())
	for _, s := range wb.Sheets() {
		fmt.Printf("  - %s (%d rows)\n", s.Name(), s.Rows())
	}
	fmt.Printf("Defined names: %d\n", len(wb.DefinedNames()))
}
