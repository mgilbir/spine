package xlsx

import (
	"github.com/mgilbir/spine/opc"
)

// Workbook represents an Excel workbook.
type Workbook struct {
	// Properties contains the document properties.
	Properties opc.CoreProperties

	sheets []*Sheet
}

// Open opens an Excel workbook from a file path.
// This function is not yet implemented.
func Open(path string) (*Workbook, error) {
	return nil, ErrNotImplemented
}

// Create creates a new, empty workbook.
// This function is not yet implemented.
func Create() *Workbook {
	return &Workbook{
		sheets: make([]*Sheet, 0),
	}
}

// Save saves the workbook to a file.
// This function is not yet implemented.
func (w *Workbook) Save(path string) error {
	return ErrNotImplemented
}

// Close closes the workbook and releases resources.
func (w *Workbook) Close() error {
	return nil
}

// Sheets returns all sheets in the workbook.
func (w *Workbook) Sheets() []*Sheet {
	return w.sheets
}

// SheetCount returns the number of sheets.
func (w *Workbook) SheetCount() int {
	return len(w.sheets)
}

// Sheet returns the sheet at the specified index (0-based).
func (w *Workbook) Sheet(index int) (*Sheet, error) {
	if index < 0 || index >= len(w.sheets) {
		return nil, ErrSheetIndex
	}
	return w.sheets[index], nil
}

// SheetByName returns the sheet with the specified name.
func (w *Workbook) SheetByName(name string) (*Sheet, error) {
	for _, sheet := range w.sheets {
		if sheet.name == name {
			return sheet, nil
		}
	}
	return nil, ErrSheetNotFound
}

// AddSheet adds a new sheet to the workbook.
func (w *Workbook) AddSheet(name string) *Sheet {
	sheet := &Sheet{
		workbook: w,
		name:     name,
		index:    len(w.sheets),
	}
	w.sheets = append(w.sheets, sheet)
	return sheet
}

// DeleteSheet removes the sheet at the specified index.
func (w *Workbook) DeleteSheet(index int) error {
	if index < 0 || index >= len(w.sheets) {
		return ErrSheetIndex
	}
	w.sheets = append(w.sheets[:index], w.sheets[index+1:]...)
	for i := index; i < len(w.sheets); i++ {
		w.sheets[i].index = i
	}
	return nil
}

// ActiveSheet returns the currently active sheet.
func (w *Workbook) ActiveSheet() *Sheet {
	if len(w.sheets) == 0 {
		return nil
	}
	return w.sheets[0]
}

// SetActiveSheet sets the active sheet by index.
func (w *Workbook) SetActiveSheet(index int) error {
	if index < 0 || index >= len(w.sheets) {
		return ErrSheetIndex
	}
	// Placeholder: would set the active sheet
	return nil
}
