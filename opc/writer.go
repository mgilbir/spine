package opc

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"strings"
)

// CompressionOption specifies how parts should be compressed.
type CompressionOption int

const (
	// CompressionNone disables compression.
	CompressionNone CompressionOption = iota

	// CompressionDeflate uses deflate compression (default).
	CompressionDeflate
)

// Writer creates OPC packages.
type Writer struct {
	// Properties contains the core properties to write.
	Properties *CoreProperties

	// ExtendedProperties contains the extended properties to write.
	ExtendedProperties *ExtendedProperties

	// CustomProperties contains the user-defined properties to write. When set
	// and not already emitted as a preserved raw part, Close writes
	// docProps/custom.xml, registers its content-type override, and adds the
	// package custom-properties relationship.
	CustomProperties *CustomProperties

	// Relationships contains package-level relationships.
	Relationships []*Relationship

	// ContentTypes manages content types for parts.
	ContentTypes *ContentTypes

	zipWriter *zip.Writer
	output    io.Writer
	parts     map[string]bool
	nextRelID int
	closed    bool

	// rawContentTypes holds [Content_Types].xml bytes handed to WriteRawFile.
	// The entry is deferred to Close so content types registered after the raw
	// write (CreatePart overrides, metadata-part registrations) can still be
	// merged in instead of silently never being serialized (C46).
	rawContentTypes []byte

	// ctAtRawWrite snapshots ContentTypes at the moment the raw
	// [Content_Types].xml was handed in. At Close, entries present in
	// ContentTypes but not in this snapshot were registered after the raw
	// write and are merged into the raw bytes when they are not already
	// covered by them.
	ctAtRawWrite *ContentTypes
}

// NewWriter creates a new Writer that writes to the provided io.Writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		zipWriter:     zip.NewWriter(w),
		output:        w,
		ContentTypes:  NewContentTypes(),
		parts:         make(map[string]bool),
		nextRelID:     1,
		Relationships: make([]*Relationship, 0),
	}
}

// CreatePart creates a new part in the package.
//
// The returned io.Writer is only valid until the next call that adds an entry
// to the package (CreatePart, WritePart, WritePreservedPart, WriteRawFile,
// WritePartRelationships, WriteDirectoryEntries) or finalizes it (Close,
// Abort): the underlying zip stream is sequential, so each new entry
// invalidates the previous part's writer. Writing to an invalidated writer
// does not interleave into the next part; it fails with an error from
// archive/zip.
func (w *Writer) CreatePart(name, contentType string, compression CompressionOption) (io.Writer, error) {
	return w.createPart(name, contentType, compression, false)
}

// createPart is the shared implementation behind CreatePart and
// WritePreservedPart. preserved marks parts carried over verbatim from a
// source package, which are exempt from the missing-content-type check.
func (w *Writer) createPart(name, contentType string, compression CompressionOption, preserved bool) (io.Writer, error) {
	if w.closed {
		return nil, ErrPackageClosed
	}

	// New parts must satisfy the full OPC part-name grammar; parts preserved
	// verbatim from a source package only need the structural rules, since
	// wild packages carry entries (e.g. /[trash]/0000.dat) whose names violate
	// the grammar and must still round-trip.
	if preserved {
		if err := validatePartNameShape(name); err != nil {
			return nil, err
		}
	} else if err := ValidatePartName(name); err != nil {
		return nil, err
	}

	normalizedName := NormalizePartName(name)
	if w.parts[strings.ToLower(normalizedName)] {
		return nil, ErrDuplicatePart
	}

	// Every new part must resolve to a content type — either the explicit one
	// passed here (emitted as an Override) or a Default mapping covering its
	// extension. A part with neither would silently be absent from
	// [Content_Types].xml, making the package OPC-invalid (Office shows a
	// repair prompt). Checked at part-creation time rather than at Close:
	// Defaults are registered before parts are written in every supported
	// flow, and failing here names the offending call site.
	//
	// Preserved parts are exempt: real-world packages contain entries (e.g.
	// /[trash]/0000.dat from some producers) with no [Content_Types].xml
	// entry at all. The source package already lacked the mapping, so
	// requiring one on write would both break round-trip fidelity and fail
	// the save of an otherwise valid package.
	if !preserved && contentType == "" && w.ContentTypes.GetContentType(normalizedName) == "" {
		return nil, fmt.Errorf("%w: part %q has no content type and no default mapping covers its extension", ErrInvalidContentType, normalizedName)
	}

	// Remove leading slash for zip file name
	zipName := strings.TrimPrefix(normalizedName, "/")

	header := &zip.FileHeader{
		Name:   zipName,
		Method: zip.Deflate,
	}

	if compression == CompressionNone {
		header.Method = zip.Store
	}

	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return nil, err
	}

	w.parts[strings.ToLower(normalizedName)] = true

	// Only add a content type override if a non-empty type isn't already covered
	// by a default extension mapping. This avoids bloating [Content_Types].xml
	// with redundant entries and, critically, never registers an empty-string
	// override (which would be a schema-invalid <Override ContentType=""/>); a
	// part with no declared type simply relies on default extension mapping.
	if contentType != "" && w.ContentTypes.GetContentType(normalizedName) != contentType {
		w.ContentTypes.SetOverride(normalizedName, contentType)
	}

	return writer, nil
}

