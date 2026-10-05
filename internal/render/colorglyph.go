package render

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
)

// Color glyphs. Forme paints a COLR, CBDT, sbix, EBDT or bdat glyph through a shape.Painter:
// a stream of clips, transforms and fills in font units. Here each fill becomes
// a drawing of the page: a solid, a gradient or an image inside the glyph's
// clips, so that PNG and SVG output draw it as they draw a clipped fill.
//
// A glyph is drawn whole or not at all. Its drawings are collected apart, and
// join the page's only when the whole glyph has been painted. What cannot be
// drawn exactly is either approximated where it stands, by the nearest paint
// the page has (a sweep gradient as its average color, say), or, where there is
// no such paint, the glyph is drawn as its outline in the text color. Strict
// preparation refuses both with ErrUnsupported; best-effort preparation reports
// each kind once. See approximate.

// glyphPPEM is the size a glyph is drawn at in pixels per em, which picks the
// strike of a bitmap glyph: the font size in CSS pixels, rounded up.
func glyphPPEM(size style.Unit) int {
	return int(math.Max(1, math.Min(math.Ceil(size.Px()), 1<<16)))
}

// HasBitmapFontGlyphs reports whether a glyph run has glyphs of a font with no
// outlines, painted from monochrome or greyscale strikes, which GlyphPaths
// leaves out: there is nothing to outline.
func HasBitmapFontGlyphs(v layout.DrawGlyphs) bool {
	if v.Face == nil {
		return false
	}
	ppem := glyphPPEM(v.Size)
	for _, g := range v.Glyphs {
		if v.Face.GlyphColour(g.GID, ppem) == shape.ColourMask {
			return true
		}
	}
	return false
}

// HasColorGlyphs reports whether a glyph run has glyphs painted in color (not
// bitmap fonts' masks), which
// GlyphPaths outlines in the one color of the run.
func HasColorGlyphs(v layout.DrawGlyphs) bool {
	if v.Face == nil {
		return false
	}
	ppem := glyphPPEM(v.Size)
	for _, g := range v.Glyphs {
		// A mask of a bitmap font is HasBitmapFontGlyphs'.
		if c := v.Face.GlyphColour(g.GID, ppem); c != shape.ColourNone && c != shape.ColourMask {
			return true
		}
	}
	return false
}

// approximate notes a detail drawn approximately. Best effort reports it,
// once for each reason, and goes on; strict preparation returns it, as an error
// wrapping ErrUnsupported.
func (b *prepareBudget) approximate(reason string) error {
	err := fmt.Errorf("%w: %s", ErrUnsupported, reason)
	if b.report == nil {
		return err
	}
	if !b.noted[reason] {
		if b.noted == nil {
			b.noted = map[string]bool{}
		}
		b.noted[reason] = true
		b.report(err)
	}
	return nil
}

// affine maps a point (x, y) to (a*x + c*y + e, b*x + d*y + f).
type affine struct{ a, b, c, d, e, f float64 }

func (m affine) apply(p shape.Point) (float64, float64) {
	return m.a*p.X + m.c*p.Y + m.e, m.b*p.X + m.d*p.Y + m.f
}

// then is the map that applies m and then outer.
func (m affine) then(outer affine) affine {
	return affine{
		a: outer.a*m.a + outer.c*m.b, b: outer.b*m.a + outer.d*m.b,
		c: outer.a*m.c + outer.c*m.d, d: outer.b*m.c + outer.d*m.d,
		e: outer.a*m.e + outer.c*m.f + outer.e, f: outer.b*m.e + outer.d*m.f + outer.f,
	}
}

// glyphImageKey names the image a bitmap glyph is drawn from.
type glyphImageKey struct {
	face      *shape.Face
	gid, ppem int
	// mask and color say that the image is a coverage mask painted in the
	// text's color.
	mask  bool
	color style.RGBA
}

// colorRun is one color glyph to draw: where its font units land on the page,
// and what it is drawn inside.
type colorRun struct {
	face      *shape.Face
	gid, ppem int
	// color is the text color, which paints what the font says to paint in the
	// foreground.
	color style.RGBA
	// region is the page, narrowed by the run's own clip, and clips are the
	// clips the run is drawn inside.
	region rectangle
	clips  []*geometry
	// base maps font units to CSS pixels.
	base         affine
	text, fontID string
}

