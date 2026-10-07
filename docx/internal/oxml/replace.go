package oxml

import "strings"

// Text returns the run's visible text: the concatenation of its w:t elements'
// content. Non-text children (breaks, tabs, drawings, fields, …) contribute
// nothing.
func (r *CT_R) Text() string {
	if r == nil {
		return ""
	}
	var sb strings.Builder
	for _, t := range r.T {
		sb.WriteString(t.Text)
	}
	return sb.String()
}

// IsTextOnly reports whether the run's only content is w:t text (no breaks,
// tabs, drawings, symbols, fields, footnote/endnote refs, or any other inline
// child). Such runs can be freely concatenated and rebuilt for text
// replacement; runs carrying anything else are treated as opaque boundaries so
// their content is never disturbed.
func (r *CT_R) IsTextOnly() bool {
	if r == nil {
		return false
	}
	return len(r.Br) == 0 && len(r.Tab) == 0 && len(r.Cr) == 0 &&
		len(r.Sym) == 0 && len(r.Drawing) == 0 && len(r.FtnRef) == 0 &&
		len(r.EndnoteRef) == 0 && len(r.LastRenderedPageBreak) == 0 &&
		len(r.NoBreakHyphen) == 0 && len(r.SoftHyphen) == 0 &&
		len(r.FldChar) == 0 && len(r.InstrText) == 0 && len(r.DelText) == 0 &&
		len(r.CommentReference) == 0 && len(r.Ptab) == 0 && len(r.Pict) == 0 &&
		len(r.Object) == 0 && len(r.AlternateContent) == 0 && len(r.Raw) == 0
}

// CloneWithText returns a new run that carries the given text and inherits the
// receiver's run properties (w:rPr) and revision-save ids. The text is stored
// as a single xml:space="preserve" w:t so leading/trailing spaces survive a
// round trip. The receiver's own w:t elements and non-property attributes are
// not copied — this is used to materialize replacement text with a source
// run's formatting.
func (r *CT_R) CloneWithText(text string) *CT_R {
	nr := &CT_R{}
	if r != nil {
		nr.RPr = r.RPr
		nr.RsidR = r.RsidR
		nr.RsidRPr = r.RsidRPr
		nr.RsidDel = r.RsidDel
	}
	nr.SetTexts([]*CT_Text{{Space: "preserve", Text: text}})
	return nr
}

// ReplaceInTextRuns rewrites the paragraph's text-only top-level runs segment
// by segment. fn is called once for each maximal run of consecutive text-only
// w:r children in document order; it returns the rewritten runs for that
// segment and whether it changed anything. Non-run children (hyperlinks,
// fields, SDTs, …) and runs that are not text-only act as segment boundaries
// and pass through untouched — a template key is deliberately not matched
// across such a boundary.
//
// Spelling and grammar markers (w:proofErr) are the exception: they are hints
// Word writes around a word it flagged, and they land between the runs of a
// placeholder as often as anywhere ("{{", spellStart, "name", spellEnd, "}}"),
// so they do not end a segment. When a segment changes, the markers inside it
// may no longer bracket the text they flagged and are dropped, together with their
// spellStart/spellEnd or gramStart/gramEnd partners so no half of a pair is
// left behind; Word proofs the text again when it opens the document.
//
// The paragraph's run slice and child order are rebuilt only when some segment
// actually changed, so a paragraph with no matching text is left byte-for-byte
// identical. It reports whether anything changed.
func (p *CT_P) ReplaceInTextRuns(fn func(runs []*CT_R) ([]*CT_R, bool)) bool {
	if p == nil {
		return false
	}

	// No recorded child order: the paragraph's runs are its only content and
	// are contiguous (a programmatically built paragraph). Still honor
	// non-text-only runs as boundaries.
	if len(p.childOrder) == 0 {
		newR, changed := replaceRunSegments(p.R, fn)
		if !changed {
			return false
		}
		p.R = newR
		return true
	}

	// A segment spans child-order positions first..last, both text-only runs;
	// any w:proofErr markers between them belong to it.
	type textSegment struct {
		first, last int
		runs        []*CT_R
		out         []*CT_R
		changed     bool
	}
	var segs []textSegment
	open := false
	for i, ref := range p.childOrder {
		switch {
		case ref.kind == pChildR && ref.index < len(p.R) && p.R[ref.index].IsTextOnly():
			if !open {
				segs = append(segs, textSegment{first: i})
				open = true
			}
			seg := &segs[len(segs)-1]
			seg.last = i
			seg.runs = append(seg.runs, p.R[ref.index])
		case ref.kind == pChildProofErr:
			// Transparent: neither ends a segment nor joins one on its own.
		default:
			open = false
		}
	}

	changed := false
	// segAt maps the child-order position of a changed segment's first run to
	// the segment, and inChanged marks every position the segment covers.
	segAt := map[int]int{}
	inChanged := make([]bool, len(p.childOrder))
	for si := range segs {
		seg := &segs[si]
		seg.out, seg.changed = fn(seg.runs)
		if !seg.changed {
			continue
		}
		changed = true
		segAt[seg.first] = si
		for i := seg.first; i <= seg.last; i++ {
			inChanged[i] = true
		}
	}
	if !changed {
		return false
	}

	dropped := p.proofErrToDrop(inChanged)

	var (
		newR        []*CT_R
		newProofErr []*CT_ProofErr
		newOrder    []pChildRef
	)
	for i := 0; i < len(p.childOrder); i++ {
		if si, ok := segAt[i]; ok {
			for _, r := range segs[si].out {
				newOrder = append(newOrder, pChildRef{pChildR, len(newR)})
				newR = append(newR, r)
			}
			i = segs[si].last
			continue
		}
		ref := p.childOrder[i]
		switch ref.kind {
		case pChildR:
			if ref.index < len(p.R) {
				newOrder = append(newOrder, pChildRef{pChildR, len(newR)})
				newR = append(newR, p.R[ref.index])
			}
		case pChildProofErr:
			if !dropped[i] && ref.index < len(p.ProofErr) {
				newOrder = append(newOrder, pChildRef{pChildProofErr, len(newProofErr)})
				newProofErr = append(newProofErr, p.ProofErr[ref.index])
			}
		default:
			newOrder = append(newOrder, ref)
		}
	}
	p.R = newR
	p.ProofErr = newProofErr
	p.childOrder = newOrder
	return true
}

