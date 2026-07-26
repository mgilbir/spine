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
// its raw bytes. Open validates every sheet up front (parse-then-discard), so a
// lazy parse here does not re-introduce the malformed-sheet error that Open
// already surfaces.
func (s *Sheet) ws() *oxml.CT_Worksheet {
	if s.wsModel == nil && !s.wsParsed {
		s.wsParsed = true
		if s.workbook != nil && s.partName != "" {
			if part, ok := s.workbook.preservedParts[s.partName]; ok && part != nil {
				m := &oxml.CT_Worksheet{}
				if err := xmlb.Unmarshal(part.Data, m); err == nil {
					s.wsModel = m
				}
			}
		}
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
	wsParsed  bool
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
// Limitation: renaming does not rewrite references to the old name held in
// formulas or defined names; those still refer to the previous name.
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
	if s.workbook != nil && s.index < len(s.workbook.workbook.Sheets.Sheet) {
		s.workbook.workbook.Sheets.Sheet[s.index].Name = name
	}
	s.markDirty()
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

// GetCellValue returns the display value of a cell as a string.
func (s *Sheet) GetCellValue(ref string) (string, error) {
	if s.ws() == nil {
		return "", nil
	}

	row, col, err := ParseCellRef(ref)
	if err != nil {
		return "", err
	}
	ref = FormatCellRef(row, col)

	for i := range s.ws().SheetData.Row {
		r := &s.ws().SheetData.Row[i]
		if rn, ok := rowNumberOf(r); ok && rn == uint32(row) {
			for _, cell := range r.C {
				if strings.EqualFold(cell.R, ref) {
					c := &Cell{sheet: s, cell: cell}
					return c.String(), nil
				}
			}
		}
	}

	return "", nil
}

// Rows returns the number of used rows.
func (s *Sheet) Rows() int {
	if s.ws() == nil {
		return 0
	}
	return len(s.ws().SheetData.Row)
}

// Cols returns the number of used columns (maximum column across all rows).
func (s *Sheet) Cols() int {
	if s.ws() == nil {
		return 0
	}

	maxCol := 0
	for _, row := range s.ws().SheetData.Row {
		for _, cell := range row.C {
			_, col, err := ParseCellRef(cell.R)
			if err == nil && col > maxCol {
				maxCol = col
			}
		}
	}
	return maxCol
}

// SetColWidth sets the width of a column (1-based). Existing <col> entries
// covering a range of columns (min < max) are split so the target column is
// carved out with the new width while the rest of the range keeps its
// original properties; appending an overlapping entry would be ambiguous and
// is rejected by Excel (C127).
func (s *Sheet) SetColWidth(col int, width float64) error {
	if col < 1 || col > MaxCol {
		return ErrInvalidCell
	}
	s.markDirty()
	s.ensureWS()

	c := uint32(col)
	w := width
	customWidth := true

	// Find or create cols element
	if len(s.ws().Cols) == 0 {
		s.ws().Cols = append(s.ws().Cols, oxml.CT_Cols{})
	}
	s.ws().EnsureChildOrder("cols")

	// Carve the target column out of any entry covering it. The [c,c] slice of
	// a covering range inherits that range's other properties (style, hidden,
	// ...) with the new width applied; the remainder keeps its properties.
	cols := s.ws().Cols[0].Col
	rebuilt := make([]oxml.CT_Col, 0, len(cols)+2)
	placed := false
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
			target.Width = &w
			target.CustomWidth = &customWidth
			rebuilt = append(rebuilt, target)
			placed = true
		}
		if entry.Max > c {
			right := entry
			right.Min = c + 1
			rebuilt = append(rebuilt, right)
		}
	}
	if !placed {
		rebuilt = append(rebuilt, oxml.CT_Col{
			Min:         c,
			Max:         c,
			Width:       &w,
			CustomWidth: &customWidth,
		})
	}
	s.ws().Cols[0].Col = rebuilt

	return nil
}

// SetRowHeight sets the height of a row (1-based).
func (s *Sheet) SetRowHeight(row int, height float64) error {
	if row < 1 {
		return ErrInvalidCell
	}
	s.markDirty()
	s.ensureWS()

	s.ws().EnsureChildOrder("sheetData")

	r := uint32(row)
	customHeight := true

	// Find or create the row. Look rows up via rowNumberOf, not the raw r
	// attribute: a row may legally omit r (C73), and matching on the attribute
	// alone would append a duplicate row for the same row number (C230).
	for i := range s.ws().SheetData.Row {
		if rn, ok := rowNumberOf(&s.ws().SheetData.Row[i]); ok && rn == r {
			s.ws().SheetData.Row[i].Ht = &height
			s.ws().SheetData.Row[i].CustomHeight = &customHeight
			return nil
		}
	}

	s.ws().SheetData.Row = append(s.ws().SheetData.Row, oxml.CT_Row{
		R:            &r,
		Ht:           &height,
		CustomHeight: &customHeight,
	})

	return nil
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
	return fmt.Sprintf("%s%d", columnLetters(col), row)
}

// FreezePanes freezes rows and columns at the specified cell reference.
// For example, "B2" freezes row 1 and column A. The reference is
// canonicalized, so "b2" behaves like "B2". Freezing at A1 freezes nothing:
// it removes any existing pane instead of emitting a frozen pane with no
// splits, which Excel flags as invalid (C133).
func (s *Sheet) FreezePanes(cellRef string) error {
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
func (s *Sheet) SetAutoFilter(rangeRef string) error {
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
