package xlsx_test

import (
	"bytes"
	"fmt"

	"github.com/mgilbir/spine/xlsx"
)

// ExampleNewStreamWriter writes a large sheet a row at a time, with memory
// bounded by one block of rows however many are written, then reopens the
// result to check it. Close completes the package; without it the output is
// a truncated archive.
func ExampleNewStreamWriter() {
	var buf bytes.Buffer
	w := xlsx.NewStreamWriter(&buf)
	sheet, err := w.AddSheet("Data")
	if err != nil {
		panic(err)
	}
	if _, err := sheet.AppendRow("id", "amount"); err != nil {
		panic(err)
	}
	for i := 1; i <= 50000; i++ {
		if _, err := sheet.AppendRow(i, float64(i)/4); err != nil {
			panic(err)
		}
	}
	if err := w.Close(); err != nil {
		panic(err)
	}

	wb, err := xlsx.OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		panic(err)
	}
	last, err := wb.Sheets()[0].Cell("B50001")
	if err != nil {
		panic(err)
	}
	fmt.Println("rows:", sheet.Rows())
	fmt.Println("B50001:", last.Value())
	// Output:
	// rows: 50001
	// B50001: 12500
}
