package pptxrender

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// Effects that blur, glow or fade are drawn from a raster of the shape:
// renderEffectScale image pixels per CSS pixel, fewer for a large shape so
// one effect's image stays within renderMaxEffectPixels, and a slide's
// effect images within renderMaxSlideEffectPixels.
const (
	renderEffectScale          = 3
	renderMaxEffectPixels      = 1 << 20
	renderMaxSlideEffectPixels = 8 << 20
)

// renderRaster is a drawing painted into a premultiplied raster whose
// origin is (x, y) in CSS pixels, scale image pixels to a CSS pixel.
type renderRaster struct {
	img         *image.RGBA
	x, y, scale float64
	w, h        int
	alpha       []float32 // coverage, 0 to 1, row by row
}

// renderOpsBounds is the box a drawing's operations cover, in CSS pixels.
func renderOpsBounds(ops []layout.Op) (x0, y0, x1, y1 float64, ok bool) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(x, y float64) {
		x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
	}
	rect := func(r layout.Rect) {
		add(r.X.Px(), r.Y.Px())
		add(r.X.Px()+r.W.Px(), r.Y.Px()+r.H.Px())
	}
	path := func(p layout.Path) {
		for _, s := range p {
			switch s.Op {
			case layout.MoveTo, layout.LineTo:
				add(s.Point.X.Px(), s.Point.Y.Px())
			case layout.ArcTo:
				cx, cy, rx, ry := s.Center.X.Px(), s.Center.Y.Px(), s.RadiusX.Px(), s.RadiusY.Px()
				add(cx-rx, cy-ry)
				add(cx+rx, cy+ry)
			}
		}
	}
	for _, op := range ops {
		switch v := op.(type) {
		case layout.FillRect:
			rect(v.Rect)
		case layout.FillPath:
			path(v.Path)
		case layout.FillGradient:
			rect(v.Clip)
		case layout.DrawImage:
			rect(v.Rect)
		case layout.ClipPath:
			path(v.Path)
		}
	}
	return x0, y0, x1, y1, x1 > x0 && y1 > y0
}

// renderTranslate moves a drawing's operations by (dx, dy) CSS pixels.
func renderTranslate(ops []layout.Op, dx, dy float64) ([]layout.Op, error) {
	ux, okX := style.FromPx(dx)
	uy, okY := style.FromPx(dy)
	if !okX || !okY {
		return nil, fmt.Errorf("%w: effect offset", render.ErrLimit)
	}
	pt := func(p layout.Point) layout.Point { return layout.Point{X: p.X + ux, Y: p.Y + uy} }
	rect := func(r layout.Rect) layout.Rect { return layout.Rect{X: r.X + ux, Y: r.Y + uy, W: r.W, H: r.H} }
	path := func(p layout.Path) layout.Path {
		out := make(layout.Path, len(p))
		for i, s := range p {
			s.Point, s.Center = pt(s.Point), pt(s.Center)
			out[i] = s
		}
		return out
	}
	clip := func(c layout.Clip) layout.Clip {
		if c.Active {
			c.Rect = rect(c.Rect)
		}
		return c
	}
	out := make([]layout.Op, 0, len(ops))
	for _, op := range ops {
		switch v := op.(type) {
		case layout.FillRect:
			v.Rect = rect(v.Rect)
			out = append(out, v)
		case layout.FillPath:
			v.Path, v.Clip = path(v.Path), clip(v.Clip)
			out = append(out, v)
		case layout.FillGradient:
			v.Clip, v.Tile = rect(v.Clip), rect(v.Tile)
			out = append(out, v)
		case layout.DrawImage:
			v.Rect, v.Clip = rect(v.Rect), clip(v.Clip)
			out = append(out, v)
		case layout.DrawGlyphs:
			v.At, v.Clip = pt(v.At), clip(v.Clip)
			out = append(out, v)
		case layout.ClipPath:
			inner, err := renderTranslate(v.Ops, dx, dy)
			if err != nil {
				return nil, err
			}
			out = append(out, layout.ClipPath{Path: path(v.Path), Ops: inner})
		default:
			return nil, fmt.Errorf("%w: effect of %T", render.ErrUnsupported, op)
		}
	}
	return out, nil
}

