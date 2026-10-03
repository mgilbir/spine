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

// renderGeometry is a supported preset shape in its box: a rectangle whose
// corners have radius r (roundRect, or rect with r 0), or an ellipse.
type renderGeometry struct {
	ellipse bool
	box     [4]dml.EMU // x, y, width, height
	radius  dml.EMU
	// text holds the preset's text rectangle insets from the box: left, top,
	// right, bottom.
	text [4]dml.EMU
	// textTurn is how far the shape's text turns clockwise, in degrees:
	// PowerPoint turns text with its shape's rotation, and over with a
	// vertical flip, but never mirrors it.
	textTurn float64
}

// renderPresetGeometry evaluates the preset geometries this profile draws,
// following presetShapeDefinitions.xml.
func renderPresetGeometry(prst string, geom *dml.PrstGeom, x, y, w, h dml.EMU) (renderGeometry, error) {
	g := renderGeometry{box: [4]dml.EMU{x, y, w, h}}
	if geom != nil && geom.Prst != prst {
		return g, fmt.Errorf("%w: preset geometry mismatch", render.ErrInvalid)
	}
	adjust := map[string]int64{}
	if geom != nil && geom.AvLst != nil {
		for _, gd := range geom.AvLst.Gd {
			if gd == nil {
				continue
			}
			v, err := renderGuideValue(gd.Fmla)
			if err != nil {
				return g, err
			}
			if _, dup := adjust[gd.Name]; dup {
				return g, fmt.Errorf("%w: repeated shape adjustment", render.ErrInvalid)
			}
			adjust[gd.Name] = v
		}
	}
	switch prst {
	case "rect":
	case "roundRect":
		a, ok := adjust["adj"]
		if !ok {
			a = 16667
		}
		delete(adjust, "adj")
		// a = pin 0 adj 50000; dx1 = ss * a / 100000; il = dx1 * 29289 / 100000.
		a = min(max(a, 0), 50000)
		g.radius = dml.EMU(int64(min(w, h)) * a / 100000)
		il := dml.EMU(int64(g.radius) * 29289 / 100000)
		g.text = [4]dml.EMU{il, il, il, il}
	case "ellipse":
		g.ellipse = true
		// idx = wd2 * cos(45°), il = hc - idx, and likewise vertically.
		ix := dml.EMU(math.Round(float64(w) / 2 * (1 - math.Sqrt2/2)))
		iy := dml.EMU(math.Round(float64(h) / 2 * (1 - math.Sqrt2/2)))
		g.text = [4]dml.EMU{ix, iy, ix, iy}
	default:
		return g, fmt.Errorf("%w: preset %s", render.ErrUnsupported, prst)
	}
	if len(adjust) > 0 {
		return g, fmt.Errorf("%w: shape adjustment", render.ErrUnsupported)
	}
	return g, nil
}

// renderGuideValue reads a literal adjustment, the only form PowerPoint writes
// in a shape's adjustment list.
func renderGuideValue(fmla string) (int64, error) {
	v, ok := strings.CutPrefix(fmla, "val ")
	if !ok {
		return 0, fmt.Errorf("%w: shape adjustment formula", render.ErrUnsupported)
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: shape adjustment value", render.ErrInvalid)
	}
	return n, nil
}

// renderJoin is how an outline's outer edge turns a sharp corner.
type renderJoin int

const (
	renderJoinUnset renderJoin = iota
	renderJoinMiter
	renderJoinBevel
	renderJoinRound
)

