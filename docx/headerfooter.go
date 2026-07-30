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
	d.markEdited()
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}
	if d.doc().Body.SectPr == nil {
		d.doc().Body.SectPr = &oxml.CT_SectPr{}
	}
	return d.addHeaderTo(d.doc().Body.SectPr, hType)
}

// addHeaderTo adds a header of the specified type to the given section
// properties, registering the part and relationship. It is the section-targeted
// core of AddHeader; callers ensure sectPr is non-nil.
func (d *Document) addHeaderTo(sectPr *oxml.CT_SectPr, hType HeaderType) *Header {
	relID := fmt.Sprintf("rId%d", d.nextRelID())

	// ECMA-376 allows at most one header reference per type in a sectPr:
	// a repeated AddHeader of the same type replaces the existing reference
	// instead of appending a duplicate. The replaced header is released — its
	// part, its .rels and its document.xml relationship — whether it was added
	// earlier in this session or came from the opened package (C492); a part
	// another section still references is kept.
	var existingRef *oxml.CT_HdrFtrRef
	for _, ref := range sectPr.HeaderReference {
		if ref.Type == hType.xmlVal() {
			existingRef = ref
			break
		}
	}
	replacedRID, replacedSession := "", false
	if existingRef != nil {
		replacedRID = existingRef.RID
		// A part added earlier in this session is dropped up front: it was
		// never written, so releasing it here also frees its part name for the
		// replacement to reuse.
		replacedSession = d.dropSessionHeader(replacedRID)
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
	// A replaced part that came from the opened package is released only after
	// the reference has been repointed, so the "is any section still pointing
	// at it?" check sees the new state.
	if replacedRID != "" && !replacedSession {
		d.dropUnreferencedHdrFtr(replacedRID)
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
	d.markEdited()
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}
	if d.doc().Body.SectPr == nil {
		d.doc().Body.SectPr = &oxml.CT_SectPr{}
	}

	relID := fmt.Sprintf("rId%d", d.nextRelID())

	// At most one footer reference per type (see AddHeader).
	var existingRef *oxml.CT_HdrFtrRef
	for _, ref := range d.doc().Body.SectPr.FooterReference {
		if ref.Type == fType.xmlVal() {
			existingRef = ref
			break
		}
	}
	replacedRID, replacedSession := "", false
	if existingRef != nil {
		replacedRID = existingRef.RID
		replacedSession = d.dropSessionFooter(replacedRID)
	}

	// Derive the part name from the parts already in the package (see AddHeader).
	partName := d.nextHdrFtrPartName("footer")

	ftr := &oxml.CT_HdrFtr{}

	// Add (or repoint) the reference in section properties
	if existingRef != nil {
		existingRef.RID = relID
	} else {
		d.doc().Body.SectPr.FooterReference = append(d.doc().Body.SectPr.FooterReference, &oxml.CT_HdrFtrRef{
			Type: fType.xmlVal(),
			RID:  relID,
		})
	}
	// See AddHeader: a preserved part is released after the reference has been
	// repointed.
	if replacedRID != "" && !replacedSession {
		d.dropUnreferencedHdrFtr(replacedRID)
	}

	// Enable first page header/footer if needed
	if fType == FooterFirst {
		d.doc().Body.SectPr.TitlePg = &oxml.CT_OnOff{}
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
		d.markSettingsModified()
	}
}

// --- read ---

// PartName returns the package part name backing the header (e.g.
// /word/header1.xml).
func (h *Header) PartName() string { return h.partName }

// Paragraphs returns the header's paragraphs in document order, descending into
// tables and block-level structured document tags. Editing one writes back to
// the header part on save.
func (h *Header) Paragraphs() []*Paragraph {
	if h == nil || h.hdr == nil {
		return nil
	}
	paras := h.hdr.AllParagraphs()
	out := make([]*Paragraph, 0, len(paras))
	for _, p := range paras {
		out = append(out, &Paragraph{document: h.document, p: p, hfPart: h.partName})
	}
	return out
}

// PartName returns the package part name backing the footer.
func (f *Footer) PartName() string { return f.partName }

// Paragraphs returns the footer's paragraphs in document order, descending into
// tables and block-level structured document tags.
func (f *Footer) Paragraphs() []*Paragraph {
	if f == nil || f.ftr == nil {
		return nil
	}
	paras := f.ftr.AllParagraphs()
	out := make([]*Paragraph, 0, len(paras))
	for _, p := range paras {
		out = append(out, &Paragraph{document: f.document, p: p, hfPart: f.partName})
	}
	return out
}

// Headers returns an editable handle for every header part in the document,
// ordered by part name. Until this existed a header's content was reachable
// only obliquely — through Document.Hyperlinks, Document.Images or
// ReplaceText — and there was no way to add a paragraph to a header that came
// from the opened file, which left several header-side mutators unreachable.
// Use Section.Header to find the header of a specific type instead.
func (d *Document) Headers() []*Header {
	out := make([]*Header, 0, len(d.headers))
	for _, name := range d.sortedHeaderNames() {
		hp := d.headers[name]
		if hp == nil || hp.hdr == nil {
			continue
		}
		out = append(out, &Header{document: d, hdr: hp.hdr, partName: name, relID: d.relIDForPart(name)})
	}
	return out
}

// Footers returns an editable handle for every footer part in the document,
// ordered by part name.
func (d *Document) Footers() []*Footer {
	out := make([]*Footer, 0, len(d.footers))
	for _, name := range d.sortedFooterNames() {
		fp := d.footers[name]
		if fp == nil || fp.ftr == nil {
			continue
		}
		out = append(out, &Footer{document: d, ftr: fp.ftr, partName: name, relID: d.relIDForPart(name)})
	}
	return out
}

// Header returns the section's header of the given type and whether the section
// declares one. It resolves the section's own headerReference, so a section that
// inherits its header from an earlier one reports false.
func (d *Document) Header(s *Section, hType HeaderType) (*Header, bool) {
	if s == nil || s.sectPr == nil {
		return nil, false
	}
	for _, ref := range s.sectPr.HeaderReference {
		if ref == nil || ref.Type != hType.xmlVal() {
			continue
		}
		name := d.partForRelID(ref.RID)
		hp := d.headers[name]
		if hp == nil || hp.hdr == nil {
			return nil, false
		}
		return &Header{document: d, hdr: hp.hdr, partName: name, relID: ref.RID}, true
	}
	return nil, false
}

// Footer returns the section's footer of the given type and whether the section
// declares one.
func (d *Document) Footer(s *Section, fType FooterType) (*Footer, bool) {
	if s == nil || s.sectPr == nil {
		return nil, false
	}
	for _, ref := range s.sectPr.FooterReference {
		if ref == nil || ref.Type != fType.xmlVal() {
			continue
		}
		name := d.partForRelID(ref.RID)
		fp := d.footers[name]
		if fp == nil || fp.ftr == nil {
			return nil, false
		}
		return &Footer{document: d, ftr: fp.ftr, partName: name, relID: ref.RID}, true
	}
	return nil, false
}

// partForRelID resolves a main-part relationship id to the part name it targets,
// or "" when the id is unknown or external.
func (d *Document) partForRelID(relID string) string {
	main := d.mainPart()
	for _, rel := range d.relationships[main] {
		if rel != nil && rel.ID == relID && rel.TargetMode != opc.TargetModeExternal {
			return opc.ResolvePartName(main, rel.Target)
		}
	}
	return ""
}

// relIDForPart is the inverse of partForRelID: the main-part relationship id
// that targets partName, or "".
func (d *Document) relIDForPart(partName string) string {
	main := d.mainPart()
	for _, rel := range d.relationships[main] {
		if rel != nil && rel.TargetMode != opc.TargetModeExternal &&
			opc.ResolvePartName(main, rel.Target) == partName {
			return rel.ID
		}
	}
	return ""
}

// --- write ---

// AddParagraph adds a paragraph to the header. On a header that came from the
// opened package this flags the part for regeneration, so the new paragraph is
// written instead of being masked by the preserved original bytes.
func (h *Header) AddParagraph() *Paragraph {
	p := &oxml.CT_P{}
	h.hdr.AppendP(p)
	if h.document != nil && h.partName != "" {
		h.document.markHdrFtrModified(h.partName)
	}
	return &Paragraph{document: h.document, p: p, hfPart: h.partName}
}

// AddParagraphWithText adds a paragraph with text to the header.
func (h *Header) AddParagraphWithText(text string) *Paragraph {
	p := h.AddParagraph()
	p.AddRun().SetText(text)
	return p
}

// AddParagraph adds a paragraph to the footer (see Header.AddParagraph).
func (f *Footer) AddParagraph() *Paragraph {
	p := &oxml.CT_P{}
	f.ftr.AppendP(p)
	if f.document != nil && f.partName != "" {
		f.document.markHdrFtrModified(f.partName)
	}
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

	if hf.OriginalRootAttrs != nil {
		// Verbatim root replay: keeps mc:Ignorable, xml:space and every
		// extension declaration the part's captured/raw children reference
		// (C370). A part built from scratch gets the standard set.
		b.StartElementWithRootAttrs(xmlb.NSWordprocessingML, rootElement, hf.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(xmlb.NSWordprocessingML, rootElement, xmlb.WordprocessingMLNamespaces())
	}
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
