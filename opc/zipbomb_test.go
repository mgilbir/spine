package opc

import (
	"bytes"
	"strings"
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
