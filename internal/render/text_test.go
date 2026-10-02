package render

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

func TestTextPlacementMatchesPositionedGlyphs(t *testing.T) {
	f := testFace(t, nil)
	op := layout.DrawText{At: layout.Point{X: unit(2), Y: unit(10)}, Text: "AA", Face: f, Size: unit(8), Color: style.RGBA{A: 1}, WidthScale: 0.5}
	p, e := Prepare(context.Background(), dml.Pixels(20), dml.Pixels(12), []layout.Op{op}, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(f.Used()) != 0 {
		t.Fatal("source font mutated")
	}
	if len(p.draws) != 2 || p.draws[0].rect.x0 != 2 || p.draws[0].rect.x1 != 6 || p.draws[1].rect.x0 != 6 || p.draws[1].rect.x1 != 10 {
		t.Fatalf("text placement: %+v", p.draws)
	}
	var out bytes.Buffer
	if e = p.WriteSVG(context.Background(), &out, 96); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(out.Bytes(), []byte("<text")) {
		t.Fatal("SVG delegated text shaping")
	}
}

func TestTextResourceAndCapabilityErrors(t *testing.T) {
	f := testFace(t, nil)
	base := layout.DrawText{Text: "AA", Face: f, Size: unit(8), Color: style.RGBA{A: 1}}
	cases := []struct {
		op   layout.DrawText
		lim  Limits
		want error
	}{
		{base, Limits{MaxGlyphs: 1}, ErrLimit}, {base, Limits{MaxShapeWork: 1}, ErrLimit}, {base, Limits{MaxRunBytes: 1}, ErrLimit},
	}
	for _, set := range []func(*layout.DrawText){func(v *layout.DrawText) { v.Sideways = true }, func(v *layout.DrawText) { v.CharSpacing = unit(1) }, func(v *layout.DrawText) { v.Text = "missing" }, func(v *layout.DrawText) { v.Features.Vertical = true }} {
		op := base
		set(&op)
		cases = append(cases, struct {
			op   layout.DrawText
			lim  Limits
			want error
		}{op, Limits{}, ErrUnsupported})
	}
	for _, tc := range cases {
		p, e := Prepare(context.Background(), dml.Pixels(20), dml.Pixels(12), []layout.Op{tc.op}, tc.lim)
		if !errors.Is(e, tc.want) || p != nil {
			t.Fatalf("got %v %v, want %v", p, e, tc.want)
		}
	}
	// Actual work is cumulative across runs. Each run fits individually.
	result, e := f.ShapeGlyphsContext(context.Background(), shape.RunInput{Text: "AA"}, shape.RunLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := Prepare(context.Background(), dml.Pixels(20), dml.Pixels(12), []layout.Op{base, base}, Limits{MaxShapeWork: result.Work})
	if !errors.Is(e, ErrLimit) || p != nil {
		t.Fatalf("aggregate budget: %v %v", p, e)
	}
}
