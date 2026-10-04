package pptx

import (
	"context"
	"encoding/xml"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/render"
)

// renderEffectPixel applies effects to one pixel.
func renderEffectPixel(t *testing.T, c color.NRGBA, colors *renderColors, effects ...*dml.BlipEffect) (color.NRGBA, error) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, c)
	out, err := colors.blipEffects(img, effects, 9525)
	if err != nil {
		return color.NRGBA{}, err
	}
	if img.NRGBAAt(0, 0) != c {
		t.Fatal("effects changed the source image")
	}
	return color.NRGBAModel.Convert(out.At(0, 0)).(color.NRGBA), nil
}

func TestRenderBlipEffects(t *testing.T) {
	strict := &renderColors{ctx: context.Background()}
	srgb := func(v string) dml.ColorChoice { return dml.ColorChoice{SrgbClr: &dml.SrgbClr{Val: v}} }
	pct := func(v int32) dml.Percentage { return dml.NewPercentage(v) }
	pctp := func(v int32) *dml.Percentage { p := dml.NewPercentage(v); return &p }
	orange := color.NRGBA{R: 200, G: 100, B: 50, A: 200}
	// Rec. 601 luma of orange: 0.299×200 + 0.587×100 + 0.114×50 = 124.2.
	var duo dml.Duotone
	if err := xmlb.Unmarshal([]byte(`<a:duotone xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:srgbClr val="000080"/><a:srgbClr val="FFFFFF"/></a:duotone>`), &duo); err != nil {
		t.Fatal(err)
	}
	from, to := srgb("C86432"), srgb("00FF00")
	useA := false
	for _, tc := range []struct {
		name   string
		effect *dml.BlipEffect
		want   color.NRGBA
	}{
		{"grayscale", &dml.BlipEffect{Grayscl: &dml.GrayscaleXML{}}, color.NRGBA{124, 124, 124, 200}},
		{"bi-level below", &dml.BlipEffect{BiLevel: &dml.BiLevelXML{Thresh: pct(50000)}}, color.NRGBA{0, 0, 0, 200}},
		{"bi-level above", &dml.BlipEffect{BiLevel: &dml.BiLevelXML{Thresh: pct(40000)}}, color.NRGBA{255, 255, 255, 200}},
		// 124.2/255 of the way from navy to white.
		{"duotone", &dml.BlipEffect{Duotone: &duo}, color.NRGBA{124, 124, 190, 200}},
		{"replace", &dml.BlipEffect{ClrRepl: &dml.ClrRepl{SrgbClr: &dml.SrgbClr{Val: "0000FF"}}}, color.NRGBA{0, 0, 255, 200}},
		{"change", &dml.BlipEffect{ClrChange: &dml.ClrChange{ClrFrom: &from, ClrTo: &to}}, color.NRGBA{0, 255, 0, 255}},
		{"change keeping alpha", &dml.BlipEffect{ClrChange: &dml.ClrChange{UseA: &useA, ClrFrom: &from, ClrTo: &to}}, color.NRGBA{0, 255, 0, 200}},
		{"no change", &dml.BlipEffect{ClrChange: &dml.ClrChange{ClrFrom: &to, ClrTo: &from}}, orange},
		{"fixed alpha", &dml.BlipEffect{AlphaModFix: &dml.AlphaModFix{Amt: pctp(50000)}}, color.NRGBA{200, 100, 50, 100}},
		{"alpha replace", &dml.BlipEffect{AlphaRepl: &dml.AlphaRepl{A: pct(25000)}}, color.NRGBA{200, 100, 50, 64}},
		{"alpha bi-level", &dml.BlipEffect{AlphaBiLevel: &dml.AlphaBiLevel{Thresh: pct(90000)}}, color.NRGBA{200, 100, 50, 0}},
		{"alpha ceiling", &dml.BlipEffect{AlphaCeiling: &dml.AlphaCeiling{}}, color.NRGBA{200, 100, 50, 255}},
		{"alpha floor", &dml.BlipEffect{AlphaFloor: &dml.AlphaFloor{}}, color.NRGBA{200, 100, 50, 0}},
		{"alpha inverse", &dml.BlipEffect{AlphaInv: &dml.AlphaInv{}}, color.NRGBA{200, 100, 50, 55}},
		// A half turn of hue swaps orange for its complement.
		{"hue", &dml.BlipEffect{Hsl: &dml.HslXML{Hue: 10800000}}, color.NRGBA{50, 150, 200, 200}},
		{"desaturate", &dml.BlipEffect{Hsl: &dml.HslXML{Sat: pct(-100000)}}, color.NRGBA{125, 125, 125, 200}},
		{"multiply", &dml.BlipEffect{FillOverlay: &dml.FillOverlayXML{Blend: "mult", SolidFill: &dml.SolidFill{SrgbClr: &dml.SrgbClr{Val: "808080"}}}}, color.NRGBA{100, 50, 25, 200}},
		{"screen", &dml.BlipEffect{FillOverlay: &dml.FillOverlayXML{Blend: "screen", SolidFill: &dml.SolidFill{SrgbClr: &dml.SrgbClr{Val: "000000"}}}}, orange},
	} {
		got, err := renderEffectPixel(t, orange, strict, tc.effect)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
	// Effects compose in order: grayscale then a 50% threshold is black,
	// and a full hue turn after grayscale stays gray.
	got, err := renderEffectPixel(t, orange, strict, &dml.BlipEffect{Grayscl: &dml.GrayscaleXML{}}, &dml.BlipEffect{Hsl: &dml.HslXML{Hue: 10800000}})
	if err != nil || got != (color.NRGBA{124, 124, 124, 200}) {
		t.Fatalf("composed: %+v %v", got, err)
	}
	// Undefined or unsupported effects are refused strictly and reported in
	// best effort.
	var warnings []string
	lenient := &renderColors{ctx: context.Background(), approx: func(err error) { warnings = append(warnings, err.Error()) }}
	for _, e := range []*dml.BlipEffect{
		{Lum: &dml.LumXML{Bright: pct(20000)}},
		{Tint: &dml.TintEffectXML{Hue: 0, Amt: pct(50000)}},
		{Blur: &dml.BlurXML{Rad: 10000}},
		{RawName: xml.Name{Space: "urn:future", Local: "future"}},
	} {
		if _, err := renderEffectPixel(t, orange, strict, e); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("strict %+v: %v", e, err)
		}
		if _, err := renderEffectPixel(t, orange, lenient, e); err != nil {
			t.Fatal(err)
		}
	}
	if len(warnings) != 4 || !strings.Contains(warnings[0], "LibreOffice") {
		t.Fatalf("warnings: %q", warnings)
	}
	// 20% brighter adds 51 to each channel, as LibreOffice adjusts.
	if got, _ := renderEffectPixel(t, orange, lenient, &dml.BlipEffect{Lum: &dml.LumXML{Bright: pct(20000)}}); got != (color.NRGBA{251, 151, 101, 200}) {
		t.Fatalf("brightness: %+v", got)
	}
	if _, err := renderEffectPixel(t, orange, strict, &dml.BlipEffect{FillOverlay: &dml.FillOverlayXML{Blend: "glow", SolidFill: &dml.SolidFill{SrgbClr: &dml.SrgbClr{Val: "000000"}}}}); !errors.Is(err, render.ErrInvalid) {
		t.Fatalf("bad blend: %v", err)
	}
}

