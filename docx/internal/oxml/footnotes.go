package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Footnotes is the root element of the footnotes part.
type CT_Footnotes struct {
	XMLName  xml.Name     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main footnotes"`
	Footnote []*CT_FtnEdn `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main footnote"`
}

// CT_Endnotes is the root element of the endnotes part.
type CT_Endnotes struct {
	XMLName xml.Name     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main endnotes"`
	Endnote []*CT_FtnEdn `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main endnote"`
}

// CT_FtnEdn represents a single footnote or endnote.
type CT_FtnEdn struct {
	Type string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	Id   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`

	P             []*CT_P             `xml:"-"`
	Tbl           []*CT_Tbl           `xml:"-"`
	SdtBlock      []*CT_SdtBlock      `xml:"-"`
	BookmarkStart []*CT_BookmarkStart `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd   `xml:"-"`
	childOrder    []bodyChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_FtnEdn.
func (f *CT_FtnEdn) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "type":
			f.Type = attr.Value
		case "id":
			f.Id = attr.Value
		}
	}
	return unmarshalBodyContent(d, &f.P, &f.Tbl, &f.SdtBlock, &f.BookmarkStart, &f.BookmarkEnd, &f.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_FtnEdn.
func (f *CT_FtnEdn) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if f.Type != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "type", Value: f.Type})
	}
	attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "id", Value: f.Id})
	b.StartElement(ns, localName, attrs...)
	marshalBodyContent(b, ns, f.P, f.Tbl, f.SdtBlock, f.BookmarkStart, f.BookmarkEnd, f.childOrder)
	b.EndElement(ns, localName)
}
