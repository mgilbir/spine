package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// materializeShapes converts the parsed XML shape tree into Go-level Shape objects.
// It runs when a slide is first parsed (lazily on access, or at construction for
// a created slide). The shapes are populated for read access; shapesModified
// remains false so that the original XML is preserved during save unless the
// user explicitly modifies shapes.
func (s *Slide) materializeShapes() {
	// Read sxModel directly, never sx(): sx() calls this after setting sxModel,
	// so routing back through sx() would recurse. The model is always present
	// here (the caller just parsed or built it).
	if s.sxModel == nil || s.sxModel.CSld == nil || s.sxModel.CSld.SpTree == nil {
		return
	}

	spTree := s.sxModel.CSld.SpTree
	s.shapeRefs = nil

	// Materialize shapes in their original z-order using childOrder if available.
	// Each materialized shape records the child reference it came from, so it can
	// later be removed surgically without rebuilding the whole tree.
	if len(spTree.ChildOrder()) > 0 {
		add := func(shape Shape, ref oxml.ChildRef) {
			s.setShapeBackRef(shape)
			s.shapeCache = append(s.shapeCache, shape)
			s.shapeRefs = append(s.shapeRefs, ref)
		}
		for _, ref := range spTree.ChildOrder() {
			switch ref.Kind {
			case oxml.ChildSp:
				if ref.Index < len(spTree.Sp) {
					if shape := oxmlShapeToGoShape(spTree.Sp[ref.Index]); shape != nil {
						add(shape, ref)
					}
				}
			case oxml.ChildPic:
				if ref.Index < len(spTree.Pic) {
					if pic := oxmlPictureToGoPicture(spTree.Pic[ref.Index]); pic != nil {
						add(pic, ref)
					}
				}
			case oxml.ChildGraphicFrame:
				// Tables are the only graphic frames we materialize for now
				if ref.Index < len(spTree.GraphicFrame) {
					if tbl := oxmlGraphicFrameToGoTable(spTree.GraphicFrame[ref.Index]); tbl != nil {
						add(tbl, ref)
					}
				}
			case oxml.ChildGrpSp:
				if ref.Index < len(spTree.GrpSp) {
					if grp := oxmlGroupShapeToGoGroupShape(spTree.GrpSp[ref.Index], s); grp != nil {
						add(grp, ref)
					}
				}
			case oxml.ChildCxnSp:
				if ref.Index < len(spTree.CxnSp) {
					if cxn := oxmlCxnSpToGoConnector(spTree.CxnSp[ref.Index]); cxn != nil {
						add(cxn, ref)
					}
				}
			}
		}
	} else {
		// No child order tracking (a tree rebuilt from the domain model, e.g.
		// after a ReplaceText on a created deck) — iterate typed slices in
		// order. Refs are recorded here too, so in-place edits and surgical
		// removals keep working on the re-materialized shapes.
		for i, sp := range spTree.Sp {
			if shape := oxmlShapeToGoShape(sp); shape != nil {
				s.setShapeBackRef(shape)
				s.shapeCache = append(s.shapeCache, shape)
				s.shapeRefs = append(s.shapeRefs, oxml.ChildRef{Kind: oxml.ChildSp, Index: i})
			}
		}
		for i, pic := range spTree.Pic {
			if p := oxmlPictureToGoPicture(pic); p != nil {
				s.setShapeBackRef(p)
				s.shapeCache = append(s.shapeCache, p)
				s.shapeRefs = append(s.shapeRefs, oxml.ChildRef{Kind: oxml.ChildPic, Index: i})
			}
		}
		for i, gf := range spTree.GraphicFrame {
			if tbl := oxmlGraphicFrameToGoTable(gf); tbl != nil {
				s.shapeCache = append(s.shapeCache, tbl)
				s.shapeRefs = append(s.shapeRefs, oxml.ChildRef{Kind: oxml.ChildGraphicFrame, Index: i})
			}
		}
		for i, grp := range spTree.GrpSp {
			if g := oxmlGroupShapeToGoGroupShape(grp, s); g != nil {
				s.shapeCache = append(s.shapeCache, g)
				s.shapeRefs = append(s.shapeRefs, oxml.ChildRef{Kind: oxml.ChildGrpSp, Index: i})
			}
		}
		for i, cxn := range spTree.CxnSp {
			if c := oxmlCxnSpToGoConnector(cxn); c != nil {
				s.shapeCache = append(s.shapeCache, c)
				s.shapeRefs = append(s.shapeRefs, oxml.ChildRef{Kind: oxml.ChildCxnSp, Index: i})
			}
		}
	}

	// Everything materialized so far is already represented in the parsed
	// XML; marshal() appends only shapes added after this point.
	s.syncedShapes = len(s.shapeCache)

	// Resolve hyperlink targets (r:id -> URL / slide number) now that the shapes
	// and the slide's relationships are both available. This reads only; it never
	// marks anything dirty, so an unmodified save stays byte-identical.
	s.resolveHyperlinks()
}