func TestBlipOrderedEffects(t *testing.T) {
	var b dml.Blip
	src := `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:lum bright="10000"/><a:grayscl/><a:alphaModFix amt="50000"/></a:blip>`
	if err := xmlb.Unmarshal([]byte(src), &b); err != nil {
		t.Fatal(err)
	}
	effects := b.OrderedEffects()
	if len(effects) != 3 || effects[0].Lum == nil || effects[1].Grayscl == nil || effects[2].AlphaModFix == nil {
		t.Fatalf("%+v", effects)
	}
}

func TestRenderPictureColorEffects(t *testing.T) {
	data, opts, _ := renderPictureDeck(t, renderGreenPNG(t), "image/png")
	effect := func(x string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
			i := strings.Index(s, "<p:pic>")
			if i < 0 {
				t.Fatalf("no picture in %s", s)
			}
			rest := s[i:]
			j := strings.Index(rest, "<a:blip ")
			k := j + strings.Index(rest[j:], ">")
			if rest[k-1] == '/' {
				return s[:i] + rest[:k-1] + ">" + x + "</a:blip>" + rest[k+1:]
			}
			return s[:i] + rest[:k+1] + x + rest[k+1:]
		}}
	}
	// The 1px green picture fills (60,40) to (70,50); its luma is 149.7.
	for _, tc := range []struct {
		xml  string
		want color.NRGBA
	}{
		{`<a:grayscl/>`, color.NRGBA{150, 150, 150, 255}},
		{`<a:duotone><a:srgbClr val="000000"/><a:srgbClr val="FFFFFF"/></a:duotone>`, color.NRGBA{150, 150, 150, 255}},
		{`<a:biLevel thresh="50000"/>`, color.NRGBA{255, 255, 255, 255}},
		{`<a:clrChange><a:clrFrom><a:srgbClr val="00FF00"/></a:clrFrom><a:clrTo><a:srgbClr val="FF0000"/></a:clrTo></a:clrChange>`, color.NRGBA{255, 0, 0, 255}},
	} {
		got := renderSlidePNG(t, data, opts, effect(tc.xml))
		if px := renderPixel(t, got, 65, 45); px != tc.want {
			t.Fatalf("%s: %+v, want %+v", tc.xml, px, tc.want)
		}
	}
	// Brightness is refused strictly and drawn in best effort.
	lum := effect(`<a:lum bright="-100000"/>`)
	if _, err := renderRewrittenPNG(t, data, opts, lum); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict brightness: %v", err)
	}
	opts.Warn = func(error) {}
	if px := renderPixel(t, renderSlidePNG(t, data, opts, lum), 65, 45); px != (color.NRGBA{0, 0, 0, 255}) {
		t.Fatalf("darkened: %+v", px)
	}
}

func TestRenderPictureBlur(t *testing.T) {
	// Black then white, each half of a 20 pixel row drawn at a pixel each:
	// a 2px blur greys the pixels either side of the step, not the ends.
	img := image.NewNRGBA(image.Rect(0, 0, 20, 1))
	for x := 0; x < 20; x++ {
		v := uint8(0)
		if x >= 10 {
			v = 255
		}
		img.SetNRGBA(x, 0, color.NRGBA{v, v, v, 255})
	}
	var warnings []string
	colors := &renderColors{ctx: context.Background(), approx: func(err error) { warnings = append(warnings, err.Error()) }}
	out, err := colors.blipEffects(img, []*dml.BlipEffect{{Blur: &dml.BlurXML{Rad: 2 * 9525}}}, 9525)
	if err != nil {
		t.Fatal(err)
	}
	at := func(x int) uint8 { return color.NRGBAModel.Convert(out.At(x, 0)).(color.NRGBA).R }
	if at(9) == 0 || at(10) == 255 || at(0) != 0 || at(19) != 255 || at(9) >= at(10) {
		t.Fatalf("blurred row: %d %d %d %d", at(0), at(9), at(10), at(19))
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "picture blur drawn approximately") {
		t.Fatalf("warnings: %q", warnings)
	}
}
