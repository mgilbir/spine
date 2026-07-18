package oxml

import "strings"

// newCommentRefRun builds a run carrying only the commentReference mark, styled
// with the CommentReference character style exactly as Word emits it.
func newCommentRefRun(id string) *CT_R {
	r := &CT_R{RPr: &CT_RPr{RStyle: &CT_String{Val: "CommentReference"}}}
	r.CommentReference = []*CT_Markup{{Id: id}}
	r.childOrder = []runChildRef{{runChildCommentReference, 0}}
	return r
}

// insertPChildRefAt inserts ref into the paragraph child order at pos.
func (p *CT_P) insertPChildRefAt(pos int, ref pChildRef) {
	if pos < 0 {
		pos = 0
	}
	if pos >= len(p.childOrder) {
		p.childOrder = append(p.childOrder, ref)
		return
	}
	p.childOrder = append(p.childOrder, pChildRef{})
	copy(p.childOrder[pos+1:], p.childOrder[pos:])
	p.childOrder[pos] = ref
}

// runChildOrderPos returns the position in childOrder of the reference to run r,
// or -1 if r is not a direct child run of the paragraph.
func (p *CT_P) runChildOrderPos(r *CT_R) int {
	for i, ref := range p.childOrder {
		if ref.kind == pChildR && ref.index < len(p.R) && p.R[ref.index] == r {
			return i
		}
	}
	return -1
}

// commentRangeStartPos returns the childOrder position of the commentRangeStart
// marker for id, or -1.
func (p *CT_P) commentRangeStartPos(id string) int {
	for i, ref := range p.childOrder {
		if ref.kind == pChildCommentRangeStart && ref.index < len(p.CommentRangeStart) &&
			p.CommentRangeStart[ref.index].Id == id {
			return i
		}
	}
	return -1
}

// commentRangeEndPos returns the childOrder position of the commentRangeEnd
// marker for id, or -1.
func (p *CT_P) commentRangeEndPos(id string) int {
	for i, ref := range p.childOrder {
		if ref.kind == pChildCommentRangeEnd && ref.index < len(p.CommentRangeEnd) &&
			p.CommentRangeEnd[ref.index].Id == id {
			return i
		}
	}
	return -1
}

// commentRefRunPos returns the childOrder position of the run carrying the
// commentReference mark for id, or -1.
func (p *CT_P) commentRefRunPos(id string) int {
	for i, ref := range p.childOrder {
		if ref.kind == pChildR && ref.index < len(p.R) {
			for _, m := range p.R[ref.index].CommentReference {
				if m.Id == id {
					return i
				}
			}
		}
	}
	return -1
}

// AddCommentAroundParagraph brackets all of the paragraph's content with a
// comment range for id and appends the reference-mark run at the end.
func (p *CT_P) AddCommentAroundParagraph(id string) {
	p.backfillChildOrder()
	p.CommentRangeStart = append(p.CommentRangeStart, &CT_CommentRangeStart{Id: id})
	p.insertPChildRefAt(0, pChildRef{pChildCommentRangeStart, len(p.CommentRangeStart) - 1})
	p.CommentRangeEnd = append(p.CommentRangeEnd, &CT_CommentRangeEnd{Id: id})
	p.childOrder = append(p.childOrder, pChildRef{pChildCommentRangeEnd, len(p.CommentRangeEnd) - 1})
	p.R = append(p.R, newCommentRefRun(id))
	p.childOrder = append(p.childOrder, pChildRef{pChildR, len(p.R) - 1})
}

// AddCommentAroundRun brackets a single run with a comment range for id and
// places the reference-mark run right after it. Returns false if r is not a
// direct child run.
func (p *CT_P) AddCommentAroundRun(r *CT_R, id string) bool {
	p.backfillChildOrder()
	pos := p.runChildOrderPos(r)
	if pos < 0 {
		return false
	}
	p.CommentRangeStart = append(p.CommentRangeStart, &CT_CommentRangeStart{Id: id})
	p.insertPChildRefAt(pos, pChildRef{pChildCommentRangeStart, len(p.CommentRangeStart) - 1})
	// r is now at pos+1; the end marker and reference run follow it.
	p.CommentRangeEnd = append(p.CommentRangeEnd, &CT_CommentRangeEnd{Id: id})
	p.insertPChildRefAt(pos+2, pChildRef{pChildCommentRangeEnd, len(p.CommentRangeEnd) - 1})
	p.R = append(p.R, newCommentRefRun(id))
	p.insertPChildRefAt(pos+3, pChildRef{pChildR, len(p.R) - 1})
	return true
}

// InsertCommentStartBeforeRun inserts a commentRangeStart for id immediately
// before run r. Returns false if r is not a direct child run.
func (p *CT_P) InsertCommentStartBeforeRun(r *CT_R, id string) bool {
	p.backfillChildOrder()
	pos := p.runChildOrderPos(r)
	if pos < 0 {
		return false
	}
	p.CommentRangeStart = append(p.CommentRangeStart, &CT_CommentRangeStart{Id: id})
	p.insertPChildRefAt(pos, pChildRef{pChildCommentRangeStart, len(p.CommentRangeStart) - 1})
	return true
}