// rematerializeShapes rebuilds the domain shapes from the slide XML after an
// in-XML mutation (text replacement) and, where possible, copies the refreshed
// state into the pre-existing domain objects so caller-held shape pointers
// stay attached to the slide — a plain re-materialization would silently
// detach them, dropping any subsequent edits made through them.
func (s *Slide) rematerializeShapes() {
	old := s.shapeCache
	s.shapeCache = nil
	s.materializeShapes()
	if len(old) != len(s.shapeCache) {
		return
	}

	// Match old to fresh shapes per tree-kind class: relative order within a
	// class is preserved by both the sync (typed slices) and materialization,
	// even when the overall interleaving differs (type-ordered rebuilt trees
	// materialize grouped by kind).
	buckets := make(map[int][]Shape)
	for _, sh := range old {
		buckets[shapeKindClass(sh)] = append(buckets[shapeKindClass(sh)], sh)
	}
	adopted := make([]Shape, len(s.shapeCache))
	for i, fresh := range s.shapeCache {
		k := shapeKindClass(fresh)
		q := buckets[k]
		if len(q) == 0 {
			return // structural mismatch: keep the freshly materialized shapes
		}
		o := q[0]
		buckets[k] = q[1:]
		if !adoptRefreshedShape(o, fresh) {
			return
		}
		adopted[i] = o
	}
	copy(s.shapeCache, adopted)
}

// shapeKindClass maps a domain shape to the spTree child kind its node has.
func shapeKindClass(sh Shape) int {
	switch sh.(type) {
	case *TextBox, *PlaceholderShape, *AutoShape:
		return int(oxml.ChildSp)
	case *Picture, *Video, *Audio:
		return int(oxml.ChildPic)
	case *Table:
		return int(oxml.ChildGraphicFrame)
	case *GroupShape:
		return int(oxml.ChildGrpSp)
	case *Connector:
		return int(oxml.ChildCxnSp)
	}
	return -1
}

