// Package metafile draws Windows metafiles, WMF, EMF and EMF+, as pictures.
//
// gowemf plays the files: it interprets the records of the graphics device
// interface (GDI) and of GDI+ — device contexts and graphics states, mapping
// modes and transforms, objects, paths, clipping, bitmaps, gradients and the
// placement of text — and resolves each drawing into geometry and paint in
// the picture's coordinates. This package is the backend it draws through: it
// fills and strokes paths, places images, shades gradient meshes and draws
// text in fonts the caller supplies, onto a transparent raster of a given
// size.
//
// The raster is painted here, with the same scan-conversion approach as the
// page painter (eight vertical samples a pixel, exact horizontal coverage)
// rather than through its display list, which fills only by the even-odd rule
// and has neither strokes nor arbitrary clip regions.
//
// Drawing is strict unless the caller sets Options.Approximate: anything that
// cannot be drawn exactly returns an error wrapping render.ErrUnsupported.
// With it set, the details drawn approximately or left out are reported to it
// and the rest is drawn. Every budget of the render limits applies.
package metafile

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/mgilbir/gowemf"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// Options controls playback.
type Options struct {
	// Limits are the render limits; zero fields take their defaults.
	Limits render.Limits
	// Approximate, when set, makes playback best effort: it receives each
	// detail that is drawn approximately or left out, as an error wrapping
	// render.ErrUnsupported, and returns nil to go on or an error to stop. It
	// may be called with the same message once only. Nil is strict.
	Approximate func(error) error
	// Fonts supplies the fonts of text. Without it text cannot be drawn.
	Fonts render.FontResolver
}

// Info describes a metafile without drawing it.
type Info struct {
	// EMF is true for an enhanced metafile, false for a Windows metafile.
	EMF bool
	// Width and Height are the picture's natural size in CSS pixels, from its
	// recorded physical size; zero when the file records none.
	Width, Height float64
	// Aspect is the ratio of the picture's width to its height.
	Aspect float64
	// Plus and PlusOnly report an EMF+ file and one with no GDI records.
	Plus, PlusOnly bool
}

// area is the picture's rectangle in device units, and what the file says of
// its device.
type area struct {
	x0, y0, x1, y1     float64
	pxPerMmX, pxPerMmY float64
	mmX, mmY           float64
	// physical is set when the file records a physical size.
	physical bool
	// window is a standard WMF's window, which stands in for its bounds.
	wndOrg, wndExt point
	placeable      bool
}

func wrapParse(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gowemf.ErrLimit):
		return fmt.Errorf("%w: metafile: %w", render.ErrLimit, err)
	case errors.Is(err, gowemf.ErrUnsupported):
		return fmt.Errorf("%w: metafile: %w", render.ErrUnsupported, err)
	case errors.Is(err, gowemf.ErrFormat), errors.Is(err, gowemf.ErrMalformed):
		return fmt.Errorf("%w: metafile: %w", render.ErrInvalid, err)
	}
	return err
}

func areaOf(h gowemf.Header, data []byte) (area, error) {
	var a area
	switch {
	case h.EMF != nil:
		e := h.EMF
		a.pxPerMmX, a.pxPerMmY = 96/25.4, 96/25.4
		a.mmX, a.mmY = 1, 1
		if e.Device.X > 0 && e.Device.Y > 0 && e.Millimeters.X > 0 && e.Millimeters.Y > 0 {
			a.pxPerMmX = float64(e.Device.X) / float64(e.Millimeters.X)
			a.pxPerMmY = float64(e.Device.Y) / float64(e.Millimeters.Y)
			a.mmX, a.mmY = float64(e.Millimeters.X), float64(e.Millimeters.Y)
		}
		f := e.Frame
		if f.Right > f.Left && f.Bottom > f.Top {
			// The frame is in hundredths of a millimetre.
			a.x0, a.x1 = float64(f.Left)/100*a.pxPerMmX, float64(f.Right)/100*a.pxPerMmX
			a.y0, a.y1 = float64(f.Top)/100*a.pxPerMmY, float64(f.Bottom)/100*a.pxPerMmY
			a.physical = true
		} else {
			// Bounds are inclusive.
			b := e.Bounds
			a.x0, a.x1 = float64(min(b.Left, b.Right)), float64(max(b.Left, b.Right))+1
			a.y0, a.y1 = float64(min(b.Top, b.Bottom)), float64(max(b.Top, b.Bottom))+1
		}
	case h.Placeable != nil:
		b := h.Placeable.Bounds
		upi := float64(h.Placeable.UnitsPerInch)
		if upi == 0 {
			upi = 1440
		}
		a.pxPerMmX, a.pxPerMmY = upi/25.4, upi/25.4
		a.mmX, a.mmY = 1, 1
		a.x0, a.x1 = float64(min(b.Left, b.Right)), float64(max(b.Left, b.Right))
		a.y0, a.y1 = float64(min(b.Top, b.Bottom)), float64(max(b.Top, b.Bottom))
		a.physical = h.Placeable.UnitsPerInch != 0
		a.placeable = true
	default:
		org, ext, ok := wmfWindow(data, gowemf.Limits{})
		if !ok {
			return a, errNoSize
		}
		a.pxPerMmX, a.pxPerMmY = 96/25.4, 96/25.4
		a.mmX, a.mmY = 1, 1
		a.wndOrg, a.wndExt = org, ext
		a.x1, a.y1 = math.Abs(ext.x), math.Abs(ext.y)
	}
	if !(a.x1 > a.x0) || !(a.y1 > a.y0) || !finite(a.x1-a.x0) || !finite(a.y1-a.y0) {
		return a, fmt.Errorf("%w: metafile: empty picture area", render.ErrInvalid)
	}
	return a, nil
}

