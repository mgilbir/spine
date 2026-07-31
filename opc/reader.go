package opc

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strings"
	"sync"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// contentTypesPartName is the canonical part name of the mandatory
// [Content_Types].xml stream. It is matched through canonicalZipEntryName
// like every other entry, so a producer that spells it "./[Content_Types].xml"
// or "[content_types].xml" is still recognised.
const contentTypesPartName = "/[Content_Types].xml"

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
// another goroutine is a data race. To override the limits for a single
// Reader without touching the globals, pass ReaderOptions to
// NewReaderWithOptions/OpenReaderWithOptions instead.
var MaxDecompressedPartSize int64 = 1 << 30 // 1 GiB

// MaxDecompressedPackageSize bounds the total number of bytes a single Reader
// may decompress across all of its parts combined. It complements
// MaxDecompressedPartSize: a hostile package can honestly declare many parts
// that each sit under the per-part cap yet together exhaust memory. Each part
// contributes at most its own decompressed size to the total, however many
// times and however partially it is read: a read charges only the bytes beyond
// the high-water mark the part has already been charged for, so re-reading a
// part consumes no additional budget while abandoning a stream part-way does
// not make the remainder free. Set it to 0 to disable the bound; raise it
// before opening a package whose parts legitimately decompress to more in
// total. See MaxDecompressedPartSize for the concurrency contract.
var MaxDecompressedPackageSize int64 = 4 << 30 // 4 GiB

// MaxPackageEntries bounds how many zip entries a package may contain. It is
// the entry-count dimension the byte-oriented bounds above cannot see: a
// modestly sized archive can hold hundreds of thousands of tiny entries, each
// of which becomes a header, a name string and a *File on open, so the
// in-memory cost is a multiple of the input size before a single part is
// decompressed. Real OOXML packages hold at most a few thousand entries.
//
// The bound is applied immediately after the central directory is parsed, so
// it caps everything this package builds per entry; archive/zip's own
// directory parse (which is already bounded by the input size) has run by
// then. Set it to 0 to disable the bound, or raise it before opening a
// package that legitimately contains more entries. It is captured once per
// Reader like the decompression limits — see MaxDecompressedPartSize for the
// concurrency contract — and can be overridden per Reader through
// MaxNestingDepth bounds how deeply elements may nest in any XML part of a
// package. Set it to 0 to disable the bound; raise it before opening a package
// that legitimately nests deeper.
//
// It is a structural dimension the byte-oriented limits cannot see, in the same
// way MaxPackageEntries is: nesting costs a decoder frame and a model frame per
// level regardless of how few bytes express it. A 244 KB slide holding 80,000
// nested p:grpSp elements cost 627 MB resident, and the per-level cost grows
// with depth; under a megabyte was enough to kill the process. The default is
// calibrated against real documents rather than taste — the deepest part in
// 170,913 parts across 3,600 Common Crawl documents nests 95 levels, so 1000
// leaves an order of magnitude of headroom over anything a producer writes.
//
// Captured once when a Reader is constructed, like the decompression limits;
// see MaxDecompressedPartSize for the concurrency contract.
var MaxNestingDepth = 1000

// ReaderOptions.
var MaxPackageEntries = 1 << 16 // 65536

// decompressionBudget holds the decompression limits captured from the
// package-level variables when a Reader is constructed, plus the running
// total of bytes the Reader has decompressed so far. It lives behind a
// pointer shared by the Reader and all of its Files, so accounting stays
// consistent even when the Reader value is copied (e.g. into a ReadCloser).
type decompressionBudget struct {
	maxPart    int64 // per-part cap; <= 0 disables
	maxPackage int64 // package-total cap; <= 0 disables
	maxDepth   int   // per-part XML nesting cap; <= 0 disables

	// mu guards total and charged. All Files of a Reader share one budget and
	// a read-only Reader invites concurrent part reads, so the running
	// accounting must be synchronized.
	mu sync.Mutex

	// total is the number of bytes decompressed so far, summing each zip
	// entry's high-water mark.
	total int64

	// charged records, per zip entry, the high-water mark of bytes already
	// counted toward total. Every read charges only the delta above that mark,
	// which is what makes an entry cost its own size at most and at least: a
	// boolean "already charged" flag set when a stream is opened let a caller
	// read one byte, abandon the stream and then decompress the whole part for
	// free, so the package bound only ever cost one byte per part (C376).
	charged map[*zip.File]int64
}

