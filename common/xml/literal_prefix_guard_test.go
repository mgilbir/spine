package xml

import (
	"strings"
	"testing"
)

// The literal-replay paths write element names verbatim, so writeQName's
// unbound-prefix check never sees them. C375 shipped through that hole: an
// mc:AlternateContent emitted under a hardcoded "mc" in a document that
// declares the namespace as ve: is malformed XML, and Finish() returned nil.
// #227 closed the narrow part of the hole (a bind without a declaration) and
// deliberately withheld the element's own prefix check, because declarations
// written through the literal paths never entered the Builder's scope: an
// ancestor replaying a captured xmlns:a14 would have made a legitimate a14:
// descendant fail the check. These tests cover both halves — the scope is now
// tracked, and the check is on.

const (
	guardNS = "urn:example:guard"
	extNS   = "urn:example:ext"
)

// openGuardRoot writes a root element declaring only guardNS, so the Builder is
// past the rootless carve-out and "g" is the one bound prefix.
func openGuardRoot(b *Builder) {
	b.RegisterNamespace(guardNS, "g")
	b.StartElementWithNS(guardNS, "root", []NSDecl{{Prefix: "g", URI: guardNS}})
}

// requireUnbound fails the test when the built output is namespace-well-formed,
// which would mean the case under test never reproduced the defect and any
// assertion about detecting it passes for the wrong reason.
func requireUnbound(t *testing.T, out string) {
	t.Helper()
	if err := namespaceWellFormed(out); err == nil {
		t.Fatalf("harness bug: output is namespace-well-formed, so there is nothing to detect:\n%s", out)
	}
}

func TestStartElementLiteral_UnboundElementPrefixIsAnError(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	b.StartElementLiteral("mc", "AlternateContent", nil)
	b.EndElementLiteral("mc", "AlternateContent")
	b.EndElement(guardNS, "root")

	requireUnbound(t, b.String())
	err := b.Finish()
	if err == nil {
		t.Fatalf("Finish accepted an element under the undeclared prefix mc:\n%s", b.String())
	}
	if !strings.Contains(err.Error(), "mc") {
		t.Errorf("error does not name the offending prefix: %v", err)
	}
}

func TestEmptyElementLiteral_UnboundElementPrefixIsAnError(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	b.EmptyElementLiteral("a14", "useLocalDpi", StrAttr("val", "1"))
	b.EndElement(guardNS, "root")

	requireUnbound(t, b.String())
	if err := b.Finish(); err == nil {
		t.Fatalf("Finish accepted a self-closing element under the undeclared prefix a14:\n%s", b.String())
	}
}

// The declaration the element carries itself binds its prefix, so the check
// must not fire on the one shape the replay paths use most.
func TestEmptyElementLiteral_OwnDeclarationBindsThePrefix(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	b.EmptyElementLiteral("a14", "useLocalDpi",
		Attr{Name: "xmlns:a14", Value: extNS}, StrAttr("val", "1"))
	b.EndElement(guardNS, "root")

	if err := b.Finish(); err != nil {
		t.Fatalf("Finish rejected an element that declares its own prefix: %v\n%s", err, b.String())
	}
	if err := namespaceWellFormed(b.String()); err != nil {
		t.Errorf("%v:\n%s", err, b.String())
	}
}

// A declaration that reaches the output only through a captured verbatim
// rendering still binds the prefix: writeLiteralAttrs copies Raw as-is, so a
// check that only understood Attr.Name would reject the producer's own file.
func TestEmptyElementLiteral_RawOnlyDeclarationBindsThePrefix(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	b.EmptyElementLiteral("a14", "useLocalDpi",
		Attr{Raw: ` xmlns:a14='` + extNS + `'`}, StrAttr("val", "1"))
	b.EndElement(guardNS, "root")

	if err := b.Finish(); err != nil {
		t.Fatalf("Finish rejected a prefix bound by a verbatim declaration: %v\n%s", err, b.String())
	}
	if err := namespaceWellFormed(b.String()); err != nil {
		t.Errorf("%v:\n%s", err, b.String())
	}
}

