package opc

// Fuzz targets for the encrypted-document read path: the CFB container parser
// (cfb.go) and the open path that drives descriptor parsing, key derivation and
// decryption over fully attacker-controlled bytes.
//
// These targets assert more than "it did not panic", because the worst bug this
// code has shipped would have passed that oracle. C360 sized an allocation from
// the unvalidated numFATSectors header field: a 512-byte file produced a 16 GiB
// allocation, reachable through the encrypted open. A 16 GiB make does not panic —
// it succeeds on a big machine and gets the process OOM-killed on a small one,
// which reads as infrastructure flake. So every call below runs inside a
// fuzzbound.Budget that fails when the parse allocates or runs out of
// proportion to its input, and the API contract (an error, never a partial
// success) is asserted explicitly.
//
// See TestEncryptionFuzzBudgetsAllowLegitimateDocuments for the evidence that
// these budgets do not fire on real encrypted documents, including one large
// enough to need chained DIFAT sectors.

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/internal/fuzzbound"
	"github.com/mgilbir/spine/internal/fuzzseed"
)

// Decompression caps for the inner (decrypted) package. They are tighter than
// the library defaults so that bomb-shaped plaintext stays cheap during fuzzing,
// and they are what makes openBudget's floor a defensible number: the reader
// promises never to inflate a package past MaxDecompressedPackageSize.
const (
	fuzzMaxPartSize    = 4 << 20
	fuzzMaxPackageSize = 8 << 20
)

func fuzzReaderOptions() ReaderOptions {
	return ReaderOptions{
		MaxDecompressedPartSize:    fuzzMaxPartSize,
		MaxDecompressedPackageSize: fuzzMaxPackageSize,
	}
}

// The budgets below are (floor + rate x input size). Their derivation:
//
//   - cfbBudget covers readCFB plus the stream reads it enables. Structurally a
//     container can materialize its FAT (input/128 bytes), its mini stream and
//     the requested streams — every one of them bounded by the image size,
//     since a chain visits each sector at most once. Measured on real
//     containers: 1.0x the input, falling to 0.01x at 13 MB. The 8x rate leaves
//     ample room over the structural worst case, and the 1 MiB floor absorbs the
//     per-call constants (it is also the budget the hand-written C360/C461
//     regression tests in cfb_hardening_test.go use).
//
//   - decryptBudget covers readCFB plus descriptor parse, key derivation and
//     decryption. That path copies the EncryptedPackage stream, then the
//     plaintext, then works segment by segment: measured 9.5x on a 4.6 KB
//     container (constant-dominated) and 3.4x on a 13.7 MB one. 16x with the
//     same floor.
//
//   - openBudget additionally covers the zip reader over the decrypted package,
//     which may legitimately inflate compressed parts — so its floor is the
//     decompression cap the reader is configured with (plus slack), which is the
//     only honest bound for that stage. It still catches an amplification driven
//     by a header field, which is unbounded rather than merely large.
//
// The time bounds are sized to fail a runaway loop rather than a slow machine.
// The slowest *legitimate* case here is an agile descriptor at the maxSpinCount
// ceiling (1e6 iterations x 3 derivations), measured at 0.44s; 15s keeps ~34x
// headroom, enough to survive a -race run (which multiplies wall clock by
// roughly 10) while still turning an unbounded loop into a reported finding
// instead of a package-level timeout. Pure CFB parsing does no key derivation
// (53 us for a 13.7 MB container), so it gets a much tighter 2s.
var (
	cfbBudget = fuzzbound.Budget{
		What:              "readCFB",
		Bytes:             1 << 20,
		BytesPerInputByte: 8,
		Time:              2 * time.Second,
		TimePerMiB:        time.Second,
	}
	decryptBudget = fuzzbound.Budget{
		What:              "decryptCFBPackage",
		Bytes:             1 << 20,
		BytesPerInputByte: 16,
		Time:              15 * time.Second,
		TimePerMiB:        2 * time.Second,
	}
	openBudget = fuzzbound.Budget{
		What:              "NewReader over an encrypted input",
		Bytes:             fuzzMaxPackageSize + 4<<20,
		BytesPerInputByte: 16,
		Time:              15 * time.Second,
		TimePerMiB:        2 * time.Second,
	}
)

const seedPassword = "hunter2"

// seedPlainPackage builds the inner package the encrypted seeds wrap: a valid
// OPC zip with a body part of the requested size, so the decrypted bytes open as
// a package and the reader walk in the fuzz body has something to read.
func seedPlainPackage(bodyLen int) []byte {
	return fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"_rels/.rels", fuzzRootRelsXML},
		{"part.xml", `<?xml version="1.0"?><root>` + strings.Repeat("A", bodyLen) + `</root>`},
	})
}

// encryptedSeed returns a valid encrypted container for the given options over a
// small package.
func encryptedSeed(tb testing.TB, opts EncryptOptions) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := SaveEncryptedWithOptions(&buf, seedPlainPackage(3000), seedPassword, opts); err != nil {
		tb.Fatalf("building encrypted seed: %v", err)
	}
	return buf.Bytes()
}

