package dml

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Several types in this package carry *two* marshalers: MarshalToBuilder for
// the production Builder, and MarshalXML for encoding/xml. The second exists
// only because the stdlib path would otherwise reconstruct the element from
// struct tags and lose what the type was written to preserve — positional fill
// order (C482), effect-child document order, blip effect children. It is the
// path anything that reaches these values through xml.Marshal takes: a wrapper
// struct, a nested model, a test fixture.
//
// None of these stdlib marshalers executed anywhere in the module. That is the
// dangerous shape: the Builder path is well covered, so the two can drift apart
// indefinitely and only a document produced through the other path would show
// it. Every test here therefore asserts the stdlib output against the *source*,
// and asserts the two paths agree.

// childNames returns the local names of a marshaled element's direct children,
// in document order.
func childNames(t *testing.T, doc []byte) []string {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(string(doc)))
	var names []string
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				names = append(names, tk.Name.Local)
			}
		case xml.EndElement:
			depth--
		}
	}
	return names
}

// refNames renders a captured fill order as "kind#index" strings.
func refNames(refs []fillChoiceRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, fmt.Sprintf("%s#%d", fillChoiceKindName[r.kind], r.index))
	}
	return out
}

// A theme fill style list that interleaves kinds against the model's field
// order, which is the only arrangement that can tell document order from
// grouped order. Grouping this by kind would yield
// noFill, solidFill×2, gradFill×2, pattFill — a different sequence at every
// position but the last, so every a:fillRef/@idx in the document would resolve
// to a different fill.
const interleavedFills = `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>` +
	`<a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:srgbClr val="00FF00"/></a:gs></a:gsLst></a:gradFill>` +
	`<a:noFill/>` +
	`<a:solidFill><a:srgbClr val="0000FF"/></a:solidFill>` +
	`<a:pattFill prst="pct5"><a:fgClr><a:srgbClr val="111111"/></a:fgClr>` +
	`<a:bgClr><a:srgbClr val="222222"/></a:bgClr></a:pattFill>` +
	`<a:gradFill><a:gsLst><a:gs pos="100000"><a:srgbClr val="333333"/></a:gs></a:gsLst></a:gradFill>`

var interleavedFillOrder = []string{
	"solidFill#0", "gradFill#0", "noFill#0", "solidFill#1", "pattFill#0", "gradFill#1",
}

func fillListSrc(local string) string {
	return `<a:` + local + ` xmlns:a="` + NsDrawingML + `">` + interleavedFills + `</a:` + local + `>`
}

// The encoding/xml path must replay the same positional order the Builder path
// does. A regression here silently repoints every styled shape in the file.
func TestFillStyleLst_StdlibMarshalKeepsPositionalOrder(t *testing.T) {
	for _, tc := range []struct {
		local string
		parse func([]byte) (interface{}, []fillChoiceRef, error)
	}{
		{"fillStyleLst", func(b []byte) (interface{}, []fillChoiceRef, error) {
			var v FillStyleLst
			err := xml.Unmarshal(b, &v)
			return &v, v.fillOrder, err
		}},
		{"bgFillStyleLst", func(b []byte) (interface{}, []fillChoiceRef, error) {
			var v BgFillStyleLst
			err := xml.Unmarshal(b, &v)
			return &v, v.fillOrder, err
		}},
	} {
		t.Run(tc.local, func(t *testing.T) {
			src := fillListSrc(tc.local)
			v, order, err := tc.parse([]byte(src))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := refNames(order); !equalStrings(got, interleavedFillOrder) {
				t.Fatalf("parse captured order %v, want %v", got, interleavedFillOrder)
			}

			out, err := xml.Marshal(v)
			if err != nil {
				t.Fatalf("xml.Marshal: %v", err)
			}
			if got := childNames(t, out); !equalStrings(got, kindsOf(interleavedFillOrder)) {
				t.Errorf("encoding/xml path emitted %v, want %v (positional order lost: "+
					"every a:fillRef/@idx now selects a different fill)", got, kindsOf(interleavedFillOrder))
			}

			// Re-parsing the stdlib output must reproduce the same order, which
			// is what a consumer downstream of an xml.Marshal actually sees.
			_, order2, err := tc.parse(out)
			if err != nil {
				t.Fatalf("re-parse: %v", err)
			}
			if got := refNames(order2); !equalStrings(got, interleavedFillOrder) {
				t.Errorf("round trip through encoding/xml changed the order to %v, want %v",
					got, interleavedFillOrder)
			}

			// And the two marshalers must not disagree.
			b := xmlb.NewPresentationMLBuilder()
			b.MarshalElement(NsDrawingML, tc.local, v)
			if got := childNames(t, []byte(b.String())); !equalStrings(got, kindsOf(interleavedFillOrder)) {
				t.Errorf("Builder path emitted %v, want %v", got, kindsOf(interleavedFillOrder))
			}
		})
	}
}

