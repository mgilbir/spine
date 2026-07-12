package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_HdrFtr is the root element for headers and footers.
// Has the same content model as the document body.
type CT_HdrFtr struct {
	XMLName       xml.Name              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hdr"`
	P             []*CT_P               `xml:"-"`
	Tbl           []*CT_Tbl             `xml:"-"`
	SdtBlock      []*CT_SdtBlock        `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
	childOrder    []bodyChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_HdrFtr.
func (hf *CT_HdrFtr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	hf.XMLName = start.Name
	return unmarshalBodyContent(d, &hf.P, &hf.Tbl, &hf.SdtBlock, &hf.BookmarkStart, &hf.BookmarkEnd, &hf.Raw, &hf.childOrder)
}

// MarshalToBuilder marshals the header/footer with the given element name.
func (hf *CT_HdrFtr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	hf.MarshalContent(b, ns)
	b.EndElement(ns, localName)
}

// MarshalContent writes the header/footer children (without the enclosing
// element) in recorded child order, including SDT blocks and raw-preserved
// children — the same path CT_Body content takes.
func (hf *CT_HdrFtr) MarshalContent(b *xmlb.Builder, ns string) {
	marshalBodyContent(b, ns, hf.P, hf.Tbl, hf.SdtBlock, hf.BookmarkStart, hf.BookmarkEnd, hf.Raw, hf.childOrder)
}

// unmarshalBodyContent is a shared loop for body-level content.
func unmarshalBodyContent(d *xml.Decoder,
	p *[]*CT_P, tbl *[]*CT_Tbl, sdtBlock *[]*CT_SdtBlock,
	bookmarkStart *[]*CT_BookmarkStart, bookmarkEnd *[]*CT_BookmarkEnd,
	raw *[]*CT_RawNamedElement,
	childOrder *[]bodyChildRef,
) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := unmarshalBodyChild(d, &t, p, tbl, sdtBlock, bookmarkStart, bookmarkEnd, raw, childOrder); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// AppendP appends a paragraph to this header/footer, maintaining child order so
// it is marshaled even on one parsed from a file. Existing untracked children
// are backfilled into the order first.
func (hf *CT_HdrFtr) AppendP(p *CT_P) {
	backfillBodyChildOrder(&hf.childOrder, hf.P, hf.Tbl, hf.SdtBlock, hf.BookmarkStart, hf.BookmarkEnd, hf.Raw)
	appendBodyP(&hf.P, &hf.childOrder, p)
}
