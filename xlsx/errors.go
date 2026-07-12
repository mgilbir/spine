// Package xlsx reads and writes Microsoft Excel (SpreadsheetML) workbooks via
// the Open Packaging Conventions. It supports opening existing .xlsx files,
// editing cells, styles, sheets, and images, and creating workbooks from
// scratch, preserving unmodified parts byte-for-byte on round-trip.
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

	// ErrInvalidCell indicates an invalid cell reference.
	ErrInvalidCell = errors.New("xlsx: invalid cell reference")

	// ErrInvalidRange indicates an invalid cell range.
	ErrInvalidRange = errors.New("xlsx: invalid range")

	// ErrNoSheets indicates an attempt to save a workbook with no sheets,
	// which Excel does not accept (a workbook requires at least one sheet).
	ErrNoSheets = errors.New("xlsx: workbook has no sheets")
)
