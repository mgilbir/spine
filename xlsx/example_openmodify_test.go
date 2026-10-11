package xlsx_test

import (
	"bytes"
	"fmt"

	"github.com/mgilbir/spine/xlsx"
)

// Example_openAndModify shows the open-edit-save cycle entirely in memory: an
// existing workbook (here built and serialized first) is opened with
// OpenReader, a cell is updated and a row added, and the result is saved and
// reopened to read the values back.
func Example_openAndModify() {
	src := xlsx.Create()
	sheet, err := src.AddSheet("Stock")
	if err != nil {
		panic(err)
	}
	for ref, v := range map[string]any{"A1": "Item", "B1": "Qty", "A2": "Bolts", "B2": 40} {
		if err := sheet.SetCellValue(ref, v); err != nil {
			panic(err)
		}
	}
	original, err := src.SaveBytes()
	if err != nil {
		panic(err)
	}

	wb, err := xlsx.OpenReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		panic(err)
	}
	stock, err := wb.SheetByName("Stock")
	if err != nil {
		panic(err)
	}
	for ref, v := range map[string]any{"B2": 35, "A3": "Nuts", "B3": 120} {
		if err := stock.SetCellValue(ref, v); err != nil {
			panic(err)
		}
	}

	edited, err := wb.SaveBytes()
	if err != nil {
		panic(err)
	}

	reopened, err := xlsx.OpenReader(bytes.NewReader(edited), int64(len(edited)))
	if err != nil {
		panic(err)
	}
	s, err := reopened.SheetByName("Stock")
	if err != nil {
		panic(err)
	}
	for _, row := range []string{"2", "3"} {
		item, _ := s.CellValue("A" + row)
		qty, _ := s.CellValue("B" + row)
		fmt.Println(item, qty)
	}
	// Output:
	// Bolts 35
	// Nuts 120
}
