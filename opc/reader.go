package opc

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
)

// MaxDecompressedPartSize bounds how many bytes any single package part may
// decompress to. It guards against decompression bombs — a small compressed
// entry that expands to an enormous amount of data — which would otherwise let
// a hostile package exhaust memory before user code runs (the mandatory
// [Content_Types].xml is decompressed during NewReader). Set it to 0 to
// disable the bound; raise it before opening a package that legitimately
// contains a larger part.
//
// Both MaxDecompressedPartSize and MaxDecompressedPackageSize are captured
// once when a Reader is constructed; changing them affects only packages
// opened afterwards, never a Reader that is already open. Set them during
// program setup, before packages are opened: they are plain package-level
// variables, so mutating one concurrently with OpenReader/NewReader in
// another goroutine is a data race.
var MaxDecompressedPartSize int64 = 1 << 30 // 1 GiB

// MaxDecompressedPackageSize bounds the total number of bytes a single Reader
// may decompress across all of its parts combined. It complements
// MaxDecompressedPartSize: a hostile package can honestly declare many parts
// that each sit under the per-part cap yet together exhaust memory. Each part
// counts toward the total once, when it is first read; re-reading a part does
// not consume additional budget. Set it to 0 to disable the bound; raise it
// before opening a package whose parts legitimately decompress to more in
// total. See MaxDecompressedPartSize for the concurrency contract.
var MaxDecompressedPackageSize int64 = 4 << 30 // 4 GiB

// decompressionBudget holds the decompression limits captured from the
// package-level variables when a Reader is constructed, plus the running
// total of bytes the Reader has decompressed so far. It lives behind a
// pointer shared by the Reader and all of its Files, so accounting stays
// consistent even when the Reader value is copied (e.g. into a ReadCloser).
type decompressionBudget struct {
	maxPart    int64 // per-part cap; <= 0 disables
	maxPackage int64 // package-total cap; <= 0 disables

	// total is the number of bytes decompressed so far, counting each zip
	// entry once (on first successful read).
	total   int64
	charged map[*zip.File]bool
}

// newDecompressionBudget snapshots the package-level limits for one Reader.
func newDecompressionBudget() *decompressionBudget {
	return &decompressionBudget{
		maxPart:    MaxDecompressedPartSize,
		maxPackage: MaxDecompressedPackageSize,
		charged:    make(map[*zip.File]bool),
	}
}

// readZipEntry decompresses a single zip entry, bounding the output to the
// per-part cap and the remaining package-total budget. It rejects entries
// whose declared uncompressed size already exceeds either bound, and
// re-checks during the read so a lying local header cannot slip past.
func (b *decompressionBudget) readZipEntry(zf *zip.File) ([]byte, error) {
	if b.maxPart > 0 && zf.UncompressedSize64 > uint64(b.maxPart) {
		return nil, fmt.Errorf("opc: part %q declares %d bytes, exceeding the %d-byte per-part decompression limit (raise MaxDecompressedPartSize before opening to allow it)", zf.Name, zf.UncompressedSize64, b.maxPart)
	}

	// The package-total bound only applies to entries not yet counted:
	// re-reading an already-charged part cannot grow the total.
	pkgRemaining := int64(-1) // -1: package bound not in effect for this read
	if b.maxPackage > 0 && !b.charged[zf] {
		pkgRemaining = b.maxPackage - b.total
		if pkgRemaining < 0 {
			pkgRemaining = 0
		}
		if zf.UncompressedSize64 > uint64(pkgRemaining) {
			return nil, fmt.Errorf("opc: part %q declares %d bytes, which would exceed the %d-byte package decompression limit with %d bytes already decompressed (raise MaxDecompressedPackageSize before opening to allow it)", zf.Name, zf.UncompressedSize64, b.maxPackage, b.total)
		}
	}

	// Effective cap on the bytes actually read; -1 means unbounded.
	limit := int64(-1)
	if b.maxPart > 0 {
		limit = b.maxPart
	}
	if pkgRemaining >= 0 && (limit < 0 || pkgRemaining < limit) {
		limit = pkgRemaining
	}

	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	data, err := readAllLimited(rc, limit)
	if err != nil {
		return nil, err
	}
	if limit >= 0 && int64(len(data)) > limit {
		if b.maxPart > 0 && int64(len(data)) > b.maxPart {
			return nil, fmt.Errorf("opc: part %q exceeds the %d-byte per-part decompression limit (raise MaxDecompressedPartSize before opening to allow it)", zf.Name, b.maxPart)
		}
		return nil, fmt.Errorf("opc: part %q exceeds the %d-byte package decompression limit with %d bytes already decompressed (raise MaxDecompressedPackageSize before opening to allow it)", zf.Name, b.maxPackage, b.total)
	}

	if b.maxPackage > 0 && !b.charged[zf] {
		b.total += int64(len(data))
		b.charged[zf] = true
	}
	return data, nil
}

// readAllLimited reads rc fully, but stops one byte past limit so the caller
// can unambiguously detect overflow via len > limit. limit < 0 means
// unbounded.
func readAllLimited(rc io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		return io.ReadAll(rc)
	}
	return io.ReadAll(io.LimitReader(rc, limit+1))
}

// File represents a file within an OPC package.
type File struct {
	// Name is the path of the file within the package.
	Name string

	// ContentType is the MIME type of the file content.
	ContentType string

	zipFile *zip.File
	budget  *decompressionBudget
}

