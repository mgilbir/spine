package oxml

import (
	"slices"
	"strings"
)

// This file replaces whole fields with plain text, the way Word's mail merge
// turns each MERGEFIELD into its value. A field is rewritten only when its
// markup has a shape the rewrite can account for completely; anything else is
// left as it is and reported, so no content the field carried is lost.

// FieldFiller supplies the callbacks FillFields drives.
type FieldFiller struct {
	// Fill returns the text that replaces the field with the given
	// instruction, and false to leave the field alone. inInstruction reports
	// that the field sits in the instruction of an enclosing field (a
	// MERGEFIELD inside an IF condition). There the text is written as
	// w:instrText so the enclosing field still reads it, as one argument: in
	// quotes, with quotes and backslashes escaped, or only escaped when the
	// field already sits inside a quoted argument.
	// lang is the language (w:lang w:val) of the run properties the text will
	// carry, or "" when they set none.
	Fill func(instr string, inInstruction bool, lang string) (string, bool)
	// Skipped is called with the instruction of each field FillFields cannot
	// rewrite (see FillFields), in document order. It may be nil.
	Skipped func(instr string)
	// Emptied, when set, is called with the top-level paragraph (one of paras)
	// holding each field FillFields replaced with no text outside an
	// instruction: the candidates for blank-line suppression.
	Emptied func(p *CT_P)
	// SuppressBlank makes FillTextBoxFields remove the text box paragraphs a
	// fill left blank (SuppressBlankParagraphs). FillFields itself never
	// removes paragraphs.
	SuppressBlank bool
}

// FillFields replaces the fields in paras — one part's paragraphs in document
// order — for which f.Fill returns text. A complex field (w:fldChar begin …
// end) becomes one run carrying the text, with the formatting of the field's
// first result run, or of its begin run when it has no result text. A simple
// field (w:fldSimple) becomes one run with the formatting of its first run.
// Line breaks and tabs in the text become w:br and w:tab.
//
// Bookmark, comment-range, permission-range and proofing markers inside the
// field are kept: the start markers before the text and the end markers
// after it, so a range that wrapped the field result wraps the text and a
// range that crossed the field boundary keeps its other end. The layout hints
// Word leaves in the field's runs (w:lastRenderedPageBreak) go with the runs;
// Word recomputes them.
//
// A field is not rewritten, and f.Skipped is called for it, when it is locked,
// when its begin and end runs are not in the same paragraph-content container
// (they straddle paragraphs, a hyperlink, a tracked change or a content
// control), when it contains another field or a hyperlink, tracked change,
// content control, math or unrecognized content, when a field character shares
// its run with other content, when its instruction runs carry anything but
// instruction text (and layout hints), or when its result carries anything but text, tabs and
// breaks — a picture, a footnote reference — that replacing it would discard.
// Fields inside a tracked deletion are not rewritten either. A field that is
// never closed is skipped too.
//
// It reports whether anything changed. Containers it does not rewrite are left
// untouched, child order included.
func FillFields(paras []*CT_P, f FieldFiller) bool {
	if f.Fill == nil {
		return false
	}
	st := &fieldScan{byBegin: map[*CT_R]*complexField{}, simpleCtx: map[*CT_SimpleField]fieldCtx{}}
	for _, p := range paras {
		st.paragraph(p)
	}
	for _, rec := range st.stack {
		rec.unsafe = true // never closed
	}

	fl := &fieldFill{f: f, scan: st, simpleDone: map[*CT_SimpleField]bool{}}
	changed := false
	for _, p := range paras {
		fl.para = p
		if p != nil && fl.container(p) {
			changed = true
		}
	}
	if f.Skipped != nil {
		for _, rec := range st.all {
			if !rec.handled {
				rec.handled = true
				f.Skipped(rec.instr.String())
			}
		}
		// Simple fields the rewrite never reached: those in a tracked
		// deletion.
		for _, fs := range st.simples {
			if !fl.simpleDone[fs] {
				f.Skipped(fs.Instr)
			}
		}
	}
	return changed
}

// fieldCtx is where a field sits relative to an enclosing field.
type fieldCtx struct {
	inInstr bool // in the enclosing field's instruction
	inQuote bool // and inside a quoted argument of it
}

