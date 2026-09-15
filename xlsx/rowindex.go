package xlsx

import (
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// rowIndex caches the position of each row number within a worksheet's
// SheetData.Row slice, so resolving a row is O(1) rather than a walk from the
// start of the sheet.
//
// Without it every row lookup — Sheet.Cell, findCell, rowEntry, editRow,
// rowCells.locate — scanned SheetData.Row from the beginning. Filling a sheet
// row by row, the ordinary way to build one, is the worst case for that scan:
// the row being written is always the last element or absent, so every existing
// row is compared before the answer is found, making the build O(rows^2 x cols).
// Issue #314 measured 10,000 rows at 3.85s against excelize's 0.29s, and 100,000
// rows still running after ten minutes at 100% CPU. editRowRange already carried
// a one-shot form of this map for exactly this reason; this is that map made
// durable and shared by every row lookup in the package.
//
// The index is a hint, never an authority. Three things keep it honest:
//
//   - It records the model it was built against and the slice length it
//     described. A different model (one a lazy parse or ensureWS materialized)
//     or a different length (rows appended behind its back) rebuilds it, so a
//     maintenance site missed here costs a rebuild, not a wrong answer.
//   - Every hit is verified in O(1) against the row it points at before being
//     returned. An in-place reordering that leaves the length alone is caught
//     here: marshalSheetData sorts SheetData.Row in place, and prunedRows hands
//     it the durable slice whenever there is nothing to prune.
//   - writeSheetPart drops the index once marshalling is done, so that sort is
//     not observed through a stale index in the first place. The verification
//     above is the backstop, not the only guard.
//
// A miss needs no verification: the only same-model, same-length mutation in the
// package is that sort, which permutes rows without changing which row numbers
// are present.
//
// Rows whose number is not derivable — no r attribute and no parseable cell
// reference, which is legal (C73) — are unaddressable and stay out of the index,
// exactly as the linear scan skipped them. Duplicate row numbers keep the first
// occurrence, matching the scan's first-match-wins result.
//
// Locking: the entry points below (lookupRow, rowsAreUnique, appendRow) take
// Sheet.rowIdxMu; rowIndexFor and rebuildRowIndex assume it is already held.
// The index is built lazily by accessors that are otherwise reads, so without
// the lock a workbook that is only being read from several goroutines races —
// which it did not before this index existed. Mutating a workbook concurrently
// was never safe and still is not; this restores the read side only.
//
// The lock is uncontended on the single-threaded paths that dominate, and the
// cost does not show: building a 640,000-cell sheet measures the same with it
// as without.
type rowIndex struct {
	model    *oxml.CT_Worksheet
	rows     int
	byNumber map[uint32]int
	// dupRows records that two rows carry the same number, so the indexed
	// first match is not provably the only one. Callers that historically
	// examined every row with a matching number — findCell, which looks for
	// the reference in each of them — fall back to the full scan when it is
	// set. It is false for every well-formed sheet.
	dupRows bool
}

// rowIndexFor returns the sheet's row index, rebuilding it when it does not
// describe ws and its current row slice. ws must be non-nil.
func (s *Sheet) rowIndexFor(ws *oxml.CT_Worksheet) *rowIndex {
	if idx := s.rowIdx; idx != nil && idx.model == ws && idx.rows == len(ws.SheetData.Row) {
		return idx
	}
	return s.rebuildRowIndex(ws)
}

// rebuildRowIndex indexes ws.SheetData.Row from scratch and caches the result.
func (s *Sheet) rebuildRowIndex(ws *oxml.CT_Worksheet) *rowIndex {
	s.rowIdxRebuilds++
	rows := ws.SheetData.Row
	idx := &rowIndex{
		model:    ws,
		rows:     len(rows),
		byNumber: make(map[uint32]int, len(rows)),
	}
	for i := range rows {
		if rn, ok := rowNumberOf(&rows[i]); ok {
			if _, dup := idx.byNumber[rn]; dup {
				idx.dupRows = true
			} else {
				idx.byNumber[rn] = i
			}
		}
	}
	s.rowIdx = idx
	return idx
}

// invalidateRowIndex drops the cached index. Call it after anything that may
// have reordered SheetData.Row without changing its length.
func (s *Sheet) invalidateRowIndex() {
	s.rowIdx = nil
}

// lookupRow returns the position of the row numbered n in ws.SheetData.Row,
// reporting false when the sheet has no addressable row with that number. It is
// the indexed replacement for the linear scan and returns the same row the scan
// did, first match included.
func (s *Sheet) lookupRow(ws *oxml.CT_Worksheet, n uint32) (int, bool) {
	s.rowIdxMu.Lock()
	defer s.rowIdxMu.Unlock()

	idx := s.rowIndexFor(ws)
	i, ok := idx.byNumber[n]
	if !ok {
		return 0, false
	}
	if rowIndexHolds(ws, i, n) {
		return i, true
	}
	// The hint points at a row that is no longer the one it named, so the slice
	// was reordered without changing length or model. Rebuild and answer from
	// the fresh index.
	idx = s.rebuildRowIndex(ws)
	if i, ok = idx.byNumber[n]; ok && rowIndexHolds(ws, i, n) {
		return i, true
	}
	return 0, false
}

// rowIndexHolds reports whether position i of ws.SheetData.Row really is a row
// numbered n.
func rowIndexHolds(ws *oxml.CT_Worksheet, i int, n uint32) bool {
	rows := ws.SheetData.Row
	if i < 0 || i >= len(rows) {
		return false
	}
	rn, ok := rowNumberOf(&rows[i])
	return ok && rn == n
}

// appendRow appends r to ws.SheetData.Row and records its position, so a loop
// that builds a sheet row by row keeps the index warm instead of rebuilding it
// on every append. It returns the new row's position.
func (s *Sheet) appendRow(ws *oxml.CT_Worksheet, r oxml.CT_Row) int {
	s.rowIdxMu.Lock()
	defer s.rowIdxMu.Unlock()
	// Take the index before the append so it still describes the current slice;
	// afterwards its recorded length is updated to match.
	idx := s.rowIndexFor(ws)
	ws.SheetData.Row = append(ws.SheetData.Row, r)
	i := len(ws.SheetData.Row) - 1
	if rn, ok := rowNumberOf(&ws.SheetData.Row[i]); ok {
		if _, dup := idx.byNumber[rn]; dup {
			idx.dupRows = true
		} else {
			idx.byNumber[rn] = i
		}
	}
	idx.rows = len(ws.SheetData.Row)
	return i
}

// rowsAreUnique reports whether every addressable row in ws carries a distinct
// number, which makes the index's first match the only match. Callers that used
// to examine every row with a matching number consult this before trusting a
// single indexed hit.
func (s *Sheet) rowsAreUnique(ws *oxml.CT_Worksheet) bool {
	s.rowIdxMu.Lock()
	defer s.rowIdxMu.Unlock()
	return !s.rowIndexFor(ws).dupRows
}
