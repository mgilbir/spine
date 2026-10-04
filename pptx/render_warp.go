package pptx

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/presetgeom"
	"github.com/mgilbir/spine/render"
)

// renderWarpStep is the longest straight piece, in CSS pixels, a warped
// outline keeps: longer edges are divided so they bend with the warp.
const renderWarpStep = 1.0

// renderPolyline is a warp path flattened, measured along its length.
type renderPolyline struct {
	pts [][2]float64
	at  []float64 // cumulative length to each point
}

func newRenderPolyline(pts [][2]float64) renderPolyline {
	p := renderPolyline{pts: pts, at: make([]float64, len(pts))}
	for i := 1; i < len(pts); i++ {
		p.at[i] = p.at[i-1] + math.Hypot(pts[i][0]-pts[i-1][0], pts[i][1]-pts[i-1][1])
	}
	return p
}

// point is the point a share u of the way along the polyline, and the
// direction it runs there.
func (p renderPolyline) point(u float64) (x, y, dx, dy float64) {
	n := len(p.pts)
	if n == 1 {
		return p.pts[0][0], p.pts[0][1], 1, 0
	}
	total := p.at[n-1]
	s := math.Max(0, math.Min(1, u)) * total
	i := sort.SearchFloat64s(p.at, s)
	i = max(1, min(i, n-1))
	a, b := p.pts[i-1], p.pts[i]
	seg := p.at[i] - p.at[i-1]
	t := 0.0
	if seg > 0 {
		t = (s - p.at[i-1]) / seg
	}
	dx, dy = b[0]-a[0], b[1]-a[1]
	if l := math.Hypot(dx, dy); l > 0 {
		dx, dy = dx/l, dy/l
	}
	return a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, dx, dy
}

// renderWarpPiece is a run's outlines awaiting the warp, and its band.
type renderWarpPiece struct {
	paths []layout.Path
	color style.RGBA
	band  int
}

