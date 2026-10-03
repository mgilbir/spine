package pptx

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderMaxGuides bounds a geometry's guide list.
const renderMaxGuides = 1024

// renderGuides holds evaluated DrawingML shape guides, in EMU or 60000ths of
// a degree, starting from the built-in guides of a w by h shape.
type renderGuides map[string]float64

func renderNewGuides(w, h float64) renderGuides {
	ss, ls := math.Min(w, h), math.Max(w, h)
	g := renderGuides{
		"w": w, "h": h, "l": 0, "t": 0, "r": w, "b": h, "hc": w / 2, "vc": h / 2, "ss": ss, "ls": ls,
		"cd2": 10800000, "cd4": 5400000, "cd8": 2700000, "3cd4": 16200000, "3cd8": 8100000, "5cd8": 13500000, "7cd8": 18900000,
	}
	for _, d := range []int{2, 3, 4, 5, 6, 8, 10, 12, 32} {
		g["wd"+strconv.Itoa(d)] = w / float64(d)
		g["hd"+strconv.Itoa(d)] = h / float64(d)
	}
	for _, d := range []int{2, 4, 6, 8, 16, 32} {
		g["ssd"+strconv.Itoa(d)] = ss / float64(d)
	}
	return g
}

// value is an operand: a guide name or an integer.
func (g renderGuides) value(tok string) (float64, error) {
	if v, ok := g[tok]; ok {
		return v, nil
	}
	n, err := strconv.ParseInt(tok, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: shape guide %q", render.ErrInvalid, tok)
	}
	return float64(n), nil
}

// eval evaluates guide lists in order; a later guide may use earlier ones.
// An adjust value already set, by the shape's own adjustments, is kept.
func (g renderGuides) eval(adjust bool, list []*dml.Gd) error {
	if len(list) > renderMaxGuides {
		return fmt.Errorf("%w: shape guides", render.ErrLimit)
	}
	for _, gd := range list {
		if gd == nil || gd.Name == "" {
			return fmt.Errorf("%w: shape guide", render.ErrInvalid)
		}
		if _, set := g[gd.Name]; adjust && set {
			continue
		}
		v, err := g.formula(gd.Fmla)
		if err != nil {
			return err
		}
		g[gd.Name] = v
	}
	return nil
}

// formula evaluates one guide formula (ECMA-376 §20.1.10.36).
func (g renderGuides) formula(fmla string) (float64, error) {
	f := strings.Fields(fmla)
	if len(f) == 0 {
		return 0, fmt.Errorf("%w: shape guide formula", render.ErrInvalid)
	}
	want := map[string]int{"*/": 3, "+-": 3, "+/": 3, "?:": 3, "abs": 1, "at2": 2, "cat2": 3, "cos": 2, "max": 2, "min": 2, "mod": 3, "pin": 3, "sat2": 3, "sin": 2, "sqrt": 1, "tan": 2, "val": 1}
	n, ok := want[f[0]]
	if !ok || len(f) != n+1 {
		return 0, fmt.Errorf("%w: shape guide formula %q", render.ErrInvalid, f[0])
	}
	var a [3]float64
	for i := range n {
		v, err := g.value(f[i+1])
		if err != nil {
			return 0, err
		}
		a[i] = v
	}
	rad := func(v float64) float64 { return v / 60000 * math.Pi / 180 }
	var v float64
	switch f[0] {
	case "*/":
		if a[2] == 0 {
			return 0, nil
		}
		v = a[0] * a[1] / a[2]
	case "+-":
		v = a[0] + a[1] - a[2]
	case "+/":
		if a[2] == 0 {
			return 0, nil
		}
		v = (a[0] + a[1]) / a[2]
	case "?:":
		v = a[2]
		if a[0] > 0 {
			v = a[1]
		}
	case "abs":
		v = math.Abs(a[0])
	case "at2":
		v = math.Atan2(a[1], a[0]) * 180 / math.Pi * 60000
	case "cat2":
		v = a[0] * math.Cos(math.Atan2(a[2], a[1]))
	case "cos":
		v = a[0] * math.Cos(rad(a[1]))
	case "max":
		v = math.Max(a[0], a[1])
	case "min":
		v = math.Min(a[0], a[1])
	case "mod":
		v = math.Sqrt(a[0]*a[0] + a[1]*a[1] + a[2]*a[2])
	case "pin":
		v = math.Max(a[0], math.Min(a[1], a[2]))
	case "sat2":
		v = a[0] * math.Sin(math.Atan2(a[2], a[1]))
	case "sin":
		v = a[0] * math.Sin(rad(a[1]))
	case "sqrt":
		v = math.Sqrt(math.Max(0, a[0]))
	case "tan":
		v = a[0] * math.Tan(rad(a[1]))
	case "val":
		v = a[0]
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("%w: shape guide value", render.ErrInvalid)
	}
	return v, nil
}

// renderSubpath is a flattened piece of a path, in pixels.
type renderSubpath struct {
	pts    [][2]float64
	closed bool
}