// rc4Seed builds a CFB container holding a legacy RC4 CryptoAPI descriptor.
// SaveEncrypted never writes RC4 (it is broken), but the open reads it, so
// the fuzzer needs a seed that reaches rc4.go.
func rc4Seed(tb testing.TB) []byte {
	tb.Helper()
	info, pkg, err := crypto.EncryptRC4CryptoAPI(seedPlainPackage(500), seedPassword, 128)
	if err != nil {
		tb.Fatalf("building RC4 seed: %v", err)
	}
	var buf bytes.Buffer
	if err := writeCFB(&buf, []cfbStream{
		{name: cfbStreamEncryptionInfo, data: info},
		{name: cfbStreamEncryptedPackage, data: pkg},
	}); err != nil {
		tb.Fatalf("writing RC4 container: %v", err)
	}
	return buf.Bytes()
}

// difatLoopSeed builds a small container whose DIFAT sectors chain in a cycle,
// the shape behind C461 (a 256 KiB input driving a 34 MB allocation).
func difatLoopSeed() []byte {
	const sectors = 8
	data := synthCFB(cfbHeaderSpec{
		firstDirSector:     cfbEndOfChain,
		firstMiniFATSector: cfbEndOfChain,
		firstDIFATSector:   0,
		numDIFATSectors:    0xFFFFFFFE,
		sectors:            sectors,
	})
	for s := 0; s < sectors; s++ {
		off := cfbHeaderSize + s*cfbWriteSectorSize
		for j := 0; j < cfbDIFATPerSector; j++ {
			binary.LittleEndian.PutUint32(data[off+j*4:off+j*4+4], 0)
		}
		binary.LittleEndian.PutUint32(data[off+cfbDIFATPerSector*4:off+cfbFATEntries*4], uint32((s+1)%sectors))
	}
	return data
}

// cfbSeeds returns the shared seed set: real containers for every scheme this
// library can produce or read, plus the malformed shapes that are expensive for
// a fuzzer to rediscover (the magic number alone is 8 bytes it would have to
// guess). Real containers matter more than random bytes here — without them the
// fuzzer spends its whole budget failing the signature check.
func cfbSeeds(tb testing.TB) [][]byte {
	tb.Helper()
	agile := encryptedSeed(tb, EncryptOptions{Scheme: SchemeAgile})
	seeds := [][]byte{
		agile,
		encryptedSeed(tb, EncryptOptions{Scheme: SchemeAgile, IncludeDataSpaces: true}),
		encryptedSeed(tb, EncryptOptions{Scheme: SchemeStandard}),
		encryptedSeed(tb, EncryptOptions{Scheme: SchemeStandard, StandardKeyBits: 128}),
		rc4Seed(tb),
		difatLoopSeed(),

		// Not a container at all, and the shortest possible near-misses.
		nil,
		[]byte("PK\x03\x04"),
		cfbSignature,
		append(append([]byte(nil), cfbSignature...), make([]byte, cfbHeaderSize-8)...),

		// Header-only images whose counts are hostile: these are the C360
		// shapes, where a 512-byte file asked for 16 GiB.
		synthCFB(cfbHeaderSpec{numFATSectors: 0xFFFFFFFF, firstDirSector: cfbEndOfChain, firstMiniFATSector: cfbEndOfChain, firstDIFATSector: cfbEndOfChain}),
		synthCFB(cfbHeaderSpec{numFATSectors: 1 << 31, firstDirSector: cfbEndOfChain, firstMiniFATSector: cfbEndOfChain, firstDIFATSector: cfbEndOfChain}),
		synthCFB(cfbHeaderSpec{numFATSectors: 0xFFFFFFFF, firstDirSector: cfbEndOfChain, firstMiniFATSector: cfbEndOfChain, numDIFATSectors: 0xFFFFFFFF, sectors: 4}),
		synthCFB(cfbHeaderSpec{firstDirSector: 0, firstMiniFATSector: 0, numMiniFATSectors: 0xFFFFFFFF, firstDIFATSector: cfbEndOfChain, sectors: 4}),
		synthCFB(cfbHeaderSpec{numFATSectors: 0xFFFFFFFF, firstDirSector: 0xFFFFFFF7, firstMiniFATSector: 0, numMiniFATSectors: 0xFFFFFFFF, firstDIFATSector: 0, numDIFATSectors: 0xFFFFFFFF, sectors: 4}),
	}

	// Truncations of a real container: header cut short, directory cut off,
	// sector pointers left dangling past EOF.
	for _, n := range []int{8, 100, cfbHeaderSize, cfbHeaderSize + 3, len(agile) / 2, len(agile) - 1} {
		if n > 0 && n < len(agile) {
			seeds = append(seeds, append([]byte(nil), agile[:n]...))
		}
	}

	// A real container with each header field an attacker would inflate set to
	// its maximum, one at a time: the parse still has to reach the streams.
	for _, off := range []int{30, 32, 44, 48, 56, 60, 64, 68, 72} {
		corrupt := append([]byte(nil), agile...)
		binary.LittleEndian.PutUint32(corrupt[off:off+4], 0xFFFFFFFF)
		seeds = append(seeds, corrupt)
	}
	return seeds
}

