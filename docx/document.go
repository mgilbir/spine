package docx

import (
	"encoding/xml"
	"fmt"
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
	reader, err := opc.OpenReader(path)
	if err != nil {
		return nil, err
	}

	return openFromReader(reader)
}

// openFromReader creates a Document from an OPC reader.
func openFromReader(reader *opc.ReadCloser) (*Document, error) {
	rels := reader.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 {
		reader.Close()
		return nil, ErrNotDOCX
	}

	mainPartName := opc.ResolvePartName("/", rels[0].Target)
	mainPart := reader.GetFile(mainPartName)
	if mainPart == nil {
		reader.Close()
		return nil, ErrNotDOCX
	}

	if mainPart.ContentType != opc.ContentTypeDocument {
		reader.Close()
		return nil, ErrNotDOCX
	}

	data, err := mainPart.ReadAll()
	if err != nil {
		reader.Close()
		return nil, err
	}

	var doc oxml.CT_Document
	if err := xml.Unmarshal(data, &doc); err != nil {
		reader.Close()
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
		reader.Close()
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
			xml.Unmarshal(data, d.styles)
		case name == "/word/numbering.xml":
			d.numbering = &oxml.CT_Numbering{}
			xml.Unmarshal(data, d.numbering)
		case name == "/word/settings.xml":
			d.settings = &oxml.CT_Settings{}
			xml.Unmarshal(data, d.settings)
		case name == "/word/footnotes.xml":
			d.footnotes = &oxml.CT_Footnotes{}
			xml.Unmarshal(data, d.footnotes)
		case name == "/word/endnotes.xml":
			d.endnotes = &oxml.CT_Endnotes{}
			xml.Unmarshal(data, d.endnotes)
		case name == "/word/comments.xml":
			d.comments = &oxml.CT_Comments{}
			xml.Unmarshal(data, d.comments)
		case name == "/word/fontTable.xml":
			// preserved in preservedParts
		case name == "/word/webSettings.xml":
			// preserved in preservedParts
		case strings.HasPrefix(name, "/word/header") && strings.HasSuffix(name, ".xml"):
			hdr := &oxml.CT_HdrFtr{}
			xml.Unmarshal(data, hdr)
			d.headers[name] = &headerPart{hdr: hdr, contentType: file.ContentType}
		case strings.HasPrefix(name, "/word/footer") && strings.HasSuffix(name, ".xml"):
			ftr := &oxml.CT_HdrFtr{}
			xml.Unmarshal(data, ftr)
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
	writer, err := opc.Create(path)
	if err != nil {
		return err
	}
	defer writer.Close()

	return d.saveTo(writer)
}

// Close closes the document and releases resources.
func (d *Document) Close() error {
	if d.reader != nil {
		return d.reader.Close()
	}
	return nil
}

// saveTo writes the document to an OPC writer.
func (d *Document) saveTo(writer *opc.Writer) error {
	if d.reader != nil {
		return d.saveRoundTrip(writer)
	}
	return d.saveNew(writer)
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

	// Write document relationships
	var docRels []*opc.Relationship
	relID := 1

	// Write default styles
	if d.styles != nil {
		data := marshalStylesXML(d.styles)
		if err := writer.WritePart("/word/styles.xml", opc.ContentTypeDocStyles, data); err != nil {
			return err
		}
		docRels = append(docRels, &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", relID),
			Type:   opc.RelTypeStyles,
			Target: "styles.xml",
		})
		relID++
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
	d.document.Body.P = append(d.document.Body.P, p)
	return &Paragraph{document: d, p: p}
}

// AddParagraphWithText adds a new paragraph with the specified text.
func (d *Document) AddParagraphWithText(text string) *Paragraph {
	p := d.AddParagraph()
	p.AddRun().SetText(text)
	return p
}

// AddHeading adds a heading paragraph with the specified level (1-9).
func (d *Document) AddHeading(text string, level int) *Paragraph {
	p := d.AddParagraph()
	p.SetStyle("Heading" + string(rune('0'+level)))
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

// Section represents a document section.
type Section struct {
	sectPr *oxml.CT_SectPr
}
