package xml

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
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
	collapseEmpty      bool              // write empty Start/End pairs as self-closing elements
	openTag            bool              // a start tag has been written but not yet closed with '>'
	elemSeparator      string            // inserted between sibling elements (e.g., " ")
	trailingWS         bool              // set by WriteRaw when data ends with whitespace
	rootEndTag         string            // verbatim root end tag override (SetRootEndTag)
	stack              []elemFrame       // open elements, for balance checking and namespace scoping
	pendingNSRestores  []nsRestore       // inline decl state to attach to the next opened element
	err                error             // first structural error encountered
}

// elemFrame records an open element: its qualified name (for balance checking)
// and the namespace-declaration state to restore when it closes. An inline
// xmlns declaration is lexically scoped to the element that carries it, so a
// later sibling in the same namespace must get its own declaration.
type elemFrame struct {
	qname    string
	restores []nsRestore
}

// nsRestore captures the declared-state of a namespace before an element
// declared it inline, so closing the element restores the previous state.
type nsRestore struct {
	uri         string
	wasDeclared bool
}

// qualifiedName returns the name as it is written to the output:
// prefix:localName when the namespace resolves to a non-empty prefix,
// otherwise just localName. Mirrors writeQName.
func (b *Builder) qualifiedName(namespace, localName string) string {
	if prefix, ok := b.namespaces[namespace]; ok && prefix != "" {
		return prefix + ":" + localName
	}
	return localName
}

// pushElem records an opened element's qualified name for balance checking.
// Any pending inline namespace declarations are attached to the new frame so
// they are un-declared (restored) when the element closes.
func (b *Builder) pushElem(qname string) {
	b.stack = append(b.stack, elemFrame{qname: qname, restores: b.pendingNSRestores})
	b.pendingNSRestores = nil
}

// applyNSRestores restores the declared-namespace state captured in restores.
func (b *Builder) applyNSRestores(restores []nsRestore) {
	for _, r := range restores {
		if r.wasDeclared {
			b.declaredNamespaces[r.uri] = true
		} else {
			delete(b.declaredNamespaces, r.uri)
		}
	}
}

// popElem matches a closing element's qualified name against the open-element
// stack, recording the first structural error and preventing the indentation
// level from going negative on an unbalanced close. Inline namespace
// declarations carried by the closing element go out of scope here.
func (b *Builder) popElem(qname string) {
	if len(b.stack) == 0 {
		if b.err == nil {
			b.err = fmt.Errorf("xml: closing </%s> with no open element", qname)
		}
		return
	}
	top := b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]
	b.applyNSRestores(top.restores)
	if top.qname != qname && b.err == nil {
		b.err = fmt.Errorf("xml: closing </%s> does not match open <%s>", qname, top.qname)
	}
	if b.level > 0 {
		b.level--
	}
}

// Err returns the first structural error the Builder encountered (an unbalanced
// or mismatched element, or a name written in an unregistered namespace), or nil.
func (b *Builder) Err() error {
	return b.err
}

// Finish reports any structural error, including elements left unclosed. Call it
// after building to validate that every StartElement was matched by an
// EndElement. Production marshal entry points must consult it before shipping
// the built bytes into a package.
func (b *Builder) Finish() error {
	if b.err != nil {
		return b.err
	}
	if len(b.stack) > 0 {
		return fmt.Errorf("xml: %d unclosed element(s), innermost <%s>", len(b.stack), b.stack[len(b.stack)-1].qname)
	}
	return nil
}

// NewBuilder creates a new XML builder.
func NewBuilder() *Builder {
	return &Builder{
		// The xml: prefix is permanently bound to the reserved XML namespace
		// and must never be declared, so every builder resolves it out of the
		// box (xml:space="preserve" would otherwise lose its prefix).
		namespaces:         map[string]string{NSXML: "xml"},
		declaredNamespaces: map[string]bool{NSXML: true},
	}
}

