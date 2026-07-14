package xml

import (
	"encoding/xml"
	"testing"
)

func TestMarshalElement_BasicStruct(t *testing.T) {
	type Inner struct {
		X int64 `xml:"x,attr"`
		Y int64 `xml:"y,attr"`
	}
	type Outer struct {
		Off *Inner `xml:"http://schemas.openxmlformats.org/drawingml/2006/main off,omitempty"`
		Ext *Inner `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext,omitempty"`
	}

	b := NewPresentationMLBuilder()
	v := &Outer{
		Off: &Inner{X: 100, Y: 200},
		Ext: &Inner{X: 300, Y: 400},
	}
	b.MarshalElement(NSDrawingML, "xfrm", v)
	got := b.String()
	// Verify it contains the key elements.
	if got == "" {
		t.Fatal("MarshalElement produced empty output")
	}
	if !contains(got, "<a:xfrm>") {
		t.Errorf("expected <a:xfrm> in output, got: %s", got)
	}
	if !contains(got, `<a:off x="100" y="200"/>`) {
		t.Errorf("expected <a:off ...> in output, got: %s", got)
	}
	if !contains(got, `<a:ext x="300" y="400"/>`) {
		t.Errorf("expected <a:ext ...> in output, got: %s", got)
	}
	if !contains(got, "</a:xfrm>") {
		t.Errorf("expected </a:xfrm> in output, got: %s", got)
	}
}

func TestMarshalElement_Omitempty(t *testing.T) {
	type Foo struct {
		Name  string `xml:"name,attr,omitempty"`
		Value string `xml:"value,attr,omitempty"`
	}

	b := NewPresentationMLBuilder()
	v := &Foo{Name: "test"} // Value is empty, should be omitted
	b.MarshalElement(NSPresentationML, "foo", v)
	got := b.String()
	if !contains(got, `name="test"`) {
		t.Errorf("expected name attr, got: %s", got)
	}
	if contains(got, "value") {
		t.Errorf("expected value attr to be omitted, got: %s", got)
	}
}

func TestMarshalElement_NestedNamespaces(t *testing.T) {
	type DMLChild struct {
		Val string `xml:"val,attr"`
	}
	type PMLParent struct {
		Child *DMLChild `xml:"http://schemas.openxmlformats.org/drawingml/2006/main child,omitempty"`
	}

	b := NewPresentationMLBuilder()
	v := &PMLParent{
		Child: &DMLChild{Val: "foo"},
	}
	b.MarshalElement(NSPresentationML, "parent", v)
	got := b.String()
	// Child should use DML namespace (a: prefix)
	if !contains(got, `<a:child val="foo"/>`) {
		t.Errorf("expected <a:child> with DML namespace, got: %s", got)
	}
	// Parent should use PML namespace (p: prefix)
	if !contains(got, `<p:parent>`) {
		t.Errorf("expected <p:parent> with PML namespace, got: %s", got)
	}
}

func TestMarshalElement_Slice(t *testing.T) {
	type Item struct {
		ID uint32 `xml:"id,attr"`
	}
	type Container struct {
		Items []*Item `xml:"item,omitempty"`
	}

	b := NewPresentationMLBuilder()
	v := &Container{
		Items: []*Item{{ID: 1}, {ID: 2}, {ID: 3}},
	}
	b.MarshalElement(NSPresentationML, "container", v)
	got := b.String()
	if !contains(got, `<p:item id="1"/>`) {
		t.Errorf("expected item 1, got: %s", got)
	}
	if !contains(got, `<p:item id="2"/>`) {
		t.Errorf("expected item 2, got: %s", got)
	}
	if !contains(got, `<p:item id="3"/>`) {
		t.Errorf("expected item 3, got: %s", got)
	}
}

func TestMarshalElement_EmptyElement(t *testing.T) {
	type Empty struct{}

	b := NewPresentationMLBuilder()
	v := &Empty{}
	b.MarshalElement(NSDrawingML, "avLst", v)
	got := b.String()
	if got != `<a:avLst/>` {
		t.Errorf("expected <a:avLst/>, got: %s", got)
	}
}

func TestMarshalElement_BoolAttr(t *testing.T) {
	type WithBool struct {
		Preserve bool  `xml:"preserve,attr,omitempty"`
		Show     *bool `xml:"show,attr,omitempty"`
	}

	tr := true
	b := NewPresentationMLBuilder()
	v := &WithBool{Preserve: true, Show: &tr}
	b.MarshalElement(NSPresentationML, "elem", v)
	got := b.String()
	if !contains(got, `preserve="1"`) {
		t.Errorf("expected preserve=1, got: %s", got)
	}
	if !contains(got, `show="1"`) {
		t.Errorf("expected show=1, got: %s", got)
	}
}

