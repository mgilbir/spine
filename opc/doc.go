// Package opc implements the Open Packaging Conventions (OPC) specification
// as defined in ECMA-376 Part 2: the zip-based container format underlying
// pptx, docx, and xlsx packages.
//
// # Reading
//
// OpenReader (a file path) and NewReader (an io.ReaderAt) parse
// [Content_Types].xml and the package relationships up front and expose the
// remaining parts for lazy reading through File.Open. Decompression is
// bounded by two package-level knobs: MaxDecompressedPartSize (per part)
// and MaxDecompressedPackageSize (total across a package). Both values are
// captured once when a Reader is constructed — changing them affects only
// Readers opened afterwards, never one that is already open — so set them
// during program setup. A Reader is safe for concurrent part reads: all of
// its Files share one decompression budget, which is synchronized
// internally.
//
// # Writing
//
// NewWriter streams parts into an io.Writer as they are added (CreatePart,
// WritePart, WritePartRelationships) and finalizes the package on Close.
// Close writes the core properties, extended properties, and package
// relationships, then — last, because earlier writes may register content
// type overrides — [Content_Types].xml. Each of these metadata writes is
// skipped when a file of the same name was already written explicitly;
// round-trip callers rely on this to copy the original metadata parts
// verbatim and still call Close. Close always closes the underlying zip
// writer, even when a metadata write fails, so the output is a structurally
// complete zip; the metadata error and any close error are joined and
// returned together.
//
// WriteRawFile writes a named file into the zip, bypassing OPC part-name
// validation (it only deduplicates, case-insensitively, against everything
// already written). It exists for files that do not follow OPC part naming
// rules — [Content_Types].xml itself — and for byte-exact preservation of
// original entries. Most files are written verbatim, so no content types are
// registered for them; the one exception is [Content_Types].xml, whose zip
// entry is deferred to Close so that content types registered after the raw
// write (e.g. by a later CreatePart) are merged into it rather than dropped —
// leaving those parts without a content-type entry (see WriteRawFile's own
// godoc). Its bytes still go out verbatim when nothing is registered afterward.
//
// A Writer must be confined to a single goroutine.
package opc
