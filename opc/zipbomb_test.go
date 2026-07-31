package opc

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// TestMaxDecompressedPartSize_LimitsPartRead verifies that reading a part whose
// decompressed size exceeds MaxDecompressedPartSize fails instead of buffering
// unbounded data (C52: decompression-bomb guard).
func TestMaxDecompressedPartSize_LimitsPartRead(t *testing.T) {
	// A highly compressible large part.
	big := bytes.Repeat([]byte("A"), 64*1024)
	parts := map[string][]byte{
		"/ppt/presentation.xml": big,
	}
	contentTypes := map[string]string{
		"/ppt/presentation.xml": ContentTypePresentationMain,
	}
	data := createTestPackage(t, parts, contentTypes)

	opts := []ReaderOption{WithMaxDecompressedPartSize(1024)}

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	f := reader.GetFile("/ppt/presentation.xml")
	if f == nil {
		t.Fatal("part not found")
	}
	if _, err := f.ReadAll(); err == nil {
		t.Fatal("expected ReadAll to reject an oversized part, got nil error")
	} else if !strings.Contains(err.Error(), "decompression limit") {
		t.Errorf("expected a decompression-limit error, got: %v", err)
	}
}

// TestMaxDecompressedPartSize_AllowsSmallParts verifies the guard does not
// reject legitimately small parts.
func TestMaxDecompressedPartSize_AllowsSmallParts(t *testing.T) {
	parts := map[string][]byte{
		"/ppt/presentation.xml": []byte("<presentation/>"),
	}
	contentTypes := map[string]string{
		"/ppt/presentation.xml": ContentTypePresentationMain,
	}
	data := createTestPackage(t, parts, contentTypes)

	opts := []ReaderOption{WithMaxDecompressedPartSize(1024)}

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	got, err := reader.GetFile("/ppt/presentation.xml").ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != "<presentation/>" {
		t.Errorf("unexpected content: %q", got)
	}
}

// decompressionLimits returns options setting both decompression limits. They
// are per-call now, so nothing has to be saved and restored and tests can run
// with different limits without coordinating.
func decompressionLimits(part, pkg int64) []ReaderOption {
	return []ReaderOption{
		WithMaxDecompressedPartSize(part),
		WithMaxDecompressedPackageSize(pkg),
	}
}

// twoPartPackage builds a package with two parts of 2000 bytes each and
// returns its bytes plus the number of bytes NewReader decompresses just to
// open it ([Content_Types].xml and /_rels/.rels).
func twoPartPackage(t *testing.T) (data []byte, openCost int64) {
	t.Helper()
	parts := map[string][]byte{
		"/ppt/presentation.xml": bytes.Repeat([]byte("A"), 2000),
		"/ppt/media/image1.png": bytes.Repeat([]byte("B"), 2000),
	}
	contentTypes := map[string]string{
		"/ppt/presentation.xml": ContentTypePresentationMain,
	}
	data = createTestPackage(t, parts, contentTypes)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	return data, reader.budget.total
}

