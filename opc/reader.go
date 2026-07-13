package opc

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
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

	// mu guards total and charged. All Files of a Reader share one budget and
	// a read-only Reader invites concurrent part reads, so the running
	// accounting must be synchronized.
	mu sync.Mutex

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

// admit performs the declared-size pre-checks for one zip entry and returns
// the effective cap on the bytes that may be read for it (-1: unbounded). It
// rejects entries whose declared uncompressed size already exceeds the
// per-part cap or the remaining package budget.
func (b *decompressionBudget) admit(zf *zip.File) (int64, error) {
	if b.maxPart > 0 && zf.UncompressedSize64 > uint64(b.maxPart) {
		return 0, fmt.Errorf("opc: part %q declares %d bytes, exceeding the %d-byte per-part decompression limit (raise MaxDecompressedPartSize before opening to allow it)", zf.Name, zf.UncompressedSize64, b.maxPart)
	}

	// Effective cap on the bytes actually read; -1 means unbounded.
	limit := int64(-1)
	if b.maxPart > 0 {
		limit = b.maxPart
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// The package-total bound only applies to entries not yet counted:
	// re-reading an already-charged part cannot grow the total.
	if b.maxPackage > 0 && !b.charged[zf] {
		pkgRemaining := b.maxPackage - b.total
		if pkgRemaining < 0 {
			pkgRemaining = 0
		}
		if zf.UncompressedSize64 > uint64(pkgRemaining) {
			return 0, fmt.Errorf("opc: part %q declares %d bytes, which would exceed the %d-byte package decompression limit with %d bytes already decompressed (raise MaxDecompressedPackageSize before opening to allow it)", zf.Name, zf.UncompressedSize64, b.maxPackage, b.total)
		}
		if limit < 0 || pkgRemaining < limit {
			limit = pkgRemaining
		}
	}
	return limit, nil
}

// charge counts n freshly decompressed bytes for zf toward the package
// total, once per entry. It re-checks the remaining budget under the lock:
// the pre-check in admit used a snapshot, and other goroutines may have
// consumed budget since; this also catches entries whose actual size exceeds
// their declared size (lying local header).
func (b *decompressionBudget) charge(zf *zip.File, n int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxPackage <= 0 || b.charged[zf] {
		return nil
	}
	if n > b.maxPackage-b.total {
		return fmt.Errorf("opc: part %q exceeds the %d-byte package decompression limit with %d bytes already decompressed (raise MaxDecompressedPackageSize before opening to allow it)", zf.Name, b.maxPackage, b.total)
	}
	b.total += n
	b.charged[zf] = true
	return nil
}

// readZipEntry decompresses a single zip entry, bounding the output to the
// per-part cap and the remaining package-total budget. It rejects entries
// whose declared uncompressed size already exceeds either bound, and
// re-checks during the read so a lying local header cannot slip past.
func (b *decompressionBudget) readZipEntry(zf *zip.File) ([]byte, error) {
	limit, err := b.admit(zf)
	if err != nil {
		return nil, err
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
	if b.maxPart > 0 && int64(len(data)) > b.maxPart {
		return nil, fmt.Errorf("opc: part %q exceeds the %d-byte per-part decompression limit (raise MaxDecompressedPartSize before opening to allow it)", zf.Name, b.maxPart)
	}

	// Any read that overflowed the package-remaining portion of limit is
	// caught here: charge re-checks the budget under the lock and the total
	// only ever grows, so an over-read cannot slip through.
	if err := b.charge(zf, int64(len(data))); err != nil {
		return nil, err
	}
	return data, nil
}

// openZipEntry opens a bounded stream over one zip entry. Declared-size
// violations are rejected immediately; violations only observable while
// decompressing (lying local headers) surface as Read errors from the
// returned stream. If the entry has not been charged against the package
// budget yet, the stream becomes its charger: bytes count toward the budget
// as they are read.
func (b *decompressionBudget) openZipEntry(zf *zip.File) (io.ReadCloser, error) {
	if _, err := b.admit(zf); err != nil {
		return nil, err
	}

	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}

	s := &budgetedReadCloser{rc: rc, b: b, name: zf.Name}
	// Mark the entry charged now so this stream and any concurrent or later
	// read of the same entry agree on charge-once semantics; the byte count
	// itself is added incrementally as the stream is consumed.
	b.mu.Lock()
	if b.maxPackage > 0 && !b.charged[zf] {
		b.charged[zf] = true
		s.charges = true
	}
	b.mu.Unlock()
	return s, nil
}

// budgetedReadCloser enforces the decompression limits on a streaming read
// of one zip entry without buffering it. Once the bytes decompressed through
// it exceed the per-part cap or the shared package budget, Read fails and
// the error is sticky.
type budgetedReadCloser struct {
	rc   io.ReadCloser
	b    *decompressionBudget
	name string

	// charges records whether this stream charges the package budget (it
	// does unless the entry was already charged when the stream was opened).
	charges  bool
	streamed int64 // bytes read through this stream so far
	err      error // sticky limit-violation error
}

func (s *budgetedReadCloser) Read(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	// Clamp the read so an oversized part is detected at most one byte past
	// the per-part cap instead of decompressing arbitrarily far beyond it.
	if s.b.maxPart > 0 {
		if headroom := s.b.maxPart - s.streamed + 1; int64(len(p)) > headroom {
			p = p[:headroom]
		}
	}
	n, err := s.rc.Read(p)
	if n > 0 {
		s.streamed += int64(n)
		if cerr := s.chargeStream(int64(n)); cerr != nil {
			s.err = cerr
			return n, cerr
		}
		if s.b.maxPart > 0 && s.streamed > s.b.maxPart {
			s.err = fmt.Errorf("opc: part %q exceeds the %d-byte per-part decompression limit (raise MaxDecompressedPartSize before opening to allow it)", s.name, s.b.maxPart)
			return n, s.err
		}
	}
	return n, err
}

// chargeStream adds n freshly decompressed bytes to the package total,
// failing once the budget is exhausted. Unlike ReadAll, which knows the full
// part size up front, a stream charges as bytes arrive.
func (s *budgetedReadCloser) chargeStream(n int64) error {
	if !s.charges {
		return nil
	}
	s.b.mu.Lock()
	s.b.total += n
	over := s.b.total > s.b.maxPackage
	// This stream is the entry's only charger, so total minus our own bytes
	// is what the rest of the package had decompressed.
	already := s.b.total - s.streamed
	s.b.mu.Unlock()
	if over {
		return fmt.Errorf("opc: part %q exceeds the %d-byte package decompression limit with %d bytes already decompressed (raise MaxDecompressedPackageSize before opening to allow it)", s.name, s.b.maxPackage, already)
	}
	return nil
}

func (s *budgetedReadCloser) Close() error {
	return s.rc.Close()
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

// Open returns an io.ReadCloser streaming the file's contents. The stream is
// bounded by the same MaxDecompressedPartSize and MaxDecompressedPackageSize
// limits as ReadAll, captured when the Reader was constructed: Open fails
// immediately when the part's declared size exceeds either bound, and Read
// returns an error once the bytes actually decompressed exceed the per-part
// limit or the remaining package budget. Bytes count toward the package
// budget as they stream; like ReadAll, a part is charged at most once, so
// re-reading an already-read part does not consume additional budget.
func (f *File) Open() (io.ReadCloser, error) {
	return f.budget.openZipEntry(f.zipFile)
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

	// DirectoryEntries lists the zip directory entries ("_rels/", "word/", …)
	// present in the source archive, in archive order. OPC ignores directory
	// entries, but some producers (WPS, Apache POI, some Excel builds) emit
	// them; a byte-faithful save re-emits the same set via
	// Writer.WriteDirectoryEntries.
	DirectoryEntries []string

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
		// Index parts under a canonical part name: zip producers (and hostile
		// packages) emit entry names like "./ppt/presentation.xml",
		// "word\document.xml" or "a//b.xml", which would otherwise be
		// unreachable through GetFile and silently droppable on round-trip.
		// The zip entry itself keeps its original raw name, so
		// GetRawZipFile(zf.Name) and preserved-part paths still see the
		// original bytes under the original name. When two entries collapse to
		// the same canonical name, the first wins (GetFile returns the first
		// match), consistent with existing duplicate handling.
		name := canonicalZipEntryName(zf.Name)

		// Directory entries carry no part data, but record them so a save can
		// reproduce the source archive's directory listing.
		if strings.HasSuffix(zf.Name, "/") || strings.HasSuffix(name, "/") {
			reader.DirectoryEntries = append(reader.DirectoryEntries, zf.Name)
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

// canonicalZipEntryName converts a raw zip entry name into the canonical
// leading-slash part name used for lookups: backslash separators become
// forward slashes, leading "./" segments are stripped, and empty segments
// ("//") are collapsed. The original raw entry name is untouched — it remains
// the key for GetRawZipFile and the name under which the entry's bytes were
// stored.
func canonicalZipEntryName(name string) string {
	s := strings.ReplaceAll(name, `\`, "/")
	for strings.HasPrefix(s, "./") {
		s = strings.TrimPrefix(s, "./")
	}
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	return "/" + strings.TrimPrefix(s, "/")
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
