package xlsx

// Sheet represents a worksheet in an Excel workbook.
type Sheet struct {
	workbook *Workbook
	name     string
	index    int
	cells    map[string]*Cell
}

// Name returns the sheet name.
func (s *Sheet) Name() string {
	return s.name
}

// SetName sets the sheet name.
func (s *Sheet) SetName(name string) {
	s.name = name
}

// Index returns the sheet index within the workbook.
func (s *Sheet) Index() int {
	return s.index
}

// Cell returns the cell at the specified reference (e.g., "A1").
func (s *Sheet) Cell(ref string) (*Cell, error) {
	if s.cells == nil {
		s.cells = make(map[string]*Cell)
	}
	cell, ok := s.cells[ref]
	if !ok {
		cell = &Cell{
			sheet: s,
			ref:   ref,
		}
		s.cells[ref] = cell
	}
	return cell, nil
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

// GetCellValue returns the value of a cell.
func (s *Sheet) GetCellValue(ref string) (interface{}, error) {
	cell, err := s.Cell(ref)
	if err != nil {
		return nil, err
	}
	return cell.Value(), nil
}

// Rows returns the number of used rows.
func (s *Sheet) Rows() int {
	// Placeholder implementation
	return 0
}

// Cols returns the number of used columns.
func (s *Sheet) Cols() int {
	// Placeholder implementation
	return 0
}

// SetColWidth sets the width of a column.
func (s *Sheet) SetColWidth(col int, width float64) error {
	return ErrNotImplemented
}

// SetRowHeight sets the height of a row.
func (s *Sheet) SetRowHeight(row int, height float64) error {
	return ErrNotImplemented
}

// MergeCells merges a range of cells.
func (s *Sheet) MergeCells(startRef, endRef string) error {
	return ErrNotImplemented
}

// UnmergeCells unmerges a range of cells.
func (s *Sheet) UnmergeCells(startRef, endRef string) error {
	return ErrNotImplemented
}

// CellRef converts row and column indices (1-based) to a cell reference.
func CellRef(row, col int) (string, error) {
	if row < 1 || col < 1 {
		return "", ErrInvalidCell
	}

	// Convert column to letter(s)
	colStr := ""
	for col > 0 {
		col--
		colStr = string(rune('A'+col%26)) + colStr
		col /= 26
	}

	return colStr + string(rune('0'+row)), nil
}
