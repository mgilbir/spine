package opc

import (
	"encoding/xml"
	"io"
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

	// elementOrder preserves the original order of child elements for round-trip fidelity.
	// Each entry is the element key used by marshalElement (e.g., "dc:title", "cp:keywords").
	elementOrder []string

	// presentFields tracks which fields had XML elements during unmarshaling,
	// even if their values were empty. Used by Marshal to preserve empty elements.
	presentFields map[string]bool

	// rawDates preserves the exact lexical form of each date element as it
	// appeared in the source (keyed by element key, e.g. "dcterms:created").
	// W3CDTF permits reduced-precision forms ("2024", "2024-01-15") and
	// non-UTC offsets that are not valid RFC3339; preserving the original text
	// lets Marshal re-emit it verbatim when the value has not been reassigned,
	// avoiding both data loss and gratuitous reformatting.
	rawDates map[string]string
}

// w3cdtfLayouts lists the date layouts accepted for W3CDTF core-property dates,
// from most to least precise. W3CDTF (ISO 8601 profile) allows truncating to
// any of these; only the first is valid RFC3339.
var w3cdtfLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
	"2006-01",
	"2006",
}

// parseW3CDTF parses a W3CDTF date string, trying each accepted layout.
func parseW3CDTF(s string) (time.Time, bool) {
	for _, layout := range w3cdtfLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// dateContent returns the text to write for a date element, or "" to omit it.
// When the source preserved a raw lexical form and t still equals it, the raw
// text is returned verbatim (preserving reduced precision, offset, and
// sub-second components); otherwise t is formatted as RFC3339 UTC.
func (cp *CoreProperties) dateContent(key string, t time.Time) string {
	if raw, ok := cp.rawDates[key]; ok {
		if parsed, pok := parseW3CDTF(raw); pok {
			if parsed.Equal(t) {
				return raw
			}
		} else if t.IsZero() {
			// Original was present but unparseable and never reassigned:
			// preserve it verbatim rather than dropping it.
			return raw
		}
	}
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
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

// marshalCoreElement writes a single core properties element to the builder.
// It writes the element if the value is non-empty, or if the field was present
// in the original XML (even with an empty value).
func (cp *CoreProperties) marshalCoreElement(b *strings.Builder, key string) {
	switch key {
	case "dc:title":
		if cp.Title != "" || cp.presentFields["dc:title"] {
			b.WriteString("<dc:title>" + xmlEscape(cp.Title) + "</dc:title>")
		}
	case "dc:subject":
		if cp.Subject != "" || cp.presentFields["dc:subject"] {
			b.WriteString("<dc:subject>" + xmlEscape(cp.Subject) + "</dc:subject>")
		}
	case "dc:creator":
		if cp.Creator != "" || cp.presentFields["dc:creator"] {
			b.WriteString("<dc:creator>" + xmlEscape(cp.Creator) + "</dc:creator>")
		}
	case "dc:description":
		if cp.Description != "" || cp.presentFields["dc:description"] {
			b.WriteString("<dc:description>" + xmlEscape(cp.Description) + "</dc:description>")
		}
	case "dc:identifier":
		if cp.Identifier != "" || cp.presentFields["dc:identifier"] {
			b.WriteString("<dc:identifier>" + xmlEscape(cp.Identifier) + "</dc:identifier>")
		}
	case "dc:language":
		if cp.Language != "" || cp.presentFields["dc:language"] {
			b.WriteString("<dc:language>" + xmlEscape(cp.Language) + "</dc:language>")
		}
	case "cp:keywords":
		if cp.Keywords != "" || cp.presentFields["cp:keywords"] {
			b.WriteString("<cp:keywords>" + xmlEscape(cp.Keywords) + "</cp:keywords>")
		}
	case "cp:lastModifiedBy":
		if cp.LastModifiedBy != "" || cp.presentFields["cp:lastModifiedBy"] {
			b.WriteString("<cp:lastModifiedBy>" + xmlEscape(cp.LastModifiedBy) + "</cp:lastModifiedBy>")
		}
	case "cp:revision":
		if cp.Revision != "" || cp.presentFields["cp:revision"] {
			b.WriteString("<cp:revision>" + xmlEscape(cp.Revision) + "</cp:revision>")
		}
	case "cp:category":
		if cp.Category != "" || cp.presentFields["cp:category"] {
			b.WriteString("<cp:category>" + xmlEscape(cp.Category) + "</cp:category>")
		}
	case "cp:contentStatus":
		if cp.ContentStatus != "" || cp.presentFields["cp:contentStatus"] {
			b.WriteString("<cp:contentStatus>" + xmlEscape(cp.ContentStatus) + "</cp:contentStatus>")
		}
	case "cp:version":
		if cp.Version != "" || cp.presentFields["cp:version"] {
			b.WriteString("<cp:version>" + xmlEscape(cp.Version) + "</cp:version>")
		}
	case "dcterms:created":
		if s := cp.dateContent(key, cp.Created); s != "" {
			b.WriteString(`<dcterms:created xsi:type="dcterms:W3CDTF">`)
			b.WriteString(s)
			b.WriteString("</dcterms:created>")
		}
	case "dcterms:modified":
		if s := cp.dateContent(key, cp.Modified); s != "" {
			b.WriteString(`<dcterms:modified xsi:type="dcterms:W3CDTF">`)
			b.WriteString(s)
			b.WriteString("</dcterms:modified>")
		}
	case "cp:lastPrinted":
		if s := cp.dateContent(key, cp.LastPrinted); s != "" {
			b.WriteString(`<cp:lastPrinted>`)
			b.WriteString(s)
			b.WriteString("</cp:lastPrinted>")
		}
	}
}

// defaultElementOrder is the element order used when creating new CoreProperties
// (not loaded from an existing file). Matches typical Microsoft Office output.
var defaultElementOrder = []string{
	"dc:title", "dc:subject", "dc:creator", "cp:keywords", "dc:description",
	"cp:lastModifiedBy", "cp:revision",
	"dcterms:created", "dcterms:modified", "cp:lastPrinted",
	"cp:category", "cp:contentStatus", "dc:identifier", "dc:language", "cp:version",
}

// Marshal converts CoreProperties to XML bytes with proper OOXML namespaces.
func (cp *CoreProperties) Marshal() ([]byte, error) {
	var b strings.Builder

	// Compact format matching Microsoft Office output style (no line breaks between elements)
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\r\n")
	b.WriteString(`<cp:coreProperties`)
	b.WriteString(` xmlns:cp="` + nsCoreProperties + `"`)
	b.WriteString(` xmlns:dc="` + nsDublinCore + `"`)
	b.WriteString(` xmlns:dcterms="` + nsDcTerms + `"`)
	b.WriteString(` xmlns:dcmitype="` + nsDcmiType + `"`)
	b.WriteString(` xmlns:xsi="` + nsXsi + `"`)
	b.WriteString(">")

	order := cp.elementOrder
	if len(order) == 0 {
		order = defaultElementOrder
	}

	// Write elements in the preserved (or default) order
	written := make(map[string]bool, len(order))
	for _, key := range order {
		cp.marshalCoreElement(&b, key)
		written[key] = true
	}

	// Write any remaining elements not in the original order
	// (e.g., fields set programmatically after loading)
	for _, key := range defaultElementOrder {
		if !written[key] {
			cp.marshalCoreElement(&b, key)
		}
	}

	b.WriteString("</cp:coreProperties>")

	return []byte(b.String()), nil
}

// xmlEscape escapes the characters that must be escaped in XML character data:
// &, <, and > (the last is only strictly required in the sequence "]]>", but
// is escaped unconditionally for safety). Unlike encoding/xml's EscapeText it
// does not escape quotes or apostrophes — those are legal in text and escaping
// them mutates otherwise-unchanged content, breaking byte-identical round-trip
// (e.g. "O'Brien" must not become "O&apos;Brien").
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
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ExtendedProperties contains the extended document properties.
type ExtendedProperties struct {
	// Application that created the document
	Application string

	// Application version
	AppVersion string

	// Total editing time in minutes
	TotalTime int

	// Word count
	Words int

	// Paragraph count
	Paragraphs int

	// Slide count (for presentations)
	Slides int

	// Notes count
	Notes int

	// Hidden slides count
	HiddenSlides int

	// Multimedia clips count
	MMClips int

	// Presentation format (e.g., "Widescreen")
	PresentationFormat string

	// Whether to scale images when cropping
	ScaleCrop bool

	// Whether links are up to date
	LinksUpToDate bool

	// Whether this is a shared document
	SharedDoc bool

	// Whether hyperlinks have changed
	HyperlinksChanged bool
}

const (
	nsExtendedProperties = "http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"
	nsDocPropsVTypes     = "http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes"
)

// Marshal converts ExtendedProperties to XML bytes.
func (ep *ExtendedProperties) Marshal() ([]byte, error) {
	var b strings.Builder

	b.WriteString(xml.Header)
	b.WriteString(`<Properties xmlns="` + nsExtendedProperties + `"`)
	b.WriteString(` xmlns:vt="` + nsDocPropsVTypes + `"`)
	b.WriteString(">\n")

	// Add properties in the expected order
	b.WriteString("  <TotalTime>0</TotalTime>\n")
	b.WriteString("  <Words>0</Words>\n")

	if ep.Application != "" {
		b.WriteString("  <Application>" + xmlEscape(ep.Application) + "</Application>\n")
	} else {
		b.WriteString("  <Application>Spine Go Library</Application>\n")
	}

	if ep.PresentationFormat != "" {
		b.WriteString("  <PresentationFormat>" + xmlEscape(ep.PresentationFormat) + "</PresentationFormat>\n")
	}

	b.WriteString("  <Paragraphs>0</Paragraphs>\n")

	if ep.Slides > 0 {
		b.WriteString("  <Slides>" + itoa(ep.Slides) + "</Slides>\n")
	} else {
		b.WriteString("  <Slides>0</Slides>\n")
	}

	b.WriteString("  <Notes>0</Notes>\n")
	b.WriteString("  <HiddenSlides>0</HiddenSlides>\n")
	b.WriteString("  <MMClips>0</MMClips>\n")
	b.WriteString("  <ScaleCrop>false</ScaleCrop>\n")
	b.WriteString("  <LinksUpToDate>false</LinksUpToDate>\n")
	b.WriteString("  <SharedDoc>false</SharedDoc>\n")
	b.WriteString("  <HyperlinksChanged>false</HyperlinksChanged>\n")

	if ep.AppVersion != "" {
		b.WriteString("  <AppVersion>" + xmlEscape(ep.AppVersion) + "</AppVersion>\n")
	} else {
		b.WriteString("  <AppVersion>1.0000</AppVersion>\n")
	}

	b.WriteString("</Properties>")

	return []byte(b.String()), nil
}

// itoa converts an integer to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// coreElementKey returns the prefixed element key (e.g. "dc:title", "cp:keywords")
// for a given XML element name, mapping namespace URIs to their standard prefixes.
func coreElementKey(name xml.Name) string {
	switch name.Space {
	case nsDublinCore:
		return "dc:" + name.Local
	case nsDcTerms:
		return "dcterms:" + name.Local
	case nsCoreProperties:
		return "cp:" + name.Local
	default:
		// For non-namespaced elements (legacy format), map to expected prefix
		switch name.Local {
		case "title", "subject", "creator", "description", "identifier", "language":
			return "dc:" + name.Local
		case "created", "modified":
			return "dcterms:" + name.Local
		default:
			return "cp:" + name.Local
		}
	}
}

// UnmarshalCoreProperties parses core properties XML into a CoreProperties struct.
// It preserves element order and tracks which fields are present (even if empty)
// for round-trip fidelity.
func UnmarshalCoreProperties(data []byte) (*CoreProperties, error) {
	cp := &CoreProperties{
		presentFields: make(map[string]bool),
		rawDates:      make(map[string]string),
	}

	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var inRoot bool

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if !inRoot {
				// This is the root <cp:coreProperties> element
				inRoot = true
				continue
			}

			key := coreElementKey(t.Name)
			cp.elementOrder = append(cp.elementOrder, key)
			cp.presentFields[key] = true

			// Read the element content
			switch key {
			case "dcterms:created":
				var d dcDate
				if err := decoder.DecodeElement(&d, &t); err == nil {
					cp.rawDates[key] = d.Value
					if parsed, ok := parseW3CDTF(d.Value); ok {
						cp.Created = parsed
					}
				}
			case "dcterms:modified":
				var d dcDate
				if err := decoder.DecodeElement(&d, &t); err == nil {
					cp.rawDates[key] = d.Value
					if parsed, ok := parseW3CDTF(d.Value); ok {
						cp.Modified = parsed
					}
				}
			case "cp:lastPrinted":
				var d dcDate
				if err := decoder.DecodeElement(&d, &t); err == nil {
					cp.rawDates[key] = d.Value
					if parsed, ok := parseW3CDTF(d.Value); ok {
						cp.LastPrinted = parsed
					}
				}
			default:
				var value string
				if err := decoder.DecodeElement(&value, &t); err == nil {
					switch key {
					case "dc:title":
						cp.Title = value
					case "dc:subject":
						cp.Subject = value
					case "dc:creator":
						cp.Creator = value
					case "dc:description":
						cp.Description = value
					case "dc:identifier":
						cp.Identifier = value
					case "dc:language":
						cp.Language = value
					case "cp:keywords":
						cp.Keywords = value
					case "cp:lastModifiedBy":
						cp.LastModifiedBy = value
					case "cp:revision":
						cp.Revision = value
					case "cp:category":
						cp.Category = value
					case "cp:contentStatus":
						cp.ContentStatus = value
					case "cp:version":
						cp.Version = value
					}
				}
			}
		}
	}

	if !inRoot {
		return nil, &xml.SyntaxError{Msg: "missing coreProperties root element", Line: 1}
	}

	return cp, nil
}
