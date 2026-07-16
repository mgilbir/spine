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

	// PowerPoint content types. The main part of a PresentationML package
	// comes in one flavor per file type (ECMA-376 and [MS-OFFDI]): regular
	// presentation (.pptx), slideshow (.ppsx), template (.potx), and the
	// macro-enabled variant of each (.pptm/.ppsm/.potm).
	ContentTypePresentationMain              = "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
	ContentTypeSlideshowMain                 = "application/vnd.openxmlformats-officedocument.presentationml.slideshow.main+xml"
	ContentTypePresentationTemplateMain      = "application/vnd.openxmlformats-officedocument.presentationml.template.main+xml"
	ContentTypePresentationMacroMain         = "application/vnd.ms-powerpoint.presentation.macroEnabled.main+xml"
	ContentTypeSlideshowMacroMain            = "application/vnd.ms-powerpoint.slideshow.macroEnabled.main+xml"
	ContentTypePresentationTemplateMacroMain = "application/vnd.ms-powerpoint.template.macroEnabled.main+xml"
	ContentTypeSlide                         = "application/vnd.openxmlformats-officedocument.presentationml.slide+xml"
	ContentTypeSlideLayout                   = "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"
	ContentTypeSlideMaster                   = "application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"
	ContentTypeTheme                         = "application/vnd.openxmlformats-officedocument.theme+xml"
	ContentTypeThemeOverride                 = "application/vnd.openxmlformats-officedocument.themeOverride+xml"
	ContentTypePresentationProps             = "application/vnd.openxmlformats-officedocument.presentationml.presProps+xml"
	ContentTypeViewProps                     = "application/vnd.openxmlformats-officedocument.presentationml.viewProps+xml"
	ContentTypeTableStyles                   = "application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"

	// Excel content types. SpreadsheetML main-part flavors: regular workbook
	// (.xlsx), template (.xltx), and the macro-enabled workbook (.xlsm),
	// template (.xltm), and add-in (.xlam) variants.
	ContentTypeWorkbook                  = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
	ContentTypeWorkbookTemplateMain      = "application/vnd.openxmlformats-officedocument.spreadsheetml.template.main+xml"
	ContentTypeWorkbookMacroMain         = "application/vnd.ms-excel.sheet.macroEnabled.main+xml"
	ContentTypeWorkbookTemplateMacroMain = "application/vnd.ms-excel.template.macroEnabled.main+xml"
	ContentTypeWorkbookAddinMacroMain    = "application/vnd.ms-excel.addin.macroEnabled.main+xml"
	ContentTypeWorksheet                 = "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"
	ContentTypeSharedStrings             = "application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"
	ContentTypeStyles                    = "application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"

	// Word content types. WordprocessingML main-part flavors: regular
	// document (.docx), template (.dotx), and the macro-enabled document
	// (.docm) and template (.dotm) variants. The .dotm spelling
	// ("macroEnabledTemplate") is [MS-OFFDI]'s, not a typo.
	ContentTypeDocument                  = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
	ContentTypeDocumentTemplateMain      = "application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml"
	ContentTypeDocumentMacroMain         = "application/vnd.ms-word.document.macroEnabled.main+xml"
	ContentTypeDocumentTemplateMacroMain = "application/vnd.ms-word.template.macroEnabledTemplate.main+xml"
	ContentTypeDocStyles                 = "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"
	ContentTypeNumbering                 = "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"
	ContentTypeDocSettings               = "application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"
	ContentTypeDocFontTable              = "application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"
	ContentTypeDocFootnotes              = "application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"
	ContentTypeDocEndnotes               = "application/vnd.openxmlformats-officedocument.wordprocessingml.endnotes+xml"
	ContentTypeDocComments               = "application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"
	ContentTypeDocHeader                 = "application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"
	ContentTypeDocFooter                 = "application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"
	ContentTypeDocWebSettings            = "application/vnd.openxmlformats-officedocument.wordprocessingml.webSettings+xml"

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

	// selfCloseSpace records that the source wrote self-closing tags as
	// " />" (System.IO.Packaging style) rather than "/>", so Marshal can
	// reproduce the entries byte-for-byte. It seeds entries added after
	// parse; parsed entries carry their own per-entry style (sources mix
	// both forms in one file).
	selfCloseSpace bool

	// rootTag is the verbatim <Types ...> open tag from the source,
	// preserving namespace declarations beyond the default one (e.g.
	// xmlns:xsd/xmlns:xsi). Empty for a ContentTypes built from scratch.
	rootTag string

	// closeSep is the raw whitespace between the last entry and </Types>
	// (pretty-printing producers put a newline there); captured from source
	// bytes so CRLF survives the decoder's EOL normalization.
	closeSep string
}

