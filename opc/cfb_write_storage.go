package opc

// This file extends the CFB writer (cfb_write.go) to containers that nest
// storages (sub-directories), which the flat two-stream writer cannot express.
// It is used only when the caller asks for the optional \x06DataSpaces metadata
// streams (see dataspaces.go); a container with no storages continues to go
// through the original writeCFB path unchanged, so the default encrypted-save
// output is byte-for-byte identical to before.

import (
	"io"
	"sort"
)

// cfbStorage is a named storage (sub-directory) holding streams and, in turn,
// nested storages.
type cfbStorage struct {
	name     string
	streams  []cfbStream
	storages []cfbStorage
}

// cfbNode is the internal, mutable representation of one directory entry during
// layout of a storage-bearing container.
type cfbNode struct {
	name      string
	isStorage bool
	data      []byte // stream payload; nil for storages

	index              uint32 // assigned directory index
	mini               bool
	start              uint32 // regular sector or mini-sector index of the first sector
	left, right, child uint32
}

// writeCFBWithStorages writes a CFB container holding the given top-level streams
// and storages. With no storages it delegates to the flat writer for identical
// output; otherwise it lays out the full directory tree.
func writeCFBWithStorages(w io.Writer, streams []cfbStream, storages []cfbStorage) error {
	if len(storages) == 0 {
		return writeCFB(w, streams)
	}

	// Directory indices are assigned root=0 then depth-first over the tree; the
	// storageTree carries the child links that cfbNode (a flat record) does not.
	tree := newStorageTree(streams, storages)
	var all []*cfbNode
	assignFromTree(tree, &all)
	root := tree.node
	root.child = linkChildren(tree.children)

	// Split streams into mini (below the cutoff) and regular, and build the mini
	// stream plus its FAT.
	var miniStream []byte
	var miniFAT []uint32
	for _, n := range all {
		if n.isStorage || len(n.data) == 0 {
			continue
		}
		if len(n.data) < cfbWriteMiniCutoff {
			n.mini = true
			n.start = uint32(len(miniStream) / cfbWriteMiniSize)
			nSect := ceilDiv(len(n.data), cfbWriteMiniSize)
			for k := 0; k < nSect; k++ {
				next := n.start + uint32(k) + 1
				if k == nSect-1 {
					next = cfbEndOfChain
				}
				miniFAT = append(miniFAT, next)
			}
			padded := make([]byte, nSect*cfbWriteMiniSize)
			copy(padded, n.data)
			miniStream = append(miniStream, padded...)
		}
	}

	dataSectors := 0
	for _, n := range all {
		if n.isStorage || n.mini || len(n.data) == 0 {
			continue
		}
		dataSectors += ceilDiv(len(n.data), cfbWriteSectorSize)
	}
	miniStreamSectors := ceilDiv(len(miniStream), cfbWriteSectorSize)
	miniFATSectors := ceilDiv(len(miniFAT)*4, cfbWriteSectorSize)
	numDirEntries := len(all) + 1
	dirSectors := ceilDiv(numDirEntries, cfbDirPerSector)

	nonFAT := dataSectors + miniStreamSectors + miniFATSectors + dirSectors
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

	idx := uint32(0)
	assign := func(n int) uint32 {
		s := idx
		idx += uint32(n)
		return s
	}
	var fat []uint32
	appendChain := func(start uint32, n int) {
		for k := 0; k < n; k++ {
			if k == n-1 {
				fat = append(fat, cfbEndOfChain)
			} else {
				fat = append(fat, start+uint32(k)+1)
			}
		}
	}

	for _, n := range all {
		if n.isStorage || n.mini || len(n.data) == 0 {
			continue
		}
		nSect := ceilDiv(len(n.data), cfbWriteSectorSize)
		n.start = assign(nSect)
		appendChain(n.start, nSect)
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

	for k := 0; k < fatSectors; k++ {
		fat = append(fat, cfbFATSect)
	}
	for k := 0; k < difatSectors; k++ {
		fat = append(fat, cfbDIFSect)
	}
	for len(fat) < fatSectors*cfbFATEntries {
		fat = append(fat, cfbFreeSect)
	}

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

	for _, n := range all {
		if n.isStorage || n.mini || len(n.data) == 0 {
			continue
		}
		writeIntoSectors(buf, n.start, n.data)
	}
	if miniStreamSectors > 0 {
		writeIntoSectors(buf, rootStart, miniStream)
	}
	if miniFATSectors > 0 {
		writeUint32Region(buf, miniFATStart, miniFATSectors, miniFAT)
	}
	writeTreeDirectory(buf, dirStart, root, all, rootStart, uint64(len(miniStream)))
	writeUint32Region(buf, fatStart, fatSectors, fat)
	if difatSectors > 0 {
		writeDIFATSectors(sectorAt, difatStart, difatSectors, fatStart, fatSectors)
	}

	_, err := w.Write(buf)
	return err
}

// storageTree mirrors the caller's streams/storages as a node tree that carries
// child links, keeping cfbNode itself a flat record.
type storageTree struct {
	node     *cfbNode
	children []*storageTree
}

func newStorageTree(streams []cfbStream, storages []cfbStorage) *storageTree {
	root := &storageTree{node: &cfbNode{name: "Root Entry", isStorage: true}}
	root.children = buildTreeChildren(streams, storages)
	return root
}

func buildTreeChildren(streams []cfbStream, storages []cfbStorage) []*storageTree {
	var out []*storageTree
	for _, s := range streams {
		out = append(out, &storageTree{node: &cfbNode{name: s.name, data: s.data}})
	}
	for _, st := range storages {
		t := &storageTree{node: &cfbNode{name: st.name, isStorage: true}}
		t.children = buildTreeChildren(st.streams, st.storages)
		out = append(out, t)
	}
	return out
}

// assignFromTree walks the tree depth-first, assigning each non-root node a
// directory index and collecting the flat node list.
func assignFromTree(t *storageTree, all *[]*cfbNode) {
	for _, c := range t.children {
		c.node.index = uint32(len(*all) + 1)
		*all = append(*all, c.node)
	}
	for _, c := range t.children {
		assignFromTree(c, all)
	}
}

// linkChildren sorts a sibling group by the CFB name collation, links it into a
// balanced tree, and returns the directory index of the subtree root (or
// NOSTREAM when empty). It recurses into storages so each storage's own child
// pointer is set.
func linkChildren(children []*storageTree) uint32 {
	for _, c := range children {
		if c.node.isStorage {
			c.node.child = linkChildren(c.children)
		}
	}
	sorted := make([]*storageTree, len(children))
	copy(sorted, children)
	sort.SliceStable(sorted, func(i, j int) bool {
		return cfbNameLess(sorted[i].node.name, sorted[j].node.name)
	})
	return buildBalancedTree(sorted, 0, len(sorted))
}

// buildBalancedTree links sorted[lo:hi] into a balanced binary tree, setting
// left/right on each node, and returns the subtree root's directory index.
func buildBalancedTree(sorted []*storageTree, lo, hi int) uint32 {
	if lo >= hi {
		return cfbNoStream
	}
	mid := (lo + hi) / 2
	sorted[mid].node.left = buildBalancedTree(sorted, lo, mid)
	sorted[mid].node.right = buildBalancedTree(sorted, mid+1, hi)
	return sorted[mid].node.index
}

// writeTreeDirectory serializes the root entry and every node in index order.
func writeTreeDirectory(buf []byte, dirStart uint32, root *cfbNode, all []*cfbNode, rootStart uint32, miniStreamSize uint64) {
	off := cfbHeaderSize + int(dirStart)*cfbWriteSectorSize
	writeDirEntry(buf[off:off+cfbDirEntrySize], dirEntryFields{
		name:       root.name,
		objectType: cfbTypeRoot,
		left:       cfbNoStream,
		right:      cfbNoStream,
		child:      root.child,
		startSect:  rootStart,
		size:       miniStreamSize,
	})
	for _, n := range all {
		eoff := off + int(n.index)*cfbDirEntrySize
		f := dirEntryFields{
			name:  n.name,
			left:  n.left,
			right: n.right,
			child: cfbNoStream,
		}
		if n.isStorage {
			f.objectType = cfbTypeStorage
			f.child = n.child
			// Storage entries carry no stream: StartingSector and StreamSize are 0.
		} else {
			f.objectType = cfbTypeStream
			f.startSect = n.start
			f.size = uint64(len(n.data))
			if len(n.data) == 0 {
				f.startSect = cfbEndOfChain
			}
		}
		writeDirEntry(buf[eoff:eoff+cfbDirEntrySize], f)
	}
}
