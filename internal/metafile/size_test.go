package metafile

import (
	"errors"
	"math/bits"
	"testing"

	"github.com/mgilbir/spine/render"
)

func TestRasterSizeFitsInt(t *testing.T) {
	// 40000 by 40000 is within these limits, but its per-pixel buffers, of up
	// to 16 bytes a pixel, are past what a 32-bit int counts.
	lim := render.Limits{MaxDimension: 1 << 20, MaxImagePixels: 1 << 40}
	err := checkRasterSize(40000, 40000, lim)
	if bits.UintSize == 32 {
		if !errors.Is(err, render.ErrLimit) {
			t.Fatalf("32-bit: %v", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("64-bit: %v", err)
	}
	if err = checkRasterSize(40000, 40000, render.Limits{MaxDimension: 1 << 20, MaxImagePixels: 1 << 20}); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("past the pixel limit: %v", err)
	}
}
