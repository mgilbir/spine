package opc

import (
	"encoding/xml"
	"strings"
	"time"
)

// CoreProperties contains the core document properties as defined in OPC.
type CoreProperties struct {
	// Category of the document
	Category string

	// ContentStatus (e.g., "Draft", "Final")
	ContentStatus string

	// Created is the creation date
	Created time.Time

	// Creator is the primary author
	Creator string

	// Description of the document content
	Description string

	// Identifier (e.g., ISBN, URL)
	Identifier string

	// Keywords for the document
	Keywords string

	// Language of the content (e.g., "en-US")
	Language string

	// LastModifiedBy is the last editor
	LastModifiedBy string

	// LastPrinted is when the document was last printed
	LastPrinted time.Time

	// Modified is the last modification date
	Modified time.Time

	// Revision number
	Revision string

	// Subject of the document
	Subject string

	// Title of the document
	Title string

	// Version number
	Version string
}

// corePropertiesXML is the XML representation of core properties.
// This is used for unmarshaling only - marshaling uses custom logic.
// It handles both namespaced (OOXML-compliant) and non-namespaced (legacy) formats.
type corePropertiesXML struct {
	XMLName xml.Name `xml:"coreProperties"`

	// Core Properties elements (cp: namespace)
	Category       string  `xml:"category,omitempty"`
	ContentStatus  string  `xml:"contentStatus,omitempty"`
	Keywords       string  `xml:"keywords,omitempty"`
	LastModifiedBy string  `xml:"lastModifiedBy,omitempty"`
	Revision       string  `xml:"revision,omitempty"`
	Version        string  `xml:"version,omitempty"`
	LastPrinted    *dcDate `xml:"lastPrinted,omitempty"`

	// Dublin Core elements (dc: namespace)
	Creator     string `xml:"creator,omitempty"`
	Description string `xml:"description,omitempty"`
	Identifier  string `xml:"identifier,omitempty"`
	Language    string `xml:"language,omitempty"`
	Subject     string `xml:"subject,omitempty"`
	Title       string `xml:"title,omitempty"`

	// Dublin Core Terms elements (dcterms: namespace)
	Created  *dcDate `xml:"created,omitempty"`
	Modified *dcDate `xml:"modified,omitempty"`
}

// dcDate represents a Dublin Core date with xsi:type attribute.
type dcDate struct {
	Type  string `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr,omitempty"`
	Value string `xml:",chardata"`
}

const (
	nsCoreProperties = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
	nsDublinCore     = "http://purl.org/dc/elements/1.1/"
	nsDcTerms        = "http://purl.org/dc/terms/"
	nsDcmiType       = "http://purl.org/dc/dcmitype/"
	nsXsi            = "http://www.w3.org/2001/XMLSchema-instance"
)