// The values must travel with their positions, not just the kinds: two
// solidFills in one list is exactly the case where an off-by-one index in the
// replay swaps two fills while the sequence of element names still looks right.
func TestFillStyleLst_StdlibMarshalKeepsValuesWithPositions(t *testing.T) {
	var v FillStyleLst
	if err := xml.Unmarshal([]byte(fillListSrc("fillStyleLst")), &v); err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := xml.Marshal(&v)
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}
	var back FillStyleLst
	if err := xml.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(back.SolidFill) != 2 {
		t.Fatalf("solidFill count = %d, want 2", len(back.SolidFill))
	}
	for i, want := range []string{"FF0000", "0000FF"} {
		if back.SolidFill[i] == nil || back.SolidFill[i].SrgbClr == nil ||
			back.SolidFill[i].SrgbClr.Val != want {
			t.Errorf("solidFill[%d] = %+v, want srgbClr %s", i, back.SolidFill[i], want)
		}
	}
	if len(back.GradFill) != 2 {
		t.Fatalf("gradFill count = %d, want 2", len(back.GradFill))
	}
	if len(back.PattFill) != 1 || back.PattFill[0].Prst != "pct5" {
		t.Errorf("pattFill lost its prst: %+v", back.PattFill)
	}
}

// A list built in code has no captured order, so both paths fall back to
// grouped XSD order — and must fall back to the *same* one.
func TestFillStyleLst_ProgrammaticListUsesGroupedOrder(t *testing.T) {
	v := &FillStyleLst{
		GradFill:  []*GradFill{{}},
		SolidFill: []*SolidFill{{SrgbClr: &SrgbClr{Val: "ABCDEF"}}},
		NoFill:    []*NoFillXML{{}},
	}
	want := []string{"noFill", "solidFill", "gradFill"}
	out, err := xml.Marshal(v)
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}
	if got := childNames(t, out); !equalStrings(got, want) {
		t.Errorf("encoding/xml grouped order = %v, want %v", got, want)
	}
	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(NsDrawingML, "fillStyleLst", v)
	if got := childNames(t, []byte(b.String())); !equalStrings(got, want) {
		t.Errorf("Builder grouped order = %v, want %v", got, want)
	}
}

// An empty list must stay an element, not vanish: a:fillStyleLst is required by
// the schema and a theme without one does not open.
func TestFillStyleLst_EmptyListStillEmitsTheElement(t *testing.T) {
	out, err := xml.Marshal(&FillStyleLst{})
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}
	if !strings.Contains(string(out), "FillStyleLst") {
		t.Errorf("empty fill style list disappeared: %q", out)
	}
	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(NsDrawingML, "fillStyleLst", &FillStyleLst{})
	if got := b.String(); !strings.Contains(got, "fillStyleLst") {
		t.Errorf("empty fill style list disappeared on the Builder path: %q", got)
	}
}

