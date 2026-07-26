package oxml

import (
	"strconv"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
)

// RevContainer is a paragraph-content holder whose tracked-change children the
// revision transforms can read and rebuild: a paragraph (CT_P), a hyperlink
// (CT_Hyperlink), a tracked-change block (CT_RunTrackChange), a simple field
// (CT_SimpleField), or run-level structured-document-tag content
// (CT_SdtContentRun). The interface method is unexported so only these library
// types satisfy it; callers hold values and pass them to the exported transform
// functions below.
type RevContainer interface {
	contentRefs() pContentRefs
}

// pContentRefs bundles pointers to a container's paragraph-content slices and
// its child order so the revision transforms can read every child in document
// order and rebuild the container from a transformed item list. Fields the
// container does not have (a hyperlink has no comment ranges, math, or
// AlternateContent) are nil; those child kinds never appear in such a
// container's order, so the transforms never touch the nil pointers.
type pContentRefs struct {
	r                 *[]*CT_R
	hyperlink         *[]*CT_Hyperlink
	bookmarkStart     *[]*CT_BookmarkStart
	bookmarkEnd       *[]*CT_BookmarkEnd
	proofErr          *[]*CT_ProofErr
	permStart         *[]*CT_PermStart
	permEnd           *[]*CT_PermEnd
	ins               *[]*CT_RunTrackChange
	del               *[]*CT_RunTrackChange
	fldSimple         *[]*CT_SimpleField
	sdtRun            *[]*CT_SdtRun
	commentRangeStart *[]*CT_CommentRangeStart
	commentRangeEnd   *[]*CT_CommentRangeEnd
	oMath             *[][]byte
	oMathPara         *[][]byte
	alternateContent  *[]*coxml.AlternateContent
	raw               *[]*CT_RawNamedElement
	childOrder        *[]pChildRef
}

func (p *CT_P) contentRefs() pContentRefs {
	return pContentRefs{
		r: &p.R, hyperlink: &p.Hyperlink, bookmarkStart: &p.BookmarkStart,
		bookmarkEnd: &p.BookmarkEnd, proofErr: &p.ProofErr, permStart: &p.PermStart,
		permEnd: &p.PermEnd, ins: &p.Ins, del: &p.Del, fldSimple: &p.FldSimple,
		sdtRun: &p.SdtRun, commentRangeStart: &p.CommentRangeStart,
		commentRangeEnd: &p.CommentRangeEnd, oMath: &p.OMath, oMathPara: &p.OMathPara,
		alternateContent: &p.AlternateContent, raw: &p.Raw, childOrder: &p.childOrder,
	}
}

func (h *CT_Hyperlink) contentRefs() pContentRefs {
	return pContentRefs{
		r: &h.R, hyperlink: &h.Hyperlink, bookmarkStart: &h.BookmarkStart,
		bookmarkEnd: &h.BookmarkEnd, proofErr: &h.ProofErr, permStart: &h.PermStart,
		permEnd: &h.PermEnd, ins: &h.Ins, del: &h.Del, fldSimple: &h.FldSimple,
		sdtRun: &h.SdtRun, raw: &h.Raw, childOrder: &h.childOrder,
	}
}

func (tc *CT_RunTrackChange) contentRefs() pContentRefs {
	return pContentRefs{
		r: &tc.R, hyperlink: &tc.Hyperlink, bookmarkStart: &tc.BookmarkStart,
		bookmarkEnd: &tc.BookmarkEnd, proofErr: &tc.ProofErr, permStart: &tc.PermStart,
		permEnd: &tc.PermEnd, ins: &tc.Ins, del: &tc.Del, fldSimple: &tc.FldSimple,
		sdtRun: &tc.SdtRun, raw: &tc.Raw, childOrder: &tc.childOrder,
	}
}

func (sc *CT_SdtContentRun) contentRefs() pContentRefs {
	return pContentRefs{
		r: &sc.R, hyperlink: &sc.Hyperlink, bookmarkStart: &sc.BookmarkStart,
		bookmarkEnd: &sc.BookmarkEnd, proofErr: &sc.ProofErr, permStart: &sc.PermStart,
		permEnd: &sc.PermEnd, ins: &sc.Ins, del: &sc.Del, fldSimple: &sc.FldSimple,
		sdtRun: &sc.SdtRun, raw: &sc.Raw, childOrder: &sc.childOrder,
	}
}

