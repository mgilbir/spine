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
