package xml

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// C239: a part declaring a non-UTF-8 charset makes CharsetReader transcode the
// stream, so the decoder's InputOffset indexes the transcoded bytes while the
// registered source is the original. Offset-based capture would then slice
// garbage and replay it. UnmarshalWithSource must not register the source for
// such parts, so capture degrades cleanly to canonical regeneration.
func TestUnmarshalWithSource_TranscodedCharsetSkipsCapture(t *testing.T) {
	// Three high bytes (0xE9 -> é) before the unknown child shift every
	// transcoded offset by three relative to the original source bytes.
	src := `<?xml version="1.0" encoding="windows-1252"?>` +
		`<w:root xmlns:w="http://example.com/w" xmlns:x="http://example.com/x">` +
		"<w:props><w:sz w:val=\"\xe9\xe9\xe9\"/><x:unknown/></w:props>" +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatalf("UnmarshalWithSource: %v", err)
	}
	got := marshalOrderedDoc(t, &doc)
	// Source not registered: the unknown child degrades to the no-source path
	// (dropped) and the typed w:sz decodes normally — no garbage span.
	want := `<w:props><w:sz w:val="ééé"/></w:props>`
	if got != want {
		t.Errorf("transcoded capture produced garbage:\n got %q\nwant %q", got, want)
	}
	// Must be well-formed regardless.
	if err := wellFormed(got); err != nil {
		t.Errorf("output not well-formed: %v\n%s", err, got)
	}
}

// C239 control: a normal UTF-8 part still registers its source, so an unknown
// child is preserved byte-identically.
func TestUnmarshalWithSource_UTF8StillCaptures(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w" xmlns:x="http://example.com/x">` +
		"<w:props><x:unknown a=\"café\"/></w:props>" +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatalf("UnmarshalWithSource: %v", err)
	}
	got := marshalOrderedDoc(t, &doc)
	want := "<w:props><x:unknown a=\"café\"/></w:props>"
	if got != want {
		t.Errorf("utf-8 capture mismatch:\n got %q\nwant %q", got, want)
	}
}

// C280: CaptureProlog must not mistake a "</" inside a trailing comment for the
// document element's end tag.
func TestCaptureProlog_TrailingCommentWithSlash(t *testing.T) {
	data := `<root></root><!-- made </by> tool -->`
	p := CaptureProlog([]byte(data))
	if p.RootEnd != "" {
		t.Errorf("RootEnd = %q, want \"\" (canonical close, not a slice of the comment)", p.RootEnd)
	}
	if want := `<!-- made </by> tool -->`; p.Trailer != want {
		t.Errorf("Trailer = %q, want %q", p.Trailer, want)
	}
	// Reconstruction is byte-identical.
	b := NewBuilder()
	b.RegisterNamespace("", "")
	b.WriteProlog(p)
	b.SetRootEndTag(p.RootEnd)
	b.StartElement("", "root")
	b.EndElement("", "root")
	b.WriteTrailer(p)
	if got := b.String(); got != data {
		t.Errorf("round trip = %q, want %q", got, data)
	}
}

// C280: a genuine non-canonical root end tag (whitespace before '>') is still
// captured verbatim.
func TestCaptureProlog_NonCanonicalRootEndPreserved(t *testing.T) {
	data := "<root></root >\n"
	p := CaptureProlog([]byte(data))
	if want := "</root >"; p.RootEnd != want {
		t.Errorf("RootEnd = %q, want %q", p.RootEnd, want)
	}
	if want := "\n"; p.Trailer != want {
		t.Errorf("Trailer = %q, want %q", p.Trailer, want)
	}
}

