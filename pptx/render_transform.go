package pptx

import (
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/render"
)

// renderShapeTransform is a shape's flips and clockwise rotation about the centre
// of its box, in pixels. Flips apply first, as DrawingML defines.
type renderShapeTransform struct {
	flipH, flipV bool
	// rot is in degrees, clockwise.
	rot    float64
	cx, cy float64
}

func (t renderShapeTransform) identity() bool {
	return !t.flipH && !t.flipV && math.Mod(t.rot, 360) == 0
}

// point maps a point in pixels.
func (t renderShapeTransform) point(x, y float64) (float64, float64) {
	dx, dy := x-t.cx, y-t.cy
	if t.flipH {
		dx = -dx
	}
	if t.flipV {
		dy = -dy
	}
	a := t.rot * math.Pi / 180
	sin, cos := math.Sin(a), math.Cos(a)
	return t.cx + dx*cos - dy*sin, t.cy + dx*sin + dy*cos
}

// renderArcSteps is how finely a transformed arc is flattened, in degrees.
const renderArcSteps = 5.0

// path maps a path, flattening its arcs, whose ellipses a rotation does not
// keep axis-aligned. It returns the mapped path's bounds.
func (t renderShapeTransform) path(p layout.Path, maxSegments int, segments *int) (layout.Path, [4]float64, error) {
	out := make(layout.Path, 0, len(p))
	bounds := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	add := func(op layout.PathOp, x, y float64) error {
		if *segments++; *segments > maxSegments {
			return fmt.Errorf("%w: transformed path segments", render.ErrLimit)
		}
		x, y = t.point(x, y)
		px, okX := style.FromPx(x)
		py, okY := style.FromPx(y)
		if !okX || !okY {
			return fmt.Errorf("%w: transformed coordinate", render.ErrLimit)
		}
		bounds = [4]float64{math.Min(bounds[0], x), math.Min(bounds[1], y), math.Max(bounds[2], x), math.Max(bounds[3], y)}
		out = append(out, layout.PathSegment{Op: op, Point: layout.Point{X: px, Y: py}})
		return nil
	}
	for _, s := range p {
		var err error
		switch s.Op {
		case layout.MoveTo, layout.LineTo:
			err = add(s.Op, s.Point.X.Px(), s.Point.Y.Px())
		case layout.ArcTo:
			// The arc starts with a line from the current point to its
			// start.
			n := max(1, int(math.Ceil(math.Abs(s.SweepAngle)/renderArcSteps)))
			for i := 0; i <= n && err == nil; i++ {
				a := (s.StartAngle + s.SweepAngle*float64(i)/float64(n)) * math.Pi / 180
				err = add(layout.LineTo, s.Center.X.Px()+s.RadiusX.Px()*math.Cos(a), s.Center.Y.Px()+s.RadiusY.Px()*math.Sin(a))
			}
		case layout.ClosePath:
			out = append(out, s)
		default:
			err = fmt.Errorf("%w: path operation", render.ErrUnsupported)
		}
		if err != nil {
			return nil, bounds, err
		}
	}
	if len(out) > 0 && out[0].Op == layout.LineTo {
		out[0].Op = layout.MoveTo
	}
	return out, bounds, nil
}

// renderRectPath is a rectangle as a closed path.
func renderRectPath(r layout.Rect) layout.Path {
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	return layout.Path{
		{Op: layout.MoveTo, Point: layout.Point{X: x0, Y: y0}}, {Op: layout.LineTo, Point: layout.Point{X: x1, Y: y0}},
		{Op: layout.LineTo, Point: layout.Point{X: x1, Y: y1}}, {Op: layout.LineTo, Point: layout.Point{X: x0, Y: y1}}, {Op: layout.ClosePath},
	}
}

// ops maps a shape's drawing. Gradients keep their direction and are spread
// over the transformed shape's bounds, which best effort reports when the
// shape turns.
func (t renderShapeTransform) ops(ops []layout.Op, colors *renderColors, maxSegments int) ([]layout.Op, error) {
	if t.identity() {
		return ops, nil
	}
	segments := 0
	out := make([]layout.Op, 0, len(ops))
	gradient := func(f layout.FillGradient, clip layout.Path) (layout.Op, error) {
		path, b, err := t.path(clip, maxSegments, &segments)
		if err != nil {
			return nil, err
		}
		if math.Mod(t.rot, 360) != 0 {
			if err := colors.approximate(fmt.Errorf("%w: gradient drawn unrotated", render.ErrUnsupported)); err != nil {
				return nil, err
			}
		}
		x, okX := style.FromPx(b[0])
		y, okY := style.FromPx(b[1])
		w, okW := style.FromPx(b[2] - b[0])
		h, okH := style.FromPx(b[3] - b[1])
		if !okX || !okY || !okW || !okH || w <= 0 || h <= 0 {
			return nil, fmt.Errorf("%w: transformed gradient", render.ErrLimit)
		}
		// The gradient's geometry is relative to its tile; keep it where it
		// was on the page, flipped with the shape.
		g := f.Gradient
		move := func(p layout.Point) layout.Point {
			px, py := f.Tile.X.Px()+p.X.Px(), f.Tile.Y.Px()+p.Y.Px()
			if t.flipH {
				px = 2*t.cx - px
			}
			if t.flipV {
				py = 2*t.cy - py
			}
			nx, _ := style.FromPx(px - b[0])
			ny, _ := style.FromPx(py - b[1])
			return layout.Point{X: nx, Y: ny}
		}
		g.Start, g.End, g.Center = move(g.Start), move(g.End), move(g.Center)
		rect := layout.Rect{X: x, Y: y, W: w, H: h}
		return layout.ClipPath{Path: path, Ops: []layout.Op{layout.FillGradient{Clip: rect, Tile: rect, StepX: w, StepY: h, Gradient: g}}}, nil
	}
	for _, op := range ops {
		switch v := op.(type) {
		case layout.FillRect:
			path, _, err := t.path(renderRectPath(v.Rect), maxSegments, &segments)
			if err != nil {
				return nil, err
			}
			out = append(out, layout.FillPath{Path: path, Color: v.Color})
		case layout.FillPath:
			if v.Clip.Active {
				return nil, fmt.Errorf("%w: clipped path", render.ErrUnsupported)
			}
			path, _, err := t.path(v.Path, maxSegments, &segments)
			if err != nil {
				return nil, err
			}
			out = append(out, layout.FillPath{Path: path, Color: v.Color})
		case layout.FillGradient:
			g, err := gradient(v, renderRectPath(v.Clip))
			if err != nil {
				return nil, err
			}
			out = append(out, g)
		case layout.ClipPath:
			if len(v.Ops) != 1 {
				return nil, fmt.Errorf("%w: clipped drawing", render.ErrUnsupported)
			}
			f, ok := v.Ops[0].(layout.FillGradient)
			if !ok {
				return nil, fmt.Errorf("%w: clipped drawing", render.ErrUnsupported)
			}
			g, err := gradient(f, v.Path)
			if err != nil {
				return nil, err
			}
			out = append(out, g)
		default:
			return nil, fmt.Errorf("%w: transformed %T", render.ErrUnsupported, op)
		}
	}
	return out, nil
}