func (f *CT_SimpleField) contentRefs() pContentRefs {
	return pContentRefs{
		r: &f.R, hyperlink: &f.Hyperlink, bookmarkStart: &f.BookmarkStart,
		bookmarkEnd: &f.BookmarkEnd, proofErr: &f.ProofErr, fldSimple: &f.FldSimple,
		raw: &f.Raw, childOrder: &f.childOrder,
	}
}

// pItem is a single paragraph-content child: its kind and the concrete element
// value (one of *CT_R, *CT_Hyperlink, ..., or []byte for math). It lets the
// transforms splice content between containers uniformly.
type pItem struct {
	kind pChildKind
	val  any
}

// elemAt returns the i-th element of *s as an any, or nil when out of range or
// the slice pointer is nil (a child kind the container does not hold).
func elemAt[T any](s *[]T, i int) any {
	if s == nil || i < 0 || i >= len(*s) {
		return nil
	}
	return (*s)[i]
}

// valueAt resolves a child reference to its concrete element value.
func (refs pContentRefs) valueAt(ref pChildRef) any {
	switch ref.kind {
	case pChildR:
		return elemAt(refs.r, ref.index)
	case pChildHyperlink:
		return elemAt(refs.hyperlink, ref.index)
	case pChildBookmarkStart:
		return elemAt(refs.bookmarkStart, ref.index)
	case pChildBookmarkEnd:
		return elemAt(refs.bookmarkEnd, ref.index)
	case pChildProofErr:
		return elemAt(refs.proofErr, ref.index)
	case pChildPermStart:
		return elemAt(refs.permStart, ref.index)
	case pChildPermEnd:
		return elemAt(refs.permEnd, ref.index)
	case pChildIns:
		return elemAt(refs.ins, ref.index)
	case pChildDel:
		return elemAt(refs.del, ref.index)
	case pChildFldSimple:
		return elemAt(refs.fldSimple, ref.index)
	case pChildSdtRun:
		return elemAt(refs.sdtRun, ref.index)
	case pChildCommentRangeStart:
		return elemAt(refs.commentRangeStart, ref.index)
	case pChildCommentRangeEnd:
		return elemAt(refs.commentRangeEnd, ref.index)
	case pChildOMath:
		return elemAt(refs.oMath, ref.index)
	case pChildOMathPara:
		return elemAt(refs.oMathPara, ref.index)
	case pChildAlternateContent:
		return elemAt(refs.alternateContent, ref.index)
	case pChildRaw:
		return elemAt(refs.raw, ref.index)
	}
	return nil
}

// backfillChildOrder records a container's existing typed children in its child
// order, grouped by kind in slice order, when the order is empty but content
// slices are populated. A container built through the docx package can carry
// typed children with no recorded order — e.g. field.go's AddMergeField appends
// to CT_SimpleField.R and hyperlink.go sets CT_Hyperlink.R directly, neither of
// which records childOrder. Without this the revision transforms read zero items
// via itemsOf and setItemsOf rebuilds the container empty, wiping the field or
// hyperlink content. Mirrors CT_P.backfillChildOrder over the generic ref set;
// nil slice pointers (a kind the container does not hold) are skipped.
func (refs pContentRefs) backfillChildOrder() {
	if refs.childOrder == nil || len(*refs.childOrder) > 0 {
		return
	}
	add := func(kind pChildKind, n int) {
		for i := 0; i < n; i++ {
			*refs.childOrder = append(*refs.childOrder, pChildRef{kind, i})
		}
	}
	if refs.r != nil {
		add(pChildR, len(*refs.r))
	}
	if refs.hyperlink != nil {
		add(pChildHyperlink, len(*refs.hyperlink))
	}
	if refs.bookmarkStart != nil {
		add(pChildBookmarkStart, len(*refs.bookmarkStart))
	}
	if refs.bookmarkEnd != nil {
		add(pChildBookmarkEnd, len(*refs.bookmarkEnd))
	}
	if refs.proofErr != nil {
		add(pChildProofErr, len(*refs.proofErr))
	}
	if refs.permStart != nil {
		add(pChildPermStart, len(*refs.permStart))
	}
	if refs.permEnd != nil {
		add(pChildPermEnd, len(*refs.permEnd))
	}
	if refs.ins != nil {
		add(pChildIns, len(*refs.ins))
	}
	if refs.del != nil {
		add(pChildDel, len(*refs.del))
	}
	if refs.fldSimple != nil {
		add(pChildFldSimple, len(*refs.fldSimple))
	}
	if refs.sdtRun != nil {
		add(pChildSdtRun, len(*refs.sdtRun))
	}
	if refs.commentRangeStart != nil {
		add(pChildCommentRangeStart, len(*refs.commentRangeStart))
	}
	if refs.commentRangeEnd != nil {
		add(pChildCommentRangeEnd, len(*refs.commentRangeEnd))
	}
	if refs.oMath != nil {
		add(pChildOMath, len(*refs.oMath))
	}
	if refs.oMathPara != nil {
		add(pChildOMathPara, len(*refs.oMathPara))
	}
	if refs.alternateContent != nil {
		add(pChildAlternateContent, len(*refs.alternateContent))
	}
	if refs.raw != nil {
		add(pChildRaw, len(*refs.raw))
	}
}

