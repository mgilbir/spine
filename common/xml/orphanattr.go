package xml

import "strconv"

// qualifyOrphanAttrs binds a prefix to every attribute namespace that nothing
// in scope declares, returning the declarations that must be written on the
// same element.
//
// It closes a gap in the literal emission paths. Those write attribute names
// verbatim, so a name is resolved once, up front, by renderedAttrName — which
// returns the *bare* local name when the Builder has no prefix registered for
// the attribute's namespace. For a raw child that is the common case, not the
// exotic one: a CT_RawElement keeps Go's parsed attributes, which carry the
// namespace URI and not the prefix the source wrote, and a raw child is
// precisely the element whose namespace this library has no model for.
//
// Writing it bare is wrong twice over. An unprefixed attribute is in no
// namespace at all, so the round trip silently moves w:val to val — a different
// attribute. And a local name only has to be a name once its prefix is gone:
// FuzzDocxNumberingXML reached w:4444444tNumId, whose local part starts with a
// digit, and the emitted word/numbering.xml did not parse.
//
// The minted prefix is a last resort. A document that gets here declares a
// namespace this library has no opinion about, and keeping the attribute in the
// right namespace under an ugly prefix beats writing a different attribute
// under a pretty one.
func (b *Builder) qualifyOrphanAttrs(attrs []Attr) ([]Attr, []Attr) {
	var decls []Attr
	minted := make(map[string]string) // URI -> prefix minted here

	out := make([]Attr, len(attrs))
	for i, a := range attrs {
		if a.Namespace == "" {
			out[i] = a
			continue
		}
		if p, ok := b.namespaces[a.Namespace]; ok && p != "" {
			a.Name = p + ":" + a.Name
			a.Namespace = ""
			out[i] = a
			continue
		}
		p, ok := minted[a.Namespace]
		if !ok {
			p = freeAttrPrefix(attrs, decls, b.namespaces)
			minted[a.Namespace] = p
			decls = append(decls, Attr{Name: "xmlns:" + p, Value: a.Namespace})
		}
		a.Name = p + ":" + a.Name
		a.Namespace = ""
		out[i] = a
	}
	return out, decls
}

// freeAttrPrefix returns the lowest ns<n> nothing in scope has taken: not a
// declaration already in this element's attribute list, not one minted for an
// earlier orphan on the same element, and not a prefix the Builder has
// registered.
//
// The loop is bounded for the same reason freeRootPrefix's is: a document
// carrying a hundred of these is past the point where the hundred-and-first
// spelling helps.
func freeAttrPrefix(attrs []Attr, decls []Attr, registered map[string]string) string {
	taken := func(p string) bool {
		name := "xmlns:" + p
		for _, a := range attrs {
			if a.Name == name {
				return true
			}
		}
		for _, d := range decls {
			if d.Name == name {
				return true
			}
		}
		for _, rp := range registered {
			if rp == p {
				return true
			}
		}
		return false
	}
	for i := 1; i < 100; i++ {
		if candidate := "ns" + strconv.Itoa(i); !taken(candidate) {
			return candidate
		}
	}
	return "ns"
}

// writeRawAttr writes an attribute's verbatim source rendering, which is
// captured with the whitespace that preceded it so a replay reproduces the
// producer's spacing exactly.
//
// A capture can carry no leading whitespace, because Go's decoder does not
// require any: it accepts <pgSz w:w="0"A=""/> and reports two attributes, the
// second of which begins at the closing quote of the first. Replaying that
// verbatim runs the previous attribute into this one and emits
// <w:pgSzA=""/> — a start tag that does not parse, which is what
// FuzzDocxDocumentPart found. One space is enough to separate them, and it is
// only ever added where the source had nothing to reproduce.
func (b *Builder) writeRawAttr(raw string) {
	if raw != "" && !isXMLSpace(raw[0]) {
		b.buf.WriteByte(' ')
	}
	b.buf.WriteString(raw)
}

// declareRootNamespaceIfMissing writes the binding for namespace when none of
// declareNS already provides it, so a root element is never written under a
// prefix nothing declares.
//
// The prefix is the one writeQName just used, which is what makes the pair
// consistent: an empty registration means the name went out bare and the
// binding has to be the default xmlns, and a registered prefix means the name
// went out prefixed and the binding has to be xmlns:prefix.
func (b *Builder) declareRootNamespaceIfMissing(namespace string, declareNS []NSDecl) {
	if namespace == "" {
		return
	}
	for _, ns := range declareNS {
		if ns.URI == namespace {
			return
		}
	}
	b.writeRootNSDecl(&NSDecl{Prefix: b.namespaces[namespace], URI: namespace}, namespace)
}