// maxGradientStops bounds the stops a gradient is laid out with when its color
// line repeats. A line that would take more repeats than that is drawn as its
// average color.
const maxGradientStops = 1 << 12

// paintColorGlyph draws a color glyph and reports whether it did. It does not
// where the glyph was drawn as its outline instead, which is the caller's to
// do, after it was reported or, in strict preparation, refused here.
func (p *Page) paintColorGlyph(ctx context.Context, run colorRun, budget *prepareBudget) (bool, error) {
	c := &colorGlyph{p: p, ctx: ctx, budget: budget, run: run, cur: run.base, outer: len(run.clips), clips: append([]*geometry(nil), run.clips...)}
	operations, segments := budget.operations, budget.segments
	// The foreground is opaque, so that the font's alpha for a fill in it is
	// all that forme applies; the run's own alpha is applied here.
	fg := shape.Color{R: channel(run.color.R), G: channel(run.color.G), B: channel(run.color.B), A: 255}
	err := run.face.PaintGlyph(run.gid, shape.PaintOptions{Foreground: fg, PPEM: run.ppem}, c)
	switch {
	case c.err != nil:
		return false, c.err
	case errors.Is(err, shape.ErrPaintLimit):
		return false, fmt.Errorf("%w: color glyph %d: %w", ErrLimit, run.gid, err)
	case err != nil:
		return false, fmt.Errorf("%w: color glyph %d: %w", ErrUnsupported, run.gid, err)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if c.fallback != "" {
		budget.operations, budget.segments = operations, segments
		if err := budget.approximate("color glyph drawn as its outline in the text color: " + c.fallback); err != nil {
			return false, err
		}
		return false, nil
	}
	for _, note := range c.notes {
		if err := budget.approximate(note); err != nil {
			return false, err
		}
	}
	if len(c.out) == 0 {
		// A glyph that paints nothing still carries the text.
		c.out = append(c.out, drawing{path: &geometry{nonzero: true, curves: make([]curve, 0)}, clips: run.clips})
	}
	c.out[0].text, c.out[0].fontID = run.text, run.fontID
	p.draws = append(p.draws, c.out...)
	return true, nil
}

func channel(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(255, v)))) }

// colorGlyph is the shape.Painter that collects a glyph's drawings. Once a
// call has failed, later calls do nothing.
type colorGlyph struct {
	p      *Page
	ctx    context.Context
	budget *prepareBudget
	run    colorRun
	// cur maps the font units of what is being painted to the page, and stack
	// holds the maps outside the transforms pushed.
	cur   affine
	stack []affine
	// clips are the run's clips, outer of them, followed by the glyph's own
	// pushed; leafClips is a copy that drawings share until a clip changes.
	clips     []*geometry
	outer     int
	leafClips []*geometry
	// groups holds, for each group pushed, how many drawings came before it.
	groups []int
	out    []drawing
	// notes are the details drawn approximately.
	notes []string
	// err stops the preparation; fallback says why the glyph is to be drawn as
	// its outline.
	err      error
	fallback string
}

func (c *colorGlyph) dead() bool { return c.err != nil || c.fallback != "" }

func (c *colorGlyph) fail(reason string) {
	if c.fallback == "" {
		c.fallback = reason
	}
}

// approximate says that a detail is drawn approximately, which strict
// preparation refuses.
func (c *colorGlyph) approximate(reason string) {
	if c.budget.report == nil {
		if c.err == nil {
			c.err = fmt.Errorf("%w: %s", ErrUnsupported, reason)
		}
		return
	}
	c.notes = append(c.notes, reason)
}

func (c *colorGlyph) place(pt shape.Point) (float64, float64) { return c.cur.apply(pt) }

func (c *colorGlyph) PushTransform(t shape.Transform) {
	if c.dead() {
		return
	}
	c.stack = append(c.stack, c.cur)
	c.cur = affine{a: t.XX, b: t.YX, c: t.XY, d: t.YY, e: t.X0, f: t.Y0}.then(c.cur)
}

func (c *colorGlyph) PopTransform() {
	if c.dead() || len(c.stack) == 0 {
		return
	}
	c.cur, c.stack = c.stack[len(c.stack)-1], c.stack[:len(c.stack)-1]
}