// itemsOf returns the container's children in document order.
func itemsOf(c RevContainer) []pItem {
	refs := c.contentRefs()
	refs.backfillChildOrder()
	order := *refs.childOrder
	out := make([]pItem, 0, len(order))
	for _, ref := range order {
		if v := refs.valueAt(ref); v != nil {
			out = append(out, pItem{ref.kind, v})
		}
	}
	return out
}

// appendItem appends one item to the container's slices, recording its position
// in the child order.
func (refs pContentRefs) appendItem(it pItem) {
	switch it.kind {
	case pChildR:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildR, len(*refs.r)})
		*refs.r = append(*refs.r, it.val.(*CT_R))
	case pChildHyperlink:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildHyperlink, len(*refs.hyperlink)})
		*refs.hyperlink = append(*refs.hyperlink, it.val.(*CT_Hyperlink))
	case pChildBookmarkStart:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildBookmarkStart, len(*refs.bookmarkStart)})
		*refs.bookmarkStart = append(*refs.bookmarkStart, it.val.(*CT_BookmarkStart))
	case pChildBookmarkEnd:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildBookmarkEnd, len(*refs.bookmarkEnd)})
		*refs.bookmarkEnd = append(*refs.bookmarkEnd, it.val.(*CT_BookmarkEnd))
	case pChildProofErr:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildProofErr, len(*refs.proofErr)})
		*refs.proofErr = append(*refs.proofErr, it.val.(*CT_ProofErr))
	case pChildPermStart:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildPermStart, len(*refs.permStart)})
		*refs.permStart = append(*refs.permStart, it.val.(*CT_PermStart))
	case pChildPermEnd:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildPermEnd, len(*refs.permEnd)})
		*refs.permEnd = append(*refs.permEnd, it.val.(*CT_PermEnd))
	case pChildIns:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildIns, len(*refs.ins)})
		*refs.ins = append(*refs.ins, it.val.(*CT_RunTrackChange))
	case pChildDel:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildDel, len(*refs.del)})
		*refs.del = append(*refs.del, it.val.(*CT_RunTrackChange))
	case pChildFldSimple:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildFldSimple, len(*refs.fldSimple)})
		*refs.fldSimple = append(*refs.fldSimple, it.val.(*CT_SimpleField))
	case pChildSdtRun:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildSdtRun, len(*refs.sdtRun)})
		*refs.sdtRun = append(*refs.sdtRun, it.val.(*CT_SdtRun))
	case pChildCommentRangeStart:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildCommentRangeStart, len(*refs.commentRangeStart)})
		*refs.commentRangeStart = append(*refs.commentRangeStart, it.val.(*CT_CommentRangeStart))
	case pChildCommentRangeEnd:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildCommentRangeEnd, len(*refs.commentRangeEnd)})
		*refs.commentRangeEnd = append(*refs.commentRangeEnd, it.val.(*CT_CommentRangeEnd))
	case pChildOMath:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildOMath, len(*refs.oMath)})
		*refs.oMath = append(*refs.oMath, it.val.([]byte))
	case pChildOMathPara:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildOMathPara, len(*refs.oMathPara)})
		*refs.oMathPara = append(*refs.oMathPara, it.val.([]byte))
	case pChildAlternateContent:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildAlternateContent, len(*refs.alternateContent)})
		*refs.alternateContent = append(*refs.alternateContent, it.val.(*coxml.AlternateContent))
	case pChildRaw:
		*refs.childOrder = append(*refs.childOrder, pChildRef{pChildRaw, len(*refs.raw)})
		*refs.raw = append(*refs.raw, it.val.(*CT_RawNamedElement))
	}
}

