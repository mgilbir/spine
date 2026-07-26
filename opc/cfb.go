package opc

// This file implements the minimal slice of the Microsoft Compound File Binary
// (CFB, a.k.a. OLE2 structured storage) format needed to carry an encrypted
// OOXML document. A password-encrypted Office file is not a zip: it is a CFB
// container holding two streams, EncryptionInfo and EncryptedPackage (see
// encryption.go and common/crypto/agile.go). Only what those two streams need
// is implemented — enough of the FAT/mini-FAT/directory machinery to read any
// conformant container and to write a fresh two-stream one.
//
// Reference: [MS-CFB], the Compound File Binary File Format specification.

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// cfbSignature is the 8-byte magic at the start of every CFB file.
var cfbSignature = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// Special sector-chain values ([MS-CFB] §2.2).
const (
	cfbMaxRegSect   = 0xFFFFFFFA // largest valid regular sector number
	cfbDIFSect      = 0xFFFFFFFC // sector holds part of the DIFAT
	cfbFATSect      = 0xFFFFFFFD // sector holds part of the FAT
	cfbEndOfChain   = 0xFFFFFFFE // end of a sector chain
	cfbFreeSect     = 0xFFFFFFFF // unallocated sector
	cfbNoStream     = 0xFFFFFFFF // directory entry has no child/sibling
	cfbHeaderSize   = 512
	cfbDirEntrySize = 128
)

// Directory-entry object types ([MS-CFB] §2.6.1).
const (
	cfbTypeStorage = 1
	cfbTypeStream  = 2
	cfbTypeRoot    = 5
)

// isCFB reports whether the first bytes of a package are the CFB magic, which
// marks a password-encrypted (or otherwise OLE2-wrapped) Office document rather
// than a plain zip. head may be shorter than the signature.
func isCFB(head []byte) bool {
	if len(head) < len(cfbSignature) {
		return false
	}
	for i, b := range cfbSignature {
		if head[i] != b {
			return false
		}
	}
	return true
}

// cfbFile is a parsed CFB container held fully in memory.
type cfbFile struct {
	data           []byte
	sectorSize     int
	miniSectorSize int
	miniCutoff     uint32
	fat            []uint32
	miniFAT        []uint32
	dir            []cfbDirEntry
	miniStream     []byte
}

// cfbDirEntry is one parsed 128-byte directory entry.
type cfbDirEntry struct {
	name       string
	objectType byte
	startSect  uint32
	size       uint64
}

// readCFB parses a CFB container from an in-memory image.
func readCFB(data []byte) (*cfbFile, error) {
	if len(data) < cfbHeaderSize || !isCFB(data) {
		return nil, fmt.Errorf("%w: not a CFB container", ErrCorruptedPackage)
	}

	sectorShift := binary.LittleEndian.Uint16(data[30:32])
	miniSectorShift := binary.LittleEndian.Uint16(data[32:34])
	if sectorShift != 9 && sectorShift != 12 {
		return nil, fmt.Errorf("%w: unsupported CFB sector shift %d", ErrCorruptedPackage, sectorShift)
	}
	if miniSectorShift != 6 {
		return nil, fmt.Errorf("%w: unsupported CFB mini sector shift %d", ErrCorruptedPackage, miniSectorShift)
	}

	f := &cfbFile{
		data:           data,
		sectorSize:     1 << sectorShift,
		miniSectorSize: 1 << miniSectorShift,
		miniCutoff:     binary.LittleEndian.Uint32(data[56:60]),
	}

	// [MS-CFB] §2.2 fixes the mini stream cutoff at 4096. A hostile container can
	// declare something else (e.g. 0xFFFFFFFF) to route a multi-gigabyte stream
	// through the mini-stream path and force an oversized allocation; reject any
	// non-conformant value up front.
	if f.miniCutoff != 4096 {
		return nil, fmt.Errorf("%w: CFB mini stream cutoff is %d (must be 4096)", ErrCorruptedPackage, f.miniCutoff)
	}

	numFATSectors := binary.LittleEndian.Uint32(data[44:48])
	firstDirSector := binary.LittleEndian.Uint32(data[48:52])
	firstMiniFATSector := binary.LittleEndian.Uint32(data[60:64])
	numMiniFATSectors := binary.LittleEndian.Uint32(data[64:68])
	firstDIFATSector := binary.LittleEndian.Uint32(data[68:72])
	numDIFATSectors := binary.LittleEndian.Uint32(data[72:76])

	if err := f.buildFAT(numFATSectors, firstDIFATSector, numDIFATSectors); err != nil {
		return nil, err
	}
	if err := f.readDirectory(firstDirSector); err != nil {
		return nil, err
	}
	if err := f.buildMiniFAT(firstMiniFATSector, numMiniFATSectors); err != nil {
		return nil, err
	}
	if err := f.loadMiniStream(); err != nil {
		return nil, err
	}
	return f, nil
}

