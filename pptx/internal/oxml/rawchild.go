package oxml

import (
	"encoding/xml"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// pmlCorePrefixes maps namespace URIs a re-marshaled PML part declares on its
// root element to their conventional prefixes: a/r/p on slide parts, and p188
// on the modern threaded-comment parts (whose root declares a/r/p188). A raw
// child in one of these namespaces re-emits with its prefix and no injected
// inline declaration. p188 never appears in a slide shape tree, so listing it
// here is safe for both part kinds.
var pmlCorePrefixes = map[string]string{
	xmlb.NSPresentationML:        xmlb.PrefixPresentationML,
	xmlb.NSDrawingML:             xmlb.PrefixDrawingML,
	xmlb.NSOfficeDocumentRels:    xmlb.PrefixRelationships,
	xmlb.NSPowerPointComment2018: xmlb.PrefixPowerPointComment,
}

// pmlExtensionPrefixes maps well-known extension namespace URIs to prefixes.
// These namespaces are NOT declared on the re-marshaled root, so a raw child
// using one gets an injected inline declaration.
var pmlExtensionPrefixes = func() map[string]string {
	m := map[string]string{
		xmlb.NSMarkupCompatibility: xmlb.PrefixMarkupCompatibility,
	}
	for prefix, uri := range xmlb.ExtensionPrefixToNS {
		m[uri] = prefix
	}
	return m
}()

// captureRaw reconstructs the verbatim XML of a child element the decoder is
// positioned on, preserving its inline namespace declarations, its inner
// content, the producer's prefix choice and its empty-tag form. It consumes the
// element.
//
// The prefix and empty-tag style must be read before the content is decoded,
// which is why capture goes through this helper rather than through
// encodeRawChild alone.
func captureRaw(d *xml.Decoder, start xml.StartElement) ([]byte, error) {
	// Both recover information the token stream cannot carry; they need a
	// registered source (xmlb.UnmarshalWithSource) and degrade to
	// ""/EmptyTagUnknown without one.
	srcPrefix, _ := xmlb.ElementPrefix(d)
	emptyStyle := xmlb.CaptureEmptyTagStyle(d)
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return nil, err
	}
	return encodeRawChild(start, inner.Content, srcPrefix, emptyStyle), nil
}

