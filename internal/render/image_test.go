package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/common/dml"
)

func TestImageOwnershipSamplingAndSVG(t *testing.T) {
	src := image.NewNRGBA(image.Rect(3, 4, 5, 5))
	src.SetNRGBA(3, 4, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(4, 4, color.NRGBA{B: 255, A: 128})
	op := layout.DrawImage{Rect: layout.Rect{W: unit(4), H: unit(2)}, Image: src, Key: "\" onload=\"evil", Clip: layout.Clip{Active: true, Rect: layout.Rect{X: unit(1), W: unit(3), H: unit(2)}}}
	p, e := Prepare(context.Background(), dml.Pixels(4), dml.Pixels(2), []layout.Op{op, op}, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if p.draws[0].image != p.draws[1].image {
		t.Fatal("shared image not reused")
	}
	clear(src.Pix)
	var out bytes.Buffer
	if e = p.WritePNG(context.Background(), &out, 96); e != nil {
		t.Fatal(e)
	}
	img, e := png.Decode(&out)
	if e != nil {
		t.Fatal(e)
	}
	// Each source pixel spans two output pixels: their centres sit a
	// quarter of a source pixel either side of its own, so the pixels
	// between interpolate, three parts to one, and the outermost hold the
	// edge colour. The clip leaves x 0 empty.
	at := func(x int) color.NRGBA { return color.NRGBAModel.Convert(img.At(x, 0)).(color.NRGBA) }
	if got := at(0); got != (color.NRGBA{}) {
		t.Fatalf("clipped pixel: %+v", got)
	}
	if got := at(3); got != (color.NRGBA{B: 255, A: 192}) {
		t.Fatalf("edge pixel: %+v", got)
	}
	// Premultiplied, three parts opaque red to one half-opaque blue.
	if got := at(1); got.R < 200 || got.B == 0 || got.R <= got.B {
		t.Fatalf("pixel 1: %+v", got)
	}
	if got := at(2); got.B <= got.R || got.R == 0 {
		t.Fatalf("pixel 2: %+v", got)
	}
	out.Reset()
	if e = p.WriteSVG(context.Background(), &out, 96); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(out.Bytes(), []byte("onload")) {
		t.Fatal("copied source key into output")
	}
	dec := xml.NewDecoder(&out)
	images := 0
	for {
		tok, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if el, ok := tok.(xml.StartElement); ok && el.Name.Local == "image" {
			images++
			for _, a := range el.Attr {
				if a.Name.Local == "href" {
					data, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(a.Value, "data:image/png;base64,"))
					if e != nil {
						t.Fatal(e)
					}
					bitmap, e := png.Decode(bytes.NewReader(data))
					if e != nil || bitmap.Bounds().Dx() != 2 {
						t.Fatalf("embedded PNG: %v", e)
					}
				}
			}
		}
	}
	if images != 2 {
		t.Fatalf("SVG images %d", images)
	}
}
func TestImageRejectsMalformedStorageAndLimits(t *testing.T) {
	valid := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	var nilImage *image.NRGBA
	cases := []struct {
		src  image.Image
		lim  Limits
		want error
	}{
		{nilImage, Limits{}, ErrInvalid},
		{&image.NRGBA{Rect: image.Rect(0, 0, 2, 2), Stride: 8, Pix: make([]byte, 3)}, Limits{}, ErrInvalid},
		{&image.Paletted{Rect: image.Rect(0, 0, 1, 1), Stride: 1, Pix: []byte{2}, Palette: color.Palette{color.Black}}, Limits{}, ErrInvalid},
		{&image.YCbCr{Rect: image.Rect(0, 0, 2, 2), Y: make([]byte, 4), YStride: 2, CStride: 1}, Limits{}, ErrInvalid},
		{valid, Limits{MaxImagePixels: 3}, ErrLimit}, {valid, Limits{MaxImageBytes: 1}, ErrLimit},
	}
	for _, tt := range cases {
		p, e := Prepare(context.Background(), dml.Pixels(4), dml.Pixels(4), []layout.Op{layout.DrawImage{Rect: layout.Rect{W: unit(4), H: unit(4)}, Image: tt.src}}, tt.lim)
		if !errors.Is(e, tt.want) || p != nil {
			t.Fatalf("%T: %v, want %v", tt.src, e, tt.want)
		}
	}
	var b bytes.Buffer
	if e := png.Encode(&b, valid); e != nil {
		t.Fatal(e)
	}
	if _, e := DecodeImage(context.Background(), b.Bytes(), Limits{MaxImagePixels: 3}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	img, e := DecodeImage(context.Background(), b.Bytes(), Limits{})
	if e != nil || img.Bounds() != valid.Bounds() {
		t.Fatal(e)
	}
	if _, e = DecodeImage(context.Background(), []byte("<svg onload='x'/>"), Limits{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
}

func FuzzRasterImage(f *testing.F) {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	f.Add(b.Bytes())
	f.Add([]byte{0xff, 0xd8, 0xff, 0xd9})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			t.Skip()
		}
		limits := Limits{MaxDimension: 64, MaxImagePixels: 4096, MaxImageBytes: 64 << 10}
		img, e := DecodeImage(context.Background(), data, limits)
		if e != nil {
			return
		}
		p, e := Prepare(context.Background(), dml.Pixels(16), dml.Pixels(16), []layout.Op{layout.DrawImage{Rect: layout.Rect{W: unit(16), H: unit(16)}, Image: img}}, limits)
		if e != nil {
			return
		}
		var out bytes.Buffer
		if e = p.WritePNG(context.Background(), &out, 96); e != nil {
			t.Fatal(e)
		}
		decoded, e := png.Decode(&out)
		if e != nil || decoded.Bounds() != image.Rect(0, 0, 16, 16) {
			t.Fatalf("PNG: %v", e)
		}
	})
}

func TestImageSnapshotCopiesNRGBARows(t *testing.T) {
	// A sub-image's rows copy from its own origin, translucent pixels as
	// they are.
	src := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	for i := range src.Pix {
		src.Pix[i] = uint8(i * 5)
	}
	sub := src.SubImage(image.Rect(1, 1, 3, 3)).(*image.NRGBA)
	p, err := Prepare(context.Background(), dml.Pixels(2), dml.Pixels(2), []layout.Op{layout.DrawImage{Rect: layout.Rect{W: unit(2), H: unit(2)}, Image: sub}}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	got := p.draws[0].image.pixels
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			if got.NRGBAAt(x, y) != sub.NRGBAAt(x+1, y+1) {
				t.Fatalf("%d,%d: %+v, want %+v", x, y, got.NRGBAAt(x, y), sub.NRGBAAt(x+1, y+1))
			}
		}
	}
}