// proofErrToDrop returns the child-order positions of the w:proofErr markers
// to remove after a replacement: those inside a rewritten segment (inChanged)
// and the partners they were paired with. A spellStart pairs with the next
// spellEnd and a gramStart with the next gramEnd, the two kinds independently.
func (p *CT_P) proofErrToDrop(inChanged []bool) map[int]bool {
	dropped := map[int]bool{}
	openAt := map[string]int{} // "spell"/"gram" -> position of the unpaired start
	for i, ref := range p.childOrder {
		if ref.kind != pChildProofErr || ref.index >= len(p.ProofErr) {
			continue
		}
		if inChanged[i] {
			dropped[i] = true
		}
		pe := p.ProofErr[ref.index]
		if pe == nil {
			continue
		}
		kind, isStart := "", false
		switch pe.Type {
		case "spellStart":
			kind, isStart = "spell", true
		case "spellEnd":
			kind = "spell"
		case "gramStart":
			kind, isStart = "gram", true
		case "gramEnd":
			kind = "gram"
		default:
			continue
		}
		if isStart {
			openAt[kind] = i
			continue
		}
		start, ok := openAt[kind]
		if !ok {
			continue
		}
		delete(openAt, kind)
		if inChanged[start] || inChanged[i] {
			dropped[start] = true
			dropped[i] = true
		}
	}
	return dropped
}

// replaceRunSegments applies fn to each maximal segment of consecutive
// text-only runs, keeping non-text-only runs in place as boundaries. It
// returns the rebuilt run slice and whether any segment changed.
func replaceRunSegments(runs []*CT_R, fn func(runs []*CT_R) ([]*CT_R, bool)) ([]*CT_R, bool) {
	var (
		out     []*CT_R
		segment []*CT_R
		changed bool
	)
	flush := func() {
		if len(segment) == 0 {
			return
		}
		res, ok := fn(segment)
		if ok {
			changed = true
		}
		out = append(out, res...)
		segment = nil
	}
	for _, r := range runs {
		if r.IsTextOnly() {
			segment = append(segment, r)
			continue
		}
		flush()
		out = append(out, r)
	}
	flush()
	return out, changed
}

// AllParagraphs returns every paragraph in the header/footer in document order,
// descending into tables and block-level structured document tags — the same
// traversal CT_Body.AllParagraphs performs for the main document body.
func (hf *CT_HdrFtr) AllParagraphs() []*CT_P {
	var out []*CT_P
	visitBlockContent(hf.childOrder, hf.P, hf.Tbl, hf.SdtBlock, blockVisitor{
		Para: func(p *CT_P) { out = append(out, p) },
	})
	return out
}
