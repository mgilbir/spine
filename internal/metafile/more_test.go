package metafile

import (
	"context"
	"errors"
	"image/color"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
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

func TestHatchBrushStrictAndBestEffort(t *testing.T) {
	e := newEMF(40, 40).brush(1, 2, rgb(255, 0, 0), 0).sel(1).sel(nullPen).rect(43, 10, 10, 30, 30).brush(2, 0, rgb(0, 0, 255), 0).sel(2).rect(43, 0, 0, 5, 5)
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
	gdi := func(e *emfBuilder) *emfBuilder {
		return e.brush(1, 0, rgb(255, 0, 0), 0).sel(1).sel(nullPen).rect(43, 10, 10, 30, 30)
	}
	dual := gdi(newEMF(40, 40).raw(plusComment(plusHeader(true)))).raw(plusComment(plusEOF()))
	if _, err := Render(context.Background(), dual.bytes(), 40, 40, Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict dual: %v", err)
	}
	var warned int
	img := draw(t, dual.bytes(), 40, 40, Options{Approximate: func(error) error { warned++; return nil }})
	expect(t, img, 20, 20, red)
	if warned == 0 {
		t.Fatal("no warning")
	}
	only := newEMF(40, 40).raw(plusComment(plusHeader(false))).raw(plusComment(plusEOF()))
	if _, err := Render(context.Background(), only.bytes(), 40, 40, Options{Approximate: func(error) error { return nil }}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("EMF+ only: %v", err)
	}
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
