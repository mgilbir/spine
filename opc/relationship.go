package opc

import (
	"bytes"
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TargetMode specifies how to interpret the target URI of a relationship.
type TargetMode string

const (
	// TargetModeInternal indicates the target is a part within the package.
	TargetModeInternal TargetMode = "Internal"

	// TargetModeExternal indicates the target is an external resource.
	TargetModeExternal TargetMode = "External"
)

// Common relationship types used in OOXML documents.
const (
	// Core properties relationship type
	RelTypeCore = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"

	// Extended properties relationship type
	RelTypeExtended = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties"

	// Thumbnail relationship type
	RelTypeThumbnail = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail"

	// Office document relationship type
	RelTypeOfficeDocument = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"

	// Digital signature relationship type
	RelTypeDigitalSignature = "http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/signature"

	// Slide relationship type
	RelTypeSlide = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"

	// Notes slide relationship type
	RelTypeNotesSlide = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"

	// Slide master relationship type
	RelTypeSlideMaster = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster"

	// Slide layout relationship type
	RelTypeSlideLayout = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout"

	// Theme relationship type
	RelTypeTheme = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"

	// Presentation properties relationship type
	RelTypePresProps = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps"

	// View properties relationship type
	RelTypeViewProps = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps"

	// Table styles relationship type
	RelTypeTableStyles = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles"

	// Word relationship types
	RelTypeStyles    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	RelTypeNumbering = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"
	RelTypeSettings  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"
	RelTypeFontTable = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/fontTable"
	RelTypeImage     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"

	// Media relationship types. Embedded video/audio uses two relationships to
	// the same media part: a "video"/"audio" link reference (a:videoFile/
	// a:audioFile r:link) and a Microsoft "media" embed reference (p14:media
	// r:embed).
	RelTypeVideo       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/video"
	RelTypeAudio       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/audio"
	RelTypeMedia       = "http://schemas.microsoft.com/office/2007/relationships/media"
	RelTypeHeader      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header"
	RelTypeFooter      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer"
	RelTypeFootnotes   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes"
	RelTypeEndnotes    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes"
	RelTypeComments    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	RelTypeHyperlink   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	RelTypeWebSettings = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/webSettings"

	// Microsoft comment-extension relationship types. Modern Word writes these
	// alongside the ECMA-376 comments part: commentsExtended carries threading
	// and resolved state, people the author registry, and commentsIds /
	// commentsExtensible the durable ids and UTC timestamps.
	RelTypeCommentsExtended   = "http://schemas.microsoft.com/office/2011/relationships/commentsExtended"
	RelTypeCommentsIds        = "http://schemas.microsoft.com/office/2016/09/relationships/commentsIds"
	RelTypeCommentsExtensible = "http://schemas.microsoft.com/office/2018/08/relationships/commentsExtensible"
	RelTypePeople             = "http://schemas.microsoft.com/office/2011/relationships/people"

	// Modern PowerPoint threaded-comment relationship types (2018 schema). A
	// slide references its threaded-comment part with RelTypeModernComments and
	// the presentation references the shared author list with RelTypeAuthors.
	RelTypeModernComments = "http://schemas.microsoft.com/office/2018/10/relationships/comments"
	RelTypeAuthors        = "http://schemas.microsoft.com/office/2018/10/relationships/authors"

	// SpreadsheetML relationship types
	RelTypeWorksheet     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"
	RelTypeSharedStrings = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings"
	RelTypeCalcChain     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/calcChain"
	RelTypeDrawing       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing"
	RelTypeChart         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart"
	RelTypePivotTable    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotTable"

	// RelTypePackage links a part to an embedded OPC package, e.g. a chart part
	// to the .xlsx workbook holding its editable data (c:externalData r:id).
	RelTypePackage       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/package"
	RelTypePivotCacheDef = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotCacheDefinition"

	// SpreadsheetML comment relationship types. RelTypeComments (the shared
	// 2006 comments URI, declared above) links a worksheet to its legacy
	// comments part; RelTypeVMLDrawing links it to the VML drawing that renders
	// the note boxes. RelTypeThreadedComment links a worksheet to its threaded
	// comments part, and RelTypePerson links the workbook to its person-list
	// part. The threaded/person relationship types are Microsoft 2017/10
	// extensions.
	RelTypeVMLDrawing      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/vmlDrawing"
	RelTypeThreadedComment = "http://schemas.microsoft.com/office/2017/10/relationships/threadedComment"
	RelTypePerson          = "http://schemas.microsoft.com/office/2017/10/relationships/person"
)

// Relationship represents a relationship between a source part and a target.
type Relationship struct {
	// ID is the unique identifier of the relationship within its source.
	ID string

	// Type is the relationship type URI.
	Type string

	// Target is the URI of the target part or external resource.
	Target string

	// TargetMode indicates whether the target is internal or external.
	TargetMode TargetMode
}

// IsExternal returns true if the relationship targets an external resource.
func (r *Relationship) IsExternal() bool {
	return r.TargetMode == TargetModeExternal
}

// relationshipsXML is the XML structure for the .rels files.
type relationshipsXML struct {
	XMLName       xml.Name          `xml:"Relationships"`
	Xmlns         string            `xml:"xmlns,attr"`
	Relationships []relationshipXML `xml:"Relationship"`
}

// relationshipXML is a single relationship element.
type relationshipXML struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr,omitempty"`
}

