package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_RunTrackChange represents an insertion/deletion of run content (w:ins, w:del).
type CT_RunTrackChange struct {
	// CapturedAttrs preserves the verbatim source attribute list (order and
	// unmodeled attributes such as w16du:dateUtc); replayed on marshal.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	Id            string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`

	// Child content (same as paragraph content)
	R             []*CT_R               `xml:"-"`
	Hyperlink     []*CT_Hyperlink       `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	ProofErr      []*CT_ProofErr        `xml:"-"`
	PermStart     []*CT_PermStart       `xml:"-"`
	PermEnd       []*CT_PermEnd         `xml:"-"`
	Del           []*CT_RunTrackChange  `xml:"-"`
	Ins           []*CT_RunTrackChange  `xml:"-"`
	FldSimple     []*CT_SimpleField     `xml:"-"`
	SdtRun        []*CT_SdtRun          `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
	childOrder    []pChildRef
}

// AppendR appends a run to the tracked-change block, recording its position in
// the block's child order. Authoring uses this on a freshly created block, so
// no backfill of pre-existing children is needed.
func (tc *CT_RunTrackChange) AppendR(r *CT_R) {
	tc.childOrder = append(tc.childOrder, pChildRef{pChildR, len(tc.R)})
	tc.R = append(tc.R, r)
}

// UnmarshalXML implements custom unmarshaling for CT_RunTrackChange.
func (tc *CT_RunTrackChange) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	tc.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "id":
			tc.Id = attr.Value
		case "author":
			tc.Author = attr.Value
		case "date":
			tc.Date = attr.Value
		}
	}

	return unmarshalPContent(d, &tc.R, &tc.Hyperlink, &tc.BookmarkStart, &tc.BookmarkEnd,
		&tc.ProofErr, &tc.PermStart, &tc.PermEnd, &tc.Ins, &tc.Del, &tc.FldSimple, &tc.SdtRun, &tc.Raw, &tc.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_RunTrackChange.
func (tc *CT_RunTrackChange) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "id", Value: tc.Id})
	attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "author", Value: tc.Author})
	if tc.Date != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "date", Value: tc.Date})
	}
	if tc.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(tc.CapturedAttrs, attrs)
	}
	b.StartElement(ns, localName, attrs...)
	marshalPContent(b, ns, tc.R, tc.Hyperlink, tc.BookmarkStart, tc.BookmarkEnd,
		tc.ProofErr, tc.PermStart, tc.PermEnd, tc.Ins, tc.Del, tc.FldSimple, tc.SdtRun, tc.Raw, tc.childOrder)
	b.EndElement(ns, localName)
}

// CT_PPrChange represents a revision of paragraph properties.
type CT_PPrChange struct {
	Id     string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date   string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	PPr    *CT_PPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPr,omitempty"`
}

// CT_SectPrChange represents a revision of section properties.
type CT_SectPrChange struct {
	Id     string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date   string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	SectPr *CT_SectPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sectPr,omitempty"`
}

// CT_TblPrChange represents a revision of table properties.
type CT_TblPrChange struct {
	Id     string    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author string    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date   string    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	TblPr  *CT_TblPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblPr,omitempty"`
}

// CT_TrPrChange represents a revision of table row properties.
type CT_TrPrChange struct {
	Id     string   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author string   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date   string   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	TrPr   *CT_TrPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main trPr,omitempty"`
}

// CT_TcPrChange represents a revision of table cell properties.
type CT_TcPrChange struct {
	Id     string   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author string   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date   string   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	TcPr   *CT_TcPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tcPr,omitempty"`
}
