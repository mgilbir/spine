package xlsx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Sheet represents a worksheet in an Excel workbook.
type Sheet struct {
	workbook  *Workbook
	name      string
	index     int
	partName  string
	relID     string
	worksheet *oxml.CT_Worksheet
	dirty     bool
}

// Name returns the sheet name.
func (s *Sheet) Name() string {
	return s.name
}

// SetName sets the sheet name.
func (s *Sheet) SetName(name string) {
	s.name = name
	// Update the workbook model if within bounds
	if s.workbook != nil && s.index < len(s.workbook.workbook.Sheets.Sheet) {
		s.workbook.workbook.Sheets.Sheet[s.index].Name = name
	}
	s.markDirty()
}

// Index returns the sheet index within the workbook.
func (s *Sheet) Index() int {
	return s.index
}

// Cell returns the cell at the specified reference (e.g., "A1").
// If the cell doesn't exist in the worksheet data, it is created.
func (s *Sheet) Cell(ref string) (*Cell, error) {
	if s.worksheet == nil {
		s.worksheet = &oxml.CT_Worksheet{
			SheetData: oxml.CT_SheetData{},
		}
	}

	// Parse the reference to get row and column
	row, _, err := ParseCellRef(ref)
	if err != nil {
		return nil, err
	}

	// Find or create the row
	var targetRow *oxml.CT_Row
	for i := range s.worksheet.SheetData.Row {
		if s.worksheet.SheetData.Row[i].R != nil && *s.worksheet.SheetData.Row[i].R == uint32(row) {
			targetRow = &s.worksheet.SheetData.Row[i]
			break
		}
	}
	if targetRow == nil {
		r := uint32(row)
		s.worksheet.SheetData.Row = append(s.worksheet.SheetData.Row, oxml.CT_Row{R: &r})
		targetRow = &s.worksheet.SheetData.Row[len(s.worksheet.SheetData.Row)-1]
	}

	// Find or create the cell
	ref = strings.ToUpper(ref)
	for i := range targetRow.C {
		if strings.EqualFold(targetRow.C[i].R, ref) {
			return &Cell{
				sheet: s,
				cell:  &targetRow.C[i],
			}, nil
		}
	}

	targetRow.C = append(targetRow.C, oxml.CT_Cell{R: ref})
	return &Cell{
		sheet: s,
		cell:  &targetRow.C[len(targetRow.C)-1],
	}, nil
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
	if s.worksheet == nil {
		return "", nil
	}

	ref = strings.ToUpper(ref)
	row, _, err := ParseCellRef(ref)
	if err != nil {
		return "", err
	}

	for i := range s.worksheet.SheetData.Row {
		r := &s.worksheet.SheetData.Row[i]
		if r.R != nil && *r.R == uint32(row) {
			for j := range r.C {
				if strings.EqualFold(r.C[j].R, ref) {
					c := &Cell{sheet: s, cell: &r.C[j]}
					return c.String(), nil
				}
			}
		}
	}

	return "", nil
}

// Rows returns the number of used rows.
func (s *Sheet) Rows() int {
	if s.worksheet == nil {
		return 0
	}
	return len(s.worksheet.SheetData.Row)
}

// Cols returns the number of used columns (maximum column across all rows).
func (s *Sheet) Cols() int {
	if s.worksheet == nil {
		return 0
	}

	maxCol := 0
	for _, row := range s.worksheet.SheetData.Row {
		for _, cell := range row.C {
			_, col, err := ParseCellRef(cell.R)
			if err == nil && col > maxCol {
				maxCol = col
			}
		}
	}
	return maxCol
}

