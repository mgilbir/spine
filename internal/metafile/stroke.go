package metafile

import (
	"fmt"
	"math"

	"github.com/mgilbir/spine/render"
)

type capStyle int

const (
	capRound capStyle = iota
	capSquare
	capFlat
)

type joinStyle int

const (
	joinRound joinStyle = iota
	joinBevel
	joinMiter
)

// strokeStyle describes how a polyline is widened. Lengths are in the units
// of the polyline. cap ends an open polyline at its start and endCap at its
// end.
type strokeStyle struct {
	width      float64
	cap        capStyle
	endCap     capStyle
	join       joinStyle
	miter      float64
	dash       []float64
	dashOffset float64
}

// polyline is a flattened subpath.
type polyline struct {
	pts    []point
	closed bool
}

func sub(a, b point) point           { return point{a.x - b.x, a.y - b.y} }
func add(a, b point) point           { return point{a.x + b.x, a.y + b.y} }
func scale(a point, k float64) point { return point{a.x * k, a.y * k} }
func dot(a, b point) float64         { return a.x*b.x + a.y*b.y }
func cross(a, b point) float64       { return a.x*b.y - a.y*b.x }
func length(a point) float64         { return math.Hypot(a.x, a.y) }

// polyArea is a polygon's signed area, positive when it runs from +x to +y.
func polyArea(c []point) float64 {
	var s float64
	for i := range c {
		j := (i + 1) % len(c)
		s += c[i].x*c[j].y - c[j].x*c[i].y
	}
	return s / 2
}

// positive returns the polygon wound the way that makes every piece of a
// stroke add under the nonzero rule.
func positive(c []point) []point {
	if polyArea(c) < 0 {
		for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
			c[i], c[j] = c[j], c[i]
		}
	}
	return c
}

// arcPoints appends the points of the circle of radius r about c from angle a0
// through sweep, excluding the first point, in steps within tol of the arc.
func arcPoints(out []point, c point, r, a0, sweep, tol float64) []point {
	n := arcSteps(r, math.Abs(sweep), tol)
	for i := 1; i <= n; i++ {
		a := a0 + sweep*float64(i)/float64(n)
		out = append(out, point{c.x + r*math.Cos(a), c.y + r*math.Sin(a)})
	}
	return out
}

// arcSteps is how many chords keep a sweep of a circle of radius r within tol.
func arcSteps(r, sweep, tol float64) int {
	if r <= tol || sweep == 0 {
		return 1
	}
	step := 2 * math.Acos(1-tol/r)
	if !(step > 1e-3) {
		step = 1e-3
	}
	n := int(math.Ceil(sweep / step))
	return max(1, min(n, 1024))
}

// dedupe drops repeated points.
func dedupe(pts []point, closed bool) []point {
	out := make([]point, 0, len(pts))
	for _, p := range pts {
		if len(out) > 0 && out[len(out)-1] == p {
			continue
		}
		out = append(out, p)
	}
	if closed && len(out) > 1 && out[0] == out[len(out)-1] {
		out = out[:len(out)-1]
	}
	return out
}

// stroker accumulates the polygons of a stroke.
type stroker struct {
	st     strokeStyle
	hw     float64
	tol    float64
	out    [][]point
	budget func(n int) error
}

// strokeLines widens polylines into polygons that fill, by the nonzero rule,
// the stroke's area.
func strokeLines(lines []polyline, st strokeStyle, tol float64, budget func(n int) error) ([][]point, error) {
	if !(st.width > 0) || !finite(st.width) {
		return nil, nil
	}
	s := &stroker{st: st, hw: st.width / 2, tol: tol, budget: budget}
	if st.miter < 1 {
		s.st.miter = 1
	}
	for _, l := range lines {
		pts := dedupe(l.pts, l.closed)
		if len(st.dash) > 0 {
			if err := s.dashed(pts, l.closed); err != nil {
				return nil, err
			}
			continue
		}
		if err := s.line(pts, l.closed); err != nil {
			return nil, err
		}
	}
	return s.out, nil
}

func (s *stroker) emit(c []point) error {
	if err := s.budget(len(c)); err != nil {
		return err
	}
	s.out = append(s.out, positive(c))
	return nil
}

// line strokes one undashed polyline.
func (s *stroker) line(pts []point, closed bool) error {
	switch {
	case len(pts) == 1:
		// A zero-length line leaves a dot only where the caps are round.
		if s.st.cap == capRound && s.st.endCap == capRound {
			c := []point{{pts[0].x + s.hw, pts[0].y}}
			c = arcPoints(c, pts[0], s.hw, 0, 2*math.Pi, s.tol)
			return s.emit(c[:len(c)-1])
		}
		return nil
	case len(pts) < 2:
		return nil
	}
	if closed && len(pts) < 3 {
		closed = false
	}
	n := len(pts)
	segs := n - 1
	if closed {
		segs = n
	}
	dirs := make([]point, segs)
	for i := 0; i < segs; i++ {
		d := sub(pts[(i+1)%n], pts[i])
		dirs[i] = scale(d, 1/length(d))
		nrm := scale(point{-dirs[i].y, dirs[i].x}, s.hw)
		a, b := pts[i], pts[(i+1)%n]
		if err := s.emit([]point{add(a, nrm), add(b, nrm), sub(b, nrm), sub(a, nrm)}); err != nil {
			return err
		}
	}
	for i := 1; i < segs; i++ {
		if err := s.joinAt(pts[i], dirs[i-1], dirs[i]); err != nil {
			return err
		}
	}
	if closed {
		return s.joinAt(pts[0], dirs[segs-1], dirs[0])
	}
	if err := s.capAt(pts[0], scale(dirs[0], -1), s.st.cap); err != nil {
		return err
	}
	return s.capAt(pts[n-1], dirs[segs-1], s.st.endCap)
}

