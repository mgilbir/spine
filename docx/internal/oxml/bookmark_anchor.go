package oxml

import "strings"

// AddBookmarkAroundParagraph brackets all of the paragraph's content with a
// bookmark: the start marker is placed before the first content child and the
// end marker after the last.
func (p *CT_P) AddBookmarkAroundParagraph(id, name string) {
	p.backfillChildOrder()
	p.BookmarkStart = append(p.BookmarkStart, &CT_BookmarkStart{Id: id, Name: name})
	p.insertPChildRefAt(0, pChildRef{pChildBookmarkStart, len(p.BookmarkStart) - 1})
	p.BookmarkEnd = append(p.BookmarkEnd, &CT_BookmarkEnd{Id: id})
	p.childOrder = append(p.childOrder, pChildRef{pChildBookmarkEnd, len(p.BookmarkEnd) - 1})
}

// InsertBookmarkStartBeforeRun inserts a bookmarkStart for id/name immediately
// before run r. Returns false if r is not a direct child run.
func (p *CT_P) InsertBookmarkStartBeforeRun(r *CT_R, id, name string) bool {
	p.backfillChildOrder()
	pos := p.runChildOrderPos(r)
	if pos < 0 {
		return false
	}
	p.BookmarkStart = append(p.BookmarkStart, &CT_BookmarkStart{Id: id, Name: name})
	p.insertPChildRefAt(pos, pChildRef{pChildBookmarkStart, len(p.BookmarkStart) - 1})
	return true
}

// InsertBookmarkEndAfterRun inserts a bookmarkEnd for id immediately after run
// r. Returns false if r is not a direct child run.
func (p *CT_P) InsertBookmarkEndAfterRun(r *CT_R, id string) bool {
	p.backfillChildOrder()
	pos := p.runChildOrderPos(r)
	if pos < 0 {
		return false
	}
	p.BookmarkEnd = append(p.BookmarkEnd, &CT_BookmarkEnd{Id: id})
	p.insertPChildRefAt(pos+1, pChildRef{pChildBookmarkEnd, len(p.BookmarkEnd) - 1})
	return true
}

// MaxBookmarkID returns the highest numeric bookmark id (w:bookmarkStart)
// anywhere in the body, or -1 if none carries a numeric id. Word's table
// column bookmarks (w:colFirst/w:colLast) are direct children of a row, a cell,
// a table or the body rather than of a paragraph, so scanning paragraphs alone
// let a newly allocated bookmark reuse an id already in the document (C507).
func MaxBookmarkID(body *CT_Body) int {
	max := -1
	consider := func(starts []*CT_BookmarkStart) {
		for _, bs := range starts {
			if bs == nil {
				continue
			}
			if v, ok := atoiOK(bs.Id); ok && v > max {
				max = v
			}
		}
	}
	if body == nil {
		return max
	}
	consider(body.BookmarkStart)
	visitBlockContent(body.childOrder, body.P, body.Tbl, body.SdtBlock, blockVisitor{
		Para: func(p *CT_P) {
			if p != nil {
				consider(p.BookmarkStart)
			}
		},
		Tbl:  func(tbl *CT_Tbl) { consider(tbl.BookmarkStart) },
		Row:  func(tr *CT_Tr) { consider(tr.BookmarkStart) },
		Cell: func(tc *CT_Tc) { consider(tc.BookmarkStart) },
		Sdt: func(s *CT_SdtBlock) {
			if s.SdtContent != nil {
				consider(s.SdtContent.BookmarkStart)
			}
		},
	})
	return max
}

// scanBookmarkRange accumulates the text of runs falling between the bookmarkStart
// and bookmarkEnd markers for id. inside/sawStart/sawEnd thread the scan across
// paragraphs (a bookmark range can span several).
func (p *CT_P) scanBookmarkRange(id string, inside, sawStart, sawEnd *bool, sb *strings.Builder) {
	order := p.childOrder
	if len(order) == 0 {
		if *inside {
			writeRunsText(sb, p.R)
		}
		return
	}
	for _, ref := range order {
		switch ref.kind {
		case pChildBookmarkStart:
			if ref.index < len(p.BookmarkStart) && p.BookmarkStart[ref.index].Id == id {
				*inside = true
				*sawStart = true
			}
		case pChildBookmarkEnd:
			if ref.index < len(p.BookmarkEnd) && p.BookmarkEnd[ref.index].Id == id {
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

// BookmarkText concatenates the document text bracketed by the bookmark range
// for id across the given paragraphs (in document order). The second result
// reports whether both a start and an end marker were found (a point bookmark
// with no spanned text returns "", true; an unresolvable id returns "", false).
func BookmarkText(paras []*CT_P, id string) (string, bool) {
	var sb strings.Builder
	var inside, sawStart, sawEnd bool
	for _, p := range paras {
		if p != nil {
			p.scanBookmarkRange(id, &inside, &sawStart, &sawEnd, &sb)
		}
	}
	return sb.String(), sawStart && sawEnd
}
