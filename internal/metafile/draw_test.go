package metafile

import (
	"context"
	"image"
	"image/color"
	"testing"
)

func draw(t *testing.T, data []byte, w, h int, opts Options) *image.NRGBA {
	t.Helper()
	img, err := Render(context.Background(), data, w, h, opts)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return img
}

func at(img *image.NRGBA, x, y int) color.NRGBA { return img.NRGBAAt(x, y) }

func near(a, b uint8, tol int) bool {
	d := int(a) - int(b)
	return d >= -tol && d <= tol
}

func expect(t *testing.T, img *image.NRGBA, x, y int, want color.NRGBA) {
	t.Helper()
	got := at(img, x, y)
	if !near(got.R, want.R, 2) || !near(got.G, want.G, 2) || !near(got.B, want.B, 2) || !near(got.A, want.A, 2) {
		t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

var (
	clear = color.NRGBA{}
	red   = color.NRGBA{255, 0, 0, 255}
	green = color.NRGBA{0, 255, 0, 255}
	blue  = color.NRGBA{0, 0, 255, 255}
	black = color.NRGBA{0, 0, 0, 255}
)

const nullPen = 0x80000008

func TestRectangleFill(t *testing.T) {
	e := newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen).rect(43, 10, 10, 30, 30)
	data := e.bytes()
	img := draw(t, data, 40, 40, Options{})
	expect(t, img, 20, 20, red)
	expect(t, img, 5, 5, clear)
	expect(t, img, 35, 35, clear)
	expect(t, img, 10, 20, red)
	expect(t, img, 9, 20, clear)
	// Stretched to twice the size, the same picture covers twice the pixels.
	big := draw(t, data, 80, 80, Options{})
	expect(t, big, 40, 40, red)
	expect(t, big, 19, 40, clear)
	expect(t, big, 21, 40, red)
	expect(t, big, 59, 40, red)
	expect(t, big, 61, 40, clear)
	// An anisotropic stretch.
	wide := draw(t, data, 80, 40, Options{})
	expect(t, wide, 41, 20, red)
	expect(t, wide, 19, 20, clear)
}

func TestPartialCoverage(t *testing.T) {
	// A rectangle edge half way through a pixel covers half of it.
	e := newEMF(40, 40).brush(1, 0, rgb(0, 0, 255), 0).sel(1).sel(nullPen).rect(43, 0, 0, 20, 40)
	img := draw(t, e.bytes(), 80, 40, Options{})
	// 20 device pixels over 40 output pixels: the edge is at x = 40.
	expect(t, img, 39, 10, blue)
	expect(t, img, 40, 10, clear)
	e = newEMF(40, 40).brush(1, 0, rgb(0, 0, 255), 0).sel(1).sel(nullPen).rect(43, 0, 0, 10, 40)
	img = draw(t, e.bytes(), 25, 40, Options{})
	// The edge at 6.25: the pixel 6 is a quarter covered.
	got := at(img, 6, 10)
	if got.A < 56 || got.A > 72 {
		t.Errorf("edge pixel alpha = %d, want about 64", got.A)
	}
}

func TestEllipseAndPolygonRules(t *testing.T) {
	e := newEMF(40, 40).brush(1, 0, rgb(0, 255, 0), 0).sel(1).sel(nullPen).rect(42, 2, 2, 38, 38)
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, green)
	expect(t, img, 3, 3, clear)
	expect(t, img, 20, 3, green)
	// A five-pointed star is filled in the middle by the winding rule only.
	star := []int32{20, 2, 31, 36, 2, 14, 38, 14, 9, 36}
	alt := newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen).r(19, 1).poly(3, star...)
	img = draw(t, alt.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, clear)
	expect(t, img, 20, 8, red)
	wind := newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen).r(19, 2).poly(3, star...)
	img = draw(t, wind.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, red)
}

func TestPenStroke(t *testing.T) {
	e := newEMF(40, 40).pen(1, 0, 6, rgb(0, 0, 0)).sel(1).r(27, 5, 20).r(54, 35, 20)
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, black)
	expect(t, img, 20, 18, black)
	expect(t, img, 20, 16, clear)
	expect(t, img, 20, 23, clear)
	// The default round caps run past the ends by half the width.
	expect(t, img, 3, 20, black)
	expect(t, img, 1, 20, clear)
	// Flat caps through an extended pen: style PS_GEOMETRIC | PS_ENDCAP_FLAT.
	f := newEMF(40, 40)
	f.handle(1)
	f.rec(95, words(1, 0, 0, 0, 0, 0x10000|0x200, 6, 0, rgb(0, 0, 0), 0, 0))
	f.sel(1).r(27, 5, 20).r(54, 35, 20)
	img = draw(t, f.bytes(), 40, 40, Options{})
	expect(t, img, 6, 20, black)
	expect(t, img, 3, 20, clear)
}

