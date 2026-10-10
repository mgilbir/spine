package metafile

import (
	"context"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

var white = color.NRGBA{255, 255, 255, 255}

func fixtureFonts(t *testing.T) render.FontResolver {
	t.Helper()
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{UnitsPerEm: 2000, Ascent: 1600, Descent: -400, Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: 2000, HasShape: true, Ink: [4]int{0, 0, 2000, 1600}}, {Rune: ' ', Advance: 1000}}}))
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, r render.FontRequest) (*shape.Face, error) { return f, nil }
}

func TestText(t *testing.T) {
	e := newEMF(40, 40).font(1, -20, 0, 400, "Fixture").sel(1).r(18, 1).r(22, 24)
	e.textW(10, 30, "A", []int32{20}, 0x100)
	data := e.bytes()
	img := draw(t, data, 40, 40, Options{Fonts: fixtureFonts(t)})
	expect(t, img, 20, 20, black)
	expect(t, img, 20, 10, clear)
	expect(t, img, 5, 20, clear)
	expect(t, img, 35, 20, clear)
	// Strict mode refuses text without fonts; best effort leaves it out.
	if _, err := Render(context.Background(), data, 40, 40, Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict without fonts: %v", err)
	}
	var warned int
	img = draw(t, data, 40, 40, Options{Approximate: func(error) error { warned++; return nil }})
	if warned == 0 || at(img, 20, 20).A != 0 {
		t.Fatalf("best effort: warned %d, pixel %v", warned, at(img, 20, 20))
	}
}

func TestBitmapOrientation(t *testing.T) {
	info, bits := dib24(2, 2, [][][3]byte{
		{{0, 0, 255}, {0, 255, 0}},     // bottom row: red, green
		{{255, 0, 0}, {255, 255, 255}}, // top row: blue, white
	})
	e := newEMF(40, 40).stretchDIBits(0, 0, 40, 40, info, bits, 0xcc0020, 2, 2)
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 5, 5, blue)
	expect(t, img, 35, 5, white)
	expect(t, img, 5, 35, red)
	expect(t, img, 35, 35, green)
	// Another raster operation cannot be drawn exactly.
	bad := newEMF(40, 40).stretchDIBits(0, 0, 40, 40, info, bits, 0x8800c6, 2, 2)
	if _, err := Render(context.Background(), bad.bytes(), 40, 40, Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	var warned int
	img = draw(t, bad.bytes(), 40, 40, Options{Approximate: func(error) error { warned++; return nil }})
	if warned != 1 || at(img, 5, 5).A != 0 {
		t.Fatalf("best effort: warned %d, pixel %v", warned, at(img, 5, 5))
	}
}

func TestHatchBrush(t *testing.T) {
	// HS_FDIAGONAL draws the hatch color where a pixel's column and row agree
	// modulo 8, and, in the default OPAQUE background mode, the background
	// color elsewhere.
	e := newEMF(40, 40).brush(1, 2, rgb(255, 0, 0), 2).sel(1).sel(nullPen).rect(43, 8, 8, 33, 33)
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, red)
	expect(t, img, 21, 21, red)
	expect(t, img, 21, 20, white)
	expect(t, img, 4, 4, clear)
	// In TRANSPARENT mode only the hatch is drawn.
	tr := newEMF(40, 40).r(18, 1).brush(1, 2, rgb(255, 0, 0), 2).sel(1).sel(nullPen).rect(43, 8, 8, 33, 33)
	img = draw(t, tr.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, red)
	expect(t, img, 21, 20, clear)
}