// RelationshipsNamespace is the XML namespace for relationship files.
const RelationshipsNamespace = "http://schemas.openxmlformats.org/package/2006/relationships"

// MarshalRelationships converts a slice of relationships to XML bytes.
// Output format matches Microsoft Office: compact single-line with self-closing elements.
func MarshalRelationships(rels []*Relationship) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	buf.WriteByte('\r')
	buf.WriteByte('\n')
	buf.WriteString(`<Relationships xmlns="`)
	buf.WriteString(RelationshipsNamespace)
	buf.WriteString(`">`)
	for _, rel := range rels {
		buf.WriteString(`<Relationship Id="`)
		buf.WriteString(xmlb.EscapeAttrValue(rel.ID))
		buf.WriteString(`" Type="`)
		buf.WriteString(xmlb.EscapeAttrValue(rel.Type))
		buf.WriteString(`" Target="`)
		buf.WriteString(xmlb.EscapeAttrValue(rel.Target))
		buf.WriteByte('"')
		if rel.TargetMode == TargetModeExternal {
			buf.WriteString(` TargetMode="External"`)
		}
		buf.WriteString("/>")
	}
	buf.WriteString("</Relationships>")
	return buf.Bytes(), nil
}

// RelationshipsEqual reports whether a and b contain the same relationships in
// the same order. Save paths use it to detect that a parsed relationship set
// was not modified, so the source .rels bytes can be preserved verbatim
// instead of regenerated in canonical form.
func RelationshipsEqual(a, b []*Relationship) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] == nil || b[i] == nil {
			if a[i] != b[i] {
				return false
			}
			continue
		}
		if *a[i] != *b[i] {
			return false
		}
	}
	return true
}

// RelationshipsEquivalent reports whether a and b contain the same
// relationship set regardless of document order: the same IDs, each mapping
// to the same Type, Target, and TargetMode. Under OPC the order of
// <Relationship> elements in a .rels part carries no meaning, so a save path
// that reconstructed the same set in a different order may still write the
// source bytes verbatim. Duplicate or nil entries make the comparison fail
// (conservatively) rather than guess.
func RelationshipsEquivalent(a, b []*Relationship) bool {
	if len(a) != len(b) {
		return false
	}
	index := make(map[string]*Relationship, len(a))
	for _, rel := range a {
		if rel == nil {
			return false
		}
		if _, dup := index[rel.ID]; dup {
			return false
		}
		index[rel.ID] = rel
	}
	for _, rel := range b {
		if rel == nil {
			return false
		}
		orig, ok := index[rel.ID]
		if !ok || *orig != *rel {
			return false
		}
		delete(index, rel.ID)
	}
	return true
}

// UnmarshalRelationships parses relationship XML into a slice of relationships.
func UnmarshalRelationships(data []byte) ([]*Relationship, error) {
	var relsXML relationshipsXML
	if err := xmlb.Unmarshal(data, &relsXML); err != nil {
		return nil, err
	}

	rels := make([]*Relationship, len(relsXML.Relationships))
	for i, relXML := range relsXML.Relationships {
		targetMode := TargetModeInternal
		if relXML.TargetMode == string(TargetModeExternal) {
			targetMode = TargetModeExternal
		}

		rels[i] = &Relationship{
			ID:         relXML.ID,
			Type:       relXML.Type,
			Target:     relXML.Target,
			TargetMode: targetMode,
		}
	}

	return rels, nil
}

// GetRelationshipsPartName returns the relationships part name for a given part.
// For the package root, partName should be empty or "/".
func GetRelationshipsPartName(partName string) string {
	if partName == "" || partName == "/" {
		return "/_rels/.rels"
	}

	// Insert "_rels/" before the filename and add ".rels" extension
	dir := ""
	name := partName

	// Find last slash
	for i := len(partName) - 1; i >= 0; i-- {
		if partName[i] == '/' {
			dir = partName[:i+1]
			name = partName[i+1:]
			break
		}
	}

	return dir + "_rels/" + name + ".rels"
}
