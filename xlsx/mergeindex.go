package xlsx

import (
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// mergeBounds is the bounding box of every merged range on a sheet, used to
// answer the overlap check MergeCells runs before accepting a new merge.
//
// That check compared the candidate against every existing <mergeCell>,
// re-parsing each stored reference as it went, so merging cells cost
// O(merges^2) with a string parse per comparison: 1000 merges took 17.6ms and
// 4000 took 276ms. Merging one range per row — a banner column, a repeated
// header — is the ordinary shape and the worst case.
//
// A candidate that falls outside the bounding box cannot overlap anything, so
// it is accepted without the scan. That is the common case for merges laid out
// down a sheet: each new range sits below every one already there. A candidate
// that does fall inside still takes the full scan, because the box says only
// that an overlap is possible.
//
// The box records the model and the number of merges it was built from, so a
// merge or unmerge that does not maintain it costs a rebuild rather than a
// wrong answer. The direction that matters is a box that is too SMALL, which
// would accept an overlapping merge that Excel then refuses to open; growing
// the box on every accepted merge is what keeps it from happening.
type mergeBounds struct {
	model  *oxml.CT_Worksheet
	merges int
	box    cellRange
	any    bool
}

// mergeCount returns how many merged ranges ws carries.
func mergeCount(ws *oxml.CT_Worksheet) int {
	if ws == nil || ws.MergeCells == nil {
		return 0
	}
	return len(ws.MergeCells.MergeCell)
}

// mergeBoundsFor returns the sheet's merge bounding box, rebuilding it when it
// no longer describes ws.
func (s *Sheet) mergeBoundsFor(ws *oxml.CT_Worksheet) *mergeBounds {
	if b := s.mergeBox; b != nil && b.model == ws && b.merges == mergeCount(ws) {
		return b
	}
	s.mergeBoxRebuilds++
	b := &mergeBounds{model: ws, merges: mergeCount(ws)}
	if ws != nil && ws.MergeCells != nil {
		for _, mc := range ws.MergeCells.MergeCell {
			rng, err := parseCellRangeRef(mc.Ref)
			if err != nil {
				// An unparseable entry imposes no constraint, exactly as the
				// scan treated it. It also cannot be described by the box, so
				// it must not widen it.
				continue
			}
			b.box, b.any = widened(b.box, b.any, rng)
		}
	}
	s.mergeBox = b
	return b
}

// widened returns box, extended to cover rng, and true. It is a function
// rather than a method on mergeBounds so that widening the box is not a write
// through a receiver: the mutation guard reads any receiver-field write as a
// change to the document model, and this is cache bookkeeping.
func widened(box cellRange, set bool, rng cellRange) (cellRange, bool) {
	if !set {
		return rng, true
	}
	box.minRow = min(box.minRow, rng.minRow)
	box.minCol = min(box.minCol, rng.minCol)
	box.maxRow = max(box.maxRow, rng.maxRow)
	box.maxCol = max(box.maxCol, rng.maxCol)
	return box, true
}

// mayOverlap reports whether rng could overlap an existing merge. False is
// conclusive; true means the caller must do the full comparison.
func (b *mergeBounds) mayOverlap(rng cellRange) bool {
	return b.any && b.box.overlaps(rng)
}
