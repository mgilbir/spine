package xml

import (
	"encoding/xml"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// BuilderMarshaler is the interface implemented by types that can marshal
// themselves to a Builder. This takes precedence over reflection-based marshaling.
// Types that contain xs:choice groups with unbounded maxOccurs (like ShapeTree)
// should implement this to preserve element ordering.
type BuilderMarshaler interface {
	MarshalToBuilder(b *Builder, ns, localName string)
}

var builderMarshalerType = reflect.TypeOf((*BuilderMarshaler)(nil)).Elem()

// xmlMarshalerType and xmlMarshalerAttrType identify the stdlib
// encoding/xml marshaling interfaces. The Builder cannot honor them (their
// output is written to an *xml.Encoder with the stdlib's own namespace
// bookkeeping, which does not compose with the Builder's prefix scheme), so a
// type that implements one of them but not the Builder's own marshaling
// interface is a latent footgun: it would be silently reflection-marshaled,
// ignoring its MarshalXML/MarshalXMLAttr. The Builder detects that case and
// records an error (surfaced via Err/Finish) instead of shipping wrong bytes.
var (
	xmlMarshalerType     = reflect.TypeOf((*xml.Marshaler)(nil)).Elem()
	xmlMarshalerAttrType = reflect.TypeOf((*xml.MarshalerAttr)(nil)).Elem()
	attrValuerType       = reflect.TypeOf((*AttrValuer)(nil)).Elem()
)

// AttrValuer is the interface implemented by attribute value types that
// render their own lexical form in reflection-based marshaling (e.g.
// percentage values that re-emit a transitional "n%" source form verbatim).
// IsZeroAttr reports the type's zero value for omitempty handling, replacing
// the kind-based check that cannot see inside a struct.
type AttrValuer interface {
	AttrValue() string
	IsZeroAttr() bool
}

// tagInfo holds parsed xml struct tag information.
type tagInfo struct {
	ns        string // namespace URI (empty = inherit parent)
	name      string // local element/attribute name
	attr      bool   // is an attribute
	omitempty bool   // omit if zero value
	innerxml  bool   // raw inner XML bytes
	chardata  bool   // character data content
	isAny     bool   // catch-all (xml:",any")
	skip      bool   // tag == "-" or XMLName field
}

// parseTag parses an xml struct tag into its components.
func parseTag(tag string, fieldName string) tagInfo {
	if tag == "" || tag == "-" {
		return tagInfo{skip: true}
	}

	var info tagInfo
	parts := strings.Split(tag, ",")

	// Parse namespace and name from first part
	nsAndName := parts[0]
	if idx := strings.LastIndex(nsAndName, " "); idx >= 0 {
		info.ns = nsAndName[:idx]
		info.name = nsAndName[idx+1:]
	} else {
		info.name = nsAndName
	}

	if info.name == "" {
		info.name = fieldName
	}

	// Parse flags
	for _, flag := range parts[1:] {
		switch strings.TrimSpace(flag) {
		case "attr":
			info.attr = true
		case "omitempty":
			info.omitempty = true
		case "innerxml":
			info.innerxml = true
		case "chardata":
			info.chardata = true
		case "any":
			info.isAny = true
		}
	}

	return info
}

// isZeroValue reports whether v is the zero value for its type (for omitempty).
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Struct:
		if av, ok := valueAsAttrValuer(v); ok {
			return av.IsZeroAttr()
		}
		return false
	default:
		return false
	}
}

// valueAsAttrValuer returns v's AttrValuer implementation, if any.
func valueAsAttrValuer(v reflect.Value) (AttrValuer, bool) {
	if !v.CanInterface() {
		return nil, false
	}
	av, ok := v.Interface().(AttrValuer)
	return av, ok
}