func (c *colorGlyph) pushClip(g *geometry) {
	if len(c.clips) >= c.p.limits.MaxClipDepth {
		c.err = fmt.Errorf("%w: clip depth", ErrLimit)
		return
	}
	c.clips = append(c.clips, g)
	c.leafClips = nil
}

func (c *colorGlyph) PushClipGlyph(gid int) {
	if c.dead() {
		return
	}
	g, err := c.p.outlineGeometry(c.ctx, c.run.face, gid, c.place, c.budget)
	if err != nil {
		c.err = err
		return
	}
	c.pushClip(g)
}

func (c *colorGlyph) PushClipRect(r shape.Rect) {
	if c.dead() {
		return
	}
	if c.budget.segments > c.p.limits.MaxPathSegments-4 {
		c.err = fmt.Errorf("%w: clip segments", ErrLimit)
		return
	}
	c.budget.segments += 4
	g := &geometry{nonzero: true, curves: make([]curve, 0, 4), bounds: rectangle{x0: math.Inf(1), y0: math.Inf(1), x1: math.Inf(-1), y1: math.Inf(-1)}}
	for i, corner := range [4]shape.Point{{X: r.XMin, Y: r.YMin}, {X: r.XMax, Y: r.YMin}, {X: r.XMax, Y: r.YMax}, {X: r.XMin, Y: r.YMax}} {
		x, y := c.place(corner)
		if !finite(x) || !finite(y) || math.Abs(x) > style.MaxUnit.Px() || math.Abs(y) > style.MaxUnit.Px() {
			c.err = fmt.Errorf("%w: clip coordinate range", ErrLimit)
			return
		}
		op := byte('L')
		if i == 0 {
			op = 'M'
		}
		g.curves = append(g.curves, curve{op: op, pts: [3]point{{x, y}}})
		g.bounds.x0, g.bounds.y0 = math.Min(g.bounds.x0, x), math.Min(g.bounds.y0, y)
		g.bounds.x1, g.bounds.y1 = math.Max(g.bounds.x1, x), math.Max(g.bounds.y1, y)
	}
	c.pushClip(g)
}

func (c *colorGlyph) PopClip() {
	if c.dead() || len(c.clips) <= c.outer {
		return
	}
	c.clips = c.clips[:len(c.clips)-1]
	c.leafClips = nil
}

func (c *colorGlyph) PushGroup() {
	if c.dead() {
		return
	}
	c.groups = append(c.groups, len(c.out))
}

// PopGroup combines a group with what is beneath it. Painting a group's
// contents straight onto what is beneath it is the same as compositing the
// group source-over, and the groups of a COLR glyph are nearly all that;
// of the rest, only a group that does not show can be drawn.
func (c *colorGlyph) PopGroup(mode shape.CompositeMode) {
	if c.dead() || len(c.groups) == 0 {
		return
	}
	start := c.groups[len(c.groups)-1]
	c.groups = c.groups[:len(c.groups)-1]
	switch mode {
	case shape.CompositeSrcOver:
	case shape.CompositeDest:
		c.out = c.out[:start]
	default:
		c.fail(fmt.Sprintf("paint composite mode %d", mode))
	}
}

// area is where a paint made now lands: the page, narrowed by the run's clip
// and by the clips of the glyph's own paints. A paint with no clip of the glyph's
// own would fill the page, and is not drawn.
func (c *colorGlyph) area(clipped bool) (rectangle, bool) {
	if clipped && len(c.clips) == c.outer {
		c.fail("paint outside any clip")
		return rectangle{}, false
	}
	r := c.run.region
	for _, g := range c.clips {
		r = meet(r, g.bounds)
	}
	return r, r.x1 > r.x0 && r.y1 > r.y0
}

// leaf adds a drawing inside the current clips.
func (c *colorGlyph) leaf(d drawing, r rectangle) {
	if err := c.ctx.Err(); err != nil {
		c.err = err
		return
	}
	if c.budget.operations >= c.p.limits.MaxOperations {
		c.err = fmt.Errorf("%w: operation count", ErrLimit)
		return
	}
	c.budget.operations++
	if c.leafClips == nil {
		c.leafClips = append([]*geometry(nil), c.clips...)
	}
	d.rect, d.clips = r, c.leafClips
	c.out = append(c.out, d)
}

