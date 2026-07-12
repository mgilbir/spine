package docx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// Document represents a Word document.
type Document struct {
	// Properties contains the document properties.
	Properties opc.CoreProperties

	reader   *opc.ReadCloser
	document *oxml.CT_Document
	// mainPartName is the resolved name of the main document part from the
	// package root relationships (usually /word/document.xml, but e.g.
	// /word/document2.xml occurs in the wild). The save paths regenerate THIS
	// part; hardcoding /word/document.xml would preserve the stale original
	// and silently drop every edit.
	mainPartName     string
	styles           *oxml.CT_Styles
	numbering        *oxml.CT_Numbering
	settings         *oxml.CT_Settings
	footnotes        *oxml.CT_Footnotes
	endnotes         *oxml.CT_Endnotes
	comments         *oxml.CT_Comments
	headers          map[string]*headerPart
	footers          map[string]*footerPart
	otherParts       map[string]*coxml.RawPart
	relationships    map[string][]*opc.Relationship
	hasCoreProps     bool
	propsSnapshot    *opc.CoreProperties       // Properties as loaded at open; detects edits at save
	preservedParts   map[string]*coxml.RawPart // all original parts for round-trip
	contentTypesData []byte                    // raw [Content_Types].xml
	imageParts       []*imagePart              // images to be written
	nextRelIDVal     int                       // counter for relationship IDs
	newHeaderParts   []*hdrFtrPart             // new headers to be written
	newFooterParts   []*hdrFtrPart             // new footers to be written
	// numberingModified / settingsModified record that the session changed the
	// numbering or settings model, so the round-trip save regenerates that part
	// (with its relationship and content-type override) instead of writing the
	// preserved original bytes.
	numberingModified bool
	settingsModified  bool
}

// mainDocumentPart is the default name of the main document part. Image
// relationships default to the main-part scope unless the image is placed in
// a header/footer part.
const mainDocumentPart = "/word/document.xml"

// mainPart returns the name of the main document part: the one resolved from
// the root relationships at open, or the default for created documents.
func (d *Document) mainPart() string {
	if d.mainPartName != "" {
		return d.mainPartName
	}
	return mainDocumentPart
}

// headerPart stores a parsed header.
type headerPart struct {
	hdr         *oxml.CT_HdrFtr
	contentType string
}

// footerPart stores a parsed footer.
type footerPart struct {
	ftr         *oxml.CT_HdrFtr
	contentType string
}

// Open opens a Word document from a file path.
func Open(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return OpenReader(bytes.NewReader(data), int64(len(data)))
}

// OpenReader opens a Word document from an in-memory reader.
func OpenReader(r io.ReaderAt, size int64) (*Document, error) {
	reader, err := opc.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	return openFromReader(&opc.ReadCloser{Reader: *reader})
}

// openFromReader creates a Document from an OPC reader.
func openFromReader(reader *opc.ReadCloser) (*Document, error) {
	rels := reader.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 {
		_ = reader.Close()
		return nil, ErrNotDOCX
	}

	mainPartName := opc.ResolvePartName("/", rels[0].Target)
	mainPart := reader.GetFile(mainPartName)
	if mainPart == nil {
		_ = reader.Close()
		return nil, ErrNotDOCX
	}

	if mainPart.ContentType != opc.ContentTypeDocument {
		_ = reader.Close()
		return nil, ErrNotDOCX
	}

	data, err := mainPart.ReadAll()
	if err != nil {
		_ = reader.Close()
		return nil, err
	}

	var doc oxml.CT_Document
	if err := xml.Unmarshal(data, &doc); err != nil {
		_ = reader.Close()
		return nil, err
	}

	d := &Document{
		reader:         reader,
		document:       &doc,
		mainPartName:   mainPartName,
		headers:        make(map[string]*headerPart),
		footers:        make(map[string]*footerPart),
		otherParts:     make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
		preservedParts: make(map[string]*coxml.RawPart),
	}

	if reader.Properties != nil {
		d.Properties = *reader.Properties
		d.hasCoreProps = true
		d.propsSnapshot = reader.Properties.Clone()
	}

	if err := d.loadAllParts(mainPartName); err != nil {
		_ = reader.Close()
		return nil, err
	}

	return d, nil
}