// adoptRefreshedShape copies the parts of a freshly materialized shape that a
// text replacement can change (text frames) into the pre-existing domain
// object, reporting whether the pair was compatible. Shapes without text keep
// the old object untouched, preserving state the fresh copy lacks (pending
// image data, media relationship IDs).
func adoptRefreshedShape(old, fresh Shape) bool {
	// Refresh the stable node identity: on loaded slides it is unchanged, but
	// after a full rebuild the re-materialized nodes carry renumbered ids.
	if ob, fb := baseShapeOf(old), baseShapeOf(fresh); ob != nil && fb != nil {
		ob.sourceID = fb.sourceID
	}
	switch o := old.(type) {
	case *TextBox:
		f, ok := fresh.(*TextBox)
		if !ok {
			return false
		}
		o.textFrame = f.textFrame
	case *PlaceholderShape:
		f, ok := fresh.(*PlaceholderShape)
		if !ok {
			return false
		}
		o.textFrame = f.textFrame
	case *AutoShape:
		f, ok := fresh.(*AutoShape)
		if !ok {
			return false
		}
		o.textFrame = f.textFrame
	case *Table:
		f, ok := fresh.(*Table)
		if !ok || len(o.rows) != len(f.rows) {
			return false
		}
		for i, row := range o.rows {
			if len(row.cells) != len(f.rows[i].cells) {
				return false
			}
		}
		for i, row := range o.rows {
			row.sourceTr = f.rows[i].sourceTr
			for j, cell := range row.cells {
				cell.textFrame = f.rows[i].cells[j].textFrame
				cell.sourceTc = f.rows[i].cells[j].sourceTc
			}
		}
		o.sourceFrame = f.sourceFrame
	case *Picture:
		if _, ok := fresh.(*Picture); !ok {
			return false
		}
	case *Video:
		// Media shapes materialize as *Picture; the old shape keeps its media
		// identity.
		if _, ok := fresh.(*Picture); !ok {
			return false
		}
	case *Audio:
		if _, ok := fresh.(*Picture); !ok {
			return false
		}
	case *GroupShape:
		f, ok := fresh.(*GroupShape)
		if !ok || len(o.children) != len(f.children) {
			return false
		}
		for i := range o.children {
			if !adoptRefreshedShape(o.children[i], f.children[i]) {
				return false
			}
		}
		o.sourceGrp = f.sourceGrp
		o.syncedChildren = f.syncedChildren
	case *Connector:
		f, ok := fresh.(*Connector)
		if !ok {
			return false
		}
		o.sourceCxn = f.sourceCxn
	default:
		return false
	}
	return true
}

// setShapeBackRef sets the slide back-reference on shapes that need it.
func (s *Slide) setShapeBackRef(shape Shape) {
	switch sh := shape.(type) {
	case *PlaceholderShape:
		sh.slide = s
	case *Picture:
		sh.slide = s
	}
}

// oxmlShapeToGoShape converts an oxml.Shape to the appropriate Go-level Shape type.
// The discrimination order is: Placeholder first, TextBox second, AutoShape fallback.
func oxmlShapeToGoShape(sp *oxml.Shape) Shape {
	if sp == nil {
		return nil
	}

	// Check for placeholder
	if sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil && sp.NvSpPr.NvPr.Ph != nil {
		return oxmlShapeToPlaceholder(sp)
	}

	// Check for textbox
	if sp.NvSpPr != nil && sp.NvSpPr.CNvSpPr != nil && sp.NvSpPr.CNvSpPr.TxBox {
		return oxmlShapeToTextBox(sp)
	}

	// Fallback: AutoShape
	return oxmlShapeToAutoShape(sp)
}

// oxmlShapeToPlaceholder converts an oxml.Shape with placeholder info to a PlaceholderShape.
func oxmlShapeToPlaceholder(sp *oxml.Shape) *PlaceholderShape {
	ph := &PlaceholderShape{
		phType:    PlaceholderType(sp.NvSpPr.NvPr.Ph.Type),
		idx:       sp.NvSpPr.NvPr.Ph.Idx,
		textFrame: NewTextFrame(),
	}

	if sp.NvSpPr.NvPr.Ph.Orient != "" {
		ph.orientation = PlaceholderOrientation(sp.NvSpPr.NvPr.Ph.Orient)
	}
	if sp.NvSpPr.NvPr.Ph.Sz != "" {
		ph.size = PlaceholderSize(sp.NvSpPr.NvPr.Ph.Sz)
	}

	populateBaseShapeFromOxml(&ph.BaseShape, sp.NvSpPr.CNvPr, sp.SpPr)

	if sp.TxBody != nil {
		ph.textFrame = oxmlToTextFrame(sp.TxBody)
	}

	return ph
}

// oxmlShapeToTextBox converts an oxml.Shape with TxBox=true to a TextBox.
func oxmlShapeToTextBox(sp *oxml.Shape) *TextBox {
	tb := &TextBox{
		textFrame: NewTextFrame(),
	}

	if sp.NvSpPr != nil {
		populateBaseShapeFromOxml(&tb.BaseShape, sp.NvSpPr.CNvPr, sp.SpPr)
	}

	if sp.TxBody != nil {
		tb.textFrame = oxmlToTextFrame(sp.TxBody)
	}

	return tb
}

