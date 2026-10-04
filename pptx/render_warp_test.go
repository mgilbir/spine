package pptx

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

func TestRenderPolyline(t *testing.T) {
	p := newRenderPolyline([][2]float64{{0, 0}, {10, 0}, {10, 30}})
	for _, tc := range []struct{ u, x, y, dx, dy float64 }{{0, 0, 0, 1, 0}, {0.25, 10, 0, 1, 0}, {0.5, 10, 10, 0, 1}, {1, 10, 30, 0, 1}, {2, 10, 30, 0, 1}} {
		x, y, dx, dy := p.point(tc.u)
		if math.Abs(x-tc.x) > 1e-9 || math.Abs(y-tc.y) > 1e-9 || dx != tc.dx || dy != tc.dy {
			t.Errorf("at %v: %v,%v going %v,%v", tc.u, x, y, dx, dy)
		}
	}
}

// renderWarpBounds is the box a drawing's path points cover.
func renderWarpBounds(ops []layout.Op) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, op := range ops {
		for _, s := range op.(layout.FillPath).Path {
			if s.Op != layout.ClosePath {
				x, y := s.Point.X.Px(), s.Point.Y.Px()
				x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
			}
		}
	}
	return
}

func TestRenderWarpText(t *testing.T) {
	px := func(v float64) style.Unit { u, _ := style.FromPx(v); return u }
	// A 40 by 10px bar, as an underline, in a 100px square box.
	bar := []layout.Op{layout.FillRect{Rect: layout.Rect{X: px(30), Y: px(45), W: px(40), H: px(10)}, Color: style.RGBA{A: 1}}}
	box := dml.Pixels(100)
	var warnings []string
	colors := &renderColors{ctx: context.Background(), approx: func(err error) { warnings = append(warnings, err.Error()) }}
	warp := func(prst string) []layout.Op {
		t.Helper()
		out, err := renderWarpText(context.Background(), bar, &dml.PrstTxWarp{Prst: prst}, 0, 0, box, box, colors, 100000)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	// Plain text fills the box: its top on the top path, its bottom on the
	// bottom one.
	if x0, y0, x1, y1 := renderWarpBounds(warp("textPlain")); math.Abs(x0) > 0.5 || math.Abs(y0) > 0.5 || math.Abs(x1-100) > 0.5 || math.Abs(y1-100) > 0.5 {
		t.Fatalf("plain: %v %v %v %v", x0, y0, x1, y1)
	}
	// An upward arch runs over the box's top; the bar, 10px high, hangs
	// below it, inside the box.
	x0, y0, x1, y1 := renderWarpBounds(warp("textArchUp"))
	if y0 < -0.5 || y1 > 60 || x0 < -0.5 || x1 > 100.5 || x1-x0 < 50 {
		t.Fatalf("arch: %v %v %v %v", x0, y0, x1, y1)
	}
	if len(warnings) != 2 || !strings.Contains(warnings[1], "text warp textArchUp drawn approximately") {
		t.Fatalf("warnings: %q", warnings)
	}
	// No warp leaves the drawing as it is; an unknown one is invalid.
	if out, err := renderWarpText(context.Background(), bar, &dml.PrstTxWarp{Prst: "textNoShape"}, 0, 0, box, box, colors, 100000); err != nil || len(out) != 1 {
		t.Fatalf("no shape: %v %v", out, err)
	}
	if _, err := renderWarpText(context.Background(), bar, &dml.PrstTxWarp{Prst: "textSpiral"}, 0, 0, box, box, colors, 100000); !errors.Is(err, render.ErrInvalid) {
		t.Fatalf("unknown warp: %v", err)
	}
	// Segments are bounded.
	if _, err := renderWarpText(context.Background(), bar, &dml.PrstTxWarp{Prst: "textWave1"}, 0, 0, box, box, colors, 20); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("segment limit: %v", err)
	}
}

func TestRenderWarpedSlideText(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	warp := map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		i := strings.Index(s, "<a:bodyPr")
		j := i + strings.Index(s[i:], ">")
		if s[j-1] == '/' {
			return s[:j-1] + `><a:prstTxWarp prst="textInflate"><a:avLst/></a:prstTxWarp></a:bodyPr>` + s[j+1:]
		}
		return s[:j+1] + `<a:prstTxWarp prst="textInflate"><a:avLst/></a:prstTxWarp>` + s[j+1:]
	}}
	if _, err := renderRewrittenPNG(t, data, opts, warp); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict warp: %v", err)
	}
	var warnings []string
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	got := renderSlidePNG(t, data, opts, warp)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "textInflate") {
		t.Fatalf("warnings: %q", warnings)
	}
	// Inflated, the two lines fill the 48px box from top to bottom.
	_, top, _, bottom := renderInkBounds(t, got)
	if top > 6 || bottom < 49 {
		t.Fatalf("inflated text spans y %d to %d", top, bottom)
	}
}
