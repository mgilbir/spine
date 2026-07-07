package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Hyperlink represents a hyperlink (w:hyperlink).
type CT_Hyperlink struct {
	RID     string `xml:"-"` // r:id - custom marshal
	Anchor  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main anchor,attr,omitempty"`
	History string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main history,attr,omitempty"`
	TgtFrame string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tgtFrame,attr,omitempty"`
	Tooltip string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tooltip,attr,omitempty"`
	DocLocation string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docLocation,attr,omitempty"`

	// Child content (same as paragraph content)
	R              []*CT_R              `xml:"-"`
	Hyperlink      []*CT_Hyperlink      `xml:"-"`
	BookmarkStart  []*CT_BookmarkStart  `xml:"-"`
	BookmarkEnd    []*CT_BookmarkEnd    `xml:"-"`
	ProofErr       []*CT_ProofErr       `xml:"-"`
	PermStart      []*CT_PermStart      `xml:"-"`
	PermEnd        []*CT_PermEnd        `xml:"-"`
	Ins            []*CT_RunTrackChange `xml:"-"`
	Del            []*CT_RunTrackChange `xml:"-"`
	FldSimple      []*CT_SimpleField    `xml:"-"`
	SdtRun         []*CT_SdtRun         `xml:"-"`
	childOrder     []pChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_Hyperlink.
func (h *CT_Hyperlink) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
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
		&h.ProofErr, &h.PermStart, &h.PermEnd, &h.Ins, &h.Del, &h.FldSimple, &h.SdtRun, &h.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Hyperlink.
func (h *CT_Hyperlink) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if h.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: h.RID})
	}
	if h.Anchor != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "anchor", Value: h.Anchor})
	}
	if h.History != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "history", Value: h.History})
	}
	if h.TgtFrame != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "tgtFrame", Value: h.TgtFrame})
	}
	if h.Tooltip != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "tooltip", Value: h.Tooltip})
	}
	if h.DocLocation != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "docLocation", Value: h.DocLocation})
	}
	b.StartElement(ns, localName, attrs...)
	marshalPContent(b, ns, h.R, h.Hyperlink, h.BookmarkStart, h.BookmarkEnd,
		h.ProofErr, h.PermStart, h.PermEnd, h.Ins, h.Del, h.FldSimple, h.SdtRun, h.childOrder)
	b.EndElement(ns, localName)
}

// CT_SimpleField represents a simple field (w:fldSimple).
type CT_SimpleField struct {
	Instr string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main instr,attr"`
	Dirty string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dirty,attr,omitempty"`
	Lock  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lock,attr,omitempty"`

	R          []*CT_R              `xml:"-"`
	Hyperlink  []*CT_Hyperlink      `xml:"-"`
	BookmarkStart []*CT_BookmarkStart `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd   `xml:"-"`
	ProofErr      []*CT_ProofErr      `xml:"-"`
	FldSimple     []*CT_SimpleField   `xml:"-"`
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
		&f.ProofErr, nil, nil, nil, nil, &f.FldSimple, nil, &f.childOrder)
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
		f.ProofErr, nil, nil, nil, nil, f.FldSimple, nil, f.childOrder)
	b.EndElement(ns, localName)
}

// CT_FldChar represents a field character (begin/separate/end).
type CT_FldChar struct {
	FldCharType string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fldCharType,attr"`
	FldLock     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fldLock,attr,omitempty"`
	Dirty       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dirty,attr,omitempty"`
}
