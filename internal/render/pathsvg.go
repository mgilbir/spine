package render

import (
	"context"
	"encoding/xml"
	"fmt"
	"math"
	"strings"

	"github.com/mgilbir/forme/layout"
)

func (p *Page) svgDrawings(ctx context.Context, e *xml.Encoder, scale float64) error {
	ids := map[*geometry]string{}
	for _, d := range p.draws {
		for _, g := range d.clips {
			if _, ok := ids[g]; ok {
				continue
			}
			id := fmt.Sprintf("clip%d", len(ids))
			ids[g] = id
			data, err := pathString(ctx, g, scale, p.limits.MaxOutputBytes)
			if err != nil {
				return err
			}
			defs := xml.StartElement{Name: xml.Name{Local: "defs"}}
			clip := xml.StartElement{Name: xml.Name{Local: "clipPath"}, Attr: []xml.Attr{attr("id", id), attr("clipPathUnits", "userSpaceOnUse")}}
			if err := e.EncodeToken(defs); err != nil {
				return err
			}
			if err := e.EncodeToken(clip); err != nil {
				return err
			}
			if err := svgElement(e, "path", []xml.Attr{attr("d", data), attr("clip-rule", "evenodd")}); err != nil {
				return err
			}
			if err := e.EncodeToken(clip.End()); err != nil {
				return err
			}
			if err := e.EncodeToken(defs.End()); err != nil {
				return err
			}
		}
	}
	for i, d := range p.draws {
		if err := ctx.Err(); err != nil {
			return err
		}
		r := d.rect
		var groups []xml.StartElement
		for _, clip := range d.clips {
			g := xml.StartElement{Name: xml.Name{Local: "g"}, Attr: []xml.Attr{attr("clip-path", "url(#"+ids[clip]+")")}}
			if err := e.EncodeToken(g); err != nil {
				return err
			}
			groups = append(groups, g)
		}
		attrs := []xml.Attr{attr("fill", fmt.Sprintf("rgb(%s,%s,%s)", number(r.color.R), number(r.color.G), number(r.color.B))), attr("fill-opacity", number(r.color.A))}
		if d.path == nil {
			attrs = append(attrs, rectAttrs(r, scale)...)
			if err := svgElement(e, "rect", attrs); err != nil {
				return err
			}
		} else {
			// Local rectangular clip, including the physical page boundary.
			id := fmt.Sprintf("bounds%d", i)
			defs := xml.StartElement{Name: xml.Name{Local: "defs"}}
			clip := xml.StartElement{Name: xml.Name{Local: "clipPath"}, Attr: []xml.Attr{attr("id", id), attr("clipPathUnits", "userSpaceOnUse")}}
			if err := e.EncodeToken(defs); err != nil {
				return err
			}
			if err := e.EncodeToken(clip); err != nil {
				return err
			}
			if err := svgElement(e, "rect", rectAttrs(r, scale)); err != nil {
				return err
			}
			if err := e.EncodeToken(clip.End()); err != nil {
				return err
			}
			if err := e.EncodeToken(defs.End()); err != nil {
				return err
			}
			data, err := pathString(ctx, d.path, scale, p.limits.MaxOutputBytes)
			if err != nil {
				return err
			}
			attrs = append(attrs, attr("d", data), attr("fill-rule", "evenodd"), attr("clip-path", "url(#"+id+")"))
			if err := svgElement(e, "path", attrs); err != nil {
				return err
			}
		}
		for j := len(groups) - 1; j >= 0; j-- {
			if err := e.EncodeToken(groups[j].End()); err != nil {
				return err
			}
		}
	}
	return nil
}

func svgElement(e *xml.Encoder, name string, attrs []xml.Attr) error {
	s := xml.StartElement{Name: xml.Name{Local: name}, Attr: attrs}
	if err := e.EncodeToken(s); err != nil {
		return err
	}
	return e.EncodeToken(s.End())
}

func rectAttrs(r rectangle, scale float64) []xml.Attr {
	return []xml.Attr{attr("x", number(r.x0*scale)), attr("y", number(r.y0*scale)), attr("width", number((r.x1-r.x0)*scale)), attr("height", number((r.y1-r.y0)*scale))}
}

func pathString(ctx context.Context, g *geometry, scale float64, limit int64) (string, error) {
	var b strings.Builder
	write := func(s string) error {
		if int64(len(s)) > limit-int64(b.Len()) {
			return fmt.Errorf("%w: SVG path bytes", ErrLimit)
		}
		_, _ = b.WriteString(s)
		return nil
	}
	open := false
	for _, s := range g.path {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		var data string
		switch s.Op {
		case layout.MoveTo:
			if open {
				data = "Z "
			}
			data += "M" + number(s.Point.X.Px()*scale) + " " + number(s.Point.Y.Px()*scale) + " "
			open = true
		case layout.LineTo:
			data = "L" + number(s.Point.X.Px()*scale) + " " + number(s.Point.Y.Px()*scale) + " "
		case layout.ClosePath:
			data = "Z "
			open = false
		case layout.ArcTo:
			at := func(angle float64) string {
				a := angle * math.Pi / 180
				return number((s.Center.X.Px()+s.RadiusX.Px()*math.Cos(a))*scale) + " " + number((s.Center.Y.Px()+s.RadiusY.Px()*math.Sin(a))*scale)
			}
			if open {
				data = "L"
			} else {
				data = "M"
				open = true
			}
			data += at(s.StartAngle) + " "
			// Split full circles and long arcs into at most two 180-degree arcs.
			n := max(1, int(math.Ceil(math.Abs(s.SweepAngle)/180)))
			sweep := "0"
			if s.SweepAngle > 0 {
				sweep = "1"
			}
			for i := 1; i <= n; i++ {
				data += "A" + number(s.RadiusX.Px()*scale) + " " + number(s.RadiusY.Px()*scale) + " 0 0 " + sweep + " " + at(s.StartAngle+s.SweepAngle*float64(i)/float64(n)) + " "
			}
		}
		if err := write(data); err != nil {
			return "", err
		}
	}
	if open {
		if err := write("Z"); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}