// Open returns an io.ReadCloser for reading the file's contents.
func (f *File) Open() (io.ReadCloser, error) {
	return f.zipFile.Open()
}

// ReadAll reads and returns the entire contents of the file. The result is
// bounded by the MaxDecompressedPartSize and MaxDecompressedPackageSize
// limits in effect when the Reader was constructed, to guard against
// decompression bombs.
func (f *File) ReadAll() ([]byte, error) {
	return f.budget.readZipEntry(f.zipFile)
}

// Reader provides read access to an OPC package.
type Reader struct {
	// Files contains all files in the package.
	Files []*File

	// Relationships contains package-level relationships.
	Relationships []*Relationship

	// ContentTypes provides content type information.
	ContentTypes *ContentTypes

	// Properties contains the core properties of the package.
	Properties *CoreProperties

	zipReader *zip.Reader
	budget    *decompressionBudget
}

// ReadCloser extends Reader with a Close method.
type ReadCloser struct {
	Reader
	file *os.File
}

// Close closes the ReadCloser.
func (rc *ReadCloser) Close() error {
	if rc.file == nil {
		return nil
	}
	return rc.file.Close()
}

// OpenReader opens an OPC package from a file path.
func OpenReader(path string) (*ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	r, err := NewReader(f, fi.Size())
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	return &ReadCloser{Reader: *r, file: f}, nil
}

// NewReader creates a Reader from an io.ReaderAt.
func NewReader(r io.ReaderAt, size int64) (*Reader, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	reader := &Reader{
		zipReader: zr,
		Files:     make([]*File, 0, len(zr.File)),
		budget:    newDecompressionBudget(),
	}

	// First pass: find and parse [Content_Types].xml
	for _, zf := range zr.File {
		if strings.EqualFold(zf.Name, "[Content_Types].xml") {
			data, err := reader.budget.readZipEntry(zf)
			if err != nil {
				return nil, err
			}

			reader.ContentTypes, err = UnmarshalContentTypes(data)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	if reader.ContentTypes == nil {
		return nil, ErrCorruptedPackage
	}

	// Second pass: create File entries for all parts (excluding special files)
	for _, zf := range zr.File {
		name := "/" + zf.Name

		// Skip directories
		if strings.HasSuffix(zf.Name, "/") {
			continue
		}

		// Skip special files
		if strings.EqualFold(zf.Name, "[Content_Types].xml") {
			continue
		}

		contentType := reader.ContentTypes.GetContentType(name)

		reader.Files = append(reader.Files, &File{
			Name:        name,
			ContentType: contentType,
			zipFile:     zf,
			budget:      reader.budget,
		})
	}

	// Parse package-level relationships
	if err := reader.parsePackageRelationships(); err != nil {
		return nil, err
	}

	// Parse core properties if they exist
	reader.parseCoreProperties()

	return reader, nil
}

// parsePackageRelationships reads the package-level .rels file.
func (r *Reader) parsePackageRelationships() error {
	relsFile := r.GetFile("/_rels/.rels")
	if relsFile == nil {
		// Package-level relationships are optional
		return nil
	}

	data, err := relsFile.ReadAll()
	if err != nil {
		return err
	}

	rels, err := UnmarshalRelationships(data)
	if err != nil {
		return err
	}

	r.Relationships = rels
	return nil
}

// parseCoreProperties reads the core properties if they exist.
func (r *Reader) parseCoreProperties() {
	// Find core properties relationship
	for _, rel := range r.Relationships {
		if rel.Type == RelTypeCore {
			target := ResolvePartName("/", rel.Target)
			f := r.GetFile(target)
			if f == nil {
				continue
			}

			data, err := f.ReadAll()
			if err != nil {
				continue
			}

			props, err := UnmarshalCoreProperties(data)
			if err != nil {
				continue
			}

			r.Properties = props
			return
		}
	}
}

// GetFile returns the file with the given path, or nil if not found.
func (r *Reader) GetFile(name string) *File {
	normalizedName := NormalizePartName(name)
	for _, f := range r.Files {
		if strings.EqualFold(f.Name, normalizedName) {
			return f
		}
	}
	return nil
}

// GetRawZipFile returns the raw data for a file in the zip archive by name.
// This can be used to access special files like [Content_Types].xml that are
// not included in the Files list.
func (r *Reader) GetRawZipFile(name string) ([]byte, error) {
	for _, zf := range r.zipReader.File {
		if strings.EqualFold(zf.Name, name) {
			return r.budget.readZipEntry(zf)
		}
	}
	return nil, fmt.Errorf("file not found: %s", name)
}

// GetRelationshipsByType returns all package-level relationships with the specified type.
func (r *Reader) GetRelationshipsByType(relType string) []*Relationship {
	var result []*Relationship
	for _, rel := range r.Relationships {
		if rel.Type == relType {
			result = append(result, rel)
		}
	}
	return result
}

// GetPartRelationships reads and returns the relationships for a specific part.
func (r *Reader) GetPartRelationships(partName string) ([]*Relationship, error) {
	relsName := GetRelationshipsPartName(partName)
	relsFile := r.GetFile(relsName)
	if relsFile == nil {
		return nil, nil
	}

	data, err := relsFile.ReadAll()
	if err != nil {
		return nil, err
	}

	return UnmarshalRelationships(data)
}