// oxmlShapeToAutoShape converts an oxml.Shape to an AutoShape.
func oxmlShapeToAutoShape(sp *oxml.Shape) *AutoShape {
	preset := ""
	if sp.SpPr != nil && sp.SpPr.PrstGeom != nil {
		preset = sp.SpPr.PrstGeom.Prst
	}

	as := &AutoShape{
		presetGeometry: preset,
	}

	if sp.NvSpPr != nil {
		populateBaseShapeFromOxml(&as.BaseShape, sp.NvSpPr.CNvPr, sp.SpPr)
	}

	if sp.TxBody != nil {
		as.textFrame = oxmlToTextFrame(sp.TxBody)
	}

	return as
}

// oxmlPictureToGoPicture converts an oxml.Picture to a Picture.
func oxmlPictureToGoPicture(pic *oxml.Picture) *Picture {
	if pic == nil {
		return nil
	}

	p := &Picture{}

	// Name, description, and stable identity from CNvPr
	if pic.NvPicPr != nil && pic.NvPicPr.CNvPr != nil {
		p.name = pic.NvPicPr.CNvPr.Name
		p.description = pic.NvPicPr.CNvPr.Descr
		p.sourceID = pic.NvPicPr.CNvPr.Id
		if hc := pic.NvPicPr.CNvPr.HlinkClick; hc != nil && !isMediaAction(hc) {
			p.hyperlink = hyperlinkFromXML(hc)
			p.hyperlink.markDirty = func() { p.dirty = true }
		}
	}

	// A picture that backs embedded video/audio carries a media reference in its
	// non-visual properties; its blip is a poster, so it is not a real picture.
	if pic.NvPicPr != nil && pic.NvPicPr.NvPr != nil {
		nv := pic.NvPicPr.NvPr
		if nv.VideoFile != nil || nv.AudioFile != nil {
			p.isMedia = true
		}
		if nv.ExtLst != nil {
			for _, ext := range nv.ExtLst.Ext {
				if ext.Media != nil {
					p.isMedia = true
				}
			}
		}
	}

	// Position and size from SpPr
	if pic.SpPr != nil && pic.SpPr.Xfrm != nil {
		if pic.SpPr.Xfrm.Off != nil {
			p.x = dml.EMU(pic.SpPr.Xfrm.Off.X)
			p.y = dml.EMU(pic.SpPr.Xfrm.Off.Y)
		}
		if pic.SpPr.Xfrm.Ext != nil {
			p.width = dml.EMU(pic.SpPr.Xfrm.Ext.Cx)
			p.height = dml.EMU(pic.SpPr.Xfrm.Ext.Cy)
		}
	}

	// Relationship ID and cropping from BlipFill
	if pic.BlipFill != nil {
		if pic.BlipFill.Blip != nil {
			p.relID = pic.BlipFill.Blip.Embed
			if pic.BlipFill.Blip.ExtLst != nil {
				for _, ext := range pic.BlipFill.Blip.ExtLst.Ext {
					if ext != nil && ext.SvgBlip != nil {
						p.svgRelID = ext.SvgBlip.Embed
						break
					}
				}
			}
		}
		if pic.BlipFill.SrcRect != nil {
			p.cropLeft = float64(pic.BlipFill.SrcRect.L.Int32()) / 100000.0
			p.cropTop = float64(pic.BlipFill.SrcRect.T.Int32()) / 100000.0
			p.cropRight = float64(pic.BlipFill.SrcRect.R.Int32()) / 100000.0
			p.cropBottom = float64(pic.BlipFill.SrcRect.B.Int32()) / 100000.0
		}
	}

	return p
}

