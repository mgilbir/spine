package opc

import (
	"bytes"
	"encoding/xml"
	"io"
	"maps"
	"path"
	"slices"
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
	ContentTypePresentationMain  = "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
	ContentTypeSlide             = "application/vnd.openxmlformats-officedocument.presentationml.slide+xml"
	ContentTypeSlideLayout       = "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"
	ContentTypeSlideMaster       = "application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"
	ContentTypeTheme             = "application/vnd.openxmlformats-officedocument.theme+xml"
	ContentTypeThemeOverride     = "application/vnd.openxmlformats-officedocument.themeOverride+xml"
	ContentTypePresentationProps = "application/vnd.openxmlformats-officedocument.presentationml.presProps+xml"
	ContentTypeViewProps         = "application/vnd.openxmlformats-officedocument.presentationml.viewProps+xml"
	ContentTypeTableStyles       = "application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"

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

	// prolog is the full prolog capture from the parsed file (declaration
	// presence and form, separator, trailer). When captured it wins over
	// OriginalXMLSep, so a source without an XML declaration does not gain one.
	prolog xmlb.Prolog

	// entryOrder preserves the document order of every <Default> and
	// <Override> entry in the parsed file, including their interleaving and
	// each entry's attribute order. Entries added after parse are appended by
	// Marshal (defaults first, then overrides, each sorted).
	entryOrder []ctEntry
}

