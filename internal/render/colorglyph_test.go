package render

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

// The glyphs are drawn 100 pixels to the em on a 120 by 120 page, with the
// origin at (10, 110): the font point (x, y) is the pixel (10 + x/10, 110 - y/10).
const (
	colorPage = 120
	colorSize = 100
)

var (
	black   = style.RGBA{A: 1}
	red     = color.RGBA{255, 0, 0, 255}
	blue    = color.RGBA{0, 0, 255, 255}
	green   = color.RGBA{0, 255, 0, 255}
	nothing = color.RGBA{}
)

// colorOps draws glyphs of a face, one after the other, from the origin.
func colorOps(face *shape.Face, text style.RGBA, gids ...int) []layout.Op {
	glyphs := make([]shape.Glyph, len(gids))
	for i, gid := range gids {
		glyphs[i] = shape.Glyph{GID: gid, XAdvance: 1000}
	}
	return []layout.Op{layout.DrawGlyphs{At: layout.Point{X: unit(10), Y: unit(110)}, Face: face, Size: unit(colorSize), Color: text, Glyphs: glyphs, Text: "x"}}
}

func prepareColor(face *shape.Face, text style.RGBA, limits Limits, approximate func(error), gids ...int) (*Page, error) {
	return PrepareBestEffort(context.Background(), dml.Pixels(colorPage), dml.Pixels(colorPage), colorOps(face, text, gids...), limits, approximate)
}

// colorRaster draws a glyph in strict preparation.
func colorRaster(t *testing.T, face *shape.Face, text style.RGBA, gid int) *image.RGBA {
	t.Helper()
	p, err := prepareColor(face, text, Limits{}, nil, gid)
	if err != nil {
		t.Fatal(err)
	}
	img, err := p.Raster(context.Background(), 96)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// probe is the pixel at a font point, premultiplied, and the point at the
// pixel's center.
func probe(img *image.RGBA, x, y float64) (color.RGBA, float64, float64) {
	px, py := int(math.Floor(10+x/10)), int(math.Floor(110-y/10))
	return img.RGBAAt(px, py), (float64(px) + 0.5 - 10) * 10, (110 - float64(py) - 0.5) * 10
}

func at(img *image.RGBA, x, y float64) color.RGBA {
	c, _, _ := probe(img, x, y)
	return c
}

func near(t *testing.T, got, want color.RGBA, tolerance int, what string) {
	t.Helper()
	d := func(a, b uint8) int { return int(math.Abs(float64(a) - float64(b))) }
	if d(got.R, want.R) > tolerance || d(got.G, want.G) > tolerance || d(got.B, want.B) > tolerance || d(got.A, want.A) > tolerance {
		t.Fatalf("%s: %v, want %v", what, got, want)
	}
}

// mix is the color a fraction w of the way from a to b, opaque, as the
// gradients' stops blend.
func mix(a, b color.RGBA, w float64) color.RGBA {
	f := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-w) + float64(y)*w)) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

func TestColorGlyphSolidAndForeground(t *testing.T) {
	face := colorFont(t, colorFace{bases: []*paintNode{
		paintGlyph(gidFull, paintSolid(0, 1)),
		paintGlyph(gidLower, paintSolid(0xFFFF, 0.5)),
		paintGlyph(gidFull, paintSolid(1, 0.5)),
	}})
	img := colorRaster(t, face, black, firstBase)
	near(t, at(img, 500, 500), red, 0, "solid")
	near(t, at(img, 20, 20), red, 0, "solid corner")
	near(t, img.RGBAAt(5, 5), nothing, 0, "outside")
	near(t, img.RGBAAt(115, 60), nothing, 0, "outside right")
	// The glyph's own outline, a small marker, is not drawn on top.
	img = colorRaster(t, face, style.RGBA{G: 200, A: 1}, firstBase+1)
	near(t, at(img, 250, 250), color.RGBA{0, 100, 0, 128}, 1, "foreground at half alpha")
	near(t, at(img, 750, 250), nothing, 0, "outside the clip")
	// The text's alpha applies to the foreground only.
	img = colorRaster(t, face, style.RGBA{G: 200, A: 0.5}, firstBase+1)
	near(t, at(img, 250, 250), color.RGBA{0, 50, 0, 64}, 1, "foreground, text at half alpha")
	img = colorRaster(t, face, style.RGBA{G: 200, A: 0.5}, firstBase+2)
	near(t, at(img, 250, 250), color.RGBA{0, 0, 128, 128}, 1, "palette color unaffected by text alpha")
}

