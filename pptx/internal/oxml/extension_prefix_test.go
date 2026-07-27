package oxml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/testutil"
)

// C375's class in the PresentationML extension leaves (p14 and p15) and in the
// modern-comment types (p188). Each replayed its captured attribute list under
// a static well-known prefix, so a producer that declares the namespace on an
// ancestor under any other prefix got a leaf whose prefix nothing binds. The
// literal path emits the name verbatim, so Builder.Finish blessed it.

const (
	extHostNS  = "urn:example:spPr"
	aliasP14   = "pp10"
	aliasP15   = "pp12"
	aliasP188  = "pcmt"
	holderName = "holder"
)

type pptxExtHolder struct {
	Ext *Extension `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext"`
}

func replayPptxExt(t *testing.T, src string, rootAttrs []xmlb.RootAttr) string {
	t.Helper()
	var h pptxExtHolder
	if err := xmlb.UnmarshalWithSource([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.Ext == nil {
		t.Fatal("ext not parsed")
	}
	b := xmlb.NewBuilder()
	b.RegisterNamespace(extHostNS, "h")
	b.StartElementWithRootAttrs(extHostNS, holderName, rootAttrs)
	h.Ext.MarshalToBuilder(b, xmlb.NSDrawingML, "ext")
	b.EndElement(extHostNS, holderName)
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v\n%s", err, b.String())
	}
	return b.String()
}

func pptxRootDecls(extra ...xmlb.RootAttr) []xmlb.RootAttr {
	return append([]xmlb.RootAttr{
		{IsNS: true, Prefix: "h", Value: extHostNS},
		{IsNS: true, Prefix: "a", Value: xmlb.NSDrawingML},
	}, extra...)
}

func TestPptxExt_P14CreationIdUsesTheDeclaredPrefix(t *testing.T) {
	src := `<h:` + holderName + ` xmlns:h="` + extHostNS + `" xmlns:a="` + xmlb.NSDrawingML + `"` +
		` xmlns:` + aliasP14 + `="` + xmlb.NSPowerPoint2010 + `">` +
		`<a:ext uri="` + xmlb.ExtURIPMLCreationId + `">` +
		`<` + aliasP14 + `:creationId val="3141592653"/>` +
		`</a:ext></h:` + holderName + `>`

	out := replayPptxExt(t, src, pptxRootDecls(xmlb.RootAttr{IsNS: true, Prefix: aliasP14, Value: xmlb.NSPowerPoint2010}))

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "p14:creationId") {
		t.Errorf("static p14 prefix re-emitted over the source's %s alias:\n%s", aliasP14, out)
	}
	if !strings.Contains(out, `val="3141592653"`) {
		t.Errorf("creationId lost its value:\n%s", out)
	}
}

// defaultImageDpi reaches the literal path through marshalExtLeaf, the shared
// helper behind most p14/p15 leaves.
func TestPptxExt_ExtLeafHelperUsesTheDeclaredPrefix(t *testing.T) {
	src := `<h:` + holderName + ` xmlns:h="` + extHostNS + `" xmlns:a="` + xmlb.NSDrawingML + `"` +
		` xmlns:` + aliasP14 + `="` + xmlb.NSPowerPoint2010 + `">` +
		`<a:ext uri="` + xmlb.ExtURIDefaultImageDpi + `">` +
		`<` + aliasP14 + `:defaultImageDpi val="220"/>` +
		`</a:ext></h:` + holderName + `>`

	out := replayPptxExt(t, src, pptxRootDecls(xmlb.RootAttr{IsNS: true, Prefix: aliasP14, Value: xmlb.NSPowerPoint2010}))

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "p14:defaultImageDpi") {
		t.Errorf("static p14 prefix re-emitted over the source's %s alias:\n%s", aliasP14, out)
	}
}

func TestPptxExt_P15PresenceInfoUsesTheDeclaredPrefix(t *testing.T) {
	src := `<h:` + holderName + ` xmlns:h="` + extHostNS + `" xmlns:a="` + xmlb.NSDrawingML + `"` +
		` xmlns:` + aliasP15 + `="` + xmlb.NSPowerPoint2012 + `">` +
		`<a:ext uri="` + xmlb.ExtURIPresenceInfo + `">` +
		`<` + aliasP15 + `:presenceInfo userId="S::user@example.com::" providerId="AD"/>` +
		`</a:ext></h:` + holderName + `>`

	out := replayPptxExt(t, src, pptxRootDecls(xmlb.RootAttr{IsNS: true, Prefix: aliasP15, Value: xmlb.NSPowerPoint2012}))

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "p15:presenceInfo") {
		t.Errorf("static p15 prefix re-emitted over the source's %s alias:\n%s", aliasP15, out)
	}
}

