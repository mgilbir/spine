package docx

import (
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
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
	partName string
}

// Footer represents a document footer.
type Footer struct {
	document *Document
	ftr      *oxml.CT_HdrFtr
	relID    string
	partName string
}

// hdrFtrPart stores header/footer data to be written.
type hdrFtrPart struct {
	partName string
	relID    string
}

// AddHeader adds a header of the specified type to the document's final
// (default) section.
func (d *Document) AddHeader(hType HeaderType) *Header {
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}
	if d.document.Body.SectPr == nil {
		d.document.Body.SectPr = &oxml.CT_SectPr{}
	}
	return d.addHeaderTo(d.document.Body.SectPr, hType)
}

// addHeaderTo adds a header of the specified type to the given section
// properties, registering the part and relationship. It is the section-targeted
// core of AddHeader; callers ensure sectPr is non-nil.
func (d *Document) addHeaderTo(sectPr *oxml.CT_SectPr, hType HeaderType) *Header {
	relID := fmt.Sprintf("rId%d", d.nextRelID())

	// ECMA-376 allows at most one header reference per type in a sectPr:
	// a repeated AddHeader of the same type replaces the existing reference
	// instead of appending a duplicate. If the replaced reference pointed at a
	// header added earlier in this session, that part and its relationship are
	// dropped so the package carries no orphan.
	var existingRef *oxml.CT_HdrFtrRef
	for _, ref := range sectPr.HeaderReference {
		if ref.Type == hType.xmlVal() {
			existingRef = ref
			break
		}
	}
	if existingRef != nil {
		d.dropSessionHeader(existingRef.RID)
	}

	// Derive the part name from the parts already in the package (preserved
	// parts, parsed headers/footers, and parts added in this session), so a
	// header added to an opened document that already contains
	// /word/header1.xml gets the next free name instead of a duplicate.
	partName := d.nextHdrFtrPartName("header")

	hdr := &oxml.CT_HdrFtr{}

	// Add (or repoint) the reference in section properties
	if existingRef != nil {
		existingRef.RID = relID
	} else {
		sectPr.HeaderReference = append(sectPr.HeaderReference, &oxml.CT_HdrFtrRef{
			Type: hType.xmlVal(),
			RID:  relID,
		})
	}

	// Enable first page header/footer if needed
	if hType == HeaderFirst {
		sectPr.TitlePg = &oxml.CT_OnOff{}
	}
	// An even-page header only renders when settings.xml carries
	// w:evenAndOddHeaders.
	if hType == HeaderEven {
		d.ensureEvenAndOddHeaders()
	}

	h := &Header{document: d, hdr: hdr, relID: relID, partName: partName}

	// Store for writing during save
	d.newHeaderParts = append(d.newHeaderParts, &hdrFtrPart{
		partName: partName,
		relID:    relID,
	})

	// Keep reference to marshal later. nextHdrFtrPartName guarantees the key
	// is fresh, so this can never clobber a parsed header already in the map.
	d.headers[partName] = &headerPart{hdr: hdr}

	// Register the relationship so it is written on the round-trip save path too
	// (the new-document path builds its own; without this a header added to an
	// opened document would have no relationship).
	d.addDocRelationship(&opc.Relationship{
		ID:     relID,
		Type:   opc.RelTypeHeader,
		Target: partName[len("/word/"):],
	})

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

	relID := fmt.Sprintf("rId%d", d.nextRelID())

	// At most one footer reference per type (see AddHeader).
	var existingRef *oxml.CT_HdrFtrRef
	for _, ref := range d.document.Body.SectPr.FooterReference {
		if ref.Type == fType.xmlVal() {
			existingRef = ref
			break
		}
	}
	if existingRef != nil {
		d.dropSessionFooter(existingRef.RID)
	}

	// Derive the part name from the parts already in the package (see AddHeader).
	partName := d.nextHdrFtrPartName("footer")

	ftr := &oxml.CT_HdrFtr{}

	// Add (or repoint) the reference in section properties
	if existingRef != nil {
		existingRef.RID = relID
	} else {
		d.document.Body.SectPr.FooterReference = append(d.document.Body.SectPr.FooterReference, &oxml.CT_HdrFtrRef{
			Type: fType.xmlVal(),
			RID:  relID,
		})
	}

	// Enable first page header/footer if needed
	if fType == FooterFirst {
		d.document.Body.SectPr.TitlePg = &oxml.CT_OnOff{}
	}
	// An even-page footer only renders when settings.xml carries
	// w:evenAndOddHeaders (the flag covers both headers and footers).
	if fType == FooterEven {
		d.ensureEvenAndOddHeaders()
	}

	f := &Footer{document: d, ftr: ftr, relID: relID, partName: partName}

	// Store for writing during save
	d.newFooterParts = append(d.newFooterParts, &hdrFtrPart{
		partName: partName,
		relID:    relID,
	})

	// nextHdrFtrPartName guarantees the key is fresh, so this can never
	// clobber a parsed footer already in the map.
	d.footers[partName] = &footerPart{ftr: ftr}

	// Register the relationship so it is written on the round-trip save path too
	// (see AddHeader).
	d.addDocRelationship(&opc.Relationship{
		ID:     relID,
		Type:   opc.RelTypeFooter,
		Target: partName[len("/word/"):],
	})

	return f
}

// ensureEvenAndOddHeaders makes sure settings.xml declares
// w:evenAndOddHeaders, creating the settings model when the document has
// none. The modified flag makes the save path regenerate (or newly write) the
// settings part; a document whose settings already carry the flag is left
// untouched so a zero-modification save stays byte-identical.
func (d *Document) ensureEvenAndOddHeaders() {
	if d.settings == nil {
		d.settings = &oxml.CT_Settings{}
	}
	if d.settings.EnsureEvenAndOddHeaders() {
		d.settingsModified = true
	}
}

// AddParagraph adds a paragraph to the header.
func (h *Header) AddParagraph() *Paragraph {
	p := &oxml.CT_P{}
	h.hdr.AppendP(p)
	return &Paragraph{document: h.document, p: p, hfPart: h.partName}
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
	return &Paragraph{document: f.document, p: p, hfPart: f.partName}
}

// AddParagraphWithText adds a paragraph with text to the footer.
func (f *Footer) AddParagraphWithText(text string) *Paragraph {
	p := f.AddParagraph()
	p.AddRun().SetText(text)
	return p
}

// marshalHdrFtrXML marshals a header/footer to XML.
func marshalHdrFtrXML(hf *oxml.CT_HdrFtr, rootElement string) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()

	nsDecls := xmlb.WordprocessingMLNamespaces()
	b.StartElementWithNS(xmlb.NSWordprocessingML, rootElement, nsDecls)
	// Route through the childOrder-driven body-content marshal so SDT blocks,
	// bookmarks, and raw-preserved children in a header/footer are emitted in
	// document order instead of being dropped.
	hf.MarshalContent(b, xmlb.NSWordprocessingML)
	b.EndElement(xmlb.NSWordprocessingML, rootElement)

	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal %s part: %w", rootElement, err)
	}
	return b.Bytes(), nil
}