// This is the case #227 named as its reason to withhold the check: the
// declaration lives on a literal ancestor, so before literal declarations were
// tracked the Builder had no way to know the descendant's prefix was bound.
func TestStartElementLiteral_AncestorLiteralDeclarationIsInScope(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	b.StartElementLiteral("g", "ext", nil, Attr{Name: "xmlns:a14", Value: extNS})
	b.EmptyElementLiteral("a14", "useLocalDpi", StrAttr("val", "1"))
	b.EndElementLiteral("g", "ext")
	b.EndElement(guardNS, "root")

	if err := b.Finish(); err != nil {
		t.Fatalf("Finish rejected a descendant of a literal declaration: %v\n%s", err, b.String())
	}
	if err := namespaceWellFormed(b.String()); err != nil {
		t.Errorf("%v:\n%s", err, b.String())
	}
}

// The same holds for a declaration passed through StartElementInlineNS's
// attribute list, which dml's a:ext uses to replay InlineNSDecls.
func TestStartElementInlineNS_AttrDeclarationIsInScopeForLiteralChildren(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	b.StartElementInlineNS(guardNS+"/x", "x", "ext", Attr{Name: "xmlns:a14", Value: extNS})
	b.EmptyElementLiteral("a14", "useLocalDpi", StrAttr("val", "1"))
	b.EndElementInlineNS("x", "ext")
	b.EndElement(guardNS, "root")

	if err := b.Finish(); err != nil {
		t.Fatalf("Finish rejected a child of an inline-NS element's own declaration: %v\n%s", err, b.String())
	}
	if err := namespaceWellFormed(b.String()); err != nil {
		t.Errorf("%v:\n%s", err, b.String())
	}
}

// A literal declaration is lexically scoped, so a later sibling gets no benefit
// from it — the same rule the output's consumers apply.
func TestStartElementLiteral_LiteralDeclarationScopeEndsWithItsElement(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	b.StartElementLiteral("g", "ext", nil, Attr{Name: "xmlns:a14", Value: extNS})
	b.EmptyElementLiteral("a14", "useLocalDpi", StrAttr("val", "1"))
	b.EndElementLiteral("g", "ext")
	b.EmptyElementLiteral("a14", "shadowObscured", StrAttr("val", "1"))
	b.EndElement(guardNS, "root")

	requireUnbound(t, b.String())
	if err := b.Finish(); err == nil {
		t.Fatalf("Finish accepted a prefix whose declaration had gone out of scope:\n%s", b.String())
	}
}

// One URI may be declared under two prefixes (Word 2007 binds the
// markup-compatibility namespace to both mc and ve). The Builder's URI-keyed
// maps hold only one of them, so the check has to be prefix-scoped: a
// URI-keyed answer would reject the alias the producer actually wrote.
func TestStartElementLiteral_AliasedPrefixForOneURIIsAccepted(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace(guardNS, "g")
	b.RegisterNamespace(NSMarkupCompatibility, "mc")
	b.StartElementWithNS(guardNS, "root", []NSDecl{
		{Prefix: "g", URI: guardNS},
		{Prefix: "mc", URI: NSMarkupCompatibility},
		{Prefix: "ve", URI: NSMarkupCompatibility},
	})
	b.StartElementLiteral("ve", "AlternateContent", nil)
	b.EndElementLiteral("ve", "AlternateContent")
	b.EndElement(guardNS, "root")

	if err := b.Finish(); err != nil {
		t.Fatalf("Finish rejected the producer's alias for a declared namespace: %v\n%s", err, b.String())
	}
	if err := namespaceWellFormed(b.String()); err != nil {
		t.Errorf("%v:\n%s", err, b.String())
	}
}

// #227's carve-out: a Builder with no root element is a fragment that will be
// spliced into a scope the Builder cannot see, so the check stays quiet rather
// than failing a save over prefixes it has no business judging.
func TestLiteralPrefixCheck_RootlessFragmentStaysQuiet(t *testing.T) {
	b := NewBuilder()
	b.StartElementLiteral("w", "frameset", nil)
	b.EmptyElementLiteral("w", "sz", Attr{Name: "w:val", Value: "50%"})
	b.EndElementLiteral("w", "frameset")

	if err := b.Finish(); err != nil {
		t.Errorf("rootless fragment was checked: %v\n%s", err, b.String())
	}
}

