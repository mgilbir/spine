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
	closed    bool

	// pkgRels holds the relationships of a package-relationships part the
	// caller wrote verbatim (round-trip preservation). Close consults it to
	// decide whether a metadata part it is about to emit is already reachable:
	// /_rels/.rels has been streamed into the zip by then and cannot be
	// rewritten, so a relationship appended to w.Relationships afterwards
	// would be silently discarded and the part orphaned (C377).
	pkgRels []*Relationship

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
		Relationships: make([]*Relationship, 0),
	}
}

// packageRelsPartName / packageRelsKey name the package-level relationships
// part, the one part Close must reconcile against because it is both written
// by callers (round-trip preservation) and appended to by Close's metadata
// writers.
const packageRelsPartName = "/_rels/.rels"

var packageRelsKey = strings.ToLower(packageRelsPartName)

// partKey is the case-insensitive key under which a part is tracked in
// w.parts. Every write path derives its key here so the four registries Close
// reconciles (parts, relationships, content types and the deferred raw
// [Content_Types].xml) agree on what a part is called.
func partKey(name string) string {
	return strings.ToLower("/" + strings.TrimPrefix(name, "/"))
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

	_, err = writer.Write(w.reconcilePackageRels(name, data))
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

	_, err = writer.Write(w.reconcilePackageRels(name, data))
	return err
}

