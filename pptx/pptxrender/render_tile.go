package pptxrender

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/internal/metafile"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// renderMaxTilePixels bounds the image a tiled fill is composed into, and
// four times it all a slide's tiled fills.
const renderMaxTilePixels = 4 << 20

// renderTileSize is a picture's tile extent in pixels at 96 DPI: its natural
// size at the fill's DPI, the file's, or 96, scaled by the tile's scale.
func renderTileSize(img image.Image, data []byte, dpi *int32, tile *dml.TileXML) (w, h float64) {
	if info, err := metafile.Inspect(data); err == nil && renderIsMetafile(data) {
		// A metafile's natural size is its recorded physical size, drawn at
		// whatever density suits; absent one, 96 pixels across.
		nw, nh := info.Width, info.Height
		if nw <= 0 || nh <= 0 {
			nw, nh = 96, 96/info.Aspect
		}
		sx, sy := float64(tile.Sx.Int32())/100000, float64(tile.Sy.Int32())/100000
		if sx == 0 {
			sx = 1
		}
		if sy == 0 {
			sy = 1
		}
		return nw * sx, nh * sy
	}
	dx, dy := 96.0, 96.0
	if dpi != nil && *dpi > 0 {
		dx, dy = float64(*dpi), float64(*dpi)
	} else if x, y, ok := core.ImageDPI(data); ok {
		dx, dy = x, y
	}
	sx, sy := float64(tile.Sx.Int32())/100000, float64(tile.Sy.Int32())/100000
	// An absent scale, read as zero, is 100%.
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	b := img.Bounds()
	return float64(b.Dx()) * 96 / dx * sx, float64(b.Dy()) * 96 / dy * sy
}

// renderTileImage composes a picture tiled over a box w by h pixels into one
// image to stretch over the box. The first tile, tw by th pixels, sits at
// the tile's alignment in the box, moved by its offset; the rest repeat from
// it every way, every other one mirrored on each axis its flip names. The
// image has the picture's density, at most four pixels per box pixel, and at
// most renderMaxTilePixels in all.
func renderTileImage(colors *renderColors, img image.Image, w, h, tw, th float64, tile *dml.TileXML) (image.Image, error) {
	for _, v := range []float64{w, h, tw, th} {
		if !(v > 0) || math.IsInf(v, 0) || v > 1<<24 {
			return nil, fmt.Errorf("%w: tile extent", render.ErrInvalid)
		}
	}
	var x0, y0 float64
	switch tile.Algn {
	case "", "tl", "l", "bl":
	case "t", "ctr", "b":
		x0 = (w - tw) / 2
	case "tr", "r", "br":
		x0 = w - tw
	default:
		return nil, fmt.Errorf("%w: tile alignment", render.ErrInvalid)
	}
	switch tile.Algn {
	case "", "tl", "t", "tr":
	case "l", "ctr", "r":
		y0 = (h - th) / 2
	default:
		y0 = h - th
	}
	px := float64(dml.EMUsPerPixel)
	if tile.Tx != nil {
		x0 += float64(*tile.Tx) / px
	}
	if tile.Ty != nil {
		y0 += float64(*tile.Ty) / px
	}
	var flipX, flipY bool
	switch tile.Flip {
	case "", "none":
	case "x":
		flipX = true
	case "y":
		flipY = true
	case "xy":
		flipX, flipY = true, true
	default:
		return nil, fmt.Errorf("%w: tile flip", render.ErrInvalid)
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return nil, fmt.Errorf("%w: empty tile picture", render.ErrInvalid)
	}
	scale := math.Min(renderMaxImageScale, math.Max(1, math.Max(float64(sw)/tw, float64(sh)/th)))
	scale = math.Min(scale, math.Sqrt(renderMaxTilePixels/(w*h)))
	ow, oh := max(1, int(math.Ceil(w*scale))), max(1, int(math.Ceil(h*scale)))
	n := int64(ow) * int64(oh)
	if n > renderMaxTilePixels*2 || n > 4*renderMaxTilePixels-colors.tilePixels {
		return nil, fmt.Errorf("%w: tiled fill pixels", render.ErrLimit)
	}
	colors.tilePixels += n
	ctx := colors.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	src, ok := img.(*image.NRGBA)
	if !ok || src.Rect.Min != (image.Point{}) {
		src = renderNRGBA(img)
	}
	// along maps a box coordinate to a picture pixel on one axis.
	along := func(u, origin, size float64, n int, flip bool) int {
		d := (u - origin) / size
		i := math.Floor(d)
		f := d - i
		if flip && math.Mod(i, 2) != 0 {
			f = 1 - f
		}
		return min(n-1, max(0, int(f*float64(n))))
	}
	columns := make([]int, ow)
	for x := range columns {
		columns[x] = along((float64(x)+0.5)/scale, x0, tw, sw, flipX)
	}
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < oh; y++ {
		if y%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		row := src.Pix[along((float64(y)+0.5)/scale, y0, th, sh, flipY)*src.Stride:]
		dst := out.Pix[y*out.Stride:]
		for x, c := range columns {
			copy(dst[4*x:4*x+4], row[4*c:4*c+4])
		}
	}
	return out, nil
}
