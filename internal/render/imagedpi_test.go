package render

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"testing"
)

// WithPNGResolution inserts a pHYs chunk of the given pixels per metre after
// a PNG's header chunk.
func withPNGResolution(t testing.TB, data []byte, x, y uint32, unit byte) []byte {
	t.Helper()
	chunk := make([]byte, 0, 21)
	chunk = binary.BigEndian.AppendUint32(chunk, 9)
	chunk = append(chunk, "pHYs"...)
	chunk = binary.BigEndian.AppendUint32(chunk, x)
	chunk = binary.BigEndian.AppendUint32(chunk, y)
	chunk = append(chunk, unit)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	// The signature and IHDR chunk take 8 and 25 bytes.
	return append(append(append([]byte(nil), data[:33]...), chunk...), data[33:]...)
}

func TestImageDPI(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	var p, j bytes.Buffer
	if err := png.Encode(&p, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&j, img, nil); err != nil {
		t.Fatal(err)
	}
	jfif := func(units byte, x, y uint16) []byte {
		app := []byte{0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, units}
		app = binary.BigEndian.AppendUint16(app, x)
		app = binary.BigEndian.AppendUint16(app, y)
		app = append(app, 0, 0)
		return append(append([]byte{0xFF, 0xD8}, app...), j.Bytes()[2:]...)
	}
	for _, tc := range []struct {
		name string
		data []byte
		x, y float64
		ok   bool
	}{
		{"png without", p.Bytes(), 0, 0, false},
		{"png 300 by 150", withPNGResolution(t, p.Bytes(), 11811, 5906, 1), 300, 150, true},
		{"png aspect only", withPNGResolution(t, p.Bytes(), 1, 1, 0), 0, 0, false},
		{"jpeg without", j.Bytes(), 0, 0, false},
		{"jpeg 72", jfif(1, 72, 72), 72, 72, true},
		{"jpeg per cm", jfif(2, 100, 100), 254, 254, true},
		{"jpeg aspect only", jfif(0, 1, 1), 0, 0, false},
		{"truncated", p.Bytes()[:20], 0, 0, false},
		{"other", []byte("GIF89a"), 0, 0, false},
	} {
		x, y, ok := ImageDPI(tc.data)
		if ok != tc.ok || math.Abs(x-tc.x) > 0.1 || math.Abs(y-tc.y) > 0.1 {
			t.Errorf("%s: %v, %v, %v", tc.name, x, y, ok)
		}
	}
	// The files still decode.
	if _, err := png.Decode(bytes.NewReader(withPNGResolution(t, p.Bytes(), 11811, 11811, 1))); err != nil {
		t.Fatal(err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(jfif(1, 72, 72))); err != nil {
		t.Fatal(err)
	}
}

func FuzzImageDPI(f *testing.F) {
	f.Add([]byte("\x89PNG\r\n\x1a\n"))
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 16})
	f.Fuzz(func(t *testing.T, data []byte) {
		x, y, ok := ImageDPI(data)
		if ok && (!(x > 0) || !(y > 0) || math.IsInf(x, 0) || math.IsInf(y, 0)) {
			t.Fatalf("%v %v", x, y)
		}
	})
}
