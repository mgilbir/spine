package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/pptx/internal/view"
)

// The slide renderer, pptx/pptxrender, reads the model through
// pptx/internal/view; see that package.
func init() {
	view.ChartRelID = chartRelIDOf
	view.ShapeFromOxml = func(sp *oxml.Shape) any { return oxmlShapeToGoShape(sp) }
	view.PictureFromOxml = func(pic *oxml.Picture) any { return oxmlPictureToGoPicture(pic) }
	view.UpdateXfrm = func(spPr *dml.SpPr, base any) { updateXfrm(spPr, base.(*BaseShape)) }
	view.UpdateTxBody = func(dst **dml.TxBody, tf any) { updateTxBody(dst, tf.(*TextFrame)) }
	view.TextFrameToOxml = func(tf any) *dml.TxBody { return textFrameToOxml(tf.(*TextFrame)) }
	view.PlaceholderToOxml = func(ph any, id uint32) *oxml.Shape { return placeholderToOxml(ph.(*PlaceholderShape), id) }
	view.DefaultTextStyle = marshalDefaultTextStyle
	view.ApplyShapeStyle = applyShapeStyle
	view.SlideOf = func(v any) *view.Slide { return v.(*Slide).view() }
	view.ShapeOf = shapeView
	view.TextFrameDirty = func(tf any) bool { return tf.(*TextFrame).isDirty() }
}

// shapeView snapshots a shape's unexported state for the renderer.
func shapeView(sh any) view.Shape {
	var out view.Shape
	switch v := sh.(type) {
	case *AutoShape:
		out.SpPr, out.Preset, out.Base = v.spPr, v.presetGeometry, &v.BaseShape
		if v.textFrame != nil {
			out.TextFrame = v.textFrame
		}
	case *TextBox:
		out.SpPr, out.Base = v.spPr, &v.BaseShape
		if v.textFrame != nil {
			out.TextFrame = v.textFrame
		}
	case *PlaceholderShape:
		out.Dirty, out.Base = v.dirty, &v.BaseShape
		out.PhType, out.Orientation, out.Size, out.FieldType, out.Idx = string(v.phType), string(v.orientation), string(v.size), string(v.fieldType), v.idx
		if v.textFrame != nil {
			out.TextFrame = v.textFrame
		}
	case *Picture:
		out.Crop = [4]float64{v.cropLeft, v.cropTop, v.cropRight, v.cropBottom}
		out.Effects, out.Opacity, out.Media, out.HasImageData, out.RelID = v.effects, v.opacity, v.isMedia, len(v.imageData) > 0, v.relID
		if v.slide != nil && v.relID != "" {
			out.ImagePart = v.slide.relTargetPart(v.relID)
		}
		out.Base = &v.BaseShape
	case *Table:
		out.Dirty = v.isDirty()
	case *GroupShape:
		out.Dirty = v.isDirty()
	case *Connector:
		out.Dirty = v.dirty
	case *ChartFrame:
		out.Part = v.partName
	}
	return out
}

// view snapshots the slide for the renderer.
func (s *Slide) view() *view.Slide {
	out := &view.Slide{PartName: s.partName, Index: s.index}
	out.Load = func() (*oxml.Slide, []any, []oxml.ChildRef) {
		model := s.sx()
		var shapes []any
		for _, sh := range s.shapeList() {
			shapes = append(shapes, sh)
		}
		return model, shapes, s.shapeRefs
	}
	if p := s.presentation; p != nil {
		w, h := p.slideDimensions()
		out.Package = &view.Package{Reader: p.reader, Presentation: p.presentation, Relationships: p.relationships, OtherParts: p.otherParts,
			ThemeData: p.themeData, ThemeEditors: p.themeEditors, PartData: p.rawPartData, TableStyles: p.tableStylesData, Width: w, Height: h}
	}
	if l := s.savedLayout(); l != nil {
		out.Layout = &view.Layout{XML: l.layoutXML, PartName: l.partName}
		if m := l.master; m != nil {
			out.Layout.Master = &view.Master{XML: m.masterXML, PartName: m.partName, ThemePart: m.resolvedThemePart}
			if out.Package != nil && m.resolvedThemePart == "" {
				out.Layout.Master.ThemePart, out.Package.ThemeData = s.presentation.savedTheme(m)
			}
		}
	}
	return out
}

// savedTheme names the theme part a master that has none resolved yet will be
// saved with, and returns the theme data including it, for the renderer. A
// merged master carries an imported theme part; a master of a deck built with
// Create gets the default theme1.xml, written only at save, so its bytes are
// added to a copy of the theme data rather than to the presentation, which
// preparing a page must not change (C610).
func (p *Presentation) savedTheme(m *SlideMaster) (string, map[string][]byte) {
	if m.themePartName != "" {
		return m.themePartName, p.themeData
	}
	if p.reader != nil {
		return "", p.themeData
	}
	const part = "/ppt/theme/theme1.xml"
	if _, ok := p.themeData[part]; ok {
		return part, p.themeData
	}
	data := make(map[string][]byte, len(p.themeData)+1)
	for k, v := range p.themeData {
		data[k] = v
	}
	data[part] = defaultThemeXML()
	return part, data
}

// savedLayout is the layout the slide is drawn with: its own, or, for a slide
// added with AddSlide (which has none until save), the deck's first layout,
// which is the one the save binds it to. Rendering through the view must see
// what the saved file will contain, or a never-saved slide has no master and
// so no color map (C610).
func (s *Slide) savedLayout() *SlideLayout {
	if s.layout != nil {
		return s.layout
	}
	if p := s.presentation; p != nil && len(p.slideLayouts) > 0 {
		return p.slideLayouts[0]
	}
	return nil
}