// contour returns the boundary offset outward by d pixels (inward when d is
// negative). An offset corner arc keeps its center; a sharp corner offset
// outward takes the join's shape, and offset inward stays sharp.
func (g renderGeometry) contour(d float64, join renderJoin) (layout.Path, bool, error) {
	px := func(v dml.EMU) float64 { return float64(v) / float64(dml.EMUsPerPixel) }
	x0, y0 := px(g.box[0])-d, px(g.box[1])-d
	x1, y1 := px(g.box[0]+g.box[2])+d, px(g.box[1]+g.box[3])+d
	if x1-x0 <= 0 || y1-y0 <= 0 {
		return nil, false, nil
	}
	var (
		path layout.Path
		ok   = true
	)
	point := func(x, y float64) layout.Point {
		ux, okX := style.FromPx(x)
		uy, okY := style.FromPx(y)
		ok = ok && okX && okY
		return layout.Point{X: ux, Y: uy}
	}
	unit := func(v float64) style.Unit {
		u, okU := style.FromPx(v)
		ok = ok && okU
		return u
	}
	if g.ellipse {
		rx, ry := (x1-x0)/2, (y1-y0)/2
		path = layout.Path{{Op: layout.ArcTo, Center: point(x0+rx, y0+ry), RadiusX: unit(rx), RadiusY: unit(ry), SweepAngle: 360}, {Op: layout.ClosePath}}
	} else {
		r := px(g.radius) + d
		switch {
		case g.radius > 0:
			r = max(r, 0)
		case d > 0 && join == renderJoinRound:
			r = d
		default:
			r = 0
		}
		if d > 0 && g.radius == 0 && join == renderJoinBevel {
			path = layout.Path{
				{Op: layout.MoveTo, Point: point(x0+d, y0)}, {Op: layout.LineTo, Point: point(x1-d, y0)}, {Op: layout.LineTo, Point: point(x1, y0+d)},
				{Op: layout.LineTo, Point: point(x1, y1-d)}, {Op: layout.LineTo, Point: point(x1-d, y1)}, {Op: layout.LineTo, Point: point(x0+d, y1)},
				{Op: layout.LineTo, Point: point(x0, y1-d)}, {Op: layout.LineTo, Point: point(x0, y0+d)}, {Op: layout.ClosePath},
			}
		} else if r == 0 {
			path = layout.Path{{Op: layout.MoveTo, Point: point(x0, y0)}, {Op: layout.LineTo, Point: point(x1, y0)}, {Op: layout.LineTo, Point: point(x1, y1)}, {Op: layout.LineTo, Point: point(x0, y1)}, {Op: layout.ClosePath}}
		} else {
			r = min(r, (x1-x0)/2, (y1-y0)/2)
			ru := unit(r)
			path = layout.Path{
				{Op: layout.MoveTo, Point: point(x0+r, y0)},
				{Op: layout.ArcTo, Center: point(x1-r, y0+r), RadiusX: ru, RadiusY: ru, StartAngle: -90, SweepAngle: 90},
				{Op: layout.ArcTo, Center: point(x1-r, y1-r), RadiusX: ru, RadiusY: ru, StartAngle: 0, SweepAngle: 90},
				{Op: layout.ArcTo, Center: point(x0+r, y1-r), RadiusX: ru, RadiusY: ru, StartAngle: 90, SweepAngle: 90},
				{Op: layout.ArcTo, Center: point(x0+r, y0+r), RadiusX: ru, RadiusY: ru, StartAngle: 180, SweepAngle: 90},
				{Op: layout.ClosePath},
			}
		}
	}
	if !ok {
		return nil, false, fmt.Errorf("%w: shape coordinate", render.ErrLimit)
	}
	return path, true, nil
}

// fill paints the shape's interior.
func (g renderGeometry) fill(p renderPaint) ([]layout.Op, error) {
	px := float64(dml.EMUsPerPixel)
	x, y, w, h := float64(g.box[0])/px, float64(g.box[1])/px, float64(g.box[2])/px, float64(g.box[3])/px
	if !g.ellipse && g.radius == 0 {
		return p.fillOps(x, y, w, h, nil)
	}
	path, ok, err := g.contour(0, renderJoinUnset)
	if err != nil || !ok {
		return nil, err
	}
	return p.fillOps(x, y, w, h, path)
}

