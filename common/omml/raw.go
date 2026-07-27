package omml

import (
	"encoding/xml"
	"reflect"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// xmlNamespace is the reserved namespace of the xml: prefix (e.g. xml:space).
const xmlNamespace = "http://www.w3.org/XML/1998/namespace"

// Raw preserves an element the typed model does not cover, verbatim and in
// position: the WordprocessingML content allowed inside math zones
// (w:EG_PContentMath), the w:rPr / w:ins / w:del children of m:ctrlPr and
// m:r (common/omml cannot model WML types), and any unknown element. A typed
// round-trip is therefore never lossier than the raw capture it came from.
type Raw struct {
	// Local is the element's local name; Space its namespace URI. An empty
	// Space means the element was unqualified. An empty Local means the
	// capture is character data rather than an element: Content then holds
	// the escaped text and Attrs is nil (see captureCharData).
	Local string
	Space string
	// Attrs preserves the start-tag attributes, including any inline xmlns
	// declarations.
	Attrs []xml.Attr
	// Content is the element's inner XML, verbatim.
	Content []byte
}

// UnmarshalXML captures the element's name, attributes, and inner XML.
func (r *Raw) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	r.Local = start.Name.Local
	r.Space = start.Name.Space
	if len(start.Attr) > 0 {
		r.Attrs = append(r.Attrs[:0], start.Attr...)
	}
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	r.Content = inner.Content
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for Raw.
func (r *Raw) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := make([]xmlb.Attr, 0, len(r.Attrs))
	for _, a := range r.Attrs {
		switch {
		case a.Name.Space == "xmlns":
			// Prefixed namespace declaration (xmlns:w="...").
			attrs = append(attrs, xmlb.Attr{Name: "xmlns:" + a.Name.Local, Value: a.Value})
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			// Default namespace declaration.
			attrs = append(attrs, xmlb.Attr{Name: "xmlns", Value: a.Value})
		case a.Name.Space == xmlNamespace:
			// Reserved xml: prefix (xml:space); never declared, always bound.
			attrs = append(attrs, xmlb.Attr{Name: "xml:" + a.Name.Local, Value: a.Value})
		default:
			attrs = append(attrs, xmlb.Attr{Namespace: a.Name.Space, Name: a.Name.Local, Value: a.Value})
		}
	}
	if len(r.Content) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	b.WriteRaw(r.Content)
	b.EndElement(ns, localName)
}

// marshal writes the element under its captured name. An element captured
// without a namespace defaults to the math namespace.
//
// When the captured attributes declare the element's own namespace — a
// producer extension carrying xmlns:foo="urn:x-foo" on the element itself —
// the name is replayed literally under the prefix the source bound. The
// Builder's registry knows nothing about a vendor URI, so resolving through
// it would strip the prefix (leaving the element in the wrong namespace) or
// fail the whole marshal; the declaration the capture carries is the
// authority here.
func (r *Raw) marshal(b *xmlb.Builder) {
	if r.Local == "" {
		// Captured character data, already escaped.
		b.WriteRaw(r.Content)
		return
	}
	ns := r.Space
	if ns == "" {
		ns = NS
	}
	captured := xmlb.CaptureAttrs(r.Attrs)
	if prefix, ok := selfDeclaredPrefix(captured, ns); ok {
		attrs := xmlb.RawAttrList(captured)
		if len(r.Content) == 0 {
			b.EmptyElementLiteral(prefix, r.Local, attrs...)
			return
		}
		b.StartElementLiteral(prefix, r.Local, []xmlb.NSDecl{{Prefix: prefix, URI: ns}}, attrs...)
		b.WriteRaw(r.Content)
		b.EndElementLiteral(prefix, r.Local)
		return
	}
	r.MarshalToBuilder(b, ns, r.Local)
}

// captureCharData returns a raw capture for character data appearing between
// a container's children, or nil for whitespace — the inter-element
// formatting of a pretty-printed source, which is not content and normalizes
// away. Non-whitespace text there is non-conformant, but dropping it silently
// is the inverse of the leniency this package extends to unknown elements.
func captureCharData(t xml.CharData) *Raw {
	s := string(t)
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &Raw{Content: []byte(xmlb.EscapeText(s))}
}

// selfDeclaredPrefix returns the prefix a captured attribute list declares for
// uri and whether such a declaration is present. A default declaration
// (xmlns="uri") reports the empty prefix, so an element the source wrote
// unprefixed stays unprefixed.
func selfDeclaredPrefix(captured []xmlb.RootAttr, uri string) (string, bool) {
	for _, ra := range captured {
		if ra.IsNS && ra.Value == uri {
			return ra.Prefix, true
		}
	}
	return "", false
}

func (r *Raw) emitMath(b *xmlb.Builder)     { r.marshal(b) }
func (r *Raw) emitRunChild(b *xmlb.Builder) { r.marshal(b) }

// extraChild anchors a raw-captured child of a fixed-sequence container to
// the schema-defined child that preceded it in the source document, so it is
// re-emitted in position. rank is the schema rank of the preceding known
// child (-1 for the element start); idx disambiguates repeated children.
type extraChild struct {
	rank int
	idx  int
	raw  *Raw
}

// seqField describes one schema-defined child of a fixed-sequence container:
// its local name in the math namespace and the struct field that holds it.
// Fields are listed in schema order; that order is the marshal order.
type seqField struct {
	name  string
	index int
}

