package testutil

import (
	"encoding/xml"
	"io"
	"reflect"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// This file is the guard for C329. Structs that opt into the common/xml
// child-capture kit (a `CapturedChildren *xmlb.ChildCapture` field) replay the
// source child order verbatim and splice children set after parse in at their
// schema position — where "schema position" is the struct's *field declaration
// order*. That equivalence between declaration order and the XSD content model
// is an unwritten convention holding across four packages, and nothing enforced
// it: inserting a new field in the wrong place, or reordering two fields, would
// silently start emitting post-parse additions in schema-invalid positions.
//
// CheckSchemaChildOrder pins it from both sides — the declaration order against
// a table transcribed from the XSD, and the emitted order of an actual
// post-parse addition against that same table.

// ContentModel classifies a type's XSD content model, which decides whether
// child order is significant.
type ContentModel string

const (
	// Sequence marks an xsd:sequence: the schema fixes the child order, so a
	// child appended after every captured sibling is schema-invalid.
	Sequence ContentModel = "xsd:sequence"
	// Choice marks a repeated xsd:choice: every child order validates, so
	// rank insertion is a canonicalization rather than a correctness fix.
	// Types whose model is a choice followed by trailing sequence members
	// (CT_RPr's rPrChange, CT_Blip's extLst) are recorded as Sequence: the
	// tail is ordered, which is the part an appended child violates.
	Choice ContentModel = "xsd:choice"
)

// SchemaOrder pins one child-capture type's content model.
type SchemaOrder struct {
	// Name identifies the type in failure messages.
	Name string
	// New allocates a zero value of the type as a pointer.
	New func() any
	// Model is the type's XSD content model.
	Model ContentModel
	// Children lists the type's *modeled* element children in content-model
	// order. Children the type does not model (a trailing extLst, w:headers,
	// w:textboxTightWrap) are captured as raw bytes and keep their source
	// position, so they do not appear here; extension children the schema does
	// not define at all (mc:AlternateContent, w14:ligatures) are listed at the
	// slot the producer emits them in.
	Children []string
}

// CheckSchemaChildOrder asserts, for every case, that the struct's element
// field declaration order equals Children, and that a child added after parse
// is emitted at its Children position rather than after the captured siblings.
//
// newBuilder and ns supply the marshaling context (the namespace an untagged
// child field inherits from its parent).
func CheckSchemaChildOrder(t *testing.T, newBuilder func() *xmlb.Builder, ns string, cases []SchemaOrder) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			checkDeclarationOrder(t, tc)
			checkInsertionOrder(t, newBuilder, ns, tc)
		})
	}
}

// checkDeclarationOrder compares the struct's element fields, in declaration
// order, against the transcribed content model.
func checkDeclarationOrder(t *testing.T, tc SchemaOrder) {
	t.Helper()
	fields := elementFields(structTypeOf(t, tc))
	got := make([]string, len(fields))
	for i, f := range fields {
		got[i] = f.local
	}
	if strings.Join(got, ",") != strings.Join(tc.Children, ",") {
		t.Errorf("%s field declaration order does not match its XSD content model\n got: %v\nwant: %v\n"+
			"post-parse additions are placed by field index, so the declaration order must be the schema order",
			tc.Name, got, tc.Children)
	}
}

