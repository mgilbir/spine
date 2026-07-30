package xlsx

import (
	"fmt"
	"strconv"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// ws returns the sheet's parsed worksheet model, parsing it lazily from the
// preserved raw bytes on first access. Returns nil for a created sheet that has
// no content yet, or when the raw bytes are unavailable. Parsing never marks
// the sheet dirty, so an accessed-but-unmodified sheet still round-trips from
// its raw bytes. Open validates every non-opaque sheet up front
// (parse-then-discard, see loadSheets), so a lazy parse here does not
// re-introduce the malformed-sheet error that Open already surfaces.
//
// If the bytes ARE present and the re-parse fails, that is in-memory corruption
// of bytes this process already parsed successfully, and it panics with a
// diagnostic rather than returning nil. A nil model here reads as an empty
// sheet: every cell gone, silently, and then written back that way on save.
// docx's doc() has always made this choice for the identical state (C568); the
// three copies of this function now agree.
func (s *Sheet) ws() *oxml.CT_Worksheet {
	if s.wsModel == nil && !s.wsParsed {
		s.wsParsed = true
		if s.workbook != nil && s.partName != "" {
			if part, ok := s.workbook.preservedParts[s.partName]; ok && part != nil {
				m := &oxml.CT_Worksheet{}
				if err := xmlb.Unmarshal(part.Data, m); err == nil {
					s.wsModel = m
				} else if !s.opaque {
					s.wsParseErr = err
				}
			}
		}
	}
	if s.wsParseErr != nil {
		panic(fmt.Sprintf("xlsx: lazy parse of worksheet part %s failed: %v "+
			"(Open validated the same bytes, so this indicates in-memory corruption)",
			s.partName, s.wsParseErr))
	}
	return s.wsModel
}

// ensureWS returns the sheet's worksheet model, parsing it lazily if needed and
// creating an empty one (with an initialized sheetData) when the sheet has no
// content yet. The result is always non-nil; callers that mutate the model use
// this. Creating the empty model does not itself mark the sheet dirty — the
// mutating caller is responsible for markDirty.
func (s *Sheet) ensureWS() *oxml.CT_Worksheet {
	if w := s.ws(); w != nil {
		return w
	}
	s.wsModel = &oxml.CT_Worksheet{SheetData: oxml.CT_SheetData{}}
	s.wsParsed = true
	return s.wsModel
}

// Sheet represents a worksheet in an Excel workbook.
type Sheet struct {
	workbook *Workbook
	name     string
	index    int
	partName string
	relID    string
	// wsModel is the parsed worksheet model. For an opened sheet it is parsed
	// lazily from the preserved raw bytes on first access (see ws): a workbook
	// that is never inspected, or is round-tripped unmodified, then holds only
	// the raw sheet bytes rather than a full model per sheet. Access it through
	// ws()/ensureWS(), never directly, except on the save path where a nil
	// wsModel must mean "not materialized" (so a clean sheet round-trips from
	// its raw bytes) and must not trigger a parse.
	wsModel *oxml.CT_Worksheet
	// wsParsed records that a lazy parse was already attempted, so a genuinely
	// empty or unparseable sheet is not re-parsed on every access.
	wsParsed bool
	// wsParseErr holds the failure of a lazy re-parse of bytes Open already
	// parsed — an impossible state that ws() reports by panicking rather than
	// by silently reading the sheet as empty (C568).
	wsParseErr error
	images    []sheetImage
	charts    []sheetChart   // charts added this session via AddChart
	newTables []*Table       // tables added this session via AddTable (to be written)
	// tablePartsBaseline is the number of <tableParts> entries present before this
	// session's AddTable calls, captured on the first save. Each save rebuilds the
	// session-added entries from this baseline instead of appending them anew, so
	// a repeated save is a projection rather than a state transition (C257: without
	// it a second save duplicated every session-added tablePart and the durable
	// model grew each pass).
	tablePartsBaseline    int
	tablePartsBaselineSet bool
	newPivots []*PivotTable  // pivot tables added this session via AddPivotTable
	oleEmbeds []pendingOLE   // OLE objects embedded this session via AddOLEObject
	comments  *sheetComments // lazily loaded comment model (read + write)
	// sparklineCache is the sheet's parsed sparkline-groups model, loaded lazily
	// from the worksheet extension list and shared by every SparklineGroup handle
	// so mutations write through consistently. nil until first accessed.
	sparklineCache *oxml.CT_SparklineGroups
	// state is the workbook-level sheet visibility ("", "hidden" or
	// "veryHidden"). AddChart marks its dedicated data sheet "hidden".
	state string
	// opaque marks a non-worksheet sheet (chartsheet/dialogsheet/macrosheet). Its
	// part is preserved verbatim and never parsed or regenerated as a worksheet;
	// the worksheet mutation API (Cell, SetCellValue, ...) refuses it and the save
	// path keeps its own relationship and bytes untouched (C241).
	opaque bool
	// pendingHyperlinkRels are External hyperlink relationships added via
	// SetHyperlink that must be merged into the sheet's .rels on save. Their ids
	// are already baked into the matching <hyperlink r:id> in the worksheet model.
	pendingHyperlinkRels []*opc.Relationship
	// removedHyperlinkRIDs are relationship ids of hyperlinks that were removed
	// or replaced this session. A hyperlink loaded from the opened file keeps
	// its relationship in w.relationships[partName]; without filtering, the
	// rebuilt sheet .rels would re-emit the now-unreferenced external rel
	// (package bloat and a stale target URL). Filtered from the cloned sheet
	// rels at save.
	removedHyperlinkRIDs map[string]bool
	dirty                bool
}

// Name returns the sheet name.
func (s *Sheet) Name() string {
	return s.name
}

// SetName renames the sheet. The name must be a legal Excel sheet name (see
// ValidateSheetName) and must not collide (case-insensitively) with another
// sheet in the workbook; invalid or duplicate names are rejected with an
// error and the sheet is left unchanged (C71).
//
// The name lives in workbook.xml, which is always regenerated, so renaming
// does not mark the worksheet part dirty: an otherwise untouched sheet still
// round-trips byte-for-byte and keeps the workbook's calcChain, whether or not
// the sheet's model happened to be materialized first (C545).
//
// Limitation: renaming does not rewrite references to the old name held in
// formulas or defined names; those still refer to the previous name.
//
// A *Sheet obtained before a DeleteSheet call that removed it is detached from
// its workbook; SetName on such a handle changes nothing.
func (s *Sheet) SetName(name string) error {
	if err := ValidateSheetName(name); err != nil {
		return err
	}
	if s.workbook != nil {
		for _, other := range s.workbook.sheets {
			if other != s && strings.EqualFold(other.name, name) {
				return fmt.Errorf("xlsx: sheet name %q already exists", name)
			}
		}
	}
	s.name = name
	// Update the workbook model if within bounds
	if s.workbook != nil && s.index >= 0 && s.index < len(s.workbook.workbook.Sheets.Sheet) {
		s.workbook.workbook.Sheets.Sheet[s.index].Name = name
	}
	// The rename persists from the always-regenerated workbook.xml and so needs
	// no regeneration flag; it is still a content change, and nothing else would
	// record it. Only reached once the new name has been accepted, so a rejected
	// rename does not move dcterms:modified.
	s.workbook.markContentEdited()
	return nil
}

// Index returns the sheet index within the workbook.
func (s *Sheet) Index() int {
	return s.index
}

// Cell returns the cell at the specified reference (e.g., "A1").
// If the cell doesn't exist in the worksheet data, it is created.
// The reference is canonicalized (case and leading zeros normalized), so
// "a1" and "A01" address the same cell as "A1" (C126).
//
// Because it creates, Cell is a mutating accessor: probing a range with it
// materializes a <c>/<row> for every reference visited. Such a cell carries no
// value, formula, inline string or style and is dropped again when the sheet is
// serialized, so it neither reaches the file nor inflates <dimension>, but a
// read-only lookup should use FindCell (or GetCellValue) instead (C425).
//
// A chartsheet, dialogsheet or macrosheet has no cell grid; Cell returns
// ErrNotWorksheet for one.
func (s *Sheet) Cell(ref string) (*Cell, error) {
	// A chartsheet/dialogsheet/macrosheet has no worksheet cell grid; refuse the
	// write rather than overwrite its preserved part with a <worksheet> root (C241).
	if s.opaque {
		return nil, ErrNotWorksheet
	}
	s.ensureWS()
	s.ws().EnsureChildOrder("sheetData")

	// Parse the reference to get row and column
	row, col, err := ParseCellRef(ref)
	if err != nil {
		return nil, err
	}
	// Canonicalize: "A01" must address the same cell as "A1", not create a
	// phantom sibling with a non-canonical r attribute.
	ref = FormatCellRef(row, col)

	// Find or create the row
	var targetRow *oxml.CT_Row
	for i := range s.ws().SheetData.Row {
		if rn, ok := rowNumberOf(&s.ws().SheetData.Row[i]); ok && rn == uint32(row) {
			targetRow = &s.ws().SheetData.Row[i]
			break
		}
	}
	if targetRow == nil {
		r := uint32(row)
		s.ws().SheetData.Row = append(s.ws().SheetData.Row, oxml.CT_Row{R: &r})
		targetRow = &s.ws().SheetData.Row[len(s.ws().SheetData.Row)-1]
	}

	// Find or create the cell. Cells are stored as pointers so this handle
	// remains valid even if later cells are appended to the same row.
	for _, cell := range targetRow.C {
		if strings.EqualFold(cell.R, ref) {
			return &Cell{sheet: s, cell: cell}, nil
		}
	}

	newCell := &oxml.CT_Cell{R: ref}
	targetRow.C = append(targetRow.C, newCell)
	return &Cell{sheet: s, cell: newCell}, nil
}

// rowNumberOf returns the 1-based row number for a parsed row. A row may omit
// the optional r attribute (legal SpreadsheetML), in which case the number is
// derived from its cell references so the row is still addressable (C73).
func rowNumberOf(r *oxml.CT_Row) (uint32, bool) {
	if r.R != nil {
		return *r.R, true
	}
	for _, c := range r.C {
		if rn, _, err := ParseCellRef(c.R); err == nil {
			return uint32(rn), true
		}
	}
	return 0, false
}

// CellByRowCol returns the cell at the specified row and column (1-based).
func (s *Sheet) CellByRowCol(row, col int) (*Cell, error) {
	ref, err := CellRef(row, col)
	if err != nil {
		return nil, err
	}
	return s.Cell(ref)
}

// SetCellValue sets the value of a cell.
func (s *Sheet) SetCellValue(ref string, value interface{}) error {
	cell, err := s.Cell(ref)
	if err != nil {
		return err
	}
	cell.SetValue(value)
	return nil
}

// CellValue returns the cell's stored value as a string: the resolved text of a
// shared or inline string, and otherwise the raw <v> literal. It is NOT the
// display value — the cell's number format is not applied, so a date reads back
// as its serial and 0.5 formatted as "50%" reads back as "0.5". Use Sheet.Text
// for the formatted rendering. An absent cell yields "" with no error; an
// unparseable reference yields ErrInvalidCell, and a chartsheet / dialogsheet /
// macrosheet ErrNotWorksheet.
//
// It is the Get-less spelling of GetCellValue, matching the rest of the
// library's accessors (C565).
func (s *Sheet) CellValue(ref string) (string, error) {
	return s.GetCellValue(ref)
}

// GetCellValue returns the cell's stored value as a string.
//
// Deprecated: use CellValue. Go accessors do not carry a Get prefix, and this
// was one of a handful of methods library-wide that did (C565).
func (s *Sheet) GetCellValue(ref string) (string, error) {
	if s.opaque {
		return "", ErrNotWorksheet
	}
	row, col, err := ParseCellRef(ref)
	if err != nil {
		return "", err
	}
	if c := s.findCell(FormatCellRef(row, col)); c != nil {
		return c.String(), nil
	}
	return "", nil
}

// FindCell returns a handle to the cell at ref without creating anything.
// Unlike Cell it is a read-only lookup: an absent cell, an unparseable
// reference, an empty sheet or a non-worksheet sheet all yield nil, so
// scanning a range never spawns phantom <c>/<row> entries in the model (C425).
func (s *Sheet) FindCell(ref string) *Cell {
	return s.findCell(ref)
}

// findCell is the unexported spelling of FindCell, kept for internal callers.
func (s *Sheet) findCell(ref string) *Cell {
	if s.opaque || s.ws() == nil {
		return nil
	}
	row, col, err := ParseCellRef(ref)
	if err != nil {
		return nil
	}
	ref = FormatCellRef(row, col)
	for i := range s.ws().SheetData.Row {
		r := &s.ws().SheetData.Row[i]
		if rn, ok := rowNumberOf(r); ok && rn == uint32(row) {
			for _, cell := range r.C {
				if strings.EqualFold(cell.R, ref) {
					return &Cell{sheet: s, cell: cell}
				}
			}
		}
	}
	return nil
}

// Rows returns the number of used rows. Rows holding nothing but cells a
// read-only Cell probe materialized are not counted — they carry no content
// and are dropped at serialization (C425).
func (s *Sheet) Rows() int {
	if s.ws() == nil {
		return 0
	}
	n := 0
	for i := range s.ws().SheetData.Row {
		if !rowIsEmptyPhantom(&s.ws().SheetData.Row[i]) {
			n++
		}
	}
	return n
}

// Cols returns the number of used columns (maximum column across all rows).
func (s *Sheet) Cols() int {
	if s.ws() == nil {
		return 0
	}

	maxCol := 0
	for i := range s.ws().SheetData.Row {
		for _, cell := range s.ws().SheetData.Row[i].C {
			if cellIsEmptyPhantom(cell) {
				continue
			}
			_, col, err := ParseCellRef(cell.R)
			if err == nil && col > maxCol {
				maxCol = col
			}
		}
	}
	return maxCol
}

// cellIsEmptyPhantom reports whether a cell carries no information at all: no
// value, formula, inline string, style, type or any of the optional metadata
// attributes. Such an element is what a read-only Cell() probe leaves behind,
// and re-emitting it is pure noise that also inflates the recorded used range
// (C425). A cell with a style but no value is real content and is not a
// phantom.
func cellIsEmptyPhantom(c *oxml.CT_Cell) bool {
	return c == nil || (c.F == nil && c.V == nil && c.Is == nil && c.S == nil &&
		c.T == "" && c.Cm == nil && c.Vm == nil && c.Ph == nil && len(c.ExtRaw) == 0)
}

// rowIsEmptyPhantom reports whether a row carries no cells with content and no
// row-level properties of its own beyond its number.
func rowIsEmptyPhantom(r *oxml.CT_Row) bool {
	if r == nil {
		return true
	}
	for _, c := range r.C {
		if !cellIsEmptyPhantom(c) {
			return false
		}
	}
	return r.Spans == "" && r.S == nil && r.CustomFormat == nil && r.Ht == nil &&
		r.Hidden == nil && r.CustomHeight == nil && r.OutlineLevel == nil &&
		r.Collapsed == nil && r.ThickTop == nil && r.ThickBot == nil &&
		r.Ph == nil && r.DyDescent == nil && len(r.ExtRaw) == 0
}

// prunedRows returns rows without the contentless <c>/<row> elements a
// read-only Cell() probe leaves in the model, so a later unrelated edit does
// not serialize them nor inflate the recorded used range (C425).
//
// It returns rows itself when there is nothing to drop, and otherwise a fresh
// slice: the durable model is never mutated, so a *Cell handle the caller
// still holds keeps addressing the model even across a save that omitted its
// (then empty) cell.
func prunedRows(rows []oxml.CT_Row) []oxml.CT_Row {
	prune := false
	for i := range rows {
		if rowIsEmptyPhantom(&rows[i]) {
			prune = true
			break
		}
		for _, c := range rows[i].C {
			if cellIsEmptyPhantom(c) {
				prune = true
				break
			}
		}
		if prune {
			break
		}
	}
	if !prune {
		return rows
	}
	out := make([]oxml.CT_Row, 0, len(rows))
	for i := range rows {
		r := rows[i] // value copy; its C slice is replaced below, never sliced in place
		if rowIsEmptyPhantom(&r) {
			continue
		}
		kept := make([]*oxml.CT_Cell, 0, len(r.C))
		for _, c := range r.C {
			if cellIsEmptyPhantom(c) {
				continue
			}
			kept = append(kept, c)
		}
		r.C = kept
		out = append(out, r)
	}
	return out
}

// SetColWidth sets the width of a column (1-based). Existing <col> entries
// covering a range of columns (min < max) are split so the target column is
// carved out with the new width while the rest of the range keeps its
// original properties; appending an overlapping entry would be ambiguous and
// is rejected by Excel (C127). It shares the carve with every other column
// mutator through editColumn (C383).
//
// A chartsheet, dialogsheet or macrosheet has no column grid; SetColWidth
// returns ErrNotWorksheet for one.
func (s *Sheet) SetColWidth(col int, width float64) error {
	w := width
	customWidth := true
	return s.editColumn(col, func(c *oxml.CT_Col) {
		c.Width = &w
		c.CustomWidth = &customWidth
	})
}

// SetRowHeight sets the height of a row (1-based). The row must lie inside the
// worksheet grid; a row past MaxRow yields ErrInvalidCell, matching editRow
// and SetColWidth rather than silently appending an out-of-grid <row> (C546).
func (s *Sheet) SetRowHeight(row int, height float64) error {
	h := height
	customHeight := true
	return s.editRow(row, func(r *oxml.CT_Row) {
		r.Ht = &h
		r.CustomHeight = &customHeight
	})
}

// cellRange is a rectangular cell range with 1-based inclusive bounds.
type cellRange struct {
	minRow, minCol, maxRow, maxCol int
}

// normalizeCellRange parses two cell references and returns the normalized
// top-left:bottom-right rectangle. Invalid references yield ErrInvalidRange.
func normalizeCellRange(startRef, endRef string) (cellRange, error) {
	r1, c1, err := ParseCellRef(startRef)
	if err != nil {
		return cellRange{}, ErrInvalidRange
	}
	r2, c2, err := ParseCellRef(endRef)
	if err != nil {
		return cellRange{}, ErrInvalidRange
	}
	return cellRange{
		minRow: min(r1, r2), minCol: min(c1, c2),
		maxRow: max(r1, r2), maxCol: max(c1, c2),
	}, nil
}

// ref renders the range as "A1:B2".
func (r cellRange) ref() string {
	return FormatCellRef(r.minRow, r.minCol) + ":" + FormatCellRef(r.maxRow, r.maxCol)
}

// overlaps reports whether the two rectangles intersect.
func (r cellRange) overlaps(o cellRange) bool {
	return r.minRow <= o.maxRow && o.minRow <= r.maxRow &&
		r.minCol <= o.maxCol && o.minCol <= r.maxCol
}

// parseCellRangeRef parses a stored "A1:B2" range reference.
func parseCellRangeRef(ref string) (cellRange, error) {
	start, end, ok := strings.Cut(ref, ":")
	if !ok {
		// A single-cell merge reference is legal; treat it as a 1x1 range.
		start, end = ref, ref
	}
	return normalizeCellRange(start, end)
}

// MergeCells merges a range of cells. The references are validated and
// normalized to top-left:bottom-right order; invalid references return
// ErrInvalidRange. A merge that duplicates or overlaps an existing merged
// range is rejected — Excel refuses overlapping merges (C128).
func (s *Sheet) MergeCells(startRef, endRef string) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	rng, err := normalizeCellRange(startRef, endRef)
	if err != nil {
		return err
	}

	if s.ws() != nil && s.ws().MergeCells != nil {
		for _, mc := range s.ws().MergeCells.MergeCell {
			existing, err := parseCellRangeRef(mc.Ref)
			if err != nil {
				continue // unparseable existing entry imposes no constraint
			}
			if rng.overlaps(existing) {
				return fmt.Errorf("xlsx: merge range %s overlaps existing merge %s", rng.ref(), mc.Ref)
			}
		}
	}

	s.markDirty()
	s.ensureWS()

	if s.ws().MergeCells == nil {
		s.ws().MergeCells = &oxml.CT_MergeCells{}
	}
	s.ws().EnsureChildOrder("mergeCells")

	s.ws().MergeCells.MergeCell = append(s.ws().MergeCells.MergeCell, oxml.CT_MergeCell{Ref: rng.ref()})
	count := uint32(len(s.ws().MergeCells.MergeCell))
	s.ws().MergeCells.Count = &count

	return nil
}

