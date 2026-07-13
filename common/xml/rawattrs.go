package xml

import "encoding/xml"

// This file provides verbatim attribute capture and replay for elements whose
// producers interleave xmlns declarations with regular attributes (extension
// children like a16:creationId, mc:Choice branches, PresentationML part
// roots). Go's decoder resolves prefixes to URIs and gives no access to the
// source's declaration placement, so a fixed emission (declaration first,
// attributes after) drifts from sources that wrote them in another order or
// carried extra declarations such as xmlns="". The captured RootAttr list
// preserves the exact interleaving; marshal replays it when present and falls
// back to the canonical emission for values built programmatically.

// CaptureAttrs converts a decoded start element's attributes into an
// interleaved RootAttr list preserving source order. Prefixes for namespaced
// attributes are resolved from well-known URIs and from declarations seen
// earlier in the same attribute list (matching the workbook root capture).
// The result is never nil, so callers can use nil to mean "not parsed from
// XML" (programmatic values) and an empty non-nil slice to mean "parsed, no
// attributes".
func CaptureAttrs(attrs []xml.Attr) []RootAttr {
	out := make([]RootAttr, 0, len(attrs))
	for _, attr := range attrs {
		switch {
		case attr.Name.Space == "xmlns":
			out = append(out, RootAttr{IsNS: true, Prefix: attr.Name.Local, Value: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			out = append(out, RootAttr{IsNS: true, Prefix: "", Value: attr.Value})
		default:
			out = append(out, RootAttr{
				IsNS:      false,
				Prefix:    prefixForAttr(attr.Name.Space, out),
				LocalName: attr.Name.Local,
				Value:     attr.Value,
			})
		}
	}
	return out
}

// prefixForAttr resolves the prefix an attribute in the given namespace should
// be written with: none for unqualified attributes, the reserved xml prefix,
// a well-known OOXML prefix, or the prefix of a declaration captured earlier
// in the same list.
func prefixForAttr(space string, seen []RootAttr) string {
	switch space {
	case "":
		return ""
	case NSXML:
		return "xml"
	}
	for _, ra := range seen {
		if ra.IsNS && ra.Value == space {
			return ra.Prefix
		}
	}
	switch space {
	case NSMarkupCompatibility:
		return PrefixMarkupCompatibility
	case NSOfficeDocumentRels:
		return PrefixRelationships
	case NSWordprocessingML:
		return PrefixWordprocessingML
	case NSDrawingML:
		return PrefixDrawingML
	case NSPresentationML:
		return PrefixPresentationML
	case NSMath:
		return PrefixMath
	}
	for p, uri := range ExtensionPrefixToNS {
		if uri == space {
			return p
		}
	}
	return ""
}

// RawAttrPrefix returns the prefix a captured element's own namespace was
// declared with, or fallback when the capture carries no declaration for it
// (the declaration is in an ancestor's scope).
func RawAttrPrefix(raw []RootAttr, uri, fallback string) string {
	for _, ra := range raw {
		if ra.IsNS && ra.Value == uri && ra.Prefix != "" {
			return ra.Prefix
		}
	}
	return fallback
}

// RawAttrList converts a captured RootAttr list into literal builder
// attributes: declarations become literal xmlns / xmlns:prefix attributes and
// regular attributes keep their prefixed names, preserving the interleaved
// source order.
func RawAttrList(raw []RootAttr) []Attr {
	if len(raw) == 0 {
		return nil
	}
	out := make([]Attr, 0, len(raw))
	for _, ra := range raw {
		out = append(out, ra.attr())
	}
	return out
}

// RawAttrListOverride is RawAttrList with current model values substituted:
// a non-declaration attribute whose rendered name (e.g. "id", "val",
// "r:embed") appears in override gets that value instead of the captured one,
// so a field mutated after parse is not shadowed by the stale capture.
func RawAttrListOverride(raw []RootAttr, override map[string]string) []Attr {
	if len(raw) == 0 {
		return nil
	}
	out := make([]Attr, 0, len(raw))
	for _, ra := range raw {
		a := ra.attr()
		if !ra.IsNS {
			if v, ok := override[a.Name]; ok {
				a.Value = v
			}
		}
		out = append(out, a)
	}
	return out
}

// attr renders one captured RootAttr as a literal builder attribute.
func (ra RootAttr) attr() Attr {
	if ra.IsNS {
		name := "xmlns"
		if ra.Prefix != "" {
			name = "xmlns:" + ra.Prefix
		}
		return Attr{Name: name, Value: ra.Value}
	}
	name := ra.LocalName
	if ra.Prefix != "" {
		name = ra.Prefix + ":" + ra.LocalName
	}
	return Attr{Name: name, Value: ra.Value}
}

// EmptyElementLiteral writes a self-closing element under an explicit prefix,
// emitting the attributes exactly as given (attribute names are written
// literally, without namespace resolution). Used to replay captured extension
// children whose declarations and attributes must keep their source order.
func (b *Builder) EmptyElementLiteral(prefix, localName string, attrs ...Attr) {
	b.flushOpenTag()
	b.writeIndent()
	b.buf.WriteByte('<')
	if prefix != "" {
		b.buf.WriteString(prefix)
		b.buf.WriteByte(':')
	}
	b.buf.WriteString(localName)
	b.writeLiteralAttrs(attrs)
	b.writeSelfClose()
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// StartElementLiteral opens an element under an explicit prefix with literal
// attributes (see EmptyElementLiteral). Each namespace binding in binds is
// registered for the element's scope so typed children resolve their
// prefixes; the corresponding declarations must already be present among
// attrs (none are synthesized). Close with EndElementLiteral.
func (b *Builder) StartElementLiteral(prefix, localName string, binds []NSDecl, attrs ...Attr) {
	b.flushOpenTag()
	for _, d := range binds {
		if d.URI == "" {
			continue
		}
		b.pendingNSRestores = append(b.pendingNSRestores, nsRestore{uri: d.URI, wasDeclared: b.declaredNamespaces[d.URI]})
		b.namespaces[d.URI] = d.Prefix
		b.declaredNamespaces[d.URI] = true
	}
	b.writeIndent()
	b.buf.WriteByte('<')
	if prefix != "" {
		b.buf.WriteString(prefix)
		b.buf.WriteByte(':')
	}
	b.buf.WriteString(localName)
	b.writeLiteralAttrs(attrs)
	if b.collapseEmpty {
		b.openTag = true
	} else {
		b.buf.WriteByte('>')
		if b.indent != "" {
			b.buf.WriteByte('\n')
		}
	}
	qname := localName
	if prefix != "" {
		qname = prefix + ":" + localName
	}
	b.pushElem(qname)
	b.level++
}

// EndElementLiteral closes an element opened with StartElementLiteral.
func (b *Builder) EndElementLiteral(prefix, localName string) {
	qname := localName
	if prefix != "" {
		qname = prefix + ":" + localName
	}
	if b.openTag {
		b.openTag = false
		b.popElem(qname)
		b.writeSelfClose()
		if b.indent != "" {
			b.buf.WriteByte('\n')
		}
		return
	}
	b.popElem(qname)
	b.writeIndent()
	b.buf.WriteString("</")
	b.buf.WriteString(qname)
	b.buf.WriteByte('>')
	if b.indent != "" {
		b.buf.WriteByte('\n')
	}
}

// writeLiteralAttrs writes attributes with their names emitted verbatim.
func (b *Builder) writeLiteralAttrs(attrs []Attr) {
	for _, attr := range attrs {
		b.buf.WriteByte(' ')
		b.buf.WriteString(attr.Name)
		b.buf.WriteString(`="`)
		b.writeAttrEscaped(attr.Value)
		b.buf.WriteByte('"')
	}
}
