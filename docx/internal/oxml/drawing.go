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