// capAt draws the cap at p of a line leaving it in direction out.
func (s *stroker) capAt(p, out point, cp capStyle) error {
	nrm := scale(point{-out.y, out.x}, s.hw)
	switch cp {
	case capSquare:
		ext := scale(out, s.hw)
		return s.emit([]point{add(p, nrm), add(add(p, nrm), ext), add(sub(p, nrm), ext), sub(p, nrm)})
	case capRound:
		a0 := math.Atan2(nrm.y, nrm.x)
		c := []point{add(p, nrm)}
		// The half turn from the left normal through out to the right one.
		sweep := -math.Pi
		if cross(nrm, out) > 0 {
			sweep = math.Pi
		}
		c = arcPoints(c, p, s.hw, a0, sweep, s.tol)
		return s.emit(c)
	}
	return nil
}

// joinAt fills the gap on the outer side of the turn from direction d0 to d1
// at v.
func (s *stroker) joinAt(v, d0, d1 point) error {
	cr, dt := cross(d0, d1), dot(d0, d1)
	if math.Abs(cr) < 1e-9 && dt > 0 {
		return nil
	}
	side := -1.0
	if cr < 0 {
		side = 1
	}
	n0 := scale(point{-d0.y, d0.x}, s.hw*side)
	n1 := scale(point{-d1.y, d1.x}, s.hw*side)
	a, b := add(v, n0), add(v, n1)
	turn := math.Atan2(math.Abs(cr), dt)
	switch s.st.join {
	case joinMiter:
		// The tip's distance over half the width is 1/cos(turn/2).
		if c := math.Cos(turn / 2); c > 1e-9 && 1/c <= s.st.miter {
			tip := add(v, scale(add(n0, n1), 1/(1+dt)))
			return s.emit([]point{v, a, tip, b})
		}
		return s.emit([]point{v, a, b})
	case joinRound:
		if turn > 0.2 {
			a0 := math.Atan2(n0.y, n0.x)
			sweep := turn
			if cross(n0, n1) < 0 {
				sweep = -turn
			}
			c := []point{v, a}
			c = arcPoints(c, v, s.hw, a0, sweep, s.tol)
			return s.emit(c)
		}
	}
	return s.emit([]point{v, a, b})
}

// dashed splits a polyline by the dash pattern and strokes the pieces.
func (s *stroker) dashed(pts []point, closed bool) error {
	pattern := s.st.dash
	var total float64
	for _, d := range pattern {
		if !(d >= 0) || !finite(d) {
			return fmt.Errorf("%w: pen dash length", render.ErrInvalid)
		}
		total += d
	}
	if !(total > 0) {
		return s.line(pts, closed)
	}
	if len(pattern)%2 == 1 {
		pattern = append(append([]float64(nil), pattern...), pattern...)
		total *= 2
	}
	if len(pts) < 2 {
		return s.line(pts, closed)
	}
	if closed && len(pts) >= 3 {
		pts = append(append([]point(nil), pts...), pts[0])
	}
	// Find where in the pattern the line begins.
	off := math.Mod(s.st.dashOffset, total)
	if off < 0 {
		off += total
	}
	idx := 0
	for off >= pattern[idx] {
		off -= pattern[idx]
		idx = (idx + 1) % len(pattern)
	}
	remain := pattern[idx] - off
	on := idx%2 == 0
	var cur []point
	flush := func() error {
		piece := dedupe(cur, false)
		cur = nil
		if len(piece) == 0 {
			return nil
		}
		return s.line(piece, false)
	}
	if on {
		cur = []point{pts[0]}
	}
	for i := 0; i+1 < len(pts); i++ {
		a, b := pts[i], pts[i+1]
		segLen := length(sub(b, a))
		if segLen == 0 {
			continue
		}
		pos := 0.0
		for {
			if segLen-pos <= remain {
				remain -= segLen - pos
				if on {
					cur = append(cur, b)
				}
				break
			}
			pos += remain
			p := add(a, scale(sub(b, a), pos/segLen))
			if on {
				cur = append(cur, p)
				if err := flush(); err != nil {
					return err
				}
			}
			on = !on
			idx = (idx + 1) % len(pattern)
			remain = pattern[idx]
			if on {
				cur = []point{p}
			}
			if err := s.budget(1); err != nil {
				return err
			}
		}
	}
	return flush()
}

// flattenCubic appends the points of the cubic Bézier curve p0..p3, excluding
// p0, within tol of the curve.
func flattenCubic(out []point, p0, p1, p2, p3 point, tol float64) []point {
	d1 := length(add(sub(p0, scale(p1, 2)), p2))
	d2 := length(add(sub(p1, scale(p2, 2)), p3))
	dd := math.Max(d1, d2)
	n := 1
	if dd > 0 && tol > 0 {
		n = int(math.Ceil(math.Sqrt(0.75 * dd / tol)))
	}
	n = max(1, min(n, 256))
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		out = append(out, point{a*p0.x + b*p1.x + c*p2.x + d*p3.x, a*p0.y + b*p1.y + c*p2.y + d*p3.y})
	}
	return out
}