// renderCustomPath is one flattened path of a custom geometry.
type renderCustomPath struct {
	subpaths     []renderSubpath
	fill, stroke bool
	// shade is a lighten or darken fill mode, drawn as a plain fill.
	shade bool
}

// renderCustomGeometry evaluates a custom geometry for a box at (x, y), w by
// h EMU, flattening arcs and Bézier curves into at most maxPoints points.
// It returns the paths and the text rectangle's insets from the box.
func renderCustomGeometry(cg *dml.CustGeom, x, y, w, h dml.EMU, maxPoints int) ([]renderCustomPath, [4]dml.EMU, error) {
	var insets [4]dml.EMU
	g := renderNewGuides(float64(w), float64(h))
	if cg.AvLst != nil {
		if err := g.eval(false, cg.AvLst.Gd); err != nil {
			return nil, insets, err
		}
	}
	if cg.GdLst != nil {
		if err := g.eval(false, cg.GdLst.Gd); err != nil {
			return nil, insets, err
		}
	}
	if r := cg.RectXML; r != nil {
		var v [4]float64
		for i, tok := range []string{r.L, r.T, r.R, r.B} {
			f, err := g.value(tok)
			if err != nil {
				return nil, insets, err
			}
			v[i] = f
		}
		insets = [4]dml.EMU{dml.EMU(v[0]), dml.EMU(v[1]), w - dml.EMU(v[2]), h - dml.EMU(v[3])}
	}
	if cg.PathLst == nil || len(cg.PathLst.Path) == 0 {
		return nil, insets, nil
	}
	px := float64(dml.EMUsPerPixel)
	ox, oy := float64(x)/px, float64(y)/px
	points := 0
	var out []renderCustomPath
	for _, p := range cg.PathLst.Path {
		if p == nil {
			return nil, insets, fmt.Errorf("%w: geometry path", render.ErrInvalid)
		}
		// Path coordinates scale from the path's own size to the shape's.
		sx, sy := 1.0, 1.0
		if p.W > 0 {
			sx = float64(w) / float64(p.W)
		}
		if p.H > 0 {
			sy = float64(h) / float64(p.H)
		}
		cp := renderCustomPath{fill: p.Fill != "none", stroke: p.Stroke == nil || *p.Stroke}
		switch p.Fill {
		case "", "norm", "none":
		case "lighten", "lightenLess", "darken", "darkenLess":
			cp.shade = true
		default:
			return nil, insets, fmt.Errorf("%w: path fill %q", render.ErrInvalid, p.Fill)
		}
		// cx, cy is the current point in path coordinates.
		var cx, cy float64
		var sub *renderSubpath
		add := func(vx, vy float64) error {
			if points++; points > maxPoints {
				return fmt.Errorf("%w: geometry points", render.ErrLimit)
			}
			if sub == nil {
				cp.subpaths = append(cp.subpaths, renderSubpath{})
				sub = &cp.subpaths[len(cp.subpaths)-1]
			}
			sub.pts = append(sub.pts, [2]float64{ox + vx*sx/px, oy + vy*sy/px})
			cx, cy = vx, vy
			return nil
		}
		pt := func(p *dml.PtXML) (float64, float64, error) {
			if p == nil {
				return 0, 0, fmt.Errorf("%w: geometry point", render.ErrInvalid)
			}
			vx, err := g.value(p.X)
			if err != nil {
				return 0, 0, err
			}
			vy, err := g.value(p.Y)
			return vx, vy, err
		}
		// curve flattens a Bézier curve from the current point.
		curve := func(ctrl ...*dml.PtXML) error {
			cs := [][2]float64{{cx, cy}}
			for _, c := range ctrl {
				vx, vy, err := pt(c)
				if err != nil {
					return err
				}
				cs = append(cs, [2]float64{vx, vy})
			}
			const steps = 16
			for i := 1; i <= steps; i++ {
				t := float64(i) / steps
				// De Casteljau.
				q := append([][2]float64(nil), cs...)
				for len(q) > 1 {
					for j := 0; j+1 < len(q); j++ {
						q[j] = [2]float64{q[j][0] + (q[j+1][0]-q[j][0])*t, q[j][1] + (q[j+1][1]-q[j][1])*t}
					}
					q = q[:len(q)-1]
				}
				if err := add(q[0][0], q[0][1]); err != nil {
					return err
				}
			}
			return nil
		}
		for _, c := range p.Commands() {
			var err error
			switch {
			case c.MoveTo != nil:
				var vx, vy float64
				if vx, vy, err = pt(c.MoveTo.Pt); err == nil {
					sub = nil
					err = add(vx, vy)
				}
			case c.LnTo != nil:
				var vx, vy float64
				if vx, vy, err = pt(c.LnTo.Pt); err == nil {
					err = add(vx, vy)
				}
			case c.QuadBezTo != nil:
				if len(c.QuadBezTo.Pt) != 2 {
					return nil, insets, fmt.Errorf("%w: quadratic curve", render.ErrInvalid)
				}
				err = curve(c.QuadBezTo.Pt...)
			case c.CubicBezTo != nil:
				if len(c.CubicBezTo.Pt) != 3 {
					return nil, insets, fmt.Errorf("%w: cubic curve", render.ErrInvalid)
				}
				err = curve(c.CubicBezTo.Pt...)
			case c.ArcTo != nil:
				err = renderArcTo(g, c.ArcTo, cx, cy, add)
			case c.Close != nil:
				if sub != nil {
					sub.closed = true
					// The next command starts from the subpath's start.
					cx, cy = (sub.pts[0][0]-ox)*px/sx, (sub.pts[0][1]-oy)*px/sy
				}
				sub = nil
			}
			if err != nil {
				return nil, insets, err
			}
		}
		out = append(out, cp)
	}
	return out, insets, nil
}

