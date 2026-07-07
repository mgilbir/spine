package opc

import (
	"bytes"
	"encoding/xml"
	"path"
	"sort"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
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
	ContentTypeDocument       = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
	ContentTypeDocStyles      = "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"
	ContentTypeNumbering      = "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"
	ContentTypeDocSettings    = "application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"
	ContentTypeDocFontTable   = "application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"
	ContentTypeDocFootnotes   = "application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"
	ContentTypeDocEndnotes    = "application/vnd.openxmlformats-officedocument.wordprocessingml.endnotes+xml"
	ContentTypeDocComments    = "application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"
	ContentTypeDocHeader      = "application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"
	ContentTypeDocFooter      = "application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"
	ContentTypeDocWebSettings = "application/vnd.openxmlformats-officedocument.wordprocessingml.webSettings+xml"

	// Drawing content type (SpreadsheetML/DrawingML drawing part)
	ContentTypeDrawing = "application/vnd.openxmlformats-officedocument.drawing+xml"

	// Image content types
	ContentTypePNG  = "image/png"
	ContentTypeJPEG = "image/jpeg"
	ContentTypeGIF  = "image/gif"
	ContentTypeBMP  = "image/bmp"
	ContentTypeTIFF = "image/tiff"
	ContentTypeWMF  = "image/x-wmf"
	ContentTypeEMF  = "image/x-emf"
	ContentTypeSVG  = "image/svg+xml"
)

// ContentTypes manages the content types for parts in a package.
type ContentTypes struct {
	// Defaults maps file extensions to content types.
	Defaults map[string]string

	// Overrides maps specific part names to content types.
	Overrides map[string]string

	// defaultOrder preserves the original ordering of default extensions.
	defaultOrder []string

	// overrideOrder preserves the original ordering of override part names.
	overrideOrder []string

	// origExt maps a lowercase extension to the original-cased spelling first
	// seen in the source (or as passed to SetDefault). OPC matches extensions
	// case-insensitively, so Defaults is keyed by lowercase, but byte-identical
	// round-trip requires re-emitting the extension exactly as it appeared
	// (e.g. "JPG", not "jpg").
	origExt map[string]string

	// OriginalXMLSep is the exact byte sequence between the XML declaration and
	// the root <Types> element in the parsed file (typically "\r\n", but some
	// producers use "\n"). It is empty for a ContentTypes created from scratch,
	// which defaults to "\r\n" on marshal.
	OriginalXMLSep string
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
			"svg":  ContentTypeSVG,
		},
		Overrides: make(map[string]string),
		origExt:   make(map[string]string),
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
	orig := strings.TrimPrefix(extension, ".")
	ext := strings.ToLower(orig)
	if _, exists := ct.Defaults[ext]; !exists {
		ct.defaultOrder = append(ct.defaultOrder, ext)
	}
	if ct.origExt == nil {
		ct.origExt = make(map[string]string)
	}
	if _, exists := ct.origExt[ext]; !exists {
		ct.origExt[ext] = orig
	}
	ct.Defaults[ext] = contentType
}

// displayExtension returns the original-cased spelling for a lowercase
// extension key, falling back to the key itself when no original casing was
// recorded.
func (ct *ContentTypes) displayExtension(ext string) string {
	if orig, ok := ct.origExt[ext]; ok {
		return orig
	}
	return ext
}

// SetOverride sets a content type override for a specific part.
func (ct *ContentTypes) SetOverride(partName, contentType string) {
	if _, exists := ct.Overrides[partName]; !exists {
		ct.overrideOrder = append(ct.overrideOrder, partName)
	}
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
// Output format matches Microsoft Office: compact single-line with self-closing elements.
func (ct *ContentTypes) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sep := ct.OriginalXMLSep
	if sep == "" {
		sep = "\r\n"
	}
	buf.WriteString(sep)
	buf.WriteString(`<Types xmlns="`)
	buf.WriteString(ContentTypesNamespace)
	buf.WriteString(`">`)

	// Write defaults: use original order, then append any new entries sorted.
	// The extension is re-emitted in its original case (OPC matches
	// case-insensitively, but round-trip must preserve the source spelling).
	exts := ct.orderedDefaults()
	for _, ext := range exts {
		if contentType, ok := ct.Defaults[ext]; ok {
			buf.WriteString(`<Default Extension="`)
			buf.WriteString(xmlb.EscapeAttrValue(ct.displayExtension(ext)))
			buf.WriteString(`" ContentType="`)
			buf.WriteString(xmlb.EscapeAttrValue(contentType))
			buf.WriteString(`"/>`)
		}
	}

	// Write overrides: use original order, then append any new entries sorted
	parts := ct.orderedOverrides()
	for _, partName := range parts {
		if contentType, ok := ct.Overrides[partName]; ok {
			buf.WriteString(`<Override PartName="`)
			buf.WriteString(xmlb.EscapeAttrValue(partName))
			buf.WriteString(`" ContentType="`)
			buf.WriteString(xmlb.EscapeAttrValue(contentType))
			buf.WriteString(`"/>`)
		}
	}

	buf.WriteString("</Types>")
	return buf.Bytes(), nil
}

