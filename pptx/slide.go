package pptx

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Slide represents a slide in a presentation.
type Slide struct {
	presentation *Presentation
	layout       *SlideLayout
	partName     string
	slideXML     *oxml.Slide
	index        int
	id           uint32
	relID        string
	shapes       []Shape
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
	s.shapes = append(s.shapes, shape)
}

// RemoveShape removes a shape from the slide.
func (s *Slide) RemoveShape(shape Shape) {
	for i, sh := range s.shapes {
		if sh == shape {
			s.shapes = append(s.shapes[:i], s.shapes[i+1:]...)
			return
		}
	}
}

// AddTextBox adds a text box shape to the slide.
func (s *Slide) AddTextBox() *TextBox {
	tb := NewTextBox()
	s.AddShape(tb)
	return tb
}

// AddPicture adds a picture shape to the slide.
func (s *Slide) AddPicture(imagePath string) (*Picture, error) {
	// Placeholder implementation
	pic := &Picture{
		BaseShape: BaseShape{},
		imagePath: imagePath,
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

	// Convert Go shapes to XML shapes
	s.syncShapesToXML()

	output, err := xml.MarshalIndent(s.slideXML, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), output...), nil
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

	// Clear existing shapes
	spTree.Sp = nil
	spTree.GraphicFrame = nil
	spTree.Pic = nil
	spTree.GrpSp = nil

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
		}
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
			CNvPr: &oxml.CNvPr{
				ID:   id,
				Name: name,
			},
			CNvSpPr: &oxml.CNvSpPr{
				TxBox: true,
			},
			NvPr: &oxml.NvPr{},
		},
		SpPr: &oxml.SpPr{
			Xfrm: &oxml.Xfrm{
				Off: &oxml.Off{X: int64(x), Y: int64(y)},
				Ext: &oxml.Ext{Cx: int64(w), Cy: int64(h)},
			},
			PrstGeom: &oxml.PrstGeom{
				Prst:  "rect",
				AvLst: &oxml.AvLst{},
			},
		},
	}

	if tb.textFrame != nil {
		sp.TxBody = textFrameToOxml(tb.textFrame)
	}

	return sp
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
			CNvPr: &oxml.CNvPr{
				ID:   id,
				Name: name,
			},
			CNvSpPr: &oxml.CNvSpPr{},
			NvPr: &oxml.NvPr{
				Ph: &oxml.PlaceholderRef{
					Type: string(ph.phType),
					Idx:  ph.idx,
				},
			},
		},
		SpPr: &oxml.SpPr{
			Xfrm: &oxml.Xfrm{
				Off: &oxml.Off{X: int64(x), Y: int64(y)},
				Ext: &oxml.Ext{Cx: int64(w), Cy: int64(h)},
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
			CNvPr: &oxml.CNvPr{
				ID:   id,
				Name: name,
			},
			CNvSpPr: &oxml.CNvSpPr{},
			NvPr:    &oxml.NvPr{},
		},
		SpPr: &oxml.SpPr{
			Xfrm: &oxml.Xfrm{
				Off: &oxml.Off{X: int64(x), Y: int64(y)},
				Ext: &oxml.Ext{Cx: int64(w), Cy: int64(h)},
			},
			PrstGeom: &oxml.PrstGeom{
				Prst:  as.presetGeometry,
				AvLst: &oxml.AvLst{},
			},
		},
	}

	if as.textFrame != nil {
		sp.TxBody = textFrameToOxml(as.textFrame)
	}

	return sp
}

// textFrameToOxml converts a TextFrame to oxml.TxBody.
func textFrameToOxml(tf *TextFrame) *oxml.TxBody {
	txBody := &oxml.TxBody{
		BodyPr: &oxml.BodyPr{
			Wrap:   string(tf.wrap),
			Anchor: string(tf.anchor),
			LIns:   int64(tf.margins.Left),
			TIns:   int64(tf.margins.Top),
			RIns:   int64(tf.margins.Right),
			BIns:   int64(tf.margins.Bottom),
		},
		LstStyle: &oxml.LstStyle{},
		P:        make([]*oxml.AP, 0, len(tf.paragraphs)),
	}

	for _, para := range tf.paragraphs {
		ap := paragraphToOxml(para)
		txBody.P = append(txBody.P, ap)
	}

	// Ensure at least one paragraph
	if len(txBody.P) == 0 {
		txBody.P = append(txBody.P, &oxml.AP{})
	}

	return txBody
}

