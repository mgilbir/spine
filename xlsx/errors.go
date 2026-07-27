// Package xlsx reads and writes Microsoft Excel (SpreadsheetML) workbooks via
// the Open Packaging Conventions. It supports opening existing .xlsx files,
// editing cells, styles, sheets, and images, and creating workbooks from
// scratch, preserving unmodified parts byte-for-byte on round-trip.
//
// Pivot slicers and timelines are read-only: Sheet.Slicers/Workbook.Slicers and
// Sheet.Timelines/Workbook.Timelines expose the slicers and timelines of an
// opened workbook (name, caption, source pivot field and controlled pivot
// tables), and their definition/cache parts plus the worksheet/workbook
// extension references round-trip byte-for-byte. Creating them is not yet
// supported: a slicer or timeline is an on-sheet drawing whose creation also
// requires injecting relationship-bearing x14/x15 extension lists into the
// shared workbook and worksheet parts at save time.
//
// The feature surface is add-and-read, not add-remove. Comments, conditional
// formats, data validations, tables, images, charts, pivot tables, scenarios
// and OLE objects can be added and read back, but only sparkline groups have a
// removal API (SparklineGroup.Delete); the sheet-level Remove*/Clear* methods
// cover the auto-filter, its column predicates, the sort state, sheet
// protection, freeze panes and the print area/titles. A replace-style edit of
// anything else therefore accretes rather than replaces, so rebuild the sheet
// instead of editing in place when a feature must go away.
//
// A Workbook is not safe for concurrent use. A single Workbook, and the sheets
// and cells reached through it, must be confined to one goroutine, or all access
// must be guarded by external synchronization. In particular Save, SaveBytes,
// and SaveTo mutate shared state while serializing, so they must not run
// concurrently with each other or with any mutation of the same Workbook.
// Distinct Workbook values may be used from different goroutines.
package xlsx

import "errors"

var (
	// ErrNotXLSX indicates the file is not a valid Excel file.
	ErrNotXLSX = errors.New("xlsx: not a valid Excel file")

	// ErrSheetNotFound indicates the requested sheet does not exist.
	ErrSheetNotFound = errors.New("xlsx: sheet not found")

	// ErrSheetIndex indicates an invalid sheet index.
	ErrSheetIndex = errors.New("xlsx: sheet index out of range")

	// ErrDuplicateSheetName indicates a sheet with the requested name (compared
	// case-insensitively, as Excel does) already exists in the workbook.
	// AddSheet reports it rather than quietly renaming the new sheet; derive a
	// free name with Workbook.UniqueSheetName when a suffix is what you want.
	ErrDuplicateSheetName = errors.New("xlsx: a sheet with that name already exists")

	// ErrInvalidCell indicates an invalid cell reference.
	ErrInvalidCell = errors.New("xlsx: invalid cell reference")

	// ErrInvalidRange indicates an invalid cell range.
	ErrInvalidRange = errors.New("xlsx: invalid range")

	// ErrNoSheets indicates an attempt to save a workbook with no sheets,
	// which Excel does not accept (a workbook requires at least one sheet).
	ErrNoSheets = errors.New("xlsx: workbook has no sheets")

	// ErrNoWorkbook indicates a sheet is not attached to a workbook, so an
	// operation that stores state at the workbook level (such as a print area or
	// print titles, which live in workbook-scoped defined names) cannot proceed.
	ErrNoWorkbook = errors.New("xlsx: sheet is not attached to a workbook")

	// ErrNotWorksheet indicates a worksheet operation (such as writing a cell) was
	// attempted on a non-worksheet sheet — a chartsheet, dialogsheet or
	// macrosheet. Such a sheet is preserved opaquely and has no worksheet cell
	// grid to mutate.
	ErrNotWorksheet = errors.New("xlsx: sheet is not a worksheet")
)
