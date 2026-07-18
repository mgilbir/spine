package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Footnotes is the root element of the footnotes part.
type CT_Footnotes struct {
	XMLName  xml.Name     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main footnotes"`
	Footnote []*CT_FtnEdn `xml:"-"`
	// OriginalNSDecls and OriginalRootAttrs preserve the source root's namespace
	// declarations and verbatim attribute list so a regenerated footnotes.xml
	// (after a footnote is added) keeps its prefixes resolving. Nil for a part
	// created from scratch, which gets a standard declaration set.
	OriginalNSDecls   []xmlb.NSDecl   `xml:"-"`
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the root's namespace declarations and verbatim
// attribute list for round-trip fidelity, then decodes each w:footnote child.
func (f *CT_Footnotes) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	f.XMLName = start.Name
	f.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	f.OriginalNSDecls = captureRootNSDecls(start.Attr)
	return unmarshalNotes(d, "footnote", &f.Footnote)
}

// MaxID returns the highest numeric w:id among the notes, or -1 if none carry a
// numeric id. Standard separator notes (id -1, 0) are included, so a fresh
// user footnote is allocated id max+1 (>= 1).
func (f *CT_Footnotes) MaxID() int { return maxNoteID(f.Footnote) }

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Footnotes, emitting
// each w:footnote child. The standalone-part marshaler (marshalFootnotesXML)
// supplies the root namespace declarations; here only the element structure is
// written so the type also round-trips through b.MarshalElement.
func (f *CT_Footnotes) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	for _, n := range f.Footnote {
		n.MarshalToBuilder(b, ns, "footnote")
	}
	b.EndElement(ns, localName)
}

// CT_Endnotes is the root element of the endnotes part.
type CT_Endnotes struct {
	XMLName xml.Name     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main endnotes"`
	Endnote []*CT_FtnEdn `xml:"-"`

	OriginalNSDecls   []xmlb.NSDecl   `xml:"-"`
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the root attributes for round-trip fidelity, then
// decodes each w:endnote child.
func (e *CT_Endnotes) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	e.XMLName = start.Name
	e.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	e.OriginalNSDecls = captureRootNSDecls(start.Attr)
	return unmarshalNotes(d, "endnote", &e.Endnote)
}

// MaxID returns the highest numeric w:id among the endnotes, or -1 if none.
func (e *CT_Endnotes) MaxID() int { return maxNoteID(e.Endnote) }

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Endnotes (see
// CT_Footnotes.MarshalToBuilder).
func (e *CT_Endnotes) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	for _, n := range e.Endnote {
		n.MarshalToBuilder(b, ns, "endnote")
	}
	b.EndElement(ns, localName)
}

// captureRootNSDecls extracts the namespace declarations (default and prefixed)
// from a root element's attribute list.
func captureRootNSDecls(attrs []xml.Attr) []xmlb.NSDecl {
	var out []xmlb.NSDecl
	for _, attr := range attrs {
		if attr.Name.Space == "xmlns" {
			out = append(out, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		} else if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
			out = append(out, xmlb.NSDecl{Prefix: "", URI: attr.Value})
		}
	}
	return out
}

// unmarshalNotes decodes the notes of a footnotes/endnotes root, decoding the
// child element named local into out and skipping anything else.
func unmarshalNotes(d *xml.Decoder, local string, out *[]*CT_FtnEdn) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == local {
				n := &CT_FtnEdn{}
				if err := d.DecodeElement(n, &t); err != nil {
					return err
				}
				*out = append(*out, n)
			} else if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// maxNoteID returns the highest numeric w:id among notes, or -1 if none parse.
func maxNoteID(notes []*CT_FtnEdn) int {
	max := -1
	for _, n := range notes {
		if v, ok := atoiOK(n.Id); ok && v > max {
			max = v
		}
	}
	return max
}

// CT_FtnEdn represents a single footnote or endnote.
type CT_FtnEdn struct {
	Type string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	Id   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`

	P             []*CT_P               `xml:"-"`
	Tbl           []*CT_Tbl             `xml:"-"`
	SdtBlock      []*CT_SdtBlock        `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
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
	return unmarshalBodyContent(d, &f.P, &f.Tbl, &f.SdtBlock, &f.BookmarkStart, &f.BookmarkEnd, &f.Raw, &f.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_FtnEdn.
func (f *CT_FtnEdn) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if f.Type != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "type", Value: f.Type})
	}
	attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "id", Value: f.Id})
	b.StartElement(ns, localName, attrs...)
	marshalBodyContent(b, ns, f.P, f.Tbl, f.SdtBlock, f.BookmarkStart, f.BookmarkEnd, f.Raw, f.childOrder)
	b.EndElement(ns, localName)
}

// AppendP appends a paragraph to the note body, maintaining child order.
func (f *CT_FtnEdn) AppendP(p *CT_P) {
	backfillBodyChildOrder(&f.childOrder, f.P, f.Tbl, f.SdtBlock, f.BookmarkStart, f.BookmarkEnd, f.Raw)
	appendBodyP(&f.P, &f.childOrder, p)
}

// Text returns the concatenated visible text of the note body paragraphs,
// joined with newlines.
func (f *CT_FtnEdn) Text() string {
	out := ""
	for i, p := range f.P {
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
