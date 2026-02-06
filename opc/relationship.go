package opc

import (
	"bytes"
	"encoding/xml"
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
		xml.EscapeText(&buf, []byte(rel.ID))
		buf.WriteString(`" Type="`)
		xml.EscapeText(&buf, []byte(rel.Type))
		buf.WriteString(`" Target="`)
		xml.EscapeText(&buf, []byte(rel.Target))
		buf.WriteByte('"')
		if rel.TargetMode == TargetModeExternal {
			buf.WriteString(` TargetMode="External"`)
		}
		buf.WriteString("/>")
	}
	buf.WriteString("</Relationships>")
	return buf.Bytes(), nil
}

// UnmarshalRelationships parses relationship XML into a slice of relationships.
func UnmarshalRelationships(data []byte) ([]*Relationship, error) {
	var relsXML relationshipsXML
	if err := xml.Unmarshal(data, &relsXML); err != nil {
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
