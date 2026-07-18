package xlsx

import (
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// MergedCells returns the merged-range references on the sheet (e.g.
// []string{"A1:B2", "D4:D8"}), in document order. It is the read counterpart of
// MergeCells; the returned slice is nil when the sheet has no merged ranges.
func (s *Sheet) MergedCells() []string {
	if s.worksheet == nil || s.worksheet.MergeCells == nil {
		return nil
	}
	merges := s.worksheet.MergeCells.MergeCell
	if len(merges) == 0 {
		return nil
	}
	out := make([]string, 0, len(merges))
	for _, mc := range merges {
		out = append(out, mc.Ref)
	}
	return out
}

// FrozenPanes reports the sheet's frozen-pane split: cols is the number of
// frozen (always-visible) leading columns, rows the number of frozen leading
// rows, and ok is true when the sheet has a frozen pane. It reads the
// sheetView pane element written by FreezePanes; ok is false for a sheet with
// no pane or with a non-frozen (scrolling split) pane, in which case cols and
// rows are zero. This is the read counterpart of FreezePanes.
func (s *Sheet) FrozenPanes() (cols, rows int, ok bool) {
	pane := s.pane()
	if pane == nil {
		return 0, 0, false
	}
	// A frozen pane is state "frozen" or "frozenSplit"; a plain "split" pane is
	// a scrolling (not frozen) split and reports no frozen rows/columns.
	if pane.State != "frozen" && pane.State != "frozenSplit" {
		return 0, 0, false
	}
	if pane.XSplit != nil {
		cols = int(*pane.XSplit)
	}
	if pane.YSplit != nil {
		rows = int(*pane.YSplit)
	}
	return cols, rows, true
}

// pane returns the first sheetView's pane element, or nil if none.
func (s *Sheet) pane() *oxml.CT_Pane {
	if s.worksheet == nil || s.worksheet.SheetViews == nil {
		return nil
	}
	if len(s.worksheet.SheetViews.SheetView) == 0 {
		return nil
	}
	return s.worksheet.SheetViews.SheetView[0].Pane
}

// AutoFilterRange returns the sheet's auto-filter range reference (e.g.
// "A1:F1") and whether an auto-filter is set. It is the read counterpart of
// SetAutoFilter.
func (s *Sheet) AutoFilterRange() (string, bool) {
	if s.worksheet == nil || s.worksheet.AutoFilter == nil {
		return "", false
	}
	return s.worksheet.AutoFilter.Ref, true
}

// ColumnWidth returns the configured width of a column (1-based) and whether a
// width is set for it. The width is in Excel column-width units (character
// widths of the default font), matching SetColWidth. When the column has no
// explicit width the sheet default applies and ok is false.
func (s *Sheet) ColumnWidth(col int) (width float64, ok bool) {
	c := s.colEntry(col)
	if c == nil || c.Width == nil {
		return 0, false
	}
	return *c.Width, true
}

// ColumnHidden reports whether a column (1-based) is hidden.
func (s *Sheet) ColumnHidden(col int) bool {
	c := s.colEntry(col)
	return c != nil && c.Hidden != nil && *c.Hidden
}

// colEntry returns the col definition covering the given 1-based column, or nil.
func (s *Sheet) colEntry(col int) *oxml.CT_Col {
	if s.worksheet == nil || col < 1 {
		return nil
	}
	c := uint32(col)
	for i := range s.worksheet.Cols {
		for j := range s.worksheet.Cols[i].Col {
			entry := &s.worksheet.Cols[i].Col[j]
			if entry.Min <= c && c <= entry.Max {
				return entry
			}
		}
	}
	return nil
}

// RowHeight returns the configured height of a row (1-based) and whether a
// height is set for it. The height is in points, matching SetRowHeight. When
// the row has no explicit height the sheet default applies and ok is false.
func (s *Sheet) RowHeight(row int) (height float64, ok bool) {
	r := s.rowEntry(row)
	if r == nil || r.Ht == nil {
		return 0, false
	}
	return *r.Ht, true
}

// RowHidden reports whether a row (1-based) is hidden.
func (s *Sheet) RowHidden(row int) bool {
	r := s.rowEntry(row)
	return r != nil && r.Hidden != nil && *r.Hidden
}

// rowEntry returns the row definition for the given 1-based row number, or nil.
func (s *Sheet) rowEntry(row int) *oxml.CT_Row {
	if s.worksheet == nil || row < 1 {
		return nil
	}
	want := uint32(row)
	for i := range s.worksheet.SheetData.Row {
		if rn, ok := rowNumberOf(&s.worksheet.SheetData.Row[i]); ok && rn == want {
			return &s.worksheet.SheetData.Row[i]
		}
	}
	return nil
}

// DataValidations returns the data-validation rules defined on the sheet, in
// document order. It is the read counterpart of AddDataValidation; the returned
// slice is nil when the sheet has none.
func (s *Sheet) DataValidations() []*DataValidation {
	if s.worksheet == nil || s.worksheet.DataValidations == nil {
		return nil
	}
	dvs := s.worksheet.DataValidations.DataValidation
	if len(dvs) == 0 {
		return nil
	}
	out := make([]*DataValidation, 0, len(dvs))
	for i := range dvs {
		out = append(out, dataValidationFromModel(&dvs[i]))
	}
	return out
}

// DataValidation returns the validation rule whose range covers this cell, or
// nil if the cell has none. When several rules cover the cell (Excel allows
// this on hand-edited files) the first in document order is returned.
func (c *Cell) DataValidation() *DataValidation {
	if c.sheet == nil || c.sheet.worksheet == nil || c.sheet.worksheet.DataValidations == nil {
		return nil
	}
	for i := range c.sheet.worksheet.DataValidations.DataValidation {
		dv := &c.sheet.worksheet.DataValidations.DataValidation[i]
		if sqrefContains(dv.Sqref, c.cell.R) {
			return dataValidationFromModel(dv)
		}
	}
	return nil
}

// dataValidationFromModel converts an internal CT_DataValidation into the public
// DataValidation read/write struct. It inverts the write-side conventions:
// showDropDown="1" (suppress dropdown) maps back to HideDropDown.
func dataValidationFromModel(v *oxml.CT_DataValidation) *DataValidation {
	dv := &DataValidation{
		Range:         v.Sqref,
		Type:          v.Type,
		Operator:      v.Operator,
		AllowBlank:    v.AllowBlank != nil && *v.AllowBlank,
		HideDropDown:  v.ShowDropDown != nil && *v.ShowDropDown,
		ErrorTitle:    v.ErrorTitle,
		ErrorMessage:  v.Error,
		PromptTitle:   v.PromptTitle,
		PromptMessage: v.Prompt,
	}
	if v.Formula1 != nil {
		dv.Formula1 = *v.Formula1
	}
	if v.Formula2 != nil {
		dv.Formula2 = *v.Formula2
	}
	return dv
}

// sqrefContains reports whether a whitespace-separated list of range references
// (an OOXML sqref, e.g. "A1:B2 D4") covers the given single-cell reference.
func sqrefContains(sqref, cellRef string) bool {
	row, col, err := ParseCellRef(cellRef)
	if err != nil {
		return false
	}
	for _, ref := range strings.Fields(sqref) {
		rng, err := parseCellRangeRef(ref)
		if err != nil {
			continue
		}
		if row >= rng.minRow && row <= rng.maxRow && col >= rng.minCol && col <= rng.maxCol {
			return true
		}
	}
	return false
}