// orderedDefaults returns default extensions in stable order:
// original order first, then any new entries sorted alphabetically.
func (ct *ContentTypes) orderedDefaults() []string {
	if len(ct.defaultOrder) == 0 {
		// No original order: sort all keys
		exts := make([]string, 0, len(ct.Defaults))
		for ext := range ct.Defaults {
			exts = append(exts, ext)
		}
		sort.Strings(exts)
		return exts
	}

	// Start with original order
	seen := make(map[string]bool, len(ct.defaultOrder))
	result := make([]string, 0, len(ct.Defaults))
	for _, ext := range ct.defaultOrder {
		if _, ok := ct.Defaults[ext]; ok {
			result = append(result, ext)
			seen[ext] = true
		}
	}

	// Append any new entries not in original order
	var extra []string
	for ext := range ct.Defaults {
		if !seen[ext] {
			extra = append(extra, ext)
		}
	}
	sort.Strings(extra)
	return append(result, extra...)
}

// orderedOverrides returns override part names in stable order:
// original order first, then any new entries sorted alphabetically.
func (ct *ContentTypes) orderedOverrides() []string {
	if len(ct.overrideOrder) == 0 {
		parts := make([]string, 0, len(ct.Overrides))
		for partName := range ct.Overrides {
			parts = append(parts, partName)
		}
		sort.Strings(parts)
		return parts
	}

	seen := make(map[string]bool, len(ct.overrideOrder))
	result := make([]string, 0, len(ct.Overrides))
	for _, partName := range ct.overrideOrder {
		if _, ok := ct.Overrides[partName]; ok {
			result = append(result, partName)
			seen[partName] = true
		}
	}

	var extra []string
	for partName := range ct.Overrides {
		if !seen[partName] {
			extra = append(extra, partName)
		}
	}
	sort.Strings(extra)
	return append(result, extra...)
}

// extractXMLSeparator returns the exact bytes appearing between the XML
// declaration ("?>") and the root element ("<") in data. This is usually
// "\r\n" but some producers emit a bare "\n"; capturing it lets Marshal
// reproduce the original prolog byte-for-byte. Returns "" when no separator is
// found.
func extractXMLSeparator(data []byte) string {
	end := bytes.Index(data, []byte("?>"))
	if end < 0 {
		return ""
	}
	rest := data[end+2:]
	lt := bytes.IndexByte(rest, '<')
	if lt < 0 {
		return ""
	}
	return string(rest[:lt])
}

// UnmarshalContentTypes parses content types XML into a ContentTypes struct.
func UnmarshalContentTypes(data []byte) (*ContentTypes, error) {
	var ctXML contentTypesXML
	if err := xml.Unmarshal(data, &ctXML); err != nil {
		return nil, err
	}

	ct := &ContentTypes{
		Defaults:       make(map[string]string, len(ctXML.Defaults)),
		Overrides:      make(map[string]string, len(ctXML.Overrides)),
		defaultOrder:   make([]string, 0, len(ctXML.Defaults)),
		overrideOrder:  make([]string, 0, len(ctXML.Overrides)),
		origExt:        make(map[string]string, len(ctXML.Defaults)),
		OriginalXMLSep: extractXMLSeparator(data),
	}

	for _, def := range ctXML.Defaults {
		ext := strings.ToLower(def.Extension)
		ct.Defaults[ext] = def.ContentType
		ct.defaultOrder = append(ct.defaultOrder, ext)
		if _, exists := ct.origExt[ext]; !exists {
			ct.origExt[ext] = def.Extension
		}
	}

	for _, override := range ctXML.Overrides {
		ct.Overrides[override.PartName] = override.ContentType
		ct.overrideOrder = append(ct.overrideOrder, override.PartName)
	}

	return ct, nil
}
