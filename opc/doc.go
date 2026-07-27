// Package opc implements the Open Packaging Conventions (OPC) specification
// as defined in ECMA-376 Part 2: the zip-based container format underlying
// pptx, docx, and xlsx packages.
//
// # Reading
//
// OpenReader (a file path) and NewReader (an io.ReaderAt) parse
// [Content_Types].xml and the package relationships up front and expose the
// remaining parts for lazy reading through File.Open. Resource use is bounded
// by three package-level knobs: MaxDecompressedPartSize (per part),
// MaxDecompressedPackageSize (total across a package, counting each part at
// most once however often it is read) and MaxPackageEntries (how many zip
// entries the archive may contain). All three are captured once when a Reader
// is constructed — changing them affects only Readers opened afterwards, never
// one that is already open — so set them during program setup, or override
// them for a single Reader through ReaderOptions. A Reader is safe for
// concurrent part reads: all of its Files share one decompression budget,
// which is synchronized internally.
//
// Entry names are canonicalized on open: backslash separators, "." and ".."
// segments and empty segments are resolved, so every part is reachable through
// GetFile under the same normalization NormalizePartName applies to the query.
// Two entries collapsing onto one part name is malformed; the first wins and
// the rest are reported in Reader.DuplicateEntries.
//
// # Writing
//
// NewWriter streams parts into an io.Writer as they are added (CreatePart,
// WritePart, WritePartRelationships) and finalizes the package on Close.
// Close writes the core properties, the extended properties, the custom
// properties and the package relationships, then — last, because earlier
// writes may register content type overrides — [Content_Types].xml. Each of
// these metadata writes is skipped when a file of the same name was already
// written explicitly; round-trip callers rely on this to copy the original
// metadata parts verbatim and still call Close. Close always closes the
// underlying zip writer, even when a metadata write fails, so the output is a
// structurally complete zip; the metadata error and any close error are
// joined and returned together.
//
// The three property parts are reconciled as a group before any of them is
// emitted: each one's package relationship and content-type override are
// settled first, so the package can never carry a metadata part that nothing
// points at. A relationship of the same type already present in Relationships
// suppresses the one Close would add, and identifiers for the ones it does add
// come from the current contents of that slice, so assigning it directly
// cannot produce colliding rIds. Because the package relationships part is
// streamed out when it is written, a caller preserving it verbatim must set
// the Properties/ExtendedProperties/CustomProperties fields *before* writing
// it: the writer then injects any missing metadata relationship into those
// bytes, leaving every other byte in place. Setting them afterwards makes
// Close fail rather than emit an unreachable part.
//
// WriteRawFile writes a named file into the zip, bypassing the OPC part-name
// grammar (it still rejects structurally unsafe names — empty, "." and ".."
// segments — and deduplicates case-insensitively against everything already
// written). It exists for files that do not follow OPC part naming
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