// TestMaxDecompressedPackageSize_LimitsTotal verifies that parts which each
// fit under the per-part cap are still rejected once their cumulative
// decompressed size exceeds MaxDecompressedPackageSize, and that re-reading
// an already-read part does not consume additional budget (C169).
func TestMaxDecompressedPackageSize_LimitsTotal(t *testing.T) {
	data, openCost := twoPartPackage(t)

	// Room for opening the package plus one 2000-byte part, but not two.
	opts := decompressionLimits(1<<20, openCost+2500)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	first := reader.GetFile("/ppt/presentation.xml")
	if _, err := first.ReadAll(); err != nil {
		t.Fatalf("first part ReadAll() error = %v", err)
	}
	// Re-reading the same part is charged only once.
	if _, err := first.ReadAll(); err != nil {
		t.Fatalf("repeated ReadAll() of the same part error = %v", err)
	}

	_, err = reader.GetFile("/ppt/media/image1.png").ReadAll()
	if err == nil {
		t.Fatal("expected ReadAll to reject the part that busts the package total, got nil error")
	}
	if !strings.Contains(err.Error(), "package decompression limit") {
		t.Errorf("expected a package-decompression-limit error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "image1.png") {
		t.Errorf("expected the error to name the offending part, got: %v", err)
	}
	if !strings.Contains(err.Error(), "MaxDecompressedPackageSize") {
		t.Errorf("expected the error to say how to raise the limit, got: %v", err)
	}
}

// TestMaxDecompressedPackageSize_RejectsAtOpen verifies that a package whose
// mandatory parts already exceed the package total fails at NewReader.
func TestMaxDecompressedPackageSize_RejectsAtOpen(t *testing.T) {
	data, _ := twoPartPackage(t)

	opts := decompressionLimits(1<<20, 10)

	if _, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...); err == nil {
		t.Fatal("expected NewReader to fail under a tiny package limit, got nil error")
	} else if !strings.Contains(err.Error(), "package decompression limit") {
		t.Errorf("expected a package-decompression-limit error, got: %v", err)
	}
}

// TestMaxDecompressedPackageSize_AllowsUnderLimit verifies that a package
// comfortably under both limits reads all parts unaffected.
func TestMaxDecompressedPackageSize_AllowsUnderLimit(t *testing.T) {
	data, _ := twoPartPackage(t)

	opts := decompressionLimits(1<<20, 1<<20)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	for _, f := range reader.Files {
		if _, err := f.ReadAll(); err != nil {
			t.Errorf("ReadAll(%s) error = %v", f.Name, err)
		}
	}
}

// TestMaxDecompressedPackageSize_DisabledWithZero verifies that setting the
// package total to 0 disables the bound.
func TestMaxDecompressedPackageSize_DisabledWithZero(t *testing.T) {
	data, _ := twoPartPackage(t)

	opts := decompressionLimits(1<<20, 0)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	for _, f := range reader.Files {
		if _, err := f.ReadAll(); err != nil {
			t.Errorf("ReadAll(%s) error = %v", f.Name, err)
		}
	}
}

// The limits used to be package-level variables captured when a Reader was
// built, and a test here asserted that capture: raising a global afterwards had
// to leave an open Reader alone. Options are resolved per call now, so there is
// no global to raise and nothing to snapshot — the property is structural
// rather than something a test can violate.

// TestDecompressionBudget_ConcurrentReadAll verifies that concurrent ReadAll
// calls on distinct (and repeated) parts of one Reader are safe: the shared
// budget's running total and charged map are mutex-guarded (C181). Run under
// -race to exercise the synchronization.
func TestDecompressionBudget_ConcurrentReadAll(t *testing.T) {
	const numParts = 64
	const partSize = 1000
	parts := make(map[string][]byte, numParts)
	for i := 0; i < numParts; i++ {
		parts[fmt.Sprintf("/ppt/part%d.xml", i)] = bytes.Repeat([]byte("A"), partSize)
	}
	data := createTestPackage(t, parts, nil)

	opts := decompressionLimits(1<<20, 1<<20)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	openCost := reader.budget.total

	var wg sync.WaitGroup
	for _, f := range reader.Files {
		// Repeated reads of the same part exercise charge-once under contention.
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func(f *File) {
				defer wg.Done()
				if _, err := f.ReadAll(); err != nil {
					t.Errorf("ReadAll(%s) error = %v", f.Name, err)
				}
			}(f)
		}
	}
	wg.Wait()

	// Every part must have been charged exactly once despite the fan-out
	// (the package .rels was already charged during NewReader).
	want := openCost + numParts*partSize
	if got := reader.budget.total; got != want {
		t.Errorf("budget total after concurrent reads = %d, want %d", got, want)
	}
}

