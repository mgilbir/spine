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

	// ErrStreamWriterClosed indicates a stream writer was used after flush.
	ErrStreamWriterClosed = errors.New("xlsx: stream writer is closed")

	// ErrStreamWriterOrder indicates rows were written out of order.
	ErrStreamWriterOrder = errors.New("xlsx: rows must be written in strictly increasing order")

	// ErrStreamWriterMixedMode indicates streaming and random-access writes were mixed.
	ErrStreamWriterMixedMode = errors.New("xlsx: cannot mix streaming and random-access writes on the same sheet")

	// ErrStreamWriterUnsupported indicates streaming is not available for this workbook state.
	ErrStreamWriterUnsupported = errors.New("xlsx: streaming is only supported for new workbooks")
)
