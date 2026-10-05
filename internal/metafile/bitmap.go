package metafile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

// Raster operation codes (ROP3) that need no source, or only copy it.
const (
	ropSrcCopy    = 0x00cc0020
	ropNotSrcCopy = 0x00330008
	ropPatCopy    = 0x00f00021
	ropBlackness  = 0x00000042
	ropWhiteness  = 0x00ff0062
)

// invert returns the inverse of the transform.
func (m affine) invert() (affine, bool) {
	det := m.a*m.d - m.b*m.c
	if det == 0 || !finite(det) || math.Abs(det) < 1e-18 {
		return affine{}, false
	}
	return affine{
		a: m.d / det, b: -m.b / det, c: -m.c / det, d: m.a / det,
		e: (m.c*m.f - m.d*m.e) / det, f: (m.b*m.e - m.a*m.f) / det,
	}, true
}

// bitmapErr maps gowemf's bitmap errors: a limit stays one, an encoding the
// library does not decode is left out, and anything else is invalid.
func (it *interp) bitmapErr(err error, what string) error {
	switch {
	case errors.Is(err, gowemf.ErrLimit):
		return fmt.Errorf("%w: metafile: %s: %w", render.ErrLimit, what, err)
	case errors.Is(err, gowemf.ErrUnsupported):
		return it.soft("%s left out: %v", what, err)
	}
	return fmt.Errorf("%w: metafile: %s: %w", render.ErrInvalid, what, err)
}

// dibBottomUp reports whether a DIB's rows run from the bottom, as the sign of
// its height tells.
func dibBottomUp(info []byte) bool {
	if len(info) >= 16 {
		switch binary.LittleEndian.Uint32(info) {
		case 12:
			return true
		default:
			return int32(binary.LittleEndian.Uint32(info[8:])) > 0
		}
	}
	return true
}

// decode turns DIB data into premultiplied pixels.
func (it *interp) decodeDIB(info, bits []byte, packed []byte, usage uint32, alpha bool) (*image.RGBA, bool, error) {
	remaining := it.b.maxBitmapPixels - it.b.bitmapPixels
	if remaining <= 0 {
		return nil, false, fmt.Errorf("%w: metafile bitmap pixels", render.ErrLimit)
	}
	limits := gowemf.ImageLimits{MaxBytes: uint64(len(info) + len(bits) + len(packed) + 1), MaxPixels: uint64(remaining)}
	var (
		d   *gowemf.DIB
		err error
	)
	if packed != nil {
		d, err = gowemf.ParsePackedDIB(packed, usage, nil, limits)
		if err == nil {
			info = packed
		}
	} else {
		d, err = gowemf.ParseDIB(info, bits, usage, nil, limits)
	}
	if err != nil {
		return nil, false, it.bitmapErr(err, "bitmap")
	}
	w, h := d.Width(), d.Height()
	if w <= 0 || h <= 0 || int64(w)*int64(h) > remaining {
		return nil, false, fmt.Errorf("%w: metafile bitmap pixels", render.ErrLimit)
	}
	it.b.bitmapPixels += int64(w) * int64(h)
	var src image.Image
	if alpha {
		src, err = d.AlphaImage()
	} else {
		src, err = d.Image()
	}
	if err != nil {
		return nil, false, it.bitmapErr(err, "bitmap")
	}
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	if r, ok := src.(*image.RGBA); ok && alpha {
		copy(out.Pix, r.Pix)
	} else {
		for y := 0; y < b.Dy(); y++ {
			if y%64 == 0 {
				if err = it.ctx.Err(); err != nil {
					return nil, false, err
				}
			}
			for x := 0; x < b.Dx(); x++ {
				r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
				i := out.PixOffset(x, y)
				out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)
			}
		}
	}
	return out, dibBottomUp(info), nil
}

// bitmapDraw describes one transfer of a source rectangle to a destination
// rectangle, in logical units.
type bitmapDraw struct {
	img            *image.RGBA
	bottomUp       bool
	sx, sy, sw, sh float64 // source rectangle, y from the bottom for a bottom-up DIB
	dx, dy, dw, dh float64
	alpha          float64 // constant alpha, 0..1
	invert         bool
	nearest        bool
}

