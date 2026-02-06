package opc

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
)

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

// ReadAll reads and returns the entire contents of the file.
func (f *File) ReadAll() ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
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
		f.Close()
		return nil, err
	}

	r, err := NewReader(f, fi.Size())
	if err != nil {
		f.Close()
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
			rc, err := zf.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
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
			rc, err := zf.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
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
