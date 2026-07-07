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
var MaxDecompressedPartSize int64 = 1 << 30 // 1 GiB

// readZipEntry decompresses a single zip entry, bounding the output to
// MaxDecompressedPartSize. It rejects entries whose declared uncompressed size
// already exceeds the cap, and re-checks during the read so a lying local
// header cannot slip past.
func readZipEntry(zf *zip.File) ([]byte, error) {
	if MaxDecompressedPartSize > 0 && zf.UncompressedSize64 > uint64(MaxDecompressedPartSize) {
		return nil, fmt.Errorf("opc: part %q declares %d bytes, exceeding the %d-byte decompression limit", zf.Name, zf.UncompressedSize64, MaxDecompressedPartSize)
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return readAllLimited(rc, zf.Name)
}

// readAllLimited reads rc fully, but fails if it yields more than
// MaxDecompressedPartSize bytes.
func readAllLimited(rc io.Reader, name string) ([]byte, error) {
	if MaxDecompressedPartSize <= 0 {
		return io.ReadAll(rc)
	}
	// Read one byte past the cap so len > cap unambiguously signals overflow.
	data, err := io.ReadAll(io.LimitReader(rc, MaxDecompressedPartSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxDecompressedPartSize {
		return nil, fmt.Errorf("opc: part %q exceeds the %d-byte decompression limit", name, MaxDecompressedPartSize)
	}
	return data, nil
}

// File represents a file within an OPC package.
type File struct {
	// Name is the path of the file within the package.
	Name string

	// ContentType is the MIME type of the file content.
	ContentType string

	zipFile *zip.File
}

// Open returns an io.ReadCloser for reading the file's contents.
func (f *File) Open() (io.ReadCloser, error) {
	return f.zipFile.Open()
}

// ReadAll reads and returns the entire contents of the file. The result is
// bounded by MaxDecompressedPartSize to guard against decompression bombs.
func (f *File) ReadAll() ([]byte, error) {
	return readZipEntry(f.zipFile)
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
	}

	// First pass: find and parse [Content_Types].xml
	for _, zf := range zr.File {
		if strings.EqualFold(zf.Name, "[Content_Types].xml") {
			data, err := readZipEntry(zf)
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
			return readZipEntry(zf)
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
