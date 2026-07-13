package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Drawing represents a drawing wrapper element (w:drawing).
// Contains wp:inline or wp:anchor from the wordprocessingDrawing namespace.
// The drawing content is preserved as raw XML.
type CT_Drawing struct {
	RawContent []byte `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Drawing.
func (dr *CT_Drawing) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	dr.RawContent = inner.Content
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Drawing.
func (dr *CT_Drawing) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	b.WriteRaw(dr.RawContent)
	b.EndElement(ns, localName)
}

// CT_RawElement preserves an element verbatim: its attributes (including any
// inline xmlns declarations) and its inner XML. It is the CT_Drawing raw
// capture pattern extended with attribute preservation, used for run children
// whose content the model does not type (w:pict VML content, w:object OLE
// wrappers) so a regenerated document.xml does not silently delete them.
type CT_RawElement struct {
	Attrs      []xml.Attr `xml:"-"`
	RawContent []byte     `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_RawElement.
func (re *CT_RawElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	re.Attrs = append(re.Attrs[:0], start.Attr...)
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	re.RawContent = inner.Content
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_RawElement.
func (re *CT_RawElement) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := make([]xmlb.Attr, 0, len(re.Attrs))
	for _, a := range re.Attrs {
		switch {
		case a.Name.Space == "xmlns":
			// Prefixed namespace declaration (xmlns:v="...").
			attrs = append(attrs, xmlb.Attr{Name: "xmlns:" + a.Name.Local, Value: a.Value})
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			// Default namespace declaration.
			attrs = append(attrs, xmlb.Attr{Name: "xmlns", Value: a.Value})
		default:
			attrs = append(attrs, xmlb.Attr{Namespace: a.Name.Space, Name: a.Name.Local, Value: a.Value})
		}
	}
	if len(re.RawContent) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	b.WriteRaw(re.RawContent)
	b.EndElement(ns, localName)
}