// renderImagePNG paints one image drawn over a w by h pixel page.
func renderImagePNG(t *testing.T, src image.Image, w, h float64) *image.NRGBA {
	t.Helper()
	p, err := Prepare(context.Background(), dml.Pixels(int(w)), dml.Pixels(int(h)), []layout.Op{layout.DrawImage{Rect: layout.Rect{W: unit(w), H: unit(h)}, Image: src}}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := p.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	n := image.NewNRGBA(img.Bounds())
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			n.Set(x, y, img.At(x, y))
		}
	}
	return n
}

func TestImageFiltering(t *testing.T) {
	// One-pixel black and white stripes drawn at half their size average
	// to grey everywhere, where sampling one pixel in two would pick out
	// one colour.
	stripes := image.NewNRGBA(image.Rect(0, 0, 40, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 40; x++ {
			v := uint8(255 * (x % 2))
			stripes.SetNRGBA(x, y, color.NRGBA{v, v, v, 255})
		}
	}
	got := renderImagePNG(t, stripes, 20, 2)
	for x := 0; x < 20; x++ {
		if c := got.NRGBAAt(x, 0); c.R < 120 || c.R > 135 {
			t.Fatalf("stripes at %d: %+v", x, c)
		}
	}
	// Two pixels drawn ten times larger blend smoothly between their
	// centres, at x 5 and 15, and hold their colours outside them.
	pair := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	pair.SetNRGBA(0, 0, color.NRGBA{0, 0, 0, 255})
	pair.SetNRGBA(1, 0, color.NRGBA{200, 200, 200, 255})
	got = renderImagePNG(t, pair, 20, 1)
	prev := -1
	for x := 0; x < 20; x++ {
		v := int(got.NRGBAAt(x, 0).R)
		if v < prev {
			t.Fatalf("not monotonic at %d: %d after %d", x, v, prev)
		}
		prev = v
	}
	if a, b, mid := got.NRGBAAt(2, 0).R, got.NRGBAAt(17, 0).R, got.NRGBAAt(10, 0).R; a != 0 || b != 200 || mid < 90 || mid > 110 {
		t.Fatalf("ends %d and %d, middle %d", a, b, mid)
	}
	// Transparent pixels add no colour: opaque red beside transparent blue
	// averages to half-transparent red.
	edge := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		edge.SetNRGBA(0, y, color.NRGBA{255, 0, 0, 255})
		edge.SetNRGBA(1, y, color.NRGBA{0, 0, 255, 0})
	}
	p, err := Prepare(context.Background(), dml.Pixels(1), dml.Pixels(1), []layout.Op{layout.DrawImage{Rect: layout.Rect{W: unit(1), H: unit(1)}, Image: edge}}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := p.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	img, _ := png.Decode(&out)
	if c := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); c.B != 0 || c.R != 255 || c.A < 125 || c.A > 130 {
		t.Fatalf("translucent edge: %+v", c)
	}
}
