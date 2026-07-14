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
	// CapturedAttrs preserves the verbatim attribute list of the w:drawing
	// element itself (some producers declare xmlns:a inline on it).
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Drawing.
func (dr *CT_Drawing) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	dr.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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
	var attrs []xmlb.Attr
	if dr.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(dr.CapturedAttrs, attrs)
	}
	b.StartElement(ns, localName, attrs...)
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
	// ElemPrefix is the element name's verbatim source prefix. It preserves
	// the producer's choice when several prefixes bind one URI (Word 2007
	// files alias markup-compatibility as both mc and ve).
	ElemPrefix         string `xml:"-"`
	elemPrefixCaptured bool
}

// UnmarshalXML implements custom unmarshaling for CT_RawElement.
func (re *CT_RawElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	re.ElemPrefix, re.elemPrefixCaptured = xmlb.ElementPrefix(d)
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

// CT_RawNamedElement pairs a raw-captured element with the name it was read
// under. It is used by containers that funnel several raw-preserved child
// kinds into a single ordered slice (body-level w:altChunk/w:customXml,
// inline w:customXml/w:smartTag/w:moveTo/w:moveFrom and their range markers,
// settings children), so a regenerated part re-emits each element under its
// own name and namespace.
type CT_RawNamedElement struct {
	Local string `xml:"-"`
	// Space is the element's namespace URI; empty means the parent's namespace.
	Space string `xml:"-"`
	CT_RawElement
}

// UnmarshalXML captures the element name alongside its attributes and content.
func (rn *CT_RawNamedElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	rn.Local = start.Name.Local
	rn.Space = start.Name.Space
	return rn.CT_RawElement.UnmarshalXML(d, start)
}

// MarshalNamed writes the element under its captured namespace, defaulting to
// the given parent namespace when none was recorded.
func (rn *CT_RawNamedElement) MarshalNamed(b *xmlb.Builder, parentNS string) {
	ns := rn.Space
	if ns == "" {
		ns = parentNS
	}
	rn.MarshalToBuilder(b, ns, rn.Local)
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
	if re.elemPrefixCaptured && re.ElemPrefix != "" {
		// Replay the element under its verbatim source prefix: with two
		// prefixes bound to one URI the resolver could pick the other one.
		lit := b.QualifyAttrs(attrs)
		if len(re.RawContent) == 0 {
			b.EmptyElementLiteral(re.ElemPrefix, localName, lit...)
			return
		}
		b.StartElementLiteral(re.ElemPrefix, localName, nil, lit...)
		b.WriteRaw(re.RawContent)
		b.EndElementLiteral(re.ElemPrefix, localName)
		return
	}
	if len(re.RawContent) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	b.WriteRaw(re.RawContent)
	b.EndElement(ns, localName)
}
