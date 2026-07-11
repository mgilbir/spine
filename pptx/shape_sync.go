package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// This file implements the dirty-shape half of the shape sync: mutations to
// shapes that are already represented in the slide XML (parsed from a file, or
// materialized after a ReplaceText) are flushed by updating their parsed nodes
// in place via shapeRefs. Only the modeled bits are re-marshaled; everything
// the domain model does not represent (extension lists, autofit settings,
// rotation, unknown attributes) is left untouched. Rebuilding the whole tree
// from the domain model instead would silently drop that content.

// hasDirtyShapes reports whether any shape on the slide carries unflushed
// domain mutations, so marshal() knows a sync is needed even when no shape was
// added or removed.
func (s *Slide) hasDirtyShapes() bool {
	for _, shape := range s.shapes {
		if shapeDirty(shape) {
			return true
		}
	}
	return false
}

// shapeDirty reports whether the shape has unflushed mutations.
func shapeDirty(shape Shape) bool {
	switch sh := shape.(type) {
	case *TextBox:
		return sh.dirty || sh.textFrame.isDirty()
	case *PlaceholderShape:
		return sh.dirty || sh.textFrame.isDirty()
	case *AutoShape:
		return sh.dirty || sh.textFrame.isDirty()
	case *Picture:
		return sh.dirty
	case *Video:
		return sh.dirty
	case *Audio:
		return sh.dirty
	case *Table:
		return sh.isDirty()
	case *GroupShape:
		return sh.dirty
	}
	return false
}

// clearShapeDirt resets the modification flags on every shape after a sync
// flushed them into the XML.
func (s *Slide) clearShapeDirt() {
	for _, shape := range s.shapes {
		clearShapeDirty(shape)
	}
}

func clearShapeDirty(shape Shape) {
	switch sh := shape.(type) {
	case *TextBox:
		sh.dirty = false
		sh.textFrame.clearDirty()
	case *PlaceholderShape:
		sh.dirty = false
		sh.textFrame.clearDirty()
	case *AutoShape:
		sh.dirty = false
		sh.textFrame.clearDirty()
	case *Picture:
		sh.dirty = false
	case *Video:
		sh.dirty = false
	case *Audio:
		sh.dirty = false
	case *Table:
		sh.clearDirty()
	case *GroupShape:
		sh.dirty = false
	}
}

// syncDirtyShapes updates the parsed nodes of dirty synced shapes in place.
// It must run after removals have been applied and shapeRefs re-indexed, so
// every ref targets its post-compaction node.
func (s *Slide) syncDirtyShapes(spTree *oxml.ShapeTree) {
	n := s.syncedShapes
	if n > len(s.shapeRefs) {
		n = len(s.shapeRefs)
	}
	for i := 0; i < n; i++ {
		shape := s.shapes[i]
		if !shapeDirty(shape) {
			continue
		}
		ref := s.shapeRefs[i]
		if ref.Index < 0 {
			continue
		}
		switch ref.Kind {
		case oxml.ChildSp:
			if ref.Index < len(spTree.Sp) {
				updateShapeNode(spTree.Sp[ref.Index], shape)
			}
		case oxml.ChildPic:
			if ref.Index < len(spTree.Pic) {
				updatePictureNode(spTree.Pic[ref.Index], shape)
			}
		case oxml.ChildGraphicFrame:
			if ref.Index < len(spTree.GraphicFrame) {
				updateGraphicFrameNode(spTree.GraphicFrame[ref.Index], shape)
			}
		case oxml.ChildGrpSp:
			if ref.Index < len(spTree.GrpSp) {
				updateGroupNode(spTree.GrpSp[ref.Index], shape)
			}
		}
	}
}

// updateShapeNode flushes a dirty text-bearing shape into its parsed p:sp
// node: name, explicit geometry, styling set through the API, and the text
// body. nvSpPr (placeholder info, locks) and everything else stay as parsed.
func updateShapeNode(sp *oxml.Shape, shape Shape) {
	var (
		base  *BaseShape
		tf    *TextFrame
		style *dml.SpPr
	)
	switch sh := shape.(type) {
	case *TextBox:
		base, tf, style = &sh.BaseShape, sh.textFrame, &sh.spPr
	case *PlaceholderShape:
		base, tf = &sh.BaseShape, sh.textFrame
	case *AutoShape:
		base, tf, style = &sh.BaseShape, sh.textFrame, &sh.spPr
	default:
		return
	}
	if base.dirty {
		if sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil && base.name != "" {
			sp.NvSpPr.CNvPr.Name = base.name
		}
		if sp.SpPr == nil {
			sp.SpPr = &dml.SpPr{}
		}
		updateXfrm(sp.SpPr, base)
		if style != nil {
			applyShapeStyle(sp.SpPr, style)
		}
	}
	updateTxBody(&sp.TxBody, tf)
}

