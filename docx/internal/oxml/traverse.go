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

// TextBlock is one top-level block-level child of a body or header/footer, in
// document order. Exactly one of P, Tbl, or Sdt is non-nil, identifying whether
// the block is a paragraph, a table, or a block-level structured document tag.
type TextBlock struct {
	P   *CT_P
	Tbl *CT_Tbl
	Sdt *CT_SdtBlock
}

// orderedTextBlocks interleaves paragraphs, tables, and block SDTs in the
// recorded document order. A container parsed from a file carries childOrder;
// one built programmatically may not, in which case the typed slices are
// emitted in their natural (paragraphs, then tables, then SDTs) order.
func orderedTextBlocks(childOrder []bodyChildRef, ps []*CT_P, tbls []*CT_Tbl, sdts []*CT_SdtBlock) []TextBlock {
	if len(childOrder) == 0 {
		out := make([]TextBlock, 0, len(ps)+len(tbls)+len(sdts))
		for _, p := range ps {
			out = append(out, TextBlock{P: p})
		}
		for _, t := range tbls {
			out = append(out, TextBlock{Tbl: t})
		}
		for _, s := range sdts {
			out = append(out, TextBlock{Sdt: s})
		}
		return out
	}
	out := make([]TextBlock, 0, len(childOrder))
	for _, ref := range childOrder {
		switch ref.kind {
		case bodyChildP:
			if ref.index < len(ps) {
				out = append(out, TextBlock{P: ps[ref.index]})
			}
		case bodyChildTbl:
			if ref.index < len(tbls) {
				out = append(out, TextBlock{Tbl: tbls[ref.index]})
			}
		case bodyChildSdt:
			if ref.index < len(sdts) {
				out = append(out, TextBlock{Sdt: sdts[ref.index]})
			}
		}
	}
	return out
}

// TextBlocks returns the body's top-level paragraphs, tables, and block SDTs in
// document order, so a reader can preserve the interleaving of paragraphs and
// tables that Paragraphs()/Tbl expose only as separate slices.
func (body *CT_Body) TextBlocks() []TextBlock {
	if body == nil {
		return nil
	}
	return orderedTextBlocks(body.childOrder, body.P, body.Tbl, body.SdtBlock)
}

// TextBlocks returns a header/footer's top-level paragraphs, tables, and block
// SDTs in document order.
func (hf *CT_HdrFtr) TextBlocks() []TextBlock {
	if hf == nil {
		return nil
	}
	return orderedTextBlocks(hf.childOrder, hf.P, hf.Tbl, hf.SdtBlock)
}

// ContentParagraphs returns the paragraphs contained in a block-level SDT, in
// document order, descending into nested SDTs.
func (s *CT_SdtBlock) ContentParagraphs() []*CT_P {
	return s.contentParagraphs()
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
