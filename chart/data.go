package chart

import (
	"fmt"
	"strconv"

	"github.com/mgilbir/spine/xlsx"
)

// EmbeddedWorkbook builds a minimal .xlsx workbook (as bytes) holding the
// chart's data laid out to match its c:f references, and returns it together
// with the DataLayout describing the cell ranges. This is what docx and pptx
// charts embed so Office can edit the data; xlsx charts reference the host
// sheet directly and do not need it.
//
// The layout places categories (or scatter X) in column A and each series in a
// subsequent column, with the series name in row 1 and its values in rows
// 2..N+1 — the same convention MarshalChartXML builds its references from, so
// the returned layout's references equal those in the emitted chart.xml.
func (c *Chart) EmbeddedWorkbook() ([]byte, DataLayout, error) {
	dl := c.layout()
	wb := xlsx.Create()
	sheet := wb.AddSheet(c.sheet())

	scatter := c.kind == KindScatter
	n := c.pointCount()

	// Column A: category labels, or scatter X values (from the first series).
	if !scatter {
		for i, label := range c.categories {
			if err := sheet.SetCellValue(cellA1(1, i+2), label); err != nil {
				return nil, dl, fmt.Errorf("chart: write category: %w", err)
			}
		}
	} else if len(c.series) > 0 {
		for i, x := range c.series[0].XValues {
			if err := sheet.SetCellValue(cellA1(1, i+2), x); err != nil {
				return nil, dl, fmt.Errorf("chart: write scatter x: %w", err)
			}
		}
	}

	// Series columns: name in row 1, values in rows 2..N+1.
	for si, s := range c.series {
		col := si + 2
		if err := sheet.SetCellValue(cellA1(col, 1), s.Name); err != nil {
			return nil, dl, fmt.Errorf("chart: write series name: %w", err)
		}
		for i := 0; i < n && i < len(s.Values); i++ {
			if err := sheet.SetCellValue(cellA1(col, i+2), s.Values[i]); err != nil {
				return nil, dl, fmt.Errorf("chart: write series value: %w", err)
			}
		}
	}

	data, err := wb.SaveBytes()
	if err != nil {
		return nil, dl, fmt.Errorf("chart: save embedded workbook: %w", err)
	}
	return data, dl, nil
}

// cellA1 builds a plain (unquoted, relative) A1 reference for the xlsx API,
// e.g. col=2,row=1 -> "B1".
func cellA1(col, row int) string {
	return colName(col) + strconv.Itoa(row)
}