func (it *interp) drawBitmap(bd bitmapDraw) error {
	if err := it.b.op(); err != nil {
		return err
	}
	if bd.sw == 0 || bd.sh == 0 || bd.dw == 0 || bd.dh == 0 {
		return nil
	}
	iw, ih := float64(bd.img.Bounds().Dx()), float64(bd.img.Bounds().Dy())
	sy := bd.sy
	if bd.bottomUp {
		// The rectangle's origin is the bottom-left; make it top-down.
		sy = ih - (bd.sy + bd.sh)
		if bd.sh < 0 {
			return it.soft("bitmap with a negative source height left out")
		}
	}
	sx := bd.sx
	if bd.sw < 0 || bd.sh < 0 {
		return it.soft("bitmap with a negative source extent left out")
	}
	if sx < 0 || sy < 0 || sx+bd.sw > iw+1e-9 || sy+bd.sh > ih+1e-9 {
		// The part of the source rectangle outside the bitmap is not drawn.
		return it.soft("bitmap source rectangle past the bitmap left out")
	}
	// Source pixel of a logical point.
	m := it.matrix()
	pix := affine{a: it.r.sx, d: it.r.sy, e: it.r.ox, f: it.r.oy}
	toPixel := m.then(pix)
	inv, ok := toPixel.invert()
	if !ok {
		return nil
	}
	toSrc := inv.then(affine{
		a: bd.sw / bd.dw, d: bd.sh / bd.dh,
		e: sx - bd.dx*bd.sw/bd.dw, f: sy - bd.dy*bd.sh/bd.dh,
	})
	// Reduce a source much larger than its destination, by averaging, first.
	rx, ry := math.Hypot(toSrc.a, toSrc.b), math.Hypot(toSrc.c, toSrc.d)
	fx, fy := max(1, int(rx)), max(1, int(ry))
	img := bd.img
	srcRect := image.Rect(int(math.Floor(sx)), int(math.Floor(sy)), int(math.Ceil(sx+bd.sw)), int(math.Ceil(sy+bd.sh))).Intersect(img.Bounds())
	if srcRect.Empty() {
		return nil
	}
	if err := it.b.visit(int64(srcRect.Dx()) * int64(srcRect.Dy())); err != nil {
		return err
	}
	if fx > 1 || fy > 1 {
		var err error
		img, err = reduce(it, img, srcRect, fx, fy)
		if err != nil {
			return err
		}
		toSrc = toSrc.then(affine{a: 1 / float64(fx), d: 1 / float64(fy), e: -float64(srcRect.Min.X) / float64(fx), f: -float64(srcRect.Min.Y) / float64(fy)})
		srcRect = img.Bounds()
	}
	// Edge clamp stays inside the source rectangle.
	minX, minY := float64(srcRect.Min.X), float64(srcRect.Min.Y)
	maxX, maxY := float64(srcRect.Max.X), float64(srcRect.Max.Y)
	alpha := bd.alpha
	paint := func(px, py int) rgba {
		fxp, fyp := float64(px)+0.5, float64(py)+0.5
		u := toSrc.a*fxp + toSrc.c*fyp + toSrc.e
		v := toSrc.b*fxp + toSrc.d*fyp + toSrc.f
		var r, g, b, a float64
		if bd.nearest {
			xi, yi := int(math.Floor(math.Min(math.Max(u, minX), maxX-1e-9))), int(math.Floor(math.Min(math.Max(v, minY), maxY-1e-9)))
			r, g, b, a = pixelAt(img, xi, yi)
		} else {
			tx, ty := u-0.5, v-0.5
			x0, y0 := math.Floor(tx), math.Floor(ty)
			wx, wy := tx-x0, ty-y0
			xa, xb := clampInt(int(x0), srcRect.Min.X, srcRect.Max.X-1), clampInt(int(x0)+1, srcRect.Min.X, srcRect.Max.X-1)
			ya, yb := clampInt(int(y0), srcRect.Min.Y, srcRect.Max.Y-1), clampInt(int(y0)+1, srcRect.Min.Y, srcRect.Max.Y-1)
			r00, g00, b00, a00 := pixelAt(img, xa, ya)
			r10, g10, b10, a10 := pixelAt(img, xb, ya)
			r01, g01, b01, a01 := pixelAt(img, xa, yb)
			r11, g11, b11, a11 := pixelAt(img, xb, yb)
			mix := func(c00, c10, c01, c11 float64) float64 {
				return (c00*(1-wx)+c10*wx)*(1-wy) + (c01*(1-wx)+c11*wx)*wy
			}
			r, g, b, a = mix(r00, r10, r01, r11), mix(g00, g10, g01, g11), mix(b00, b10, b01, b11), mix(a00, a10, a01, a11)
		}
		if a <= 0 {
			return rgba{}
		}
		// r, g, b are premultiplied here.
		out := rgba{clamp8(r / a * 255), clamp8(g / a * 255), clamp8(b / a * 255), clamp8(a * alpha)}
		if bd.invert {
			out.r, out.g, out.b = 255-out.r, 255-out.g, 255-out.b
		}
		return out
	}
	quad := [][]point{{
		m.apply(point{bd.dx, bd.dy}), m.apply(point{bd.dx + bd.dw, bd.dy}),
		m.apply(point{bd.dx + bd.dw, bd.dy + bd.dh}), m.apply(point{bd.dx, bd.dy + bd.dh}),
	}}
	return it.r.fill(quad, false, fillStyle{fn: paint}, it.dc.clip)
}

