package oxml

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// acHolder parses a document whose root carries the namespace declarations and
// whose single child is the mc:AlternateContent under test, mirroring how the
// format packages reach this type.
type acHolder struct {
	AC *AlternateContent `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 AlternateContent"`
}

// namespaceWellFormed reports the first element or attribute written with a
// prefix that no in-scope declaration binds. Go's decoder accepts such input
// (it reports the bare prefix as the name's Space), so neither plain
// well-formedness nor Builder.Finish catches it; this does.
func namespaceWellFormed(s string) error {
	d := xml.NewDecoder(strings.NewReader(s))
	declared := map[string]bool{"http://www.w3.org/XML/1998/namespace": true}
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
			return errUnbound(start.Name.Space + ":" + start.Name.Local)
		}
		for _, a := range start.Attr {
			if a.Name.Space == "" || a.Name.Space == "xmlns" {
				continue
			}
			if !declared[a.Name.Space] {
				return errUnbound(a.Name.Space + ":" + a.Name.Local)
			}
		}
	}
}

type errUnbound string

func (e errUnbound) Error() string { return "unbound namespace prefix on " + string(e) }

// replayAC re-emits a parsed holder: the root replays its captured
// declarations verbatim (as saveRoundTrip does), then the AlternateContent
// marshals itself inside that live namespace scope.
func replayAC(t *testing.T, src string, rootNS string, rootAttrs []xmlb.RootAttr) string {
	t.Helper()
	var h acHolder
	if err := xmlb.UnmarshalWithSource([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.AC == nil {
		t.Fatal("AlternateContent not parsed")
	}
	b := xmlb.NewBuilder()
	b.RegisterNamespace(rootNS, "w")
	b.StartElementWithRootAttrs(rootNS, "document", rootAttrs)
	h.AC.MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "AlternateContent")
	b.EndElement(rootNS, "document")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	return b.String()
}

const (
	wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	// vNS stands in for an extension namespace the Choice content uses.
	vNS = "urn:example:v"
)

// C375: Word 2007 aliases the markup-compatibility namespace as ve: and
// declares it on the document root, so the AlternateContent element itself
// carries no declaration. The marshal resolved its prefix through
// RawAttrPrefix, whose fallback is the hardcoded "mc" — re-emitting
// <mc:AlternateContent> with mc never declared. The output is malformed XML on
// a zero-modification save, and Finish() returns nil because literal emission
// bypasses writeQName's unbound-prefix check.
func TestAlternateContent_AliasedPrefixStaysDeclared(t *testing.T) {
	src := `<w:document xmlns:w="` + wNS + `" xmlns:v="` + vNS + `" xmlns:ve="` + xmlb.NSMarkupCompatibility + `">` +
		`<ve:AlternateContent><ve:Choice Requires="v"><v:x/></ve:Choice>` +
		`<ve:Fallback><w:y/></ve:Fallback></ve:AlternateContent></w:document>`

	out := replayAC(t, src, wNS, []xmlb.RootAttr{
		{IsNS: true, Prefix: "w", Value: wNS},
		{IsNS: true, Prefix: "v", Value: vNS},
		{IsNS: true, Prefix: "ve", Value: xmlb.NSMarkupCompatibility},
	})

	if err := namespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "mc:AlternateContent") {
		t.Errorf("hardcoded mc prefix re-emitted over the source's ve alias:\n%s", out)
	}
	for _, want := range []string{"<ve:AlternateContent>", "<ve:Choice ", "<ve:Fallback>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// C375: a source that declares the markup-compatibility namespace as the
// default (xmlns=) writes its AlternateContent unprefixed. RawAttrPrefix skips
// default declarations entirely, so this hit the same hardcoded fallback.
func TestAlternateContent_DefaultDeclaredNamespaceStaysUnprefixed(t *testing.T) {
	src := `<w:document xmlns:w="` + wNS + `" xmlns:v="` + vNS + `">` +
		`<AlternateContent xmlns="` + xmlb.NSMarkupCompatibility + `">` +
		`<Choice Requires="v"><v:x/></Choice><Fallback/></AlternateContent></w:document>`

	out := replayAC(t, src, wNS, []xmlb.RootAttr{{IsNS: true, Prefix: "w", Value: wNS}, {IsNS: true, Prefix: "v", Value: vNS}})

	if err := namespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "mc:") {
		t.Errorf("undeclared mc prefix emitted for a default-declared namespace:\n%s", out)
	}
	want := `<AlternateContent xmlns="` + xmlb.NSMarkupCompatibility + `">` +
		`<Choice Requires="v"><v:x/></Choice><Fallback></Fallback></AlternateContent>`
	if !strings.Contains(out, want) {
		t.Errorf("default-namespace AlternateContent did not replay verbatim:\ngot  %s\nwant %s", out, want)
	}
}

// C375: an AlternateContent whose own attribute list declares the namespace
// still round-trips verbatim, and the declaration stays on the element that
// carried it.
func TestAlternateContent_OwnDeclarationStillReplays(t *testing.T) {
	src := `<w:document xmlns:w="` + wNS + `" xmlns:v="` + vNS + `">` +
		`<mc:AlternateContent xmlns:mc="` + xmlb.NSMarkupCompatibility + `">` +
		`<mc:Choice Requires="v"><v:x/></mc:Choice></mc:AlternateContent></w:document>`

	out := replayAC(t, src, wNS, []xmlb.RootAttr{{IsNS: true, Prefix: "w", Value: wNS}, {IsNS: true, Prefix: "v", Value: vNS}})
	if err := namespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	want := `<mc:AlternateContent xmlns:mc="` + xmlb.NSMarkupCompatibility + `">` +
		`<mc:Choice Requires="v"><v:x/></mc:Choice></mc:AlternateContent>`
	if !strings.Contains(out, want) {
		t.Errorf("self-declaring AlternateContent did not replay verbatim:\ngot  %s\nwant %s", out, want)
	}
}

// C476: mc:Choice and mc:Fallback captured their attributes with CaptureAttrs
// rather than CaptureAttrsSource, so a producer writing Requires='p14' in
// single quotes (or with unusual spacing) had its rendering re-canonicalized
// and the save was no longer byte-identical. The decoder is positioned right
// past each child's start tag, so the verbatim rendering is recoverable there.
func TestAlternateContent_ChoiceAttrRenderingPreserved(t *testing.T) {
	src := `<w:document xmlns:w="` + wNS + `" xmlns:v="` + vNS + `" xmlns:mc="` + xmlb.NSMarkupCompatibility + `">` +
		`<mc:AlternateContent><mc:Choice  Requires='v'><v:x/></mc:Choice>` +
		`<mc:Fallback  xmlns=''><w:y/></mc:Fallback></mc:AlternateContent></w:document>`

	out := replayAC(t, src, wNS, []xmlb.RootAttr{
		{IsNS: true, Prefix: "w", Value: wNS},
		{IsNS: true, Prefix: "v", Value: vNS},
		{IsNS: true, Prefix: "mc", Value: xmlb.NSMarkupCompatibility},
	})

	if !strings.Contains(out, `<mc:Choice  Requires='v'>`) {
		t.Errorf("mc:Choice attribute rendering was re-canonicalized:\n%s", out)
	}
	if !strings.Contains(out, `<mc:Fallback  xmlns=''>`) {
		t.Errorf("mc:Fallback attribute rendering was re-canonicalized:\n%s", out)
	}
}