// color is a color of the font as the page states one: the font's palette
// color, or the text color for a fill in the foreground, with the font's alpha.
func (c *colorGlyph) color(col shape.Color, foreground bool) style.RGBA {
	alpha := float64(col.A) / 255
	if foreground {
		fg := c.run.color
		return style.RGBA{R: fg.R, G: fg.G, B: fg.B, A: alpha * fg.A}
	}
	return style.RGBA{R: float64(col.R), G: float64(col.G), B: float64(col.B), A: alpha}
}

func (c *colorGlyph) Solid(col shape.Color, foreground bool) {
	if c.dead() {
		return
	}
	c.solid(c.color(col, foreground))
}

func (c *colorGlyph) solid(col style.RGBA) {
	r, ok := c.area(true)
	if !ok || col.A <= 0 || c.dead() {
		return
	}
	r.color = col
	c.leaf(drawing{}, r)
}

// stops are a color line's stops, in order, as the page states them.
func (c *colorGlyph) stops(line shape.ColorLine) []layout.GradientStop {
	out := make([]layout.GradientStop, len(line.Stops))
	for i, s := range line.Stops {
		out[i] = layout.GradientStop{Offset: s.Offset, Color: c.color(s.Color, s.Foreground), Exponent: 1}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Offset < out[j].Offset })
	return out
}

// meanColor is the one color that stands for a color line, to approximate it:
// the average of its stops' colors, premultiplied, each weighted by the stretch
// of the line it lies in.
func meanColor(stops []layout.GradientStop) style.RGBA {
	n := len(stops)
	span := stops[n-1].Offset - stops[0].Offset
	if n == 1 || span <= 0 {
		return stops[n-1].Color
	}
	var r, g, b, a float64
	for i := 0; i+1 < n; i++ {
		w := (stops[i+1].Offset - stops[i].Offset) / span / 2
		for _, s := range stops[i : i+2] {
			r, g, b, a = r+s.Color.R*s.Color.A*w, g+s.Color.G*s.Color.A*w, b+s.Color.B*s.Color.A*w, a+s.Color.A*w
		}
	}
	if a <= 0 {
		return style.RGBA{}
	}
	return style.RGBA{R: r / a, G: g / a, B: b / a, A: a}
}

// lineStops lays a color line out over the window [ta, tb] of its parameter as
// the page states a gradient's stops, with offsets from 0 to 1 across the
// window. Where the line repeats or reflects it is laid out as many times as
// the window holds. It reports false where that is more than maxGradientStops.
func lineStops(stops []layout.GradientStop, extend shape.Extend, ta, tb float64, limit int) ([]layout.GradientStop, bool) {
	lo, hi := stops[0].Offset, stops[len(stops)-1].Offset
	period := hi - lo
	reference := layout.Gradient{Stops: stops}
	at := func(t float64) style.RGBA {
		if period > 0 {
			switch extend {
			case shape.ExtendRepeat:
				t = lo + math.Mod(t-lo, period)
				if t < lo {
					t += period
				}
			case shape.ExtendReflect:
				t = math.Mod(t-lo, 2*period)
				if t < 0 {
					t += 2 * period
				}
				if t > period {
					t = 2*period - t
				}
				t += lo
			}
		}
		return reference.ColorAtOffset(t)
	}
	offset := func(t float64) float64 { return (t - ta) / (tb - ta) }
	out := []layout.GradientStop{{Offset: 0, Color: at(ta), Exponent: 1}}
	inside := func(t float64, col style.RGBA) {
		if t > ta && t < tb {
			out = append(out, layout.GradientStop{Offset: offset(t), Color: col, Exponent: 1})
		}
	}
	if extend == shape.ExtendPad || period <= 0 {
		for _, s := range stops {
			inside(s.Offset, s.Color)
		}
	} else {
		first, last := math.Floor((ta-lo)/period), math.Ceil((tb-lo)/period)
		if !finite(first) || !finite(last) || (last-first)*float64(len(stops)) > float64(limit) {
			return nil, false
		}
		for k := first; k < last; k++ {
			reflected := extend == shape.ExtendReflect && math.Mod(k, 2) != 0
			for i := range stops {
				s := stops[i]
				t := lo + k*period + (s.Offset - lo)
				if reflected {
					s = stops[len(stops)-1-i]
					t = lo + (k+1)*period - (s.Offset - lo)
				}
				inside(t, s.Color)
			}
		}
	}
	return append(out, layout.GradientStop{Offset: 1, Color: at(tb), Exponent: 1}), true
}