// formatValue formats a reflect.Value as a string for XML output.
func formatValue(v reflect.Value) string {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	if av, ok := valueAsAttrValuer(v); ok {
		return av.AttrValue()
	}

	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		if v.Bool() {
			return "1"
		}
		return "0"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Float32:
		// Round-trip through float32's own shortest form: widening to float64
		// first and asking for float64's shortest decimal reprints
		// float32(0.1) as "0.10000000149011612".
		return strconv.FormatFloat(v.Float(), 'f', -1, 32)
	case reflect.Float64:
		return FormatFloat(v.Float())
	default:
		return fmt.Sprint(v.Interface())
	}
}

// FormatFloat renders a float as an OOXML numeric lexical value: the shortest
// decimal that round-trips, never in exponent form ("1000000", not "1e+06").
//
// This is the one float formatting policy for every XML output path. Go's 'g'
// verb — the obvious choice, and what this package used to use — switches to
// exponent notation at magnitudes at or above 1e6 and below 1e-4. That range is
// not exotic: a sparkline's manual axis bound, a pivot field's numeric grouping
// interval, and a chart axis scale all reach it with ordinary data. E-notation
// is legal xsd:double, but Office does not write it in these positions, so
// emitting it makes spine's output textually unlike every real producer's — and
// worse, re-emits a *parsed* value at that magnitude in a spelling its own
// source never used, which is byte drift (C531, C556, C559).
//
// Values whose source spelling must survive verbatim cannot be served here at
// all: by the time a float reaches this function the spelling is gone. Excel
// writes theme tints as "-4.9989318521683403E-2", and no formatting rule
// recovers that from the number. Those fields carry a lexical type instead
// (oxml.FloatLex, oxml.AnimVariantFloat) that replays the original and falls
// back to this function only for values built programmatically.
//
// Extreme magnitudes make long output — 1e300 is 301 characters. That is
// bounded (about 1080 at the smallest denormal), valid, and absent from
// documents Office writes, so it is not worth reintroducing E-notation for.
func FormatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

var xmlNameType = reflect.TypeOf(xml.Name{})

// MarshalRoot marshals a Go struct as an XML root element with namespace declarations.
// This is used for top-level elements like <p:sld>, <p:sldLayout>, <p:sldMaster>.
func (b *Builder) MarshalRoot(ns, localName string, v interface{}, nsDecls []NSDecl, extraAttrs ...Attr) {
	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}

	modeled := b.collectStructAttrs(val)
	// Build a fresh slice: appending onto the caller's variadic slice would
	// write the modeled attributes into its backing array whenever it has
	// spare capacity, silently corrupting a reused attribute buffer.
	attrs := make([]Attr, 0, len(extraAttrs)+len(modeled))
	attrs = append(attrs, extraAttrs...)
	attrs = append(attrs, modeled...)

	b.StartElementWithNS(ns, localName, nsDecls, attrs...)
	b.marshalStructChildren(ns, val)
	b.EndElement(ns, localName)
}

// MarshalElement marshals a Go value as an XML element using registered namespace prefixes.
// The element name and namespace are specified by the caller.
func (b *Builder) MarshalElement(ns, localName string, v interface{}) {
	val := reflect.ValueOf(v)
	b.marshalReflect(ns, localName, val)
}

// MarshalChildren marshals the child elements of a struct (without the enclosing element).
func (b *Builder) MarshalChildren(parentNS string, v interface{}) {
	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	b.marshalStructChildren(parentNS, val)
}

