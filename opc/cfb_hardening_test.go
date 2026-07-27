package opc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"runtime"
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

// cfbHeaderSpec describes the attacker-controlled header fields a synthetic
// container declares. Everything else (magic, sector shifts, mini cutoff) is
// filled in conformantly so parsing reaches the allocation sites under test.
type cfbHeaderSpec struct {
	numFATSectors      uint32
	firstDirSector     uint32
	firstMiniFATSector uint32
	numMiniFATSectors  uint32
	firstDIFATSector   uint32
	numDIFATSectors    uint32
	sectors            int // regular sectors appended after the header
}

// synthCFB builds a CFB image whose header declares spec. All 109 header DIFAT
// slots are FREESECT unless a caller overwrites them.
func synthCFB(spec cfbHeaderSpec) []byte {
	data := make([]byte, cfbHeaderSize+spec.sectors*cfbWriteSectorSize)
	copy(data[0:8], cfbSignature)
	binary.LittleEndian.PutUint16(data[30:32], 9) // 512-byte sectors
	binary.LittleEndian.PutUint16(data[32:34], 6) // 64-byte mini sectors
	binary.LittleEndian.PutUint32(data[44:48], spec.numFATSectors)
	binary.LittleEndian.PutUint32(data[48:52], spec.firstDirSector)
	binary.LittleEndian.PutUint32(data[56:60], cfbWriteMiniCutoff)
	binary.LittleEndian.PutUint32(data[60:64], spec.firstMiniFATSector)
	binary.LittleEndian.PutUint32(data[64:68], spec.numMiniFATSectors)
	binary.LittleEndian.PutUint32(data[68:72], spec.firstDIFATSector)
	binary.LittleEndian.PutUint32(data[72:76], spec.numDIFATSectors)
	for i := 0; i < cfbHeaderDIFAT; i++ {
		binary.LittleEndian.PutUint32(data[76+i*4:80+i*4], cfbFreeSect)
	}
	return data
}

// allocDelta returns the bytes allocated while fn ran. TotalAlloc is cumulative
// and unaffected by collection, so it measures allocation volume rather than
// residency — which is the point: a transient 16 GiB make kills the process just
// as dead as a retained one.
func allocDelta(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestReadCFBBoundsHeaderDrivenAllocations feeds hostile values for every header
// field readCFB sizes an allocation from and asserts the allocation stays within
// a bound derived from the input size. Returning an error is not enough: the
// unhardened code also errored ("CFB directory has no root entry") — after
// allocating 16 GiB for a 512-byte file (C360).
func TestReadCFBBoundsHeaderDrivenAllocations(t *testing.T) {
	// A real container, so the "hostile count on an otherwise valid file" case
	// exercises the clamp on the success path rather than on an early error.
	var valid bytes.Buffer
	if err := writeCFB(&valid, []cfbStream{
		{name: cfbStreamEncryptionInfo, data: []byte("info")},
		{name: cfbStreamEncryptedPackage, data: bytes.Repeat([]byte("x"), 5000)},
	}); err != nil {
		t.Fatalf("writeCFB: %v", err)
	}
	inflatedFAT := append([]byte(nil), valid.Bytes()...)
	binary.LittleEndian.PutUint32(inflatedFAT[44:48], 0xFFFFFFFF)

	const maxUint32 = 0xFFFFFFFF
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{{
		name:    "numFATSectors max",
		data:    synthCFB(cfbHeaderSpec{numFATSectors: maxUint32, firstDirSector: cfbEndOfChain, firstMiniFATSector: cfbEndOfChain, firstDIFATSector: cfbEndOfChain}),
		wantErr: true,
	}, {
		name:    "numFATSectors 2^31",
		data:    synthCFB(cfbHeaderSpec{numFATSectors: 1 << 31, firstDirSector: cfbEndOfChain, firstMiniFATSector: cfbEndOfChain, firstDIFATSector: cfbEndOfChain}),
		wantErr: true,
	}, {
		name:    "numDIFATSectors max, chain starts in range",
		data:    synthCFB(cfbHeaderSpec{numFATSectors: maxUint32, firstDirSector: cfbEndOfChain, firstMiniFATSector: cfbEndOfChain, numDIFATSectors: maxUint32, sectors: 4}),
		wantErr: true,
	}, {
		name:    "numMiniFATSectors max",
		data:    synthCFB(cfbHeaderSpec{firstDirSector: 0, firstMiniFATSector: 0, numMiniFATSectors: maxUint32, firstDIFATSector: cfbEndOfChain, sectors: 4}),
		wantErr: true,
	}, {
		name:    "every count max",
		data:    synthCFB(cfbHeaderSpec{numFATSectors: maxUint32, firstDirSector: maxUint32 - 8, firstMiniFATSector: 0, numMiniFATSectors: maxUint32, firstDIFATSector: 0, numDIFATSectors: maxUint32, sectors: 4}),
		wantErr: true,
	}, {
		// numFATSectors is only ever a capacity hint (the locations come from the
		// DIFAT), so an inflated count must not change the parse result — only the
		// allocation it used to drive.
		name:    "valid container with inflated numFATSectors",
		data:    inflatedFAT,
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			delta := allocDelta(func() { _, err = readCFB(tt.data) })
			if tt.wantErr && !errors.Is(err, ErrCorruptedPackage) {
				t.Fatalf("err = %v, want ErrCorruptedPackage", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("err = %v, want success", err)
			}
			// Generous but far below any header-driven amplification: the fields
			// above ask for 16 GiB apiece unclamped.
			budget := uint64(len(tt.data)) + 1<<20
			if delta > budget {
				t.Fatalf("allocated %d bytes for a %d-byte input (budget %d)", delta, len(tt.data), budget)
			}
		})
	}
}

// TestReadCFBBoundsDIFATChainAllocation covers the amplification the old
// post-loop 1<<24 guard let through (C461): a DIFAT chain over a small file
// collects sectors*127 FAT locations, each of which the FAT pre-allocation
// multiplies by another 128 entries. A 256 KiB input drove a 34 MB allocation.
func TestReadCFBBoundsDIFATChainAllocation(t *testing.T) {
	const sectors = 512
	data := synthCFB(cfbHeaderSpec{
		firstDirSector:     cfbEndOfChain,
		firstMiniFATSector: cfbEndOfChain,
		firstDIFATSector:   0,
		numDIFATSectors:    0xFFFFFFFE, // +1 must not wrap the loop bound to zero
		sectors:            sectors,
	})
	// Every DIFAT sector fills its 127 location slots with the (valid) sector 0
	// and points its last slot at the next DIFAT sector.
	for s := 0; s < sectors; s++ {
		off := cfbHeaderSize + s*cfbWriteSectorSize
		for j := 0; j < cfbDIFATPerSector; j++ {
			binary.LittleEndian.PutUint32(data[off+j*4:off+j*4+4], 0)
		}
		next := uint32(cfbEndOfChain)
		if s < sectors-1 {
			next = uint32(s + 1)
		}
		binary.LittleEndian.PutUint32(data[off+cfbDIFATPerSector*4:off+cfbFATEntries*4], next)
	}

	var err error
	delta := allocDelta(func() { _, err = readCFB(data) })
	if !errors.Is(err, ErrCorruptedPackage) {
		t.Fatalf("err = %v, want ErrCorruptedPackage", err)
	}
	budget := uint64(len(data)) + 1<<20
	if delta > budget {
		t.Fatalf("allocated %d bytes for a %d-byte input (budget %d)", delta, len(data), budget)
	}
}
