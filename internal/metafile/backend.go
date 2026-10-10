package metafile

import (
	"context"
	"fmt"
	"math"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

var (
	errPath       = fmt.Errorf("%w: metafile: path structure", render.ErrInvalid)
	errCoordinate = fmt.Errorf("%w: metafile coordinate", render.ErrInvalid)
)

// backend draws what gowemf.Play resolves onto the raster. Play hands it
// geometry in destination coordinates, which are the raster's pixels: the
// picture's destination box is the whole raster.
type backend struct {
	ctx  context.Context
	opts Options
	r    *raster
	b    *budget
	tol  float64 // flattening tolerance in pixels

	seen map[string]bool

	// Clip regions are immutable and shared, so their coverage is cached by
	// node and translation; whole clips by the regions they combine.
	regions  map[regionKey]*clip
	clips    map[string]*clip
	regionID map[*gowemf.ClipRegion]int

	// Fonts are resolved once for each request.
	faces    map[render.FontRequest]*shape.Face
	noFace   map[render.FontRequest]bool
	maxFonts int

	// Images converted for drawing, by source image and rectangle.
	images map[imageKey]*converted
}

func newBackend(ctx context.Context, w, h int, opts Options, b *budget, maxFonts int) *backend {
	return &backend{
		ctx: ctx, opts: opts, r: newRaster(ctx, w, h, b), b: b, tol: 1.0 / 16,
		seen: map[string]bool{}, regions: map[regionKey]*clip{}, clips: map[string]*clip{},
		regionID: map[*gowemf.ClipRegion]int{}, maxFonts: maxFonts, images: map[imageKey]*converted{},
	}
}

// soft reports a detail drawn approximately or left out and returns nil in
// best-effort mode, or the error in strict mode. Each message is reported
// once.
func (be *backend) soft(format string, args ...any) error {
	err := fmt.Errorf("%w: metafile: %s", render.ErrUnsupported, fmt.Sprintf(format, args...))
	if be.opts.Approximate == nil {
		return err
	}
	if be.seen[err.Error()] {
		return nil
	}
	if len(be.seen) < 64 {
		be.seen[err.Error()] = true
	}
	return be.opts.Approximate(err)
}

// unsupported receives what Play cannot draw: best effort reports it and has
// Play leave it out.
func (be *backend) unsupported(op gowemf.UnsupportedOperation) error {
	return be.soft("%s left out (%s)", op.Reason, recordName(op.Source))
}

// recordName names a record's kind for a report, without its position, so a
// repeated record is reported once.
func recordName(r gowemf.Record) string {
	switch r.Format {
	case gowemf.WMF:
		return fmt.Sprintf("WMF record %#x", r.Type&0xff)
	case gowemf.EMFPlus:
		return fmt.Sprintf("EMF+ record %#x", r.Type)
	}
	return fmt.Sprintf("EMF record %d", r.Type)
}

// begin starts a drawing call: it checks for cancellation and charges one
// operation.
func (be *backend) begin() error {
	if err := be.ctx.Err(); err != nil {
		return err
	}
	return be.b.op()
}

func (be *backend) charge(n int) error { return be.b.addSegments(n) }

// FillPath fills a path with a paint under a clip.
func (be *backend) FillPath(path gowemf.Path, rule gowemf.FillRule, paint gowemf.Paint, clip gowemf.Clip) error {
	if err := be.begin(); err != nil {
		return err
	}
	figs, err := figures(path, be.tol, be.charge)
	if err != nil {
		return err
	}
	st, ok, err := be.fillStyle(paint)
	if err != nil || !ok {
		return err
	}
	c, err := be.clip(clip)
	if err != nil {
		return err
	}
	return be.r.fill(fillContours(figs), rule != gowemf.NonZero, st, c)
}

// StrokePath strokes a path with a pen under a clip.
func (be *backend) StrokePath(path gowemf.Path, s gowemf.Stroke, clip gowemf.Clip) error {
	if err := be.begin(); err != nil {
		return err
	}
	figs, err := figures(path, be.tol, be.charge)
	if err != nil {
		return err
	}
	pen, err := be.pen(s)
	if err != nil || pen == nil {
		return err
	}
	c, err := be.clip(clip)
	if err != nil {
		return err
	}
	if len(s.Compound) > 0 {
		return be.compound(figs, pen, s, c)
	}
	if s.Gap != nil && len(pen.style.dash) > 0 {
		// Opaque background mode paints the gaps between dashes.
		gap, ok, err := be.fillStyle(*s.Gap)
		if err != nil {
			return err
		}
		if ok {
			solid := *pen
			solid.style.dash = nil
			polys, err := solid.outline(figs, be.charge)
			if err != nil {
				return err
			}
			if err = be.r.fill(polys, false, gap, c); err != nil {
				return err
			}
		}
	}
	st, ok, err := be.fillStyle(s.Paint)
	if err != nil || !ok {
		return err
	}
	polys, err := pen.outline(figs, be.charge)
	if err != nil || len(polys) == 0 {
		return err
	}
	return be.r.fill(polys, false, st, c)
}

// penOutline widens figures in pen space and maps the result to pixels.
type penOutline struct {
	style   strokeStyle
	toPen   affine // pixels to pen space
	fromPen affine // pen space to pixels
	tol     float64
}

// outline is the stroke's area as polygons that fill it by the nonzero rule.
func (p *penOutline) outline(figs []polyline, charge func(int) error) ([][]point, error) {
	lines := make([]polyline, len(figs))
	for i, f := range figs {
		pts := make([]point, len(f.pts))
		for k, q := range f.pts {
			pts[k] = p.toPen.apply(q)
		}
		lines[i] = polyline{pts: pts, closed: f.closed}
	}
	polys, err := strokeLines(lines, p.style, p.tol, charge)
	if err != nil {
		return nil, err
	}
	for _, poly := range polys {
		for k, q := range poly {
			poly[k] = p.fromPen.apply(q)
		}
		positive(poly)
	}
	return polys, nil
}

// Windows' predefined dash patterns, in units of the pen width for a
// geometric pen and of device pixels for a hairline. The lengths are what GDI
// uses on a display; they depend on the device.
var presetDashes = map[gowemf.DashStyle][]float64{
	gowemf.DashDash:       {18, 6},
	gowemf.DashDot:        {3, 3},
	gowemf.DashDashDot:    {9, 6, 3, 6},
	gowemf.DashDashDotDot: {9, 3, 3, 3, 3, 3},
	gowemf.DashAlternate:  {1, 1},
}

// pen resolves a stroke into how its outline is made, or nil when it draws
// nothing.
func (be *backend) pen(s gowemf.Stroke) (*penOutline, error) {
	// GDI strokes come with the half device pixel GDI draws lines through;
	// EMF+ strokes without, their geometry already placed. The raster is the
	// device GDI draws on, as when a picture is played onto a page: its
	// lines run through raster pixel centers, are never narrower than a
	// raster pixel, and a hairline is one raster pixel wide.
	gdi := s.PixelCenter != (gowemf.Point{})
	center := identityAffine
	if gdi {
		center = affine{a: 1, d: 1, e: 0.5, f: 0.5}
	}
	p := &penOutline{tol: be.tol}
	// The unit of dash lengths, where they are in pen widths or in pen
	// space: the pen width in pen space, or a pixel for a hairline.
	unit, userUnit := s.Width, 1.0
	if s.Hairline {
		// A hairline is one pixel wide, with square ends and mitered corners
		// so that its pixels run on along the line.
		unit = 1
		p.style = strokeStyle{width: 1, cap: capSquare, endCap: capSquare, join: joinMiter, miter: 2}
		p.toPen, p.fromPen = center, identityAffine
	} else {
		if !(s.Width > 0) || !finite(s.Width) {
			return nil, nil
		}
		from := fromMatrix(s.Transform)
		inv, ok := from.invert()
		if !ok {
			// A pen squashed to nothing draws nothing.
			return nil, nil
		}
		p.toPen, p.fromPen = center.then(inv), from
		big, small := from.singular()
		if big > 0 {
			p.tol = be.tol / big
		}
		p.style = strokeStyle{width: s.Width, cap: capOf(s.Cap), endCap: capOf(s.Cap), join: joinOf(s.Join), miter: s.MiterLimit}
		if s.EndCap != 0 {
			p.style.endCap = capOf(s.EndCap)
		}
		if gdi && s.Width*big < 1 {
			// A pen narrower than a pixel every way is drawn a pixel wide,
			// in raster space, with its dashes at the transform's mean scale.
			mean := math.Sqrt(big * small)
			p.style.width = 1
			p.toPen, p.fromPen, p.tol = center, identityAffine, be.tol
			unit, userUnit = s.Width*mean, mean
		}
	}
	switch s.Dash {
	case gowemf.DashSolid, 0:
	case gowemf.DashUser:
		for _, d := range s.Dashes {
			p.style.dash = append(p.style.dash, d*userUnit)
		}
		p.style.dashOffset = s.DashOffset * userUnit
	default:
		pattern, ok := presetDashes[s.Dash]
		if !ok {
			return nil, fmt.Errorf("%w: metafile: dash style %d", render.ErrInvalid, s.Dash)
		}
		if err := be.soft("preset pen dash pattern drawn from Windows' display lengths"); err != nil {
			return nil, err
		}
		u := unit
		for _, d := range pattern {
			p.style.dash = append(p.style.dash, d*u)
		}
		p.style.dashOffset = s.DashOffset * userUnit
	}
	return p, nil
}

var identityAffine = affine{a: 1, d: 1}

func capOf(c gowemf.LineCap) capStyle {
	switch c {
	case gowemf.CapSquare:
		return capSquare
	case gowemf.CapFlat:
		return capFlat
	}
	return capRound
}

func joinOf(j gowemf.LineJoin) joinStyle {
	switch j {
	case gowemf.JoinBevel:
		return joinBevel
	case gowemf.JoinMiter:
		return joinMiter
	}
	return joinRound
}

// compound draws a compound pen: parallel bands across its width. Play passes
// only arrays symmetric about the center, so each band is exactly the stroke
// at its outer edge minus the stroke at its inner edge (or the whole stroke at
// its outer edge for the band across the center).
func (be *backend) compound(figs []polyline, pen *penOutline, s gowemf.Stroke, c *clip) error {
	if len(s.Compound)%2 != 0 || s.Hairline {
		return fmt.Errorf("%w: metafile: compound pen", render.ErrInvalid)
	}
	st, ok, err := be.fillStyle(s.Paint)
	if err != nil || !ok {
		return err
	}
	width := pen.style.width
	sized := func(f float64) ([][]point, error) {
		q := *pen
		q.style.width = width * f
		return q.outline(figs, be.charge)
	}
	type band struct{ outer, inner [][]point }
	var bands []band
	var all [][]point
	for i := 0; i+1 < len(s.Compound); i += 2 {
		a, b := s.Compound[i], s.Compound[i+1]
		switch {
		case b <= 0.5:
			outer, err := sized(1 - 2*a)
			if err != nil {
				return err
			}
			inner, err := sized(1 - 2*b)
			if err != nil {
				return err
			}
			bands = append(bands, band{outer, inner})
			all = append(all, outer...)
		case a < 0.5:
			outer, err := sized(1 - 2*a)
			if err != nil {
				return err
			}
			bands = append(bands, band{outer: outer})
			all = append(all, outer...)
		}
		// Bands past the center mirror those before it.
	}
	if len(bands) == 0 {
		return nil
	}
	x0, y0, x1, y1, ok := be.r.contourBounds(all)
	if !ok {
		return nil
	}
	area := c.within(x0, y0, x1, y1)
	if area.empty() {
		return nil
	}
	acc, err := be.r.newCoverage(area)
	if err != nil {
		return err
	}
	for _, bd := range bands {
		outer, err := be.r.coverage(bd.outer, false, area)
		if err != nil {
			return err
		}
		if bd.inner != nil {
			inner, err := be.r.coverage(bd.inner, false, area)
			if err != nil {
				return err
			}
			for i := range outer.a {
				outer.a[i] *= 1 - inner.a[i]
			}
		}
		for i := range acc.a {
			// The bands are disjoint: their union adds.
			acc.a[i] = float32(math.Min(1, float64(acc.a[i]+outer.a[i])))
		}
	}
	return be.r.paintCoverage(acc, st, c)
}