// gradientPoint is a point of the page as a gradient states it.
func gradientPoint(x, y float64) (layout.Point, bool) {
	px, okX := style.FromPx(x)
	py, okY := style.FromPx(y)
	return layout.Point{X: px, Y: py}, okX && okY && finite(x) && finite(y)
}

// fillGradient adds a gradient filling an area, its stops counted against the
// page's path segments.
func (c *colorGlyph) fillGradient(g layout.Gradient, r rectangle) {
	if len(g.Stops) > c.p.limits.MaxPathSegments-c.budget.segments {
		c.err = fmt.Errorf("%w: gradient stops", ErrLimit)
		return
	}
	c.budget.segments += len(g.Stops)
	r.color = style.RGBA{A: 1}
	c.leaf(drawing{gradient: &gradientFill{g: g}}, r)
}

// gradientWindowBudget is how many stops a gradient may be laid out with.
func (c *colorGlyph) gradientWindowBudget() int {
	return max(0, min(maxGradientStops, c.p.limits.MaxPathSegments-c.budget.segments))
}

func (c *colorGlyph) LinearGradient(g shape.LinearGradient) {
	if c.dead() {
		return
	}
	r, ok := c.area(true)
	stops := c.stops(g.Line)
	if !ok || len(stops) == 0 || c.dead() {
		return
	}
	if len(stops) == 1 {
		c.solid(stops[0].Color)
		return
	}
	mean := func(reason string) {
		c.approximate(reason)
		c.solid(meanColor(stops))
	}
	if g.Line.Extend != shape.ExtendPad && stops[0].Offset == stops[len(stops)-1].Offset {
		mean("gradient color line without extent drawn as one color")
		return
	}
	// The gradient's colors stand along lines parallel to the one from P0 to
	// P2, and the page's perpendicular to its line: the line is the projection
	// of P0 to P1 onto the perpendicular to P0 to P2. A transform keeps this,
	// so the points are taken to the page first.
	x0, y0 := c.place(g.P0)
	x1, y1 := c.place(g.P1)
	x2, y2 := c.place(g.P2)
	norm := math.Hypot(x2-x0, y2-y0)
	nx, ny := (y0-y2)/norm, (x2-x0)/norm
	s := (x1-x0)*nx + (y1-y0)*ny
	if !finite(s) || !finite(norm) || norm == 0 || math.Abs(s) <= 1e-9*math.Hypot(x1-x0, y1-y0) {
		mean("degenerate linear gradient drawn as one color")
		return
	}
	// The window of the line's parameter that the area reaches.
	ta, tb := math.Inf(1), math.Inf(-1)
	for _, x := range [2]float64{r.x0, r.x1} {
		for _, y := range [2]float64{r.y0, r.y1} {
			t := ((x-x0)*nx + (y-y0)*ny) / s
			ta, tb = math.Min(ta, t), math.Max(tb, t)
		}
	}
	// A little wider, so that no end of the window is at a stop.
	pad := math.Max(1e-6*(tb-ta), 1e-9)
	ta, tb = ta-pad, tb+pad
	window, ok := lineStops(stops, g.Line.Extend, ta, tb, c.gradientWindowBudget())
	if !ok {
		mean("gradient repeating too often drawn as one color")
		return
	}
	start, okStart := gradientPoint(x0+nx*s*ta, y0+ny*s*ta)
	end, okEnd := gradientPoint(x0+nx*s*tb, y0+ny*s*tb)
	if !okStart || !okEnd {
		c.fail("gradient out of range")
		return
	}
	if start == end {
		c.solid(window[0].Color)
		return
	}
	c.fillGradient(layout.Gradient{Kind: layout.LinearGradient, Start: start, End: end, Stops: window}, r)
}

