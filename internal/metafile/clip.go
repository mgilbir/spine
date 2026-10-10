package metafile

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

// regionKey names the coverage of a clip region node translated by an
// offset: an Offset step moves everything beneath it, so the same node can be
// needed at more than one place.
type regionKey struct {
	n      *gowemf.ClipRegion
	dx, dy float64
}

// clip returns the coverage of a clip: the intersection of its regions, the
// whole raster when it has none.
func (be *backend) clip(c gowemf.Clip) (*clip, error) {
	if len(c) == 0 {
		return be.r.fullClip(), nil
	}
	if len(c) == 1 {
		return be.region(c[0], point{})
	}
	var key strings.Builder
	for _, n := range c {
		id, ok := be.regionID[n]
		if !ok {
			id = len(be.regionID) + 1
			be.regionID[n] = id
		}
		key.WriteString(strconv.Itoa(id))
		key.WriteByte(',')
	}
	if out, ok := be.clips[key.String()]; ok {
		return out, nil
	}
	out := be.r.fullClip()
	for _, n := range c {
		rc, err := be.region(n, point{})
		if err != nil {
			return nil, err
		}
		if out, err = be.r.intersect(out, rc); err != nil {
			return nil, err
		}
	}
	be.clips[key.String()] = out
	return out, nil
}

// region returns the coverage of a clip region node translated by off. A nil
// node is the whole drawing surface.
func (be *backend) region(n *gowemf.ClipRegion, off point) (*clip, error) {
	if n == nil {
		return be.r.fullClip(), nil
	}
	key := regionKey{n, off.x, off.y}
	if c, ok := be.regions[key]; ok {
		return c, nil
	}
	if err := be.ctx.Err(); err != nil {
		return nil, err
	}
	var out *clip
	var err error
	if n.Op == gowemf.ClipOffset {
		out, err = be.region(n.Base, point{off.x + n.Offset.X, off.y + n.Offset.Y})
	} else {
		out, err = be.combine(n, off)
	}
	if err != nil {
		return nil, err
	}
	be.regions[key] = out
	return out, nil
}

// operand is the region a node combines with its base: a region tree, or an
// area filled by a rule.
func (be *backend) operand(n *gowemf.ClipRegion, off point) (*clip, error) {
	if n.Operand != nil {
		return be.region(n.Operand, off)
	}
	figs, err := figures(n.Area, be.tol, be.charge)
	if err != nil {
		return nil, err
	}
	contours := fillContours(figs)
	if off.x != 0 || off.y != 0 {
		for _, c := range contours {
			for i := range c {
				c[i] = point{c[i].x + off.x, c[i].y + off.y}
			}
		}
	}
	return be.r.areaClip(contours, n.Rule != gowemf.NonZero)
}

func (be *backend) combine(n *gowemf.ClipRegion, off point) (*clip, error) {
	op, err := be.operand(n, off)
	if err != nil {
		return nil, err
	}
	if n.Op == gowemf.ClipReplace {
		return op, nil
	}
	base, err := be.region(n.Base, off)
	if err != nil {
		return nil, err
	}
	switch n.Op {
	case gowemf.ClipIntersect:
		return be.r.intersect(base, op)
	case gowemf.ClipUnion:
		return be.r.union(base, op)
	case gowemf.ClipXor:
		return be.r.xor(base, op)
	case gowemf.ClipDifference:
		return be.r.subtract(base, op)
	case gowemf.ClipComplement:
		return be.r.subtract(op, base)
	}
	return nil, fmt.Errorf("%w: metafile: clip operation %d", render.ErrInvalid, n.Op)
}

// within is the clip restricted to a pixel-space box.
func (c *clip) within(x0, y0, x1, y1 float64) *clip {
	n := *c
	n.x0, n.y0 = math.Max(c.x0, math.Floor(x0)), math.Max(c.y0, math.Floor(y0))
	n.x1, n.y1 = math.Min(c.x1, math.Ceil(x1)), math.Min(c.y1, math.Ceil(y1))
	return &n
}

// rectOnly reports whether the clip is a plain rectangle.
func (c *clip) rectOnly() bool { return c.m == nil }