// Step 2 of the guard work, pinned deliberately: once the Builder knows a
// literal ancestor declared a namespace, a typed descendant in it stops
// emitting the redundant inline declaration it used to add. The output is the
// shape the source had — the declaration stays where the producer put it — and
// no current caller reaches this combination (every literal element that
// declares a namespace today has verbatim or literal-only children), so this
// changes no round-trip bytes. It is asserted here so a future caller that does
// reach it finds the behaviour documented rather than surprising.
func TestStartElementLiteral_TypedDescendantUsesTheAncestorDeclaration(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	b.RegisterNamespace(extNS, "e")
	b.StartElementLiteral("g", "wrap", nil, Attr{Name: "xmlns:e", Value: extNS})
	b.MarshalElement(extNS, "child", &struct {
		Val string `xml:"val,attr"`
	}{Val: "v"})
	b.EndElementLiteral("g", "wrap")
	b.EndElement(guardNS, "root")

	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v\n%s", err, b.String())
	}
	out := b.String()
	if err := namespaceWellFormed(out); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if strings.Count(out, `xmlns:e="`+extNS+`"`) != 1 {
		t.Errorf("expected the ancestor's declaration to serve the descendant, got:\n%s", out)
	}
	if !strings.Contains(out, "<e:child ") {
		t.Errorf("descendant did not resolve through the ancestor's binding:\n%s", out)
	}
}

// The prefix scope must unwind exactly: a declaration on a closed element must
// not keep binding its prefix, and one that was in scope before must come back.
func TestLiteralPrefixScope_UnwindsAcrossSiblings(t *testing.T) {
	b := NewBuilder()
	openGuardRoot(b)
	for i := 0; i < 3; i++ {
		b.StartElementLiteral("g", "ext", nil, Attr{Name: "xmlns:a14", Value: extNS})
		b.EmptyElementLiteral("a14", "useLocalDpi", StrAttr("val", "1"))
		b.EndElementLiteral("g", "ext")
	}
	b.EndElement(guardNS, "root")

	if err := b.Finish(); err != nil {
		t.Fatalf("repeated sibling declarations were rejected: %v\n%s", err, b.String())
	}
	if err := namespaceWellFormed(b.String()); err != nil {
		t.Errorf("%v:\n%s", err, b.String())
	}
	if b.prefixScope["a14"] != 0 {
		t.Errorf("prefix scope leaked: a14 still counted %d times", b.prefixScope["a14"])
	}
}

// LiteralPrefixForCaptured must prefer the live binding over its static
// fallback, which is the whole point of migrating the RawAttrPrefix sites.
func TestLiteralPrefixForCaptured_PrefersTheLiveBinding(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace(guardNS, "g")
	b.RegisterNamespace(extNS, "alias")
	b.StartElementWithRootAttrs(guardNS, "root", []RootAttr{
		{IsNS: true, Prefix: "g", Value: guardNS},
		{IsNS: true, Prefix: "alias", Value: extNS},
	})
	if got := b.LiteralPrefixForCaptured(extNS, []RootAttr{}, "static"); got != "alias" {
		t.Errorf("resolved %q, want the in-scope alias", got)
	}
	// The element's own declaration still wins over the ancestor's, including
	// a default one, which RawAttrPrefix skipped entirely.
	own := []RootAttr{{IsNS: true, Prefix: "", Value: extNS}}
	if got := b.LiteralPrefixForCaptured(extNS, own, "static"); got != "" {
		t.Errorf("resolved %q, want the element's own default declaration (no prefix)", got)
	}
	// With no authority at all the static fallback still applies, so values
	// built programmatically keep their canonical prefix.
	if got := b.LiteralPrefixForCaptured("urn:example:unknown", nil, "static"); got != "static" {
		t.Errorf("resolved %q, want the static fallback", got)
	}
}