// complexField is one w:fldChar begin … end field seen by the scan.
type complexField struct {
	begin, end *CT_R
	instr      strings.Builder
	inResult   bool     // a separate has been seen
	ctx        fieldCtx // where the field begins
	unsafe     bool     // a shape FillFields does not rewrite
	handled    bool     // rewritten, declined by Fill, or reported as skipped
}

// fieldScan walks a part's runs in document order, pairing field characters
// across paragraphs and containers so nesting context is known even for
// fields that straddle a paragraph.
type fieldScan struct {
	stack     []*complexField
	all       []*complexField // closed and unclosed fields, in begin order
	byBegin   map[*CT_R]*complexField
	simples   []*CT_SimpleField // in document order
	simpleCtx map[*CT_SimpleField]fieldCtx
	inDel     int
}

func (st *fieldScan) top() *complexField {
	if len(st.stack) == 0 {
		return nil
	}
	return st.stack[len(st.stack)-1]
}

// context reports where the scan position is relative to the innermost open
// field.
func (st *fieldScan) context() fieldCtx {
	t := st.top()
	if t == nil || t.inResult {
		return fieldCtx{}
	}
	return fieldCtx{inInstr: true, inQuote: quoteOpen(t.instr.String())}
}

// quoteOpen reports whether instruction text so far ends inside a quoted
// argument: it holds an odd number of quotes not escaped by a backslash.
func quoteOpen(instr string) bool {
	open := false
	escaped := false
	for _, r := range instr {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			open = !open
		}
	}
	return open
}

func (st *fieldScan) paragraph(p *CT_P) {
	if p == nil {
		return
	}
	st.refs(p.contentRefs())
}

// refs visits a container's children in document order. Containers other than
// plain runs inside an open field make it unsafe; a deletion's fields are
// recorded but marked unsafe so they are reported, never rewritten.
func (st *fieldScan) refs(refs pContentRefs) {
	for _, ref := range refs.orderedChildren() {
		v := refs.valueAt(ref)
		switch ref.kind {
		case pChildR:
			if r, ok := v.(*CT_R); ok && r != nil {
				st.run(r)
			}
		case pChildBookmarkStart, pChildBookmarkEnd, pChildProofErr, pChildPermStart,
			pChildPermEnd, pChildCommentRangeStart, pChildCommentRangeEnd:
			// Markers: carried through a rewrite.
		case pChildHyperlink:
			st.markOpenUnsafe()
			if h, ok := v.(*CT_Hyperlink); ok && h != nil {
				st.refs(h.contentRefs())
			}
		case pChildIns:
			st.markOpenUnsafe()
			if tc, ok := v.(*CT_RunTrackChange); ok && tc != nil {
				st.refs(tc.contentRefs())
			}
		case pChildDel:
			st.markOpenUnsafe()
			if tc, ok := v.(*CT_RunTrackChange); ok && tc != nil {
				st.inDel++
				st.refs(tc.contentRefs())
				st.inDel--
			}
		case pChildSdtRun:
			st.markOpenUnsafe()
			if s, ok := v.(*CT_SdtRun); ok && s != nil && s.SdtContent != nil {
				st.refs(s.SdtContent.contentRefs())
			}
		case pChildFldSimple:
			st.markOpenUnsafe()
			if fs, ok := v.(*CT_SimpleField); ok && fs != nil {
				st.simples = append(st.simples, fs)
				st.simpleCtx[fs] = st.context()
				st.refs(fs.contentRefs())
			}
		default:
			// Math, AlternateContent and unrecognized content: opaque.
			st.markOpenUnsafe()
		}
	}
}

// markOpenUnsafe marks every open field unsafe: something it would have to
// discard or move lies inside it.
func (st *fieldScan) markOpenUnsafe() {
	for _, rec := range st.stack {
		rec.unsafe = true
	}
}

