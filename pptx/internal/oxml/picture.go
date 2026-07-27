package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// Picture represents a picture element (p:pic) in a slide.
type Picture struct {
	XMLName  xml.Name       `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pic"`
	NvPicPr  *NvPicPr       `xml:"nvPicPr"`
	BlipFill *dml.BlipFill  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main blipFill"`
	SpPr     *dml.SpPr      `xml:"spPr"`
	Style    *dml.Style     `xml:"style,omitempty"`
	ExtLst   *ExtensionList `xml:"extLst,omitempty"`
	// CapturedChildren records the source child sequence; children the model
	// does not type — a mc:AlternateContent wrapping the blip fill in
	// Mac-authored decks — are preserved verbatim instead of dropped.
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// UnmarshalXML captures the child sequence while decoding the children into
// the struct fields.
func (p *Picture) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return xmlb.UnmarshalOrderedChildren(d, p)
}

// NvPicPr contains non-visual picture properties.
type NvPicPr struct {
	CNvPr    *dml.CNvPr    `xml:"cNvPr"`
	CNvPicPr *dml.CNvPicPr `xml:"cNvPicPr"`
	NvPr     *NvPr         `xml:"nvPr"`
}

// ConnectionShape represents a connector shape (p:cxnSp).
type ConnectionShape struct {
	XMLName   xml.Name       `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cxnSp"`
	NvCxnSpPr *NvCxnSpPr     `xml:"nvCxnSpPr"`
	SpPr      *dml.SpPr      `xml:"spPr"`
	Style     *dml.Style     `xml:"style,omitempty"`
	ExtLst    *ExtensionList `xml:"extLst,omitempty"`
}

// NvCxnSpPr contains non-visual connection shape properties.
type NvCxnSpPr struct {
	CNvPr      *dml.CNvPr      `xml:"cNvPr"`
	CNvCxnSpPr *dml.CNvCxnSpPr `xml:"cNvCxnSpPr"`
	NvPr       *NvPr           `xml:"nvPr"`
}

// GroupShape represents a group shape (p:grpSp).
// Like ShapeTree, it preserves child element ordering (z-order).
type GroupShape struct {
	XMLName          xml.Name           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main grpSp"`
	NvGrpSpPr        *NvGrpSpPr         `xml:"nvGrpSpPr"`
	GrpSpPr          *GrpSpPr           `xml:"grpSpPr"`
	Shapes           []*Shape           `xml:"-"`
	Pictures         []*Picture         `xml:"-"`
	GraphicFrames    []*GraphicFrame    `xml:"-"`
	GroupShapes      []*GroupShape      `xml:"-"`
	ConnectionShapes []*ConnectionShape `xml:"-"`
	// AltContent and RawXML preserve unmodeled children in childOrder position
	// (see ShapeTree).
	AltContent []*AlternateContent `xml:"-"`
	RawXML     [][]byte            `xml:"-"`
	childOrder []ChildRef
}

// ChildOrder returns the child order tracking slice.
func (gs *GroupShape) ChildOrder() []ChildRef {
	return gs.childOrder
}

// AppendSp appends a shape as the last child of the group, keeping z-order
// tracking consistent on groups parsed from a file (see ShapeTree.AppendSp).
func (gs *GroupShape) AppendSp(sp *Shape) {
	gs.Shapes = append(gs.Shapes, sp)
	if gs.childOrder != nil {
		gs.childOrder = append(gs.childOrder, ChildRef{ChildSp, len(gs.Shapes) - 1})
	}
}

// AppendPic appends a picture as the last child of the group (see AppendSp).
func (gs *GroupShape) AppendPic(pic *Picture) {
	gs.Pictures = append(gs.Pictures, pic)
	if gs.childOrder != nil {
		gs.childOrder = append(gs.childOrder, ChildRef{ChildPic, len(gs.Pictures) - 1})
	}
}

// AppendGraphicFrame appends a graphic frame as the last child of the group
// (see AppendSp).
func (gs *GroupShape) AppendGraphicFrame(gf *GraphicFrame) {
	gs.GraphicFrames = append(gs.GraphicFrames, gf)
	if gs.childOrder != nil {
		gs.childOrder = append(gs.childOrder, ChildRef{ChildGraphicFrame, len(gs.GraphicFrames) - 1})
	}
}

// AppendGrpSp appends a nested group as the last child of the group (see
// AppendSp).
func (gs *GroupShape) AppendGrpSp(sub *GroupShape) {
	gs.GroupShapes = append(gs.GroupShapes, sub)
	if gs.childOrder != nil {
		gs.childOrder = append(gs.childOrder, ChildRef{ChildGrpSp, len(gs.GroupShapes) - 1})
	}
}

// AppendCxnSp appends a connector as the last child of the group (see
// AppendSp).
func (gs *GroupShape) AppendCxnSp(cs *ConnectionShape) {
	gs.ConnectionShapes = append(gs.ConnectionShapes, cs)
	if gs.childOrder != nil {
		gs.childOrder = append(gs.childOrder, ChildRef{ChildCxnSp, len(gs.ConnectionShapes) - 1})
	}
}