func TestColorGlyphLayersV0(t *testing.T) {
	face := colorFont(t, colorFace{v0: [][][2]int{{{gidLower, 0}, {gidFull, 1}}, {{gidLower, 0xFFFF}, {gidRight, 2}}}})
	img := colorRaster(t, face, style.RGBA{G: 128, A: 1}, firstBase)
	near(t, at(img, 250, 250), blue, 0, "upper layer covers the lower")
	img = colorRaster(t, face, style.RGBA{G: 128, A: 1}, firstBase+1)
	near(t, at(img, 250, 250), color.RGBA{0, 128, 0, 255}, 1, "foreground layer")
	near(t, at(img, 750, 250), green, 0, "second layer")
	near(t, at(img, 750, 750), nothing, 0, "outside")
}

func TestColorGlyphLayerList(t *testing.T) {
	face := colorFont(t, colorFace{
		layers: []*paintNode{paintGlyph(gidFull, paintSolid(0, 1)), paintGlyph(gidLower, paintSolid(1, 1))},
		bases:  []*paintNode{paintLayers(0, 2)},
	})
	img := colorRaster(t, face, black, firstBase)
	near(t, at(img, 250, 250), blue, 0, "second layer")
	near(t, at(img, 750, 750), red, 0, "first layer")
}

// stops2 runs from red to blue.
var stops2 = []colorStop{{0, 0, 1}, {1, 1, 1}}

// fold is a color line's parameter within its stops, 0 to 1, as its extend
// mode takes it there.
func fold(t float64, extend int) float64 {
	switch extend {
	case 1:
		return t - math.Floor(t)
	case 2:
		t = math.Mod(t, 2)
		if t < 0 {
			t += 2
		}
		if t > 1 {
			t = 2 - t
		}
		return t
	}
	return math.Max(0, math.Min(1, t))
}

// gradientCase is a gradient painted into a glyph, and what it should paint at
// each font point: the fraction of the way from red to blue, as the font's own
// geometry works it out, independently of the page.
type gradientCase struct {
	name   string
	paint  *paintNode
	weight func(x, y float64) float64
	// inside says where the gradient is painted, and defaults to the full box.
	inside func(x, y float64) bool
	// jumps, for a gradient that repeats, is the parameter, which jumps where
	// it comes to a whole number.
	jumps func(x, y float64) float64
}

func inBox(x, y float64) bool { return x > 30 && x < 970 && y > 30 && y < 970 }

