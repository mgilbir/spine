package opc

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"testing"
)

// TestGuardedReaderAtNegativeOffset verifies the offset guard maps a negative
// ReadAt (as a corrupt zip64 local-header offset would drive) to a clean,
// named package error rather than the underlying reader's raw
// "bytes.Reader.ReadAt: negative offset".
func TestGuardedReaderAtNegativeOffset(t *testing.T) {
	g := guardedReaderAt{bytes.NewReader([]byte("hello"))}

	if _, err := g.ReadAt(make([]byte, 2), -1); err == nil {
		t.Fatal("expected error for negative offset")
	} else if !errors.Is(err, ErrCorruptedPackage) {
		t.Errorf("negative offset error = %v, want wrapping ErrCorruptedPackage", err)
	}

	// A valid non-negative read passes through unchanged.
	buf := make([]byte, 5)
	if n, err := g.ReadAt(buf, 0); err != nil || n != 5 || string(buf) != "hello" {
		t.Errorf("passthrough ReadAt = %q, %d, %v; want hello, 5, nil", buf, n, err)
	}
}

// buildNegativeOffsetZip hand-assembles a single-entry zip whose central
// directory marks the local-header offset as zip64 (0xFFFFFFFF) and supplies a
// zip64 extra field carrying an offset with the high bit set, which
// archive/zip stores as a negative int64 and later feeds to ReadAt.
func buildNegativeOffsetZip() []byte {
	// Name the entry [Content_Types].xml so NewReader reads it during open,
	// driving the corrupt zip64 offset through the guarded reader.
	const name = "[Content_Types].xml"
	data := []byte("x")
	crc := crc32.ChecksumIEEE(data)

	var buf bytes.Buffer
	le := binary.LittleEndian

	// --- Local file header at offset 0 ---
	lfh := make([]byte, 30)
	le.PutUint32(lfh[0:], 0x04034b50)
	le.PutUint16(lfh[4:], 20)   // version needed
	le.PutUint16(lfh[6:], 0)    // flags
	le.PutUint16(lfh[8:], 0)    // method: store
	le.PutUint32(lfh[14:], crc) // crc32
	le.PutUint32(lfh[18:], uint32(len(data)))
	le.PutUint32(lfh[22:], uint32(len(data)))
	le.PutUint16(lfh[26:], uint16(len(name)))
	le.PutUint16(lfh[28:], 0)
	buf.Write(lfh)
	buf.WriteString(name)
	buf.Write(data)

	cdOffset := buf.Len()

	// --- Central directory header ---
	zip64Extra := make([]byte, 4+8)
	le.PutUint16(zip64Extra[0:], 0x0001) // zip64 extra tag
	le.PutUint16(zip64Extra[2:], 8)      // size: one 8-byte field (offset)
	le.PutUint64(zip64Extra[4:], 0x8000000000000000)

	cdh := make([]byte, 46)
	le.PutUint32(cdh[0:], 0x02014b50)
	le.PutUint16(cdh[4:], 20)
	le.PutUint16(cdh[6:], 20)
	le.PutUint32(cdh[16:], crc)
	le.PutUint32(cdh[20:], uint32(len(data)))
	le.PutUint32(cdh[24:], uint32(len(data)))
	le.PutUint16(cdh[28:], uint16(len(name)))
	le.PutUint16(cdh[30:], uint16(len(zip64Extra)))
	le.PutUint32(cdh[42:], 0xFFFFFFFF) // offset -> zip64
	buf.Write(cdh)
	buf.WriteString(name)
	buf.Write(zip64Extra)

	cdSize := buf.Len() - cdOffset

	// --- End of central directory ---
	eocd := make([]byte, 22)
	le.PutUint32(eocd[0:], 0x06054b50)
	le.PutUint16(eocd[8:], 1)
	le.PutUint16(eocd[10:], 1)
	le.PutUint32(eocd[12:], uint32(cdSize))
	le.PutUint32(eocd[16:], uint32(cdOffset))
	buf.Write(eocd)

	return buf.Bytes()
}

// TestOpenCorruptZipNegativeOffset drives a corrupt zip end-to-end: opening it
// must fail with a clean, named error, never a panic or the raw
// "bytes.Reader.ReadAt: negative offset".
func TestOpenCorruptZipNegativeOffset(t *testing.T) {
	fixture := buildNegativeOffsetZip()

	zr, err := zip.NewReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		// If the stdlib rejects the central directory outright the negative
		// offset never reaches ReadAt; that is also a clean rejection.
		t.Skipf("stdlib rejected fixture at NewReader (also acceptable): %v", err)
	}
	if len(zr.File) == 0 {
		t.Skip("fixture produced no entries")
	}

	// Confirm the raw stdlib path yields the negative-offset error we guard.
	if _, err := zr.File[0].Open(); err == nil {
		t.Skip("stdlib did not drive a negative offset for this fixture")
	}

	// Now through the guarded OPC reader: the error must be clean and named.
	_, err = NewReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err == nil {
		t.Fatal("expected corrupt-package error, got nil")
	}
	if bytes.Contains([]byte(err.Error()), []byte("bytes.Reader.ReadAt")) {
		t.Errorf("raw negative-offset error leaked: %v", err)
	}
	if !errors.Is(err, ErrCorruptedPackage) {
		t.Errorf("open error = %v, want wrapping ErrCorruptedPackage", err)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("negative offset")) {
		t.Errorf("open error should name the negative offset, got %v", err)
	}
}