// ReaderOptions configures a single Reader, overriding the package-level
// defaults. The zero value means "use the defaults", so ReaderOptions{} is
// always safe to pass.
type ReaderOptions struct {
	// MaxDecompressedPartSize overrides the package-level
	// MaxDecompressedPartSize for this Reader: it bounds how many bytes any
	// single part may decompress to. Zero uses the package-level default; a
	// negative value disables the bound.
	MaxDecompressedPartSize int64

	// MaxDecompressedPackageSize overrides the package-level
	// MaxDecompressedPackageSize for this Reader: it bounds the total bytes
	// the Reader may decompress across all parts. Zero uses the package-level
	// default; a negative value disables the bound.
	MaxDecompressedPackageSize int64

	// MaxPackageEntries overrides the package-level MaxPackageEntries for this
	// Reader: it bounds how many zip entries the package may contain. Zero
	// uses the package-level default; a negative value disables the bound.
	MaxPackageEntries int

	// MaxNestingDepth overrides the package-level MaxNestingDepth for this
	// Reader: it bounds how deeply elements may nest in any XML part. Zero
	// uses the package-level default; a negative value disables the bound.
	MaxNestingDepth int

	// AllowMissingDataIntegrity applies only to the encrypted-open paths
	// (OpenEncryptedWithOptions and the format packages' encrypted opens); the
	// plain-zip readers ignore it. It is passed straight through to
	// crypto.DecryptOptions.AllowMissingDataIntegrity: an agile-encrypted
	// package whose EncryptionInfo descriptor carries no dataIntegrity element
	// is decrypted WITHOUT verifying the package HMAC.
	//
	// Leave it false unless you know you need it. The descriptor is plaintext
	// and unauthenticated, so an attacker who can modify the file can delete
	// the dataIntegrity element as easily as they can flip bits in the
	// (malleable, CBC-mode) ciphertext; honoring its absence therefore turns an
	// authenticated format into an unauthenticated one at the attacker's
	// option. The default rejects it with crypto.ErrIntegrityCheckFailed. It
	// never relaxes a *failed* HMAC and never accepts a half-present
	// dataIntegrity block.
	AllowMissingDataIntegrity bool
}

// maxPackageEntries resolves the effective entry-count bound for one Reader:
// a non-zero option overrides the package-level default, and a negative value
// means unbounded (reported as 0, which every caller treats as disabled).
func (o ReaderOptions) maxPackageEntries() int {
	max := MaxPackageEntries
	if o.MaxPackageEntries != 0 {
		max = o.MaxPackageEntries
	}
	if max < 0 {
		return 0
	}
	return max
}

// newDecompressionBudget snapshots the limits for one Reader: each option
// field overrides the corresponding package-level variable when non-zero,
// with negative meaning unbounded (the budget treats <= 0 as disabled).
func newDecompressionBudget(opts ReaderOptions) *decompressionBudget {
	b := &decompressionBudget{
		maxPart:    MaxDecompressedPartSize,
		maxPackage: MaxDecompressedPackageSize,
		maxDepth:   MaxNestingDepth,
		charged:    make(map[*zip.File]int64),
	}
	if opts.MaxNestingDepth != 0 {
		b.maxDepth = opts.MaxNestingDepth
	}
	if opts.MaxDecompressedPartSize != 0 {
		b.maxPart = opts.MaxDecompressedPartSize
	}
	if opts.MaxDecompressedPackageSize != 0 {
		b.maxPackage = opts.MaxDecompressedPackageSize
	}
	return b
}