// NewPresentationMLBuilder creates a builder pre-configured for PresentationML documents.
func NewPresentationMLBuilder() *Builder {
	b := NewBuilder()
	b.RegisterNamespace(NSPresentationML, PrefixPresentationML)
	b.RegisterNamespace(NSDrawingML, PrefixDrawingML)
	b.RegisterNamespace(NSOfficeDocumentRels, PrefixRelationships)
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

// SetCollapseEmptyElements controls whether an element opened with
// StartElement and closed without any intervening content is written as a
// self-closing tag (<name/>) instead of an open/close pair (<name></name>).
// Producers that self-close their empty elements (Word, PowerPoint, Excel)
// get this enabled from per-part capture; the default (false) preserves the
// expanded style.
func (b *Builder) SetCollapseEmptyElements(v bool) { b.collapseEmpty = v }

// flushOpenTag completes a deferred start tag ('>' not yet written) before
// any other content is emitted. Every output method calls it first, so the
// only way a start tag is still open when its EndElement arrives is that the
// element is empty — which is exactly when it may collapse to self-closing.
func (b *Builder) flushOpenTag() {
	if !b.openTag {
		return
	}
	b.openTag = false
	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// SetElementSeparator sets a string to insert between sibling elements (e.g., " " for spaced format).
func (b *Builder) SetElementSeparator(sep string) { b.elemSeparator = sep }

// WriteRaw writes raw content directly to the output buffer without escaping.
func (b *Builder) WriteRaw(data []byte) {
	// An empty write carries no content, so it must not complete a deferred
	// start tag: marshal code that unconditionally writes captured raw
	// content (e.g. an empty w:sdtEndPr) would otherwise defeat the
	// collapse-empty capture and expand the producer's self-closed element.
	if len(data) == 0 {
		return
	}
	b.flushOpenTag()
	b.buf.Write(data)
	last := data[len(data)-1]
	b.trailingWS = last == ' ' || last == '\t' || last == '\n' || last == '\r'
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

// writeOneAttr writes a single attribute, preferring its verbatim source
// rendering when captured (Attr.Raw).
func (b *Builder) writeOneAttr(attr Attr) {
	if attr.Raw != "" {
		b.buf.WriteString(attr.Raw)
		return
	}
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

// StartElement starts an element with the given namespace and local name.
// If attrs is provided, they are written as attributes.
func (b *Builder) StartElement(namespace, localName string, attrs ...Attr) {
	b.flushOpenTag()
	b.writeIndent()
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	for _, attr := range attrs {
		b.writeOneAttr(attr)
	}

	if b.collapseEmpty {
		// Defer the '>' so an element that turns out to be empty can be
		// written self-closing, matching producers that never expand empties.
		b.openTag = true
	} else {
		b.buf.WriteByte('>')
		if b.indent != "" {
			b.buf.WriteByte('\n')
		}
	}
	b.pushElem(b.qualifiedName(namespace, localName))
	b.level++
}

// StartElementWithNS starts an element and declares namespaces.
// This is typically used for the root element.
func (b *Builder) StartElementWithNS(namespace, localName string, declareNS []NSDecl, attrs ...Attr) {
	b.flushOpenTag()
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
		b.writeOneAttr(attr)
	}

	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.pushElem(b.qualifiedName(namespace, localName))
	b.level++
}

// StartElementWithRootAttrs starts a root element writing all attributes in the
// exact order provided. Each RootAttr is either a namespace declaration or a
// regular attribute, preserving the interleaved ordering from the original XML.
//
// The root element's own namespace binding is resolved from the declarations
// before the tag name is written: a source whose root is namespace-prefixed
// (e.g. <x:workbook xmlns:x="…spreadsheetml…">) must re-emit the prefixed
// form. Writing the name first and registering prefixes while emitting the
// declarations would produce an unprefixed open tag whose children and close
// tag then resolve to the prefixed form — malformed XML. A default (xmlns=)
// declaration for the root's namespace wins over a prefixed one, matching the
// unprefixed element names such a source uses.
func (b *Builder) StartElementWithRootAttrs(namespace, localName string, rootAttrs []RootAttr, extraAttrs ...Attr) {
	rootNSBound := false
	for _, ra := range rootAttrs {
		if !ra.IsNS || ra.Value != namespace {
			continue
		}
		if ra.Prefix == "" {
			// A default declaration covers the root's namespace: element
			// names stay unprefixed, regardless of any prefixed alias.
			b.namespaces[namespace] = ""
			break
		}
		if !rootNSBound {
			b.namespaces[namespace] = ra.Prefix
			rootNSBound = true
		}
	}

	b.flushOpenTag()
	b.writeIndent()
	b.hasRoot = true
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	for _, ra := range rootAttrs {
		if ra.IsNS {
			// Namespace declaration
			if ra.Raw != "" {
				b.buf.WriteString(ra.Raw)
			} else {
				if ra.Prefix == "" {
					b.buf.WriteString(` xmlns="`)
				} else {
					b.buf.WriteString(` xmlns:`)
					b.buf.WriteString(ra.Prefix)
					b.buf.WriteString(`="`)
				}
				b.writeAttrEscaped(ra.Value)
				b.buf.WriteByte('"')
			}
			b.declaredNamespaces[ra.Value] = true
			// Also register prefix so writeQName can resolve it for extension
			// attrs — but never clobber the binding chosen for the root's own
			// namespace above (a default declaration must keep element names
			// unprefixed even when a prefixed alias for the same URI exists).
			if ra.Prefix != "" && ra.Value != namespace {
				b.namespaces[ra.Value] = ra.Prefix
			}
		} else if ra.Raw != "" {
			// Regular attribute with a verbatim source rendering.
			b.buf.WriteString(ra.Raw)
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
		if attr.Raw != "" {
			b.buf.WriteString(attr.Raw)
			continue
		}
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
	b.pushElem(b.qualifiedName(namespace, localName))
	b.level++
}

// StartElementWithRootAttrsMerged starts a root element like
// StartElementWithRootAttrs but merges the captured source attribute list with
// the modeled attributes derived from struct fields, via ReplayCapturedAttrs.
// This gives the root the same modeled-wins semantics that non-root elements
// already enjoy: captured entries keep their source order, a modeled value
// overrides the captured one for the same attribute (so post-parse root setters
// such as Presentation.SetEmbedTrueTypeFonts or SlideLayout.SetName take
// effect), and modeled attributes with no captured counterpart follow in model
// order. When nothing was modified the merge reproduces the captured list
// verbatim, preserving a byte-identical round trip.
//
// The root's own namespace binding is resolved from the (untouched) captured
// declarations exactly as StartElementWithRootAttrs does — the modeled list only
// ever carries regular attributes, never namespace declarations.
func (b *Builder) StartElementWithRootAttrsMerged(namespace, localName string, rootAttrs []RootAttr, modeled []Attr, extraAttrs ...Attr) {
	// Bind the root element's own namespace before writing the tag name (see
	// StartElementWithRootAttrs for why the declarations must be consulted
	// first). A default (xmlns=) declaration wins over a prefixed alias.
	rootNSBound := false
	for _, ra := range rootAttrs {
		if !ra.IsNS || ra.Value != namespace {
			continue
		}
		if ra.Prefix == "" {
			b.namespaces[namespace] = ""
			break
		}
		if !rootNSBound {
			b.namespaces[namespace] = ra.Prefix
			rootNSBound = true
		}
	}
	// Register declared prefixes so writeQName can resolve extension attrs, and
	// mark the namespaces declared — without clobbering the root's own binding.
	for _, ra := range rootAttrs {
		if !ra.IsNS {
			continue
		}
		b.declaredNamespaces[ra.Value] = true
		if ra.Prefix != "" && ra.Value != namespace {
			b.namespaces[ra.Value] = ra.Prefix
		}
	}

	merged := b.ReplayCapturedAttrs(rootAttrs, modeled)

	b.flushOpenTag()
	b.writeIndent()
	b.hasRoot = true
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)
	b.writeLiteralAttrs(merged)

	// Write any extra attributes (same handling as StartElementWithRootAttrs).
	for _, attr := range extraAttrs {
		if attr.Raw != "" {
			b.buf.WriteString(attr.Raw)
			continue
		}
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
	b.pushElem(b.qualifiedName(namespace, localName))
	b.level++
}

// EndElement ends the current element. If the element's start tag is still
// open (collapse-empty mode and no content was written), it is completed as a
// self-closing tag instead of an open/close pair.
func (b *Builder) EndElement(namespace, localName string) {
	if b.openTag {
		b.openTag = false
		b.popElem(b.qualifiedName(namespace, localName))
		b.writeSelfClose()
		if b.indent != "" {
			b.buf.WriteByte('\n')
		}
		return
	}
	b.popElem(b.qualifiedName(namespace, localName))
	if b.rootEndTag != "" && b.level == 0 {
		// The captured source form of the document element's end tag (some
		// producers write "</p:sld >").
		b.buf.WriteString(b.rootEndTag)
		return
	}
	b.writeIndent()
	b.buf.WriteString("</")
	b.writeQName(namespace, localName)
	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// SetRootEndTag overrides the document element's end tag with the captured
// verbatim source form (Prolog.RootEnd); empty keeps the canonical form.
func (b *Builder) SetRootEndTag(tag string) { b.rootEndTag = tag }

// EmptyElement writes a self-closing element.
func (b *Builder) EmptyElement(namespace, localName string, attrs ...Attr) {
	b.flushOpenTag()
	b.writeIndent()
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	for _, attr := range attrs {
		b.writeOneAttr(attr)
	}

	b.writeSelfClose()
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.endPendingNSScope()
}

// WriteElement writes a complete element with text content.
func (b *Builder) WriteElement(namespace, localName, content string, attrs ...Attr) {
	b.flushOpenTag()
	b.writeIndent()
	b.buf.WriteByte('<')
	b.writeQName(namespace, localName)

	// writeOneAttr honors a captured verbatim rendering (Attr.Raw), so an
	// attribute whose source form is preserved — e.g. xml:space='preserve'
	// written with single quotes — round-trips through this path too, matching
	// StartElement. A plain inline loop here would re-quote it.
	for _, attr := range attrs {
		b.writeOneAttr(attr)
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
	b.endPendingNSScope()
}

// declareNamespaceIfNeeded checks if a namespace needs an inline xmlns declaration
// and marks it as declared. Returns true if a declaration was needed.
// The declaration is lexically scoped to the element it is emitted on: a
// restore entry is queued so the namespace is un-declared again when that
// element closes (or immediately, for a self-closing element).
func (b *Builder) declareNamespaceIfNeeded(namespace string) bool {
	if namespace == "" || b.declaredNamespaces[namespace] {
		return false
	}
	if _, ok := b.namespaces[namespace]; ok {
		b.declaredNamespaces[namespace] = true
		b.pendingNSRestores = append(b.pendingNSRestores, nsRestore{uri: namespace, wasDeclared: false})
		return true
	}
	return false
}

// endPendingNSScope closes the scope of inline declarations that were queued
// for an element that does not remain open (self-closing or text element).
func (b *Builder) endPendingNSScope() {
	b.applyNSRestores(b.pendingNSRestores)
	b.pendingNSRestores = nil
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
// A non-empty namespace with no registered prefix is a caller bug: the name is
// still written unprefixed (keeping the output well-formed), but the error is
// recorded on the Builder so Err/Finish surface it instead of shipping a part
// whose element silently landed in the wrong namespace.
func (b *Builder) writeQName(namespace, localName string) {
	prefix, ok := b.namespaces[namespace]
	if ok && prefix != "" {
		b.buf.WriteString(prefix)
		b.buf.WriteByte(':')
	}
	if !ok && namespace != "" && b.err == nil {
		b.err = fmt.Errorf("xml: no prefix registered for namespace %q (writing %q)", namespace, localName)
	}
	b.buf.WriteString(localName)
}

// EmptyElementInlineNS writes a self-closing element with an inline namespace declaration.
// This is used for extension elements that carry their own namespace declaration.
func (b *Builder) EmptyElementInlineNS(nsURI, prefix, localName string, attrs ...Attr) {
	b.flushOpenTag()
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
// namespace can resolve the prefix. The declaration is lexically scoped to
// this element: the matching EndElementInlineNS restores the namespace's
// previous declared-state automatically.
func (b *Builder) StartElementInlineNS(nsURI, prefix, localName string, attrs ...Attr) {
	b.flushOpenTag()
	b.pendingNSRestores = append(b.pendingNSRestores, nsRestore{uri: nsURI, wasDeclared: b.declaredNamespaces[nsURI]})
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

	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
	b.pushElem(prefix + ":" + localName)
	b.level++
}

// EndElementInlineNS ends an element that was started with StartElementInlineNS.
func (b *Builder) EndElementInlineNS(prefix, localName string) {
	b.popElem(prefix + ":" + localName)
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
// Carriage return is written as a character reference because XML §2.11
// end-of-line handling makes every conforming parser normalize a literal
// \r (or \r\n) in element content to \n; only &#xD; survives a reparse.
// Tab and newline are NOT escaped: parsers preserve them verbatim in
// character data, and escaping them would cause round-trip byte drift.
var textEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\r", "&#xD;",
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

// TextEscapeReproduces reports whether escaping text as XML character data —
// the exact transformation WriteElement applies to element content — yields
// raw. When true, a separately captured verbatim inner form is byte-for-byte
// redundant with text and need not be retained: the normal escape path already
// reproduces the source. For clean text (no characters the escaper rewrites)
// this is allocation-free — the replacer and the invalid-char strip both return
// the input unchanged and the comparison is a byte compare.
func TextEscapeReproduces(text string, raw []byte) bool {
	return textEscaper.Replace(stripInvalidXMLChars(text)) == string(raw)
}

// isInvalidXMLByte reports whether c is an ASCII byte that cannot appear in
// a well-formed XML 1.0 document. The only control characters permitted are
// tab (0x09), newline (0x0A), and carriage return (0x0D); every other byte
// below 0x20 is illegal and cannot even be represented as a character
// reference (XML spec §2.2).
func isInvalidXMLByte(c byte) bool {
	return c < 0x20 && c != '\t' && c != '\n' && c != '\r'
}

// isValidXMLRune reports whether r matches the XML 1.0 Char production
// (spec §2.2): #x9 | #xA | #xD | [#x20-#xD7FF] | [#xE000-#xFFFD] |
// [#x10000-#x10FFFF]. Notably U+FFFE and U+FFFF are excluded. Surrogate
// code points ([#xD800-#xDFFF]) cannot occur here because valid UTF-8
// never encodes them; they arrive as invalid byte sequences, which the
// caller strips.
func isValidXMLRune(r rune) bool {
	return r == 0x9 || r == 0xA || r == 0xD ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

// stripInvalidXMLChars drops characters that violate the XML 1.0 Char
// production from s: illegal control characters, U+FFFE/U+FFFF, and invalid
// UTF-8 sequences (including lone surrogates encoded as raw bytes). It
// allocates only when something must be stripped; the common case (already
// valid) returns s unchanged.
func stripInvalidXMLChars(s string) string {
	// Fast path: scan without allocating until the first invalid character.
	i := 0
	for i < len(s) {
		if c := s[i]; c < utf8.RuneSelf {
			if isInvalidXMLByte(c) {
				break
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || !isValidXMLRune(r) {
			break
		}
		i += size
	}
	if i == len(s) {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	sb.WriteString(s[:i])
	for i < len(s) {
		if c := s[i]; c < utf8.RuneSelf {
			if !isInvalidXMLByte(c) {
				sb.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r != utf8.RuneError || size > 1) && isValidXMLRune(r) {
			sb.WriteString(s[i : i+size])
		}
		i += size
	}
	return sb.String()
}

// Attr represents an XML attribute.
type Attr struct {
	Namespace string // Namespace URI (empty for no namespace)
	Name      string // Local name
	Value     string
	// Raw, when set, is the verbatim source rendering of this attribute
	// including its leading whitespace (e.g. ` w:val='x'`); it is written
	// as-is, preserving the producer's prefix, quote style, and spacing.
	Raw string
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
	// Space is the attribute's namespace URI (regular attributes only); it
	// lets replay match a captured attribute to its modeled field even when
	// no prefix could be resolved from the same element's declarations.
	Space string
	// Raw, when set, is the verbatim source rendering including leading
	// whitespace (see Attr.Raw); captured by CaptureAttrsSource.
	Raw string
}

// QualifyAttrs resolves namespace-based attribute names to their registered
// prefixed literal form (e.g. {NSWordprocessingML, "val"} -> "w:val"), for
// literal emission paths that write names verbatim.
func (b *Builder) QualifyAttrs(attrs []Attr) []Attr {
	out := make([]Attr, len(attrs))
	for i, a := range attrs {
		if a.Namespace != "" {
			a.Name = b.renderedAttrName(a)
			a.Namespace = ""
		}
		out[i] = a
	}
	return out
}

// PresentationMLNamespaces returns the standard namespace declarations for PresentationML.
func PresentationMLNamespaces() []NSDecl {
	return []NSDecl{
		{PrefixDrawingML, NSDrawingML},
		{PrefixRelationships, NSOfficeDocumentRels},
		{PrefixPresentationML, NSPresentationML},
	}
}

// WordprocessingMLNamespaces returns the standard namespace declarations for WordprocessingML.
func WordprocessingMLNamespaces() []NSDecl {
	return []NSDecl{
		{PrefixWordprocessingML, NSWordprocessingML},
		{PrefixRelationships, NSOfficeDocumentRels},
		{PrefixMarkupCompatibility, NSMarkupCompatibility},
	}
}

// SpreadsheetMLNamespaces returns the standard namespace declarations for SpreadsheetML.
func SpreadsheetMLNamespaces() []NSDecl {
	return []NSDecl{
		{"", NSSpreadsheetML},
		{PrefixRelationships, NSOfficeDocumentRels},
	}
}

// NewSpreadsheetMLBuilder creates a builder pre-configured for SpreadsheetML documents.
func NewSpreadsheetMLBuilder() *Builder {
	b := NewBuilder()
	b.RegisterNamespace(NSSpreadsheetML, "")
	b.RegisterNamespace(NSOfficeDocumentRels, PrefixRelationships)
	b.RegisterNamespace(NSMarkupCompatibility, PrefixMarkupCompatibility)
	return b
}

// NewWordprocessingMLBuilder creates a builder pre-configured for WordprocessingML documents.
func NewWordprocessingMLBuilder() *Builder {
	b := NewBuilder()
	b.RegisterNamespace(NSWordprocessingML, PrefixWordprocessingML)
	b.RegisterNamespace(NSOfficeDocumentRels, PrefixRelationships)
	b.RegisterNamespace(NSDrawingML, PrefixDrawingML)
	b.RegisterNamespace(NSMarkupCompatibility, PrefixMarkupCompatibility)
	b.RegisterNamespace(NSDrawingMLWordprocessing, "wp")
	b.RegisterNamespace(NSWord2010, PrefixWord2010)
	b.RegisterNamespace(NSWord2012, PrefixWord2012)
	b.RegisterNamespace(NSMath, PrefixMath)
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
	return Attr{Namespace: NSOfficeDocumentRels, Name: name, Value: value}
}

// NSDeclAttrs converts captured inline namespace declarations back into
// literal xmlns attributes (xmlns:prefix="uri", or xmlns="uri" for a default
// declaration), appending them to attrs. Raw child content re-emitted inside
// the carrying element then resolves its prefixes exactly as in the source.
func NSDeclAttrs(attrs []Attr, decls []NSDecl) []Attr {
	for _, d := range decls {
		name := "xmlns"
		if d.Prefix != "" {
			name = "xmlns:" + d.Prefix
		}
		attrs = append(attrs, Attr{Name: name, Value: d.URI})
	}
	return attrs
}

// itoa converts int64 to string.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	// math.MinInt64 has no positive counterpart in int64: negating it
	// overflows back to itself and the digit loop would never run.
	if n == math.MinInt64 {
		return "-9223372036854775808"
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
