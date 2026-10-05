package pptxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// renderColorFace is a 2000-unit font whose letter A is a red box and whose
// letter B is a box of a sweep gradient, which the page cannot draw: both are
// COLRv1 glyphs of the box in the third glyph, 1.6 em high.
func renderColorFace(t *testing.T) *shape.Face {
	t.Helper()
	be16 := func(v int) []byte { return []byte{byte(v >> 8), byte(v)} }
	be32 := func(v int) []byte { return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)} }
	cat := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	// PaintGlyph of glyph 3, then what it paints, 6 bytes on.
	solid := cat([]byte{10, 0, 0, 6}, be16(3), []byte{2}, be16(0), be16(0x4000))
	sweep := cat([]byte{10, 0, 0, 6}, be16(3),
		[]byte{8, 0, 0, 12}, be16(1000), be16(800), be16(0), be16(0x4000),
		[]byte{0}, be16(2), be16(0), be16(0), be16(0x4000), be16(0x4000), be16(1), be16(0x4000))
	const header = 34
	list := cat(be32(2), be16(1), be32(4+12), be16(2), be32(4+12+len(solid)), solid, sweep)
	colr := cat(be16(1), be16(0), be32(header), be32(header), be16(0), be32(header), be32(0), be32(0), be32(0), be32(0), list)
	cpal := cat(be16(0), be16(2), be16(1), be16(2), be32(14), be16(0), []byte{0, 0, 255, 255, 255, 0, 0, 255})
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{UnitsPerEm: 2000, Ascent: 1600, Descent: -400, Glyphs: []fonttest.Glyph{
		{Rune: 'A', Advance: 2000, HasShape: true, Ink: [4]int{900, 700, 1100, 900}},
		{Rune: 'B', Advance: 2000, HasShape: true, Ink: [4]int{900, 700, 1100, 900}},
		{Unmapped: true, Advance: 2000, HasShape: true, Ink: [4]int{0, 0, 2000, 1600}},
		{Rune: ' ', Advance: 1000},
	}, Extra: map[string][]byte{"COLR": colr, "CPAL": cpal}}))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestRenderColorGlyphs(t *testing.T) {
	data, opts := renderInheritedText(t)
	face := renderColorFace(t)
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }
	text := func(letter, shapeXfrm string) map[string]func(string) string {
		rewrites := map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, `<a:p><a:r><a:rPr lang="en-US"/><a:t>`+letter+`</a:t></a:r></a:p>`)}
		if shapeXfrm != "" {
			body := rewrites["ppt/slides/slide1.xml"]
			rewrites["ppt/slides/slide1.xml"] = func(s string) string {
				s = body(s)
				i := strings.Index(s, "<p:sp>")
				return s[:i] + strings.Replace(s[i:], `<a:xfrm>`, shapeXfrm, 1)
			}
		}
		return rewrites
	}
	isRed := func(r, g, b uint8) bool { return r > 240 && g < 20 && b < 20 }
	isBlack := func(r, g, b uint8) bool { return r < 20 && g < 20 && b < 20 }
	isMean := func(r, g, b uint8) bool { return r > 100 && r < 150 && g < 20 && b > 100 && b < 150 }

	// The A is drawn red, exactly: strict preparation draws it.
	got := renderSlidePNG(t, data, opts, text("A", ""))
	if !renderInk(t, got, 0, 0, 80, 60, isRed) || renderInk(t, got, 0, 0, 80, 60, isBlack) {
		t.Fatal("color glyph not drawn in its colors")
	}

	// The B is drawn as one color, approximately.
	if _, err := renderRewrittenPNG(t, data, opts, text("B", "")); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	var warnings []string
	lenient := opts
	lenient.Warn = func(err error) {
		// The slide has other warnings of its own.
		if errors.Is(err, render.ErrApproximated) {
			if !errors.Is(err, render.ErrUnsupported) {
				t.Errorf("%v does not say what is unsupported", err)
			}
			warnings = append(warnings, err.Error())
		}
	}
	got = renderSlidePNG(t, data, lenient, text("B", ""))
	if len(warnings) != 1 || !strings.Contains(warnings[0], "sweep gradient drawn as one color") {
		t.Fatalf("warnings: %q", warnings)
	}
	if !renderInk(t, got, 0, 0, 80, 60, isMean) {
		t.Fatal("approximated color glyph not drawn")
	}

	// A turned shape draws its text as outlines, which have no colors.
	turned := `<a:xfrm rot="5400000">`
	if _, err := renderRewrittenPNG(t, data, opts, text("A", turned)); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict, turned: %v", err)
	}
	warnings = nil
	got = renderSlidePNG(t, data, lenient, text("A", turned))
	if len(warnings) != 1 || !strings.Contains(warnings[0], "turned color glyphs") {
		t.Fatalf("turned warnings: %q", warnings)
	}
	if !renderInk(t, got, 0, 0, 80, 60, isBlack) || renderInk(t, got, 0, 0, 80, 60, isRed) {
		t.Fatal("turned color glyph not drawn in the text color")
	}
}
