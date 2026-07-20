package xlsx

import (
	"errors"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// ErrNilWorkbook is returned when a copy operation is given a nil source
// workbook.
var ErrNilWorkbook = errors.New("xlsx: source workbook is nil")

// CopySheetFrom copies the sheet named sheetName from other into this workbook
// under a unique name (a suffix is appended if the name is already taken),
// returning the new sheet. Cell values, styles, formulas, merged ranges, and
// column widths / row heights are carried over. Shared-string cell values are
// resolved and written as inline strings so the two workbooks' string tables
// need not be merged, and cell style indices are remapped into this workbook's
// stylesheet (deduplicated).
//
// Images and charts embedded in the source sheet are not copied (deferred), and
// cross-sheet references in copied formulas are not rewritten.
func (w *Workbook) CopySheetFrom(other *Workbook, sheetName string) (*Sheet, error) {
	if other == nil {
		return nil, ErrNilWorkbook
	}
	if other == w {
		return nil, errors.New("xlsx: cannot copy a sheet from a workbook into itself")
	}

	src, err := other.SheetByName(sheetName)
	if err != nil {
		return nil, err
	}
	if src.worksheet == nil {
		return nil, ErrSheetNotFound
	}

	dst := w.AddSheet(sheetName)

	// styleCache maps a source cellXfs index to the index it was assigned in
	// this workbook's stylesheet, so identical styles are registered once.
	styleCache := make(map[uint32]uint32)

	for i := range src.worksheet.SheetData.Row {
		row := &src.worksheet.SheetData.Row[i]
		for _, sc := range row.C {
			if sc == nil || sc.R == "" {
				continue
			}
			dc, err := dst.Cell(sc.R)
			if err != nil {
				return nil, err
			}
			if err := copyCellValue(w, other, sc, dc); err != nil {
				return nil, err
			}
			if sc.S != nil {
				newIdx, err := remapStyleIndex(w, other, *sc.S, styleCache)
				if err != nil {
					return nil, err
				}
				dc.SetStyleIndex(newIdx)
			}
		}
	}

	copyMerges(src, dst)
	copyColumnWidths(src, dst)
	copyRowHeights(src, dst)

	return dst, nil
}

// copyCellValue transfers the value of source cell sc into destination cell dc,
// resolving shared strings and preserving formulas and self-contained values.
func copyCellValue(dstWB, srcWB *Workbook, sc *oxml.CT_Cell, dc *Cell) error {
	switch {
	case sc.F != nil:
		// Formula: copy the formula element and its cached value/type verbatim.
		f := *sc.F
		dc.cell.F = &f
		dc.cell.T = sc.T
		dc.cell.V = cloneStringPtr(sc.V)
		dc.cell.Is = cloneRst(sc.Is)
	case sc.T == "s":
		// Shared string: resolve against the source table and store inline so
		// the destination does not depend on the source's string table.
		if sc.V != nil {
			idx, err := strconv.Atoi(strings.TrimSpace(*sc.V))
			if err != nil {
				return nil
			}
			dc.SetString(srcWB.resolveSharedString(idx))
		}
	default:
		// Numbers, booleans, errors, inline and cached strings are
		// self-contained; copy the type, value, and any inline string verbatim.
		dc.cell.T = sc.T
		dc.cell.V = cloneStringPtr(sc.V)
		dc.cell.Is = cloneRst(sc.Is)
	}
	dc.markSheetDirty()
	return nil
}

// remapStyleIndex returns the index in dst's stylesheet equivalent to srcIdx in
// src's stylesheet, registering (and deduplicating) the style on first use.
func remapStyleIndex(dst, src *Workbook, srcIdx uint32, cache map[uint32]uint32) (uint32, error) {
	if mapped, ok := cache[srcIdx]; ok {
		return mapped, nil
	}
	style, err := src.Styles().GetCellStyle(srcIdx)
	if err != nil {
		return 0, err
	}
	newIdx, err := dst.Styles().NewCellStyle(style)
	if err != nil {
		return 0, err
	}
	cache[srcIdx] = newIdx
	return newIdx, nil
}

// copyMerges copies the merged ranges of src into dst.
func copyMerges(src, dst *Sheet) {
	if src.worksheet.MergeCells == nil {
		return
	}
	for _, mc := range src.worksheet.MergeCells.MergeCell {
		start, end, ok := strings.Cut(mc.Ref, ":")
		if !ok {
			continue
		}
		// Best-effort: a fresh sheet has no overlapping ranges, but ignore any
		// individual failure rather than abort the whole copy.
		_ = dst.MergeCells(start, end)
	}
}

// copyColumnWidths copies custom column widths from src to dst.
func copyColumnWidths(src, dst *Sheet) {
	for _, cols := range src.worksheet.Cols {
		for _, col := range cols.Col {
			if col.Width == nil {
				continue
			}
			for c := col.Min; c <= col.Max && c <= uint32(MaxCol); c++ {
				_ = dst.SetColWidth(int(c), *col.Width)
			}
		}
	}
}

// copyRowHeights copies custom row heights from src to dst.
func copyRowHeights(src, dst *Sheet) {
	for i := range src.worksheet.SheetData.Row {
		row := &src.worksheet.SheetData.Row[i]
		if row.Ht == nil {
			continue
		}
		if num, ok := rowNumberOf(row); ok {
			_ = dst.SetRowHeight(int(num), *row.Ht)
		}
	}
}

// cloneStringPtr returns a copy of a *string, or nil.
func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

// cloneRst returns a shallow copy of a *CT_Rst, or nil. The copy is safe to
// attach to another workbook because callers never mutate the shared run/text
// slices after the copy.
func cloneRst(r *oxml.CT_Rst) *oxml.CT_Rst {
	if r == nil {
		return nil
	}
	c := *r
	return &c
}
