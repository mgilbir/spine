package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Hyperlink represents a hyperlink (w:hyperlink).
type CT_Hyperlink struct {
	// CapturedAttrs preserves the verbatim source attribute list; replayed
	// on marshal so producer attribute order and unmodeled attributes
	// survive.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	RID           string          `xml:"-"` // r:id - custom marshal
	Anchor        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main anchor,attr,omitempty"`
	History       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main history,attr,omitempty"`
	TgtFrame      string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tgtFrame,attr,omitempty"`
	Tooltip       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tooltip,attr,omitempty"`
	DocLocation   string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docLocation,attr,omitempty"`

	// Child content (same as paragraph content)
	R             []*CT_R               `xml:"-"`
	Hyperlink     []*CT_Hyperlink       `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	ProofErr      []*CT_ProofErr        `xml:"-"`
	PermStart     []*CT_PermStart       `xml:"-"`
	PermEnd       []*CT_PermEnd         `xml:"-"`
	Ins           []*CT_RunTrackChange  `xml:"-"`
	Del           []*CT_RunTrackChange  `xml:"-"`
	FldSimple     []*CT_SimpleField     `xml:"-"`
	SdtRun        []*CT_SdtRun          `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
	childOrder    []pChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_Hyperlink.
func (h *CT_Hyperlink) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	h.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			h.RID = attr.Value
		case attr.Name.Local == "r:id":
			h.RID = attr.Value
		case attr.Name.Local == "anchor":
			h.Anchor = attr.Value
		case attr.Name.Local == "history":
			h.History = attr.Value
		case attr.Name.Local == "tgtFrame":
			h.TgtFrame = attr.Value
		case attr.Name.Local == "tooltip":
			h.Tooltip = attr.Value
		case attr.Name.Local == "docLocation":
			h.DocLocation = attr.Value
		}
	}

	return unmarshalPContent(d, &h.R, &h.Hyperlink, &h.BookmarkStart, &h.BookmarkEnd,
		&h.ProofErr, &h.PermStart, &h.PermEnd, &h.Ins, &h.Del, &h.FldSimple, &h.SdtRun, &h.Raw, &h.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Hyperlink.
func (h *CT_Hyperlink) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if h.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: h.RID})
	}
	// Word's attribute order: r:id, w:anchor, w:tgtFrame, w:tooltip,
	// w:history, w:docLocation (corpus-verified; w:history precedes only
	// w:docLocation).
	if h.Anchor != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "anchor", Value: h.Anchor})
	}
	if h.TgtFrame != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "tgtFrame", Value: h.TgtFrame})
	}
	if h.Tooltip != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "tooltip", Value: h.Tooltip})
	}
	if h.History != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "history", Value: h.History})
	}
	if h.DocLocation != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "docLocation", Value: h.DocLocation})
	}
	if h.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(h.CapturedAttrs, attrs)
	}
	b.StartElement(ns, localName, attrs...)
	marshalPContent(b, ns, h.R, h.Hyperlink, h.BookmarkStart, h.BookmarkEnd,
		h.ProofErr, h.PermStart, h.PermEnd, h.Ins, h.Del, h.FldSimple, h.SdtRun, h.Raw, h.childOrder)
	b.EndElement(ns, localName)
}

// CT_SimpleField represents a simple field (w:fldSimple).
type CT_SimpleField struct {
	Instr string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main instr,attr"`
	Dirty string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dirty,attr,omitempty"`
	Lock  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lock,attr,omitempty"`

	R             []*CT_R               `xml:"-"`
	Hyperlink     []*CT_Hyperlink       `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	ProofErr      []*CT_ProofErr        `xml:"-"`
	FldSimple     []*CT_SimpleField     `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
	childOrder    []pChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_SimpleField.
func (f *CT_SimpleField) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "instr":
			f.Instr = attr.Value
		case "dirty":
			f.Dirty = attr.Value
		case "lock":
			f.Lock = attr.Value
		}
	}

	return unmarshalPContent(d, &f.R, &f.Hyperlink, &f.BookmarkStart, &f.BookmarkEnd,
		&f.ProofErr, nil, nil, nil, nil, &f.FldSimple, nil, &f.Raw, &f.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SimpleField.
func (f *CT_SimpleField) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "instr", Value: f.Instr})
	if f.Dirty != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "dirty", Value: f.Dirty})
	}
	if f.Lock != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "lock", Value: f.Lock})
	}
	b.StartElement(ns, localName, attrs...)
	marshalPContent(b, ns, f.R, f.Hyperlink, f.BookmarkStart, f.BookmarkEnd,
		f.ProofErr, nil, nil, nil, nil, f.FldSimple, nil, f.Raw, f.childOrder)
	b.EndElement(ns, localName)
}

// CT_FldChar represents a field character (begin/separate/end).
type CT_FldChar struct {
	FldCharType string `xml:"-"`
	FldLock     string `xml:"-"`
	Dirty       string `xml:"-"`

	// Raw preserves the optional children (w:fldData, w:ffData,
	// w:numberingChange) verbatim; they were previously dropped, losing
	// form-field definitions from legacy Word forms.
	Raw []*CT_RawNamedElement `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_FldChar, capturing its
// attributes and preserving child elements raw.
func (fc *CT_FldChar) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "fldCharType":
			fc.FldCharType = attr.Value
		case "fldLock":
			fc.FldLock = attr.Value
		case "dirty":
			fc.Dirty = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			rn := &CT_RawNamedElement{}
			if err := rn.UnmarshalXML(d, t); err != nil {
				return err
			}
			fc.Raw = append(fc.Raw, rn)
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_FldChar.
func (fc *CT_FldChar) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if fc.FldCharType != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "fldCharType", Value: fc.FldCharType})
	}
	if fc.FldLock != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "fldLock", Value: fc.FldLock})
	}
	if fc.Dirty != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "dirty", Value: fc.Dirty})
	}
	// A childless fldChar self-closes (matching both Word and the previous
	// reflection-based marshal, which programmatic builders like AddTable-
	// OfContents rely on even in parts without collapse-empty capture).
	if len(fc.Raw) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	for _, rn := range fc.Raw {
		rn.MarshalNamed(b, ns)
	}
	b.EndElement(ns, localName)
}
