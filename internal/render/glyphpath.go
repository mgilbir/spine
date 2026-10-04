package render

import (
	"context"
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
)

// glyphPathTolerance is how far a flattened glyph curve may stray from the
// outline, in CSS pixels: a sixteenth of a pixel at four times 96 DPI.
const glyphPathTolerance = 1.0 / 64

// GlyphPaths outlines a glyph run as one path per inked glyph, each point
// mapped through at, so that callers can draw text turned or moved as a path
// is. Curves are flattened before mapping, which keeps the tolerance under a
// rotation. Fonts fill their outlines by the nonzero rule and paths by the
// even-odd rule; exact is false when some glyph's contours cross, or nest
// with a winding that the two rules fill apart, so the paths are not the
// glyphs' exact shape. segments counts the path segments made against
// maxSegments.
func GlyphPaths(ctx context.Context, v layout.DrawGlyphs, at func(x, y float64) (float64, float64), maxSegments int, segments *int) (paths []layout.Path, exact bool, err error) {
	if v.Face == nil || v.Face.UnitsPerEm() <= 0 || v.Size <= 0 || !validColor(v.Color) || at == nil || segments == nil {
		return nil, false, fmt.Errorf("%w: glyph run", ErrInvalid)
	}
	if v.Clip.Active {
		return nil, false, fmt.Errorf("%w: clipped glyph run outline", ErrUnsupported)
	}
	exact = true
	size, upem := v.Size.Px(), float64(v.Face.UnitsPerEm())
	pen := 0.0
	for _, glyph := range v.Glyphs {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if glyph.GID <= 0 || glyph.GID >= v.Face.NumGlyphs() {
			return nil, false, fmt.Errorf("%w: invalid/missing glyph %d", ErrUnsupported, glyph.GID)
		}
		if !finite(glyph.XAdvance) || !finite(glyph.XOffset) || !finite(glyph.YOffset) || glyph.YAdvance != 0 || glyph.VOriginX != 0 || glyph.VOriginY != 0 {
			return nil, false, fmt.Errorf("%w: glyph placement", ErrInvalid)
		}
		ox := v.At.X.Px() + (pen+glyph.XOffset)*size/1000
		oy := v.At.Y.Px() - glyph.YOffset*size/1000
		var (
			curves        []curve
			callbackError error
		)
		err := v.Face.GlyphOutline(glyph.GID, func(s shape.Segment) bool {
			if len(curves) >= maxSegments-*segments {
				callbackError = fmt.Errorf("%w: glyph segments", ErrLimit)
				return false
			}
			var c curve
			n := 1
			switch s.Op {
			case shape.MoveTo:
				c.op = 'M'
			case shape.LineTo:
				c.op = 'L'
			case shape.QuadTo:
				c.op, n = 'Q', 2
			case shape.CubicTo:
				c.op, n = 'C', 3
			default:
				callbackError = fmt.Errorf("%w: glyph segment", ErrUnsupported)
				return false
			}
			for i := 0; i < n; i++ {
				x, y := ox+s.Pts[i].X*size/upem, oy-s.Pts[i].Y*size/upem
				if !finite(x) || !finite(y) || math.Abs(x) > style.MaxUnit.Px() || math.Abs(y) > style.MaxUnit.Px() {
					callbackError = fmt.Errorf("%w: glyph coordinate range", ErrLimit)
					return false
				}
				c.pts[i] = point{x, y}
			}
			curves = append(curves, c)
			return true
		})
		if callbackError != nil {
			return nil, false, callbackError
		}
		if err != nil {
			return nil, false, fmt.Errorf("%w: glyph outline: %w", ErrUnsupported, err)
		}
		pen += glyph.XAdvance
		if !finite(pen) {
			return nil, false, fmt.Errorf("%w: glyph advance", ErrLimit)
		}
		if len(curves) == 0 {
			continue
		}
		// Flattening works at a scale where its sixteenth of a pixel is the
		// tolerance.
		scale := 1 / (16 * glyphPathTolerance)
		remaining := maxSegments - *segments
		edges, err := flattenCurves(ctx, curves, scale, &remaining)
		if err != nil {
			return nil, false, err
		}
		contours := glyphContours(edges, scale)
		if !evenOddMatchesNonzero(contours) {
			exact = false
		}
		var path layout.Path
		for _, contour := range contours {
			for i, p := range contour {
				if *segments >= maxSegments {
					return nil, false, fmt.Errorf("%w: glyph path segments", ErrLimit)
				}
				*segments++
				x, y := at(p.x, p.y)
				px, okX := style.FromPx(x)
				py, okY := style.FromPx(y)
				if !okX || !okY {
					return nil, false, fmt.Errorf("%w: glyph coordinate range", ErrLimit)
				}
				op := layout.LineTo
				if i == 0 {
					op = layout.MoveTo
				}
				path = append(path, layout.PathSegment{Op: op, Point: layout.Point{X: px, Y: py}})
			}
			path = append(path, layout.PathSegment{Op: layout.ClosePath})
		}
		if len(path) > 0 {
			paths = append(paths, path)
		}
	}
	return paths, exact, nil
}