// kindsOf strips the "#index" suffix from refNames output.
func kindsOf(refs []string) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r[:strings.IndexByte(r, '#')])
	}
	return out
}

// An a:cont / a:effectDag whose children mix the one typed kind (a:blur) with
// unmodeled ones. Effect containers are ordered: an effect list is applied in
// sequence, so moving the blur past the shadow changes the rendering.
const effectContainerChildren = `<a:outerShdw blurRad="63500" dist="25400">` +
	`<a:srgbClr val="000000"><a:alpha val="40000"/></a:srgbClr></a:outerShdw>` +
	`<a:blur rad="12700" grow="0"/>` +
	`<a:glow rad="50800"><a:srgbClr val="FFC000"/></a:glow>`

func TestEffectContainer_StdlibMarshalKeepsChildOrderAndAttrs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		local string
		parse func([]byte) (interface{}, error)
	}{
		{"cont", "cont", func(b []byte) (interface{}, error) {
			v := &EffectContainer{}
			return v, xml.Unmarshal(b, v)
		}},
		{"effectDag", "effectDag", func(b []byte) (interface{}, error) {
			v := &EffectDag{}
			return v, xml.Unmarshal(b, v)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `<a:` + tc.local + ` xmlns:a="` + NsDrawingML + `" type="sib" name="fx1">` +
				effectContainerChildren + `</a:` + tc.local + `>`
			v, err := tc.parse([]byte(src))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			out, err := xml.Marshal(v)
			if err != nil {
				t.Fatalf("xml.Marshal: %v", err)
			}
			want := []string{"outerShdw", "blur", "glow"}
			if got := childNames(t, out); !equalStrings(got, want) {
				t.Errorf("encoding/xml path emitted children %v, want %v "+
					"(effects apply in sequence, so reordering them changes the rendering)", got, want)
			}
			s := string(out)
			for _, attr := range []string{`type="sib"`, `name="fx1"`} {
				if !strings.Contains(s, attr) {
					t.Errorf("container attribute %s dropped by the encoding/xml path: %s", attr, s)
				}
			}
			// The unmodeled children are raw captures; their content must come
			// back, not just their names.
			for _, frag := range []string{`blurRad="63500"`, `val="000000"`, `rad="50800"`, `val="FFC000"`} {
				if !strings.Contains(s, frag) {
					t.Errorf("raw effect child content %s lost by the encoding/xml path: %s", frag, s)
				}
			}
			// The typed a:blur child must keep its own attributes.
			if !strings.Contains(s, `rad="12700"`) {
				t.Errorf("typed a:blur child lost its rad: %s", s)
			}

			// Both marshalers must agree on the child sequence.
			b := xmlb.NewPresentationMLBuilder()
			b.MarshalElement(NsDrawingML, tc.local, v)
			if got := childNames(t, []byte(b.String())); !equalStrings(got, want) {
				t.Errorf("Builder path emitted children %v, want %v", got, want)
			}
		})
	}
}

// The Blur field is documented as settable after parse, and the substitution
// must survive the encoding/xml path too: replacing it must keep the blur at
// its captured position rather than appending a second one.
func TestEffectContainer_StdlibMarshalHonoursPostParseBlurEdit(t *testing.T) {
	src := `<a:cont xmlns:a="` + NsDrawingML + `" name="fx1">` + effectContainerChildren + `</a:cont>`
	var ec EffectContainer
	if err := xml.Unmarshal([]byte(src), &ec); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ec.Blur == nil {
		t.Fatal("typed a:blur child not surfaced on the Blur field")
	}
	ec.Blur = &BlurXML{Rad: 999}

	out, err := xml.Marshal(&ec)
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}
	want := []string{"outerShdw", "blur", "glow"}
	if got := childNames(t, out); !equalStrings(got, want) {
		t.Errorf("children after replacing Blur = %v, want %v (the replacement must take the "+
			"captured child's position, not be appended)", got, want)
	}
	if s := string(out); !strings.Contains(s, `rad="999"`) || strings.Contains(s, `rad="12700"`) {
		t.Errorf("post-parse Blur assignment discarded by the encoding/xml path: %s", s)
	}
}

