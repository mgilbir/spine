package pptx

import (
	"encoding/xml"
	"fmt"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Slide represents a slide in a presentation.
type Slide struct {
	presentation   *Presentation
	layout         *SlideLayout
	partName       string
	slideXML       *oxml.Slide
	index          int
	id             uint32
	relID          string
	shapes         []Shape
	shapesModified bool // true if shapes changed via Go API

	// syncedShapes counts the leading shapes already represented in slideXML
	// (the parsed shapes of a loaded slide). When only appends happened,
	// marshal() syncs just the new shapes into the parsed tree instead of
	// rebuilding it — a rebuild drops slide content the domain model cannot
	// represent (group shapes, connectors) and re-numbers every shape.
	syncedShapes int
	// forceShapeRebuild is set by structural edits the surgical/append sync
	// cannot express, forcing a full rebuild from the domain model.
	forceShapeRebuild bool
	// shapeRefs records, for each materialized (parsed) shape in shapes, the
	// child reference in the parsed shape tree it came from. It stays aligned
	// with shapes[:syncedShapes] so a removed shape can be deleted surgically.
	shapeRefs []oxml.ChildRef
	// removedRefs collects the parsed child references removed via RemoveShape,
	// applied at sync time by deleting just those nodes (preserving everything
	// else, including content the domain model cannot represent).
	removedRefs []oxml.ChildRef
	// autoplayMedia records the shape ids of auto-play video/audio added to the
	// slide, collected during the shape sync and used to build the timing tree.
	autoplayMedia []mediaTimingRef
}

// Index returns the 0-based index of the slide in the presentation.
func (s *Slide) Index() int {
	return s.index
}

// Name returns the name of the slide, if any.
func (s *Slide) Name() string {
	if s.slideXML != nil && s.slideXML.CSld != nil {
		return s.slideXML.CSld.Name
	}
	return ""
}

// SetName sets the name of the slide.
func (s *Slide) SetName(name string) {
	if s.slideXML == nil {
		s.slideXML = newSlideXML()
	}
	if s.slideXML.CSld == nil {
		s.slideXML.CSld = &oxml.CommonSlideData{}
	}
	s.slideXML.CSld.Name = name
	s.shapesModified = true
}

// Layout returns the slide layout, or nil if none is set.
func (s *Slide) Layout() *SlideLayout {
	return s.layout
}

// Shapes returns all shapes on the slide.
func (s *Slide) Shapes() []Shape {
	return s.shapes
}

// AddShape adds a shape to the slide.
func (s *Slide) AddShape(shape Shape) {
	s.setShapeBackRef(shape)
	s.shapes = append(s.shapes, shape)
	s.shapesModified = true
}

// RemoveShape removes a shape from the slide. On a slide loaded from a file the
// removed shape's node is deleted surgically from the parsed tree on save, so
// other content (including group shapes, connectors, and other kinds the domain
// model does not represent) is preserved.
func (s *Slide) RemoveShape(shape Shape) {
	for i, sh := range s.shapes {
		if sh != shape {
			continue
		}
		switch {
		case i < s.syncedShapes && i < len(s.shapeRefs):
			// A synced shape: mark its source node for surgical deletion. A
			// sentinel ref (Index -1) means the shape never made it into the
			// tree, so there is nothing to delete there.
			if ref := s.shapeRefs[i]; ref.Index >= 0 {
				s.removedRefs = append(s.removedRefs, ref)
			}
			s.shapeRefs = append(s.shapeRefs[:i], s.shapeRefs[i+1:]...)
			s.syncedShapes--
		case i < s.syncedShapes:
			// Parsed shape without a tracked source (no child-order info):
			// fall back to a full rebuild.
			s.forceShapeRebuild = true
		}
		// Shapes added after the last sync were never written; dropping them
		// from the slice is sufficient.
		s.shapes = append(s.shapes[:i], s.shapes[i+1:]...)
		s.shapesModified = true
		return
	}
}

// AddTextBox adds a text box shape to the slide.
func (s *Slide) AddTextBox() *TextBox {
	tb := NewTextBox()
	s.AddShape(tb)
	return tb
}

// AddPicture adds a picture shape to the slide, reading the image from
// imagePath. It returns an error if the file cannot be read (previously the
// path was stored without reading it, so a nonexistent file returned no error
// and produced a blip with no image reference). The image data is embedded as a
// media part when the presentation is saved.
func (s *Slide) AddPicture(imagePath string) (*Picture, error) {
	pic := NewPicture()
	if err := pic.SetImage(imagePath); err != nil {
		return nil, err
	}
	s.AddShape(pic)
	return pic, nil
}

// AddTable adds a table to the slide.
func (s *Slide) AddTable(rows, cols int) *Table {
	table := NewTable(rows, cols)
	s.AddShape(table)
	return table
}

// AddVideo embeds a video in the slide from raw media bytes and their content
// type (e.g. "video/mp4"). The video is stored inside the .pptx and plays on
// click in PowerPoint. Set a poster image with Video.SetPoster; otherwise a
// placeholder preview is generated on save.
func (s *Slide) AddVideo(data []byte, contentType string) *Video {
	v := NewVideo(data, contentType)
	s.AddShape(v)
	return v
}

// AddAudio embeds an audio clip in the slide from raw media bytes and their
// content type (e.g. "audio/mpeg"). Set an icon image with Audio.SetPoster;
// otherwise a placeholder is generated on save.
func (s *Slide) AddAudio(data []byte, contentType string) *Audio {
	a := NewAudio(data, contentType)
	s.AddShape(a)
	return a
}

// Placeholders returns all placeholder shapes on the slide.
func (s *Slide) Placeholders() []*PlaceholderShape {
	var placeholders []*PlaceholderShape
	for _, shape := range s.shapes {
		if ph, ok := shape.(*PlaceholderShape); ok {
			placeholders = append(placeholders, ph)
		}
	}
	return placeholders
}

// GetPlaceholder returns the placeholder with the specified type, or nil.
func (s *Slide) GetPlaceholder(phType PlaceholderType) *PlaceholderShape {
	for _, shape := range s.shapes {
		if ph, ok := shape.(*PlaceholderShape); ok {
			if ph.PlaceholderType() == phType {
				return ph
			}
		}
	}
	return nil
}

// ShapeByName returns the first shape with the given name, or nil if not found.
func (s *Slide) ShapeByName(name string) Shape {
	for _, shape := range s.shapes {
		if found := shapeByName(shape, name); found != nil {
			return found
		}
	}
	return nil
}

func shapeByName(shape Shape, name string) Shape {
	if shape.Name() == name {
		return shape
	}
	if group, ok := shape.(*GroupShape); ok {
		for _, child := range group.Children() {
			if found := shapeByName(child, name); found != nil {
				return found
			}
		}
	}
	return nil
}

// TitlePlaceholder returns the title placeholder, or nil if none exists.
func (s *Slide) TitlePlaceholder() *PlaceholderShape {
	return s.GetPlaceholder(PlaceholderTitle)
}

// BodyPlaceholder returns the body/content placeholder, or nil if none exists.
func (s *Slide) BodyPlaceholder() *PlaceholderShape {
	return s.GetPlaceholder(PlaceholderBody)
}

// marshal converts the slide to XML bytes.
func (s *Slide) marshal() ([]byte, error) {
	if s.slideXML == nil {
		s.slideXML = newSlideXML()
	}

	// Only sync Go shapes to XML when shapes were modified via the API.
	// When loading from a file, the slideXML already contains the parsed shapes.
	if s.shapesModified {
		s.syncShapesToXML()
	}

	// Process any pending image replacements after the XML tree is up to date.
	// This modifies the XML directly, converting p:sp elements to p:pic elements
	// or swapping the blip reference on existing pictures.
	if err := s.processPendingImages(); err != nil {
		return nil, err
	}

	// Build the timing tree for any auto-play media added to the slide (needs the
	// shape ids assigned during the sync above).
	s.applyMediaTiming()

	// Use the namespace-aware marshaler for PowerPoint compatibility
	return marshalSlide(s.slideXML), nil
}

// syncShapesToXML converts Go shapes to oxml types in the shape tree.
func (s *Slide) syncShapesToXML() {
	if s.slideXML.CSld == nil {
		s.slideXML.CSld = &oxml.CommonSlideData{}
	}
	if s.slideXML.CSld.SpTree == nil {
		s.slideXML.CSld.SpTree = newShapeTree()
	}

	spTree := s.slideXML.CSld.SpTree

	// Loaded slide with removals: delete just the removed nodes from the parsed
	// tree (preserving all other content, including kinds the domain model does
	// not represent), then append any new shapes. Avoids the destructive full
	// rebuild below.
	if len(s.removedRefs) > 0 && !s.forceShapeRebuild {
		spTree.RemoveChildren(s.removedRefs)
		s.reindexShapeRefsAfterRemoval(s.removedRefs)
		s.removedRefs = nil
		if len(s.shapes) > s.syncedShapes {
			s.appendShapesToXML(spTree, s.shapes[s.syncedShapes:])
		}
		s.syncedShapes = len(s.shapes)
		s.shapesModified = false
		return
	}

	// Loaded slide, appends only: marshal just the new shapes into the parsed
	// tree. The full rebuild below regenerates the tree from the domain model,
	// which drops content the model does not represent (group shapes,
	// connectors) and re-numbers every existing shape — acceptable for decks
	// built programmatically, destructive for decks loaded from a file.
	if s.syncedShapes > 0 && !s.forceShapeRebuild && len(s.shapes) >= s.syncedShapes {
		s.appendShapesToXML(spTree, s.shapes[s.syncedShapes:])
		s.syncedShapes = len(s.shapes)
		s.shapesModified = false
		return
	}

	// Clear existing shapes and child order tracking
	spTree.Sp = nil
	spTree.GraphicFrame = nil
	spTree.Pic = nil
	spTree.GrpSp = nil
	spTree.ClearChildOrder()

	// Convert each shape
	var shapeID uint32 = 2 // Start from 2 since 1 is used by spTree itself
	for _, shape := range s.shapes {
		switch sh := shape.(type) {
		case *TextBox:
			sp := textBoxToOxml(sh, shapeID)
			spTree.Sp = append(spTree.Sp, sp)
			shapeID++
		case *PlaceholderShape:
			sp := placeholderToOxml(sh, shapeID)
			spTree.Sp = append(spTree.Sp, sp)
			shapeID++
		case *AutoShape:
			sp := autoShapeToOxml(sh, shapeID)
			spTree.Sp = append(spTree.Sp, sp)
			shapeID++
		case *Table:
			gf := tableToOxml(sh, shapeID)
			spTree.GraphicFrame = append(spTree.GraphicFrame, gf)
			shapeID++
		case *Picture:
			pic := pictureToOxml(sh, shapeID)
			spTree.Pic = append(spTree.Pic, pic)
			shapeID++
		case *Video:
			spTree.Pic = append(spTree.Pic, s.buildMediaPic(&sh.mediaShape, shapeID, mediaVideo))
			shapeID++
		case *Audio:
			spTree.Pic = append(spTree.Pic, s.buildMediaPic(&sh.mediaShape, shapeID, mediaAudio))
			shapeID++
		}
	}

	// The rebuild regenerates the whole tree from the domain model, so any
	// surgical bookkeeping recorded against the old tree is void. Leaving
	// syncedShapes at 0 keeps the slide in rebuild mode, where every domain
	// mutation is flushed on the next sync.
	s.syncedShapes = 0
	s.shapeRefs = nil
	s.removedRefs = nil
	s.shapesModified = false
}

// reindexShapeRefsAfterRemoval rewrites shapeRefs from the pre-compaction
// indices they were recorded with to the indices the surviving children occupy
// after RemoveChildren compacted the typed slices: every removed same-kind
// child below a ref shifts it down by one. Without this, refs recorded before
// one removal target the wrong nodes in the next removal cycle.
func (s *Slide) reindexShapeRefsAfterRemoval(removed []oxml.ChildRef) {
	for i, ref := range s.shapeRefs {
		if ref.Index < 0 {
			continue
		}
		dec := 0
		for _, rem := range removed {
			if rem.Kind == ref.Kind && rem.Index < ref.Index {
				dec++
			}
		}
		if dec > 0 {
			s.shapeRefs[i] = oxml.ChildRef{Kind: ref.Kind, Index: ref.Index - dec}
		}
	}
}

// appendShapesToXML marshals newly added shapes into a parsed shape tree,
// assigning ids above everything already on the slide. Each appended shape's
// child reference is recorded in shapeRefs so a later removal (or in-place
// update) can target its node surgically instead of forcing a full rebuild;
// shapes the append path cannot express get a sentinel ref (Index -1) to keep
// shapeRefs aligned with the shapes slice.
func (s *Slide) appendShapesToXML(spTree *oxml.ShapeTree, shapes []Shape) {
	id := spTree.MaxShapeID() + 1
	if id < 2 {
		id = 2 // 1 belongs to the shape tree itself
	}
	for _, shape := range shapes {
		ref := oxml.ChildRef{Index: -1}
		switch sh := shape.(type) {
		case *TextBox:
			spTree.AppendSp(textBoxToOxml(sh, id))
			ref = oxml.ChildRef{Kind: oxml.ChildSp, Index: len(spTree.Sp) - 1}
		case *PlaceholderShape:
			spTree.AppendSp(placeholderToOxml(sh, id))
			ref = oxml.ChildRef{Kind: oxml.ChildSp, Index: len(spTree.Sp) - 1}
		case *AutoShape:
			spTree.AppendSp(autoShapeToOxml(sh, id))
			ref = oxml.ChildRef{Kind: oxml.ChildSp, Index: len(spTree.Sp) - 1}
		case *Table:
			gf := tableToOxml(sh, id)
			spTree.AppendGraphicFrame(gf)
			// Later row/cell mutations reach the XML via SyncXML.
			sh.sourceFrame = gf
			ref = oxml.ChildRef{Kind: oxml.ChildGraphicFrame, Index: len(spTree.GraphicFrame) - 1}
		case *Picture:
			spTree.AppendPic(pictureToOxml(sh, id))
			ref = oxml.ChildRef{Kind: oxml.ChildPic, Index: len(spTree.Pic) - 1}
		case *Video:
			spTree.AppendPic(s.buildMediaPic(&sh.mediaShape, id, mediaVideo))
			ref = oxml.ChildRef{Kind: oxml.ChildPic, Index: len(spTree.Pic) - 1}
		case *Audio:
			spTree.AppendPic(s.buildMediaPic(&sh.mediaShape, id, mediaAudio))
			ref = oxml.ChildRef{Kind: oxml.ChildPic, Index: len(spTree.Pic) - 1}
		}
		if ref.Index >= 0 {
			id++
		}
		s.shapeRefs = append(s.shapeRefs, ref)
	}
}

// textBoxToOxml converts a TextBox to oxml.Shape.
func textBoxToOxml(tb *TextBox, id uint32) *oxml.Shape {
	name := tb.Name()
	if name == "" {
		name = "TextBox"
	}

	x, y := tb.Position()
	w, h := tb.Size()

	sp := &oxml.Shape{
		NvSpPr: &oxml.NvSpPr{
			CNvPr: &dml.CNvPr{
				Id:   id,
				Name: name,
			},
			CNvSpPr: &dml.CNvSpPr{
				TxBox: true,
			},
			NvPr: &oxml.NvPr{},
		},
		SpPr: &dml.SpPr{
			Xfrm: &dml.Xfrm{
				Off: &dml.OffXML{X: int64(x), Y: int64(y)},
				Ext: &dml.ExtXML{Cx: int64(w), Cy: int64(h)},
			},
			PrstGeom: &dml.PrstGeom{
				Prst:  "rect",
				AvLst: &dml.AvLst{},
			},
		},
	}

	// Carry any fill/line/shadow set via SetFill/SetLine/SetShadow into the XML
	// (previously textBoxToOxml ignored tb.spPr entirely, so those setters were
	// silent no-ops).
	applyShapeStyle(sp.SpPr, &tb.spPr)

	if tb.textFrame != nil {
		sp.TxBody = textFrameToOxml(tb.textFrame)
	}

	return sp
}

// applyShapeStyle copies the fill, line, and effect properties that a domain
// shape stored (via Fill/Line/Shadow ApplyToSpPr) into the SpPr being
// marshaled. Every fill kind is copied, not just SolidFill, so gradient,
// pattern, and no-fill values are not dropped.
func applyShapeStyle(dst *dml.SpPr, src *dml.SpPr) {
	if src == nil {
		return
	}
	if src.NoFill != nil {
		dst.NoFill = src.NoFill
	}
	if src.SolidFill != nil {
		dst.SolidFill = src.SolidFill
	}
	if src.GradFill != nil {
		dst.GradFill = src.GradFill
	}
	if src.PattFill != nil {
		dst.PattFill = src.PattFill
	}
	if src.BlipFill != nil {
		dst.BlipFill = src.BlipFill
	}
	if src.GrpFill != nil {
		dst.GrpFill = src.GrpFill
	}
	if src.Ln != nil {
		dst.Ln = src.Ln
	}
	if src.EffectLst != nil {
		dst.EffectLst = src.EffectLst
	}
	if src.EffectDag != nil {
		dst.EffectDag = src.EffectDag
	}
}

// placeholderToOxml converts a PlaceholderShape to oxml.Shape.
func placeholderToOxml(ph *PlaceholderShape, id uint32) *oxml.Shape {
	name := ph.Name()
	if name == "" {
		name = string(ph.phType)
	}

	x, y := ph.Position()
	w, h := ph.Size()

	sp := &oxml.Shape{
		NvSpPr: &oxml.NvSpPr{
			CNvPr: &dml.CNvPr{
				Id:   id,
				Name: name,
			},
			CNvSpPr: &dml.CNvSpPr{},
			NvPr: &oxml.NvPr{
				Ph: &oxml.Placeholder{
					Type: string(ph.phType),
					Idx:  ph.idx,
				},
			},
		},
		SpPr: &dml.SpPr{
			Xfrm: &dml.Xfrm{
				Off: &dml.OffXML{X: int64(x), Y: int64(y)},
				Ext: &dml.ExtXML{Cx: int64(w), Cy: int64(h)},
			},
		},
	}

	if ph.textFrame != nil {
		sp.TxBody = textFrameToOxml(ph.textFrame)
	}

	return sp
}

// autoShapeToOxml converts an AutoShape to oxml.Shape.
func autoShapeToOxml(as *AutoShape, id uint32) *oxml.Shape {
	name := as.Name()
	if name == "" {
		name = "Shape"
	}

	x, y := as.Position()
	w, h := as.Size()

	sp := &oxml.Shape{
		NvSpPr: &oxml.NvSpPr{
			CNvPr: &dml.CNvPr{
				Id:   id,
				Name: name,
			},
			CNvSpPr: &dml.CNvSpPr{},
			NvPr:    &oxml.NvPr{},
		},
		SpPr: &dml.SpPr{
			Xfrm: &dml.Xfrm{
				Off: &dml.OffXML{X: int64(x), Y: int64(y)},
				Ext: &dml.ExtXML{Cx: int64(w), Cy: int64(h)},
			},
			PrstGeom: &dml.PrstGeom{
				Prst:  as.presetGeometry,
				AvLst: &dml.AvLst{},
			},
		},
	}
	// Copy every fill kind plus line/effects (previously only SolidFill was
	// carried, so gradient/pattern/no-fill fills set via SetFill were dropped).
	applyShapeStyle(sp.SpPr, &as.spPr)

	if as.textFrame != nil {
		sp.TxBody = textFrameToOxml(as.textFrame)
	}

	return sp
}

// textFrameToOxml converts a TextFrame to dml.TxBody.
func textFrameToOxml(tf *TextFrame) *dml.TxBody {
	lIns := int64(tf.margins.Left)
	tIns := int64(tf.margins.Top)
	rIns := int64(tf.margins.Right)
	bIns := int64(tf.margins.Bottom)
	txBody := &dml.TxBody{
		BodyPr: &dml.BodyPr{
			Wrap:   string(tf.wrap),
			Anchor: string(tf.anchor),
			LIns:   &lIns,
			TIns:   &tIns,
			RIns:   &rIns,
			BIns:   &bIns,
		},
		LstStyle: &dml.LstStyle{},
		P:        make([]*dml.P, 0, len(tf.paragraphs)),
	}

	for _, para := range tf.paragraphs {
		ap := paragraphToOxml(para)
		txBody.P = append(txBody.P, ap)
	}

	// Ensure at least one paragraph
	if len(txBody.P) == 0 {
		txBody.P = append(txBody.P, &dml.P{})
	}

	return txBody
}

// paragraphToOxml converts a Paragraph to dml.P.
func paragraphToOxml(p *Paragraph) *dml.P {
	ap := &dml.P{
		R: make([]*dml.R, 0, len(p.runs)),
	}

	// Set paragraph properties if needed
	needSpacing := p.lineSpacing != 0 || p.spaceBefore != 0 || p.spaceAfter != 0
	if p.alignment != "" || p.level > 0 || p.bulletType != BulletInherit || needSpacing {
		lvl := int32(p.level)
		ap.PPr = &dml.PPr{
			Algn: string(p.alignment),
			Lvl:  &lvl,
		}

		// Emit a bullet element only when the bullet was set explicitly.
		// BulletInherit (the default) emits nothing so the paragraph keeps the
		// layout/master bullet even when other properties (alignment, spacing)
		// are set.
		switch p.bulletType {
		case BulletNone:
			ap.PPr.BuNone = &dml.BuNone{}
		case BulletChar:
			ap.PPr.BuChar = &dml.BuChar{Char: p.bulletChar}
		case BulletNumber, BulletAuto:
			ap.PPr.BuAutoNum = &dml.BuAutoNum{Type: "arabicPeriod"}
		}

		// Spacing (symmetric with the oxml->domain read): line spacing is a
		// percentage, space before/after are point values.
		if p.lineSpacing != 0 {
			ap.PPr.LnSpc = &dml.LnSpc{SpcPct: &dml.SpcPct{Val: dml.Percentage(p.lineSpacing)}}
		}
		if p.spaceBefore != 0 {
			ap.PPr.SpcBef = &dml.SpcBef{SpcPts: &dml.SpcPts{Val: int32(p.spaceBefore)}}
		}
		if p.spaceAfter != 0 {
			ap.PPr.SpcAft = &dml.SpcAft{SpcPts: &dml.SpcPts{Val: int32(p.spaceAfter)}}
		}
	}

	// Convert runs
	for _, run := range p.runs {
		ar := runToOxml(run)
		ap.R = append(ap.R, ar)
	}

	return ap
}

// runToOxml converts a Run to dml.R.
func runToOxml(r *Run) *dml.R {
	ar := &dml.R{
		T: r.text,
	}

	// Set run properties if any formatting is applied
	if r.fontName != "" || r.fontSize > 0 || r.bold || r.italic ||
		r.underline != "" || r.strike != "" || r.color != nil || r.baseline != 0 ||
		r.highlight != nil {
		ar.RPr = &dml.RPr{}

		if r.highlight != nil {
			ar.RPr.Highlight = colorToColorChoiceOxml(r.highlight)
		}

		if r.fontSize > 0 {
			ar.RPr.Sz = int32(r.fontSize * 100) // Convert points to hundredths
		}

		if r.bold {
			b := true
			ar.RPr.B = &b
		}

		if r.italic {
			i := true
			ar.RPr.I = &i
		}

		if r.underline != "" {
			ar.RPr.U = string(r.underline)
		}

		if r.strike != "" {
			ar.RPr.Strike = string(r.strike)
		}

		if r.baseline != 0 {
			baseline := int32(r.baseline)
			ar.RPr.Baseline = &baseline
		}

		if r.fontName != "" {
			ar.RPr.Latin = &dml.TextFont{Typeface: r.fontName}
		}

		if r.color != nil {
			ar.RPr.SolidFill = colorToOxml(r.color)
		}
	}

	return ar
}

// colorToColorChoiceOxml converts a Color to a dml.ColorChoice (used where the
// schema expects a bare color element, e.g. a:highlight).
func colorToColorChoiceOxml(c *dml.Color) *dml.ColorChoice {
	if c == nil {
		return nil
	}
	cc := &dml.ColorChoice{}
	if c.Type == dml.ColorTypeTheme {
		cc.SchemeClr = &dml.SchemeClrTransform{Val: c.Theme.String()}
	} else {
		cc.SrgbClr = &dml.SrgbClr{Val: c.RGB.String()}
	}
	return cc
}

// colorToOxml converts a Color to dml.SolidFill.
func colorToOxml(c *dml.Color) *dml.SolidFill {
	if c == nil {
		return nil
	}

	sf := &dml.SolidFill{}
	if c.Type == dml.ColorTypeTheme {
		sf.SchemeClr = &dml.SchemeClrTransform{Val: c.Theme.String()}
	} else {
		sf.SrgbClr = &dml.SrgbClr{Val: c.RGB.String()}
	}
	return sf
}

// tableToOxml converts a Table to oxml.GraphicFrame.
func tableToOxml(t *Table, id uint32) *oxml.GraphicFrame {
	name := t.Name()
	if name == "" {
		name = "Table"
	}

	x, y := t.Position()
	w, h := t.Size()

	gf := &oxml.GraphicFrame{
		NvGraphicFramePr: &oxml.NvGraphicFramePr{
			CNvPr: &dml.CNvPr{
				Id:   id,
				Name: name,
			},
			CNvGraphicFramePr: &oxml.CNvGraphicFramePr{
				GraphicFrameLocks: &oxml.GraphicFrameLocks{NoGrp: true},
			},
			NvPr: &oxml.NvPr{},
		},
		Xfrm: &dml.Xfrm{
			Off: &dml.OffXML{X: int64(x), Y: int64(y)},
			Ext: &dml.ExtXML{Cx: int64(w), Cy: int64(h)},
		},
		Graphic: &oxml.AGraphic{
			GraphicData: &oxml.AGraphicData{
				URI:   oxml.TableGraphicDataURI,
				Table: tableDataToOxml(t),
			},
		},
	}

	return gf
}

// tableDataToOxml converts Table data to oxml.ATable.
func tableDataToOxml(t *Table) *oxml.ATable {
	tbl := &oxml.ATable{
		TblPr: &oxml.ATblPr{
			FirstRow: t.firstRow,
			FirstCol: t.firstCol,
			LastRow:  t.lastRow,
			LastCol:  t.lastCol,
			BandRow:  t.bandRow,
			BandCol:  t.bandCol,
		},
		TblGrid: &oxml.ATblGrid{
			GridCol: make([]*oxml.AGridCol, len(t.colWidths)),
		},
		Tr: make([]*oxml.ATr, len(t.rows)),
	}

	// Set column widths
	for i, w := range t.colWidths {
		tbl.TblGrid.GridCol[i] = &oxml.AGridCol{W: int64(w)}
	}

	// Convert rows
	for i, row := range t.rows {
		tr := &oxml.ATr{
			H:  int64(row.height),
			Tc: make([]*oxml.ATc, len(row.cells)),
		}

		for j, cell := range row.cells {
			tc := &oxml.ATc{}

			// Set cell properties
			if cell.fill != nil || cell.vertAlign != "" ||
				cell.borderLeft != nil || cell.borderRight != nil ||
				cell.borderTop != nil || cell.borderBottom != nil {
				tc.TcPr = &oxml.ATcPr{}
				if cell.vertAlign != "" {
					tc.TcPr.Anchor = string(cell.vertAlign)
				}
				tc.TcPr.LnL = tableBorderToLn(cell.borderLeft)
				tc.TcPr.LnR = tableBorderToLn(cell.borderRight)
				tc.TcPr.LnT = tableBorderToLn(cell.borderTop)
				tc.TcPr.LnB = tableBorderToLn(cell.borderBottom)
				if cell.fill != nil {
					tc.TcPr.SolidFill = colorToOxml(cell.fill)
				}
			}

			// Set row/col span
			if cell.rowSpan > 1 {
				tc.RowSpan = cell.rowSpan
			}
			if cell.colSpan > 1 {
				tc.GridSpan = cell.colSpan
			}
			tc.HMerge = cell.hMerge
			tc.VMerge = cell.vMerge

			// Convert text
			if cell.textFrame != nil {
				tc.TxBody = textFrameToOxml(cell.textFrame)
			}

			tr.Tc[j] = tc
		}

		tbl.Tr[i] = tr
	}

	return tbl
}

// tableBorderToLn converts a domain TableBorder to a dml.Ln for a cell edge.
func tableBorderToLn(b *TableBorder) *dml.Ln {
	if b == nil {
		return nil
	}
	ln := &dml.Ln{}
	if b.Width > 0 {
		w := int64(b.Width)
		ln.W = &w
	}
	switch b.Style {
	case BorderStyleNone:
		// An explicit "no border" — emit a line with no fill.
		ln.NoFill = &dml.NoFillXML{}
		return ln
	case BorderStyleDashed:
		ln.PrstDash = &dml.PrstDash{Val: "dash"}
	case BorderStyleDotted:
		ln.PrstDash = &dml.PrstDash{Val: "dot"}
	case BorderStyleDouble:
		ln.Cmpd = "dbl"
	}
	ln.SolidFill = colorToOxml(&b.Color)
	return ln
}

// pictureToOxml converts a Picture to oxml.Picture.
func pictureToOxml(p *Picture, id uint32) *oxml.Picture {
	name := p.Name()
	if name == "" {
		name = "Picture"
	}

	x, y := p.Position()
	w, h := p.Size()

	pic := &oxml.Picture{
		NvPicPr: &oxml.NvPicPr{
			CNvPr: &dml.CNvPr{
				Id:    id,
				Name:  name,
				Descr: p.description,
			},
			CNvPicPr: &dml.CNvPicPr{
				PicLocks: &dml.PicLocks{NoChangeAspect: true},
			},
			NvPr: &oxml.NvPr{},
		},
		BlipFill: &dml.BlipFill{
			Blip: &dml.Blip{
				// The embed reference will be set when saving with relationships
				Embed: p.relID,
			},
			Stretch: &dml.Stretch{
				FillRect: &dml.RelRect{},
			},
		},
		SpPr: &dml.SpPr{
			Xfrm: &dml.Xfrm{
				Off: &dml.OffXML{X: int64(x), Y: int64(y)},
				Ext: &dml.ExtXML{Cx: int64(w), Cy: int64(h)},
			},
			PrstGeom: &dml.PrstGeom{
				Prst:  "rect",
				AvLst: &dml.AvLst{},
			},
		},
	}

	// Apply cropping if set
	if p.cropLeft > 0 || p.cropTop > 0 || p.cropRight > 0 || p.cropBottom > 0 {
		pic.BlipFill.SrcRect = &dml.SrcRect{
			L: int32(p.cropLeft * 100000),
			T: int32(p.cropTop * 100000),
			R: int32(p.cropRight * 100000),
			B: int32(p.cropBottom * 100000),
		}
	}

	return pic
}

// Duplicate creates a copy of the slide and adds it after this slide.
func (s *Slide) Duplicate() *Slide {
	if s.shapesModified {
		s.syncShapesToXML()
	}

	newSlide := s.presentation.AddSlide()
	newSlide.layout = s.layout
	newSlide.partName = s.presentation.nextAvailableSlidePartName()

	// Copy slide XML and slide-level relationships so the duplicate remains self-contained.
	if s.slideXML != nil {
		data := marshalSlide(s.slideXML)
		var copyXML oxml.Slide
		if err := xml.Unmarshal(data, &copyXML); err == nil {
			newSlide.slideXML = &copyXML
		}
	}
	if s.partName != "" {
		// Reuse the part name already allocated above; allocating a second one
		// here would treat the first as taken and skip (burn) a slide number.
		s.presentation.clonePartRelationships(s.partName, newSlide.partName)
		// Give the duplicate its own notes slide rather than sharing the
		// original's (otherwise editing one slide's notes changes the other).
		s.presentation.deepCloneNotesSlide(newSlide.partName)
	}
	if newSlide.slideXML == nil {
		newSlide.slideXML = newSlideXML()
	}
	newSlide.materializeShapes()
	if newSlide.partName == "" {
		newSlide.partName = fmt.Sprintf("/ppt/slides/slide%d.xml", newSlide.index+1)
	}

	// Move to position after original
	_ = s.presentation.MoveSlide(newSlide.index, s.index+1)

	return newSlide
}

// Delete removes this slide from the presentation.
func (s *Slide) Delete() error {
	return s.presentation.RemoveSlide(s.index)
}
