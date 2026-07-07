package docx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
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

	reader           *opc.ReadCloser
	document         *oxml.CT_Document
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
	preservedParts   map[string]*coxml.RawPart // all original parts for round-trip
	contentTypesData []byte                    // raw [Content_Types].xml
	imageParts       []*imagePart              // images to be written
	imageCount       int                       // counter for image numbering
	nextRelIDVal     int                       // counter for relationship IDs
	newHeaderParts   []*hdrFtrPart             // new headers to be written
	newFooterParts   []*hdrFtrPart             // new footers to be written
	hdrFtrCount      int                       // counter for header/footer numbering
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
		headers:        make(map[string]*headerPart),
		footers:        make(map[string]*footerPart),
		otherParts:     make(map[string]*coxml.RawPart),
		relationships:  make(map[string][]*opc.Relationship),
		preservedParts: make(map[string]*coxml.RawPart),
	}

	if reader.Properties != nil {
		d.Properties = *reader.Properties
		d.hasCoreProps = true
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

	// Preserve original content types
	if d.reader != nil && d.reader.ContentTypes != nil {
		writer.ContentTypes = d.reader.ContentTypes
	}

	// Write [Content_Types].xml as raw file if preserved
	if len(d.contentTypesData) > 0 {
		if err := writer.WriteRawFile("[Content_Types].xml", d.contentTypesData); err != nil {
			return err
		}
	}

	// Write core.xml as preserved raw bytes if original had it
	if d.hasCoreProps {
		if part, ok := d.preservedParts["/docProps/core.xml"]; ok {
			if err := writer.WritePart("/docProps/core.xml", part.ContentType, part.Data); err != nil {
				return err
			}
		}
	}

	// Write all preserved parts except document.xml (which is regenerated),
	// core.xml (handled above), and .rels files (handled separately)
	mainPartName := "/word/document.xml"
	for name, part := range d.preservedParts {
		if name == mainPartName {
			continue
		}
		if name == "/docProps/core.xml" {
			continue
		}
		if strings.HasSuffix(name, ".rels") {
			continue
		}
		if err := writer.WritePart(name, part.ContentType, part.Data); err != nil {
			return err
		}
	}

	// Write all .rels files from preserved parts (except document.xml.rels which is regenerated)
	for name, part := range d.preservedParts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		if name == "/word/_rels/document.xml.rels" {
			continue
		}
		if err := writer.WritePart(name, part.ContentType, part.Data); err != nil {
			return err
		}
	}

	// Write document.xml (regenerated)
	docData := marshalDocumentXML(d.document)
	if err := writer.WritePart("/word/document.xml", opc.ContentTypeDocument, docData); err != nil {
		return err
	}

	// Write document relationships
	if err := d.writeDocumentRelationships(writer); err != nil {
		return err
	}

	// Add main relationship
	writer.AddRelationship(opc.RelTypeOfficeDocument, "word/document.xml", opc.TargetModeInternal)

	return nil
}

// saveNew saves a newly created document.
func (d *Document) saveNew(writer *opc.Writer) error {
	writer.Properties = &d.Properties

	// Write document.xml
	docData := marshalDocumentXML(d.document)
	if err := writer.WritePart("/word/document.xml", opc.ContentTypeDocument, docData); err != nil {
		return err
	}

	// Collect document relationships - all allocated via nextRelID()
	var docRels []*opc.Relationship

	// Write default styles
	if d.styles != nil {
		data := marshalStylesXML(d.styles)
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
		data := marshalNumberingXML(d.numbering)
		if err := writer.WritePart("/word/numbering.xml", opc.ContentTypeNumbering, data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", d.nextRelID()),
			Type:   opc.RelTypeNumbering,
			Target: "numbering.xml",
		})
	}

	// Write image parts and add their relationships
	for _, img := range d.imageParts {
		if err := writer.WritePart(img.partName, img.contentType, img.data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     img.relID,
			Type:   opc.RelTypeImage,
			Target: img.partName[len("/word/"):], // relative to /word/
		})
	}

	// Write header/footer parts and relationships
	for _, hp := range d.newHeaderParts {
		if hdrPart, ok := d.headers[hp.partName]; ok {
			data := marshalHdrFtrXML(hdrPart.hdr, "hdr")
			if err := writer.WritePart(hp.partName, opc.ContentTypeDocHeader, data); err != nil {
				return err
			}
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     hp.relID,
			Type:   opc.RelTypeHeader,
			Target: hp.partName[len("/word/"):],
		})
	}
	for _, fp := range d.newFooterParts {
		if ftrPart, ok := d.footers[fp.partName]; ok {
			data := marshalHdrFtrXML(ftrPart.ftr, "ftr")
			if err := writer.WritePart(fp.partName, opc.ContentTypeDocFooter, data); err != nil {
				return err
			}
		}
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
	writer.AddRelationship(opc.RelTypeOfficeDocument, "word/document.xml", opc.TargetModeInternal)

	return nil
}

// writeDocumentRelationships writes the document.xml.rels file.
func (d *Document) writeDocumentRelationships(writer *opc.Writer) error {
	rels, ok := d.relationships["/word/document.xml"]
	if !ok || len(rels) == 0 {
		return nil
	}

	data, err := opc.MarshalRelationships(rels)
	if err != nil {
		return err
	}

	return writer.WritePart("/word/_rels/document.xml.rels", opc.ContentTypeRelationships, data)
}

// Paragraphs returns all paragraphs in the document body.
func (d *Document) Paragraphs() []*Paragraph {
	if d.document == nil || d.document.Body == nil {
		return nil
	}
	result := make([]*Paragraph, len(d.document.Body.P))
	for i, p := range d.document.Body.P {
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
		result[i] = &Table{tbl: tbl}
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
	// Create rows with cells
	for i := 0; i < rows; i++ {
		tr := &oxml.CT_Tr{}
		for j := 0; j < cols; j++ {
			tc := &oxml.CT_Tc{
				P: []*oxml.CT_P{{}},
			}
			tr.Tc = append(tr.Tc, tc)
		}
		tbl.Tr = append(tbl.Tr, tr)
	}
	d.document.Body.AppendTbl(tbl)
	return &Table{tbl: tbl}
}

// nextRelID returns the next available relationship ID number.
func (d *Document) nextRelID() int {
	if d.nextRelIDVal == 0 {
		// Seed past the highest existing numeric rId, not the count: rIds are
		// often non-contiguous after Word edits (e.g. rId1, rId3), so seeding
		// from the count would collide with an existing id.
		max := 0
		for _, rel := range d.relationships["/word/document.xml"] {
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

// addDocRelationship adds a relationship to the document.xml relationships.
func (d *Document) addDocRelationship(rel *opc.Relationship) {
	d.relationships["/word/document.xml"] = append(d.relationships["/word/document.xml"], rel)
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
// last paragraph and creating a new default section.
func (d *Document) AddSectionBreak() *Section {
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}

	// Move current body sectPr into the last paragraph
	oldSectPr := d.document.Body.SectPr
	if oldSectPr == nil {
		oldSectPr = &oxml.CT_SectPr{}
	}

	// Ensure there's at least one paragraph to attach the section break to
	if len(d.document.Body.P) == 0 {
		d.AddParagraph()
	}

	lastP := d.document.Body.P[len(d.document.Body.P)-1]
	if lastP.PPr == nil {
		lastP.PPr = &oxml.CT_PPr{}
	}
	lastP.PPr.SectPr = oldSectPr

	// Create new body-level section
	d.document.Body.SectPr = &oxml.CT_SectPr{}
	return &Section{sectPr: d.document.Body.SectPr}
}