// SetColWidth sets the width of a column (1-based).
func (s *Sheet) SetColWidth(col int, width float64) error {
	if col < 1 {
		return ErrInvalidCell
	}
	s.markDirty()
	if s.worksheet == nil {
		s.worksheet = &oxml.CT_Worksheet{
			SheetData: oxml.CT_SheetData{},
		}
	}

	c := uint32(col)
	w := width
	customWidth := true

	// Find or create cols element
	if len(s.worksheet.Cols) == 0 {
		s.worksheet.Cols = append(s.worksheet.Cols, oxml.CT_Cols{})
	}

	// Find existing col entry or create new
	for i := range s.worksheet.Cols[0].Col {
		if s.worksheet.Cols[0].Col[i].Min == c && s.worksheet.Cols[0].Col[i].Max == c {
			s.worksheet.Cols[0].Col[i].Width = &w
			s.worksheet.Cols[0].Col[i].CustomWidth = &customWidth
			return nil
		}
	}

	s.worksheet.Cols[0].Col = append(s.worksheet.Cols[0].Col, oxml.CT_Col{
		Min:         c,
		Max:         c,
		Width:       &w,
		CustomWidth: &customWidth,
	})

	return nil
}

// SetRowHeight sets the height of a row (1-based).
func (s *Sheet) SetRowHeight(row int, height float64) error {
	if row < 1 {
		return ErrInvalidCell
	}
	s.markDirty()
	if s.worksheet == nil {
		s.worksheet = &oxml.CT_Worksheet{
			SheetData: oxml.CT_SheetData{},
		}
	}

	r := uint32(row)
	customHeight := true

	// Find or create the row
	for i := range s.worksheet.SheetData.Row {
		if s.worksheet.SheetData.Row[i].R != nil && *s.worksheet.SheetData.Row[i].R == r {
			s.worksheet.SheetData.Row[i].Ht = &height
			s.worksheet.SheetData.Row[i].CustomHeight = &customHeight
			return nil
		}
	}

	s.worksheet.SheetData.Row = append(s.worksheet.SheetData.Row, oxml.CT_Row{
		R:            &r,
		Ht:           &height,
		CustomHeight: &customHeight,
	})

	return nil
}

// MergeCells merges a range of cells.
func (s *Sheet) MergeCells(startRef, endRef string) error {
	s.markDirty()
	if s.worksheet == nil {
		s.worksheet = &oxml.CT_Worksheet{
			SheetData: oxml.CT_SheetData{},
		}
	}

	ref := strings.ToUpper(startRef) + ":" + strings.ToUpper(endRef)

	if s.worksheet.MergeCells == nil {
		s.worksheet.MergeCells = &oxml.CT_MergeCells{}
	}

	s.worksheet.MergeCells.MergeCell = append(s.worksheet.MergeCells.MergeCell, oxml.CT_MergeCell{Ref: ref})
	count := uint32(len(s.worksheet.MergeCells.MergeCell))
	s.worksheet.MergeCells.Count = &count

	return nil
}

// UnmergeCells unmerges a range of cells.
func (s *Sheet) UnmergeCells(startRef, endRef string) error {
	if s.worksheet == nil || s.worksheet.MergeCells == nil {
		return nil
	}

	ref := strings.ToUpper(startRef) + ":" + strings.ToUpper(endRef)

	for i, mc := range s.worksheet.MergeCells.MergeCell {
		if strings.EqualFold(mc.Ref, ref) {
			s.markDirty()
			s.worksheet.MergeCells.MergeCell = append(
				s.worksheet.MergeCells.MergeCell[:i],
				s.worksheet.MergeCells.MergeCell[i+1:]...,
			)
			count := uint32(len(s.worksheet.MergeCells.MergeCell))
			s.worksheet.MergeCells.Count = &count
			if len(s.worksheet.MergeCells.MergeCell) == 0 {
				s.worksheet.MergeCells = nil
			}
			return nil
		}
	}

	return nil
}

// CellRef converts row and column indices (1-based) to a cell reference.
func CellRef(row, col int) (string, error) {
	if row < 1 || col < 1 {
		return "", ErrInvalidCell
	}

	// Convert column to letter(s)
	colStr := ""
	c := col
	for c > 0 {
		c--
		colStr = string(rune('A'+c%26)) + colStr
		c /= 26
	}

	return colStr + strconv.Itoa(row), nil
}