// An a:blip's effect children are a choice of 17 typed kinds plus raw capture,
// all of them xml:"-" on the struct, so the stdlib path can only reproduce them
// through BlipEffect.marshalXML. Nothing exercised it.
func TestBlipEffects_StdlibMarshalKeepsTypedAndRawChildren(t *testing.T) {
	src := `<a:blip xmlns:a="` + NsDrawingML + `" xmlns:r="` + xmlb.NSOfficeDocumentRels +
		`" xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" r:embed="rId4" cstate="print">` +
		`<a:alphaModFix amt="50000"/>` +
		`<a:clrChange useA="1"><a:clrFrom><a:srgbClr val="FFFFFF"/></a:clrFrom>` +
		`<a:clrTo><a:srgbClr val="000000"/></a:clrTo></a:clrChange>` +
		`<a14:imgProps><a14:imgLayer r:embed="rId5"/></a14:imgProps>` +
		`<a:biLevel thresh="25000"/>` +
		`</a:blip>`
	var blip BlipXML
	if err := xml.Unmarshal([]byte(src), &blip); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(blip.Effects) != 4 {
		t.Fatalf("effects parsed = %d, want 4: %+v", len(blip.Effects), blip.Effects)
	}

	out, err := xml.Marshal(&blip)
	if err != nil {
		t.Fatalf("xml.Marshal: %v", err)
	}
	want := []string{"alphaModFix", "clrChange", "imgProps", "biLevel"}
	if got := childNames(t, out); !equalStrings(got, want) {
		t.Errorf("encoding/xml path emitted blip children %v, want %v", got, want)
	}
	s := string(out)
	for _, frag := range []string{`amt="50000"`, `val="FFFFFF"`, `val="000000"`,
		`thresh="25000"`, "imgLayer", `rId4`} {
		if !strings.Contains(s, frag) {
			t.Errorf("blip content %s lost by the encoding/xml path: %s", frag, s)
		}
	}
}

// dsp:dataModelExt carries the relationship id of a SmartArt data model. It is
// an a:ext with a *recognised* URI, so parsing dispatches it to a typed field
// and re-marshal rebuilds it from that field alone — there is no RawContent
// fallback to catch anything the hook failed to bind. Any attribute the hook
// does not read is therefore deleted on save, and the diagram loses its data
// model. This pins both attributes through the cycle.
//
// Note the hook binds relId only in the relationships namespace (r:relId),
// which is the spelling asserted below because it is the only one the model
// supports. The dsp schema declares the attribute as name="relId" rather than
// ref="r:id", i.e. unqualified; if producers write it that way, RelId parses
// empty and the reference is deleted on save with no raw fallback to catch it.
// That case is deliberately not asserted here — settling it needs a real
// diagram part to check the spelling against.
func TestDspDataModelExt_RoundTrip(t *testing.T) {
	src := `<a:ext xmlns:a="` + NsDrawingML + `" xmlns:dsp="` + xmlb.NSDrawingDiagram2008 +
		`" xmlns:r="` + xmlb.NSOfficeDocumentRels + `" uri="` + xmlb.ExtURIDataModelExt + `">` +
		`<dsp:dataModelExt r:relId="rId7" minVer="http://schemas.openxmlformats.org/drawingml/2006/diagram"/>` +
		`</a:ext>`
	var e Ext
	if err := xml.Unmarshal([]byte(src), &e); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if e.DataModelExt == nil {
		t.Fatal("dsp:dataModelExt not dispatched to its typed field")
	}
	if e.DataModelExt.RelId != "rId7" {
		t.Errorf("relId = %q, want rId7: the diagram's data-model relationship is dropped",
			e.DataModelExt.RelId)
	}
	if e.DataModelExt.MinVer != "http://schemas.openxmlformats.org/drawingml/2006/diagram" {
		t.Errorf("minVer = %q, want the diagram namespace", e.DataModelExt.MinVer)
	}

	out := buildFragment(t, "ext", &e)
	for _, frag := range []string{`r:relId="rId7"`,
		`minVer="http://schemas.openxmlformats.org/drawingml/2006/diagram"`,
		"dataModelExt", xmlb.ExtURIDataModelExt} {
		if !strings.Contains(out, frag) {
			t.Errorf("dsp:dataModelExt lost %s on re-marshal: %s", frag, out)
		}
	}
}