func TestMarshalElement_TextContent(t *testing.T) {
	type Run struct {
		Text string `xml:"http://schemas.openxmlformats.org/drawingml/2006/main t"`
	}

	b := NewPresentationMLBuilder()
	v := &Run{Text: "Hello World"}
	b.MarshalElement(NSDrawingML, "r", v)
	got := b.String()
	if !contains(got, "<a:t>Hello World</a:t>") {
		t.Errorf("expected text element, got: %s", got)
	}
}

func TestMarshalElement_XMLNameSkipped(t *testing.T) {
	type WithXMLName struct {
		XMLName xml.Name `xml:"http://example.com test"`
		ID      uint32   `xml:"id,attr"`
	}

	b := NewPresentationMLBuilder()
	v := &WithXMLName{ID: 5}
	// Element name comes from caller, not XMLName
	b.MarshalElement(NSPresentationML, "sp", v)
	got := b.String()
	if !contains(got, `<p:sp id="5"/>`) {
		t.Errorf("expected <p:sp id=\"5\"/>, got: %s", got)
	}
}

func TestMarshalElement_NilPointerOmitted(t *testing.T) {
	type Inner struct {
		Val string `xml:"val,attr"`
	}
	type Outer struct {
		Child *Inner `xml:"child,omitempty"`
	}

	b := NewPresentationMLBuilder()
	v := &Outer{} // Child is nil
	b.MarshalElement(NSPresentationML, "outer", v)
	got := b.String()
	if got != `<p:outer/>` {
		t.Errorf("expected empty element, got: %s", got)
	}
}

func TestMarshalRoot(t *testing.T) {
	type SlideData struct {
		Name string `xml:"name,attr,omitempty"`
	}
	type Slide struct {
		XMLName xml.Name   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sld"`
		CSld    *SlideData `xml:"cSld,omitempty"`
	}

	b := NewPresentationMLBuilder()
	b.WriteHeader()
	v := &Slide{
		CSld: &SlideData{Name: "test"},
	}
	b.MarshalRoot(NSPresentationML, "sld", v, PresentationMLNamespaces())
	got := b.String()
	if !contains(got, `xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`) {
		t.Errorf("expected xmlns:p declaration, got: %s", got)
	}
	if !contains(got, `<p:cSld name="test"/>`) {
		t.Errorf("expected cSld element, got: %s", got)
	}
}

func TestMarshalElement_RelAttr(t *testing.T) {
	type Blip struct {
		Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
	}

	b := NewPresentationMLBuilder()
	v := &Blip{Embed: "rId2"}
	b.MarshalElement(NSDrawingML, "blip", v)
	got := b.String()
	if !contains(got, `r:embed="rId2"`) {
		t.Errorf("expected r:embed attr, got: %s", got)
	}
}