// updateXfrm writes the domain position/size into an existing xfrm, creating
// one only when the shape has an explicit placement — a parsed shape without
// an xfrm inherits its placement (typically from the layout placeholder), and
// writing zeroes would move it to the origin. Rotation and flips on an
// existing xfrm are preserved.
func updateXfrm(spPr *dml.SpPr, base *BaseShape) {
	if spPr.Xfrm == nil {
		if base.x == 0 && base.y == 0 && base.width == 0 && base.height == 0 {
			return
		}
		spPr.Xfrm = &dml.Xfrm{}
	}
	spPr.Xfrm.Off = &dml.OffXML{X: int64(base.x), Y: int64(base.y)}
	spPr.Xfrm.Ext = &dml.ExtXML{Cx: int64(base.width), Cy: int64(base.height)}
}

// updateTxBody flushes text-frame edits into an existing a:txBody. Content
// edits replace the a:p children wholesale (the domain model owns them once
// the caller rewrites text), while body-level edits update only the bodyPr
// attributes the API models, so unmodeled ones (autofit, columns, vertical
// text, ...) survive.
func updateTxBody(dst **dml.TxBody, tf *TextFrame) {
	if tf == nil || !tf.isDirty() {
		return
	}
	if *dst == nil {
		*dst = textFrameToOxml(tf)
		return
	}
	body := *dst
	if tf.isContentDirty() {
		ps := make([]*dml.P, 0, len(tf.paragraphs))
		for _, para := range tf.paragraphs {
			ps = append(ps, paragraphToOxml(para))
		}
		if len(ps) == 0 {
			ps = append(ps, &dml.P{})
		}
		body.P = ps
	}
	if tf.bodyDirty {
		if body.BodyPr == nil {
			body.BodyPr = &dml.BodyPr{}
		}
		bp := body.BodyPr
		bp.Wrap = string(tf.wrap)
		bp.Anchor = string(tf.anchor)
		l := int64(tf.margins.Left)
		t := int64(tf.margins.Top)
		r := int64(tf.margins.Right)
		b := int64(tf.margins.Bottom)
		bp.LIns, bp.TIns, bp.RIns, bp.BIns = &l, &t, &r, &b
	}
}

// updatePictureNode flushes a dirty picture (or embedded media shape) into its
// parsed p:pic node: name, alt text, explicit geometry, and cropping. The blip
// references are managed by the image-replacement and media-embedding paths.
func updatePictureNode(pic *oxml.Picture, shape Shape) {
	var (
		base      *BaseShape
		domainPic *Picture
	)
	switch sh := shape.(type) {
	case *Picture:
		base, domainPic = &sh.BaseShape, sh
	case *Video:
		base = &sh.BaseShape
	case *Audio:
		base = &sh.BaseShape
	default:
		return
	}
	if !base.dirty {
		return
	}
	if pic.NvPicPr != nil && pic.NvPicPr.CNvPr != nil {
		if base.name != "" {
			pic.NvPicPr.CNvPr.Name = base.name
		}
		if domainPic != nil {
			pic.NvPicPr.CNvPr.Descr = domainPic.description
		}
	}
	if pic.SpPr == nil {
		pic.SpPr = &dml.SpPr{}
	}
	updateXfrm(pic.SpPr, base)
	if domainPic != nil && pic.BlipFill != nil {
		if domainPic.cropLeft > 0 || domainPic.cropTop > 0 || domainPic.cropRight > 0 || domainPic.cropBottom > 0 {
			pic.BlipFill.SrcRect = &dml.SrcRect{
				L: int32(domainPic.cropLeft * 100000),
				T: int32(domainPic.cropTop * 100000),
				R: int32(domainPic.cropRight * 100000),
				B: int32(domainPic.cropBottom * 100000),
			}
		} else {
			pic.BlipFill.SrcRect = nil
		}
	}
}

// updateGraphicFrameNode flushes a dirty table into its parsed p:graphicFrame
// node. Text and property edits patch the parsed a:tbl in place, so untouched
// cells keep everything the domain model does not represent (margins, borders,
// fills, table style references). Only structural changes (rows/columns added
// or removed) regenerate the node — and even then parsed styling is carried
// over for surviving cells (see regenerateTableNode). Non-table graphic frames
// are never dirty (they are not materialized), so they are untouched.
func updateGraphicFrameNode(gf *oxml.GraphicFrame, shape Shape) {
	tbl, ok := shape.(*Table)
	if !ok {
		return
	}
	if tbl.BaseShape.dirty {
		if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil && tbl.name != "" {
			gf.NvGraphicFramePr.CNvPr.Name = tbl.name
		}
		if gf.Xfrm == nil {
			gf.Xfrm = &dml.Xfrm{}
		}
		gf.Xfrm.Off = &dml.OffXML{X: int64(tbl.x), Y: int64(tbl.y)}
		gf.Xfrm.Ext = &dml.ExtXML{Cx: int64(tbl.width), Cy: int64(tbl.height)}
	}
	if tbl.contentDirty() && gf.Graphic != nil && gf.Graphic.GraphicData != nil && gf.Graphic.GraphicData.Table != nil {
		atbl := gf.Graphic.GraphicData.Table
		if !tbl.structDirty && tableShapeMatches(tbl, atbl) {
			patchTableNode(atbl, tbl)
		} else {
			gf.Graphic.GraphicData.Table = regenerateTableNode(tbl, atbl)
		}
	}
}

