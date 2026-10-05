package render

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/mgilbir/forme/style"
)

// boxes are the widths of three box blurs that approximate a
// Gaussian of deviation sigma.
func boxes(sigma float64) [3]int {
	ideal := math.Sqrt(12*sigma*sigma/3 + 1)
	lo := int(math.Floor(ideal))
	if lo%2 == 0 {
		lo--
	}
	hi := lo + 2
	m := int(math.Round((12*sigma*sigma - 3*float64(lo*lo) - 12*float64(lo) - 9) / (-4*float64(lo) - 4)))
	var out [3]int
	for i := range out {
		if i < m {
			out[i] = lo
		} else {
			out[i] = hi
		}
	}
	return out
}

// Blur blurs a w by h plane in place with an approximate Gaussian of
// deviation sigma pixels, edges extending outwards.
func Blur(ctx context.Context, plane []float32, w, h int, sigma float64) error {
	if sigma <= 0.3 || w == 0 || h == 0 {
		return nil
	}
	tmp := make([]float32, len(plane))
	pass := func(src, dst []float32, n, stride, count, lineStride, r int) {
		for line := 0; line < count; line++ {
			base := line * lineStride
			at := func(i int) float32 { return src[base+max(0, min(n-1, i))*stride] }
			var sum float32
			for i := -r; i <= r; i++ {
				sum += at(i)
			}
			inv := 1 / float32(2*r+1)
			for i := 0; i < n; i++ {
				dst[base+i*stride] = sum * inv
				sum += at(i+r+1) - at(i-r)
			}
		}
	}
	for _, box := range boxes(sigma) {
		if err := ctx.Err(); err != nil {
			return err
		}
		r := (box - 1) / 2
		pass(plane, tmp, w, 1, h, w, r)
		pass(tmp, plane, h, w, w, 1, r)
	}
	return nil
}

func HSL(c style.RGBA) (h, s, l float64) {
	r, g, b := c.R/255, c.G/255, c.B/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (hi + lo) / 2
	d := hi - lo
	if d == 0 {
		return 0, 0, l
	}
	if l > 0.5 {
		s = d / (2 - hi - lo)
	} else {
		s = d / (hi + lo)
	}
	switch hi {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

// FromHSL converts HSL to sRGB channels in 0-1, unrounded.
func FromHSL(h, s, l float64) [3]float64 {
	c := fromHSLWith(h, s, l, func(v float64) float64 { return math.Min(1, math.Max(0, v)) })
	return [3]float64{c.R, c.G, c.B}
}

func fromHSLWith(h, s, l float64, channel func(float64) float64) style.RGBA {
	if s == 0 {
		return style.RGBA{R: channel(l), G: channel(l), B: channel(l), A: 1}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		switch {
		case t < 0:
			t++
		case t > 1:
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return style.RGBA{R: channel(hue(h + 1.0/3)), G: channel(hue(h)), B: channel(hue(h - 1.0/3)), A: 1}
}

// MaxImageScale is how many image pixels per drawn CSS pixel a picture
// keeps: enough for 384 DPI output.
const MaxImageScale = 4

// Downscale averages an image down to at most w by h pixels; a
// picture drawn smaller than its pixels needs no more.
func Downscale(img image.Image, w, h float64) image.Image {
	b := img.Bounds()
	tw, th := int(math.Ceil(w)), int(math.Ceil(h))
	if tw < 1 || th < 1 || (b.Dx() <= tw && b.Dy() <= th) {
		return img
	}
	tw, th = min(tw, b.Dx()), min(th, b.Dy())
	out := image.NewNRGBA(image.Rect(0, 0, tw, th))
	// Each source row is read once and summed into the target columns it
	// spans; channels are averaged premultiplied, as compositing would.
	read := RowReader(img)
	row := make([]uint32, 4*b.Dx())
	acc := make([]uint64, 4*tw)
	spans := make([][2]int, tw)
	for x := range spans {
		spans[x] = [2]int{x * b.Dx() / tw, max((x+1)*b.Dx()/tw, x*b.Dx()/tw+1)}
	}
	for y := 0; y < th; y++ {
		y0, y1 := y*b.Dy()/th, max((y+1)*b.Dy()/th, y*b.Dy()/th+1)
		clear(acc)
		for sy := y0; sy < y1; sy++ {
			read(sy, row)
			for x, sp := range spans {
				a := acc[4*x : 4*x+4]
				for sx := sp[0]; sx < sp[1]; sx++ {
					a[0] += uint64(row[4*sx])
					a[1] += uint64(row[4*sx+1])
					a[2] += uint64(row[4*sx+2])
					a[3] += uint64(row[4*sx+3])
				}
			}
		}
		p := out.Pix[y*out.Stride:]
		for x, sp := range spans {
			n := uint64((sp[1] - sp[0]) * (y1 - y0))
			a := acc[4*x : 4*x+4]
			Unpremultiply(p[4*x:4*x+4], uint32(a[0]/n), uint32(a[1]/n), uint32(a[2]/n), uint32(a[3]/n))
		}
	}
	return out
}

// Fade multiplies an image's opacity.
func Fade(img image.Image, alpha float64) image.Image {
	out := NRGBA(img)
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = uint8(math.Round(float64(out.Pix[i]) * alpha))
	}
	return out
}

// Crop keeps the part of an image a picture's source rectangle
// selects; crop fractions are of the image's width and height.
func Crop(img image.Image, l, t, r, b float64) (image.Image, error) {
	if l == 0 && t == 0 && r == 0 && b == 0 {
		return img, nil
	}
	if l < 0 || t < 0 || r < 0 || b < 0 || l+r >= 1 || t+b >= 1 {
		return nil, fmt.Errorf("%w: extended or empty picture crop", ErrUnsupported)
	}
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return nil, fmt.Errorf("%w: picture crop", ErrUnsupported)
	}
	bounds := img.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	rect := image.Rect(bounds.Min.X+int(math.Round(l*w)), bounds.Min.Y+int(math.Round(t*h)), bounds.Max.X-int(math.Round(r*w)), bounds.Max.Y-int(math.Round(b*h)))
	if rect.Empty() {
		return nil, fmt.Errorf("%w: empty picture crop", ErrUnsupported)
	}
	return sub.SubImage(rect), nil
}
