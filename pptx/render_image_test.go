package pptx

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

func TestRenderDecodesRepeatedImagesOnce(t *testing.T) {
	p, s, _ := renderTestSlide(t)
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}
	pic := NewPicture()
	pic.SetImageData(encoded.Bytes(), "image/png")
	pic.SetPosition(dml.Pixels(5), dml.Pixels(1))
	pic.SetSize(dml.Pixels(1), dml.Pixels(1))
	if err := s.AddShape(pic); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// A second picture referencing the same media part, one pixel lower.
	picture := regexp.MustCompile(`<p:pic>.*</p:pic>`)
	repeat := func(s string) string {
		one := picture.FindString(s)
		two := regexp.MustCompile(`<a:off x="(\d+)" y="9525"/>`).ReplaceAllString(one, `<a:off x="$1" y="19050"/>`)
		return picture.ReplaceAllLiteralString(s, one+two)
	}
	opts := render.Options{Limits: render.Limits{MaxImagePixels: 1, MaxImageBytes: int64(encoded.Len())}}
	got, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": repeat})
	if err != nil {
		t.Fatal(err)
	}
	for _, y := range []int{1, 2} {
		if px := renderPixel(t, got, 5, y); px != (color.NRGBA{G: 255, A: 255}) {
			t.Fatalf("picture at row %d: %+v", y, px)
		}
	}
	// Distinct images are still charged separately.
	pic2 := NewPicture()
	pic2.SetImageData(append([]byte(nil), encoded.Bytes()...), "image/png")
	pic2.SetPosition(dml.Pixels(6), dml.Pixels(1))
	pic2.SetSize(dml.Pixels(1), dml.Pixels(1))
	if err = s.AddShape(pic2); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PrepareRender(t.Context(), opts); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("distinct images: %v", err)
	}
}
