// Example: Create a simple Excel spreadsheet
//
// This example demonstrates how to use the spine library to create
// a basic Excel workbook with multiple sheets, different cell types,
// and formulas.
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
	// Create a new workbook
	wb := xlsx.Create()

	// Set document properties
	wb.Properties.Title = "Spine Demo Spreadsheet"
	wb.Properties.Creator = "Spine Library"
	wb.Properties.Subject = "Demonstration of the spine XLSX library"

	// --- Sheet 1: Sales Data ---
	sales := wb.AddSheet("Sales Data")

	// Headers
	headers := []string{"Product", "Q1", "Q2", "Q3", "Q4", "Total"}
	for i, h := range headers {
		ref, _ := xlsx.CellRef(1, i+1)
		must(sales.SetCellValue(ref, h))
	}

	// Data rows
	data := []struct {
		product string
		q1, q2, q3, q4 float64
	}{
		{"Widgets", 1500, 1800, 2100, 2400},
		{"Gadgets", 3200, 2900, 3100, 3500},
		{"Sprockets", 800, 950, 1100, 1250},
		{"Flanges", 2100, 2300, 2000, 2600},
	}

	for i, row := range data {
		r := i + 2 // data starts at row 2
		must(sales.SetCellValue(fmt.Sprintf("A%d", r), row.product))
		must(sales.SetCellValue(fmt.Sprintf("B%d", r), row.q1))
		must(sales.SetCellValue(fmt.Sprintf("C%d", r), row.q2))
		must(sales.SetCellValue(fmt.Sprintf("D%d", r), row.q3))
		must(sales.SetCellValue(fmt.Sprintf("E%d", r), row.q4))

		// Total formula for each row
		cell, _ := sales.Cell(fmt.Sprintf("F%d", r))
		cell.SetFormula(fmt.Sprintf("SUM(B%d:E%d)", r, r))
	}

	// --- Sheet 2: Summary ---
	summary := wb.AddSheet("Summary")

	must(summary.SetCellValue("A1", "Metric"))
	must(summary.SetCellValue("B1", "Value"))

	must(summary.SetCellValue("A2", "Total Products"))
	must(summary.SetCellValue("B2", len(data)))

	must(summary.SetCellValue("A3", "Quarters"))
	must(summary.SetCellValue("B3", 4))

	must(summary.SetCellValue("A4", "Report Generated"))
	must(summary.SetCellValue("B4", true))

	// Save the workbook
	outputPath := "output.xlsx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}

	err := wb.Save(outputPath)
	if err != nil {
		log.Fatalf("Failed to save workbook: %v", err)
	}

	fmt.Printf("Spreadsheet saved to: %s\n", outputPath)
	fmt.Printf("Sheets: %d\n", wb.SheetCount())
	for _, s := range wb.Sheets() {
		fmt.Printf("  - %s (%d rows)\n", s.Name(), s.Rows())
	}
}