// Marshal converts CoreProperties to XML bytes with proper OOXML namespaces.
func (cp *CoreProperties) Marshal() ([]byte, error) {
	var b strings.Builder

	b.WriteString(xml.Header)
	b.WriteString(`<cp:coreProperties`)
	b.WriteString(` xmlns:cp="` + nsCoreProperties + `"`)
	b.WriteString(` xmlns:dc="` + nsDublinCore + `"`)
	b.WriteString(` xmlns:dcterms="` + nsDcTerms + `"`)
	b.WriteString(` xmlns:dcmitype="` + nsDcmiType + `"`)
	b.WriteString(` xmlns:xsi="` + nsXsi + `"`)
	b.WriteString(">\n")

	// Dublin Core elements (dc: prefix)
	if cp.Title != "" {
		b.WriteString("  <dc:title>" + xmlEscape(cp.Title) + "</dc:title>\n")
	}
	if cp.Subject != "" {
		b.WriteString("  <dc:subject>" + xmlEscape(cp.Subject) + "</dc:subject>\n")
	}
	if cp.Creator != "" {
		b.WriteString("  <dc:creator>" + xmlEscape(cp.Creator) + "</dc:creator>\n")
	}
	if cp.Description != "" {
		b.WriteString("  <dc:description>" + xmlEscape(cp.Description) + "</dc:description>\n")
	}
	if cp.Identifier != "" {
		b.WriteString("  <dc:identifier>" + xmlEscape(cp.Identifier) + "</dc:identifier>\n")
	}
	if cp.Language != "" {
		b.WriteString("  <dc:language>" + xmlEscape(cp.Language) + "</dc:language>\n")
	}

	// Core Properties elements (cp: prefix)
	if cp.Keywords != "" {
		b.WriteString("  <cp:keywords>" + xmlEscape(cp.Keywords) + "</cp:keywords>\n")
	}
	if cp.LastModifiedBy != "" {
		b.WriteString("  <cp:lastModifiedBy>" + xmlEscape(cp.LastModifiedBy) + "</cp:lastModifiedBy>\n")
	}
	if cp.Revision != "" {
		b.WriteString("  <cp:revision>" + xmlEscape(cp.Revision) + "</cp:revision>\n")
	}
	if cp.Category != "" {
		b.WriteString("  <cp:category>" + xmlEscape(cp.Category) + "</cp:category>\n")
	}
	if cp.ContentStatus != "" {
		b.WriteString("  <cp:contentStatus>" + xmlEscape(cp.ContentStatus) + "</cp:contentStatus>\n")
	}
	if cp.Version != "" {
		b.WriteString("  <cp:version>" + xmlEscape(cp.Version) + "</cp:version>\n")
	}

	// Dublin Core Terms elements (dcterms: prefix) - dates with xsi:type
	if !cp.Created.IsZero() {
		b.WriteString(`  <dcterms:created xsi:type="dcterms:W3CDTF">`)
		b.WriteString(cp.Created.UTC().Format(time.RFC3339))
		b.WriteString("</dcterms:created>\n")
	}
	if !cp.Modified.IsZero() {
		b.WriteString(`  <dcterms:modified xsi:type="dcterms:W3CDTF">`)
		b.WriteString(cp.Modified.UTC().Format(time.RFC3339))
		b.WriteString("</dcterms:modified>\n")
	}
	if !cp.LastPrinted.IsZero() {
		b.WriteString(`  <cp:lastPrinted>`)
		b.WriteString(cp.LastPrinted.UTC().Format(time.RFC3339))
		b.WriteString("</cp:lastPrinted>\n")
	}

	b.WriteString("</cp:coreProperties>")

	return []byte(b.String()), nil
}

// xmlEscape escapes special XML characters in a string.
func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// UnmarshalCoreProperties parses core properties XML into a CoreProperties struct.
func UnmarshalCoreProperties(data []byte) (*CoreProperties, error) {
	var cpXML corePropertiesXML
	if err := xml.Unmarshal(data, &cpXML); err != nil {
		return nil, err
	}

	cp := &CoreProperties{
		Category:       cpXML.Category,
		ContentStatus:  cpXML.ContentStatus,
		Creator:        cpXML.Creator,
		Description:    cpXML.Description,
		Identifier:     cpXML.Identifier,
		Keywords:       cpXML.Keywords,
		Language:       cpXML.Language,
		LastModifiedBy: cpXML.LastModifiedBy,
		Revision:       cpXML.Revision,
		Subject:        cpXML.Subject,
		Title:          cpXML.Title,
		Version:        cpXML.Version,
	}

	if cpXML.Created != nil {
		t, err := time.Parse(time.RFC3339, cpXML.Created.Value)
		if err == nil {
			cp.Created = t
		}
	}

	if cpXML.Modified != nil {
		t, err := time.Parse(time.RFC3339, cpXML.Modified.Value)
		if err == nil {
			cp.Modified = t
		}
	}

	if cpXML.LastPrinted != nil {
		t, err := time.Parse(time.RFC3339, cpXML.LastPrinted.Value)
		if err == nil {
			cp.LastPrinted = t
		}
	}

	return cp, nil
}