// renderWarpText maps a text body's glyph runs, underlines and highlights
// onto a preset text warp over its box (x, y, w, h EMU). The warp's paths,
// evaluated for the box, come in pairs, the top and bottom of a band of
// lines, or each alone, a line its text bends along; lines go to the band
// that holds their middle. A band's text is stretched to its paths: across
// their length, and for a pair from one path to the other, or for a single
// path at its own height, hanging inwards from it. Office does not document how it
// fits text to a warp, so the result is reported as approximate.
func renderWarpText(ctx context.Context, ops []layout.Op, warp *dml.PrstTxWarp, x, y, w, h dml.EMU, colors *renderColors, maxSegments int) ([]layout.Op, error) {
	if warp.Prst == "" || warp.Prst == "textNoShape" || len(ops) == 0 {
		return ops, nil
	}
	def, ok := presetgeom.LookupTextWarp(warp.Prst)
	if !ok {
		return nil, fmt.Errorf("%w: text warp %q", render.ErrInvalid, warp.Prst)
	}
	if err := colors.approximate(fmt.Errorf("%w: text warp %s drawn approximately", render.ErrUnsupported, warp.Prst)); err != nil {
		return nil, err
	}
	cg := *def
	if warp.AvLst != nil {
		var adjust dml.AvLst
		if def.AvLst != nil {
			adjust.Gd = append(adjust.Gd, def.AvLst.Gd...)
		}
		adjust.Gd = append(adjust.Gd, warp.AvLst.Gd...)
		cg.AvLst = &adjust
	}
	paths, _, err := renderCustomGeometry(&cg, x, y, w, h, maxSegments)
	if err != nil {
		return nil, err
	}
	var lines []renderPolyline
	for _, p := range paths {
		var pts [][2]float64
		for _, sub := range p.subpaths {
			pts = append(pts, sub.pts...)
		}
		if len(pts) == 0 {
			return nil, fmt.Errorf("%w: empty text warp path", render.ErrInvalid)
		}
		lines = append(lines, newRenderPolyline(pts))
	}
	paired := len(lines)%2 == 0
	bands := len(lines)
	if paired {
		bands /= 2
	}
	if bands == 0 {
		return ops, nil
	}
	// Outline every run as drawn, unwarped.
	segments := 0
	same := func(x, y float64) (float64, float64) { return x, y }
	var pieces []renderWarpPiece
	for _, op := range ops {
		switch v := op.(type) {
		case layout.DrawGlyphs:
			outlines, _, err := core.GlyphPaths(ctx, v, same, maxSegments, &segments)
			if err != nil {
				return nil, err
			}
			pieces = append(pieces, renderWarpPiece{paths: outlines, color: v.Color})
		case layout.FillRect:
			pieces = append(pieces, renderWarpPiece{paths: []layout.Path{renderRectPath(v.Rect)}, color: v.Color})
		case layout.FillPath:
			if v.Clip.Active {
				return nil, fmt.Errorf("%w: clipped warped text", render.ErrUnsupported)
			}
			pieces = append(pieces, renderWarpPiece{paths: []layout.Path{v.Path}, color: v.Color})
		default:
			return nil, fmt.Errorf("%w: warped %T", render.ErrUnsupported, op)
		}
	}
	bounds := func(ps []layout.Path) (x0, y0, x1, y1 float64) {
		x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, p := range ps {
			for _, s := range p {
				if s.Op == layout.ClosePath {
					continue
				}
				px, py := s.Point.X.Px(), s.Point.Y.Px()
				x0, y0, x1, y1 = math.Min(x0, px), math.Min(y0, py), math.Max(x1, px), math.Max(y1, py)
			}
		}
		return
	}
	var all []layout.Path
	for _, p := range pieces {
		all = append(all, p.paths...)
	}
	_, top, _, bottom := bounds(all)
	if !(bottom > top) {
		return ops, nil
	}
	// Each run goes to the band holding its middle.
	for i := range pieces {
		_, py0, _, py1 := bounds(pieces[i].paths)
		mid := (py0 + py1) / 2
		pieces[i].band = max(0, min(bands-1, int((mid-top)/(bottom-top)*float64(bands))))
	}
	var out []layout.Op
	for b := 0; b < bands; b++ {
		var in []layout.Path
		for _, p := range pieces {
			if p.band == b {
				in = append(in, p.paths...)
			}
		}
		if len(in) == 0 {
			continue
		}
		bx0, by0, bx1, by1 := bounds(in)
		bw, bh := math.Max(bx1-bx0, 1e-9), math.Max(by1-by0, 1e-9)
		var mapPoint func(px, py float64) (float64, float64)
		if paired {
			upper, lower := lines[2*b], lines[2*b+1]
			mapPoint = func(px, py float64) (float64, float64) {
				u, v := (px-bx0)/bw, (py-by0)/bh
				ux, uy, _, _ := upper.point(u)
				lx, ly, _, _ := lower.point(u)
				return ux + (lx-ux)*v, uy + (ly-uy)*v
			}
		} else {
			line := lines[b]
			// Text runs along the path, up to its left, and hangs inwards
			// from it, towards the box's middle: from its top where the
			// path's left faces out, as along a top arch or round a circle,
			// standing on it where the left faces in, as along a bottom
			// arch, and centred on a path through the middle.
			cx, cy := (float64(x)+float64(w)/2)/float64(dml.EMUsPerPixel), (float64(y)+float64(h)/2)/float64(dml.EMUsPerPixel)
			mx, my, mdx, mdy := line.point(0.5)
			facing := mdy*(mx-cx) - mdx*(my-cy) // left normal against the outward direction
			edge := (by0 + by1) / 2
			switch {
			case facing > bh/4:
				edge = by0
			case facing < -bh/4:
				edge = by1
			}
			mapPoint = func(px, py float64) (float64, float64) {
				u := (px - bx0) / bw
				lx, ly, dx, dy := line.point(u)
				above := edge - py
				return lx + dy*above, ly - dx*above
			}
		}
		for _, p := range pieces {
			if p.band != b {
				continue
			}
			for _, path := range p.paths {
				warped, err := renderWarpPath(path, mapPoint, maxSegments, &segments)
				if err != nil {
					return nil, err
				}
				out = append(out, layout.FillPath{Path: warped, Color: p.color})
			}
		}
	}
	return out, nil
}

// renderWarpPath maps a straight-edged path's points, dividing its edges
// into pieces of at most renderWarpStep so they bend with the map.
func renderWarpPath(p layout.Path, mapPoint func(x, y float64) (float64, float64), maxSegments int, segments *int) (layout.Path, error) {
	out := make(layout.Path, 0, len(p))
	add := func(op layout.PathOp, x, y float64) error {
		if *segments >= maxSegments {
			return fmt.Errorf("%w: warped text segments", render.ErrLimit)
		}
		*segments++
		mx, my := mapPoint(x, y)
		ux, okX := style.FromPx(mx)
		uy, okY := style.FromPx(my)
		if !okX || !okY {
			return fmt.Errorf("%w: warped coordinate", render.ErrLimit)
		}
		out = append(out, layout.PathSegment{Op: op, Point: layout.Point{X: ux, Y: uy}})
		return nil
	}
	var startX, startY, curX, curY float64
	line := func(x, y float64) error {
		n := int(math.Ceil(math.Hypot(x-curX, y-curY) / renderWarpStep))
		for i := 1; i <= n; i++ {
			t := float64(i) / float64(n)
			if err := add(layout.LineTo, curX+(x-curX)*t, curY+(y-curY)*t); err != nil {
				return err
			}
		}
		curX, curY = x, y
		return nil
	}
	for _, s := range p {
		x, y := s.Point.X.Px(), s.Point.Y.Px()
		var err error
		switch s.Op {
		case layout.MoveTo:
			startX, startY, curX, curY = x, y, x, y
			err = add(layout.MoveTo, x, y)
		case layout.LineTo:
			err = line(x, y)
		case layout.ClosePath:
			if err = line(startX, startY); err == nil {
				out = append(out, s)
			}
		default:
			return nil, fmt.Errorf("%w: warped arc", render.ErrUnsupported)
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
