package xlsx

import (
	"fmt"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Filter comparison operators (CT_CustomFilter operator, ST_FilterOperator).
// An empty operator means Equal.
const (
	FilterEqual              = "equal"
	FilterNotEqual           = "notEqual"
	FilterGreaterThan        = "greaterThan"
	FilterGreaterThanOrEqual = "greaterThanOrEqual"
	FilterLessThan           = "lessThan"
	FilterLessThanOrEqual    = "lessThanOrEqual"
)

// FilterColumn describes the filter predicate applied to one column of a
// sheet's auto-filter. A column carries either a value-list filter (Values,
// optionally with Blank) or a custom comparison filter (Custom); when both are
// set the value list wins.
type FilterColumn struct {
	// ColID is the zero-based column offset within the auto-filter range.
	ColID uint32
	// Values, when non-empty, is a value-list filter: a row passes when the
	// column's displayed value matches one of these strings.
	Values []string
	// Blank, when true, also lets blank cells through the value-list filter.
	Blank bool
	// Custom, when non-empty, holds one or two custom comparison criteria.
	Custom []CustomFilter
	// CustomAnd combines two custom criteria with AND; the default (false) is OR.
	CustomAnd bool
	// HiddenButton hides the filter dropdown button on the column.
	HiddenButton bool
	// ShowButton, when explicitly set to false, hides the dropdown button.
	// A nil value leaves the attribute unset (button shown).
	ShowButton *bool
}

// CustomFilter is one criterion of a custom (comparison) auto-filter, e.g.
// {Operator: FilterGreaterThan, Value: "100"}.
type CustomFilter struct {
	Operator string // one of the Filter* operator constants; empty means Equal
	Value    string
}

// SetFilterColumn sets (or replaces) the filter predicate for a single column
// of the sheet's auto-filter. An auto-filter range must already be set with
// SetAutoFilter.
//
// The predicate is recorded but not applied: this package does not evaluate it
// against the sheet's data and does not hide the rows it excludes. Excel
// persists a filtered-out row as <row hidden="1">, and writes no such flags
// here, so the saved workbook opens showing every row with the filter dropdown
// indicating a predicate is set. Re-applying the filter in Excel (Data >
// Reapply) hides the rows. Set the row-hidden flags with SetRowHidden if the
// file must open already filtered.
func (s *Sheet) SetFilterColumn(fc FilterColumn) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	s.ensureWorksheet()
	if s.ws().AutoFilter == nil {
		return fmt.Errorf("xlsx: no auto-filter set; call SetAutoFilter first")
	}
	s.markDirty()

	col := oxml.CT_FilterColumn{ColId: fc.ColID}
	if fc.HiddenButton {
		hb := true
		col.HiddenButton = &hb
	}
	if fc.ShowButton != nil {
		sb := *fc.ShowButton
		col.ShowButton = &sb
	}
	switch {
	case len(fc.Values) > 0 || fc.Blank:
		filters := &oxml.CT_Filters{}
		if fc.Blank {
			bl := true
			filters.Blank = &bl
		}
		for _, v := range fc.Values {
			filters.Filter = append(filters.Filter, oxml.CT_Filter{Val: v})
		}
		col.Filters = filters
	case len(fc.Custom) > 0:
		cf := &oxml.CT_CustomFilters{}
		if fc.CustomAnd {
			and := true
			cf.And = &and
		}
		for _, c := range fc.Custom {
			cf.CustomFilter = append(cf.CustomFilter, oxml.CT_CustomFilter{
				Operator: c.Operator,
				Val:      c.Value,
			})
		}
		col.CustomFilters = cf
	}

	af := s.ws().AutoFilter
	for i := range af.FilterColumn {
		if af.FilterColumn[i].ColId == fc.ColID {
			af.FilterColumn[i] = col
			return nil
		}
	}
	af.FilterColumn = append(af.FilterColumn, col)
	return nil
}

// FilterColumns returns the per-column filter predicates of the sheet's
// auto-filter, in document order. It returns nil when no auto-filter or no
// column filters are set.
func (s *Sheet) FilterColumns() []FilterColumn {
	if s.ws() == nil || s.ws().AutoFilter == nil {
		return nil
	}
	cols := s.ws().AutoFilter.FilterColumn
	if len(cols) == 0 {
		return nil
	}
	result := make([]FilterColumn, 0, len(cols))
	for i := range cols {
		result = append(result, oxmlToFilterColumn(&cols[i]))
	}
	return result
}

// ClearFilterColumns removes all per-column filter predicates while leaving the
// auto-filter range in place.
func (s *Sheet) ClearFilterColumns() {
	// An opaque sheet is never regenerated from its worksheet model, so
	// mutating that model here would change nothing at save (C423).
	if s.opaque || s.ws() == nil || s.ws().AutoFilter == nil {
		return
	}
	if len(s.ws().AutoFilter.FilterColumn) == 0 {
		return
	}
	s.markDirty()
	s.ws().AutoFilter.FilterColumn = nil
}

func oxmlToFilterColumn(c *oxml.CT_FilterColumn) FilterColumn {
	fc := FilterColumn{ColID: c.ColId}
	if c.HiddenButton != nil && *c.HiddenButton {
		fc.HiddenButton = true
	}
	if c.ShowButton != nil {
		sb := *c.ShowButton
		fc.ShowButton = &sb
	}
	if c.Filters != nil {
		if c.Filters.Blank != nil && *c.Filters.Blank {
			fc.Blank = true
		}
		for _, f := range c.Filters.Filter {
			fc.Values = append(fc.Values, f.Val)
		}
	}
	if c.CustomFilters != nil {
		if c.CustomFilters.And != nil && *c.CustomFilters.And {
			fc.CustomAnd = true
		}
		for _, cust := range c.CustomFilters.CustomFilter {
			fc.Custom = append(fc.Custom, CustomFilter{
				Operator: cust.Operator,
				Value:    cust.Val,
			})
		}
	}
	return fc
}

