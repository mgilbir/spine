package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

const blipRTNS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`

func marshalBlipForTest(t *testing.T, v interface{}) string {
	t.Helper()
	b := xmlb.NewPresentationMLBuilder()
	b.RegisterNamespace("urn:z", "z")
	b.MarshalElement(NsDrawingML, "blip", v)
	if err := b.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// A repeated effect and an unmodeled effect are preserved in place on a
// source-registered parse: the raw bytes are the source's own.
func TestBlipRoundTripKeepsRepeatedAndUnknownEffects(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		order    []xmlb.ChildRef
		raw      []string
	}{
		{
			"repeated effect",
			`<a:blip ` + blipRTNS + `><a:lum bright="10000"/><a:grayscl/><a:grayscl/><a:alphaModFix amt="50000"/></a:blip>`,
			[]xmlb.ChildRef{{Field: 18}, {Field: 16}, {Field: -1}, {Field: 8}},
			[]string{`<a:grayscl/>`},
		},
		{
			"unknown effect",
			`<a:blip ` + blipRTNS + `><a:lum bright="10000"/><a:futureEffect/><a:alphaModFix amt="50000"/></a:blip>`,
			[]xmlb.ChildRef{{Field: 18}, {Field: -1}, {Field: 8}},
			[]string{`<a:futureEffect/>`},
		},
		{
			"foreign namespace with its own prefix and content",
			`<a:blip ` + blipRTNS + ` xmlns:z="urn:z"><z:fx z:k="v"><z:in>t</z:in></z:fx><a:grayscl/></a:blip>`,
			[]xmlb.ChildRef{{Field: -1}, {Field: 16}},
			[]string{`<z:fx z:k="v"><z:in>t</z:in></z:fx>`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bl Blip
			if err := xmlb.UnmarshalWithSource([]byte(tc.in), &bl); err != nil {
				t.Fatal(err)
			}
			cc := bl.CapturedChildren
			if len(cc.Order) != len(tc.order) {
				t.Fatalf("order = %+v, want %+v", cc.Order, tc.order)
			}
			for i, ref := range cc.Order {
				if ref != tc.order[i] {
					t.Fatalf("order = %+v, want %+v", cc.Order, tc.order)
				}
			}
			if len(cc.Raw) != len(tc.raw) {
				t.Fatalf("raw = %q, want %q", cc.Raw, tc.raw)
			}
			for i, r := range cc.Raw {
				if string(r) != tc.raw[i] {
					t.Fatalf("raw[%d] = %q, want %q", i, r, tc.raw[i])
				}
			}
			// The marshal path replays the root attributes (namespace
			// declarations included) captured at parse time.
			if got := marshalBlipForTest(t, &bl); got != tc.in {
				t.Errorf("round trip:\n got %s\nwant %s", got, tc.in)
			}
		})
	}
}

// Without a registered source (plain decoder: deep copies, transcoded parts)
// the same children are rebuilt from tokens, not dropped.
func TestBlipRoundTripWithoutSourceKeepsRepeatedAndUnknownEffects(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{
			"repeated effect",
			`<a:blip ` + blipRTNS + `><a:lum bright="10000"/><a:grayscl/><a:grayscl/><a:alphaModFix amt="50000"/></a:blip>`,
			`<a:blip ` + blipRTNS + `><a:lum bright="10000"/><a:grayscl/><a:grayscl ` + blipRTNS + `/><a:alphaModFix amt="50000"/></a:blip>`,
		},
		{
			"unknown effect",
			`<a:blip ` + blipRTNS + ` xmlns:z="urn:z"><a:lum bright="10000"/><z:fx z:k="v"><z:in>t</z:in></z:fx></a:blip>`,
			`<a:blip ` + blipRTNS + ` xmlns:z="urn:z"><a:lum bright="10000"/><ns1:fx xmlns:ns1="urn:z" ns1:k="v"><z:in>t</z:in></ns1:fx></a:blip>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bl Blip
			if err := xml.Unmarshal([]byte(tc.in), &bl); err != nil {
				t.Fatal(err)
			}
			if got := marshalBlipForTest(t, &bl); got != tc.want {
				t.Errorf("round trip:\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// BlipXML keeps repeated typed effects and unmodeled ones, in order.
func TestBlipXMLRoundTripKeepsRepeatedAndUnknownEffects(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"repeated", `<a:blip ` + blipRTNS + `><a:lum bright="10000"/><a:grayscl/><a:grayscl/><a:alphaModFix amt="50000"/></a:blip>`, ""},
		{"unknown", `<a:blip ` + blipRTNS + `><a:lum bright="10000"/><a:futureEffect k="v"><a:in/></a:futureEffect><a:grayscl/></a:blip>`, ""},
		// An element named like an effect in another namespace is not that
		// effect; it keeps its namespace.
		{"foreign namespace", `<a:blip ` + blipRTNS + ` xmlns:z="urn:z"><z:grayscl/><a:grayscl/></a:blip>`,
			`<a:blip><z:grayscl/><a:grayscl/></a:blip>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bl BlipXML
			if err := xmlb.UnmarshalWithSource([]byte(tc.in), &bl); err != nil {
				t.Fatal(err)
			}
			got := marshalBlipForTest(t, &bl)
			// MarshalElement writes no root declarations of its own.
			want := tc.want
			if want == "" {
				want = strings.Replace(tc.in, " "+blipRTNS, "", 1)
			}
			if got != want {
				t.Errorf("round trip:\n got %s\nwant %s", got, want)
			}
		})
	}
}

// Raw-captured children keep their place in the ordered effect list, named, so
// a consumer applying effects in order sees them.
func TestBlipOrderedEffectsSurfacesRawChildren(t *testing.T) {
	var bl Blip
	src := `<a:blip ` + blipRTNS + ` xmlns:z="urn:z"><a:lum bright="10000"/><a:grayscl/><!-- c --><a:grayscl/><z:fx/><a:alphaModFix amt="50000"/></a:blip>`
	if err := xmlb.UnmarshalWithSource([]byte(src), &bl); err != nil {
		t.Fatal(err)
	}
	effects := bl.OrderedEffects()
	if len(effects) != 5 || effects[0].Lum == nil || effects[1].Grayscl == nil ||
		effects[2].RawName.Local != "grayscl" || effects[3].RawName.Local != "fx" || effects[4].AlphaModFix == nil {
		t.Fatalf("%+v", effects)
	}
	if name, _, ok := effects[2].typedChild(); ok {
		t.Fatalf("raw effect reads as typed %s", name)
	}
}
