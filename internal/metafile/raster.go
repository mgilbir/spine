package metafile

import (
	"context"
	"fmt"
	"math"
	"math/bits"
	"slices"

	"github.com/mgilbir/spine/render"
)

type point struct{ x, y float64 }

// rgba is a straight (non-premultiplied) eight-bit color.
type rgba struct{ r, g, b, a uint8 }

// subSamples is the number of vertical samples per pixel row. Horizontal
// coverage is exact, as in the page painter.
const subSamples = 8

// budget counts the work a metafile asks for against the render limits.
type budget struct {
	ops         int
	segments    int
	glyphs      int
	shapeWork   int64
	edgeChecks  int64
	pixelVisits int64
	maskPixels  int64

	maxOps         int
	maxSegments    int
	maxGlyphs      int
	maxShapeWork   int64
	maxRunBytes    int
	maxEdgeChecks  int64
	maxPixelVisits int64
	maxMaskPixels  int64
}

func (b *budget) op() error {
	if b.ops >= b.maxOps {
		return fmt.Errorf("%w: metafile operation count", render.ErrLimit)
	}
	b.ops++
	return nil
}

func (b *budget) addSegments(n int) error {
	if n > b.maxSegments-b.segments {
		return fmt.Errorf("%w: metafile path segments", render.ErrLimit)
	}
	b.segments += n
	return nil
}

func (b *budget) visit(n int64) error {
	if n > b.maxPixelVisits-b.pixelVisits {
		return fmt.Errorf("%w: metafile pixel visits", render.ErrLimit)
	}
	b.pixelVisits += n
	return nil
}

// raster is the canvas. Coordinates are pixels, x right and y down from the
// top left corner.
type raster struct {
	ctx  context.Context
	w, h int
	pix  []uint8 // premultiplied RGBA
	b    *budget

	edges  []edge
	cross  []crossing
	active []int32
	acc    []float32
	diff   []float32
	covRow []float32
}

func newRaster(ctx context.Context, w, h int, b *budget) *raster {
	return &raster{ctx: ctx, w: w, h: h, pix: make([]uint8, 4*w*h), b: b,
		acc: make([]float32, w+2), diff: make([]float32, w+2), covRow: make([]float32, w+1)}
}

type edge struct {
	x0, y0, x1, y1 float64 // y0 < y1
	slope          float64
	dir            int32
}

type crossing struct {
	x   float64
	dir int32
}

// mask is a coverage multiplier over the pixels in its bounds, zero outside.
type mask struct {
	x0, y0, w, h int
	a            []uint8
}

func (m *mask) at(x, y int) uint8 {
	x -= m.x0
	y -= m.y0
	if x < 0 || y < 0 || x >= m.w || y >= m.h {
		return 0
	}
	return m.a[y*m.w+x]
}

// clip is the current clipping region: a pixel-space rectangle with exact
// fractional edges, and an optional mask inside it. A clip is immutable, so
// saved device contexts share it.
type clip struct {
	x0, y0, x1, y1 float64
	m              *mask
}

func (r *raster) fullClip() *clip {
	return &clip{x0: 0, y0: 0, x1: float64(r.w), y1: float64(r.h)}
}

func (c *clip) empty() bool { return !(c.x1 > c.x0 && c.y1 > c.y0) }

