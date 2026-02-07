package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_SdtPr represents structured document tag properties (w:sdtPr).
// Uses innerxml fallback for incremental typing of the many possible properties.
type CT_SdtPr struct {
	RawContent []byte `xml:",innerxml"`
}

// CT_SdtBlock represents a block-level structured document tag (w:sdt in body).
type CT_SdtBlock struct {
	SdtPr      *CT_SdtPr           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtPr,omitempty"`
	SdtEndPr   *CT_SdtPr           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtEndPr,omitempty"`
	SdtContent *CT_SdtContentBlock `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtContent,omitempty"`
}

// CT_SdtContentBlock represents block-level SDT content.
type CT_SdtContentBlock struct {
	P              []*CT_P              `xml:"-"`
	Tbl            []*CT_Tbl            `xml:"-"`
	SdtBlock       []*CT_SdtBlock       `xml:"-"`
	BookmarkStart  []*CT_BookmarkStart  `xml:"-"`
	BookmarkEnd    []*CT_BookmarkEnd    `xml:"-"`
	childOrder     []bodyChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_SdtContentBlock.
func (sc *CT_SdtContentBlock) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return unmarshalBodyContent(d, &sc.P, &sc.Tbl, &sc.SdtBlock, &sc.BookmarkStart, &sc.BookmarkEnd, &sc.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SdtContentBlock.
func (sc *CT_SdtContentBlock) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	marshalBodyContent(b, ns, sc.P, sc.Tbl, sc.SdtBlock, sc.BookmarkStart, sc.BookmarkEnd, sc.childOrder)
	b.EndElement(ns, localName)
}

// CT_SdtRun represents an inline/run-level structured document tag.
type CT_SdtRun struct {
	SdtPr      *CT_SdtPr        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtPr,omitempty"`
	SdtEndPr   *CT_SdtPr        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtEndPr,omitempty"`
	SdtContent *CT_SdtContentRun `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtContent,omitempty"`
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SdtRun.
func (sr *CT_SdtRun) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	if sr.SdtPr != nil {
		b.StartElement(ns, "sdtPr")
		b.WriteRaw(sr.SdtPr.RawContent)
		b.EndElement(ns, "sdtPr")
	}
	if sr.SdtEndPr != nil {
		b.StartElement(ns, "sdtEndPr")
		b.WriteRaw(sr.SdtEndPr.RawContent)
		b.EndElement(ns, "sdtEndPr")
	}
	if sr.SdtContent != nil {
		sr.SdtContent.MarshalToBuilder(b, ns, "sdtContent")
	}
	b.EndElement(ns, localName)
}

// CT_SdtContentRun represents run-level SDT content.
type CT_SdtContentRun struct {
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

// UnmarshalXML implements custom unmarshaling for CT_SdtContentRun.
func (sc *CT_SdtContentRun) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return unmarshalPContent(d, &sc.R, &sc.Hyperlink, &sc.BookmarkStart, &sc.BookmarkEnd,
		&sc.ProofErr, &sc.PermStart, &sc.PermEnd, &sc.Ins, &sc.Del, &sc.FldSimple, &sc.SdtRun, &sc.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SdtContentRun.
func (sc *CT_SdtContentRun) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	marshalPContent(b, ns, sc.R, sc.Hyperlink, sc.BookmarkStart, sc.BookmarkEnd,
		sc.ProofErr, sc.PermStart, sc.PermEnd, sc.Ins, sc.Del, sc.FldSimple, sc.SdtRun, sc.childOrder)
	b.EndElement(ns, localName)
}
