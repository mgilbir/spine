package oxml

// AllParagraphs returns every paragraph reachable from the body in document
// order, descending into tables and block-level structured document tags. This
// covers content that Paragraphs() (top-level only) does not, so document-wide
// readers (hyperlinks, images) do not under-report table-nested content.
func (body *CT_Body) AllParagraphs() []*CT_P {
	var out []*CT_P
	if len(body.childOrder) == 0 {
		out = append(out, body.P...)
		for _, tbl := range body.Tbl {
			collectTableParagraphs(tbl, &out)
		}
		for _, s := range body.SdtBlock {
			out = append(out, s.contentParagraphs()...)
		}
		return out
	}
	for _, ref := range body.childOrder {
		switch ref.kind {
		case bodyChildP:
			if ref.index < len(body.P) {
				out = append(out, body.P[ref.index])
			}
		case bodyChildTbl:
			if ref.index < len(body.Tbl) {
				collectTableParagraphs(body.Tbl[ref.index], &out)
			}
		case bodyChildSdt:
			if ref.index < len(body.SdtBlock) {
				out = append(out, body.SdtBlock[ref.index].contentParagraphs()...)
			}
		}
	}
	return out
}

// collectTableParagraphs appends every paragraph in a table (all cells, nested
// tables, and cell-level SDTs) to out in row-major document order.
func collectTableParagraphs(tbl *CT_Tbl, out *[]*CT_P) {
	if tbl == nil {
		return
	}
	for _, tr := range tbl.Tr {
		if tr == nil {
			continue
		}
		for _, tc := range tr.Tc {
			if tc == nil {
				continue
			}
			*out = append(*out, tc.P...)
			for _, nested := range tc.Tbl {
				collectTableParagraphs(nested, out)
			}
			for _, s := range tc.SdtBlock {
				*out = append(*out, s.contentParagraphs()...)
			}
		}
	}
}

// InsertRunAfter inserts newRun into the paragraph immediately after target,
// maintaining child order. Returns false if target is not a direct child run of
// the paragraph.
func (p *CT_P) InsertRunAfter(target, newRun *CT_R) bool {
	p.backfillChildOrder()
	pos := p.runChildOrderPos(target)
	if pos < 0 {
		return false
	}
	p.R = append(p.R, newRun)
	p.insertPChildRefAt(pos+1, pChildRef{pChildR, len(p.R) - 1})
	return true
}
