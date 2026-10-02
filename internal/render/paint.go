package render

import (
	"context"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"strconv"
)

// WritePNG paints onto transparent black and encodes a PNG directly, without an
// intermediate SVG/PDF. Fractional rectangle edges use analytic area coverage;
// colors are composited in sRGB using source-over in painter order. Sizing and
// cumulative pixel visits and scanline work are checked before pixel allocation.
// Filled paths use even-odd scanlines and eight vertical samples per pixel.
// Output, writer failure
// or cancellation can leave a partial PNG in w. A blocking writer must itself
// support cancellation; this method cannot interrupt a blocked Write call.
func (p *Page) WritePNG(ctx context.Context, w io.Writer, dpi float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w == nil {
		return fmt.Errorf("%w: nil writer", ErrInvalid)
	}
	width, height, err := p.Size(dpi)
	if err != nil {
		return err
	}
	if dpi == 0 {
		dpi = 96
	}
	scale := dpi / 96
	commands, err := p.paintCommands(ctx, scale, width, height)
	if err != nil {
		return err
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for _, cmd := range commands {
		r := cmd.d.rect
		if cmd.d.path != nil || len(cmd.clips) > 0 {
			if err := paintPath(ctx, img, cmd, scale); err != nil {
				return err
			}
			continue
		}
		x0, y0, x1, y1 := pixelBounds(r, scale, width, height)
		left, top, right, bottom := r.x0*scale, r.y0*scale, r.x1*scale, r.y1*scale
		for y := y0; y < y1; y++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			cy := math.Min(float64(y+1), bottom) - math.Max(float64(y), top)
			for x := x0; x < x1; x++ {
				if (x-x0)%1024 == 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				cx := math.Min(float64(x+1), right) - math.Max(float64(x), left)
				c := cmd.d.pixelColor(x, y, scale)
				a := math.Max(0, math.Min(1, cx*cy)) * c.A
				i := y*img.Stride + x*4
				// image.RGBA stores premultiplied channels. Source channels
				// remain floating point until the composited pixel is rounded.
				img.Pix[i] = uint8(math.Round(c.R*a + float64(img.Pix[i])*(1-a)))
				img.Pix[i+1] = uint8(math.Round(c.G*a + float64(img.Pix[i+1])*(1-a)))
				img.Pix[i+2] = uint8(math.Round(c.B*a + float64(img.Pix[i+2])*(1-a)))
				img.Pix[i+3] = uint8(math.Round(255*a + float64(img.Pix[i+3])*(1-a)))
			}
		}
	}
	return png.Encode(&outputWriter{ctx: ctx, w: w, remaining: p.limits.MaxOutputBytes}, img)
}

func pixelBounds(r rectangle, scale float64, width, height int) (x0, y0, x1, y1 int) {
	// Coordinates were clipped during preparation and Size bounded all pixel
	// conversions. Clamp for any rounding at the edge of the allocation.
	x0 = max(0, min(width, int(math.Floor(r.x0*scale))))
	y0 = max(0, min(height, int(math.Floor(r.y0*scale))))
	x1 = max(x0, min(width, int(math.Ceil(r.x1*scale))))
	y1 = max(y0, min(height, int(math.Ceil(r.y1*scale))))
	return
}

// WriteSVG serializes the same snapshot without rasterizing. Coordinates are
// scaled explicitly to output pixels; root width/height use the same rounding
// as PNG. Only generated shapes, internal clipping references and numeric attributes are emitted, with
// no external references or source markup. Raster images use generated PNG data
// URLs; source URLs are never retained. Output or cancellation errors may
// leave a partial SVG. Cancellation cannot interrupt a blocking writer.
func (p *Page) WriteSVG(ctx context.Context, w io.Writer, dpi float64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w == nil {
		return fmt.Errorf("%w: nil writer", ErrInvalid)
	}
	width, height, err := p.Size(dpi)
	if err != nil {
		return err
	}
	if dpi == 0 {
		dpi = 96
	}
	scale := dpi / 96
	e := xml.NewEncoder(&outputWriter{ctx: ctx, w: w, remaining: p.limits.MaxOutputBytes})
	root := xml.StartElement{Name: xml.Name{Local: "svg"}, Attr: []xml.Attr{
		attr("xmlns", "http://www.w3.org/2000/svg"), attr("width", strconv.Itoa(width)), attr("height", strconv.Itoa(height)),
		attr("viewBox", fmt.Sprintf("0 0 %d %d", width, height)), attr("color-interpolation", "sRGB"),
	}}
	if err := e.EncodeToken(root); err != nil {
		return err
	}
	if err := p.svgDrawings(ctx, e, scale); err != nil {
		return err
	}
	if err := e.EncodeToken(root.End()); err != nil {
		return err
	}
	return e.Flush()
}

func number(v float64) string          { return strconv.FormatFloat(v, 'f', -1, 64) }
func attr(name, value string) xml.Attr { return xml.Attr{Name: xml.Name{Local: name}, Value: value} }

// outputWriter enforces encoded byte limits during writes and preserves the
// caller's writer errors. Refuse an over-budget chunk before writing any of it.
type outputWriter struct {
	ctx       context.Context
	w         io.Writer
	remaining int64
}

func (w *outputWriter) Write(b []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(b)) > w.remaining {
		return 0, fmt.Errorf("%w: encoded bytes", ErrLimit)
	}
	n, err := w.w.Write(b)
	if n < 0 || n > len(b) {
		return 0, fmt.Errorf("%w: invalid writer count", ErrInvalid)
	}
	w.remaining -= int64(n)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = w.ctx.Err()
	}
	return n, err
}