// encodeRawChild reconstructs the raw XML for an unmodeled child (p:contentPart,
// extLst, ink, ...) from its start element and captured inner content, so a
// save re-emits it verbatim instead of deleting it (C32).
//
// Prefixes resolve, in order, from xmlns declarations carried on the element
// itself, the root-declared PresentationML prefixes, the well-known extension
// prefixes, and finally the producer's own prefix recovered from the source
// (srcPrefix). Every namespace that is not root-declared gets an inline
// declaration injected so it stays bound: previously a namespace outside the
// core and extension tables fell through to *unprefixed* emission, silently
// moving the element into a different namespace (C529). Inner content is raw
// bytes, so nested elements keep their original prefixes and declarations
// without further handling.
//
// emptyStyle is the source's form for an element with no content, so
// `<x></x>` is not rewritten to `<x/>`; EmptyTagUnknown keeps the compact form.
func encodeRawChild(start xml.StartElement, inner []byte, srcPrefix string, emptyStyle xmlb.EmptyTagStyle) []byte {
	inline := make(map[string]string) // URI -> prefix declared on the element
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			inline[attr.Value] = attr.Name.Local
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			inline[attr.Value] = ""
		}
	}

	// Namespaces neither declared on the element nor guaranteed by the
	// re-marshaled root need an inline declaration; injected records them in
	// first-use order so the emission is deterministic.
	injected := map[string]string{} // URI -> prefix
	var injectedOrder []string

	prefixFor := func(uri string) (string, bool) {
		if p, ok := inline[uri]; ok {
			return p, true
		}
		if p, ok := pmlCorePrefixes[uri]; ok {
			return p, true
		}
		if p, ok := pmlExtensionPrefixes[uri]; ok {
			return p, true
		}
		if p, ok := injected[uri]; ok {
			return p, true
		}
		return "", false
	}

	// prefixTaken reports whether a candidate prefix is already bound to some
	// other namespace in this element's scope.
	prefixTaken := func(p string) bool {
		for uri, q := range inline {
			if q == p && uri != "" {
				return true
			}
		}
		for _, q := range injected {
			if q == p {
				return true
			}
		}
		return false
	}

	// bind ensures uri is bound, injecting a declaration when needed. hint is
	// the producer's prefix for the element name, used when it is available and
	// unclaimed; otherwise a synthetic nsN prefix is generated.
	bind := func(uri, hint string) {
		if uri == "" || uri == "xmlns" {
			return
		}
		if _, ok := prefixFor(uri); ok {
			return
		}
		p := hint
		if p == "" || prefixTaken(p) {
			for i := 1; ; i++ {
				p = "ns" + strconv.Itoa(i)
				if !prefixTaken(p) {
					break
				}
			}
		}
		injected[uri] = p
		injectedOrder = append(injectedOrder, uri)
	}

	// An extension namespace resolves through pmlExtensionPrefixes but is not
	// declared on the re-marshaled root, so it still needs a declaration.
	needsExtDecl := func(uri string) {
		if uri == "" || uri == "xmlns" {
			return
		}
		if _, ok := inline[uri]; ok {
			return
		}
		if _, ok := pmlCorePrefixes[uri]; ok {
			return
		}
		p, ok := pmlExtensionPrefixes[uri]
		if !ok {
			return
		}
		if _, done := injected[uri]; done {
			return
		}
		injected[uri] = p
		injectedOrder = append(injectedOrder, uri)
	}

	needsExtDecl(start.Name.Space)
	for _, attr := range start.Attr {
		needsExtDecl(attr.Name.Space)
	}
	bind(start.Name.Space, srcPrefix)
	for _, attr := range start.Attr {
		if attr.Name.Space == "xmlns" || attr.Name.Space == "" {
			continue
		}
		bind(attr.Name.Space, "")
	}

	writeName := func(buf []byte, name xml.Name) []byte {
		if name.Space != "" {
			if prefix, ok := prefixFor(name.Space); ok && prefix != "" {
				buf = append(buf, prefix...)
				buf = append(buf, ':')
			}
		}
		return append(buf, name.Local...)
	}

	var buf []byte
	buf = append(buf, '<')
	buf = writeName(buf, start.Name)
	for _, uri := range injectedOrder {
		buf = append(buf, " xmlns:"...)
		buf = append(buf, injected[uri]...)
		buf = append(buf, `="`...)
		buf = append(buf, xmlb.EscapeAttrValue(uri)...)
		buf = append(buf, '"')
	}
	for _, attr := range start.Attr {
		buf = append(buf, ' ')
		switch {
		case attr.Name.Space == "xmlns":
			buf = append(buf, "xmlns:"...)
			buf = append(buf, attr.Name.Local...)
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			buf = append(buf, "xmlns"...)
		default:
			buf = writeName(buf, attr.Name)
		}
		buf = append(buf, `="`...)
		buf = append(buf, xmlb.EscapeAttrValue(attr.Value)...)
		buf = append(buf, '"')
	}

	if len(inner) == 0 {
		switch emptyStyle {
		case xmlb.EmptyTagExpanded:
			buf = append(buf, '>')
			buf = append(buf, "</"...)
			buf = writeName(buf, start.Name)
			return append(buf, '>')
		case xmlb.EmptyTagSelfCloseSpaced:
			return append(buf, " />"...)
		default:
			return append(buf, "/>"...)
		}
	}
	buf = append(buf, '>')
	buf = append(buf, inner...)
	buf = append(buf, "</"...)
	buf = writeName(buf, start.Name)
	return append(buf, '>')
}
