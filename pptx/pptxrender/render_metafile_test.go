package pptxrender

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image/color"
	"image/png"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

// renderEMF writes an EMF of a 40 by 40 pixel picture filled by one rectangle
// in solid red, in the given mix mode (ROP2); 0 leaves the default, copying
// the brush. XOR (7) mixes with the picture underneath, which is not drawn.
func renderEMF(rop2 uint32) []byte {
	var recs [][]byte
	rec := func(typ uint32, vals ...uint32) {
		b := make([]byte, 8+4*len(vals))
		binary.LittleEndian.PutUint32(b, typ)
		binary.LittleEndian.PutUint32(b[4:], uint32(len(b)))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[8+4*i:], v)
		}
		recs = append(recs, b)
	}
	if rop2 != 0 {
		rec(20, rop2) // EMR_SETROP2
	}
	rec(39, 1, 0, 0x0000ff, 0) // brush
	rec(37, 1)
	rec(37, 0x80000008) // null pen
	rec(43, 0, 0, 40, 40)
	hdr := make([]byte, 88)
	put := func(off int, v uint32) { binary.LittleEndian.PutUint32(hdr[off:], v) }
	put(0, 1)
	put(4, 88)
	put(16, 39)
	put(20, 39)
	put(32, 1000)
	put(36, 1000)
	put(40, 0x464d4520)
	put(44, 0x10000)
	put(52, uint32(len(recs)+2))
	binary.LittleEndian.PutUint16(hdr[56:], 2)
	put(72, 1000)
	put(76, 1000)
	put(80, 250)
	put(84, 250)
	out := hdr
	for _, r := range recs {
		out = append(out, r...)
	}
	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof, 14)
	binary.LittleEndian.PutUint32(eof[4:], 20)
	binary.LittleEndian.PutUint32(eof[16:], 20)
	out = append(out, eof...)
	binary.LittleEndian.PutUint32(out[48:], uint32(len(out)))
	return out
}

func renderMetafileSlide(t *testing.T, emf []byte, opts render.Options) (color.NRGBA, []error, error) {
	t.Helper()
	_, s, _, base := renderTextSlide(t)
	opts.Fonts = base.Fonts
	pic := pptx.NewPicture()
	pic.SetImageData(emf, "image/x-emf")
	pic.SetPosition(dml.Pixels(60), dml.Pixels(40))
	pic.SetSize(dml.Pixels(10), dml.Pixels(10))
	if err := s.AddShape(pic); err != nil {
		t.Fatal(err)
	}
	var warnings []error
	if opts.Warn != nil {
		opts.Warn = func(err error) { warnings = append(warnings, err) }
	}
	page, err := PrepareSlide(context.Background(), s, opts)
	if err != nil {
		return color.NRGBA{}, warnings, err
	}
	var out bytes.Buffer
	if err = page.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	return color.NRGBAModel.Convert(img.At(65, 45)).(color.NRGBA), warnings, nil
}

func TestRenderEMFPicture(t *testing.T) {
	got, _, err := renderMetafileSlide(t, renderEMF(0), render.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("pixel = %v", got)
	}
}

func TestRenderEMFPictureStrictAndBestEffort(t *testing.T) {
	xor := renderEMF(7)
	if _, _, err := renderMetafileSlide(t, xor, render.Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	got, warnings, err := renderMetafileSlide(t, xor, render.Options{Warn: func(error) {}})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 || !errors.Is(warnings[0], render.ErrApproximated) {
		t.Fatalf("warnings: %v", warnings)
	}
	if got != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("XOR fill left out, pixel = %v", got)
	}
}

func TestRenderEMFPictureBudget(t *testing.T) {
	_, _, _, base := renderTextSlide(t)
	opts := render.Options{Fonts: base.Fonts, Limits: render.Limits{MaxImagePixels: 20}}
	if _, _, err := renderMetafileSlide(t, renderEMF(0), opts); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("budget: %v", err)
	}
}