// UnmergeCells unmerges a range of cells.
func (s *Sheet) UnmergeCells(startRef, endRef string) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	if s.ws() == nil || s.ws().MergeCells == nil {
		return nil
	}

	rng, err := normalizeCellRange(startRef, endRef)
	if err != nil {
		return err
	}
	ref := rng.ref()

	for i, mc := range s.ws().MergeCells.MergeCell {
		if strings.EqualFold(mc.Ref, ref) {
			s.markDirty()
			s.ws().MergeCells.MergeCell = append(
				s.ws().MergeCells.MergeCell[:i],
				s.ws().MergeCells.MergeCell[i+1:]...,
			)
			count := uint32(len(s.ws().MergeCells.MergeCell))
			s.ws().MergeCells.Count = &count
			if len(s.ws().MergeCells.MergeCell) == 0 {
				s.ws().MergeCells = nil
			}
			return nil
		}
	}

	return nil
}

// CellRef converts row and column indices (1-based) to a cell reference.
func CellRef(row, col int) (string, error) {
	if row < 1 || row > MaxRow || col < 1 || col > MaxCol {
		return "", ErrInvalidCell
	}
	return columnLetters(col) + strconv.Itoa(row), nil
}

// columnLetters converts a 1-based column number to column letters. It returns
// "" for a non-positive column, which callers must treat as invalid.
func columnLetters(col int) string {
	if col < 1 {
		return ""
	}
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

// FormatCellRef creates a cell reference from 1-based row and column numbers.
// It returns "" for coordinates outside the worksheet grid rather than an
// invalid reference such as "5" (column 0).
func FormatCellRef(row, col int) string {
	if row < 1 || row > MaxRow || col < 1 || col > MaxCol {
		return ""
	}
	// Plain concatenation rather than fmt.Sprintf: this is on the hot path of
	// every range walk, and the formatted form cost an interface boxing plus a
	// reflection-driven format pass per cell.
	return columnLetters(col) + strconv.Itoa(row)
}

// FreezePanes freezes rows and columns at the specified cell reference.
// For example, "B2" freezes row 1 and column A. The reference is
// canonicalized, so "b2" behaves like "B2". Freezing at A1 freezes nothing:
// it removes any existing pane instead of emitting a frozen pane with no
// splits, which Excel flags as invalid (C133).
func (s *Sheet) FreezePanes(cellRef string) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	row, col, err := ParseCellRef(cellRef)
	if err != nil {
		return err
	}
	cellRef = FormatCellRef(row, col)
	s.markDirty()

	// A1 means "no frozen rows or columns": drop the pane entirely.
	if row == 1 && col == 1 {
		s.UnfreezePanes()
		return nil
	}

	s.ensureWorksheet()
	sv := s.ensureSheetView()

	xSplit := float64(col - 1)
	ySplit := float64(row - 1)

	sv.Pane = &oxml.CT_Pane{
		TopLeftCell: cellRef,
		State:       "frozen",
	}
	if xSplit > 0 {
		sv.Pane.XSplit = &xSplit
	}
	if ySplit > 0 {
		sv.Pane.YSplit = &ySplit
	}

	// Set active pane
	if xSplit > 0 && ySplit > 0 {
		sv.Pane.ActivePane = "bottomRight"
	} else if ySplit > 0 {
		sv.Pane.ActivePane = "bottomLeft"
	} else if xSplit > 0 {
		sv.Pane.ActivePane = "topRight"
	}

	// Add selection for the active pane
	sv.Selection = []oxml.CT_Selection{{
		Pane:       sv.Pane.ActivePane,
		ActiveCell: cellRef,
		SqRef:      cellRef,
	}}

	return nil
}

