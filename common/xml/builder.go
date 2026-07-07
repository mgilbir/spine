package xml

import (
	"fmt"
	"strings"
)

// Builder creates XML with proper namespace prefixes for OOXML documents.
// Unlike Go's encoding/xml, this builder uses prefixed namespaces (p:, a:, r:)
// which is required for Microsoft Office compatibility.
type Builder struct {
	buf                strings.Builder
	indent             string
	level              int
	namespaces         map[string]string // URI -> prefix
	declaredNamespaces map[string]bool   // URIs that have been declared (root or inline)
	hasRoot            bool              // true after StartElementWithNS is called
	selfClosingSpace   bool              // true: " />" false: "/>"
	elemSeparator      string            // inserted between sibling elements (e.g., " ")
	trailingWS         bool              // set by WriteRaw when data ends with whitespace
	stack              []string          // open element local names, for balance checking
	err                error             // first structural error encountered
}

// pushElem records an opened element for balance checking.
func (b *Builder) pushElem(localName string) {
	b.stack = append(b.stack, localName)
}

// popElem matches a closing element against the open-element stack, recording
// the first structural error and preventing the indentation level from going
// negative on an unbalanced close.
func (b *Builder) popElem(localName string) {
	if len(b.stack) == 0 {
		if b.err == nil {
			b.err = fmt.Errorf("xml: closing </%s> with no open element", localName)
		}
		return
	}
	top := b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]
	if top != localName && b.err == nil {
		b.err = fmt.Errorf("xml: closing </%s> does not match open <%s>", localName, top)
	}
	if b.level > 0 {
		b.level--
	}
}

// Err returns the first structural error the Builder encountered (an unbalanced
// or mismatched element), or nil.
func (b *Builder) Err() error {
	return b.err
}

// Finish reports any structural error, including elements left unclosed. Call it
// after building to validate that every StartElement was matched by an
// EndElement.
func (b *Builder) Finish() error {
	if b.err != nil {
		return b.err
	}
	if len(b.stack) > 0 {
		return fmt.Errorf("xml: %d unclosed element(s), innermost <%s>", len(b.stack), b.stack[len(b.stack)-1])
	}
	return nil
}

// NewBuilder creates a new XML builder.
func NewBuilder() *Builder {
	return &Builder{
		namespaces:         make(map[string]string),
		declaredNamespaces: make(map[string]bool),
	}
}

// NewPresentationMLBuilder creates a builder pre-configured for PresentationML documents.
func NewPresentationMLBuilder() *Builder {
	b := NewBuilder()
	b.RegisterNamespace(NSPresentationML, PrefixPresentationML)
	b.RegisterNamespace(NSDrawingML, PrefixDrawingML)
	b.RegisterNamespace(NSPresentationRels, PrefixRelationships)
	b.RegisterNamespace(NSDrawingMLChart, PrefixDrawingMLChart)
	b.RegisterNamespace(NSDrawingMLDiagram, PrefixDrawingMLDiagram)
	return b
}

// RegisterNamespace registers a namespace URI with a prefix.
func (b *Builder) RegisterNamespace(uri, prefix string) {
	b.namespaces[uri] = prefix
}

// SetIndent sets the indentation string (e.g., "  " for 2 spaces).
func (b *Builder) SetIndent(indent string) {
	b.indent = indent
}

// String returns the built XML as a string.
func (b *Builder) String() string {
	return b.buf.String()
}

// Bytes returns the built XML as bytes.
func (b *Builder) Bytes() []byte {
	return []byte(b.buf.String())
}

// SetSelfClosingSpace controls whether self-closing elements use " />" (true) or "/>" (false).
func (b *Builder) SetSelfClosingSpace(v bool) { b.selfClosingSpace = v }

// SetElementSeparator sets a string to insert between sibling elements (e.g., " " for spaced format).
func (b *Builder) SetElementSeparator(sep string) { b.elemSeparator = sep }

// WriteRaw writes raw content directly to the output buffer without escaping.
func (b *Builder) WriteRaw(data []byte) {
	b.buf.Write(data)
	if len(data) > 0 {
		last := data[len(data)-1]
		b.trailingWS = last == ' ' || last == '\t' || last == '\n' || last == '\r'
	}
}

// writeSelfClose writes the self-closing tag end ("/>" or " />").
func (b *Builder) writeSelfClose() {
	if b.selfClosingSpace {
		b.buf.WriteString(" />")
	} else {
		b.buf.WriteString("/>")
	}
}

