package opc

// CFB writer: emits a minimal, conformant Compound File Binary container for a
// fixed set of named streams (in practice EncryptionInfo and EncryptedPackage).
// It uses version-3 512-byte sectors, 64-byte mini sectors, and a 4096-byte
// mini-stream cutoff — the layout Office itself writes for encrypted documents.
// Streams below the cutoff go in the mini stream; larger streams get their own
// regular-FAT chains. DIFAT sectors are emitted when a file needs more than 109
// FAT sectors, so packages of arbitrary size round-trip.

import (
	"encoding/binary"
	"io"
	"sort"
	"unicode/utf16"
)

const (
	cfbWriteSectorSize = 512
	cfbWriteMiniSize   = 64
	cfbWriteMiniCutoff = 4096
	cfbFATEntries      = cfbWriteSectorSize / 4 // 128 FAT entries per sector
	cfbDirPerSector    = cfbWriteSectorSize / cfbDirEntrySize
	cfbHeaderDIFAT     = 109 // FAT sector locations stored in the header
	cfbDIFATPerSector  = cfbFATEntries - 1
)

// cfbStream is one named stream to place in a written container.
type cfbStream struct {
	name string
	data []byte
}

// cfbWriteEntry is the internal per-stream bookkeeping during layout.
type cfbWriteEntry struct {
	name  string
	data  []byte
	mini  bool
	start uint32 // regular sector or mini-sector index of the stream's first sector

	// Directory tree sibling links, filled once the entries are sorted. The
	// stream entries never have children, so no child link is tracked here.
	left, right uint32
}

