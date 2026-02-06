// Package docprops provides document property types from shared-documentPropertiesCore.xsd
// and shared-documentPropertiesExtended.xsd (Dublin Core metadata).
// These types implement the cp: and dc: namespace elements.
package docprops

// CoreProperties represents CT_CoreProperties (cp:coreProperties)
// Based on Dublin Core metadata and Office-specific extensions
type CoreProperties struct {
	// Dublin Core elements
	Category       string `xml:"http://purl.org/dc/elements/1.1/ category,omitempty"`
	ContentStatus  string `xml:"http://purl.org/dc/elements/1.1/ contentStatus,omitempty"`
	Created        string `xml:"http://purl.org/dc/terms/ created,omitempty"`     // W3CDTF datetime
	Creator        string `xml:"http://purl.org/dc/elements/1.1/ creator,omitempty"`
	Description    string `xml:"http://purl.org/dc/elements/1.1/ description,omitempty"`
	Identifier     string `xml:"http://purl.org/dc/elements/1.1/ identifier,omitempty"`
	Keywords       string `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties keywords,omitempty"`
	Language       string `xml:"http://purl.org/dc/elements/1.1/ language,omitempty"`
	LastModifiedBy string `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties lastModifiedBy,omitempty"`
	LastPrinted    string `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties lastPrinted,omitempty"`
	Modified       string `xml:"http://purl.org/dc/terms/ modified,omitempty"`    // W3CDTF datetime
	Revision       string `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties revision,omitempty"`
	Subject        string `xml:"http://purl.org/dc/elements/1.1/ subject,omitempty"`
	Title          string `xml:"http://purl.org/dc/elements/1.1/ title,omitempty"`
	Version        string `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties version,omitempty"`
}

// ExtendedProperties represents CT_Properties (extended document properties)
// Based on shared-documentPropertiesExtended.xsd
type ExtendedProperties struct {
	// Application info
	Application     string `xml:"Application,omitempty"`
	AppVersion      string `xml:"AppVersion,omitempty"`
	DocSecurity     int32  `xml:"DocSecurity,omitempty"`
	ScaleCrop       bool   `xml:"ScaleCrop,omitempty"`
	LinksUpToDate   bool   `xml:"LinksUpToDate,omitempty"`
	SharedDoc       bool   `xml:"SharedDoc,omitempty"`
	HyperlinksChanged bool `xml:"HyperlinksChanged,omitempty"`

	// Document statistics
	Template       string `xml:"Template,omitempty"`
	TotalTime      int32  `xml:"TotalTime,omitempty"` // editing time in minutes
	Words          int32  `xml:"Words,omitempty"`
	Pages          int32  `xml:"Pages,omitempty"`
	Paragraphs     int32  `xml:"Paragraphs,omitempty"`
	Lines          int32  `xml:"Lines,omitempty"`
	Characters     int32  `xml:"Characters,omitempty"`
	CharactersWithSpaces int32 `xml:"CharactersWithSpaces,omitempty"`

	// Presentation-specific
	Slides           int32  `xml:"Slides,omitempty"`
	Notes            int32  `xml:"Notes,omitempty"`
	HiddenSlides     int32  `xml:"HiddenSlides,omitempty"`
	MMClips          int32  `xml:"MMClips,omitempty"` // multimedia clips
	PresentationFormat string `xml:"PresentationFormat,omitempty"`

	// Spreadsheet-specific
	Worksheets int32 `xml:"Worksheets,omitempty"`

	// Manager and company
	Manager string `xml:"Manager,omitempty"`
	Company string `xml:"Company,omitempty"`

	// Heading pairs and titles
	HeadingPairs *VectorVariant `xml:"HeadingPairs,omitempty"`
	TitlesOfParts *VectorLpstr  `xml:"TitlesOfParts,omitempty"`

	// Hyperlinks
	HLinks *VectorVariant `xml:"HLinks,omitempty"`

	// Digital signature
	DigSig *DigSigBlob `xml:"DigSig,omitempty"`
}

// VectorVariant represents vt:vector with variant elements
type VectorVariant struct {
	BaseType string     `xml:"baseType,attr"`
	Size     uint32     `xml:"size,attr"`
	Variant  []*Variant `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes variant,omitempty"`
}

// VectorLpstr represents vt:vector with lpstr elements
type VectorLpstr struct {
	BaseType string   `xml:"baseType,attr"`
	Size     uint32   `xml:"size,attr"`
	Lpstr    []string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes lpstr,omitempty"`
}

// Variant represents vt:variant choice group
type Variant struct {
	Lpstr  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes lpstr,omitempty"`
	I4     int32  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes i4,omitempty"`
	Bool   bool   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes bool,omitempty"`
	Vector *VectorVariant `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes vector,omitempty"`
}

// DigSigBlob represents CT_DigSigBlob
type DigSigBlob struct {
	Blob string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes blob,omitempty"`
}

// CustomProperties represents CT_Properties (custom document properties)
type CustomProperties struct {
	Property []*CustomDocumentProperty `xml:"property,omitempty"`
}

// CustomDocumentProperty represents CT_Property
type CustomDocumentProperty struct {
	FMTID   string `xml:"fmtid,attr"`
	PID     int32  `xml:"pid,attr"`
	Name    string `xml:"name,attr"`
	LinkTarget string `xml:"linkTarget,attr,omitempty"`
	// Value choices
	Lpstr    string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes lpstr,omitempty"`
	Lpwstr   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes lpwstr,omitempty"`
	I4       *int32 `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes i4,omitempty"`
	R8       *float64 `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes r8,omitempty"`
	Filetime string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes filetime,omitempty"`
	Bool     *bool  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes bool,omitempty"`
}