// areaClip is the coverage of contours filled by a rule: an exact rectangle
// when they are one axis-aligned rectangle, otherwise a mask over their
// bounds.
func (r *raster) areaClip(contours [][]point, evenOdd bool) (*clip, error) {
	if x0, y0, x1, y1, ok := r.axisRect(contours); ok {
		return &clip{x0: math.Max(0, x0), y0: math.Max(0, y0), x1: math.Min(float64(r.w), x1), y1: math.Min(float64(r.h), y1)}, nil
	}
	bx0, by0, bx1, by1, ok := r.contourBounds(contours)
	if !ok {
		return &clip{}, nil
	}
	within := r.fullClip().within(bx0, by0, bx1, by1)
	if within.empty() {
		return &clip{}, nil
	}
	m, err := r.polyMask(contours, evenOdd, within)
	if err != nil {
		return nil, err
	}
	return &clip{x0: float64(m.x0), y0: float64(m.y0), x1: float64(m.x0 + m.w), y1: float64(m.y0 + m.h), m: m}, nil
}

// maskOver builds the clip over a box whose coverage is f of two clips'.
func (r *raster) maskOver(x0, y0, x1, y1 float64, a, b *clip, f func(ca, cb float32) float32) (*clip, error) {
	box := r.fullClip().within(x0, y0, x1, y1)
	if box.empty() {
		return &clip{}, nil
	}
	mx0, my0 := int(box.x0), int(box.y0)
	w, h := int(box.x1)-mx0, int(box.y1)-my0
	if err := r.chargeMask(int64(w) * int64(h)); err != nil {
		return nil, err
	}
	m := &mask{x0: mx0, y0: my0, w: w, h: h, a: make([]uint8, w*h)}
	for y := 0; y < h; y++ {
		if y%64 == 0 {
			if err := r.ctx.Err(); err != nil {
				return nil, err
			}
		}
		for x := 0; x < w; x++ {
			v := f(a.coverage(mx0+x, my0+y), b.coverage(mx0+x, my0+y))
			m.a[y*w+x] = uint8(math.Min(255, math.Max(0, float64(v)*255+0.5)))
		}
	}
	return &clip{x0: box.x0, y0: box.y0, x1: box.x1, y1: box.y1, m: m}, nil
}

func (r *raster) intersect(a, b *clip) (*clip, error) {
	x0, y0 := math.Max(a.x0, b.x0), math.Max(a.y0, b.y0)
	x1, y1 := math.Min(a.x1, b.x1), math.Min(a.y1, b.y1)
	if !(x1 > x0 && y1 > y0) {
		return &clip{}, nil
	}
	switch {
	case a.rectOnly() && b.rectOnly():
		return &clip{x0: x0, y0: y0, x1: x1, y1: y1}, nil
	case b.rectOnly():
		n := *a
		n.x0, n.y0, n.x1, n.y1 = x0, y0, x1, y1
		return &n, nil
	case a.rectOnly():
		n := *b
		n.x0, n.y0, n.x1, n.y1 = x0, y0, x1, y1
		return &n, nil
	}
	return r.maskOver(x0, y0, x1, y1, a, b, func(ca, cb float32) float32 { return ca * cb })
}

func (r *raster) union(a, b *clip) (*clip, error) {
	switch {
	case b.empty():
		return a, nil
	case a.empty():
		return b, nil
	case a.rectOnly() && b.rectOnly() && contains(a, b):
		return a, nil
	case a.rectOnly() && b.rectOnly() && contains(b, a):
		return b, nil
	}
	return r.maskOver(math.Min(a.x0, b.x0), math.Min(a.y0, b.y0), math.Max(a.x1, b.x1), math.Max(a.y1, b.y1), a, b,
		func(ca, cb float32) float32 { return ca + cb - ca*cb })
}

func (r *raster) xor(a, b *clip) (*clip, error) {
	switch {
	case b.empty():
		return a, nil
	case a.empty():
		return b, nil
	}
	return r.maskOver(math.Min(a.x0, b.x0), math.Min(a.y0, b.y0), math.Max(a.x1, b.x1), math.Max(a.y1, b.y1), a, b,
		func(ca, cb float32) float32 { return ca + cb - 2*ca*cb })
}

// subtract is a minus b.
func (r *raster) subtract(a, b *clip) (*clip, error) {
	if a.empty() || b.empty() || !(math.Min(a.x1, b.x1) > math.Max(a.x0, b.x0) && math.Min(a.y1, b.y1) > math.Max(a.y0, b.y0)) {
		return a, nil
	}
	return r.maskOver(a.x0, a.y0, a.x1, a.y1, a, b, func(ca, cb float32) float32 { return ca * (1 - cb) })
}

// contains reports whether rectangle a covers rectangle b.
func contains(a, b *clip) bool {
	return a.x0 <= b.x0 && a.y0 <= b.y0 && a.x1 >= b.x1 && a.y1 >= b.y1
}