// Inspect checks a metafile's framing and reports its size.
func Inspect(data []byte) (Info, error) {
	h, err := gowemf.Walk(data, gowemf.Limits{}, nil)
	if err != nil {
		return Info{}, wrapParse(err)
	}
	a, err := areaOf(h, data)
	if err != nil {
		return Info{}, err
	}
	info := Info{EMF: h.EMF != nil, Aspect: (a.x1 - a.x0) / (a.y1 - a.y0)}
	if h.EMFPlus != nil {
		info.Plus, info.PlusOnly = true, !h.EMFPlus.Dual
	}
	if w, hh, err := h.SizeInPoints(); err == nil {
		info.Width, info.Height = w*96/72, hh*96/72
	} else if h.EMF == nil && h.Placeable == nil {
		// A standard WMF's window, taken as pixels.
		info.Width, info.Height = a.x1-a.x0, a.y1-a.y0
	}
	return info, nil
}

// checkRasterSize refuses a raster past the limits, or whose per-pixel
// buffers, of up to 16 bytes a pixel, int cannot count, as on 32-bit
// platforms.
func checkRasterSize(width, height int, lim render.Limits) error {
	if width <= 0 || height <= 0 || width > lim.MaxDimension || height > lim.MaxDimension || int64(width)*int64(height) > lim.MaxImagePixels || width > (math.MaxInt/32)/height {
		return fmt.Errorf("%w: metafile raster size", render.ErrLimit)
	}
	return nil
}

// Render draws a metafile onto a transparent raster of width by height pixels,
// stretching the picture over all of it.
func Render(ctx context.Context, data []byte, width, height int, opts Options) (*image.NRGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lim, err := core.ResolveLimits(opts.Limits)
	if err != nil {
		return nil, err
	}
	if err = checkRasterSize(width, height, lim); err != nil {
		return nil, err
	}
	if int64(len(data)) > lim.MaxImageBytes {
		return nil, fmt.Errorf("%w: metafile bytes", render.ErrLimit)
	}
	framing := gowemf.Limits{MaxBytes: uint64(len(data))}
	h, err := gowemf.Walk(data, framing, nil)
	if err != nil {
		return nil, wrapParse(err)
	}
	a, err := areaOf(h, data)
	if err != nil {
		return nil, err
	}
	b := &budget{
		maxOps: lim.MaxOperations, maxSegments: lim.MaxPathSegments, maxGlyphs: lim.MaxGlyphs,
		maxEdgeChecks: lim.MaxEdgeChecks, maxPixelVisits: lim.MaxPixelVisits,
		maxMaskPixels: max(16*int64(width)*int64(height), 1<<20),
	}
	be := newBackend(ctx, width, height, opts, b, lim.MaxFonts)
	po := gowemf.PlayOptions{
		Stream:      gowemf.StreamOptions{Framing: framing},
		Destination: gowemf.Box{Width: float64(width), Height: float64(height)},
		// Decoded bitmaps share the image pixel budget, as one picture.
		Images:         gowemf.ImageLimits{MaxBytes: uint64(lim.MaxImageBytes), MaxPixels: uint64(lim.MaxImagePixels), MaxDecodedBytes: uint64(4 * lim.MaxImagePixels)},
		MaxImagePixels: uint64(lim.MaxImagePixels),
		MaxPathPoints:  uint64(lim.MaxPathSegments),
	}
	if opts.Approximate != nil {
		po.Unsupported = be.unsupported
	}
	if h.EMFPlus != nil && h.EMFPlus.Dual {
		if po.Stream.PreferGDI, err = be.preferGDI(data, po); err != nil {
			return nil, err
		}
	}
	if h.WMF != nil && h.Placeable == nil {
		// A WMF without a placeable header plays in the window its first
		// window records set, taken as pixels at 96 per inch.
		po.Placeable = &gowemf.PlaceableHeader{
			Bounds:       gowemf.Rect{Left: int32(a.wndOrg.x), Top: int32(a.wndOrg.y), Right: int32(a.wndOrg.x + a.wndExt.x), Bottom: int32(a.wndOrg.y + a.wndExt.y)},
			UnitsPerInch: 96,
		}
	}
	_, err = gowemf.Play(data, po, be)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		if errors.Is(err, gowemf.ErrUnsupported) && opts.Approximate != nil {
			// A record the player cannot read ends the picture; what came
			// before it stays.
			if err = be.soft("records from %v on left out", err); err != nil {
				return nil, err
			}
		} else {
			return nil, wrapParse(err)
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return be.r.toNRGBA(), nil
}

// toNRGBA converts the canvas to straight alpha.
func (r *raster) toNRGBA() *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, r.w, r.h))
	for i := 0; i+3 < len(r.pix); i += 4 {
		a := uint32(r.pix[i+3])
		if a == 0 {
			continue
		}
		out.Pix[i] = uint8(min(255, (uint32(r.pix[i])*255+a/2)/a))
		out.Pix[i+1] = uint8(min(255, (uint32(r.pix[i+1])*255+a/2)/a))
		out.Pix[i+2] = uint8(min(255, (uint32(r.pix[i+2])*255+a/2)/a))
		out.Pix[i+3] = uint8(a)
	}
	return out
}