func (c *colorGlyph) RadialGradient(g shape.RadialGradient) {
	if c.dead() {
		return
	}
	r, ok := c.area(true)
	stops := c.stops(g.Line)
	if !ok || len(stops) == 0 || c.dead() {
		return
	}
	if len(stops) == 1 {
		c.solid(stops[0].Color)
		return
	}
	mean := func(reason string) {
		c.approximate(reason)
		c.solid(meanColor(stops))
	}
	if g.Line.Extend != shape.ExtendPad && stops[0].Offset == stops[len(stops)-1].Offset {
		mean("gradient color line without extent drawn as one color")
		return
	}
	// The page's radial gradient is one ellipse about a center, as wide as it
	// is high on the axes. A font's is between two circles; only circles about
	// one center, taken to such an ellipse, are drawn exactly.
	sx, sy := math.Hypot(c.cur.a, c.cur.c), math.Hypot(c.cur.b, c.cur.d)
	if !finite(sx) || !finite(sy) || sx == 0 || sy == 0 {
		return
	}
	if g.C0 != g.C1 {
		c.approximate("radial gradient between circles of different centers drawn about the end circle's")
	}
	if math.Abs(c.cur.a*c.cur.b+c.cur.c*c.cur.d) > 1e-9*sx*sy {
		c.approximate("radial gradient turned or skewed drawn with axis-aligned radii")
	}
	r0, r1 := g.R0, g.R1
	if r1 < r0 {
		// Run it from the larger circle in: the parameter reversed.
		r0, r1 = r1, r0
		for i, j := 0, len(stops)-1; i <= j; i, j = i+1, j-1 {
			stops[i], stops[j] = stops[j], stops[i]
		}
		for i := range stops {
			stops[i].Offset = 1 - stops[i].Offset
		}
	}
	dr := r1 - r0
	if !(dr > 0) || !finite(dr) || r0 < 0 {
		mean("degenerate radial gradient drawn as one color")
		return
	}
	cx, cy := c.place(g.C1)
	// How far the area reaches, in radii of the circles' units.
	reach := 0.0
	for _, x := range [2]float64{r.x0, r.x1} {
		for _, y := range [2]float64{r.y0, r.y1} {
			reach = math.Max(reach, math.Hypot((x-cx)/sx, (y-cy)/sy))
		}
	}
	// The parameter is 0 at the first circle and 1 at the last, and the radius
	// is nothing at ta, before the first circle if it has any radius.
	ta := -r0 / dr
	tb := (reach - r0) / dr
	pad := math.Max(1e-6*(tb-ta), 1e-9)
	tb += pad
	if !(tb > ta) || !finite(tb) {
		return
	}
	window, ok := lineStops(stops, g.Line.Extend, ta, tb, c.gradientWindowBudget())
	if !ok {
		mean("gradient repeating too often drawn as one color")
		return
	}
	center, okCenter := gradientPoint(cx, cy)
	radius := r0 + tb*dr
	rx, okX := style.FromPx(radius * sx)
	ry, okY := style.FromPx(radius * sy)
	if !okCenter || !okX || !okY || rx <= 0 || ry <= 0 {
		c.fail("gradient out of range")
		return
	}
	c.fillGradient(layout.Gradient{Kind: layout.RadialGradient, Center: center, RadiusX: rx, RadiusY: ry, Stops: window}, r)
}

// SweepGradient has no counterpart among the page's gradients: SVG has no
// conic gradient and the PNG painter's would not match it, so it is drawn as
// its average color.
func (c *colorGlyph) SweepGradient(g shape.SweepGradient) {
	if c.dead() {
		return
	}
	stops := c.stops(g.Line)
	if len(stops) == 0 {
		return
	}
	if len(stops) > 1 {
		c.approximate("sweep gradient drawn as one color")
	}
	c.solid(meanColor(stops))
}

// maskBitmap is the image of a monochrome or greyscale glyph (EBDT, bdat): its
// coverage as the alpha of the text's color, times the text's alpha.
func (c *colorGlyph) maskBitmap(img shape.Image) (*bitmap, error) {
	w, h := img.Width, img.Height
	if w <= 0 || h <= 0 || int64(w)*int64(h) != int64(len(img.Data)) {
		return nil, fmt.Errorf("%w: bitmap glyph mask", ErrInvalid)
	}
	if err := checkImageSize(w, h, c.p.limits); err != nil {
		return nil, err
	}
	fg := c.run.color
	rgb := [3]uint8{channel(fg.R), channel(fg.G), channel(fg.B)}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		if err := c.ctx.Err(); err != nil {
			return nil, err
		}
		row := out.Pix[y*out.Stride:]
		for x, cov := range img.Data[y*w : (y+1)*w] {
			row[4*x], row[4*x+1], row[4*x+2] = rgb[0], rgb[1], rgb[2]
			row[4*x+3] = channel(float64(cov) * fg.A)
		}
	}
	return c.p.bitmapOf(c.ctx, out, false, c.budget)
}