// C281: ordered-child capture must preserve interleaved comments and PIs, not
// silently drop them.
func TestOrderedChildren_PreservesCommentsAndPIs(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><!-- note --><w:b/><?pi data?><w:i/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatalf("UnmarshalWithSource: %v", err)
	}
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><!-- note --><w:b/><?pi data?><w:i/></w:props>`
	if got != want {
		t.Errorf("comment/PI replay mismatch:\n got %s\nwant %s", got, want)
	}
}

// C282: captured raw child bytes must not alias the source buffer, or one
// captured child pins the whole part in memory for the model's lifetime.
func TestOrderedChildren_RawChildDoesNotAliasSource(t *testing.T) {
	src := []byte(`<w:root xmlns:w="http://example.com/w" xmlns:x="http://example.com/x">` +
		`<w:props><x:unknown/></w:props>` +
		`</w:root>`)
	var doc orderedDoc
	if err := UnmarshalWithSource(src, &doc); err != nil {
		t.Fatalf("UnmarshalWithSource: %v", err)
	}
	cc := doc.Props.CapturedChildren
	if cc == nil || len(cc.Raw) == 0 {
		t.Fatal("expected a captured raw child")
	}
	before := string(cc.Raw[0])
	// Scribble over the original backing array; a clone is unaffected.
	for i := range src {
		src[i] = 'Z'
	}
	if after := string(cc.Raw[0]); after != before {
		t.Errorf("captured Raw aliases source buffer: before %q, after mutation %q", before, after)
	}
}

// C279: an inline-NS element that aliases an already-bound prefix (Word writes
// mc as "ve") must not leave that binding in place; a later sibling in the same
// namespace has to emit the in-scope prefix.
func TestBuilder_InlineNSPrefixBindingRestored(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace(NSMarkupCompatibility, PrefixMarkupCompatibility) // mc
	b.StartElementInlineNS(NSMarkupCompatibility, "ve", "AlternateContent")
	b.EndElementInlineNS("ve", "AlternateContent")
	b.StartElement(NSMarkupCompatibility, "Fallback")
	b.EndElement(NSMarkupCompatibility, "Fallback")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	out := b.String()
	if strings.Contains(out, "ve:Fallback") {
		t.Errorf("later element emitted the out-of-scope inline prefix:\n%s", out)
	}
	if !strings.Contains(out, PrefixMarkupCompatibility+":Fallback") {
		t.Errorf("later element did not emit the in-scope prefix:\n%s", out)
	}
}

// C279: StartElementLiteral binds prefixes the same way and must also restore
// them on close.
func TestBuilder_LiteralNSPrefixBindingRestored(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace(NSMarkupCompatibility, PrefixMarkupCompatibility) // mc
	b.StartElementLiteral("ve", "AlternateContent",
		[]NSDecl{{Prefix: "ve", URI: NSMarkupCompatibility}},
		Attr{Name: "xmlns:ve", Value: NSMarkupCompatibility})
	b.EndElementLiteral("ve", "AlternateContent")
	b.StartElement(NSMarkupCompatibility, "Fallback")
	b.EndElement(NSMarkupCompatibility, "Fallback")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if out := b.String(); strings.Contains(out, "ve:Fallback") {
		t.Errorf("literal inline prefix leaked past its element:\n%s", out)
	}
}

// C279: when the inline namespace had no prior binding, closing the element must
// remove the binding entirely (not leave a dangling, out-of-scope prefix), so a
// later use in that namespace is flagged rather than silently prefixed.
func TestBuilder_InlineNSPrefixBindingRemovedWhenUnbound(t *testing.T) {
	const ns = "http://example.com/ext"
	b := NewBuilder()
	b.StartElementInlineNS(ns, "p14", "ext")
	b.EndElementInlineNS("p14", "ext")
	b.StartElement(ns, "after")
	b.EndElement(ns, "after")
	if err := b.Finish(); err == nil {
		t.Error("out-of-scope inline binding left in place: expected a no-prefix error")
	}
}

// wellFormed reports the first XML syntax error in s (unbalanced or malformed
// tags), or nil when the whole token stream parses.
func wellFormed(s string) error {
	d := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// A captured root end tag has to be the root's end tag.
//
// The capture exists to preserve a producer's odd spacing — "</p:sld >" is
// written by real tools and has to come back byte for byte — and it decides
// what to keep by looking at the end of the part. Two things can sit there that
// are not the document element's close: a trailing comment containing "</"
// (C280, covered above), and, as the nightly fuzz run found, a part whose root
// is self-closing followed by something that merely looks like a close.
//
// "<A/></ >" is the second: a self-closing root and then garbage, where "</ >"
// passes a whitespace test because it has a space where the name should be.
// Replaying it wrote a slide master with an opening tag and no matching close,
// and the package this library had just written would not re-open.
func TestCaptureProlog_RootEndMustNameTheRoot(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		// Kept: the non-canonical spacing this capture exists for.
		"space before the close":   {`<p:sld xmlns:p="u"></p:sld >`, `</p:sld >`},
		"newline before the close": {"<root></root\n>", "</root\n>"},

		// Not kept: canonical, so the builder's own close stays in charge.
		"canonical close": {`<p:sld xmlns:p="u"></p:sld>`, ``},

		// Not kept: not the root's end tag at all.
		"garbage after a self-closing root": {`<A/></ >`, ``},
		"a different element's close":       {`<root></other >`, ``},
		"a close inside a trailing comment": {`<root></root><!-- </by> -->`, ``},
		"nothing but a close":               {`</ >`, ``},
	} {
		t.Run(name, func(t *testing.T) {
			if got := CaptureProlog([]byte(tc.in)).RootEnd; got != tc.want {
				t.Errorf("RootEnd = %q, want %q", got, tc.want)
			}
		})
	}
}
