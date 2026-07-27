package xml

import (
	"encoding/xml"
	"testing"
)

// TestLexTagAttrs_TrailingWhitespaceForcesVerbatimFallback covers C347: a start
// tag with whitespace before '>' (e.g. `<a foo="1" >`) has no attribute to
// attach the trailing space to. lexTagAttrs must report ok=false so callers
// keep the verbatim source rather than silently dropping the space on replay.
func TestLexTagAttrs_TrailingWhitespaceForcesVerbatimFallback(t *testing.T) {
	if _, ok := lexTagAttrs([]byte(`a foo="1" `)); ok {
		t.Fatal("expected ok=false for a tag with trailing whitespace before '>' (the space would otherwise be dropped on replay)")
	}
	// The clean, space-free form must still lex to one attribute.
	raws, ok := lexTagAttrs([]byte(`a foo="1"`))
	if !ok || len(raws) != 1 || raws[0] != ` foo="1"` {
		t.Fatalf("clean tag: ok=%v raws=%v", ok, raws)
	}
}

// TestCaptureAttrs_ExtensionNamespacePrefix covers C348 (known C147): an
// attribute in an Office revision/2015+ extension namespace, captured with
// only its URI known (the xmlns declaration lives on an ancestor part root,
// so there is no same-tag declaration to resolve from), must recover its
// conventional prefix from the ExtensionPrefixToNS table. Before the fix
// prefixForAttr returned "" for these namespaces and replay emitted a bare
// local name, silently changing the attribute's namespace.
func TestCaptureAttrs_ExtensionNamespacePrefix(t *testing.T) {
	cases := []struct {
		space, wantPrefix string
	}{
		{NSSpreadsheetRevision, "xr"},
		{NSSpreadsheetRevision2, "xr2"},
		{NSSpreadsheetRevision3, "xr3"},
		{NSSpreadsheet2015Main, "x16r2"},
		{NSPowerPointComment2018, "p188"},
	}
	for _, tc := range cases {
		got := CaptureAttrs([]xml.Attr{
			{Name: xml.Name{Space: tc.space, Local: "uid"}, Value: "{00000000-0000-0000-0000-000000000000}"},
		})
		if len(got) != 1 {
			t.Fatalf("%s: expected 1 captured attr, got %d", tc.space, len(got))
		}
		if got[0].Prefix != tc.wantPrefix {
			t.Errorf("namespace %s: prefix = %q, want %q (bare local name would change the attribute's namespace on replay)", tc.space, got[0].Prefix, tc.wantPrefix)
		}
	}
}

// capAttrsProbe records the attributes CaptureAttrsSource resolves for its
// element, driven through the real decoder/source plumbing.
type capAttrsProbe struct {
	Attrs []RootAttr
}

func (p *capAttrsProbe) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.Attrs = CaptureAttrsSource(d, start.Attr)
	return d.Skip()
}

type capAttrsRoot struct {
	XMLName xml.Name       `xml:"urn:cap root"`
	Child   *capAttrsProbe `xml:"urn:cap child"`
}

// TestCaptureAttrsSource_UnknownNamespaceKeepsSourcePrefix covers the C348
// hardening: an attribute in a genuinely unknown namespace (not in any table,
// declared on an ancestor so no same-tag declaration resolves it) must keep
// the producer's prefix from its verbatim source rendering rather than
// degrade to a bare local name.
func TestCaptureAttrsSource_UnknownNamespaceKeepsSourcePrefix(t *testing.T) {
	src := `<root xmlns="urn:cap" xmlns:zz="urn:unknown-future-ns">` +
		`<child zz:mark="v"/></root>`
	var doc capAttrsRoot
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatalf("UnmarshalWithSource: %v", err)
	}
	if doc.Child == nil || len(doc.Child.Attrs) != 1 {
		t.Fatalf("expected 1 captured attr on child, got %+v", doc.Child)
	}
	ra := doc.Child.Attrs[0]
	if ra.Prefix != "zz" {
		t.Errorf("prefix = %q, want %q (source prefix must survive an unmapped namespace)", ra.Prefix, "zz")
	}
	if ra.Raw != ` zz:mark="v"` {
		t.Errorf("raw = %q, want %q", ra.Raw, ` zz:mark="v"`)
	}
}
