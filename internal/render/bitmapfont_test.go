package render

import (
	"context"
	"encoding/binary"
	"errors"
	"image/color"
	"sort"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

// Builders of fonts of monochrome and greyscale bitmap strikes (EBDT and EBLC,
// or Apple's bdat and bloc) for the tests, written here so that what a glyph
// paints is stated beside the test that checks it.

// strikeGlyph is a bitmap of a strike, one byte a pixel, rows from the top, in
// EBDT's image format 1 (small metrics, rows each starting on a byte, at the
// strike's bit depth).
type strikeGlyph struct {
	w, h int
	// bearingX and bearingY are the bitmap's top left corner from the
	// origin, in pixels, up positive.
	bearingX, bearingY int
	// samples are w*h values, each of the strike's bit depth.
	samples []byte
}

// strike is one strike: the size it is drawn for, in pixels per em, its bit
// depth, and the bitmaps of the glyphs from gid 1 in order.
type strike struct {
	ppem, depth int
	glyphs      []strikeGlyph
}

// block is a w by h glyph with the left half of its columns set to on and the
// rest to off.
func block(w, h, depth, on int) strikeGlyph {
	g := strikeGlyph{w: w, h: h, bearingY: h, samples: make([]byte, w*h)}
	for y := range h {
		for x := range w / 2 {
			g.samples[y*w+x] = byte(on)
		}
	}
	return g
}

func (g strikeGlyph) data(depth int) []byte {
	out := []byte{byte(g.h), byte(g.w), byte(int8(g.bearingX)), byte(int8(g.bearingY)), byte(g.w)}
	for y := range g.h {
		var bits, n int
		for x := range g.w {
			bits, n = bits<<depth|int(g.samples[y*g.w+x]), n+depth
			if n == 8 {
				out, bits, n = append(out, byte(bits)), 0, 0
			}
		}
		if n > 0 {
			out = append(out, byte(bits<<(8-n)))
		}
	}
	return out
}

// strikeTables are EBLC and EBDT holding strikes: for each, an index
// subtable of format 1 over the glyphs 1 on.
func strikeTables(strikes []strike) (eblc, ebdt []byte) {
	ebdt = join(be16(2), be16(0))
	header := join(be16(2), be16(0), be32(len(strikes)))
	var sizes, indexes []byte
	indexAt := len(header) + 48*len(strikes)
	for _, s := range strikes {
		array := join(be16(1), be16(len(s.glyphs)), be32(8))
		sub := join(be16(1), be16(1), be32(len(ebdt)))
		start := len(ebdt)
		for _, g := range s.glyphs {
			sub = join(sub, be32(len(ebdt)-start))
			ebdt = join(ebdt, g.data(s.depth))
		}
		sub = join(sub, be32(len(ebdt)-start))
		line := join([]byte{byte(s.ppem), 0, 0, 0}, make([]byte, 8))
		sizes = join(sizes, be32(indexAt+len(indexes)), be32(len(array)+len(sub)), be32(1), be32(0), line, line,
			be16(1), be16(len(s.glyphs)), []byte{byte(s.ppem), byte(s.ppem), byte(s.depth), 1})
		indexes = join(indexes, array, sub)
	}
	return join(header, sizes, indexes), ebdt
}

// withoutOutlines drops the outlines of a font, so that it has glyphs of
// bitmaps only.
func withoutOutlines(font []byte) []byte {
	n := int(binary.BigEndian.Uint16(font[4:6]))
	tables := map[string][]byte{}
	for i := range n {
		rec := font[12+16*i:]
		tag := string(rec[:4])
		if tag == "glyf" || tag == "loca" {
			continue
		}
		off, length := binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
		tables[tag] = font[off : off+length]
	}
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	out := make([]byte, 12+16*len(tags))
	binary.BigEndian.PutUint32(out, binary.BigEndian.Uint32(font))
	binary.BigEndian.PutUint16(out[4:], uint16(len(tags)))
	for i, tag := range tags {
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		copy(out[12+16*i:], tag)
		binary.BigEndian.PutUint32(out[12+16*i+8:], uint32(len(out)))
		binary.BigEndian.PutUint32(out[12+16*i+12:], uint32(len(tables[tags[i]])))
		out = append(out, tables[tag]...)
	}
	return out
}

// strikeFont loads a font of the strikes' glyphs. Apple's table names are used
// where apple says so, and the font has outlines where outlines says so.
func strikeFont(t testing.TB, apple, outlines bool, strikes ...strike) *shape.Face {
	t.Helper()
	eblc, ebdt := strikeTables(strikes)
	extra := map[string][]byte{"EBLC": eblc, "EBDT": ebdt}
	if apple {
		extra = map[string][]byte{"bloc": eblc, "bdat": ebdt}
	}
	glyphs := make([]fonttest.Glyph, len(strikes[0].glyphs))
	for i := range glyphs {
		glyphs[i] = fonttest.Glyph{Rune: rune('A' + i), Advance: 1000, HasShape: true, Ink: [4]int{0, 0, 1000, 1000}}
	}
	font := fonttest.SFNT(fonttest.SFNTOptions{Glyphs: glyphs, Extra: extra})
	if !outlines {
		font = withoutOutlines(font)
	}
	f, err := shape.Load(font)
	if err != nil {
		t.Fatal(err)
	}
	if f.BitmapOnly() == outlines {
		t.Fatalf("bitmap only %v, outlines %v", f.BitmapOnly(), outlines)
	}
	return f
}

// strikeRun draws the glyphs of a face in strict (nil approximate) or best
// effort preparation, size pixels to the em, from (4, 14) of a 40 by 20 page.
func strikeRun(face *shape.Face, size float64, text style.RGBA, approximate func(error), gids ...int) (*Page, error) {
	glyphs := make([]shape.Glyph, len(gids))
	for i, gid := range gids {
		glyphs[i] = shape.Glyph{GID: gid, XAdvance: 1000}
	}
	op := layout.DrawGlyphs{At: layout.Point{X: unit(4), Y: unit(14)}, Face: face, Size: unit(size), Color: text, Glyphs: glyphs, Text: "x"}
	return PrepareBestEffort(context.Background(), dml.Pixels(40), dml.Pixels(20), []layout.Op{op}, Limits{}, approximate)
}

func strikeRaster(t *testing.T, p *Page) interface{ RGBAAt(x, y int) color.RGBA } {
	t.Helper()
	img, err := p.Raster(context.Background(), 96)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestBitmapOnlyFontExactStrike(t *testing.T) {
	text := style.RGBA{R: 255, A: 1}
	for _, apple := range []bool{false, true} {
		for _, depth := range []int{1, 2, 4, 8} {
			on := 1<<depth - 1
			face := strikeFont(t, apple, false, strike{ppem: 10, depth: depth, glyphs: []strikeGlyph{block(8, 8, depth, on)}})
			// Glyph 1's strike is the size asked for, so strict preparation
			// draws it, and its pixels are the page's: 8 by 8 from (4, 6).
			p, err := strikeRun(face, 10, text, nil, 1)
			if err != nil {
				t.Fatalf("apple %v depth %d: %v", apple, depth, err)
			}
			img := strikeRaster(t, p)
			for _, tc := range []struct {
				x, y int
				want color.RGBA
			}{{4, 6, red}, {7, 13, red}, {8, 6, nothing}, {11, 13, nothing}, {3, 10, nothing}, {4, 5, nothing}, {4, 14, nothing}} {
				if got := img.RGBAAt(tc.x, tc.y); got != tc.want {
					t.Fatalf("apple %v depth %d at %d, %d: %v, want %v", apple, depth, tc.x, tc.y, got, tc.want)
				}
			}
			if len(p.draws) != 1 || p.draws[0].text != "x" || p.draws[0].fontID == "" {
				t.Fatalf("metadata: %+v", p.draws)
			}
		}
	}
}

func TestBitmapOnlyFontColourAndAlpha(t *testing.T) {
	// A greyscale glyph's coverage scales the text's alpha, and the color is
	// the text's, whatever the strike says.
	glyph := block(8, 8, 8, 128)
	face := strikeFont(t, false, false, strike{ppem: 10, depth: 8, glyphs: []strikeGlyph{glyph}})
	for _, tc := range []struct {
		text style.RGBA
		want color.RGBA
	}{
		{style.RGBA{B: 255, A: 1}, color.RGBA{0, 0, 128, 128}},
		{style.RGBA{G: 255, A: 0.5}, color.RGBA{0, 64, 0, 64}},
	} {
		p, err := strikeRun(face, 10, tc.text, nil, 1)
		if err != nil {
			t.Fatal(err)
		}
		got := strikeRaster(t, p).RGBAAt(5, 10)
		near(t, got, tc.want, 2, "greyscale glyph")
	}
}

func TestBitmapOnlyFontScaledStrike(t *testing.T) {
	face := strikeFont(t, false, false, strike{ppem: 10, depth: 1, glyphs: []strikeGlyph{block(8, 8, 1, 1)}})
	for _, tc := range []struct {
		name string
		size float64
	}{
		{"larger than the strike", 20},
		{"smaller than the strike", 5},
		{"between sizes", 9.5},
	} {
		if _, err := strikeRun(face, tc.size, black, nil, 1); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("%s, strict: %v", tc.name, err)
		}
		var reports []string
		p, err := strikeRun(face, tc.size, red2, collect(&reports), 1)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(reports) != 1 || !strings.Contains(reports[0], "bitmap font glyph scaled from its strike") {
			t.Fatalf("%s: reports %q", tc.name, reports)
		}
		// It is drawn, scaled, in the text's color.
		img := strikeRaster(t, p)
		found := false
		for y := range 20 {
			for x := range 40 {
				if c := img.RGBAAt(x, y); c == red {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("%s: nothing drawn", tc.name)
		}
	}
	// Reported once for a run of glyphs.
	var reports []string
	if _, err := strikeRun(face, 20, black, collect(&reports), 1, 1, 1); err != nil || len(reports) != 1 {
		t.Fatalf("%v, reports %q", err, reports)
	}
}

var red2 = style.RGBA{R: 255, A: 1}

func TestBitmapOnlyFontStrikeChoice(t *testing.T) {
	// The strike drawn is the smallest at least the size, so a size of a strike
	// is exact whichever others the font has.
	face := strikeFont(t, false, false,
		strike{ppem: 8, depth: 1, glyphs: []strikeGlyph{block(6, 6, 1, 1)}},
		strike{ppem: 10, depth: 1, glyphs: []strikeGlyph{block(8, 8, 1, 1)}})
	for _, size := range []float64{8, 10} {
		if _, err := strikeRun(face, size, black, nil, 1); err != nil {
			t.Fatalf("size %v: %v", size, err)
		}
	}
	if _, err := strikeRun(face, 9, black, nil, 1); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("size 9: %v", err)
	}
}

func TestBitmapOnlyFontMissingGlyphs(t *testing.T) {
	// A glyph past the font's, or the .notdef, is refused as for any font; a
	// glyph the strike has no bitmap for draws nothing.
	face := strikeFont(t, false, false, strike{ppem: 10, depth: 1, glyphs: []strikeGlyph{block(8, 8, 1, 1), {}}})
	for _, gid := range []int{0, 3} {
		if _, err := strikeRun(face, 10, black, nil, gid); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("glyph %d: %v", gid, err)
		}
	}
	p, err := strikeRun(face, 10, black, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	img := strikeRaster(t, p)
	for y := range 20 {
		for x := range 40 {
			if c := img.RGBAAt(x, y); c != nothing {
				t.Fatalf("pixel %d, %d: %v", x, y, c)
			}
		}
	}
}

func TestBitmapOnlyFontTurnedText(t *testing.T) {
	// A bitmap font has no outline to turn, so text drawn as outlines, as
	// turned text is, is refused.
	face := strikeFont(t, false, false, strike{ppem: 10, depth: 1, glyphs: []strikeGlyph{block(8, 8, 1, 1)}})
	v := layout.DrawGlyphs{Face: face, Size: unit(10), Color: black, Glyphs: []shape.Glyph{{GID: 1, XAdvance: 1000}}}
	segments := 0
	if _, _, err := GlyphPaths(context.Background(), v, func(x, y float64) (float64, float64) { return x, y }, 1000, &segments); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	if !HasColorGlyphs(v) {
		t.Fatal("bitmap glyph is not reported as one painted from an image")
	}
}

func TestBitmapFontWithOutlinesDrawsOutlines(t *testing.T) {
	// A font with outlines and strikes, as the system's Courier New, is drawn
	// from its outlines: the glyph is the 1000 unit square, not the strike's
	// left half.
	face := strikeFont(t, false, true, strike{ppem: 10, depth: 1, glyphs: []strikeGlyph{block(8, 8, 1, 1)}})
	p, err := strikeRun(face, 10, black, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	img := strikeRaster(t, p)
	if got := img.RGBAAt(11, 6); got != (color.RGBA{A: 255}) {
		t.Fatalf("right half of the square: %v", got)
	}
	// And at a size no strike has, with no complaint.
	if _, err := strikeRun(face, 20, black, nil, 1); err != nil {
		t.Fatal(err)
	}
}

func TestBitmapOnlyFontLimits(t *testing.T) {
	face := strikeFont(t, false, false, strike{ppem: 10, depth: 8, glyphs: []strikeGlyph{block(8, 8, 8, 255)}})
	// The image's pixels are counted, as a color glyph's are.
	if _, err := Prepare(context.Background(), dml.Pixels(40), dml.Pixels(20), []layout.Op{layout.DrawGlyphs{At: layout.Point{X: unit(4), Y: unit(14)}, Face: face, Size: unit(10), Color: black, Glyphs: []shape.Glyph{{GID: 1, XAdvance: 1000}}}}, Limits{MaxImagePixels: 63}); !errors.Is(err, ErrLimit) {
		t.Fatalf("pixels: %v", err)
	}
	if _, err := Prepare(context.Background(), dml.Pixels(40), dml.Pixels(20), []layout.Op{layout.DrawGlyphs{At: layout.Point{X: unit(4), Y: unit(14)}, Face: face, Size: unit(10), Color: black, Glyphs: []shape.Glyph{{GID: 1, XAdvance: 1000}}}}, Limits{MaxOperations: 1}); !errors.Is(err, ErrLimit) {
		t.Fatalf("operations: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	op := layout.DrawGlyphs{At: layout.Point{X: unit(4), Y: unit(14)}, Face: face, Size: unit(10), Color: black, Glyphs: []shape.Glyph{{GID: 1, XAdvance: 1000}}}
	if _, err := Prepare(ctx, dml.Pixels(40), dml.Pixels(20), []layout.Op{op}, Limits{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
	// Each color a glyph is drawn in is another image, all within the budget.
	var ops []layout.Op
	for i := range 40 {
		op := op
		op.Color = style.RGBA{R: float64(i), A: 1}
		ops = append(ops, op)
	}
	if _, err := Prepare(context.Background(), dml.Pixels(40), dml.Pixels(20), ops, Limits{MaxImagePixels: 64 * 10}); !errors.Is(err, ErrLimit) {
		t.Fatalf("many colors: %v", err)
	}
}

func FuzzBitmapOnlyFont(f *testing.F) {
	eblc, ebdt := strikeTables([]strike{{ppem: 10, depth: 1, glyphs: []strikeGlyph{block(8, 8, 1, 1)}}})
	f.Add(eblc, ebdt, uint8(10))
	f.Add(eblc, ebdt, uint8(25))
	f.Fuzz(func(t *testing.T, eblc, ebdt []byte, size uint8) {
		if len(eblc)+len(ebdt) > 1<<16 || len(eblc) == 0 || len(ebdt) == 0 {
			return
		}
		font := withoutOutlines(fonttest.SFNT(fonttest.SFNTOptions{
			Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: 1000, HasShape: true}},
			Extra:  map[string][]byte{"EBLC": eblc, "EBDT": ebdt},
		}))
		face, err := shape.Load(font)
		if err != nil {
			return
		}
		fuzzColorFace(t, face, 1, size)
	})
}
