package pptx

import (
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderStyleEffects returns the theme effect style a style reference
// selects, and the reference color its phClr names; nil for none.
func renderStyleEffects(st *dml.Style, colors *renderColors) (*dml.EffectStyle, *style.RGBA, error) {
	if st == nil || st.EffectRef == nil || st.EffectRef.Idx == 0 {
		return nil, nil, nil
	}
	r := st.EffectRef
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
	c, err := colors.color(renderColorOf(r.SrgbClr, r.SchemeClr, r.SysClr, r.PrstClr, r.ScrgbClr != nil, r.HslClr != nil), nil)
	if err != nil {
		return nil, nil, err
	}
	return list.EffectStyle[r.Idx-1], &c, nil
}

// renderShapeEffects draws a shape's effects with its drawing: its own
// effect list, or else its style's theme effects; see blurredEffects. Best
// effort leaves out fill overlays, preset shadows, effect graphs and 3-D.
func renderShapeEffects(ops []layout.Op, own *dml.EffectLst, dag, threeD bool, st *dml.Style, colors *renderColors, maxOps int) ([]layout.Op, error) {
	effects := own
	var placeholder *style.RGBA
	if !renderEffects(effects) && !dag {
		e, c, err := renderStyleEffects(st, colors)
		if err != nil {
			return nil, err
		}
		if e != nil {
			effects, placeholder = e.EffectLst, c
			dag, threeD = dag || e.EffectDag != nil, threeD || e.Scene3d != nil || e.Sp3d != nil
		}
	}
	if dag || threeD {
		if err := colors.approximate(fmt.Errorf("%w: effect graph or 3-D left out", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	if !renderEffects(effects) {
		return ops, nil
	}
	e := *effects
	if e.FillOverlay != nil || e.PrstShdw != nil {
		if err := colors.approximate(fmt.Errorf("%w: fill overlay and preset shadow effects left out", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	below, shape, above, err := colors.blurredEffects(ops, &e, placeholder)
	if err != nil {
		return nil, err
	}
	out := append(append(append([]layout.Op{}, below...), shape...), above...)
	if len(out) > maxOps {
		return nil, fmt.Errorf("%w: effect operations", render.ErrLimit)
	}
	return out, nil
}

// renderShadowCopies is how many offset copies stand for a shadow's blur.
const renderShadowCopies = 9

// renderShadow is a drawing's outer shadow: its shapes in the shadow color,
// offset by the shadow's distance in its direction. A blur is approximated
// by copies spread over the blur radius whose opacities compound to the
// shadow's; scaling and skewing are left out.
func renderShadow(ops []layout.Op, sh *dml.OuterShdw, placeholder *style.RGBA, colors *renderColors, maxOps int) ([]layout.Op, error) {
	c, err := colors.color(renderColorOf(sh.SrgbClr, sh.SchemeClr, sh.SysClr, sh.PrstClr, sh.ScRgbClr != nil, sh.HslClr != nil), placeholder)
	if err != nil {
		return nil, err
	}
	px := float64(dml.EMUsPerPixel)
	var blur, dist, dir float64
	if sh.BlurRad != nil {
		blur = float64(*sh.BlurRad) / px
	}
	if sh.Dist != nil {
		dist = float64(*sh.Dist) / px
	}
	if sh.Dir != nil {
		dir = float64(*sh.Dir) / 60000 * math.Pi / 180
	}
	if blur < 0 || dist < 0 || math.IsNaN(blur+dist) {
		return nil, fmt.Errorf("%w: shadow", render.ErrInvalid)
	}
	scaled := sh.Sx != nil && sh.Sx.Int32() != 100000 || sh.Sy != nil && sh.Sy.Int32() != 100000 || sh.Kx != nil && *sh.Kx != 0 || sh.Ky != nil && *sh.Ky != 0
	if blur > 0 || scaled {
		if err := colors.approximate(fmt.Errorf("%w: blurred, scaled or skewed shadow drawn approximately", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	dx, dy := dist*math.Cos(dir), dist*math.Sin(dir)
	offsets := [][2]float64{{0, 0}}
	if blur > 0 {
		for k := range renderShadowCopies - 1 {
			a := float64(k) * 2 * math.Pi / (renderShadowCopies - 1)
			offsets = append(offsets, [2]float64{blur / 2 * math.Cos(a), blur / 2 * math.Sin(a)})
		}
		// n copies of opacity a' over one another show 1-(1-a')^n.
		c.A = 1 - math.Pow(1-c.A, 1/float64(len(offsets)))
	}
	var out []layout.Op
	for _, o := range offsets {
		for _, op := range ops {
			moved, err := renderShadowOp(op, dx+o[0], dy+o[1], c)
			if err != nil {
				return nil, err
			}
			out = append(out, moved...)
			if len(out) > maxOps {
				return nil, fmt.Errorf("%w: shadow operations", render.ErrLimit)
			}
		}
	}
	return out, nil
}

// renderShadowOp is one operation's shadow: its area, moved and recolored.
// Text casts none; a picture casts its box.
func renderShadowOp(op layout.Op, dx, dy float64, c style.RGBA) ([]layout.Op, error) {
	ux, okX := style.FromPx(dx)
	uy, okY := style.FromPx(dy)
	if !okX || !okY {
		return nil, fmt.Errorf("%w: shadow offset", render.ErrLimit)
	}
	move := func(p layout.Point) layout.Point { return layout.Point{X: p.X + ux, Y: p.Y + uy} }
	path := func(p layout.Path) layout.Path {
		out := make(layout.Path, len(p))
		for i, s := range p {
			s.Point, s.Center = move(s.Point), move(s.Center)
			out[i] = s
		}
		return out
	}
	rect := func(r layout.Rect) layout.Rect { return layout.Rect{X: r.X + ux, Y: r.Y + uy, W: r.W, H: r.H} }
	switch v := op.(type) {
	case layout.FillRect:
		return []layout.Op{layout.FillRect{Rect: rect(v.Rect), Color: c}}, nil
	case layout.FillPath:
		if v.Clip.Active {
			return nil, fmt.Errorf("%w: clipped shadow", render.ErrUnsupported)
		}
		return []layout.Op{layout.FillPath{Path: path(v.Path), Color: c}}, nil
	case layout.FillGradient:
		return []layout.Op{layout.FillRect{Rect: rect(v.Clip), Color: c}}, nil
	case layout.DrawImage:
		r := v.Rect
		if v.Clip.Active {
			r = v.Clip.Rect
		}
		return []layout.Op{layout.FillRect{Rect: rect(r), Color: c}}, nil
	case layout.ClipPath:
		if _, ok := renderOnlyGradient(v.Ops); ok {
			return []layout.Op{layout.FillPath{Path: path(v.Path), Color: c}}, nil
		}
		var inner []layout.Op
		for _, o := range v.Ops {
			moved, err := renderShadowOp(o, dx, dy, c)
			if err != nil {
				return nil, err
			}
			inner = append(inner, moved...)
		}
		return []layout.Op{layout.ClipPath{Path: path(v.Path), Ops: inner}}, nil
	case layout.DrawGlyphs:
		return nil, nil
	}
	return nil, fmt.Errorf("%w: shadow of %T", render.ErrUnsupported, op)
}
