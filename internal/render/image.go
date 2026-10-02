package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
)

type bitmap struct {
	pixels *image.NRGBA
	png    []byte
}

// DecodeImage accepts PNG and JPEG bytes with preventive dimensions and encoded
// size checks. It does not load URLs or apply EXIF orientation. Standard-library
// decoders cannot be interrupted inside Decode; dimensions and bytes bound work.
// The returned image remains caller-owned until Prepare copies it.
func DecodeImage(ctx context.Context, data []byte, limits Limits) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l, err := limits.resolved()
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > l.MaxImageBytes {
		return nil, fmt.Errorf("%w: image bytes", ErrLimit)
	}
	var cfg image.Config
	isPNG := bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	isJPEG := bytes.HasPrefix(data, []byte{0xff, 0xd8})
	switch {
	case isPNG:
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
	case isJPEG:
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(data))
	default:
		return nil, fmt.Errorf("%w: image encoding", ErrUnsupported)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: image header: %w", ErrInvalid, err)
	}
	if err = checkImageSize(cfg.Width, cfg.Height, l); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	var img image.Image
	if isPNG {
		img, err = png.Decode(bytes.NewReader(data))
	} else {
		img, err = jpeg.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("%w: image decode: %w", ErrInvalid, err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return img, nil
}
func checkImageSize(w, h int, l Limits) error {
	maxInt := int(^uint(0) >> 1)
	if w <= 0 || h <= 0 {
		return fmt.Errorf("%w: empty image", ErrInvalid)
	}
	// JPEG progressive coefficient buffers and PNG intermediate storage cost
	// more than the final four-byte pixels. Keep address-space headroom too.
	if w > l.MaxDimension || h > l.MaxDimension || int64(w) > l.MaxImagePixels/int64(h) || w > (maxInt/32)/h {
		return fmt.Errorf("%w: image dimensions", ErrLimit)
	}
	return nil
}
func imageBounds(source image.Image) (image.Rectangle, error) {
	// Do not call arbitrary Image methods. Only standard-library storage types
	// with validated buffers may cross the snapshot boundary.
	var r image.Rectangle
	var pix []byte
	var stride, bpp int
	switch v := source.(type) {
	case *image.NRGBA:
		if v == nil {
			break
		}
		r, pix, stride, bpp = v.Rect, v.Pix, v.Stride, 4
	case *image.RGBA:
		if v == nil {
			break
		}
		r, pix, stride, bpp = v.Rect, v.Pix, v.Stride, 4
	case *image.NRGBA64:
		if v == nil {
			break
		}
		r, pix, stride, bpp = v.Rect, v.Pix, v.Stride, 8
	case *image.RGBA64:
		if v == nil {
			break
		}
		r, pix, stride, bpp = v.Rect, v.Pix, v.Stride, 8
	case *image.Gray:
		if v == nil {
			break
		}
		r, pix, stride, bpp = v.Rect, v.Pix, v.Stride, 1
	case *image.Gray16:
		if v == nil {
			break
		}
		r, pix, stride, bpp = v.Rect, v.Pix, v.Stride, 2
	case *image.CMYK:
		if v == nil {
			break
		}
		r, pix, stride, bpp = v.Rect, v.Pix, v.Stride, 4
	case *image.Paletted:
		if v == nil {
			break
		}
		r, pix, stride, bpp = v.Rect, v.Pix, v.Stride, 1
		if len(v.Palette) == 0 || len(v.Palette) > 256 {
			return r, fmt.Errorf("%w: palette", ErrInvalid)
		}
		for _, c := range v.Palette {
			switch c.(type) {
			case color.RGBA, color.NRGBA, color.RGBA64, color.NRGBA64, color.Gray, color.Gray16, color.CMYK:
			default:
				return r, fmt.Errorf("%w: palette color", ErrUnsupported)
			}
		}
	case *image.YCbCr:
		if v == nil {
			break
		}
		r = v.Rect
		if r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > 1<<20 || r.Max.Y > 1<<20 {
			return r, fmt.Errorf("%w: YCbCr origin", ErrInvalid)
		}
		switch v.SubsampleRatio {
		case image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422, image.YCbCrSubsampleRatio420, image.YCbCrSubsampleRatio440, image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410:
		default:
			return r, fmt.Errorf("%w: subsampling", ErrInvalid)
		}
		if !validBuffer(v.Y, v.YStride, r.Dx(), r.Dy(), 1) || v.CStride <= 0 || v.CStride > int(^uint(0)>>1)/max(1, r.Dy()) {
			return r, fmt.Errorf("%w: image buffer", ErrInvalid)
		}
		if r.Empty() {
			return r, fmt.Errorf("%w: image extent", ErrInvalid)
		}
		if v.COffset(r.Max.X-1, r.Min.Y)+1 > v.CStride {
			return r, fmt.Errorf("%w: chroma stride", ErrInvalid)
		}
		i := v.COffset(r.Max.X-1, r.Max.Y-1)
		if i < 0 || i >= len(v.Cb) || i >= len(v.Cr) {
			return r, fmt.Errorf("%w: chroma buffer", ErrInvalid)
		}
		return r, nil
	default:
		return r, fmt.Errorf("%w: image storage %T", ErrUnsupported, source)
	}
	if r.Min.X < -(1<<20) || r.Min.Y < -(1<<20) || r.Max.X > 1<<20 || r.Max.Y > 1<<20 || r.Min.X > 1<<20 || r.Min.Y > 1<<20 || r.Max.X < -(1<<20) || r.Max.Y < -(1<<20) || !validBuffer(pix, stride, r.Dx(), r.Dy(), bpp) {
		return r, fmt.Errorf("%w: image buffer", ErrInvalid)
	}
	return r, nil
}
func validBuffer(pix []byte, stride, w, h, bpp int) bool {
	if w <= 0 || h <= 0 || bpp <= 0 || w > len(pix)/bpp {
		return false
	}
	row := w * bpp
	if stride < row || row > len(pix) {
		return false
	}
	return h-1 <= (len(pix)-row)/stride
}
func (p *Page) collectImage(ctx context.Context, v layout.DrawImage, clips []*geometry, budget *prepareBudget) error {
	if v.Rect.W <= 0 || v.Rect.H <= 0 {
		return fmt.Errorf("%w: image destination", ErrInvalid)
	}
	sourceBounds, err := imageBounds(v.Image)
	if err != nil {
		return err
	}
	if err = checkImageSize(sourceBounds.Dx(), sourceBounds.Dy(), p.limits); err != nil {
		return err
	}
	b := budget.images[v.Image]
	if b == nil {
		if len(budget.images) >= p.limits.MaxImages {
			return fmt.Errorf("%w: image count", ErrLimit)
		}
		pixels := int64(sourceBounds.Dx()) * int64(sourceBounds.Dy())
		if pixels > p.limits.MaxImagePixels-budget.imagePixels {
			return fmt.Errorf("%w: image pixels", ErrLimit)
		}
		budget.imagePixels += pixels
		dst := image.NewNRGBA(image.Rect(0, 0, sourceBounds.Dx(), sourceBounds.Dy()))
		for y := 0; y < dst.Rect.Dy(); y++ {
			if err = ctx.Err(); err != nil {
				return err
			}
			for x := 0; x < dst.Rect.Dx(); x++ {
				if palette, ok := v.Image.(*image.Paletted); ok && int(palette.Pix[y*palette.Stride+x]) >= len(palette.Palette) {
					return fmt.Errorf("%w: palette index", ErrInvalid)
				}
				c := v.Image.At(x+sourceBounds.Min.X, y+sourceBounds.Min.Y)
				r, g, b, a := c.RGBA()
				if r > a || g > a || b > a {
					return fmt.Errorf("%w: premultiplied image color", ErrInvalid)
				}
				dst.SetNRGBA(x, y, color.NRGBAModel.Convert(c).(color.NRGBA))
			}
		}
		var encoded bytes.Buffer
		if err = png.Encode(&outputWriter{ctx: ctx, w: &encoded, remaining: p.limits.MaxImageBytes - budget.imageBytes}, dst); err != nil {
			return err
		}
		budget.imageBytes += int64(encoded.Len())
		b = &bitmap{pixels: dst, png: encoded.Bytes()}
		if budget.images == nil {
			budget.images = map[image.Image]*bitmap{}
		}
		budget.images[v.Image] = b
	}
	box := rectangle{v.Rect.X.Px(), v.Rect.Y.Px(), v.Rect.X.Px() + v.Rect.W.Px(), v.Rect.Y.Px() + v.Rect.H.Px(), style.RGBA{A: 1}}
	r := meet(box, rectangle{0, 0, p.width, p.height, style.RGBA{}})
	if v.Clip.Active {
		c := v.Clip.Rect
		if c.W < 0 || c.H < 0 {
			return fmt.Errorf("%w: image clip", ErrInvalid)
		}
		r = meet(r, rectangle{c.X.Px(), c.Y.Px(), c.X.Px() + c.W.Px(), c.Y.Px() + c.H.Px(), style.RGBA{}})
	}
	p.draws = append(p.draws, drawing{rect: r, clips: clips, image: b, imageBox: box})
	return nil
}
func (d drawing) pixelColor(x, y int, scale float64) style.RGBA {
	if d.gradient != nil {
		return d.gradient.color((float64(x)+0.5)/scale, (float64(y)+0.5)/scale)
	}
	if d.image == nil {
		return d.rect.color
	}
	// Nearest-neighbour sample at the destination pixel centre. Clip rectangles
	// restrict coverage without changing the source-to-destination mapping.
	box := d.imageBox
	src := d.image.pixels
	sx := int(math.Floor(((float64(x)+0.5)/scale - box.x0) / (box.x1 - box.x0) * float64(src.Rect.Dx())))
	sy := int(math.Floor(((float64(y)+0.5)/scale - box.y0) / (box.y1 - box.y0) * float64(src.Rect.Dy())))
	c := src.NRGBAAt(max(0, min(src.Rect.Dx()-1, sx)), max(0, min(src.Rect.Dy()-1, sy)))
	return style.RGBA{R: float64(c.R), G: float64(c.G), B: float64(c.B), A: float64(c.A) / 255}
}
func (p *Page) svgImage(e *xml.Encoder, d drawing, scale float64) error {
	// Bound the base64 allocation before it happens; the final output writer
	// independently enforces the complete document's serialized limit.
	if int64(len(d.image.png)) > (p.limits.MaxOutputBytes-32)/4*3 {
		return fmt.Errorf("%w: SVG image bytes", ErrLimit)
	}
	attrs := rectAttrs(d.imageBox, scale)
	attrs = append(attrs, attr("href", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(d.image.png)), attr("preserveAspectRatio", "none"), attr("image-rendering", "pixelated"))
	return svgElement(e, "image", attrs)
}