// loadAllParts loads all parts from the package.
func (d *Document) loadAllParts(mainPartName string) error {
	if d.reader == nil {
		return nil
	}

	d.loadAllRelationships()

	// Preserve [Content_Types].xml
	if ctData, err := d.reader.GetRawZipFile("[Content_Types].xml"); err == nil {
		d.contentTypesData = ctData
	}

	for _, file := range d.reader.Files {
		name := file.Name
		data, err := file.ReadAll()
		if err != nil {
			continue
		}

		// Preserve all parts as raw bytes for round-trip
		d.preservedParts[name] = &coxml.RawPart{
			ContentType: file.ContentType,
			Data:        data,
		}

		switch {
		case strings.HasSuffix(name, ".rels"):
			continue
		case name == mainPartName:
			continue
		case name == "/docProps/core.xml":
			continue
		case name == "/docProps/app.xml":
			// preserved in preservedParts
		case name == "/word/styles.xml":
			d.styles = &oxml.CT_Styles{}
			if err := xml.Unmarshal(data, d.styles); err != nil {
				return err
			}
		case name == "/word/numbering.xml":
			d.numbering = &oxml.CT_Numbering{}
			if err := xml.Unmarshal(data, d.numbering); err != nil {
				return err
			}
		case name == "/word/settings.xml":
			d.settings = &oxml.CT_Settings{}
			if err := xml.Unmarshal(data, d.settings); err != nil {
				return err
			}
		case name == "/word/footnotes.xml":
			d.footnotes = &oxml.CT_Footnotes{}
			if err := xml.Unmarshal(data, d.footnotes); err != nil {
				return err
			}
		case name == "/word/endnotes.xml":
			d.endnotes = &oxml.CT_Endnotes{}
			if err := xml.Unmarshal(data, d.endnotes); err != nil {
				return err
			}
		case name == "/word/comments.xml":
			d.comments = &oxml.CT_Comments{}
			if err := xml.Unmarshal(data, d.comments); err != nil {
				return err
			}
		case name == "/word/fontTable.xml":
			// preserved in preservedParts
		case name == "/word/webSettings.xml":
			// preserved in preservedParts
		case strings.HasPrefix(name, "/word/header") && strings.HasSuffix(name, ".xml"):
			hdr := &oxml.CT_HdrFtr{}
			if err := xml.Unmarshal(data, hdr); err != nil {
				return err
			}
			d.headers[name] = &headerPart{hdr: hdr, contentType: file.ContentType}
		case strings.HasPrefix(name, "/word/footer") && strings.HasSuffix(name, ".xml"):
			ftr := &oxml.CT_HdrFtr{}
			if err := xml.Unmarshal(data, ftr); err != nil {
				return err
			}
			d.footers[name] = &footerPart{ftr: ftr, contentType: file.ContentType}
		default:
			d.otherParts[name] = &coxml.RawPart{
				ContentType: file.ContentType,
				Data:        data,
			}
		}
	}

	return nil
}

// loadAllRelationships loads all relationship files into the model.
func (d *Document) loadAllRelationships() {
	if d.reader == nil {
		return
	}

	for _, file := range d.reader.Files {
		if !strings.HasSuffix(file.Name, ".rels") {
			continue
		}

		data, err := file.ReadAll()
		if err != nil {
			continue
		}

		rels, err := opc.UnmarshalRelationships(data)
		if err != nil {
			continue
		}

		sourcePart := coxml.RelsPathToSourcePart(file.Name)
		d.relationships[sourcePart] = rels
	}
}

// Create creates a new, empty document.
func Create() *Document {
	doc := &oxml.CT_Document{
		Body: &oxml.CT_Body{},
	}

	return &Document{
		document:       doc,
		mainPartName:   mainDocumentPart,
		headers:        make(map[string]*headerPart),
		footers:        make(map[string]*footerPart),
		otherParts:     make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
		preservedParts: make(map[string]*coxml.RawPart),
	}
}