// setItemsOf rebuilds the container's content slices and child order from items.
func setItemsOf(c RevContainer, items []pItem) {
	refs := c.contentRefs()
	resetSlice(refs.r)
	resetSlice(refs.hyperlink)
	resetSlice(refs.bookmarkStart)
	resetSlice(refs.bookmarkEnd)
	resetSlice(refs.proofErr)
	resetSlice(refs.permStart)
	resetSlice(refs.permEnd)
	resetSlice(refs.ins)
	resetSlice(refs.del)
	resetSlice(refs.fldSimple)
	resetSlice(refs.sdtRun)
	resetSlice(refs.commentRangeStart)
	resetSlice(refs.commentRangeEnd)
	resetSlice(refs.oMath)
	resetSlice(refs.oMathPara)
	resetSlice(refs.alternateContent)
	resetSlice(refs.raw)
	*refs.childOrder = nil
	for _, it := range items {
		refs.appendItem(it)
	}
}

func resetSlice[T any](s *[]T) {
	if s != nil {
		*s = nil
	}
}

// unwrapBlock replaces the ins/del block (a direct child of c) with the block's
// own content spliced in at the same position. When convertDel is set (rejecting
// a deletion) the restored runs' w:delText is converted back to normal w:t.
// Returns false when the block is not a direct child of c.
func unwrapBlock(c RevContainer, block *CT_RunTrackChange, convertDel bool) bool {
	items := itemsOf(c)
	for i, it := range items {
		if (it.kind == pChildIns || it.kind == pChildDel) && it.val == any(block) {
			inner := itemsOf(block)
			if convertDel {
				convertItemsDelText(inner)
			}
			out := make([]pItem, 0, len(items)-1+len(inner))
			out = append(out, items[:i]...)
			out = append(out, inner...)
			out = append(out, items[i+1:]...)
			setItemsOf(c, out)
			return true
		}
	}
	return false
}

// removeBlock removes the ins/del block (a direct child of c) and all its
// content. Returns false when the block is not a direct child of c.
func removeBlock(c RevContainer, block *CT_RunTrackChange) bool {
	items := itemsOf(c)
	for i, it := range items {
		if (it.kind == pChildIns || it.kind == pChildDel) && it.val == any(block) {
			out := make([]pItem, 0, len(items)-1)
			out = append(out, items[:i]...)
			out = append(out, items[i+1:]...)
			setItemsOf(c, out)
			return true
		}
	}
	return false
}

// convertItemsDelText converts every w:delText back to w:t in the runs reachable
// from items, descending into nested containers. Runs are pointers, so the
// in-place mutation is visible without rebuilding the parent order.
func convertItemsDelText(items []pItem) {
	for _, it := range items {
		switch it.kind {
		case pChildR:
			it.val.(*CT_R).convertDelTextToText()
		case pChildIns, pChildDel:
			convertItemsDelText(itemsOf(it.val.(*CT_RunTrackChange)))
		case pChildHyperlink:
			convertItemsDelText(itemsOf(it.val.(*CT_Hyperlink)))
		case pChildFldSimple:
			convertItemsDelText(itemsOf(it.val.(*CT_SimpleField)))
		case pChildSdtRun:
			if s := it.val.(*CT_SdtRun); s.SdtContent != nil {
				convertItemsDelText(itemsOf(s.SdtContent))
			}
		}
	}
}

