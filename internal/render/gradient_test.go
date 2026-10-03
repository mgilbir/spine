package render

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"strings"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

func TestGradientFill(t *testing.T) {
	rect := layout.Rect{W: unit(8), H: unit(2)}
	stops := []layout.GradientStop{{Offset: 0, Color: style.RGBA{A: 1}}, {Offset: 1, Color: style.RGBA{R: 255, G: 255, B: 255, A: 1}, Exponent: 1}}
	fill := layout.FillGradient{Clip: rect, Tile: rect, StepX: unit(8), StepY: unit(2), Gradient: layout.Gradient{
		Kind: layout.LinearGradient, Start: layout.Point{}, End: layout.Point{X: unit(8)}, Stops: stops,
	}}
	p, err := Prepare(context.Background(), dml.Pixels(8), dml.Pixels(2), []layout.Op{fill}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	stops[0].Color = style.RGBA{R: 255, A: 1}
	var b bytes.Buffer
	if err = p.WritePNG(context.Background(), &b, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&b)
	if err != nil {
		t.Fatal(err)
	}
	// Pixel centres sit at offsets 1/16, 3/16, …: dark to light, gray.
	prev := -1
	for x := 0; x < 8; x++ {
		r, g, bl, _ := img.At(x, 1).RGBA()
		if r != g || g != bl || int(r) <= prev {
			t.Fatalf("pixel %d: %d %d %d after %d", x, r, g, bl, prev)
		}
		prev = int(r)
	}
	var svg bytes.Buffer
	if err = p.WriteSVG(context.Background(), &svg, 96); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg.String(), "<linearGradient") || !strings.Contains(svg.String(), `fill="url(#gradient0)"`) {
		t.Fatalf("SVG: %s", svg.String())
	}
	for name, g := range map[string]layout.Gradient{
		"repeating":     {Kind: layout.LinearGradient, Repeating: true, End: layout.Point{X: unit(8)}, Stops: stops},
		"conic":         {Kind: layout.ConicGradient, Stops: stops},
		"stop past end": {Kind: layout.LinearGradient, End: layout.Point{X: unit(8)}, Stops: []layout.GradientStop{{Offset: 1.5, Color: style.RGBA{A: 1}}}},
		"hint":          {Kind: layout.LinearGradient, End: layout.Point{X: unit(8)}, Stops: []layout.GradientStop{stops[0], {Offset: 1, Color: style.RGBA{A: 1}, Exponent: 2}}},
	} {
		f := fill
		f.Gradient = g
		if _, err := Prepare(context.Background(), dml.Pixels(8), dml.Pixels(2), []layout.Op{f}, Limits{}); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func FuzzGradient(f *testing.F) {
	f.Add(0.0, 0.0, 8.0, 0.0, 0.0, 0.5, 1.0, uint8(0), 4.0, 2.0)
	f.Add(4.0, 1.0, 4.0, 1.0, 0.2, 0.2, 0.9, uint8(1), 0.001, 9.0)
	f.Fuzz(func(t *testing.T, x0, y0, x1, y1, a, b, c float64, kind uint8, rx, ry float64) {
		ux, ok1 := style.FromPx(x0)
		uy, ok2 := style.FromPx(y0)
		vx, ok3 := style.FromPx(x1)
		vy, ok4 := style.FromPx(y1)
		urx, ok5 := style.FromPx(rx)
		ury, ok6 := style.FromPx(ry)
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
			return
		}
		g := layout.Gradient{Kind: layout.LinearGradient, Start: layout.Point{X: ux, Y: uy}, End: layout.Point{X: vx, Y: vy}, Center: layout.Point{X: ux, Y: uy}, RadiusX: urx, RadiusY: ury}
		if kind%2 == 1 {
			g.Kind = layout.RadialGradient
		}
		for i, o := range []float64{a, b, c} {
			g.Stops = append(g.Stops, layout.GradientStop{Offset: o, Color: style.RGBA{R: float64(i) * 100, G: 50, B: 200, A: 0.5 + float64(i)/4}, Exponent: 1})
		}
		rect := layout.Rect{W: unit(8), H: unit(4)}
		p, err := Prepare(context.Background(), dml.Pixels(8), dml.Pixels(4), []layout.Op{layout.FillGradient{Clip: rect, Tile: rect, StepX: unit(8), StepY: unit(4), Gradient: g}}, Limits{})
		if err != nil {
			return
		}
		var b2 bytes.Buffer
		if err = p.WritePNG(context.Background(), &b2, 96); err != nil {
			t.Fatal(err)
		}
		if err = p.WriteSVG(context.Background(), &b2, 96); err != nil {
			t.Fatal(err)
		}
	})
}