// writeCFB serializes streams into a CFB container written to w.
func writeCFB(w io.Writer, streams []cfbStream) error {
	entries := make([]*cfbWriteEntry, len(streams))
	for i, s := range streams {
		entries[i] = &cfbWriteEntry{
			name: s.name,
			data: s.data,
			mini: len(s.data) < cfbWriteMiniCutoff,
		}
	}

	// Build the mini stream from the small streams, recording each one's first
	// mini-sector index, and build the mini FAT that chains them.
	var miniStream []byte
	var miniFAT []uint32
	for _, e := range entries {
		if !e.mini {
			continue
		}
		e.start = uint32(len(miniStream) / cfbWriteMiniSize)
		nSect := ceilDiv(len(e.data), cfbWriteMiniSize)
		for k := 0; k < nSect; k++ {
			next := e.start + uint32(k) + 1
			if k == nSect-1 {
				next = cfbEndOfChain
			}
			miniFAT = append(miniFAT, next)
		}
		padded := make([]byte, nSect*cfbWriteMiniSize)
		copy(padded, e.data)
		miniStream = append(miniStream, padded...)
	}

	// Sector-count accounting for the fixed-size regions.
	dataSectors := 0
	for _, e := range entries {
		if !e.mini {
			dataSectors += ceilDiv(len(e.data), cfbWriteSectorSize)
		}
	}
	miniStreamSectors := ceilDiv(len(miniStream), cfbWriteSectorSize)
	miniFATSectors := ceilDiv(len(miniFAT)*4, cfbWriteSectorSize)
	numDirEntries := len(entries) + 1 // +1 for the root entry
	dirSectors := ceilDiv(numDirEntries, cfbDirPerSector)

	nonFAT := dataSectors + miniStreamSectors + miniFATSectors + dirSectors

	// FAT and DIFAT counts are mutually dependent; iterate to a fixed point.
	fatSectors, difatSectors := 0, 0
	for {
		total := nonFAT + fatSectors + difatSectors
		newFAT := ceilDiv(total, cfbFATEntries)
		newDIFAT := 0
		if newFAT > cfbHeaderDIFAT {
			newDIFAT = ceilDiv(newFAT-cfbHeaderDIFAT, cfbDIFATPerSector)
		}
		if newFAT == fatSectors && newDIFAT == difatSectors {
			break
		}
		fatSectors, difatSectors = newFAT, newDIFAT
	}

	// Assign contiguous sector ranges in file order.
	idx := uint32(0)
	assign := func(n int) uint32 {
		s := idx
		idx += uint32(n)
		return s
	}

	// Regular stream data first; chain and record each stream's start sector.
	fat := []uint32{} // grown as we assign; final length padded to fatSectors*128
	appendChain := func(start uint32, n int) {
		for k := 0; k < n; k++ {
			if k == n-1 {
				fat = append(fat, cfbEndOfChain)
			} else {
				fat = append(fat, start+uint32(k)+1)
			}
		}
	}

	for _, e := range entries {
		if e.mini {
			continue
		}
		n := ceilDiv(len(e.data), cfbWriteSectorSize)
		e.start = assign(n)
		appendChain(e.start, n)
	}

	rootStart := uint32(cfbEndOfChain)
	if miniStreamSectors > 0 {
		rootStart = assign(miniStreamSectors)
		appendChain(rootStart, miniStreamSectors)
	}

	miniFATStart := uint32(cfbEndOfChain)
	if miniFATSectors > 0 {
		miniFATStart = assign(miniFATSectors)
		appendChain(miniFATStart, miniFATSectors)
	}

	dirStart := assign(dirSectors)
	appendChain(dirStart, dirSectors)

	fatStart := assign(fatSectors)
	difatStart := uint32(cfbEndOfChain)
	if difatSectors > 0 {
		difatStart = assign(difatSectors)
	}
	totalSectors := int(idx)

	// FAT sectors mark themselves FATSECT; DIFAT sectors mark themselves
	// DIFSECT. Append those markers after the chain entries assigned above.
	for k := 0; k < fatSectors; k++ {
		fat = append(fat, cfbFATSect)
	}
	for k := 0; k < difatSectors; k++ {
		fat = append(fat, cfbDIFSect)
	}
	// Pad the FAT to a whole number of sectors with FREESECT.
	for len(fat) < fatSectors*cfbFATEntries {
		fat = append(fat, cfbFreeSect)
	}

	// Build the directory: root at index 0, streams sorted by CFB name order
	// and linked into a balanced red-black-ish tree rooted at root.child.
	sortEntriesForTree(entries)
	rootChild := buildDirTree(entries, 0, len(entries))

	// Compose the whole file image: header + every sector.
	buf := make([]byte, cfbHeaderSize+totalSectors*cfbWriteSectorSize)

	writeCFBHeader(buf[:cfbHeaderSize], cfbHeaderFields{
		numFATSectors:      uint32(fatSectors),
		firstDirSector:     dirStart,
		firstMiniFATSector: miniFATStart,
		numMiniFATSectors:  uint32(miniFATSectors),
		firstDIFATSector:   difatStart,
		numDIFATSectors:    uint32(difatSectors),
		fatStart:           fatStart,
		difatStart:         difatStart,
		difatSectors:       difatSectors,
	})

	sectorAt := func(sector uint32) []byte {
		off := cfbHeaderSize + int(sector)*cfbWriteSectorSize
		return buf[off : off+cfbWriteSectorSize]
	}

	// Regular stream data.
	for _, e := range entries {
		if e.mini {
			continue
		}
		writeIntoSectors(buf, e.start, e.data)
	}
	// Mini stream container.
	if miniStreamSectors > 0 {
		writeIntoSectors(buf, rootStart, miniStream)
	}
	// Mini FAT.
	if miniFATSectors > 0 {
		writeUint32Region(buf, miniFATStart, miniFATSectors, miniFAT)
	}
	// Directory.
	writeDirectory(buf, dirStart, entries, rootStart, uint64(len(miniStream)), rootChild)
	// FAT.
	writeUint32Region(buf, fatStart, fatSectors, fat)
	// DIFAT sectors.
	if difatSectors > 0 {
		writeDIFATSectors(sectorAt, difatStart, difatSectors, fatStart, fatSectors)
	}

	_, err := w.Write(buf)
	return err
}

type cfbHeaderFields struct {
	numFATSectors      uint32
	firstDirSector     uint32
	firstMiniFATSector uint32
	numMiniFATSectors  uint32
	firstDIFATSector   uint32
	numDIFATSectors    uint32
	fatStart           uint32
	difatStart         uint32
	difatSectors       int
}

