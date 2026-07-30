package vml

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/internal/fuzzbound"
)

// vmlDecls are the prefix bindings the root of a VML part carries — the legacy
// <xml> root of a vmlDrawing part, or the w:document root a w:pict sits under.
// A fragment is decoded with them injected onto its own start tag so the
// fuzzer's mutations land on the content rather than on the declarations, and
// so a prefix the input uses is always bound to the URI the model expects.
const vmlDecls = ` xmlns:v="urn:schemas-microsoft-com:vml"` +
	` xmlns:o="urn:schemas-microsoft-com:office:office"` +
	` xmlns:x="urn:schemas-microsoft-com:office:excel"` +
	` xmlns:w10="urn:schemas-microsoft-com:office:word"` +
	` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
	` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

// vmlBudget bounds one parse of a VML fragment.
//
// Nothing on this path scales super-linearly with its input by design — the
// verbatim captures (v:textbox local children, w:txbxContent) copy a subtree
// once — so the budget exists to notice a change that makes one of them
// quadratic, and to fail a pathological nesting depth as a finding rather than
// as an OOM kill nobody attributes to the parser. Measured on the corpus'
// largest legacy drawings the parse allocates well under 30x the fragment size
// once the decoder's fixed buffers are accounted for; a 1 MiB floor absorbs
// those and a 64x rate leaves ample room for a legitimately attribute-dense
// shape while staying orders of magnitude below an unbounded blow-up.
var vmlBudget = fuzzbound.Budget{
	What:              "vml parse",
	Bytes:             1 << 20,
	BytesPerInputByte: 64,
	Time:              10 * time.Second,
	TimePerMiB:        5 * time.Second,
}

// withVMLDecls injects vmlDecls onto the fragment's own start tag, which must
// be tag. It returns false when the input is not a fragment rooted at that
// element, which is the only input shape either target has anything to say
// about.
func withVMLDecls(src, tag string) (string, bool) {
	if !strings.HasPrefix(src, tag) {
		return "", false
	}
	rest := src[len(tag):]
	if rest == "" || !strings.ContainsAny(rest[:1], " \t\r\n/>") {
		return "", false // <v:shapetype…> is not <v:shape…>
	}
	return tag + vmlDecls + rest, true
}

// roundTripVML runs one parse/marshal cycle over src, returning the marshaled
// bytes and the value they came from, or ok=false when a step failed.
func roundTripVML[T any](src string, v *T) (string, bool) {
	if err := xml.Unmarshal([]byte(src), v); err != nil {
		return "", false
	}
	out, err := xml.Marshal(v)
	if err != nil {
		return "", false
	}
	return string(out), true
}

// FuzzShapeRoundTrip drives the v:shape model — the type Excel comments, form
// controls, OLE pictures and Word watermarks all parse into, and the one most
// likely to be malformed in the field because VML predates the strict OOXML
// tooling by a decade.
//
// The oracle is a fixed point after one round trip: whatever the first
// marshal normalizes, parsing that output and marshaling again must reproduce
// it byte for byte. A value that keeps changing is a parser and serializer
// that disagree about what the model means. The property was verified to hold
// on all 1927 distinct v:shape elements in the legacy drawings of the 3600-file
// corpus before this target was written, so a failure here is a regression
// rather than a shape the model never handled.
//
// Three properties deliberately *not* asserted, each because the package's
// documented contract excludes it rather than because it is untested:
//
//   - Byte identity with the input. Child order normalizes to schema order
//     (VML content models are xs:choice) and unmodeled attributes and elements
//     are dropped, both by design; see the package comment.
//   - Namespace well-formedness of the output. A w:txbxContent body is
//     captured opaquely, so marshal can only re-declare the prefixes in
//     txbxContentPrefixes; a producer prefix outside that table is emitted
//     unbound. That is the documented limit of an ",innerxml" capture, not a
//     defect this target should report on every mutation that invents a prefix.
//   - "The output invents no attribute the input did not have." This one does
//     not hold, and the reason is a live defect rather than a design boundary:
//     encoding/xml matches an attribute against every field whose tag names it,
//     and a tag with no namespace matches *any* namespace. So `v:fill r:id=…`
//     populates both Fill.RID and Fill.ID, and marshal then writes an
//     `id` attribute the source never had — likewise Stroke.ID, ImageData.ID,
//     Shape.Spt against o:spt, and Shape.ConnectorType against o:connectortype.
//     A `<v:imagedata r:id="rId4"/>`, which is how every Word watermark and
//     inline legacy picture is written, comes back as
//     `<imagedata id="rId4" r:id="rId4"/>`. It is a stable invention, so the
//     fixed point above does not see it; fixing it needs custom unmarshalers on
//     the affected types and is out of this change's scope.
func FuzzShapeRoundTrip(f *testing.F) {
	// Seeds are the shapes real producers write, reduced from the legacy
	// drawings of the corpus: an Excel comment (textbox + ClientData), a form
	// control, an embedded picture, a Word-anchored shape with a wrap, and a
	// watermark with WordArt text.
	f.Add(`<v:shape id="_x0000_s1029" type="#_x0000_t202" style='position:absolute;visibility:hidden' fillcolor="#ffffe1" o:insetmode="auto">` +
		`<v:fill color2="#ffffe1"/><v:textbox><div style='text-align:left'></div></v:textbox>` +
		`<x:ClientData ObjectType="Note"><x:MoveWithCells/><x:SizeWithCells/>` +
		`<x:Anchor>3, 95, 0, 0, 3, 387, 3, 77</x:Anchor><x:Row>1</x:Row><x:Column>3</x:Column></x:ClientData></v:shape>`)
	f.Add(`<v:shape id="_x0000_s19459" type="#_x0000_t201" style='position:absolute;mso-wrap-style:tight' filled="f" fillcolor="white [9]" stroked="f" o:insetmode="auto">` +
		`<v:path shadowok="t" strokeok="t" fillok="t"/><o:lock v:ext="edit" rotation="t"/>` +
		`<v:textbox o:singleclick="f"><div style='text-align:left'></div></v:textbox>` +
		`<x:ClientData ObjectType="Checkbox"><x:SizeWithCells/><x:Anchor>6, 11, 6, 14, 7, 11, 8, 5</x:Anchor>` +
		`<x:AutoFill>False</x:AutoFill><x:AutoLine>False</x:AutoLine><x:TextVAlign>Center</x:TextVAlign>` +
		`<x:FmlaLink>$G$8</x:FmlaLink><x:NoThreeD/></x:ClientData></v:shape>`)
	f.Add(`<v:shape id="LH" o:spid="_x0000_s1025" type="#_x0000_t75" style='position:absolute;z-index:1'>` +
		`<v:imagedata o:relid="rId1" o:title="MMR"/><o:lock v:ext="edit" rotation="t"/></v:shape>`)
	f.Add(`<v:shape id="shape_0" fillcolor="white" stroked="t" type="#_x0000_t202" style="position:absolute">` +
		`<v:fill o:detectmouseclick="t" type="solid" color2="black"/><v:shadow on="t" obscured="t" color="black"/>` +
		`<w10:wrap type="none"/><w10:anchorlock/>` +
		`<v:stroke color="black" startarrow="block" joinstyle="miter" endcap="flat"/></v:shape>`)
	f.Add(`<v:shape id="PowerPlusWaterMarkObject" o:spid="_x0000_s2049" type="#_x0000_t136" o:gfxdata="UEsDBBQ" style='position:absolute' fillcolor="silver" stroked="f">` +
		`<v:fill opacity=".5"/><v:textpath style='font-family:"Calibri"' string="DRAFT"/></v:shape>`)
	f.Add(`<v:shape id="s" type="#t202"><v:textbox><w:txbxContent><w:p><w:r><w:t>hi</w:t></w:r></w:p></w:txbxContent></v:textbox></v:shape>`)
	f.Add(`<v:shape/>`)

	f.Fuzz(func(t *testing.T, src string) {
		full, ok := withVMLDecls(src, "<v:shape")
		if !ok || fuzzbound.Tripped() {
			return
		}
		var first string
		vmlBudget.Check(t, len(full), func() {
			var s Shape
			first, ok = roundTripVML(full, &s)
		})
		if !ok {
			return
		}
		var back Shape
		second, ok := roundTripVML(first, &back)
		if !ok {
			t.Fatalf("the output of a successful parse+marshal must itself parse and marshal:\n%s", first)
		}
		if first != second {
			t.Fatalf("marshal is not a fixed point after normalization:\n%s\n%s", first, second)
		}
	})
}