// checkInsertionOrder simulates the C329 shape: an instance parsed with only
// its last-ranked child present, then given its first-ranked child after parse.
// The addition must precede the captured child.
func checkInsertionOrder(t *testing.T, newBuilder func() *xmlb.Builder, ns string, tc SchemaOrder) {
	t.Helper()
	typ := structTypeOf(t, tc)
	fields := elementFields(typ)
	if len(fields) < 2 {
		return // nothing to order against
	}
	lo, hi := fields[0], fields[len(fields)-1]

	v := tc.New()
	rv := reflect.ValueOf(v).Elem()
	if !allocChild(rv.Field(hi.index)) || !setCapture(rv, hi.index) {
		t.Fatalf("%s: cannot synthesize a captured %s child", tc.Name, hi.local)
	}
	if !allocChild(rv.Field(lo.index)) {
		t.Fatalf("%s: cannot synthesize a post-parse %s child", tc.Name, lo.local)
	}

	b := newBuilder()
	b.MarshalElement(ns, "probe", v)
	names := topLevelChildren(t, string(b.Bytes()))

	iLo, iHi := indexOf(names, lo.local), indexOf(names, hi.local)
	if iLo < 0 || iHi < 0 {
		t.Fatalf("%s: probe did not emit both children (want %s and %s), got %v", tc.Name, lo.local, hi.local, names)
	}
	if iLo > iHi {
		t.Errorf("%s: post-parse <%s> emitted after captured <%s> (%v); it must be spliced at its schema position",
			tc.Name, lo.local, hi.local, names)
	}
}

func structTypeOf(t *testing.T, tc SchemaOrder) reflect.Type {
	t.Helper()
	typ := reflect.TypeOf(tc.New())
	if typ == nil || typ.Kind() != reflect.Pointer || typ.Elem().Kind() != reflect.Struct {
		t.Fatalf("%s: New must return a pointer to a struct, got %v", tc.Name, typ)
	}
	return typ.Elem()
}

// elementField is one child-element field of a capture-kit struct.
type elementField struct {
	index int
	local string
}

var xmlNameType = reflect.TypeOf(xml.Name{})

// elementFields returns the struct's child-element fields in declaration
// order, mirroring the marshaler's own field filter (attributes, chardata,
// innerxml, `,any` and XMLName carry no position in the content model).
func elementFields(typ reflect.Type) []elementField {
	var out []elementField
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() || f.Type == xmlNameType {
			continue
		}
		tag := f.Tag.Get("xml")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		skip := false
		for _, opt := range strings.Split(opts, ",") {
			switch opt {
			case "attr", "chardata", "innerxml", "any", "cdata", "comment":
				skip = true
			}
		}
		if skip {
			continue
		}
		if name == "" {
			name = f.Name
		}
		if i := strings.LastIndex(name, " "); i >= 0 {
			name = name[i+1:]
		}
		out = append(out, elementField{index: i, local: name})
	}
	return out
}

// allocChild gives a child field a value that marshals to an element.
func allocChild(fv reflect.Value) bool {
	switch fv.Kind() {
	case reflect.Pointer:
		fv.Set(reflect.New(fv.Type().Elem()))
		return true
	case reflect.Slice:
		et := fv.Type().Elem()
		if et.Kind() == reflect.Pointer {
			fv.Set(reflect.Append(fv, reflect.New(et.Elem())))
		} else {
			fv.Set(reflect.Append(fv, reflect.New(et).Elem()))
		}
		return true
	case reflect.String:
		fv.SetString("x")
		return true
	case reflect.Struct:
		return true // a value struct always emits
	default:
		return false
	}
}

// setCapture installs a capture recording exactly one child: field fieldIndex,
// standing in for a document parsed with that single child present.
func setCapture(rv reflect.Value, fieldIndex int) bool {
	f := rv.FieldByName("CapturedChildren")
	if !f.IsValid() || !f.CanSet() {
		return false
	}
	cc, ok := reflect.New(f.Type().Elem()).Interface().(*xmlb.ChildCapture)
	if !ok {
		return false
	}
	cc.Order = []xmlb.ChildRef{{Field: fieldIndex}}
	f.Set(reflect.ValueOf(cc))
	return true
}

// topLevelChildren returns the local names of the direct children of the
// outermost element of s.
func topLevelChildren(t *testing.T, s string) []string {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(s))
	var names []string
	depth := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatalf("decode probe output %q: %v", s, err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				names = append(names, tok.Name.Local)
			}
		case xml.EndElement:
			depth--
		}
	}
}

func indexOf(names []string, want string) int {
	for i, n := range names {
		if n == want {
			return i
		}
	}
	return -1
}