// writeCFBHeader fills the 512-byte header.
func writeCFBHeader(h []byte, f cfbHeaderFields) {
	copy(h[0:8], cfbSignature)
	// CLSID (8..24) stays zero.
	binary.LittleEndian.PutUint16(h[24:26], 0x003E) // minor version
	binary.LittleEndian.PutUint16(h[26:28], 0x0003) // major version (v3)
	binary.LittleEndian.PutUint16(h[28:30], 0xFFFE) // byte order (little endian)
	binary.LittleEndian.PutUint16(h[30:32], 9)      // sector shift (512)
	binary.LittleEndian.PutUint16(h[32:34], 6)      // mini sector shift (64)
	// reserved (34..40) stays zero.
	binary.LittleEndian.PutUint32(h[40:44], 0) // number of directory sectors (0 for v3)
	binary.LittleEndian.PutUint32(h[44:48], f.numFATSectors)
	binary.LittleEndian.PutUint32(h[48:52], f.firstDirSector)
	binary.LittleEndian.PutUint32(h[52:56], 0)                  // transaction signature
	binary.LittleEndian.PutUint32(h[56:60], cfbWriteMiniCutoff) // mini stream cutoff

	firstMiniFAT := f.firstMiniFATSector
	if f.numMiniFATSectors == 0 {
		firstMiniFAT = cfbEndOfChain
	}
	binary.LittleEndian.PutUint32(h[60:64], firstMiniFAT)
	binary.LittleEndian.PutUint32(h[64:68], f.numMiniFATSectors)

	firstDIFAT := uint32(cfbEndOfChain)
	if f.difatSectors > 0 {
		firstDIFAT = f.firstDIFATSector
	}
	binary.LittleEndian.PutUint32(h[68:72], firstDIFAT)
	binary.LittleEndian.PutUint32(h[72:76], f.numDIFATSectors)

	// Header DIFAT: the first 109 FAT sector locations.
	for i := 0; i < cfbHeaderDIFAT; i++ {
		loc := uint32(cfbFreeSect)
		if i < int(f.numFATSectors) {
			loc = f.fatStart + uint32(i)
		}
		binary.LittleEndian.PutUint32(h[76+i*4:80+i*4], loc)
	}
}

// writeIntoSectors copies data into the regular sectors starting at start,
// zero-padding the final sector.
func writeIntoSectors(buf []byte, start uint32, data []byte) {
	off := cfbHeaderSize + int(start)*cfbWriteSectorSize
	copy(buf[off:], data)
}

// writeUint32Region serializes a slice of uint32 into a run of nSectors regular
// sectors starting at start, padding unused trailing entries with FREESECT.
func writeUint32Region(buf []byte, start uint32, nSectors int, values []uint32) {
	off := cfbHeaderSize + int(start)*cfbWriteSectorSize
	region := buf[off : off+nSectors*cfbWriteSectorSize]
	for i := 0; i < nSectors*cfbFATEntries; i++ {
		v := uint32(cfbFreeSect)
		if i < len(values) {
			v = values[i]
		}
		binary.LittleEndian.PutUint32(region[i*4:i*4+4], v)
	}
}

// writeDIFATSectors fills the chained DIFAT sectors with the FAT sector
// locations that did not fit in the header DIFAT.
func writeDIFATSectors(sectorAt func(uint32) []byte, difatStart uint32, difatSectors int, fatStart uint32, fatSectors int) {
	remaining := make([]uint32, 0, fatSectors-cfbHeaderDIFAT)
	for i := cfbHeaderDIFAT; i < fatSectors; i++ {
		remaining = append(remaining, fatStart+uint32(i))
	}
	pos := 0
	for s := 0; s < difatSectors; s++ {
		sd := sectorAt(difatStart + uint32(s))
		for j := 0; j < cfbDIFATPerSector; j++ {
			v := uint32(cfbFreeSect)
			if pos < len(remaining) {
				v = remaining[pos]
				pos++
			}
			binary.LittleEndian.PutUint32(sd[j*4:j*4+4], v)
		}
		// Last slot points to the next DIFAT sector, or ends the chain.
		next := uint32(cfbEndOfChain)
		if s < difatSectors-1 {
			next = difatStart + uint32(s) + 1
		}
		binary.LittleEndian.PutUint32(sd[cfbDIFATPerSector*4:cfbFATEntries*4], next)
	}
}

