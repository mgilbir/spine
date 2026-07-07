package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Object represents an embedded object (w:object).
type CT_Object struct {
	DxaOrig string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dxaOrig,attr,omitempty"`
	DyaOrig string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dyaOrig,attr,omitempty"`
}

// CT_ObjectEmbed represents an embedded OLE object (w:objectEmbed).
type CT_ObjectEmbed struct {
	RID       string `xml:"-"` // r:id attr
	DrawAspect string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main drawAspect,attr,omitempty"`
	ProgID    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main progId,attr,omitempty"`
	ShapeID   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shapeId,attr,omitempty"`
	FieldCodes string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fieldCodes,attr,omitempty"`
}

func (o *CT_ObjectEmbed) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "drawAspect":
			o.DrawAspect = attr.Value
		case attr.Name.Local == "progId":
			o.ProgID = attr.Value
		case attr.Name.Local == "shapeId":
			o.ShapeID = attr.Value
		case attr.Name.Local == "fieldCodes":
			o.FieldCodes = attr.Value
		case attr.Name.Local == "id" && (attr.Name.Space == NsRelationships || attr.Name.Space == "r"):
			o.RID = attr.Value
		}
	}
	return d.Skip()
}

func (o *CT_ObjectEmbed) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if o.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: o.RID})
	}
	if o.DrawAspect != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "drawAspect", Value: o.DrawAspect})
	}
	if o.ProgID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "progId", Value: o.ProgID})
	}
	if o.ShapeID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "shapeId", Value: o.ShapeID})
	}
	if o.FieldCodes != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "fieldCodes", Value: o.FieldCodes})
	}
	b.EmptyElement(ns, localName, attrs...)
}

// CT_ObjectLink represents a linked OLE object (w:objectLink).
type CT_ObjectLink struct {
	RID         string `xml:"-"` // r:id attr
	DrawAspect  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main drawAspect,attr,omitempty"`
	ProgID      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main progId,attr,omitempty"`
	ShapeID     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shapeId,attr,omitempty"`
	FieldCodes  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fieldCodes,attr,omitempty"`
	UpdateMode  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main updateMode,attr,omitempty"`
	LockedField string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lockedField,attr,omitempty"`
}

func (o *CT_ObjectLink) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "drawAspect":
			o.DrawAspect = attr.Value
		case attr.Name.Local == "progId":
			o.ProgID = attr.Value
		case attr.Name.Local == "shapeId":
			o.ShapeID = attr.Value
		case attr.Name.Local == "fieldCodes":
			o.FieldCodes = attr.Value
		case attr.Name.Local == "updateMode":
			o.UpdateMode = attr.Value
		case attr.Name.Local == "lockedField":
			o.LockedField = attr.Value
		case attr.Name.Local == "id" && (attr.Name.Space == NsRelationships || attr.Name.Space == "r"):
			o.RID = attr.Value
		}
	}
	return d.Skip()
}

func (o *CT_ObjectLink) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if o.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: o.RID})
	}
	if o.DrawAspect != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "drawAspect", Value: o.DrawAspect})
	}
	if o.ProgID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "progId", Value: o.ProgID})
	}
	if o.ShapeID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "shapeId", Value: o.ShapeID})
	}
	if o.FieldCodes != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "fieldCodes", Value: o.FieldCodes})
	}
	if o.UpdateMode != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "updateMode", Value: o.UpdateMode})
	}
	if o.LockedField != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "lockedField", Value: o.LockedField})
	}
	b.EmptyElement(ns, localName, attrs...)
}