func TestColorGlyphGradients(t *testing.T) {
	line := func(extend int) *paintNode { return colorLine(extend, stops2...) }
	linear := func(extend, x0, y0, x1, y1, x2, y2 int) *paintNode {
		return paintGlyph(gidFull, paintLinear(line(extend), x0, y0, x1, y1, x2, y2))
	}
	radial := func(extend, r0, r1 int) *paintNode {
		return paintGlyph(gidFull, paintRadial(line(extend), 500, 500, r0, 500, 500, r1))
	}
	dist := func(x, y, cx, cy float64) float64 { return math.Hypot(x-cx, y-cy) }
	// The point of the glyph a point of the page is, under a turn of 45 degrees
	// and a move of 500 to the right.
	turned := func(x, y float64) (float64, float64) {
		return (x - 500 + y) / math.Sqrt2, (y - x + 500) / math.Sqrt2
	}
	inLower := func(x, y float64) bool {
		u, v := turned(x, y)
		return u > 60 && u < 440 && v > 60 && v < 440
	}
	cases := []gradientCase{
		{name: "linear", paint: linear(0, 0, 0, 1000, 0, 0, 100), weight: func(x, y float64) float64 { return x / 1000 }},
		{name: "linear padded", paint: linear(0, 250, 0, 750, 0, 250, 100), weight: func(x, y float64) float64 { return fold((x-250)/500, 0) }},
		{name: "linear repeat", paint: linear(1, 0, 0, 250, 0, 0, 100), weight: func(x, y float64) float64 { return fold(x/250, 1) }, jumps: func(x, y float64) float64 { return x / 250 }},
		{name: "linear reflect", paint: linear(2, 0, 0, 250, 0, 0, 100), weight: func(x, y float64) float64 { return fold(x/250, 2) }},
		// The colors stand along the line from P0 to P2, a diagonal, so the
		// parameter is (x - y) / 1000.
		{name: "linear turned", paint: linear(0, 0, 0, 1000, 0, 500, 500), weight: func(x, y float64) float64 { return fold((x-y)/1000, 0) }},
		{name: "linear reflect turned", paint: linear(2, 0, 0, 500, 0, 500, 500), weight: func(x, y float64) float64 { return fold((x-y)/500, 2) }},
		// Stops out of order and past the ends: red at -1/4 and blue at 5/4.
		{name: "linear unsorted", paint: paintGlyph(gidFull, paintLinear(colorLine(0, colorStop{1.25, 1, 1}, colorStop{-0.25, 0, 1}), 0, 0, 1000, 0, 0, 100)),
			weight: func(x, y float64) float64 { return (x/1000 + 0.25) / 1.5 }},
		{name: "linear moved and scaled", paint: paintTransform(linear(0, 0, 0, 1000, 0, 0, 100), 0.5, 0, 0, 0.5, 250, 250),
			weight: func(x, y float64) float64 { return fold((x-250)/500, 0) }, inside: func(x, y float64) bool { return x > 280 && x < 720 && y > 280 && y < 720 }},
		{name: "linear turned by the glyph", paint: paintTranslate(paintRotate(paintGlyph(gidLower, paintLinear(line(0), 0, 0, 500, 0, 0, 100)), 0.25), 500, 0),
			weight: func(x, y float64) float64 { u, _ := turned(x, y); return fold(u/500, 0) }, inside: inLower},
		{name: "radial", paint: radial(0, 0, 500), weight: func(x, y float64) float64 { return fold(dist(x, y, 500, 500)/500, 0) }},
		{name: "radial from a radius", paint: radial(0, 200, 400), weight: func(x, y float64) float64 { return fold((dist(x, y, 500, 500)-200)/200, 0) }},
		{name: "radial inwards", paint: radial(0, 400, 200), weight: func(x, y float64) float64 { return fold((dist(x, y, 500, 500)-400)/-200, 0) }},
		{name: "radial repeat", paint: radial(1, 0, 200), weight: func(x, y float64) float64 { return fold(dist(x, y, 500, 500)/200, 1) }, jumps: func(x, y float64) float64 { return dist(x, y, 500, 500) / 200 }},
		{name: "radial repeat from a radius", paint: radial(1, 100, 300), weight: func(x, y float64) float64 { return fold((dist(x, y, 500, 500)-100)/200, 1) }, jumps: func(x, y float64) float64 { return (dist(x, y, 500, 500) - 100) / 200 }},
		{name: "radial reflect", paint: radial(2, 0, 200), weight: func(x, y float64) float64 { return fold(dist(x, y, 500, 500)/200, 2) }},
		// Scaled unevenly the circle about (400, 400) is an ellipse about
		// (600, 200), 300 by 100.
		{name: "radial ellipse", paint: paintScale(paintGlyph(gidFull, paintRadial(line(0), 400, 400, 0, 400, 400, 200)), 1.5, 0.5),
			weight: func(x, y float64) float64 { return fold(dist(x/1.5, y/0.5, 400, 400)/200, 0) }, inside: func(x, y float64) bool { return x > 30 && x < 1090 && y > 30 && y < 470 }},
		{name: "radial turned by the glyph", paint: paintTranslate(paintRotate(paintGlyph(gidLower, paintRadial(line(0), 250, 250, 0, 250, 250, 250)), 0.25), 500, 0),
			weight: func(x, y float64) float64 { u, v := turned(x, y); return fold(dist(u, v, 250, 250)/250, 0) }, inside: inLower},
	}
	var bases []*paintNode
	for _, c := range cases {
		bases = append(bases, c.paint)
	}
	face := colorFont(t, colorFace{bases: bases})
	for i, c := range cases {
		inside := c.inside
		if inside == nil {
			inside = inBox
		}
		img := colorRaster(t, face, black, firstBase+i)
		probed := 0
		for gx := 0; gx < 30; gx++ {
			for gy := 0; gy < 30; gy++ {
				got, x, y := probe(img, 20+float64(gx)*47, 20+float64(gy)*47)
				if !inside(x, y) {
					continue
				}
				if c.jumps != nil {
					if edge := c.jumps(x, y); math.Abs(edge-math.Round(edge)) < 0.02 {
						continue
					}
				}
				near(t, got, mix(red, blue, c.weight(x, y)), 3, fmt.Sprintf("%s at %.1f, %.1f", c.name, x, y))
				probed++
			}
		}
		if probed < 50 {
			t.Fatalf("%s: %d points probed", c.name, probed)
		}
	}
}

// collect returns what best-effort preparation reports.
func collect(reports *[]string) func(error) {
	return func(err error) {
		if !errors.Is(err, ErrUnsupported) {
			panic(err)
		}
		*reports = append(*reports, err.Error())
	}
}