// The two a14 flag extensions are emitted through marshalA14Simple whenever the
// model was built in code (a parsed one replays its captured attribute list
// instead). Both are xsd:boolean and both have a meaningful absent form, so the
// nil case must emit the element *without* a val, not with val="0".
func TestA14SimpleExtensions_ProgrammaticMarshal(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name  string
		ext   *Ext
		want  string
		avoid string
	}{
		{"useLocalDpi true",
			&Ext{URI: xmlb.ExtURIUseLocalDpi, UseLocalDpi: &A14UseLocalDpi{Val: &yes}},
			`<a14:useLocalDpi xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" val="1"/>`, ""},
		{"useLocalDpi false",
			&Ext{URI: xmlb.ExtURIUseLocalDpi, UseLocalDpi: &A14UseLocalDpi{Val: &no}},
			`<a14:useLocalDpi xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" val="0"/>`, ""},
		{"useLocalDpi absent",
			&Ext{URI: xmlb.ExtURIUseLocalDpi, UseLocalDpi: &A14UseLocalDpi{}},
			`<a14:useLocalDpi`, "val="},
		{"shadowObscured true",
			&Ext{URI: xmlb.ExtURIShadowObscured, ShadowObscured: &A14ShadowObscured{Val: &yes}},
			`<a14:shadowObscured xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" val="1"/>`, ""},
		{"shadowObscured absent",
			&Ext{URI: xmlb.ExtURIShadowObscured, ShadowObscured: &A14ShadowObscured{}},
			`<a14:shadowObscured`, "val="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := buildFragment(t, "ext", tt.ext)
			if !strings.Contains(out, tt.want) {
				t.Errorf("want %s in %s", tt.want, out)
			}
			if tt.avoid != "" && strings.Contains(out, tt.avoid) {
				t.Errorf("a nil *bool must not be spelled out as a value: %s in %s", tt.avoid, out)
			}
			// The element carries its own namespace declaration; without it the
			// a14 prefix is unbound wherever the extension lands.
			if !strings.Contains(out, `xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main"`) {
				t.Errorf("a14 extension emitted without its inline xmlns: %s", out)
			}
		})
	}
}

// FuzzFillStyleListStdlibRoundTrip drives the positional-order machinery with
// arbitrary input. A fill list that parses must marshal, and re-parsing what it
// marshaled must give the same cross-kind order: an order replay that drops,
// duplicates or reorders a ref under some input shape shows up as a fixed
// point that is not fixed.
func FuzzFillStyleListStdlibRoundTrip(f *testing.F) {
	f.Add(fillListSrc("fillStyleLst"))
	f.Add(`<a:fillStyleLst xmlns:a="` + NsDrawingML + `"/>`)
	f.Add(`<a:fillStyleLst xmlns:a="` + NsDrawingML + `"><a:noFill/><a:grpFill/><a:noFill/></a:fillStyleLst>`)
	f.Add(`<a:fillStyleLst xmlns:a="` + NsDrawingML + `"><a:futureFill/><a:solidFill/></a:fillStyleLst>`)

	f.Fuzz(func(t *testing.T, src string) {
		var v FillStyleLst
		if err := xml.Unmarshal([]byte(src), &v); err != nil {
			return // not a fill list; nothing to promise
		}
		first, err := xml.Marshal(&v)
		if err != nil {
			t.Fatalf("a parsed fill list must marshal: %v", err)
		}
		var back FillStyleLst
		if err := xml.Unmarshal(first, &back); err != nil {
			t.Fatalf("output of xml.Marshal must re-parse: %v\n%s", err, first)
		}
		if a, b := refNames(v.fillOrder), refNames(back.fillOrder); !equalStrings(a, b) {
			t.Fatalf("fill order changed across a marshal/unmarshal cycle: %v -> %v\n%s", a, b, first)
		}
		second, err := xml.Marshal(&back)
		if err != nil {
			t.Fatalf("re-marshal: %v", err)
		}
		if string(first) != string(second) {
			t.Fatalf("marshal is not a fixed point:\n%s\n%s", first, second)
		}
	})
}

