package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
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
	for _, shape := range s.shapeCache {
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
		return sh.dirty || sh.hasPendingProps()
	case *Audio:
		return sh.dirty || sh.hasPendingProps()
	case *Table:
		return sh.isDirty()
	case *GroupShape:
		return sh.isDirty()
	case *ChartFrame:
		return sh.dirty
	case *SmartArtFrame:
		return sh.dirty
	case *OLEObjectFrame:
		return sh.dirty
	case *Connector:
		return sh.dirty
	}
	return false
}

// clearShapeDirt resets the modification flags on every shape after a sync
// flushed them into the XML.
func (s *Slide) clearShapeDirt() {
	for _, shape := range s.shapeCache {
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
		sh.timingDirty, sh.posterDirty = false, false
	case *Audio:
		sh.dirty = false
		sh.timingDirty, sh.posterDirty = false, false
	case *Table:
		sh.clearDirty()
	case *GroupShape:
		sh.dirty = false
		sh.childrenModified = false
		sh.removedChildIDs = nil
		for _, child := range sh.children {
			clearShapeDirty(child)
		}
	case *ChartFrame:
		sh.dirty = false
	case *SmartArtFrame:
		sh.dirty = false
	case *OLEObjectFrame:
		sh.dirty = false
	case *Connector:
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
		shape := s.shapeCache[i]
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
				s.flushMediaShapeProps(spTree.Pic[ref.Index], shape)
			}
		case oxml.ChildGraphicFrame:
			if ref.Index < len(spTree.GraphicFrame) {
				updateGraphicFrameNode(spTree.GraphicFrame[ref.Index], shape)
			}
		case oxml.ChildGrpSp:
			if ref.Index < len(spTree.GrpSp) {
				s.updateGroupNode(spTree.GrpSp[ref.Index], shape, spTree)
			}
		case oxml.ChildCxnSp:
			if ref.Index < len(spTree.CxnSp) {
				if c, ok := shape.(*Connector); ok {
					updateConnectorNode(spTree.CxnSp[ref.Index], c)
				}
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
		if sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil && base.hyperlink != nil {
			sp.NvSpPr.CNvPr.HlinkClick = hyperlinkToXML(base.hyperlink)
		}
		if sp.SpPr == nil {
			sp.SpPr = &dml.SpPr{}
		}
		updateXfrm(sp.SpPr, base)
		if style != nil {
			applyShapeStyle(sp.SpPr, style)
		}
		// Flush placeholder attributes (type/orient/idx/sz) onto the parsed
		// p:ph so SetOrientation/SetIndex/SetPlaceholderSize reach the XML on a
		// materialized placeholder (C309). The modeled fields are authoritative
		// over the captured attr list, so unmodeled attrs on p:ph survive.
		if ph, ok := shape.(*PlaceholderShape); ok {
			flushPlaceholderAttrs(sp, ph)
		}
	}
	// A furniture placeholder set to an auto field (slide number / date) owns
	// its whole text body: replace it with the field body rather than flushing
	// text-frame edits.
	if ph, ok := shape.(*PlaceholderShape); ok && ph.fieldType != "" {
		sp.TxBody = fieldTextBody(ph.fieldType, ph.fieldText)
		return
	}
	updateTxBody(&sp.TxBody, tf)
}

// dropCapturedAttrs removes the named attributes from a captured attribute
// list, returning the remainder.
//
// ReplayCapturedAttrs replays any captured attribute the model does not match,
// which is what makes an unmodeled attribute and an explicit zero survive — and
// simultaneously makes a modeled value impossible to *clear*, because
// omitempty suppresses the zero the setter just wrote and replay then restores
// the source's value (audit tension T-D). A setter that owns an attribute must
// therefore drop it from the capture: after that, "modeled wins" holds even
// when the modeled value is a zero. Attributes not named here are untouched, so
// the fidelity guarantee for everything the model does not represent is intact.
func dropCapturedAttrs(captured []xmlb.RootAttr, names ...string) []xmlb.RootAttr {
	if len(captured) == 0 {
		return captured
	}
	drop := make(map[string]bool, len(names))
	for _, n := range names {
		drop[n] = true
	}
	out := captured[:0]
	for _, a := range captured {
		if !a.IsNS && a.Prefix == "" && drop[a.LocalName] {
			continue
		}
		out = append(out, a)
	}
	return out
}

