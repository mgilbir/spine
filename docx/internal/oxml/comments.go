package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Comments is the root element of the comments part.
type CT_Comments struct {
	XMLName xml.Name      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main comments"`
	Comment []*CT_Comment `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main comment"`
}

// CT_Comment represents a single comment.
type CT_Comment struct {
	Id     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	Initials string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main initials,attr,omitempty"`

	P             []*CT_P             `xml:"-"`
	Tbl           []*CT_Tbl           `xml:"-"`
	SdtBlock      []*CT_SdtBlock      `xml:"-"`
	BookmarkStart []*CT_BookmarkStart `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd   `xml:"-"`
	childOrder    []bodyChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_Comment.
func (c *CT_Comment) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "id":
			c.Id = attr.Value
		case "author":
			c.Author = attr.Value
		case "date":
			c.Date = attr.Value
		case "initials":
			c.Initials = attr.Value
		}
	}
	return unmarshalBodyContent(d, &c.P, &c.Tbl, &c.SdtBlock, &c.BookmarkStart, &c.BookmarkEnd, &c.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Comment.
func (c *CT_Comment) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "id", Value: c.Id})
	attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "author", Value: c.Author})
	if c.Date != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "date", Value: c.Date})
	}
	if c.Initials != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "initials", Value: c.Initials})
	}
	b.StartElement(ns, localName, attrs...)
	marshalBodyContent(b, ns, c.P, c.Tbl, c.SdtBlock, c.BookmarkStart, c.BookmarkEnd, c.childOrder)
	b.EndElement(ns, localName)
}