// Save saves the document to a file.
func (d *Document) Save(path string) error {
	data, err := d.SaveBytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SaveBytes saves the document to an in-memory buffer.
func (d *Document) SaveBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.SaveTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SaveTo saves the document to an arbitrary writer.
func (d *Document) SaveTo(dst io.Writer) error {
	writer := opc.NewWriter(dst)
	var err error
	if d.reader != nil {
		err = d.saveRoundTrip(writer)
	} else {
		err = d.saveNew(writer)
	}
	if err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

// Close closes the document and releases resources.
func (d *Document) Close() error {
	if d.reader != nil {
		return d.reader.Close()
	}
	return nil
}

// saveRoundTrip saves a document opened from a file, preserving all parts.
// Only document.xml is regenerated; all other parts are preserved as original bytes.
func (d *Document) saveRoundTrip(writer *opc.Writer) error {
	if d.hasCoreProps {
		writer.Properties = &d.Properties
	}

	// Preserve original content types. Hand the writer a clone: Close mutates
	// its ContentTypes (SetOverride for regenerated metadata parts), so
	// sharing the reader's instance would let repeated saves observe each
	// other's side effects and make concurrent saves race.
	if d.reader != nil && d.reader.ContentTypes != nil {
		writer.ContentTypes = d.reader.ContentTypes.Clone()
	}

	// Write [Content_Types].xml as raw file if preserved. When parts were added
	// after open (images/headers/footers), skip the raw copy so the writer
	// regenerates [Content_Types].xml from the (preserved plus newly registered)
	// content types — otherwise the new parts' content types would be missing.
	if len(d.contentTypesData) > 0 && !d.hasAddedParts() {
		if err := writer.WriteRawFile("[Content_Types].xml", d.contentTypesData); err != nil {
			return err
		}
	}

	// Write core.xml as preserved raw bytes only when the in-memory
	// properties still match the snapshot taken at open. Writing the raw part
	// first would win under the opc writer's skip-if-written rule and
	// silently drop any edits; when the properties changed, skip the raw copy
	// so Close regenerates core.xml from d.Properties.
	if d.hasCoreProps && d.Properties.Equal(d.propsSnapshot) {
		if part, ok := d.preservedParts["/docProps/core.xml"]; ok {
			if err := writer.WritePreservedPart("/docProps/core.xml", part.ContentType, part.Data); err != nil {
				return err
			}
		}
	}

	// Write all preserved parts except the main document part (which is
	// regenerated), core.xml (handled above), and .rels files (handled
	// separately). Names are sorted so the zip entry order is deterministic
	// across saves and processes instead of following map iteration order.
	mainPartName := d.mainPart()
	mainRelsName := opc.GetRelationshipsPartName(mainPartName)
	preservedNames := make([]string, 0, len(d.preservedParts))
	for name := range d.preservedParts {
		preservedNames = append(preservedNames, name)
	}
	sort.Strings(preservedNames)
	for _, name := range preservedNames {
		if name == mainPartName {
			continue
		}
		if name == "/docProps/core.xml" {
			continue
		}
		// Regenerated below from the (possibly extended) parsed model.
		if name == "/word/numbering.xml" && d.numberingModified {
			continue
		}
		if name == "/word/settings.xml" && d.settingsModified {
			continue
		}
		if strings.HasSuffix(name, ".rels") {
			continue
		}
		if err := writer.WritePreservedPart(name, d.preservedParts[name].ContentType, d.preservedParts[name].Data); err != nil {
			return err
		}
	}

	// Write all .rels files from preserved parts (except the main part's rels,
	// which are regenerated).
	for _, name := range preservedNames {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		if name == mainRelsName {
			continue
		}
		if err := writer.WritePreservedPart(name, d.preservedParts[name].ContentType, d.preservedParts[name].Data); err != nil {
			return err
		}
	}

	// Write parts added through the mutation API (images, headers, footers).
	if err := d.writeAddedParts(writer); err != nil {
		return err
	}

	// Write the numbering/settings parts when the session modified them, and
	// make sure document.xml carries a relationship to each. A document opened
	// without the part gets a fresh part, relationship, and content-type
	// override; one that already had it gets the parsed-and-extended content
	// in place of the preserved bytes.
	if err := d.writeModifiedMetadataParts(writer); err != nil {
		return err
	}

	// Write the main document part (regenerated) under its resolved name:
	// writing to a hardcoded /word/document.xml while the package's root
	// relationship points elsewhere would orphan the edits.
	docData, err := marshalDocumentXML(d.document)
	if err != nil {
		return err
	}
	if err := writer.WritePart(mainPartName, opc.ContentTypeDocument, docData); err != nil {
		return err
	}

	// Write document relationships
	if err := d.writeDocumentRelationships(writer); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, strings.TrimPrefix(mainPartName, "/"), opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// writeAddedParts writes the media, header, and footer parts added through the
// mutation API. Both save paths call it, so parts added to a document opened
// from a file are written into the package — previously only the new-document
// path wrote them, leaving the relationships and references dangling.
func (d *Document) writeAddedParts(writer *opc.Writer) error {
	for _, img := range d.imageParts {
		if err := writer.WritePart(img.partName, img.contentType, img.data); err != nil {
			return err
		}
	}
	for _, hp := range d.newHeaderParts {
		hdrPart, ok := d.headers[hp.partName]
		if !ok {
			continue
		}
		data, err := marshalHdrFtrXML(hdrPart.hdr, "hdr")
		if err != nil {
			return err
		}
		if err := writer.WritePart(hp.partName, opc.ContentTypeDocHeader, data); err != nil {
			return err
		}
		if err := d.writeHdrFtrRelationships(writer, hp.partName); err != nil {
			return err
		}
	}
	for _, fp := range d.newFooterParts {
		ftrPart, ok := d.footers[fp.partName]
		if !ok {
			continue
		}
		data, err := marshalHdrFtrXML(ftrPart.ftr, "ftr")
		if err != nil {
			return err
		}
		if err := writer.WritePart(fp.partName, opc.ContentTypeDocFooter, data); err != nil {
			return err
		}
		if err := d.writeHdrFtrRelationships(writer, fp.partName); err != nil {
			return err
		}
	}
	return nil
}

// writeHdrFtrRelationships writes the .rels part for a header/footer added in
// this session, so relationships registered against that part (e.g. images in
// the header) resolve. Parts without registered relationships get no rels
// part, matching Word's output for plain headers.
func (d *Document) writeHdrFtrRelationships(writer *opc.Writer, partName string) error {
	rels := d.relationships[partName]
	if len(rels) == 0 {
		return nil
	}
	return writer.WritePartRelationships(partName, rels)
}

// writeModifiedMetadataParts regenerates the numbering and settings parts on
// the round-trip path when the session modified them, registering the
// document.xml relationship if the opened package did not already carry one.
func (d *Document) writeModifiedMetadataParts(writer *opc.Writer) error {
	if d.numberingModified && d.numbering != nil {
		data, err := marshalNumberingXML(d.numbering)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/numbering.xml", opc.ContentTypeNumbering, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeNumbering, "numbering.xml")
	}
	if d.settingsModified && d.settings != nil {
		data, err := marshalSettingsXML(d.settings)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/settings.xml", opc.ContentTypeDocSettings, data); err != nil {
			return err
		}
		d.ensureDocRelationship(opc.RelTypeSettings, "settings.xml")
	}
	return nil
}

// ensureDocRelationship adds a document.xml relationship of the given type
// unless one already exists.
func (d *Document) ensureDocRelationship(relType, target string) {
	for _, rel := range d.relationships[d.mainPart()] {
		if rel.Type == relType {
			return
		}
	}
	d.addDocRelationship(&opc.Relationship{
		ID:     fmt.Sprintf("rId%d", d.nextRelID()),
		Type:   relType,
		Target: target,
	})
}

// hasAddedParts reports whether the mutation API added parts or modified a
// metadata part, requiring [Content_Types].xml to be regenerated so the new
// parts' content types are declared.
func (d *Document) hasAddedParts() bool {
	return len(d.imageParts) > 0 || len(d.newHeaderParts) > 0 || len(d.newFooterParts) > 0 ||
		d.numberingModified || d.settingsModified
}

// saveNew saves a newly created document.
func (d *Document) saveNew(writer *opc.Writer) error {
	writer.Properties = &d.Properties

	// A created document always gets a styles part: AddHeading references
	// "Heading1".."Heading9", which would otherwise be undefined and render as
	// plain text.
	if d.styles == nil {
		d.styles = defaultStyles()
	}

	// Write document.xml
	docData, err := marshalDocumentXML(d.document)
	if err != nil {
		return err
	}
	if err := writer.WritePart("/word/document.xml", opc.ContentTypeDocument, docData); err != nil {
		return err
	}

	// Collect document relationships - all allocated via nextRelID()
	var docRels []*opc.Relationship

	// Write default styles
	if d.styles != nil {
		data, err := marshalStylesXML(d.styles)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/styles.xml", opc.ContentTypeDocStyles, data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", d.nextRelID()),
			Type:   opc.RelTypeStyles,
			Target: "styles.xml",
		})
	}

	// Write numbering definitions
	if d.numbering != nil && (len(d.numbering.AbstractNum) > 0 || len(d.numbering.Num) > 0) {
		data, err := marshalNumberingXML(d.numbering)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/numbering.xml", opc.ContentTypeNumbering, data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", d.nextRelID()),
			Type:   opc.RelTypeNumbering,
			Target: "numbering.xml",
		})
	}

	// Write settings (created when the API needs a document-level flag, e.g.
	// evenAndOddHeaders for even headers/footers).
	if d.settings != nil {
		data, err := marshalSettingsXML(d.settings)
		if err != nil {
			return err
		}
		if err := writer.WritePart("/word/settings.xml", opc.ContentTypeDocSettings, data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", d.nextRelID()),
			Type:   opc.RelTypeSettings,
			Target: "settings.xml",
		})
	}

	// Write the media/header/footer parts, then record their relationships.
	if err := d.writeAddedParts(writer); err != nil {
		return err
	}
	for _, img := range d.imageParts {
		// Images placed in a header/footer paragraph are related from that
		// part's own rels (written in writeAddedParts), not from document.xml.
		if img.owner != d.mainPart() {
			continue
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     img.relID,
			Type:   opc.RelTypeImage,
			Target: img.partName[len("/word/"):], // relative to /word/
		})
	}
	for _, hp := range d.newHeaderParts {
		docRels = append(docRels, &opc.Relationship{
			ID:     hp.relID,
			Type:   opc.RelTypeHeader,
			Target: hp.partName[len("/word/"):],
		})
	}
	for _, fp := range d.newFooterParts {
		docRels = append(docRels, &opc.Relationship{
			ID:     fp.relID,
			Type:   opc.RelTypeFooter,
			Target: fp.partName[len("/word/"):],
		})
	}

	if err := writer.WritePartRelationships("/word/document.xml", docRels); err != nil {
		return err
	}

	// Add main relationship
	if _, err := writer.AddRelationship(opc.RelTypeOfficeDocument, "word/document.xml", opc.TargetModeInternal); err != nil {
		return err
	}

	return nil
}

