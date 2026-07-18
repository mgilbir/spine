package oxml

import (
	"encoding/xml"

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

// encodeRawChild reconstructs the raw XML for an unmodeled shape-tree child
// (p:contentPart, extLst, ink, ...) from its start element and captured inner
// content, so a save re-emits it verbatim instead of deleting it (C32).
//
// Prefixes resolve, in order, from xmlns declarations carried on the element
// itself, the root-declared PresentationML prefixes, and the well-known
// extension prefixes; extension namespaces without an inline declaration get
// one injected (the re-marshaled root does not declare them). Inner content
// is raw bytes, so nested elements keep their original prefixes and
// declarations without further handling.
func encodeRawChild(start xml.StartElement, inner []byte) []byte {
	inline := make(map[string]string) // URI -> prefix declared on the element
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			inline[attr.Value] = attr.Name.Local
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			inline[attr.Value] = ""
		}
	}

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
		return "", false
	}

	// Extension namespaces used by the element or its attributes that carry no
	// inline declaration need one injected to stay bound.
	var injected []xmlb.NSDecl
	needsDecl := func(uri string) {
		if uri == "" || uri == "xmlns" {
			return
		}
		if _, ok := inline[uri]; ok {
			return
		}
		if _, ok := pmlCorePrefixes[uri]; ok {
			return
		}
		if p, ok := pmlExtensionPrefixes[uri]; ok {
			for _, d := range injected {
				if d.URI == uri {
					return
				}
			}
			injected = append(injected, xmlb.NSDecl{Prefix: p, URI: uri})
		}
	}
	needsDecl(start.Name.Space)
	for _, attr := range start.Attr {
		needsDecl(attr.Name.Space)
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
	for _, d := range injected {
		buf = append(buf, " xmlns:"...)
		buf = append(buf, d.Prefix...)
		buf = append(buf, `="`...)
		buf = append(buf, xmlb.EscapeAttrValue(d.URI)...)
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
		return append(buf, "/>"...)
	}
	buf = append(buf, '>')
	buf = append(buf, inner...)
	buf = append(buf, "</"...)
	buf = writeName(buf, start.Name)
	return append(buf, '>')
}