func TestJoins(t *testing.T) {
	// A right angle with bevel, round and miter joins differs at the corner.
	corner := func(style int32) *image.NRGBA {
		f := newEMF(40, 40)
		f.handle(1)
		f.rec(95, words(1, 0, 0, 0, 0, 0x10000|0x200|style, 10, 0, rgb(0, 0, 0), 0, 0))
		f.sel(1).poly(4, 5, 35, 5, 5, 35, 5)
		return draw(t, f.bytes(), 40, 40, Options{})
	}
	miter := corner(0x2000)
	expect(t, miter, 1, 1, black)
	bevel := corner(0x1000)
	expect(t, bevel, 1, 1, clear)
	expect(t, bevel, 3, 3, black)
	round := corner(0)
	expect(t, round, 0, 0, clear)
	expect(t, round, 2, 2, black)
}

func TestDashes(t *testing.T) {
	// A user style: 10 on, 10 off.
	f := newEMF(40, 40)
	f.handle(1)
	f.rec(95, append(words(1, 0, 0, 0, 0, 0x10000|0x200|7, 4, 0, rgb(0, 0, 0), 0, 2), words(10, 10)...))
	f.sel(1).r(27, 0, 20).r(54, 40, 20)
	img := draw(t, f.bytes(), 40, 40, Options{})
	expect(t, img, 5, 20, black)
	expect(t, img, 15, 20, clear)
	expect(t, img, 25, 20, black)
	expect(t, img, 35, 20, clear)
}

func TestPathFillStrokeAndClip(t *testing.T) {
	e := newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen)
	e.r(59).r(27, 10, 10).r(54, 30, 10).r(54, 30, 30).r(54, 10, 30).r(61).r(60)
	e.rec(62, words(0, 0, 40, 40))
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, red)
	expect(t, img, 5, 5, clear)
	// A path clip confines the next fill.
	c := newEMF(40, 40).brush(1, 0, rgb(0, 0, 255), 0).sel(1).sel(nullPen)
	c.r(59).rect(42, 5, 5, 25, 25).r(60).r(67, 5)
	c.rect(43, 0, 0, 40, 40)
	img = draw(t, c.bytes(), 40, 40, Options{})
	expect(t, img, 15, 15, blue)
	expect(t, img, 5, 30, clear)
	expect(t, img, 30, 30, clear)
	// A rectangle clip, then excluding part of it.
	d := newEMF(40, 40).brush(1, 0, rgb(0, 255, 0), 0).sel(1).sel(nullPen)
	d.rect(30, 10, 10, 30, 30).rect(29, 15, 15, 25, 25).rect(43, 0, 0, 40, 40)
	img = draw(t, d.bytes(), 40, 40, Options{})
	expect(t, img, 12, 12, green)
	expect(t, img, 20, 20, clear)
	expect(t, img, 5, 5, clear)
}

func TestSaveRestoreAndTransforms(t *testing.T) {
	// SaveDC, a new map mode and window, then RestoreDC.
	e := newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen)
	e.r(33)
	e.r(17, 8)                 // anisotropic
	e.r(9, 10, 10)             // window ext
	e.r(11, 40, 40)            // viewport ext
	e.rect(43, 1, 1, 3, 3)     // device 4..12
	e.r(34, -1)                // restore
	e.rect(43, 20, 20, 30, 30) // device 20..30
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 8, 8, red)
	expect(t, img, 2, 2, clear)
	expect(t, img, 25, 25, red)
	// A world transform that moves and doubles.
	w := newEMF(40, 40).brush(1, 0, rgb(0, 0, 255), 0).sel(1).sel(nullPen)
	w.r(35, f32(2), 0, 0, f32(2), f32(10), f32(10))
	w.rect(43, 0, 0, 5, 5)
	img = draw(t, w.bytes(), 40, 40, Options{})
	expect(t, img, 15, 15, blue)
	expect(t, img, 9, 9, clear)
	expect(t, img, 21, 21, clear)
}

func TestMetricMappingFlipsY(t *testing.T) {
	// LOMETRIC: one unit is 0.1 mm, at four pixels a millimetre; y runs up.
	m := newEMF(40, 40).brush(1, 0, rgb(0, 255, 0), 0).sel(1).sel(nullPen)
	m.r(17, 2).r(12, 0, 40)
	m.rect(43, 0, 0, 25, 25) // x 0..10 px; y 0 to -10: the rectangle runs up from y = 40
	img := draw(t, m.bytes(), 40, 40, Options{})
	expect(t, img, 5, 35, green)
	expect(t, img, 5, 5, clear)
}

func TestClipRegionRecord(t *testing.T) {
	// EMR_EXTSELECTCLIPRGN: RGN_COPY with one rectangle in device units.
	rgn := words(32, 1, 1, 16, 5, 5, 25, 25, 5, 5, 25, 25)
	e := newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen)
	e.rec(75, append(words(int32(len(rgn)), 5), rgn...))
	e.rect(43, 0, 0, 40, 40)
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 15, 15, red)
	expect(t, img, 30, 30, clear)
}