// p15:sldGuideLst opens an element rather than closing one, so it exercises
// StartElementLiteral's bind list as well as the prefix resolution.
func TestPptxExt_P15SldGuideLstUsesTheDeclaredPrefix(t *testing.T) {
	src := `<h:` + holderName + ` xmlns:h="` + extHostNS + `" xmlns:a="` + xmlb.NSDrawingML + `"` +
		` xmlns:` + aliasP15 + `="` + xmlb.NSPowerPoint2012 + `">` +
		`<a:ext uri="` + xmlb.ExtURISldGuideLst + `">` +
		`<` + aliasP15 + `:sldGuideLst><` + aliasP15 + `:guide id="1" orient="horz" pos="2160"/></` + aliasP15 + `:sldGuideLst>` +
		`</a:ext></h:` + holderName + `>`

	out := replayPptxExt(t, src, pptxRootDecls(xmlb.RootAttr{IsNS: true, Prefix: aliasP15, Value: xmlb.NSPowerPoint2012}))

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "p15:sldGuideLst") {
		t.Errorf("static p15 prefix re-emitted over the source's %s alias:\n%s", aliasP15, out)
	}
}

// A leaf carrying its own declaration keeps its verbatim replay (C523: it must
// not gain a second declaration either).
func TestPptxExt_OwnDeclarationOnTheLeafStillReplaysVerbatim(t *testing.T) {
	src := `<h:` + holderName + ` xmlns:h="` + extHostNS + `" xmlns:a="` + xmlb.NSDrawingML + `">` +
		`<a:ext uri="` + xmlb.ExtURIPMLCreationId + `">` +
		`<p14:creationId xmlns:p14="` + xmlb.NSPowerPoint2010 + `" val="3141592653"/>` +
		`</a:ext></h:` + holderName + `>`

	out := replayPptxExt(t, src, pptxRootDecls())

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	want := `<p14:creationId xmlns:p14="` + xmlb.NSPowerPoint2010 + `" val="3141592653"/>`
	if !strings.Contains(out, want) {
		t.Errorf("self-declaring leaf did not replay verbatim:\ngot  %s\nwant %s", out, want)
	}
}

// --- p188 modern comments ---------------------------------------------------

type modernCmLstHolder struct {
	Comments []*ModernComment `xml:"http://schemas.microsoft.com/office/powerpoint/2018/8/main cm"`
}

// The modern-comment part root declares the p188 namespace, so every cm/reply
// element relies on it and none carries its own declaration — exactly the shape
// the static fallback got wrong the moment a producer picked another prefix.
func TestModernComment_AliasedP188StaysDeclared(t *testing.T) {
	src := `<` + aliasP188 + `:cmLst xmlns:` + aliasP188 + `="` + nsP188 + `">` +
		`<` + aliasP188 + `:cm id="{1}" authorId="{2}" created="2026-07-27T00:00:00.000"/>` +
		`</` + aliasP188 + `:cmLst>`

	var h modernCmLstHolder
	if err := xmlb.UnmarshalWithSource([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(h.Comments) != 1 {
		t.Fatalf("parsed %d comments, want 1", len(h.Comments))
	}

	b := xmlb.NewBuilder()
	b.RegisterNamespace(nsAdml, xmlb.PrefixDrawingML)
	b.RegisterNamespace(nsP188, aliasP188)
	b.StartElementWithRootAttrs(nsP188, "cmLst", []xmlb.RootAttr{
		{IsNS: true, Prefix: xmlb.PrefixDrawingML, Value: nsAdml},
		{IsNS: true, Prefix: aliasP188, Value: nsP188},
	})
	h.Comments[0].marshal(b)
	b.EndElement(nsP188, "cmLst")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v\n%s", err, b.String())
	}

	out := b.String()
	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "p188:cm") {
		t.Errorf("static p188 prefix re-emitted over the source's %s alias:\n%s", aliasP188, out)
	}
}