// convertDelTextToText relabels the run's w:delText children as w:t, so a
// rejected deletion restores its text as normal run content. Both elements are
// CT_Text, so only the child-order kind and the owning slice change.
func (r *CT_R) convertDelTextToText() {
	if len(r.DelText) == 0 {
		return
	}
	r.backfillChildOrder()
	newT := make([]*CT_Text, 0, len(r.T)+len(r.DelText))
	newOrder := make([]runChildRef, 0, len(r.childOrder))
	for _, ref := range r.childOrder {
		switch ref.kind {
		case runChildT:
			newOrder = append(newOrder, runChildRef{runChildT, len(newT)})
			newT = append(newT, r.T[ref.index])
		case runChildDelText:
			newOrder = append(newOrder, runChildRef{runChildT, len(newT)})
			newT = append(newT, r.DelText[ref.index])
		default:
			newOrder = append(newOrder, ref)
		}
	}
	r.T = newT
	r.DelText = nil
	r.childOrder = newOrder
}

// convertTextToDelText relabels the run's w:t children as w:delText, so a run
// authored into a deletion carries deletion text (the inverse of
// convertDelTextToText). Both elements are CT_Text, so only the child-order kind
// and the owning slice change; each text keeps its position among the run's
// other children.
func (r *CT_R) convertTextToDelText() {
	if len(r.T) == 0 {
		return
	}
	r.backfillChildOrder()
	newDel := make([]*CT_Text, 0, len(r.DelText)+len(r.T))
	newOrder := make([]runChildRef, 0, len(r.childOrder))
	for _, ref := range r.childOrder {
		switch ref.kind {
		case runChildDelText:
			newOrder = append(newOrder, runChildRef{runChildDelText, len(newDel)})
			newDel = append(newDel, r.DelText[ref.index])
		case runChildT:
			newOrder = append(newOrder, runChildRef{runChildDelText, len(newDel)})
			newDel = append(newDel, r.T[ref.index])
		default:
			newOrder = append(newOrder, ref)
		}
	}
	r.DelText = newDel
	r.T = nil
	r.childOrder = newOrder
}

// --- Authoring entry points (called from the docx package) ---

// WrapRunInsertion replaces run r (a direct child of c) with an insertion block
// (w:ins, carrying id/author/date) that wraps r at the same position, so it
// reads back as a tracked insertion. Returns false when r is not a direct child
// of c.
func WrapRunInsertion(c RevContainer, r *CT_R, id, author, date string) bool {
	return wrapRunInBlock(c, r, id, author, date, pChildIns)
}

// WrapRunDeletion replaces run r (a direct child of c) with a deletion block
// (w:del, carrying id/author/date) that wraps r at the same position, first
// converting the run's w:t children to w:delText so the content reads back as a
// tracked deletion. Returns false when r is not a direct child of c.
func WrapRunDeletion(c RevContainer, r *CT_R, id, author, date string) bool {
	r.convertTextToDelText()
	return wrapRunInBlock(c, r, id, author, date, pChildDel)
}

// wrapRunInBlock finds run r among c's children and replaces it, in place, with
// a new tracked-change block of the given kind wrapping r.
func wrapRunInBlock(c RevContainer, r *CT_R, id, author, date string, kind pChildKind) bool {
	items := itemsOf(c)
	for i, it := range items {
		if it.kind == pChildR && it.val == any(r) {
			block := &CT_RunTrackChange{Id: id, Author: author, Date: date}
			block.AppendR(r)
			items[i] = pItem{kind, block}
			setItemsOf(c, items)
			return true
		}
	}
	return false
}

// MaxRevisionID returns the highest numeric w:id among every tracked-change
// element reachable from the body — insertions, deletions, run/paragraph
// property changes (descending into tables, hyperlinks, fields, and SDTs), plus
// table/row/cell structural revisions and the section-property change — or 0
// when none is present or numeric. Authoring assigns the next id above this so a
// newly created revision never collides with one already in the document.
func MaxRevisionID(body *CT_Body) int {
	if body == nil {
		return 0
	}
	maxID := 0
	consider := func(s string) {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > maxID {
			maxID = n
		}
	}
	for _, p := range body.AllParagraphs() {
		for _, rr := range CollectParagraphRevisions(p) {
			consider(rr.Id)
		}
		MaxMoveID(p, consider)
	}
	for _, tbl := range body.Tbl {
		maxTableRevisionID(tbl, consider)
	}
	// Tables wrapped in a block-level SDT carry their own structural revisions;
	// walking only body.Tbl would let a newly authored revision collide with an
	// id already used inside an SDT-wrapped table.
	for _, s := range body.SdtBlock {
		maxSdtBlockTableRevisionID(s, consider)
	}
	if body.SectPr != nil && body.SectPr.SectPrChange != nil {
		consider(body.SectPr.SectPrChange.Id)
	}
	return maxID
}

