// Package docx provides functionality for reading and writing Word documents.
// This package is currently a placeholder and will be fully implemented in a future release.
package docx

import "errors"

var (
	// ErrNotImplemented indicates the feature is not yet implemented.
	ErrNotImplemented = errors.New("docx: not implemented")

	// ErrNotDOCX indicates the file is not a valid Word document.
	ErrNotDOCX = errors.New("docx: not a valid Word document")

	// ErrInvalidParagraph indicates an invalid paragraph reference.
	ErrInvalidParagraph = errors.New("docx: invalid paragraph")

	// ErrInvalidTable indicates an invalid table reference.
	ErrInvalidTable = errors.New("docx: invalid table")
)
