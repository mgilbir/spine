package xlsx_test

import (
	"bytes"
	"fmt"

	"github.com/mgilbir/spine/xlsx"
)

// ExampleCreate mirrors the README quick start for Excel: a sheet with cell
// values and a SUM formula, serialized with SaveBytes and reopened from memory
// to read a value and the sheet count back — no files touched.
func ExampleCreate() {
	wb := xlsx.Create()

	sheet := wb.AddSheet("Sales")
	cells := []struct {
		ref string
		val interface{}
	}{
		{"A1", "Product"}, {"B1", "Revenue"},
		{"A2", "Widgets"}, {"B2", 1500.0},
		{"A3", "Gadgets"}, {"B3", 3200.0},
	}
	for _, c := range cells {
		if err := sheet.SetCellValue(c.ref, c.val); err != nil {
			panic(err)
		}
	}

	cell, _ := sheet.Cell("B4")
	cell.SetFormula("SUM(B2:B3)")

	data, err := wb.SaveBytes()
	if err != nil {
		panic(err)
	}

	reopened, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic(err)
	}
	defer func() { _ = reopened.Close() }()

	s2, _ := reopened.SheetByName("Sales")
	a1, _ := s2.Cell("A1")
	b4, _ := s2.Cell("B4")
	fmt.Printf("sheets=%d A1=%v B4.formula=%s\n", reopened.SheetCount(), a1.Value(), b4.Formula())
	// Output: sheets=1 A1=Product B4.formula=SUM(B2:B3)
}