// maxSdtBlockTableRevisionID feeds the structural revision ids of every table
// wrapped by a block-level SDT (directly, or in a nested block SDT) to consider.
func maxSdtBlockTableRevisionID(s *CT_SdtBlock, consider func(string)) {
	if s == nil || s.SdtContent == nil {
		return
	}
	sc := s.SdtContent
	for _, tbl := range sc.Tbl {
		maxTableRevisionID(tbl, consider)
	}
	for _, nested := range sc.SdtBlock {
		maxSdtBlockTableRevisionID(nested, consider)
	}
}

// maxTableRevisionID feeds every structural revision id of a table (table/row/
// cell property changes, row/cell insertions and deletions, and cell merges) to
// consider, descending into nested tables.
func maxTableRevisionID(tbl *CT_Tbl, consider func(string)) {
	if tbl == nil {
		return
	}
	if tbl.TblPr != nil && tbl.TblPr.TblPrChange != nil {
		consider(tbl.TblPr.TblPrChange.Id)
	}
	for _, tr := range tbl.Tr {
		if tr == nil {
			continue
		}
		if tr.TrPr != nil {
			if tr.TrPr.Ins != nil {
				consider(tr.TrPr.Ins.Id)
			}
			if tr.TrPr.Del != nil {
				consider(tr.TrPr.Del.Id)
			}
			if tr.TrPr.TrPrChange != nil {
				consider(tr.TrPr.TrPrChange.Id)
			}
		}
		for _, tc := range tr.Tc {
			if tc == nil {
				continue
			}
			if tc.TcPr != nil {
				if tc.TcPr.CellIns != nil {
					consider(tc.TcPr.CellIns.Id)
				}
				if tc.TcPr.CellDel != nil {
					consider(tc.TcPr.CellDel.Id)
				}
				if tc.TcPr.CellMerge != nil {
					consider(tc.TcPr.CellMerge.Id)
				}
				if tc.TcPr.TcPrChange != nil {
					consider(tc.TcPr.TcPrChange.Id)
				}
			}
			for _, nested := range tc.Tbl {
				maxTableRevisionID(nested, consider)
			}
		}
	}
}

// --- Exported transform entry points (called from the docx package) ---

// AcceptInsertion unwraps an insertion (w:ins), keeping its content as normal
// runs at the same position. block must be a direct child of c.
func AcceptInsertion(c RevContainer, block *CT_RunTrackChange) bool {
	return unwrapBlock(c, block, false)
}

// RejectInsertion removes an insertion (w:ins) and its content. block must be a
// direct child of c.
func RejectInsertion(c RevContainer, block *CT_RunTrackChange) bool {
	return removeBlock(c, block)
}

// AcceptDeletion removes a deletion (w:del) and its content. block must be a
// direct child of c.
func AcceptDeletion(c RevContainer, block *CT_RunTrackChange) bool {
	return removeBlock(c, block)
}

// RejectDeletion restores a deletion's content as normal runs (w:delText becomes
// w:t) at the same position. block must be a direct child of c.
func RejectDeletion(c RevContainer, block *CT_RunTrackChange) bool {
	return unwrapBlock(c, block, true)
}

// AcceptRunFormat keeps the run's current properties and drops the w:rPrChange
// record.
func AcceptRunFormat(r *CT_R) {
	if r != nil && r.RPr != nil {
		r.RPr.RPrChange = nil
	}
}

// RejectRunFormat reverts the run to the properties recorded in w:rPrChange,
// dropping the change record.
func RejectRunFormat(r *CT_R) {
	if r == nil || r.RPr == nil || r.RPr.RPrChange == nil {
		return
	}
	old := r.RPr.RPrChange.RPr
	if old == nil {
		old = &CT_RPr{}
	}
	// The recorded old rPr carries no rPrChange child of its own, so adopting it
	// wholesale both reverts the formatting and clears the record.
	r.RPr = old
}

