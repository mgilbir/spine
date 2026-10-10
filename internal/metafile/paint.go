package metafile

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

// hatchRows are GDI's hatch patterns, HS_HORIZONTAL (0) through HS_DIAGCROSS
// (5): eight rows of eight pixels, the most significant bit leftmost, set
// where the hatch color is drawn.
var hatchRows = [6][8]uint8{
	{0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00},
	{0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08},
	{0x80, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02, 0x01},
	{0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80},
	{0x08, 0x08, 0x08, 0xff, 0x08, 0x08, 0x08, 0x08},
	{0x81, 0x42, 0x24, 0x18, 0x18, 0x24, 0x42, 0x81},
}

func rgbaOf(c color.NRGBA) rgba { return rgba{c.R, c.G, c.B, c.A} }

// fillStyle resolves a paint into a fill style. ok is false when the paint
// draws nothing.
func (be *backend) fillStyle(p gowemf.Paint) (fillStyle, bool, error) {
	switch p.Kind {
	case gowemf.PaintSolid:
		return fillStyle{solid: rgbaOf(p.Color)}, p.Color.A != 0, nil
	case gowemf.PaintHatch:
		return be.hatch(p)
	case gowemf.PaintPattern:
		return be.pattern(p)
	case gowemf.PaintLinearGradient:
		return be.linearGradient(p)
	}
	return fillStyle{}, false, fmt.Errorf("%w: metafile: paint kind %d", render.ErrInvalid, p.Kind)
}

// hatch draws Color on the hatch's pixels in pattern space, which
// PatternTransform places on the raster, and Background, if any, elsewhere.
func (be *backend) hatch(p gowemf.Paint) (fillStyle, bool, error) {
	if p.Hatch >= uint32(len(hatchRows)) {
		return fillStyle{}, false, be.soft("hatch style %d left out", p.Hatch)
	}
	inv, ok := fromMatrix(p.PatternTransform).invert()
	if !ok {
		return fillStyle{}, false, nil
	}
	rows := hatchRows[p.Hatch]
	fg := rgbaOf(p.Color)
	var bg rgba
	if p.Background != nil {
		bg = rgbaOf(*p.Background)
	}
	return fillStyle{fn: func(x, y int) rgba {
		q := inv.apply(point{float64(x) + 0.5, float64(y) + 0.5})
		u, v := wrapMod(math.Floor(q.x), 8), wrapMod(math.Floor(q.y), 8)
		if rows[v]&(0x80>>u) != 0 {
			return fg
		}
		return bg
	}}, true, nil
}

// wrapMod is v modulo n, in [0, n).
func wrapMod(v float64, n int) int {
	m := math.Mod(v, float64(n))
	if m < 0 {
		m += float64(n)
	}
	i := int(m)
	if i >= n || i < 0 {
		return 0
	}
	return i
}

// converted is a pattern or picture as straight-alpha pixels, read once.
type converted struct {
	img *image.NRGBA
}

// imageKey identifies a converted rectangle of an image. Only images held by
// pointer are cached: a key holding any other image type might not be
// comparable.
type imageKey struct {
	img any
	r   image.Rectangle
}

func cacheable(img image.Image) bool {
	switch img.(type) {
	case *image.NRGBA, *image.RGBA, *image.Paletted, *image.Gray, *image.NRGBA64, *image.RGBA64, *image.Gray16, *image.CMYK, *image.YCbCr, *image.Alpha:
		return true
	}
	return false
}

// toNRGBA returns an image's pixels in a rectangle as straight alpha, charging
// the pixels to the pixel-visit budget and caching them by image.
func (be *backend) toNRGBA(img image.Image, r image.Rectangle) (*image.NRGBA, error) {
	k := imageKey{img, r}
	cache := cacheable(img)
	if cache {
		if c, ok := be.images[k]; ok {
			return c.img, nil
		}
	}
	if r.Empty() {
		return image.NewNRGBA(image.Rectangle{}), nil
	}
	if err := be.b.visit(int64(r.Dx()) * int64(r.Dy())); err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	if n, ok := img.(*image.NRGBA); ok {
		for y := 0; y < r.Dy(); y++ {
			copy(out.Pix[y*out.Stride:y*out.Stride+4*r.Dx()], n.Pix[n.PixOffset(r.Min.X, r.Min.Y+y):])
		}
	} else {
		for y := 0; y < r.Dy(); y++ {
			if y%64 == 0 {
				if err := be.ctx.Err(); err != nil {
					return nil, err
				}
			}
			for x := 0; x < r.Dx(); x++ {
				c := color.NRGBAModel.Convert(img.At(r.Min.X+x, r.Min.Y+y)).(color.NRGBA)
				out.SetNRGBA(x, y, c)
			}
		}
	}
	if cache {
		be.images[k] = &converted{img: out}
	}
	return out, nil
}