func (st *fieldScan) run(r *CT_R) {
	if len(r.FldChar) == 0 {
		t := st.top()
		if t == nil {
			return
		}
		if !t.inResult {
			if !runHoldsOnly(r, runChildInstrText, runChildLastRenderedPageBreak) {
				t.unsafe = true
			}
			for _, it := range r.InstrText {
				if it != nil {
					t.instr.WriteString(it.Text)
				}
			}
			return
		}
		if !runHoldsOnly(r, runChildT, runChildTab, runChildBr, runChildCr,
			runChildLastRenderedPageBreak, runChildNoBreakHyphen, runChildSoftHyphen) {
			t.unsafe = true
		}
		return
	}
	// A field character must have its run to itself, but for the layout hint
	// Word writes where it last broke a page: the rewrite removes the whole
	// run, and Word recomputes the hint.
	shared := len(r.FldChar) > 1 || !runHoldsOnly(r, runChildFldChar, runChildLastRenderedPageBreak)
	if shared {
		st.markOpenUnsafe()
	}
	for _, fc := range r.FldChar {
		if fc == nil {
			continue
		}
		switch fc.FldCharType {
		case "begin":
			rec := &complexField{begin: r, ctx: st.context()}
			if shared || st.inDel > 0 || fieldLocked(fc.FldLock) {
				rec.unsafe = true
			}
			st.markOpenUnsafe() // the enclosing fields now contain a field
			st.stack = append(st.stack, rec)
			st.all = append(st.all, rec)
			st.byBegin[r] = rec
		case "separate":
			if t := st.top(); t != nil && !t.inResult {
				t.inResult = true
			}
		case "end":
			if t := st.top(); t != nil {
				t.end = r
				st.stack = st.stack[:len(st.stack)-1]
			}
		}
	}
	// Instruction text in a run that also holds a field character (already
	// marked unsafe) still names the field for the Skipped report.
	if t := st.top(); t != nil && !t.inResult {
		for _, it := range r.InstrText {
			if it != nil {
				t.instr.WriteString(it.Text)
			}
		}
	}
}

// fieldLocked reports whether a w:fldLock value is on.
func fieldLocked(v string) bool {
	switch v {
	case "true", "1", "on":
		return true
	}
	return false
}

// runHoldsOnly reports whether every content child of r is one of kinds. Run
// properties are not content.
func runHoldsOnly(r *CT_R, kinds ...runChildKind) bool {
	for _, k := range r.contentKinds() {
		if !slices.Contains(kinds, k) {
			return false
		}
	}
	return true
}

// contentKinds returns the kinds of the run's content children, from the
// recorded order or, for a run built without one, from its typed slices.
func (r *CT_R) contentKinds() []runChildKind {
	if len(r.childOrder) > 0 {
		out := make([]runChildKind, len(r.childOrder))
		for i, ref := range r.childOrder {
			out[i] = ref.kind
		}
		return out
	}
	var out []runChildKind
	add := func(k runChildKind, n int) {
		for range n {
			out = append(out, k)
		}
	}
	add(runChildT, len(r.T))
	add(runChildBr, len(r.Br))
	add(runChildTab, len(r.Tab))
	add(runChildCr, len(r.Cr))
	add(runChildSym, len(r.Sym))
	add(runChildDrawing, len(r.Drawing))
	add(runChildFtnRef, len(r.FtnRef))
	add(runChildEndnoteRef, len(r.EndnoteRef))
	add(runChildLastRenderedPageBreak, len(r.LastRenderedPageBreak))
	add(runChildNoBreakHyphen, len(r.NoBreakHyphen))
	add(runChildSoftHyphen, len(r.SoftHyphen))
	add(runChildFldChar, len(r.FldChar))
	add(runChildInstrText, len(r.InstrText))
	add(runChildDelText, len(r.DelText))
	add(runChildCommentReference, len(r.CommentReference))
	add(runChildPtab, len(r.Ptab))
	add(runChildPict, len(r.Pict))
	add(runChildObject, len(r.Object))
	add(runChildAlternateContent, len(r.AlternateContent))
	add(runChildRaw, len(r.Raw))
	return out
}

// fieldFill is the rewrite pass over the containers.
type fieldFill struct {
	f          FieldFiller
	scan       *fieldScan
	simpleDone map[*CT_SimpleField]bool // reached by the rewrite
	para       *CT_P                    // the top-level paragraph being rewritten
}

