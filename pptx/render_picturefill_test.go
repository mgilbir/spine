package pptx

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderPictureFillSlide is the text slide with a 2 by 1 pixel picture,
// green then blue, drawn off the page so that a shape can take its
// relationship for a picture fill. fill returns that shape's XML.
func renderPictureFillSlide(t *testing.T) ([]byte, render.Options, func(xfrm, geometry, blipFill string) map[string]func(string) string) {
	t.Helper()
	p, s, _, opts := renderTextSlide(t)
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}
	pic := NewPicture()
	pic.SetImageData(encoded.Bytes(), "image/png")
	pic.SetPosition(dml.Pixels(200), dml.Pixels(200))
	pic.SetSize(dml.Pixels(2), dml.Pixels(1))
	if err := s.AddShape(pic); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	embed := regexp.MustCompile(`r:embed="([^"]+)"`)
	fill := func(xfrm, geometry, blipFill string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			m := embed.FindStringSubmatch(s)
			if m == nil {
				t.Fatalf("no picture in %s", s)
			}
			s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
			// A 20 by 10px shape at (60,40).
			return renderAddToTree(`<p:sp><p:nvSpPr><p:cNvPr id="90" name="Filled"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm` + xfrm + `><a:off x="571500" y="381000"/><a:ext cx="190500" cy="95250"/></a:xfrm>` + geometry + strings.ReplaceAll(blipFill, "EMBED", m[1]) + `</p:spPr></p:sp>`)(s)
		}}
	}
	return data, opts, fill
}

func TestRenderPictureFills(t *testing.T) {
	data, opts, fill := renderPictureFillSlide(t)
	rect, ellipse := `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`, `<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>`
	stretch := `<a:blipFill><a:blip r:embed="EMBED"/><a:stretch><a:fillRect/></a:stretch></a:blipFill>`
	green, blue, white := color.NRGBA{G: 255, A: 255}, color.NRGBA{B: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	near := func(a, b color.NRGBA) bool {
		d := func(x, y uint8) bool { return x-y < 24 || y-x < 24 }
		return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
	}
	for _, tc := range []struct {
		name, xfrm, geometry, fill string
		want                       map[[2]int]color.NRGBA
	}{
		// Stretched over the box, green on the left and blue on the right.
		{"stretched", "", rect, stretch, map[[2]int]color.NRGBA{{62, 45}: green, {78, 45}: blue, {58, 45}: white}},
		// Clipped to the ellipse, whose corners stay white.
		{"ellipse", "", ellipse, stretch, map[[2]int]color.NRGBA{{62, 45}: green, {78, 45}: blue, {60, 40}: white, {79, 49}: white}},
		// Inset to the right half of the box.
		{"fill rectangle", "", rect, strings.Replace(stretch, `<a:fillRect/>`, `<a:fillRect l="50000"/>`, 1), map[[2]int]color.NRGBA{{62, 45}: white, {72, 45}: green, {78, 45}: blue}},
		// Cropped to its right pixel.
		{"source rectangle", "", rect, strings.Replace(stretch, `<a:stretch>`, `<a:srcRect l="50000"/><a:stretch>`, 1), map[[2]int]color.NRGBA{{62, 45}: blue, {78, 45}: blue}},
		// Half transparent over white.
		{"opacity", "", rect, strings.Replace(stretch, `<a:blip r:embed="EMBED"/>`, `<a:blip r:embed="EMBED"><a:alphaModFix amt="50000"/></a:blip>`, 1), map[[2]int]color.NRGBA{{62, 45}: {R: 127, G: 255, B: 127, A: 255}}},
		// Mirrored with the shape.
		{"flipped", ` flipH="1"`, rect, stretch, map[[2]int]color.NRGBA{{62, 45}: blue, {78, 45}: green}},
		// Turned a quarter with the shape about (70,45): 10 by 20px, green
		// above blue.
		{"turned", ` rot="5400000"`, rect, stretch, map[[2]int]color.NRGBA{{70, 38}: green, {70, 52}: blue, {62, 45}: white}},
	} {
		got := renderSlidePNG(t, data, opts, fill(tc.xfrm, tc.geometry, tc.fill))
		for at, want := range tc.want {
			if px := renderPixel(t, got, at[0], at[1]); !near(px, want) {
				t.Fatalf("%s at %v: %+v, want %+v", tc.name, at, px, want)
			}
		}
	}
	// A tiled picture is drawn stretched, approximately.
	tiled := fill("", rect, `<a:blipFill><a:blip r:embed="EMBED"/><a:tile/></a:blipFill>`)
	if _, err := renderRewrittenPNG(t, data, opts, tiled); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict tile: %v", err)
	}
	// A fill naming no picture fails.
	missing := fill("", rect, `<a:blipFill><a:blip r:embed="rId999"/><a:stretch><a:fillRect/></a:stretch></a:blipFill>`)
	if _, err := renderRewrittenPNG(t, data, opts, missing); err == nil {
		t.Fatal("missing picture drawn")
	}
}
