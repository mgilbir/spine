package xml

import (
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
		b.writeEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.level++
}

// StartElementWithNS starts an element and declares namespaces.
// This is typically used for the root element.
func (b *Builder) StartElementWithNS(namespace, localName string, declareNS []NSDecl, attrs ...Attr) {
	b.hasRoot = true
	b.writeIndent()
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	// Write namespace declarations
	for _, ns := range declareNS {
		b.buf.WriteString(` xmlns:`)
		b.buf.WriteString(ns.Prefix)
		b.buf.WriteString(`="`)
		b.buf.WriteString(ns.URI)
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
		b.writeEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.level++
}

// EndElement ends the current element.
func (b *Builder) EndElement(namespace, localName string) {
	b.level--
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
		b.writeEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	b.buf.WriteString("/>")
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
		b.writeEscaped(attr.Value)
		b.buf.WriteByte('"')
	}

	if content == "" {
		b.buf.WriteString("/>")
	} else {
		b.buf.WriteByte('>')
		b.writeEscaped(content)
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
		if prefix, ok := b.namespaces[elemNS]; ok && prefix != "" {
			decls = append(decls, Attr{Name: "xmlns:" + prefix, Value: elemNS})
		}
	}

	// Check attribute namespaces
	for _, attr := range attrs {
		if attr.Namespace != "" {
			if b.declareNamespaceIfNeeded(attr.Namespace) {
				if prefix, ok := b.namespaces[attr.Namespace]; ok && prefix != "" {
					decls = append(decls, Attr{Name: "xmlns:" + prefix, Value: attr.Namespace})
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

// writeIndent writes the current indentation.
func (b *Builder) writeIndent() {
	for i := 0; i < b.level; i++ {
		b.buf.WriteString(b.indent)
	}
}

// writeEscaped writes escaped XML content.
func (b *Builder) writeEscaped(s string) {
	for _, r := range s {
		switch r {
		case '&':
			b.buf.WriteString("&amp;")
		case '<':
			b.buf.WriteString("&lt;")
		case '>':
			b.buf.WriteString("&gt;")
		case '"':
			b.buf.WriteString("&quot;")
		case '\'':
			// Apostrophes don't need escaping in double-quoted attributes or text content.
			b.buf.WriteRune('\'')
		default:
			b.buf.WriteRune(r)
		}
	}
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

// PresentationMLNamespaces returns the standard namespace declarations for PresentationML.
func PresentationMLNamespaces() []NSDecl {
	return []NSDecl{
		{PrefixDrawingML, NSDrawingML},
		{PrefixRelationships, NSPresentationRels},
		{PrefixPresentationML, NSPresentationML},
	}
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
