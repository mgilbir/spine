package render

import (
	"context"
	"errors"
	"image"
	"io"
	"os"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

// fuzzColorFace draws a color glyph of a face, in strict and in best-effort
// preparation, within tight limits, and paints the result both ways. Anything
// may be refused, but only as an invalid, unsupported or over-limit request.
func fuzzColorFace(t *testing.T, face *shape.Face, gid int, size uint8) {
	limits := Limits{
		MaxOperations: 2000, MaxPathSegments: 20000, MaxClipDepth: 16, MaxGlyphs: 16, MaxFonts: 2, MaxFontBytes: 1 << 20,
		MaxImagePixels: 1 << 16, MaxImageBytes: 1 << 20, MaxImages: 4, MaxPixelVisits: 1 << 22, MaxEdgeChecks: 1 << 22, MaxOutputBytes: 1 << 22,
	}
	if face.NumGlyphs() < 2 {
		return
	}
	gid = 1 + gid%(face.NumGlyphs()-1)
	op := colorOps(face, style.RGBA{R: 10, G: 200, B: 30, A: 0.8}, gid, gid)
	run := op[0].(layout.DrawGlyphs)
	run.Size = unit(float64(size)/2 + 1)
	op[0] = run
	for _, approximate := range []func(error){nil, func(error) {}} {
		p, err := PrepareBestEffort(context.Background(), dml.Pixels(64), dml.Pixels(64), op, limits, approximate)
		if err != nil {
			if !errors.Is(err, ErrInvalid) && !errors.Is(err, ErrLimit) && !errors.Is(err, ErrUnsupported) {
				t.Fatalf("unclassified error: %v", err)
			}
			continue
		}
		if _, err := p.Raster(context.Background(), 96); err != nil && !errors.Is(err, ErrLimit) {
			t.Fatal(err)
		}
		if err := p.WriteSVG(context.Background(), io.Discard, 96); err != nil && !errors.Is(err, ErrLimit) {
			t.Fatal(err)
		}
	}
}

// FuzzColorGlyphTables fuzzes the color tables of a font whose outlines are
// sound.
func FuzzColorGlyphTables(f *testing.F) {
	seed := colorFace{
		layers: []*paintNode{paintGlyph(gidFull, paintSolid(0, 1)), paintGlyph(gidLower, paintSolid(1, 0.5))},
		bases: []*paintNode{
			paintLayers(0, 2),
			paintGlyph(gidFull, paintLinear(colorLine(1, stops2...), 0, 0, 250, 0, 0, 100)),
			paintGlyph(gidFull, paintRadial(colorLine(2, stops2...), 500, 500, 100, 500, 500, 400)),
			paintTranslate(paintRotate(paintGlyph(gidLower, paintSweep(colorLine(0, stops2...), 250, 250, 0, 1)), 0.25), 500, 0),
			paintComposite(paintGlyph(gidLower, paintSolid(0, 1)), 23, paintGlyph(gidFull, paintSolid(1, 1))),
		},
		v0: [][][2]int{{{gidLower, 0}, {gidFull, 0xFFFF}}},
	}
	tables := seed.tables()
	f.Add(tables["COLR"], tables["CPAL"], []byte(nil), []byte(nil), uint8(3), uint8(40))
	for _, sbix := range []bool{true, false} {
		b := colorFace{sbix: sbix, bitmaps: []*image.NRGBA{quadrants(8)}}.tables()
		if sbix {
			f.Add([]byte(nil), []byte(nil), b["sbix"], []byte(nil), uint8(0), uint8(100))
		} else {
			f.Add([]byte(nil), []byte(nil), b["CBLC"], b["CBDT"], uint8(0), uint8(100))
		}
	}
	f.Fuzz(func(t *testing.T, colr, cpal, bitmapA, bitmapB []byte, gid, size uint8) {
		if len(colr)+len(cpal)+len(bitmapA)+len(bitmapB) > 1<<16 {
			return
		}
		extra := map[string][]byte{}
		for tag, data := range map[string][]byte{"COLR": colr, "CPAL": cpal} {
			if len(data) > 0 {
				extra[tag] = data
			}
		}
		if len(bitmapA) > 0 && len(bitmapB) > 0 {
			extra["CBLC"], extra["CBDT"] = bitmapA, bitmapB
		} else if len(bitmapA) > 0 {
			extra["sbix"] = bitmapA
		}
		face, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: colorFace{bases: make([]*paintNode, 8)}.glyphs(), Extra: extra}))
		if err != nil {
			return
		}
		fuzzColorFace(t, face, int(gid), size)
	})
}

// FuzzColorGlyphFont fuzzes whole fonts, starting from the test fonts of
// forme's: COLRv0 and COLRv1, SVG, CBDT and sbix.
func FuzzColorGlyphFont(f *testing.F) {
	for _, name := range []string{"ColourPaint.ttf", "ColourInk.ttf", "SbixInk.ttf", "BitmapInk.ttf", "SVGPaint.ttf"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data, uint8(18), uint8(50))
	}
	f.Fuzz(func(t *testing.T, data []byte, gid, size uint8) {
		if len(data) > 1<<16 {
			return
		}
		face, err := shape.Load(data)
		if err != nil {
			return
		}
		fuzzColorFace(t, face, int(gid), size)
	})
}