// WritePart writes a complete part to the package.
func (w *Writer) WritePart(name, contentType string, data []byte) error {
	writer, err := w.CreatePart(name, contentType, CompressionDeflate)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	return err
}

// WritePreservedPart writes a part carried over unchanged from a source
// package. It behaves like WritePart except that a part whose content type is
// empty and whose extension has no Default mapping is written as-is instead of
// being rejected: the source package legitimately lacked a content-type entry
// for it, and preserving the part must preserve that status exactly (no entry
// in the source → none written → no error). New parts must go through
// WritePart/CreatePart, which keep the strict check.
func (w *Writer) WritePreservedPart(name, contentType string, data []byte) error {
	writer, err := w.createPart(name, contentType, CompressionDeflate, true)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	return err
}

// WriteDirectoryEntries writes zero-length zip directory entries (names ending
// in "/"). OPC consumers ignore directory entries, but re-emitting the ones a
// source archive carried (Reader.DirectoryEntries) keeps a round-tripped
// package's entry listing faithful to its producer. Names that do not end in
// "/" or that were already written are skipped rather than rejected, so a
// caller can replay a captured list verbatim.
func (w *Writer) WriteDirectoryEntries(names []string) error {
	if w.closed {
		return ErrPackageClosed
	}
	for _, name := range names {
		name = strings.TrimPrefix(name, "/")
		if name == "" || !strings.HasSuffix(name, "/") {
			continue
		}
		key := strings.ToLower("/" + name)
		if w.parts[key] {
			continue
		}
		header := &zip.FileHeader{
			Name:   name,
			Method: zip.Store,
		}
		if _, err := w.zipWriter.CreateHeader(header); err != nil {
			return err
		}
		w.parts[key] = true
	}
	return nil
}

// WriteRawFile writes a raw file to the package without part name validation.
// This is used for special files like [Content_Types].xml that don't follow
// OPC part naming rules.
//
// A raw [Content_Types].xml is special-cased: its zip entry is emitted during
// Close rather than immediately, so content types registered afterwards (e.g.
// by CreatePart for a new part) are merged into the raw bytes instead of
// accumulating in memory and never being serialized — which would leave the
// new parts without a content-type entry, a silently OPC-invalid package
// (C46). When nothing was registered after the raw write, the bytes are
// emitted verbatim.
func (w *Writer) WriteRawFile(name string, data []byte) error {
	if w.closed {
		return ErrPackageClosed
	}

	// Track the file to prevent duplicates
	key := strings.ToLower("/" + strings.TrimPrefix(name, "/"))
	if w.parts[key] {
		return ErrDuplicatePart
	}

	if key == "/[content_types].xml" {
		// Defer to Close (see above). Copy the bytes so a caller reusing the
		// buffer cannot corrupt the deferred write.
		w.rawContentTypes = append([]byte(nil), data...)
		w.ctAtRawWrite = w.ContentTypes.Clone()
		w.parts[key] = true
		return nil
	}

	header := &zip.FileHeader{
		// Zip entry names never carry a leading slash; writing one verbatim
		// would produce an absolute-path entry many consumers reject.
		Name:   strings.TrimPrefix(name, "/"),
		Method: zip.Deflate,
	}

	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	if err != nil {
		return err
	}

	w.parts[key] = true
	return nil
}

// AddRelationship adds a package-level relationship and returns it. It fails
// with ErrPackageClosed after Close: the package-level .rels part is emitted
// during Close, so a relationship added later could never be written and
// would silently dangle.
func (w *Writer) AddRelationship(relType, target string, targetMode TargetMode) (*Relationship, error) {
	if w.closed {
		return nil, ErrPackageClosed
	}
	return w.addRelationship(relType, target, targetMode), nil
}

// addRelationship is the internal, no-closed-check variant used during Close
// itself, after closed is set but before the .rels part has been written.
func (w *Writer) addRelationship(relType, target string, targetMode TargetMode) *Relationship {
	rel := &Relationship{
		ID:         fmt.Sprintf("rId%d", w.nextRelID),
		Type:       relType,
		Target:     target,
		TargetMode: targetMode,
	}
	w.nextRelID++
	w.Relationships = append(w.Relationships, rel)
	return rel
}