// RemoveChildren rebuilds the group's children, omitting the given child
// references and preserving every other child (including kinds the domain
// model does not materialize, such as connectors) in order. It mirrors
// ShapeTree.RemoveChildren.
func (gs *GroupShape) RemoveChildren(refs []ChildRef) {
	if len(refs) == 0 {
		return
	}
	drop := make(map[ChildRef]bool, len(refs))
	for _, r := range refs {
		drop[r] = true
	}

	if len(gs.childOrder) == 0 {
		var sp []*Shape
		for i, v := range gs.Shapes {
			if !drop[ChildRef{ChildSp, i}] {
				sp = append(sp, v)
			}
		}
		var pic []*Picture
		for i, v := range gs.Pictures {
			if !drop[ChildRef{ChildPic, i}] {
				pic = append(pic, v)
			}
		}
		var gf []*GraphicFrame
		for i, v := range gs.GraphicFrames {
			if !drop[ChildRef{ChildGraphicFrame, i}] {
				gf = append(gf, v)
			}
		}
		var grp []*GroupShape
		for i, v := range gs.GroupShapes {
			if !drop[ChildRef{ChildGrpSp, i}] {
				grp = append(grp, v)
			}
		}
		var cxn []*ConnectionShape
		for i, v := range gs.ConnectionShapes {
			if !drop[ChildRef{ChildCxnSp, i}] {
				cxn = append(cxn, v)
			}
		}
		gs.Shapes, gs.Pictures, gs.GraphicFrames, gs.GroupShapes, gs.ConnectionShapes = sp, pic, gf, grp, cxn
		return
	}

	var (
		sp    []*Shape
		pic   []*Picture
		gf    []*GraphicFrame
		grp   []*GroupShape
		cxn   []*ConnectionShape
		ac    []*AlternateContent
		raw   [][]byte
		order []ChildRef
	)
	for _, ref := range gs.childOrder {
		if drop[ref] {
			continue
		}
		switch ref.Kind {
		case ChildSp:
			if ref.Index < len(gs.Shapes) {
				order = append(order, ChildRef{ChildSp, len(sp)})
				sp = append(sp, gs.Shapes[ref.Index])
			}
		case ChildPic:
			if ref.Index < len(gs.Pictures) {
				order = append(order, ChildRef{ChildPic, len(pic)})
				pic = append(pic, gs.Pictures[ref.Index])
			}
		case ChildGraphicFrame:
			if ref.Index < len(gs.GraphicFrames) {
				order = append(order, ChildRef{ChildGraphicFrame, len(gf)})
				gf = append(gf, gs.GraphicFrames[ref.Index])
			}
		case ChildGrpSp:
			if ref.Index < len(gs.GroupShapes) {
				order = append(order, ChildRef{ChildGrpSp, len(grp)})
				grp = append(grp, gs.GroupShapes[ref.Index])
			}
		case ChildCxnSp:
			if ref.Index < len(gs.ConnectionShapes) {
				order = append(order, ChildRef{ChildCxnSp, len(cxn)})
				cxn = append(cxn, gs.ConnectionShapes[ref.Index])
			}
		case ChildAltContent:
			if ref.Index < len(gs.AltContent) {
				order = append(order, ChildRef{ChildAltContent, len(ac)})
				ac = append(ac, gs.AltContent[ref.Index])
			}
		case ChildRawXML:
			if ref.Index < len(gs.RawXML) {
				order = append(order, ChildRef{ChildRawXML, len(raw)})
				raw = append(raw, gs.RawXML[ref.Index])
			}
		}
	}
	gs.Shapes, gs.Pictures, gs.GraphicFrames, gs.GroupShapes, gs.ConnectionShapes = sp, pic, gf, grp, cxn
	gs.AltContent, gs.RawXML = ac, raw
	gs.childOrder = order
}

