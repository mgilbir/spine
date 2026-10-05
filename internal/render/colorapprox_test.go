package render

import (
	"context"
	"errors"
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
)

// fixtureFace loads one of the forme test fonts in testdata, copied from
// forme's testdata/harfbuzz/fonts, where paint_fixture.py and colrink_fixture.py
// build them: ColourPaint, ColourInk, SbixInk, BitmapInk and SVGPaint.
func fixtureFace(t testing.TB, name string) *shape.Face {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	f, err := shape.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// approximated runs a glyph through strict and best-effort preparation, and
// returns the page of the second and what it reported. Strict preparation must
// refuse the glyph.
func approximated(t *testing.T, face *shape.Face, text style.RGBA, gid int) (*Page, []string) {
	t.Helper()
	if _, err := prepareColor(face, text, Limits{}, nil, gid); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	var reports []string
	p, err := prepareColor(face, text, Limits{}, collect(&reports), gid)
	if err != nil {
		t.Fatal(err)
	}
	return p, reports
}

func rasterOf(t *testing.T, p *Page) (img interface{ RGBAAt(x, y int) color.RGBA }) {
	t.Helper()
	r, err := p.Raster(context.Background(), 96)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestColorGlyphApproximations(t *testing.T) {
	sweep := paintGlyph(gidFull, paintSweep(colorLine(0, stops2...), 500, 500, 0, 1))
	line := colorLine(0, stops2...)
	cases := []struct {
		name   string
		paint  *paintNode
		report string
		// at is where to look, and want what is there: the one color the
		// gradient is drawn as, or the text color of the outline.
		x, y float64
		want color.RGBA
	}{
		{"sweep", sweep, "sweep gradient", 300, 300, color.RGBA{128, 0, 128, 255}},
		{"radial between circles of different centers", paintGlyph(gidFull, paintRadial(line, 300, 500, 0, 500, 500, 300)), "different centers", 500, 500, red},
		{"radial skewed", paintTransform(paintGlyph(gidFull, paintRadial(line, 500, 500, 0, 500, 500, 500)), 1, 0, 0.5, 1, 0, 0), "turned or skewed", 700, 500, mix(red, blue, 0.08)},
		{"radial between equal circles", paintGlyph(gidFull, paintRadial(line, 500, 500, 100, 500, 500, 100)), "degenerate radial", 300, 300, color.RGBA{128, 0, 128, 255}},
		{"linear along its own line", paintGlyph(gidFull, paintLinear(line, 0, 0, 500, 500, 100, 100)), "degenerate linear", 300, 300, color.RGBA{128, 0, 128, 255}},
		{"line without extent, repeating", paintGlyph(gidFull, paintLinear(colorLine(1, colorStop{0.5, 0, 1}, colorStop{0.5, 1, 1}), 0, 0, 1000, 0, 0, 100)), "without extent", 300, 300, blue},
		{"line repeating too often", paintGlyph(gidFull, paintLinear(colorLine(1, colorStop{0, 0, 1}, colorStop{0.0001, 1, 1}), 0, 0, 1000, 0, 0, 100)), "repeating too often", 300, 300, color.RGBA{128, 0, 128, 255}},
		// A group composited in any mode but source-over is not drawn: the glyph
		// is its outline, a marker at its center, in the text color.
		{"multiply", paintComposite(paintGlyph(gidLower, paintSolid(0, 1)), 23, paintGlyph(gidFull, paintSolid(1, 1))), "composite mode 23", 500, 500, color.RGBA{0, 0, 0, 255}},
		{"in", paintComposite(paintGlyph(gidLower, paintSolid(0, 1)), 5, paintGlyph(gidFull, paintSolid(1, 1))), "composite mode 5", 500, 500, color.RGBA{0, 0, 0, 255}},
	}
	var bases []*paintNode
	for _, c := range cases {
		bases = append(bases, c.paint)
	}
	face := colorFont(t, colorFace{bases: bases})
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, reports := approximated(t, face, black, firstBase+i)
			if len(reports) != 1 || !strings.Contains(reports[0], c.report) {
				t.Fatalf("reported %q, want %q", reports, c.report)
			}
			img := rasterOf(t, p)
			px, py := int(10+c.x/10), int(110-c.y/10)
			near(t, img.RGBAAt(px, py), c.want, 8, c.name)
		})
	}
	// The glyphs drawn as outlines draw nothing else.
	for i := len(cases) - 2; i < len(cases); i++ {
		p, _ := approximated(t, face, black, firstBase+i)
		near(t, rasterOf(t, p).RGBAAt(30, 90), nothing, 0, cases[i].name+" outside the outline")
	}
}

