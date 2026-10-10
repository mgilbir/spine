package metafile

import (
	"math"

	"github.com/mgilbir/gowemf"
)

// affine maps (x, y) to (a*x + c*y + e, b*x + d*y + f).
type affine struct{ a, b, c, d, e, f float64 }

func (m affine) apply(p point) point {
	return point{m.a*p.x + m.c*p.y + m.e, m.b*p.x + m.d*p.y + m.f}
}

// then is the transform that applies m, then n.
func (m affine) then(n affine) affine {
	return affine{
		a: n.a*m.a + n.c*m.b, b: n.b*m.a + n.d*m.b,
		c: n.a*m.c + n.c*m.d, d: n.b*m.c + n.d*m.d,
		e: n.a*m.e + n.c*m.f + n.e, f: n.b*m.e + n.d*m.f + n.f,
	}
}

func (m affine) finite() bool {
	return finite(m.a) && finite(m.b) && finite(m.c) && finite(m.d) && finite(m.e) && finite(m.f)
}

// singular returns the larger and smaller scale of the linear part.
func (m affine) singular() (big, small float64) {
	p := m.a*m.a + m.b*m.b + m.c*m.c + m.d*m.d
	q := math.Abs(m.a*m.d - m.b*m.c)
	disc := math.Sqrt(math.Max(0, p*p-4*q*q))
	big = math.Sqrt((p + disc) / 2)
	small = math.Sqrt(math.Max(0, (p-disc)/2))
	return
}

// invert returns the inverse of the transform.
func (m affine) invert() (affine, bool) {
	det := m.a*m.d - m.b*m.c
	if det == 0 || !finite(det) || math.Abs(det) < 1e-18 {
		return affine{}, false
	}
	inv := affine{
		a: m.d / det, b: -m.b / det, c: -m.c / det, d: m.a / det,
		e: (m.c*m.f - m.d*m.e) / det, f: (m.b*m.e - m.a*m.f) / det,
	}
	return inv, inv.finite()
}

// fromMatrix converts gowemf's row-vector matrix, where x' = x*M11 + y*M21 +
// Dx and y' = x*M12 + y*M22 + Dy.
func fromMatrix(m gowemf.Matrix) affine {
	return affine{a: m.M11, b: m.M12, c: m.M21, d: m.M22, e: m.Dx, f: m.Dy}
}

// figures flattens a destination-space path into polylines, curves within tol
// of their true shape. A figure is closed when PathClose ends it. The point
// count is charged to the budget as it grows.
func figures(p gowemf.Path, tol float64, charge func(int) error) ([]polyline, error) {
	var out []polyline
	var cur *polyline
	var start point // where the last figure began, for a figure resumed after a close
	haveStart := false
	pts := p.Points
	k := 0
	take := func(n int) ([]gowemf.Point, bool) {
		if k+n > len(pts) {
			return nil, false
		}
		s := pts[k : k+n]
		k += n
		return s, true
	}
	flush := func() {
		if cur != nil && len(cur.pts) > 0 {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, v := range p.Verbs {
		switch v {
		case gowemf.PathMoveTo:
			q, ok := take(1)
			if !ok {
				return nil, errPath
			}
			flush()
			cur = &polyline{pts: []point{{q[0].X, q[0].Y}}}
			start, haveStart = cur.pts[0], true
		case gowemf.PathLineTo:
			q, ok := take(1)
			if ok && cur == nil && haveStart {
				cur = &polyline{pts: []point{start}}
			}
			if !ok || cur == nil {
				return nil, errPath
			}
			if err := charge(1); err != nil {
				return nil, err
			}
			cur.pts = append(cur.pts, point{q[0].X, q[0].Y})
		case gowemf.PathCubicTo:
			q, ok := take(3)
			if ok && cur == nil && haveStart {
				cur = &polyline{pts: []point{start}}
			}
			if !ok || cur == nil {
				return nil, errPath
			}
			before := len(cur.pts)
			cur.pts = flattenCubic(cur.pts, cur.pts[len(cur.pts)-1], point{q[0].X, q[0].Y}, point{q[1].X, q[1].Y}, point{q[2].X, q[2].Y}, tol)
			if err := charge(len(cur.pts) - before); err != nil {
				return nil, err
			}
		case gowemf.PathClose:
			if cur != nil {
				// A figure drawn on after its close starts again where it
				// began, as a closed figure ends at its start.
				cur.closed = true
				flush()
			}
		default:
			return nil, errPath
		}
	}
	flush()
	for _, l := range out {
		for _, q := range l.pts {
			if !finite(q.x) || !finite(q.y) {
				return nil, errCoordinate
			}
		}
	}
	return out, nil
}

// fillContours are the figures of a path as contours to fill: every figure
// is closed for filling.
func fillContours(figs []polyline) [][]point {
	out := make([][]point, 0, len(figs))
	for _, l := range figs {
		if len(l.pts) >= 3 {
			out = append(out, l.pts)
		}
	}
	return out
}
