package render

import (
	"context"
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
)

type geometry struct {
	path    layout.Path
	curves  []curve
	nonzero bool
	bounds  rectangle
}
type drawing struct {
	rect     rectangle
	path     *geometry
	clips    []*geometry
	text     string
	fontID   string
	image    *bitmap
	imageBox rectangle
}
type prepareBudget struct {
	operations, segments, glyphs, textBytes       int
	fontBytes, shapeWork, imagePixels, imageBytes int64
	images                                        map[image.Image]*bitmap
	faces                                         map[*shape.Face]*shape.Face
	fontIDs                                       map[*shape.Face]string
}

func (p *Page) collect(ctx context.Context, ops []layout.Op, clips []*geometry, budget *prepareBudget) error {
	if len(clips) > p.limits.MaxClipDepth {
		return fmt.Errorf("%w: clip depth", ErrLimit)
	}
	if len(ops) > p.limits.MaxOperations-budget.operations {
		return fmt.Errorf("%w: operation count", ErrLimit)
	}
	budget.operations += len(ops)
	for i, op := range ops {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch v := op.(type) {
		case layout.ClipPath:
			g, err := p.copyPath(ctx, v.Path, budget)
			if err != nil {
				return err
			}
			inner := append(append([]*geometry(nil), clips...), g)
			if err := p.collect(ctx, v.Ops, inner, budget); err != nil {
				return err
			}
		case layout.FillRect:
			if v.Rect.W < 0 || v.Rect.H < 0 || !validColor(v.Color) {
				return fmt.Errorf("%w: rectangle %d", ErrInvalid, i)
			}
			r := rectangle{v.Rect.X.Px(), v.Rect.Y.Px(), v.Rect.X.Px() + v.Rect.W.Px(), v.Rect.Y.Px() + v.Rect.H.Px(), v.Color}
			r = meet(r, rectangle{0, 0, p.width, p.height, style.RGBA{}})
			p.draws = append(p.draws, drawing{rect: r, clips: clips})
		case layout.DrawImage:
			if err := p.collectImage(ctx, v, clips, budget); err != nil {
				return err
			}
		case layout.DrawText:
			if err := p.collectText(ctx, v, clips, budget); err != nil {
				return err
			}
		case layout.DrawGlyphs:
			if err := p.collectGlyphs(ctx, v, clips, budget); err != nil {
				return err
			}
		case layout.FillPath:
			if !validColor(v.Color) {
				return fmt.Errorf("%w: path color", ErrInvalid)
			}
			g, err := p.copyPath(ctx, v.Path, budget)
			if err != nil {
				return err
			}
			r := meet(g.bounds, rectangle{0, 0, p.width, p.height, style.RGBA{}})
			if v.Clip.Active {
				if v.Clip.Rect.W < 0 || v.Clip.Rect.H < 0 {
					return fmt.Errorf("%w: path clip", ErrInvalid)
				}
				c := v.Clip.Rect
				r = meet(r, rectangle{c.X.Px(), c.Y.Px(), c.X.Px() + c.W.Px(), c.Y.Px() + c.H.Px(), style.RGBA{}})
			}
			r.color = v.Color
			p.draws = append(p.draws, drawing{rect: r, path: g, clips: clips})
		default:
			return fmt.Errorf("%w: operation %d (%T)", ErrUnsupported, i, op)
		}
	}
	return nil
}

func meet(a, b rectangle) rectangle {
	a.x0, a.y0 = math.Max(a.x0, b.x0), math.Max(a.y0, b.y0)
	a.x1, a.y1 = math.Max(a.x0, math.Min(a.x1, b.x1)), math.Max(a.y0, math.Min(a.y1, b.y1))
	return a
}