// TestFileOpen_RejectsDeclaredOversizedPart verifies that File.Open honors
// the per-part cap for parts whose declared size already exceeds it (C183:
// Open used to bypass both decompression limits entirely).
func TestFileOpen_RejectsDeclaredOversizedPart(t *testing.T) {
	big := bytes.Repeat([]byte("A"), 64*1024)
	data := createTestPackage(t, map[string][]byte{"/ppt/presentation.xml": big}, nil)

	opts := decompressionLimits(1024, 1<<20)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	_, err = reader.GetFile("/ppt/presentation.xml").Open()
	if err == nil {
		t.Fatal("expected Open to reject a declared-oversized part, got nil error")
	}
	if !strings.Contains(err.Error(), "per-part decompression limit") {
		t.Errorf("expected a per-part decompression-limit error, got: %v", err)
	}
}

// TestBudgetedReadCloser_EnforcesPartLimitMidStream exercises the streaming
// per-part cap on the wrapper directly (C183). archive/zip itself stops an
// entry that overruns its declared size, so through File.Open this guard is
// defense in depth against sources the zip layer does not police; here it is
// driven with a raw stream to prove it fails at most one byte past the cap.
func TestBudgetedReadCloser_EnforcesPartLimitMidStream(t *testing.T) {
	b := &decompressionBudget{maxPart: 1024, charged: make(map[*zip.File]int64)}
	src := io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("A"), 64*1024)))
	rc := &budgetedReadCloser{rc: src, b: b, name: "ppt/bomb.bin"}

	n, err := io.Copy(io.Discard, rc)
	if err == nil {
		t.Fatal("expected the stream to fail at the per-part cap, got nil error")
	}
	if !strings.Contains(err.Error(), "per-part decompression limit") {
		t.Errorf("expected a per-part decompression-limit error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bomb.bin") {
		t.Errorf("expected the error to name the offending part, got: %v", err)
	}
	if !strings.Contains(err.Error(), "MaxDecompressedPartSize") {
		t.Errorf("expected the error to say how to raise the limit, got: %v", err)
	}
	if n > 1024+1 {
		t.Errorf("stream decompressed %d bytes, want at most one byte past the 1024-byte cap", n)
	}
	// The error is sticky.
	if _, err := rc.Read(make([]byte, 1)); err == nil {
		t.Error("expected reads after a limit violation to keep failing")
	}
}

// TestFileOpen_StreamEnforcesPackageLimit verifies that bytes streamed via
// File.Open are charged against the package budget and that a stream fails
// once the budget is exhausted (C183). Two streams are opened before either
// is consumed: each part alone passes the declared-size pre-check, so only
// the incremental charging can stop the second one.
func TestFileOpen_StreamEnforcesPackageLimit(t *testing.T) {
	data, openCost := twoPartPackage(t)

	// Room for one 2000-byte part but not both.
	opts := decompressionLimits(1<<20, openCost+3000)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	a, err := reader.GetFile("/ppt/presentation.xml").Open()
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	defer func() { _ = a.Close() }()
	b, err := reader.GetFile("/ppt/media/image1.png").Open()
	if err != nil {
		t.Fatalf("Open(second) error = %v (nothing streamed yet, so it must pass the pre-check)", err)
	}
	defer func() { _ = b.Close() }()

	if n, err := io.Copy(io.Discard, a); err != nil || n != 2000 {
		t.Fatalf("streaming the first part = (%d, %v), want (2000, nil)", n, err)
	}

	if _, err := io.Copy(io.Discard, b); err == nil {
		t.Fatal("expected the second stream to fail at the package budget, got nil error")
	} else if !strings.Contains(err.Error(), "package decompression limit") {
		t.Errorf("expected a package-decompression-limit error, got: %v", err)
	} else if !strings.Contains(err.Error(), "MaxDecompressedPackageSize") {
		t.Errorf("expected the error to say how to raise the limit, got: %v", err)
	}
}

