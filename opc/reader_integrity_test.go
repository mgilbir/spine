package opc

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
)

// --- C376: the decompression budget must count bytes, not entries ----------

// TestBudgetChargesAbandonedStreamBytes is the regression test for C376.
// openZipEntry used to mark an entry "charged" the moment a stream was opened,
// so reading a single byte and dropping the stream made the whole part free:
// a later ReadAll of the same entry decompressed it in full for zero
// additional budget, and repeating the trick across parts made
// MaxDecompressedPackageSize meaningless. The assertion is on the *charged
// byte accounting*, not merely on some eventual error.
func TestBudgetChargesAbandonedStreamBytes(t *testing.T) {
	data, openCost := twoPartPackage(t)

	// The audit's measurement: a package budget with room for one 2000-byte
	// part but not both.
	withDecompressionLimits(t, 1<<20, openCost+2500)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	first := reader.GetFile("/ppt/presentation.xml")
	second := reader.GetFile("/ppt/media/image1.png")
	if first == nil || second == nil {
		t.Fatal("test package is missing one of its parts")
	}

	// Open a stream, take one byte, abandon it.
	rc, err := first.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	var one [1]byte
	if n, err := io.ReadFull(rc, one[:]); n != 1 || err != nil {
		t.Fatalf("reading one byte = (%d, %v), want (1, nil)", n, err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := reader.budget.chargedBytes(first.zipFile); got != 1 {
		t.Errorf("after streaming 1 byte the entry is charged %d bytes, want 1", got)
	}

	// Now decompress the whole part. Under the boolean flag this was free.
	body, err := first.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(body) != 2000 {
		t.Fatalf("ReadAll() returned %d bytes, want 2000", len(body))
	}
	if got := reader.budget.chargedBytes(first.zipFile); got != 2000 {
		t.Errorf("after decompressing the whole 2000-byte part it is charged %d bytes, want 2000 (C376: the abandoned stream must not make the remainder free)", got)
	}

	// The full accounting must reflect every decompressed byte, so the second
	// part no longer fits the package budget.
	if want := openCost + 2000; reader.budget.total != want {
		t.Errorf("package total = %d, want %d (open cost %d + the 2000-byte part)", reader.budget.total, want, openCost)
	}
	if _, err := second.ReadAll(); err == nil {
		t.Fatal("expected the second part to bust the package budget, got nil error")
	} else if !strings.Contains(err.Error(), "package decompression limit") {
		t.Errorf("expected a package-decompression-limit error, got: %v", err)
	}
}

// TestBudgetChargesEachPartAtMostOnce guards the other half of the C376
// contract: charging deltas must not turn into charging every read. A part
// read three different ways still costs its own size exactly once.
func TestBudgetChargesEachPartAtMostOnce(t *testing.T) {
	data, openCost := twoPartPackage(t)
	withDecompressionLimits(t, 1<<20, openCost+2500)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	first := reader.GetFile("/ppt/presentation.xml")

	if _, err := first.ReadAll(); err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	rc, err := first.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Fatalf("streaming an already-charged part error = %v", err)
	}
	_ = rc.Close()
	if _, err := first.ReadAll(); err != nil {
		t.Fatalf("second ReadAll() error = %v", err)
	}

	if want := openCost + 2000; reader.budget.total != want {
		t.Errorf("package total = %d after reading one 2000-byte part three times, want %d", reader.budget.total, want)
	}
}

// TestBudgetAbandonedStreamsAcrossPartsStillBounded is the amplification the
// audit measured: N parts each opened, read one byte from, and abandoned, then
// all read in full. The package bound must still hold.
func TestBudgetAbandonedStreamsAcrossPartsStillBounded(t *testing.T) {
	parts := map[string][]byte{
		"/ppt/a.bin": bytes.Repeat([]byte("A"), 2000),
		"/ppt/b.bin": bytes.Repeat([]byte("B"), 2000),
		"/ppt/c.bin": bytes.Repeat([]byte("C"), 2000),
		"/ppt/d.bin": bytes.Repeat([]byte("D"), 2000),
	}
	data := createTestPackage(t, parts, nil)

	MaxDecompressedPartSize, MaxDecompressedPackageSize = 1<<20, 1<<30
	probe, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	openCost := probe.budget.total

	// Room for two of the four parts.
	withDecompressionLimits(t, 1<<20, openCost+4500)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	names := []string{"/ppt/a.bin", "/ppt/b.bin", "/ppt/c.bin", "/ppt/d.bin"}
	for _, name := range names {
		rc, err := reader.GetFile(name).Open()
		if err != nil {
			t.Fatalf("Open(%s) error = %v", name, err)
		}
		var one [1]byte
		if _, err := io.ReadFull(rc, one[:]); err != nil {
			t.Fatalf("reading one byte of %s error = %v", name, err)
		}
		_ = rc.Close()
	}

	var decompressed int64
	var failed bool
	for _, name := range names {
		body, err := reader.GetFile(name).ReadAll()
		if err != nil {
			failed = true
			break
		}
		decompressed += int64(len(body))
	}
	if !failed {
		t.Fatal("all four parts decompressed in full under a two-part budget (C376)")
	}
	if decompressed > 4500 {
		t.Errorf("decompressed %d bytes under a %d-byte remaining budget", decompressed, 4500)
	}
}

// --- C459: entry-count bound ----------------------------------------------

// TestMaxPackageEntries bounds the one dimension the byte limits cannot see.
func TestMaxPackageEntries(t *testing.T) {
	parts := map[string][]byte{}
	for i := range 40 {
		parts[fmt.Sprintf("/ppt/p%d.bin", i)] = []byte("x")
	}
	data := createTestPackage(t, parts, nil)

	if _, err := NewReaderWithOptions(bytes.NewReader(data), int64(len(data)), ReaderOptions{MaxPackageEntries: 10}); err == nil {
		t.Fatal("expected a package with more entries than the bound to be rejected")
	} else if !strings.Contains(err.Error(), "MaxPackageEntries") {
		t.Errorf("expected the error to name the knob, got: %v", err)
	}

	// Generous bound and disabled bound both open it.
	if _, err := NewReaderWithOptions(bytes.NewReader(data), int64(len(data)), ReaderOptions{MaxPackageEntries: 1000}); err != nil {
		t.Errorf("NewReaderWithOptions(bound=1000) error = %v", err)
	}
	if _, err := NewReaderWithOptions(bytes.NewReader(data), int64(len(data)), ReaderOptions{MaxPackageEntries: -1}); err != nil {
		t.Errorf("NewReaderWithOptions(bound disabled) error = %v", err)
	}
	// The package-level default is generous enough for a real package.
	if _, err := NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Errorf("NewReader() with the default bound error = %v", err)
	}
}