// writeDocumentRelationships writes the main document part's .rels file.
func (d *Document) writeDocumentRelationships(writer *opc.Writer) error {
	rels, ok := d.relationships[d.mainPart()]
	if !ok || len(rels) == 0 {
		return nil
	}
	return writer.WritePartRelationships(d.mainPart(), rels)
}

// Paragraphs returns all paragraphs in the document body in document order,
// including paragraphs wrapped in body-level structured document tags.
func (d *Document) Paragraphs() []*Paragraph {
	if d.document == nil || d.document.Body == nil {
		return nil
	}
	paras := d.document.Body.Paragraphs()
	result := make([]*Paragraph, len(paras))
	for i, p := range paras {
		result[i] = &Paragraph{document: d, p: p}
	}
	return result
}

// AddParagraph adds a new paragraph to the document body.
func (d *Document) AddParagraph() *Paragraph {
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}
	p := &oxml.CT_P{}
	d.document.Body.AppendP(p)
	return &Paragraph{document: d, p: p}
}

// AddParagraphWithText adds a new paragraph with the specified text.
func (d *Document) AddParagraphWithText(text string) *Paragraph {
	p := d.AddParagraph()
	p.AddRun().SetText(text)
	return p
}

// AddHeading adds a heading paragraph with the specified level. The level is
// clamped to the valid 1-9 range so out-of-range values cannot produce a
// nonsensical style name (e.g. level 10 previously yielded "Heading:").
func (d *Document) AddHeading(text string, level int) *Paragraph {
	if level < 1 {
		level = 1
	} else if level > 9 {
		level = 9
	}
	p := d.AddParagraph()
	p.SetStyle("Heading" + strconv.Itoa(level))
	p.AddRun().SetText(text)
	return p
}