// scaledFromStrike reports whether a mask glyph drawn in box is resampled:
// the strike is not the size asked for, or the glyph is drawn at another size
// or stretched, or a transform of the glyph's own scales it. Placing a glyph
// between pixels is not scaling it. Only a glyph that is not scaled is exact,
// at one device pixel to the CSS pixel.
func (c *colorGlyph) scaledFromStrike(img shape.Image, box rectangle) bool {
	const tolerance = 0.1
	upem := float64(c.run.face.UnitsPerEm())
	base := c.run.base
	if !img.Exact || img.Width <= 0 || img.Height <= 0 || base.b != 0 || base.c != 0 {
		return true
	}
	if math.Abs(-base.d*upem-float64(c.run.ppem)) > 1e-6 || math.Abs(base.a+base.d) > 1e-9*math.Abs(base.d) {
		return true
	}
	return math.Abs((box.x1-box.x0)-float64(img.Width)) > tolerance || math.Abs((box.y1-box.y0)-float64(img.Height)) > tolerance
}

func (c *colorGlyph) Image(img shape.Image) {
	if c.dead() {
		return
	}
	mask := img.Format == shape.ImageMask
	if img.Format != shape.ImagePNG && !mask {
		c.fail("SVG glyph")
		return
	}
	if c.cur.b != 0 || c.cur.c != 0 {
		c.fail("bitmap glyph turned or skewed")
		return
	}
	key := glyphImageKey{face: c.run.face, gid: c.run.gid, ppem: c.run.ppem}
	if mask {
		// A mask is made in the text's color, so it is kept for it.
		key.mask, key.color = true, c.run.color
	}
	bm := c.budget.glyphImages[key]
	if bm == nil {
		var err error
		if mask {
			bm, err = c.maskBitmap(img)
		} else {
			var decoded image.Image
			decoded, err = DecodeImage(c.ctx, img.Data, c.p.limits)
			if err == nil {
				bm, err = c.p.bitmapOf(c.ctx, decoded, false, c.budget)
			}
		}
		if err != nil {
			if cerr := c.ctx.Err(); cerr != nil || errors.Is(err, ErrLimit) {
				c.err = err
				return
			}
			c.fail("bitmap glyph that cannot be read")
			return
		}
		if c.budget.glyphImages == nil {
			c.budget.glyphImages = map[glyphImageKey]*bitmap{}
		}
		c.budget.glyphImages[key] = bm
	}
	x0, y0 := c.cur.apply(shape.Point{X: img.Box.XMin, Y: img.Box.YMax})
	x1, y1 := c.cur.apply(shape.Point{X: img.Box.XMax, Y: img.Box.YMin})
	box := rectangle{x0: math.Min(x0, x1), y0: math.Min(y0, y1), x1: math.Max(x0, x1), y1: math.Max(y0, y1), color: style.RGBA{A: 1}}
	if !finite(box.x0) || !finite(box.y0) || !finite(box.x1) || !finite(box.y1) || math.Max(math.Abs(box.x0), math.Max(math.Abs(box.y0), math.Max(math.Abs(box.x1), math.Abs(box.y1)))) > style.MaxUnit.Px() {
		c.err = fmt.Errorf("%w: glyph image range", ErrLimit)
		return
	}
	if box.x1 <= box.x0 || box.y1 <= box.y0 {
		return
	}
	r, ok := c.area(false)
	if !ok {
		return
	}
	// A monochrome or greyscale strike is a pixel design for one size, so
	// scaling it is approximate. A color strike, as of an emoji font, is
	// made to be scaled, and is drawn smoothed as ever.
	if mask && c.scaledFromStrike(img, box) {
		c.approximate("bitmap font glyph scaled from its strike")
		if c.dead() {
			return
		}
	}
	r = meet(r, box)
	r.color = box.color
	c.leaf(drawing{image: bm, imageBox: box, smooth: true}, r)
}