// sectorData returns the bytes of one regular sector, or an error if the sector
// number is out of range for the file image.
func (f *cfbFile) sectorData(sector uint32) ([]byte, error) {
	off := int64(f.sectorSize) * (int64(sector) + 1)
	if off < 0 || off+int64(f.sectorSize) > int64(len(f.data)) {
		return nil, fmt.Errorf("%w: CFB sector %d is out of range", ErrCorruptedPackage, sector)
	}
	return f.data[off : off+int64(f.sectorSize)], nil
}

// buildFAT assembles the full FAT array from the header DIFAT (109 entries) and
// any chained DIFAT sectors.
func (f *cfbFile) buildFAT(numFATSectors, firstDIFATSector, numDIFATSectors uint32) error {
	fatSectorLocs := make([]uint32, 0, numFATSectors)

	// The first 109 FAT sector locations live in the header DIFAT.
	for i := 0; i < 109; i++ {
		loc := binary.LittleEndian.Uint32(f.data[76+i*4 : 80+i*4])
		if loc == cfbFreeSect || loc == cfbEndOfChain {
			continue
		}
		fatSectorLocs = append(fatSectorLocs, loc)
	}

	// Remaining FAT sector locations live in a chain of DIFAT sectors.
	entriesPerSector := f.sectorSize / 4
	difatSector := firstDIFATSector
	seen := make(map[uint32]bool)
	for i := uint32(0); difatSector != cfbEndOfChain && difatSector <= cfbMaxRegSect && i < numDIFATSectors+1; i++ {
		if seen[difatSector] {
			return fmt.Errorf("%w: CFB DIFAT sector chain loops", ErrCorruptedPackage)
		}
		seen[difatSector] = true
		sd, err := f.sectorData(difatSector)
		if err != nil {
			return err
		}
		for j := 0; j < entriesPerSector-1; j++ {
			loc := binary.LittleEndian.Uint32(sd[j*4 : j*4+4])
			if loc == cfbFreeSect || loc == cfbEndOfChain {
				continue
			}
			fatSectorLocs = append(fatSectorLocs, loc)
		}
		difatSector = binary.LittleEndian.Uint32(sd[(entriesPerSector-1)*4 : entriesPerSector*4])
	}

	if len(fatSectorLocs) > 1<<24 {
		return fmt.Errorf("%w: CFB declares an implausible number of FAT sectors", ErrCorruptedPackage)
	}

	f.fat = make([]uint32, 0, len(fatSectorLocs)*entriesPerSector)
	for _, loc := range fatSectorLocs {
		sd, err := f.sectorData(loc)
		if err != nil {
			return err
		}
		for j := 0; j < entriesPerSector; j++ {
			f.fat = append(f.fat, binary.LittleEndian.Uint32(sd[j*4:j*4+4]))
		}
	}
	return nil
}

// chain follows a FAT sector chain from start and returns the sector numbers in
// order. It bounds the length and rejects loops so a malformed file cannot make
// it spin or allocate without limit.
func (f *cfbFile) chain(start uint32) ([]uint32, error) {
	var out []uint32
	seen := make(map[uint32]bool)
	for s := start; s != cfbEndOfChain && s <= cfbMaxRegSect; {
		if int(s) >= len(f.fat) {
			return nil, fmt.Errorf("%w: CFB sector %d is out of FAT range", ErrCorruptedPackage, s)
		}
		if seen[s] {
			return nil, fmt.Errorf("%w: CFB sector chain loops at %d", ErrCorruptedPackage, s)
		}
		seen[s] = true
		out = append(out, s)
		if len(out) > len(f.fat) {
			return nil, fmt.Errorf("%w: CFB sector chain exceeds FAT size", ErrCorruptedPackage)
		}
		s = f.fat[s]
	}
	return out, nil
}

// readRegularStream reads a stream stored in the regular FAT, truncated to size.
func (f *cfbFile) readRegularStream(start uint32, size uint64) ([]byte, error) {
	sectors, err := f.chain(start)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 0, len(sectors)*f.sectorSize)
	for _, s := range sectors {
		sd, err := f.sectorData(s)
		if err != nil {
			return nil, err
		}
		buf = append(buf, sd...)
	}
	if size > uint64(len(buf)) {
		return nil, fmt.Errorf("%w: CFB stream declares %d bytes but its chain holds %d", ErrCorruptedPackage, size, len(buf))
	}
	return buf[:size], nil
}

// readDirectory parses the directory entry chain into f.dir.
func (f *cfbFile) readDirectory(firstDirSector uint32) error {
	sectors, err := f.chain(firstDirSector)
	if err != nil {
		return err
	}
	for _, s := range sectors {
		sd, err := f.sectorData(s)
		if err != nil {
			return err
		}
		for off := 0; off+cfbDirEntrySize <= len(sd); off += cfbDirEntrySize {
			f.dir = append(f.dir, parseDirEntry(sd[off:off+cfbDirEntrySize]))
		}
	}
	if len(f.dir) == 0 || f.dir[0].objectType != cfbTypeRoot {
		return fmt.Errorf("%w: CFB directory has no root entry", ErrCorruptedPackage)
	}
	return nil
}

