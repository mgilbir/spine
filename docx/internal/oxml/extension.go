package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// WExtensionList contains WML extension elements (w:extLst).
type WExtensionList struct {
	Ext []WExtension `xml:"ext"`
}

// WExtension represents a single extension in a WML extension list.
// Unknown extensions use RawContent for round-trip preservation.
type WExtension struct {
	URI        string `xml:"uri,attr"`
	RawContent []byte `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for WExtension.
func (e *WExtension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "uri" {
			e.URI = attr.Value
		}
	}

	// Preserve raw content for all extensions
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	e.RawContent = inner.Content
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for WExtension.
func (e *WExtension) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName, xmlb.StrAttr("uri", e.URI))
	if len(e.RawContent) > 0 {
		b.WriteRaw(e.RawContent)
	}
	b.EndElement(ns, localName)
}