// Sort-by kinds (CT_SortCondition sortBy, ST_SortBy). An empty value means Value.
const (
	SortByValue     = "value"
	SortByCellColor = "cellColor"
	SortByFontColor = "fontColor"
	SortByIcon      = "icon"
)

// SortState describes the sort applied to a range or auto-filter
// (CT_SortState).
type SortState struct {
	// Ref is the sorted range (e.g. "A2:D100").
	Ref string
	// CaseSensitive sorts case-sensitively.
	CaseSensitive bool
	// ColumnSort sorts left-to-right (by columns) instead of top-to-bottom.
	ColumnSort bool
	// SortMethod selects a stroke/pinYin method for East-Asian sorts
	// (ST_SortMethod: "stroke", "pinYin", "none").
	SortMethod string
	// Conditions lists the sort conditions in priority order.
	Conditions []SortCondition
}

// SortCondition is a single sort key (CT_SortCondition).
type SortCondition struct {
	// Ref is the range the key sorts on (e.g. "B2:B100").
	Ref string
	// Descending sorts high-to-low.
	Descending bool
	// SortBy selects what to sort on (one of the SortBy* constants; empty means
	// value).
	SortBy string
	// CustomList is a comma-separated custom sort order.
	CustomList string
}

// SetSortState writes the sheet's sort state. It updates the element the sheet
// already carries — the worksheet-level <sortState>, or the one nested in
// <autoFilter> when that is where the sort state lives — and creates a
// worksheet-level element only when the sheet has neither. The Ref must be set.
//
// Only the modeled fields are overwritten; the target element's unmodeled
// children (the x14 sort-by-color conditions in its extLst) are left alone.
// Rebuilding the element from scratch dropped them on every call.
func (s *Sheet) SetSortState(ss SortState) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	if ss.Ref == "" {
		return fmt.Errorf("xlsx: sort state requires a Ref range")
	}
	s.ensureWorksheet()
	s.markDirty()

	// Patch in place so CapturedChildren (the unmodeled extLst) survives.
	out := s.ws().SortState
	switch {
	case out != nil:
	case s.ws().AutoFilter != nil && s.ws().AutoFilter.SortState != nil:
		out = s.ws().AutoFilter.SortState
	default:
		out = &oxml.CT_SortState{}
		s.ws().SortState = out
		s.ws().EnsureChildOrder("sortState")
	}

	out.Ref = ss.Ref
	out.SortMethod = ss.SortMethod
	out.CaseSensitive = nil
	if ss.CaseSensitive {
		cs := true
		out.CaseSensitive = &cs
	}
	out.ColumnSort = nil
	if ss.ColumnSort {
		col := true
		out.ColumnSort = &col
	}
	out.SortCondition = nil
	for _, c := range ss.Conditions {
		cond := oxml.CT_SortCondition{Ref: c.Ref, SortBy: c.SortBy, CustomList: c.CustomList}
		if c.Descending {
			d := true
			cond.Descending = &d
		}
		out.SortCondition = append(out.SortCondition, cond)
	}
	return nil
}

// SortState returns the sheet's sort state. It reads a worksheet-level
// sortState element when present, otherwise the sortState nested in the
// auto-filter. The second result reports whether a sort state exists.
func (s *Sheet) SortState() (SortState, bool) {
	if s.ws() == nil {
		return SortState{}, false
	}
	src := s.ws().SortState
	if src == nil && s.ws().AutoFilter != nil {
		src = s.ws().AutoFilter.SortState
	}
	if src == nil {
		return SortState{}, false
	}
	return oxmlToSortState(src), true
}

// RemoveSortState removes the sheet's sort state, both the worksheet-level
// <sortState> element and the one nested in <autoFilter>. It removes exactly
// what SortState reads: removing only the worksheet-level element left
// SortState still reporting ok on the files — common Excel output — whose sort
// state lives inside <autoFilter> (C536). The auto-filter range itself is kept;
// use RemoveAutoFilter to drop that.
func (s *Sheet) RemoveSortState() {
	if s.opaque || s.ws() == nil {
		return
	}
	nested := s.ws().AutoFilter != nil && s.ws().AutoFilter.SortState != nil
	if s.ws().SortState == nil && !nested {
		return
	}
	s.markDirty()
	s.ws().SortState = nil
	if nested {
		s.ws().AutoFilter.SortState = nil
	}
}

func oxmlToSortState(src *oxml.CT_SortState) SortState {
	ss := SortState{
		Ref:           src.Ref,
		SortMethod:    src.SortMethod,
		CaseSensitive: src.CaseSensitive != nil && *src.CaseSensitive,
		ColumnSort:    src.ColumnSort != nil && *src.ColumnSort,
	}
	for i := range src.SortCondition {
		c := &src.SortCondition[i]
		ss.Conditions = append(ss.Conditions, SortCondition{
			Ref:        c.Ref,
			Descending: c.Descending != nil && *c.Descending,
			SortBy:     c.SortBy,
			CustomList: c.CustomList,
		})
	}
	return ss
}
