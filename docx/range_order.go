package docx

import "github.com/mgilbir/spine/docx/internal/oxml"

// Shared endpoint checking for the two range-marker APIs (AddCommentOnRange and
// AddBookmarkOnRange).
//
// Both place a start marker before one run and an end marker after another, and
// both already refuse endpoints that are not direct paragraph children (C296) —
// a marker cannot be spliced into a run's parent when the parent is a hyperlink
// or a tracked-change block. Neither checked that the endpoints were the right
// way round, so passing them reversed emitted commentRangeEnd before
// commentRangeStart (and bookmarkEnd before bookmarkStart): schema-shaped but
// semantically inverted markup that Validate() does not see and AnchorText()
// reads back as the wrong text (C404).

// orderRunRange validates a pair of range endpoints and returns them in
// document order.
//
// paras is the ordered paragraph list the range is resolved across (the same
// walk the range readers use, so "resolvable" here means "resolvable by the
// reader that will later read this range back"). ok is false when either
// endpoint is nil, is not a direct child run of its paragraph, or sits in a
// paragraph the walk does not reach — in each case the caller must place no
// markers at all.
//
// When both endpoints resolve but end precedes start, the two are returned
// swapped: the caller asked for the span between them, and emitting it in the
// order the document actually reads is what honors that. Endpoints in the same
// position (the same run passed twice) are left as they are — that is a
// well-formed point range.
func orderRunRange(paras []*oxml.CT_P, start, end *Run) (s, e *Run, ok bool) {
	if start == nil || end == nil || start.paragraph == nil || end.paragraph == nil {
		return nil, nil, false
	}
	if !start.paragraph.p.HasDirectChildRun(start.r) || !end.paragraph.p.HasDirectChildRun(end.r) {
		return nil, nil, false
	}
	startPara, startOK := paragraphPosition(paras, start.paragraph.p)
	endPara, endOK := paragraphPosition(paras, end.paragraph.p)
	if !startOK || !endOK {
		// An endpoint in a paragraph the range walk never reaches would produce
		// a marker no reader can pair up. Refuse rather than write a range that
		// only half exists.
		return nil, nil, false
	}
	if startPara != endPara {
		if endPara < startPara {
			return end, start, true
		}
		return start, end, true
	}
	if end.paragraph.p.DirectChildRunIndex(end.r) < start.paragraph.p.DirectChildRunIndex(start.r) {
		return end, start, true
	}
	return start, end, true
}

// paragraphPosition returns the index of p in paras, and whether it was found.
func paragraphPosition(paras []*oxml.CT_P, p *oxml.CT_P) (int, bool) {
	for i, cand := range paras {
		if cand == p {
			return i, true
		}
	}
	return 0, false
}
