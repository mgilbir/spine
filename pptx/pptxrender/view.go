package pptxrender

import (
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/pptx/internal/view"
)

// The renderer reads what the pptx package keeps unexported through
// pptx/internal/view; these wrap its functions in their pptx types.

const (
	nsP = xmlb.NSPresentationML
	nsA = xmlb.NSDrawingML
	nsR = xmlb.NSOfficeDocumentRels
)

func chartRelIDOf(gf *oxml.GraphicFrame) string { return view.ChartRelID(gf) }

func oxmlShapeToGoShape(sp *oxml.Shape) pptx.Shape { return view.ShapeFromOxml(sp).(pptx.Shape) }

func oxmlPictureToGoPicture(pic *oxml.Picture) *pptx.Picture {
	return view.PictureFromOxml(pic).(*pptx.Picture)
}

func updateXfrm(spPr *dml.SpPr, base *pptx.BaseShape) { view.UpdateXfrm(spPr, base) }

func updateTxBody(dst **dml.TxBody, tf *pptx.TextFrame) { view.UpdateTxBody(dst, tf) }

func textFrameToOxml(tf *pptx.TextFrame) *dml.TxBody { return view.TextFrameToOxml(tf) }

func placeholderToOxml(ph *pptx.PlaceholderShape, id uint32) *oxml.Shape {
	return view.PlaceholderToOxml(ph, id)
}

func marshalDefaultTextStyle(b *xmlb.Builder) { view.DefaultTextStyle(b) }

func applyShapeStyle(dst, src *dml.SpPr) { view.ApplyShapeStyle(dst, src) }

// renderBox is a shape's box, properties, preset and text as drawn: an
// AutoShape's own, a text box drawn as one, or a placeholder's inherited.
type renderBox struct {
	x, y, width, height dml.EMU
	spPr                dml.SpPr
	textFrame           *pptx.TextFrame
	presetGeometry      string
}

func (b *renderBox) Position() (dml.EMU, dml.EMU) { return b.x, b.y }
func (b *renderBox) Size() (dml.EMU, dml.EMU)     { return b.width, b.height }

// shapeState snapshots a shape's unexported state.
func shapeState(sh pptx.Shape) view.Shape { return view.ShapeOf(sh) }

// textFrameOf is a snapshot's text frame, or nil.
func textFrameOf(st view.Shape) *pptx.TextFrame {
	if tf, ok := st.TextFrame.(*pptx.TextFrame); ok {
		return tf
	}
	return nil
}

// boxOf is a shape's box, drawn as preset, or its own preset for "".
func boxOf(sh pptx.Shape, preset string) *renderBox {
	st := shapeState(sh)
	x, y := sh.Position()
	w, h := sh.Size()
	if preset == "" {
		preset = st.Preset
	}
	return &renderBox{x: x, y: y, width: w, height: h, spPr: st.SpPr, textFrame: textFrameOf(st), presetGeometry: preset}
}

// textFrameDirty reports whether a text frame has unsaved edits.
func textFrameDirty(tf *pptx.TextFrame) bool { return view.TextFrameDirty(tf) }
