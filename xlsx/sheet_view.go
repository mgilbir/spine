package xlsx

import (
	"fmt"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// ---------------------------------------------------------------------------
// Sheet visibility
// ---------------------------------------------------------------------------

// SheetVisibility is a worksheet's visibility state, stored on the workbook's
// <sheet state> attribute. A hidden sheet can be unhidden through Excel's UI; a
// very-hidden sheet can only be revealed programmatically (or via VBA).
type SheetVisibility string

const (
	// SheetVisible is the default: the sheet tab is shown.
	SheetVisible SheetVisibility = ""
	// SheetHidden hides the sheet; the user can unhide it from the UI.
	SheetHidden SheetVisibility = "hidden"
	// SheetVeryHidden hides the sheet from the unhide UI entirely.
	SheetVeryHidden SheetVisibility = "veryHidden"
)

// Visibility returns the sheet's visibility state.
func (s *Sheet) Visibility() SheetVisibility {
	return SheetVisibility(s.state)
}

// Visible reports whether the sheet is shown (neither hidden nor very hidden).
func (s *Sheet) Visible() bool {
	return s.state == ""
}

// SetVisibility sets the sheet's visibility. Hiding is refused when the sheet
// is the workbook's last visible one — Excel requires at least one visible
// sheet and rejects a workbook without one. The change is applied to the
// workbook model directly (workbook.xml is always regenerated on save), so it
// takes effect without dirtying the worksheet part.
func (s *Sheet) SetVisibility(v SheetVisibility) error {
	switch v {
	case SheetVisible, SheetHidden, SheetVeryHidden:
	default:
		return fmt.Errorf("xlsx: invalid sheet visibility %q", v)
	}

	// Refuse to hide the last visible sheet.
	if v != SheetVisible && s.state == "" && s.workbook != nil {
		visible := 0
		for _, other := range s.workbook.sheets {
			if other.state == "" {
				visible++
			}
		}
		if visible <= 1 {
			return fmt.Errorf("xlsx: cannot hide the last visible sheet")
		}
	}

	s.state = string(v)
	if s.workbook != nil && s.index >= 0 && s.index < len(s.workbook.workbook.Sheets.Sheet) {
		s.workbook.workbook.Sheets.Sheet[s.index].SetState(string(v))
	}
	// Workbook-level state: persists from the always-regenerated workbook.xml
	// without a sheet flag, so the content edit is recorded here instead.
	s.workbook.markContentEdited()
	return nil
}

// SetVisible shows (visible=true) or hides (visible=false) the sheet. It is a
// convenience wrapper over SetVisibility using SheetHidden for the hidden
// state; use SetVisibility directly for veryHidden.
func (s *Sheet) SetVisible(visible bool) error {
	if visible {
		return s.SetVisibility(SheetVisible)
	}
	return s.SetVisibility(SheetHidden)
}

// ---------------------------------------------------------------------------
// Row & column hidden write
// ---------------------------------------------------------------------------

// SetRowHidden hides (hidden=true) or shows (hidden=false) a row (1-based). It
// is the write counterpart of RowHidden.
func (s *Sheet) SetRowHidden(row int, hidden bool) error {
	return s.editRow(row, func(r *oxml.CT_Row) {
		if hidden {
			b := true
			r.Hidden = &b
		} else {
			r.Hidden = nil
		}
	})
}

// SetColumnHidden hides (hidden=true) or shows (hidden=false) a column
// (1-based). It is the write counterpart of ColumnHidden. A column entry
// spanning a range is split so only the target column is affected.
func (s *Sheet) SetColumnHidden(col int, hidden bool) error {
	return s.editColumn(col, func(c *oxml.CT_Col) {
		if hidden {
			b := true
			c.Hidden = &b
		} else {
			c.Hidden = nil
		}
	})
}

// ---------------------------------------------------------------------------
// Sheet-view toggles
// ---------------------------------------------------------------------------

// Sheet view modes for the sheetView view attribute, accepted by SetView and
// returned by View.
const (
	ViewNormal           = "normal"
	ViewPageLayout       = "pageLayout"
	ViewPageBreakPreview = "pageBreakPreview"
)

// View returns the sheet's view mode: ViewNormal (the default), ViewPageLayout,
// or ViewPageBreakPreview.
func (s *Sheet) View() string {
	if sv := s.sheetView(); sv != nil && sv.View != "" {
		return sv.View
	}
	return ViewNormal
}

// SetView sets the sheet's view mode. Valid values are ViewNormal,
// ViewPageLayout and ViewPageBreakPreview; any other value is rejected.
// ViewNormal is the OOXML default and is emitted as the absence of the
// attribute so a normal sheet is not perturbed.
func (s *Sheet) SetView(view string) error {
	switch view {
	case ViewNormal, ViewPageLayout, ViewPageBreakPreview:
	default:
		return fmt.Errorf("xlsx: invalid sheet view %q", view)
	}
	s.markDirty()
	s.ensureWorksheet()
	sv := s.ensureSheetView()
	if view == ViewNormal {
		sv.View = ""
	} else {
		sv.View = view
	}
	return nil
}

// ShowRowColHeaders reports whether row and column headers are shown. Defaults
// to true when unset (the OOXML default).
func (s *Sheet) ShowRowColHeaders() bool {
	return viewBoolAttr(s.sheetView(), func(sv *oxml.CT_SheetView) *bool { return sv.ShowRowColHeaders }, true)
}

// SetShowRowColHeaders sets whether row and column headers are shown.
func (s *Sheet) SetShowRowColHeaders(show bool) {
	s.markDirty()
	s.ensureWorksheet()
	s.ensureSheetView().ShowRowColHeaders = &show
}

// RightToLeft reports whether the sheet is displayed right-to-left. Defaults to
// false when unset.
func (s *Sheet) RightToLeft() bool {
	return viewBoolAttr(s.sheetView(), func(sv *oxml.CT_SheetView) *bool { return sv.RightToLeft }, false)
}

// SetRightToLeft sets whether the sheet is displayed right-to-left (columns run
// from right to left).
func (s *Sheet) SetRightToLeft(rtl bool) {
	s.markDirty()
	s.ensureWorksheet()
	s.ensureSheetView().RightToLeft = &rtl
}

// ShowFormulas reports whether cell formulas are shown instead of their
// results. Defaults to false when unset.
func (s *Sheet) ShowFormulas() bool {
	return viewBoolAttr(s.sheetView(), func(sv *oxml.CT_SheetView) *bool { return sv.ShowFormulas }, false)
}

// SetShowFormulas sets whether cell formulas are shown instead of their results.
func (s *Sheet) SetShowFormulas(show bool) {
	s.markDirty()
	s.ensureWorksheet()
	s.ensureSheetView().ShowFormulas = &show
}

// ShowZeros reports whether zero values are shown. Defaults to true when unset.
func (s *Sheet) ShowZeros() bool {
	return viewBoolAttr(s.sheetView(), func(sv *oxml.CT_SheetView) *bool { return sv.ShowZeros }, true)
}

// SetShowZeros sets whether cells holding zero display the value (true) or
// appear blank (false).
func (s *Sheet) SetShowZeros(show bool) {
	s.markDirty()
	s.ensureWorksheet()
	s.ensureSheetView().ShowZeros = &show
}

// ShowRuler reports whether the ruler is shown in page-layout view. Defaults to
// true when unset.
func (s *Sheet) ShowRuler() bool {
	return viewBoolAttr(s.sheetView(), func(sv *oxml.CT_SheetView) *bool { return sv.ShowRuler }, true)
}

// SetShowRuler sets whether the ruler is shown in page-layout view.
func (s *Sheet) SetShowRuler(show bool) {
	s.markDirty()
	s.ensureWorksheet()
	s.ensureSheetView().ShowRuler = &show
}

// ---------------------------------------------------------------------------
// Split panes
// ---------------------------------------------------------------------------

// SplitPanes creates a scrolling (unfrozen) split of the sheet view, distinct
// from FreezePanes which freezes rows/columns. xSplit and ySplit are the split
// bar positions measured in twentieths of a point (twips) from the left and top
// of the sheet; topLeftCell is the cell shown at the top-left of the
// bottom-right pane. activePane selects the initially active pane ("topLeft",
// "topRight", "bottomLeft" or "bottomRight"); when empty it is derived from
// which splits are present. Both offsets zero removes any existing pane.
func (s *Sheet) SplitPanes(xSplit, ySplit float64, topLeftCell, activePane string) error {
	if xSplit < 0 || ySplit < 0 {
		return fmt.Errorf("xlsx: split offsets must be non-negative")
	}
	switch activePane {
	case "", "topLeft", "topRight", "bottomLeft", "bottomRight":
	default:
		return fmt.Errorf("xlsx: invalid active pane %q", activePane)
	}
	if topLeftCell != "" {
		row, col, err := ParseCellRef(topLeftCell)
		if err != nil {
			return err
		}
		topLeftCell = FormatCellRef(row, col)
	}

	s.markDirty()
	s.ensureWorksheet()

	// Both offsets zero means no split: drop the pane (and pane-scoped
	// selections), mirroring FreezePanes at A1.
	if xSplit == 0 && ySplit == 0 {
		s.UnfreezePanes()
		return nil
	}

	if activePane == "" {
		switch {
		case xSplit > 0 && ySplit > 0:
			activePane = "bottomRight"
		case ySplit > 0:
			activePane = "bottomLeft"
		default:
			activePane = "topRight"
		}
	}

	sv := s.ensureSheetView()
	sv.Pane = &oxml.CT_Pane{
		TopLeftCell: topLeftCell,
		State:       "split",
		ActivePane:  activePane,
	}
	if xSplit > 0 {
		sv.Pane.XSplit = &xSplit
	}
	if ySplit > 0 {
		sv.Pane.YSplit = &ySplit
	}
	if topLeftCell != "" {
		sv.Selection = []oxml.CT_Selection{{
			Pane:       activePane,
			ActiveCell: topLeftCell,
			SqRef:      topLeftCell,
		}}
	}
	return nil
}

// SplitPanePosition reports a scrolling split created by SplitPanes: the split
// offsets in twips, the top-left cell of the bottom-right pane, and ok=true
// when the sheet has a split pane. ok is false for no pane or a frozen pane
// (see FrozenPanes for the frozen case).
func (s *Sheet) SplitPanePosition() (xSplit, ySplit float64, topLeftCell string, ok bool) {
	pane := s.pane()
	if pane == nil || pane.State != "split" {
		return 0, 0, "", false
	}
	if pane.XSplit != nil {
		xSplit = *pane.XSplit
	}
	if pane.YSplit != nil {
		ySplit = *pane.YSplit
	}
	return xSplit, ySplit, pane.TopLeftCell, true
}

// ---------------------------------------------------------------------------
// Row & column grouping / outline
// ---------------------------------------------------------------------------

// maxOutlineLevel is the deepest outline (grouping) level Excel supports.
const maxOutlineLevel = 7

// RowOutlineLevel returns a row's outline (grouping) level, 0 when the row is
// ungrouped.
func (s *Sheet) RowOutlineLevel(row int) uint8 {
	r := s.rowEntry(row)
	if r == nil || r.OutlineLevel == nil {
		return 0
	}
	return *r.OutlineLevel
}

// SetRowOutlineLevel sets a row's outline (grouping) level. Level 0 clears the
// grouping; levels above 7 (Excel's maximum) are rejected.
func (s *Sheet) SetRowOutlineLevel(row int, level uint8) error {
	if level > maxOutlineLevel {
		return fmt.Errorf("xlsx: outline level %d out of range (0-%d)", level, maxOutlineLevel)
	}
	return s.editRow(row, func(r *oxml.CT_Row) {
		if level == 0 {
			r.OutlineLevel = nil
		} else {
			l := level
			r.OutlineLevel = &l
		}
	})
}

// RowCollapsed reports whether a grouped row is collapsed.
func (s *Sheet) RowCollapsed(row int) bool {
	r := s.rowEntry(row)
	return r != nil && r.Collapsed != nil && *r.Collapsed
}

// SetRowCollapsed sets a row's collapsed flag (whether its outline group is
// collapsed). Note that collapsing an outline for display also requires hiding
// the member rows; this sets only the flag on the summary row.
func (s *Sheet) SetRowCollapsed(row int, collapsed bool) error {
	return s.editRow(row, func(r *oxml.CT_Row) {
		if collapsed {
			b := true
			r.Collapsed = &b
		} else {
			r.Collapsed = nil
		}
	})
}

// GroupRows increases the outline level of every row in [startRow, endRow]
// (1-based, inclusive) by one, up to Excel's maximum of 7. It is the counterpart
// of UngroupRows.
func (s *Sheet) GroupRows(startRow, endRow int) error {
	if startRow < 1 || endRow < startRow {
		return ErrInvalidRange
	}
	return s.editRowRange(startRow, endRow, func(r *oxml.CT_Row) {
		level := outlineLevelOf(r.OutlineLevel)
		if level < maxOutlineLevel {
			level++
		}
		setOutlineLevel(&r.OutlineLevel, level)
	})
}

// outlineLevelOf reads an optional outline level, treating an unset one as 0.
func outlineLevelOf(p *uint8) uint8 {
	if p == nil {
		return 0
	}
	return *p
}

// setOutlineLevel writes an outline level, clearing the attribute at level 0 so
// an ungrouped row or column emits no outlineLevel at all.
func setOutlineLevel(p **uint8, level uint8) {
	if level == 0 {
		*p = nil
		return
	}
	l := level
	*p = &l
}

// UngroupRows decreases the outline level of every row in [startRow, endRow]
// (1-based, inclusive) by one, down to zero.
func (s *Sheet) UngroupRows(startRow, endRow int) error {
	if startRow < 1 || endRow < startRow {
		return ErrInvalidRange
	}
	return s.editRowRange(startRow, endRow, func(r *oxml.CT_Row) {
		level := outlineLevelOf(r.OutlineLevel)
		if level > 0 {
			level--
		}
		setOutlineLevel(&r.OutlineLevel, level)
	})
}

// ColumnOutlineLevel returns a column's outline (grouping) level, 0 when the
// column is ungrouped.
func (s *Sheet) ColumnOutlineLevel(col int) uint8 {
	c := s.colEntry(col)
	if c == nil || c.OutlineLevel == nil {
		return 0
	}
	return *c.OutlineLevel
}

// SetColumnOutlineLevel sets a column's outline (grouping) level. Level 0 clears
// the grouping; levels above 7 (Excel's maximum) are rejected.
func (s *Sheet) SetColumnOutlineLevel(col int, level uint8) error {
	if level > maxOutlineLevel {
		return fmt.Errorf("xlsx: outline level %d out of range (0-%d)", level, maxOutlineLevel)
	}
	return s.editColumn(col, func(c *oxml.CT_Col) {
		if level == 0 {
			c.OutlineLevel = nil
		} else {
			l := level
			c.OutlineLevel = &l
		}
	})
}

// ColumnCollapsed reports whether a grouped column is collapsed.
func (s *Sheet) ColumnCollapsed(col int) bool {
	c := s.colEntry(col)
	return c != nil && c.Collapsed != nil && *c.Collapsed
}

// SetColumnCollapsed sets a column's collapsed flag (whether its outline group
// is collapsed).
func (s *Sheet) SetColumnCollapsed(col int, collapsed bool) error {
	return s.editColumn(col, func(c *oxml.CT_Col) {
		if collapsed {
			b := true
			c.Collapsed = &b
		} else {
			c.Collapsed = nil
		}
	})
}

// GroupColumns increases the outline level of every column in [startCol, endCol]
// (1-based, inclusive) by one, up to Excel's maximum of 7.
func (s *Sheet) GroupColumns(startCol, endCol int) error {
	if startCol < 1 || endCol < startCol {
		return ErrInvalidRange
	}
	return s.editColumnRange(startCol, endCol, func(c *oxml.CT_Col) {
		level := outlineLevelOf(c.OutlineLevel)
		if level < maxOutlineLevel {
			level++
		}
		setOutlineLevel(&c.OutlineLevel, level)
	})
}

// UngroupColumns decreases the outline level of every column in [startCol,
// endCol] (1-based, inclusive) by one, down to zero.
func (s *Sheet) UngroupColumns(startCol, endCol int) error {
	if startCol < 1 || endCol < startCol {
		return ErrInvalidRange
	}
	return s.editColumnRange(startCol, endCol, func(c *oxml.CT_Col) {
		level := outlineLevelOf(c.OutlineLevel)
		if level > 0 {
			level--
		}
		setOutlineLevel(&c.OutlineLevel, level)
	})
}

// OutlineSummary reports the sheet's outline summary placement: below reports
// whether summary rows sit below their detail (the default), right whether
// summary columns sit to the right of their detail (the default). Both default
// to true when unset (the OOXML default).
func (s *Sheet) OutlineSummary() (below, right bool) {
	below, right = true, true
	if s.ws() != nil && s.ws().SheetPr != nil && s.ws().SheetPr.OutlinePr != nil {
		op := s.ws().SheetPr.OutlinePr
		if op.SummaryBelow != nil {
			below = *op.SummaryBelow
		}
		if op.SummaryRight != nil {
			right = *op.SummaryRight
		}
	}
	return below, right
}

// SetOutlineSummary sets the sheet's outline summary placement (see
// OutlineSummary). It writes the sheetPr/outlinePr element.
func (s *Sheet) SetOutlineSummary(below, right bool) {
	s.markDirty()
	s.ensureWorksheet()
	if s.ws().SheetPr == nil {
		s.ws().SheetPr = &oxml.CT_SheetPr{}
	}
	s.ws().EnsureChildOrder("sheetPr")
	if s.ws().SheetPr.OutlinePr == nil {
		s.ws().SheetPr.OutlinePr = &oxml.CT_OutlinePr{}
	}
	s.ws().SheetPr.OutlinePr.SummaryBelow = &below
	s.ws().SheetPr.OutlinePr.SummaryRight = &right
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// sheetView returns the sheet's first sheetView, or nil when none exists.
func (s *Sheet) sheetView() *oxml.CT_SheetView {
	if s.ws() == nil || s.ws().SheetViews == nil ||
		len(s.ws().SheetViews.SheetView) == 0 {
		return nil
	}
	return &s.ws().SheetViews.SheetView[0]
}

// viewBoolAttr resolves a *bool sheetView attribute, returning def when the
// sheetView or the attribute is absent.
func viewBoolAttr(sv *oxml.CT_SheetView, get func(*oxml.CT_SheetView) *bool, def bool) bool {
	if sv == nil {
		return def
	}
	if p := get(sv); p != nil {
		return *p
	}
	return def
}

// editRow finds (or creates) the row for a 1-based row number and applies
// apply to it. It marks the sheet dirty and records the sheetData child order.
// Rows are looked up via rowNumberOf, not the raw r attribute: a row may
// legally omit r (C73), and matching on the attribute alone would append a
// duplicate row for the same row number (C230).
func (s *Sheet) editRow(row int, apply func(*oxml.CT_Row)) error {
	// A chartsheet/dialogsheet/macrosheet has no row grid; refuse rather than
	// report success for a change markDirty would discard (C241, C423).
	if s.opaque {
		return ErrNotWorksheet
	}
	if row < 1 || row > MaxRow {
		return ErrInvalidCell
	}
	s.markDirty()
	s.ensureWorksheet()
	s.ws().EnsureChildOrder("sheetData")

	r := uint32(row)
	for i := range s.ws().SheetData.Row {
		if rn, ok := rowNumberOf(&s.ws().SheetData.Row[i]); ok && rn == r {
			apply(&s.ws().SheetData.Row[i])
			return nil
		}
	}
	newRow := oxml.CT_Row{R: &r}
	apply(&newRow)
	s.ws().SheetData.Row = append(s.ws().SheetData.Row, newRow)
	return nil
}

// editRowRange applies apply to every row in [startRow, endRow], creating the
// rows that do not exist yet. It is editRow over a range, done in one pass:
// calling editRow per row re-scanned SheetData.Row every time, and since the
// loop appends rows as it goes that made grouping a tall range O(rows^2).
//
// The result is identical to the per-row loop, including the first-match-wins
// choice among duplicate row numbers and the append-at-the-end placement of new
// rows (marshalling sorts rows into ascending order).
func (s *Sheet) editRowRange(startRow, endRow int, apply func(*oxml.CT_Row)) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	if startRow < 1 || endRow > MaxRow {
		return ErrInvalidCell
	}
	s.markDirty()
	s.ensureWorksheet()
	s.ws().EnsureChildOrder("sheetData")

	sd := &s.ws().SheetData
	byNumber := make(map[uint32]int, len(sd.Row))
	for i := range sd.Row {
		if rn, ok := rowNumberOf(&sd.Row[i]); ok {
			if _, dup := byNumber[rn]; !dup {
				byNumber[rn] = i
			}
		}
	}
	for row := startRow; row <= endRow; row++ {
		r := uint32(row)
		if i, ok := byNumber[r]; ok {
			apply(&sd.Row[i])
			continue
		}
		newRow := oxml.CT_Row{R: &r}
		apply(&newRow)
		sd.Row = append(sd.Row, newRow)
		byNumber[r] = len(sd.Row) - 1
	}
	return nil
}

// editColumn carves the target 1-based column out of every <col> entry that
// spans it — across EVERY <cols> group, not just the first — and applies apply
// to the resulting single-column entry.
//
// This is the one place the column carve lives: SetColWidth, SetColumnHidden,
// SetColumnStyle, SetColumnOutlineLevel, SetColumnCollapsed and GroupColumns
// all route through it. colEntry resolves a column against all groups, so a
// covering entry left in a later group would overlap the carved [c,c] target
// and be rejected by Excel — the C127 defect, which had been fixed in
// SetColWidth alone and re-opened here as C383 because the two carves were
// ~90% duplicated. The [c,c] slice of a covering range inherits that range's
// other properties (width, style, hidden, ...) before apply runs, so a
// property the caller does not set is preserved; the remainder of the range
// keeps its own. The target is placed once, at the first covering entry found;
// a column no entry covers gets a fresh entry in the first group.
func (s *Sheet) editColumn(col int, apply func(*oxml.CT_Col)) error {
	// A chartsheet/dialogsheet/macrosheet has no column grid; refuse rather
	// than report success for a change markDirty would discard (C241, C423).
	if s.opaque {
		return ErrNotWorksheet
	}
	if col < 1 || col > MaxCol {
		return ErrInvalidCell
	}
	s.markDirty()
	s.ensureWorksheet()
	if len(s.ws().Cols) == 0 {
		s.ws().Cols = append(s.ws().Cols, oxml.CT_Cols{})
	}
	s.ws().EnsureChildOrder("cols")

	c := uint32(col)
	placed := false
	for gi := range s.ws().Cols {
		cols := s.ws().Cols[gi].Col
		rebuilt := make([]oxml.CT_Col, 0, len(cols)+2)
		for _, entry := range cols {
			if entry.Min > c || entry.Max < c {
				rebuilt = append(rebuilt, entry)
				continue
			}
			if entry.Min < c {
				left := entry
				left.Max = c - 1
				rebuilt = append(rebuilt, left)
			}
			if !placed {
				target := entry
				target.Min, target.Max = c, c
				apply(&target)
				rebuilt = append(rebuilt, target)
				placed = true
			}
			if entry.Max > c {
				right := entry
				right.Min = c + 1
				rebuilt = append(rebuilt, right)
			}
		}
		s.ws().Cols[gi].Col = rebuilt
	}
	if !placed {
		target := oxml.CT_Col{Min: c, Max: c}
		apply(&target)
		s.ws().Cols[0].Col = append(s.ws().Cols[0].Col, target)
	}
	return nil
}

// editColumnRange is editColumn over [startCol, endCol], carving every column in
// the range out of the existing <col> entries in a single pass.
//
// editColumn rebuilds every <cols> group from scratch on each call, so calling
// it per column made grouping a wide range O(cols^2) in both time and
// allocations — a full-width GroupColumns took seconds. One pass produces the
// same entries in the same order: each covered column becomes its own
// single-column entry in place (inheriting the covering entry's other
// attributes, first covering entry wins), the uncovered remainder of an entry is
// kept as its left/right fragments, and columns no entry covered are appended to
// the first group in ascending order.
func (s *Sheet) editColumnRange(startCol, endCol int, apply func(*oxml.CT_Col)) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	if startCol < 1 || endCol > MaxCol {
		return ErrInvalidCell
	}
	s.markDirty()
	s.ensureWorksheet()
	if len(s.ws().Cols) == 0 {
		s.ws().Cols = append(s.ws().Cols, oxml.CT_Cols{})
	}
	s.ws().EnsureChildOrder("cols")

	lo, hi := uint32(startCol), uint32(endCol)
	placed := make(map[uint32]bool, endCol-startCol+1)
	for gi := range s.ws().Cols {
		cols := s.ws().Cols[gi].Col
		rebuilt := make([]oxml.CT_Col, 0, len(cols)+2)
		for _, entry := range cols {
			if entry.Min > hi || entry.Max < lo {
				rebuilt = append(rebuilt, entry)
				continue
			}
			if entry.Min < lo {
				left := entry
				left.Max = lo - 1
				rebuilt = append(rebuilt, left)
			}
			from, to := max(entry.Min, lo), min(entry.Max, hi)
			for c := from; c <= to; c++ {
				// A column already carved out of an earlier group keeps that
				// entry; this one just loses the overlapping stretch, exactly as
				// the per-column carve did.
				if placed[c] {
					continue
				}
				target := entry
				target.Min, target.Max = c, c
				apply(&target)
				rebuilt = append(rebuilt, target)
				placed[c] = true
			}
			if entry.Max > hi {
				right := entry
				right.Min = hi + 1
				rebuilt = append(rebuilt, right)
			}
		}
		s.ws().Cols[gi].Col = rebuilt
	}
	for c := lo; c <= hi; c++ {
		if placed[c] {
			continue
		}
		target := oxml.CT_Col{Min: c, Max: c}
		apply(&target)
		s.ws().Cols[0].Col = append(s.ws().Cols[0].Col, target)
	}
	return nil
}
