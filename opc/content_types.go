package opc

import (
	"encoding/xml"
	"path"
	"strings"
)

// Common content types used in OOXML documents.
const (
	ContentTypeRelationships = "application/vnd.openxmlformats-package.relationships+xml"
	ContentTypeCoreProps     = "application/vnd.openxmlformats-package.core-properties+xml"
	ContentTypeExtendedProps = "application/vnd.openxmlformats-officedocument.extended-properties+xml"

	// PowerPoint content types
	ContentTypePresentationMain    = "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
	ContentTypeSlide               = "application/vnd.openxmlformats-officedocument.presentationml.slide+xml"
	ContentTypeSlideLayout         = "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"
	ContentTypeSlideMaster         = "application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"
	ContentTypeTheme               = "application/vnd.openxmlformats-officedocument.theme+xml"
	ContentTypePresentationProps   = "application/vnd.openxmlformats-officedocument.presentationml.presProps+xml"
	ContentTypeViewProps           = "application/vnd.openxmlformats-officedocument.presentationml.viewProps+xml"
	ContentTypeTableStyles         = "application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"

	// Excel content types
	ContentTypeWorkbook      = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
	ContentTypeWorksheet     = "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"
	ContentTypeSharedStrings = "application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"
	ContentTypeStyles        = "application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"

	// Word content types
	ContentTypeDocument = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
	ContentTypeDocStyles = "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"
	ContentTypeNumbering = "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"

	// Image content types
	ContentTypePNG  = "image/png"
	ContentTypeJPEG = "image/jpeg"
	ContentTypeGIF  = "image/gif"
	ContentTypeBMP  = "image/bmp"
	ContentTypeTIFF = "image/tiff"
	ContentTypeWMF  = "image/x-wmf"
	ContentTypeEMF  = "image/x-emf"
)

// ContentTypes manages the content types for parts in a package.
type ContentTypes struct {
	// Defaults maps file extensions to content types.
	Defaults map[string]string

	// Overrides maps specific part names to content types.
	Overrides map[string]string
}

// NewContentTypes creates a new ContentTypes with default extension mappings.
func NewContentTypes() *ContentTypes {
	return &ContentTypes{
		Defaults: map[string]string{
			"rels": ContentTypeRelationships,
			"xml":  "application/xml",
			"png":  ContentTypePNG,
			"jpeg": ContentTypeJPEG,
			"jpg":  ContentTypeJPEG,
			"gif":  ContentTypeGIF,
			"bmp":  ContentTypeBMP,
		},
		Overrides: make(map[string]string),
	}
}

// GetContentType returns the content type for a part.
// It first checks overrides, then defaults by extension.
func (ct *ContentTypes) GetContentType(partName string) string {
	// Check overrides first
	if contentType, ok := ct.Overrides[partName]; ok {
		return contentType
	}

	// Get extension and check defaults
	ext := strings.TrimPrefix(path.Ext(partName), ".")
	ext = strings.ToLower(ext)
	if contentType, ok := ct.Defaults[ext]; ok {
		return contentType
	}

	return ""
}

// SetDefault sets a default content type for a file extension.
func (ct *ContentTypes) SetDefault(extension, contentType string) {
	ext := strings.TrimPrefix(extension, ".")
	ext = strings.ToLower(ext)
	ct.Defaults[ext] = contentType
}

// SetOverride sets a content type override for a specific part.
func (ct *ContentTypes) SetOverride(partName, contentType string) {
	ct.Overrides[partName] = contentType
}

// contentTypesXML is the XML structure for [Content_Types].xml
type contentTypesXML struct {
	XMLName   xml.Name            `xml:"Types"`
	Xmlns     string              `xml:"xmlns,attr"`
	Defaults  []defaultXML        `xml:"Default"`
	Overrides []overrideXML       `xml:"Override"`
}

type defaultXML struct {
	Extension   string `xml:"Extension,attr"`
	ContentType string `xml:"ContentType,attr"`
}

type overrideXML struct {
	PartName    string `xml:"PartName,attr"`
	ContentType string `xml:"ContentType,attr"`
}

// ContentTypesNamespace is the XML namespace for content types.
const ContentTypesNamespace = "http://schemas.openxmlformats.org/package/2006/content-types"

// Marshal converts ContentTypes to XML bytes.
func (ct *ContentTypes) Marshal() ([]byte, error) {
	ctXML := contentTypesXML{
		Xmlns:     ContentTypesNamespace,
		Defaults:  make([]defaultXML, 0, len(ct.Defaults)),
		Overrides: make([]overrideXML, 0, len(ct.Overrides)),
	}

	for ext, contentType := range ct.Defaults {
		ctXML.Defaults = append(ctXML.Defaults, defaultXML{
			Extension:   ext,
			ContentType: contentType,
		})
	}

	for partName, contentType := range ct.Overrides {
		ctXML.Overrides = append(ctXML.Overrides, overrideXML{
			PartName:    partName,
			ContentType: contentType,
		})
	}

	output, err := xml.MarshalIndent(ctXML, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), output...), nil
}

// UnmarshalContentTypes parses content types XML into a ContentTypes struct.
func UnmarshalContentTypes(data []byte) (*ContentTypes, error) {
	var ctXML contentTypesXML
	if err := xml.Unmarshal(data, &ctXML); err != nil {
		return nil, err
	}

	ct := &ContentTypes{
		Defaults:  make(map[string]string, len(ctXML.Defaults)),
		Overrides: make(map[string]string, len(ctXML.Overrides)),
	}

	for _, def := range ctXML.Defaults {
		ct.Defaults[strings.ToLower(def.Extension)] = def.ContentType
	}

	for _, override := range ctXML.Overrides {
		ct.Overrides[override.PartName] = override.ContentType
	}

	return ct, nil
}