// writeDirectory serializes the root entry and every stream entry.
func writeDirectory(buf []byte, dirStart uint32, entries []*cfbWriteEntry, rootStart uint32, miniStreamSize uint64, rootChild uint32) {
	off := cfbHeaderSize + int(dirStart)*cfbWriteSectorSize

	// Root entry.
	writeDirEntry(buf[off:off+cfbDirEntrySize], dirEntryFields{
		name:       "Root Entry",
		objectType: cfbTypeRoot,
		left:       cfbNoStream,
		right:      cfbNoStream,
		child:      rootChild,
		startSect:  rootStart,
		size:       miniStreamSize,
	})

	for i, e := range entries {
		eoff := off + (i+1)*cfbDirEntrySize
		start := e.start
		if len(e.data) == 0 {
			// A zero-length stream owns no sector, so its start must be ENDOFCHAIN.
			// The mini-stream layout leaves e.start at the next free mini-sector
			// index, which is either past the end of the mini-FAT (readMiniStream
			// rejects it as out of range) or the first sector of whichever stream
			// follows. writeTreeDirectory already writes ENDOFCHAIN here, so this
			// keeps the two writers' output identical for the no-storage case (C453).
			start = cfbEndOfChain
		}
		writeDirEntry(buf[eoff:eoff+cfbDirEntrySize], dirEntryFields{
			name:       e.name,
			objectType: cfbTypeStream,
			left:       e.left,
			right:      e.right,
			child:      cfbNoStream,
			startSect:  start,
			size:       uint64(len(e.data)),
		})
	}
}

type dirEntryFields struct {
	name               string
	objectType         byte
	left, right, child uint32
	startSect          uint32
	size               uint64
}

// writeDirEntry serializes one 128-byte directory entry.
func writeDirEntry(b []byte, f dirEntryFields) {
	u16 := utf16.Encode([]rune(f.name))
	if len(u16) > 31 { // 31 chars + null terminator = 32 units = 64 bytes
		u16 = u16[:31]
	}
	for i, c := range u16 {
		binary.LittleEndian.PutUint16(b[i*2:i*2+2], c)
	}
	nameLen := (len(u16) + 1) * 2 // include the null terminator
	binary.LittleEndian.PutUint16(b[64:66], uint16(nameLen))
	b[66] = f.objectType
	b[67] = 1 // color: black
	binary.LittleEndian.PutUint32(b[68:72], f.left)
	binary.LittleEndian.PutUint32(b[72:76], f.right)
	binary.LittleEndian.PutUint32(b[76:80], f.child)
	// CLSID (80..96), state bits (96..100), timestamps (100..116) stay zero.
	binary.LittleEndian.PutUint32(b[116:120], f.startSect)
	binary.LittleEndian.PutUint64(b[120:128], f.size)
}

// sortEntriesForTree orders the stream entries by the CFB directory collation:
// shorter UTF-16 names first, then a case-insensitive uppercase comparison.
func sortEntriesForTree(entries []*cfbWriteEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return cfbNameLess(entries[i].name, entries[j].name)
	})
}

// cfbNameLess implements the [MS-CFB] §2.6.4 name comparison.
func cfbNameLess(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	if len(ua) != len(ub) {
		return len(ua) < len(ub)
	}
	for i := range ua {
		ca, cb := upperUTF16(ua[i]), upperUTF16(ub[i])
		if ca != cb {
			return ca < cb
		}
	}
	return false
}

// upperUTF16 upper-cases a single UTF-16 code unit for name comparison (ASCII
// range is sufficient for the stream names this package writes).
func upperUTF16(c uint16) uint16 {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

// buildDirTree links the sorted entries [lo,hi) into a balanced binary tree and
// returns the directory index (1-based, since root is 0) of the subtree root,
// or NOSTREAM for an empty range.
func buildDirTree(entries []*cfbWriteEntry, lo, hi int) uint32 {
	if lo >= hi {
		return cfbNoStream
	}
	mid := (lo + hi) / 2
	entries[mid].left = buildDirTree(entries, lo, mid)
	entries[mid].right = buildDirTree(entries, mid+1, hi)
	return uint32(mid + 1) // +1 to skip the root entry at directory index 0
}

// ceilDiv returns ceil(a/b) for non-negative a and positive b.
func ceilDiv(a, b int) int {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}