// FuzzEffectContainerStdlibRoundTrip does the same for effect containers, whose
// unmodeled children are captured as raw bytes and replayed by re-tokenizing
// them — the place an exotic child can change shape on every pass.
//
// Two scoping decisions, both from what this target found rather than guessed:
//
//   - The fixed point is asserted from the *second* marshal on, not the first.
//     The first pass has a real normalization: a raw child captured in no
//     namespace at all is emitted with no xmlns of its own, so it inherits the
//     container's default declaration and comes back namespaced
//     (`<a:cont><a:cont><A/></a:cont></a:cont>`). Children must still all
//     survive that pass, asserted separately; any drift that is *not* that
//     one-time re-homing persists past it and still fails here.
//   - A step that errors ends the case instead of failing it. Go's decoder is
//     lenient about things a conforming parser rejects — an NCName like `0` in
//     `<A:0/>`, a directive inside element content — and replaying those
//     verbatim either errors or produces XML that will not re-parse. That is
//     real, but it is a "reject malformed input" promise, which this path has
//     never made and which no OOXML producer can trigger; the promise under
//     test is fidelity.
func FuzzEffectContainerStdlibRoundTrip(f *testing.F) {
	f.Add(`<a:cont xmlns:a="` + NsDrawingML + `" type="sib" name="fx1">` +
		effectContainerChildren + `</a:cont>`)
	f.Add(`<a:cont xmlns:a="` + NsDrawingML + `"/>`)
	f.Add(`<a:cont xmlns:a="` + NsDrawingML + `"><a:cont><a:blur rad="1"/></a:cont></a:cont>`)
	f.Add(`<a:cont><a:cont><A/></a:cont></a:cont>`)

	// marshalReparse returns nil when a step fails on input outside the domain
	// described above.
	marshalReparse := func(ec *EffectContainer) ([]byte, *EffectContainer) {
		out, err := xml.Marshal(ec)
		if err != nil {
			return nil, nil
		}
		var back EffectContainer
		if err := xml.Unmarshal(out, &back); err != nil {
			return nil, nil
		}
		return out, &back
	}

	f.Fuzz(func(t *testing.T, src string) {
		var ec EffectContainer
		if err := xml.Unmarshal([]byte(src), &ec); err != nil {
			return
		}
		first, v1 := marshalReparse(&ec)
		if v1 == nil {
			return
		}
		second, v2 := marshalReparse(v1)
		if v2 == nil {
			t.Fatalf("the second cycle must succeed once the first did:\n%s", first)
		}
		third, v3 := marshalReparse(v2)
		if v3 == nil {
			t.Fatalf("the third cycle must succeed once the second did:\n%s", second)
		}

		// No child may be lost or gain a sibling on the normalizing pass.
		if a, b := childNames(t, first), childNames(t, second); !equalStrings(a, b) {
			t.Fatalf("children changed across a marshal/unmarshal cycle: %v -> %v\n%s\n%s",
				a, b, first, second)
		}
		if string(second) != string(third) {
			t.Fatalf("marshal is not a fixed point after normalization:\n%s\n%s", second, third)
		}
	})
}
