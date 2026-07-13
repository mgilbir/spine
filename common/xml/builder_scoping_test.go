package xml

import (
	"math"
	"strings"
	"testing"
)

// C186: an inline xmlns declaration emitted by the reflection marshaler is
// lexically scoped to the element that carries it — a later sibling subtree in
// the same namespace must get its own declaration, or its prefix is unbound.
func TestMarshal_InlineNSDeclPerSiblingSubtree(t *testing.T) {
	type chartElem struct {
		Val string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart v"`
	}

	b := NewPresentationMLBuilder()
	b.StartElementWithNS(NSPresentationML, "root", PresentationMLNamespaces())
	b.MarshalElement(NSDrawingMLChart, "chart", &chartElem{Val: "1"})
	b.MarshalElement(NSDrawingMLChart, "chart", &chartElem{Val: "2"})
	b.EndElement(NSPresentationML, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	out := b.String()
	decl := `xmlns:c="` + NSDrawingMLChart + `"`
	if got := strings.Count(out, decl); got != 2 {
		t.Errorf("got %d xmlns:c declarations, want 2 (one per sibling subtree):\n%s", got, out)
	}
	if got := strings.Count(out, "<c:chart"); got != 2 {
		t.Fatalf("got %d <c:chart> elements, want 2:\n%s", got, out)
	}
	// Nested reuse inside a subtree declares once: the child <c:v> must not
	// carry its own declaration.
	if strings.Contains(out, `<c:v xmlns`) {
		t.Errorf("nested element in an already-declared scope re-declared the namespace:\n%s", out)
	}
}

// C186: a self-closing element's inline declaration is scoped to that element
// only; the next sibling still gets its own.
func TestMarshal_InlineNSDeclPerSelfClosingSibling(t *testing.T) {
	type empty struct{}

	b := NewPresentationMLBuilder()
	b.StartElementWithNS(NSPresentationML, "root", PresentationMLNamespaces())
	b.MarshalElement(NSDrawingMLChart, "chart", &empty{})
	b.MarshalElement(NSDrawingMLChart, "chart", &empty{})
	b.EndElement(NSPresentationML, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	decl := `xmlns:c="` + NSDrawingMLChart + `"`
	if got := strings.Count(b.String(), decl); got != 2 {
		t.Errorf("got %d xmlns:c declarations, want 2:\n%s", got, b.String())
	}
}

// C186: StartElementInlineNS scopes its declaration to the element; the
// previous declared-state is restored when the element closes.
func TestBuilder_InlineNSScopeRestored(t *testing.T) {
	const ns = "http://example.com/ext"

	b := NewBuilder()
	b.StartElementInlineNS(ns, "p14", "ext")
	if !b.IsNamespaceDeclared(ns) {
		t.Error("namespace not declared inside its own element")
	}
	b.EndElementInlineNS("p14", "ext")
	if b.IsNamespaceDeclared(ns) {
		t.Error("inline declaration leaked past its element")
	}
	if err := b.Finish(); err != nil {
		t.Errorf("Finish: %v", err)
	}
}

// C186/C215: an inline declaration for a namespace that was already declared
// at the root must not erase the root declaration when the element closes.
func TestBuilder_InlineNSRestoresRootDeclared(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace(NSPresentationML, PrefixPresentationML)
	b.RegisterNamespace(NSMarkupCompatibility, PrefixMarkupCompatibility)
	b.StartElementWithNS(NSPresentationML, "root", []NSDecl{
		{PrefixPresentationML, NSPresentationML},
		{PrefixMarkupCompatibility, NSMarkupCompatibility},
	})
	b.StartElementInlineNS(NSMarkupCompatibility, PrefixMarkupCompatibility, "AlternateContent")
	b.EndElementInlineNS(PrefixMarkupCompatibility, "AlternateContent")
	if !b.IsNamespaceDeclared(NSMarkupCompatibility) {
		t.Error("root-declared namespace forgotten after a scoped inline declaration")
	}
}

// C103: StartElementInlineNS must escape the namespace URI like every other
// attribute value.
func TestBuilder_StartElementInlineNSEscapesURI(t *testing.T) {
	b := NewBuilder()
	b.StartElementInlineNS("http://x/?a=1&b=2", "p14", "ext")
	b.EndElementInlineNS("p14", "ext")
	if !strings.Contains(b.String(), `xmlns:p14="http://x/?a=1&amp;b=2"`) {
		t.Errorf("URI not attribute-escaped: %s", b.String())
	}
}

// C147: writing a name in a namespace with no registered prefix is a caller
// bug that must surface through Err/Finish instead of silently emitting an
// unprefixed (wrong-namespace) element.
func TestBuilder_UnregisteredNamespaceSurfacesError(t *testing.T) {
	b := NewBuilder()
	b.StartElement("http://example.com/unregistered", "el")
	b.EndElement("http://example.com/unregistered", "el")
	err := b.Finish()
	if err == nil {
		t.Fatal("expected an unregistered-namespace error, got nil")
	}
	if !strings.Contains(err.Error(), "no prefix registered") {
		t.Errorf("unexpected error: %v", err)
	}
	// The output itself stays well-formed.
	if !strings.Contains(b.String(), "<el>") {
		t.Errorf("element not emitted: %s", b.String())
	}
}

// C147: itoa must handle math.MinInt64, whose negation overflows.
func TestIntAttr_Bounds(t *testing.T) {
	cases := map[int64]string{
		math.MinInt64: "-9223372036854775808",
		math.MaxInt64: "9223372036854775807",
		-1:            "-1",
		0:             "0",
	}
	for in, want := range cases {
		if got := IntAttr("v", in).Value; got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}

// C187: a BuilderMarshaler that leaves an element open is caught by Finish.
type unbalancedMarshaler struct{}

func (u *unbalancedMarshaler) MarshalToBuilder(b *Builder, ns, localName string) {
	b.StartElement(ns, localName) // deliberately never closed
}

func TestMarshalElement_UnbalancedMarshalerCaughtByFinish(t *testing.T) {
	b := NewPresentationMLBuilder()
	b.StartElementWithNS(NSPresentationML, "root", PresentationMLNamespaces())
	b.MarshalElement(NSPresentationML, "bad", &unbalancedMarshaler{})
	b.EndElement(NSPresentationML, "root")
	if err := b.Finish(); err == nil {
		t.Error("unbalanced BuilderMarshaler output not reported by Finish")
	}
}

// C212: a nil pointer attribute without omitempty is omitted (encoding/xml
// behavior), not emitted as attr="".
func TestMarshal_NilPointerAttrOmitted(t *testing.T) {
	type elem struct {
		P *int32 `xml:"p,attr"` // no omitempty
	}

	b := NewPresentationMLBuilder()
	b.MarshalElement(NSPresentationML, "e", &elem{})
	if strings.Contains(b.String(), `p=""`) {
		t.Errorf("nil pointer attr emitted as empty string: %s", b.String())
	}

	// A non-nil pointer to the zero value is still emitted.
	v := int32(0)
	b2 := NewPresentationMLBuilder()
	b2.MarshalElement(NSPresentationML, "e", &elem{P: &v})
	if !strings.Contains(b2.String(), `p="0"`) {
		t.Errorf("pointer-to-zero attr dropped: %s", b2.String())
	}
}