func TestColorGlyphExactCompositing(t *testing.T) {
	over := paintComposite(paintGlyph(gidLower, paintSolid(0, 1)), 3, paintGlyph(gidFull, paintSolid(1, 1)))
	dest := paintComposite(paintGlyph(gidLower, paintSolid(0, 1)), 2, paintGlyph(gidFull, paintSolid(1, 1)))
	face := colorFont(t, colorFace{bases: []*paintNode{over, dest}})
	img := colorRaster(t, face, black, firstBase)
	near(t, at(img, 250, 250), red, 0, "source over backdrop")
	near(t, at(img, 750, 750), blue, 0, "backdrop")
	img = colorRaster(t, face, black, firstBase+1)
	near(t, at(img, 250, 250), blue, 0, "only the backdrop")
}

// A paint with no stops draws nothing, and one with a single stop is a solid.
func TestColorGlyphDegenerateGradients(t *testing.T) {
	face := colorFont(t, colorFace{bases: []*paintNode{
		paintGlyph(gidFull, paintLinear(colorLine(0), 0, 0, 1000, 0, 0, 100)),
		paintGlyph(gidFull, paintRadial(colorLine(0, colorStop{0.3, 1, 1}), 500, 500, 0, 500, 500, 500)),
		paintGlyph(gidFull, paintLinear(colorLine(0, colorStop{0.5, 0, 1}, colorStop{0.5, 1, 1}), 0, 0, 1000, 0, 0, 100)),
	}})
	img := colorRaster(t, face, black, firstBase)
	near(t, at(img, 500, 500), nothing, 0, "no stops")
	img = colorRaster(t, face, black, firstBase+1)
	near(t, at(img, 500, 500), blue, 0, "one stop")
	img = colorRaster(t, face, black, firstBase+2)
	near(t, at(img, 255, 500), red, 0, "a hard stop, before")
	near(t, at(img, 755, 500), blue, 0, "a hard stop, after")
}

// Approximations are reported once, however many glyphs there are.
func TestColorGlyphReportsOnce(t *testing.T) {
	face := colorFont(t, colorFace{bases: []*paintNode{paintGlyph(gidFull, paintSweep(colorLine(0, stops2...), 500, 500, 0, 1))}})
	var reports []string
	if _, err := prepareColor(face, black, Limits{}, collect(&reports), firstBase, firstBase, firstBase); err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports: %q", reports)
	}
}

func TestColorGlyphFixtureFonts(t *testing.T) {
	// ColourPaint's glyphs: 15 is a radial gradient between circles of
	// different centers, 20 a sweep, 6 a multiplied composite, 18 a solid, 31
	// COLRv0 layers.
	paint := fixtureFace(t, "ColourPaint.ttf")
	for gid, report := range map[int]string{15: "different centers", 20: "sweep gradient", 6: "composite mode"} {
		if _, reports := approximated(t, paint, black, gid); len(reports) == 0 || !strings.Contains(strings.Join(reports, ";"), report) {
			t.Fatalf("glyph %d reported %q, want %q", gid, reports, report)
		}
	}
	for _, gid := range []int{18, 31, 7, 14} {
		p, err := prepareColor(paint, black, Limits{}, nil, gid)
		if err != nil {
			t.Fatalf("glyph %d: %v", gid, err)
		}
		if _, err := p.Raster(context.Background(), 96); err != nil {
			t.Fatal(err)
		}
	}
	// A glyph with an SVG document is drawn as its outline.
	svg := fixtureFace(t, "SVGPaint.ttf")
	p, reports := approximated(t, svg, black, 1)
	if len(reports) != 1 || !strings.Contains(reports[0], "SVG glyph") || len(p.draws) != 1 || p.draws[0].path == nil {
		t.Fatalf("SVG glyph: %q, %d drawings", reports, len(p.draws))
	}
	// The fixtures' bitmaps are placeholders, not images.
	for _, name := range []string{"BitmapInk.ttf", "SbixInk.ttf"} {
		_, reports := approximated(t, fixtureFace(t, name), black, 1)
		if len(reports) != 1 || !strings.Contains(reports[0], "cannot be read") {
			t.Fatalf("%s: %q", name, reports)
		}
	}
}
