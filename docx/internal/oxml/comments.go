package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Comments is the root element of the comments part (word/comments.xml).
type CT_Comments struct {
	XMLName xml.Name      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main comments"`
	Comment []*CT_Comment `xml:"-"`
	// OriginalNSDecls and OriginalRootAttrs preserve the source root's namespace
	// declarations and verbatim attribute list so a regenerated comments.xml
	// (after a comment is added/edited) keeps its prefixes resolving. Nil for a
	// part created from scratch, which gets a standard declaration set.
	OriginalNSDecls   []xmlb.NSDecl   `xml:"-"`
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Comments, capturing the
// root's namespace declarations for round-trip fidelity.
func (c *CT_Comments) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.XMLName = start.Name
	c.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Space == "xmlns" {
			c.OriginalNSDecls = append(c.OriginalNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		} else if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
			c.OriginalNSDecls = append(c.OriginalNSDecls, xmlb.NSDecl{Prefix: "", URI: attr.Value})
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "comment" {
				cm := &CT_Comment{}
				if err := d.DecodeElement(cm, &t); err != nil {
					return err
				}
				c.Comment = append(c.Comment, cm)
			} else if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MaxID returns the highest numeric w:id among the comments, or -1 if none of
// them carry a numeric id. Used to allocate the next comment id.
func (c *CT_Comments) MaxID() int {
	max := -1
	for _, cm := range c.Comment {
		if n, ok := atoiOK(cm.Id); ok && n > max {
			max = n
		}
	}
	return max
}

// CT_Comment represents a single comment (w:comment). Its block content (the
// paragraphs) is the comment body.
type CT_Comment struct {
	Id       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	Initials string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main initials,attr,omitempty"`
	// CapturedAttrs preserves the verbatim source attribute list. w:comment is
	// an annotation record like the *Change family: Word 2021+ writes an
	// unmodeled w16du:dateUtc alongside w:date, and producer attribute order
	// varies (C411).
	CapturedAttrs []xmlb.RootAttr `xml:"-"`

	P             []*CT_P               `xml:"-"`
	Tbl           []*CT_Tbl             `xml:"-"`
	SdtBlock      []*CT_SdtBlock        `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
	childOrder    []bodyChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_Comment.
func (c *CT_Comment) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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
	return unmarshalBodyContent(d, &c.P, &c.Tbl, &c.SdtBlock, &c.BookmarkStart, &c.BookmarkEnd, &c.Raw, &c.childOrder)
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
	if c.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(c.CapturedAttrs, attrs)
	}
	b.StartElement(ns, localName, attrs...)
	marshalBodyContent(b, ns, c.P, c.Tbl, c.SdtBlock, c.BookmarkStart, c.BookmarkEnd, c.Raw, c.childOrder)
	b.EndElement(ns, localName)
}

// AppendP appends a paragraph to the comment body, maintaining child order.
func (c *CT_Comment) AppendP(p *CT_P) {
	backfillBodyChildOrder(&c.childOrder, c.P, c.Tbl, c.SdtBlock, c.BookmarkStart, c.BookmarkEnd, c.Raw)
	appendBodyP(&c.P, &c.childOrder, p)
}

// LastParaID returns the w14:paraId of the comment's last body paragraph, which
// is the value commentsExtended threads on. Empty if the comment has no
// paragraph or the last paragraph carries no paraId.
func (c *CT_Comment) LastParaID() string {
	for i := len(c.P) - 1; i >= 0; i-- {
		if c.P[i] != nil {
			return c.P[i].ParaId
		}
	}
	return ""
}

// Text returns the concatenated visible text of the comment body paragraphs,
// joined with newlines.
func (c *CT_Comment) Text() string {
	out := ""
	for i, p := range c.P {
		if p == nil {
			continue
		}
		if i > 0 {
			out += "\n"
		}
		out += p.Text()
	}
	return out
}

// atoiOK parses a base-10 integer, reporting whether the whole string parsed.
func atoiOK(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	neg := false
	for i, r := range s {
		if i == 0 && r == '-' {
			neg = true
			continue
		}
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}
