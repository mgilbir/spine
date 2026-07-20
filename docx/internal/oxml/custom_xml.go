package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_CustomXml represents a custom XML element (w:customXml).
type CT_CustomXml struct {
	URI         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uri,attr,omitempty"`
	Element     string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main element,attr,omitempty"`
	CustomXmlPr *CT_CustomXmlPr `xml:"-"`
	// Block content (paragraphs, tables, etc.) within the customXml
	P         []CT_P         `xml:"-"`
	CustomXml []CT_CustomXml `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_CustomXml.
func (cx *CT_CustomXml) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "uri":
			cx.URI = attr.Value
		case "element":
			cx.Element = attr.Value
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
			case "customXmlPr":
				var pr CT_CustomXmlPr
				if err := d.DecodeElement(&pr, &t); err != nil {
					return err
				}
				cx.CustomXmlPr = &pr
			case "p":
				var p CT_P
				if err := d.DecodeElement(&p, &t); err != nil {
					return err
				}
				cx.P = append(cx.P, p)
			case "customXml":
				var nested CT_CustomXml
				if err := d.DecodeElement(&nested, &t); err != nil {
					return err
				}
				cx.CustomXml = append(cx.CustomXml, nested)
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

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_CustomXml.
func (cx *CT_CustomXml) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if cx.URI != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "uri", Value: cx.URI})
	}
	if cx.Element != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "element", Value: cx.Element})
	}
	b.StartElement(ns, localName, attrs...)
	if cx.CustomXmlPr != nil {
		b.MarshalElement(ns, "customXmlPr", cx.CustomXmlPr)
	}
	for i := range cx.P {
		cx.P[i].MarshalToBuilder(b, ns, "p")
	}
	for i := range cx.CustomXml {
		cx.CustomXml[i].MarshalToBuilder(b, ns, "customXml")
	}
	b.EndElement(ns, localName)
}

// CT_CustomXmlPr represents custom XML properties (w:customXmlPr).
type CT_CustomXmlPr struct {
	Placeholder *CT_String         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main placeholder,omitempty"`
	Attr        []CT_CustomXmlAttr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main attr,omitempty"`
}

// CT_CustomXmlAttr represents a custom XML attribute (w:attr).
type CT_CustomXmlAttr struct {
	URI  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uri,attr,omitempty"`
	Name string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// DataBinding returns the xpath, storeItemID, and prefixMappings of the content
// control's w:dataBinding child, and whether a binding is present. The binding
// lives among the verbatim-preserved w:sdtPr children (pr.Raw).
func (pr *CT_SdtPr) DataBinding() (xpath, storeItemID, prefixMappings string, ok bool) {
	if pr == nil {
		return "", "", "", false
	}
	for _, rn := range pr.Raw {
		if rn.Local == "dataBinding" {
			return rn.Attr("xpath"), rn.Attr("storeItemID"), rn.Attr("prefixMappings"), true
		}
	}
	return "", "", "", false
}

// SetDataBinding sets (creating if necessary) the w:dataBinding child that binds
// the content control to a node of a custom-XML data part. An existing binding
// has its attributes replaced in place, preserving its position; a new binding
// is inserted immediately before the control-type child so the emitted w:sdtPr
// child order stays schema-valid.
func (pr *CT_SdtPr) SetDataBinding(xpath, storeItemID, prefixMappings string) {
	attrs := dataBindingAttrs(xpath, storeItemID, prefixMappings)
	for _, rn := range pr.Raw {
		if rn.Local == "dataBinding" {
			rn.Attrs = attrs
			rn.RawContent = nil
			return
		}
	}
	// backfillChildOrder must run while the new child is not yet in pr.Raw so it
	// records only the existing children.
	pr.backfillChildOrder()
	rn := &CT_RawNamedElement{Local: "dataBinding", Space: NsWml}
	rn.Attrs = attrs
	idx := len(pr.Raw)
	pr.Raw = append(pr.Raw, rn)
	pr.insertRawBeforeControl(idx)
}

// RemoveDataBinding drops the w:dataBinding child if present, reporting whether
// one was removed.
func (pr *CT_SdtPr) RemoveDataBinding() bool {
	if pr == nil {
		return false
	}
	for i, rn := range pr.Raw {
		if rn.Local != "dataBinding" {
			continue
		}
		pr.Raw = append(pr.Raw[:i], pr.Raw[i+1:]...)
		pr.dropRawChild(i)
		return true
	}
	return false
}

// dataBindingAttrs builds the attribute list for a w:dataBinding element in the
// schema-defined order (w:prefixMappings, w:xpath, w:storeItemID). The optional
// prefixMappings attribute is omitted when empty.
func dataBindingAttrs(xpath, storeItemID, prefixMappings string) []xml.Attr {
	var attrs []xml.Attr
	if prefixMappings != "" {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Space: NsWml, Local: "prefixMappings"}, Value: prefixMappings})
	}
	attrs = append(attrs,
		xml.Attr{Name: xml.Name{Space: NsWml, Local: "xpath"}, Value: xpath},
		xml.Attr{Name: xml.Name{Space: NsWml, Local: "storeItemID"}, Value: storeItemID},
	)
	return attrs
}

// insertRawBeforeControl records a raw child in childOrder just before the
// control-type child, or at the end when there is no control child.
func (pr *CT_SdtPr) insertRawBeforeControl(rawIndex int) {
	ref := sdtPrChildRef{sdtPrChildRaw, rawIndex}
	for i, r := range pr.childOrder {
		if r.kind == sdtPrChildControl {
			tail := append([]sdtPrChildRef{ref}, pr.childOrder[i:]...)
			pr.childOrder = append(pr.childOrder[:i:i], tail...)
			return
		}
	}
	pr.childOrder = append(pr.childOrder, ref)
}

// dropRawChild removes the childOrder entry pointing at the raw child at
// rawIndex and shifts the indices of later raw children down by one so they keep
// pointing at the right pr.Raw entry.
func (pr *CT_SdtPr) dropRawChild(rawIndex int) {
	out := pr.childOrder[:0]
	for _, ref := range pr.childOrder {
		if ref.kind == sdtPrChildRaw {
			switch {
			case ref.index == rawIndex:
				continue
			case ref.index > rawIndex:
				ref.index--
			}
		}
		out = append(out, ref)
	}
	pr.childOrder = out
}