// UnmarshalXML implements custom unmarshaling for GroupShape to preserve child order.
func (gs *GroupShape) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "nvGrpSpPr":
				gs.NvGrpSpPr = &NvGrpSpPr{}
				if err := d.DecodeElement(gs.NvGrpSpPr, &t); err != nil {
					return err
				}
			case "grpSpPr":
				gs.GrpSpPr = &GrpSpPr{}
				if err := d.DecodeElement(gs.GrpSpPr, &t); err != nil {
					return err
				}
			case "sp":
				sp := &Shape{}
				if err := d.DecodeElement(sp, &t); err != nil {
					return err
				}
				gs.childOrder = append(gs.childOrder, ChildRef{ChildSp, len(gs.Shapes)})
				gs.Shapes = append(gs.Shapes, sp)
			case "pic":
				pic := &Picture{}
				if err := d.DecodeElement(pic, &t); err != nil {
					return err
				}
				gs.childOrder = append(gs.childOrder, ChildRef{ChildPic, len(gs.Pictures)})
				gs.Pictures = append(gs.Pictures, pic)
			case "graphicFrame":
				gf := &GraphicFrame{}
				if err := d.DecodeElement(gf, &t); err != nil {
					return err
				}
				gs.childOrder = append(gs.childOrder, ChildRef{ChildGraphicFrame, len(gs.GraphicFrames)})
				gs.GraphicFrames = append(gs.GraphicFrames, gf)
			case "grpSp":
				sub := &GroupShape{}
				if err := d.DecodeElement(sub, &t); err != nil {
					return err
				}
				gs.childOrder = append(gs.childOrder, ChildRef{ChildGrpSp, len(gs.GroupShapes)})
				gs.GroupShapes = append(gs.GroupShapes, sub)
			case "cxnSp":
				cs := &ConnectionShape{}
				if err := d.DecodeElement(cs, &t); err != nil {
					return err
				}
				gs.childOrder = append(gs.childOrder, ChildRef{ChildCxnSp, len(gs.ConnectionShapes)})
				gs.ConnectionShapes = append(gs.ConnectionShapes, cs)
			default:
				if t.Name.Space == xmlb.NSMarkupCompatibility && t.Name.Local == "AlternateContent" {
					ac := &AlternateContent{}
					if err := d.DecodeElement(ac, &t); err != nil {
						return err
					}
					gs.childOrder = append(gs.childOrder, ChildRef{ChildAltContent, len(gs.AltContent)})
					gs.AltContent = append(gs.AltContent, ac)
					continue
				}
				// Unmodeled child (p:contentPart, extLst, ...): capture the
				// whole element raw so a save re-emits it in position (C32).
				raw, err := captureRaw(d, t)
				if err != nil {
					return err
				}
				gs.childOrder = append(gs.childOrder, ChildRef{ChildRawXML, len(gs.RawXML)})
				gs.RawXML = append(gs.RawXML, raw)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler to preserve child element order.
func (gs *GroupShape) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)

	if gs.NvGrpSpPr != nil {
		b.MarshalElement(ns, "nvGrpSpPr", gs.NvGrpSpPr)
	}
	if gs.GrpSpPr != nil {
		b.MarshalElement(ns, "grpSpPr", gs.GrpSpPr)
	}

	if len(gs.childOrder) > 0 {
		for _, ref := range gs.childOrder {
			switch ref.Kind {
			case ChildSp:
				if ref.Index < len(gs.Shapes) {
					b.MarshalElement(ns, "sp", gs.Shapes[ref.Index])
				}
			case ChildPic:
				if ref.Index < len(gs.Pictures) {
					b.MarshalElement(ns, "pic", gs.Pictures[ref.Index])
				}
			case ChildGraphicFrame:
				if ref.Index < len(gs.GraphicFrames) {
					b.MarshalElement(ns, "graphicFrame", gs.GraphicFrames[ref.Index])
				}
			case ChildGrpSp:
				if ref.Index < len(gs.GroupShapes) {
					b.MarshalElement(ns, "grpSp", gs.GroupShapes[ref.Index])
				}
			case ChildAltContent:
				if ref.Index < len(gs.AltContent) {
					b.MarshalElement(xmlb.NSMarkupCompatibility, "AlternateContent", gs.AltContent[ref.Index])
				}
			case ChildRawXML:
				if ref.Index < len(gs.RawXML) {
					b.WriteRaw(gs.RawXML[ref.Index])
				}
			case ChildCxnSp:
				if ref.Index < len(gs.ConnectionShapes) {
					b.MarshalElement(ns, "cxnSp", gs.ConnectionShapes[ref.Index])
				}
			}
		}
	} else {
		for _, sp := range gs.Shapes {
			b.MarshalElement(ns, "sp", sp)
		}
		for _, pic := range gs.Pictures {
			b.MarshalElement(ns, "pic", pic)
		}
		for _, gf := range gs.GraphicFrames {
			b.MarshalElement(ns, "graphicFrame", gf)
		}
		for _, sub := range gs.GroupShapes {
			b.MarshalElement(ns, "grpSp", sub)
		}
		for _, cs := range gs.ConnectionShapes {
			b.MarshalElement(ns, "cxnSp", cs)
		}
		for _, ac := range gs.AltContent {
			b.MarshalElement(xmlb.NSMarkupCompatibility, "AlternateContent", ac)
		}
		for _, raw := range gs.RawXML {
			b.WriteRaw(raw)
		}
	}

	b.EndElement(ns, localName)
}

// NvGrpSpPr contains non-visual group shape properties.
type NvGrpSpPr struct {
	CNvPr      *dml.CNvPr      `xml:"cNvPr"`
	CNvGrpSpPr *dml.CNvGrpSpPr `xml:"cNvGrpSpPr"`
	NvPr       *NvPr           `xml:"nvPr"`
}

// GrpSpPr contains group shape properties (a:CT_GroupShapeProperties).
// It aliases the complete dml type so parsed group fills, effects, scene3d,
// and extLst round-trip instead of being deleted on save (C33).
type GrpSpPr = dml.GrpSpPr