// AcceptParagraphFormat keeps the paragraph's current properties and drops the
// w:pPrChange record (and any paragraph-mark w:rPrChange).
func AcceptParagraphFormat(p *CT_P) {
	if p == nil || p.PPr == nil {
		return
	}
	p.PPr.PPrChange = nil
	if p.PPr.RPr != nil {
		p.PPr.RPr.RPrChange = nil
	}
}

// RejectParagraphFormat reverts the paragraph's base properties to those
// recorded in w:pPrChange, dropping the change record. Section properties and
// the paragraph-mark run properties are preserved: the recorded old pPr is a
// CT_PPrBase and carries neither.
func RejectParagraphFormat(p *CT_P) {
	if p == nil || p.PPr == nil || p.PPr.PPrChange == nil {
		return
	}
	old := p.PPr.PPrChange.PPr
	if old == nil {
		old = &CT_PPr{}
	}
	old.SectPr = p.PPr.SectPr
	old.RPr = p.PPr.RPr
	old.PPrChange = nil
	p.PPr = old
}

// AcceptAllInContainer accepts every insertion, deletion, and run-format change
// reachable from c, in place: insertions are unwrapped, deletions removed, and
// w:rPrChange records dropped. Nested containers are processed recursively.
func AcceptAllInContainer(c RevContainer) {
	items := itemsOf(c)
	out := make([]pItem, 0, len(items))
	for _, it := range items {
		switch it.kind {
		case pChildIns:
			blk := it.val.(*CT_RunTrackChange)
			AcceptAllInContainer(blk)
			out = append(out, itemsOf(blk)...)
		case pChildDel:
			// Accepting a deletion removes its content.
		case pChildR:
			AcceptRunFormat(it.val.(*CT_R))
			out = append(out, it)
		case pChildHyperlink:
			AcceptAllInContainer(it.val.(*CT_Hyperlink))
			out = append(out, it)
		case pChildFldSimple:
			AcceptAllInContainer(it.val.(*CT_SimpleField))
			out = append(out, it)
		case pChildSdtRun:
			if s := it.val.(*CT_SdtRun); s.SdtContent != nil {
				AcceptAllInContainer(s.SdtContent)
			}
			out = append(out, it)
		case pChildRaw:
			if next, handled := acceptMoveRaw(it.val.(*CT_RawNamedElement), out); handled {
				out = next
			} else {
				out = append(out, it)
			}
		default:
			out = append(out, it)
		}
	}
	setItemsOf(c, out)
}

// RejectAllInContainer rejects every insertion, deletion, and run-format change
// reachable from c, in place: insertions are removed, deletions restored
// (w:delText becomes w:t), and w:rPrChange records reverted. Nested containers
// are processed recursively.
func RejectAllInContainer(c RevContainer) {
	items := itemsOf(c)
	out := make([]pItem, 0, len(items))
	for _, it := range items {
		switch it.kind {
		case pChildIns:
			// Rejecting an insertion removes its content.
		case pChildDel:
			blk := it.val.(*CT_RunTrackChange)
			RejectAllInContainer(blk)
			inner := itemsOf(blk)
			convertItemsDelText(inner)
			out = append(out, inner...)
		case pChildR:
			RejectRunFormat(it.val.(*CT_R))
			out = append(out, it)
		case pChildHyperlink:
			RejectAllInContainer(it.val.(*CT_Hyperlink))
			out = append(out, it)
		case pChildFldSimple:
			RejectAllInContainer(it.val.(*CT_SimpleField))
			out = append(out, it)
		case pChildSdtRun:
			if s := it.val.(*CT_SdtRun); s.SdtContent != nil {
				RejectAllInContainer(s.SdtContent)
			}
			out = append(out, it)
		case pChildRaw:
			if next, handled := rejectMoveRaw(it.val.(*CT_RawNamedElement), out); handled {
				out = next
			} else {
				out = append(out, it)
			}
		default:
			out = append(out, it)
		}
	}
	setItemsOf(c, out)
}

// BlockText returns the concatenated visible text of a tracked-change block:
// every w:t and w:delText in the runs reachable from it, in document order.
func BlockText(block *CT_RunTrackChange) string {
	return string(appendBlockText(nil, itemsOf(block)))
}

