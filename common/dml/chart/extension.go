package chart

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// ExtLst represents CT_ExtensionList from dml-chart.xsd (c:extLst).
// Chart extension lists hold c:ext children — not a:ext — so they need their
// own type: borrowing dml.ExtLst (whose Ext matches the DrawingML-main
// namespace) parses chart extensions to nothing and deletes them on save.
type ExtLst struct {
	Ext []*Ext `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ext,omitempty"`
}

// Ext represents CT_Extension from dml-chart.xsd (c:ext). The XSD defines the
// content as <xsd:any processContents="lax"/>; the content is preserved as
// raw bytes, along with any xmlns declarations carried on the ext element so
// prefixes used by the content stay bound when re-emitted.
type Ext struct {
	URI           string        `xml:"uri,attr"`
	RawContent    []byte        `xml:"-"`
	InlineNSDecls []xmlb.NSDecl `xml:"-"`
}

// UnmarshalXML captures the uri attribute, inline xmlns declarations, and the
// extension content verbatim.
func (e *Ext) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "uri" && attr.Name.Space == "":
			e.URI = attr.Value
		case attr.Name.Space == "xmlns":
			e.InlineNSDecls = append(e.InlineNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			// A default declaration matching the chart namespace is the
			// container's own namespace (always in scope); re-declaring it
			// would be redundant, so only foreign defaults are kept.
			if attr.Value != xmlb.NSDrawingMLChart {
				e.InlineNSDecls = append(e.InlineNSDecls, xmlb.NSDecl{URI: attr.Value})
			}
		}
	}

	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	e.RawContent = inner.Content
	return nil
}

// MarshalXML implements xml.Marshaler, re-emitting the captured content and
// namespace declarations verbatim.
func (e *Ext) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "uri"}, Value: e.URI})
	for _, d := range e.InlineNSDecls {
		name := "xmlns"
		if d.Prefix != "" {
			name = "xmlns:" + d.Prefix
		}
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: name}, Value: d.URI})
	}
	aux := struct {
		Content []byte `xml:",innerxml"`
	}{e.RawContent}
	return enc.EncodeElement(&aux, start)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for Builder-based
// serialization (mirrors the encoding/xml path).
func (e *Ext) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := xmlb.NSDeclAttrs([]xmlb.Attr{xmlb.StrAttr("uri", e.URI)}, e.InlineNSDecls)
	if len(e.RawContent) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	b.WriteRaw(e.RawContent)
	b.EndElement(ns, localName)
}
