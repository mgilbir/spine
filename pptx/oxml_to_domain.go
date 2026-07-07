package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// materializeShapes converts the parsed XML shape tree into Go-level Shape objects.
// This is called eagerly when loading slides from an existing file.
// The shapes are populated for read access; shapesModified remains false so that
// the original XML is preserved during save unless the user explicitly modifies shapes.
func (s *Slide) materializeShapes() {
	if s.slideXML == nil || s.slideXML.CSld == nil || s.slideXML.CSld.SpTree == nil {
		return
	}

	spTree := s.slideXML.CSld.SpTree

	// Materialize shapes in their original z-order using childOrder if available.
	if len(spTree.ChildOrder()) > 0 {
		for _, ref := range spTree.ChildOrder() {
			switch ref.Kind {
			case oxml.ChildSp:
				if ref.Index < len(spTree.Sp) {
					if shape := oxmlShapeToGoShape(spTree.Sp[ref.Index]); shape != nil {
						s.setShapeBackRef(shape)
						s.shapes = append(s.shapes, shape)
					}
				}
			case oxml.ChildPic:
				if ref.Index < len(spTree.Pic) {
					if pic := oxmlPictureToGoPicture(spTree.Pic[ref.Index]); pic != nil {
						s.setShapeBackRef(pic)
						s.shapes = append(s.shapes, pic)
					}
				}
			case oxml.ChildGraphicFrame:
				// Tables are the only graphic frames we materialize for now
				if ref.Index < len(spTree.GraphicFrame) {
					if tbl := oxmlGraphicFrameToGoTable(spTree.GraphicFrame[ref.Index]); tbl != nil {
						s.shapes = append(s.shapes, tbl)
					}
				}
			case oxml.ChildGrpSp:
				if ref.Index < len(spTree.GrpSp) {
					if grp := oxmlGroupShapeToGoGroupShape(spTree.GrpSp[ref.Index], s); grp != nil {
						s.shapes = append(s.shapes, grp)
					}
				}
			}
		}
	} else {
		// No child order tracking — iterate typed slices in order
		for _, sp := range spTree.Sp {
			if shape := oxmlShapeToGoShape(sp); shape != nil {
				s.setShapeBackRef(shape)
				s.shapes = append(s.shapes, shape)
			}
		}
		for _, pic := range spTree.Pic {
			if p := oxmlPictureToGoPicture(pic); p != nil {
				s.setShapeBackRef(p)
				s.shapes = append(s.shapes, p)
			}
		}
		for _, gf := range spTree.GraphicFrame {
			if tbl := oxmlGraphicFrameToGoTable(gf); tbl != nil {
				s.shapes = append(s.shapes, tbl)
			}
		}
		for _, grp := range spTree.GrpSp {
			if g := oxmlGroupShapeToGoGroupShape(grp, s); g != nil {
				s.shapes = append(s.shapes, g)
			}
		}
	}

	// Everything materialized so far is already represented in the parsed
	// XML; marshal() appends only shapes added after this point.
	s.syncedShapes = len(s.shapes)
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
			p.cropLeft = float64(pic.BlipFill.SrcRect.L) / 100000.0
			p.cropTop = float64(pic.BlipFill.SrcRect.T) / 100000.0
			p.cropRight = float64(pic.BlipFill.SrcRect.R) / 100000.0
			p.cropBottom = float64(pic.BlipFill.SrcRect.B) / 100000.0
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
	if gs.NvGrpSpPr != nil && gs.NvGrpSpPr.CNvPr != nil {
		g.name = gs.NvGrpSpPr.CNvPr.Name
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
					g.AddChild(shape)
				}
			}
		case oxml.ChildPic:
			if ref.Index < len(gs.Pictures) {
				if pic := oxmlPictureToGoPicture(gs.Pictures[ref.Index]); pic != nil {
					slide.setShapeBackRef(pic)
					g.AddChild(pic)
				}
			}
		case oxml.ChildGraphicFrame:
			if ref.Index < len(gs.GraphicFrames) {
				if tbl := oxmlGraphicFrameToGoTable(gs.GraphicFrames[ref.Index]); tbl != nil {
					g.AddChild(tbl)
				}
			}
		case oxml.ChildGrpSp:
			if ref.Index < len(gs.GroupShapes) {
				if sub := oxmlGroupShapeToGoGroupShape(gs.GroupShapes[ref.Index], slide); sub != nil {
					g.AddChild(sub)
				}
			}
		}
	}

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

	// Name
	if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil {
		tbl.name = gf.NvGraphicFramePr.CNvPr.Name
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
		tbl.rows[i].height = dml.EMU(tr.H)

		for j, tc := range tr.Tc {
			if j >= len(tbl.rows[i].cells) {
				break
			}
			cell := tbl.rows[i].cells[j]

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

			cell.rowSpan = tc.RowSpan
			cell.colSpan = tc.GridSpan
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
		if txBody.BodyPr.LIns != nil {
			tf.margins.Left = dml.EMU(*txBody.BodyPr.LIns)
		}
		if txBody.BodyPr.TIns != nil {
			tf.margins.Top = dml.EMU(*txBody.BodyPr.TIns)
		}
		if txBody.BodyPr.RIns != nil {
			tf.margins.Right = dml.EMU(*txBody.BodyPr.RIns)
		}
		if txBody.BodyPr.BIns != nil {
			tf.margins.Bottom = dml.EMU(*txBody.BodyPr.BIns)
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
		runs:        make([]*Run, 0, len(p.R)),
		lineSpacing: 100000, // default
	}

	// Paragraph properties
	if p.PPr != nil {
		if p.PPr.Algn != "" {
			para.alignment = enum.TextAlign(p.PPr.Algn)
		}
		if p.PPr.Lvl != nil {
			para.level = int(*p.PPr.Lvl)
		}

		// Bullet type
		if p.PPr.BuNone != nil {
			para.bulletType = BulletNone
		} else if p.PPr.BuChar != nil {
			para.bulletType = BulletChar
			para.bulletChar = p.PPr.BuChar.Char
		} else if p.PPr.BuAutoNum != nil {
			para.bulletType = BulletNumber
		}

		// Line spacing
		if p.PPr.LnSpc != nil && p.PPr.LnSpc.SpcPct != nil {
			para.lineSpacing = int32(p.PPr.LnSpc.SpcPct.Val)
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
			run.baseline = *rpr.Baseline
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
