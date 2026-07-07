package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_CustomXml represents a custom XML element (w:customXml).
type CT_CustomXml struct {
	URI         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uri,attr,omitempty"`
	Element     string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main element,attr,omitempty"`
	CustomXmlPr *CT_CustomXmlPr `xml:"-"`
	// Block content (paragraphs, tables, etc.) within the customXml
	P          []CT_P   `xml:"-"`
	CustomXml  []CT_CustomXml `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_CustomXml.
func (cx *CT_CustomXml) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "uri":
			cx.URI = attr.Value
		case "element":
			cx.Element = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "customXmlPr":
				var pr CT_CustomXmlPr
				if err := d.DecodeElement(&pr, &t); err != nil {
					return err
				}
				cx.CustomXmlPr = &pr
			case "p":
				var p CT_P
				if err := d.DecodeElement(&p, &t); err != nil {
					return err
				}
				cx.P = append(cx.P, p)
			case "customXml":
				var nested CT_CustomXml
				if err := d.DecodeElement(&nested, &t); err != nil {
					return err
				}
				cx.CustomXml = append(cx.CustomXml, nested)
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

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_CustomXml.
func (cx *CT_CustomXml) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if cx.URI != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "uri", Value: cx.URI})
	}
	if cx.Element != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "element", Value: cx.Element})
	}
	b.StartElement(ns, localName, attrs...)
	if cx.CustomXmlPr != nil {
		b.MarshalElement(ns, "customXmlPr", cx.CustomXmlPr)
	}
	for i := range cx.P {
		cx.P[i].MarshalToBuilder(b, ns, "p")
	}
	for i := range cx.CustomXml {
		cx.CustomXml[i].MarshalToBuilder(b, ns, "customXml")
	}
	b.EndElement(ns, localName)
}

// CT_CustomXmlPr represents custom XML properties (w:customXmlPr).
type CT_CustomXmlPr struct {
	Placeholder *CT_String     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main placeholder,omitempty"`
	Attr        []CT_CustomXmlAttr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main attr,omitempty"`
}

// CT_CustomXmlAttr represents a custom XML attribute (w:attr).
type CT_CustomXmlAttr struct {
	URI  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uri,attr,omitempty"`
	Name string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}