// parseDirEntry decodes one 128-byte directory entry.
func parseDirEntry(b []byte) cfbDirEntry {
	nameLen := int(binary.LittleEndian.Uint16(b[64:66]))
	var name string
	if nameLen >= 2 && nameLen <= 64 {
		// nameLen counts bytes including the UTF-16 null terminator.
		u16 := make([]uint16, 0, nameLen/2)
		for i := 0; i+2 <= nameLen-2; i += 2 {
			u16 = append(u16, binary.LittleEndian.Uint16(b[i:i+2]))
		}
		name = string(utf16.Decode(u16))
	}
	return cfbDirEntry{
		name:       name,
		objectType: b[66],
		startSect:  binary.LittleEndian.Uint32(b[116:120]),
		size:       binary.LittleEndian.Uint64(b[120:128]),
	}
}

// buildMiniFAT loads the mini-FAT sector chain into f.miniFAT.
func (f *cfbFile) buildMiniFAT(firstMiniFATSector, numMiniFATSectors uint32) error {
	if numMiniFATSectors == 0 || firstMiniFATSector == cfbEndOfChain {
		return nil
	}
	sectors, err := f.chain(firstMiniFATSector)
	if err != nil {
		return err
	}
	entriesPerSector := f.sectorSize / 4
	f.miniFAT = make([]uint32, 0, len(sectors)*entriesPerSector)
	for _, s := range sectors {
		sd, err := f.sectorData(s)
		if err != nil {
			return err
		}
		for j := 0; j < entriesPerSector; j++ {
			f.miniFAT = append(f.miniFAT, binary.LittleEndian.Uint32(sd[j*4:j*4+4]))
		}
	}
	return nil
}

// loadMiniStream materializes the mini stream container (the root entry's
// regular-FAT stream) so mini streams can be sliced out of it.
func (f *cfbFile) loadMiniStream() error {
	root := f.dir[0]
	if root.size == 0 || root.startSect == cfbEndOfChain {
		return nil
	}
	ms, err := f.readRegularStream(root.startSect, root.size)
	if err != nil {
		return err
	}
	f.miniStream = ms
	return nil
}

// readMiniStream reads a stream stored in the mini stream, following the
// mini-FAT from start and truncating to size.
func (f *cfbFile) readMiniStream(start uint32, size uint64) ([]byte, error) {
	// size comes from the directory entry and is attacker-controlled; the chain
	// can hold at most the whole mini stream, so cap the pre-allocation to the
	// reachable bytes. A lying size then grows the buffer only as far as real
	// data allows instead of forcing one huge allocation.
	capHint := size
	if reachable := uint64(len(f.miniStream)); capHint > reachable {
		capHint = reachable
	}
	buf := make([]byte, 0, capHint)
	seen := make(map[uint32]bool)
	for s := start; s != cfbEndOfChain && s <= cfbMaxRegSect; {
		if int(s) >= len(f.miniFAT) {
			return nil, fmt.Errorf("%w: CFB mini sector %d is out of mini-FAT range", ErrCorruptedPackage, s)
		}
		if seen[s] {
			return nil, fmt.Errorf("%w: CFB mini sector chain loops at %d", ErrCorruptedPackage, s)
		}
		seen[s] = true
		off := int(s) * f.miniSectorSize
		if off+f.miniSectorSize > len(f.miniStream) {
			return nil, fmt.Errorf("%w: CFB mini sector %d is out of range", ErrCorruptedPackage, s)
		}
		buf = append(buf, f.miniStream[off:off+f.miniSectorSize]...)
		s = f.miniFAT[s]
	}
	if size > uint64(len(buf)) {
		return nil, fmt.Errorf("%w: CFB mini stream declares %d bytes but its chain holds %d", ErrCorruptedPackage, size, len(buf))
	}
	return buf[:size], nil
}

// stream returns the named stream's bytes. It matches the directory entry name
// case-sensitively (CFB stream names are case-sensitive in practice for the
// EncryptionInfo/EncryptedPackage streams Office writes).
func (f *cfbFile) stream(name string) ([]byte, error) {
	for i := 1; i < len(f.dir); i++ {
		e := f.dir[i]
		if e.objectType != cfbTypeStream || e.name != name {
			continue
		}
		if e.size < uint64(f.miniCutoff) {
			return f.readMiniStream(e.startSect, e.size)
		}
		return f.readRegularStream(e.startSect, e.size)
	}
	return nil, fmt.Errorf("%w: CFB container has no %q stream", ErrCorruptedPackage, name)
}