// writeContentTypes writes the [Content_Types].xml file.
func (w *Writer) writeContentTypes() error {
	// A raw-written [Content_Types].xml was deferred to this point: emit it
	// now, merging in any content types registered after the raw write (C46).
	if w.rawContentTypes != nil {
		data, err := w.mergedRawContentTypes()
		if err != nil {
			return err
		}
		return w.writeMetadataEntry("[Content_Types].xml", data)
	}

	// Skip if already written (for round-trip support)
	if w.parts[strings.ToLower("/[content_types].xml")] {
		return nil
	}

	data, err := w.ContentTypes.Marshal()
	if err != nil {
		return err
	}
	return w.writeMetadataEntry("[Content_Types].xml", data)
}

// mergedRawContentTypes returns the raw [Content_Types].xml bytes handed to
// WriteRawFile, with content types registered after that raw write merged in.
// The raw bytes are returned verbatim when nothing new was registered or when
// every late registration is already covered by them; otherwise the raw file
// is parsed, extended, and re-marshaled — the parse captures the source
// formatting (prolog, entry order, attribute order, self-closing style), so
// the original entries are reproduced byte-for-byte and the new ones are
// appended.
func (w *Writer) mergedRawContentTypes() ([]byte, error) {
	// Collect registrations made after the raw write: entries in the live
	// ContentTypes that the snapshot taken at WriteRawFile time did not carry.
	var newDefaults, newOverrides []string
	for _, ext := range w.ContentTypes.orderedDefaults() {
		if prev, ok := w.ctAtRawWrite.Defaults[ext]; ok && prev == w.ContentTypes.Defaults[ext] {
			continue
		}
		newDefaults = append(newDefaults, ext)
	}
	for _, name := range w.ContentTypes.orderedOverrides() {
		if prev, ok := w.ctAtRawWrite.Overrides[name]; ok && prev == w.ContentTypes.Overrides[name] {
			continue
		}
		newOverrides = append(newOverrides, name)
	}
	if len(newDefaults) == 0 && len(newOverrides) == 0 {
		return w.rawContentTypes, nil
	}

	parsed, err := UnmarshalContentTypes(w.rawContentTypes)
	if err != nil {
		// The raw bytes need additions but cannot be parsed: failing loudly
		// beats emitting a package whose new parts have no content type.
		return nil, fmt.Errorf("opc: content types were registered after WriteRawFile(\"[Content_Types].xml\") but the raw bytes do not parse, so they cannot be merged (first unmergeable: %s): %w",
			firstNewContentTypeEntry(newDefaults, newOverrides), err)
	}

	merged := false
	for _, ext := range newDefaults {
		ct := w.ContentTypes.Defaults[ext]
		if parsed.Defaults[ext] == ct {
			continue // the raw file already carries it
		}
		parsed.SetDefault(w.ContentTypes.displayExtension(ext), ct)
		merged = true
	}
	for _, name := range newOverrides {
		ct := w.ContentTypes.Overrides[name]
		if parsed.GetContentType(name) == ct {
			continue // already covered by a raw override or default
		}
		parsed.SetOverride(name, ct)
		merged = true
	}
	if !merged {
		return w.rawContentTypes, nil
	}
	return parsed.Marshal()
}

// firstNewContentTypeEntry names one late-registered entry for error messages.
func firstNewContentTypeEntry(newDefaults, newOverrides []string) string {
	if len(newOverrides) > 0 {
		return "override for " + newOverrides[0]
	}
	if len(newDefaults) > 0 {
		return "default for extension ." + newDefaults[0]
	}
	return "none"
}