// flushPlaceholderAttrs writes the modeled placeholder attributes (type,
// orient, idx, sz) into the parsed p:ph node, creating the nvPr/ph chain when
// the shape had none. Unmodeled attributes on p:ph survive the flush.
//
// The four modeled attributes are dropped from the capture first: p:ph@idx is
// omitempty, so SetIndex(0) used to write a zero that was suppressed and then
// replaced by the source's idx="3" on replay — the setter was a silent no-op
// (C585). The same applied to clearing type/orient/sz.
func flushPlaceholderAttrs(sp *oxml.Shape, ph *PlaceholderShape) {
	if sp.NvSpPr == nil {
		return
	}
	if sp.NvSpPr.NvPr == nil {
		sp.NvSpPr.NvPr = &oxml.NvPr{}
	}
	if sp.NvSpPr.NvPr.Ph == nil {
		sp.NvSpPr.NvPr.Ph = &oxml.Placeholder{}
	}
	p := sp.NvSpPr.NvPr.Ph
	p.CapturedAttrs = dropCapturedAttrs(p.CapturedAttrs, "type", "orient", "sz", "idx")
	p.Type = string(ph.phType)
	p.Orient = string(ph.orientation)
	p.Sz = string(ph.size)
	p.Idx = ph.idx
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
		// When the caller only edited existing paragraphs/runs in place (no
		// paragraph or run added or removed), patch the parsed a:p nodes so
		// content the domain model does not represent — a:br, a:fld,
		// endParaRPr, the interleaved child order, and unmodeled attributes on
		// untouched runs — survives the edit. A structural rewrite
		// (SetText/AddParagraph) sets contentDirty and falls back to
		// regenerating the paragraph list from the domain model, which then
		// owns it.
		if tf.contentDirty || !patchParagraphsInPlace(body.P, tf.paragraphs) {
			ps := make([]*dml.P, 0, len(tf.paragraphs))
			for _, para := range tf.paragraphs {
				ps = append(ps, paragraphToOxml(para))
			}
			if len(ps) == 0 {
				ps = append(ps, &dml.P{})
			}
			body.P = ps
		}
	}
	if tf.bodyDirty {
		if body.BodyPr == nil {
			body.BodyPr = &dml.BodyPr{}
		}
		bp := body.BodyPr
		bp.Wrap = string(tf.wrap)
		bp.Anchor = string(tf.anchor)
		// Only rewrite the four insets when the caller explicitly set them. A
		// parsed body without inset attributes materializes zero-value margins;
		// writing those would replace the inherited defaults (~91440/45720)
		// with zeros and shift the text. A parsed body that already carried
		// insets keeps them here untouched.
		if tf.marginsDirty {
			l := int64(tf.margins.Left)
			t := int64(tf.margins.Top)
			r := int64(tf.margins.Right)
			b := int64(tf.margins.Bottom)
			bp.LIns, bp.TIns, bp.RIns, bp.BIns = &l, &t, &r, &b
		}
		// Only rewrite the autofit child when the caller explicitly changed it,
		// so a parsed a:normAutofit (with its font-scale attributes) survives an
		// anchor/wrap/margin edit untouched.
		if tf.autofitDirty {
			applyAutofit(bp, tf.autofit)
		}
	}
}

// patchParagraphsInPlace flushes in-place paragraph and run edits into the
// parsed a:p nodes without regenerating them, so content the domain model does
// not represent — a:br, a:fld, endParaRPr, the interleaved child order, and
// unmodeled attributes on untouched runs — is preserved. It reports false,
// leaving the nodes untouched, when the domain and parsed structure diverge (a
// paragraph or run was added or removed), so the caller falls back to a full
// rebuild from the domain model.
func patchParagraphsInPlace(parsed []*dml.P, paras []*Paragraph) bool {
	if len(parsed) != len(paras) {
		return false
	}
	for k, dp := range paras {
		if parsed[k] == nil || len(dp.runs) != len(parsed[k].R) {
			return false
		}
	}
	for k, dp := range paras {
		pnode := parsed[k]
		if dp.dirty {
			patchParagraphPropsInPlace(pnode, dp)
		}
		for i, dr := range dp.runs {
			if dr.dirty {
				patchRunInPlace(pnode.R[i], dr)
			}
		}
	}
	return true
}

