package oxml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C173: captured math marshaled in a context whose root does not declare the
// math namespace must emit the declaration inline on the element itself —
// prefixed and bound, never an unprefixed <oMath> or an unbound m: prefix.
func TestOMathMarshalInlineDeclarationWhenRootLacksIt(t *testing.T) {
	src := `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">` +
		`<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath>` +
		`<m:oMath><m:r><m:t>y</m:t></m:r></m:oMath></w:p>`
	var p CT_P
	if err := xml.Unmarshal([]byte(src), &p); err != nil {
		t.Fatal(err)
	}
	// Drop the fragment-scaffolding attribute capture (the xmlns declarations
	// exist only to make the standalone fragment parseable): this test checks
	// the synthesized-declaration path.
	p.CapturedAttrs = nil

	b := xmlb.NewWordprocessingMLBuilder()
	// Root element without a math namespace declaration (e.g. a header part).
	b.StartElementWithNS(xmlb.NSWordprocessingML, "hdr", xmlb.WordprocessingMLNamespaces())
	p.MarshalToBuilder(b, xmlb.NSWordprocessingML, "p")
	b.EndElement(xmlb.NSWordprocessingML, "hdr")
	out := b.String()

	if strings.Contains(out, "<oMath") {
		t.Fatalf("oMath emitted unprefixed: %s", out)
	}
	// Both siblings must carry their own inline declaration: the first
	// element's binding goes out of scope when it closes.
	want := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><m:r><m:t>x</m:t></m:r></m:oMath>`
	if !strings.Contains(out, want) {
		t.Errorf("first oMath missing inline declaration: %s", out)
	}
	if strings.Count(out, `xmlns:m=`) != 2 {
		t.Errorf("each sibling oMath needs its own inline declaration, got: %s", out)
	}
}

// C173: when the root declared the math namespace, the elements are emitted
// prefixed with no redundant inline declarations.
func TestOMathMarshalUsesRootDeclaration(t *testing.T) {
	src := `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">` +
		`<m:oMathPara><m:oMath><m:r><m:t>x</m:t></m:r></m:oMath></m:oMathPara></w:p>`
	var p CT_P
	if err := xml.Unmarshal([]byte(src), &p); err != nil {
		t.Fatal(err)
	}
	p.CapturedAttrs = nil // see TestOMathMarshalInlineDeclarationWhenRootLacksIt

	b := xmlb.NewWordprocessingMLBuilder()
	decls := append(xmlb.WordprocessingMLNamespaces(),
		xmlb.NSDecl{Prefix: xmlb.PrefixMath, URI: xmlb.NSMath})
	b.StartElementWithNS(xmlb.NSWordprocessingML, "document", decls)
	p.MarshalToBuilder(b, xmlb.NSWordprocessingML, "p")
	b.EndElement(xmlb.NSWordprocessingML, "document")
	out := b.String()

	if !strings.Contains(out, `<m:oMathPara><m:oMath><m:r><m:t>x</m:t></m:r></m:oMath></m:oMathPara>`) {
		t.Errorf("math not re-emitted prefixed from root declaration: %s", out)
	}
	// One declaration at the root, none inline.
	if strings.Count(out, `xmlns:m=`) != 1 {
		t.Errorf("unexpected inline math declarations: %s", out)
	}
}