// stroke paints a solid outline as the even-odd ring between the boundary's
// outer and inner offsets. Offsetting an ellipse does not give an ellipse, so
// only circular ellipses are outlined.
func (g renderGeometry) stroke(ln *dml.Ln, placeholder *style.RGBA, colors *renderColors, maxSegments int) ([]layout.Op, error) {
	ln, err := renderLineWidth(ln, colors)
	if err != nil {
		return nil, err
	}
	if (ln.Cmpd != "" && ln.Cmpd != "sng") || ln.CustDash != nil || ln.GradFill != nil || ln.PattFill != nil || ln.ExtLst != nil {
		return nil, fmt.Errorf("%w: compound, custom-dashed or patterned outline", render.ErrUnsupported)
	}
	var pattern []float64
	if ln.PrstDash != nil && ln.PrstDash.Val != "" && ln.PrstDash.Val != "solid" {
		if pattern = renderDashes[ln.PrstDash.Val]; pattern == nil {
			return nil, fmt.Errorf("%w: dash %s", render.ErrInvalid, ln.PrstDash.Val)
		}
		// The schema gives no default cap; PowerPoint's shape styles use flat.
		if ln.Cap != "" && ln.Cap != "flat" {
			if err := colors.approximate(fmt.Errorf("%w: outline dash caps drawn flat", render.ErrUnsupported)); err != nil {
				return nil, err
			}
		}
	}
	join := renderJoinUnset
	joins := 0
	if ln.Round != nil {
		join, joins = renderJoinRound, joins+1
	}
	if ln.Bevel != nil {
		join, joins = renderJoinBevel, joins+1
	}
	if ln.Miter != nil {
		join, joins = renderJoinMiter, joins+1
		// A right angle's miter is √2 times the line width; a lower limit
		// bevels it.
		if lim := ln.Miter.Lim.Int32(); lim != 0 && float64(lim)/100000 < math.Sqrt2 {
			join = renderJoinBevel
		}
	}
	if joins > 1 {
		return nil, fmt.Errorf("%w: line join choice", render.ErrInvalid)
	}
	if g.ellipse && g.box[2] != g.box[3] {
		// The offsets of an ellipse are not ellipses; best effort draws the
		// ring between the ellipses with offset radii.
		if err := colors.approximate(fmt.Errorf("%w: elliptical outline", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	if !g.ellipse && g.radius == 0 && join == renderJoinUnset {
		if err := colors.approximate(fmt.Errorf("%w: unspecified line join drawn mitered", render.ErrUnsupported)); err != nil {
			return nil, err
		}
		join = renderJoinMiter
	}
	c, err := colors.solid(ln.SolidFill, placeholder)
	if err != nil {
		return nil, err
	}
	w := float64(*ln.W) / float64(dml.EMUsPerPixel)
	outer, inner := w/2, -w/2
	switch ln.Algn {
	case "", "ctr":
	case "in":
		outer, inner = 0, -w
	default:
		return nil, fmt.Errorf("%w: line alignment", render.ErrInvalid)
	}
	if pattern != nil {
		paths, err := g.dashed(pattern, w, outer, inner, join, maxSegments)
		if err != nil {
			return nil, err
		}
		var ops []layout.Op
		for _, path := range paths {
			if len(path) > 0 {
				ops = append(ops, layout.FillPath{Path: path, Color: c})
			}
		}
		return ops, nil
	}
	path, ok, err := g.contour(outer, join)
	if err != nil || !ok {
		return nil, err
	}
	hole, ok, err := g.contour(inner, join)
	if err != nil {
		return nil, err
	}
	if ok {
		path = append(path, hole...)
	}
	return []layout.Op{layout.FillPath{Path: path, Color: c}}, nil
}

// renderDashes are the preset dash patterns in line widths, alternating dash
// and gap, from ST_PresetLineDashVal.
var renderDashes = map[string][]float64{
	"dot": {1, 3}, "dash": {4, 3}, "lgDash": {8, 3}, "dashDot": {4, 3, 1, 3}, "lgDashDot": {8, 3, 1, 3},
	"lgDashDotDot": {8, 3, 1, 3, 1, 3}, "sysDash": {3, 1}, "sysDot": {1, 1}, "sysDashDot": {3, 1, 1, 1},
	"sysDashDotDot": {3, 1, 1, 1, 1, 1},
}

// renderEdge is one piece of a boundary centerline in pixels: a line from a
// to b, or an arc of radius r about c from angle a0 sweeping sweep degrees
// clockwise.
type renderEdge struct {
	arc            bool
	ax, ay, bx, by float64
	cx, cy, r      float64
	a0, sweep      float64
	start, length  float64 // distance along the boundary
}

func (e renderEdge) normal() (float64, float64) {
	// The boundary runs clockwise on the page, so the outward normal is on
	// the left of travel.
	dx, dy := e.bx-e.ax, e.by-e.ay
	n := math.Hypot(dx, dy)
	return dy / n, -dx / n
}

// edges returns the boundary as the preset path draws it: from its first
// moveTo, clockwise.
func (g renderGeometry) edges() []renderEdge {
	px := func(v dml.EMU) float64 { return float64(v) / float64(dml.EMUsPerPixel) }
	l, t := px(g.box[0]), px(g.box[1])
	r, b := px(g.box[0]+g.box[2]), px(g.box[1]+g.box[3])
	var out []renderEdge
	line := func(ax, ay, bx, by float64) {
		if ax != bx || ay != by {
			out = append(out, renderEdge{ax: ax, ay: ay, bx: bx, by: by, length: math.Hypot(bx-ax, by-ay)})
		}
	}
	arc := func(cx, cy, rad, a0, sweep float64) {
		out = append(out, renderEdge{arc: true, cx: cx, cy: cy, r: rad, a0: a0, sweep: sweep, length: rad * math.Abs(sweep) * math.Pi / 180})
	}
	switch rad := px(g.radius); {
	case g.ellipse:
		arc((l+r)/2, (t+b)/2, (r-l)/2, 180, 360)
	case g.radius == 0:
		line(l, t, r, t)
		line(r, t, r, b)
		line(r, b, l, b)
		line(l, b, l, t)
	default:
		rad = min(rad, (r-l)/2, (b-t)/2)
		arc(l+rad, t+rad, rad, 180, 90)
		line(l+rad, t, r-rad, t)
		arc(r-rad, t+rad, rad, 270, 90)
		line(r, t+rad, r, b-rad)
		arc(r-rad, b-rad, rad, 0, 90)
		line(r-rad, b, l+rad, b)
		arc(l+rad, b-rad, rad, 90, 90)
		line(l, b-rad, l, t+rad)
	}
	s := 0.0
	for i := range out {
		out[i].start = s
		s += out[i].length
	}
	return out
}

// dashed paints a preset-dashed outline with flat caps. Each dash is one
// contour: its outer side forward, then its inner side back, with the line
// join at a sharp corner on the side offset outward and the offset sides'
// meeting point on the other. Each dash is its own path: one shared path
// would make every scanline of the shape test every dash's edges, and where
// the pattern restarts at the boundary's start the last dash may overlap the
// first, which even-odd filling of one path would cut out.
func (g renderGeometry) dashed(pattern []float64, w, outer, inner float64, join renderJoin, maxSegments int) ([]layout.Path, error) {
	edges := g.edges()
	total := 0.0
	for _, e := range edges {
		total += e.length
	}
	period := 0.0
	for _, v := range pattern {
		period += v * w
	}
	if total <= 0 || period <= 0 {
		return nil, nil
	}
	// Each dash costs a handful of segments per edge it crosses.
	if total/period*float64(len(pattern)/2)*float64(4+2*len(edges)) > float64(maxSegments) {
		return nil, fmt.Errorf("%w: dash count", render.ErrLimit)
	}
	var (
		path layout.Path
		ok   = true
	)
	point := func(x, y float64) layout.Point {
		ux, okX := style.FromPx(x)
		uy, okY := style.FromPx(y)
		ok = ok && okX && okY
		return layout.Point{X: ux, Y: uy}
	}
	unit := func(v float64) style.Unit {
		u, okU := style.FromPx(v)
		ok = ok && okU
		return u
	}
	type piece struct {
		e      renderEdge
		t0, t1 float64
	}
	// side traces the pieces offset by d, forward or back.
	side := func(pieces []piece, d float64, forward bool) error {
		n := len(pieces)
		for k := 0; k < n; k++ {
			i := k
			if !forward {
				i = n - 1 - k
			}
			p := pieces[i]
			from, to := p.t0, p.t1
			if !forward {
				from, to = p.t1, p.t0
			}
			if p.e.arc {
				if p.e.r+d < 0 {
					return fmt.Errorf("%w: dashed outline wider than its corner", render.ErrUnsupported)
				}
				a0 := p.e.a0 + p.e.sweep*from
				path = append(path, layout.PathSegment{Op: layout.ArcTo, Center: point(p.e.cx, p.e.cy), RadiusX: unit(p.e.r + d), RadiusY: unit(p.e.r + d), StartAngle: a0, SweepAngle: p.e.sweep * (to - from)})
				continue
			}
			nx, ny := p.e.normal()
			at := func(t float64) (float64, float64) {
				return p.e.ax + (p.e.bx-p.e.ax)*t + d*nx, p.e.ay + (p.e.by-p.e.ay)*t + d*ny
			}
			// Where the dash turns a sharp corner, an inward side ends at
			// the offset sides' meeting point.
			x0, y0 := at(from)
			x1, y1 := at(to)
			prev, next := i-1, i+1
			if !forward {
				prev, next = i+1, i-1
			}
			if d < 0 && prev >= 0 && prev < n && !pieces[prev].e.arc {
				mx, my := pieces[prev].e.normal()
				cx, cy := p.e.ax, p.e.ay
				if !forward {
					cx, cy = p.e.bx, p.e.by
				}
				x0, y0 = cx+d*(nx+mx), cy+d*(ny+my)
			}
			if d < 0 && next >= 0 && next < n && !pieces[next].e.arc {
				mx, my := pieces[next].e.normal()
				cx, cy := p.e.bx, p.e.by
				if !forward {
					cx, cy = p.e.ax, p.e.ay
				}
				x1, y1 = cx+d*(nx+mx), cy+d*(ny+my)
			}
			path = append(path, layout.PathSegment{Op: layout.LineTo, Point: point(x0, y0)}, layout.PathSegment{Op: layout.LineTo, Point: point(x1, y1)})
			// An outward side turns the corner with the line join.
			if d > 0 && next >= 0 && next < n && !pieces[next].e.arc {
				mx, my := pieces[next].e.normal()
				cx, cy := p.e.bx, p.e.by
				if !forward {
					cx, cy = p.e.ax, p.e.ay
				}
				switch join {
				case renderJoinMiter:
					path = append(path, layout.PathSegment{Op: layout.LineTo, Point: point(cx+d*(nx+mx), cy+d*(ny+my))})
				case renderJoinRound:
					a := math.Atan2(ny, nx) * 180 / math.Pi
					sweep := 90.0
					if !forward {
						sweep = -90
					}
					path = append(path, layout.PathSegment{Op: layout.ArcTo, Center: point(cx, cy), RadiusX: unit(d), RadiusY: unit(d), StartAngle: a, SweepAngle: sweep})
				}
			}
		}
		return nil
	}
	dash := func(s0, s1 float64) error {
		var pieces []piece
		for _, e := range edges {
			a, b := max(s0, e.start), min(s1, e.start+e.length)
			if b > a {
				pieces = append(pieces, piece{e: e, t0: (a - e.start) / e.length, t1: (b - e.start) / e.length})
			}
		}
		if len(pieces) == 0 {
			return nil
		}
		start := len(path)
		if err := side(pieces, outer, true); err != nil {
			return err
		}
		if err := side(pieces, inner, false); err != nil {
			return err
		}
		// The contour begins where its outer side does.
		first := path[start]
		if first.Op == layout.ArcTo {
			path = append(path[:start], append(layout.Path{{Op: layout.MoveTo, Point: point(arcStart(first))}}, path[start:]...)...)
		} else {
			path[start].Op = layout.MoveTo
		}
		path = append(path, layout.PathSegment{Op: layout.ClosePath})
		return nil
	}
	var paths []layout.Path
	s, on := 0.0, true
	for i := 0; s < total; i = (i + 1) % len(pattern) {
		next := s + pattern[i]*w
		if on {
			path = nil
			if err := dash(s, min(next, total)); err != nil {
				return nil, err
			}
			if len(path) > 0 {
				paths = append(paths, path)
			}
		}
		s, on = next, !on
	}
	if !ok {
		return nil, fmt.Errorf("%w: shape coordinate", render.ErrLimit)
	}
	return paths, nil
}

// arcStart is where an arc segment begins, in pixels.
func arcStart(s layout.PathSegment) (float64, float64) {
	a := s.StartAngle * math.Pi / 180
	return s.Center.X.Px() + s.RadiusX.Px()*math.Cos(a), s.Center.Y.Px() + s.RadiusY.Px()*math.Sin(a)
}