// declaredSize returns zf's declared uncompressed size clamped into int64, so
// a header claiming more than 2^63-1 bytes saturates instead of wrapping
// negative when it takes part in the budget arithmetic.
func declaredSize(zf *zip.File) int64 {
	if zf.UncompressedSize64 > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(zf.UncompressedSize64)
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

	// The package-total bound applies to the bytes this entry has not been
	// charged for yet: re-reading the already-charged prefix of a part cannot
	// grow the total, but everything past its high-water mark can.
	if b.maxPackage > 0 {
		pkgRemaining := b.maxPackage - b.total
		if pkgRemaining < 0 {
			pkgRemaining = 0
		}
		already := b.charged[zf]
		uncharged := declaredSize(zf) - already
		if uncharged < 0 {
			uncharged = 0
		}
		if uncharged > pkgRemaining {
			return 0, fmt.Errorf("opc: part %q declares %d bytes, which would exceed the %d-byte package decompression limit with %d bytes already decompressed (raise MaxDecompressedPackageSize before opening to allow it)", zf.Name, zf.UncompressedSize64, b.maxPackage, b.total)
		}
		// A read may legitimately reach the entry's high-water mark plus the
		// package's remaining budget before it would overrun the bound.
		if cap := already + pkgRemaining; cap >= 0 && (limit < 0 || cap < limit) {
			limit = cap
		}
	}
	return limit, nil
}

// charge records that n bytes have now been decompressed for zf and counts
// the delta above the entry's previous high-water mark toward the package
// total. It re-checks the remaining budget under the lock: the pre-check in
// admit used a snapshot, and other goroutines may have consumed budget since;
// this also catches entries whose actual size exceeds their declared size
// (lying local header).
func (b *decompressionBudget) charge(zf *zip.File, n int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxPackage <= 0 {
		return nil
	}
	delta := n - b.charged[zf]
	if delta <= 0 {
		// Nothing beyond what this entry was already charged for.
		return nil
	}
	if delta > b.maxPackage-b.total {
		return fmt.Errorf("opc: part %q exceeds the %d-byte package decompression limit with %d bytes already decompressed (raise MaxDecompressedPackageSize before opening to allow it)", zf.Name, b.maxPackage, b.total)
	}
	b.total += delta
	b.charged[zf] = n
	return nil
}

// chargedBytes reports how many bytes zf has been charged for. It exists for
// the accounting assertions in the tests.
func (b *decompressionBudget) chargedBytes(zf *zip.File) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.charged[zf]
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

	// Nesting is checked here rather than at unmarshal because this is the one
	// chokepoint every part's bytes pass through, and it is where the limits
	// captured for this Reader are in scope — common/xml is the substrate and
	// holds no policy. Only markup is scanned: media parts have no element
	// structure to nest, and skipping them keeps the cost off the bytes that
	// dominate a package.
	if isMarkupPart(zf.Name) {
		if err := xmlb.CheckNestingDepth(data, b.maxDepth); err != nil {
			return nil, fmt.Errorf("opc: part %q: %w (raise MaxNestingDepth before opening to allow it)", zf.Name, err)
		}
	}

	// Any read that overflowed the package-remaining portion of limit is
	// caught here: charge re-checks the budget under the lock and the total
	// only ever grows, so an over-read cannot slip through.
	if err := b.charge(zf, int64(len(data))); err != nil {
		return nil, err
	}
	return data, nil
}

// isMarkupPart reports whether a zip entry name looks like XML this package
// should depth-check. OPC part names are case-insensitive by convention, and
// the two extensions cover every markup part a package carries: .rels for
// relationship parts, .xml for everything else (including .vml parts, which are
// stored as .vml but only reached through drawing parts that are .xml).
func isMarkupPart(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels") ||
		strings.HasSuffix(lower, ".vml")
}

// openZipEntry opens a bounded stream over one zip entry. Declared-size
// violations are rejected immediately; violations only observable while
// decompressing (lying local headers) surface as Read errors from the
// returned stream. The stream charges the package budget as it is consumed,
// but only for bytes past the entry's high-water mark, so a part costs its
// own size however it is read and abandoning a stream leaves the untouched
// remainder still payable.
func (b *decompressionBudget) openZipEntry(zf *zip.File) (io.ReadCloser, error) {
	if _, err := b.admit(zf); err != nil {
		return nil, err
	}

	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}

	return &budgetedReadCloser{rc: rc, b: b, zf: zf, name: zf.Name}, nil
}

