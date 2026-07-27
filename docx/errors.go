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

	// ErrRevisionStale is returned by Revision.Accept and Revision.Reject when
	// the revision's content is no longer where it was enumerated — typically
	// because an earlier Accept or Reject rebuilt the container it lived in, as
	// Document.Revisions' godoc warns. The document is left unchanged; re-read
	// Revisions and retry.
	ErrRevisionStale = errors.New("docx: revision no longer resolvable")
)