// coverage is the clip's coverage of pixel (x, y).
func (c *clip) coverage(x, y int) float32 {
	cx := math.Min(float64(x+1), c.x1) - math.Max(float64(x), c.x0)
	cy := math.Min(float64(y+1), c.y1) - math.Max(float64(y), c.y0)
	if cx <= 0 || cy <= 0 {
		return 0
	}
	v := float32(cx * cy)
	if c.m != nil {
		v *= float32(c.m.at(x, y)) / 255
	}
	return v
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// buildEdges converts contours, each implicitly closed, to pixel-space edges
// and returns their pixel bounds. Edges wholly outside the rows [ry0, ry1)
// are dropped.
func (r *raster) buildEdges(contours [][]point, ry0, ry1 float64) (minX, minY, maxX, maxY float64, err error) {
	r.edges = r.edges[:0]
	minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range contours {
		if len(c) < 2 {
			continue
		}
		if err = r.b.addSegments(len(c)); err != nil {
			return
		}
		prev := c[len(c)-1]
		if !finite(prev.x) || !finite(prev.y) {
			return 0, 0, 0, 0, fmt.Errorf("%w: metafile coordinate", render.ErrInvalid)
		}
		for _, q := range c {
			cur := q
			if !finite(cur.x) || !finite(cur.y) {
				return 0, 0, 0, 0, fmt.Errorf("%w: metafile coordinate", render.ErrInvalid)
			}
			minX, maxX = math.Min(minX, cur.x), math.Max(maxX, cur.x)
			minY, maxY = math.Min(minY, cur.y), math.Max(maxY, cur.y)
			if prev.y != cur.y {
				e := edge{x0: prev.x, y0: prev.y, x1: cur.x, y1: cur.y, dir: 1}
				if e.y0 > e.y1 {
					e.x0, e.y0, e.x1, e.y1, e.dir = e.x1, e.y1, e.x0, e.y0, -1
				}
				if e.y1 > ry0 && e.y0 < ry1 {
					e.slope = (e.x1 - e.x0) / (e.y1 - e.y0)
					r.edges = append(r.edges, e)
				}
			}
			prev = cur
		}
	}
	return
}

// cover scans the contours' coverage inside the clip's rectangle, calling row
// for each pixel row with any coverage, with the coverage of the pixels from
// x0 on. The coverage slice is reused by the next call.
func (r *raster) cover(contours [][]point, evenOdd bool, c *clip, row func(y, x0 int, cov []float32) error) error {
	if c.empty() {
		return nil
	}
	ry0, ry1 := math.Max(0, c.y0), math.Min(float64(r.h), c.y1)
	rx0, rx1 := math.Max(0, c.x0), math.Min(float64(r.w), c.x1)
	if !(ry1 > ry0 && rx1 > rx0) {
		return nil
	}
	minX, minY, maxX, maxY, err := r.buildEdges(contours, ry0, ry1)
	if err != nil {
		return err
	}
	if len(r.edges) == 0 || maxX <= rx0 || minX >= rx1 {
		return nil
	}
	bx0, bx1 := int(math.Floor(math.Max(minX, rx0))), int(math.Ceil(math.Min(maxX, rx1)))
	by0, by1 := int(math.Floor(math.Max(minY, ry0))), int(math.Ceil(math.Min(maxY, ry1)))
	bx0, bx1 = max(0, bx0), min(r.w, bx1)
	by0, by1 = max(0, by0), min(r.h, by1)
	if bx1 <= bx0 || by1 <= by0 {
		return nil
	}
	if err = r.b.visit(int64(bx1-bx0) * int64(by1-by0)); err != nil {
		return err
	}
	n := int64(len(r.edges))
	if n > 1 {
		if err = r.charge(n * int64(bits.Len64(uint64(n)))); err != nil {
			return err
		}
	}
	slices.SortFunc(r.edges, func(a, b edge) int {
		switch {
		case a.y0 < b.y0:
			return -1
		case a.y0 > b.y0:
			return 1
		}
		return 0
	})
	edges := r.edges
	active := r.active[:0]
	next := 0
	// Skip edges that end before the first row.
	for next < len(edges) && edges[next].y1 <= float64(by0) && edges[next].y0 < float64(by0) {
		next++
	}
	// Edges that start above the first row but reach it must still be active.
	acc, diff := r.acc, r.diff
	weight := float32(1.0 / subSamples)
	for y := by0; y < by1; y++ {
		if err = r.ctx.Err(); err != nil {
			return err
		}
		touched := false
		for s := 0; s < subSamples; s++ {
			sy := float64(y) + (float64(s)+0.5)/subSamples
			if sy < ry0 || sy >= ry1 {
				continue
			}
			for next < len(edges) && edges[next].y0 <= sy {
				if edges[next].y1 > sy {
					active = append(active, int32(next))
				}
				next++
			}
			cross := r.cross[:0]
			k := 0
			for _, i := range active {
				e := &edges[i]
				if e.y1 <= sy {
					continue
				}
				active[k] = i
				k++
				cross = append(cross, crossing{e.x0 + (sy-e.y0)*e.slope, e.dir})
			}
			active = active[:k]
			if len(cross) < 2 {
				r.cross = cross
				continue
			}
			if err = r.charge(int64(len(cross)) * int64(bits.Len(uint(len(cross)))+1)); err != nil {
				return err
			}
			slices.SortFunc(cross, func(a, b crossing) int {
				switch {
				case a.x < b.x:
					return -1
				case a.x > b.x:
					return 1
				}
				return 0
			})
			winding := int32(0)
			for i := 0; i < len(cross)-1; i++ {
				if evenOdd {
					winding ^= 1
				} else {
					winding += cross[i].dir
				}
				if winding == 0 {
					continue
				}
				xa, xb := math.Max(cross[i].x, rx0), math.Min(cross[i+1].x, rx1)
				if xb <= xa {
					continue
				}
				touched = true
				addSpan(acc, diff, xa, xb, weight)
			}
			r.cross = cross
		}
		if !touched {
			continue
		}
		cov := r.covRow[:0]
		var run float32
		any := false
		for x := bx0; x < bx1; x++ {
			run += diff[x]
			v := acc[x] + run
			acc[x], diff[x] = 0, 0
			if v > 1 {
				v = 1
			}
			if v > 0.0005 {
				any = true
			}
			cov = append(cov, v)
		}
		// Clear what the span ends added just past the box.
		acc[bx1], diff[bx1] = 0, 0
		acc[bx1+1], diff[bx1+1] = 0, 0
		if any {
			if err = row(y, bx0, cov); err != nil {
				return err
			}
		}
	}
	r.active = active[:0]
	return nil
}

func (r *raster) charge(n int64) error {
	if n > r.b.maxEdgeChecks-r.b.edgeChecks {
		return fmt.Errorf("%w: metafile edge checks", render.ErrLimit)
	}
	r.b.edgeChecks += n
	return nil
}

// addSpan adds weight times the coverage of the span [xa, xb) of one sample
// row to the accumulation arrays: the partly covered end pixels directly, and
// the run of whole pixels between them as a difference.
func addSpan(acc, diff []float32, xa, xb float64, weight float32) {
	ia, ib := int(xa), int(xb)
	if ia == ib {
		acc[ia] += float32(xb-xa) * weight
		return
	}
	acc[ia] += float32(float64(ia+1)-xa) * weight
	acc[ib] += float32(xb-float64(ib)) * weight
	if ib > ia+1 {
		diff[ia+1] += weight
		diff[ib] -= weight
	}
}

// fillStyle is a fill's paint: one color, or a function of the pixel.
type fillStyle struct {
	solid rgba
	fn    func(x, y int) rgba
}

// fill paints the contours, filled by the even-odd or nonzero rule, under the
// clip.
func (r *raster) fill(contours [][]point, evenOdd bool, st fillStyle, c *clip) error {
	return r.cover(contours, evenOdd, c, func(y, x0 int, cov []float32) error {
		base := y * r.w * 4
		for i, v := range cov {
			if v <= 0 {
				continue
			}
			x := x0 + i
			if c.m != nil {
				v *= float32(c.m.at(x, y)) / 255
				if v <= 0 {
					continue
				}
			}
			col := st.solid
			if st.fn != nil {
				col = st.fn(x, y)
			}
			r.blend(base+4*x, col, v)
		}
		return nil
	})
}

// blend composites a straight color at coverage cov over a premultiplied pixel.
func (r *raster) blend(i int, c rgba, cov float32) {
	a := float32(c.a) / 255 * cov
	if a <= 0 {
		return
	}
	inv := 1 - a
	p := r.pix[i : i+4 : i+4]
	p[0] = uint8(float32(c.r)*a + float32(p[0])*inv + 0.5)
	p[1] = uint8(float32(c.g)*a + float32(p[1])*inv + 0.5)
	p[2] = uint8(float32(c.b)*a + float32(p[2])*inv + 0.5)
	p[3] = uint8(255*a + float32(p[3])*inv + 0.5)
}

func (r *raster) chargeMask(n int64) error {
	if n > r.b.maxMaskPixels-r.b.maskPixels {
		return fmt.Errorf("%w: metafile clip masks", render.ErrLimit)
	}
	r.b.maskPixels += n
	return nil
}

// polyMask rasterizes contours into a mask over the clip's rectangle bounds.
func (r *raster) polyMask(contours [][]point, evenOdd bool, within *clip) (*mask, error) {
	x0, y0 := int(math.Floor(math.Max(0, within.x0))), int(math.Floor(math.Max(0, within.y0)))
	x1, y1 := int(math.Ceil(math.Min(float64(r.w), within.x1))), int(math.Ceil(math.Min(float64(r.h), within.y1)))
	if x1 <= x0 || y1 <= y0 {
		return &mask{}, nil
	}
	w, h := x1-x0, y1-y0
	if err := r.chargeMask(int64(w) * int64(h)); err != nil {
		return nil, err
	}
	m := &mask{x0: x0, y0: y0, w: w, h: h, a: make([]uint8, w*h)}
	// Only the rectangle bounds the scan: the coverage of a mask is the
	// polygon's alone.
	bounds := &clip{x0: float64(x0), y0: float64(y0), x1: float64(x1), y1: float64(y1)}
	err := r.cover(contours, evenOdd, bounds, func(y, xs int, cov []float32) error {
		row := m.a[(y-y0)*w:]
		for i, v := range cov {
			row[xs+i-x0] = uint8(math.Min(255, float64(v)*255+0.5))
		}
		return nil
	})
	return m, err
}

// axisRect reports whether the contours are exactly one axis-aligned rectangle
// in the raster's pixel space, returning its corners.
func (r *raster) axisRect(contours [][]point) (x0, y0, x1, y1 float64, ok bool) {
	if len(contours) != 1 || len(contours[0]) != 4 {
		return
	}
	c := contours[0]
	p := [4]point{c[0], c[1], c[2], c[3]}
	const eps = 1e-9
	horizontalFirst := math.Abs(p[0].y-p[1].y) < eps && math.Abs(p[1].x-p[2].x) < eps && math.Abs(p[2].y-p[3].y) < eps && math.Abs(p[3].x-p[0].x) < eps
	verticalFirst := math.Abs(p[0].x-p[1].x) < eps && math.Abs(p[1].y-p[2].y) < eps && math.Abs(p[2].x-p[3].x) < eps && math.Abs(p[3].y-p[0].y) < eps
	if !horizontalFirst && !verticalFirst {
		return
	}
	x0, x1 = math.Min(p[0].x, p[2].x), math.Max(p[0].x, p[2].x)
	y0, y1 = math.Min(p[0].y, p[2].y), math.Max(p[0].y, p[2].y)
	return x0, y0, x1, y1, true
}

// contourBounds is the pixel-space bounding box of the contours.
func (r *raster) contourBounds(contours [][]point) (x0, y0, x1, y1 float64, ok bool) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range contours {
		for _, q := range c {
			p := q
			if !finite(p.x) || !finite(p.y) {
				return 0, 0, 0, 0, false
			}
			x0, x1 = math.Min(x0, p.x), math.Max(x1, p.x)
			y0, y1 = math.Min(y0, p.y), math.Max(y1, p.y)
		}
	}
	return x0, y0, x1, y1, x1 >= x0
}

