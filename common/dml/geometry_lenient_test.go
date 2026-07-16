package dml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

const aNS = "http://schemas.openxmlformats.org/drawingml/2006/main"

// Wild slide masters carry fractional lexical forms (e.g. "-1.5") for integer
// EMU coordinates and angles. A single such attribute must not fail the whole
// Open (graceful degradation); the model rounds it while the verbatim source
// attribute round-trips via CapturedAttrs.
func TestLenientIntegerCoordinates(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"off fractional x", `<a:off xmlns:a="` + aNS + `" x="-1.5" y="0"/>`},
		{"ext fractional cx", `<a:ext xmlns:a="` + aNS + `" cx="12.7" cy="34"/>`},
		{"xfrm fractional rot", `<a:xfrm xmlns:a="` + aNS + `" rot="90.5"><a:off x="1" y="2"/><a:ext cx="3" cy="4"/></a:xfrm>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			switch {
			case strings.Contains(tc.src, "<a:off"):
				v = &OffXML{}
			case strings.Contains(tc.src, "<a:ext"):
				v = &ExtXML{}
			default:
				v = &Xfrm{}
			}
			if err := xmlb.UnmarshalWithSource([]byte(tc.src), v); err != nil {
				t.Fatalf("Open must not fail on a fractional coordinate: %v", err)
			}
		})
	}

	// The fractional value rounds in the model (-1.5 -> -2). Valid integer
	// files are untouched: coerceIntAttrs is a no-op when ParseInt succeeds,
	// so the byte-identity corpus (all integer coordinates) is unaffected.
	var off OffXML
	if err := xmlb.UnmarshalWithSource([]byte(`<a:off xmlns:a="`+aNS+`" x="-1.5" y="0"/>`), &off); err != nil {
		t.Fatal(err)
	}
	if off.X != -2 {
		t.Errorf("rounded X = %d, want -2 (round(-1.5))", off.X)
	}

	// A valid integer coordinate round-trips verbatim through the reflection
	// marshaler (modeled value equals the captured source, so its raw
	// rendering is kept): the lenience never perturbs a conformant file.
	var off2 OffXML
	if err := xmlb.UnmarshalWithSource([]byte(`<a:off xmlns:a="`+aNS+`" x="-123456" y="0"/>`), &off2); err != nil {
		t.Fatal(err)
	}
	var b xmlb.Builder
	b.MarshalElement(aNS, "off", &off2)
	if out := b.String(); !strings.Contains(out, `x="-123456"`) {
		t.Errorf("valid coordinate not preserved on round-trip; got %q", out)
	}
}
