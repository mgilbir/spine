// Package docx provides functionality for reading and writing Word documents.
package docx

import "errors"

var (
	// ErrNotDOCX indicates the file is not a valid Word document.
	ErrNotDOCX = errors.New("docx: not a valid Word document")
)