func TestMarshalElement_InheritedNamespace(t *testing.T) {
	type Child struct {
		ID uint32 `xml:"id,attr"`
	}
	type Parent struct {
		// No namespace in tag → inherits parent
		Child *Child `xml:"child,omitempty"`
	}

	b := NewPresentationMLBuilder()
	v := &Parent{Child: &Child{ID: 1}}
	b.MarshalElement(NSPresentationML, "parent", v)
	got := b.String()
	// Child should inherit p: namespace from parent
	if !contains(got, `<p:child id="1"/>`) {
		t.Errorf("expected p:child (inherited ns), got: %s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// C188: a scalar field in a registered-but-undeclared namespace must get an
// inline xmlns declaration, exactly like a struct field would.
func TestMarshal_ScalarFieldInUndeclaredNamespace(t *testing.T) {
	type root struct {
		V string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart v"`
	}

	// NewPresentationMLBuilder registers the chart namespace (prefix c), but
	// PresentationMLNamespaces does not declare it at the root.
	b := NewPresentationMLBuilder()
	b.MarshalRoot(NSPresentationML, "root", &root{V: "x"}, PresentationMLNamespaces())
	got := b.String()

	if !contains(got, `<c:v xmlns:c="`+NSDrawingMLChart+`">x</c:v>`) {
		t.Errorf("scalar element in undeclared namespace missing inline xmlns declaration: %s", got)
	}
}

// C188: a scalar in an already-declared namespace must not get a redundant
// inline declaration.
func TestMarshal_ScalarFieldInDeclaredNamespace(t *testing.T) {
	type root struct {
		V string `xml:"http://schemas.openxmlformats.org/drawingml/2006/main v"`
	}

	b := NewPresentationMLBuilder()
	b.MarshalRoot(NSPresentationML, "root", &root{V: "x"}, PresentationMLNamespaces())
	got := b.String()

	if !contains(got, `<a:v>x</a:v>`) {
		t.Errorf("scalar in root-declared namespace gained a spurious declaration: %s", got)
	}
}

// A struct carrying the conventional CapturedAttrs field replays the source's
// attribute order, unmodeled attributes, and inline xmlns declarations, while
// modeled values stay authoritative.
func TestMarshal_CapturedAttrsReplay(t *testing.T) {
	type node struct {
		Id  uint32 `xml:"id,attr,omitempty"`
		Dur string `xml:"dur,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{
		Id:  7,
		Dur: "500",
		CapturedAttrs: []RootAttr{
			{LocalName: "dur", Value: "old"},              // modeled: order kept, current value wins
			{LocalName: "mystery", Value: "kept"},         // unmodeled: survives with captured value
			{IsNS: true, Prefix: "p14", Value: "urn:p14"}, // inline declaration at its source slot
			{LocalName: "id", Value: "1"},                 // modeled after the declaration
		},
	}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n dur="500" mystery="kept" xmlns:p14="urn:p14" id="7"/>`
	if got := b.String(); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// Without a capture (programmatic value), attributes emit in struct order.
func TestMarshal_CapturedAttrsNilUsesStructOrder(t *testing.T) {
	type node struct {
		Id  uint32 `xml:"id,attr,omitempty"`
		Dur string `xml:"dur,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{Id: 7, Dur: "500"}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n id="7" dur="500"/>`
	if got := b.String(); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// --- C106: Builder honors BuilderMarshaler and refuses stdlib-only marshalers ---

// bmType implements BuilderMarshaler and is dispatched through the Builder.
type bmType struct{ V string }

func (t bmType) MarshalToBuilder(b *Builder, ns, localName string) {
	b.WriteElement(ns, localName, t.V)
}

// stdlibOnly implements the stdlib xml.Marshaler but NOT BuilderMarshaler.
// Reaching the Builder's reflection path with such a type must fail loudly
// rather than silently discarding MarshalXML (C106).
type stdlibOnly struct{ V string }

func (t stdlibOnly) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(t.V, start)
}

func TestMarshal_C106_BuilderMarshalerDispatched(t *testing.T) {
	type wrap struct {
		Item bmType `xml:"item"`
	}
	b := NewPresentationMLBuilder()
	b.MarshalElement(NSDrawingML, "root", &wrap{Item: bmType{V: "hello"}})
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish reported error for BuilderMarshaler type: %v", err)
	}
	if got := b.String(); !contains(got, "hello") {
		t.Errorf("BuilderMarshaler output missing content: %s", got)
	}
}

func TestMarshal_C106_StdlibMarshalerFailsLoudly(t *testing.T) {
	type wrap struct {
		Item stdlibOnly `xml:"item"`
	}
	b := NewPresentationMLBuilder()
	b.MarshalElement(NSDrawingML, "root", &wrap{Item: stdlibOnly{V: "hi"}})
	err := b.Finish()
	if err == nil {
		t.Fatal("expected Finish to report an error for a stdlib-only xml.Marshaler reaching the Builder")
	}
	if !contains(err.Error(), "xml.Marshaler") {
		t.Errorf("error should mention xml.Marshaler, got: %v", err)
	}
}

// attrMarshalerOnly implements the stdlib xml.MarshalerAttr but not AttrValuer.
type attrMarshalerOnly struct{ V string }

func (a attrMarshalerOnly) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	return xml.Attr{Name: name, Value: a.V}, nil
}

func TestMarshal_C106_StdlibAttrMarshalerFailsLoudly(t *testing.T) {
	type wrap struct {
		A attrMarshalerOnly `xml:"a,attr"`
	}
	b := NewPresentationMLBuilder()
	b.MarshalElement(NSDrawingML, "root", &wrap{A: attrMarshalerOnly{V: "x"}})
	err := b.Finish()
	if err == nil {
		t.Fatal("expected Finish to report an error for a stdlib-only xml.MarshalerAttr reaching the Builder")
	}
	if !contains(err.Error(), "xml.MarshalerAttr") {
		t.Errorf("error should mention xml.MarshalerAttr, got: %v", err)
	}
}
