package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// Picture represents a picture element (p:pic) in a slide.
type Picture struct {
	XMLName  xml.Name      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pic"`
	NvPicPr  *NvPicPr      `xml:"nvPicPr"`
	BlipFill *dml.BlipFill `xml:"http://schemas.openxmlformats.org/presentationml/2006/main blipFill"`
	SpPr     *dml.SpPr     `xml:"spPr"`
	Style    *dml.Style    `xml:"style,omitempty"`
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
	NvCxnSpPr *NvCxnSpPr    `xml:"nvCxnSpPr"`
	SpPr      *dml.SpPr     `xml:"spPr"`
	Style     *dml.Style    `xml:"style,omitempty"`
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
	childOrder       []ChildRef
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
				if err := d.Skip(); err != nil {
					return err
				}
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
	}

	b.EndElement(ns, localName)
}

// NvGrpSpPr contains non-visual group shape properties.
type NvGrpSpPr struct {
	CNvPr      *dml.CNvPr      `xml:"cNvPr"`
	CNvGrpSpPr *dml.CNvGrpSpPr `xml:"cNvGrpSpPr"`
	NvPr       *NvPr           `xml:"nvPr"`
}

// GrpSpPr contains group shape properties.
type GrpSpPr struct {
	BwMode string        `xml:"bwMode,attr,omitempty"`
	Xfrm   *dml.GrpXfrm `xml:"http://schemas.openxmlformats.org/drawingml/2006/main xfrm,omitempty"`
}
