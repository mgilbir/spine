package xlsx

import (
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// colCoverage records which columns are already covered by some <col> entry.
//
// editColumn has to carve the target column out of every entry that spans it,
// across every <cols> group, and it did that by rebuilding every group's entry
// slice on every call — allocating a fresh slice per group whether or not
// anything in it changed. Setting widths column by column is the ordinary way
// to lay out a sheet and is the worst case for that: each call copies all the
// single-column entries the previous calls created, so the run costs
// O(cols^2) in time and allocation both. Measured on a fresh sheet, 1000
// columns took 9.4ms and 4000 took 217ms — a growth exponent of 2.27, worse
// than quadratic because of the allocation. editColumnRange's own doc already
// recorded that calling editColumn per column "took seconds" for a full-width
// range.
//
// A column no entry covers needs no carve at all: it is appended as a fresh
// single-column entry, which is what editColumn's own !placed branch does. That
// is the common case when laying out a sheet, and this set answers it in O(1)
// so the rebuild is skipped entirely. A column that IS covered still takes the
// full carve — splitting a spanning entry genuinely has to rewrite it.
//
// Coverage only ever grows, and only by the append path: carving a column out
// of a spanning entry replaces it with fragments that cover exactly the same
// columns. So the set is maintained on append and otherwise only rebuilt when
// it no longer describes the sheet.
//
// Like the row index (rowindex.go) this is a hint that must not be able to
// answer wrongly. It records the model and the total entry count it was built
// from; either changing rebuilds it. The entry count moves on both paths — a
// carve turns one entry into as many as three — so both update it.
type colCoverage struct {
	model   *oxml.CT_Worksheet
	entries int
	covered map[uint32]bool
}

// countColEntries returns the total number of <col> entries across all groups.
func countColEntries(ws *oxml.CT_Worksheet) int {
	n := 0
	for gi := range ws.Cols {
		n += len(ws.Cols[gi].Col)
	}
	return n
}

// colCoverageFor returns the sheet's column coverage set, rebuilding it when it
// no longer describes ws. ws must be non-nil.
func (s *Sheet) colCoverageFor(ws *oxml.CT_Worksheet) *colCoverage {
	if cov := s.colCov; cov != nil && cov.model == ws && cov.entries == countColEntries(ws) {
		return cov
	}
	return s.rebuildColCoverage(ws)
}

// rebuildColCoverage marks every column spanned by an existing entry.
//
// Ranges are clamped to MaxCol before being walked. A <col> entry in a wild
// file can carry any uint32 max — 4 billion — and marking that range would be
// an out-of-memory the file controls. Clamping loses nothing: editColumn
// rejects a column outside 1..MaxCol before it ever consults this set.
func (s *Sheet) rebuildColCoverage(ws *oxml.CT_Worksheet) *colCoverage {
	s.colCovRebuilds++
	cov := &colCoverage{
		model:   ws,
		entries: countColEntries(ws),
		covered: make(map[uint32]bool),
	}
	for gi := range ws.Cols {
		for ei := range ws.Cols[gi].Col {
			entry := &ws.Cols[gi].Col[ei]
			lo, hi := entry.Min, entry.Max
			if lo < 1 {
				lo = 1
			}
			if hi > MaxCol {
				hi = MaxCol
			}
			for c := lo; c <= hi; c++ {
				cov.covered[c] = true
			}
		}
	}
	s.colCov = cov
	return cov
}

// invalidateColCoverage drops the cached set.
func (s *Sheet) invalidateColCoverage() {
	s.colCov = nil
}