// Body returns the document body text.
func (d *Document) Body() string {
	paras := d.Paragraphs()
	text := ""
	for _, p := range paras {
		if text != "" {
			text += "\n"
		}
		text += p.Text()
	}
	return text
}

// Tables returns all tables in the document body.
func (d *Document) Tables() []*Table {
	if d.document == nil || d.document.Body == nil {
		return nil
	}
	result := make([]*Table, len(d.document.Body.Tbl))
	for i, tbl := range d.document.Body.Tbl {
		result[i] = &Table{document: d, tbl: tbl}
	}
	return result
}

// AddTable creates a new table with the specified number of rows and columns.
func (d *Document) AddTable(rows, cols int) *Table {
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}
	tbl := &oxml.CT_Tbl{
		TblPr:   &oxml.CT_TblPr{},
		TblGrid: &oxml.CT_TblGrid{},
	}
	// Create grid columns
	for i := 0; i < cols; i++ {
		tbl.TblGrid.GridCol = append(tbl.TblGrid.GridCol, oxml.CT_GridCol{})
	}
	// Create rows with cells, seeding through the tracking helpers so the
	// required empty cell paragraph is recorded in the child order and is not
	// dropped by the first tracked append on the cell.
	for i := 0; i < rows; i++ {
		tr := &oxml.CT_Tr{}
		for j := 0; j < cols; j++ {
			tc := &oxml.CT_Tc{}
			tc.AppendP(&oxml.CT_P{})
			tr.AppendCell(tc)
		}
		tbl.AppendRow(tr)
	}
	d.document.Body.AppendTbl(tbl)
	return &Table{document: d, tbl: tbl}
}

