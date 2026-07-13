package xml

import (
	"encoding/xml"
	"fmt"
	"reflect"
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
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", v.Float())
	default:
		return fmt.Sprint(v.Interface())
	}
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

	attrs := b.collectStructAttrs(val)
	attrs = append(extraAttrs, attrs...)

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
	// Dereference pointer
	for val.Kind() == reflect.Pointer {
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
			b.EmptyElement(ns, localName, attrs...)
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

// collectStructAttrs collects all attribute fields from a struct as Attr values.
func (b *Builder) collectStructAttrs(val reflect.Value) []Attr {
	var attrs []Attr
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
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
		if fval.Kind() == reflect.Pointer && fval.IsNil() {
			continue
		}
		if info.omitempty && isZeroValue(fval) {
			continue
		}

		attrs = append(attrs, Attr{
			Namespace: info.ns,
			Name:      info.name,
			Value:     formatValue(fval),
		})
	}

	return attrs
}

// hasStructChildren reports whether a struct has any non-empty child elements to write.
func (b *Builder) hasStructChildren(parentNS string, val reflect.Value) bool {
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
			if !isZeroValue(fval) {
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

		// innerxml: write raw bytes
		if info.innerxml {
			if fval.Kind() == reflect.Slice && fval.Type().Elem().Kind() == reflect.Uint8 && fval.Len() > 0 {
				b.flushOpenTag()
				b.buf.Write(fval.Bytes())
			}
			continue
		}

		// chardata: write escaped text
		if info.chardata {
			if !isZeroValue(fval) {
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
