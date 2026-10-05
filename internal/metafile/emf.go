package metafile

import (
	"fmt"
	"math"

	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

// Stock objects, as SelectObject names them with the high bit set.
func stockObject(n uint32) any {
	switch n {
	case 0:
		return &brushObj{color: 0xffffff}
	case 1:
		return &brushObj{color: 0xc0c0c0}
	case 2:
		return &brushObj{color: 0x808080}
	case 3:
		return &brushObj{color: 0x404040}
	case 4:
		return &brushObj{color: 0}
	case 5:
		return &brushObj{null: true}
	case 6:
		return &penObj{cosmetic: true, width: 1, color: 0xffffff}
	case 7:
		return &penObj{cosmetic: true, width: 1, color: 0}
	case 8:
		return &penObj{null: true}
	case 18:
		return &brushObj{color: 0xffffff}
	case 19:
		return &penObj{cosmetic: true, width: 1, color: 0}
	case 10, 11, 12, 13, 14, 16, 17:
		return &gowemf.Font{}
	}
	return nil
}

func (it *interp) newDC() dc {
	return dc{
		mapMode: mmText, wndExt: point{1, 1}, vpExt: point{1, 1}, world: identity,
		pen: stockObject(7).(*penObj), brush: stockObject(0).(*brushObj),
		textColor: 0, bkColor: 0xffffff, bkMode: 2, polyFill: 1, rop2: 13, textAlign: 0,
		miter: 10, clip: it.r.fullClip(),
	}
}

func (it *interp) saveDC() error {
	if len(it.stack) >= 1024 {
		return fmt.Errorf("%w: metafile: saved device contexts", render.ErrLimit)
	}
	it.stack = append(it.stack, it.dc)
	return nil
}

// restoreDC pops to the saved context n names: negative counts back from the
// last, positive is an absolute index from one.
func (it *interp) restoreDC(n int32) error {
	var level int
	if n < 0 {
		level = len(it.stack) + int(n)
	} else {
		level = int(n) - 1
	}
	if level < 0 || level >= len(it.stack) {
		return fmt.Errorf("%w: metafile: restore of an unsaved device context", render.ErrInvalid)
	}
	it.dc = it.stack[level]
	it.stack = it.stack[:level]
	return nil
}

func (it *interp) selectObject(id uint32) error {
	var o any
	if id&0x80000000 != 0 {
		o = stockObject(id & 0x7fffffff)
		if o == nil {
			return nil // palettes and others
		}
	} else {
		o = it.objs[id]
	}
	switch v := o.(type) {
	case *penObj:
		it.dc.pen = v
	case *brushObj:
		it.dc.brush = v
	case *gowemf.Font:
		it.dc.font = v
	}
	return nil
}

func (it *interp) createObject(c gowemf.Command) error {
	if !c.HasObjectID {
		return nil
	}
	var o any
	switch v := c.Body.(type) {
	case gowemf.Pen:
		o = it.pen(c.Source.Type == gowemf.EMRExtCreatePen, v)
	case gowemf.Brush:
		o = it.brush(v)
	case gowemf.PatternBrush, gowemf.PackedPatternBrush, gowemf.BitmapPatternBrush:
		o = &brushObj{unsupported: "pattern brush"}
	case gowemf.Font:
		f := v
		o = &f
	case gowemf.WMFRegion:
		o = v
	default:
		o = struct{}{} // palettes, regions, color spaces
	}
	if len(it.objs) >= 1<<16 {
		return fmt.Errorf("%w: metafile: objects", render.ErrLimit)
	}
	it.objs[c.ObjectID] = o
	return nil
}

func (it *interp) brush(v gowemf.Brush) *brushObj {
	switch v.Style {
	case 0:
		return &brushObj{color: v.Color}
	case 1:
		return &brushObj{null: true}
	case 2:
		return &brushObj{color: v.Color, style: 2, hatch: v.Hatch, unsupported: "hatched brush"}
	}
	return &brushObj{unsupported: "pattern brush"}
}

// pen builds a pen from a LOGPEN or EXTLOGPEN. A pen wider than one device
// unit is geometric, with round caps and joins unless an extended pen says
// otherwise; the rest are cosmetic, one unit wide.
func (it *interp) pen(ext bool, v gowemf.Pen) *penObj {
	p := &penObj{color: v.Color, width: v.Width}
	style := v.Style & 0xf
	geometric := v.Width > 1 || (ext && v.Style&0x10000 != 0)
	switch style {
	case 5:
		p.null = true
		return p
	case 6:
		p.insideFrame = true
	case 1, 2, 3, 4:
		p.preset = int(style)
	case 7:
		for i := 0; i < v.Dashes.Len(); i++ {
			p.userDash = append(p.userDash, float64(v.Dashes.At(i)))
		}
		if len(p.userDash) == 0 {
			p.unsupported = "pen with a user style but no dashes left out"
		}
	}
	if ext && v.BrushStyle != 0 {
		p.unsupported = "pen painted with a brush left out"
	}
	if !geometric {
		p.cosmetic, p.width, p.cap, p.join = true, 1, capFlat, joinRound
		return p
	}
	if ext {
		switch v.Style & 0xf00 {
		case 0x100:
			p.cap = capSquare
		case 0x200:
			p.cap = capFlat
		}
		switch v.Style & 0xf000 {
		case 0x1000:
			p.join = joinBevel
		case 0x2000:
			p.join = joinMiter
		}
	}
	return p
}

// command plays one decoded record.
func (it *interp) command(c gowemf.Command) error {
	if err := it.ctx.Err(); err != nil {
		return err
	}
	if c.HasObjectID {
		if err := it.createObject(c); err != nil {
			return err
		}
		return nil
	}
	switch c.Source.Format {
	case gowemf.EMF:
		return it.emfRecord(c)
	case gowemf.WMF:
		return it.wmfRecord(c)
	}
	return nil
}

func rectOf(r gowemf.Rect) (l, t, rr, b float64) {
	return float64(r.Left), float64(r.Top), float64(r.Right), float64(r.Bottom)
}

func (it *interp) emfRecord(c gowemf.Command) error {
	t := c.Source.Type
	switch v := c.Body.(type) {
	case gowemf.Poly:
		return it.emfPoly(t, v)
	case gowemf.Value:
		return it.emfValue(t, v.Value)
	case gowemf.PointRecord:
		p := point{v.Point.X, v.Point.Y}
		switch t {
		case gowemf.EMRSetWindowExtEx:
			if !it.fixedMode() {
				it.dc.wndExt = p
				it.fixIsotropic()
			}
		case gowemf.EMRSetViewportExtEx:
			if !it.fixedMode() {
				it.dc.vpExt = p
				it.fixIsotropic()
			}
		case gowemf.EMRSetWindowOrgEx:
			it.dc.wndOrg = p
		case gowemf.EMRSetViewportOrgEx:
			it.dc.vpOrg = p
		case gowemf.EMRSetBrushOrgEx:
		case gowemf.EMRMoveToEx:
			it.moveTo(p)
		case gowemf.EMRLineTo:
			return it.lineTo(p)
		case gowemf.EMROffsetClipRgn:
			return it.offsetClip(p)
		}
		return nil
	case gowemf.RectRecord:
		l, tp, r, b := rectOf(v.Rect)
		switch t {
		case gowemf.EMRIntersectClipRect:
			return it.clipRect(l, tp, r, b, clipAnd)
		case gowemf.EMRExcludeClipRect:
			return it.clipRect(l, tp, r, b, clipDiff)
		case gowemf.EMREllipse:
			return it.ellipseShape(l, tp, r, b)
		case gowemf.EMRRectangle:
			return it.rectShape(l, tp, r, b, nil)
		case gowemf.EMRFillPath, gowemf.EMRStrokeAndFillPath, gowemf.EMRStrokePath:
			return it.finishPath(t)
		}
		return nil
	case gowemf.RoundRect:
		l, tp, r, b := rectOf(v.Rect)
		return it.rectShape(l, tp, r, b, &point{v.Corner.X, v.Corner.Y})
	case gowemf.Arc:
		switch t {
		case gowemf.EMRArc:
			return it.arc(arcOpen, v.Rect, v.Start, v.End)
		case gowemf.EMRChord:
			return it.arc(arcChord, v.Rect, v.Start, v.End)
		case gowemf.EMRPie:
			return it.arc(arcPie, v.Rect, v.Start, v.End)
		case gowemf.EMRArcTo:
			return it.arcTo(v)
		}
	case gowemf.AngleArc:
		return it.angleArc(v)
	case gowemf.Scale:
		if it.fixedMode() {
			return nil
		}
		d := &it.dc
		if t == gowemf.EMRScaleWindowExtEx {
			d.wndExt = point{d.wndExt.x * float64(v.XNum) / float64(v.XDenom), d.wndExt.y * float64(v.YNum) / float64(v.YDenom)}
		} else {
			d.vpExt = point{d.vpExt.x * float64(v.XNum) / float64(v.XDenom), d.vpExt.y * float64(v.YNum) / float64(v.YDenom)}
		}
		it.fixIsotropic()
		return nil
	case gowemf.Transform:
		return it.worldTransform(t, v)
	case gowemf.SignedValue:
		if t == gowemf.EMRRestoreDC {
			return it.restoreDC(v.Value)
		}
	case gowemf.FloatValue:
		if t == gowemf.EMRSetMiterLimit {
			it.dc.miter = v.Value
		}
	case gowemf.Pixel:
		return it.setPixel(v)
	case gowemf.Empty:
		return it.emfEmpty(t)
	case gowemf.Region:
		if t == gowemf.EMRExtSelectClipRgn {
			return it.selectClipRegion(v)
		}
	case gowemf.EMFRegionPaint:
		return it.regionPaint(t, v)
	case gowemf.Text:
		return it.text(v)
	case gowemf.PolyText:
		for _, s := range v.Strings {
			if err := it.text(s); err != nil {
				return err
			}
		}
		return nil
	case gowemf.TextJustification:
		it.dc.justExtra, it.dc.justCount = v.Extra, v.Count
		return nil
	case gowemf.BitmapTransfer:
		return it.emfBitmap(t, v)
	case gowemf.RasterTransfer:
		return it.emfRaster(t, v)
	case gowemf.MaskedBitmapTransfer:
		return it.soft("masked or parallelogram bitmap transfer left out")
	case gowemf.Gradient:
		return it.gradient(v)
	case gowemf.FloodFill:
		return it.soft("flood fill left out")
	case gowemf.Comment, gowemf.ColorAdjustment, gowemf.ColorProfile, gowemf.Palette, gowemf.ColorSpaceObject:
		return nil
	}
	return it.soft("record type %d left out", t)
}

func (it *interp) emfEmpty(t uint32) error {
	switch t {
	case gowemf.EMRSaveDC:
		return it.saveDC()
	case gowemf.EMRBeginPath:
		it.path = &pathBuilder{}
		return nil
	case gowemf.EMREndPath:
		if it.path != nil {
			it.done, it.path = it.path, nil
		}
		return nil
	case gowemf.EMRCloseFigure:
		if it.path != nil && it.path.open && len(it.path.figs) > 0 {
			it.path.figs[len(it.path.figs)-1].closed = true
			it.path.open = false
			it.dc.pos = it.path.start
		}
		return nil
	case gowemf.EMRAbortPath:
		it.path, it.done = nil, nil
		return nil
	case gowemf.EMRFlattenPath:
		return nil
	case gowemf.EMRWidenPath:
		it.done = nil
		return it.soft("widened path left out")
	case gowemf.EMRSetMetaRgn:
		return it.soft("meta region left out")
	}
	return nil
}

func (it *interp) emfValue(t, v uint32) error {
	d := &it.dc
	switch t {
	case gowemf.EMRSetMapMode:
		return it.setMapMode(int(v))
	case gowemf.EMRSetBkMode:
		d.bkMode = v
	case gowemf.EMRSetPolyFillMode:
		d.polyFill = v
	case gowemf.EMRSetROP2:
		d.rop2 = v
	case gowemf.EMRSetStretchBltMode:
		d.stretchMode = v
	case gowemf.EMRSetTextAlign:
		d.textAlign = v
	case gowemf.EMRSetTextColor:
		d.textColor = v
	case gowemf.EMRSetBkColor:
		d.bkColor = v
	case gowemf.EMRSelectObject:
		return it.selectObject(v)
	case gowemf.EMRDeleteObject:
		delete(it.objs, v)
	case gowemf.EMRSetArcDirection:
		d.arcClockwise = v == 2
	case gowemf.EMRSelectClipPath:
		return it.clipPath(v)
	case gowemf.EMRSetLayout:
		if v&1 != 0 {
			return it.soft("right-to-left layout drawn left to right")
		}
	case gowemf.EMRSetICMMode, gowemf.EMRSetMapperFlags, gowemf.EMRSelectPalette,
		gowemf.EMRSetColorSpace, gowemf.EMRDeleteColorSpace:
	default:
		return it.soft("record type %d left out", t)
	}
	return nil
}

func (it *interp) worldTransform(t uint32, v gowemf.Transform) error {
	m := fromMatrix(v.Matrix)
	d := &it.dc
	switch {
	case t == gowemf.EMRSetWorldTransform:
		d.world = m
	case v.Mode == 1:
		d.world = identity
	case v.Mode == 2:
		d.world = m.then(d.world)
	case v.Mode == 3:
		d.world = d.world.then(m)
	case v.Mode == 4:
		d.world = m
	}
	if !d.world.finite() {
		return fmt.Errorf("%w: metafile: world transform", render.ErrInvalid)
	}
	return nil
}

func (it *interp) emfPoly(t uint32, p gowemf.Poly) error {
	switch t {
	case gowemf.EMRPolyBezier, gowemf.EMRPolyBezier16, gowemf.EMRPolyBezierTo, gowemf.EMRPolyBezierTo16:
		to := t == gowemf.EMRPolyBezierTo || t == gowemf.EMRPolyBezierTo16
		pts := it.devPoints(p.Points)
		var from point
		ctrl := pts
		if to {
			from = it.dev(it.dc.pos.x, it.dc.pos.y)
		} else {
			if len(pts) < 4 {
				return nil
			}
			from, ctrl = pts[0], pts[1:]
		}
		line, err := it.bezierPoints(from, ctrl)
		if err != nil {
			return err
		}
		if to {
			last := p.Points.At(p.Points.Len() - 1)
			it.dc.pos = point{last.X, last.Y}
			if it.path != nil && it.path.open {
				for _, q := range line[1:] {
					it.path.lineTo(q)
				}
				return nil
			}
		}
		return it.emit([]polyline{{pts: line}}, false, true)
	case gowemf.EMRPolygon, gowemf.EMRPolygon16:
		return it.polys(p, true, false)
	case gowemf.EMRPolyline, gowemf.EMRPolyline16:
		return it.polys(p, false, false)
	case gowemf.EMRPolylineTo, gowemf.EMRPolylineTo16:
		return it.polys(p, false, true)
	case gowemf.EMRPolyPolyline, gowemf.EMRPolyPolyline16:
		return it.polyPolys(p, false)
	case gowemf.EMRPolyPolygon, gowemf.EMRPolyPolygon16:
		return it.polyPolys(p, true)
	case gowemf.EMRPolyDraw, gowemf.EMRPolyDraw16:
		return it.polyDraw(p)
	}
	return it.soft("record type %d left out", t)
}

func (it *interp) arcTo(v gowemf.Arc) error {
	l, t, r, b := rectOf(v.Rect)
	a0, sweep := it.arcSweep(l, t, r, b, point{v.Start.X, v.Start.Y}, point{v.End.X, v.End.Y})
	if sweep == 0 {
		return nil
	}
	pts, err := it.ellipsePoints(l, t, r, b, a0, sweep, true)
	if err != nil {
		return err
	}
	cx, cy := (l+r)/2, (t+b)/2
	end := a0 + sweep
	endL := point{cx + math.Abs(r-l)/2*math.Cos(end), cy + math.Abs(b-t)/2*math.Sin(end)}
	from := it.dev(it.dc.pos.x, it.dc.pos.y)
	it.dc.pos = endL
	if it.path != nil {
		if !it.path.open {
			it.path.move(from)
		}
		for _, q := range pts {
			it.path.lineTo(q)
		}
		return nil
	}
	return it.emit([]polyline{{pts: append([]point{from}, pts...)}}, false, true)
}

func (it *interp) angleArc(v gowemf.AngleArc) error {
	cx, cy, rad := v.Center.X, v.Center.Y, float64(v.Radius)
	s := v.StartAngle * math.Pi / 180
	sw := v.SweepAngle * math.Pi / 180
	// Angles run counterclockwise from the x-axis; the device's y runs down.
	pts, err := it.ellipsePoints(cx-rad, cy-rad, cx+rad, cy+rad, -s, -sw, true)
	if err != nil {
		return err
	}
	from := it.dev(it.dc.pos.x, it.dc.pos.y)
	e := -(s + sw)
	it.dc.pos = point{cx + rad*math.Cos(e), cy + rad*math.Sin(e)}
	if it.path != nil {
		if !it.path.open {
			it.path.move(from)
		}
		for _, q := range pts {
			it.path.lineTo(q)
		}
		return nil
	}
	return it.emit([]polyline{{pts: append([]point{from}, pts...)}}, false, true)
}

// Paths.

func (it *interp) finishPath(t uint32) error {
	p := it.done
	it.done = nil
	if p == nil {
		return nil
	}
	var lines []polyline
	for _, l := range p.figs {
		if len(l.pts) >= 2 {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	if err := it.b.op(); err != nil {
		return err
	}
	if t == gowemf.EMRFillPath || t == gowemf.EMRStrokeAndFillPath {
		if err := it.fillLines(lines); err != nil {
			return err
		}
	}
	if t == gowemf.EMRStrokePath || t == gowemf.EMRStrokeAndFillPath {
		return it.strokeLines(lines)
	}
	return nil
}

func (it *interp) clipPath(mode uint32) error {
	p := it.done
	it.done = nil
	if p == nil {
		return nil
	}
	if mode < 1 || mode > 5 {
		return fmt.Errorf("%w: metafile: clip mode", render.ErrInvalid)
	}
	var cs [][]point
	for _, l := range p.figs {
		if len(l.pts) >= 3 {
			cs = append(cs, l.pts)
		}
	}
	return it.combineClip(cs, it.evenOdd(), clipMode(mode))
}

func (it *interp) combineClip(cs [][]point, evenOdd bool, mode clipMode) error {
	if err := it.b.op(); err != nil {
		return err
	}
	if len(cs) == 0 {
		// An empty region: and and copy leave nothing, the rest leave the clip.
		switch mode {
		case clipAnd, clipCopy:
			n := *it.dc.clip
			n.x1, n.y1 = n.x0, n.y0
			it.dc.clip = &n
		}
		return nil
	}
	n, err := it.r.combine(it.dc.clip, cs, evenOdd, mode)
	if err != nil {
		return err
	}
	it.dc.clip = n
	return nil
}

func (it *interp) clipRect(l, t, r, b float64, mode clipMode) error {
	if l > r {
		l, r = r, l
	}
	if t > b {
		t, b = b, t
	}
	return it.combineClip([][]point{it.rectContour(l, t, r, b)}, false, mode)
}

func (it *interp) offsetClip(p point) error {
	c := it.dc.clip
	if c.x0 <= 0 && c.y0 <= 0 && c.x1 >= float64(it.r.w) && c.y1 >= float64(it.r.h) && c.m == nil {
		return nil
	}
	return it.soft("clip region offset left out")
}

// selectClipRegion plays EMR_EXTSELECTCLIPRGN, whose rectangles are in device
// units.
func (it *interp) selectClipRegion(v gowemf.Region) error {
	if v.Count == 0 {
		if v.Mode == 5 {
			it.dc.clip = it.r.fullClip()
			return nil
		}
		return it.soft("empty clip region combination left out")
	}
	cs, err := it.regionContours(v)
	if err != nil {
		return err
	}
	return it.combineClip(cs, false, clipMode(v.Mode))
}

func (it *interp) regionContours(v gowemf.Region) ([][]point, error) {
	if err := it.charge(4 * int(v.Count)); err != nil {
		return nil, err
	}
	var cs [][]point
	for i := 0; i < int(v.Count); i++ {
		r := v.RectangleAt(i)
		if r.Right <= r.Left || r.Bottom <= r.Top {
			continue
		}
		l, t, rr, b := float64(r.Left), float64(r.Top), float64(r.Right), float64(r.Bottom)
		cs = append(cs, []point{{l, t}, {rr, t}, {rr, b}, {l, b}})
	}
	return cs, nil
}

func (it *interp) regionPaint(t uint32, v gowemf.EMFRegionPaint) error {
	switch t {
	case gowemf.EMRFillRgn, gowemf.EMRPaintRgn:
	default:
		return it.soft("region frame or invert left out")
	}
	br := it.dc.brush
	if t == gowemf.EMRFillRgn {
		id := v.Brush
		var o any
		if id&0x80000000 != 0 {
			o = stockObject(id & 0x7fffffff)
		} else {
			o = it.objs[id]
		}
		bo, ok := o.(*brushObj)
		if !ok {
			return nil
		}
		br = bo
	}
	if br == nil || br.null {
		return nil
	}
	if br.unsupported != "" {
		return it.soft("%s fill left out", br.unsupported)
	}
	cs, err := it.regionContours(v.Region)
	if err != nil {
		return err
	}
	if err = it.b.op(); err != nil {
		return err
	}
	col, err := it.color(br.color)
	if err != nil {
		return err
	}
	return it.r.fill(cs, false, fillStyle{solid: col}, it.dc.clip)
}

func (it *interp) setPixel(v gowemf.Pixel) error {
	col, err := it.color(v.Color)
	if err != nil {
		return err
	}
	if err = it.b.op(); err != nil {
		return err
	}
	// A pixel is one device unit square.
	p := it.dev(v.Point.X, v.Point.Y)
	cs := [][]point{{{p.x, p.y}, {p.x + 1, p.y}, {p.x + 1, p.y + 1}, {p.x, p.y + 1}}}
	return it.r.fill(cs, false, fillStyle{solid: col}, it.dc.clip)
}
