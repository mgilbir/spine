package chart

import (
	"strconv"
	"strings"
)

// DataLayout describes where a chart's data lives in a worksheet. The c:f
// formula references in the emitted chart.xml and the cells written into the
// embedded workbook are both derived from it, so they line up: Office can open
// the workbook and edit the exact ranges the chart points at.
//
// Layout convention: categories (or scatter X) in column A, each series in a
// subsequent column with the series name in row 1 and its values in rows 2..N+1.
type DataLayout struct {
	// Sheet is the worksheet name (the reference base).
	Sheet string
	// CategoriesRef is the range holding category labels (empty for scatter).
	CategoriesRef string
	// Series holds one entry per series, in order.
	Series []SeriesLayout
}

// SeriesLayout describes the cell references for a single series.
type SeriesLayout struct {
	// NameRef is the single-cell reference holding the series name (row 1).
	NameRef string
	// XValuesRef is the range holding X values (scatter and bubble only; empty
	// otherwise).
	XValuesRef string
	// ValuesRef is the range holding the series values (Y for scatter/bubble).
	ValuesRef string
	// SizesRef is the range holding bubble sizes (bubble only; empty otherwise).
	SizesRef string
}

// pointCount returns the number of data points the chart carries: the number
// of categories, or (for scatter) the max series length.
func (c *Chart) pointCount() int {
	if c.kind == KindScatter || c.kind == KindBubble {
		n := 0
		for _, s := range c.series {
			if len(s.Values) > n {
				n = len(s.Values)
			}
			if len(s.XValues) > n {
				n = len(s.XValues)
			}
			if len(s.Sizes) > n {
				n = len(s.Sizes)
			}
		}
		return n
	}
	return len(c.categories)
}

// Layout returns the DataLayout describing where the chart's data lives: the
// cell ranges its c:f references point at. Format integrations use it to place
// the data (in a host sheet or an embedded workbook) to match the references.
func (c *Chart) Layout() DataLayout { return c.layout() }

// layout computes the DataLayout for the chart's current data.
func (c *Chart) layout() DataLayout {
	sheet := c.sheet()
	n := c.pointCount()
	dl := DataLayout{Sheet: sheet}

	if c.kind == KindBubble {
		return c.bubbleLayout(sheet, n)
	}

	scatter := c.kind == KindScatter
	// Categories / scatter-X occupy column A (col 1). Series start at column B.
	if !scatter && n > 0 {
		dl.CategoriesRef = rangeRef(sheet, 1, 2, 1, n+1)
	}
	for i := range c.series {
		col := i + 2 // B, C, ...
		sl := SeriesLayout{
			NameRef: cellRef(sheet, col, 1),
		}
		if n > 0 {
			sl.ValuesRef = rangeRef(sheet, col, 2, col, n+1)
			if scatter {
				sl.XValuesRef = rangeRef(sheet, 1, 2, 1, n+1)
			}
		}
		dl.Series = append(dl.Series, sl)
	}
	return dl
}

// bubbleLayout computes the DataLayout for a bubble chart. The shared X values
// occupy column A; each series takes two adjacent columns — its Y values (with
// the series name in row 1) and its sizes.
func (c *Chart) bubbleLayout(sheet string, n int) DataLayout {
	dl := DataLayout{Sheet: sheet}
	for i := range c.series {
		yCol := 2 + 2*i // B, D, F, ...
		sizeCol := yCol + 1
		sl := SeriesLayout{NameRef: cellRef(sheet, yCol, 1)}
		if n > 0 {
			sl.XValuesRef = rangeRef(sheet, 1, 2, 1, n+1)
			sl.ValuesRef = rangeRef(sheet, yCol, 2, yCol, n+1)
			sl.SizesRef = rangeRef(sheet, sizeCol, 2, sizeCol, n+1)
		}
		dl.Series = append(dl.Series, sl)
	}
	return dl
}

// colName converts a 1-based column index to its spreadsheet letters (1 -> A,
// 26 -> Z, 27 -> AA).
func colName(col int) string {
	if col < 1 {
		col = 1
	}
	var b []byte
	for col > 0 {
		col--
		b = append([]byte{byte('A' + col%26)}, b...)
		col /= 26
	}
	return string(b)
}

// quoteSheet wraps a sheet name in single quotes if a formula reference
// requires it (names with spaces or characters outside the safe set).
func quoteSheet(name string) string {
	safe := name != ""
	for _, r := range name {
		isSafe := r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' || r == '_' || r == '.'
		if !isSafe {
			safe = false
			break
		}
	}
	if r0 := []rune(name); len(r0) > 0 && r0[0] >= '0' && r0[0] <= '9' {
		safe = false
	}
	if safe {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// cellRef builds an absolute single-cell reference, e.g. Sheet1!$B$1.
func cellRef(sheet string, col, row int) string {
	return quoteSheet(sheet) + "!$" + colName(col) + "$" + strconv.Itoa(row)
}

// rangeRef builds an absolute range reference, e.g. Sheet1!$B$2:$B$5. A
// single-cell range collapses to one reference.
func rangeRef(sheet string, col1, row1, col2, row2 int) string {
	start := "$" + colName(col1) + "$" + strconv.Itoa(row1)
	if col1 == col2 && row1 == row2 {
		return quoteSheet(sheet) + "!" + start
	}
	end := "$" + colName(col2) + "$" + strconv.Itoa(row2)
	return quoteSheet(sheet) + "!" + start + ":" + end
}
