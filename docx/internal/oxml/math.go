package oxml

// ContainsMath reports whether the document carries any captured Office Math
// content (m:oMath / m:oMathPara). It walks every paragraph reachable from the
// body: top-level paragraphs, tables (recursively, including nested tables and
// SDT cells), and block-level structured document tags. Used by the document
// marshaler to decide whether the math namespace must be declared on the root
// element so the re-emitted, prefixed math elements stay bound.
func (doc *CT_Document) ContainsMath() bool {
	if doc == nil || doc.Body == nil {
		return false
	}
	return blockContentHasMath(doc.Body.P, doc.Body.Tbl, doc.Body.SdtBlock)
}

func blockContentHasMath(ps []*CT_P, tbls []*CT_Tbl, sdts []*CT_SdtBlock) bool {
	for _, p := range ps {
		if p != nil && (len(p.OMath) > 0 || len(p.OMathPara) > 0) {
			return true
		}
	}
	for _, tbl := range tbls {
		if tblHasMath(tbl) {
			return true
		}
	}
	for _, sdt := range sdts {
		if sdtHasMath(sdt) {
			return true
		}
	}
	return false
}

func tblHasMath(tbl *CT_Tbl) bool {
	if tbl == nil {
		return false
	}
	for _, tr := range tbl.Tr {
		if tr == nil {
			continue
		}
		for _, tc := range tr.Tc {
			if tc != nil && blockContentHasMath(tc.P, tc.Tbl, tc.SdtBlock) {
				return true
			}
		}
		for _, sdt := range tr.SdtCell {
			if sdtHasMath(sdt) {
				return true
			}
		}
	}
	return false
}

func sdtHasMath(sdt *CT_SdtBlock) bool {
	if sdt == nil || sdt.SdtContent == nil {
		return false
	}
	return blockContentHasMath(sdt.SdtContent.P, sdt.SdtContent.Tbl, sdt.SdtContent.SdtBlock)
}