// seqFields resolves "elemName=FieldName" specs against a container's struct
// type at package init, so a typo fails every test instead of silently
// dropping a child.
func seqFields(prototype interface{}, specs ...string) []seqField {
	t := reflect.TypeOf(prototype)
	out := make([]seqField, 0, len(specs))
	for _, s := range specs {
		name, fieldName, ok := strings.Cut(s, "=")
		if !ok {
			panic("omml: malformed seqField spec " + s)
		}
		f, ok := t.FieldByName(fieldName)
		if !ok {
			panic("omml: no field " + fieldName + " on " + t.Name())
		}
		out = append(out, seqField{name: name, index: f.Index[0]})
	}
	return out
}

// unmarshalSeq decodes a fixed-sequence container: children in the math
// namespace matching a schema field land in their struct field, everything
// else is raw-captured in position (anchored to the preceding known child).
func unmarshalSeq(d *xml.Decoder, v interface{}, fields []seqField, extra *[]extraChild) error {
	rv := reflect.ValueOf(v).Elem()
	lastRank, lastIdx := -1, 0
	captureRaw := func(t xml.StartElement) error {
		r := &Raw{}
		if err := r.UnmarshalXML(d, t); err != nil {
			return err
		}
		*extra = append(*extra, extraChild{rank: lastRank, idx: lastIdx, raw: r})
		return nil
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			// Non-whitespace character data inside a property bag is
			// non-conformant, but dropping it silently is the inverse of the
			// leniency this package applies to unknown elements. Keep it in
			// position (whitespace of a pretty-printed source is not content).
			if text := captureCharData(t); text != nil {
				*extra = append(*extra, extraChild{rank: lastRank, idx: lastIdx, raw: text})
			}
		case xml.StartElement:
			rank := -1
			if t.Name.Space == NS {
				for i, f := range fields {
					if f.name == t.Name.Local {
						rank = i
						break
					}
				}
			}
			if rank < 0 {
				if err := captureRaw(t); err != nil {
					return err
				}
				continue
			}
			fv := rv.Field(fields[rank].index)
			if fv.Kind() == reflect.Slice {
				ev := reflect.New(fv.Type().Elem().Elem())
				if err := d.DecodeElement(ev.Interface(), &t); err != nil {
					return err
				}
				lastRank, lastIdx = rank, fv.Len()
				fv.Set(reflect.Append(fv, ev))
			} else {
				if !fv.IsNil() {
					// A repeated singleton is non-conformant input. Overwriting
					// the field dropped the first occurrence silently; route the
					// second to the in-position raw capture so nothing is lost.
					if err := captureRaw(t); err != nil {
						return err
					}
					continue
				}
				ev := reflect.New(fv.Type().Elem())
				if err := d.DecodeElement(ev.Interface(), &t); err != nil {
					return err
				}
				fv.Set(ev)
				lastRank, lastIdx = rank, 0
			}
		case xml.EndElement:
			return nil
		}
	}
}

// marshalSeq writes a fixed-sequence container: schema children in schema
// order with raw-captured extras re-emitted after their anchor child. An
// element with no children self-closes.
func marshalSeq(b *xmlb.Builder, ns, localName string, v interface{}, fields []seqField, extra []extraChild) {
	rv := reflect.ValueOf(v).Elem()
	if len(extra) == 0 && seqEmpty(rv, fields) {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	marshalSeqContent(b, rv, fields, extra)
	b.EndElement(ns, localName)
}

// seqEmpty reports whether none of the schema fields holds a child.
func seqEmpty(rv reflect.Value, fields []seqField) bool {
	for _, f := range fields {
		fv := rv.Field(f.index)
		if fv.Kind() == reflect.Slice {
			if fv.Len() > 0 {
				return false
			}
		} else if !fv.IsNil() {
			return false
		}
	}
	return true
}

// marshalSeqContent writes the children of a fixed-sequence container.
func marshalSeqContent(b *xmlb.Builder, rv reflect.Value, fields []seqField, extra []extraChild) {
	ei := 0
	// flush emits the extras anchored at or before the (rank, idx) child that
	// was just written. Extras anchored to an earlier child that is now absent
	// (removed after parsing) are emitted at the next opportunity.
	flush := func(rank, idx int) {
		for ei < len(extra) && (extra[ei].rank < rank ||
			(extra[ei].rank == rank && extra[ei].idx <= idx)) {
			extra[ei].raw.marshal(b)
			ei++
		}
	}
	flush(-1, 0)
	for i, f := range fields {
		fv := rv.Field(f.index)
		if fv.Kind() == reflect.Slice {
			for j := 0; j < fv.Len(); j++ {
				emitSeqChild(b, f.name, fv.Index(j))
				flush(i, j)
			}
		} else if !fv.IsNil() {
			emitSeqChild(b, f.name, fv)
			flush(i, 0)
		}
	}
	// Extras whose anchor child was removed after parsing still emit.
	for ; ei < len(extra); ei++ {
		extra[ei].raw.marshal(b)
	}
}

func emitSeqChild(b *xmlb.Builder, name string, fv reflect.Value) {
	fv.Interface().(xmlb.BuilderMarshaler).MarshalToBuilder(b, NS, name)
}
