package oxml

// ContainsMath reports whether the document carries any captured Office Math
// content (m:oMath / m:oMathPara). It walks every paragraph reachable from the
// body through the shared block visitor (tables, rows, cells, nested tables and
// every SDT wrapper), so the math namespace declaration the marshaler derives
// from it cannot be missed for content only one walker knew how to reach.
func (doc *CT_Document) ContainsMath() bool {
	if doc == nil || doc.Body == nil {
		return false
	}
	found := false
	visitBlockContent(doc.Body.childOrder, doc.Body.P, doc.Body.Tbl, doc.Body.SdtBlock, blockVisitor{
		Para: func(p *CT_P) {
			if p != nil && (len(p.OMath) > 0 || len(p.OMathPara) > 0) {
				found = true
			}
		},
	})
	return found
}
