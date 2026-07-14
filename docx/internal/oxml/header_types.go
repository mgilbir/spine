package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_HeaderReference represents a standalone header/footer reference (w:headerReference, w:footerReference).
// Used for spec test mapping when headerReference appears as a root element.
// This differs from CT_HdrFtrRef which is used inside CT_SectPr with custom marshal.
type CT_HeaderReference struct {
	Type string `xml:"-"` // w:type attr
	RID  string `xml:"-"` // r:id attr
	// CapturedAttrs preserves the verbatim source attribute list; replayed
	// on marshal (producers disagree on r:id vs w:type order).
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_HeaderReference to handle r:id attributes.
func (h *CT_HeaderReference) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	h.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "type":
			h.Type = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			h.RID = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == "r":
			h.RID = attr.Value
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_HeaderReference.
func (h *CT_HeaderReference) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if h.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: h.RID})
	}
	if h.Type != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "type", Value: h.Type})
	}
	if h.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(h.CapturedAttrs, attrs)
	}
	b.EmptyElement(ns, localName, attrs...)
}
