package opc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// TestReadCFBRejectsNonStandardMiniCutoff confirms a CFB header declaring a mini
// stream cutoff other than the spec-mandated 4096 ([MS-CFB] §2.2) is rejected.
// A hostile container can set the cutoff to 0xFFFFFFFF so a multi-gigabyte
// stream is routed through the mini-stream path and its declared size forces an
// oversized allocation; the guard blocks that before any chain walk.
func TestReadCFBRejectsNonStandardMiniCutoff(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCFB(&buf, []cfbStream{
		{name: cfbStreamEncryptionInfo, data: []byte("info")},
		{name: cfbStreamEncryptedPackage, data: bytes.Repeat([]byte("x"), 5000)},
	}); err != nil {
		t.Fatalf("writeCFB: %v", err)
	}
	data := buf.Bytes()

	// A faithfully written container parses.
	if _, err := readCFB(append([]byte(nil), data...)); err != nil {
		t.Fatalf("readCFB of well-formed container: %v", err)
	}

	// Patch the mini stream cutoff (header bytes 56:60) to a hostile value.
	for _, bad := range []uint32{0xFFFFFFFF, 0, 512, 8192} {
		corrupt := append([]byte(nil), data...)
		binary.LittleEndian.PutUint32(corrupt[56:60], bad)
		if _, err := readCFB(corrupt); !errors.Is(err, ErrCorruptedPackage) {
			t.Fatalf("miniCutoff=%#x: got %v, want ErrCorruptedPackage", bad, err)
		}
	}
}

// TestReadMiniStreamBoundsAllocation confirms readMiniStream caps its
// pre-allocation to the bytes actually reachable through the mini stream, so a
// directory entry declaring an absurd size cannot force an oversized (or
// panicking) make. Before the cap, make([]byte, 0, 1<<62) panics outright.
func TestReadMiniStreamBoundsAllocation(t *testing.T) {
	f := &cfbFile{
		miniSectorSize: 64,
		miniStream:     make([]byte, 64),
		miniFAT:        []uint32{cfbEndOfChain},
	}
	// A single 64-byte mini sector is reachable, but the entry lies about size.
	if _, err := f.readMiniStream(0, 1<<62); !errors.Is(err, ErrCorruptedPackage) {
		t.Fatalf("oversized declared size: got %v, want ErrCorruptedPackage", err)
	}
	// An honest size within the chain still reads correctly.
	got, err := f.readMiniStream(0, 64)
	if err != nil {
		t.Fatalf("honest size: %v", err)
	}
	if len(got) != 64 {
		t.Fatalf("honest read length = %d, want 64", len(got))
	}
}
