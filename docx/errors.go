// Package docx provides functionality for reading and writing Word documents.
//
// A Document is not safe for concurrent use. A single Document, and the
// paragraphs, runs, and tables reached through it, must be confined to one
// goroutine, or all access must be guarded by external synchronization. In
// particular Save, SaveBytes, and SaveTo mutate shared state while serializing,
// so they must not run concurrently with each other or with any mutation of the
// same Document. Distinct Document values may be used from different goroutines.
package docx

import "errors"

var (
	// ErrNotDOCX indicates the file is not a valid Word document.
	ErrNotDOCX = errors.New("docx: not a valid Word document")
)
