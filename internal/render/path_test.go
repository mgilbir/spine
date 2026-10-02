package render

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"image/color"
	"image/png"
	"io"
	"math"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

func polygon(xy ...float64) layout.Path {
	var path layout.Path
	for i := 0; i < len(xy); i += 2 {
		op := layout.LineTo
		if i == 0 {
			op = layout.MoveTo
		}
		path = append(path, layout.PathSegment{Op: op, Point: layout.Point{X: unit(xy[i]), Y: unit(xy[i+1])}})
	}
	return append(path, layout.PathSegment{Op: layout.ClosePath})
}

func TestPathHolesAndClipping(t *testing.T) {
	outer := polygon(0, 0, 4, 0, 4, 4, 0, 4)
	hole := polygon(1, 1, 3, 1, 3, 3, 1, 3) // Same winding: even-odd still cuts a hole.
	path := append(outer, hole...)
	clip := polygon(0, 0, 2, 0, 2, 4, 0, 4)
	ops := []layout.Op{
		layout.ClipPath{Path: clip, Ops: []layout.Op{layout.FillPath{Path: path, Color: style.RGBA{R: 255, A: 1}}}},
		layout.FillRect{Rect: layout.Rect{X: unit(3), Y: unit(3), W: unit(1), H: unit(1)}, Color: style.RGBA{B: 255, A: 1}},
	}
	p, err := Prepare(context.Background(), dml.Pixels(4), dml.Pixels(4), ops, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	// Mutation of every input slice after preparation must have no effect.
	path[0].Point.X = unit(100)
	clip[0].Point.Y = unit(100)
	ops[0] = layout.FillRect{}
	var b bytes.Buffer
	if err := p.WritePNG(context.Background(), &b, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&b)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			want := color.NRGBA{}
			if x == 0 || (x == 1 && (y == 0 || y == 3)) {
				want = color.NRGBA{R: 255, A: 255}
			}
			if x == 3 && y == 3 {
				want = color.NRGBA{B: 255, A: 255}
			}
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if got != want {
				t.Fatalf("%d,%d: %+v want %+v", x, y, got, want)
			}
		}
	}
}

func TestIntersectClipsBeforeCoverage(t *testing.T) {
	// An identical half-pixel clip applied twice stays half coverage, not quarter.
	clip := polygon(0, 0, 0.5, 0, 0.5, 1, 0, 1)
	op := layout.ClipPath{Path: clip, Ops: []layout.Op{layout.ClipPath{Path: clip, Ops: []layout.Op{layout.FillRect{Rect: layout.Rect{W: unit(1), H: unit(1)}, Color: style.RGBA{G: 255, A: 1}}}}}}
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
	if got := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); got != (color.NRGBA{G: 255, A: 128}) {
		t.Fatal(got)
	}
}

func TestEllipseArcsAndSVG(t *testing.T) {
	path := layout.Path{{Op: layout.ArcTo, Center: layout.Point{X: unit(2), Y: unit(2)}, RadiusX: unit(2), RadiusY: unit(2), StartAngle: 0, SweepAngle: 360}}
	p, err := Prepare(context.Background(), dml.Pixels(4), dml.Pixels(4), []layout.Op{layout.FillPath{Path: path, Color: style.RGBA{R: 255, A: 1}}}, Limits{})
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
	// Center is fully inside; extreme corner lies mostly outside the circle.
	_, _, _, center := img.At(1, 1).RGBA()
	_, _, _, corner := img.At(0, 0).RGBA()
	if center != 65535 || corner >= center/2 {
		t.Fatalf("center %d corner %d", center, corner)
	}
	b.Reset()
	if err := p.WriteSVG(context.Background(), &b, 96); err != nil {
		t.Fatal(err)
	}
	d := xml.NewDecoder(&b)
	var data string
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if s, ok := token.(xml.StartElement); ok && s.Name.Local == "path" {
			for _, a := range s.Attr {
				if a.Name.Local == "d" {
					data = a.Value
				}
			}
		}
	}
	if bytes.Count([]byte(data), []byte("A")) != 2 {
		t.Fatalf("full circle must use two arcs: %s", data)
	}
}

func TestPathResourceLimitsAndInvalidSegments(t *testing.T) {
	path := polygon(0, 0, 4, 0, 4, 4, 0, 4)
	fill := layout.FillPath{Path: path, Color: style.RGBA{A: 1}}
	for _, limits := range []Limits{{MaxPathSegments: 4}, {MaxOperations: 1}} {
		ops := []layout.Op{fill}
		if limits.MaxOperations == 1 {
			ops = []layout.Op{layout.ClipPath{Path: path, Ops: ops}}
		}
		if _, err := Prepare(context.Background(), dml.Pixels(4), dml.Pixels(4), ops, limits); !errors.Is(err, ErrLimit) {
			t.Fatal(err)
		}
	}
	// A cyclic operation graph terminates at the configured depth limit.
	cycle := make([]layout.Op, 1)
	cycle[0] = layout.ClipPath{Path: path, Ops: cycle}
	if _, err := Prepare(context.Background(), 1, 1, cycle, Limits{MaxClipDepth: 2}); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	for _, s := range []layout.PathSegment{
		{Op: layout.LineTo}, {Op: layout.ClosePath}, {Op: layout.ArcTo, StartAngle: math.NaN()},
		{Op: layout.ArcTo, SweepAngle: 361}, {Op: layout.ArcTo, RadiusX: -1},
	} {
		if _, err := Prepare(context.Background(), 1, 1, []layout.Op{layout.FillPath{Path: layout.Path{s}}}, Limits{}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%+v: %v", s, err)
		}
	}
	if _, err := Prepare(context.Background(), 1, 1, []layout.Op{layout.FillPath{Path: layout.Path{{Op: 99}}}}, Limits{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	p, err := Prepare(context.Background(), dml.Pixels(4), dml.Pixels(4), []layout.Op{fill}, Limits{MaxEdgeChecks: 1})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := p.WritePNG(context.Background(), &b, 96); !errors.Is(err, ErrLimit) || b.Len() != 0 {
		t.Fatalf("edge work: %v", err)
	}
	arc := layout.Path{{Op: layout.ArcTo, RadiusX: style.MaxUnit, RadiusY: style.MaxUnit, SweepAngle: 360}}
	p, err = Prepare(context.Background(), 1, 1, []layout.Op{layout.FillPath{Path: arc}}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WritePNG(context.Background(), &b, 960000); !errors.Is(err, ErrLimit) || b.Len() != 0 {
		t.Fatalf("subdivision: %v", err)
	}
}

func FuzzPathArcs(f *testing.F) {
	f.Add(1.0, 360.0, int32(128))
	f.Add(math.MaxFloat64, math.Inf(1), int32(math.MaxInt32))
	f.Fuzz(func(t *testing.T, start, sweep float64, radius int32) {
		path := layout.Path{{Op: layout.ArcTo, RadiusX: style.Unit(radius), RadiusY: style.Unit(radius), StartAngle: start, SweepAngle: sweep}}
		p, err := Prepare(context.Background(), dml.Pixels(8), dml.Pixels(8), []layout.Op{layout.FillPath{Path: path}}, Limits{MaxPathSegments: 128, MaxEdgeChecks: 10000})
		if err != nil {
			return
		}
		var b bytes.Buffer
		if err := p.WritePNG(context.Background(), &b, 96); err != nil {
			if !errors.Is(err, ErrLimit) {
				t.Fatal(err)
			}
			return
		}
		if _, err := png.Decode(&b); err != nil {
			t.Fatal(err)
		}
	})
}
