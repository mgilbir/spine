package dml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/testutil"
)

// C375's class in the a14/a16/asvg extension leaves. Every branch of
// Ext.marshalContent resolved its replay prefix from RawAttrPrefix, whose
// fallback is the static well-known prefix. A producer that declares the
// extension namespace on an ancestor under any other prefix — or as the
// default — got a leaf under a prefix nothing in the document binds, on a save
// that changed nothing. #231 saw the gap and deferred it: closing it needed
// #227's live-scope resolver, which did not exist on its base.

const (
	extHolderNS = "urn:example:spPr"
	aliasA16    = "adrw"
	aliasA14    = "d14"
)

type extHolder struct {
	Ext *Ext `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext"`
}

// replayExt parses a holder whose root carries the declarations and re-emits
// the a:ext into a Builder standing in that same live scope.
func replayExt(t *testing.T, src string, rootAttrs []xmlb.RootAttr) string {
	t.Helper()
	var h extHolder
	if err := xmlb.UnmarshalWithSource([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.Ext == nil {
		t.Fatal("a:ext not parsed")
	}
	b := xmlb.NewBuilder()
	b.RegisterNamespace(extHolderNS, "h")
	b.StartElementWithRootAttrs(extHolderNS, "holder", rootAttrs)
	h.Ext.MarshalToBuilder(b, xmlb.NSDrawingML, "ext")
	b.EndElement(extHolderNS, "holder")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v\n%s", err, b.String())
	}
	return b.String()
}

func rootDecls(extra ...xmlb.RootAttr) []xmlb.RootAttr {
	return append([]xmlb.RootAttr{
		{IsNS: true, Prefix: "h", Value: extHolderNS},
		{IsNS: true, Prefix: "a", Value: xmlb.NSDrawingML},
	}, extra...)
}

func TestExt_A16CreationIdUsesTheDeclaredPrefix(t *testing.T) {
	src := `<h:holder xmlns:h="` + extHolderNS + `" xmlns:a="` + xmlb.NSDrawingML + `"` +
		` xmlns:` + aliasA16 + `="` + xmlb.NSDrawing2014 + `">` +
		`<a:ext uri="` + xmlb.ExtURICreationId + `">` +
		`<` + aliasA16 + `:creationId id="{2E7C0C1B-0000-0000-0000-000000000000}"/>` +
		`</a:ext></h:holder>`

	out := replayExt(t, src, rootDecls(xmlb.RootAttr{IsNS: true, Prefix: aliasA16, Value: xmlb.NSDrawing2014}))

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "a16:creationId") {
		t.Errorf("static a16 prefix re-emitted over the source's %s alias:\n%s", aliasA16, out)
	}
	if !strings.Contains(out, "<"+aliasA16+":creationId ") {
		t.Errorf("creationId did not replay under the declared prefix:\n%s", out)
	}
}

func TestExt_A14UseLocalDpiUsesTheDeclaredPrefix(t *testing.T) {
	src := `<h:holder xmlns:h="` + extHolderNS + `" xmlns:a="` + xmlb.NSDrawingML + `"` +
		` xmlns:` + aliasA14 + `="` + xmlb.NSDrawing2010 + `">` +
		`<a:ext uri="` + xmlb.ExtURIUseLocalDpi + `">` +
		`<` + aliasA14 + `:useLocalDpi val="0"/>` +
		`</a:ext></h:holder>`

	out := replayExt(t, src, rootDecls(xmlb.RootAttr{IsNS: true, Prefix: aliasA14, Value: xmlb.NSDrawing2010}))

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "a14:useLocalDpi") {
		t.Errorf("static a14 prefix re-emitted over the source's %s alias:\n%s", aliasA14, out)
	}
}

// asvg:svgBlip carries an r:embed that must survive the prefix correction.
func TestExt_SvgBlipUsesTheDeclaredPrefix(t *testing.T) {
	src := `<h:holder xmlns:h="` + extHolderNS + `" xmlns:a="` + xmlb.NSDrawingML + `"` +
		` xmlns:r="` + xmlb.NSOfficeDocumentRels + `"` +
		` xmlns:svg2="` + xmlb.NSDrawingSVG2016 + `">` +
		`<a:ext uri="` + xmlb.ExtURISvgBlip + `">` +
		`<svg2:svgBlip r:embed="rId7"/>` +
		`</a:ext></h:holder>`

	out := replayExt(t, src, rootDecls(
		xmlb.RootAttr{IsNS: true, Prefix: "r", Value: xmlb.NSOfficeDocumentRels},
		xmlb.RootAttr{IsNS: true, Prefix: "svg2", Value: xmlb.NSDrawingSVG2016},
	))

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "asvg:svgBlip") {
		t.Errorf("static asvg prefix re-emitted over the source's svg2 alias:\n%s", out)
	}
	if !strings.Contains(out, `r:embed="rId7"`) {
		t.Errorf("svgBlip lost its relationship reference:\n%s", out)
	}
}

// A leaf that declares its own namespace keeps its verbatim replay, which is
// the shape every fidelity fixture exercises.
func TestExt_OwnDeclarationOnTheLeafStillReplaysVerbatim(t *testing.T) {
	src := `<h:holder xmlns:h="` + extHolderNS + `" xmlns:a="` + xmlb.NSDrawingML + `">` +
		`<a:ext uri="` + xmlb.ExtURICreationId + `">` +
		`<a16:creationId xmlns:a16="` + xmlb.NSDrawing2014 + `" id="{2E7C0C1B-0000-0000-0000-000000000000}"/>` +
		`</a:ext></h:holder>`

	out := replayExt(t, src, rootDecls())

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	want := `<a16:creationId xmlns:a16="` + xmlb.NSDrawing2014 + `" id="{2E7C0C1B-0000-0000-0000-000000000000}"/>`
	if !strings.Contains(out, want) {
		t.Errorf("self-declaring leaf did not replay verbatim:\ngot  %s\nwant %s", out, want)
	}
}