// patchRunInPlace overlays a dirty run's explicitly-set properties onto its
// parsed a:r, leaving everything else as parsed.
//
// Regenerating the run from the domain model instead (the pre-C380 behavior)
// narrowed it to the modeled subset: a run styled "Accent1, Lighter 40%"
// (schemeClr accent1 + lumMod 60000 + lumOff 40000) came back as a bare
// accent1 — the text visibly changed color on screen — and lang, spc, kern,
// cap and the rPr effect lists went with it, after nothing more than a
// SetText. Only what the caller actually assigned is written here, so an
// untouched property keeps its source form (audit tension T-B, "patch don't
// regenerate").
func patchRunInPlace(node *dml.R, r *Run) {
	if node == nil || r == nil {
		return
	}
	if r.isSet(runPropText) {
		node.T = r.text
	}
	const rprProps = runPropFont | runPropSize | runPropBold | runPropItalic |
		runPropUnderline | runPropStrike | runPropColor | runPropHighlight |
		runPropBaseline | runPropHyperlink
	if r.setProps&rprProps == 0 {
		return
	}
	if node.RPr == nil {
		node.RPr = &dml.RPr{}
	}
	rpr := node.RPr
	if r.isSet(runPropSize) {
		rpr.Sz = int32(r.fontSize * 100) // points -> hundredths
	}
	if r.isSet(runPropBold) {
		// Written through a pointer so an explicit SetBold(false) emits b="0"
		// rather than being suppressed as a zero value and re-inheriting bold
		// from the placeholder (C518).
		b := r.bold
		rpr.B = &b
	}
	if r.isSet(runPropItalic) {
		i := r.italic
		rpr.I = &i
	}
	if r.isSet(runPropUnderline) {
		rpr.U = string(r.underline)
	}
	if r.isSet(runPropStrike) {
		rpr.Strike = string(r.strike)
	}
	if r.isSet(runPropBaseline) {
		baseline := r.baseline
		rpr.Baseline = &baseline
	}
	if r.isSet(runPropFont) {
		if r.fontName == "" {
			rpr.Latin = nil
		} else {
			rpr.Latin = &dml.TextFont{Typeface: r.fontName}
		}
	}
	if r.isSet(runPropColor) {
		rpr.SolidFill = colorToOxml(r.color)
	}
	if r.isSet(runPropHighlight) {
		rpr.Highlight = colorToColorChoiceOxml(r.highlight)
	}
	if r.isSet(runPropHyperlink) {
		rpr.HlinkClick = hyperlinkToXML(r.hyperlink)
	}
}

// patchParagraphPropsInPlace overlays a dirty paragraph's explicitly-set
// properties onto its parsed a:pPr, leaving everything else as parsed.
//
// Replacing the whole a:pPr with paragraphToOxml's output (the pre-C517
// behavior) dropped defRPr, rtl, fontAlgn, defTabSz and buSzPts from any
// paragraph the caller merely realigned, and emitted lvl="0" on a paragraph
// that never carried one.
func patchParagraphPropsInPlace(node *dml.P, p *Paragraph) {
	if node == nil || p == nil || p.setProps == 0 {
		return
	}
	if node.PPr == nil {
		node.PPr = &dml.PPr{}
	}
	pp := node.PPr
	if p.isSet(paraPropAlign) {
		pp.Algn = string(p.alignment)
	}
	if p.isSet(paraPropLevel) {
		lvl := int32(p.level)
		pp.Lvl = &lvl
	}
	if p.isSet(paraPropMarginLeft) {
		pp.MarL = p.marL
	}
	if p.isSet(paraPropIndent) {
		pp.Indent = p.indent
	}
	if p.isSet(paraPropBulletColor) {
		// a:buClr and a:buClrTx are an exclusive choice.
		pp.BuClr, pp.BuClrTx = colorToBuClr(p.bulletColor), nil
	}
	if p.isSet(paraPropBulletSize) {
		// a:buSzPct, a:buSzPts and a:buSzTx are an exclusive choice; setting a
		// percentage clears the point-size and follow-text forms.
		pp.BuSzPct, pp.BuSzPts, pp.BuSzTx = nil, nil, nil
		if p.bulletSizePct != 0 {
			pp.BuSzPct = &dml.BuSzPct{Val: dml.NewPercentage(p.bulletSizePct)}
		}
	}
	if p.isSet(paraPropBulletFont) {
		// a:buFont and a:buFontTx are an exclusive choice.
		pp.BuFont, pp.BuFontTx = nil, nil
		if p.bulletFont != "" {
			pp.BuFont = &dml.BuFont{Typeface: p.bulletFont}
		}
	}
	if p.isSet(paraPropBullet | paraPropBulletChar | paraPropBulletAutoNum) {
		applyBulletKind(pp, p)
	}
	if p.isSet(paraPropTabStops) {
		pp.TabLst = tabStopsToOxml(p.tabStops)
	}
	if p.isSet(paraPropLineSpacing) {
		// 0 is documented as "restore inheritance", so it clears the element.
		pp.LnSpc = nil
		if p.lineSpacing != 0 {
			pp.LnSpc = &dml.LnSpc{SpcPct: &dml.SpcPct{Val: dml.NewPercentage(p.lineSpacing)}}
		}
	}
	if p.isSet(paraPropSpaceBefore) {
		pp.SpcBef = &dml.SpcBef{SpcPts: &dml.SpcPts{Val: int32(p.spaceBefore)}}
	}
	if p.isSet(paraPropSpaceAfter) {
		pp.SpcAft = &dml.SpcAft{SpcPts: &dml.SpcPts{Val: int32(p.spaceAfter)}}
	}
}