// writeMetadataEntry emits one deflate-compressed zip entry for a package
// metadata file written during Close.
func (w *Writer) writeMetadataEntry(name string, data []byte) error {
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

// writeRelationships writes the package-level relationships file.
func (w *Writer) writeRelationships() error {
	// Skip if already written (for round-trip support)
	if w.parts[strings.ToLower("/_rels/.rels")] {
		return nil
	}

	if len(w.Relationships) == 0 {
		return nil
	}

	data, err := MarshalRelationships(w.Relationships)
	if err != nil {
		return err
	}

	header := &zip.FileHeader{
		Name:   "_rels/.rels",
		Method: zip.Deflate,
	}

	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	return err
}

// writeCoreProperties writes the core properties if set.
func (w *Writer) writeCoreProperties() error {
	// Skip if already written (for round-trip support)
	if w.parts[strings.ToLower("/docprops/core.xml")] {
		return nil
	}

	if w.Properties == nil {
		return nil
	}

	data, err := w.Properties.Marshal()
	if err != nil {
		return err
	}

	header := &zip.FileHeader{
		Name:   "docProps/core.xml",
		Method: zip.Deflate,
	}

	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	if err != nil {
		return err
	}

	// Add relationship and content type
	w.ContentTypes.SetOverride("/docProps/core.xml", ContentTypeCoreProps)
	w.addRelationship(RelTypeCore, "docProps/core.xml", TargetModeInternal)

	return nil
}

// writeExtendedProperties writes the extended properties if set.
func (w *Writer) writeExtendedProperties() error {
	// Skip if already written (for round-trip support)
	if w.parts[strings.ToLower("/docprops/app.xml")] {
		return nil
	}

	if w.ExtendedProperties == nil {
		return nil
	}

	data, err := w.ExtendedProperties.Marshal()
	if err != nil {
		return err
	}

	header := &zip.FileHeader{
		Name:   "docProps/app.xml",
		Method: zip.Deflate,
	}

	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	if err != nil {
		return err
	}

	// Add relationship and content type
	w.ContentTypes.SetOverride("/docProps/app.xml", ContentTypeExtendedProps)
	w.addRelationship(RelTypeExtended, "docProps/app.xml", TargetModeInternal)

	return nil
}

// writeCustomProperties writes the custom properties if set.
func (w *Writer) writeCustomProperties() error {
	// Skip if already written (for round-trip support): a preserved raw
	// custom.xml wins, keeping producer formatting byte-identical.
	if w.parts[strings.ToLower("/docprops/custom.xml")] {
		return nil
	}

	if w.CustomProperties == nil {
		return nil
	}

	data, err := w.CustomProperties.Marshal()
	if err != nil {
		return err
	}

	header := &zip.FileHeader{
		Name:   "docProps/custom.xml",
		Method: zip.Deflate,
	}

	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	if _, err := writer.Write(data); err != nil {
		return err
	}
	w.parts[strings.ToLower("/docprops/custom.xml")] = true

	// Add relationship and content type. In a round-trip save the package
	// relationships and content types are preserved verbatim (which already
	// carry these when the part existed), so writeRelationships/writeContentTypes
	// skip the duplicates; a newly created package emits them here.
	w.ContentTypes.SetOverride("/docProps/custom.xml", ContentTypeCustomProps)
	w.addRelationship(RelTypeCustom, "docProps/custom.xml", TargetModeInternal)

	return nil
}

// WritePartRelationships writes a relationships file for a specific part. Like
// the other write methods it honors the closed check and rejects a duplicate
// write of the same .rels part (which would otherwise emit two zip entries with
// the same name).
func (w *Writer) WritePartRelationships(partName string, rels []*Relationship) error {
	if len(rels) == 0 {
		return nil
	}
	if w.closed {
		return ErrPackageClosed
	}

	relsName := GetRelationshipsPartName(partName)
	key := strings.ToLower("/" + strings.TrimPrefix(relsName, "/"))
	if w.parts[key] {
		return ErrDuplicatePart
	}

	data, err := MarshalRelationships(rels)
	if err != nil {
		return err
	}

	zipName := strings.TrimPrefix(relsName, "/")
	header := &zip.FileHeader{
		Name:   zipName,
		Method: zip.Deflate,
	}

	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	if _, err := writer.Write(data); err != nil {
		return err
	}
	w.parts[key] = true
	return nil
}

// Abort discards the package being written: it closes the underlying zip
// writer without emitting any package metadata (core/extended properties,
// package relationships, [Content_Types].xml) and marks the Writer closed, so
// every subsequent call fails with ErrPackageClosed. The bytes already written
// to the output are not a valid OPC package and must be discarded — Abort
// exists so an error path can terminate the writer without Close finalizing a
// half-written package as if it were good.
func (w *Writer) Abort() error {
	if w.closed {
		return ErrPackageClosed
	}
	w.closed = true
	return w.zipWriter.Close()
}

// Close finalizes the package and writes all metadata.
//
// If Close returns an error the output is incomplete and must be discarded:
// some parts or metadata may be missing and the zip central directory may not
// reflect what was written. Even when a metadata write fails, Close still
// closes the underlying zip writer (so the stream is flushed and not left as
// a truncated non-zip with no central directory) and reports every error via
// errors.Join. Callers abandoning a package after a part-write error should
// call Abort instead, which skips the metadata writes entirely.
func (w *Writer) Close() error {
	if w.closed {
		return ErrPackageClosed
	}
	w.closed = true

	// Write package metadata, stopping at the first failure. Content types
	// must be written last: earlier writes may register overrides.
	var metaErr error
	for _, write := range []func() error{
		w.writeCoreProperties,
		w.writeExtendedProperties,
		w.writeCustomProperties,
		w.writeRelationships,
		w.writeContentTypes,
	} {
		if metaErr = write(); metaErr != nil {
			break
		}
	}

	// Always close the zip writer, even after a metadata failure.
	return errors.Join(metaErr, w.zipWriter.Close())
}
