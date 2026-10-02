package pptx

import (
	"fmt"
	"math"
	"sort"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderPaint is a resolved area fill: a solid color, or a gradient laid out
// in the coordinates of the box it fills, measured from its top left.
type renderPaint struct {
	color    style.RGBA
	gradient *layout.Gradient
}

// renderMaxGradientStops bounds a gradient's stop list.
const renderMaxGradientStops = 64

// fillPaint resolves a solid or gradient fill for a box w by h pixels;
// placeholder is the color a theme style's phClr names.
func (c *renderColors) fillPaint(solid *dml.SolidFill, grad *dml.GradFill, placeholder *style.RGBA, w, h float64) (renderPaint, error) {
	if grad == nil {
		col, err := c.solid(solid, placeholder)
		return renderPaint{color: col}, err
	}
	g, err := c.gradient(grad, placeholder, w, h)
	if err != nil {
		return renderPaint{}, err
	}
	return renderPaint{gradient: &g}, nil
}

// gradient lays a DrawingML gradient out in a w by h pixel box. A linear
// gradient's angle runs clockwise from the x axis, and its offsets span the
// box corner to corner along it; scaled, the angle applies to the box
// stretched to a square. A path gradient runs from the box's edge, at offset
// 0, to its fill-to rectangle's centre: a circle exactly, and rectangle and
// shape paths approximately as ellipses.
func (c *renderColors) gradient(gf *dml.GradFill, placeholder *style.RGBA, w, h float64) (layout.Gradient, error) {
	var g layout.Gradient
	if gf.GsLst == nil || len(gf.GsLst.Gs) == 0 {
		return g, fmt.Errorf("%w: gradient without stops", render.ErrInvalid)
	}
	if len(gf.GsLst.Gs) > renderMaxGradientStops {
		return g, fmt.Errorf("%w: gradient stops", render.ErrLimit)
	}
	if w <= 0 || h <= 0 {
		return g, fmt.Errorf("%w: gradient box", render.ErrInvalid)
	}
	if gf.Lin != nil && gf.PathShade != nil {
		return g, fmt.Errorf("%w: gradient shade", render.ErrInvalid)
	}
	tiled := gf.TileRect != nil && (gf.TileRect.L.Int32() != 0 || gf.TileRect.T.Int32() != 0 || gf.TileRect.R.Int32() != 0 || gf.TileRect.B.Int32() != 0)
	if (gf.Flip != "" && gf.Flip != "none") || tiled {
		if err := c.approximate(fmt.Errorf("%w: gradient tiling left out", render.ErrUnsupported)); err != nil {
			return g, err
		}
	}
	reverse := gf.PathShade != nil
	for _, s := range gf.GsLst.Gs {
		if s == nil {
			return g, fmt.Errorf("%w: gradient stop", render.ErrInvalid)
		}
		col, err := c.color(renderColorOf(s.SrgbClr, s.SchemeClr, s.SysClr, s.ScRgbClr != nil, s.HslClr != nil, s.PrstClr != nil), placeholder)
		if err != nil {
			return g, err
		}
		pos := math.Min(1, math.Max(0, float64(s.Pos.Int32())/100000))
		if reverse {
			pos = 1 - pos
		}
		g.Stops = append(g.Stops, layout.GradientStop{Offset: pos, Color: col, Exponent: 1})
	}
	sort.SliceStable(g.Stops, func(i, j int) bool { return g.Stops[i].Offset < g.Stops[j].Offset })
	point := func(x, y float64) (layout.Point, error) {
		px, okX := style.FromPx(x)
		py, okY := style.FromPx(y)
		if !okX || !okY {
			return layout.Point{}, fmt.Errorf("%w: gradient coordinate", render.ErrLimit)
		}
		return layout.Point{X: px, Y: py}, nil
	}
	if p := gf.PathShade; p != nil {
		// The fill-to rectangle's insets are fractions of the box.
		l, t, r, b := 0.5, 0.5, 0.5, 0.5
		if f := p.FillToRect; f != nil {
			l, t, r, b = float64(f.L.Int32())/100000, float64(f.T.Int32())/100000, float64(f.R.Int32())/100000, float64(f.B.Int32())/100000
		}
		cx, cy := w*(l+1-r)/2, h*(t+1-b)/2
		dx, dy := math.Max(cx, w-cx), math.Max(cy, h-cy)
		rx, ry := math.Hypot(dx, dy), math.Hypot(dx, dy)
		switch p.Path {
		case "circle":
		case "rect", "shape":
			if err := c.approximate(fmt.Errorf("%w: %s gradient drawn elliptical", render.ErrUnsupported, p.Path)); err != nil {
				return g, err
			}
			rx, ry = dx*math.Sqrt2, dy*math.Sqrt2
		default:
			return g, fmt.Errorf("%w: gradient path %q", render.ErrInvalid, p.Path)
		}
		center, err := point(cx, cy)
		if err != nil {
			return g, err
		}
		radiusX, okX := style.FromPx(math.Max(rx, 1.0/64))
		radiusY, okY := style.FromPx(math.Max(ry, 1.0/64))
		if !okX || !okY {
			return g, fmt.Errorf("%w: gradient radius", render.ErrLimit)
		}
		g.Kind, g.Center, g.RadiusX, g.RadiusY = layout.RadialGradient, center, radiusX, radiusY
		return g, nil
	}
	angle, scaled := 0.0, false
	if gf.Lin != nil {
		if gf.Lin.Ang != nil {
			angle = float64(*gf.Lin.Ang) / 60000 * math.Pi / 180
		}
		scaled = renderTrue(gf.Lin.Scaled)
	}
	cos, sin := math.Cos(angle), math.Sin(angle)
	// The offset is linear in the point: offset = (P - centre)·k + 1/2, with
	// k scaled so the box's corners span 0 to 1.
	var kx, ky float64
	if scaled {
		span := math.Abs(cos) + math.Abs(sin)
		kx, ky = cos/(w*span), sin/(h*span)
	} else {
		span := math.Abs(w*cos) + math.Abs(h*sin)
		kx, ky = cos/span, sin/span
	}
	k2 := kx*kx + ky*ky
	sx, sy := w/2-0.5*kx/k2, h/2-0.5*ky/k2
	start, err := point(sx, sy)
	if err != nil {
		return g, err
	}
	end, err := point(sx+kx/k2, sy+ky/k2)
	if err != nil {
		return g, err
	}
	if start == end {
		return g, fmt.Errorf("%w: gradient line", render.ErrLimit)
	}
	g.Kind, g.Start, g.End = layout.LinearGradient, start, end
	return g, nil
}

// fillOps paints a box, in pixels, with a paint, clipped to path when it is
// not nil.
func (p renderPaint) fillOps(x, y, w, h float64, path layout.Path) ([]layout.Op, error) {
	rx, okX := style.FromPx(x)
	ry, okY := style.FromPx(y)
	rw, okW := style.FromPx(w)
	rh, okH := style.FromPx(h)
	if !okX || !okY || !okW || !okH {
		return nil, fmt.Errorf("%w: fill box", render.ErrLimit)
	}
	rect := layout.Rect{X: rx, Y: ry, W: rw, H: rh}
	if p.gradient == nil {
		if path != nil {
			return []layout.Op{layout.FillPath{Path: path, Color: p.color}}, nil
		}
		return []layout.Op{layout.FillRect{Rect: rect, Color: p.color}}, nil
	}
	if rw <= 0 || rh <= 0 {
		return nil, nil
	}
	var op layout.Op = layout.FillGradient{Clip: rect, Tile: rect, StepX: rw, StepY: rh, Gradient: *p.gradient}
	if path != nil {
		op = layout.ClipPath{Path: path, Ops: []layout.Op{op}}
	}
	return []layout.Op{op}, nil
}

// representative resolves a gradient or pattern fill to the one color that
// stands for it where only solid colors are drawn, reporting the
// approximation: a gradient's middle stop, a pattern's foreground.
func (c *renderColors) representative(grad *dml.GradFill, patt *dml.PattFill, placeholder *style.RGBA) (style.RGBA, error) {
	if err := c.approximate(fmt.Errorf("%w: gradient or pattern drawn in one color", render.ErrUnsupported)); err != nil {
		return style.RGBA{}, err
	}
	if grad != nil {
		g, err := c.gradient(grad, placeholder, 1, 1)
		if err != nil {
			return style.RGBA{}, err
		}
		return renderPaint{gradient: &g}.approximateColor(), nil
	}
	if patt == nil || patt.FgClr == nil {
		return style.RGBA{}, fmt.Errorf("%w: pattern without a foreground", render.ErrInvalid)
	}
	return c.color(renderChoiceColor(patt.FgClr), placeholder)
}

// renderSolidOf is a solid fill of an sRGB color, its alpha dropped.
func renderSolidOf(c style.RGBA) *dml.SolidFill {
	return &dml.SolidFill{SrgbClr: &dml.SrgbClr{Val: fmt.Sprintf("%02X%02X%02X", uint8(c.R), uint8(c.G), uint8(c.B))}}
}

// approximateColor is one color standing for a paint, for strokes and text
// that draw only solid colors: a gradient's middle stop.
func (p renderPaint) approximateColor() style.RGBA {
	if p.gradient == nil {
		return p.color
	}
	return p.gradient.Stops[len(p.gradient.Stops)/2].Color
}