// marshalReflect marshals a value as an XML element, dispatching based on kind.
func (b *Builder) marshalReflect(ns, localName string, val reflect.Value) {
	// Dereference pointers and interfaces down to the concrete value.
	// hasStructChildren counts a non-nil interface field as a child, so the
	// parent opens an element for it; without unwrapping here the kind switch
	// below would match nothing and the parent would emit an empty pair with
	// no error — the silent-wrong-bytes case the xml.Marshaler guard exists to
	// prevent. The two functions must agree on what counts as a child.
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}

	// Check if the value implements BuilderMarshaler (check pointer receiver too)
	if val.CanAddr() && val.Addr().Type().Implements(builderMarshalerType) {
		val.Addr().Interface().(BuilderMarshaler).MarshalToBuilder(b, ns, localName)
		return
	}
	if val.Type().Implements(builderMarshalerType) {
		val.Interface().(BuilderMarshaler).MarshalToBuilder(b, ns, localName)
		return
	}

	// A type that implements the stdlib xml.Marshaler but not BuilderMarshaler
	// would fall through to reflection below, silently discarding its
	// MarshalXML. Refuse to emit likely-wrong bytes; the caller must implement
	// MarshalToBuilder. (Types marshaled through the stdlib encoder elsewhere
	// never reach this path, so this stays inert for them.)
	if b.err == nil {
		var mt reflect.Type
		if val.CanAddr() {
			mt = val.Addr().Type()
		}
		if (mt != nil && mt.Implements(xmlMarshalerType)) || val.Type().Implements(xmlMarshalerType) {
			b.err = fmt.Errorf("xml: type %s implements xml.Marshaler but not xmlb.BuilderMarshaler; "+
				"the Builder cannot marshal it (implement MarshalToBuilder)", val.Type())
			return
		}
	}

	switch val.Kind() {
	case reflect.Struct:
		if val.Type() == xmlNameType {
			return
		}

		attrs := b.collectStructAttrs(val)
		// Collect namespace declarations needed for this element and its attributes
		attrs = b.prependNamespaceDecls(ns, attrs)
		if b.hasStructChildren(ns, val) {
			b.StartElement(ns, localName, attrs...)
			b.marshalStructChildren(ns, val)
			b.EndElement(ns, localName)
		} else {
			// A struct may carry a `CapturedEmptyTag EmptyTagStyle` field
			// recording whether the source wrote <name/> or <name></name>;
			// producers mix both forms within one part.
			b.EmptyElementStyled(capturedEmptyTagOf(val), ns, localName, attrs...)
		}

	case reflect.Slice:
		// Byte slices are handled by innerxml, not here
		if val.Type().Elem().Kind() == reflect.Uint8 {
			return
		}
		for i := 0; i < val.Len(); i++ {
			b.marshalReflect(ns, localName, val.Index(i))
		}

	case reflect.String:
		// Scalars need the same inline namespace declarations as structs:
		// an element in a namespace not declared at the root would otherwise
		// be emitted with an unbound prefix.
		b.WriteElement(ns, localName, val.String(), b.prependNamespaceDecls(ns, nil)...)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.WriteElement(ns, localName, fmt.Sprintf("%d", val.Int()), b.prependNamespaceDecls(ns, nil)...)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		b.WriteElement(ns, localName, fmt.Sprintf("%d", val.Uint()), b.prependNamespaceDecls(ns, nil)...)

	case reflect.Bool:
		content := "0"
		if val.Bool() {
			content = "1"
		}
		b.WriteElement(ns, localName, content, b.prependNamespaceDecls(ns, nil)...)
	}
}

// rootAttrSliceType identifies the conventional CapturedAttrs field.
var rootAttrSliceType = reflect.TypeOf([]RootAttr(nil))

// emptyTagStyleType identifies the conventional CapturedEmptyTag field.
var emptyTagStyleType = reflect.TypeOf(EmptyTagStyle(0))

// capturedEmptyTagOf returns the struct's CapturedEmptyTag field value, or
// EmptyTagUnknown when the struct does not carry one.
func capturedEmptyTagOf(val reflect.Value) EmptyTagStyle {
	f, ok := val.Type().FieldByName("CapturedEmptyTag")
	if !ok || f.Type != emptyTagStyleType || len(f.Index) != 1 {
		return EmptyTagUnknown
	}
	return val.Field(f.Index[0]).Interface().(EmptyTagStyle)
}

