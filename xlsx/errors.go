// Package xlsx reads and writes Microsoft Excel (SpreadsheetML) workbooks via
// the Open Packaging Conventions. It supports opening existing .xlsx files,
// editing cells, styles, sheets, and images, and creating workbooks from
// scratch, preserving unmodified parts byte-for-byte on round-trip.
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
)