// ctEntry is one <Default> or <Override> element in source document order.
type ctEntry struct {
	isDefault bool
	// key is the lowercase extension (defaults) or the part name (overrides).
	key string
	// contentTypeFirst records that the source wrote the ContentType
	// attribute before Extension/PartName.
	contentTypeFirst bool
	// sep is the raw whitespace preceding this entry in the source
	// (pretty-printing producers indent each entry).
	sep string
	// raw is the verbatim source start tag; replayed while the entry's
	// content type is unchanged, preserving lexical forms the regenerated
	// tag cannot (spaces around '=', expanded </Default> close).
	raw string
	// expanded records that the source wrote an open/close pair instead of
	// a self-closing tag.
	expanded bool
	// origCT is the content type at parse time, to detect edits.
	origCT string
	// space records this entry's self-closing style (" />" vs "/>");
	// producers mix both forms within one file.
	space bool
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
		selfCloseSpace: ct.selfCloseSpace,
		rootTag:        ct.rootTag,
		closeSep:       ct.closeSep,
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
	if ct.rootTag != "" {
		// Verbatim source root tag: preserves declarations beyond the default
		// namespace (xmlns:xsd, xmlns:xsi) and their order.
		buf.WriteString(ct.rootTag)
	} else {
		buf.WriteString(`<Types xmlns="`)
		buf.WriteString(ContentTypesNamespace)
		buf.WriteString(`">`)
	}

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
				buf.WriteString(e.sep)
				if e.raw != "" && contentType == e.origCT {
					// Verbatim replay of the unchanged source entry.
					buf.WriteString(e.raw)
					if e.expanded {
						buf.WriteString("</Default>")
					}
					continue
				}
				ct.writeDefault(&buf, e.key, contentType, e.contentTypeFirst, e.space)
			}
		} else {
			if seenOverride[e.key] {
				continue
			}
			if contentType, ok := ct.Overrides[e.key]; ok {
				seenOverride[e.key] = true
				buf.WriteString(e.sep)
				if e.raw != "" && contentType == e.origCT {
					// Verbatim replay of the unchanged source entry.
					buf.WriteString(e.raw)
					if e.expanded {
						buf.WriteString("</Override>")
					}
					continue
				}
				ct.writeOverride(&buf, e.key, contentType, e.contentTypeFirst, e.space)
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
			ct.writeDefault(&buf, ext, contentType, false, ct.selfCloseSpace)
		}
	}

	// Then remaining overrides.
	for _, partName := range ct.orderedOverrides() {
		if seenOverride[partName] {
			continue
		}
		if contentType, ok := ct.Overrides[partName]; ok {
			seenOverride[partName] = true
			ct.writeOverride(&buf, partName, contentType, false, ct.selfCloseSpace)
		}
	}

	buf.WriteString(ct.closeSep)
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
func (ct *ContentTypes) writeDefault(buf *bytes.Buffer, ext, contentType string, contentTypeFirst, space bool) {
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
	buf.WriteString(selfCloseEnd(space))
}

// selfCloseEnd returns the self-closing tag terminator for one entry: `" />"`
// (System.IO.Packaging style) or `"/>"`.
func selfCloseEnd(space bool) string {
	if space {
		return `" />`
	}
	return `"/>`
}

// writeOverride writes one <Override> entry (see writeDefault).
func (ct *ContentTypes) writeOverride(buf *bytes.Buffer, partName, contentType string, contentTypeFirst, space bool) {
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
	buf.WriteString(selfCloseEnd(space))
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
		selfCloseSpace: xmlb.DetectSelfClosingSpace(data),
	}

	// The parse tracks decoder byte offsets so raw source spans survive: the
	// verbatim <Types ...> open tag, each entry's leading whitespace and
	// self-closing style, and the whitespace before </Types>. CharData values
	// cannot serve here — the decoder EOL-normalizes them, losing CRLF.
	d := xmlb.NewDecoder(bytes.NewReader(data))
	sawTypes := false
	last := int64(0)
	pendingSep := ""
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		cur := d.InputOffset()
		raw := ""
		if last >= 0 && cur >= last && cur <= int64(len(data)) {
			raw = string(data[last:cur])
		}
		last = cur

		if cd, ok := tok.(xml.CharData); ok {
			if len(bytes.TrimSpace(cd)) == 0 {
				pendingSep = raw
			} else {
				pendingSep = ""
			}
			continue
		}
		if ee, ok := tok.(xml.EndElement); ok {
			if ee.Name.Local == "Types" {
				ct.closeSep = pendingSep
			}
			pendingSep = ""
			continue
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			pendingSep = ""
			continue
		}
		sep := pendingSep
		pendingSep = ""
		// raw holds this element's verbatim tag text; a self-closing tag ends
		// in "/>" and its style is " />" when a space precedes the slash.
		space := strings.HasSuffix(raw, "/>") && strings.HasSuffix(raw, " />")

		switch se.Name.Local {
		case "Types":
			sawTypes = true
			ct.rootTag = strings.TrimLeft(raw, " \t\r\n")
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
			ct.entryOrder = append(ct.entryOrder, ctEntry{isDefault: true, key: key, contentTypeFirst: ctFirst, sep: sep, space: space,
				raw: raw, expanded: !strings.HasSuffix(raw, "/>"), origCT: contentType})
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
			ct.entryOrder = append(ct.entryOrder, ctEntry{isDefault: false, key: partName, contentTypeFirst: ctFirst, sep: sep, space: space,
				raw: raw, expanded: !strings.HasSuffix(raw, "/>"), origCT: contentType})
		}
	}
	if !sawTypes {
		return nil, ErrCorruptedPackage
	}

	return ct, nil
}
