package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/gif"
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
	isGIF := bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
	switch {
	case isPNG:
		// A size past the limits is refused as such before the decoder sees
		// it: on 32-bit platforms the decoder refuses one whose pixels int
		// cannot count, which is a limit there, not a corrupt image.
		if w, h, ok := pngDeclaredSize(data); ok {
			if err = checkDeclaredSize(w, h, l); err != nil {
				return nil, err
			}
		}
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
	case isJPEG:
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(data))
	case isGIF:
		cfg, err = gif.DecodeConfig(bytes.NewReader(data))
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
	switch {
	case isPNG:
		img, err = png.Decode(bytes.NewReader(data))
	case isJPEG:
		img, err = jpeg.Decode(bytes.NewReader(data))
	default:
		// An animated GIF shows its first frame, as a static slide does.
		img, err = gif.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("%w: image decode: %w", ErrInvalid, err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return img, nil
}

// pngDeclaredSize reads the width and height a PNG's IHDR chunk, which must
// come first, declares. ok is false when there is no such chunk, or its sizes
// are not the positive 31-bit values the format allows; the decoder then
// refuses the image.
func pngDeclaredSize(data []byte) (w, h int64, ok bool) {
	if len(data) < 24 || string(data[12:16]) != "IHDR" {
		return 0, 0, false
	}
	w, h = int64(binary.BigEndian.Uint32(data[16:20])), int64(binary.BigEndian.Uint32(data[20:24]))
	if w <= 0 || h <= 0 || w > math.MaxInt32 || h > math.MaxInt32 {
		return 0, 0, false
	}
	return w, h, true
}

// checkDeclaredSize applies the dimension and pixel limits to a declared
// size, in 64-bit arithmetic on every platform.
func checkDeclaredSize(w, h int64, l Limits) error {
	if w > int64(l.MaxDimension) || h > int64(l.MaxDimension) || w > l.MaxImagePixels/h {
		return fmt.Errorf("%w: image dimensions", ErrLimit)
	}
	return nil
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
	b, err := p.bitmapOf(ctx, v.Image, true, budget)
	if err != nil {
		return err
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

// bitmapOf is a copy of an image the page owns, made once per source image and
// counted against the image pixel and byte budgets. An image drawn as a picture
// also counts against the image budget's count; one a glyph is drawn from does
// not, as there can be as many of them as distinct glyphs, and the pixels and
// bytes bound them.
func (p *Page) bitmapOf(ctx context.Context, source image.Image, counted bool, budget *prepareBudget) (*bitmap, error) {
	sourceBounds, err := imageBounds(source)
	if err != nil {
		return nil, err
	}
	if err = checkImageSize(sourceBounds.Dx(), sourceBounds.Dy(), p.limits); err != nil {
		return nil, err
	}
	b := budget.images[source]
	if b == nil {
		if counted && budget.pictures >= p.limits.MaxImages {
			return nil, fmt.Errorf("%w: image count", ErrLimit)
		}
		pixels := int64(sourceBounds.Dx()) * int64(sourceBounds.Dy())
		if pixels > p.limits.MaxImagePixels-budget.imagePixels {
			return nil, fmt.Errorf("%w: image pixels", ErrLimit)
		}
		budget.imagePixels += pixels
		dst := image.NewNRGBA(image.Rect(0, 0, sourceBounds.Dx(), sourceBounds.Dy()))
		// Non-premultiplied 8-bit pixels, validated above, copy row by row;
		// other storage converts pixel by pixel.
		nrgba, direct := source.(*image.NRGBA)
		for y := 0; y < dst.Rect.Dy(); y++ {
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			if direct {
				copy(dst.Pix[y*dst.Stride:y*dst.Stride+4*dst.Rect.Dx()], nrgba.Pix[nrgba.PixOffset(sourceBounds.Min.X, sourceBounds.Min.Y+y):])
				continue
			}
			for x := 0; x < dst.Rect.Dx(); x++ {
				if palette, ok := source.(*image.Paletted); ok && int(palette.Pix[y*palette.Stride+x]) >= len(palette.Palette) {
					return nil, fmt.Errorf("%w: palette index", ErrInvalid)
				}
				c := source.At(x+sourceBounds.Min.X, y+sourceBounds.Min.Y)
				r, g, b, a := c.RGBA()
				if r > a || g > a || b > a {
					return nil, fmt.Errorf("%w: premultiplied image color", ErrInvalid)
				}
				dst.SetNRGBA(x, y, color.NRGBAModel.Convert(c).(color.NRGBA))
			}
		}
		var encoded bytes.Buffer
		if err = png.Encode(&outputWriter{ctx: ctx, w: &encoded, remaining: p.limits.MaxImageBytes - budget.imageBytes}, dst); err != nil {
			return nil, err
		}
		budget.imageBytes += int64(encoded.Len())
		b = &bitmap{pixels: dst, png: encoded.Bytes()}
		if budget.images == nil {
			budget.images = map[image.Image]*bitmap{}
		}
		budget.images[source] = b
		if counted {
			budget.pictures++
		}
	}
	return b, nil
}
func (d drawing) pixelColor(x, y int, scale float64) style.RGBA {
	if d.gradient != nil {
		return d.gradient.color((float64(x)+0.5)/scale, (float64(y)+0.5)/scale)
	}
	if d.image == nil {
		return d.rect.color
	}
	// Images are painted through an imageSampler; see paintCommand.color.
	return d.rect.color

}

// imageSampler filters an image drawing for the destination pixels of one
// paint, a row at a time. Each destination pixel's square, in source
// pixels, is filtered: an axis it spans more than one source pixel of
// averages them, weighted by area, and an axis it spans less of
// interpolates linearly between the two nearest at its centre. Channels
// are weighted premultiplied, and edges extend outwards. The filter is
// separable: each source row is filtered across once, into the paint's
// columns, and kept while later destination rows still need it. Clip
// rectangles restrict coverage without changing the source-to-destination
// mapping.
type imageSampler struct {
	src *image.NRGBA
	x0  int
	// Column i weights source columns lo[i] onwards by weights[off[i]:
	// off[i+1]], normalized to sum to one.
	lo, off    []int
	weights    []float64
	fy, startY float64
	h          int
	// across holds source rows first onwards, filtered across,
	// premultiplied RGBA per column, nil until filtered; spare keeps
	// dropped rows' buffers. row is the current destination row,
	// unpremultiplied, and acc its sums.
	across [][]float32
	first  int
	spare  [][]float32
	acc    []float64
	row    []style.RGBA
}

func newImageSampler(d drawing, scale float64, x0, x1 int) *imageSampler {
	box := d.imageBox
	src := d.image.pixels
	w, h := src.Rect.Dx(), src.Rect.Dy()
	fx := float64(w) / ((box.x1 - box.x0) * scale)
	s := &imageSampler{src: src, x0: x0, h: h, fy: float64(h) / ((box.y1 - box.y0) * scale), startY: box.y0 * scale, row: make([]style.RGBA, x1-x0), acc: make([]float64, 4*(x1-x0))}
	s.lo, s.off = make([]int, x1-x0), make([]int, x1-x0+1)
	for i := range s.lo {
		a := imageAxis((float64(x0+i)-box.x0*scale)*fx, fx, w)
		s.lo[i] = a.lo
		sum := 0.0
		for sx := a.lo; sx <= a.hi; sx++ {
			sum += a.weight(sx)
		}
		for sx := a.lo; sx <= a.hi; sx++ {
			wt := 0.0
			if sum > 0 {
				wt = a.weight(sx) / sum
			}
			s.weights = append(s.weights, wt)
		}
		s.off[i+1] = len(s.weights)
	}
	return s
}

// filterAcross filters a source row into the paint's columns.
func (s *imageSampler) filterAcross(sy int) []float32 {
	for sy >= s.first+len(s.across) {
		s.across = append(s.across, nil)
	}
	if r := s.across[sy-s.first]; r != nil {
		return r
	}
	var out []float32
	if n := len(s.spare); n > 0 {
		out, s.spare = s.spare[n-1], s.spare[:n-1]
	} else {
		out = make([]float32, 4*len(s.lo))
	}
	src := s.src.Pix[s.src.PixOffset(s.src.Rect.Min.X, s.src.Rect.Min.Y+sy):]
	for i, lo := range s.lo {
		var r, g, b, al float64
		p := src[4*lo:]
		for _, wt := range s.weights[s.off[i]:s.off[i+1]] {
			alpha := float64(p[3]) * wt
			r, g, b, al = r+float64(p[0])*alpha, g+float64(p[1])*alpha, b+float64(p[2])*alpha, al+alpha
			p = p[4:]
		}
		out[4*i], out[4*i+1], out[4*i+2], out[4*i+3] = float32(r/255), float32(g/255), float32(b/255), float32(al/255)
	}
	s.across[sy-s.first] = out
	return out
}

// beginRow filters destination row y, dropping source rows above it.
func (s *imageSampler) beginRow(y int) {
	a := imageAxis((float64(y)-s.startY)*s.fy, s.fy, s.h)
	// Destination rows run downwards, so source rows above this one's are
	// not needed again.
	for len(s.across) > 0 && s.first < a.lo {
		if r := s.across[0]; r != nil {
			s.spare = append(s.spare, r)
		}
		s.across, s.first = s.across[1:], s.first+1
	}
	if len(s.across) == 0 {
		s.first = a.lo
	}
	clear(s.acc)
	total := 0.0
	for sy := a.lo; sy <= a.hi; sy++ {
		wy := a.weight(sy)
		if wy == 0 {
			continue
		}
		total += wy
		r := s.filterAcross(sy)
		for i, v := range r {
			s.acc[i] += wy * float64(v)
		}
	}
	for i := range s.row {
		al := s.acc[4*i+3] / total
		if total <= 0 || al <= 0 {
			s.row[i] = style.RGBA{}
			continue
		}
		s.row[i] = style.RGBA{R: s.acc[4*i] / total / al, G: s.acc[4*i+1] / total / al, B: s.acc[4*i+2] / total / al, A: al}
	}
}

// imageFilterAxis is one axis of a destination pixel's footprint in a
// source image n pixels long: the source pixels lo to hi it weights.
type imageFilterAxis struct {
	lo, hi int
	// box is set for an area average over [a, b); otherwise the pixels lo
	// and hi interpolate with weights 1-t and t.
	box  bool
	a, b float64
	t    float64
	n    int
}

// imageAxis is the axis filter for a footprint starting at start and size
// source pixels long.
func imageAxis(start, size float64, n int) imageFilterAxis {
	clamp := func(i int) int { return max(0, min(n-1, i)) }
	if size > 1 {
		a, b := math.Max(0, start), math.Min(float64(n), start+size)
		if b <= a {
			// The footprint lies past an edge: the edge pixel.
			i := clamp(int(math.Floor(start)))
			return imageFilterAxis{lo: i, hi: i, t: 0, n: n}
		}
		return imageFilterAxis{lo: clamp(int(math.Floor(a))), hi: clamp(int(math.Ceil(b)) - 1), box: true, a: a, b: b, n: n}
	}
	c := start + size/2 - 0.5
	i := int(math.Floor(c))
	t := c - float64(i)
	lo, hi := clamp(i), clamp(i+1)
	if lo == hi {
		t = 0
	}
	return imageFilterAxis{lo: lo, hi: hi, t: t, n: n}
}

func (f imageFilterAxis) weight(i int) float64 {
	if f.box {
		return math.Max(0, math.Min(f.b, float64(i+1))-math.Max(f.a, float64(i)))
	}
	switch {
	case f.lo == f.hi:
		return 1
	case i == f.lo:
		return 1 - f.t
	case i == f.hi:
		return f.t
	}
	return 0

}
func (p *Page) svgImage(e *xml.Encoder, d drawing, scale float64) error {
	// Bound the base64 allocation before it happens; the final output writer
	// independently enforces the complete document's serialized limit.
	if int64(len(d.image.png)) > (p.limits.MaxOutputBytes-32)/4*3 {
		return fmt.Errorf("%w: SVG image bytes", ErrLimit)
	}
	attrs := rectAttrs(d.imageBox, scale)
	attrs = append(attrs, attr("href", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(d.image.png)), attr("preserveAspectRatio", "none"))
	if !d.smooth {
		attrs = append(attrs, attr("image-rendering", "pixelated"))
	}
	return svgElement(e, "image", attrs)
}
