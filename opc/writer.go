package opc

import (
	"archive/zip"
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

	// Relationships contains package-level relationships.
	Relationships []*Relationship

	// ContentTypes manages content types for parts.
	ContentTypes *ContentTypes

	zipWriter *zip.Writer
	output    io.Writer
	parts     map[string]bool
	nextRelID int
	closed    bool
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
func (w *Writer) CreatePart(name, contentType string, compression CompressionOption) (io.Writer, error) {
	if w.closed {
		return nil, ErrPackageClosed
	}

	if err := ValidatePartName(name); err != nil {
		return nil, err
	}

	normalizedName := NormalizePartName(name)
	if w.parts[strings.ToLower(normalizedName)] {
		return nil, ErrDuplicatePart
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

// WriteRawFile writes a raw file to the package without part name validation.
// This is used for special files like [Content_Types].xml that don't follow
// OPC part naming rules.
func (w *Writer) WriteRawFile(name string, data []byte) error {
	if w.closed {
		return ErrPackageClosed
	}

	// Track the file to prevent duplicates
	key := strings.ToLower("/" + strings.TrimPrefix(name, "/"))
	if w.parts[key] {
		return ErrDuplicatePart
	}

	header := &zip.FileHeader{
		Name:   name,
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

// AddRelationship adds a package-level relationship.
func (w *Writer) AddRelationship(relType, target string, targetMode TargetMode) *Relationship {
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
	// Skip if already written (for round-trip support)
	if w.parts[strings.ToLower("/[content_types].xml")] {
		return nil
	}

	data, err := w.ContentTypes.Marshal()
	if err != nil {
		return err
	}

	header := &zip.FileHeader{
		Name:   "[Content_Types].xml",
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
	w.AddRelationship(RelTypeCore, "docProps/core.xml", TargetModeInternal)

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
	w.AddRelationship(RelTypeExtended, "docProps/app.xml", TargetModeInternal)

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

// Close finalizes the package and writes all metadata.
func (w *Writer) Close() error {
	if w.closed {
		return ErrPackageClosed
	}
	w.closed = true

	// Write core properties
	if err := w.writeCoreProperties(); err != nil {
		return err
	}

	// Write extended properties
	if err := w.writeExtendedProperties(); err != nil {
		return err
	}

	// Write package-level relationships
	if err := w.writeRelationships(); err != nil {
		return err
	}

	// Write content types (must be last)
	if err := w.writeContentTypes(); err != nil {
		return err
	}

	// Close zip writer
	if err := w.zipWriter.Close(); err != nil {
		return err
	}

	return nil
}