// tableShapeMatches reports whether the domain table and the parsed a:tbl have
// the same grid shape, so per-cell patching can pair them up by index.
func tableShapeMatches(t *Table, atbl *oxml.ATable) bool {
	if atbl == nil || len(t.rows) != len(atbl.Tr) {
		return false
	}
	for i, row := range t.rows {
		if len(row.cells) != len(atbl.Tr[i].Tc) {
			return false
		}
	}
	return atbl.TblGrid != nil && len(t.colWidths) == len(atbl.TblGrid.GridCol)
}

// patchTableNode flushes non-structural table edits into the parsed a:tbl in
// place: table-level properties and column widths when they were set, row
// heights of dirty rows, and per-cell text/properties of dirty cells. Cells
// without unflushed edits are not touched at all.
func patchTableNode(atbl *oxml.ATable, t *Table) {
	if t.propsDirty {
		if atbl.TblPr == nil {
			atbl.TblPr = &oxml.ATblPr{}
		}
		pr := atbl.TblPr
		pr.FirstRow, pr.FirstCol = t.firstRow, t.firstCol
		pr.LastRow, pr.LastCol = t.lastRow, t.lastCol
		pr.BandRow, pr.BandCol = t.bandRow, t.bandCol
		for i, gc := range atbl.TblGrid.GridCol {
			gc.W = int64(t.colWidths[i])
		}
	}
	for i, row := range t.rows {
		tr := atbl.Tr[i]
		if row.dirty {
			tr.H = int64(row.height)
		}
		row.sourceTr = tr
		for j, cell := range row.cells {
			tc := tr.Tc[j]
			if cell.textFrame != nil && cell.textFrame.isDirty() {
				updateTxBody(&tc.TxBody, cell.textFrame)
			}
			if cell.dirty {
				applyCellProps(tc, cell)
			}
			cell.sourceTc = tc
		}
	}
}

// applyCellProps writes the cell properties the domain API models into the
// parsed a:tc node, leaving everything it does not model (margins, per-cell
// extension content) alone. Borders are written only for edges the domain
// carries a border for: the parse does not materialize borders, so a non-nil
// edge always means an explicit SetBorder call.
func applyCellProps(tc *oxml.ATc, cell *TableCell) {
	if tc.TcPr == nil {
		tc.TcPr = &oxml.ATcPr{}
	}
	pr := tc.TcPr
	if cell.vertAlign != "" {
		pr.Anchor = string(cell.vertAlign)
	}
	if cell.borderLeft != nil {
		pr.LnL = tableBorderToLn(cell.borderLeft)
	}
	if cell.borderRight != nil {
		pr.LnR = tableBorderToLn(cell.borderRight)
	}
	if cell.borderTop != nil {
		pr.LnT = tableBorderToLn(cell.borderTop)
	}
	if cell.borderBottom != nil {
		pr.LnB = tableBorderToLn(cell.borderBottom)
	}
	if cell.fill != nil {
		pr.SolidFill = colorToOxml(cell.fill)
		pr.NoFill = nil
	} else {
		// ClearFill and never-filled both hold nil: either way the domain
		// says the cell has no solid fill.
		pr.SolidFill = nil
	}
	if cell.rowSpan > 1 {
		tc.RowSpan = cell.rowSpan
	} else {
		tc.RowSpan = 0
	}
	if cell.colSpan > 1 {
		tc.GridSpan = cell.colSpan
	} else {
		tc.GridSpan = 0
	}
	tc.HMerge = cell.hMerge
	tc.VMerge = cell.vMerge
}

// updateGroupNode flushes a dirty group's name and placement into its parsed
// p:grpSp node. Group content is not materialized for mutation, so only the
// group-level frame is synced; the child coordinate space (chOff/chExt) is
// preserved.
func updateGroupNode(gs *oxml.GroupShape, shape Shape) {
	grp, ok := shape.(*GroupShape)
	if !ok || !grp.dirty {
		return
	}
	if gs.NvGrpSpPr != nil && gs.NvGrpSpPr.CNvPr != nil && grp.name != "" {
		gs.NvGrpSpPr.CNvPr.Name = grp.name
	}
	if gs.GrpSpPr != nil && gs.GrpSpPr.Xfrm != nil {
		gs.GrpSpPr.Xfrm.Off = &dml.OffXML{X: int64(grp.x), Y: int64(grp.y)}
		gs.GrpSpPr.Xfrm.Ext = &dml.ExtXML{Cx: int64(grp.width), Cy: int64(grp.height)}
	}
}
