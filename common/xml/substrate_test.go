package xml

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// namespaceWellFormed reports the first namespace error in s: an element or
// attribute written with a prefix that no in-scope declaration binds. Go's
// decoder does not reject these — it reports the bare prefix as the name's
// Space — so a check on Space against the declared URIs catches exactly the
// "prefix emitted but never declared" defect that plain well-formedness and
// Builder.Finish both miss.
func namespaceWellFormed(s string) error {
	d := xml.NewDecoder(strings.NewReader(s))
	declared := map[string]bool{NSXML: true}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range start.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				declared[a.Value] = true
			}
		}
		if start.Name.Space != "" && !declared[start.Name.Space] {
			return &namespaceError{name: start.Name}
		}
		for _, a := range start.Attr {
			if a.Name.Space == "xmlns" || a.Name.Space == "" {
				continue
			}
			if !declared[a.Name.Space] {
				return &namespaceError{name: a.Name}
			}
		}
	}
}

type namespaceError struct{ name xml.Name }

func (e *namespaceError) Error() string {
	return "unbound namespace prefix " + e.name.Space + " on " + e.name.Local
}

// --- C437: RawTokenBytes must not alias the registered source ---------------

type rawTokenProbe struct {
	Raw []byte
}

func (p *rawTokenProbe) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		pre := d.InputOffset()
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.Comment:
			p.Raw = RawTokenBytes(d, pre)
		case xml.EndElement:
			return nil
		}
	}
}

// C437: every production caller retains RawTokenBytes' result in long-lived
// model state, so returning a sub-slice of the registered source pins the whole
// part in memory — the C282 class. The clone is observable: mutating the source
// buffer afterwards must not change what was captured.
func TestRawTokenBytes_ClonesSource(t *testing.T) {
	data := []byte(`<root><!--keep me--></root>`)
	var p rawTokenProbe
	if err := UnmarshalWithSource(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Raw) == 0 {
		t.Fatal("no raw token captured")
	}
	before := string(p.Raw)
	for i := range data {
		data[i] = 'X'
	}
	if after := string(p.Raw); after != before {
		t.Errorf("captured token aliases the source buffer: before %q, after source mutation %q", before, after)
	}
}

// --- C438: interface-typed children --------------------------------------

type ifaceChild struct {
	Val string `xml:"val,attr"`
}

type ifaceParent struct {
	Child interface{} `xml:"http://example.com/ns child"`
}

// C438: hasStructChildren counts a non-nil interface field as a child, so the
// parent opens an element for it. marshalReflect had no reflect.Interface case,
// so the child emitted nothing and the parent shipped an empty pair with
// Finish() == nil — silent wrong bytes. The two must agree.
func TestMarshalReflect_InterfaceChildIsEmitted(t *testing.T) {
	const ns = "http://example.com/ns"
	b := NewBuilder()
	b.RegisterNamespace(ns, "e")
	b.MarshalElement(ns, "parent", &ifaceParent{Child: &ifaceChild{Val: "v"}})
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, "<e:child") || !strings.Contains(out, `val="v"`) {
		t.Errorf("interface-typed child was dropped while the parent expanded for it:\n%s", out)
	}
}

// --- C472: MarshalRoot must not write into the caller's slice --------------

type rootAttrsProbe struct {
	A string `xml:"a,attr"`
	B string `xml:"b,attr"`
}

// C472: append(extraAttrs, attrs...) writes the modeled attributes into the
// caller's backing array whenever it has spare capacity, silently corrupting a
// reused attribute buffer.
func TestMarshalRoot_DoesNotWriteCallerBackingArray(t *testing.T) {
	const ns = "http://example.com/ns"
	buf := make([]Attr, 1, 8) // one element, room for the modeled attrs
	buf[0] = Attr{Name: "extra", Value: "1"}
	sentinel := buf[:cap(buf)]
	for i := 1; i < len(sentinel); i++ {
		sentinel[i] = Attr{Name: "untouched", Value: "yes"}
	}

	b := NewBuilder()
	b.RegisterNamespace(ns, "e")
	b.MarshalRoot(ns, "root", &rootAttrsProbe{A: "x", B: "y"}, []NSDecl{{"e", ns}}, buf...)
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	for i := 1; i < len(sentinel); i++ {
		if sentinel[i].Name != "untouched" {
			t.Fatalf("MarshalRoot wrote into the caller's backing array at %d: %+v", i, sentinel[i])
		}
	}
}

