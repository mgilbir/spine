package docx

import (
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// HeaderType identifies the type of header.
type HeaderType int

const (
	HeaderDefault HeaderType = iota
	HeaderFirst
	HeaderEven
)

func (ht HeaderType) xmlVal() string {
	switch ht {
	case HeaderFirst:
		return "first"
	case HeaderEven:
		return "even"
	default:
		return "default"
	}
}

// FooterType identifies the type of footer.
type FooterType int

const (
	FooterDefault FooterType = iota
	FooterFirst
	FooterEven
)

func (ft FooterType) xmlVal() string {
	switch ft {
	case FooterFirst:
		return "first"
	case FooterEven:
		return "even"
	default:
		return "default"
	}
}

// Header represents a document header.
type Header struct {
	document *Document
	hdr      *oxml.CT_HdrFtr
	relID    string
}

// Footer represents a document footer.
type Footer struct {
	document *Document
	ftr      *oxml.CT_HdrFtr
	relID    string
}

// hdrFtrPart stores header/footer data to be written.
type hdrFtrPart struct {
	partName string
	relID    string
}

// AddHeader adds a header of the specified type to the document.
func (d *Document) AddHeader(hType HeaderType) *Header {
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}
	if d.document.Body.SectPr == nil {
		d.document.Body.SectPr = &oxml.CT_SectPr{}
	}

	d.hdrFtrCount++
	num := d.hdrFtrCount
	relID := fmt.Sprintf("rId%d", d.nextRelID())
	partName := fmt.Sprintf("/word/header%d.xml", num)

	hdr := &oxml.CT_HdrFtr{}

	// Add reference to section properties
	d.document.Body.SectPr.HeaderReference = append(d.document.Body.SectPr.HeaderReference, &oxml.CT_HdrFtrRef{
		Type: hType.xmlVal(),
		RID:  relID,
	})

	// Enable first page header/footer if needed
	if hType == HeaderFirst {
		d.document.Body.SectPr.TitlePg = &oxml.CT_OnOff{}
	}

	h := &Header{document: d, hdr: hdr, relID: relID}

	// Store for writing during save
	d.newHeaderParts = append(d.newHeaderParts, &hdrFtrPart{
		partName: partName,
		relID:    relID,
	})

	// Keep reference to marshal later
	d.headers[partName] = &headerPart{hdr: hdr}

	return h
}

// AddFooter adds a footer of the specified type to the document.
func (d *Document) AddFooter(fType FooterType) *Footer {
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}
	if d.document.Body.SectPr == nil {
		d.document.Body.SectPr = &oxml.CT_SectPr{}
	}

	d.hdrFtrCount++
	num := d.hdrFtrCount
	relID := fmt.Sprintf("rId%d", d.nextRelID())
	partName := fmt.Sprintf("/word/footer%d.xml", num)

	ftr := &oxml.CT_HdrFtr{}

	// Add reference to section properties
	d.document.Body.SectPr.FooterReference = append(d.document.Body.SectPr.FooterReference, &oxml.CT_HdrFtrRef{
		Type: fType.xmlVal(),
		RID:  relID,
	})

	// Enable first page header/footer if needed
	if fType == FooterFirst {
		d.document.Body.SectPr.TitlePg = &oxml.CT_OnOff{}
	}

	f := &Footer{document: d, ftr: ftr, relID: relID}

	// Store for writing during save
	d.newFooterParts = append(d.newFooterParts, &hdrFtrPart{
		partName: partName,
		relID:    relID,
	})

	d.footers[partName] = &footerPart{ftr: ftr}

	return f
}

// AddParagraph adds a paragraph to the header.
func (h *Header) AddParagraph() *Paragraph {
	p := &oxml.CT_P{}
	h.hdr.AppendP(p)
	return &Paragraph{document: h.document, p: p}
}

// AddParagraphWithText adds a paragraph with text to the header.
func (h *Header) AddParagraphWithText(text string) *Paragraph {
	p := h.AddParagraph()
	p.AddRun().SetText(text)
	return p
}

// AddParagraph adds a paragraph to the footer.
func (f *Footer) AddParagraph() *Paragraph {
	p := &oxml.CT_P{}
	f.ftr.AppendP(p)
	return &Paragraph{document: f.document, p: p}
}

// AddParagraphWithText adds a paragraph with text to the footer.
func (f *Footer) AddParagraphWithText(text string) *Paragraph {
	p := f.AddParagraph()
	p.AddRun().SetText(text)
	return p
}

// marshalHdrFtrXML marshals a header/footer to XML.
func marshalHdrFtrXML(hf *oxml.CT_HdrFtr, rootElement string) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()

	nsDecls := xmlb.WordprocessingMLNamespaces()
	b.StartElementWithNS(xmlb.NSWordprocessingML, rootElement, nsDecls)
	marshalHdrFtrContent(b, xmlb.NSWordprocessingML, hf)
	b.EndElement(xmlb.NSWordprocessingML, rootElement)

	return b.Bytes()
}

// marshalHdrFtrContent marshals the body content of a header/footer.
func marshalHdrFtrContent(b *xmlb.Builder, ns string, hf *oxml.CT_HdrFtr) {
	for _, p := range hf.P {
		p.MarshalToBuilder(b, ns, "p")
	}
	for _, tbl := range hf.Tbl {
		tbl.MarshalToBuilder(b, ns, "tbl")
	}
}