func (p *Page) copyPath(ctx context.Context, source layout.Path, budget *prepareBudget) (*geometry, error) {
	if len(source) > p.limits.MaxPathSegments-budget.segments {
		return nil, fmt.Errorf("%w: path segments", ErrLimit)
	}
	budget.segments += len(source)
	g := &geometry{bounds: rectangle{x0: math.Inf(1), y0: math.Inf(1), x1: math.Inf(-1), y1: math.Inf(-1)}}
	open := false
	bound := func(x, y float64) {
		g.bounds.x0, g.bounds.y0 = math.Min(g.bounds.x0, x), math.Min(g.bounds.y0, y)
		g.bounds.x1, g.bounds.y1 = math.Max(g.bounds.x1, x), math.Max(g.bounds.y1, y)
	}
	for _, s := range source {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch s.Op {
		case layout.MoveTo:
			open = true
			bound(s.Point.X.Px(), s.Point.Y.Px())
		case layout.LineTo:
			if !open {
				return nil, fmt.Errorf("%w: line without contour", ErrInvalid)
			}
			bound(s.Point.X.Px(), s.Point.Y.Px())
		case layout.ClosePath:
			if !open {
				return nil, fmt.Errorf("%w: close without contour", ErrInvalid)
			}
			open = false
		case layout.ArcTo:
			if s.RadiusX < 0 || s.RadiusY < 0 || !finite(s.StartAngle) || !finite(s.SweepAngle) || math.Abs(s.SweepAngle) > 360 {
				return nil, fmt.Errorf("%w: invalid ellipse arc", ErrInvalid)
			}
			s.StartAngle = math.Remainder(s.StartAngle, 360)
			open = true // Forme permits an arc to begin its own contour.
			bound(s.Center.X.Px()-s.RadiusX.Px(), s.Center.Y.Px()-s.RadiusY.Px())
			bound(s.Center.X.Px()+s.RadiusX.Px(), s.Center.Y.Px()+s.RadiusY.Px())
		default:
			return nil, fmt.Errorf("%w: path segment %d", ErrUnsupported, s.Op)
		}
		g.path = append(g.path, s)
	}
	if len(source) == 0 {
		g.bounds = rectangle{}
	}
	return g, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

type point struct{ x, y float64 }
type edge struct{ a, b point }
type interval struct{ lo, hi float64 }

// flatten uses an ellipse sagitta bound of 1/16 output pixel. A per-render
// segment budget prevents enormous radii or DPI from expanding without limit.
func flatten(ctx context.Context, g *geometry, scale float64, remaining *int) ([]edge, error) {
	if g.curves != nil {
		return flattenCurves(ctx, g.curves, scale, remaining)
	}
	var edges []edge
	var start, current point
	open := false
	line := func(next point) error {
		if current != next {
			if *remaining <= 0 {
				return fmt.Errorf("%w: flattened edges", ErrLimit)
			}
			*remaining--
			edges = append(edges, edge{current, next})
		}
		current = next
		return nil
	}
	closePath := func() error {
		if open {
			if err := line(start); err != nil {
				return err
			}
		}
		open = false
		return nil
	}
	for _, s := range g.path {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch s.Op {
		case layout.MoveTo:
			if err := closePath(); err != nil {
				return nil, err
			}
			current = point{s.Point.X.Px() * scale, s.Point.Y.Px() * scale}
			start = current
			open = true
		case layout.LineTo:
			if err := line(point{s.Point.X.Px() * scale, s.Point.Y.Px() * scale}); err != nil {
				return nil, err
			}
		case layout.ClosePath:
			if err := closePath(); err != nil {
				return nil, err
			}
		case layout.ArcTo:
			at := func(angle float64) point {
				a := angle * math.Pi / 180
				return point{(s.Center.X.Px() + s.RadiusX.Px()*math.Cos(a)) * scale, (s.Center.Y.Px() + s.RadiusY.Px()*math.Sin(a)) * scale}
			}
			first := at(s.StartAngle)
			if !open {
				current, start, open = first, first, true
			}
			if err := line(first); err != nil {
				return nil, err
			}
			radius := math.Max(s.RadiusX.Px(), s.RadiusY.Px()) * scale
			nf := math.Max(1, math.Ceil(math.Abs(s.SweepAngle)*math.Pi/180*math.Sqrt(radius/(8*(1.0/16)))))
			if !finite(nf) || nf > float64(*remaining) {
				return nil, fmt.Errorf("%w: arc subdivision", ErrLimit)
			}
			n := int(nf)
			for i := 1; i <= n; i++ {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if err := line(at(s.StartAngle + s.SweepAngle*float64(i)/float64(n))); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := closePath(); err != nil {
		return nil, err
	}
	return edges, nil
}

// scan returns even-odd fill intervals with a half-open vertex convention.
// Sorted crossings are paired, so reversed contours and holes agree.
type crossing struct {
	x       float64
	winding int
}

func scan(edges []edge, y float64, crossings []crossing, nonzero bool) ([]interval, []crossing) {
	crossings = crossings[:0]
	for _, e := range edges {
		a, b := e.a, e.b
		winding := 1
		if a.y > b.y {
			a, b = b, a
			winding = -1
		}
		if y >= a.y && y < b.y {
			crossings = append(crossings, crossing{a.x + (y-a.y)/(b.y-a.y)*(b.x-a.x), winding})
		}
	}
	sort.Slice(crossings, func(i, j int) bool { return crossings[i].x < crossings[j].x })
	var intervals []interval
	winding := 0
	var start float64
	for i := 0; i < len(crossings); {
		x := crossings[i].x
		before := winding
		for i < len(crossings) && crossings[i].x == x {
			if nonzero {
				winding += crossings[i].winding
			} else {
				winding ^= 1
			}
			i++
		}
		if before == 0 && winding != 0 {
			start = x
		}
		if before != 0 && winding == 0 && x > start {
			intervals = append(intervals, interval{start, x})
		}
	}
	return intervals, crossings
}

func intersect(a, b []interval) []interval {
	var out []interval
	for i, j := 0, 0; i < len(a) && j < len(b); {
		lo, hi := math.Max(a[i].lo, b[j].lo), math.Min(a[i].hi, b[j].hi)
		if hi > lo {
			out = append(out, interval{lo, hi})
		}
		if a[i].hi < b[j].hi {
			i++
		} else {
			j++
		}
	}
	return out
}
