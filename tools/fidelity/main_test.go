package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// slide is a white w by h image with a black square at x.
func slide(w, h, x int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for i := 0; i < w; i++ {
			c := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			if i >= x && i < x+w/4 && y >= h/4 && y < h/2 {
				c = color.NRGBA{A: 255}
			}
			img.SetNRGBA(i, y, c)
		}
	}
	return img
}

func TestCompare(t *testing.T) {
	dir := t.TempDir()
	ours, ref, out := filepath.Join(dir, "ours"), filepath.Join(dir, "ref"), filepath.Join(dir, "out")
	for _, d := range []string{ours, ref} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Slide 1 matches, at twice the reference's size; slide 2 has its
	// square moved; slide 3 has no reference.
	writePNG(t, filepath.Join(ours, "slide-0001.png"), slide(128, 96, 32))
	writePNG(t, filepath.Join(ref, "Slide1.png"), slide(64, 48, 16))
	writePNG(t, filepath.Join(ours, "slide-0002.png"), slide(64, 48, 32))
	writePNG(t, filepath.Join(ref, "Slide2.png"), slide(64, 48, 8))
	writePNG(t, filepath.Join(ours, "slide-0003.png"), slide(64, 48, 8))
	if err := run(config{ours: ours, ref: ref, out: out, threshold: 32}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Slides) != 2 || len(r.Unpaired) != 1 || filepath.Base(r.Unpaired[0]) != "slide-0003.png" {
		t.Fatalf("report: %+v", r)
	}
	same, moved := r.Slides[0], r.Slides[1]
	if same.Number != 1 || math.Abs(same.SSIM-1) > 1e-9 || same.MeanError != 0 || same.Changed != 0 {
		t.Fatalf("same slide: %+v", same)
	}
	// The square moved over 16 of its 16 columns: 2 × 16 × 12 of 64 × 48
	// pixels changed.
	if moved.SSIM >= 0.9 || math.Abs(moved.Changed-2*16*12/(64.0*48)) > 1e-9 || moved.MeanError <= 0 {
		t.Fatalf("moved slide: %+v", moved)
	}
	if math.Abs(r.MeanSSIM-(same.SSIM+moved.SSIM)/2) > 1e-9 {
		t.Fatalf("mean: %v", r.MeanSSIM)
	}
	for _, name := range []string{"report.md", "diff-0001.png", "diff-0002.png"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatal(err)
		}
	}
	// An existing report is not overwritten.
	if err := run(config{ours: ours, ref: ref, out: out, threshold: 32}); err == nil {
		t.Fatal("report overwritten")
	}
	if err := run(config{ours: ours, ref: ref, out: out, threshold: 300}); err == nil {
		t.Fatal("bad threshold accepted")
	}
}

func TestSSIM(t *testing.T) {
	a := luma(slide(16, 16, 0))
	if s := ssim(a, a, 16, 16); math.Abs(s-1) > 1e-9 {
		t.Fatalf("identical: %v", s)
	}
	inverted := make([]float64, len(a))
	for i, v := range a {
		inverted[i] = 255 - v
	}
	if s := ssim(a, inverted, 16, 16); s >= 0.5 {
		t.Fatalf("inverted: %v", s)
	}
}