// WriteDirectoryEntries writes zero-length zip directory entries (names ending
// in "/"). OPC consumers ignore directory entries, but re-emitting the ones a
// source archive carried (Reader.DirectoryEntries) keeps a round-tripped
// package's entry listing faithful to its producer. Names that do not name a
// directory or that were already written are skipped rather than rejected, so
// a caller can replay a captured list verbatim.
//
// Each name goes through the same canonicalization the Reader applies, so a
// producer that separates with backslashes ("word\") — which the Reader
// records as a directory entry — is re-emitted rather than silently dropped
// by a raw trailing-slash test (C456).
func (w *Writer) WriteDirectoryEntries(names []string) error {
	if w.closed {
		return ErrPackageClosed
	}
	for _, raw := range names {
		name := strings.TrimPrefix(canonicalZipEntryName(raw), "/")
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

// WriteRawFile writes a raw file to the package without OPC part-name
// grammar validation. This is used for special files like [Content_Types].xml
// that don't follow OPC part naming rules.
//
// The name still has to be a structurally sane path: it is checked against the
// same lenient shape rules as a preserved part (no empty, "." or ".."
// segments), which admit [Content_Types].xml and the wild names real producers
// emit while rejecting a name that escapes the package root. Without that
// check the name went into the archive verbatim, so a caller passing
// "../../etc/evil.conf" produced an archive entry that a naive extractor
// writes outside the destination directory (C392).
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

	if err := validatePartNameShape("/" + strings.TrimPrefix(name, "/")); err != nil {
		return err
	}

	// Track the file to prevent duplicates
	key := partKey(name)
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

	if _, err := writer.Write(w.reconcilePackageRels(name, data)); err != nil {
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
//
// The identifier is derived from the current contents of the exported
// Relationships slice rather than from a private counter. The counter could
// not see a slice the caller assigned or appended to directly — which opc's
// own signer does — so it happily minted an rId already in use (C394).
func (w *Writer) addRelationship(relType, target string, targetMode TargetMode) *Relationship {
	rel := &Relationship{
		ID:         nextRelationshipID(w.Relationships),
		Type:       relType,
		Target:     target,
		TargetMode: targetMode,
	}
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
// metadata file written during Close, and records it in w.parts. Recording is
// what keeps the four registries consistent: two of the three metadata writers
// used to skip it, which was harmless only because they all ran after closed
// was set, and was exactly the drift that becomes a duplicate zip entry (C447).
func (w *Writer) writeMetadataEntry(name string, data []byte) error {
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	writer, err := w.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	w.parts[partKey(name)] = true
	return nil
}

// writeRelationships writes the package-level relationships file.
func (w *Writer) writeRelationships() error {
	// Skip if already written (for round-trip support). Anything Close needed
	// to add to those bytes was settled by reconcilePackageRels at the moment
	// they were written, or reported by ensurePackageRelationship above.
	if w.parts[packageRelsKey] {
		return nil
	}

	if len(w.Relationships) == 0 {
		return nil
	}

	data, err := MarshalRelationships(w.Relationships)
	if err != nil {
		return err
	}
	return w.writeMetadataEntry("_rels/.rels", data)
}

// metadataPart describes one package metadata part that Close derives from an
// exported Writer field. All three (core, extended and custom properties) need
// the same four registries updated in lockstep — the zip entry, w.parts, the
// content-type override and the package relationship that makes the part
// reachable — so they share one implementation rather than three copies that
// drifted apart (C377, C394, C447).
type metadataPart struct {
	partName    string // canonical part name, e.g. "/docProps/core.xml"
	zipName     string // zip entry name, e.g. "docProps/core.xml"
	contentType string
	relType     string
	relTarget   string
	// marshal returns the part bytes, or nil when the source field is unset
	// and the part must not be written at all.
	marshal func() ([]byte, error)
}

// metadataParts returns the metadata part descriptors in the order Close
// emits them. The order is load-bearing only in that [Content_Types].xml is
// written afterwards, since these registrations feed into it.
func (w *Writer) metadataParts() []metadataPart {
	return []metadataPart{
		{
			partName: "/docProps/core.xml", zipName: "docProps/core.xml",
			contentType: ContentTypeCoreProps,
			relType:     RelTypeCore, relTarget: "docProps/core.xml",
			marshal: func() ([]byte, error) {
				if w.Properties == nil {
					return nil, nil
				}
				return w.Properties.Marshal()
			},
		},
		{
			partName: "/docProps/app.xml", zipName: "docProps/app.xml",
			contentType: ContentTypeExtendedProps,
			relType:     RelTypeExtended, relTarget: "docProps/app.xml",
			marshal: func() ([]byte, error) {
				if w.ExtendedProperties == nil {
					return nil, nil
				}
				return w.ExtendedProperties.Marshal()
			},
		},
		{
			partName: "/docProps/custom.xml", zipName: "docProps/custom.xml",
			contentType: ContentTypeCustomProps,
			relType:     RelTypeCustom, relTarget: "docProps/custom.xml",
			marshal: func() ([]byte, error) {
				if w.CustomProperties == nil {
					return nil, nil
				}
				return w.CustomProperties.Marshal()
			},
		},
	}
}

// pending reports whether p still has to be written: its source field is set
// and no part of that name was written explicitly (a preserved raw copy wins,
// keeping producer formatting byte-identical).
func (w *Writer) pending(p metadataPart) bool {
	if w.parts[partKey(p.partName)] {
		return false
	}
	switch p.relType {
	case RelTypeCore:
		return w.Properties != nil
	case RelTypeExtended:
		return w.ExtendedProperties != nil
	case RelTypeCustom:
		return w.CustomProperties != nil
	}
	return false
}

// writeMetadataParts is Close's reconciliation pass over the metadata parts.
// For each one it decides — once, in one place — whether the part is to be
// written, whether a package relationship reaching it already exists, and
// which content-type override it needs, then applies all of that together. In
// particular the relationship is settled *before* the zip entry is emitted, so
// the package can never end up carrying a metadata part that nothing points at.
func (w *Writer) writeMetadataParts() error {
	for _, p := range w.metadataParts() {
		if !w.pending(p) {
			continue
		}
		data, err := p.marshal()
		if err != nil {
			return err
		}
		if data == nil {
			continue
		}
		if err := w.ensurePackageRelationship(p); err != nil {
			return err
		}
		if err := w.writeMetadataEntry(p.zipName, data); err != nil {
			return err
		}
		w.ContentTypes.SetOverride(p.partName, p.contentType)
	}
	return nil
}

// ensurePackageRelationship makes sure the package relationships will contain
// a relationship of p's type, appending one when they will not.
//
// A package-level metadata relationship is a singleton (ECMA-376 Part 2 allows
// one core-properties relationship per package, and Office writes one each for
// app.xml and custom.xml), so an existing relationship of the same type
// already reaches the part whatever target spelling it uses — appending a
// second would mint a duplicate (C394).
//
// When the relationships part was written verbatim it is already in the zip
// stream and cannot be amended; appending to w.Relationships would be silently
// dropped by writeRelationships and leave the part orphaned (C377). That is
// reported as an error rather than written out, and because this runs before
// the part is emitted, nothing is written at all. Callers preserving
// /_rels/.rels should set the properties fields *before* writing it —
// WritePart/WritePreservedPart/WriteRawFile then inject the missing
// relationship into the preserved bytes for them.
func (w *Writer) ensurePackageRelationship(p metadataPart) error {
	for _, rel := range w.Relationships {
		if rel != nil && rel.Type == p.relType {
			return nil
		}
	}
	for _, rel := range w.pkgRels {
		if rel != nil && rel.Type == p.relType {
			return nil
		}
	}
	if w.parts[packageRelsKey] {
		return fmt.Errorf("opc: %s must be written but %s was already emitted without a %s relationship, so the part would be unreachable; set the properties before writing the package relationships part",
			p.partName, packageRelsPartName, p.relType)
	}
	w.addRelationship(p.relType, p.relTarget, TargetModeInternal)
	return nil
}

// reconcilePackageRels is the write-time half of the same reconciliation: when
// the caller hands over the package relationships part verbatim, any metadata
// part Close still has to write needs a relationship in those bytes, and this
// is the last moment at which they can be amended. EnsureRelationshipInRels
// inserts the element immediately before </Relationships>, leaving every
// existing byte in place, so a package that needs nothing added round-trips
// byte-for-byte.
//
// It returns the bytes to write and records the resulting relationship set for
// ensurePackageRelationship's coverage check.
func (w *Writer) reconcilePackageRels(name string, data []byte) []byte {
	if partKey(NormalizePartName(name)) != packageRelsKey {
		return data
	}

	parsed, err := UnmarshalRelationships(data)
	if err != nil {
		// Malformed rels bytes are preserved untouched, as they always were;
		// ensurePackageRelationship will report anything it cannot cover.
		return data
	}
	covered := make(map[string]bool, len(parsed))
	for _, rel := range parsed {
		if rel != nil {
			covered[rel.Type] = true
		}
	}
	for _, p := range w.metadataParts() {
		if covered[p.relType] || !w.pending(p) {
			continue
		}
		augmented, rel, added := EnsureRelationshipInRels(data, p.relType, p.relTarget)
		if !added {
			continue
		}
		data = augmented
		parsed = append(parsed, rel)
		covered[p.relType] = true
	}
	w.pkgRels = parsed
	return data
}

// WritePartRelationships writes a relationships file for a specific part. Like
// the other write methods it honors the closed check and rejects a duplicate
// write of the same .rels part (which would otherwise emit two zip entries with
// the same name).
func (w *Writer) WritePartRelationships(partName string, rels []*Relationship) error {
	// The closed check comes first: reporting success on a closed Writer for
	// the empty-slice case alone was inconsistent with every other write
	// method and with AddRelationship (C450).
	if w.closed {
		return ErrPackageClosed
	}
	if len(rels) == 0 {
		return nil
	}

	relsName := GetRelationshipsPartName(partName)
	key := partKey(relsName)
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

	// Write package metadata, stopping at the first failure. The metadata
	// parts (core, extended and custom properties) are reconciled together in
	// one pass, which settles each part's relationship and content type before
	// its bytes go out; the package relationships and then — last, because
	// every earlier write may register overrides — [Content_Types].xml follow.
	var metaErr error
	for _, write := range []func() error{
		w.writeMetadataParts,
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