// UnfreezePanes removes any frozen panes from the sheet, along with
// selections that referenced a pane (a pane-scoped selection is invalid once
// the pane is gone).
func (s *Sheet) UnfreezePanes() {
	if s.ws() == nil || s.ws().SheetViews == nil {
		return
	}
	if len(s.ws().SheetViews.SheetView) > 0 {
		s.markDirty()
		sv := &s.ws().SheetViews.SheetView[0]
		sv.Pane = nil
		kept := sv.Selection[:0]
		for _, sel := range sv.Selection {
			if sel.Pane == "" {
				kept = append(kept, sel)
			}
		}
		sv.Selection = kept
	}
}

// SetZoom sets the zoom percentage for the sheet view (e.g., 100 for 100%).
func (s *Sheet) SetZoom(percent uint32) {
	s.markDirty()
	s.ensureWorksheet()
	sv := s.ensureSheetView()
	sv.ZoomScale = &percent
	sv.ZoomScaleNormal = &percent
}

// SetShowGridLines sets whether grid lines are displayed.
func (s *Sheet) SetShowGridLines(show bool) {
	s.markDirty()
	s.ensureWorksheet()
	sv := s.ensureSheetView()
	sv.ShowGridLines = &show
}

// SetTabColor sets the sheet tab color as a hex RGB string (e.g., "FF0000").
func (s *Sheet) SetTabColor(hexColor string) {
	s.markDirty()
	s.ensureWorksheet()
	if s.ws().SheetPr == nil {
		s.ws().SheetPr = &oxml.CT_SheetPr{}
	}
	s.ws().EnsureChildOrder("sheetPr")
	s.ws().SheetPr.TabColor = &oxml.CT_Color{
		Rgb: hexColor,
	}
}

