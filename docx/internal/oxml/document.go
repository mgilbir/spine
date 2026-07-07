package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Document is the root element of a document part (w:document).
type CT_Document struct {
	XMLName xml.Name `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main document"`
	// Ignorable is the mc:Ignorable attribute value.
	Ignorable string `xml:"-"`
	// Conformance is the w:conformance attribute (e.g. "strict"/"transitional").
	Conformance string        `xml:"-"`
	Body        *CT_Body      `xml:"-"`
	Background  *CT_Background `xml:"-"`
	// OriginalNSDecls preserves the namespace declarations from the original XML
	// for byte-identical round-trip of document.xml.
	OriginalNSDecls []xmlb.NSDecl `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Document.
func (doc *CT_Document) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	doc.XMLName = start.Name
	for _, attr := range start.Attr {
		if attr.Name.Local == "Ignorable" {
			doc.Ignorable = attr.Value
		}
		if attr.Name.Local == "conformance" {
			doc.Conformance = attr.Value
		}
		// Capture namespace declarations for round-trip preservation
		if attr.Name.Space == "xmlns" {
			doc.OriginalNSDecls = append(doc.OriginalNSDecls, xmlb.NSDecl{
				Prefix: attr.Name.Local,
				URI:    attr.Value,
			})
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
			case "body":
				doc.Body = &CT_Body{}
				if err := d.DecodeElement(doc.Body, &t); err != nil {
					return err
				}
			case "background":
				doc.Background = &CT_Background{}
				if err := d.DecodeElement(doc.Background, &t); err != nil {
					return err
				}
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

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Document.
func (doc *CT_Document) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	if doc.Background != nil {
		b.MarshalElement(ns, "background", doc.Background)
	}
	if doc.Body != nil {
		doc.Body.MarshalToBuilder(b, ns, "body")
	}
	b.EndElement(ns, localName)
}

// CT_Background represents the document background.
type CT_Background struct {
	Color      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	ThemeColor string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeTint  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeTint,attr,omitempty"`
	ThemeShade string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeShade,attr,omitempty"`
}

// bodyChildKind identifies body-level child element types.
type bodyChildKind int

const (
	bodyChildP bodyChildKind = iota
	bodyChildTbl
	bodyChildSdt
	bodyChildBookmarkStart
	bodyChildBookmarkEnd
)

// bodyChildRef references a body child element.
type bodyChildRef struct {
	kind  bodyChildKind
	index int
}