// paragraphToOxml converts a Paragraph to oxml.AP.
func paragraphToOxml(p *Paragraph) *oxml.AP {
	ap := &oxml.AP{
		R: make([]*oxml.AR, 0, len(p.runs)),
	}

	// Set paragraph properties if needed
	if p.alignment != "" || p.level > 0 || p.bulletType != BulletNone {
		ap.PPr = &oxml.APPr{
			Algn: string(p.alignment),
			Lvl:  p.level,
		}

		switch p.bulletType {
		case BulletNone:
			ap.PPr.BuNone = &oxml.BuNone{}
		case BulletChar:
			ap.PPr.BuChar = &oxml.BuChar{Char: p.bulletChar}
		case BulletNumber:
			ap.PPr.BuAutoNum = &oxml.BuAutoNum{Type: "arabicPeriod"}
		}
	}

	// Convert runs
	for _, run := range p.runs {
		ar := runToOxml(run)
		ap.R = append(ap.R, ar)
	}

	return ap
}

// runToOxml converts a Run to oxml.AR.
func runToOxml(r *Run) *oxml.AR {
	ar := &oxml.AR{
		T: r.text,
	}

	// Set run properties if any formatting is applied
	if r.fontName != "" || r.fontSize > 0 || r.bold || r.italic ||
		r.underline != "" || r.strike != "" || r.color != nil || r.baseline != 0 {
		ar.RPr = &oxml.ARPr{}

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
			ar.RPr.Baseline = r.baseline
		}

		if r.fontName != "" {
			ar.RPr.Latin = &oxml.Latin{Typeface: r.fontName}
		}

		if r.color != nil {
			ar.RPr.SolidFill = colorToOxml(r.color)
		}
	}

	return ar
}

// colorToOxml converts a Color to oxml.ASolidFill.
func colorToOxml(c *dml.Color) *oxml.ASolidFill {
	if c == nil {
		return nil
	}

	sf := &oxml.ASolidFill{}
	if c.Type == dml.ColorTypeTheme {
		sf.SchemeClr = &oxml.ASchemeClr{Val: c.Theme.String()}
	} else {
		sf.SrgbClr = &oxml.ASrgbClr{Val: c.RGB.String()}
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
			CNvPr: &oxml.CNvPr{
				ID:   id,
				Name: name,
			},
			CNvGraphicFramePr: &oxml.CNvGraphicFramePr{
				GraphicFrameLocks: &oxml.GraphicFrameLocks{NoGrp: true},
			},
			NvPr: &oxml.NvPr{},
		},
		Xfrm: &oxml.Xfrm{
			Off: &oxml.Off{X: int64(x), Y: int64(y)},
			Ext: &oxml.Ext{Cx: int64(w), Cy: int64(h)},
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
			if cell.fill != nil || cell.vertAlign != "" {
				tc.TcPr = &oxml.ATcPr{}
				if cell.vertAlign != "" {
					tc.TcPr.Anchor = string(cell.vertAlign)
				}
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
			CNvPr: &oxml.CNvPr{
				ID:    id,
				Name:  name,
				Descr: p.description,
			},
			CNvPicPr: &oxml.CNvPicPr{
				PicLocks: &oxml.PicLocks{NoChangeAspect: true},
			},
			NvPr: &oxml.NvPr{},
		},
		BlipFill: &oxml.BlipFill{
			Blip: &oxml.ABlip{
				// The embed reference will be set when saving with relationships
				Embed: p.relID,
			},
			Stretch: &oxml.AStretch{
				FillRect: &oxml.AFillRect{},
			},
		},
		SpPr: &oxml.SpPr{
			Xfrm: &oxml.Xfrm{
				Off: &oxml.Off{X: int64(x), Y: int64(y)},
				Ext: &oxml.Ext{Cx: int64(w), Cy: int64(h)},
			},
			PrstGeom: &oxml.PrstGeom{
				Prst:  "rect",
				AvLst: &oxml.AvLst{},
			},
		},
	}

	// Apply cropping if set
	if p.cropLeft > 0 || p.cropTop > 0 || p.cropRight > 0 || p.cropBottom > 0 {
		pic.BlipFill.SrcRect = &oxml.ASrcRect{
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
	newSlide := s.presentation.AddSlide()

	// Copy slide XML (deep copy would be needed for full implementation)
	if s.slideXML != nil {
		data, _ := xml.Marshal(s.slideXML)
		var copyXML oxml.Slide
		xml.Unmarshal(data, &copyXML)
		newSlide.slideXML = &copyXML
	}

	// Move to position after original
	s.presentation.MoveSlide(newSlide.index, s.index+1)

	return newSlide
}

// Delete removes this slide from the presentation.
func (s *Slide) Delete() error {
	return s.presentation.RemoveSlide(s.index)
}
