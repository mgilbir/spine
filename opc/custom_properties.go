package opc

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// nsCustomProperties is the XML namespace of docProps/custom.xml. The typed
// values inside each <property> use the docPropsVTypes namespace (nsDocPropsVTypes,
// declared with the extended properties).
const nsCustomProperties = "http://schemas.openxmlformats.org/officeDocument/2006/custom-properties"

// CustomPropertiesFmtID is the format identifier (a fixed GUID) that every
// user-defined document property carries. Office writes it verbatim on each
// <property> element and it is the value used for properties created through
// this API.
const CustomPropertiesFmtID = "{D5CDD505-2E9C-101B-9397-08002B2CF9AE}"

// customPropertyFirstPID is the property id Office assigns to the first custom
// property; ids increase from there. pid 0 and 1 are reserved.
const customPropertyFirstPID = 2

// customProperty is a single user-defined document property.
type customProperty struct {
	// fmtid is the property's format identifier GUID. Preserved from the source
	// (properties created programmatically use CustomPropertiesFmtID).
	fmtid string

	// pid is the property id, unique within the part.
	pid int

	// name is the human-readable property name.
	name string

	// value holds the typed value: string, int64, float64, bool, or time.Time.
	// It is nil when the source used a variant type this package does not model;
	// rawVT then preserves the original child XML verbatim.
	value any

	// rawVT holds the verbatim inner XML of the <property> element (its <vt:*>
	// child) when value is nil, so regenerating custom.xml re-emits an
	// unmodeled variant type unchanged instead of dropping it.
	rawVT string
}

// clone returns a deep copy of the property.
func (p *customProperty) clone() *customProperty {
	c := *p
	return &c
}

// valueEqual reports whether two property values are equal, comparing time
// values with time.Time.Equal.
func valueEqual(a, b any) bool {
	if at, ok := a.(time.Time); ok {
		bt, ok := b.(time.Time)
		return ok && at.Equal(bt)
	}
	return a == b
}

// CustomProperties models the user-defined document properties stored in
// docProps/custom.xml. Values are one of string, int64, float64, bool, or
// time.Time. The zero value is an empty, ready-to-use set.
type CustomProperties struct {
	props []*customProperty
}

// Len returns the number of properties.
func (cp *CustomProperties) Len() int {
	if cp == nil {
		return 0
	}
	return len(cp.props)
}

// find returns the property with the given name (case-sensitive, matching how
// Office treats property names) and its index, or nil and -1.
func (cp *CustomProperties) find(name string) (*customProperty, int) {
	for i, p := range cp.props {
		if p.name == name {
			return p, i
		}
	}
	return nil, -1
}

// Get returns the typed value of the named property. The boolean is false when
// no such property exists or it carries an unmodeled variant type.
func (cp *CustomProperties) Get(name string) (any, bool) {
	if cp == nil {
		return nil, false
	}
	if p, _ := cp.find(name); p != nil && p.value != nil {
		return p.value, true
	}
	return nil, false
}

// Names returns the property names in document order.
func (cp *CustomProperties) Names() []string {
	if cp == nil {
		return nil
	}
	names := make([]string, len(cp.props))
	for i, p := range cp.props {
		names[i] = p.name
	}
	return names
}

// AsMap returns the modeled properties as a name→value map. Properties with an
// unmodeled variant type are omitted (they are still preserved on save).
func (cp *CustomProperties) AsMap() map[string]any {
	if cp == nil || len(cp.props) == 0 {
		return nil
	}
	m := make(map[string]any, len(cp.props))
	for _, p := range cp.props {
		if p.value != nil {
			m[p.name] = p.value
		}
	}
	return m
}

// nextPID returns an unused property id: one past the current maximum, never
// below customPropertyFirstPID.
func (cp *CustomProperties) nextPID() int {
	max := customPropertyFirstPID - 1
	for _, p := range cp.props {
		if p.pid > max {
			max = p.pid
		}
	}
	return max + 1
}

// normalizeCustomValue coerces a supported Go value to the canonical type
// stored in a property (int/int32 → int64, float32 → float64), rejecting
// unsupported types.
func normalizeCustomValue(v any) (any, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		return x, nil
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case float32:
		return float64(x), nil
	case float64:
		return x, nil
	case time.Time:
		return x, nil
	default:
		return nil, fmt.Errorf("opc: unsupported custom property value type %T (want string, int64, float64, bool, or time.Time)", v)
	}
}

