package metafile

import (
	"image/color"
	"math"

	"github.com/mgilbir/gowemf"
)

// FillGradient fills a mesh of Gouraud-shaded triangles: each point's color
// interpolates its triangle's corner colors. The triangles of a mesh share
// edges, and painting each with its own edge coverage would leave seams where
// two half-covered pixels do not add up to one; so their coverage and color are
// accumulated together and the mesh is composited once.
func (be *backend) FillGradient(mesh []gowemf.GradientTriangle, cl gowemf.Clip) error {
	if err := be.begin(); err != nil {
		return err
	}
	if len(mesh) == 0 {
		return nil
	}
	if err := be.charge(3 * len(mesh)); err != nil {
		return err
	}
	all := make([][]point, 0, len(mesh))
	for _, t := range mesh {
		tri := []point{{t.Points[0].X, t.Points[0].Y}, {t.Points[1].X, t.Points[1].Y}, {t.Points[2].X, t.Points[2].Y}}
		for _, q := range tri {
			if !finite(q.x) || !finite(q.y) {
				return errCoordinate
			}
		}
		all = append(all, tri)
	}
	c, err := be.clip(cl)
	if err != nil {
		return err
	}
	x0, y0, x1, y1, ok := be.r.contourBounds(all)
	if !ok {
		return nil
	}
	area := c.within(x0, y0, x1, y1)
	if area.empty() {
		return nil
	}
	cov, err := be.r.newCoverage(area)
	if err != nil || cov.w == 0 {
		return err
	}
	// Premultiplied color sums, weighted by coverage, in 0..1.
	if err = be.r.chargeMask(4 * int64(cov.w) * int64(cov.h)); err != nil {
		return err
	}
	sum := make([]float32, 4*cov.w*cov.h)
	bounds := &clip{x0: float64(cov.x0), y0: float64(cov.y0), x1: float64(cov.x0 + cov.w), y1: float64(cov.y0 + cov.h)}
	for k, t := range mesh {
		if err = be.ctx.Err(); err != nil {
			return err
		}
		tri := all[k]
		shade := gouraud(tri, t.Colors)
		err = be.r.cover([][]point{tri}, false, bounds, func(y, xs int, row []float32) error {
			base := (y - cov.y0) * cov.w
			for i, v := range row {
				if v <= 0 {
					continue
				}
				x := xs + i
				j := base + x - cov.x0
				cov.a[j] += v
				r, g, b, a := shade(float64(x)+0.5, float64(y)+0.5)
				sum[4*j] += v * float32(r*a)
				sum[4*j+1] += v * float32(g*a)
				sum[4*j+2] += v * float32(b*a)
				sum[4*j+3] += v * float32(a)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return be.r.paintCoverage(cov, fillStyle{fn: func(x, y int) rgba {
		j := (y-cov.y0)*cov.w + x - cov.x0
		n := cov.a[j]
		if n <= 0 {
			return rgba{}
		}
		a := sum[4*j+3] / n
		if a <= 0 {
			return rgba{}
		}
		return rgba{
			clamp8(float64(sum[4*j]/n/a) * 255), clamp8(float64(sum[4*j+1]/n/a) * 255),
			clamp8(float64(sum[4*j+2]/n/a) * 255), clamp8(float64(a) * 255),
		}
	}}, c)
}

// gouraud returns the straight color, in 0..1, of a point by interpolating a
// triangle's corner colors over it, each channel linearly. Points just
// outside, at its antialiased edge, take the color of the nearest point on it.
func gouraud(tri []point, cols [3]color.NRGBA64) func(x, y float64) (r, g, b, a float64) {
	p0, p1, p2 := tri[0], tri[1], tri[2]
	den := (p1.y-p2.y)*(p0.x-p2.x) + (p2.x-p1.x)*(p0.y-p2.y)
	ch := func(i int) [4]float64 {
		c := cols[i]
		return [4]float64{float64(c.R) / 0xffff, float64(c.G) / 0xffff, float64(c.B) / 0xffff, float64(c.A) / 0xffff}
	}
	c0, c1, c2 := ch(0), ch(1), ch(2)
	return func(x, y float64) (r, g, b, a float64) {
		w0, w1, w2 := 1.0/3, 1.0/3, 1.0/3
		if den != 0 && finite(den) {
			w0 = ((p1.y-p2.y)*(x-p2.x) + (p2.x-p1.x)*(y-p2.y)) / den
			w1 = ((p2.y-p0.y)*(x-p2.x) + (p0.x-p2.x)*(y-p2.y)) / den
			w0, w1 = math.Min(1, math.Max(0, w0)), math.Min(1, math.Max(0, w1))
			w2 = math.Max(0, 1-w0-w1)
			if s := w0 + w1 + w2; s > 0 {
				w0, w1, w2 = w0/s, w1/s, w2/s
			}
		}
		r = c0[0]*w0 + c1[0]*w1 + c2[0]*w2
		g = c0[1]*w0 + c1[1]*w1 + c2[1]*w2
		b = c0[2]*w0 + c1[2]*w1 + c2[2]*w2
		a = c0[3]*w0 + c1[3]*w1 + c2[3]*w2
		return r, g, b, a
	}
}
