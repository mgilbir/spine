package render

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image/color"
	"image/png"
	"math"
	"sync"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

func testFace(t testing.TB, extra map[string][]byte) *shape.Face {
	t.Helper()
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: 1000, HasShape: true, Ink: [4]int{0, 0, 1000, 1000}}}, Extra: extra}))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPositionedGlyphsAndOwnership(t *testing.T) {
	f := testFace(t, nil)
	glyphs := []shape.Glyph{{GID: 1, XAdvance: 1000}, {GID: 1, XOffset: 500, XAdvance: 1000}}
	op := layout.DrawGlyphs{At: layout.Point{X: unit(1), Y: unit(10)}, Face: f, Size: unit(10), Color: style.RGBA{R: 255, A: 1}, Glyphs: glyphs, Text: "AA", Clip: layout.Clip{Active: true, Rect: layout.Rect{X: unit(1), W: unit(20), H: unit(12)}}}
	p, err := Prepare(context.Background(), dml.Pixels(30), dml.Pixels(12), []layout.Op{op}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.draws) != 2 || p.draws[0].text != "AA" || p.draws[1].text != "" || p.draws[0].fontID == "" {
		t.Fatalf("metadata: %+v", p.draws)
	}
	if len(f.Used()) != 0 {
		t.Fatal("source font usage changed")
	}
	glyphs[0].XAdvance = 0
	f.Use(1)
	var b bytes.Buffer
	if err := p.WritePNG(context.Background(), &b, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&b)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		x, y int
		ink  bool
	}{{2, 2, true}, {17, 2, true}, {12, 2, false}, {22, 2, false}, {2, 11, false}} {
		_, _, _, a := img.At(tc.x, tc.y).RGBA()
		if (a == 65535) != tc.ink {
			t.Fatalf("%d,%d alpha %d", tc.x, tc.y, a)
		}
	}
	b.Reset()
	if err := p.WriteSVG(context.Background(), &b, 96); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b.Bytes(), []byte(`fill-rule="nonzero"`)) || bytes.Contains(b.Bytes(), []byte("<text")) {
		t.Fatalf("SVG glyphs: %s", b.String())
	}
}

func TestGlyphOverdrawPreservesAlpha(t *testing.T) {
	f := testFace(t, nil)
	op := layout.DrawGlyphs{Face: f, At: layout.Point{Y: unit(1)}, Size: unit(1), Color: style.RGBA{R: 255, A: 0.5}, Glyphs: []shape.Glyph{{GID: 1}, {GID: 1}}}
	p, err := Prepare(context.Background(), dml.Pixels(1), dml.Pixels(1), []layout.Op{op}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := p.WritePNG(context.Background(), &b, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&b)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); got != (color.NRGBA{R: 255, A: 192}) {
		t.Fatal(got)
	}
}

func TestGlyphLimitsAndUnsupportedFonts(t *testing.T) {
	f := testFace(t, nil)
	op := layout.DrawGlyphs{Face: f, Size: unit(10), Glyphs: []shape.Glyph{{GID: 1}, {GID: 1}}, Text: "AA"}
	for _, l := range []Limits{{MaxGlyphs: 1}, {MaxTextBytes: 1}, {MaxFontBytes: 1}, {MaxPathSegments: 1}} {
		if _, err := Prepare(context.Background(), 1, 1, []layout.Op{op}, l); !errors.Is(err, ErrLimit) {
			t.Fatalf("%+v: %v", l, err)
		}
	}
	other := op
	other.Face = testFace(t, nil)
	if _, err := Prepare(context.Background(), 1, 1, []layout.Op{op, other}, Limits{MaxFonts: 1}); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	for _, g := range []shape.Glyph{{GID: 0}, {GID: 99}, {GID: 1, XAdvance: math.NaN()}, {GID: 1, YAdvance: 1}, {GID: 1, XOffset: math.MaxFloat64}} {
		bad := op
		bad.Glyphs = []shape.Glyph{g}
		if _, err := Prepare(context.Background(), 1, 1, []layout.Op{bad}, Limits{}); err == nil {
			t.Fatalf("accepted %+v", g)
		}
	}
	// Apple's bitmap strikes beside outlines, as in the system's Courier New,
	// are passed over: the outlines are drawn.
	bitmapFont := op
	bitmapFont.Face = testFace(t, map[string][]byte{"bdat": make([]byte, 8), "bloc": make([]byte, 8)})
	if _, err := Prepare(context.Background(), 1, 1, []layout.Op{bitmapFont}, Limits{}); err != nil {
		t.Fatal(err)
	}
	missing := op
	missing.Face = nil
	if _, err := Prepare(context.Background(), 1, 1, []layout.Op{missing}, Limits{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestCurveFlatteningAndNonzero(t *testing.T) {
	curves := []curve{{op: 'M', pts: [3]point{{0, 0}}}, {op: 'Q', pts: [3]point{{4, 0}, {4, 4}}}}
	remaining := 100
	edges, err := flattenCurves(context.Background(), curves, 1, &remaining)
	if err != nil {
		t.Fatal(err)
	}
	// Independent integral of this quadratic and its closing diagonal: 16/3.
	area := 0.0
	for _, e := range edges {
		area += (e.a.x*e.b.y - e.b.x*e.a.y) / 2
	}
	if math.Abs(math.Abs(area)-16.0/3) > 0.1 {
		t.Fatalf("quadratic area %v", area)
	}
	remaining = 1
	if _, err := flattenCurves(context.Background(), curves, 1, &remaining); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	data, err := curveString(context.Background(), curves, 1, 1000)
	if err != nil || !bytes.Contains([]byte(data), []byte("Q4 0 4 4")) {
		t.Fatalf("%s: %v", data, err)
	}
	// Two coincident contours fill under nonzero, cancel under even-odd.
	doubled := append(append([]edge(nil), edges...), edges...)
	nonzero, _ := scan(doubled, 1, nil, true)
	evenodd, _ := scan(doubled, 1, nil, false)
	if len(nonzero) == 0 || len(evenodd) != 0 {
		t.Fatalf("nonzero %v, evenodd %v", nonzero, evenodd)
	}
}

func TestConcurrentGlyphPreparation(t *testing.T) {
	f := testFace(t, nil)
	op := layout.DrawGlyphs{Face: f, Size: unit(10), Glyphs: []shape.Glyph{{GID: 1}}}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			p, err := Prepare(context.Background(), dml.Pixels(20), dml.Pixels(20), []layout.Op{op}, Limits{})
			if err != nil {
				t.Error(err)
				return
			}
			if err := p.WritePNG(context.Background(), &bytes.Buffer{}, 96); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func TestFontDirectory(t *testing.T) {
	dir := func(tags ...string) []byte {
		b := make([]byte, 12+16*len(tags))
		binary.BigEndian.PutUint16(b[4:6], uint16(len(tags)))
		for i, tag := range tags {
			copy(b[12+16*i:], tag)
		}
		return b
	}
	// Bitmap strikes are no reason to refuse a font: it draws from its
	// outlines or, with none, its strikes.
	for _, tables := range [][]string{{"bdat", "bloc"}, {"EBDT", "EBLC"}, {"bdat", "bloc", "glyf"}, {"EBDT", "EBLC", "CFF "}} {
		if err := checkFontDirectory(dir(append(tables, "cmap", "head")...)); err != nil {
			t.Errorf("%v: %v", tables, err)
		}
	}
	if err := checkFontDirectory(append(dir("glyf")[:4], 0, 9)); !errors.Is(err, ErrInvalid) {
		t.Errorf("short directory: %v", err)
	}
}