// ctEntry is one <Default> or <Override> element in source document order.
type ctEntry struct {
	isDefault bool
	// key is the lowercase extension (defaults) or the part name (overrides).
	key string
	// contentTypeFirst records that the source wrote the ContentType
	// attribute before Extension/PartName.
	contentTypeFirst bool
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

// Clone returns a deep copy of ct. Writers mutate their ContentTypes while
// finalizing a package (SetOverride for regenerated metadata parts), so a
// consumer that hands a reader's ContentTypes to successive writers must pass
// a clone to keep repeated saves independent and race-free.
func (ct *ContentTypes) Clone() *ContentTypes {
	if ct == nil {
		return nil
	}
	c := &ContentTypes{
		Defaults:       maps.Clone(ct.Defaults),
		Overrides:      maps.Clone(ct.Overrides),
		defaultOrder:   slices.Clone(ct.defaultOrder),
		overrideOrder:  slices.Clone(ct.overrideOrder),
		origExt:        maps.Clone(ct.origExt),
		OriginalXMLSep: ct.OriginalXMLSep,
		prolog:         ct.prolog,
		entryOrder:     slices.Clone(ct.entryOrder),
	}
	// maps.Clone(nil) is nil; keep the invariant that the maps are usable.
	if c.Defaults == nil {
		c.Defaults = make(map[string]string)
	}
	if c.Overrides == nil {
		c.Overrides = make(map[string]string)
	}
	if c.origExt == nil {
		c.origExt = make(map[string]string)
	}
	return c
}

// GetContentType returns the content type for a part.
// It first checks overrides, then defaults by extension.
func (ct *ContentTypes) GetContentType(partName string) string {
	// Check overrides first (exact, then case-insensitive: OPC part names
	// compare case-insensitively, so a differently-cased part must still match
	// its override rather than falling through to a default extension type).
	if contentType, ok := ct.Overrides[partName]; ok {
		return contentType
	}
	// The case-insensitive fallback walks the ordered override list rather
	// than ranging over the map: with case-variant duplicate overrides, map
	// iteration would return a different winner per call. First declared wins,
	// stably.
	for _, name := range ct.overrideOrder {
		if strings.EqualFold(name, partName) {
			if contentType, ok := ct.Overrides[name]; ok {
				return contentType
			}
		}
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

// RemoveOverride removes the content type override for a specific part, so a
// part dropped from a package does not leave a dangling Override entry. Part
// names match case-insensitively, mirroring GetContentType.
func (ct *ContentTypes) RemoveOverride(partName string) {
	name := partName
	if _, ok := ct.Overrides[name]; !ok {
		name = ""
		for existing := range ct.Overrides {
			if strings.EqualFold(existing, partName) {
				name = existing
				break
			}
		}
		if name == "" {
			return
		}
	}
	delete(ct.Overrides, name)
	for i, entry := range ct.overrideOrder {
		if entry == name {
			ct.overrideOrder = append(ct.overrideOrder[:i], ct.overrideOrder[i+1:]...)
			break
		}
	}
}

// ContentTypesNamespace is the XML namespace for content types.
const ContentTypesNamespace = "http://schemas.openxmlformats.org/package/2006/content-types"

// Marshal converts ContentTypes to XML bytes.
// Output format matches Microsoft Office: compact single-line with
// self-closing elements. A ContentTypes parsed from a source file reproduces
// the source's prolog, the document order of its Default/Override entries
// (including their interleaving), and each entry's attribute order; entries
// added after parse are appended (defaults first, then overrides, sorted).
func (ct *ContentTypes) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	if ct.prolog.Captured {
		buf.WriteString(ct.prolog.Decl)
		buf.WriteString(ct.prolog.Sep)
	} else {
		buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
		sep := ct.OriginalXMLSep
		if sep == "" {
			sep = "\r\n"
		}
		buf.WriteString(sep)
	}
	buf.WriteString(`<Types xmlns="`)
	buf.WriteString(ContentTypesNamespace)
	buf.WriteString(`">`)

	// Replay the source entries in document order, skipping entries that were
	// removed since parse and duplicates (OPC requires unique entries).
	seenDefault := make(map[string]bool, len(ct.Defaults))
	seenOverride := make(map[string]bool, len(ct.Overrides))
	for _, e := range ct.entryOrder {
		if e.isDefault {
			if seenDefault[e.key] {
				continue
			}
			if contentType, ok := ct.Defaults[e.key]; ok {
				seenDefault[e.key] = true
				ct.writeDefault(&buf, e.key, contentType, e.contentTypeFirst)
			}
		} else {
			if seenOverride[e.key] {
				continue
			}
			if contentType, ok := ct.Overrides[e.key]; ok {
				seenOverride[e.key] = true
				ct.writeOverride(&buf, e.key, contentType, e.contentTypeFirst)
			}
		}
	}

	// Write remaining defaults: original two-bucket order for a ContentTypes
	// built from scratch, plus entries registered after parse.
	for _, ext := range ct.orderedDefaults() {
		if seenDefault[ext] {
			continue
		}
		if contentType, ok := ct.Defaults[ext]; ok {
			seenDefault[ext] = true
			ct.writeDefault(&buf, ext, contentType, false)
		}
	}

	// Then remaining overrides.
	for _, partName := range ct.orderedOverrides() {
		if seenOverride[partName] {
			continue
		}
		if contentType, ok := ct.Overrides[partName]; ok {
			seenOverride[partName] = true
			ct.writeOverride(&buf, partName, contentType, false)
		}
	}

	buf.WriteString("</Types>")
	if ct.prolog.Captured {
		buf.WriteString(ct.prolog.Trailer)
	}
	return buf.Bytes(), nil
}

// writeDefault writes one <Default> entry. The extension is re-emitted in its
// original case (OPC matches case-insensitively, but round-trip must preserve
// the source spelling), and contentTypeFirst reproduces the source's
// attribute order.
func (ct *ContentTypes) writeDefault(buf *bytes.Buffer, ext, contentType string, contentTypeFirst bool) {
	buf.WriteString(`<Default `)
	if contentTypeFirst {
		buf.WriteString(`ContentType="`)
		buf.WriteString(xmlb.EscapeAttrValue(contentType))
		buf.WriteString(`" Extension="`)
		buf.WriteString(xmlb.EscapeAttrValue(ct.displayExtension(ext)))
	} else {
		buf.WriteString(`Extension="`)
		buf.WriteString(xmlb.EscapeAttrValue(ct.displayExtension(ext)))
		buf.WriteString(`" ContentType="`)
		buf.WriteString(xmlb.EscapeAttrValue(contentType))
	}
	buf.WriteString(`"/>`)
}

// writeOverride writes one <Override> entry (see writeDefault).
func (ct *ContentTypes) writeOverride(buf *bytes.Buffer, partName, contentType string, contentTypeFirst bool) {
	buf.WriteString(`<Override `)
	if contentTypeFirst {
		buf.WriteString(`ContentType="`)
		buf.WriteString(xmlb.EscapeAttrValue(contentType))
		buf.WriteString(`" PartName="`)
		buf.WriteString(xmlb.EscapeAttrValue(partName))
	} else {
		buf.WriteString(`PartName="`)
		buf.WriteString(xmlb.EscapeAttrValue(partName))
		buf.WriteString(`" ContentType="`)
		buf.WriteString(xmlb.EscapeAttrValue(contentType))
	}
	buf.WriteString(`"/>`)
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

	// Start with original order, skipping duplicates so a source file with a
	// repeated <Default> does not re-emit it (OPC requires unique extensions).
	seen := make(map[string]bool, len(ct.defaultOrder))
	result := make([]string, 0, len(ct.Defaults))
	for _, ext := range ct.defaultOrder {
		if seen[ext] {
			continue
		}
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
		if seen[partName] {
			continue
		}
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
// The parse walks the document token by token so the interleaved order of
// Default and Override entries — and each entry's attribute order — is
// captured for byte-faithful regeneration.
func UnmarshalContentTypes(data []byte) (*ContentTypes, error) {
	ct := &ContentTypes{
		Defaults:       make(map[string]string),
		Overrides:      make(map[string]string),
		origExt:        make(map[string]string),
		OriginalXMLSep: extractXMLSeparator(data),
		prolog:         xmlb.CaptureProlog(data),
	}

	d := xml.NewDecoder(bytes.NewReader(data))
	sawTypes := false
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "Types":
			sawTypes = true
		case "Default":
			var ext, contentType string
			ctFirst := false
			for i, attr := range se.Attr {
				switch attr.Name.Local {
				case "Extension":
					ext = attr.Value
				case "ContentType":
					contentType = attr.Value
					ctFirst = i == 0
				}
			}
			key := strings.ToLower(ext)
			if _, exists := ct.Defaults[key]; !exists {
				ct.defaultOrder = append(ct.defaultOrder, key)
			}
			ct.Defaults[key] = contentType
			if _, exists := ct.origExt[key]; !exists {
				ct.origExt[key] = ext
			}
			ct.entryOrder = append(ct.entryOrder, ctEntry{isDefault: true, key: key, contentTypeFirst: ctFirst})
		case "Override":
			var partName, contentType string
			ctFirst := false
			for i, attr := range se.Attr {
				switch attr.Name.Local {
				case "PartName":
					partName = attr.Value
				case "ContentType":
					contentType = attr.Value
					ctFirst = i == 0
				}
			}
			if _, exists := ct.Overrides[partName]; !exists {
				ct.overrideOrder = append(ct.overrideOrder, partName)
			}
			ct.Overrides[partName] = contentType
			ct.entryOrder = append(ct.entryOrder, ctEntry{isDefault: false, key: partName, contentTypeFirst: ctFirst})
		}
	}
	if !sawTypes {
		return nil, ErrCorruptedPackage
	}

	return ct, nil
}