// FuzzGroupRoundTrip drives v:group, which is a shape *and* a container: it
// carries the same fill/stroke/textbox/lock/wrap/ClientData children as
// v:shape and nests further groups and shapes inside itself. The recursion is
// what makes it worth a target of its own — it is the one place in this package
// where a mutation can grow the model super-linearly in the input — and the
// nested-container path is where children were silently dropped before.
//
// Same oracle and same documented exclusions as FuzzShapeRoundTrip.
func FuzzGroupRoundTrip(f *testing.F) {
	f.Add(`<v:group id="g1" style="position:absolute;width:200pt;height:100pt" coordsize="21600,21600" coordorigin="0,0">` +
		`<v:shape id="s1" type="#_x0000_t75" style="position:absolute"><v:imagedata r:id="rId4" o:title=""/></v:shape>` +
		`<v:rect id="r1" style="position:absolute" fillcolor="#c0c0c0"><v:fill on="t"/></v:rect>` +
		`<v:oval id="o1" style="position:absolute"/><v:line id="l1" from="0,0" to="10,10"/>` +
		`<o:lock v:ext="edit" grouping="t"/><w10:wrap type="topAndBottom"/></v:group>`)
	f.Add(`<v:group id="g2"><v:group id="g3"><v:group id="g4"><v:shape id="s"/></v:group></v:group>` +
		`<v:shapetype id="_x0000_t75" coordsize="21600,21600" o:spt="75" path="m@4@5l@4@11@9@11@9@5xe" filled="f" stroked="f">` +
		`<v:stroke joinstyle="miter"/><v:formulas><v:f eqn="if lineDrawn pixelLineWidth 0"/><v:f eqn="sum @0 1 0"/></v:formulas>` +
		`<v:path o:extrusionok="f" gradientshapeok="t" o:connecttype="rect"/><o:lock v:ext="edit" aspectratio="t"/></v:shapetype></v:group>`)
	f.Add(`<v:group id="g5"><v:textbox inset="0,0,0,0"><div/><div style='text-align:right'/></v:textbox>` +
		`<x:ClientData ObjectType="Group"><x:Visible/><x:Anchor>1, 2, 3, 4</x:Anchor></x:ClientData></v:group>`)
	f.Add(`<v:group/>`)

	f.Fuzz(func(t *testing.T, src string) {
		full, ok := withVMLDecls(src, "<v:group")
		if !ok || fuzzbound.Tripped() {
			return
		}
		var first string
		vmlBudget.Check(t, len(full), func() {
			var g Group
			first, ok = roundTripVML(full, &g)
		})
		if !ok {
			return
		}
		var back Group
		second, ok := roundTripVML(first, &back)
		if !ok {
			t.Fatalf("the output of a successful parse+marshal must itself parse and marshal:\n%s", first)
		}
		if first != second {
			t.Fatalf("marshal is not a fixed point after normalization:\n%s\n%s", first, second)
		}
	})
}
