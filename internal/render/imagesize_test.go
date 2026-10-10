package render

import (
	"context"
	"errors"
	"math/bits"
	"testing"
)

func TestImageSizeFitsInt(t *testing.T) {
	// 40000 by 40000 is within these limits, but its decoding buffers, of up
	// to 32 bytes a pixel, are past what a 32-bit int counts.
	l := Limits{MaxDimension: 1 << 20, MaxImagePixels: 1 << 40}
	err := checkImageSize(40000, 40000, l)
	if bits.UintSize == 32 {
		if !errors.Is(err, ErrLimit) {
			t.Fatalf("32-bit: %v", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("64-bit: %v", err)
	}
}

func TestDeclaredPNGSizePastLimits(t *testing.T) {
	// A PNG header declaring 100000 by 100000 pixels is too large, on every
	// platform: on 32-bit the standard decoder would refuse it first, as an
	// overflow, before the limits are applied.
	ihdr := []byte{0, 1, 0x86, 0xa0, 0, 1, 0x86, 0xa0, 8, 6, 0, 0, 0}
	data := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"), ihdr...)
	data = append(data, 0, 0, 0, 0) // CRC, unchecked before the size
	if _, err := DecodeImage(context.Background(), data, Limits{}); !errors.Is(err, ErrLimit) {
		t.Fatalf("got %v, want a limit", err)
	}
}