// columnLetters converts a 1-based column number to column letters.
func columnLetters(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

// FormatCellRef creates a cell reference from row and column numbers (1-based).
func FormatCellRef(row, col int) string {
	return fmt.Sprintf("%s%d", columnLetters(col), row)
}

// FreezePanes freezes rows and columns at the specified cell reference.
// For example, "B2" freezes row 1 and column A.
func (s *Sheet) FreezePanes(cellRef string) error {
	row, col, err := ParseCellRef(cellRef)
	if err != nil {
		return err
	}
	s.markDirty()

	s.ensureWorksheet()
	sv := s.ensureSheetView()

	xSplit := float64(col - 1)
	ySplit := float64(row - 1)

	sv.Pane = &oxml.CT_Pane{
		TopLeftCell: strings.ToUpper(cellRef),
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

// UnfreezePanes removes any frozen panes from the sheet.
func (s *Sheet) UnfreezePanes() {
	if s.worksheet == nil || s.worksheet.SheetViews == nil {
		return
	}
	if len(s.worksheet.SheetViews.SheetView) > 0 {
		s.markDirty()
		s.worksheet.SheetViews.SheetView[0].Pane = nil
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
	if s.worksheet.SheetPr == nil {
		s.worksheet.SheetPr = &oxml.CT_SheetPr{}
	}
	s.worksheet.SheetPr.TabColor = &oxml.CT_Color{
		Rgb: hexColor,
	}
}

// SetAutoFilter sets an auto-filter on the specified range (e.g., "A1:F1").
func (s *Sheet) SetAutoFilter(rangeRef string) error {
	s.markDirty()
	s.ensureWorksheet()
	s.worksheet.AutoFilter = &oxml.CT_AutoFilter{
		Ref: strings.ToUpper(rangeRef),
	}
	return nil
}

// RemoveAutoFilter removes the auto-filter from the sheet.
func (s *Sheet) RemoveAutoFilter() {
	if s.worksheet != nil {
		s.markDirty()
		s.worksheet.AutoFilter = nil
	}
}

// DataValidation represents a data validation rule.
type DataValidation struct {
	Range        string // cell range (e.g., "B2:B100")
	Type         string // "list", "whole", "decimal", "date", "textLength", "custom"
	Operator     string // "between", "lessThan", "equal", etc.
	Formula1     string
	Formula2     string
	AllowBlank   bool
	ShowDropDown bool
	ErrorTitle   string
	ErrorMessage string
}

// AddDataValidation adds a data validation rule to the sheet.
func (s *Sheet) AddDataValidation(dv DataValidation) error {
	s.markDirty()
	s.ensureWorksheet()
	if s.worksheet.DataValidations == nil {
		s.worksheet.DataValidations = &oxml.CT_DataValidations{}
	}

	v := oxml.CT_DataValidation{
		Sqref:      strings.ToUpper(dv.Range),
		Type:       dv.Type,
		Operator:   dv.Operator,
		ErrorTitle: dv.ErrorTitle,
		Error:      dv.ErrorMessage,
	}

	if dv.AllowBlank {
		v.AllowBlank = &dv.AllowBlank
	}
	if dv.ShowDropDown {
		// Note: In OOXML, showDropDown=false means show dropdown (counterintuitive)
		// But our public API uses intuitive semantics
		show := false
		v.ShowDropDown = &show
	}
	if dv.Formula1 != "" {
		v.Formula1 = &dv.Formula1
	}
	if dv.Formula2 != "" {
		v.Formula2 = &dv.Formula2
	}

	s.worksheet.DataValidations.DataValidation = append(s.worksheet.DataValidations.DataValidation, v)
	count := uint32(len(s.worksheet.DataValidations.DataValidation))
	s.worksheet.DataValidations.Count = &count

	return nil
}

func (s *Sheet) ensureWorksheet() {
	if s.worksheet == nil {
		s.worksheet = &oxml.CT_Worksheet{
			SheetData: oxml.CT_SheetData{},
		}
	}
}

func (s *Sheet) markDirty() {
	if s != nil {
		s.dirty = true
	}
}

func (s *Sheet) ensureSheetView() *oxml.CT_SheetView {
	if s.worksheet.SheetViews == nil {
		s.worksheet.SheetViews = &oxml.CT_SheetViews{}
	}
	if len(s.worksheet.SheetViews.SheetView) == 0 {
		s.worksheet.SheetViews.SheetView = append(s.worksheet.SheetViews.SheetView, oxml.CT_SheetView{})
	}
	return &s.worksheet.SheetViews.SheetView[0]
}