func clampInt(v, lo, hi int) int { return min(max(v, lo), hi) }

func clamp8(v float64) uint8 { return uint8(math.Min(255, math.Max(0, v+0.5))) }

// pixelAt reads a premultiplied pixel as floats of 0..255.
func pixelAt(img *image.RGBA, x, y int) (r, g, b, a float64) {
	i := img.PixOffset(x, y)
	p := img.Pix[i : i+4 : i+4]
	return float64(p[0]), float64(p[1]), float64(p[2]), float64(p[3])
}

// reduce averages the pixels of a rectangle of img in blocks of fx by fy.
func reduce(it *interp, img *image.RGBA, rect image.Rectangle, fx, fy int) (*image.RGBA, error) {
	w, h := (rect.Dx()+fx-1)/fx, (rect.Dy()+fy-1)/fy
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		if err := it.ctx.Err(); err != nil {
			return nil, err
		}
		for x := 0; x < w; x++ {
			var sr, sg, sb, sa, n int
			for yy := rect.Min.Y + y*fy; yy < min(rect.Max.Y, rect.Min.Y+(y+1)*fy); yy++ {
				for xx := rect.Min.X + x*fx; xx < min(rect.Max.X, rect.Min.X+(x+1)*fx); xx++ {
					i := img.PixOffset(xx, yy)
					sr += int(img.Pix[i])
					sg += int(img.Pix[i+1])
					sb += int(img.Pix[i+2])
					sa += int(img.Pix[i+3])
					n++
				}
			}
			i := out.PixOffset(x, y)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = uint8(sr/n), uint8(sg/n), uint8(sb/n), uint8(sa/n)
		}
	}
	return out, nil
}

// patBlt fills a destination rectangle by a raster operation that needs no
// source.
func (it *interp) patBlt(rop uint32, x, y, w, h float64) error {
	var col rgba
	switch rop {
	case ropPatCopy:
		br := it.dc.brush
		if br == nil || br.null {
			return nil
		}
		if br.unsupported != "" {
			return it.soft("%s fill left out", br.unsupported)
		}
		c, err := it.color(br.color)
		if err != nil {
			return err
		}
		col = c
	case ropBlackness:
		col = rgba{0, 0, 0, 255}
	case ropWhiteness:
		col = rgba{255, 255, 255, 255}
	default:
		return it.soft("raster operation %#x left out", rop)
	}
	if err := it.b.op(); err != nil {
		return err
	}
	return it.r.fill([][]point{it.rectContour(x, y, x+w, y+h)}, false, fillStyle{solid: col}, it.dc.clip)
}

func needsSource(rop uint32) bool {
	switch rop {
	case ropPatCopy, ropBlackness, ropWhiteness, 0x00550009, 0x005a0049, 0x000f0001:
		return false
	}
	return true
}

func (it *interp) copyROP(rop uint32) (invert bool, err error) {
	switch rop {
	case ropSrcCopy:
		return false, nil
	case ropNotSrcCopy:
		return true, nil
	}
	return false, it.soft("raster operation %#x left out", rop)
}

// emfBitmap plays SetDIBitsToDevice and StretchDIBits.
func (it *interp) emfBitmap(t uint32, v gowemf.BitmapTransfer) error {
	if len(v.Info) == 0 || len(v.Bits) == 0 {
		return nil
	}
	if v.Usage != 0 {
		return it.soft("bitmap with a palette color table left out")
	}
	bd := bitmapDraw{alpha: 1, sx: v.Source.X, sy: v.Source.Y, sw: v.SourceSize.X, sh: v.SourceSize.Y, dx: v.Destination.X, dy: v.Destination.Y}
	if t == gowemf.EMRSetDIBitsToDevice {
		bd.dw, bd.dh = v.SourceSize.X, v.SourceSize.Y
	} else {
		bd.dw, bd.dh = v.DestinationSize.X, v.DestinationSize.Y
		inv, err := it.copyROP(v.RasterOperation)
		if err != nil || (!inv && v.RasterOperation != ropSrcCopy) {
			return err
		}
		bd.invert = inv
	}
	img, bottomUp, err := it.decodeDIB(v.Info, v.Bits, nil, v.Usage, false)
	if err != nil || img == nil {
		return err
	}
	if t == gowemf.EMRSetDIBitsToDevice && (v.StartScan != 0 || int(v.Scans) != img.Bounds().Dy()) {
		return it.soft("bitmap transferred in part left out")
	}
	bd.img, bd.bottomUp = img, bottomUp
	return it.drawBitmap(bd)
}