func TestHatchStyleStrictAndBestEffort(t *testing.T) {
	// HS_DITHEREDCLR dithers the brush color, as the device would.
	e := newEMF(40, 40).brush(1, 2, rgb(255, 0, 0), 7).sel(1).sel(nullPen).rect(43, 10, 10, 30, 30).brush(2, 0, rgb(0, 0, 255), 0).sel(2).rect(43, 0, 0, 5, 5)
	if _, err := Render(context.Background(), e.bytes(), 40, 40, Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	var warned int
	img := draw(t, e.bytes(), 40, 40, Options{Approximate: func(error) error { warned++; return nil }})
	if warned != 1 {
		t.Fatalf("warned %d", warned)
	}
	expect(t, img, 2, 2, blue)
	expect(t, img, 20, 20, clear)
}

func TestEMFPlus(t *testing.T) {
	// A Dual file: EMF+ records filling blue, and GDI records filling red for
	// readers that cannot play them.
	dual := func(plus ...[]byte) []byte {
		recs := append([][]byte{plusHeader(true), plusPixels()}, plus...)
		recs = append(recs, plusEOF())
		e := newEMF(40, 40).raw(plusComment(recs...))
		return e.brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen).rect(43, 10, 10, 31, 31).bytes()
	}
	fill := plusFill(10, 10, 20, 20, 0xff0000ff)
	warnings := func(data []byte) (*image.NRGBA, []string) {
		var got []string
		img := draw(t, data, 40, 40, Options{Approximate: func(err error) error { got = append(got, err.Error()); return nil }})
		return img, got
	}

	// EMF+ records that can all be drawn are drawn, exactly.
	img := draw(t, dual(fill), 40, 40, Options{})
	expect(t, img, 20, 20, blue)
	if img, w := warnings(dual(fill)); len(w) != 0 || at(img, 20, 20) != blue {
		t.Fatalf("best effort: %v %v", w, at(img, 20, 20))
	}

	// EMF+ records that cannot be read: an object of an undefined type.
	unreadable := dual(fill, plusRecord(0x4008, 10<<8, words(int32(-0x243fefee))))
	if _, err := Render(context.Background(), unreadable, 40, 40, Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict unreadable: %v", err)
	}
	img, w := warnings(unreadable)
	expect(t, img, 20, 20, red)
	if len(w) != 1 || !strings.Contains(w[0], "drawn from its GDI records") {
		t.Fatalf("unreadable warnings %q", w)
	}

	// EMF+ records not all of which can be drawn, when the GDI records can:
	// a fill in Display units, whose size the device defines.
	display := dual(plusRecord(0x4030, 1, words(f32(1))), fill)
	if _, err := Render(context.Background(), display, 40, 40, Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict display: %v", err)
	}
	img, w = warnings(display)
	expect(t, img, 20, 20, red)
	if len(w) != 1 || !strings.Contains(w[0], "drawn from its GDI records") {
		t.Fatalf("display warnings %q", w)
	}

	// When neither can be drawn completely, the EMF+ records are drawn,
	// leaving out what they cannot.
	both := newEMF(40, 40).raw(plusComment(plusHeader(true), plusPixels(), plusFill(0, 0, 5, 5, 0xff00ff00), plusRecord(0x4030, 1, words(f32(1))), fill, plusEOF()))
	both.brush(1, 2, rgb(255, 0, 0), 7).sel(1).sel(nullPen).rect(43, 10, 10, 31, 31)
	img, w = warnings(both.bytes())
	expect(t, img, 2, 2, green)
	expect(t, img, 20, 20, clear)
	if len(w) != 1 || !strings.Contains(w[0], "page unit") {
		t.Fatalf("both warnings %q", w)
	}

	// An EMF+ Only file has no GDI records to fall back on: it is drawn.
	only := newEMF(40, 40).raw(plusComment(plusHeader(false), plusPixels(), fill, plusEOF()))
	img = draw(t, only.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, blue)
	expect(t, img, 5, 5, clear)
}