// coverBuf is the coverage of a box of pixels, without a clip.
type coverBuf struct {
	x0, y0, w, h int
	a            []float32
}

// newCoverage allocates a coverage buffer over a clip's integer box, charged
// to the mask budget.
func (r *raster) newCoverage(area *clip) (*coverBuf, error) {
	x0, y0 := int(math.Floor(math.Max(0, area.x0))), int(math.Floor(math.Max(0, area.y0)))
	x1, y1 := int(math.Ceil(math.Min(float64(r.w), area.x1))), int(math.Ceil(math.Min(float64(r.h), area.y1)))
	if x1 <= x0 || y1 <= y0 {
		return &coverBuf{}, nil
	}
	w, h := x1-x0, y1-y0
	if err := r.chargeMask(int64(w) * int64(h)); err != nil {
		return nil, err
	}
	return &coverBuf{x0: x0, y0: y0, w: w, h: h, a: make([]float32, w*h)}, nil
}

// coverage rasterizes contours into a coverage buffer over the area's box.
func (r *raster) coverage(contours [][]point, evenOdd bool, area *clip) (*coverBuf, error) {
	buf, err := r.newCoverage(area)
	if err != nil || buf.w == 0 {
		return buf, err
	}
	bounds := &clip{x0: float64(buf.x0), y0: float64(buf.y0), x1: float64(buf.x0 + buf.w), y1: float64(buf.y0 + buf.h)}
	err = r.cover(contours, evenOdd, bounds, func(y, xs int, cov []float32) error {
		copy(buf.a[(y-buf.y0)*buf.w+xs-buf.x0:], cov)
		return nil
	})
	return buf, err
}

// paintCoverage composites a paint at a coverage buffer's coverage, under a
// clip.
func (r *raster) paintCoverage(buf *coverBuf, st fillStyle, c *clip) error {
	if buf.w == 0 {
		return nil
	}
	if err := r.b.visit(int64(buf.w) * int64(buf.h)); err != nil {
		return err
	}
	for y := 0; y < buf.h; y++ {
		if y%64 == 0 {
			if err := r.ctx.Err(); err != nil {
				return err
			}
		}
		py := buf.y0 + y
		for x := 0; x < buf.w; x++ {
			v := min(buf.a[y*buf.w+x], 1)
			if v <= 0 {
				continue
			}
			px := buf.x0 + x
			v *= c.coverage(px, py)
			if v <= 0 {
				continue
			}
			col := st.solid
			if st.fn != nil {
				col = st.fn(px, py)
			}
			r.blend((py*r.w+px)*4, col, v)
		}
	}
	return nil
}