// emptied reports a field replaced with no text in the current paragraph.
func (fl *fieldFill) emptied(items []pItem, ctx fieldCtx) {
	if fl.f.Emptied == nil || ctx.inInstr || fl.para == nil {
		return
	}
	for _, it := range items {
		if it.kind == pChildR {
			return
		}
	}
	fl.f.Emptied(fl.para)
}

// container rewrites the fields whose begin and end are direct children of c,
// then recurses into nested containers (except tracked deletions). It reports
// whether c or anything inside it changed.
func (fl *fieldFill) container(c RevContainer) bool {
	refs := c.contentRefs()
	order := refs.orderedChildren()
	items := make([]pItem, 0, len(order))
	for _, ref := range order {
		if v := refs.valueAt(ref); v != nil {
			items = append(items, pItem{ref.kind, v})
		}
	}

	changed := false
	var out []pItem
	for i := 0; i < len(items); i++ {
		it := items[i]
		switch it.kind {
		case pChildR:
			r, _ := it.val.(*CT_R)
			rec := fl.scan.byBegin[r]
			if rec == nil || rec.handled {
				out = append(out, it)
				continue
			}
			j := endIndex(items, i, rec.end)
			if rec.unsafe || j < 0 {
				out = append(out, it)
				continue // reported after the walk
			}
			rPr := resultRPr(items[i : j+1])
			text, ok := fl.f.Fill(rec.instr.String(), rec.ctx.inInstr, rPrLang(rPr))
			rec.handled = true
			if !ok {
				out = append(out, it)
				continue
			}
			repl := replacement(items[i:j+1], rPr, text, rec.ctx)
			fl.emptied(repl, rec.ctx)
			out = append(out, repl...)
			changed = true
			i = j
		case pChildFldSimple:
			fs, _ := it.val.(*CT_SimpleField)
			if fs == nil {
				out = append(out, it)
				continue
			}
			if r, done := fl.simple(fs); done {
				out = append(out, r...)
				changed = true
				continue
			}
			if fl.container(fs) {
				changed = true
			}
			out = append(out, it)
		case pChildHyperlink:
			if h, ok := it.val.(*CT_Hyperlink); ok && h != nil && fl.container(h) {
				changed = true
			}
			out = append(out, it)
		case pChildIns:
			if tc, ok := it.val.(*CT_RunTrackChange); ok && tc != nil && fl.container(tc) {
				changed = true
			}
			out = append(out, it)
		case pChildSdtRun:
			if s, ok := it.val.(*CT_SdtRun); ok && s != nil && s.SdtContent != nil && fl.container(s.SdtContent) {
				changed = true
			}
			out = append(out, it)
		default:
			out = append(out, it)
		}
	}
	if changed {
		setItemsOf(c, out)
	}
	return changed
}

// simple rewrites a w:fldSimple whose content is text runs and markers. It
// returns the replacement items and true when the field was rewritten. A field
// it cannot rewrite is reported through Skipped without asking Fill; one Fill
// declines is not reported.
func (fl *fieldFill) simple(fs *CT_SimpleField) ([]pItem, bool) {
	fl.simpleDone[fs] = true
	refs := fs.contentRefs()
	var items []pItem
	safe := !fieldLocked(fs.FldLock)
	for _, ref := range refs.orderedChildren() {
		v := refs.valueAt(ref)
		if v == nil {
			continue
		}
		switch ref.kind {
		case pChildR:
			r, _ := v.(*CT_R)
			if r == nil || !runHoldsOnly(r, runChildT, runChildTab, runChildBr, runChildCr,
				runChildLastRenderedPageBreak, runChildNoBreakHyphen, runChildSoftHyphen) {
				safe = false
			}
		case pChildBookmarkStart, pChildBookmarkEnd, pChildProofErr, pChildPermStart, pChildPermEnd:
		default:
			safe = false
		}
		items = append(items, pItem{ref.kind, v})
	}
	ctx := fl.scan.simpleCtx[fs]
	if !safe {
		if fl.f.Skipped != nil {
			fl.f.Skipped(fs.Instr)
		}
		return nil, false
	}
	var rPr *CT_RPr
	for _, it := range items {
		if r, isRun := it.val.(*CT_R); isRun && r != nil && r.RPr != nil {
			rPr = r.RPr
			break
		}
	}
	text, ok := fl.f.Fill(fs.Instr, ctx.inInstr, rPrLang(rPr))
	if !ok {
		return nil, false
	}
	repl := replacement(items, rPr, text, ctx)
	fl.emptied(repl, ctx)
	return repl, true
}

