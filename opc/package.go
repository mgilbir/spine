package opc

import (
	"encoding/xml"
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
type corePropertiesXML struct {
	XMLName xml.Name `xml:"coreProperties"`

	Category       string  `xml:"category,omitempty"`
	ContentStatus  string  `xml:"contentStatus,omitempty"`
	Created        *dcDate `xml:"created,omitempty"`
	Creator        string  `xml:"creator,omitempty"`
	Description    string  `xml:"description,omitempty"`
	Identifier     string  `xml:"identifier,omitempty"`
	Keywords       string  `xml:"keywords,omitempty"`
	Language       string  `xml:"language,omitempty"`
	LastModifiedBy string  `xml:"lastModifiedBy,omitempty"`
	LastPrinted    *dcDate `xml:"lastPrinted,omitempty"`
	Modified       *dcDate `xml:"modified,omitempty"`
	Revision       string  `xml:"revision,omitempty"`
	Subject        string  `xml:"subject,omitempty"`
	Title          string  `xml:"title,omitempty"`
	Version        string  `xml:"version,omitempty"`
}

// dcDate represents a Dublin Core date with xsi:type attribute.
type dcDate struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",chardata"`
}

const (
	nsCoreProperties = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
	nsDublinCore     = "http://purl.org/dc/elements/1.1/"
	nsDcTerms        = "http://purl.org/dc/terms/"
	nsDcmiType       = "http://purl.org/dc/dcmitype/"
	nsXsi            = "http://www.w3.org/2001/XMLSchema-instance"
)

// Marshal converts CoreProperties to XML bytes.
func (cp *CoreProperties) Marshal() ([]byte, error) {
	cpXML := corePropertiesXML{
		Category:       cp.Category,
		ContentStatus:  cp.ContentStatus,
		Creator:        cp.Creator,
		Description:    cp.Description,
		Identifier:     cp.Identifier,
		Keywords:       cp.Keywords,
		Language:       cp.Language,
		LastModifiedBy: cp.LastModifiedBy,
		Revision:       cp.Revision,
		Subject:        cp.Subject,
		Title:          cp.Title,
		Version:        cp.Version,
	}

	if !cp.Created.IsZero() {
		cpXML.Created = &dcDate{
			Type:  "dcterms:W3CDTF",
			Value: cp.Created.Format(time.RFC3339),
		}
	}

	if !cp.Modified.IsZero() {
		cpXML.Modified = &dcDate{
			Type:  "dcterms:W3CDTF",
			Value: cp.Modified.Format(time.RFC3339),
		}
	}

	if !cp.LastPrinted.IsZero() {
		cpXML.LastPrinted = &dcDate{
			Value: cp.LastPrinted.Format(time.RFC3339),
		}
	}

	output, err := xml.MarshalIndent(cpXML, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), output...), nil
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
