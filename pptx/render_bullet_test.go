package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// renderBulletList is a list style whose first level hangs a "•" bullet in a
// 0.25" (24px) indent.
func renderBulletList(bullet string) string {
	return `<a:lstStyle><a:lvl1pPr marL="228600" indent="-228600">` + bullet + `</a:lvl1pPr></a:lstStyle>`
}

// renderInk reports whether any pixel in [x0,x1)x[y0,y1) passes the test.
func renderInk(t *testing.T, data []byte, x0, y0, x1, y1 int, test func(r, g, b uint8) bool) bool {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if test(uint8(r>>8), uint8(g>>8), uint8(b>>8)) {
				return true
			}
		}
	}
	return false
}

func TestRenderBullets(t *testing.T) {
	data, opts := renderInheritedText(t)
	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil }
	run := `<a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r></a:p>`
	slide := func(list, paragraphs string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(list, paragraphs)}
	}
	dark := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	reddish := func(r, g, b uint8) bool { return r > 128 && g < 100 && b < 100 }
	// The box's text starts at x 4: the bullet hangs at 4-28 and the text
	// starts at 28.
	got := renderSlidePNG(t, data, opts, slide(renderBulletList(`<a:buFont typeface="Arial"/><a:buChar char="•"/>`), run))
	if !renderInk(t, got, 4, 4, 16, 24, dark) {
		t.Fatal("bullet missing")
	}
	if !renderInk(t, got, 28, 4, 44, 24, dark) || renderInk(t, got, 16, 4, 27, 24, dark) {
		t.Fatal("text not at the left margin")
	}
	colored := renderSlidePNG(t, data, opts, slide(renderBulletList(`<a:buClr><a:srgbClr val="FF0000"/></a:buClr><a:buChar char="•"/>`), run))
	if !renderInk(t, colored, 4, 4, 16, 24, reddish) || renderInk(t, colored, 28, 4, 44, 24, reddish) {
		t.Fatal("bullet color")
	}
	// An empty paragraph shows no bullet.
	empty := renderSlidePNG(t, data, opts, slide(renderBulletList(`<a:buChar char="•"/>`), run+`<a:p><a:endParaRPr sz="1200"/></a:p>`))
	if renderInk(t, empty, 4, 24, 16, 44, dark) {
		t.Fatal("bullet on an empty paragraph")
	}
	// Numbered paragraphs each hang their number.
	numbered := renderSlidePNG(t, data, opts, slide(renderBulletList(`<a:buAutoNum type="arabicPeriod"/>`), run+run))
	if !renderInk(t, numbered, 4, 4, 16, 24, dark) || !renderInk(t, numbered, 4, 24, 16, 44, dark) {
		t.Fatal("numbers missing")
	}
	for name, tc := range map[string][2]string{
		"wider than the indent": {`<a:lstStyle><a:lvl1pPr marL="19050" indent="-19050"><a:buChar char="•"/></a:lvl1pPr></a:lstStyle>`, run},
		"taller than the line":  {renderBulletList(`<a:buSzPct val="300000"/><a:buChar char="•"/>`), run},
		"indent without bullet": {renderBulletList(`<a:buNone/>`), run},
	} {
		if _, err := renderRewrittenPNG(t, data, opts, slide(tc[0], tc[1])); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestRenderAutoNumber(t *testing.T) {
	for _, tc := range []struct {
		scheme string
		n      int32
		want   string
	}{
		{"arabicPeriod", 3, "3."}, {"arabicParenR", 12, "12)"}, {"arabicParenBoth", 1, "(1)"}, {"arabicPlain", 7, "7"},
		{"alphaLcPeriod", 1, "a."}, {"alphaUcParenR", 28, "BB)"}, {"romanLcPeriod", 4, "iv."}, {"romanUcParenBoth", 1994, "(MCMXCIV)"},
	} {
		if got, ok := renderAutoNumber(tc.scheme, tc.n); !ok || got != tc.want {
			t.Fatalf("%s %d: %q %v", tc.scheme, tc.n, got, ok)
		}
	}
	for _, scheme := range []string{"circleNumDbPlain", "arabicDbPeriod", "romanLc"} {
		if _, ok := renderAutoNumber(scheme, 1); ok {
			t.Fatalf("%s accepted", scheme)
		}
	}
}
