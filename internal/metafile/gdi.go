package metafile

import (
	"context"
	"fmt"
	"math"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

// affine maps (x, y) to (a*x + c*y + e, b*x + d*y + f).
type affine struct{ a, b, c, d, e, f float64 }

var identity = affine{a: 1, d: 1}

func (m affine) apply(p point) point {
	return point{m.a*p.x + m.c*p.y + m.e, m.b*p.x + m.d*p.y + m.f}
}

// then is the transform that applies m, then n.
func (m affine) then(n affine) affine {
	return affine{
		a: n.a*m.a + n.c*m.b, b: n.b*m.a + n.d*m.b,
		c: n.a*m.c + n.c*m.d, d: n.b*m.c + n.d*m.d,
		e: n.a*m.e + n.c*m.f + n.e, f: n.b*m.e + n.d*m.f + n.f,
	}
}

func (m affine) finite() bool {
	return finite(m.a) && finite(m.b) && finite(m.c) && finite(m.d) && finite(m.e) && finite(m.f)
}

// singular returns the larger and smaller scale of the linear part.
func (m affine) singular() (big, small float64) {
	p := m.a*m.a + m.b*m.b + m.c*m.c + m.d*m.d
	q := math.Abs(m.a*m.d - m.b*m.c)
	disc := math.Sqrt(math.Max(0, p*p-4*q*q))
	big = math.Sqrt((p + disc) / 2)
	small = math.Sqrt(math.Max(0, (p-disc)/2))
	return
}

func fromMatrix(m gowemf.Matrix) affine {
	return affine{a: m.M11, b: m.M12, c: m.M21, d: m.M22, e: m.Dx, f: m.Dy}
}

// Mapping modes.
const (
	mmText        = 1
	mmLoMetric    = 2
	mmHiMetric    = 3
	mmLoEnglish   = 4
	mmHiEnglish   = 5
	mmTwips       = 6
	mmIsotropic   = 7
	mmAnisotropic = 8
)

// dc is a device context: what SaveDC saves.
type dc struct {
	mapMode        int
	wndOrg, wndExt point
	vpOrg, vpExt   point
	world          affine
	pen            *penObj
	brush          *brushObj
	font           *gowemf.Font
	textColor      uint32
	bkColor        uint32
	bkMode         uint32
	polyFill       uint32
	rop2           uint32
	textAlign      uint32
	charExtra      int32
	justExtra      int32
	justCount      int32
	arcClockwise   bool
	miter          float64
	pos            point // current position, logical
	clip           *clip
	stretchMode    uint32
}

type penObj struct {
	null        bool
	cosmetic    bool
	width       float64 // logical units
	color       uint32
	cap         capStyle
	join        joinStyle
	userDash    []float64
	preset      int // 1 dash, 2 dot, 3 dash-dot, 4 dash-dot-dot
	insideFrame bool
	unsupported string // why the pen cannot be drawn exactly, if so
}

type brushObj struct {
	null        bool
	color       uint32
	style       uint32
	hatch       uint32
	unsupported string
}

type interp struct {
	ctx  context.Context
	opts Options
	r    *raster
	b    *budget
	emf  bool

	// Device geometry: pixels per millimetre of the reference device, for
	// the fixed mapping modes and isotropic scaling.
	pxPerMmX, pxPerMmY float64
	mmX, mmY           float64 // millimetres across the device, for isotropic fixing

	dc    dc
	stack []dc
	objs  map[uint32]any
	path  *pathBuilder
	done  *pathBuilder // the last completed path

	faces    map[render.FontRequest]*shape.Face
	maxFonts int
	seen     map[string]bool

	tol float64 // flattening tolerance in device units
}

type pathBuilder struct {
	figs  []polyline
	open  bool
	start point // logical start of the current figure
}

func (p *pathBuilder) move(pt point) {
	p.figs = append(p.figs, polyline{pts: []point{pt}})
	p.open = true
}

func (p *pathBuilder) lineTo(pt point) {
	p.figs[len(p.figs)-1].pts = append(p.figs[len(p.figs)-1].pts, pt)
}

// soft reports a detail the metafile draws approximately or leaves out and
// returns nil in best-effort mode, or the error in strict mode.
func (it *interp) soft(format string, args ...any) error {
	err := fmt.Errorf("%w: metafile: %s", render.ErrUnsupported, fmt.Sprintf(format, args...))
	if it.opts.Approximate == nil {
		return err
	}
	if it.seen[err.Error()] {
		return nil
	}
	if len(it.seen) < 64 {
		it.seen[err.Error()] = true
	}
	return it.opts.Approximate(err)
}

func (it *interp) lenient() bool { return it.opts.Approximate != nil }

func (it *interp) charge(n int) error { return it.b.addSegments(n) }

// matrix is the transform from logical coordinates to device units.
func (it *interp) matrix() affine {
	d := &it.dc
	sx, sy := 1.0, 1.0
	if d.wndExt.x != 0 {
		sx = d.vpExt.x / d.wndExt.x
	}
	if d.wndExt.y != 0 {
		sy = d.vpExt.y / d.wndExt.y
	}
	m := affine{a: sx, d: sy, e: d.vpOrg.x - d.wndOrg.x*sx, f: d.vpOrg.y - d.wndOrg.y*sy}
	return d.world.then(m)
}

func (it *interp) dev(x, y float64) point { return it.matrix().apply(point{x, y}) }

// fixedScale is the device units per logical unit of a fixed mapping mode.
func (it *interp) fixedScale(mode int) (sx, sy float64) {
	var mm float64
	switch mode {
	case mmLoMetric:
		mm = 0.1
	case mmHiMetric:
		mm = 0.01
	case mmLoEnglish:
		mm = 0.254
	case mmHiEnglish:
		mm = 0.0254
	case mmTwips:
		mm = 25.4 / 1440
	default:
		return 1, 1
	}
	return mm * it.pxPerMmX, -mm * it.pxPerMmY
}

func (it *interp) setMapMode(mode int) error {
	d := &it.dc
	switch mode {
	case mmText:
		d.wndExt, d.vpExt = point{1, 1}, point{1, 1}
	case mmLoMetric, mmHiMetric, mmLoEnglish, mmHiEnglish, mmTwips:
		sx, sy := it.fixedScale(mode)
		d.wndExt, d.vpExt = point{1, 1}, point{sx, sy}
	case mmIsotropic, mmAnisotropic:
	default:
		return fmt.Errorf("%w: metafile: mapping mode %d", render.ErrInvalid, mode)
	}
	d.mapMode = mode
	it.fixIsotropic()
	return nil
}

// fixIsotropic keeps the units square in the isotropic mapping mode, by
// shrinking whichever viewport extent would stretch them.
func (it *interp) fixIsotropic() {
	d := &it.dc
	if d.mapMode != mmIsotropic || d.wndExt.x == 0 || d.wndExt.y == 0 || it.mmX == 0 || it.mmY == 0 {
		return
	}
	xdim := math.Abs(d.vpExt.x * it.mmY / (it.mmX * d.wndExt.x))
	ydim := math.Abs(d.vpExt.y * it.mmX / (it.mmY * d.wndExt.y))
	switch {
	case xdim > ydim && xdim != 0:
		d.vpExt.x = d.vpExt.x * ydim / xdim
	case ydim > xdim && ydim != 0:
		d.vpExt.y = d.vpExt.y * xdim / ydim
	}
}

func (it *interp) fixedMode() bool {
	m := it.dc.mapMode
	return m != mmIsotropic && m != mmAnisotropic
}

// Colors.

func (it *interp) color(c uint32) (rgba, error) {
	switch c >> 24 {
	case 0, 2:
		return rgba{uint8(c), uint8(c >> 8), uint8(c >> 16), 255}, nil
	}
	if err := it.soft("palette-index color drawn black"); err != nil {
		return rgba{}, err
	}
	return rgba{0, 0, 0, 255}, nil
}

// Drawing.

// logicalPoints converts GDI points to device units.
func (it *interp) devPoints(pts gowemf.Points) []point {
	m := it.matrix()
	out := make([]point, pts.Len())
	for i := range out {
		p := pts.At(i)
		out[i] = m.apply(point{p.X, p.Y})
	}
	return out
}

func (it *interp) rectContour(l, t, r, b float64) []point {
	m := it.matrix()
	return []point{m.apply(point{l, t}), m.apply(point{r, t}), m.apply(point{r, b}), m.apply(point{l, b})}
}

func (it *interp) evenOdd() bool { return it.dc.polyFill != 2 }

// emit draws, or in a path bracket records, the figures of a shape: filled
// with the brush when fill is set, and stroked with the pen when stroke is.
func (it *interp) emit(lines []polyline, fill, stroke bool) error {
	if it.path != nil {
		for _, l := range lines {
			if len(l.pts) == 0 {
				continue
			}
			if err := it.charge(len(l.pts)); err != nil {
				return err
			}
			it.path.figs = append(it.path.figs, l)
		}
		it.path.open = false
		return nil
	}
	if err := it.b.op(); err != nil {
		return err
	}
	if fill {
		if err := it.fillLines(lines); err != nil {
			return err
		}
	}
	if stroke {
		return it.strokeLines(lines)
	}
	return nil
}

func contours(lines []polyline) [][]point {
	out := make([][]point, 0, len(lines))
	for _, l := range lines {
		if len(l.pts) >= 3 {
			out = append(out, l.pts)
		}
	}
	return out
}

// paintColor applies the binary raster operation to a pen or brush color:
// copy draws it, no-op draws nothing, black and white override it, and the
// operations that combine with the destination cannot be drawn on a picture
// with no backdrop.
func (it *interp) paintColor(c rgba) (rgba, bool, error) {
	switch it.dc.rop2 {
	case 13:
		return c, true, nil
	case 11:
		return c, false, nil
	case 1:
		return rgba{0, 0, 0, 255}, true, nil
	case 16:
		return rgba{255, 255, 255, 255}, true, nil
	}
	return c, false, it.soft("binary raster operation %d left out", it.dc.rop2)
}

func (it *interp) fillLines(lines []polyline) error {
	br := it.dc.brush
	if br == nil || br.null {
		return nil
	}
	if br.unsupported != "" {
		return it.soft("%s fill left out", br.unsupported)
	}
	c, err := it.color(br.color)
	if err != nil {
		return err
	}
	c, ok, err := it.paintColor(c)
	if err != nil || !ok {
		return err
	}
	return it.r.fill(contours(lines), it.evenOdd(), fillStyle{solid: c}, it.dc.clip)
}

// penWidth is the pen's width in device units.
func (it *interp) penWidth(p *penObj) (float64, error) {
	if p.cosmetic {
		return 1, nil
	}
	big, small := it.matrix().singular()
	if big > 0 && (big-small)/big > 0.01 {
		if err := it.soft("pen width under a non-uniform transform drawn at the mean scale"); err != nil {
			return 0, err
		}
	}
	w := p.width * math.Sqrt(big*small)
	if w < 1 {
		w = 1
		if p.width > 0 {
			w = math.Max(p.width*math.Sqrt(big*small), 1)
		}
	}
	return w, nil
}

func (it *interp) strokeLines(lines []polyline) error {
	p := it.dc.pen
	if p == nil || p.null {
		return nil
	}
	if p.unsupported != "" {
		if err := it.soft("%s", p.unsupported); err != nil {
			return err
		}
		return nil
	}
	w, err := it.penWidth(p)
	if err != nil {
		return err
	}
	st := strokeStyle{width: w, cap: p.cap, join: p.join, miter: it.dc.miter}
	if !p.cosmetic {
		// Dashes in logical units scale like the width.
		st.dash = nil
	}
	switch {
	case len(p.userDash) > 0:
		k := 1.0
		if !p.cosmetic {
			big, small := it.matrix().singular()
			k = math.Sqrt(big * small)
		}
		for _, d := range p.userDash {
			st.dash = append(st.dash, d*k)
		}
	case p.preset > 0:
		if err = it.soft("preset pen dash pattern drawn from Windows' usual lengths"); err != nil {
			return err
		}
		u := math.Max(w, 1)
		switch p.preset {
		case 1:
			st.dash = []float64{18 * u, 6 * u}
		case 2:
			st.dash = []float64{3 * u, 3 * u}
		case 3:
			st.dash = []float64{9 * u, 6 * u, 3 * u, 6 * u}
		case 4:
			st.dash = []float64{9 * u, 3 * u, 3 * u, 3 * u, 3 * u, 3 * u}
		}
	}
	lines = it.insetFrame(p, lines, w)
	polys, err := strokeLines(lines, st, it.tol, it.charge)
	if err != nil {
		return err
	}
	if len(polys) == 0 {
		return nil
	}
	if len(st.dash) > 0 && p.cosmetic && it.dc.bkMode == 2 {
		// Opaque mode fills the gaps of a dashed cosmetic pen with the
		// background color.
		bk, err := it.color(it.dc.bkColor)
		if err != nil {
			return err
		}
		solid := st
		solid.dash = nil
		gap, err := strokeLines(lines, solid, it.tol, it.charge)
		if err != nil {
			return err
		}
		if err = it.r.fill(gap, false, fillStyle{solid: bk}, it.dc.clip); err != nil {
			return err
		}
	}
	c, err := it.color(p.color)
	if err != nil {
		return err
	}
	c, ok, err := it.paintColor(c)
	if err != nil || !ok {
		return err
	}
	return it.r.fill(polys, false, fillStyle{solid: c}, it.dc.clip)
}

// insetFrame moves the outline of a closed rectangle or ellipse in by half the
// pen's width for an inside-frame pen, where the transform keeps the shape
// upright; others are stroked centred.
func (it *interp) insetFrame(p *penObj, lines []polyline, w float64) []polyline {
	if !p.insideFrame {
		return lines
	}
	m := it.matrix()
	if m.b != 0 || m.c != 0 {
		_ = it.soft("inside-frame pen on a turned shape drawn centred")
		return lines
	}
	out := make([]polyline, 0, len(lines))
	for _, l := range lines {
		if !l.closed || len(l.pts) < 3 {
			out = append(out, l)
			continue
		}
		var minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, q := range l.pts {
			minX, maxX = math.Min(minX, q.x), math.Max(maxX, q.x)
			minY, maxY = math.Min(minY, q.y), math.Max(maxY, q.y)
		}
		cx, cy := (minX+maxX)/2, (minY+maxY)/2
		hw := w / 2
		sx := math.Max(0, (maxX-minX)/2-hw) / math.Max((maxX-minX)/2, 1e-12)
		sy := math.Max(0, (maxY-minY)/2-hw) / math.Max((maxY-minY)/2, 1e-12)
		pts := make([]point, len(l.pts))
		for i, q := range l.pts {
			pts[i] = point{cx + (q.x-cx)*sx, cy + (q.y-cy)*sy}
		}
		out = append(out, polyline{pts: pts, closed: true})
	}
	return out
}

// Shapes.

func (it *interp) rectShape(l, t, r, b float64, round *point) error {
	if l > r {
		l, r = r, l
	}
	if t > b {
		t, b = b, t
	}
	m := it.matrix()
	var pts []point
	if round == nil || round.x <= 0 || round.y <= 0 {
		pts = []point{m.apply(point{l, t}), m.apply(point{r, t}), m.apply(point{r, b}), m.apply(point{l, b})}
	} else {
		rx, ry := math.Min(round.x/2, (r-l)/2), math.Min(round.y/2, (b-t)/2)
		big, _ := m.singular()
		steps := arcSteps(math.Max(rx, ry)*big, math.Pi/2, it.tol)
		if err := it.charge(4 * (steps + 1)); err != nil {
			return err
		}
		corner := func(cx, cy, a0 float64) {
			for i := 0; i <= steps; i++ {
				a := a0 + (math.Pi/2)*float64(i)/float64(steps)
				pts = append(pts, m.apply(point{cx + rx*math.Cos(a), cy + ry*math.Sin(a)}))
			}
		}
		corner(r-rx, t+ry, -math.Pi/2)
		corner(r-rx, b-ry, 0)
		corner(l+rx, b-ry, math.Pi/2)
		corner(l+rx, t+ry, math.Pi)
	}
	return it.emit([]polyline{{pts: pts, closed: true}}, true, true)
}

// ellipsePoints are the points of the ellipse in the logical rectangle, from
// parameter a0 through sweep, in device units.
func (it *interp) ellipsePoints(l, t, r, b, a0, sweep float64, first bool) ([]point, error) {
	m := it.matrix()
	cx, cy := (l+r)/2, (t+b)/2
	rx, ry := math.Abs(r-l)/2, math.Abs(b-t)/2
	big, _ := m.singular()
	n := arcSteps(math.Max(rx, ry)*big, math.Abs(sweep), it.tol)
	if err := it.charge(n + 1); err != nil {
		return nil, err
	}
	var pts []point
	start := 1
	if first {
		start = 0
	}
	for i := start; i <= n; i++ {
		a := a0 + sweep*float64(i)/float64(n)
		pts = append(pts, m.apply(point{cx + rx*math.Cos(a), cy + ry*math.Sin(a)}))
	}
	return pts, nil
}

func (it *interp) ellipseShape(l, t, r, b float64) error {
	pts, err := it.ellipsePoints(l, t, r, b, 0, 2*math.Pi, true)
	if err != nil {
		return err
	}
	return it.emit([]polyline{{pts: pts[:len(pts)-1], closed: true}}, true, true)
}

// arcKind distinguishes the three shapes cut from an ellipse.
type arcKind int

const (
	arcOpen arcKind = iota
	arcChord
	arcPie
)

// arcAngles are the ellipse parameters of the rays from the centre of the
// rectangle through two points, and the sweep between them in the current
// arc direction.
func (it *interp) arcSweep(l, t, r, b float64, start, end point) (a0, sweep float64) {
	cx, cy := (l+r)/2, (t+b)/2
	rx, ry := math.Abs(r-l)/2, math.Abs(b-t)/2
	if rx == 0 || ry == 0 {
		return 0, 0
	}
	a0 = math.Atan2((start.y-cy)/ry, (start.x-cx)/rx)
	a1 := math.Atan2((end.y-cy)/ry, (end.x-cx)/rx)
	// Parameters grow clockwise on a y-down device; the default direction is
	// counterclockwise as drawn.
	m := it.matrix()
	flipped := m.a*m.d-m.b*m.c < 0
	clockwise := it.dc.arcClockwise
	increasing := clockwise != flipped
	if increasing {
		sweep = math.Mod(a1-a0, 2*math.Pi)
		if sweep <= 0 {
			sweep += 2 * math.Pi
		}
	} else {
		sweep = math.Mod(a1-a0, 2*math.Pi)
		if sweep >= 0 {
			sweep -= 2 * math.Pi
		}
	}
	return a0, sweep
}

func (it *interp) arc(kind arcKind, rc gowemf.Rect, start, end gowemf.Point) error {
	l, t, r, b := float64(rc.Left), float64(rc.Top), float64(rc.Right), float64(rc.Bottom)
	a0, sweep := it.arcSweep(l, t, r, b, point{start.X, start.Y}, point{end.X, end.Y})
	if sweep == 0 {
		return nil
	}
	pts, err := it.ellipsePoints(l, t, r, b, a0, sweep, true)
	if err != nil {
		return err
	}
	switch kind {
	case arcOpen:
		return it.emit([]polyline{{pts: pts}}, false, true)
	case arcChord:
		return it.emit([]polyline{{pts: pts, closed: true}}, true, true)
	}
	cx, cy := (l+r)/2, (t+b)/2
	pts = append(pts, it.dev(cx, cy))
	return it.emit([]polyline{{pts: pts, closed: true}}, true, true)
}

// polyBezier flattens cubic Bézier curves from points p[0], then groups of
// three, in device units.
func (it *interp) bezierPoints(from point, ctrl []point) ([]point, error) {
	out := []point{from}
	cur := from
	for i := 0; i+2 < len(ctrl); i += 3 {
		before := len(out)
		out = flattenCubic(out, cur, ctrl[i], ctrl[i+1], ctrl[i+2], it.tol)
		if err := it.charge(len(out) - before); err != nil {
			return nil, err
		}
		cur = ctrl[i+2]
	}
	return out, nil
}

// moveTo and lineTo track the current position.
func (it *interp) moveTo(p point) {
	it.dc.pos = p
	if it.path != nil {
		it.path.move(it.dev(p.x, p.y))
		it.path.start = p
	}
}

func (it *interp) lineTo(p point) error {
	from := it.dc.pos
	it.dc.pos = p
	a, b := it.dev(from.x, from.y), it.dev(p.x, p.y)
	if it.path != nil {
		if !it.path.open {
			it.path.move(a)
			it.path.start = from
		}
		if err := it.charge(1); err != nil {
			return err
		}
		it.path.lineTo(b)
		return nil
	}
	return it.emit([]polyline{{pts: []point{a, b}}}, false, true)
}

// Polygons and polylines.

func (it *interp) polys(p gowemf.Poly, closed, to bool) error {
	n := p.Points.Len()
	pts := it.devPoints(p.Points)
	if err := it.charge(n); err != nil {
		return err
	}
	switch {
	case to:
		if n == 0 {
			return nil
		}
		from := it.dev(it.dc.pos.x, it.dc.pos.y)
		last := p.Points.At(n - 1)
		it.dc.pos = point{last.X, last.Y}
		line := polyline{pts: append([]point{from}, pts...)}
		if it.path != nil && it.path.open {
			for _, q := range pts {
				it.path.lineTo(q)
			}
			return nil
		}
		return it.emit([]polyline{line}, false, true)
	case closed:
		if n < 2 {
			return nil
		}
		return it.emit([]polyline{{pts: pts, closed: true}}, true, true)
	}
	if n < 2 {
		return nil
	}
	return it.emit([]polyline{{pts: pts}}, false, true)
}

func (it *interp) polyPolys(p gowemf.Poly, closed bool) error {
	pts := it.devPoints(p.Points)
	if err := it.charge(len(pts)); err != nil {
		return err
	}
	var lines []polyline
	at := 0
	for i := 0; i < p.Counts.Len(); i++ {
		n := int(p.Counts.At(i))
		if n > len(pts)-at {
			return fmt.Errorf("%w: metafile: polygon point counts", render.ErrInvalid)
		}
		if n >= 2 {
			lines = append(lines, polyline{pts: pts[at : at+n], closed: closed})
		}
		at += n
	}
	if len(lines) == 0 {
		return nil
	}
	if closed {
		return it.emit(lines, true, true)
	}
	return it.emit(lines, false, true)
}

// polyDraw plays EMR_POLYDRAW: points with a type byte each.
func (it *interp) polyDraw(p gowemf.Poly) error {
	n := p.Points.Len()
	if len(p.Types) < n {
		return fmt.Errorf("%w: metafile: polydraw types", render.ErrInvalid)
	}
	if err := it.charge(n); err != nil {
		return err
	}
	m := it.matrix()
	var lines []polyline
	var cur *polyline
	pos := it.dev(it.dc.pos.x, it.dc.pos.y)
	flush := func() {
		if cur != nil && len(cur.pts) > 0 {
			lines = append(lines, *cur)
		}
		cur = nil
	}
	for i := 0; i < n; i++ {
		t := p.Types[i]
		q := p.Points.At(i)
		pt := m.apply(point{q.X, q.Y})
		switch t &^ 1 {
		case 6: // PT_MOVETO
			flush()
			cur = &polyline{pts: []point{pt}}
			pos = pt
		case 2: // PT_LINETO
			if cur == nil {
				cur = &polyline{pts: []point{pos}}
			}
			cur.pts = append(cur.pts, pt)
			pos = pt
		case 4: // PT_BEZIERTO
			if i+2 >= n || p.Types[i+1]&^1 != 4 || p.Types[i+2]&^1 != 4 {
				return fmt.Errorf("%w: metafile: polydraw Bézier points", render.ErrInvalid)
			}
			if cur == nil {
				cur = &polyline{pts: []point{pos}}
			}
			c1 := pt
			q2, q3 := p.Points.At(i+1), p.Points.At(i+2)
			c2, c3 := m.apply(point{q2.X, q2.Y}), m.apply(point{q3.X, q3.Y})
			before := len(cur.pts)
			cur.pts = flattenCubic(cur.pts, cur.pts[len(cur.pts)-1], c1, c2, c3, it.tol)
			if err := it.charge(len(cur.pts) - before); err != nil {
				return err
			}
			pos = c3
			// The type byte of the last point carries the close flag.
			if p.Types[i+2]&1 != 0 {
				cur.closed = true
			}
			i += 2
			t = p.Types[i]
		default:
			return fmt.Errorf("%w: metafile: polydraw point type", render.ErrInvalid)
		}
		if t&1 != 0 && cur != nil {
			cur.closed = true
			flush()
		}
	}
	flush()
	if n > 0 {
		last := p.Points.At(n - 1)
		it.dc.pos = point{last.X, last.Y}
	}
	if len(lines) == 0 {
		return nil
	}
	return it.emit(lines, false, true)
}