func TestWMF(t *testing.T) {
	// Placeable: 40 units at 40 per inch.
	w := newWMF().place(0, 0, 40, 40, 40)
	w.objects = 2
	w.rec(0x02fc, 0, 0x00ff, 0, 0) // red brush
	w.rec(0x02fa, 5, 0, 0, 0, 0)   // null pen
	w.rec(0x012d, 0)
	w.rec(0x012d, 1)
	w.rec(0x041b, 30, 30, 10, 10)
	data := w.bytes()
	img := draw(t, data, 40, 40, Options{})
	expect(t, img, 20, 20, red)
	expect(t, img, 5, 5, clear)
	info, err := Inspect(data)
	if err != nil || info.Width < 95.9 || info.Width > 96.1 || info.EMF {
		t.Fatalf("Inspect: %+v %v", info, err)
	}
	// Without a placeable header the window sets the picture's area.
	n := newWMF()
	n.objects = 2
	n.rec(0x020b, 0, 0).rec(0x020c, 100, 100)
	n.rec(0x02fc, 0, 0x00ff, 0, 0).rec(0x02fa, 5, 0, 0, 0, 0).rec(0x012d, 0).rec(0x012d, 1)
	n.rec(0x041b, 75, 75, 25, 25)
	img = draw(t, n.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, red)
	expect(t, img, 5, 5, clear)
	// No window and no header: no size.
	if _, err := Render(context.Background(), newWMF().bytes(), 40, 40, Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("no size: %v", err)
	}
}

func TestInspectEMF(t *testing.T) {
	info, err := Inspect(newEMF(40, 20).bytes())
	if err != nil || !info.EMF || info.Aspect != 2 || info.Width < 37.7 || info.Width > 37.9 {
		t.Fatalf("%+v %v", info, err)
	}
}

func TestLimitsAndCancellation(t *testing.T) {
	e := newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen)
	for i := 0; i < 10; i++ {
		e.rect(43, 0, 0, 40, 40)
	}
	data := e.bytes()
	tight := []render.Limits{
		{MaxOperations: 3},
		{MaxPixelVisits: 1000},
		{MaxEdgeChecks: 10},
		{MaxPathSegments: 5},
		{MaxImagePixels: 100},
	}
	for i, l := range tight {
		if _, err := Render(context.Background(), data, 40, 40, Options{Limits: l}); !errors.Is(err, render.ErrLimit) {
			t.Errorf("limit %d: %v", i, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, data, 40, 40, Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("cancel: %v", err)
	}
	// A hostile polygon count is refused by the parser before any drawing.
	if _, err := Render(context.Background(), data[:len(data)-4], 40, 40, Options{}); !errors.Is(err, render.ErrInvalid) {
		t.Errorf("truncated: %v", err)
	}
	if _, err := Render(context.Background(), []byte("not a metafile"), 40, 40, Options{}); !errors.Is(err, render.ErrInvalid) {
		t.Errorf("garbage: %v", err)
	}
}

func TestHostileGeometryIsBounded(t *testing.T) {
	// Huge coordinates and many points end in a result or a limit, quickly.
	pts := []int32{}
	for i := int32(0); i < 4000; i++ {
		pts = append(pts, (i%2)*2000000000-1000000000, i*500000)
	}
	e := newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).pen(2, 0, 9, 0).sel(2).poly(3, pts...)
	_, err := Render(context.Background(), e.bytes(), 40, 40, Options{Limits: render.Limits{MaxEdgeChecks: 1 << 20, MaxPathSegments: 20000}})
	if err != nil && !errors.Is(err, render.ErrLimit) {
		t.Fatal(err)
	}
}

// namedFonts resolves every request to a face whose PostScript name is name,
// holding 'A' as glyph 1, and reports each request.
func namedFonts(t *testing.T, name string) render.FontResolver {
	t.Helper()
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Name: name, UnitsPerEm: 2000, Ascent: 1600, Descent: -400, Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: 2000, HasShape: true, Ink: [4]int{0, 0, 2000, 1600}}}}))
	if err != nil {
		t.Fatal(err)
	}
	if g, ok := f.GlyphID('A'); !ok || g != 1 {
		t.Fatalf("glyph of A: %d %v", g, ok)
	}
	return func(ctx context.Context, r render.FontRequest) (*shape.Face, error) { return f, nil }
}

