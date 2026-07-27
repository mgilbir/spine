package opc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// dirEntryByName returns the parsed directory entry with the given name.
func dirEntryByName(t *testing.T, cf *cfbFile, name string) cfbDirEntry {
	t.Helper()
	for _, e := range cf.dir {
		if e.name == name {
			return e
		}
	}
	t.Fatalf("no directory entry named %q", name)
	return cfbDirEntry{}
}

// TestWriteCFBZeroLengthStreamStartsAtEndOfChain pins the start sector of a
// zero-length stream in both writers (C453). The flat writer used to leave the
// mini-stream layout's next-free index in the directory entry: when the empty
// stream was laid out last that index is past the end of the mini-FAT and
// readCFB rejects its own output, and when another stream followed it silently
// pointed at that stream's first mini sector.
func TestWriteCFBZeroLengthStreamStartsAtEndOfChain(t *testing.T) {
	streams := []cfbStream{
		{name: cfbStreamEncryptionInfo, data: []byte("info")},
		{name: "Empty", data: nil},
	}

	var flat bytes.Buffer
	if err := writeCFB(&flat, streams); err != nil {
		t.Fatalf("writeCFB: %v", err)
	}
	cf, err := readCFB(flat.Bytes())
	if err != nil {
		t.Fatalf("readCFB: %v", err)
	}
	if e := dirEntryByName(t, cf, "Empty"); e.startSect != cfbEndOfChain {
		t.Fatalf("flat writer: zero-length stream startSect = %#x, want ENDOFCHAIN (%#x)", e.startSect, uint32(cfbEndOfChain))
	}
	got, err := cf.stream("Empty")
	if err != nil {
		t.Fatalf("flat writer: reading back the zero-length stream: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("flat writer: zero-length stream read back %d bytes", len(got))
	}
	// The neighbouring stream must still be readable and intact.
	if info, err := cf.stream(cfbStreamEncryptionInfo); err != nil || !bytes.Equal(info, []byte("info")) {
		t.Fatalf("flat writer: EncryptionInfo = %q, %v", info, err)
	}

	// The storage-aware writer must agree; the two are documented to produce the
	// same container for the no-storage case.
	var tree bytes.Buffer
	if err := writeCFBWithStorages(&tree, []cfbStream{{name: cfbStreamEncryptionInfo, data: []byte("info")}}, []cfbStorage{
		{name: "Stor", streams: []cfbStream{{name: "Empty", data: nil}}},
	}); err != nil {
		t.Fatalf("writeCFBWithStorages: %v", err)
	}
	cfTree, err := readCFB(tree.Bytes())
	if err != nil {
		t.Fatalf("readCFB (tree): %v", err)
	}
	if e := dirEntryByName(t, cfTree, "Empty"); e.startSect != cfbEndOfChain {
		t.Fatalf("tree writer: zero-length stream startSect = %#x, want ENDOFCHAIN (%#x)", e.startSect, uint32(cfbEndOfChain))
	}
}

// TestWriteCFBBytesAreStable guards the round-trip promise: the container this
// package writes for the ordinary two non-empty streams (the encrypted-save
// shape) must be byte-for-byte what it has always been. The digest was taken
// before the zero-length-stream fix (C453), which must not move any byte of a
// container that has no zero-length stream.
func TestWriteCFBBytesAreStable(t *testing.T) {
	const want = "1164817e36ade1fbf2960e3cedfa8b20372fa55ff19980ac75b93f182e4c1a6a"
	var buf bytes.Buffer
	if err := writeCFB(&buf, []cfbStream{
		{name: cfbStreamEncryptionInfo, data: bytes.Repeat([]byte("i"), 300)},
		{name: cfbStreamEncryptedPackage, data: bytes.Repeat([]byte("p"), 9000)},
	}); err != nil {
		t.Fatalf("writeCFB: %v", err)
	}
	sum := sha256.Sum256(buf.Bytes())
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("container digest = %s (%d bytes), want %s", got, buf.Len(), want)
	}
}
