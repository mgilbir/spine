package render

import (
	"image"
	"image/color"
)

// RowReader reads an image's rows as premultiplied 16-bit RGBA, four
// values a pixel, exactly as image.Image's At(...).RGBA() returns them. The
// image types decoders produce are read directly; others through At.
func RowReader(img image.Image) func(y int, dst []uint32) {
	b := img.Bounds()
	w := b.Dx()
	switch src := img.(type) {
	case *image.NRGBA:
		return func(y int, dst []uint32) {
			p := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):]
			for x := 0; x < w; x++ {
				r, g, bl, a := uint32(p[4*x]), uint32(p[4*x+1]), uint32(p[4*x+2]), uint32(p[4*x+3])
				// color.NRGBA.RGBA.
				r |= r << 8
				r *= a
				r /= 0xff
				g |= g << 8
				g *= a
				g /= 0xff
				bl |= bl << 8
				bl *= a
				bl /= 0xff
				a |= a << 8
				dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = r, g, bl, a
			}
		}
	case *image.RGBA:
		return func(y int, dst []uint32) {
			p := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):]
			for x := 0; x < 4*w; x++ {
				v := uint32(p[x])
				dst[x] = v | v<<8
			}
		}
	case *image.YCbCr:
		return func(y int, dst []uint32) {
			for x := 0; x < w; x++ {
				yi, ci := src.YOffset(b.Min.X+x, b.Min.Y+y), src.COffset(b.Min.X+x, b.Min.Y+y)
				r, g, bl, a := color.YCbCr{Y: src.Y[yi], Cb: src.Cb[ci], Cr: src.Cr[ci]}.RGBA()
				dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = r, g, bl, a
			}
		}
	case *image.Gray:
		return func(y int, dst []uint32) {
			p := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):]
			for x := 0; x < w; x++ {
				v := uint32(p[x])
				v |= v << 8
				dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = v, v, v, 0xffff
			}
		}
	case *image.Paletted:
		table := make([][4]uint32, len(src.Palette))
		for i, c := range src.Palette {
			r, g, bl, a := c.RGBA()
			table[i] = [4]uint32{r, g, bl, a}
		}
		return func(y int, dst []uint32) {
			p := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):]
			for x := 0; x < w; x++ {
				// An index past the palette, which decoders reject, reads as
				// transparent black.
				var c [4]uint32
				if int(p[x]) < len(table) {
					c = table[p[x]]
				}
				copy(dst[4*x:4*x+4], c[:])
			}
		}
	}
	return func(y int, dst []uint32) {
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = r, g, bl, a
		}
	}
}

// Unpremultiply stores a premultiplied 16-bit pixel as NRGBA, as
// color.NRGBAModel converts it.
func Unpremultiply(p []uint8, r, g, b, a uint32) {
	switch a {
	case 0xffff:
		p[0], p[1], p[2], p[3] = uint8(r>>8), uint8(g>>8), uint8(b>>8), 0xff
	case 0:
		p[0], p[1], p[2], p[3] = 0, 0, 0, 0
	default:
		r, g, b = (r*0xffff)/a, (g*0xffff)/a, (b*0xffff)/a
		p[0], p[1], p[2], p[3] = uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
	}
}

// NRGBA copies an image into a new NRGBA image at the origin.
func NRGBA(img image.Image) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	if src, ok := img.(*image.NRGBA); ok {
		// Copied as is: premultiplying would round translucent pixels.
		for y := 0; y < h; y++ {
			copy(out.Pix[y*out.Stride:y*out.Stride+4*w], src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):])
		}
		return out
	}
	read := RowReader(img)
	row := make([]uint32, 4*w)
	for y := 0; y < h; y++ {
		read(y, row)
		p := out.Pix[y*out.Stride:]
		for x := 0; x < w; x++ {
			Unpremultiply(p[4*x:4*x+4], row[4*x], row[4*x+1], row[4*x+2], row[4*x+3])
		}
	}
	return out
}