// SetAutoFilter sets an auto-filter on the specified range (e.g., "A1:F1").
// The range must be a single rectangular reference; an unparseable one returns
// ErrInvalidRange rather than reaching <autoFilter ref="...">, where it makes
// Excel offer to repair the workbook (C538).
//
// Only the <autoFilter> element is written. Excel additionally maintains a
// hidden sheet-scoped _xlnm._FilterDatabase defined name over the same range;
// this package neither creates it here nor removes it in RemoveAutoFilter.
// Excel recreates it when the user next touches the filter, so its absence is
// not an error, but a workbook opened, filtered here and reopened will not show
// the name until then.
func (s *Sheet) SetAutoFilter(rangeRef string) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	if _, err := parseCellRangeRef(strings.TrimSpace(rangeRef)); err != nil {
		return err
	}
	s.markDirty()
	s.ensureWorksheet()
	s.ws().AutoFilter = &oxml.CT_AutoFilter{
		Ref: strings.ToUpper(rangeRef),
	}
	s.ws().EnsureChildOrder("autoFilter")
	return nil
}

// RemoveAutoFilter removes the auto-filter from the sheet.
func (s *Sheet) RemoveAutoFilter() {
	if s.ws() != nil {
		s.markDirty()
		s.ws().AutoFilter = nil
	}
}