// budgetedReadCloser enforces the decompression limits on a streaming read
// of one zip entry without buffering it. Once the bytes decompressed through
// it exceed the per-part cap or the shared package budget, Read fails and
// the error is sticky.
type budgetedReadCloser struct {
	rc   io.ReadCloser
	b    *decompressionBudget
	zf   *zip.File
	name string

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
		if cerr := s.chargeStream(); cerr != nil {
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

// chargeStream raises the entry's charged high-water mark to the number of
// bytes this stream has decompressed, adding the delta to the package total
// and failing once the budget is exhausted. Unlike ReadAll, which knows the
// full part size up front, a stream charges as bytes arrive; charging the
// delta rather than a flat "already charged" flag is what keeps a partially
// consumed and abandoned stream from making the rest of the part free.
func (s *budgetedReadCloser) chargeStream() error {
	b := s.b
	if b.maxPackage <= 0 {
		return nil
	}
	b.mu.Lock()
	delta := s.streamed - b.charged[s.zf]
	if delta <= 0 {
		// Re-reading bytes another read of this entry already paid for.
		b.mu.Unlock()
		return nil
	}
	already := b.total
	over := delta > b.maxPackage-b.total
	if !over {
		b.total += delta
		b.charged[s.zf] = s.streamed
	}
	b.mu.Unlock()
	if over {
		return fmt.Errorf("opc: part %q exceeds the %d-byte package decompression limit with %d bytes already decompressed (raise MaxDecompressedPackageSize before opening to allow it)", s.name, b.maxPackage, already)
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

	// ExtendedProperties contains the extended properties (docProps/app.xml)
	// of the package, or nil when the package has none or they fail to parse.
	// Like Properties, the parse is best-effort and feeds the typed API only;
	// consumers that preserve parts byte-for-byte keep the raw app.xml part.
	ExtendedProperties *ExtendedProperties

	// CustomProperties contains the user-defined properties (docProps/custom.xml)
	// of the package, or nil when the package has none or they fail to parse.
	// Like the other property parses this is best-effort and feeds the typed
	// API only; consumers preserving parts byte-for-byte keep the raw part.
	CustomProperties *CustomProperties

	// DirectoryEntries lists the zip directory entries ("_rels/", "word/", …)
	// present in the source archive, in archive order, under their raw entry
	// names. OPC ignores directory entries, but some producers (WPS, Apache
	// POI, some Excel builds) emit them; a byte-faithful save re-emits the
	// same set via Writer.WriteDirectoryEntries, which canonicalizes each name
	// the same way this Reader does, so a producer that separates with
	// backslashes round-trips too.
	DirectoryEntries []string

	// DuplicateEntries lists the raw names of zip entries that collapsed onto
	// the canonical part name of an earlier entry and were therefore left out
	// of Files. OPC part names are unique and compare case-insensitively, so a
	// package declaring the same part twice is malformed; the first occurrence
	// wins (matching GetFile) and the rest are recorded here. Keeping them in
	// Files would contradict GetFile and make every replay save fail with
	// ErrDuplicatePart.
	DuplicateEntries []string

	zipReader *zip.Reader
	budget    *decompressionBudget

	// index memoizes the case-insensitive part lookups behind a pointer, so
	// copying a Reader (ReadCloser embeds one by value) shares the index and
	// does not copy a mutex.
	index *partIndex
}

// partIndex is the lazily built lookup index behind Reader.GetFile and
// Reader.GetRawZipFile. Both were linear case-folding scans, which every save
// and signing path walks once per part — O(n²) with a case-fold in the inner
// loop on a package with thousands of parts.
type partIndex struct {
	mu sync.Mutex

	// files maps a lowercased canonical part name to the first File carrying
	// it; nFiles records the len(Reader.Files) the map was built from, so a
	// caller that appends to the exported slice invalidates it.
	files  map[string]*File
	nFiles int

	// raw maps a lowercased canonical entry name to its zip entry.
	raw map[string]*zip.File

	// foldSensitive records that some indexed name differs under
	// strings.ToLower from what strings.EqualFold would match (i.e. it carries
	// non-ASCII letters). Only then does a lookup miss fall back to the exact
	// EqualFold scan the index replaced.
	filesFoldSensitive bool
	rawFoldSensitive   bool
}

// foldSensitive reports whether s contains a byte outside ASCII, for which
// strings.ToLower and strings.EqualFold can disagree.
func foldSensitive(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
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
	return OpenReaderWithOptions(path, ReaderOptions{})
}

// OpenReaderWithOptions opens an OPC package from a file path with per-Reader
// options (e.g. decompression limits overriding the package-level defaults).
func OpenReaderWithOptions(path string, opts ReaderOptions) (*ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	r, err := NewReaderWithOptions(f, fi.Size(), opts)
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	return &ReadCloser{Reader: *r, file: f}, nil
}

// guardedReaderAt wraps the caller's io.ReaderAt so a corrupt package that
// drives archive/zip to read at a negative offset — e.g. a zip64 entry whose
// local-header offset has the high bit set, which archive/zip stores as a
// negative int64 and then passes straight to ReadAt — fails with a clean,
// named package error instead of the underlying reader's raw
// "bytes.Reader.ReadAt: negative offset". Non-negative reads pass through
// untouched, so valid packages are unaffected.
type guardedReaderAt struct{ r io.ReaderAt }

func (g guardedReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("%w: zip entry has a negative offset (%d)", ErrCorruptedPackage, off)
	}
	return g.r.ReadAt(p, off)
}

// NewReader creates a Reader from an io.ReaderAt.
func NewReader(r io.ReaderAt, size int64) (*Reader, error) {
	return NewReaderWithOptions(r, size, ReaderOptions{})
}

// NewReaderWithOptions creates a Reader from an io.ReaderAt with per-Reader
// options. Unlike the package-level MaxDecompressedPartSize and
// MaxDecompressedPackageSize variables — which remain the documented defaults
// and require setup-time mutation — the options apply to this Reader alone,
// so concurrent opens with different limits need no global coordination.
func NewReaderWithOptions(r io.ReaderAt, size int64, opts ReaderOptions) (*Reader, error) {
	// A password-encrypted OOXML document is a CFB container, not a zip. Detect
	// it from the leading magic and steer the caller to OpenEncrypted instead
	// of failing with an opaque "not a valid zip file" error.
	if size >= int64(len(cfbSignature)) {
		var head [8]byte
		if n, _ := r.ReadAt(head[:], 0); n == len(head) && isCFB(head[:]) {
			return nil, ErrEncrypted
		}
	}

	zr, err := zip.NewReader(guardedReaderAt{r}, size)
	if err != nil {
		return nil, err
	}

	// Bound the entry count before building anything per entry (C459). The
	// byte-oriented decompression limits cannot see this dimension: every
	// entry costs a header, a name and a *File whether or not it is ever read.
	if max := opts.maxPackageEntries(); max > 0 && len(zr.File) > max {
		return nil, fmt.Errorf("opc: package contains %d entries, exceeding the %d-entry limit (raise MaxPackageEntries before opening to allow it)", len(zr.File), max)
	}

	reader := &Reader{
		zipReader: zr,
		Files:     make([]*File, 0, len(zr.File)),
		budget:    newDecompressionBudget(opts),
		index:     &partIndex{},
	}

	// First pass: find and parse [Content_Types].xml. The lookup goes through
	// the same canonicalization as every other entry, so a producer spelling
	// it "./[Content_Types].xml" is not reported as a corrupted package (C452).
	for _, zf := range zr.File {
		if strings.EqualFold(canonicalZipEntryName(zf.Name), contentTypesPartName) {
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
	seen := make(map[string]bool, len(zr.File))
	for _, zf := range zr.File {
		// Index parts under a canonical part name: zip producers (and hostile
		// packages) emit entry names like "./ppt/presentation.xml",
		// "word\document.xml", "a//b.xml" or "a/../b.xml", which would
		// otherwise be unreachable through GetFile and silently droppable on
		// round-trip. canonicalZipEntryName is the one normalization used at
		// every boundary — it agrees with NormalizePartName, which is what
		// GetFile runs its query through. The zip entry itself keeps its
		// original raw name, so GetRawZipFile still finds the original bytes.
		name := canonicalZipEntryName(zf.Name)

		// Directory entries carry no part data, but record them so a save can
		// reproduce the source archive's directory listing. The raw name is
		// recorded; Writer.WriteDirectoryEntries canonicalizes it the same way
		// before emitting, so a backslash-separated directory entry is not
		// silently dropped on write (C456).
		if strings.HasSuffix(name, "/") {
			reader.DirectoryEntries = append(reader.DirectoryEntries, zf.Name)
			continue
		}

		// Skip special files
		if strings.EqualFold(name, contentTypesPartName) {
			continue
		}

		// Two entries collapsing onto one part name is malformed; the first
		// wins, matching GetFile and this package's documented rule. Keeping
		// both in Files contradicted that rule and made every replay save fail
		// with ErrDuplicatePart (C395).
		key := strings.ToLower(name)
		if seen[key] {
			reader.DuplicateEntries = append(reader.DuplicateEntries, zf.Name)
			continue
		}
		seen[key] = true

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

	// Parse core, extended, and custom properties if they exist
	reader.parseCoreProperties()
	reader.parseExtendedProperties()
	reader.parseCustomProperties()

	return reader, nil
}

// canonicalZipEntryName converts a raw zip entry name into the canonical
// leading-slash part name used for lookups: backslash separators become
// forward slashes and the result is path.Clean-ed, which strips "." segments,
// collapses empty segments ("//") and resolves ".." — including a leading
// ".." that would otherwise escape the package root. Cleaning here is what
// makes this function agree with NormalizePartName, the normalization GetFile
// runs its query through: without it an entry named "a/../b.xml" was reachable
// under no name at all and could not be written back out, so a package
// carrying one could not round-trip (C390).
//
// A trailing slash survives cleaning: it is the only marker distinguishing a
// zip directory entry from a part.
//
// The original raw entry name is untouched — it remains the name under which
// the entry's bytes were stored, and GetRawZipFile canonicalizes both sides of
// its comparison so a raw name still resolves.
func canonicalZipEntryName(name string) string {
	s := strings.ReplaceAll(name, `\`, "/")
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	dir := len(s) > 1 && strings.HasSuffix(s, "/")
	s = path.Clean(s)
	if dir && !strings.HasSuffix(s, "/") {
		s += "/"
	}
	return s
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

// parseExtendedProperties reads the extended properties (app.xml) if they
// exist. Like parseCoreProperties it is best-effort: a missing part or a
// parse failure leaves ExtendedProperties nil rather than failing the open.
func (r *Reader) parseExtendedProperties() {
	for _, rel := range r.Relationships {
		if rel.Type != RelTypeExtended {
			continue
		}
		target := ResolvePartName("/", rel.Target)
		f := r.GetFile(target)
		if f == nil {
			continue
		}

		data, err := f.ReadAll()
		if err != nil {
			continue
		}

		props, err := UnmarshalExtendedProperties(data)
		if err != nil {
			continue
		}

		r.ExtendedProperties = props
		return
	}
}

// parseCustomProperties reads the custom properties (docProps/custom.xml) if
// they exist. Like the other property parses it is best-effort: a missing part
// or a parse failure leaves CustomProperties nil rather than failing the open.
func (r *Reader) parseCustomProperties() {
	for _, rel := range r.Relationships {
		if rel.Type != RelTypeCustom {
			continue
		}
		target := ResolvePartName("/", rel.Target)
		f := r.GetFile(target)
		if f == nil {
			continue
		}

		data, err := f.ReadAll()
		if err != nil {
			continue
		}

		props, err := UnmarshalCustomProperties(data)
		if err != nil {
			continue
		}

		r.CustomProperties = props
		return
	}
}

// GetFile returns the file with the given path, or nil if not found. Part
// names compare case-insensitively, as OPC requires. Lookups are served from a
// lazily built index rather than a linear scan: every save and signing path
// resolves parts inside a loop over the parts, which made the whole traversal
// quadratic.
func (r *Reader) GetFile(name string) *File {
	normalizedName := NormalizePartName(name)
	if r.index == nil {
		// A Reader assembled by a caller rather than by NewReader.
		return lookupFileLinear(r.Files, normalizedName)
	}

	r.index.mu.Lock()
	if r.index.files == nil || r.index.nFiles != len(r.Files) {
		m := make(map[string]*File, len(r.Files))
		fold := false
		for _, f := range r.Files {
			if f == nil {
				continue
			}
			if foldSensitive(f.Name) {
				fold = true
			}
			key := strings.ToLower(f.Name)
			if _, ok := m[key]; !ok {
				m[key] = f // first wins, matching the scan it replaces
			}
		}
		r.index.files = m
		r.index.nFiles = len(r.Files)
		r.index.filesFoldSensitive = fold
	}
	f, ok := r.index.files[strings.ToLower(normalizedName)]
	fold := r.index.filesFoldSensitive
	r.index.mu.Unlock()

	if ok {
		return f
	}
	if fold {
		// Some name folds differently under ToLower than under EqualFold
		// (non-ASCII letters); fall back to the exact comparison.
		return lookupFileLinear(r.Files, normalizedName)
	}
	return nil
}

// lookupFileLinear is the exact EqualFold scan the index shortcuts.
func lookupFileLinear(files []*File, normalizedName string) *File {
	for _, f := range files {
		if f != nil && strings.EqualFold(f.Name, normalizedName) {
			return f
		}
	}
	return nil
}

// GetRawZipFile returns the raw data for a file in the zip archive by name.
// This can be used to access special files like [Content_Types].xml that are
// not included in the Files list. The name is matched through the same
// canonicalization as Files, so both the raw entry name and the canonical part
// name resolve. Like GetFile it is served from a lazily built index.
func (r *Reader) GetRawZipFile(name string) ([]byte, error) {
	zf := r.rawZipEntry(name)
	if zf == nil {
		return nil, fmt.Errorf("file not found: %s", name)
	}
	return r.budget.readZipEntry(zf)
}

// rawZipEntry resolves a raw or canonical entry name to its zip entry.
func (r *Reader) rawZipEntry(name string) *zip.File {
	if r.zipReader == nil {
		return nil
	}
	canonical := canonicalZipEntryName(name)
	if r.index == nil {
		return lookupRawLinear(r.zipReader.File, canonical)
	}

	r.index.mu.Lock()
	if r.index.raw == nil {
		m := make(map[string]*zip.File, len(r.zipReader.File))
		fold := false
		for _, zf := range r.zipReader.File {
			c := canonicalZipEntryName(zf.Name)
			if foldSensitive(c) {
				fold = true
			}
			key := strings.ToLower(c)
			if _, ok := m[key]; !ok {
				m[key] = zf // first wins, matching the scan it replaces
			}
		}
		r.index.raw = m
		r.index.rawFoldSensitive = fold
	}
	zf, ok := r.index.raw[strings.ToLower(canonical)]
	fold := r.index.rawFoldSensitive
	r.index.mu.Unlock()

	if ok {
		return zf
	}
	if fold {
		return lookupRawLinear(r.zipReader.File, canonical)
	}
	return nil
}

// lookupRawLinear is the exact EqualFold scan rawZipEntry shortcuts.
func lookupRawLinear(entries []*zip.File, canonical string) *zip.File {
	for _, zf := range entries {
		if strings.EqualFold(canonicalZipEntryName(zf.Name), canonical) {
			return zf
		}
	}
	return nil
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
