package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_SdtBlock represents a block-level structured document tag (w:sdt in body).
type CT_SdtBlock struct {
	SdtPr      *CT_SdtPr           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtPr,omitempty"`
	SdtEndPr   *CT_SdtPr           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtEndPr,omitempty"`
	SdtContent *CT_SdtContentBlock `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtContent,omitempty"`
}

// CT_SdtContentBlock represents block-level SDT content. Besides the body
// content model it types w:tc and w:tr: a block SDT wrapping table cells
// (inside w:tr) or rows (inside w:tbl) carries them directly in its
// sdtContent, and they were previously dropped on save.
type CT_SdtContentBlock struct {
	P             []*CT_P               `xml:"-"`
	Tbl           []*CT_Tbl             `xml:"-"`
	SdtBlock      []*CT_SdtBlock        `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
	Tc            []*CT_Tc              `xml:"-"`
	Tr            []*CT_Tr              `xml:"-"`
	childOrder    []bodyChildRef
}

// AppendP appends a paragraph to the SDT content, maintaining child order.
func (sc *CT_SdtContentBlock) AppendP(p *CT_P) {
	backfillBodyChildOrder(&sc.childOrder, sc.P, sc.Tbl, sc.SdtBlock, sc.BookmarkStart, sc.BookmarkEnd, sc.Raw)
	appendBodyP(&sc.P, &sc.childOrder, p)
}

// UnmarshalXML implements custom unmarshaling for CT_SdtContentBlock: table
// cells and rows wrapped by the SDT decode into their typed models, everything
// else follows the body content model.
func (sc *CT_SdtContentBlock) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tc":
				v := &CT_Tc{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				sc.childOrder = append(sc.childOrder, bodyChildRef{bodyChildTc, len(sc.Tc)})
				sc.Tc = append(sc.Tc, v)
			case "tr":
				v := &CT_Tr{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				sc.childOrder = append(sc.childOrder, bodyChildRef{bodyChildTr, len(sc.Tr)})
				sc.Tr = append(sc.Tr, v)
			default:
				if err := unmarshalBodyChild(d, &t, &sc.P, &sc.Tbl, &sc.SdtBlock, &sc.BookmarkStart, &sc.BookmarkEnd, &sc.Raw, &sc.childOrder); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SdtContentBlock.
func (sc *CT_SdtContentBlock) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	if len(sc.childOrder) > 0 {
		for _, ref := range sc.childOrder {
			switch ref.kind {
			case bodyChildTc:
				if ref.index < len(sc.Tc) {
					sc.Tc[ref.index].MarshalToBuilder(b, ns, "tc")
				}
			case bodyChildTr:
				if ref.index < len(sc.Tr) {
					sc.Tr[ref.index].MarshalToBuilder(b, ns, "tr")
				}
			default:
				marshalBodyContent(b, ns, sc.P, sc.Tbl, sc.SdtBlock, sc.BookmarkStart, sc.BookmarkEnd, sc.Raw, []bodyChildRef{ref})
			}
		}
	} else {
		marshalBodyContent(b, ns, sc.P, sc.Tbl, sc.SdtBlock, sc.BookmarkStart, sc.BookmarkEnd, sc.Raw, nil)
	}
	b.EndElement(ns, localName)
}

// contentParagraphs returns the paragraphs inside this block-level SDT in
// document order, descending into nested SDT blocks.
func (s *CT_SdtBlock) contentParagraphs() []*CT_P {
	if s.SdtContent == nil {
		return nil
	}
	sc := s.SdtContent
	if len(sc.childOrder) == 0 {
		result := append([]*CT_P{}, sc.P...)
		for _, tbl := range sc.Tbl {
			collectTableParagraphs(tbl, &result)
		}
		for _, nested := range sc.SdtBlock {
			result = append(result, nested.contentParagraphs()...)
		}
		return result
	}
	var result []*CT_P
	for _, ref := range sc.childOrder {
		switch ref.kind {
		case bodyChildP:
			if ref.index < len(sc.P) {
				result = append(result, sc.P[ref.index])
			}
		case bodyChildTbl:
			if ref.index < len(sc.Tbl) {
				collectTableParagraphs(sc.Tbl[ref.index], &result)
			}
		case bodyChildSdt:
			if ref.index < len(sc.SdtBlock) {
				result = append(result, sc.SdtBlock[ref.index].contentParagraphs()...)
			}
		case bodyChildTc:
			if ref.index < len(sc.Tc) {
				result = append(result, sc.Tc[ref.index].P...)
			}
		case bodyChildTr:
			if ref.index < len(sc.Tr) {
				for _, tc := range sc.Tr[ref.index].Tc {
					result = append(result, tc.P...)
				}
			}
		}
	}
	return result
}

// CT_SdtRun represents an inline/run-level structured document tag.
type CT_SdtRun struct {
	SdtPr      *CT_SdtPr         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtPr,omitempty"`
	SdtEndPr   *CT_SdtPr         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtEndPr,omitempty"`
	SdtContent *CT_SdtContentRun `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sdtContent,omitempty"`
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SdtRun.
func (sr *CT_SdtRun) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	if sr.SdtPr != nil {
		sr.SdtPr.MarshalToBuilder(b, ns, "sdtPr")
	}
	if sr.SdtEndPr != nil {
		sr.SdtEndPr.MarshalToBuilder(b, ns, "sdtEndPr")
	}
	if sr.SdtContent != nil {
		sr.SdtContent.MarshalToBuilder(b, ns, "sdtContent")
	}
	b.EndElement(ns, localName)
}

// CT_SdtContentRun represents run-level SDT content.
type CT_SdtContentRun struct {
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

// UnmarshalXML implements custom unmarshaling for CT_SdtContentRun.
func (sc *CT_SdtContentRun) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return unmarshalPContent(d, &sc.R, &sc.Hyperlink, &sc.BookmarkStart, &sc.BookmarkEnd,
		&sc.ProofErr, &sc.PermStart, &sc.PermEnd, &sc.Ins, &sc.Del, &sc.FldSimple, &sc.SdtRun, &sc.Raw, &sc.childOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SdtContentRun.
func (sc *CT_SdtContentRun) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	marshalPContent(b, ns, sc.R, sc.Hyperlink, sc.BookmarkStart, sc.BookmarkEnd,
		sc.ProofErr, sc.PermStart, sc.PermEnd, sc.Ins, sc.Del, sc.FldSimple, sc.SdtRun, sc.Raw, sc.childOrder)
	b.EndElement(ns, localName)
}