// applyBulletKind writes the paragraph's bullet kind into pPr, clearing the
// other kinds of the a:buNone/a:buAutoNum/a:buChar/a:buBlip exclusive choice so
// a switch between kinds does not leave two of them behind.
func applyBulletKind(pp *dml.PPr, p *Paragraph) {
	pp.BuNone, pp.BuAutoNum, pp.BuChar, pp.BuBlip = nil, nil, nil, nil
	switch p.bulletType {
	case BulletNone:
		pp.BuNone = &dml.BuNone{}
	case BulletChar:
		pp.BuChar = &dml.BuChar{Char: p.bulletChar}
	case BulletNumber, BulletAuto:
		pp.BuAutoNum = &dml.BuAutoNum{Type: autoNumScheme(p), StartAt: p.bulletAutoNumStartAt}
	}
	// BulletInherit leaves all four cleared, restoring inheritance.
}

// autoNumScheme returns the paragraph's auto-numbering scheme, defaulting to
// arabicPeriod when none was chosen.
func autoNumScheme(p *Paragraph) string {
	if s := string(p.bulletAutoNumType); s != "" {
		return s
	}
	return "arabicPeriod"
}

// tabStopsToOxml converts the domain tab stops to an a:tabLst, or nil when
// there are none (which clears a parsed list).
func tabStopsToOxml(stops []TabStop) *dml.TabLst {
	if len(stops) == 0 {
		return nil
	}
	tabLst := &dml.TabLst{Tab: make([]*dml.Tab, 0, len(stops))}
	for _, ts := range stops {
		pos := int32(ts.Position)
		algn := ts.Align
		if algn == "" {
			algn = TabAlignLeft
		}
		tabLst.Tab = append(tabLst.Tab, &dml.Tab{Pos: &pos, Algn: string(algn)})
	}
	return tabLst
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
		if base.hyperlink != nil {
			pic.NvPicPr.CNvPr.HlinkClick = hyperlinkToXML(base.hyperlink)
		}
	}
	if pic.SpPr == nil {
		pic.SpPr = &dml.SpPr{}
	}
	updateXfrm(pic.SpPr, base)
	if domainPic != nil && pic.BlipFill != nil {
		if domainPic.cropLeft > 0 || domainPic.cropTop > 0 || domainPic.cropRight > 0 || domainPic.cropBottom > 0 {
			pic.BlipFill.SrcRect = &dml.SrcRect{
				L: dml.NewPercentage(int32(domainPic.cropLeft * 100000)),
				T: dml.NewPercentage(int32(domainPic.cropTop * 100000)),
				R: dml.NewPercentage(int32(domainPic.cropRight * 100000)),
				B: dml.NewPercentage(int32(domainPic.cropBottom * 100000)),
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
	if cf, ok := shape.(*ChartFrame); ok {
		if !cf.dirty {
			return
		}
		if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil && cf.name != "" {
			gf.NvGraphicFramePr.CNvPr.Name = cf.name
		}
		if gf.Xfrm == nil {
			gf.Xfrm = &dml.Xfrm{}
		}
		gf.Xfrm.Off = &dml.OffXML{X: int64(cf.x), Y: int64(cf.y)}
		gf.Xfrm.Ext = &dml.ExtXML{Cx: int64(cf.width), Cy: int64(cf.height)}
		return
	}
	if sf, ok := shape.(*SmartArtFrame); ok {
		if !sf.dirty {
			return
		}
		if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil && sf.name != "" {
			gf.NvGraphicFramePr.CNvPr.Name = sf.name
		}
		if gf.Xfrm == nil {
			gf.Xfrm = &dml.Xfrm{}
		}
		gf.Xfrm.Off = &dml.OffXML{X: int64(sf.x), Y: int64(sf.y)}
		gf.Xfrm.Ext = &dml.ExtXML{Cx: int64(sf.width), Cy: int64(sf.height)}
		return
	}
	if of, ok := shape.(*OLEObjectFrame); ok {
		if !of.dirty {
			return
		}
		if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil && of.name != "" {
			gf.NvGraphicFramePr.CNvPr.Name = of.name
		}
		if gf.Xfrm == nil {
			gf.Xfrm = &dml.Xfrm{}
		}
		gf.Xfrm.Off = &dml.OffXML{X: int64(of.x), Y: int64(of.y)}
		gf.Xfrm.Ext = &dml.ExtXML{Cx: int64(of.width), Cy: int64(of.height)}
		return
	}
	tbl, ok := shape.(*Table)
	if !ok {
		return
	}
	if tbl.dirty {
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
		// Mark gridSpan/rowSpan covered cells as merge-continuation cells before
		// deciding how to flush, so the in-place patch path picks them up too
		// (they become dirty) and the emitted grid stays valid (C310).
		tbl.normalizeMergeCells()
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
		// All six banding/heading flags are omitempty booleans the domain model
		// mirrors, so clearing one wrote a false that was suppressed and then
		// replaced by the source's ="1" on replay: SetFirstRow(false) and its
		// five siblings were silent no-ops on any parsed table (C583). Dropping
		// them from the capture makes the modeled value authoritative in both
		// directions. rtl is not modeled here, so it stays captured.
		pr.CapturedAttrs = dropCapturedAttrs(pr.CapturedAttrs,
			"firstRow", "firstCol", "lastRow", "lastCol", "bandRow", "bandCol")
		pr.FirstRow, pr.FirstCol = t.firstRow, t.firstCol
		pr.LastRow, pr.LastCol = t.lastRow, t.lastCol
		pr.BandRow, pr.BandCol = t.bandRow, t.bandCol
		pr.TableStyleId = t.tableStyleID
		for i, gc := range atbl.TblGrid.GridCol {
			gc.W = int64(t.colWidths[i])
		}
	}
	for i, row := range t.rows {
		tr := atbl.Tr[i]
		if row.dirty {
			rowH := int64(row.height)
			tr.H = &rowH
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
	applyCellMargins(pr, cell)
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

// applyCellMargins writes explicit cell text insets into the parsed tcPr, or
// clears them on a ClearMargins. When neither was set the parsed insets are
// left untouched, so a cell edited for an unrelated reason keeps its margins.
func applyCellMargins(pr *oxml.ATcPr, cell *TableCell) {
	switch {
	case cell.margins != nil:
		l, t := int64(cell.margins.left), int64(cell.margins.top)
		r, b := int64(cell.margins.right), int64(cell.margins.bottom)
		pr.MarL, pr.MarT, pr.MarR, pr.MarB = &l, &t, &r, &b
	case cell.marginsCleared:
		pr.MarL, pr.MarT, pr.MarR, pr.MarB = nil, nil, nil, nil
	}
}

// updateGroupNode flushes a dirty group into its parsed p:grpSp node: the
// group's own name and placement (the child coordinate space chOff/chExt is
// preserved), removals of children, in-place edits to children, and appends
// of children added via AddChild. Children are matched to their nodes by
// cNvPr id — slice indices shift as siblings come and go. Nested groups are
// flushed recursively.
func (s *Slide) updateGroupNode(gs *oxml.GroupShape, shape Shape, spTree *oxml.ShapeTree) {
	grp, ok := shape.(*GroupShape)
	if !ok {
		return
	}
	if grp.dirty {
		if gs.NvGrpSpPr != nil && gs.NvGrpSpPr.CNvPr != nil && grp.name != "" {
			gs.NvGrpSpPr.CNvPr.Name = grp.name
		}
		if gs.GrpSpPr != nil && gs.GrpSpPr.Xfrm != nil {
			gs.GrpSpPr.Xfrm.Off = &dml.OffXML{X: int64(grp.x), Y: int64(grp.y)}
			gs.GrpSpPr.Xfrm.Ext = &dml.ExtXML{Cx: int64(grp.width), Cy: int64(grp.height)}
		}
	}

	// Apply removals surgically, matched by cNvPr id.
	if len(grp.removedChildIDs) > 0 {
		removeGroupChildrenByID(gs, grp.removedChildIDs)
		grp.removedChildIDs = nil
	}

	// Flush dirty synced children by locating their nodes inside this grpSp.
	n := grp.syncedChildren
	if n > len(grp.children) {
		n = len(grp.children)
	}
	for i := 0; i < n; i++ {
		if child := grp.children[i]; shapeDirty(child) {
			s.flushGroupChild(gs, child, spTree)
		}
	}

	// Append children added via AddChild, with slide-wide unique ids.
	if len(grp.children) > n {
		id := spTree.MaxShapeID()
		alloc := func() uint32 { id++; return id }
		for _, child := range grp.children[n:] {
			s.appendGroupChild(gs, child, alloc)
		}
	}
	grp.syncedChildren = len(grp.children)
	grp.childrenModified = false
}

// flushGroupChild updates the parsed node of a dirty group child in place,
// locating it among the group's children of the matching kind by cNvPr id.
// Children without a recorded id (never written) are skipped — they are
// handled by the append path.
func (s *Slide) flushGroupChild(gs *oxml.GroupShape, child Shape, spTree *oxml.ShapeTree) {
	base := baseShapeOf(child)
	if base == nil || base.sourceID == 0 {
		return
	}
	switch sh := child.(type) {
	case *TextBox, *PlaceholderShape, *AutoShape:
		for _, sp := range gs.Shapes {
			if sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil && sp.NvSpPr.CNvPr.Id == base.sourceID {
				updateShapeNode(sp, child)
				return
			}
		}
	case *Picture, *Video, *Audio:
		for _, pic := range gs.Pictures {
			if pic.NvPicPr != nil && pic.NvPicPr.CNvPr != nil && pic.NvPicPr.CNvPr.Id == base.sourceID {
				updatePictureNode(pic, child)
				s.flushMediaShapeProps(pic, child)
				return
			}
		}
	case *Table:
		for _, gf := range gs.GraphicFrames {
			if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil && gf.NvGraphicFramePr.CNvPr.Id == base.sourceID {
				updateGraphicFrameNode(gf, sh)
				return
			}
		}
	case *GroupShape:
		for _, sub := range gs.GroupShapes {
			if sub.NvGrpSpPr != nil && sub.NvGrpSpPr.CNvPr != nil && sub.NvGrpSpPr.CNvPr.Id == base.sourceID {
				s.updateGroupNode(sub, sh, spTree)
				return
			}
		}
	case *Connector:
		for _, cs := range gs.ConnectionShapes {
			if cs.NvCxnSpPr != nil && cs.NvCxnSpPr.CNvPr != nil && cs.NvCxnSpPr.CNvPr.Id == base.sourceID {
				updateConnectorNode(cs, sh)
				return
			}
		}
	}
}

// removeGroupChildrenByID deletes the group children whose cNvPr ids are in
// ids, preserving all other children in order.
func removeGroupChildrenByID(gs *oxml.GroupShape, ids []uint32) {
	var refs []oxml.ChildRef
	for _, id := range ids {
		if ref, ok := groupChildRefByID(gs, id); ok {
			refs = append(refs, ref)
		}
	}
	gs.RemoveChildren(refs)
}

// groupChildRefByID locates a direct child of the group by its cNvPr id.
func groupChildRefByID(gs *oxml.GroupShape, id uint32) (oxml.ChildRef, bool) {
	for i, sp := range gs.Shapes {
		if sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil && sp.NvSpPr.CNvPr.Id == id {
			return oxml.ChildRef{Kind: oxml.ChildSp, Index: i}, true
		}
	}
	for i, pic := range gs.Pictures {
		if pic.NvPicPr != nil && pic.NvPicPr.CNvPr != nil && pic.NvPicPr.CNvPr.Id == id {
			return oxml.ChildRef{Kind: oxml.ChildPic, Index: i}, true
		}
	}
	for i, gf := range gs.GraphicFrames {
		if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil && gf.NvGraphicFramePr.CNvPr.Id == id {
			return oxml.ChildRef{Kind: oxml.ChildGraphicFrame, Index: i}, true
		}
	}
	for i, sub := range gs.GroupShapes {
		if sub.NvGrpSpPr != nil && sub.NvGrpSpPr.CNvPr != nil && sub.NvGrpSpPr.CNvPr.Id == id {
			return oxml.ChildRef{Kind: oxml.ChildGrpSp, Index: i}, true
		}
	}
	for i, cs := range gs.ConnectionShapes {
		if cs.NvCxnSpPr != nil && cs.NvCxnSpPr.CNvPr != nil && cs.NvCxnSpPr.CNvPr.Id == id {
			return oxml.ChildRef{Kind: oxml.ChildCxnSp, Index: i}, true
		}
	}
	return oxml.ChildRef{Index: -1}, false
}

// appendGroupChild marshals a child added via GroupShape.AddChild into the
// parsed p:grpSp, assigning it the next slide-wide unique id and recording it
// as the child's node identity for later in-place edits and removals.
func (s *Slide) appendGroupChild(gs *oxml.GroupShape, child Shape, alloc func() uint32) {
	switch sh := child.(type) {
	case *TextBox:
		id := alloc()
		gs.AppendSp(textBoxToOxml(sh, id))
		sh.sourceID = id
	case *PlaceholderShape:
		id := alloc()
		gs.AppendSp(placeholderToOxml(sh, id))
		sh.sourceID = id
	case *AutoShape:
		id := alloc()
		gs.AppendSp(autoShapeToOxml(sh, id))
		sh.sourceID = id
	case *Table:
		id := alloc()
		gf := tableToOxml(sh, id)
		gs.AppendGraphicFrame(gf)
		sh.sourceFrame = gf
		sh.sourceID = id
	case *Picture:
		id := alloc()
		if len(sh.imageData) > 0 {
			sh.relID = s.embedImageData(sh.imageData, sh.contentType)
			sh.imageData = nil
			sh.imagePath = ""
		}
		pic := pictureToOxml(sh, id)
		if len(sh.svgData) > 0 {
			sh.svgRelID = s.embedImageData(sh.svgData, sh.svgContentType)
			setBlipSVGExtension(pic.BlipFill.Blip, sh.svgRelID)
			sh.svgData = nil
			sh.svgContentType = ""
		}
		gs.AppendPic(pic)
		sh.sourceID = id
	case *Video:
		id := alloc()
		gs.AppendPic(s.buildMediaPic(&sh.mediaShape, id, mediaVideo))
		sh.sourceID = id
	case *Audio:
		id := alloc()
		gs.AppendPic(s.buildMediaPic(&sh.mediaShape, id, mediaAudio))
		sh.sourceID = id
	case *Connector:
		id := alloc()
		gs.AppendCxnSp(connectorToOxml(sh, id))
		sh.sourceID = id
	case *GroupShape:
		gs.AppendGrpSp(s.buildGroupNode(sh, alloc))
	}
}

// buildGroupNode marshals an API-created GroupShape (added to a loaded group
// via AddChild) into a p:grpSp node, recursing into its children. The child
// coordinate space is seeded from the group's own frame.
func (s *Slide) buildGroupNode(grp *GroupShape, alloc func() uint32) *oxml.GroupShape {
	name := grp.Name()
	if name == "" {
		name = "Group"
	}
	id := alloc()
	node := &oxml.GroupShape{
		NvGrpSpPr: &oxml.NvGrpSpPr{
			CNvPr:      &dml.CNvPr{Id: id, Name: name},
			CNvGrpSpPr: &dml.CNvGrpSpPr{},
			NvPr:       &oxml.NvPr{},
		},
		GrpSpPr: &oxml.GrpSpPr{Xfrm: &dml.GrpXfrm{
			Off:   &dml.OffXML{X: int64(grp.x), Y: int64(grp.y)},
			Ext:   &dml.ExtXML{Cx: int64(grp.width), Cy: int64(grp.height)},
			ChOff: &dml.OffXML{X: int64(grp.x), Y: int64(grp.y)},
			ChExt: &dml.ExtXML{Cx: int64(grp.width), Cy: int64(grp.height)},
		}},
	}
	grp.sourceID = id
	grp.sourceGrp = node
	for _, child := range grp.children {
		s.appendGroupChild(node, child, alloc)
	}
	grp.syncedChildren = len(grp.children)
	grp.childrenModified = false
	return node
}
