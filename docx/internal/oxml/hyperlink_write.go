package oxml

// AppendHyperlink appends a hyperlink (w:hyperlink) to the paragraph,
// maintaining child order like AppendR so it survives the childOrder-gated
// marshal of paragraphs parsed from a file.
func (p *CT_P) AppendHyperlink(h *CT_Hyperlink) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildHyperlink, len(p.Hyperlink)})
	p.Hyperlink = append(p.Hyperlink, h)
}