// nextRelID returns the next available relationship ID number.
func (d *Document) nextRelID() int {
	if d.nextRelIDVal == 0 {
		// Seed past the highest existing numeric rId, not the count: rIds are
		// often non-contiguous after Word edits (e.g. rId1, rId3), so seeding
		// from the count would collide with an existing id.
		max := 0
		for _, rel := range d.relationships[d.mainPart()] {
			if n := relIDNumber(rel.ID); n > max {
				max = n
			}
		}
		d.nextRelIDVal = max + 1
	}
	id := d.nextRelIDVal
	d.nextRelIDVal++
	return id
}

// relIDNumber parses the numeric suffix of a relationship id like "rId7",
// returning 0 if it does not have that form.
func relIDNumber(id string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "rId"))
	if err != nil {
		return 0
	}
	return n
}

// nextImageNumber returns the smallest positive N for which no
// /word/media/imageN.* part exists. Word shares a single number space across
// image extensions (image1.png and image1.jpeg collide), so numbering is by
// basename number regardless of extension. The scan covers the parts preserved
// from an opened package as well as images added through the mutation API, so
// names stay collision-free across open→add→save cycles.
func (d *Document) nextImageNumber() int {
	used := make(map[int]bool)
	mark := func(name string) {
		const prefix = "/word/media/image"
		// OPC part names are case-insensitive: /word/media/IMAGE1.PNG occupies
		// the same name as image1.png, so match case-insensitively or the
		// generated name would collide at save time.
		name = strings.ToLower(name)
		if !strings.HasPrefix(name, prefix) {
			return
		}
		rest := name[len(prefix):]
		dot := strings.IndexByte(rest, '.')
		if dot <= 0 {
			return
		}
		if n, err := strconv.Atoi(rest[:dot]); err == nil && n > 0 {
			used[n] = true
		}
	}
	for name := range d.preservedParts {
		mark(name)
	}
	for name := range d.otherParts {
		mark(name)
	}
	for _, img := range d.imageParts {
		mark(img.partName)
	}
	for n := 1; ; n++ {
		if !used[n] {
			return n
		}
	}
}