// Scales a picture is drawn at, in pixels for each CSS pixel it covers.
const (
	drawScale    = 2.0
	minDrawScale = 0.5
)

// Plan sizes the raster for a metafile drawn over a box of w by h CSS pixels
// (zero for its natural size): twice the box's pixels, reduced to fit the
// remaining image pixels and the largest dimension, and refused below half.
func Plan(data []byte, w, h float64, remaining int64, maxDim int) (pw, ph int, err error) {
	info, err := Inspect(data)
	if err != nil {
		return 0, 0, err
	}
	if !(w > 0) || !(h > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) {
		w, h = info.Width, info.Height
		if !(w > 0) || !(h > 0) {
			w, h = 96, 96/info.Aspect
		}
	}
	scale := drawScale
	if area := w * h; area*scale*scale > float64(remaining) {
		scale = math.Sqrt(float64(remaining) / area)
	}
	if side := math.Max(w, h); side*scale > float64(maxDim) {
		scale = float64(maxDim) / side
	}
	if !(scale >= minDrawScale) {
		return 0, 0, fmt.Errorf("%w: image budget for a metafile", render.ErrLimit)
	}
	pw, ph = max(1, int(math.Round(w*scale))), max(1, int(math.Round(h*scale)))
	if int64(pw)*int64(ph) > remaining || pw > maxDim || ph > maxDim {
		return 0, 0, fmt.Errorf("%w: image budget for a metafile", render.ErrLimit)
	}
	return pw, ph, nil
}

// wmfBounds finds the window a WMF without a placeable header draws in, from
// its first window origin and extent records.
func wmfWindow(data []byte, limits gowemf.Limits) (org, ext point, ok bool) {
	var haveExt bool
	_, err := gowemf.Walk(data, limits, func(r gowemf.Record) error {
		if r.Format != gowemf.WMF || haveExt {
			return nil
		}
		switch r.Type & 0xff {
		case 0x0b, 0x0c:
			body, err := gowemf.Decode(r, gowemf.DecodeLimits{})
			if err != nil {
				return nil
			}
			p, isPoint := body.(gowemf.PointRecord)
			if !isPoint {
				return nil
			}
			if r.Type&0xff == 0x0b {
				org = point{p.Point.X, p.Point.Y}
			} else {
				ext, haveExt = point{p.Point.X, p.Point.Y}, true
			}
		}
		return nil
	})
	if err != nil || !haveExt || ext.x == 0 || ext.y == 0 || math.IsNaN(ext.x) {
		return point{}, point{}, false
	}
	return org, ext, true
}

var errNoSize = fmt.Errorf("%w: metafile: WMF without a placeable header or window extent", render.ErrUnsupported)
