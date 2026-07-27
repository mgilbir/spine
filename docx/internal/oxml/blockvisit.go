package oxml

// This file holds the single descent used by every walker over WML block
// content (tables, rows, cells, block SDTs).
//
// Before it there were half a dozen hand-rolled walkers with independently
// chosen descent sets — collectTableParagraphs, tblHasMath, collectTableSdt,
// maxTableRevisionID, maxSdtBlockTableRevisionID, ContentControls — and each
// time a container was added to the model only some of them learned about it.
// The block SDTs that wrap rows (w:tbl/w:sdt) and cells (w:tr/w:sdt) reached
// two of the six (C330); the ones it missed then re-surfaced as C412, C413 and
// C508. Callers state *what* they want from each node and the descent stays in
// one place, so a new container is taught once.
//
// The public-side walkers in package docx (Document.Tables, the hyperlink and
// image readers) want the same descent; they reach it through the exported
// helpers built on top of this (CT_Body.AllParagraphs and friends).

// blockVisitor collects callbacks for the node kinds a block-content walk can
// reach. Every field is optional; a nil hook simply is not called, and the
// descent happens regardless so a walker interested only in leaves does not
// have to re-implement the tree.
type blockVisitor struct {
	// Para is called for each paragraph, in document order.
	Para func(*CT_P)
	// Tbl is called for each table before its rows are visited.
	Tbl func(*CT_Tbl)
	// Row is called for each row before its cells are visited.
	Row func(*CT_Tr)
	// Cell is called for each cell before its content is visited.
	Cell func(*CT_Tc)
	// Sdt is called for each block-level SDT before its content is visited.
	Sdt func(*CT_SdtBlock)
	// RowTrackChange is called for each row-level w:ins/w:del wrapper
	// (EG_ContentRowContent's tracked row insertion/deletion).
	RowTrackChange func(*CT_RunTrackChange)
}

// visitTable walks a table: its row-wrapping SDTs and its rows, in document
// order when the source order was captured, declaration order otherwise.
func visitTable(tbl *CT_Tbl, v blockVisitor) {
	if tbl == nil {
		return
	}
	if v.Tbl != nil {
		v.Tbl(tbl)
	}
	if len(tbl.childOrder) == 0 {
		for _, tr := range tbl.Tr {
			visitRow(tr, v)
		}
		for _, s := range tbl.SdtBlock {
			visitSdtBlock(s, v)
		}
		return
	}
	for _, ref := range tbl.childOrder {
		switch ref.kind {
		case tblChildTr:
			if ref.index < len(tbl.Tr) {
				visitRow(tbl.Tr[ref.index], v)
			}
		case tblChildSdt:
			if ref.index < len(tbl.SdtBlock) {
				visitSdtBlock(tbl.SdtBlock[ref.index], v)
			}
		}
	}
}

// visitRow walks a row: its cells, its cell-wrapping SDTs and its tracked
// row insertion/deletion wrappers.
func visitRow(tr *CT_Tr, v blockVisitor) {
	if tr == nil {
		return
	}
	if v.Row != nil {
		v.Row(tr)
	}
	if len(tr.childOrder) == 0 {
		for _, tc := range tr.Tc {
			visitCell(tc, v)
		}
		for _, s := range tr.SdtCell {
			visitSdtBlock(s, v)
		}
		if v.RowTrackChange != nil {
			for _, rtc := range tr.Ins {
				v.RowTrackChange(rtc)
			}
			for _, rtc := range tr.Del {
				v.RowTrackChange(rtc)
			}
		}
		return
	}
	for _, ref := range tr.childOrder {
		switch ref.kind {
		case trChildTc:
			if ref.index < len(tr.Tc) {
				visitCell(tr.Tc[ref.index], v)
			}
		case trChildSdtCell:
			if ref.index < len(tr.SdtCell) {
				visitSdtBlock(tr.SdtCell[ref.index], v)
			}
		case trChildIns:
			if v.RowTrackChange != nil && ref.index < len(tr.Ins) {
				v.RowTrackChange(tr.Ins[ref.index])
			}
		case trChildDel:
			if v.RowTrackChange != nil && ref.index < len(tr.Del) {
				v.RowTrackChange(tr.Del[ref.index])
			}
		}
	}
}

// visitCell walks a cell's block content: paragraphs, nested tables and
// nested block SDTs.
func visitCell(tc *CT_Tc, v blockVisitor) {
	if tc == nil {
		return
	}
	if v.Cell != nil {
		v.Cell(tc)
	}
	visitBlockContent(tc.childOrder, tc.P, tc.Tbl, tc.SdtBlock, v)
}

// visitSdtBlock walks a block-level SDT's content, which may hold ordinary
// block content as well as bare cells or rows (a w:sdt wrapping a w:tc or a
// w:tr sits inside a row or a table respectively).
func visitSdtBlock(s *CT_SdtBlock, v blockVisitor) {
	if s == nil {
		return
	}
	if v.Sdt != nil {
		v.Sdt(s)
	}
	sc := s.SdtContent
	if sc == nil {
		return
	}
	if len(sc.childOrder) == 0 {
		visitBlockContent(nil, sc.P, sc.Tbl, sc.SdtBlock, v)
		for _, tc := range sc.Tc {
			visitCell(tc, v)
		}
		for _, tr := range sc.Tr {
			visitRow(tr, v)
		}
		return
	}
	for _, ref := range sc.childOrder {
		switch ref.kind {
		case bodyChildP:
			if v.Para != nil && ref.index < len(sc.P) {
				v.Para(sc.P[ref.index])
			}
		case bodyChildTbl:
			if ref.index < len(sc.Tbl) {
				visitTable(sc.Tbl[ref.index], v)
			}
		case bodyChildSdt:
			if ref.index < len(sc.SdtBlock) {
				visitSdtBlock(sc.SdtBlock[ref.index], v)
			}
		case bodyChildTc:
			if ref.index < len(sc.Tc) {
				visitCell(sc.Tc[ref.index], v)
			}
		case bodyChildTr:
			if ref.index < len(sc.Tr) {
				visitRow(sc.Tr[ref.index], v)
			}
		}
	}
}

// visitBlockContent walks a body-shaped child list (paragraphs, tables, block
// SDTs) in captured document order, falling back to declaration order for
// programmatically built content.
func visitBlockContent(order []bodyChildRef, ps []*CT_P, tbls []*CT_Tbl, sdts []*CT_SdtBlock, v blockVisitor) {
	if len(order) == 0 {
		if v.Para != nil {
			for _, p := range ps {
				v.Para(p)
			}
		}
		for _, tbl := range tbls {
			visitTable(tbl, v)
		}
		for _, s := range sdts {
			visitSdtBlock(s, v)
		}
		return
	}
	for _, ref := range order {
		switch ref.kind {
		case bodyChildP:
			if v.Para != nil && ref.index < len(ps) {
				v.Para(ps[ref.index])
			}
		case bodyChildTbl:
			if ref.index < len(tbls) {
				visitTable(tbls[ref.index], v)
			}
		case bodyChildSdt:
			if ref.index < len(sdts) {
				visitSdtBlock(sdts[ref.index], v)
			}
		}
	}
}