// InsertCommentEndAndRefAfterRun inserts a commentRangeEnd for id and the
// reference-mark run immediately after run r. Returns false if r is not a direct
// child run.
func (p *CT_P) InsertCommentEndAndRefAfterRun(r *CT_R, id string) bool {
	p.backfillChildOrder()
	pos := p.runChildOrderPos(r)
	if pos < 0 {
		return false
	}
	p.CommentRangeEnd = append(p.CommentRangeEnd, &CT_CommentRangeEnd{Id: id})
	p.insertPChildRefAt(pos+1, pChildRef{pChildCommentRangeEnd, len(p.CommentRangeEnd) - 1})
	p.R = append(p.R, newCommentRefRun(id))
	p.insertPChildRefAt(pos+2, pChildRef{pChildR, len(p.R) - 1})
	return true
}

// NestCommentStartAfter inserts a commentRangeStart for newID immediately after
// the commentRangeStart for parentID (nesting a reply's range inside the
// parent's). Returns false if parentID's start marker is not in this paragraph.
func (p *CT_P) NestCommentStartAfter(parentID, newID string) bool {
	pos := p.commentRangeStartPos(parentID)
	if pos < 0 {
		return false
	}
	p.CommentRangeStart = append(p.CommentRangeStart, &CT_CommentRangeStart{Id: newID})
	p.insertPChildRefAt(pos+1, pChildRef{pChildCommentRangeStart, len(p.CommentRangeStart) - 1})
	return true
}

// NestCommentEndBefore inserts a commentRangeEnd for newID immediately before
// the commentRangeEnd for parentID. Returns false if parentID's end marker is
// not in this paragraph.
func (p *CT_P) NestCommentEndBefore(parentID, newID string) bool {
	pos := p.commentRangeEndPos(parentID)
	if pos < 0 {
		return false
	}
	p.CommentRangeEnd = append(p.CommentRangeEnd, &CT_CommentRangeEnd{Id: newID})
	p.insertPChildRefAt(pos, pChildRef{pChildCommentRangeEnd, len(p.CommentRangeEnd) - 1})
	return true
}

// AppendCommentRefRunAfter inserts the reference-mark run for newID immediately
// after the reference-mark run for parentID. Returns false if parentID's
// reference run is not in this paragraph.
func (p *CT_P) AppendCommentRefRunAfter(parentID, newID string) bool {
	pos := p.commentRefRunPos(parentID)
	if pos < 0 {
		return false
	}
	p.R = append(p.R, newCommentRefRun(newID))
	p.insertPChildRefAt(pos+1, pChildRef{pChildR, len(p.R) - 1})
	return true
}

// scanCommentRange walks the paragraph content in order accumulating the text of
// runs that fall inside the comment range for id. inside/sawStart/sawEnd thread
// the scan across paragraphs (a range can span several).
func (p *CT_P) scanCommentRange(id string, inside, sawStart, sawEnd *bool, sb *strings.Builder) {
	order := p.childOrder
	if len(order) == 0 {
		if *inside {
			writeRunsText(sb, p.R)
		}
		return
	}
	for _, ref := range order {
		switch ref.kind {
		case pChildCommentRangeStart:
			if ref.index < len(p.CommentRangeStart) && p.CommentRangeStart[ref.index].Id == id {
				*inside = true
				*sawStart = true
			}
		case pChildCommentRangeEnd:
			if ref.index < len(p.CommentRangeEnd) && p.CommentRangeEnd[ref.index].Id == id {
				*inside = false
				*sawEnd = true
			}
		case pChildR:
			if *inside && ref.index < len(p.R) {
				writeRunText(sb, p.R[ref.index])
			}
		case pChildHyperlink:
			if *inside && ref.index < len(p.Hyperlink) {
				writeRunsText(sb, p.Hyperlink[ref.index].R)
			}
		case pChildIns:
			if *inside && ref.index < len(p.Ins) {
				writeRunsText(sb, p.Ins[ref.index].R)
			}
		case pChildFldSimple:
			if *inside && ref.index < len(p.FldSimple) {
				writeRunsText(sb, p.FldSimple[ref.index].R)
			}
		case pChildSdtRun:
			if *inside && ref.index < len(p.SdtRun) && p.SdtRun[ref.index].SdtContent != nil {
				writeRunsText(sb, p.SdtRun[ref.index].SdtContent.R)
			}
		}
	}
}

// AnchorText concatenates the document text bracketed by the comment range for
// id across the given paragraphs (in document order). The second result reports
// whether both a start and an end marker were found (a point anchor with no
// spanned text returns "", true; an unresolvable id returns "", false).
func AnchorText(paras []*CT_P, id string) (string, bool) {
	var sb strings.Builder
	var inside, sawStart, sawEnd bool
	for _, p := range paras {
		if p != nil {
			p.scanCommentRange(id, &inside, &sawStart, &sawEnd, &sb)
		}
	}
	return sb.String(), sawStart && sawEnd
}