// oxmlGroupShapeToGoGroupShape converts an oxml.GroupShape to a GroupShape.
func oxmlGroupShapeToGoGroupShape(gs *oxml.GroupShape, slide *Slide) *GroupShape {
	if gs == nil {
		return nil
	}

	g := NewGroupShape()
	g.sourceGrp = gs
	if gs.NvGrpSpPr != nil && gs.NvGrpSpPr.CNvPr != nil {
		g.name = gs.NvGrpSpPr.CNvPr.Name
		g.sourceID = gs.NvGrpSpPr.CNvPr.Id
	}

	if gs.GrpSpPr != nil && gs.GrpSpPr.Xfrm != nil {
		xfrm := gs.GrpSpPr.Xfrm
		// A group's on-slide placement is off/ext; chOff/chExt define the child
		// coordinate space and must not be reported as the group's position.
		if xfrm.Off != nil {
			g.x = dml.EMU(xfrm.Off.X)
			g.y = dml.EMU(xfrm.Off.Y)
		}
		if xfrm.Ext != nil {
			g.width = dml.EMU(xfrm.Ext.Cx)
			g.height = dml.EMU(xfrm.Ext.Cy)
		}
	}

	for _, ref := range gs.ChildOrder() {
		switch ref.Kind {
		case oxml.ChildSp:
			if ref.Index < len(gs.Shapes) {
				if shape := oxmlShapeToGoShape(gs.Shapes[ref.Index]); shape != nil {
					slide.setShapeBackRef(shape)
					_ = g.AddChild(shape) // materialization only yields supported kinds
				}
			}
		case oxml.ChildPic:
			if ref.Index < len(gs.Pictures) {
				if pic := oxmlPictureToGoPicture(gs.Pictures[ref.Index]); pic != nil {
					slide.setShapeBackRef(pic)
					_ = g.AddChild(pic)
				}
			}
		case oxml.ChildGraphicFrame:
			if ref.Index < len(gs.GraphicFrames) {
				if tbl := oxmlGraphicFrameToGoTable(gs.GraphicFrames[ref.Index]); tbl != nil {
					_ = g.AddChild(tbl)
				}
			}
		case oxml.ChildGrpSp:
			if ref.Index < len(gs.GroupShapes) {
				if sub := oxmlGroupShapeToGoGroupShape(gs.GroupShapes[ref.Index], slide); sub != nil {
					_ = g.AddChild(sub)
				}
			}
		case oxml.ChildCxnSp:
			if ref.Index < len(gs.ConnectionShapes) {
				if cxn := oxmlCxnSpToGoConnector(gs.ConnectionShapes[ref.Index]); cxn != nil {
					slide.setShapeBackRef(cxn)
					_ = g.AddChild(cxn)
				}
			}
		}
	}

	// Everything added above came from the parsed node; only children added
	// via AddChild after this point need appending on sync.
	g.syncedChildren = len(g.children)
	g.childrenModified = false

	return g
}

