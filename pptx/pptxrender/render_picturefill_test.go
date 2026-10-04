package pptxrender

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
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
	pic := pptx.NewPicture()
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
	// Tiled, the 2 by 1 pixel picture at 96 DPI repeats every 2px from the
	// box's left edge, x 60.
	tile := func(attrs string) string {
		return `<a:blipFill><a:blip r:embed="EMBED"/><a:tile` + attrs + `/></a:blipFill>`
	}
	for _, tc := range []struct {
		name, fill string
		want       map[int]color.NRGBA
	}{
		{"tiled", tile(""), map[int]color.NRGBA{60: green, 61: blue, 70: green, 71: blue}},
		// Five times larger, each color 5px wide.
		{"scaled", tile(` sx="500000" sy="500000"`), map[int]color.NRGBA{62: green, 67: blue, 72: green}},
		// Every other tile mirrored.
		{"flipped", tile(` sx="500000" sy="500000" flip="x"`), map[int]color.NRGBA{62: green, 67: blue, 72: blue, 77: green}},
		// The first tile centred, x 65 to 75.
		{"centred", tile(` sx="500000" sy="500000" algn="ctr"`), map[int]color.NRGBA{62: blue, 67: green, 72: blue}},
		// Moved 3px (28575 EMU) right.
		{"offset", tile(` sx="500000" sy="500000" tx="28575"`), map[int]color.NRGBA{62: blue, 64: green, 69: blue}},
		// At 48 DPI, each pixel is 2px.
		{"dpi", strings.Replace(tile(""), `<a:blipFill>`, `<a:blipFill dpi="48">`, 1), map[int]color.NRGBA{60: green, 62: blue, 64: green}},
	} {
		got := renderSlidePNG(t, data, opts, fill("", rect, tc.fill))
		for x, want := range tc.want {
			if px := renderPixel(t, got, x, 45); !near(px, want) {
				t.Fatalf("%s at %d: %+v, want %+v", tc.name, x, px, want)
			}
		}
	}
	bad := fill("", rect, tile(` flip="sideways"`))
	if _, err := renderRewrittenPNG(t, data, opts, bad); !errors.Is(err, render.ErrInvalid) {
		t.Fatalf("bad flip: %v", err)
	}
	// A fill naming no picture fails.
	missing := fill("", rect, `<a:blipFill><a:blip r:embed="rId999"/><a:stretch><a:fillRect/></a:stretch></a:blipFill>`)
	if _, err := renderRewrittenPNG(t, data, opts, missing); err == nil {
		t.Fatal("missing picture drawn")
	}
}

func TestRenderTiledBackground(t *testing.T) {
	data, opts, _ := renderPictureFillSlide(t)
	embed := regexp.MustCompile(`r:embed="([^"]+)"`)
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		m := embed.FindStringSubmatch(s)
		if m == nil || !strings.Contains(s, `<p:cSld><p:spTree>`) {
			t.Fatalf("slide: %s", s)
		}
		s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
		return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgPr><a:blipFill><a:blip r:embed="`+m[1]+`"/><a:tile/></a:blipFill><a:effectLst/></p:bgPr></p:bg><p:spTree>`, 1)
	}})
	// The 2 by 1 pixel picture repeats from the slide's corner.
	green, blue := color.NRGBA{G: 255, A: 255}, color.NRGBA{B: 255, A: 255}
	for at, want := range map[[2]int]color.NRGBA{{0, 0}: green, {1, 0}: blue, {10, 30}: green, {31, 50}: blue} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("at %v: %+v, want %+v", at, px, want)
		}
	}
}

func TestRenderTileSizeReadsResolution(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 30, 10))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}
	plain := encoded.Bytes()
	// 192 DPI (7559 pixels per metre) halves the picture's size.
	dense := append(append(append([]byte(nil), plain[:33]...), []byte{0, 0, 0, 9, 'p', 'H', 'Y', 's', 0, 0, 0x1d, 0x87, 0, 0, 0x1d, 0x87, 1, 0, 0, 0, 0}...), plain[33:]...)
	for _, tc := range []struct {
		name string
		data []byte
		dpi  *int32
		tile dml.TileXML
		w, h float64
	}{
		{"96 DPI", plain, nil, dml.TileXML{}, 30, 10},
		{"file", dense, nil, dml.TileXML{}, 15, 5},
		{"fill over file", dense, func() *int32 { v := int32(48); return &v }(), dml.TileXML{}, 60, 20},
		{"scaled", plain, nil, dml.TileXML{Sx: dml.NewPercentage(50000), Sy: dml.NewPercentage(200000)}, 15, 20},
	} {
		if w, h := renderTileSize(src, tc.data, tc.dpi, &tc.tile); math.Abs(w-tc.w) > 0.1 || math.Abs(h-tc.h) > 0.1 {
			t.Errorf("%s: %v by %v, want %v by %v", tc.name, w, h, tc.w, tc.h)
		}
	}
}

func TestRenderTileImageBudget(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	colors := &renderColors{}
	// A 1000 by 1000 pixel box at the picture's density, four pixels per
	// box pixel, is capped at four million pixels.
	img, err := renderTileImage(colors, src, 1000, 1000, 1, 1, &dml.TileXML{})
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); int64(b.Dx())*int64(b.Dy()) > 2*renderMaxTilePixels || colors.tilePixels == 0 {
		t.Fatalf("composed %v, counted %d", b, colors.tilePixels)
	}
	// The slide's tiled fills share four times that.
	colors.tilePixels = 4*renderMaxTilePixels - 10
	if _, err := renderTileImage(colors, src, 100, 100, 4, 4, &dml.TileXML{}); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("slide budget: %v", err)
	}
	for _, bad := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := renderTileImage(&renderColors{}, src, 10, 10, bad, 4, &dml.TileXML{}); !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("tile width %v: %v", bad, err)
		}
	}
}
