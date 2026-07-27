package oxml

// Structural (table/row/cell/section) tracked changes, enumerated through the
// shared block visitor.
//
// These were walked twice with two different descents. MaxRevisionID reached
// them wherever they sit — nested tables, and rows or cells wrapped in a block
// SDT — because C413 pointed it at the block visitor. The public Revisions()
// enumeration kept its own hand-rolled walk over Body.Tbl, so a structural
// revision inside a body-level SDT was allocated an id it never reported: the
// gap was known for allocation and not for enumeration (C495). The same walk
// also never reached a paragraph-level w:sectPr, so a w:sectPrChange on any
// mid-document section break was invisible while the body-level one was
// reported.

// StructRevKind names a structural tracked change.
type StructRevKind int

const (
	// StructRevTableFormat is a table-property change (w:tblPrChange).
	StructRevTableFormat StructRevKind = iota
	// StructRevRowInsertion is an inserted table row (w:trPr/w:ins).
	StructRevRowInsertion
	// StructRevRowDeletion is a deleted table row (w:trPr/w:del).
	StructRevRowDeletion
	// StructRevRowFormat is a row-property change (w:trPr/w:trPrChange).
	StructRevRowFormat
	// StructRevCellInsertion is an inserted table cell (w:tcPr/w:cellIns).
	StructRevCellInsertion
	// StructRevCellDeletion is a deleted table cell (w:tcPr/w:cellDel).
	StructRevCellDeletion
	// StructRevCellMerge is a cell-merge revision (w:tcPr/w:cellMerge).
	StructRevCellMerge
	// StructRevCellFormat is a cell-property change (w:tcPr/w:tcPrChange).
	StructRevCellFormat
	// StructRevSectionFormat is a section-property change (w:sectPrChange),
	// whether on the body's final w:sectPr or on a paragraph-level one marking
	// a mid-document section break.
	StructRevSectionFormat
)

// StructRevision is one structural tracked change: its kind and the author and
// timestamp recorded on the change element. These revisions are read-only —
// transforming them safely is out of scope — so no transform target is carried.
type StructRevision struct {
	Kind   StructRevKind
	Author string
	Date   string
}

// StructuralRevisions returns the body's structural tracked changes in document
// order: every table, row and cell revision reachable through the block visitor
// (including tables nested in cells and rows or cells wrapped in a block SDT),
// every paragraph-level w:sectPrChange, and finally the body-level one.
func (body *CT_Body) StructuralRevisions() []StructRevision {
	if body == nil {
		return nil
	}
	out := collectStructRevisions(body.childOrder, body.P, body.Tbl, body.SdtBlock)
	if body.SectPr != nil && body.SectPr.SectPrChange != nil {
		out = append(out, StructRevision{
			Kind:   StructRevSectionFormat,
			Author: body.SectPr.SectPrChange.Author,
			Date:   body.SectPr.SectPrChange.Date,
		})
	}
	return out
}

// StructuralRevisions returns a header or footer's structural tracked changes,
// in document order (see CT_Body.StructuralRevisions). A header/footer carries
// no body-level section properties.
func (hf *CT_HdrFtr) StructuralRevisions() []StructRevision {
	if hf == nil {
		return nil
	}
	return collectStructRevisions(hf.childOrder, hf.P, hf.Tbl, hf.SdtBlock)
}

// collectStructRevisions walks body-shaped block content through the shared
// visitor, recording each structural revision it reaches in document order.
func collectStructRevisions(order []bodyChildRef, ps []*CT_P, tbls []*CT_Tbl, sdts []*CT_SdtBlock) []StructRevision {
	var out []StructRevision
	add := func(kind StructRevKind, author, date string) {
		out = append(out, StructRevision{Kind: kind, Author: author, Date: date})
	}
	visitBlockContent(order, ps, tbls, sdts, blockVisitor{
		Para: func(p *CT_P) {
			// A paragraph-level w:sectPr marks a mid-document section break;
			// its w:sectPrChange is as much a revision as the body-level one.
			if p == nil || p.PPr == nil || p.PPr.SectPr == nil {
				return
			}
			if c := p.PPr.SectPr.SectPrChange; c != nil {
				add(StructRevSectionFormat, c.Author, c.Date)
			}
		},
		Tbl: func(tbl *CT_Tbl) {
			if tbl.TblPr != nil && tbl.TblPr.TblPrChange != nil {
				add(StructRevTableFormat, tbl.TblPr.TblPrChange.Author, tbl.TblPr.TblPrChange.Date)
			}
		},
		Row: func(tr *CT_Tr) {
			if tr.TrPr == nil {
				return
			}
			if tr.TrPr.Ins != nil {
				add(StructRevRowInsertion, tr.TrPr.Ins.Author, tr.TrPr.Ins.Date)
			}
			if tr.TrPr.Del != nil {
				add(StructRevRowDeletion, tr.TrPr.Del.Author, tr.TrPr.Del.Date)
			}
			if tr.TrPr.TrPrChange != nil {
				add(StructRevRowFormat, tr.TrPr.TrPrChange.Author, tr.TrPr.TrPrChange.Date)
			}
		},
		Cell: func(tc *CT_Tc) {
			if tc.TcPr == nil {
				return
			}
			if tc.TcPr.CellIns != nil {
				add(StructRevCellInsertion, tc.TcPr.CellIns.Author, tc.TcPr.CellIns.Date)
			}
			if tc.TcPr.CellDel != nil {
				add(StructRevCellDeletion, tc.TcPr.CellDel.Author, tc.TcPr.CellDel.Date)
			}
			if tc.TcPr.CellMerge != nil {
				add(StructRevCellMerge, tc.TcPr.CellMerge.Author, tc.TcPr.CellMerge.Date)
			}
			if tc.TcPr.TcPrChange != nil {
				add(StructRevCellFormat, tc.TcPr.TcPrChange.Author, tc.TcPr.TcPrChange.Date)
			}
		},
	})
	return out
}

// MaxHdrFtrRevisionID returns the highest numeric tracked-change w:id in a
// header or footer part — the same scan MaxRevisionID performs over the body.
// Revision-id allocation needs it because Revisions() enumerates header and
// footer revisions too, so seeding from the body alone hands an authored
// revision an id a header already uses (C496, the revision twin of C408).
func MaxHdrFtrRevisionID(hf *CT_HdrFtr) int {
	if hf == nil {
		return 0
	}
	maxID := 0
	consider := func(s string) {
		if n, ok := atoiOK(s); ok && n > maxID {
			maxID = n
		}
	}
	for _, p := range hf.AllParagraphs() {
		for _, rr := range CollectParagraphRevisions(p) {
			consider(rr.Id)
		}
		MaxMoveID(p, consider)
		if p.PPr != nil && p.PPr.SectPr != nil && p.PPr.SectPr.SectPrChange != nil {
			consider(p.PPr.SectPr.SectPrChange.Id)
		}
	}
	visitBlockContent(hf.childOrder, nil, hf.Tbl, hf.SdtBlock, structuralRevisionVisitor(consider))
	return maxID
}