// oxmlGraphicFrameToGoTable converts an oxml.GraphicFrame to a Table, if it contains a table.
// Returns nil if the graphic frame is not a table.
func oxmlGraphicFrameToGoTable(gf *oxml.GraphicFrame) *Table {
	if gf == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil {
		return nil
	}
	if gf.Graphic.GraphicData.URI != oxml.TableGraphicDataURI {
		return nil
	}
	if gf.Graphic.GraphicData.Table == nil {
		return nil
	}

	atbl := gf.Graphic.GraphicData.Table

	// Determine row/col count
	rowCount := len(atbl.Tr)
	colCount := 0
	if atbl.TblGrid != nil {
		colCount = len(atbl.TblGrid.GridCol)
	}

	tbl := NewTable(rowCount, colCount)
	tbl.sourceFrame = gf

	// Name and stable node identity
	if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil {
		tbl.name = gf.NvGraphicFramePr.CNvPr.Name
		tbl.sourceID = gf.NvGraphicFramePr.CNvPr.Id
	}

	// Position and size
	if gf.Xfrm != nil {
		if gf.Xfrm.Off != nil {
			tbl.x = dml.EMU(gf.Xfrm.Off.X)
			tbl.y = dml.EMU(gf.Xfrm.Off.Y)
		}
		if gf.Xfrm.Ext != nil {
			tbl.width = dml.EMU(gf.Xfrm.Ext.Cx)
			tbl.height = dml.EMU(gf.Xfrm.Ext.Cy)
		}
	}

	// Table properties
	if atbl.TblPr != nil {
		tbl.firstRow = atbl.TblPr.FirstRow
		tbl.firstCol = atbl.TblPr.FirstCol
		tbl.lastRow = atbl.TblPr.LastRow
		tbl.lastCol = atbl.TblPr.LastCol
		tbl.bandRow = atbl.TblPr.BandRow
		tbl.bandCol = atbl.TblPr.BandCol
		tbl.tableStyleID = atbl.TblPr.TableStyleId
	}

	// Column widths
	if atbl.TblGrid != nil {
		for i, gc := range atbl.TblGrid.GridCol {
			if i < len(tbl.colWidths) {
				tbl.colWidths[i] = dml.EMU(gc.W)
			}
		}
	}

	// Rows and cells
	for i, tr := range atbl.Tr {
		if i >= len(tbl.rows) {
			break
		}
		if tr.H != nil {
			tbl.rows[i].height = dml.EMU(*tr.H)
		}
		tbl.rows[i].sourceTr = tr

		for j, tc := range tr.Tc {
			if j >= len(tbl.rows[i].cells) {
				break
			}
			cell := tbl.rows[i].cells[j]
			cell.sourceTc = tc

			if tc.TxBody != nil {
				cell.textFrame = oxmlToTextFrame(tc.TxBody)
			}

			if tc.TcPr != nil {
				if tc.TcPr.Anchor != "" {
					cell.vertAlign = enum.VerticalAlign(tc.TcPr.Anchor)
				}
				if tc.TcPr.SolidFill != nil {
					cell.fill = oxmlToColor(tc.TcPr.SolidFill)
				}
			}

			// An ordinary cell omits rowSpan/gridSpan (they default to 1). Normalize
			// the absent 0 to 1 so a loaded cell reports the same span as one created
			// via NewTableCell — callers multiplying by span would otherwise get 0.
			cell.rowSpan = tc.RowSpan
			if cell.rowSpan == 0 {
				cell.rowSpan = 1
			}
			cell.colSpan = tc.GridSpan
			if cell.colSpan == 0 {
				cell.colSpan = 1
			}
			cell.hMerge = tc.HMerge
			cell.vMerge = tc.VMerge
		}
	}

	return tbl
}

// populateBaseShapeFromOxml fills in the BaseShape fields from CNvPr and SpPr.
func populateBaseShapeFromOxml(base *BaseShape, cnvPr *dml.CNvPr, spPr *dml.SpPr) {
	if cnvPr != nil {
		base.name = cnvPr.Name
		base.sourceID = cnvPr.Id
		if cnvPr.HlinkClick != nil && !isMediaAction(cnvPr.HlinkClick) {
			base.hyperlink = hyperlinkFromXML(cnvPr.HlinkClick)
			base.hyperlink.markDirty = func() { base.dirty = true }
		}
	}

	if spPr != nil && spPr.Xfrm != nil {
		if spPr.Xfrm.Off != nil {
			base.x = dml.EMU(spPr.Xfrm.Off.X)
			base.y = dml.EMU(spPr.Xfrm.Off.Y)
		}
		if spPr.Xfrm.Ext != nil {
			base.width = dml.EMU(spPr.Xfrm.Ext.Cx)
			base.height = dml.EMU(spPr.Xfrm.Ext.Cy)
		}
	}
}

