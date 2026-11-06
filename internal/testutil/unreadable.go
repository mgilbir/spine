package testutil

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"sync/atomic"
	"testing"
)

// WithEntryHeader returns a copy of pkg in which the named entry's header has
// been passed through edit, every entry's compressed bytes copied verbatim.
// It is how a test makes one part present but unreadable: declaring an
// uncompressed size past the per-part decompression limit trips the limit
// before a byte is inflated, and a wrong CRC-32 makes the read fail with
// zip.ErrChecksum. The test fails when pkg is not a zip or carries no entry of
// that name.
func WithEntryHeader(t testing.TB, pkg []byte, name string, edit func(*zip.FileHeader)) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("WithEntryHeader: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	found := false
	for _, f := range zr.File {
		raw, err := f.OpenRaw()
		if err != nil {
			t.Fatalf("WithEntryHeader: opening %s: %v", f.Name, err)
		}
		hdr := f.FileHeader
		if f.Name == name {
			found = true
			edit(&hdr)
		}
		w, err := zw.CreateRaw(&hdr)
		if err != nil {
			t.Fatalf("WithEntryHeader: creating %s: %v", f.Name, err)
		}
		if _, err := io.Copy(w, raw); err != nil {
			t.Fatalf("WithEntryHeader: copying %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("WithEntryHeader: %v", err)
	}
	if !found {
		t.Fatalf("WithEntryHeader: no entry %q in package", name)
	}
	return buf.Bytes()
}

// WithoutEntry returns a copy of pkg with the named entry removed and every
// other entry copied verbatim — the "part genuinely absent" counterpart to the
// unreadable-part helpers, for pinning the leniency a missing optional part
// keeps. The test fails when pkg carries no entry of that name.
func WithoutEntry(t testing.TB, pkg []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("WithoutEntry: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	found := false
	for _, f := range zr.File {
		if f.Name == name {
			found = true
			continue
		}
		if err := zw.Copy(f); err != nil {
			t.Fatalf("WithoutEntry: copying %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("WithoutEntry: %v", err)
	}
	if !found {
		t.Fatalf("WithoutEntry: no entry %q in package", name)
	}
	return buf.Bytes()
}

// WithOversizedEntry returns a copy of pkg whose named entry declares an
// uncompressed size of 2 GiB, past the default 1 GiB per-part decompression
// limit, so any read of that part fails with opc.ErrDecompressionLimit while
// the part itself stays present in the package.
func WithOversizedEntry(t testing.TB, pkg []byte, name string) []byte {
	t.Helper()
	return WithEntryHeader(t, pkg, name, func(h *zip.FileHeader) {
		h.UncompressedSize64 = 2 << 30
	})
}

// WithBadCRC returns a copy of pkg whose named entry carries a wrong CRC-32,
// so reading the part fails with zip.ErrChecksum.
func WithBadCRC(t testing.TB, pkg []byte, name string) []byte {
	t.Helper()
	return WithEntryHeader(t, pkg, name, func(h *zip.FileHeader) {
		h.CRC32 ^= 0xFFFFFFFF
	})
}

// ErrInjectedRead is the error a SwitchableReaderAt returns once failing.
var ErrInjectedRead = errors.New("testutil: injected read failure")

// SwitchableReaderAt serves ReadAt from a byte slice until Fail is called,
// then fails every read with ErrInjectedRead. It models a source that was
// readable at open but stops being so afterwards (a file truncated or replaced
// underneath a still-open document), which is what a lazily re-read part
// meets.
type SwitchableReaderAt struct {
	data    []byte
	failing atomic.Bool
}

// NewSwitchableReaderAt returns a reader over data that has not failed yet.
func NewSwitchableReaderAt(data []byte) *SwitchableReaderAt {
	return &SwitchableReaderAt{data: data}
}

// Fail makes every later ReadAt fail.
func (r *SwitchableReaderAt) Fail() { r.failing.Store(true) }

// ReadAt implements io.ReaderAt.
func (r *SwitchableReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if r.failing.Load() {
		return 0, ErrInjectedRead
	}
	return bytes.NewReader(r.data).ReadAt(p, off)
}

// Size reports the length of the underlying data.
func (r *SwitchableReaderAt) Size() int64 { return int64(len(r.data)) }