// FuzzCFBContainer drives the CFB container parser with arbitrary bytes:
// malformed headers, absurd sector counts, looping DIFAT chains, truncated
// images and sector pointers past EOF.
//
// Beyond "no panic" it asserts that (a) the parse stays inside cfbBudget, which
// is what a C360-shaped allocation would break; (b) an error never comes back
// with a parsed file, and a nil error never with a nil file; and (c) no stream
// materializes more bytes than the image itself holds — a chain visits each
// sector at most once, so a stream longer than the file means a sector was
// counted twice.
func FuzzCFBContainer(f *testing.F) {
	for _, seed := range cfbSeeds(f) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var (
			cf      *cfbFile
			err     error
			streams [][]byte
		)
		cfbBudget.Check(t, len(data), func() {
			cf, err = readCFB(data)
			streams = streams[:0]
			if err != nil {
				return
			}
			for _, name := range []string{
				cfbStreamEncryptionInfo,
				cfbStreamEncryptedPackage,
				"\x06DataSpaces",
				"", // a directory entry with an unparsable name
			} {
				s, serr := cf.stream(name)
				if serr == nil {
					streams = append(streams, s)
				} else if s != nil {
					t.Fatalf("stream(%q) returned %d bytes with error %v", name, len(s), serr)
				}
			}
		})

		switch {
		case err != nil && cf != nil:
			t.Fatalf("readCFB returned both a file and error %v", err)
		case err == nil && cf == nil:
			t.Fatal("readCFB returned a nil file and a nil error")
		}
		for i, s := range streams {
			if len(s) > len(data) {
				t.Fatalf("stream %d is %d bytes, longer than the %d-byte image", i, len(s), len(data))
			}
		}
	})
}

// FuzzOpenEncrypted drives the whole encrypted-open path — container
// detection, CFB parse, EncryptionInfo descriptor parse, key derivation,
// decryption, and the zip reader over the recovered package — with an
// arbitrary container and an arbitrary password.
//
// Beyond "no panic" it asserts the resource budgets above, and the API
// contract: a rejected container yields an error with no plaintext and no
// Reader, an accepted one yields a non-nil Reader, and the public entry point
// never accepts a container the decrypt stage rejected.
func FuzzOpenEncrypted(f *testing.F) {
	for _, seed := range cfbSeeds(f) {
		f.Add(seed, seedPassword)
	}
	agile := encryptedSeed(f, EncryptOptions{Scheme: SchemeAgile})
	f.Add(agile, "")
	f.Add(agile, "wrong")
	f.Add(agile, strings.Repeat("é", 300)) // over-long, non-ASCII
	f.Add(agile, "\x00\xff")               // invalid UTF-8

	// A container whose EncryptionInfo stream has been replaced by hostile
	// descriptor bytes: the fuzzer reaches the descriptor parsers much faster
	// from here than by mutating a whole container.
	for _, info := range [][]byte{
		nil,
		[]byte("\x04\x00\x04\x00\x40\x00\x00\x00<encryption/>"),
		[]byte("\x04\x00\x04\x00\x40\x00\x00\x00" + strings.Repeat("<a>", 200)),
		{0x03, 0x00, 0x02, 0x00, 0x24, 0x00, 0x00, 0x00},
	} {
		var buf bytes.Buffer
		if err := writeCFB(&buf, []cfbStream{
			{name: cfbStreamEncryptionInfo, data: info},
			{name: cfbStreamEncryptedPackage, data: bytes.Repeat([]byte("z"), 200)},
		}); err == nil {
			f.Add(buf.Bytes(), seedPassword)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte, password string) {
		opts := fuzzReaderOptions()

		var (
			plain []byte
			derr  error
		)
		decryptBudget.Check(t, len(data), func() {
			plain, derr = decryptCFBPackage(data, password, opts)
		})
		if derr != nil && plain != nil {
			t.Fatalf("decryptCFBPackage returned %d plaintext bytes with error %v", len(plain), derr)
		}

		var (
			r    *Reader
			oerr error
		)
		openBudget.Check(t, len(data), func() {
			r, oerr = NewReader(bytes.NewReader(data), int64(len(data)), WithReaderOptions(opts), WithPassword(password))
		})
		switch {
		case oerr != nil && r != nil:
			t.Fatalf("the open returned both a Reader and error %v", oerr)
		case oerr == nil && r == nil:
			t.Fatal("the open returned a nil Reader and a nil error")
		case oerr == nil && derr != nil && isCFB(data):
			// Gated on the input actually being a container: the open now
			// takes plain zips too, and a mutation that lands on a valid one is
			// legitimately opened while decryptCFBPackage rejects it.
			t.Fatalf("the open accepted a container the decrypt stage rejected with %v", derr)
		}
		if r == nil {
			return
		}
		for i, file := range r.Files {
			if i >= 16 {
				break
			}
			if _, err := file.ReadAll(); err != nil {
				continue
			}
			if _, err := r.GetPartRelationships(file.Name); err != nil {
				continue
			}
		}
	})
}
