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

	var (
		newR     []*CT_R
		newOrder []pChildRef
		segment  []*CT_R
		changed  bool
	)
	flush := func() {
		if len(segment) == 0 {
			return
		}
		out, ok := fn(segment)
		if ok {
			changed = true
		}
		for _, r := range out {
			newOrder = append(newOrder, pChildRef{pChildR, len(newR)})
			newR = append(newR, r)
		}
		segment = nil
	}
	for _, ref := range p.childOrder {
		if ref.kind == pChildR && ref.index < len(p.R) && p.R[ref.index].IsTextOnly() {
			segment = append(segment, p.R[ref.index])
			continue
		}
		flush()
		if ref.kind == pChildR {
			if ref.index < len(p.R) {
				newOrder = append(newOrder, pChildRef{pChildR, len(newR)})
				newR = append(newR, p.R[ref.index])
			}
			continue
		}
		newOrder = append(newOrder, ref)
	}
	flush()

	if !changed {
		return false
	}
	p.R = newR
	p.childOrder = newOrder
	return true
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
	if len(hf.childOrder) == 0 {
		out = append(out, hf.P...)
		for _, tbl := range hf.Tbl {
			collectTableParagraphs(tbl, &out)
		}
		for _, s := range hf.SdtBlock {
			out = append(out, s.contentParagraphs()...)
		}
		return out
	}
	for _, ref := range hf.childOrder {
		switch ref.kind {
		case bodyChildP:
			if ref.index < len(hf.P) {
				out = append(out, hf.P[ref.index])
			}
		case bodyChildTbl:
			if ref.index < len(hf.Tbl) {
				collectTableParagraphs(hf.Tbl[ref.index], &out)
			}
		case bodyChildSdt:
			if ref.index < len(hf.SdtBlock) {
				out = append(out, hf.SdtBlock[ref.index].contentParagraphs()...)
			}
		}
	}
	return out
}