// oxmlToTextFrame converts a dml.TxBody to a TextFrame.
func oxmlToTextFrame(txBody *dml.TxBody) *TextFrame {
	if txBody == nil {
		return NewTextFrame()
	}

	tf := &TextFrame{
		paragraphs: make([]*Paragraph, 0, len(txBody.P)),
	}

	// Body properties
	if txBody.BodyPr != nil {
		if txBody.BodyPr.Wrap != "" {
			tf.wrap = enum.TextWrapping(txBody.BodyPr.Wrap)
		}
		if txBody.BodyPr.Anchor != "" {
			tf.anchor = enum.TextAnchor(txBody.BodyPr.Anchor)
		}
		// Record whether the source carried any inset at all: without it a body
		// that inherits its insets is indistinguishable from one explicitly set
		// to zero, and Margins would report (0,0,0,0) for a body that actually
		// renders with the ~91440/45720 defaults. See TextFrame.MarginsSet.
		if txBody.BodyPr.LIns != nil {
			tf.margins.Left = dml.EMU(*txBody.BodyPr.LIns)
			tf.marginsExplicit = true
		}
		if txBody.BodyPr.TIns != nil {
			tf.margins.Top = dml.EMU(*txBody.BodyPr.TIns)
			tf.marginsExplicit = true
		}
		if txBody.BodyPr.RIns != nil {
			tf.margins.Right = dml.EMU(*txBody.BodyPr.RIns)
			tf.marginsExplicit = true
		}
		if txBody.BodyPr.BIns != nil {
			tf.margins.Bottom = dml.EMU(*txBody.BodyPr.BIns)
			tf.marginsExplicit = true
		}
		switch {
		case txBody.BodyPr.SpAutoFit != nil:
			tf.autofit = AutofitShape
		case txBody.BodyPr.NormAutofit != nil:
			tf.autofit = AutofitNormal
		case txBody.BodyPr.NoAutofit != nil:
			tf.autofit = AutofitNone
		}
	}

	// Paragraphs
	for _, p := range txBody.P {
		tf.paragraphs = append(tf.paragraphs, oxmlToParagraph(p))
	}

	return tf
}

// oxmlToParagraph converts a dml.P to a Paragraph.
func oxmlToParagraph(p *dml.P) *Paragraph {
	if p == nil {
		return NewParagraph()
	}

	para := &Paragraph{
		runs: make([]*Run, 0, len(p.R)),
	}

	// Paragraph properties
	if p.PPr != nil {
		if p.PPr.Algn != "" {
			para.alignment = enum.TextAlign(p.PPr.Algn)
		}
		if p.PPr.Lvl != nil {
			para.level = int(*p.PPr.Lvl)
		}
		if p.PPr.MarL != nil {
			marL := *p.PPr.MarL
			para.marL = &marL
		}
		if p.PPr.Indent != nil {
			indent := *p.PPr.Indent
			para.indent = &indent
		}

		// Bullet type
		if p.PPr.BuNone != nil {
			para.bulletType = BulletNone
		} else if p.PPr.BuChar != nil {
			para.bulletType = BulletChar
			para.bulletChar = p.PPr.BuChar.Char
		} else if p.PPr.BuAutoNum != nil {
			para.bulletType = BulletNumber
			para.bulletAutoNumType = AutoNumberScheme(p.PPr.BuAutoNum.Type)
			para.bulletAutoNumStartAt = p.PPr.BuAutoNum.StartAt
		}

		// Bullet styling
		if p.PPr.BuClr != nil {
			para.bulletColor = buClrToColor(p.PPr.BuClr)
		}
		if p.PPr.BuSzPct != nil {
			para.bulletSizePct = p.PPr.BuSzPct.Val.Int32()
		}
		if p.PPr.BuFont != nil {
			para.bulletFont = p.PPr.BuFont.Typeface
		}

		// Tab stops
		if p.PPr.TabLst != nil {
			for _, tab := range p.PPr.TabLst.Tab {
				if tab == nil {
					continue
				}
				ts := TabStop{Align: TabAlign(tab.Algn)}
				if tab.Pos != nil {
					ts.Position = dml.EMU(*tab.Pos)
				}
				para.tabStops = append(para.tabStops, ts)
			}
		}

		// Line spacing
		if p.PPr.LnSpc != nil && p.PPr.LnSpc.SpcPct != nil {
			para.lineSpacing = p.PPr.LnSpc.SpcPct.Val.Int32()
		}

		// Space before/after
		if p.PPr.SpcBef != nil && p.PPr.SpcBef.SpcPts != nil {
			para.spaceBefore = dml.EMU(p.PPr.SpcBef.SpcPts.Val)
		}
		if p.PPr.SpcAft != nil && p.PPr.SpcAft.SpcPts != nil {
			para.spaceAfter = dml.EMU(p.PPr.SpcAft.SpcPts.Val)
		}
	}

	// Runs
	for _, r := range p.R {
		para.runs = append(para.runs, oxmlToRun(r))
	}

	return para
}