// rPrLang returns the w:lang w:val of run properties, or "".
func rPrLang(rPr *CT_RPr) string {
	if rPr == nil || rPr.Lang == nil {
		return ""
	}
	return rPr.Lang.Val
}

// endIndex returns the index of the item holding end at or after i, or -1. The
// span between must be runs and markers only.
func endIndex(items []pItem, i int, end *CT_R) int {
	if end == nil {
		return -1
	}
	for j := i; j < len(items); j++ {
		switch items[j].kind {
		case pChildR:
			if r, _ := items[j].val.(*CT_R); r == end {
				return j
			}
		case pChildBookmarkStart, pChildBookmarkEnd, pChildProofErr, pChildPermStart,
			pChildPermEnd, pChildCommentRangeStart, pChildCommentRangeEnd:
		default:
			return -1
		}
	}
	return -1
}

// resultRPr returns the run properties of the first run after the field's
// separate that carries text, or the begin run's when there is none.
func resultRPr(span []pItem) *CT_RPr {
	inResult := false
	var begin *CT_RPr
	for k, it := range span {
		r, ok := it.val.(*CT_R)
		if !ok || r == nil {
			continue
		}
		if k == 0 {
			begin = r.RPr
		}
		for _, fc := range r.FldChar {
			if fc != nil && fc.FldCharType == "separate" {
				inResult = true
			}
		}
		if inResult && len(r.FldChar) == 0 && len(r.T) > 0 {
			return r.RPr
		}
	}
	return begin
}

// replacement returns the items that take the place of a field's span: its
// start markers, the run with the text, and its end markers.
func replacement(span []pItem, rPr *CT_RPr, text string, ctx fieldCtx) []pItem {
	var starts, ends []pItem
	for _, it := range span {
		switch it.kind {
		case pChildBookmarkStart, pChildPermStart, pChildCommentRangeStart:
			starts = append(starts, it)
		case pChildBookmarkEnd, pChildPermEnd, pChildCommentRangeEnd:
			ends = append(ends, it)
		case pChildProofErr:
			if pe, _ := it.val.(*CT_ProofErr); pe != nil && (pe.Type == "spellStart" || pe.Type == "gramStart") {
				starts = append(starts, it)
			} else {
				ends = append(ends, it)
			}
		}
	}
	out := starts
	if r := textRun(rPr, text, ctx); r != nil {
		out = append(out, pItem{pChildR, r})
	}
	return append(out, ends...)
}

// textRun builds a run carrying text with the given properties, or nil when
// there is nothing to write. In an instruction the text is one w:instrText
// holding a single argument: quoted, with quotes and backslashes escaped, or
// only escaped inside an argument already quoted. Elsewhere line breaks become
// w:br and tabs w:tab between w:t elements.
func textRun(rPr *CT_RPr, text string, ctx fieldCtx) *CT_R {
	r := &CT_R{RPr: rPr}
	if ctx.inInstr {
		arg := strings.ReplaceAll(text, `\`, `\\`)
		arg = strings.ReplaceAll(arg, `"`, `\"`)
		if !ctx.inQuote {
			arg = `"` + arg + `"`
		}
		if arg == "" {
			return nil
		}
		r.AppendInstrText(&CT_Text{Space: "preserve", Text: arg})
		return r
	}
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var seg strings.Builder
	flush := func() {
		if seg.Len() == 0 {
			return
		}
		r.backfillChildOrder()
		r.childOrder = append(r.childOrder, runChildRef{runChildT, len(r.T)})
		r.T = append(r.T, &CT_Text{Space: "preserve", Text: seg.String()})
		seg.Reset()
	}
	for _, ch := range text {
		switch ch {
		case '\n':
			flush()
			r.AppendBr(&CT_Br{})
		case '\t':
			flush()
			r.AppendTab()
		default:
			seg.WriteRune(ch)
		}
	}
	flush()
	return r
}
