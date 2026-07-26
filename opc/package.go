package opc

import (
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	xmlb "github.com/mgilbir/spine/common/xml"
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

	// unknownChildren preserves child elements that marshalCoreElement cannot
	// regenerate (vendor extensions, foreign-namespace elements) as verbatim
	// raw XML, keyed by the synthetic "unknown:N" keys recorded in
	// elementOrder. Marshal re-emits them at their original position so
	// regenerating core.xml does not drop them.
	unknownChildren map[string]string
}

// Clone returns a deep copy of cp, including the unexported round-trip
// bookkeeping (element order, raw date forms, unknown children). Mutating the
// clone never affects the original.
func (cp *CoreProperties) Clone() *CoreProperties {
	if cp == nil {
		return nil
	}
	c := *cp
	c.elementOrder = slices.Clone(cp.elementOrder)
	c.presentFields = maps.Clone(cp.presentFields)
	c.rawDates = maps.Clone(cp.rawDates)
	c.unknownChildren = maps.Clone(cp.unknownChildren)
	return &c
}

// Equal reports whether cp and o carry the same user-visible property values.
// It compares only the exported fields — the unexported round-trip bookkeeping
// is ignored — with date fields compared via time.Time.Equal. Consumers use it
// to detect whether properties were edited after a package was opened.
func (cp *CoreProperties) Equal(o *CoreProperties) bool {
	if cp == nil || o == nil {
		return cp == o
	}
	return cp.Category == o.Category &&
		cp.ContentStatus == o.ContentStatus &&
		cp.Created.Equal(o.Created) &&
		cp.Creator == o.Creator &&
		cp.Description == o.Description &&
		cp.Identifier == o.Identifier &&
		cp.Keywords == o.Keywords &&
		cp.Language == o.Language &&
		cp.LastModifiedBy == o.LastModifiedBy &&
		cp.LastPrinted.Equal(o.LastPrinted) &&
		cp.Modified.Equal(o.Modified) &&
		cp.Revision == o.Revision &&
		cp.Subject == o.Subject &&
		cp.Title == o.Title &&
		cp.Version == o.Version
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

// dateContent returns the text to write for a date element and whether the
// element should be written at all. When the source preserved a raw lexical
// form and t still equals it, the raw text is returned verbatim (preserving
// reduced precision, offset, and sub-second components); a present-but-empty
// or unparseable source element that was never reassigned is likewise
// re-emitted verbatim (possibly as an empty element) rather than dropped;
// otherwise t is formatted as RFC3339 UTC, or the element is omitted when t
// is the zero time.
func (cp *CoreProperties) dateContent(key string, t time.Time) (string, bool) {
	if raw, ok := cp.rawDates[key]; ok {
		if parsed, pok := parseW3CDTF(raw); pok {
			if parsed.Equal(t) {
				return raw, true
			}
		} else if t.IsZero() {
			// Original was present but unparseable (or empty) and never
			// reassigned: preserve it verbatim rather than dropping it.
			return raw, true
		}
	}
	if t.IsZero() {
		return "", false
	}
	return t.UTC().Format(time.RFC3339), true
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
		if s, ok := cp.dateContent(key, cp.Created); ok {
			b.WriteString(`<dcterms:created xsi:type="dcterms:W3CDTF">`)
			b.WriteString(s)
			b.WriteString("</dcterms:created>")
		}
	case "dcterms:modified":
		if s, ok := cp.dateContent(key, cp.Modified); ok {
			b.WriteString(`<dcterms:modified xsi:type="dcterms:W3CDTF">`)
			b.WriteString(s)
			b.WriteString("</dcterms:modified>")
		}
	case "cp:lastPrinted":
		if s, ok := cp.dateContent(key, cp.LastPrinted); ok {
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
		if raw, ok := cp.unknownChildren[key]; ok {
			// Unknown child captured at parse time: re-emit verbatim at its
			// original position so vendor extensions survive regeneration.
			b.WriteString(raw)
		} else {
			cp.marshalCoreElement(&b, key)
		}
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

// Marshal converts ExtendedProperties to XML bytes. Every settable field is
// emitted with its actual value; zero-valued counters and false booleans
// produce the same bytes as before they were settable, so packages that never
// touch these fields marshal identically.
func (ep *ExtendedProperties) Marshal() ([]byte, error) {
	var b strings.Builder

	b.WriteString(xml.Header)
	b.WriteString(`<Properties xmlns="` + nsExtendedProperties + `"`)
	b.WriteString(` xmlns:vt="` + nsDocPropsVTypes + `"`)
	b.WriteString(">\n")

	// Add properties in the expected order
	b.WriteString("  <TotalTime>" + itoa(ep.TotalTime) + "</TotalTime>\n")
	b.WriteString("  <Words>" + itoa(ep.Words) + "</Words>\n")

	if ep.Application != "" {
		b.WriteString("  <Application>" + xmlEscape(ep.Application) + "</Application>\n")
	} else {
		b.WriteString("  <Application>Spine Go Library</Application>\n")
	}

	if ep.PresentationFormat != "" {
		b.WriteString("  <PresentationFormat>" + xmlEscape(ep.PresentationFormat) + "</PresentationFormat>\n")
	}

	b.WriteString("  <Paragraphs>" + itoa(ep.Paragraphs) + "</Paragraphs>\n")
	b.WriteString("  <Slides>" + itoa(ep.Slides) + "</Slides>\n")
	b.WriteString("  <Notes>" + itoa(ep.Notes) + "</Notes>\n")
	b.WriteString("  <HiddenSlides>" + itoa(ep.HiddenSlides) + "</HiddenSlides>\n")
	b.WriteString("  <MMClips>" + itoa(ep.MMClips) + "</MMClips>\n")
	b.WriteString("  <ScaleCrop>" + xmlBool(ep.ScaleCrop) + "</ScaleCrop>\n")
	b.WriteString("  <LinksUpToDate>" + xmlBool(ep.LinksUpToDate) + "</LinksUpToDate>\n")
	b.WriteString("  <SharedDoc>" + xmlBool(ep.SharedDoc) + "</SharedDoc>\n")
	b.WriteString("  <HyperlinksChanged>" + xmlBool(ep.HyperlinksChanged) + "</HyperlinksChanged>\n")

	if ep.AppVersion != "" {
		b.WriteString("  <AppVersion>" + xmlEscape(ep.AppVersion) + "</AppVersion>\n")
	} else {
		b.WriteString("  <AppVersion>1.0000</AppVersion>\n")
	}

	b.WriteString("</Properties>")

	return []byte(b.String()), nil
}

// xmlBool renders a bool in the lowercase form OOXML uses for the extended
// properties booleans.
func xmlBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// UnmarshalExtendedProperties parses extended properties (docProps/app.xml)
// into an ExtendedProperties struct. It is deliberately minimal: only the
// fields ExtendedProperties models are populated, and element values that do
// not parse as their expected type are skipped rather than failing the whole
// part (wild files carry empty <Words/> elements and similar). Consumers that
// need byte-level fidelity for app.xml should preserve the raw part; this
// parse only feeds the typed API.
func UnmarshalExtendedProperties(data []byte) (*ExtendedProperties, error) {
	// All values decode as strings first: an empty or malformed counter
	// element must not fail the parse.
	var raw struct {
		Application        string `xml:"Application"`
		AppVersion         string `xml:"AppVersion"`
		TotalTime          string `xml:"TotalTime"`
		Words              string `xml:"Words"`
		Paragraphs         string `xml:"Paragraphs"`
		Slides             string `xml:"Slides"`
		Notes              string `xml:"Notes"`
		HiddenSlides       string `xml:"HiddenSlides"`
		MMClips            string `xml:"MMClips"`
		PresentationFormat string `xml:"PresentationFormat"`
		ScaleCrop          string `xml:"ScaleCrop"`
		LinksUpToDate      string `xml:"LinksUpToDate"`
		SharedDoc          string `xml:"SharedDoc"`
		HyperlinksChanged  string `xml:"HyperlinksChanged"`
	}
	if err := xmlb.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	ep := &ExtendedProperties{
		Application:        raw.Application,
		AppVersion:         raw.AppVersion,
		PresentationFormat: raw.PresentationFormat,
	}
	setInt := func(dst *int, s string) {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			*dst = n
		}
	}
	setBool := func(dst *bool, s string) {
		if v, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
			*dst = v
		}
	}
	setInt(&ep.TotalTime, raw.TotalTime)
	setInt(&ep.Words, raw.Words)
	setInt(&ep.Paragraphs, raw.Paragraphs)
	setInt(&ep.Slides, raw.Slides)
	setInt(&ep.Notes, raw.Notes)
	setInt(&ep.HiddenSlides, raw.HiddenSlides)
	setInt(&ep.MMClips, raw.MMClips)
	setBool(&ep.ScaleCrop, raw.ScaleCrop)
	setBool(&ep.LinksUpToDate, raw.LinksUpToDate)
	setBool(&ep.SharedDoc, raw.SharedDoc)
	setBool(&ep.HyperlinksChanged, raw.HyperlinksChanged)
	return ep, nil
}

// itoa converts an integer to its decimal string representation. It delegates
// to strconv.Itoa, which correctly handles math.MinInt; a previous hand-rolled
// implementation infinitely recursed on MinInt because -MinInt overflows back
// to MinInt.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// knownCoreKeys lists the element keys marshalCoreElement can regenerate from
// the typed fields; any other child of cp:coreProperties is captured verbatim
// as an unknown child and re-emitted as-is.
var knownCoreKeys = map[string]bool{
	"dc:title": true, "dc:subject": true, "dc:creator": true,
	"dc:description": true, "dc:identifier": true, "dc:language": true,
	"cp:keywords": true, "cp:lastModifiedBy": true, "cp:revision": true,
	"cp:category": true, "cp:contentStatus": true, "cp:version": true,
	"dcterms:created": true, "dcterms:modified": true, "cp:lastPrinted": true,
}

// corePropsRootPrefixes maps the namespace prefixes that Marshal always
// declares on the regenerated <cp:coreProperties> root to their URIs. An
// unknown child that relies on one of these declarations round-trips
// verbatim; any other prefix must carry its own declaration inline.
var corePropsRootPrefixes = map[string]string{
	"cp":       nsCoreProperties,
	"dc":       nsDublinCore,
	"dcterms":  nsDcTerms,
	"dcmitype": nsDcmiType,
	"xsi":      nsXsi,
}

// ensureSelfContainedNS returns raw — a verbatim-captured child element of
// cp:coreProperties — with a namespace declaration for its own prefix injected
// into the start tag when the source declared that prefix on an ancestor
// (typically the root). The regenerated root only declares the standard
// prefixes, so without the injection the re-emitted element would reference an
// undeclared prefix and the output would not be namespace-well-formed.
func ensureSelfContainedNS(raw, space string) string {
	if len(raw) < 2 || raw[0] != '<' {
		return raw
	}
	// Locate the end of the element name in the start tag.
	i := 1
	for i < len(raw) {
		c := raw[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '/' || c == '>' {
			break
		}
		i++
	}
	name := raw[1:i]
	prefix := ""
	if c := strings.IndexByte(name, ':'); c >= 0 {
		prefix = name[:c]
	}
	if prefix == "" {
		if space == "" || strings.Contains(raw, `xmlns=`) {
			return raw
		}
		return raw[:i] + ` xmlns="` + xmlb.EscapeAttrValue(space) + `"` + raw[i:]
	}
	if corePropsRootPrefixes[prefix] == space || strings.Contains(raw, "xmlns:"+prefix+"=") {
		return raw
	}
	return raw[:i] + " xmlns:" + prefix + `="` + xmlb.EscapeAttrValue(space) + `"` + raw[i:]
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
		if name.Space != "" {
			// Unknown namespace: never map onto the standard dc/dcterms/cp
			// fields — a foreign-namespace <evil:creator> must not be captured
			// as, and re-emitted as, a genuine <dc:creator>. The braced key
			// cannot collide with any "prefix:local" key, so the element
			// populates no field; UnmarshalCoreProperties preserves it
			// verbatim as an unknown child instead.
			return "{" + name.Space + "}" + name.Local
		}
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
		presentFields:   make(map[string]bool),
		rawDates:        make(map[string]string),
		unknownChildren: make(map[string]string),
	}

	src := string(data)
	decoder := xmlb.NewDecoder(strings.NewReader(src))
	var inRoot bool

	for {
		// Offset of the upcoming token; for a StartElement this is the byte
		// position of its '<' in src (any preceding character data is a
		// separate token), which lets unknown children be captured verbatim.
		tokStart := decoder.InputOffset()

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
			if !knownCoreKeys[key] {
				// Unknown child (vendor extension or foreign-namespace
				// element): preserve the raw bytes so regenerating core.xml
				// re-emits it verbatim instead of dropping it.
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
				raw := ensureSelfContainedNS(src[tokStart:decoder.InputOffset()], t.Name.Space)
				ukey := "unknown:" + itoa(len(cp.unknownChildren))
				cp.elementOrder = append(cp.elementOrder, ukey)
				cp.unknownChildren[ukey] = raw
				continue
			}
			cp.elementOrder = append(cp.elementOrder, key)
			cp.presentFields[key] = true

			// Read the element content. A DecodeElement failure is a real
			// problem for a small file like core.xml: silently swallowing it
			// would leave the field unset yet still record the key in
			// elementOrder/presentFields, so a later regenerate would emit a
			// present-but-empty element with no diagnostic. Surface the error
			// (identifying the property) instead. Callers such as
			// Reader.parseCoreProperties already treat this as best-effort and
			// skip the properties part on error.
			switch key {
			case "dcterms:created":
				var d dcDate
				if err := decoder.DecodeElement(&d, &t); err != nil {
					return nil, fmt.Errorf("opc: decoding core property %s: %w", key, err)
				}
				cp.rawDates[key] = d.Value
				if parsed, ok := parseW3CDTF(d.Value); ok {
					cp.Created = parsed
				}
			case "dcterms:modified":
				var d dcDate
				if err := decoder.DecodeElement(&d, &t); err != nil {
					return nil, fmt.Errorf("opc: decoding core property %s: %w", key, err)
				}
				cp.rawDates[key] = d.Value
				if parsed, ok := parseW3CDTF(d.Value); ok {
					cp.Modified = parsed
				}
			case "cp:lastPrinted":
				var d dcDate
				if err := decoder.DecodeElement(&d, &t); err != nil {
					return nil, fmt.Errorf("opc: decoding core property %s: %w", key, err)
				}
				cp.rawDates[key] = d.Value
				if parsed, ok := parseW3CDTF(d.Value); ok {
					cp.LastPrinted = parsed
				}
			default:
				var value string
				if err := decoder.DecodeElement(&value, &t); err != nil {
					return nil, fmt.Errorf("opc: decoding core property %s: %w", key, err)
				}
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

	if !inRoot {
		return nil, &xml.SyntaxError{Msg: "missing coreProperties root element", Line: 1}
	}

	return cp, nil
}