// TestFileOpen_ChargesOnce verifies that a part fully streamed via Open is
// charged against the package budget exactly once: a later ReadAll of the
// same part is free, while a different part that no longer fits is rejected
// at Open (C183, matching ReadAll's charge-once semantics).
func TestFileOpen_ChargesOnce(t *testing.T) {
	data, openCost := twoPartPackage(t)

	opts := decompressionLimits(1<<20, openCost+2500)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	first := reader.GetFile("/ppt/presentation.xml")
	rc, err := first.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if n, err := io.Copy(io.Discard, rc); err != nil || n != 2000 {
		t.Fatalf("streaming the first part = (%d, %v), want (2000, nil)", n, err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Re-reading an already-streamed part consumes no additional budget.
	if _, err := first.ReadAll(); err != nil {
		t.Fatalf("ReadAll() after streaming the same part error = %v", err)
	}

	// The second part's declared size no longer fits the remaining budget.
	if _, err := reader.GetFile("/ppt/media/image1.png").Open(); err == nil {
		t.Fatal("expected Open to reject the part that busts the package total, got nil error")
	} else if !strings.Contains(err.Error(), "package decompression limit") {
		t.Errorf("expected a package-decompression-limit error, got: %v", err)
	}
}

// TestReaderOptions_OverridesGlobalLimits verifies that per-Reader options
// override the package-level decompression limits (C169): a stricter per-call
// part limit rejects a part the global would allow, and a negative option
// disables the bound for that Reader alone, leaving the globals untouched.
func TestReaderOptions_OverridesGlobalLimits(t *testing.T) {
	big := bytes.Repeat([]byte("A"), 64*1024)
	parts := map[string][]byte{
		"/ppt/presentation.xml": big,
	}
	contentTypes := map[string]string{
		"/ppt/presentation.xml": ContentTypePresentationMain,
	}
	data := createTestPackage(t, parts, contentTypes)

	// Stricter than the global default: the part must be rejected.
	reader, err := NewReader(bytes.NewReader(data), int64(len(data)),
		WithMaxDecompressedPartSize(1024))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	f := reader.GetFile("/ppt/presentation.xml")
	if f == nil {
		t.Fatal("part not found")
	}
	if _, err := f.ReadAll(); err == nil {
		t.Fatal("expected ReadAll to reject a part over the per-Reader limit, got nil error")
	} else if !strings.Contains(err.Error(), "decompression limit") {
		t.Errorf("expected a decompression-limit error, got: %v", err)
	}

	// There is no global to fall back to: a bound is whatever the resolved
	// configuration says, and zero or less disables it. A negative option
	// therefore admits a part a positive one rejects.
	reader, err = NewReader(bytes.NewReader(data), int64(len(data)),
		WithMaxDecompressedPartSize(1024))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, err := reader.GetFile("/ppt/presentation.xml").ReadAll(); err == nil {
		t.Fatal("a 1024-byte bound must reject the part; ReadAll succeeded")
	}

	reader, err = NewReader(bytes.NewReader(data), int64(len(data)),
		WithMaxDecompressedPartSize(-1))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	got, err := reader.GetFile("/ppt/presentation.xml").ReadAll()
	if err != nil {
		t.Fatalf("negative option must disable the bound; ReadAll error = %v", err)
	}
	if !bytes.Equal(got, big) {
		t.Error("part content mismatch under disabled bound")
	}
}

// TestReaderOptions_PackageLimit verifies the package-total override.
func TestReaderOptions_PackageLimit(t *testing.T) {
	big := bytes.Repeat([]byte("B"), 32*1024)
	parts := map[string][]byte{
		"/p/one.xml": big,
		"/p/two.xml": big,
	}
	data := createTestPackage(t, parts, nil)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)),
		WithMaxDecompressedPackageSize(40*1024))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, err := reader.GetFile("/p/one.xml").ReadAll(); err != nil {
		t.Fatalf("first part should fit the package budget: %v", err)
	}
	if _, err := reader.GetFile("/p/two.xml").ReadAll(); err == nil {
		t.Fatal("second part should exceed the per-Reader package budget")
	} else if !strings.Contains(err.Error(), "package decompression limit") {
		t.Errorf("expected a package-limit error, got: %v", err)
	}
}