// collectStructAttrs collects all attribute fields from a struct as Attr
// values. A struct may carry a `CapturedAttrs []RootAttr` field (tagged
// xml:"-") holding the verbatim source attribute list; when non-nil it is
// replayed via ReplayCapturedAttrs so the source's attribute order, inline
// xmlns declarations, and unmodeled attributes survive the round trip.
//
// Attributes an omitempty tag suppresses are not silently discarded: they are
// collected separately and handed to replay as the "cleared" list, so a modeled
// field the caller reset to its zero value can actually delete the source's
// attribute instead of having it replayed back (C586, tension T-D). See
// ReplayCapturedAttrsClearing for why that is sound.
func (b *Builder) collectStructAttrs(val reflect.Value) []Attr {
	var attrs []Attr
	var cleared []ClearedAttr
	var captured []RootAttr
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		if field.Name == "CapturedAttrs" && field.Type == rootAttrSliceType {
			if fv := val.Field(i); !fv.IsNil() {
				captured = fv.Interface().([]RootAttr)
			}
			continue
		}

		tag := field.Tag.Get("xml")
		info := parseTag(tag, field.Name)

		if info.skip || !info.attr {
			continue
		}

		fval := val.Field(i)
		// A nil pointer has no value to emit: omit the attribute even without
		// omitempty, matching encoding/xml (previously the Builder diverged
		// and emitted attr="").
		//
		// A nil pointer is deliberately *not* reported as cleared. Parsing sets
		// the pointer whenever the source carried the attribute, so nil next to
		// a captured attribute means the model never read it — "we hold no
		// value", not "the caller cleared it". This is what keeps the XSD
		// default-TRUE attributes modeled as *bool (C526) safe.
		if fval.Kind() == reflect.Pointer && fval.IsNil() {
			continue
		}
		if info.omitempty && isZeroValue(fval) {
			if z, ok := clearedAttr(info, fval); ok {
				cleared = append(cleared, z)
			}
			continue
		}

		// An attribute type implementing the stdlib xml.MarshalerAttr but not
		// the Builder's AttrValuer would be formatted by formatValue's generic
		// fallback, ignoring its MarshalXMLAttr. Fail loudly (see marshalReflect).
		if b.err == nil && implementsMarshalerAttr(fval) && !implementsAttrValuer(fval) {
			b.err = fmt.Errorf("xml: attribute type %s implements xml.MarshalerAttr but not xmlb.AttrValuer; "+
				"the Builder cannot marshal it (implement AttrValue/IsZeroAttr)", fval.Type())
			continue
		}

		attrs = append(attrs, Attr{
			Namespace: info.ns,
			Name:      info.name,
			Value:     formatValue(fval),
			Numeric:   isNumericAttr(fval),
		})
	}

	if captured != nil {
		return b.ReplayCapturedAttrsClearing(captured, attrs, cleared)
	}
	return attrs
}

// lexicalSpaceOf maps a field kind to the lexical space its attribute values
// are drawn from, reporting ok=false for kinds replay cannot reason about.
func lexicalSpaceOf(k reflect.Kind) (AttrLexicalSpace, bool) {
	switch k {
	case reflect.String:
		return AttrLexText, true
	case reflect.Bool:
		return AttrLexBool, true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return AttrLexInt, true
	case reflect.Float32, reflect.Float64:
		return AttrLexFloat, true
	}
	return 0, false
}

// isNumericAttr reports whether v renders as a number. A type that renders its
// own lexical form (AttrValuer) is excluded: its AttrValue is authoritative and
// must not be second-guessed against a captured spelling.
func isNumericAttr(v reflect.Value) bool {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	if implementsAttrValuer(v) {
		return false
	}
	space, ok := lexicalSpaceOf(v.Kind())
	return ok && (space == AttrLexInt || space == AttrLexFloat)
}

