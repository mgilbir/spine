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
func (g renderGeometry) fill(c style.RGBA) ([]layout.Op, error) {
	if !g.ellipse && g.radius == 0 {
		return []layout.Op{layout.FillRect{Rect: layout.Rect{X: renderUnit(g.box[0]), Y: renderUnit(g.box[1]), W: renderUnit(g.box[2]), H: renderUnit(g.box[3])}, Color: c}}, nil
	}
	path, ok, err := g.contour(0, renderJoinUnset)
	if err != nil || !ok {
		return nil, err
	}
	return []layout.Op{layout.FillPath{Path: path, Color: c}}, nil
}

// stroke paints a solid outline as the even-odd ring between the boundary's
// outer and inner offsets. Offsetting an ellipse does not give an ellipse, so
// only circular ellipses are outlined.
func (g renderGeometry) stroke(ln *dml.Ln, colors *renderColors) ([]layout.Op, error) {
	if ln.W == nil || *ln.W <= 0 {
		return nil, fmt.Errorf("%w: outline without a width", render.ErrUnsupported)
	}
	if (ln.Cmpd != "" && ln.Cmpd != "sng") || (ln.PrstDash != nil && ln.PrstDash.Val != "" && ln.PrstDash.Val != "solid") || ln.CustDash != nil || ln.GradFill != nil || ln.PattFill != nil || ln.ExtLst != nil {
		return nil, fmt.Errorf("%w: compound, dashed or patterned outline", render.ErrUnsupported)
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
		return nil, fmt.Errorf("%w: elliptical outline", render.ErrUnsupported)
	}
	if !g.ellipse && g.radius == 0 && join == renderJoinUnset {
		return nil, fmt.Errorf("%w: unspecified line join on a sharp corner", render.ErrUnsupported)
	}
	c, err := colors.solid(ln.SolidFill, nil)
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
