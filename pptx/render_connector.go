package pptx

import (
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// renderSourceConnector returns the parsed p:cxnSp behind a shape, or nil.
func (s *Slide) renderSourceConnector(index int) *oxml.ConnectionShape {
	if index >= len(s.shapeRefs) {
		return nil
	}
	ref := s.shapeRefs[index]
	if ref.Kind != oxml.ChildCxnSp || ref.Index < 0 || ref.Index >= len(s.sxModel.CSld.SpTree.CxnSp) {
		return nil
	}
	return s.sxModel.CSld.SpTree.CxnSp[ref.Index]
}

// renderConnector paints a straight connector: its line comes from the theme
// line style its style reference selects, overridden by its own a:ln. The
// stored geometry is drawn; bindings to other shapes only move it when they
// move. Arrowheads fail: their sizes are not specified.
//
// Only a parsed connector without pending edits is drawn: the save path
// resolves bindings from the domain model, which preparation must not do.
func (s *Slide) renderConnector(index int, c *Connector, colors *renderColors, limits render.Limits) ([]layout.Op, error) {
	src := s.renderSourceConnector(index)
	if src == nil || c.dirty {
		return nil, fmt.Errorf("%w: new or edited connector; save and reopen to preview it", render.ErrUnsupported)
	}
	p := src.SpPr
	if p == nil || p.Xfrm == nil || p.Xfrm.Off == nil || p.Xfrm.Ext == nil || src.ExtLst != nil {
		return nil, fmt.Errorf("%w: connector geometry", render.ErrUnsupported)
	}
	if p.Xfrm.Rot != 0 || p.PrstGeom == nil || (p.PrstGeom.Prst != "straightConnector1" && p.PrstGeom.Prst != "line") || (p.PrstGeom.AvLst != nil && len(p.PrstGeom.AvLst.Gd) > 0) || p.CustGeom != nil {
		return nil, fmt.Errorf("%w: rotated, bent or curved connector", render.ErrUnsupported)
	}
	// A line has no interior, so a fill on it paints nothing.
	if p.GradFill != nil || p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil || p.ExtLst != nil || p.BwMode != "" {
		return nil, fmt.Errorf("%w: connector fill", render.ErrUnsupported)
	}
	if renderEffects(p.EffectLst) || p.EffectDag != nil || p.Scene3d != nil || p.Sp3d != nil {
		if err := colors.approximate(fmt.Errorf("%w: connector effects left out", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	line, placeholder, err := renderStyledLine(src.Style, p.Ln, colors)
	if err != nil || line == nil {
		return nil, err
	}
	x0, y0 := float64(p.Xfrm.Off.X), float64(p.Xfrm.Off.Y)
	x1, y1 := x0+float64(p.Xfrm.Ext.Cx), y0+float64(p.Xfrm.Ext.Cy)
	if p.Xfrm.FlipH {
		x0, x1 = x1, x0
	}
	if p.Xfrm.FlipV {
		y0, y1 = y1, y0
	}
	px := float64(dml.EMUsPerPixel)
	return renderLineStroke(line, placeholder, colors, x0/px, y0/px, x1/px, y1/px, limits.MaxPathSegments)
}

// renderStyledLine resolves a line from a style reference and an explicit
// a:ln, whose set properties win. It returns nil when there is no line, and
// the style reference color that a theme line's phClr names.
func renderStyledLine(st *dml.Style, own *dml.Ln, colors *renderColors) (*dml.Ln, *style.RGBA, error) {
	var (
		line        dml.Ln
		placeholder *style.RGBA
	)
	if st != nil {
		if r := st.EffectRef; r != nil && r.Idx != 0 {
			theme, err := colors.loadTheme()
			if err != nil {
				return nil, nil, err
			}
			var list *dml.EffectStyleLst
			if theme.ThemeElements != nil && theme.ThemeElements.FmtScheme != nil {
				list = theme.ThemeElements.FmtScheme.EffectStyleLst
			}
			if list == nil || int(r.Idx) > len(list.EffectStyle) || list.EffectStyle[r.Idx-1] == nil {
				return nil, nil, fmt.Errorf("%w: effect style %d", render.ErrInvalid, r.Idx)
			}
			e := list.EffectStyle[r.Idx-1]
			if renderEffects(e.EffectLst) || e.EffectDag != nil || e.Scene3d != nil || e.Sp3d != nil {
				if err := colors.approximate(fmt.Errorf("%w: theme effect style left out", render.ErrUnsupported)); err != nil {
					return nil, nil, err
				}
			}
		}
		if r := st.LnRef; r != nil && r.Idx != 0 {
			c, err := colors.color(renderColorOf(r.SrgbClr, r.SchemeClr, r.SysClr, r.ScrgbClr != nil, r.HslClr != nil, r.PrstClr != nil), nil)
			if err != nil {
				return nil, nil, err
			}
			placeholder = &c
			theme, err := colors.loadTheme()
			if err != nil {
				return nil, nil, err
			}
			var list *dml.LnStyleLst
			if theme.ThemeElements != nil && theme.ThemeElements.FmtScheme != nil {
				list = theme.ThemeElements.FmtScheme.LnStyleLst
			}
			if list == nil || int(r.Idx) > len(list.Ln) || list.Ln[r.Idx-1] == nil {
				return nil, nil, fmt.Errorf("%w: line style %d", render.ErrInvalid, r.Idx)
			}
			line = *list.Ln[r.Idx-1]
		}
	}
	if own != nil {
		if own.W != nil {
			line.W = own.W
		}
		for _, f := range []struct {
			to *string
			v  string
		}{{&line.Cap, own.Cap}, {&line.Cmpd, own.Cmpd}, {&line.Algn, own.Algn}} {
			if f.v != "" {
				*f.to = f.v
			}
		}
		if own.NoFill != nil || own.SolidFill != nil || own.GradFill != nil || own.PattFill != nil {
			line.NoFill, line.SolidFill, line.GradFill, line.PattFill = own.NoFill, own.SolidFill, own.GradFill, own.PattFill
		}
		if own.PrstDash != nil || own.CustDash != nil {
			line.PrstDash, line.CustDash = own.PrstDash, own.CustDash
		}
		if own.Round != nil || own.Bevel != nil || own.Miter != nil {
			line.Round, line.Bevel, line.Miter = own.Round, own.Bevel, own.Miter
		}
		if own.HeadEnd != nil {
			line.HeadEnd = own.HeadEnd
		}
		if own.TailEnd != nil {
			line.TailEnd = own.TailEnd
		}
		if own.ExtLst != nil {
			line.ExtLst = own.ExtLst
		}
	}
	if line.SolidFill == nil {
		if line.GradFill != nil || line.PattFill != nil {
			return nil, nil, fmt.Errorf("%w: patterned line", render.ErrUnsupported)
		}
		return nil, nil, nil
	}
	if line.NoFill != nil {
		return nil, nil, fmt.Errorf("%w: ambiguous line fill", render.ErrInvalid)
	}
	return &line, placeholder, nil
}

// renderLineStroke paints an open straight line from (x0,y0) to (x1,y1) in
// pixels, with its caps and preset dashes. Dashes take flat caps; the schema
// gives no default cap and PowerPoint's styles use flat.
func renderLineStroke(ln *dml.Ln, placeholder *style.RGBA, colors *renderColors, x0, y0, x1, y1 float64, maxSegments int) ([]layout.Op, error) {
	if ln.W == nil || *ln.W <= 0 {
		return nil, fmt.Errorf("%w: line without a width", render.ErrUnsupported)
	}
	if (ln.Cmpd != "" && ln.Cmpd != "sng") || (ln.Algn != "" && ln.Algn != "ctr") || ln.CustDash != nil || ln.ExtLst != nil {
		return nil, fmt.Errorf("%w: compound, inset or custom-dashed line", render.ErrUnsupported)
	}
	var pattern []float64
	if ln.PrstDash != nil && ln.PrstDash.Val != "" && ln.PrstDash.Val != "solid" {
		if pattern = renderDashes[ln.PrstDash.Val]; pattern == nil {
			return nil, fmt.Errorf("%w: dash %s", render.ErrInvalid, ln.PrstDash.Val)
		}
	}
	extend, round := 0.0, false
	switch ln.Cap {
	case "", "flat":
	case "sq":
		if pattern != nil {
			return nil, fmt.Errorf("%w: dash caps other than flat", render.ErrUnsupported)
		}
		extend = 1
	case "rnd":
		if pattern != nil {
			return nil, fmt.Errorf("%w: dash caps other than flat", render.ErrUnsupported)
		}
		round = true
	default:
		return nil, fmt.Errorf("%w: line cap", render.ErrInvalid)
	}
	c, err := colors.solid(ln.SolidFill, placeholder)
	if err != nil {
		return nil, err
	}
	w := float64(*ln.W) / float64(dml.EMUsPerPixel)
	h := w / 2
	length := math.Hypot(x1-x0, y1-y0)
	if length == 0 {
		// A zero-length line shows only its caps.
		if extend == 0 && !round {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: zero-length capped line", render.ErrUnsupported)
	}
	ux, uy := (x1-x0)/length, (y1-y0)/length
	nx, ny := -uy, ux
	ok := true
	point := func(x, y float64) layout.Point {
		px, okX := style.FromPx(x)
		py, okY := style.FromPx(y)
		ok = ok && okX && okY
		return layout.Point{X: px, Y: py}
	}
	// piece is the stroked stretch of the line from distance a to b.
	piece := func(a, b float64, roundEnds bool) layout.Path {
		ax, ay := x0+ux*a, y0+uy*a
		bx, by := x0+ux*b, y0+uy*b
		if !roundEnds {
			return layout.Path{
				{Op: layout.MoveTo, Point: point(ax+nx*h, ay+ny*h)}, {Op: layout.LineTo, Point: point(bx+nx*h, by+ny*h)},
				{Op: layout.LineTo, Point: point(bx-nx*h, by-ny*h)}, {Op: layout.LineTo, Point: point(ax-nx*h, ay-ny*h)}, {Op: layout.ClosePath},
			}
		}
		r, rok := style.FromPx(h)
		ok = ok && rok
		a0 := math.Atan2(ny, nx) * 180 / math.Pi
		return layout.Path{
			{Op: layout.MoveTo, Point: point(ax+nx*h, ay+ny*h)}, {Op: layout.LineTo, Point: point(bx+nx*h, by+ny*h)},
			{Op: layout.ArcTo, Center: point(bx, by), RadiusX: r, RadiusY: r, StartAngle: a0, SweepAngle: -180},
			{Op: layout.LineTo, Point: point(ax-nx*h, ay-ny*h)},
			{Op: layout.ArcTo, Center: point(ax, ay), RadiusX: r, RadiusY: r, StartAngle: a0 + 180, SweepAngle: -180},
			{Op: layout.ClosePath},
		}
	}
	// Arrowheads are filled triangles with their tips at the line's ends,
	// which stop short inside them. DrawingML does not specify their sizes;
	// they approximate LibreOffice's: small, medium and large are two, three
	// and five line widths. Other head shapes are drawn as triangles too.
	var heads []layout.Op
	start, end := -extend*h, length+extend*h
	for i, e := range []*dml.LineEnd{ln.HeadEnd, ln.TailEnd} {
		if e == nil || e.Type == "" || e.Type == "none" {
			continue
		}
		if err := colors.approximate(fmt.Errorf("%w: arrowhead %s sized approximately", render.ErrUnsupported, e.Type)); err != nil {
			return nil, err
		}
		size := map[string]float64{"sm": 2, "med": 3, "lg": 5}
		hw, hl := size[e.W], size[e.Len]
		if hw == 0 {
			hw = 3
		}
		if hl == 0 {
			hl = 3
		}
		hw, hl = hw*max(w, 1)/2, hl*max(w, 1)
		if 2*hl > length {
			hl = length / 2
		}
		tx, ty, dx, dy := x0, y0, ux, uy
		if i == 1 {
			tx, ty, dx, dy = x1, y1, -ux, -uy
			end = min(end, length-hl/2)
		} else {
			start = max(start, hl/2)
		}
		bx, by := tx+dx*hl, ty+dy*hl
		heads = append(heads, layout.FillPath{Path: layout.Path{
			{Op: layout.MoveTo, Point: point(tx, ty)}, {Op: layout.LineTo, Point: point(bx+nx*hw, by+ny*hw)},
			{Op: layout.LineTo, Point: point(bx-nx*hw, by-ny*hw)}, {Op: layout.ClosePath},
		}, Color: c})
	}
	var ops []layout.Op
	if pattern == nil {
		ops = append(ops, layout.FillPath{Path: piece(start, end, round), Color: c})
	} else {
		period := 0.0
		for _, v := range pattern {
			period += v * w
		}
		if length/period*float64(len(pattern)/2)*5 > float64(maxSegments) {
			return nil, fmt.Errorf("%w: dash count", render.ErrLimit)
		}
		s, on := 0.0, true
		for i := 0; s < length; i = (i + 1) % len(pattern) {
			next := s + pattern[i]*w
			if on && min(next, end) > max(s, start) {
				ops = append(ops, layout.FillPath{Path: piece(max(s, start), min(next, end), false), Color: c})
			}
			s, on = next, !on
		}
	}
	ops = append(ops, heads...)
	if !ok {
		return nil, fmt.Errorf("%w: line coordinate", render.ErrLimit)
	}
	return ops, nil
}