// DataValidation represents a data validation rule.
type DataValidation struct {
	Range    string // cell range (e.g., "B2:B100")
	Type     string // "list", "whole", "decimal", "date", "textLength", "custom"
	Operator string // "between", "lessThan", "equal", etc.
	Formula1 string
	Formula2 string
	// AllowBlank permits empty cells regardless of the rule.
	AllowBlank bool
	// HideDropDown suppresses the in-cell dropdown arrow for list validations.
	// By default Excel shows the dropdown; the underlying OOXML attribute
	// showDropDown counterintuitively means "suppress the dropdown", so this
	// field is named for what it actually does (C76).
	HideDropDown bool
	// ErrorTitle/ErrorMessage define the alert Excel shows on invalid input.
	// When either is set, showErrorMessage is emitted automatically — without
	// it Excel never displays the alert.
	ErrorTitle   string
	ErrorMessage string
	// PromptTitle/PromptMessage define the input hint shown when the cell is
	// selected. When either is set, showInputMessage is emitted automatically.
	PromptTitle   string
	PromptMessage string
	// ErrorStyle selects the alert icon/behavior for invalid input:
	// ValidationErrorStop (the default, rejects the entry), ValidationErrorWarning
	// (allows it after a prompt) or ValidationErrorInformation (informational
	// only). Empty leaves the attribute unset, which Excel treats as stop.
	ErrorStyle string
	// ImeMode controls the Input Method Editor state for the cell (East-Asian
	// text entry), e.g. "off", "on", "disabled", "hiragana". Empty leaves it
	// unset.
	ImeMode string
}