// renderArcTo flattens an arcTo from the current point: an arc of the
// ellipse with radii wR and hR, from the angle stAng sweeping swAng, both in
// 60000ths of a degree clockwise and measured as seen on the ellipse.
func renderArcTo(g renderGuides, a *dml.ArcToXML, cx, cy float64, add func(x, y float64) error) error {
	var v [4]float64
	for i, tok := range []string{a.WR, a.HR, a.StAng, a.SwAng} {
		f, err := g.value(tok)
		if err != nil {
			return err
		}
		v[i] = f
	}
	wr, hr := math.Abs(v[0]), math.Abs(v[1])
	st, sw := v[2]/60000*math.Pi/180, v[3]/60000*math.Pi/180
	if wr == 0 || hr == 0 || sw == 0 {
		return nil
	}
	// The ellipse's parametric angle of a visual angle.
	param := func(a float64) float64 { return math.Atan2(wr*math.Sin(a), hr*math.Cos(a)) }
	t0 := param(st)
	t1 := param(st + sw)
	// Keep the sweep's direction and whole turns.
	turns := math.Trunc(sw / (2 * math.Pi))
	d := t1 - t0
	if sw > 0 {
		for d <= 0 {
			d += 2 * math.Pi
		}
	} else {
		for d >= 0 {
			d -= 2 * math.Pi
		}
	}
	d += turns * 2 * math.Pi
	if math.Abs(d) > 2*math.Pi {
		d = math.Copysign(2*math.Pi, d)
	}
	ex, ey := cx-wr*math.Cos(t0), cy-hr*math.Sin(t0)
	steps := max(1, int(math.Ceil(math.Abs(d)/(math.Pi/36))))
	for i := 1; i <= steps; i++ {
		t := t0 + d*float64(i)/float64(steps)
		if err := add(ex+wr*math.Cos(t), ey+hr*math.Sin(t)); err != nil {
			return err
		}
	}
	return nil
}

// renderCustomPathOps fills a custom path's subpaths with a paint, over the
// shape's box in pixels.
func renderCustomFill(cp renderCustomPath, paint renderPaint, box [4]float64) ([]layout.Op, error) {
	var path layout.Path
	for _, sub := range cp.subpaths {
		if len(sub.pts) < 3 {
			continue
		}
		for i, p := range sub.pts {
			x, okX := style.FromPx(p[0])
			y, okY := style.FromPx(p[1])
			if !okX || !okY {
				return nil, fmt.Errorf("%w: geometry coordinate", render.ErrLimit)
			}
			op := layout.LineTo
			if i == 0 {
				op = layout.MoveTo
			}
			path = append(path, layout.PathSegment{Op: op, Point: layout.Point{X: x, Y: y}})
		}
		path = append(path, layout.PathSegment{Op: layout.ClosePath})
	}
	if len(path) == 0 {
		return nil, nil
	}
	return paint.fillOps(box[0], box[1], box[2], box[3], path)
}

// renderCustomStroke outlines a custom path's subpaths segment by segment,
// with round joins between them: the line's caps and dashes restart at each
// segment, and arrowheads apply to an open subpath's ends.
func renderCustomStroke(cp renderCustomPath, ln *dml.Ln, placeholder *style.RGBA, colors *renderColors, maxSegments int) ([]layout.Op, error) {
	var ops []layout.Op
	for _, sub := range cp.subpaths {
		pts := sub.pts
		if sub.closed && len(pts) > 1 && pts[0] != pts[len(pts)-1] {
			pts = append(append([][2]float64(nil), pts...), pts[0])
		}
		for i := 0; i+1 < len(pts); i++ {
			seg := *ln
			seg.HeadEnd, seg.TailEnd = nil, nil
			if !sub.closed && i == 0 {
				seg.HeadEnd = ln.HeadEnd
			}
			if !sub.closed && i+2 == len(pts) {
				seg.TailEnd = ln.TailEnd
			}
			if len(pts) > 2 {
				seg.Cap = "rnd"
				seg.PrstDash = nil
			}
			drawn, err := renderLineStroke(&seg, placeholder, colors, pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1], maxSegments)
			if err != nil {
				return nil, err
			}
			ops = append(ops, drawn...)
		}
	}
	return ops, nil
}