// tileIndex maps v into [0, n) as the wrap mode repeats a tile along one
// axis, or reports it outside a clamped tile.
func tileIndex(v float64, n int, flip, clamp bool) (int, bool) {
	f := math.Floor(v)
	if clamp {
		return int(f), f >= 0 && f < float64(n)
	}
	period := float64(n)
	if flip {
		period *= 2
	}
	m := math.Mod(f, period)
	if m < 0 {
		m += period
	}
	i := int(m)
	if i >= n {
		i = 2*n - 1 - i
	}
	if i < 0 || i >= n {
		return 0, false
	}
	return i, true
}

// pattern repeats an image across pattern space, which PatternTransform maps
// onto the raster, as the wrap mode says. Pattern pixels are drawn as they
// are, without smoothing.
func (be *backend) pattern(p gowemf.Paint) (fillStyle, bool, error) {
	if p.Pattern == nil {
		return fillStyle{}, false, fmt.Errorf("%w: metafile: pattern paint without a pattern", render.ErrInvalid)
	}
	b := p.Pattern.Bounds()
	if b.Empty() {
		return fillStyle{}, false, nil
	}
	img, err := be.toNRGBA(p.Pattern, b)
	if err != nil {
		return fillStyle{}, false, err
	}
	inv, ok := fromMatrix(p.PatternTransform).invert()
	if !ok {
		return fillStyle{}, false, nil
	}
	flipX := p.Wrap == gowemf.WrapTileFlipX || p.Wrap == gowemf.WrapTileFlipXY
	flipY := p.Wrap == gowemf.WrapTileFlipY || p.Wrap == gowemf.WrapTileFlipXY
	clamp := p.Wrap == gowemf.WrapClamp
	w, h := b.Dx(), b.Dy()
	// Pattern space has its origin at the image's top left pixel.
	return fillStyle{fn: func(x, y int) rgba {
		q := inv.apply(point{float64(x) + 0.5, float64(y) + 0.5})
		u, okx := tileIndex(q.x, w, flipX, clamp)
		v, oky := tileIndex(q.y, h, flipY, clamp)
		if !okx || !oky {
			return rgba{}
		}
		i := img.PixOffset(u, v)
		return rgba{img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]}
	}}, true, nil
}

// linearGradient interpolates the stops along gradient space's x axis:
// Transform maps raster pixels into it, with 0 at the first stop and 1 at the
// last. Outside [0, 1] the parameter repeats, mirrors or, for a clamp, takes
// the end colors.
func (be *backend) linearGradient(p gowemf.Paint) (fillStyle, bool, error) {
	g := p.Gradient
	if g == nil || len(g.Stops) == 0 {
		return fillStyle{}, false, fmt.Errorf("%w: metafile: linear gradient without stops", render.ErrInvalid)
	}
	m := fromMatrix(g.Transform)
	if !m.finite() {
		return fillStyle{}, false, errCoordinate
	}
	stops := g.Stops
	return fillStyle{fn: func(x, y int) rgba {
		t := m.apply(point{float64(x) + 0.5, float64(y) + 0.5}).x
		switch g.Wrap {
		case gowemf.WrapTile, gowemf.WrapTileFlipY:
			t -= math.Floor(t)
		case gowemf.WrapTileFlipX, gowemf.WrapTileFlipXY:
			t = math.Mod(math.Abs(t), 2)
			if t > 1 {
				t = 2 - t
			}
		}
		if !(t > stops[0].Offset) {
			return rgbaOf(stops[0].Color)
		}
		for i := 1; i < len(stops); i++ {
			if t <= stops[i].Offset {
				a, b := stops[i-1], stops[i]
				if b.Offset <= a.Offset {
					return rgbaOf(b.Color)
				}
				f := (t - a.Offset) / (b.Offset - a.Offset)
				mix := func(u, v uint8) uint8 { return clamp8(float64(u) + (float64(v)-float64(u))*f) }
				return rgba{mix(a.Color.R, b.Color.R), mix(a.Color.G, b.Color.G), mix(a.Color.B, b.Color.B), mix(a.Color.A, b.Color.A)}
			}
		}
		return rgbaOf(stops[len(stops)-1].Color)
	}}, true, nil
}

func clamp8(v float64) uint8 { return uint8(math.Min(255, math.Max(0, v+0.5))) }