// rasterize paints a drawing's operations into a raster of their bounds
// grown by pad CSS pixels on every side.
func (c *renderColors) rasterize(ops []layout.Op, pad float64) (*renderRaster, error) {
	x0, y0, x1, y1, ok := renderOpsBounds(ops)
	if !ok || pad < 0 || math.IsNaN(pad) || pad > 1<<16 {
		return nil, nil
	}
	x0, y0, x1, y1 = math.Floor(x0-pad), math.Floor(y0-pad), math.Ceil(x1+pad), math.Ceil(y1+pad)
	w, h := x1-x0, y1-y0
	scale := math.Min(renderEffectScale, math.Sqrt(renderMaxEffectPixels/(w*h)))
	pixels := int64(math.Ceil(w*scale) * math.Ceil(h*scale))
	if pixels > renderMaxSlideEffectPixels-c.effectPixels {
		return nil, fmt.Errorf("%w: effect pixels", render.ErrLimit)
	}
	c.effectPixels += pixels
	moved, err := renderTranslate(ops, -x0, -y0)
	if err != nil {
		return nil, err
	}
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	// Only the shape's alpha matters here, and the shape itself is drawn, and
	// its approximations reported, on its own: a color glyph drawn approximately
	// is as good as one drawn exactly for the effect.
	page, err := core.PrepareBestEffort(ctx, dml.EMU(math.Ceil(w*float64(dml.EMUsPerPixel))), dml.EMU(math.Ceil(h*float64(dml.EMUsPerPixel))), moved, c.limits, func(error) {})
	if err != nil {
		return nil, err
	}
	img, err := page.Raster(ctx, 96*scale)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	r := &renderRaster{img: img, x: x0, y: y0, scale: scale, w: b.Dx(), h: b.Dy()}
	r.alpha = make([]float32, r.w*r.h)
	for i := range r.alpha {
		r.alpha[i] = float32(img.Pix[4*i+3]) / 255
	}
	return r, nil
}

// layer draws an alpha plane in one color as an image over the raster's
// box.
func (r *renderRaster) layer(alpha []float32, c style.RGBA, dx, dy float64) (layout.Op, error) {
	img := image.NewNRGBA(image.Rect(0, 0, r.w, r.h))
	cr, cg, cb := uint8(math.Round(c.R)), uint8(math.Round(c.G)), uint8(math.Round(c.B))
	for i, a := range alpha {
		p := img.Pix[4*i : 4*i+4]
		p[0], p[1], p[2], p[3] = cr, cg, cb, uint8(math.Round(math.Min(1, math.Max(0, float64(a)*c.A))*255))
	}
	return r.image(img, dx, dy)
}

// image draws a picture the size of the raster over its box, moved by
// (dx, dy) CSS pixels.
func (r *renderRaster) image(img image.Image, dx, dy float64) (layout.Op, error) {
	x, okX := style.FromPx(r.x + dx)
	y, okY := style.FromPx(r.y + dy)
	w, okW := style.FromPx(float64(r.w) / r.scale)
	h, okH := style.FromPx(float64(r.h) / r.scale)
	if !okX || !okY || !okW || !okH {
		return nil, fmt.Errorf("%w: effect box", render.ErrLimit)
	}
	return layout.DrawImage{Rect: layout.Rect{X: x, Y: y, W: w, H: h}, Image: img}, nil
}

