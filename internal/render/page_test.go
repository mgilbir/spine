package render

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

func unit(px float64) style.Unit { u, _ := style.FromPx(px); return u }

func TestPhysicalSizing(t *testing.T) {
	p, err := Prepare(context.Background(), dml.EMUsPerInch, 2*dml.EMUsPerInch, nil, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dpi  float64
		w, h int
	}{{0, 96, 192}, {72, 72, 144}, {144, 144, 288}, {96.25, 97, 193}} {
		w, h, err := p.Size(tc.dpi)
		if err != nil || w != tc.w || h != tc.h {
			t.Fatalf("DPI %v: %dx%d, %v", tc.dpi, w, h, err)
		}
	}
	for _, dpi := range []float64{-1, math.NaN(), math.Inf(1), math.SmallestNonzeroFloat64} {
		if _, _, err := p.Size(dpi); !errors.Is(err, ErrInvalid) {
			t.Fatalf("DPI %v: %v", dpi, err)
		}
	}
	if _, _, err := p.Size(math.MaxFloat64); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}

func TestLimitsBeforePixelAllocation(t *testing.T) {
	for _, limits := range []Limits{{MaxDimension: 95}, {MaxPixels: 96*192 - 1}} {
		p, err := Prepare(context.Background(), dml.EMUsPerInch, 2*dml.EMUsPerInch, nil, limits)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := p.Size(96); !errors.Is(err, ErrLimit) {
			t.Fatal(err)
		}
	}
	p, err := Prepare(context.Background(), dml.EMU(math.MaxInt64), dml.EMU(math.MaxInt64), nil, Limits{MaxDimension: int(^uint(0) >> 1), MaxPixels: math.MaxInt64})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Size(96); !errors.Is(err, ErrLimit) {
		t.Fatalf("huge page: %v", err)
	}
}

func TestPrepareSnapshotAndClipping(t *testing.T) {
	r := layout.FillRect{Rect: layout.Rect{X: unit(-10), Y: unit(5), W: unit(30), H: unit(200)}, Color: style.RGBA{R: 255, A: 0.5}}
	ops := []layout.Op{r}
	p, err := Prepare(context.Background(), dml.Pixels(96), dml.Pixels(96), ops, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	ops[0] = layout.FillRect{}
	if len(p.draws) != 1 || p.draws[0].rect != (rectangle{0, 5, 20, 96, r.Color}) {
		t.Fatalf("snapshot: %+v", p.draws)
	}
	// Even invisible unsupported operations must not bypass capability checks.
	if _, err := Prepare(context.Background(), dml.Pixels(96), dml.Pixels(96), []layout.Op{layout.TileImage{}}, Limits{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
}

func TestPrepareRejectsInvalidAndCancelled(t *testing.T) {
	for _, c := range []style.RGBA{{R: 256, A: 1}, {G: -1, A: 1}, {B: math.Inf(1), A: 1}, {A: math.NaN()}, {A: 2}} {
		_, err := Prepare(context.Background(), 1, 1, []layout.Op{layout.FillRect{Color: c}}, Limits{})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("color %+v: %v", c, err)
		}
	}
	for _, l := range []Limits{{MaxDimension: -1}, {MaxPixels: -1}, {MaxOperations: -1}, {MaxPixelVisits: -1}, {MaxOutputBytes: -1}} {
		if _, err := Prepare(context.Background(), 1, 1, nil, l); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := Prepare(context.Background(), 1, 1, []layout.Op{layout.FillRect{}, layout.FillRect{}}, Limits{MaxOperations: 1}); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), 1, 1, []layout.Op{layout.FillRect{Rect: layout.Rect{W: -1}}}, Limits{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), 0, 1, nil, Limits{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Prepare(ctx, 1, 1, nil, Limits{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var p Page
	if _, _, err := p.Size(96); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func FuzzSizing(f *testing.F) {
	f.Add(int64(914400), int64(1828800), 96.0)
	f.Add(int64(math.MaxInt64), int64(1), math.MaxFloat64)
	f.Fuzz(func(t *testing.T, w, h int64, dpi float64) {
		p, err := Prepare(context.Background(), dml.EMU(w), dml.EMU(h), nil, Limits{})
		if err != nil {
			return
		}
		pw, ph, err := p.Size(dpi)
		if err == nil && (pw <= 0 || ph <= 0 || pw > 8192 || ph > 8192 || int64(pw)*int64(ph) > 16<<20) {
			t.Fatalf("unbounded size %dx%d", pw, ph)
		}
	})
}