// --- C390 / C452 / C395 / C456: one canonicalization at every boundary -----

// rawZipPackage builds a zip with verbatim entry names, bypassing the Writer's
// part-name rules, so hostile and sloppy producer spellings can be exercised.
func rawZipPackage(t *testing.T, entries [][2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: e[0], Method: zip.Store})
		if err != nil {
			t.Fatalf("CreateHeader(%q) error = %v", e[0], err)
		}
		if _, err := w.Write([]byte(e[1])); err != nil {
			t.Fatalf("Write(%q) error = %v", e[0], err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close() error = %v", err)
	}
	return buf.Bytes()
}

const minimalContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`

// TestCanonicalZipEntryNameCleansDotDot is the regression test for C390: an
// entry named "a/../b.xml" used to be reachable under no name at all — neither
// its raw spelling nor its cleaned one — and could not be written back out, so
// a package carrying one could not round-trip.
func TestCanonicalZipEntryNameCleansDotDot(t *testing.T) {
	data := rawZipPackage(t, [][2]string{
		{"[Content_Types].xml", minimalContentTypes},
		{"a/../b.xml", "<b/>"},
		{"../escape.xml", "<e/>"},
	})

	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	var names []string
	for _, f := range r.Files {
		names = append(names, f.Name)
	}
	for _, want := range []string{"/b.xml", "/escape.xml"} {
		if !slices.Contains(names, want) {
			t.Errorf("Files = %v, want a part named %q", names, want)
		}
	}

	// Every spelling resolves, and each part is reachable.
	for _, q := range []string{"/a/../b.xml", "/b.xml", "b.xml"} {
		if r.GetFile(q) == nil {
			t.Errorf("GetFile(%q) = nil, want the cleaned part", q)
		}
	}
	for _, q := range []string{"/../escape.xml", "/escape.xml"} {
		if r.GetFile(q) == nil {
			t.Errorf("GetFile(%q) = nil, want the cleaned part", q)
		}
	}

	// And the canonical name is savable, so the package round-trips.
	var out bytes.Buffer
	w := NewWriter(&out)
	for _, f := range r.Files {
		body, err := f.ReadAll()
		if err != nil {
			t.Fatalf("ReadAll(%s) error = %v", f.Name, err)
		}
		if err := w.WritePreservedPart(f.Name, f.ContentType, body); err != nil {
			t.Fatalf("WritePreservedPart(%s) error = %v", f.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := NewReader(bytes.NewReader(out.Bytes()), int64(out.Len())); err != nil {
		t.Fatalf("reopening the round-tripped package error = %v", err)
	}
}

// TestContentTypesFoundUnderCanonicalName is the regression test for C452: the
// mandatory part was located by raw-name EqualFold only, so the "./" spelling
// C51 added canonicalization for made NewReader report a corrupted package.
func TestContentTypesFoundUnderCanonicalName(t *testing.T) {
	for _, name := range []string{"./[Content_Types].xml", `.\[Content_Types].xml`, "//[Content_Types].xml"} {
		data := rawZipPackage(t, [][2]string{
			{name, minimalContentTypes},
			{"word/document.xml", "<d/>"},
		})
		r, err := NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Errorf("NewReader() with [Content_Types].xml spelled %q error = %v", name, err)
			continue
		}
		if r.GetFile("/word/document.xml") == nil {
			t.Errorf("with [Content_Types].xml spelled %q, the document part is unreachable", name)
		}
		// The special entry must not leak into Files under any spelling.
		for _, f := range r.Files {
			if strings.EqualFold(f.Name, contentTypesPartName) {
				t.Errorf("with spelling %q, [Content_Types].xml leaked into Files as %q", name, f.Name)
			}
		}
	}
}

// TestDuplicateEntriesDoNotDuplicateFiles is the regression test for C395: the
// comment promised "the first wins (GetFile returns the first match)" while
// Files carried both, so any replay save failed with ErrDuplicatePart.
func TestDuplicateEntriesDoNotDuplicateFiles(t *testing.T) {
	data := rawZipPackage(t, [][2]string{
		{"[Content_Types].xml", minimalContentTypes},
		{"part.xml", "<first/>"},
		{"part.xml", "<second/>"},
	})

	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if len(r.Files) != 1 {
		var names []string
		for _, f := range r.Files {
			names = append(names, f.Name)
		}
		t.Fatalf("Files = %v (%d entries), want exactly one: the first occurrence wins", names, len(r.Files))
	}
	if len(r.DuplicateEntries) != 1 || r.DuplicateEntries[0] != "part.xml" {
		t.Errorf("DuplicateEntries = %v, want the dropped collision recorded", r.DuplicateEntries)
	}
	body, err := r.Files[0].ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "<first/>" {
		t.Errorf("kept entry = %q, want the first occurrence", body)
	}

	// A replay save must now succeed instead of failing "duplicate part".
	var out bytes.Buffer
	w := NewWriter(&out)
	for _, f := range r.Files {
		b, err := f.ReadAll()
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if err := w.WritePreservedPart(f.Name, f.ContentType, b); err != nil {
			t.Fatalf("WritePreservedPart(%s) error = %v", f.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// TestDirectoryEntriesWithBackslashRoundTrip is the regression test for C456:
// a backslash-separated directory entry was recorded on read and silently
// skipped on write.
func TestDirectoryEntriesWithBackslashRoundTrip(t *testing.T) {
	data := rawZipPackage(t, [][2]string{
		{"[Content_Types].xml", minimalContentTypes},
		{`word\`, ""},
		{"docProps/", ""},
		{"word/document.xml", "<d/>"},
	})

	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if len(r.DirectoryEntries) != 2 {
		t.Fatalf("DirectoryEntries = %v, want both directory entries recorded", r.DirectoryEntries)
	}

	var out bytes.Buffer
	w := NewWriter(&out)
	if err := w.WriteDirectoryEntries(r.DirectoryEntries); err != nil {
		t.Fatalf("WriteDirectoryEntries() error = %v", err)
	}
	if err := w.WritePreservedPart("/word/document.xml", "", []byte("<d/>")); err != nil {
		t.Fatalf("WritePreservedPart() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	var dirs []string
	for _, zf := range zr.File {
		if strings.HasSuffix(zf.Name, "/") {
			dirs = append(dirs, zf.Name)
		}
	}
	if len(dirs) != 2 {
		t.Errorf("saved directory entries = %v, want both re-emitted (C456)", dirs)
	}
	if !slices.Contains(dirs, "word/") {
		t.Errorf("saved directory entries = %v, want the backslash-separated entry emitted as %q", dirs, "word/")
	}
}

// TestGetRawZipFileAcceptsCanonicalName pins the third notion of a part name
// to the same canonicalization (T3): the raw entry name and the canonical part
// name must both resolve.
func TestGetRawZipFileAcceptsCanonicalName(t *testing.T) {
	data := rawZipPackage(t, [][2]string{
		{"[Content_Types].xml", minimalContentTypes},
		{"./word/document.xml", "<d/>"},
	})
	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	for _, q := range []string{"./word/document.xml", "/word/document.xml", "word/document.xml"} {
		b, err := r.GetRawZipFile(q)
		if err != nil {
			t.Errorf("GetRawZipFile(%q) error = %v", q, err)
			continue
		}
		if string(b) != "<d/>" {
			t.Errorf("GetRawZipFile(%q) = %q, want the part bytes", q, b)
		}
	}
	if _, err := r.GetRawZipFile("/nope.xml"); err == nil {
		t.Error("GetRawZipFile of a missing entry returned no error")
	}
}

// BenchmarkGetFile measures the per-part lookup cost that made every save and
// signing path quadratic (C457).
func BenchmarkGetFile(b *testing.B) {
	parts := map[string][]byte{}
	names := make([]string, 0, 5000)
	for i := range 5000 {
		n := fmt.Sprintf("/xl/worksheets/sheet%d.xml", i)
		parts[n] = []byte("<w/>")
		names = append(names, n)
	}
	data := createTestPackageB(b, parts)

	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		b.Fatalf("NewReader() error = %v", err)
	}
	b.ResetTimer()
	for b.Loop() {
		// The shape the audit describes: an O(n) lookup inside an O(n) loop.
		for _, n := range names {
			if r.GetFile(n) == nil {
				b.Fatalf("GetFile(%s) = nil", n)
			}
		}
	}
}

// createTestPackageB is createTestPackage for benchmarks.
func createTestPackageB(b *testing.B, parts map[string][]byte) []byte {
	b.Helper()
	var buf bytes.Buffer
	w := NewWriter(&buf)
	for name, data := range parts {
		if err := w.WritePart(name, "", data); err != nil {
			b.Fatalf("WritePart(%s) error = %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		b.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}
