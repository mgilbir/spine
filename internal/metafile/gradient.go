package metafile

import (
	"fmt"
	"math"

	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

// gradient plays EMR_GRADIENTFILL: rectangles shaded along one axis, or
// triangles shaded between their corners, interpolating linearly in RGB.
func (it *interp) gradient(g gowemf.Gradient) error {
	step := 3
	if g.Mode == 0 || g.Mode == 1 {
		step = 2
	} else if g.Mode != 2 {
		return fmt.Errorf("%w: metafile: gradient mode", render.ErrInvalid)
	}
	m := it.matrix()
	toPixel := m.then(affine{a: it.r.sx, d: it.r.sy, e: it.r.ox, f: it.r.oy})
	inv, ok := toPixel.invert()
	if !ok {
		return nil
	}
	vertex := func(i uint32) (point, [3]float64, error) {
		if int(i) >= g.Vertices.Len() {
			return point{}, [3]float64{}, fmt.Errorf("%w: metafile: gradient vertex index", render.ErrInvalid)
		}
		v := g.Vertices.At(int(i))
		return point{v.Point.X, v.Point.Y}, [3]float64{float64(v.Red >> 8), float64(v.Green >> 8), float64(v.Blue >> 8)}, nil
	}
	for i := 0; i+step <= g.Indexes.Len(); i += step {
		if err := it.ctx.Err(); err != nil {
			return err
		}
		if err := it.b.op(); err != nil {
			return err
		}
		var pts [3]point
		var cols [3][3]float64
		for k := 0; k < step; k++ {
			var err error
			if pts[k], cols[k], err = vertex(g.Indexes.At(i + k)); err != nil {
				return err
			}
		}
		var contour []point
		var paint func(x, y int) rgba
		if step == 2 {
			x0, y0, x1, y1 := pts[0].x, pts[0].y, pts[1].x, pts[1].y
			contour = it.rectContour(x0, y0, x1, y1)
			horizontal := g.Mode == 0
			paint = func(px, py int) rgba {
				l := inv.apply(point{float64(px) + 0.5, float64(py) + 0.5})
				var t float64
				if horizontal && x1 != x0 {
					t = (l.x - x0) / (x1 - x0)
				} else if !horizontal && y1 != y0 {
					t = (l.y - y0) / (y1 - y0)
				}
				t = math.Min(1, math.Max(0, t))
				return rgba{clamp8(cols[0][0] + (cols[1][0]-cols[0][0])*t), clamp8(cols[0][1] + (cols[1][1]-cols[0][1])*t), clamp8(cols[0][2] + (cols[1][2]-cols[0][2])*t), 255}
			}
		} else {
			contour = []point{m.apply(pts[0]), m.apply(pts[1]), m.apply(pts[2])}
			den := (pts[1].y-pts[2].y)*(pts[0].x-pts[2].x) + (pts[2].x-pts[1].x)*(pts[0].y-pts[2].y)
			if den == 0 {
				continue
			}
			paint = func(px, py int) rgba {
				l := inv.apply(point{float64(px) + 0.5, float64(py) + 0.5})
				a := ((pts[1].y-pts[2].y)*(l.x-pts[2].x) + (pts[2].x-pts[1].x)*(l.y-pts[2].y)) / den
				b := ((pts[2].y-pts[0].y)*(l.x-pts[2].x) + (pts[0].x-pts[2].x)*(l.y-pts[2].y)) / den
				c := 1 - a - b
				a, b, c = math.Min(1, math.Max(0, a)), math.Min(1, math.Max(0, b)), math.Min(1, math.Max(0, c))
				s := a + b + c
				a, b, c = a/s, b/s, c/s
				return rgba{
					clamp8(cols[0][0]*a + cols[1][0]*b + cols[2][0]*c),
					clamp8(cols[0][1]*a + cols[1][1]*b + cols[2][1]*c),
					clamp8(cols[0][2]*a + cols[1][2]*b + cols[2][2]*c), 255}
			}
		}
		if err := it.r.fill([][]point{contour}, false, fillStyle{fn: paint}, it.dc.clip); err != nil {
			return err
		}
	}
	return nil
}
