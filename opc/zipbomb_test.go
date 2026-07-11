package opc

import (
	"bytes"
	"fmt"
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

	old := MaxDecompressedPartSize
	MaxDecompressedPartSize = 1024
	defer func() { MaxDecompressedPartSize = old }()

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
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

	old := MaxDecompressedPartSize
	MaxDecompressedPartSize = 1024
	defer func() { MaxDecompressedPartSize = old }()

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
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

// withDecompressionLimits overrides both decompression limits for the
// duration of the test.
func withDecompressionLimits(t *testing.T, part, pkg int64) {
	t.Helper()
	oldPart, oldPkg := MaxDecompressedPartSize, MaxDecompressedPackageSize
	MaxDecompressedPartSize, MaxDecompressedPackageSize = part, pkg
	t.Cleanup(func() {
		MaxDecompressedPartSize, MaxDecompressedPackageSize = oldPart, oldPkg
	})
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
	withDecompressionLimits(t, 1<<20, openCost+2500)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
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

	withDecompressionLimits(t, 1<<20, 10)

	if _, err := NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
		t.Fatal("expected NewReader to fail under a tiny package limit, got nil error")
	} else if !strings.Contains(err.Error(), "package decompression limit") {
		t.Errorf("expected a package-decompression-limit error, got: %v", err)
	}
}

// TestMaxDecompressedPackageSize_AllowsUnderLimit verifies that a package
// comfortably under both limits reads all parts unaffected.
func TestMaxDecompressedPackageSize_AllowsUnderLimit(t *testing.T) {
	data, _ := twoPartPackage(t)

	withDecompressionLimits(t, 1<<20, 1<<20)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
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

	withDecompressionLimits(t, 1<<20, 0)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	for _, f := range reader.Files {
		if _, err := f.ReadAll(); err != nil {
			t.Errorf("ReadAll(%s) error = %v", f.Name, err)
		}
	}
}

// TestDecompressionLimits_SnapshotPerReader verifies that the limits are
// captured at Reader construction: changing the globals afterwards affects
// only subsequently opened packages, and raising a too-small limit lets a
// fresh Reader succeed.
func TestDecompressionLimits_SnapshotPerReader(t *testing.T) {
	data, openCost := twoPartPackage(t)

	withDecompressionLimits(t, 1<<20, 1<<20)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}

	// Shrinking the globals must not affect the already-open Reader.
	MaxDecompressedPartSize = 1
	MaxDecompressedPackageSize = 1
	if _, err := reader.GetFile("/ppt/presentation.xml").ReadAll(); err != nil {
		t.Errorf("open Reader affected by later global change: %v", err)
	}

	// A Reader opened under the shrunken limits fails...
	if _, err := NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
		t.Error("expected NewReader to fail under shrunken limits, got nil error")
	}

	// ...and raising the limits makes the next Reader work again.
	MaxDecompressedPartSize = 1 << 20
	MaxDecompressedPackageSize = openCost + 5000
	reader2, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader() after raising limits error = %v", err)
	}
	for _, f := range reader2.Files {
		if _, err := f.ReadAll(); err != nil {
			t.Errorf("ReadAll(%s) after raising limits error = %v", f.Name, err)
		}
	}
}

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

	withDecompressionLimits(t, 1<<20, 1<<20)

	reader, err := NewReader(bytes.NewReader(data), int64(len(data)))
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
