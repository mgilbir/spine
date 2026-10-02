package render

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

var (
	ErrInvalid     = errors.New("render: invalid request")
	ErrLimit       = errors.New("render: resource limit")
	ErrUnsupported = errors.New("render: unsupported operation")
)

// Limits bounds both preparation and painting. Zero fields select defaults;
// negative fields are invalid. Limits apply independently to each render.
type Limits struct {
	MaxDimension   int
	MaxPixels      int64
	MaxOperations  int
	MaxPixelVisits int64
	MaxOutputBytes int64
}

func (l Limits) resolved() (Limits, error) {
	if l.MaxDimension < 0 || l.MaxPixels < 0 || l.MaxOperations < 0 || l.MaxPixelVisits < 0 || l.MaxOutputBytes < 0 {
		return Limits{}, fmt.Errorf("%w: negative limit", ErrInvalid)
	}
	if l.MaxDimension == 0 {
		l.MaxDimension = 8192
	}
	if l.MaxPixels == 0 {
		l.MaxPixels = 16 << 20
	}
	if l.MaxOperations == 0 {
		l.MaxOperations = 100000
	}
	if l.MaxPixelVisits == 0 {
		l.MaxPixelVisits = 64 << 20
	}
	if l.MaxOutputBytes == 0 {
		l.MaxOutputBytes = 32 << 20
	}
	return l, nil
}

// Page is an immutable snapshot in CSS pixels (96 per inch). It contains no
// references to the supplied operations. Its zero value is invalid.
type Page struct {
	width, height float64
	rects         []rectangle
	limits        Limits
}

type rectangle struct {
	x0, y0, x1, y1 float64
	color          style.RGBA
}

// Prepare snapshots a page's solid rectangles in painter order. Physical page
// dimensions use EMU; Forme operations use CSS pixels from the top left.
// Rectangles are clipped to the page. Negative extents and invalid colors fail;
// zero-area rectangles are accepted and paint nothing. Overhang is layout
// metadata and does not alter painting.
func Prepare(ctx context.Context, width, height dml.EMU, ops []layout.Op, limits Limits) (*Page, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("%w: nonpositive page size", ErrInvalid)
	}
	l, err := limits.resolved()
	if err != nil {
		return nil, err
	}
	if len(ops) > l.MaxOperations {
		return nil, fmt.Errorf("%w: operation count", ErrLimit)
	}
	p := &Page{width: float64(width) / float64(dml.EMUsPerPixel), height: float64(height) / float64(dml.EMUsPerPixel), limits: l}
	// Avoid allocating by the operation count before validating each operation.
	for i, op := range ops {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r, ok := op.(layout.FillRect)
		if !ok {
			return nil, fmt.Errorf("%w: operation %d (%T)", ErrUnsupported, i, op)
		}
		if r.Rect.W < 0 || r.Rect.H < 0 || !validColor(r.Color) {
			return nil, fmt.Errorf("%w: rectangle %d", ErrInvalid, i)
		}
		// Sum in floating point, not saturating Forme units: saturation would
		// change an extreme rectangle's right or bottom edge.
		x0, y0 := r.Rect.X.Px(), r.Rect.Y.Px()
		x1, y1 := x0+r.Rect.W.Px(), y0+r.Rect.H.Px()
		x0, y0 = math.Max(0, x0), math.Max(0, y0)
		x1, y1 = math.Min(p.width, x1), math.Min(p.height, y1)
		if x1 > x0 && y1 > y0 {
			p.rects = append(p.rects, rectangle{x0, y0, x1, y1, r.Color})
		}
	}
	return p, nil
}

func validColor(c style.RGBA) bool {
	for _, v := range []float64{c.R, c.G, c.B} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 255 {
			return false
		}
	}
	return !math.IsNaN(c.A) && !math.IsInf(c.A, 0) && c.A >= 0 && c.A <= 1
}

// Size resolves a DPI to pixel dimensions, rounding each extent upwards so no
// physical content is cropped. Zero DPI selects 96. Limits and address-space
// overflow are checked before a backend allocates a four-byte pixel buffer.
func (p *Page) Size(dpi float64) (width, height int, err error) {
	if p == nil || p.width <= 0 || p.height <= 0 {
		return 0, 0, fmt.Errorf("%w: unprepared page", ErrInvalid)
	}
	if dpi == 0 {
		dpi = 96
	}
	if dpi <= 0 || math.IsNaN(dpi) || math.IsInf(dpi, 0) {
		return 0, 0, fmt.Errorf("%w: DPI", ErrInvalid)
	}
	w, h := math.Ceil(p.width*(dpi/96)), math.Ceil(p.height*(dpi/96))
	// Positive subnormal DPI may underflow. Never convert an unchecked float
	// to int, including on 32-bit architectures or at float64's rounded MaxInt.
	maxInt := int(^uint(0) >> 1)
	if w < 1 || h < 1 {
		return 0, 0, fmt.Errorf("%w: DPI underflow", ErrInvalid)
	}
	if w > float64(p.limits.MaxDimension) || h > float64(p.limits.MaxDimension) || w >= float64(maxInt) || h >= float64(maxInt) {
		return 0, 0, fmt.Errorf("%w: pixel dimension", ErrLimit)
	}
	width, height = int(w), int(h)
	if int64(width) > p.limits.MaxPixels/int64(height) || width > (maxInt/4)/height {
		return 0, 0, fmt.Errorf("%w: pixel allocation", ErrLimit)
	}
	return width, height, nil
}