// Data-validation errorStyle values (ST_DataValidationErrorStyle): the alert
// behavior Excel applies when a cell fails validation.
const (
	ValidationErrorStop        = "stop"
	ValidationErrorWarning     = "warning"
	ValidationErrorInformation = "information"
)

// AddDataValidation adds a data validation rule to the sheet.
func (s *Sheet) AddDataValidation(dv DataValidation) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	// Range is an sqref: one range or a space-separated list. An unparseable
	// one used to be accepted here and surface only as a non-blocking save
	// warning, unlike every sibling API that parses its range (C538).
	if _, _, err := normalizeSqref(dv.Range); err != nil {
		return err
	}
	s.markDirty()
	s.ensureWorksheet()
	if s.ws().DataValidations == nil {
		s.ws().DataValidations = &oxml.CT_DataValidations{}
	}
	s.ws().EnsureChildOrder("dataValidations")

	v := oxml.CT_DataValidation{
		Sqref:       strings.ToUpper(dv.Range),
		Type:        dv.Type,
		Operator:    dv.Operator,
		ErrorStyle:  dv.ErrorStyle,
		ImeMode:     dv.ImeMode,
		ErrorTitle:  dv.ErrorTitle,
		Error:       dv.ErrorMessage,
		PromptTitle: dv.PromptTitle,
		Prompt:      dv.PromptMessage,
	}

	show := true
	if dv.AllowBlank {
		v.AllowBlank = &dv.AllowBlank
	}
	if dv.HideDropDown {
		// OOXML: showDropDown="1" SUPPRESSES the in-cell dropdown; absent
		// means the dropdown is shown (the Excel default).
		v.ShowDropDown = &show
	}
	if dv.ErrorTitle != "" || dv.ErrorMessage != "" {
		v.ShowErrorMessage = &show
	}
	if dv.PromptTitle != "" || dv.PromptMessage != "" {
		v.ShowInputMessage = &show
	}
	if dv.Formula1 != "" {
		v.Formula1 = &dv.Formula1
	}
	if dv.Formula2 != "" {
		v.Formula2 = &dv.Formula2
	}

	s.ws().DataValidations.DataValidation = append(s.ws().DataValidations.DataValidation, v)
	count := uint32(len(s.ws().DataValidations.DataValidation))
	s.ws().DataValidations.Count = &count

	return nil
}

func (s *Sheet) ensureWorksheet() {
	s.ensureWS()
}

func (s *Sheet) markDirty() {
	// An opaque (non-worksheet) sheet is preserved verbatim and never regenerated
	// from a worksheet model, so it must never be marked dirty — a dirty opaque
	// sheet would otherwise be treated as a rewritable worksheet on save (C241).
	if s != nil && !s.opaque {
		s.dirty = true
		// Recording the edit here, inside the guard, is what makes the
		// dcterms:modified stamp inherit this flag's correctness: an edit the
		// save path discards (opaque sheet) does not move the timestamp either.
		s.workbook.markContentEdited()
	}
}

func (s *Sheet) ensureSheetView() *oxml.CT_SheetView {
	if s.ws().SheetViews == nil {
		s.ws().SheetViews = &oxml.CT_SheetViews{}
	}
	s.ws().EnsureChildOrder("sheetViews")
	if len(s.ws().SheetViews.SheetView) == 0 {
		s.ws().SheetViews.SheetView = append(s.ws().SheetViews.SheetView, oxml.CT_SheetView{})
	}
	return &s.ws().SheetViews.SheetView[0]
}
