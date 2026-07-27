package chart

import (
	"strconv"

	"github.com/mgilbir/spine/internal/sheetref"
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

// pointCount returns the number of shared category labels the chart carries,
// which is what the categories range spans. Scatter and bubble charts have no
// category labels — their X coordinates are per series — and lay their data out
// through their own functions.
func (c *Chart) pointCount() int {
	return len(c.categories)
}

// seriesLen is the single source of truth for how many points series i carries:
// the count its numeric cache declares, the number of cells written for it into
// the data sheet, and the number of rows its c:f value reference covers.
//
// Sizing the reference from the category count instead (as layout once did)
// makes the three disagree whenever a series is longer or shorter than the
// category list — Excel then resolves the reference over the cache on refresh
// and silently drops the tail (C434). Deriving all three from the series itself
// keeps a ref, a cache and a column describing the same points; a series
// shorter than the categories simply covers fewer rows. The same rule sizes the
// other per-series ranges of scatter and bubble charts, which are built from
// the length of the coordinate list each one caches.
func (c *Chart) seriesLen(i int) int {
	if i < 0 || i >= len(c.series) {
		return 0
	}
	return len(c.series[i].Values)
}

// Layout returns the DataLayout describing where the chart's data lives: the
// cell ranges its c:f references point at. Format integrations use it to place
// the data (in a host sheet or an embedded workbook) to match the references.
func (c *Chart) Layout() DataLayout { return c.layout() }

// layout computes the DataLayout for the chart's current data.
func (c *Chart) layout() DataLayout {
	sheet := c.sheet()
	if c.kind == KindBubble {
		return c.bubbleLayout(sheet)
	}
	if c.kind == KindScatter {
		return c.scatterLayout(sheet)
	}

	dl := DataLayout{Sheet: sheet}
	n := c.pointCount()
	// Categories occupy column A (col 1). Series start at column B.
	if n > 0 {
		dl.CategoriesRef = rangeRef(sheet, 1, 2, 1, n+1)
	}
	for i := range c.series {
		col := i + 2 // B, C, ...
		sl := SeriesLayout{
			NameRef:   cellRef(sheet, col, 1),
			ValuesRef: columnRef(sheet, col, c.seriesLen(i)),
		}
		dl.Series = append(dl.Series, sl)
	}
	return dl
}

// columnRef returns the reference covering a data column's n value rows (rows
// 2..n+1, row 1 being the series name), or "" when there are no values. It is
// how every series range in a layout is built, so a range always spans exactly
// the points its cache declares (C434).
func columnRef(sheet string, col, n int) string {
	if n <= 0 {
		return ""
	}
	return rangeRef(sheet, col, 2, col, n+1)
}

// scatterLayout computes the DataLayout for a scatter chart. Each series takes
// two adjacent columns — its own X values and its Y values (with the series name
// in row 1) — so every series' c:xVal references a distinct column that holds
// that series' X, rather than every series sharing column A (C251). The columns
// are A/B, C/D, E/F, ... for series 0, 1, 2, ...
func (c *Chart) scatterLayout(sheet string) DataLayout {
	dl := DataLayout{Sheet: sheet}
	for i, s := range c.series {
		xCol := 1 + 2*i // A, C, E, ...
		yCol := xCol + 1
		dl.Series = append(dl.Series, SeriesLayout{
			NameRef:    cellRef(sheet, yCol, 1),
			XValuesRef: columnRef(sheet, xCol, len(s.XValues)),
			ValuesRef:  columnRef(sheet, yCol, c.seriesLen(i)),
		})
	}
	return dl
}

// bubbleLayout computes the DataLayout for a bubble chart. The shared X values
// occupy column A; each series takes two adjacent columns — its Y values (with
// the series name in row 1) and its sizes.
func (c *Chart) bubbleLayout(sheet string) DataLayout {
	dl := DataLayout{Sheet: sheet}
	for i, s := range c.series {
		yCol := 2 + 2*i // B, D, F, ...
		sizeCol := yCol + 1
		dl.Series = append(dl.Series, SeriesLayout{
			NameRef:    cellRef(sheet, yCol, 1),
			XValuesRef: columnRef(sheet, 1, len(s.XValues)),
			ValuesRef:  columnRef(sheet, yCol, c.seriesLen(i)),
			SizesRef:   columnRef(sheet, sizeCol, len(s.Sizes)),
		})
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
// requires it: names with spaces or characters outside the safe set, names
// starting with a digit, and — the ambiguity the character check alone misses
// (C558) — names that lex as a cell reference ("A1", "R1C1", a bare "R") or as
// a boolean literal. Excel's formula grammar reads `A1!$B$1` as a reference to
// a cell, not to a sheet called A1; `'A1'!$B$1` is the unambiguous form.
func quoteSheet(name string) string {
	return sheetref.QuoteName(name)
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