// Set adds or replaces the named property. Replacing keeps the existing pid and
// fmtid; a new property is appended with the next free pid and the standard
// custom-properties fmtid. The value must be a string, int/int32/int64,
// float32/float64, bool, or time.Time.
//
// Reader.CustomProperties is documented as nil when the package carries no
// custom properties, so nil is the state a caller is most likely to hold when
// reaching for Set. A nil receiver cannot be appended to, so it reports an
// error naming the fix rather than panicking as it used to (C455); every other
// method on the type nil-guards.
func (cp *CustomProperties) Set(name string, value any) error {
	if cp == nil {
		return fmt.Errorf("opc: Set on a nil *CustomProperties; allocate one first (&opc.CustomProperties{} is a ready-to-use empty set)")
	}
	if name == "" {
		return fmt.Errorf("opc: custom property name must not be empty")
	}
	nv, err := normalizeCustomValue(value)
	if err != nil {
		return err
	}
	if p, _ := cp.find(name); p != nil {
		p.value = nv
		p.rawVT = ""
		return nil
	}
	cp.props = append(cp.props, &customProperty{
		fmtid: CustomPropertiesFmtID,
		pid:   cp.nextPID(),
		name:  name,
		value: nv,
	})
	return nil
}

// Remove deletes the named property, reporting whether it existed.
func (cp *CustomProperties) Remove(name string) bool {
	if cp == nil {
		return false
	}
	if _, i := cp.find(name); i >= 0 {
		cp.props = slices.Delete(cp.props, i, i+1)
		return true
	}
	return false
}

// Clone returns a deep copy of cp; mutating the clone never affects the
// original.
func (cp *CustomProperties) Clone() *CustomProperties {
	if cp == nil {
		return nil
	}
	c := &CustomProperties{props: make([]*customProperty, len(cp.props))}
	for i, p := range cp.props {
		c.props[i] = p.clone()
	}
	return c
}

// Equal reports whether cp and o hold the same properties in the same order,
// comparing values by type (time values via time.Time.Equal). It is used to
// detect edits made after a package was opened.
func (cp *CustomProperties) Equal(o *CustomProperties) bool {
	if cp == nil || o == nil {
		return cp.Len() == o.Len()
	}
	if len(cp.props) != len(o.props) {
		return false
	}
	for i, p := range cp.props {
		q := o.props[i]
		if p.name != q.name || p.pid != q.pid || p.fmtid != q.fmtid || p.rawVT != q.rawVT {
			return false
		}
		if !valueEqual(p.value, q.value) {
			return false
		}
	}
	return true
}

// marshalVTValue writes the typed <vt:*> child for a property value. When value
// is nil the preserved raw child XML is emitted verbatim.
func marshalVTValue(b *strings.Builder, value any, rawVT string) {
	switch x := value.(type) {
	case nil:
		b.WriteString(rawVT)
	case string:
		b.WriteString("<vt:lpwstr>")
		b.WriteString(xmlEscape(x))
		b.WriteString("</vt:lpwstr>")
	case bool:
		b.WriteString("<vt:bool>")
		b.WriteString(xmlBool(x))
		b.WriteString("</vt:bool>")
	case int64:
		if x >= math.MinInt32 && x <= math.MaxInt32 {
			b.WriteString("<vt:i4>")
			b.WriteString(strconv.FormatInt(x, 10))
			b.WriteString("</vt:i4>")
		} else {
			b.WriteString("<vt:i8>")
			b.WriteString(strconv.FormatInt(x, 10))
			b.WriteString("</vt:i8>")
		}
	case float64:
		b.WriteString("<vt:r8>")
		b.WriteString(xmlb.FormatFloat(x))
		b.WriteString("</vt:r8>")
	case time.Time:
		b.WriteString("<vt:filetime>")
		b.WriteString(x.UTC().Format(time.RFC3339))
		b.WriteString("</vt:filetime>")
	}
}

// Marshal serializes the properties to docProps/custom.xml bytes. The output
// matches Microsoft Office's compact style: a CRLF after the XML declaration
// and no whitespace between <property> elements. Regeneration only happens when
// the set was modified; an unmodified part round-trips as its preserved source
// bytes, so this need not reproduce every producer's formatting.
//
// A nil receiver marshals to an empty but well-formed part rather than
// panicking, matching the nil-guarded readers on this type (C455).
func (cp *CustomProperties) Marshal() ([]byte, error) {
	var props []*customProperty
	if cp != nil {
		props = cp.props
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\r\n")
	b.WriteString(`<Properties xmlns="` + nsCustomProperties + `"`)
	b.WriteString(` xmlns:vt="` + nsDocPropsVTypes + `">`)
	for _, p := range props {
		b.WriteString(`<property fmtid="`)
		b.WriteString(xmlb.EscapeAttrValue(p.fmtid))
		b.WriteString(`" pid="`)
		b.WriteString(strconv.Itoa(p.pid))
		b.WriteString(`" name="`)
		b.WriteString(xmlb.EscapeAttrValue(p.name))
		b.WriteString(`">`)
		marshalVTValue(&b, p.value, p.rawVT)
		b.WriteString("</property>")
	}
	b.WriteString("</Properties>")
	return []byte(b.String()), nil
}

// parseVTScalar converts the text of a known scalar variant element to a typed
// value. It reports false when the text does not parse as its declared type, so
// a malformed value degrades to verbatim preservation rather than failing the
// whole part.
func parseVTScalar(local, text string) (any, bool) {
	t := strings.TrimSpace(text)
	switch local {
	case "lpwstr", "lpstr", "bstr":
		// Strings preserve their exact text (including surrounding spaces).
		return text, true
	case "i1", "i2", "i4", "int", "i8", "ui1", "ui2", "ui4", "uint", "ui8":
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return n, true
		}
	case "r4", "r8", "decimal":
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f, true
		}
	case "bool":
		switch t {
		case "true", "1":
			return true, true
		case "false", "0":
			return false, true
		}
	case "filetime", "date":
		if tm, ok := parseW3CDTF(t); ok {
			return tm, true
		}
	}
	return nil, false
}