// emfRaster plays BitBlt, StretchBlt, AlphaBlend and TransparentBlt.
func (it *interp) emfRaster(t uint32, v gowemf.RasterTransfer) error {
	m := v.SourceTransform
	if len(v.Info) != 0 && (m.M11 != 1 || m.M22 != 1 || m.M12 != 0 || m.M21 != 0 || m.Dx != 0 || m.Dy != 0) {
		return it.soft("bitmap with a source transform left out")
	}
	dx, dy, dw, dh := v.Destination.X, v.Destination.Y, v.DestinationSize.X, v.DestinationSize.Y
	rop := v.Operation
	if t == gowemf.EMRBitBlt || t == gowemf.EMRStretchBlt {
		if !needsSource(rop) || len(v.Info) == 0 {
			if len(v.Info) == 0 && needsSource(rop) {
				return it.soft("bitmap transfer without a bitmap left out")
			}
			return it.patBlt(rop, dx, dy, dw, dh)
		}
	}
	if len(v.Info) == 0 || len(v.Bits) == 0 {
		return nil
	}
	if v.Usage != 0 {
		return it.soft("bitmap with a palette color table left out")
	}
	bd := bitmapDraw{alpha: 1, sx: v.Source.X, sy: v.Source.Y, sw: v.SourceSize.X, sh: v.SourceSize.Y, dx: dx, dy: dy, dw: dw, dh: dh}
	switch t {
	case gowemf.EMRBitBlt, gowemf.EMRStretchBlt:
		inv, err := it.copyROP(rop)
		if err != nil || (!inv && rop != ropSrcCopy) {
			return err
		}
		bd.invert = inv
		img, up, err := it.decodeDIB(v.Info, v.Bits, nil, v.Usage, false)
		if err != nil || img == nil {
			return err
		}
		bd.img, bd.bottomUp = img, up
	case gowemf.EMRAlphaBlend:
		if byte(rop) != 0 {
			return it.soft("blend function other than source over left out")
		}
		bd.alpha = float64(byte(rop>>16)) / 255
		perPixel := byte(rop>>24) == 1
		img, up, err := it.decodeDIB(v.Info, v.Bits, nil, v.Usage, perPixel)
		if err != nil || img == nil {
			return err
		}
		bd.img, bd.bottomUp = img, up
	case gowemf.EMRTransparentBlt:
		img, up, err := it.decodeDIB(v.Info, v.Bits, nil, v.Usage, false)
		if err != nil || img == nil {
			return err
		}
		key, err := it.color(v.Operation)
		if err != nil {
			return err
		}
		for i := 0; i+3 < len(img.Pix); i += 4 {
			if img.Pix[i] == key.r && img.Pix[i+1] == key.g && img.Pix[i+2] == key.b {
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 0, 0, 0
			}
		}
		bd.img, bd.bottomUp, bd.nearest = img, up, true
	}
	return it.drawBitmap(bd)
}

// wmfBitmap plays WMF's bitmap transfers.
func (it *interp) wmfBitmap(t uint32, v gowemf.PackedDIBTransfer) error {
	switch t {
	case 0x1d:
		return it.patBlt(v.RasterOperation, v.Destination.X, v.Destination.Y, v.DestinationSize.X, v.DestinationSize.Y)
	case 0x40, 0x41:
		if v.DeviceSource {
			return it.patBlt(v.RasterOperation, v.Destination.X, v.Destination.Y, v.DestinationSize.X, v.DestinationSize.Y)
		}
	}
	if len(v.DIB) == 0 {
		return nil
	}
	if v.Usage != 0 {
		return it.soft("bitmap with a palette color table left out")
	}
	bd := bitmapDraw{alpha: 1, sx: v.Source.X, sy: v.Source.Y, sw: v.SourceSize.X, sh: v.SourceSize.Y,
		dx: v.Destination.X, dy: v.Destination.Y, dw: v.DestinationSize.X, dh: v.DestinationSize.Y}
	if t != 0x33 {
		inv, err := it.copyROP(v.RasterOperation)
		if err != nil || (!inv && v.RasterOperation != ropSrcCopy) {
			return err
		}
		bd.invert = inv
	}
	img, up, err := it.decodeDIB(nil, nil, v.DIB, v.Usage, false)
	if err != nil || img == nil {
		return err
	}
	if t == 0x33 && (v.StartScan != 0 || int(v.Scans) != img.Bounds().Dy()) {
		return it.soft("bitmap transferred in part left out")
	}
	bd.img, bd.bottomUp = img, up
	return it.drawBitmap(bd)
}
