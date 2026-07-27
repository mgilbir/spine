package chart

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/testutil"
)

// C375's class, reached through c:chart. RelId replayed its captured attribute
// list under RawAttrPrefix's static "c" fallback, and its own comment records
// why that is not enough: "some sources omit xmlns:c, relying on the root". A
// root that binds the chart namespace to any other prefix — or declares it as
// the default — therefore got <c:chart> with c bound to nothing, on a save that
// changed no content. The literal path writes the name verbatim, so neither
// Builder.Finish nor a plain well-formedness check noticed.

const gdNS = "urn:example:graphicdata"

type chartRefHolder struct {
	Chart *RelId `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart chart"`
}

// replayChartRef parses a graphicData whose root carries the declarations and
// re-emits it into a Builder in that same live scope, as a save does.
func replayChartRef(t *testing.T, src string, rootAttrs []xmlb.RootAttr) string {
	t.Helper()
	var h chartRefHolder
	if err := xmlb.UnmarshalWithSource([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.Chart == nil {
		t.Fatal("c:chart not parsed")
	}
	b := xmlb.NewBuilder()
	b.RegisterNamespace(gdNS, "gd")
	b.StartElementWithRootAttrs(gdNS, "graphicData", rootAttrs)
	h.Chart.MarshalToBuilder(b, xmlb.NSDrawingMLChart, "chart")
	b.EndElement(gdNS, "graphicData")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v\n%s", err, b.String())
	}
	return b.String()
}

func TestRelId_AliasedChartNamespaceStaysDeclared(t *testing.T) {
	src := `<gd:graphicData xmlns:gd="` + gdNS + `" xmlns:chrt="` + xmlb.NSDrawingMLChart + `"` +
		` xmlns:r="` + xmlb.NSOfficeDocumentRels + `">` +
		`<chrt:chart r:id="rId3"/></gd:graphicData>`

	out := replayChartRef(t, src, []xmlb.RootAttr{
		{IsNS: true, Prefix: "gd", Value: gdNS},
		{IsNS: true, Prefix: "chrt", Value: xmlb.NSDrawingMLChart},
		{IsNS: true, Prefix: "r", Value: xmlb.NSOfficeDocumentRels},
	})

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "<c:chart") {
		t.Errorf("hardcoded c prefix re-emitted over the source's chrt alias:\n%s", out)
	}
	if !strings.Contains(out, `<chrt:chart r:id="rId3"`) {
		t.Errorf("chart reference did not replay under the source's prefix:\n%s", out)
	}
}

func TestRelId_DefaultDeclaredChartNamespaceStaysUnprefixed(t *testing.T) {
	src := `<gd:graphicData xmlns:gd="` + gdNS + `" xmlns="` + xmlb.NSDrawingMLChart + `"` +
		` xmlns:r="` + xmlb.NSOfficeDocumentRels + `">` +
		`<chart r:id="rId3"/></gd:graphicData>`

	out := replayChartRef(t, src, []xmlb.RootAttr{
		{IsNS: true, Prefix: "gd", Value: gdNS},
		{IsNS: true, Prefix: "", Value: xmlb.NSDrawingMLChart},
		{IsNS: true, Prefix: "r", Value: xmlb.NSOfficeDocumentRels},
	})

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Contains(out, "c:chart") {
		t.Errorf("undeclared c prefix emitted for a default-declared namespace:\n%s", out)
	}
	if !strings.Contains(out, `<chart r:id="rId3"`) {
		t.Errorf("chart reference did not replay unprefixed:\n%s", out)
	}
}

// The element's own declaration keeps winning, so the ordinary shape — the one
// every fidelity fixture exercises — still round-trips verbatim.
func TestRelId_OwnDeclarationStillReplaysVerbatim(t *testing.T) {
	src := `<gd:graphicData xmlns:gd="` + gdNS + `">` +
		`<c:chart xmlns:c="` + xmlb.NSDrawingMLChart + `" xmlns:r="` + xmlb.NSOfficeDocumentRels + `" r:id="rId3"/>` +
		`</gd:graphicData>`

	out := replayChartRef(t, src, []xmlb.RootAttr{{IsNS: true, Prefix: "gd", Value: gdNS}})

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	want := `<c:chart xmlns:c="` + xmlb.NSDrawingMLChart + `" xmlns:r="` + xmlb.NSOfficeDocumentRels + `" r:id="rId3"/>`
	if !strings.Contains(out, want) {
		t.Errorf("self-declaring chart reference did not replay verbatim:\ngot  %s\nwant %s", out, want)
	}
}