// UnmarshalCustomProperties parses docProps/custom.xml into a CustomProperties.
// It is best-effort like the core/extended property parsers: an unmodeled
// variant type is preserved verbatim (so a modify-and-save re-emits it
// unchanged) and a value that does not parse as its declared type is likewise
// kept as raw XML rather than failing the open.
func UnmarshalCustomProperties(data []byte) (*CustomProperties, error) {
	cp := &CustomProperties{}
	src := string(data)
	decoder := xmlb.NewDecoder(strings.NewReader(src))
	var inRoot bool

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !inRoot {
			// The root must actually be <Properties>. Accepting any root name
			// meant an arbitrary document parsed as custom properties while
			// the error below claimed to check for a missing Properties root —
			// a check that did not exist (C451). The namespace is not
			// required to match: the parse is best-effort and producers spell
			// it in several ways, but the element name is unambiguous.
			if start.Name.Local != "Properties" {
				return nil, &xml.SyntaxError{Msg: "missing custom Properties root element", Line: 1}
			}
			inRoot = true
			continue
		}
		if start.Name.Local != "property" {
			if err := decoder.Skip(); err != nil {
				return nil, err
			}
			continue
		}

		prop := &customProperty{fmtid: CustomPropertiesFmtID, pid: cp.nextPID()}
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "fmtid":
				prop.fmtid = attr.Value
			case "pid":
				if n, err := strconv.Atoi(strings.TrimSpace(attr.Value)); err == nil {
					prop.pid = n
				}
			case "name":
				prop.name = attr.Value
			}
		}

		if err := decodePropertyValue(decoder, src, prop); err != nil {
			return nil, err
		}
		cp.props = append(cp.props, prop)
	}

	if !inRoot {
		return nil, &xml.SyntaxError{Msg: "missing custom Properties root element", Line: 1}
	}
	return cp, nil
}

// knownVTScalars is the set of variant element local names decodePropertyValue
// maps onto a typed Go value; any other child is preserved as raw XML.
var knownVTScalars = map[string]bool{
	"lpwstr": true, "lpstr": true, "bstr": true,
	"i1": true, "i2": true, "i4": true, "int": true, "i8": true,
	"ui1": true, "ui2": true, "ui4": true, "uint": true, "ui8": true,
	"r4": true, "r8": true, "decimal": true,
	"bool": true, "filetime": true, "date": true,
}

// decodePropertyValue reads the child of a <property> element, setting the
// property's typed value for a known scalar variant or preserving the child's
// raw XML for an unmodeled (or unparseable) type. The decoder is positioned
// just after the property's start tag; on return it has consumed the
// property's matching end tag.
func decodePropertyValue(decoder *xml.Decoder, src string, prop *customProperty) error {
	for {
		childStart := decoder.InputOffset()
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.EndElement:
			// </property> reached with no recognized child value.
			return nil
		case xml.StartElement:
			if knownVTScalars[t.Name.Local] {
				var text string
				if err := decoder.DecodeElement(&text, &t); err != nil {
					return err
				}
				if v, ok := parseVTScalar(t.Name.Local, text); ok {
					prop.value = v
				} else {
					// Declared type but unparseable text: preserve verbatim.
					prop.value = nil
					prop.rawVT = rebuildScalarElement(t.Name, text)
				}
			} else {
				// Unmodeled variant type (vt:vector, vt:cy, …): capture the
				// child's bytes verbatim so regeneration re-emits it unchanged.
				if err := decoder.Skip(); err != nil {
					return err
				}
				prop.value = nil
				if end := decoder.InputOffset(); childStart >= 0 && int(end) <= len(src) && childStart <= end {
					prop.rawVT = src[childStart:end]
				}
			}
			// Consume the property's end tag (and any trailing nodes).
			return decoder.Skip()
		}
	}
}

// rebuildScalarElement reconstructs a simple <vt:local>text</vt:local> element
// for a declared-but-unparseable scalar, so it survives regeneration.
func rebuildScalarElement(name xml.Name, text string) string {
	tag := "vt:" + name.Local
	return "<" + tag + ">" + xmlEscape(text) + "</" + tag + ">"
}
