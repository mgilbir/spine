// Package xlsx provides functionality for reading and writing Excel spreadsheets.
// This package is currently a placeholder and will be fully implemented in a future release.
package xlsx

import "errors"

var (
	// ErrNotImplemented indicates the feature is not yet implemented.
	ErrNotImplemented = errors.New("xlsx: not implemented")

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