// shifted is the raster's coverage moved by (dx, dy) image pixels, empty
// where it moved from outside.
func (r *renderRaster) shifted(dx, dy int, outside float32) []float32 {
	out := make([]float32, len(r.alpha))
	for y := 0; y < r.h; y++ {
		for x := 0; x < r.w; x++ {
			sx, sy := x-dx, y-dy
			if sx < 0 || sy < 0 || sx >= r.w || sy >= r.h {
				out[y*r.w+x] = outside
				continue
			}
			out[y*r.w+x] = r.alpha[sy*r.w+sx]
		}
	}
	return out
}

// renderEffectColor resolves an effect's color choice.
func (c *renderColors) effectColor(choice dml.ColorChoice, placeholder *style.RGBA) (style.RGBA, error) {
	return c.color(renderChoiceColor(&choice), placeholder)
}

// renderBlurredEffects draws a shape's outer shadow, reflection, glow, soft
// edges and inner shadow from rasters of its drawing, blurred with an
// approximate Gaussian of half each effect's radius. Office does not
// document how it draws them, so each is reported as approximate: an outer
// shadow, the shape's coverage offset and blurred in the shadow color, with
// scaling and skewing left out; a reflection, the shape flipped below its
// box, faded from its start to its end opacity and blurred; a glow, the
// coverage blurred and doubled in the glow color beneath the shape; soft
// edges, the shape's own pixels faded over the radius inside its edge; an
// inner shadow, the uncovered area offset, blurred and kept inside the
// shape. It returns what goes beneath the shape, the shape itself, and what
// goes over it.
func (c *renderColors) blurredEffects(ops []layout.Op, e *dml.EffectLst, placeholder *style.RGBA) (below, shape, above []layout.Op, err error) {
	shape = ops
	px := float64(dml.EMUsPerPixel)
	approx := func(what string) error {
		return c.approximate(fmt.Errorf("%w: %s drawn approximately", render.ErrUnsupported, what))
	}
	if sh := e.OuterShdw; sh != nil {
		col, err := c.color(renderColorOf(sh.SrgbClr, sh.SchemeClr, sh.SysClr, sh.PrstClr, sh.ScRgbClr != nil, sh.HslClr != nil), placeholder)
		if err != nil {
			return nil, nil, nil, err
		}
		blur, dist, dir := renderEffectLengths(sh.BlurRad, sh.Dist, sh.Dir)
		if blur < 0 || dist < 0 {
			return nil, nil, nil, fmt.Errorf("%w: shadow", render.ErrInvalid)
		}
		scaled := sh.Sx != nil && sh.Sx.Int32() != 100000 || sh.Sy != nil && sh.Sy.Int32() != 100000 || sh.Kx != nil && *sh.Kx != 0 || sh.Ky != nil && *sh.Ky != 0
		if blur > 0 || scaled {
			if err := approx("blurred, scaled or skewed shadow"); err != nil {
				return nil, nil, nil, err
			}
		}
		dx, dy := dist*math.Cos(dir), dist*math.Sin(dir)
		if blur == 0 {
			// A sharp shadow stays vector.
			sharp, err := renderShadow(ops, sh, placeholder, c, math.MaxInt32)
			if err != nil {
				return nil, nil, nil, err
			}
			below = append(below, sharp...)
		} else {
			r, err := c.rasterize(ops, blur)
			if err != nil || r == nil {
				return nil, nil, nil, err
			}
			alpha := append([]float32(nil), r.alpha...)
			if err := core.Blur(c.ctx, alpha, r.w, r.h, blur/2*r.scale); err != nil {
				return nil, nil, nil, err
			}
			op, err := r.layer(alpha, col, dx, dy)
			if err != nil {
				return nil, nil, nil, err
			}
			below = append(below, op)
		}
	}
	if rf := e.Reflection; rf != nil {
		if err := approx("reflection"); err != nil {
			return nil, nil, nil, err
		}
		op, err := c.reflection(ops, rf)
		if err != nil {
			return nil, nil, nil, err
		}
		if op != nil {
			below = append(below, op)
		}
	}
	if g := e.Glow; g != nil && g.Rad > 0 {
		if err := approx("glow"); err != nil {
			return nil, nil, nil, err
		}
		col, err := c.effectColor(dml.ColorChoice{SrgbClr: g.SrgbClr, SchemeClr: g.SchemeClr, SysClr: g.SysClr, PrstClr: g.PrstClr, HslClr: g.HslClr, ScrgbClr: g.ScRgbClr}, placeholder)
		if err != nil {
			return nil, nil, nil, err
		}
		rad := float64(g.Rad) / px
		r, err := c.rasterize(ops, rad)
		if err != nil || r == nil {
			return nil, nil, nil, err
		}
		alpha := append([]float32(nil), r.alpha...)
		if err := core.Blur(c.ctx, alpha, r.w, r.h, rad/2*r.scale); err != nil {
			return nil, nil, nil, err
		}
		for i, a := range alpha {
			alpha[i] = float32(math.Min(1, 2*float64(a)))
		}
		op, err := r.layer(alpha, col, 0, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		below = append(below, op)
	}
	if b := e.Blur; b != nil && b.Rad > 0 {
		// The shape itself blurred, spreading past its box unless grow is
		// off.
		if err := approx("blurred shape"); err != nil {
			return nil, nil, nil, err
		}
		rad := float64(b.Rad) / px
		pad := rad
		if b.Grow != nil && !*b.Grow {
			pad = 0
		}
		r, err := c.rasterize(shape, pad)
		if err != nil || r == nil {
			return nil, nil, nil, err
		}
		planes := make([][]float32, 4)
		for k := range planes {
			planes[k] = make([]float32, r.w*r.h)
			for i := range planes[k] {
				planes[k][i] = float32(r.img.Pix[4*i+k])
			}
			if err := core.Blur(c.ctx, planes[k], r.w, r.h, rad/2*r.scale); err != nil {
				return nil, nil, nil, err
			}
		}
		img := image.NewNRGBA(image.Rect(0, 0, r.w, r.h))
		for i := 0; i < r.w*r.h; i++ {
			if a := planes[3][i]; a > 0 {
				q := img.Pix[4*i : 4*i+4]
				q[0], q[1], q[2], q[3] = core.Byte(float64(planes[0][i]/a*255)), core.Byte(float64(planes[1][i]/a*255)), core.Byte(float64(planes[2][i]/a*255)), core.Byte(float64(a))
			}
		}
		op, err := r.image(img, 0, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		shape = []layout.Op{op}
	}
	if se := e.SoftEdge; se != nil && se.Rad > 0 {
		if err := approx("soft edges"); err != nil {
			return nil, nil, nil, err
		}
		rad := float64(se.Rad) / px
		// The margin lets the blur see the transparent outside.
		r, err := c.rasterize(ops, rad)
		if err != nil || r == nil {
			return nil, nil, nil, err
		}
		fade := append([]float32(nil), r.alpha...)
		if err := core.Blur(c.ctx, fade, r.w, r.h, rad/2*r.scale); err != nil {
			return nil, nil, nil, err
		}
		img := image.NewNRGBA(image.Rect(0, 0, r.w, r.h))
		for i := range fade {
			f := math.Min(1, math.Max(0, 2*float64(fade[i])-1))
			p, q := r.img.Pix[4*i:4*i+4], img.Pix[4*i:4*i+4]
			if a := uint32(p[3]); a > 0 {
				q[0], q[1], q[2] = uint8(uint32(p[0])*255/a), uint8(uint32(p[1])*255/a), uint8(uint32(p[2])*255/a)
				q[3] = uint8(math.Round(float64(a) * f))
			}
		}
		op, err := r.image(img, 0, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		shape = []layout.Op{op}
	}
	if sh := e.InnerShdw; sh != nil {
		if err := approx("inner shadow"); err != nil {
			return nil, nil, nil, err
		}
		col, err := c.effectColor(dml.ColorChoice{SrgbClr: sh.SrgbClr, SchemeClr: sh.SchemeClr, SysClr: sh.SysClr, PrstClr: sh.PrstClr, HslClr: sh.HslClr, ScrgbClr: sh.ScRgbClr}, placeholder)
		if err != nil {
			return nil, nil, nil, err
		}
		blur, dist, dir := renderEffectLengths(sh.BlurRad, sh.Dist, sh.Dir)
		r, err := c.rasterize(ops, 0)
		if err != nil || r == nil {
			return nil, nil, nil, err
		}
		dx, dy := int(math.Round(dist*math.Cos(dir)*r.scale)), int(math.Round(dist*math.Sin(dir)*r.scale))
		// The area outside the shape, cast inwards by the offset.
		inverse := r.shifted(dx, dy, 0)
		for i, a := range inverse {
			inverse[i] = 1 - a
		}
		if err := core.Blur(c.ctx, inverse, r.w, r.h, blur/2*r.scale); err != nil {
			return nil, nil, nil, err
		}
		for i := range inverse {
			inverse[i] *= r.alpha[i]
		}
		op, err := r.layer(inverse, col, 0, 0)
		if err != nil {
			return nil, nil, nil, err
		}
		above = append(above, op)
	}
	return below, shape, above, nil
}

// renderEffectLengths reads an effect's blur radius and distance in CSS
// pixels and its direction in radians.
func renderEffectLengths(blurRad, dist *int64, dir *int32) (blur, distance, angle float64) {
	px := float64(dml.EMUsPerPixel)
	if blurRad != nil {
		blur = float64(*blurRad) / px
	}
	if dist != nil {
		distance = float64(*dist) / px
	}
	if dir != nil {
		angle = float64(*dir) / 60000 * math.Pi / 180
	}
	return blur, distance, angle
}

// reflection draws a shape's reflection: its raster flipped about the
// bottom of its box, dist below it, faded from stA at stPos to endA at
// endPos down the reflection, and blurred by blurRad. Reflections in other
// directions or scaled other than flipped are drawn as below, approximately.
func (c *renderColors) reflection(ops []layout.Op, rf *dml.ReflectionXML) (layout.Op, error) {
	pct := func(p *dml.Percentage, def float64) float64 {
		if p == nil {
			return def
		}
		return float64(p.Int32()) / 100000
	}
	blur, dist, _ := renderEffectLengths(rf.BlurRad, rf.Dist, nil)
	stA, stPos, endA, endPos := pct(rf.StA, 1), pct(rf.StPos, 0), pct(rf.EndA, 0), pct(rf.EndPos, 1)
	r, err := c.rasterize(ops, 0)
	if err != nil || r == nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, r.w, r.h))
	alpha := make([]float32, r.w*r.h)
	for y := 0; y < r.h; y++ {
		src := r.h - 1 - y
		t := (float64(y) + 0.5) / float64(r.h)
		f := 0.0
		switch {
		case t < stPos:
			f = stA
		case t <= endPos && endPos > stPos:
			f = stA + (endA-stA)*(t-stPos)/(endPos-stPos)
		case t <= endPos:
			f = endA
		}
		for x := 0; x < r.w; x++ {
			p, q := r.img.Pix[4*(src*r.w+x):], img.Pix[4*(y*r.w+x):]
			if a := uint32(p[3]); a > 0 {
				q[0], q[1], q[2] = uint8(uint32(p[0])*255/a), uint8(uint32(p[1])*255/a), uint8(uint32(p[2])*255/a)
			}
			alpha[y*r.w+x] = float32(float64(p[3]) / 255 * math.Max(0, math.Min(1, f)))
		}
	}
	if err := core.Blur(c.ctx, alpha, r.w, r.h, blur/2*r.scale); err != nil {
		return nil, err
	}
	for i, a := range alpha {
		img.Pix[4*i+3] = uint8(math.Round(math.Min(1, math.Max(0, float64(a))) * 255))
	}
	return r.image(img, 0, float64(r.h)/r.scale+dist)
}
