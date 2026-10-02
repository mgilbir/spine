package render

import (
	"encoding/xml"
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
)

// gradientFill is a gradient painted in one tile whose top left is at (x, y)
// in CSS pixels.
type gradientFill struct {
	g    layout.Gradient
	x, y float64
}

// collectGradient checks a gradient fill and records it. This profile paints
// a single tile of a linear or radial, non-repeating gradient whose stops lie
// in [0, 1] and blend linearly, which PNG and SVG output draw alike. Stops
// count toward the path segment budget.
func (p *Page) collectGradient(v layout.FillGradient, clips []*geometry, budget *prepareBudget) error {
	g := v.Gradient
	if cols, rows := v.Tiles(); cols > 1 || rows > 1 {
		return fmt.Errorf("%w: tiled gradient", ErrUnsupported)
	}
	if g.Repeating || (g.Kind != layout.LinearGradient && g.Kind != layout.RadialGradient) {
		return fmt.Errorf("%w: gradient kind", ErrUnsupported)
	}
	if len(g.Stops) == 0 || v.Clip.W < 0 || v.Clip.H < 0 || v.Tile.W <= 0 || v.Tile.H <= 0 {
		return fmt.Errorf("%w: gradient", ErrInvalid)
	}
	if len(g.Stops) > p.limits.MaxPathSegments-budget.segments {
		return fmt.Errorf("%w: gradient stops", ErrLimit)
	}
	budget.segments += len(g.Stops)
	last := 0.0
	for i, s := range g.Stops {
		if !validColor(s.Color) || math.IsNaN(s.Offset) || s.Offset < last || s.Offset > 1 || (i > 0 && s.Exponent != 1) {
			return fmt.Errorf("%w: gradient stop", ErrUnsupported)
		}
		last = s.Offset
	}
	switch g.Kind {
	case layout.LinearGradient:
		if g.Start == g.End {
			return fmt.Errorf("%w: gradient line", ErrInvalid)
		}
	case layout.RadialGradient:
		if g.RadiusX <= 0 || g.RadiusY <= 0 {
			return fmt.Errorf("%w: gradient radius", ErrInvalid)
		}
	}
	// The page owns its inputs.
	g.Stops = append([]layout.GradientStop(nil), g.Stops...)
	c, t := v.Clip, v.Tile
	r := meet(rectangle{c.X.Px(), c.Y.Px(), c.X.Px() + c.W.Px(), c.Y.Px() + c.H.Px(), style.RGBA{A: 1}}, rectangle{t.X.Px(), t.Y.Px(), t.X.Px() + t.W.Px(), t.Y.Px() + t.H.Px(), style.RGBA{}})
	r = meet(r, rectangle{0, 0, p.width, p.height, style.RGBA{}})
	r.color = style.RGBA{A: 1}
	p.draws = append(p.draws, drawing{rect: r, clips: clips, gradient: &gradientFill{g: g, x: t.X.Px(), y: t.Y.Px()}})
	return nil
}

// color is the gradient's color at a point in CSS pixels.
func (f *gradientFill) color(x, y float64) style.RGBA {
	px, _ := style.FromPx(x - f.x)
	py, _ := style.FromPx(y - f.y)
	return f.g.ColorAt(layout.Point{X: px, Y: py})
}

// svgGradient writes the gradient as a paint server with the given id.
func (f *gradientFill) svg(e *xml.Encoder, id string, scale float64) error {
	at := func(p layout.Point) (string, string) {
		return number((f.x + p.X.Px()) * scale), number((f.y + p.Y.Px()) * scale)
	}
	var server xml.StartElement
	switch f.g.Kind {
	case layout.LinearGradient:
		x1, y1 := at(f.g.Start)
		x2, y2 := at(f.g.End)
		server = xml.StartElement{Name: xml.Name{Local: "linearGradient"}, Attr: []xml.Attr{attr("x1", x1), attr("y1", y1), attr("x2", x2), attr("y2", y2)}}
	default:
		cx, cy := at(f.g.Center)
		rx, ry := f.g.RadiusX.Px()*scale, f.g.RadiusY.Px()*scale
		server = xml.StartElement{Name: xml.Name{Local: "radialGradient"}, Attr: []xml.Attr{attr("cx", cx), attr("cy", cy), attr("r", number(rx)),
			attr("gradientTransform", fmt.Sprintf("translate(%s %s) scale(1 %s) translate(-%s -%s)", cx, cy, number(ry/rx), cx, cy))}}
	}
	server.Attr = append(server.Attr, attr("id", id), attr("gradientUnits", "userSpaceOnUse"))
	defs := xml.StartElement{Name: xml.Name{Local: "defs"}}
	if err := e.EncodeToken(defs); err != nil {
		return err
	}
	if err := e.EncodeToken(server); err != nil {
		return err
	}
	for _, s := range f.g.Stops {
		c := s.Color
		if err := svgElement(e, "stop", []xml.Attr{attr("offset", number(s.Offset)), attr("stop-color", fmt.Sprintf("rgb(%s,%s,%s)", number(c.R), number(c.G), number(c.B))), attr("stop-opacity", number(c.A))}); err != nil {
			return err
		}
	}
	if err := e.EncodeToken(server.End()); err != nil {
		return err
	}
	return e.EncodeToken(defs.End())
}