// clearedAttr describes an omitempty-suppressed field to replay: the attribute
// name it would have been written under and the lexical space its values come
// from. Replay uses the lexical space to decide whether a captured value
// denotes the zero the model now holds.
//
// Only the plain scalar kinds qualify. A pointer never reaches here (nil is
// handled earlier and a non-nil pointer is not zero), and a type that renders
// its own lexical form (AttrValuer) is excluded because its lexical space is
// its own business — for those the captured value is kept, which is the
// pre-existing behaviour.
func clearedAttr(info tagInfo, fval reflect.Value) (ClearedAttr, bool) {
	if implementsAttrValuer(fval) {
		return ClearedAttr{}, false
	}
	space, ok := lexicalSpaceOf(fval.Kind())
	if !ok {
		return ClearedAttr{}, false
	}
	return ClearedAttr{Namespace: info.ns, Name: info.name, Lexical: space}, true
}

// implementsMarshalerAttr reports whether v (or its addressable pointer)
// implements the stdlib xml.MarshalerAttr interface.
func implementsMarshalerAttr(v reflect.Value) bool {
	if v.Type().Implements(xmlMarshalerAttrType) {
		return true
	}
	return v.CanAddr() && v.Addr().Type().Implements(xmlMarshalerAttrType)
}

// implementsAttrValuer reports whether v (or its addressable pointer)
// implements the Builder's AttrValuer interface.
func implementsAttrValuer(v reflect.Value) bool {
	if v.Type().Implements(attrValuerType) {
		return true
	}
	return v.CanAddr() && v.Addr().Type().Implements(attrValuerType)
}

// renderedAttrName returns the name an attribute is written with: the
// registered prefix joined to the local name, or the local name alone.
func (b *Builder) renderedAttrName(a Attr) string {
	if a.Namespace == "" {
		return a.Name
	}
	if prefix, ok := b.namespaces[a.Namespace]; ok && prefix != "" {
		return prefix + ":" + a.Name
	}
	return a.Name
}

// ReplayCapturedAttrs merges a captured source attribute list with the
// modeled attributes derived from struct fields. It is
// ReplayCapturedAttrsClearing with no cleared attributes.
func (b *Builder) ReplayCapturedAttrs(captured []RootAttr, modeled []Attr) []Attr {
	return b.ReplayCapturedAttrsClearing(captured, modeled, nil)
}