// CT_Body represents the document body (w:body).
type CT_Body struct {
	P             []*CT_P             `xml:"-"`
	Tbl           []*CT_Tbl           `xml:"-"`
	SdtBlock      []*CT_SdtBlock      `xml:"-"`
	BookmarkStart []*CT_BookmarkStart  `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd    `xml:"-"`
	SectPr        *CT_SectPr          `xml:"-"`
	childOrder    []bodyChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_Body to preserve child order.
func (body *CT_Body) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "sectPr":
				body.SectPr = &CT_SectPr{}
				if err := d.DecodeElement(body.SectPr, &t); err != nil {
					return err
				}
			default:
				if err := unmarshalBodyChild(d, &t, &body.P, &body.Tbl, &body.SdtBlock, &body.BookmarkStart, &body.BookmarkEnd, &body.childOrder); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Body.
func (body *CT_Body) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	marshalBodyContent(b, ns, body.P, body.Tbl, body.SdtBlock, body.BookmarkStart, body.BookmarkEnd, body.childOrder)
	if body.SectPr != nil {
		body.SectPr.MarshalToBuilder(b, ns, "sectPr")
	}
	b.EndElement(ns, localName)
}

// appendBodyP appends a paragraph to a body-level container, recording it in the
// child order. The marshal walks childOrder, so a container parsed from a file
// (whose order is already populated) would otherwise drop appended content.
// Appending to a nil order also starts tracking it, which keeps paragraph/table
// interleaving correct for containers built from scratch.
func appendBodyP(p *[]*CT_P, childOrder *[]bodyChildRef, para *CT_P) {
	*childOrder = append(*childOrder, bodyChildRef{bodyChildP, len(*p)})
	*p = append(*p, para)
}

// appendBodyTbl appends a table to a body-level container, recording it in the
// child order (see appendBodyP).
func appendBodyTbl(tbl *[]*CT_Tbl, childOrder *[]bodyChildRef, t *CT_Tbl) {
	*childOrder = append(*childOrder, bodyChildRef{bodyChildTbl, len(*tbl)})
	*tbl = append(*tbl, t)
}

// AppendP appends a paragraph to the document body, maintaining child order.
func (body *CT_Body) AppendP(p *CT_P) { appendBodyP(&body.P, &body.childOrder, p) }

// AppendTbl appends a table to the document body, maintaining child order.
func (body *CT_Body) AppendTbl(t *CT_Tbl) { appendBodyTbl(&body.Tbl, &body.childOrder, t) }

// unmarshalBodyChild handles a single body-level child element start tag.
// The decoder is positioned at the start element; this function decodes or skips it.
func unmarshalBodyChild(d *xml.Decoder, t *xml.StartElement,
	p *[]*CT_P, tbl *[]*CT_Tbl, sdtBlock *[]*CT_SdtBlock,
	bookmarkStart *[]*CT_BookmarkStart, bookmarkEnd *[]*CT_BookmarkEnd,
	childOrder *[]bodyChildRef,
) error {
	switch t.Name.Local {
	case "p":
		v := &CT_P{}
		if err := d.DecodeElement(v, t); err != nil {
			return err
		}
		*childOrder = append(*childOrder, bodyChildRef{bodyChildP, len(*p)})
		*p = append(*p, v)
	case "tbl":
		v := &CT_Tbl{}
		if err := d.DecodeElement(v, t); err != nil {
			return err
		}
		*childOrder = append(*childOrder, bodyChildRef{bodyChildTbl, len(*tbl)})
		*tbl = append(*tbl, v)
	case "sdt":
		if sdtBlock != nil {
			v := &CT_SdtBlock{}
			if err := d.DecodeElement(v, t); err != nil {
				return err
			}
			*childOrder = append(*childOrder, bodyChildRef{bodyChildSdt, len(*sdtBlock)})
			*sdtBlock = append(*sdtBlock, v)
		} else {
			if err := d.Skip(); err != nil {
				return err
			}
		}
	case "bookmarkStart":
		if bookmarkStart != nil {
			v := &CT_BookmarkStart{}
			if err := d.DecodeElement(v, t); err != nil {
				return err
			}
			*childOrder = append(*childOrder, bodyChildRef{bodyChildBookmarkStart, len(*bookmarkStart)})
			*bookmarkStart = append(*bookmarkStart, v)
		} else {
			if err := d.Skip(); err != nil {
				return err
			}
		}
	case "bookmarkEnd":
		if bookmarkEnd != nil {
			v := &CT_BookmarkEnd{}
			if err := d.DecodeElement(v, t); err != nil {
				return err
			}
			*childOrder = append(*childOrder, bodyChildRef{bodyChildBookmarkEnd, len(*bookmarkEnd)})
			*bookmarkEnd = append(*bookmarkEnd, v)
		} else {
			if err := d.Skip(); err != nil {
				return err
			}
		}
	default:
		if err := d.Skip(); err != nil {
			return err
		}
	}
	return nil
}

// marshalBodyContent writes body-level content using child order tracking.
func marshalBodyContent(b *xmlb.Builder, ns string,
	p []*CT_P, tbl []*CT_Tbl, sdtBlock []*CT_SdtBlock,
	bookmarkStart []*CT_BookmarkStart, bookmarkEnd []*CT_BookmarkEnd,
	childOrder []bodyChildRef,
) {
	if len(childOrder) > 0 {
		for _, ref := range childOrder {
			switch ref.kind {
			case bodyChildP:
				if ref.index < len(p) {
					p[ref.index].MarshalToBuilder(b, ns, "p")
				}
			case bodyChildTbl:
				if ref.index < len(tbl) {
					tbl[ref.index].MarshalToBuilder(b, ns, "tbl")
				}
			case bodyChildSdt:
				if ref.index < len(sdtBlock) {
					b.MarshalElement(ns, "sdt", sdtBlock[ref.index])
				}
			case bodyChildBookmarkStart:
				if ref.index < len(bookmarkStart) {
					b.MarshalElement(ns, "bookmarkStart", bookmarkStart[ref.index])
				}
			case bodyChildBookmarkEnd:
				if ref.index < len(bookmarkEnd) {
					b.MarshalElement(ns, "bookmarkEnd", bookmarkEnd[ref.index])
				}
			}
		}
	} else {
		for _, v := range p {
			v.MarshalToBuilder(b, ns, "p")
		}
		for _, v := range tbl {
			v.MarshalToBuilder(b, ns, "tbl")
		}
	}
}
