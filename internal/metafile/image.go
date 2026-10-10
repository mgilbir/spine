package metafile

import (
	"image"
	"math"

	"github.com/mgilbir/gowemf"
)

// DrawImage places the Source pixels of an image, through Transform, which
// maps image pixel coordinates (pixel x covers [x, x+1)) onto the raster.
// Pixels outside Source are not drawn. Smooth asks for interpolation (GDI's
// HALFTONE stretch mode, or an EMF+ interpolation mode other than nearest
// neighbor); without it each output pixel takes the source pixel under its
// center, as GDI stretches.
func (be *backend) DrawImage(d gowemf.ImageDraw, clip gowemf.Clip) error {
	if err := be.begin(); err != nil {
		return err
	}
	if d.Image == nil || !(d.Opacity > 0) {
		return nil
	}
	src := d.Source.Intersect(d.Image.Bounds())
	if src.Empty() {
		return nil
	}
	m := fromMatrix(d.Transform)
	if !m.finite() {
		return errCoordinate
	}
	toSrc, ok := m.invert()
	if !ok {
		return nil
	}
	img, err := be.toNRGBA(d.Image, src)
	if err != nil {
		return err
	}
	// From here, source coordinates are relative to src.
	toSrc = toSrc.then(affine{a: 1, d: 1, e: -float64(src.Min.X), f: -float64(src.Min.Y)})
	x0, y0, x1, y1 := float64(src.Min.X), float64(src.Min.Y), float64(src.Max.X), float64(src.Max.Y)
	quad := [][]point{{m.apply(point{x0, y0}), m.apply(point{x1, y0}), m.apply(point{x1, y1}), m.apply(point{x0, y1})}}
	c, err := be.clip(clip)
	if err != nil {
		return err
	}
	opacity := math.Min(1, d.Opacity)
	var sample func(u, v float64) rgba
	if d.Smooth {
		// Reduce a source much larger than its destination by averaging
		// first, as interpolation alone would skip most of its pixels.
		// The factors are at most the image's size.
		rx, ry := math.Hypot(toSrc.a, toSrc.b), math.Hypot(toSrc.c, toSrc.d)
		fx := max(1, int(math.Min(rx, float64(img.Rect.Dx()))))
		fy := max(1, int(math.Min(ry, float64(img.Rect.Dy()))))
		if fx > 1 || fy > 1 {
			if img, err = be.reduce(img, fx, fy); err != nil {
				return err
			}
			toSrc = toSrc.then(affine{a: 1 / float64(fx), d: 1 / float64(fy)})
		}
		sample = bilinear(img)
	} else {
		sample = nearest(img)
	}
	return be.r.fill(quad, false, fillStyle{fn: func(px, py int) rgba {
		q := toSrc.apply(point{float64(px) + 0.5, float64(py) + 0.5})
		col := sample(q.x, q.y)
		col.a = clamp8(float64(col.a) * opacity)
		return col
	}}, c)
}

// nearest samples the pixel under a point, clamped to the image.
func nearest(img *image.NRGBA) func(u, v float64) rgba {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	return func(u, v float64) rgba {
		x := clampInt(int(math.Floor(u)), 0, w-1)
		y := clampInt(int(math.Floor(v)), 0, h-1)
		i := img.PixOffset(x, y)
		return rgba{img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]}
	}
}

// bilinear interpolates the four pixels around a point, in premultiplied
// alpha, clamped to the image's edges.
func bilinear(img *image.NRGBA) func(u, v float64) rgba {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	at := func(x, y int) (r, g, b, a float64) {
		i := img.PixOffset(clampInt(x, 0, w-1), clampInt(y, 0, h-1))
		p := img.Pix[i : i+4 : i+4]
		a = float64(p[3])
		return float64(p[0]) * a / 255, float64(p[1]) * a / 255, float64(p[2]) * a / 255, a
	}
	return func(u, v float64) rgba {
		tx, ty := u-0.5, v-0.5
		fx, fy := math.Floor(tx), math.Floor(ty)
		wx, wy := tx-fx, ty-fy
		xa, ya := int(fx), int(fy)
		r00, g00, b00, a00 := at(xa, ya)
		r10, g10, b10, a10 := at(xa+1, ya)
		r01, g01, b01, a01 := at(xa, ya+1)
		r11, g11, b11, a11 := at(xa+1, ya+1)
		mix := func(c00, c10, c01, c11 float64) float64 {
			return (c00*(1-wx)+c10*wx)*(1-wy) + (c01*(1-wx)+c11*wx)*wy
		}
		a := mix(a00, a10, a01, a11)
		if a <= 0 {
			return rgba{}
		}
		return rgba{clamp8(mix(r00, r10, r01, r11) / a * 255), clamp8(mix(g00, g10, g01, g11) / a * 255), clamp8(mix(b00, b10, b01, b11) / a * 255), clamp8(a)}
	}
}

func clampInt(v, lo, hi int) int { return min(max(v, lo), hi) }

// reduce averages an image in blocks of fx by fy pixels, in premultiplied
// alpha.
func (be *backend) reduce(img *image.NRGBA, fx, fy int) (*image.NRGBA, error) {
	sw, sh := img.Rect.Dx(), img.Rect.Dy()
	w, h := (sw+fx-1)/fx, (sh+fy-1)/fy
	if err := be.b.visit(int64(sw) * int64(sh)); err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		if err := be.ctx.Err(); err != nil {
			return nil, err
		}
		for x := 0; x < w; x++ {
			var sr, sg, sb, sa float64
			n := 0
			for yy := y * fy; yy < min(sh, (y+1)*fy); yy++ {
				for xx := x * fx; xx < min(sw, (x+1)*fx); xx++ {
					i := img.PixOffset(xx, yy)
					a := float64(img.Pix[i+3])
					sr += float64(img.Pix[i]) * a
					sg += float64(img.Pix[i+1]) * a
					sb += float64(img.Pix[i+2]) * a
					sa += a
					n++
				}
			}
			i := out.PixOffset(x, y)
			if sa > 0 {
				out.Pix[i], out.Pix[i+1], out.Pix[i+2] = clamp8(sr/sa), clamp8(sg/sa), clamp8(sb/sa)
			}
			out.Pix[i+3] = clamp8(sa / float64(n))
		}
	}
	return out, nil
}
