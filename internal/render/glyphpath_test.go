package render

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

func TestGlyphPathsTurnTheOutline(t *testing.T) {
	f := testFace(t, nil)
	run := layout.DrawGlyphs{At: layout.Point{X: unit(2), Y: unit(12)}, Face: f, Size: unit(10), Color: style.RGBA{A: 1}, Glyphs: []shape.Glyph{{GID: 1, XAdvance: 1000}, {GID: 1, XAdvance: 1000}}}
	// A quarter turn clockwise about (12, 12) takes the row of two 10px
	// squares above the baseline to a column right of x 12.
	turn := func(x, y float64) (float64, float64) { return 12 - (y - 12), 12 + (x - 12) }
	segments := 0
	paths, exact, err := GlyphPaths(context.Background(), run, turn, 1000, &segments)
	if err != nil || !exact || len(paths) != 2 || segments == 0 {
		t.Fatalf("paths %d exact %v segments %d err %v", len(paths), exact, segments, err)
	}
	var ops []layout.Op
	for _, p := range paths {
		ops = append(ops, layout.FillPath{Path: p, Color: run.Color})
	}
	p, err := Prepare(context.Background(), dml.Pixels(30), dml.Pixels(30), ops, Limits{})
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
	for _, tc := range []struct {
		x, y int
		ink  bool
	}{{17, 3, true}, {17, 20, true}, {5, 5, false}, {25, 5, false}, {17, 25, false}} {
		_, _, _, a := img.At(tc.x, tc.y).RGBA()
		if (a == 65535) != tc.ink {
			t.Fatalf("%d,%d alpha %d", tc.x, tc.y, a)
		}
	}
}

func TestGlyphPathsLimits(t *testing.T) {
	f := testFace(t, nil)
	run := layout.DrawGlyphs{Face: f, Size: unit(10), Color: style.RGBA{A: 1}, Glyphs: []shape.Glyph{{GID: 1, XAdvance: 1000}}}
	same := func(x, y float64) (float64, float64) { return x, y }
	segments := 0
	if _, _, err := GlyphPaths(context.Background(), run, same, 2, &segments); !errors.Is(err, ErrLimit) {
		t.Fatalf("segment limit: %v", err)
	}
	run.Clip = layout.Clip{Active: true}
	if _, _, err := GlyphPaths(context.Background(), run, same, 1000, &segments); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("clipped run: %v", err)
	}
	run.Clip = layout.Clip{}
	run.Glyphs[0].GID = 99
	if _, _, err := GlyphPaths(context.Background(), run, same, 1000, &segments); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("missing glyph: %v", err)
	}
	run.Glyphs[0].GID = 1
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := GlyphPaths(ctx, run, same, 1000, &segments); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}

func TestEvenOddMatchesNonzero(t *testing.T) {
	square := func(x0, y0, x1, y1 float64, clockwise bool) []point {
		if clockwise {
			return []point{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
		}
		return []point{{x0, y0}, {x0, y1}, {x1, y1}, {x1, y0}}
	}
	for _, tc := range []struct {
		name     string
		contours [][]point
		match    bool
	}{
		{"one", [][]point{square(0, 0, 10, 10, true)}, true},
		{"apart", [][]point{square(0, 0, 10, 10, true), square(20, 0, 30, 10, false)}, true},
		{"hole", [][]point{square(0, 0, 10, 10, true), square(2, 2, 8, 8, false)}, true},
		{"island in a hole", [][]point{square(0, 0, 10, 10, true), square(2, 2, 8, 8, false), square(4, 4, 6, 6, true)}, true},
		{"nested alike", [][]point{square(0, 0, 10, 10, true), square(2, 2, 8, 8, true)}, false},
		{"crossing", [][]point{square(0, 0, 10, 10, true), square(5, 5, 15, 15, true)}, false},
	} {
		if got := evenOddMatchesNonzero(tc.contours); got != tc.match {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
