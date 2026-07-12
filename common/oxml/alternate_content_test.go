package oxml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

func marshalAC(ac *AlternateContent) string {
	b := xmlb.NewPresentationMLBuilder()
	ac.MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "AlternateContent")
	return b.String()
}

// C41: multiple mc:Choice branches are preserved (previously collapsed to the last).
func TestAlternateContent_MultipleChoices(t *testing.T) {
	src := `<AlternateContent xmlns="http://schemas.openxmlformats.org/markup-compatibility/2006">` +
		`<Choice Requires="p14"><p14:one/></Choice>` +
		`<Choice Requires="p15"><p15:two/></Choice>` +
		`<Fallback/>` +
		`</AlternateContent>`

	var ac AlternateContent
	if err := xml.Unmarshal([]byte(src), &ac); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ac.Choices) != 2 {
		t.Fatalf("got %d choices, want 2", len(ac.Choices))
	}
	if !ac.HasFallback {
		t.Error("empty Fallback was dropped (HasFallback false)")
	}

	out := marshalAC(&ac)
	if !strings.Contains(out, `Requires="p14"`) || !strings.Contains(out, `Requires="p15"`) {
		t.Errorf("a Choice was lost on marshal:\n%s", out)
	}
	if !strings.Contains(out, "Fallback") {
		t.Errorf("empty Fallback not re-emitted:\n%s", out)
	}
}

// C41: a Requires listing several prefixes declares each namespace.
func TestAlternateContent_MultiPrefixRequires(t *testing.T) {
	src := `<AlternateContent xmlns="http://schemas.openxmlformats.org/markup-compatibility/2006">` +
		`<Choice Requires="p14 p15"><p14:x/></Choice>` +
		`</AlternateContent>`

	var ac AlternateContent
	if err := xml.Unmarshal([]byte(src), &ac); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := marshalAC(&ac)
	if !strings.Contains(out, "xmlns:p14=") || !strings.Contains(out, "xmlns:p15=") {
		t.Errorf("multi-prefix Requires left a prefix undeclared:\n%s", out)
	}
}

// C215: a namespace already declared (e.g. at the root) is not re-declared by
// an AlternateContent, and its declared-state survives the AC — including for
// a second AC in sequence.
func TestAlternateContent_RootDeclaredNamespacesSurvive(t *testing.T) {
	b := xmlb.NewPresentationMLBuilder()
	b.StartElementWithRootAttrs(xmlb.NSPresentationML, "root", []xmlb.RootAttr{
		{IsNS: true, Prefix: "p", Value: xmlb.NSPresentationML},
		{IsNS: true, Prefix: "mc", Value: xmlb.NSMarkupCompatibility},
		{IsNS: true, Prefix: "p14", Value: xmlb.NSPowerPoint2010},
	})

	ac1 := &AlternateContent{Choices: []AlternateContentChoice{{Requires: "p14", Content: []byte("<p14:x/>")}}}
	ac2 := &AlternateContent{Choices: []AlternateContentChoice{{Requires: "p14", Content: []byte("<p14:y/>")}}}
	ac1.MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "AlternateContent")
	ac2.MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "AlternateContent")
	b.EndElement(xmlb.NSPresentationML, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	out := b.String()
	// The root declaration is in scope: neither AC re-declares p14.
	if n := strings.Count(out, `xmlns:p14=`); n != 1 {
		t.Errorf("got %d xmlns:p14 declarations, want 1 (root only):\n%s", n, out)
	}
	// The root declarations are still known after both ACs.
	if !b.IsNamespaceDeclared(xmlb.NSPowerPoint2010) {
		t.Error("root-declared p14 forgotten after AlternateContent marshal")
	}
	if !b.IsNamespaceDeclared(xmlb.NSMarkupCompatibility) {
		t.Error("root-declared mc forgotten after AlternateContent marshal")
	}
}

// C215/C186: without a root declaration, each sibling AC carries its own
// extension declaration (the first AC's inline declaration is out of scope for
// the second).
func TestAlternateContent_SiblingACsEachDeclare(t *testing.T) {
	b := xmlb.NewPresentationMLBuilder()
	b.StartElementWithNS(xmlb.NSPresentationML, "root", xmlb.PresentationMLNamespaces())

	ac1 := &AlternateContent{Choices: []AlternateContentChoice{{Requires: "p14", Content: []byte("<p14:x/>")}}}
	ac2 := &AlternateContent{Choices: []AlternateContentChoice{{Requires: "p14", Content: []byte("<p14:y/>")}}}
	ac1.MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "AlternateContent")
	ac2.MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "AlternateContent")
	b.EndElement(xmlb.NSPresentationML, "root")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	out := b.String()
	if n := strings.Count(out, `xmlns:p14=`); n != 2 {
		t.Errorf("got %d xmlns:p14 declarations, want 2 (one per AC):\n%s", n, out)
	}
	if n := strings.Count(out, `xmlns:mc=`); n != 2 {
		t.Errorf("got %d xmlns:mc declarations, want 2 (one per AC):\n%s", n, out)
	}
}

// C216: Choice/Fallback local names from a foreign namespace are not captured
// as mc branches.
func TestAlternateContent_ForeignChoiceIgnored(t *testing.T) {
	src := `<AlternateContent xmlns="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:x="urn:other">` +
		`<x:Choice Requires="p14"><p14:bogus/></x:Choice>` +
		`<x:Fallback><x:f/></x:Fallback>` +
		`<Choice Requires="p15"><p15:real/></Choice>` +
		`</AlternateContent>`

	var ac AlternateContent
	if err := xml.Unmarshal([]byte(src), &ac); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ac.Choices) != 1 || ac.Choices[0].Requires != "p15" {
		t.Errorf("foreign-namespace Choice captured: %+v", ac.Choices)
	}
	if ac.HasFallback {
		t.Error("foreign-namespace Fallback captured")
	}
}