// oxmlToRun converts a dml.R to a Run.
func oxmlToRun(r *dml.R) *Run {
	if r == nil {
		return NewRun()
	}

	run := &Run{
		text: r.T,
	}

	if r.RPr != nil {
		rpr := r.RPr

		// Font size (stored in hundredths of a point)
		if rpr.Sz > 0 {
			run.fontSize = float64(rpr.Sz) / 100.0
		}

		// Bold
		if rpr.B != nil && *rpr.B {
			run.bold = true
		}

		// Italic
		if rpr.I != nil && *rpr.I {
			run.italic = true
		}

		// Underline
		if rpr.U != "" {
			run.underline = enum.UnderlineStyle(rpr.U)
		}

		// Strikethrough
		if rpr.Strike != "" {
			run.strike = enum.StrikeStyle(rpr.Strike)
		}

		// Baseline (super/subscript)
		if rpr.Baseline != nil {
			run.baseline = rpr.Baseline.Int32()
		}

		// Font name
		if rpr.Latin != nil && rpr.Latin.Typeface != "" {
			run.fontName = rpr.Latin.Typeface
		}

		// Color
		if rpr.SolidFill != nil {
			run.color = oxmlToColor(rpr.SolidFill)
		}

		// Highlight
		if rpr.Highlight != nil {
			run.highlight = oxmlColorChoiceToColor(rpr.Highlight)
		}

		// Hyperlink (a:hlinkClick on the run properties). The URL/anchor are
		// resolved later, once slide context is available.
		if rpr.HlinkClick != nil {
			run.hyperlink = hyperlinkFromXML(rpr.HlinkClick)
			run.hyperlink.markDirty = func() { run.dirty = true }
		}
	}

	return run
}

// oxmlToColor converts a dml.SolidFill to a dml.Color.
func oxmlToColor(sf *dml.SolidFill) *dml.Color {
	if sf == nil {
		return nil
	}

	if sf.SrgbClr != nil {
		rgb, err := dml.ParseRGB(sf.SrgbClr.Val)
		if err == nil {
			c := rgb.ToColor()
			return &c
		}
	}

	if sf.SchemeClr != nil {
		tc := parseThemeColorString(sf.SchemeClr.Val)
		c := tc.ToColor()
		return &c
	}

	return nil
}

// oxmlColorChoiceToColor converts a dml.ColorChoice to a dml.Color.
func oxmlColorChoiceToColor(cc *dml.ColorChoice) *dml.Color {
	if cc == nil {
		return nil
	}

	if cc.SrgbClr != nil {
		rgb, err := dml.ParseRGB(cc.SrgbClr.Val)
		if err == nil {
			c := rgb.ToColor()
			return &c
		}
	}

	if cc.SchemeClr != nil {
		tc := parseThemeColorString(cc.SchemeClr.Val)
		c := tc.ToColor()
		return &c
	}

	return nil
}

// themeColorMap maps OOXML scheme color names to ThemeColor constants.
var themeColorMap = map[string]dml.ThemeColor{
	"dk1":      dml.ThemeColorDark1,
	"lt1":      dml.ThemeColorLight1,
	"dk2":      dml.ThemeColorDark2,
	"lt2":      dml.ThemeColorLight2,
	"accent1":  dml.ThemeColorAccent1,
	"accent2":  dml.ThemeColorAccent2,
	"accent3":  dml.ThemeColorAccent3,
	"accent4":  dml.ThemeColorAccent4,
	"accent5":  dml.ThemeColorAccent5,
	"accent6":  dml.ThemeColorAccent6,
	"hlink":    dml.ThemeColorHyperlink,
	"folHlink": dml.ThemeColorFollowedHyperlink,
}

// parseThemeColorString converts a scheme color string (e.g., "dk1", "accent1")
// to a dml.ThemeColor constant.
func parseThemeColorString(val string) dml.ThemeColor {
	if tc, ok := themeColorMap[val]; ok {
		return tc
	}
	return dml.ThemeColorDark1 // default fallback
}
