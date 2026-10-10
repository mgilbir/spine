package metafile

import (
	"context"
	"testing"

	"github.com/mgilbir/spine/render"
)

func FuzzRender(f *testing.F) {
	star := []int32{20, 2, 31, 36, 2, 14, 38, 14, 9, 36}
	f.Add(newEMF(40, 40).brush(1, 0, rgb(255, 0, 0), 0).sel(1).pen(2, 0, 3, 0).sel(2).r(19, 2).poly(3, star...).rect(44, 2, 2, 30, 30).r(59).r(27, 1, 1).r(54, 9, 9).r(60).rec(64, words(0, 0, 40, 40)).bytes())
	f.Add(newEMF(40, 40).font(1, -20, 0, 400, "Fixture").sel(1).textW(10, 30, "AA", []int32{10, 10}, 0).bytes())
	info, bits := dib24(2, 2, [][][3]byte{{{0, 0, 255}, {0, 255, 0}}, {{255, 0, 0}, {255, 255, 255}}})
	f.Add(newEMF(40, 40).stretchDIBits(0, 0, 40, 40, info, bits, 0xcc0020, 2, 2).bytes())
	w := newWMF().place(0, 0, 40, 40, 40)
	w.objects = 2
	f.Add(w.rec(0x02fc, 0, 0x00ff, 0, 0).rec(0x012d, 0).rec(0x041b, 30, 30, 10, 10).bytes())
	f.Add(newEMF(40, 40).raw(plusComment(plusHeader(true), plusPixels(), plusFill(10, 10, 20, 20, 0xff0000ff), plusEOF())).brush(1, 0, rgb(255, 0, 0), 0).sel(1).rect(43, 10, 10, 31, 31).bytes())
	limits := render.Limits{MaxDimension: 64, MaxPixels: 4096, MaxOperations: 256, MaxPathSegments: 4096, MaxPixelVisits: 1 << 20, MaxEdgeChecks: 1 << 20, MaxImagePixels: 4096, MaxImageBytes: 64 << 10, MaxGlyphs: 256, MaxFonts: 2}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 32<<10 {
			t.Skip()
		}
		// Strict and best effort must both end, without a panic, in a picture
		// or an error.
		_, _ = Render(context.Background(), data, 32, 32, Options{Limits: limits})
		_, _ = Render(context.Background(), data, 17, 9, Options{Limits: limits, Approximate: func(error) error { return nil }})
		_, _ = Inspect(data)
	})
}
