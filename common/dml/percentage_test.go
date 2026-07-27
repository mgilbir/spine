package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// ECMA-376 transitional producers write ST_Percentage-family values as "n%"
// or "n.m%" strings (Common Crawl corpus: <a:fillToRect t="50%"/> in a slide
// master), while strict writes thousandths integers. Both must parse, and the
// original lexical form must be re-emitted verbatim for round-trip fidelity.
func TestPercentage_TransitionalPercentForm(t *testing.T) {
	cases := []struct {
		in   string
		want int32
	}{
		{"50%", 50000},
		{"-20%", -20000},
		{"20.5%", 20500},
		{"0%", 0},
		{"100000", 100000},
		{"-50000", -50000},
	}
	for _, tc := range cases {
		var p Percentage
		if err := p.UnmarshalXMLAttr(xml.Attr{Name: xml.Name{Local: "val"}, Value: tc.in}); err != nil {
			t.Errorf("UnmarshalXMLAttr(%q): %v", tc.in, err)
			continue
		}
		if p.Int32() != tc.want {
			t.Errorf("UnmarshalXMLAttr(%q) = %d, want %d", tc.in, p.Int32(), tc.want)
		}
		if got := p.AttrValue(); got != tc.in {
			t.Errorf("AttrValue after %q = %q, want the original form back", tc.in, got)
		}
	}
}

// A fillToRect parsed from the transitional percent form must survive the
// full unmarshal -> Builder marshal cycle with its lexical form intact.
func TestRelRect_PercentFormRoundTrip(t *testing.T) {
	in := `<path xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" path="circle"><fillToRect l="50%" t="50%" r="50%" b="50%"/></path>`
	var v PathXML
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.FillToRect == nil || v.FillToRect.T.Int32() != 50000 {
		t.Fatalf("fillToRect not parsed: %+v", v.FillToRect)
	}

	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, "a")
	b.MarshalElement(NsDrawingML, "path", &v)
	if err := b.Err(); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, `l="50%"`) || !strings.Contains(out, `b="50%"`) {
		t.Errorf("percent form not re-emitted verbatim: %s", out)
	}
}

// The strict integer form must keep marshaling as integers.
func TestRelRect_IntegerFormRoundTrip(t *testing.T) {
	in := `<path xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" path="circle"><fillToRect l="50000" t="50000" r="50000" b="50000"/></path>`
	var v PathXML
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, "a")
	b.MarshalElement(NsDrawingML, "path", &v)
	if err := b.Err(); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, `l="50000"`) {
		t.Errorf("integer form changed: %s", out)
	}
}

// omitempty semantics: an unset Percentage stays omitted, while an explicit
// source "0%" is not dropped (it re-emits its lexical form).
func TestPercentage_OmitEmpty(t *testing.T) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, "a")
	b.MarshalElement(NsDrawingML, "fillRect", &RelRect{})
	out := b.String()
	if strings.Contains(out, "l=") || strings.Contains(out, "t=") {
		t.Errorf("zero RelRect edges emitted: %s", out)
	}

	var r RelRect
	if err := xml.Unmarshal([]byte(`<fillRect xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" l="0%"/>`), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2 := xmlb.NewBuilder()
	b2.RegisterNamespace(NsDrawingML, "a")
	b2.MarshalElement(NsDrawingML, "fillRect", &r)
	if out := b2.String(); !strings.Contains(out, `l="0%"`) {
		t.Errorf("explicit 0%% dropped: %s", out)
	}
}

// arcTo wR/hR are ST_AdjCoordinate and stAng/swAng ST_AdjAngle: unions of a
// number or a geometry guide name. Guide references like wR="f35" (Common
// Crawl corpus, custom geometry) are legal and must round-trip verbatim
// instead of failing the whole slide parse.
func TestArcTo_GuideNameReferences(t *testing.T) {
	in := `<path xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" w="100" h="100"><arcTo wR="f35" hR="f36" stAng="f71" swAng="f59"/></path>`
	var v PathXML2D
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(v.ArcTo) != 1 || v.ArcTo[0].WR != "f35" || v.ArcTo[0].SwAng != "f59" {
		t.Fatalf("arcTo guide refs not parsed: %+v", v.ArcTo)
	}

	out, err := xml.Marshal(&v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `wR="f35"`) || !strings.Contains(string(out), `swAng="f59"`) {
		t.Errorf("guide refs not re-emitted verbatim: %s", out)
	}
}

// TestPercentage_NonCanonicalIntegerRoundTrip pins C342: a strict integer that
// is not the canonical rendering of its value — a zero-padded "050000" or a
// signed "+50000" — must round-trip verbatim. The integer branch previously
// cleared orig unconditionally, so AttrValue re-emitted the canonical "50000".
func TestPercentage_NonCanonicalIntegerRoundTrip(t *testing.T) {
	for _, in := range []string{"050000", "+50000", "-050000"} {
		var p Percentage
		if err := p.UnmarshalXMLAttr(xml.Attr{Name: xml.Name{Local: "val"}, Value: in}); err != nil {
			t.Errorf("UnmarshalXMLAttr(%q): %v", in, err)
			continue
		}
		if got := p.AttrValue(); got != in {
			t.Errorf("AttrValue after %q = %q, want the original form back", in, got)
		}
	}
	// The canonical form must still leave orig empty so an explicit "0" keeps
	// reporting zero (IsZeroAttr) — the AlphaModFix pointer model depends on it.
	var z Percentage
	if err := z.UnmarshalXMLAttr(xml.Attr{Name: xml.Name{Local: "val"}, Value: "0"}); err != nil {
		t.Fatalf("UnmarshalXMLAttr(0): %v", err)
	}
	if !z.IsZeroAttr() {
		t.Errorf("canonical \"0\" should report IsZeroAttr, got false")
	}
	var c Percentage
	if err := c.UnmarshalXMLAttr(xml.Attr{Name: xml.Name{Local: "val"}, Value: "50000"}); err != nil {
		t.Fatalf("UnmarshalXMLAttr(50000): %v", err)
	}
	if got := c.AttrValue(); got != "50000" {
		t.Errorf("canonical AttrValue = %q, want 50000", got)
	}
}