// nextHdrFtrPartName returns the first free /word/<kind>N.xml part name, where
// kind is "header" or "footer". It scans the parsed header/footer maps, the
// parts added earlier in this session, and everything preserved from the
// opened package, so the returned name never collides with an existing part
// and is always a fresh key into d.headers/d.footers (an existing parsed
// header is never clobbered in memory).
func (d *Document) nextHdrFtrPartName(kind string) string {
	used := make(map[string]bool,
		len(d.preservedParts)+len(d.headers)+len(d.footers)+len(d.newHeaderParts)+len(d.newFooterParts))
	for name := range d.preservedParts {
		used[name] = true
	}
	for name := range d.headers {
		used[name] = true
	}
	for name := range d.footers {
		used[name] = true
	}
	for _, hp := range d.newHeaderParts {
		used[hp.partName] = true
	}
	for _, fp := range d.newFooterParts {
		used[fp.partName] = true
	}
	for n := 1; ; n++ {
		name := fmt.Sprintf("/word/%s%d.xml", kind, n)
		if !used[name] {
			return name
		}
	}
}

// addDocRelationship adds a relationship to the main document part's
// relationships.
func (d *Document) addDocRelationship(rel *opc.Relationship) {
	d.addPartRelationship(d.mainPart(), rel)
}

// addPartRelationship adds a relationship to the given source part's
// relationship set (e.g. a header part's own rels for images placed in it).
func (d *Document) addPartRelationship(partName string, rel *opc.Relationship) {
	d.relationships[partName] = append(d.relationships[partName], rel)
}

// removeDocRelationship removes the document.xml relationship with the given
// ID, if present.
func (d *Document) removeDocRelationship(relID string) {
	main := d.mainPart()
	rels := d.relationships[main]
	for i, rel := range rels {
		if rel.ID == relID {
			d.relationships[main] = append(rels[:i], rels[i+1:]...)
			return
		}
	}
}

// dropSessionHeader removes a header added earlier in this session, identified
// by its document relationship ID: the pending part, its part-scoped
// relationships, and its document.xml relationship. A relID that does not
// match a session-added header (e.g. one parsed from the original package) is
// left untouched — preserved parts are never dropped.
func (d *Document) dropSessionHeader(relID string) {
	for i, hp := range d.newHeaderParts {
		if hp.relID != relID {
			continue
		}
		delete(d.headers, hp.partName)
		delete(d.relationships, hp.partName)
		d.newHeaderParts = append(d.newHeaderParts[:i], d.newHeaderParts[i+1:]...)
		d.removeDocRelationship(relID)
		return
	}
}

// dropSessionFooter is the footer counterpart of dropSessionHeader.
func (d *Document) dropSessionFooter(relID string) {
	for i, fp := range d.newFooterParts {
		if fp.relID != relID {
			continue
		}
		delete(d.footers, fp.partName)
		delete(d.relationships, fp.partName)
		d.newFooterParts = append(d.newFooterParts[:i], d.newFooterParts[i+1:]...)
		d.removeDocRelationship(relID)
		return
	}
}

// DefaultSection returns the document's default (last) section.
// If no section properties exist, they are created with default values.
func (d *Document) DefaultSection() *Section {
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}
	if d.document.Body.SectPr == nil {
		d.document.Body.SectPr = &oxml.CT_SectPr{}
	}
	return &Section{sectPr: d.document.Body.SectPr}
}

// AddSectionBreak adds a section break by setting section properties on the
// last block-level paragraph and creating a new default section. When the
// body's last block is not a paragraph (e.g. the document ends with a table),
// a new paragraph is appended after it to carry the section properties —
// attaching them to an earlier paragraph would move the section boundary.
func (d *Document) AddSectionBreak() *Section {
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}

	// Move current body sectPr into the last paragraph
	oldSectPr := d.document.Body.SectPr
	if oldSectPr == nil {
		oldSectPr = &oxml.CT_SectPr{}
	}

	lastP := d.document.Body.LastBlockParagraph()
	if lastP == nil {
		lastP = &oxml.CT_P{}
		d.document.Body.AppendP(lastP)
	}
	if lastP.PPr == nil {
		lastP.PPr = &oxml.CT_PPr{}
	}
	lastP.PPr.SectPr = oldSectPr

	// Create new body-level section
	d.document.Body.SectPr = &oxml.CT_SectPr{}
	return &Section{sectPr: d.document.Body.SectPr}
}
