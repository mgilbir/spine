// Package docx provides functionality for reading and writing Word documents.
//
// A Document may be READ from several goroutines at once. The main part is
// parsed on first use and that parse is synchronized, so concurrent readers of
// one Document are safe even when they race to be the first to touch it.
//
// MODIFYING one is not safe, and internal locking would not make it so. A
// mutating call hands back a handle — a *Paragraph, *Run, *Table — that outlives
// the call and is not covered by any lock the call took. And a read-modify-write
// spanning two calls has nothing to hold it together: Paragraphs() followed by
// indexing the result is stale the moment another goroutine adds or removes one.
// Only the caller knows where its own operation begins and ends, so confine
// modification to one goroutine or guard it with external synchronization at
// that granularity.
//
// Save, SaveBytes and SaveTo mutate shared state while serializing, so they
// count as modification: they must not run concurrently with each other, with a
// mutation, or with a read of the same Document. Distinct Document values may be
// used from different goroutines.
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
