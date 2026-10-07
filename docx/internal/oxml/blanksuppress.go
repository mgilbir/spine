package oxml

import (
	"bytes"
	"encoding/xml"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Blank-line suppression: after a merge, Word removes a line (paragraph) that
// the merge left empty. This file removes such paragraphs from block content,
// conservatively: anything that a removal would lose or break keeps the
// paragraph.

// SuppressBlankParagraphs removes the paragraphs in candidates that are blank
// from the block content of c (a *CT_Body, *CT_HdrFtr, *CT_FtnEdn or
// *CT_TxbxContent), descending into tables and block-level content controls.
// A paragraph is blank when its only children are runs holding nothing but
// whitespace text (and the layout hint w:lastRenderedPageBreak) and proofing
// markers. A candidate is kept when it is the last paragraph of its container
// (a cell, a note, a header, a text box and the body all need one), when it
// carries section properties or a tracked change of its paragraph mark or
// properties, when it is all that separates two tables (Word would join them)
// or other non-paragraph blocks, or when it holds anything else — a bookmark,
// a comment range, a field, a picture. It reports whether anything was
// removed.
func SuppressBlankParagraphs(c any, candidates map[*CT_P]bool) bool {
	if len(candidates) == 0 {
		return false
	}
	switch v := c.(type) {
	case *CT_Body:
		if v == nil {
			return false
		}
		return suppressIn(blockList{&v.P, &v.childOrder, func() {
			backfillBodyChildOrder(&v.childOrder, v.P, v.Tbl, v.SdtBlock, v.BookmarkStart, v.BookmarkEnd, v.Raw)
		}}, v.Tbl, v.SdtBlock, candidates)
	case *CT_HdrFtr:
		if v == nil {
			return false
		}
		return suppressIn(blockList{&v.P, &v.childOrder, func() {
			backfillBodyChildOrder(&v.childOrder, v.P, v.Tbl, v.SdtBlock, v.BookmarkStart, v.BookmarkEnd, v.Raw)
		}}, v.Tbl, v.SdtBlock, candidates)
	case *CT_FtnEdn:
		if v == nil {
			return false
		}
		return suppressIn(blockList{&v.P, &v.childOrder, func() {
			backfillBodyChildOrder(&v.childOrder, v.P, v.Tbl, v.SdtBlock, v.BookmarkStart, v.BookmarkEnd, v.Raw)
		}}, v.Tbl, v.SdtBlock, candidates)
	case *CT_TxbxContent:
		if v == nil {
			return false
		}
		return suppressIn(blockList{&v.P, &v.childOrder, func() {
			backfillBodyChildOrder(&v.childOrder, v.P, v.Tbl, v.SdtBlock, v.BookmarkStart, v.BookmarkEnd, v.Raw)
		}}, v.Tbl, v.SdtBlock, candidates)
	}
	return false
}

// blockList is a container's paragraph slice and block child order, with the
// backfill that records the order for content built without one.
type blockList struct {
	ps       *[]*CT_P
	order    *[]bodyChildRef
	backfill func()
}

// suppressIn removes the blank candidates among the list's own paragraphs,
// then descends into its tables and content controls.
func suppressIn(l blockList, tbls []*CT_Tbl, sdts []*CT_SdtBlock, candidates map[*CT_P]bool) bool {
	changed := false
	for _, tbl := range tbls {
		if suppressInTable(tbl, candidates) {
			changed = true
		}
	}
	for _, s := range sdts {
		if suppressInSdt(s, candidates) {
			changed = true
		}
	}
	if suppressParagraphs(l, candidates) {
		changed = true
	}
	return changed
}

func suppressInTable(tbl *CT_Tbl, candidates map[*CT_P]bool) bool {
	if tbl == nil {
		return false
	}
	changed := false
	for _, tr := range tbl.Tr {
		if suppressInRow(tr, candidates) {
			changed = true
		}
	}
	for _, s := range tbl.SdtBlock {
		if suppressInSdt(s, candidates) {
			changed = true
		}
	}
	return changed
}

func suppressInRow(tr *CT_Tr, candidates map[*CT_P]bool) bool {
	if tr == nil {
		return false
	}
	changed := false
	for _, tc := range tr.Tc {
		if suppressInCell(tc, candidates) {
			changed = true
		}
	}
	for _, s := range tr.SdtCell {
		if suppressInSdt(s, candidates) {
			changed = true
		}
	}
	return changed
}

func suppressInCell(tc *CT_Tc, candidates map[*CT_P]bool) bool {
	if tc == nil {
		return false
	}
	return suppressIn(blockList{&tc.P, &tc.childOrder, func() {
		backfillBodyChildOrder(&tc.childOrder, tc.P, tc.Tbl, tc.SdtBlock, tc.BookmarkStart, tc.BookmarkEnd, tc.Raw)
	}}, tc.Tbl, tc.SdtBlock, candidates)
}

func suppressInSdt(s *CT_SdtBlock, candidates map[*CT_P]bool) bool {
	if s == nil || s.SdtContent == nil {
		return false
	}
	sc := s.SdtContent
	changed := false
	for _, tc := range sc.Tc {
		if suppressInCell(tc, candidates) {
			changed = true
		}
	}
	for _, tr := range sc.Tr {
		if suppressInRow(tr, candidates) {
			changed = true
		}
	}
	// A content control holding cells or rows has no paragraphs of its own
	// to lose; one holding block content keeps its last paragraph like any
	// container.
	if len(sc.Tc) == 0 && len(sc.Tr) == 0 {
		if suppressIn(blockList{&sc.P, &sc.childOrder, func() {
			backfillBodyChildOrder(&sc.childOrder, sc.P, sc.Tbl, sc.SdtBlock, sc.BookmarkStart, sc.BookmarkEnd, sc.Raw)
		}}, sc.Tbl, sc.SdtBlock, candidates) {
			changed = true
		}
	} else {
		for _, tbl := range sc.Tbl {
			if suppressInTable(tbl, candidates) {
				changed = true
			}
		}
	}
	return changed
}

// suppressParagraphs removes the list's blank candidate paragraphs that are
// followed by another paragraph of the same list.
func suppressParagraphs(l blockList, candidates map[*CT_P]bool) bool {
	found := false
	for _, p := range *l.ps {
		if candidates[p] {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	l.backfill()
	order := *l.order
	drop := map[int]bool{} // P index -> removed
	for pos, ref := range order {
		if ref.kind != bodyChildP || ref.index >= len(*l.ps) {
			continue
		}
		p := (*l.ps)[ref.index]
		if !candidates[p] || !paragraphBlank(p) || !laterParagraph(order, pos, drop) ||
			separatesBlocks(order, pos, drop) {
			continue
		}
		drop[ref.index] = true
	}
	if len(drop) == 0 {
		return false
	}
	newPs := make([]*CT_P, 0, len(*l.ps)-len(drop))
	newIndex := make([]int, len(*l.ps))
	for i, p := range *l.ps {
		if drop[i] {
			newIndex[i] = -1
			continue
		}
		newIndex[i] = len(newPs)
		newPs = append(newPs, p)
	}
	newOrder := make([]bodyChildRef, 0, len(order))
	for _, ref := range order {
		if ref.kind == bodyChildP && ref.index < len(newIndex) {
			if newIndex[ref.index] < 0 {
				continue
			}
			ref.index = newIndex[ref.index]
		}
		newOrder = append(newOrder, ref)
	}
	*l.ps = newPs
	*l.order = newOrder
	return true
}

// laterParagraph reports whether a paragraph that is not being removed follows
// position pos in order.
func laterParagraph(order []bodyChildRef, pos int, drop map[int]bool) bool {
	for _, ref := range order[pos+1:] {
		if ref.kind == bodyChildP && !drop[ref.index] {
			return true
		}
	}
	return false
}

// separatesBlocks reports whether the paragraph at pos is all that stands
// between two blocks that are not paragraphs — two tables, which Word would
// join into one once nothing separates them, or a table and a content
// control or preserved block. Bookmark markers do not separate blocks and are
// skipped; paragraphs already removed are too.
func separatesBlocks(order []bodyChildRef, pos int, drop map[int]bool) bool {
	neighbour := func(step int) (bodyChildKind, bool) {
		for i := pos + step; i >= 0 && i < len(order); i += step {
			ref := order[i]
			switch {
			case ref.kind == bodyChildBookmarkStart || ref.kind == bodyChildBookmarkEnd:
			case ref.kind == bodyChildP && drop[ref.index]:
			default:
				return ref.kind, true
			}
		}
		return 0, false
	}
	before, okBefore := neighbour(-1)
	after, okAfter := neighbour(1)
	return okBefore && okAfter && before != bodyChildP && after != bodyChildP
}

// paragraphBlank reports whether p holds nothing a reader would see or a
// removal would lose.
func paragraphBlank(p *CT_P) bool {
	if p == nil {
		return false
	}
	if pp := p.PPr; pp != nil {
		if pp.SectPr != nil || pp.PPrChange != nil || markTracked(pp.RPr) {
			return false
		}
	}
	refs := p.contentRefs()
	for _, ref := range refs.orderedChildren() {
		switch ref.kind {
		case pChildR:
			r, _ := refs.valueAt(ref).(*CT_R)
			if r == nil {
				continue
			}
			if r.RPr != nil && r.RPr.RPrChange != nil {
				return false
			}
			if !runHoldsOnly(r, runChildT, runChildLastRenderedPageBreak) {
				return false
			}
			for _, t := range r.T {
				if t != nil && strings.TrimSpace(t.Text) != "" {
					return false
				}
			}
		case pChildProofErr:
		default:
			return false
		}
	}
	return true
}

// markTracked reports whether paragraph-mark run properties carry a tracked
// change (w:ins, w:del, w:moveFrom, w:moveTo or w:rPrChange). The markers are
// captured rather than modeled, so the properties are serialized and their
// element names read back.
func markTracked(rPr *CT_RPr) bool {
	if rPr == nil {
		return false
	}
	if rPr.RPrChange != nil {
		return true
	}
	b := xmlb.NewWordprocessingMLBuilder()
	b.MarshalElement(NsWml, "rPr", rPr)
	dec := xml.NewDecoder(bytes.NewReader(b.Bytes()))
	for {
		tok, err := dec.RawToken()
		if err != nil {
			return false
		}
		if se, ok := tok.(xml.StartElement); ok {
			switch se.Name.Local {
			case "ins", "del", "moveFrom", "moveTo", "rPrChange":
				return true
			}
		}
	}
}