func TestGlyphIndexText(t *testing.T) {
	// ETO_GLYPH_INDEX: the text is glyph 1 of the font named "Fixture".
	e := newEMF(40, 40).font(1, -20, 0, 400, "Fixture").sel(1).r(18, 1).r(22, 24)
	e.textW(10, 30, "\x01", []int32{20}, 0x10)
	data := e.bytes()
	img := draw(t, data, 40, 40, Options{Fonts: namedFonts(t, "Fixture")})
	expect(t, img, 20, 20, black)
	// In another font glyph 1 is another glyph: the text is left out.
	other := namedFonts(t, "Other")
	if _, err := Render(context.Background(), data, 40, 40, Options{Fonts: other}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict in a substitute: %v", err)
	}
	var warnings []string
	img = draw(t, data, 40, 40, Options{Fonts: other, Approximate: func(err error) error { warnings = append(warnings, err.Error()); return nil }})
	expect(t, img, 20, 20, clear)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "substitute") {
		t.Fatalf("warnings %q", warnings)
	}
}

func TestMissingCharacters(t *testing.T) {
	// The face has no 'B': it is left out, without an advance of its own, and
	// reported; the 'A' after it is drawn at its own position.
	e := newEMF(40, 40).font(1, -20, 0, 400, "Fixture").sel(1).r(18, 1).r(22, 24)
	e.textW(0, 30, "BA", nil, 0)
	data := e.bytes()
	fonts := namedFonts(t, "Fixture")
	if _, err := Render(context.Background(), data, 40, 40, Options{Fonts: fonts}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	var warnings []string
	img := draw(t, data, 40, 40, Options{Fonts: fonts, Approximate: func(err error) error { warnings = append(warnings, err.Error()); return nil }})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "characters the font lacks left out") {
		t.Fatalf("warnings %q", warnings)
	}
	// 'A' is 20 wide from x = 0, its ink over the em.
	expect(t, img, 10, 20, black)
	expect(t, img, 30, 20, clear)
	// The face's .notdef is blank here; a character the face lacks is not
	// drawn as it either.
	face, _ := fonts(context.Background(), render.FontRequest{})
	gids, _, missing := elements(gowemf.TextRun{Text: []uint16{'B', 'A'}}, face)
	if !missing || gids[0] != -1 || gids[1] != 1 {
		t.Fatalf("elements: %v %v", gids, missing)
	}
}

func TestThinPen(t *testing.T) {
	// A pen 3 units wide in a picture drawn at a tenth of its size is 0.3
	// pixels wide; GDI draws it one pixel wide, through the pixel centers.
	e := newEMF(400, 400).pen(1, 0, 3, rgb(0, 0, 0)).sel(1).r(27, 50, 200).r(54, 350, 200)
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 20, 20, black)
	expect(t, img, 20, 19, clear)
	expect(t, img, 20, 21, clear)
}

func TestStrokePixelCenters(t *testing.T) {
	// Drawn at twice its size, a line along y = 10 is at y = 20 on the
	// raster; GDI draws it through the raster's pixel centers, at 20.5, and
	// its pen 2 units wide covers 18.5 to 22.5.
	e := newEMF(20, 20).pen(1, 0, 2, rgb(0, 0, 0)).sel(1).r(27, 2, 10).r(54, 18, 10)
	img := draw(t, e.bytes(), 40, 40, Options{})
	expect(t, img, 20, 19, black)
	expect(t, img, 20, 21, black)
	for _, y := range []int{18, 22} {
		if a := at(img, 20, y).A; a < 120 || a > 136 {
			t.Errorf("edge row %d alpha %d, want about 128", y, a)
		}
	}
}