// ReplayCapturedAttrsClearing merges a captured source attribute list with the
// modeled attributes derived from struct fields, preserving fidelity on a
// no-op round trip while keeping model edits authoritative:
//
//   - captured entries are emitted in source order;
//   - a captured declaration (xmlns / xmlns:prefix) is emitted verbatim;
//   - a captured attribute that matches a modeled one (by rendered name)
//     takes the modeled value, so post-parse mutations win — but when the two
//     differ only in lexical form (a boolean written "false" vs "0", a number
//     written "1.0" vs "1" or "-4.9989318521683403E-2" vs its decimal
//     expansion) the producer's spelling is kept, since nothing changed;
//   - a captured attribute with no modeled match is emitted with its captured
//     value — this keeps unmodeled attributes and explicit zero values that
//     omitempty fields cannot represent;
//   - modeled attributes absent from the capture (set after parse) follow in
//     model order.
//
// cleared holds the modeled attributes an omitempty tag suppressed, each
// naming the lexical space its field's values come from. A captured attribute
// matching one of them is dropped when the captured value denotes a value other
// than the zero the model now holds, and kept otherwise.
//
// That test is what distinguishes "the caller cleared this" from "the producer
// wrote the zero explicitly", which the field alone cannot express: a Go bool
// that is false because nobody touched it and one that was just reset are the
// same bit. The capture supplies the missing half. It is a snapshot of what the
// parse read, so a model that disagrees with it can only have been changed
// after parse — and a model that agrees with it was not. Concretely:
//
//   - captured firstRow="1", model false  → the parse produced true, so the
//     false came from a setter: drop the attribute (C583).
//   - captured firstRow="0", model false  → the parse produced false and the
//     model still says false: nothing was cleared, replay verbatim. This is the
//     explicit-zero preservation the capture convention exists for, and it is
//     load-bearing for every attribute whose XSD default is not the Go zero
//     (showComments="0" means false against a default of true; spokes="0"
//     against a default of 4).
//   - captured idx="3", model 0 → drop (C585); captured name="x", model ""
//     → drop (C584).
//
// The inference has one precondition: parsing must populate the modeled field
// from the attribute it captured. Where it provably did not, the captured entry
// is kept. Two mechanisms enforce that, and both matter in practice:
//
//   - Matching is namespace-aware. A docx field tagged with the full
//     WordprocessingML URI is only filled by the stdlib from an attribute in
//     that namespace, so a producer's *unprefixed* color="FFFFFF" leaves the
//     field zero; it also fails to pair with the cleared entry, and survives.
//   - The captured value must be readable in the field's own lexical space. The
//     DrawingML readers degrade a malformed integer to zero on purpose and rely
//     on the capture to carry the original (roundInt64, coerceIntAttrs), so
//     rot="0.4" against an int32 field is not an integer lexeme, is not
//     evidence of a clear, and is replayed verbatim.
//
// Every undecidable case resolves toward keeping the capture, so this can miss
// a clear but never invent one: the failure mode is a setter that does not take
// effect, not deleted content.
//
// The returned attributes carry literal names (Namespace empty), ready for
// StartElement/EmptyElement.
func (b *Builder) ReplayCapturedAttrsClearing(captured []RootAttr, modeled []Attr, cleared []ClearedAttr) []Attr {
	used := make([]bool, len(modeled))
	rendered := make([]string, len(modeled))
	for i, a := range modeled {
		rendered[i] = b.renderedAttrName(a)
	}
	usedCleared := make([]bool, len(cleared))
	clearedNames := make([]string, len(cleared))
	for i, c := range cleared {
		clearedNames[i] = b.renderedAttrName(Attr{Namespace: c.Namespace, Name: c.Name})
	}
	out := make([]Attr, 0, len(captured)+len(modeled))
	for _, ra := range captured {
		lit := ra.attr()
		if ra.IsNS {
			out = append(out, lit)
			continue
		}
		matched := false
		for i, rn := range rendered {
			// Match by rendered name, or by namespace + local name when the
			// capture could not resolve a prefix (declared on an ancestor).
			if !used[i] && (rn == lit.Name ||
				(ra.Space == modeled[i].Namespace && ra.LocalName == modeled[i].Name)) {
				a := Attr{Name: lit.Name, Value: modeled[i].Value, Raw: lit.Raw}
				if modeled[i].Value != ra.Value {
					if boolLexEquivalent(modeled[i].Value, ra.Value) ||
						(modeled[i].Numeric && numLexEquivalent(modeled[i].Value, ra.Value)) {
						// Same value, different lexical form: keep the
						// producer's spelling rather than renormalizing it.
						a.Value = ra.Value
					} else {
						// The model changed the value: the verbatim source
						// rendering is stale, re-render normally.
						a.Raw = ""
					}
				}
				out = append(out, a)
				used[i] = true
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// No modeled attribute is being written under this name. It may still
		// be modeled and merely suppressed as a zero — in which case the model
		// deliberately says "not present" and the capture must not resurrect it.
		dropped := false
		for i, cn := range clearedNames {
			if usedCleared[i] {
				continue
			}
			// Same match rule as the modeled pass: rendered name, or namespace
			// plus local name when the capture could not resolve a prefix.
			if cn != lit.Name &&
				(ra.Space != cleared[i].Namespace || ra.LocalName != cleared[i].Name) {
				continue
			}
			usedCleared[i] = true
			dropped = cleared[i].Lexical.differsFromZero(ra.Value)
			break
		}
		if !dropped {
			out = append(out, lit)
		}
	}
	for i, a := range modeled {
		if !used[i] {
			out = append(out, a)
		}
	}
	return out
}

// AttrLexicalSpace identifies the lexical space a modeled attribute's values
// are drawn from. It lets replay read a captured value the way the parse would
// have read it, which is what makes "the model no longer agrees with the
// source" a sound signal (see ReplayCapturedAttrsClearing).
type AttrLexicalSpace uint8

const (
	// AttrLexText is an attribute backed by a string field: any character
	// sequence is a value, and only "" is the zero.
	AttrLexText AttrLexicalSpace = iota
	// AttrLexBool is an xsd:boolean attribute ("true", "false", "1", "0").
	AttrLexBool
	// AttrLexInt is an integer-typed attribute. A fractional lexeme is
	// deliberately *not* in this space: the DrawingML readers round such a
	// value into the field and rely on the capture to carry the original.
	AttrLexInt
	// AttrLexFloat is a floating-point (xsd:double) attribute.
	AttrLexFloat
)

// ClearedAttr names a modeled attribute that is currently suppressed because
// its field holds the zero value, together with the lexical space of that
// field. Replay uses it to tell a deliberate clear from an explicitly-written
// zero; see ReplayCapturedAttrsClearing.
type ClearedAttr struct {
	Namespace string // namespace URI (empty for no namespace)
	Name      string // local name
	Lexical   AttrLexicalSpace
}

// differsFromZero reports whether a captured attribute value denotes something
// other than the zero of a field in this lexical space. True means the model
// must have been changed after parse, so the captured attribute is stale.
//
// It answers false whenever the captured value is not a member of the space:
// such a value cannot have produced the field's current zero by an ordinary
// parse, so the field was either never populated from it or was leniently
// coerced, and "the caller cleared it" would be an unfounded conclusion.
// Keeping the attribute is the pre-existing behaviour and the safe direction —
// a missed clear is a setter that does not take effect, a wrong drop is data
// loss.
func (s AttrLexicalSpace) differsFromZero(capturedValue string) bool {
	switch s {
	case AttrLexText:
		return capturedValue != ""
	case AttrLexBool:
		v, ok := parseXSDBool(capturedValue)
		return ok && v
	case AttrLexInt:
		n, err := strconv.ParseInt(strings.TrimSpace(capturedValue), 10, 64)
		if err != nil {
			// Not an integer lexeme. Unsigned fields can hold values above
			// MaxInt64, so try that spelling too before giving up.
			u, uerr := strconv.ParseUint(strings.TrimSpace(capturedValue), 10, 64)
			return uerr == nil && u != 0
		}
		return n != 0
	case AttrLexFloat:
		f, err := strconv.ParseFloat(strings.TrimSpace(capturedValue), 64)
		return err == nil && f != 0
	}
	return false
}

// parseXSDBool parses an xsd:boolean lexical value. The canonical forms are
// "true", "false", "1" and "0"; the mixed-case spellings some producers emit
// are accepted too, since the stdlib decoder also accepts them and so the
// modeled field would have been populated from them.
func parseXSDBool(v string) (bool, bool) {
	switch v {
	case "1", "true", "True", "TRUE":
		return true, true
	case "0", "false", "False", "FALSE":
		return false, true
	}
	return false, false
}

// boolLexEquivalent reports whether two attribute values are the same
// xsd:boolean under different lexical forms ("0"/"false", "1"/"true").
func boolLexEquivalent(a, b string) bool {
	norm := func(v string) string {
		switch v {
		case "0", "false":
			return "0"
		case "1", "true":
			return "1"
		}
		return v
	}
	na, nb := norm(a), norm(b)
	return na == nb && (na == "0" || na == "1")
}

// numLexEquivalent reports whether two attribute values are the same number
// written differently ("1" vs "1.0", "7" vs "007", a decimal expansion vs the
// E-notation the producer used). Callers must only apply it to attributes whose
// modeled field is numeric (Attr.Numeric): a string-valued attribute whose
// contents merely look like numbers keeps its exact spelling.
func numLexEquivalent(a, b string) bool {
	fa, err := strconv.ParseFloat(a, 64)
	if err != nil {
		return false
	}
	fb, err := strconv.ParseFloat(b, 64)
	if err != nil {
		return false
	}
	return fa == fb
}

// hasStructChildren reports whether a struct has any non-empty child elements to write.
func (b *Builder) hasStructChildren(parentNS string, val reflect.Value) bool {
	// Raw captured children (unmodeled elements, duplicated singletons) are
	// emitted by marshalCapturedChildren even when every typed field is empty.
	if hasCapturedRawChildren(val) {
		return true
	}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		// Skip XMLName
		if field.Type == xmlNameType {
			continue
		}

		tag := field.Tag.Get("xml")
		info := parseTag(tag, field.Name)

		if info.skip || info.attr || info.isAny {
			continue
		}

		fval := val.Field(i)

		if info.innerxml {
			if fval.Kind() == reflect.Slice && fval.Type().Elem().Kind() == reflect.Uint8 && fval.Len() > 0 {
				return true
			}
			continue
		}

		if info.chardata {
			// Mirror marshalStructChildren: a non-string chardata field always
			// emits content (even a zero), an empty string does not. omitempty
			// is checked here since it follows this branch.
			if info.omitempty && isZeroValue(fval) {
				continue
			}
			if fval.Kind() != reflect.String || fval.Len() > 0 {
				return true
			}
			continue
		}

		if info.omitempty && isZeroValue(fval) {
			continue
		}

		// A field not skipped by omitempty is written by marshalStructChildren
		// unless it is an absent pointer/interface/collection. In particular a
		// mandatory zero-valued scalar or struct still produces an element, so
		// the parent must not self-close (previously such a child was dropped).
		switch fval.Kind() {
		case reflect.Pointer, reflect.Interface:
			if fval.IsNil() {
				continue
			}
		case reflect.Slice, reflect.Map:
			if fval.Len() == 0 {
				continue
			}
		}
		return true
	}

	return false
}

// marshalStructChildren marshals all child element fields of a struct.
func (b *Builder) marshalStructChildren(parentNS string, val reflect.Value) {
	// A struct may carry a `CapturedChildren *ChildCapture` field (tagged
	// xml:"-") recording the source's child sequence; when set it is replayed
	// so child order, unmodeled children, and duplicated singletons survive
	// the round trip (see children.go).
	if cc := capturedChildrenOf(val); cc != nil {
		b.marshalCapturedChildren(parentNS, val, cc)
		return
	}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		// Skip XMLName
		if field.Type == xmlNameType {
			continue
		}

		tag := field.Tag.Get("xml")
		info := parseTag(tag, field.Name)

		if info.skip || info.attr || info.isAny {
			continue
		}

		fval := val.Field(i)

		if info.omitempty && isZeroValue(fval) {
			continue
		}

		// innerxml: write raw bytes through WriteRaw so trailingWS / empty-write
		// handling matches every other raw-write site (avoids a stray separator
		// after raw content in separator mode).
		if info.innerxml {
			if fval.Kind() == reflect.Slice && fval.Type().Elem().Kind() == reflect.Uint8 && fval.Len() > 0 {
				b.WriteRaw(fval.Bytes())
			}
			continue
		}

		// chardata: write escaped text. A non-string field always has content
		// (an int 0 must emit "0", not vanish); an empty string genuinely has
		// none. flushOpenTag first so the text isn't fused into a deferred
		// start tag under collapse-empty. omitempty was already handled above.
		if info.chardata {
			if fval.Kind() == reflect.String {
				if fval.Len() > 0 {
					b.flushOpenTag()
					b.writeTextEscaped(fval.String())
				}
			} else {
				b.flushOpenTag()
				b.writeTextEscaped(formatValue(fval))
			}
			continue
		}

		// Determine element namespace
		elemNS := info.ns
		if elemNS == "" {
			elemNS = parentNS
		}

		// Handle slices (non-byte): marshal each element with the same name
		if fval.Kind() == reflect.Slice && fval.Type().Elem().Kind() != reflect.Uint8 {
			for j := 0; j < fval.Len(); j++ {
				b.marshalReflect(elemNS, info.name, fval.Index(j))
			}
			continue
		}

		// Marshal as element
		b.marshalReflect(elemNS, info.name, fval)
	}
}