// WriteHeader writes the XML declaration with CRLF line ending for Windows compatibility.
func (b *Builder) WriteHeader() {
	b.buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\r\n")
}

// StartElement starts an element with the given namespace and local name.
// If attrs is provided, they are written as attributes.
func (b *Builder) StartElement(namespace, localName string, attrs ...Attr) {
	b.writeIndent()
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	for _, attr := range attrs {
		b.buf.WriteByte(' ')
		if attr.Namespace != "" {
			b.writeQName(attr.Namespace, attr.Name)
		} else {
			b.buf.WriteString(attr.Name)
		}
		b.buf.WriteString(`="`)
		b.writeAttrEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.pushElem(localName)
	b.level++
}

// StartElementWithNS starts an element and declares namespaces.
// This is typically used for the root element.
func (b *Builder) StartElementWithNS(namespace, localName string, declareNS []NSDecl, attrs ...Attr) {
	b.writeIndent()
	b.hasRoot = true
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	// Write namespace declarations
	for _, ns := range declareNS {
		if ns.Prefix == "" {
			// Default namespace: xmlns="URI"
			b.buf.WriteString(` xmlns="`)
		} else {
			b.buf.WriteString(` xmlns:`)
			b.buf.WriteString(ns.Prefix)
			b.buf.WriteString(`="`)
		}
		b.writeAttrEscaped(ns.URI)
		b.buf.WriteByte('"')
		b.declaredNamespaces[ns.URI] = true
	}

	// Write attributes
	for _, attr := range attrs {
		b.buf.WriteByte(' ')
		if attr.Namespace != "" {
			b.writeQName(attr.Namespace, attr.Name)
		} else {
			b.buf.WriteString(attr.Name)
		}
		b.buf.WriteString(`="`)
		b.writeAttrEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.pushElem(localName)
	b.level++
}

// StartElementWithRootAttrs starts a root element writing all attributes in the
// exact order provided. Each RootAttr is either a namespace declaration or a
// regular attribute, preserving the interleaved ordering from the original XML.
func (b *Builder) StartElementWithRootAttrs(namespace, localName string, rootAttrs []RootAttr, extraAttrs ...Attr) {
	b.writeIndent()
	b.hasRoot = true
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	for _, ra := range rootAttrs {
		if ra.IsNS {
			// Namespace declaration
			if ra.Prefix == "" {
				b.buf.WriteString(` xmlns="`)
			} else {
				b.buf.WriteString(` xmlns:`)
				b.buf.WriteString(ra.Prefix)
				b.buf.WriteString(`="`)
			}
			b.writeAttrEscaped(ra.Value)
			b.buf.WriteByte('"')
			b.declaredNamespaces[ra.Value] = true
			// Also register prefix so writeQName can resolve it for extension attrs.
			if ra.Prefix != "" {
				b.namespaces[ra.Value] = ra.Prefix
			}
		} else {
			// Regular attribute (e.g., mc:Ignorable, conformance)
			b.buf.WriteByte(' ')
			if ra.Prefix != "" {
				b.buf.WriteString(ra.Prefix)
				b.buf.WriteByte(':')
			}
			b.buf.WriteString(ra.LocalName)
			b.buf.WriteString(`="`)
			b.writeAttrEscaped(ra.Value)
			b.buf.WriteByte('"')
		}
	}

	// Write any extra attributes
	for _, attr := range extraAttrs {
		b.buf.WriteByte(' ')
		if attr.Namespace != "" {
			b.writeQName(attr.Namespace, attr.Name)
		} else {
			b.buf.WriteString(attr.Name)
		}
		b.buf.WriteString(`="`)
		b.writeAttrEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.pushElem(localName)
	b.level++
}

// EndElement ends the current element.
func (b *Builder) EndElement(namespace, localName string) {
	b.popElem(localName)
	b.writeIndent()
	b.buf.WriteString("</")
	b.writeQName(namespace, localName)
	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// EmptyElement writes a self-closing element.
func (b *Builder) EmptyElement(namespace, localName string, attrs ...Attr) {
	b.writeIndent()
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	for _, attr := range attrs {
		b.buf.WriteByte(' ')
		if attr.Namespace != "" {
			b.writeQName(attr.Namespace, attr.Name)
		} else {
			b.buf.WriteString(attr.Name)
		}
		b.buf.WriteString(`="`)
		b.writeAttrEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.writeSelfClose()
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// WriteElement writes a complete element with text content.
func (b *Builder) WriteElement(namespace, localName, content string, attrs ...Attr) {
	b.writeIndent()
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	for _, attr := range attrs {
		b.buf.WriteByte(' ')
		if attr.Namespace != "" {
			b.writeQName(attr.Namespace, attr.Name)
		} else {
			b.buf.WriteString(attr.Name)
		}
		b.buf.WriteString(`="`)
		b.writeAttrEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	if content == "" {
		b.writeSelfClose()
	} else {
		b.buf.WriteByte('>')
		b.writeTextEscaped(content)
		b.buf.WriteString("</")
		b.writeQName(namespace, localName)
		b.buf.WriteByte('>')
	}

	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// declareNamespaceIfNeeded checks if a namespace needs an inline xmlns declaration
// and marks it as declared. Returns true if a declaration was needed.
func (b *Builder) declareNamespaceIfNeeded(namespace string) bool {
	if namespace == "" || b.declaredNamespaces[namespace] {
		return false
	}
	if _, ok := b.namespaces[namespace]; ok {
		b.declaredNamespaces[namespace] = true
		return true
	}
	return false
}

// prependNamespaceDecls checks if the element namespace or any attribute
// namespaces need inline declarations and prepends xmlns attributes.
// This is used by the reflection marshaler to add declarations for
// namespaces that weren't declared at the root element.
func (b *Builder) prependNamespaceDecls(elemNS string, attrs []Attr) []Attr {
	// Only add inline declarations when a root element with namespace
	// declarations has been written. Without a root, all registered
	// namespaces are assumed to be externally declared.
	if !b.hasRoot {
		return attrs
	}

	var decls []Attr

	// Check element namespace
	if b.declareNamespaceIfNeeded(elemNS) {
		if prefix, ok := b.namespaces[elemNS]; ok {
			if prefix == "" {
				decls = append(decls, Attr{Name: "xmlns", Value: elemNS})
			} else {
				decls = append(decls, Attr{Name: "xmlns:" + prefix, Value: elemNS})
			}
		}
	}

	// Check attribute namespaces
	for _, attr := range attrs {
		if attr.Namespace != "" {
			if b.declareNamespaceIfNeeded(attr.Namespace) {
				if prefix, ok := b.namespaces[attr.Namespace]; ok {
					if prefix == "" {
						decls = append(decls, Attr{Name: "xmlns", Value: attr.Namespace})
					} else {
						decls = append(decls, Attr{Name: "xmlns:" + prefix, Value: attr.Namespace})
					}
				}
			}
		}
	}

	if len(decls) > 0 {
		return append(decls, attrs...)
	}
	return attrs
}

// writeQName writes a qualified name (prefix:localName or just localName).
func (b *Builder) writeQName(namespace, localName string) {
	if prefix, ok := b.namespaces[namespace]; ok && prefix != "" {
		b.buf.WriteString(prefix)
		b.buf.WriteByte(':')
	}
	b.buf.WriteString(localName)
}

// EmptyElementInlineNS writes a self-closing element with an inline namespace declaration.
// This is used for extension elements that carry their own namespace declaration.
func (b *Builder) EmptyElementInlineNS(nsURI, prefix, localName string, attrs ...Attr) {
	b.writeIndent()
	b.buf.WriteByte('<')
	b.buf.WriteString(prefix)
	b.buf.WriteByte(':')
	b.buf.WriteString(localName)
	b.buf.WriteString(` xmlns:`)
	b.buf.WriteString(prefix)
	b.buf.WriteString(`="`)
	b.writeAttrEscaped(nsURI)
	b.buf.WriteByte('"')

	for _, attr := range attrs {
		b.buf.WriteByte(' ')
		if attr.Namespace != "" {
			b.writeQName(attr.Namespace, attr.Name)
		} else {
			b.buf.WriteString(attr.Name)
		}
		b.buf.WriteString(`="`)
		b.writeAttrEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.writeSelfClose()
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// StartElementInlineNS starts an element with an inline namespace declaration.
// The namespace is registered in the builder so child elements using the same
// namespace can resolve the prefix. Call ResetNamespaceDeclaration after EndElement
// to allow the next usage to get its own inline declaration.
func (b *Builder) StartElementInlineNS(nsURI, prefix, localName string, attrs ...Attr) {
	b.namespaces[nsURI] = prefix
	b.declaredNamespaces[nsURI] = true

	b.writeIndent()
	b.buf.WriteByte('<')
	b.buf.WriteString(prefix)
	b.buf.WriteByte(':')
	b.buf.WriteString(localName)
	b.buf.WriteString(` xmlns:`)
	b.buf.WriteString(prefix)
	b.buf.WriteString(`="`)
	b.buf.WriteString(nsURI)
	b.buf.WriteByte('"')

	for _, attr := range attrs {
		b.buf.WriteByte(' ')
		if attr.Namespace != "" {
			b.writeQName(attr.Namespace, attr.Name)
		} else {
			b.buf.WriteString(attr.Name)
		}
		b.buf.WriteString(`="`)
		b.writeAttrEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.pushElem(localName)
	b.level++
}

// EndElementInlineNS ends an element that was started with StartElementInlineNS.
func (b *Builder) EndElementInlineNS(prefix, localName string) {
	b.popElem(localName)
	b.writeIndent()
	b.buf.WriteString("</")
	b.buf.WriteString(prefix)
	b.buf.WriteByte(':')
	b.buf.WriteString(localName)
	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// ResetNamespaceDeclaration marks a namespace as undeclared so the next usage
// will produce a fresh inline xmlns declaration. Used between extension elements.
func (b *Builder) ResetNamespaceDeclaration(nsURI string) {
	delete(b.declaredNamespaces, nsURI)
}

// IsNamespaceDeclared returns true if the namespace URI has been declared
// (either at the root level or via an inline declaration).
func (b *Builder) IsNamespaceDeclared(nsURI string) bool {
	return b.declaredNamespaces[nsURI]
}

// writeIndent writes the current indentation or element separator.
func (b *Builder) writeIndent() {
	// Capture and reset trailingWS flag. This ensures stale state from
	// a prior WriteRaw doesn't leak through intervening element writes.
	ws := b.trailingWS
	b.trailingWS = false

	if b.indent != "" {
		for i := 0; i < b.level; i++ {
			b.buf.WriteString(b.indent)
		}
	} else if b.elemSeparator != "" && b.hasRoot {
		if !ws {
			b.buf.WriteString(b.elemSeparator)
		}
	}
}

// attrEscaper escapes XML attribute values delimited by double quotes.
// Per XML spec §3.3.3: &, <, and the delimiting quote character must be escaped.
// Whitespace characters are written as character references to survive
// XML attribute-value normalization (XML spec §3.3.3).
var attrEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"\n", "&#xA;",
	"\r", "&#xD;",
	"\t", "&#x9;",
)

// textEscaper escapes XML text content (character data).
// Per XML spec §2.4: only & and < must be escaped in character data.
// > is also escaped for safety (required in the sequence ]]>).
var textEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

// EscapeAttrValue returns s escaped for use as the value of a double-quoted XML
// attribute, applying the same rules as the Builder's own attribute writer:
// &, <, >, and the delimiting quote are escaped, whitespace is written as
// character references to survive attribute-value normalization, and
// XML-illegal control characters are dropped.
func EscapeAttrValue(s string) string {
	return attrEscaper.Replace(stripInvalidXMLChars(s))
}

// writeAttrEscaped writes escaped XML attribute value content.
func (b *Builder) writeAttrEscaped(s string) {
	_, _ = attrEscaper.WriteString(&b.buf, stripInvalidXMLChars(s))
}

// writeTextEscaped writes escaped XML text content.
func (b *Builder) writeTextEscaped(s string) {
	_, _ = textEscaper.WriteString(&b.buf, stripInvalidXMLChars(s))
}

// isInvalidXMLByte reports whether c is a byte that cannot appear in a
// well-formed XML 1.0 document. The only control characters permitted are
// tab (0x09), newline (0x0A), and carriage return (0x0D); every other byte
// below 0x20 is illegal and cannot even be represented as a character
// reference (XML spec §2.2). Such bytes are always < 0x80, so filtering them
// out at the byte level never splits a multi-byte UTF-8 sequence.
func isInvalidXMLByte(c byte) bool {
	return c < 0x20 && c != '\t' && c != '\n' && c != '\r'
}

// stripInvalidXMLChars drops XML-1.0-illegal control characters from s. It
// allocates only when such a character is present; the common case (no
// invalid bytes) returns s unchanged.
func stripInvalidXMLChars(s string) string {
	i := 0
	for ; i < len(s); i++ {
		if isInvalidXMLByte(s[i]) {
			break
		}
	}
	if i == len(s) {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	sb.WriteString(s[:i])
	for ; i < len(s); i++ {
		if c := s[i]; !isInvalidXMLByte(c) {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// Attr represents an XML attribute.
type Attr struct {
	Namespace string // Namespace URI (empty for no namespace)
	Name      string // Local name
	Value     string
}

// NSDecl represents a namespace declaration.
type NSDecl struct {
	Prefix string
	URI    string
}

// RootAttr represents an attribute on a root XML element, which can be either
// a namespace declaration (xmlns:prefix="URI" or xmlns="URI") or a regular
// attribute (e.g., mc:Ignorable="x15"). This preserves the exact ordering of
// all attributes for byte-identical round-trip.
type RootAttr struct {
	IsNS      bool   // true = namespace declaration, false = regular attribute
	Prefix    string // For NS: the namespace prefix (empty for default xmlns). For attr: the prefix (e.g., "mc")
	LocalName string // For NS: unused. For attr: the local name (e.g., "Ignorable")
	Value     string // For NS: the namespace URI. For attr: the attribute value
}

// PresentationMLNamespaces returns the standard namespace declarations for PresentationML.
func PresentationMLNamespaces() []NSDecl {
	return []NSDecl{
		{PrefixDrawingML, NSDrawingML},
		{PrefixRelationships, NSPresentationRels},
		{PrefixPresentationML, NSPresentationML},
	}
}

// WordprocessingMLNamespaces returns the standard namespace declarations for WordprocessingML.
func WordprocessingMLNamespaces() []NSDecl {
	return []NSDecl{
		{PrefixWordprocessingML, NSWordprocessingML},
		{PrefixRelationships, NSPresentationRels},
		{PrefixMarkupCompatibility, NSMarkupCompatibility},
	}
}

// SpreadsheetMLNamespaces returns the standard namespace declarations for SpreadsheetML.
func SpreadsheetMLNamespaces() []NSDecl {
	return []NSDecl{
		{"", NSSpreadsheetML},
		{PrefixRelationships, NSPresentationRels},
	}
}

// NewSpreadsheetMLBuilder creates a builder pre-configured for SpreadsheetML documents.
func NewSpreadsheetMLBuilder() *Builder {
	b := NewBuilder()
	b.RegisterNamespace(NSSpreadsheetML, "")
	b.RegisterNamespace(NSPresentationRels, PrefixRelationships)
	b.RegisterNamespace(NSMarkupCompatibility, PrefixMarkupCompatibility)
	return b
}

// NewWordprocessingMLBuilder creates a builder pre-configured for WordprocessingML documents.
func NewWordprocessingMLBuilder() *Builder {
	b := NewBuilder()
	b.RegisterNamespace(NSWordprocessingML, PrefixWordprocessingML)
	b.RegisterNamespace(NSPresentationRels, PrefixRelationships)
	b.RegisterNamespace(NSDrawingML, PrefixDrawingML)
	b.RegisterNamespace(NSMarkupCompatibility, PrefixMarkupCompatibility)
	b.RegisterNamespace(NSDrawingMLWordprocessing, "wp")
	b.RegisterNamespace(NSWord2010, PrefixWord2010)
	b.RegisterNamespace(NSWord2012, PrefixWord2012)
	return b
}

// Helper functions for common attribute patterns

// IntAttr creates an integer attribute.
func IntAttr(name string, value int64) Attr {
	return Attr{Name: name, Value: itoa(value)}
}

// Int32Attr creates an int32 attribute.
func Int32Attr(name string, value int32) Attr {
	return Attr{Name: name, Value: itoa(int64(value))}
}

// UintAttr creates an unsigned integer attribute.
func UintAttr(name string, value uint32) Attr {
	return Attr{Name: name, Value: uitoa(value)}
}

// BoolAttr creates a boolean attribute (1/0).
func BoolAttr(name string, value bool) Attr {
	if value {
		return Attr{Name: name, Value: "1"}
	}
	return Attr{Name: name, Value: "0"}
}

// StrAttr creates a string attribute.
func StrAttr(name, value string) Attr {
	return Attr{Name: name, Value: value}
}

// RelAttr creates a relationship ID attribute (r:id).
func RelAttr(name, value string) Attr {
	return Attr{Namespace: NSPresentationRels, Name: name, Value: value}
}

// itoa converts int64 to string.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// uitoa converts uint32 to string.
func uitoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