func appendBlockText(dst []byte, items []pItem) []byte {
	for _, it := range items {
		switch it.kind {
		case pChildR:
			r := it.val.(*CT_R)
			for _, t := range r.T {
				dst = append(dst, t.Text...)
			}
			for _, t := range r.DelText {
				dst = append(dst, t.Text...)
			}
		case pChildIns, pChildDel:
			dst = appendBlockText(dst, itemsOf(it.val.(*CT_RunTrackChange)))
		case pChildHyperlink:
			dst = appendBlockText(dst, itemsOf(it.val.(*CT_Hyperlink)))
		case pChildFldSimple:
			dst = appendBlockText(dst, itemsOf(it.val.(*CT_SimpleField)))
		case pChildSdtRun:
			if s := it.val.(*CT_SdtRun); s.SdtContent != nil {
				dst = appendBlockText(dst, itemsOf(s.SdtContent))
			}
		}
	}
	return dst
}

// --- Enumeration (called from the docx package) ---

// RevKind tags the kind of a RawRevision enumerated from paragraph content.
type RevKind int

const (
	RevKindInsertion RevKind = iota
	RevKindDeletion
	RevKindRunFormat
	RevKindParagraphFormat
)

// RawRevision is one enumerated tracked change in paragraph content, carrying
// both its display metadata and the targets the transforms need. Exactly the
// target relevant to Kind is set: Container+Block for insertions/deletions, Run
// for run-format changes, Para for paragraph-format changes.
type RawRevision struct {
	Kind      RevKind
	Id        string
	Author    string
	Date      string
	Text      string
	Container RevContainer
	Block     *CT_RunTrackChange
	Run       *CT_R
	Para      *CT_P
}

// CollectParagraphRevisions returns the tracked changes reachable from a
// paragraph in document order: its paragraph-format change (w:pPrChange), its
// runs' run-format changes (w:rPrChange), and every insertion and deletion,
// descending into hyperlinks, fields, structured document tags, and the
// insertion/deletion blocks themselves.
func CollectParagraphRevisions(p *CT_P) []RawRevision {
	var dst []RawRevision
	if p.PPr != nil && p.PPr.PPrChange != nil {
		dst = append(dst, RawRevision{
			Kind: RevKindParagraphFormat, Id: p.PPr.PPrChange.Id,
			Author: p.PPr.PPrChange.Author,
			Date:   p.PPr.PPrChange.Date, Text: p.Text(), Para: p,
		})
	}
	return collectContainerRevs(dst, p)
}

func collectContainerRevs(dst []RawRevision, c RevContainer) []RawRevision {
	for _, it := range itemsOf(c) {
		switch it.kind {
		case pChildR:
			r := it.val.(*CT_R)
			if r.RPr != nil && r.RPr.RPrChange != nil {
				dst = append(dst, RawRevision{
					Kind: RevKindRunFormat, Id: r.RPr.RPrChange.Id,
					Author: r.RPr.RPrChange.Author,
					Date:   r.RPr.RPrChange.Date, Text: RunText(r), Run: r,
				})
			}
		case pChildHyperlink:
			dst = collectContainerRevs(dst, it.val.(*CT_Hyperlink))
		case pChildFldSimple:
			dst = collectContainerRevs(dst, it.val.(*CT_SimpleField))
		case pChildSdtRun:
			if s := it.val.(*CT_SdtRun); s.SdtContent != nil {
				dst = collectContainerRevs(dst, s.SdtContent)
			}
		case pChildIns:
			blk := it.val.(*CT_RunTrackChange)
			dst = append(dst, RawRevision{
				Kind: RevKindInsertion, Id: blk.Id, Author: blk.Author, Date: blk.Date,
				Text: BlockText(blk), Container: c, Block: blk,
			})
			dst = collectContainerRevs(dst, blk)
		case pChildDel:
			blk := it.val.(*CT_RunTrackChange)
			dst = append(dst, RawRevision{
				Kind: RevKindDeletion, Id: blk.Id, Author: blk.Author, Date: blk.Date,
				Text: BlockText(blk), Container: c, Block: blk,
			})
			dst = collectContainerRevs(dst, blk)
		}
	}
	return dst
}

// RunText returns the concatenated w:t and w:delText content of a single run.
func RunText(r *CT_R) string {
	if r == nil {
		return ""
	}
	var out []byte
	for _, t := range r.T {
		out = append(out, t.Text...)
	}
	for _, t := range r.DelText {
		out = append(out, t.Text...)
	}
	return string(out)
}