// glyphContours splits flattened edges, in order, back into closed polygons
// in unscaled pixels. flattenCurves joins each contour's edges end to start
// and closes it, so a contour ends at the edge returning to its first point.
func glyphContours(edges []edge, scale float64) [][]point {
	var (
		contours [][]point
		current  []point
		start    point
	)
	for _, e := range edges {
		if len(current) == 0 {
			start = e.a
			current = append(current, point{e.a.x / scale, e.a.y / scale})
		}
		if e.b == start {
			if len(current) >= 3 {
				contours = append(contours, current)
			}
			current = nil
			continue
		}
		current = append(current, point{e.b.x / scale, e.b.y / scale})
	}
	if len(current) >= 3 {
		contours = append(contours, current)
	}
	return contours
}

// evenOddMatchesNonzero reports whether the even-odd rule fills a glyph's
// contours as the nonzero rule does. Crossing contours never count as
// matching. Otherwise each contour's inside, less the contours nested in it,
// has a winding of the orientations of the contours enclosing it, itself
// included, and a parity of their number; the rules agree when each such
// region is filled by both or by neither.
func evenOddMatchesNonzero(contours [][]point) bool {
	if len(contours) < 2 {
		return true
	}
	// A glyph has few contours; each pair is compared edge by edge.
	for i := range contours {
		for j := i + 1; j < len(contours); j++ {
			if contoursCross(contours[i], contours[j]) {
				return false
			}
		}
	}
	orientation := make([]int, len(contours))
	for i, c := range contours {
		area := 0.0
		for k := range c {
			a, b := c[k], c[(k+1)%len(c)]
			area += a.x*b.y - b.x*a.y
		}
		switch {
		case area > 0:
			orientation[i] = 1
		case area < 0:
			orientation[i] = -1
		}
	}
	for i, c := range contours {
		winding, depth := orientation[i], 1
		for j, d := range contours {
			if j != i && polygonContains(d, c[0]) {
				winding += orientation[j]
				depth++
			}
		}
		if (winding != 0) != (depth%2 == 1) {
			return false
		}
	}
	return true
}

func contoursCross(a, b []point) bool {
	for i := range a {
		p, q := a[i], a[(i+1)%len(a)]
		for j := range b {
			r, s := b[j], b[(j+1)%len(b)]
			if segmentsCross(p, q, r, s) {
				return true
			}
		}
	}
	return false
}

// segmentsCross reports whether two segments meet, touching included.
func segmentsCross(p, q, r, s point) bool {
	cross := func(o, a, b point) float64 { return (a.x-o.x)*(b.y-o.y) - (a.y-o.y)*(b.x-o.x) }
	d1, d2 := cross(r, s, p), cross(r, s, q)
	d3, d4 := cross(p, q, r), cross(p, q, s)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	on := func(o, a, b point, d float64) bool {
		return d == 0 && math.Min(o.x, a.x) <= b.x && b.x <= math.Max(o.x, a.x) && math.Min(o.y, a.y) <= b.y && b.y <= math.Max(o.y, a.y)
	}
	return on(r, s, p, d1) || on(r, s, q, d2) || on(p, q, r, d3) || on(p, q, s, d4)
}

// polygonContains casts a ray to the right of p, by the even-odd rule.
func polygonContains(polygon []point, p point) bool {
	inside := false
	for i := range polygon {
		a, b := polygon[i], polygon[(i+1)%len(polygon)]
		if (a.y > p.y) != (b.y > p.y) && p.x < a.x+(p.y-a.y)*(b.x-a.x)/(b.y-a.y) {
			inside = !inside
		}
	}
	return inside
}