// --- C473: the element separator must not become element content ----------

// C473: EndElement called writeIndent unconditionally, so in separator mode an
// open/close pair with no content became <t> </t> — the separator turned into
// character data and a spaced-format source containing <t></t> could not round
// trip.
func TestBuilder_SeparatorNotWrittenInsideEmptyPair(t *testing.T) {
	const ns = "http://example.com/ns"
	b := NewBuilder()
	b.RegisterNamespace(ns, "e")
	b.SetElementSeparator(" ")
	b.StartElementWithNS(ns, "root", []NSDecl{{"e", ns}})
	b.StartElement(ns, "t")
	b.EndElement(ns, "t")
	b.StartElement(ns, "u")
	b.EndElement(ns, "u")
	b.EndElement(ns, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	out := b.String()
	if strings.Contains(out, "<e:t> </e:t>") {
		t.Errorf("separator emitted as content inside an empty element pair:\n%s", out)
	}
	if !strings.Contains(out, "<e:t></e:t>") {
		t.Errorf("empty pair not preserved:\n%s", out)
	}
	// The separator still marks the boundary between the two siblings.
	if !strings.Contains(out, "</e:t> <e:u>") {
		t.Errorf("sibling separator lost:\n%s", out)
	}
}

// C473: a non-empty element still gets its separator before the end tag, so
// the fix must not swallow the spaced form producers actually write.
func TestBuilder_SeparatorKeptAroundContent(t *testing.T) {
	const ns = "http://example.com/ns"
	b := NewBuilder()
	b.RegisterNamespace(ns, "e")
	b.SetElementSeparator(" ")
	b.StartElementWithNS(ns, "root", []NSDecl{{"e", ns}})
	b.StartElement(ns, "outer")
	b.EmptyElement(ns, "inner")
	b.EndElement(ns, "outer")
	b.EndElement(ns, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if out := b.String(); !strings.Contains(out, "<e:inner/> </e:outer>") {
		t.Errorf("separator before the end tag of a non-empty element was dropped:\n%s", out)
	}
}

// --- C474: the whitespace run before a self-closing slash ------------------

type emptyStyleProbe struct {
	Style EmptyTagStyle
}

func (p *emptyStyleProbe) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "leaf" {
				p.Style = CaptureEmptyTagStyle(d)
			}
			if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// C474: the capture collapsed the whitespace-before-slash lexeme to one bit —
// only ' ' and '\t' at a fixed offset counted, and every "spaced" tag replayed
// exactly one space. <leaf\n/> classified as tight and lost the newline;
// <leaf\t/> and <leaf  /> both replayed as <leaf />.
func TestCaptureEmptyTagStyle_WhitespaceRunRoundTrips(t *testing.T) {
	const ns = "http://example.com/ns"
	for _, ws := range []string{"", " ", "\t", "\n", "\r\n", "  ", " \t", "\n\t "} {
		src := "<root><leaf" + ws + "/></root>"
		var p emptyStyleProbe
		if err := UnmarshalWithSource([]byte(src), &p); err != nil {
			t.Fatalf("unmarshal %q: %v", src, err)
		}
		if !p.Style.IsSelfClose() {
			t.Errorf("%q: captured style is not self-closing (%d)", src, p.Style)
			continue
		}
		b := NewBuilder()
		b.RegisterNamespace(ns, "")
		b.EmptyElementStyled(p.Style, ns, "leaf")
		if err := b.Finish(); err != nil {
			t.Fatalf("%q: Finish: %v", src, err)
		}
		want := "<leaf" + ws + "/>"
		if got := b.String(); got != want {
			t.Errorf("whitespace run before the slash drifted: source %q replayed as %q, want %q", src, got, want)
		}
	}
}

// C474: the builder's per-part self-closing style must survive an element that
// carried its own captured whitespace run.
func TestEmptyElementStyled_RestoresPartStyle(t *testing.T) {
	const ns = "http://example.com/ns"
	var p emptyStyleProbe
	if err := UnmarshalWithSource([]byte("<root><leaf\t/></root>"), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b := NewBuilder()
	b.RegisterNamespace(ns, "")
	b.SetSelfClosingSpace(true)
	b.EmptyElementStyled(p.Style, ns, "a")
	b.EmptyElement(ns, "b")
	if got, want := b.String(), "<a\t/><b />"; got != want {
		t.Errorf("per-instance whitespace leaked into the part style: got %q, want %q", got, want)
	}
}

// --- C475: mixed content in ordered child capture --------------------------

type orderedMixed struct {
	Kid              *orderedKid   `xml:"http://example.com/ns kid"`
	CapturedChildren *ChildCapture `xml:"-"`
}

type orderedKid struct {
	V string `xml:"v,attr"`
}

func (m *orderedMixed) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return UnmarshalOrderedChildren(d, m)
}

// C475: UnmarshalOrderedChildren dropped every non-whitespace CharData token
// with no guard and no error, so a struct that gained a text-bearing child lost
// user content silently on the next regeneration.
func TestUnmarshalOrderedChildren_KeepsMixedContent(t *testing.T) {
	const ns = "http://example.com/ns"
	src := `<mixed xmlns="http://example.com/ns">before<kid v="1"/>after</mixed>`
	var m orderedMixed
	if err := UnmarshalWithSource([]byte(src), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Kid == nil || m.Kid.V != "1" {
		t.Fatalf("typed child lost: %+v", m.Kid)
	}

	b := NewBuilder()
	b.RegisterNamespace(ns, "")
	b.MarshalElement(ns, "mixed", &m)
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, "before") || !strings.Contains(out, "after") {
		t.Errorf("mixed content silently discarded:\n%s", out)
	}
}

// --- C477: inherited xml:* attributes in subset canonicalization -----------

// C477: inclusive C14N requires the apex element of a document subset to import
// the xml-namespace attributes its ancestors declare (§2.4). Omitting them made
// an Object subtree under an ancestor xml:space canonicalize differently from a
// conforming verifier, so the digest never matched.
func TestC14N_SubsetImportsInheritedXMLAttrs(t *testing.T) {
	doc := `<outer xml:space="preserve" xml:lang="en"><obj Id="o1"><t>x</t></obj></outer>`
	root, err := ParseC14N([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	obj := root.FindByID("o1")
	if obj == nil {
		t.Fatal("Object not found")
	}
	got := string(obj.Canonical())
	want := `<obj Id="o1" xml:lang="en" xml:space="preserve"><t>x</t></obj>`
	if got != want {
		t.Errorf("subset canonicalization dropped inherited xml:* attributes:\ngot  %s\nwant %s", got, want)
	}
}

// C477: the nearest ancestor wins, and an attribute the apex carries itself is
// never overwritten by an ancestor's.
func TestC14N_InheritedXMLAttrsNearestAncestorWins(t *testing.T) {
	doc := `<a xml:lang="en" xml:space="preserve"><b xml:lang="fr"><c Id="c1" xml:space="default"/></b></a>`
	root, err := ParseC14N([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := string(root.FindByID("c1").Canonical())
	want := `<c Id="c1" xml:lang="fr" xml:space="default"></c>`
	if got != want {
		t.Errorf("inheritance resolution wrong:\ngot  %s\nwant %s", got, want)
	}
}

// C477: a descendant rendered inside the subset must not repeat the apex's
// inherited attributes — only the apex imports them.
func TestC14N_DescendantsDoNotRepeatInheritedXMLAttrs(t *testing.T) {
	doc := `<outer xml:space="preserve"><obj Id="o1"><t>x</t></obj></outer>`
	root, err := ParseC14N([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := string(root.FindByID("o1").Canonical())
	if strings.Count(got, `xml:space`) != 1 {
		t.Errorf("inherited attribute repeated on descendants: %s", got)
	}
}

// C477: whole-document canonicalization is unchanged — the root has no
// ancestors to inherit from.
func TestC14N_RootCanonicalizationUnchanged(t *testing.T) {
	doc := `<a xml:space="preserve"><b/></a>`
	got, err := Canonicalize([]byte(doc))
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	want := `<a xml:space="preserve"><b></b></a>`
	if string(got) != want {
		t.Errorf("root canonicalization changed:\ngot  %s\nwant %s", got, want)
	}
}

// --- C478: a captured default declaration for a non-root URI ---------------

// C478: the registration guard skipped every default (xmlns=) declaration, so a
// root that defaults some other namespace marked the URI declared while leaving
// b.namespaces untouched; a modeled child in that URI then emitted a stale or
// undeclared prefix.
func TestStartElementWithRootAttrs_DefaultDeclForOtherURI(t *testing.T) {
	const rootNS = "http://example.com/root"
	const otherNS = "http://example.com/other"

	b := NewBuilder()
	b.RegisterNamespace(rootNS, "r")
	// A stale registration for otherNS, as a preconfigured builder would carry.
	b.RegisterNamespace(otherNS, "stale")
	b.StartElementWithRootAttrs(rootNS, "root", []RootAttr{
		{IsNS: true, Prefix: "r", Value: rootNS},
		{IsNS: true, Prefix: "", Value: otherNS},
	})
	b.EmptyElement(otherNS, "child")
	b.EndElement(rootNS, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	out := b.String()
	if strings.Contains(out, "stale:child") {
		t.Errorf("child emitted an undeclared prefix for a default-declared namespace:\n%s", out)
	}
	if err := namespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
}

// C478: the merged root variant shares the registration logic and the fix.
func TestStartElementWithRootAttrsMerged_DefaultDeclForOtherURI(t *testing.T) {
	const rootNS = "http://example.com/root"
	const otherNS = "http://example.com/other"

	b := NewBuilder()
	b.RegisterNamespace(rootNS, "r")
	b.RegisterNamespace(otherNS, "stale")
	b.StartElementWithRootAttrsMerged(rootNS, "root", []RootAttr{
		{IsNS: true, Prefix: "r", Value: rootNS},
		{IsNS: true, Prefix: "", Value: otherNS},
	}, nil)
	b.EmptyElement(otherNS, "child")
	b.EndElement(rootNS, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if out := b.String(); strings.Contains(out, "stale:child") {
		t.Errorf("child emitted an undeclared prefix for a default-declared namespace:\n%s", out)
	}
}

// --- literal-replay guard --------------------------------------------------

// The literal replay paths write names verbatim and so bypass writeQName's
// unbound-prefix check. StartElementLiteral's binds parameter is the one input
// that can register a prefix the output never declares (C375 shipped exactly
// that): the contract says the declaration must be present among attrs, and
// violating it is now an error rather than silently malformed XML.
func TestStartElementLiteral_BindWithoutDeclarationIsError(t *testing.T) {
	const rootNS = "http://example.com/root"
	const extNS = "http://example.com/ext"

	b := NewBuilder()
	b.RegisterNamespace(rootNS, "r")
	b.StartElementWithNS(rootNS, "root", []NSDecl{{"r", rootNS}})
	b.StartElementLiteral("x", "thing", []NSDecl{{Prefix: "x", URI: extNS}})
	b.EndElementLiteral("x", "thing")
	b.EndElement(rootNS, "root")

	if err := b.Finish(); err == nil {
		t.Errorf("literal element bound a prefix it never declared, and Finish blessed it:\n%s", b.String())
	}
}

// The guard must stay quiet for the legitimate forms: the declaration written
// among attrs, and a binding already in scope under the same prefix.
func TestStartElementLiteral_DeclaredBindsAccepted(t *testing.T) {
	const rootNS = "http://example.com/root"
	const extNS = "http://example.com/ext"

	b := NewBuilder()
	b.RegisterNamespace(rootNS, "r")
	b.StartElementWithNS(rootNS, "root", []NSDecl{{"r", rootNS}})
	b.StartElementLiteral("x", "own", []NSDecl{{Prefix: "x", URI: extNS}},
		Attr{Name: "xmlns:x", Value: extNS})
	b.EndElementLiteral("x", "own")
	b.EndElement(rootNS, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("declaration written among attrs was rejected: %v", err)
	}

	b2 := NewBuilder()
	b2.RegisterNamespace(rootNS, "r")
	b2.StartElementWithNS(rootNS, "root", []NSDecl{{"r", rootNS}, {"x", extNS}})
	b2.StartElementLiteral("x", "inherited", []NSDecl{{Prefix: "x", URI: extNS}})
	b2.EndElementLiteral("x", "inherited")
	b2.EndElement(rootNS, "root")
	if err := b2.Finish(); err != nil {
		t.Fatalf("binding already in scope was rejected: %v", err)
	}
}

// A rootless Builder is a fragment whose prefixes are declared by whoever
// embeds it, so the guard must not fire there.
func TestStartElementLiteral_RootlessFragmentNotGuarded(t *testing.T) {
	b := NewBuilder()
	b.StartElementLiteral("x", "thing", []NSDecl{{Prefix: "x", URI: "http://example.com/ext"}})
	b.EndElementLiteral("x", "thing")
	if err := b.Finish(); err != nil {
		t.Fatalf("guard fired on a rootless fragment: %v", err)
	}
}
