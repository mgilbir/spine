package metafile

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

// realized is a logical font realized in a face: the face, and the scale from
// its font units to text-space units.
type realized struct {
	face *shape.Face
	k    float64 // text-space units per font unit along the em
	kx   float64 // the same across the baseline's direction of advance
	// Ascent and descent in text-space units, both positive.
	ascent, descent float64
}

// fontRequest is the face a logical font asks the resolver for. GDI realizes
// weight 600 and up from a bold face when there is no other between regular
// and bold, as the nearer of the two.
func fontRequest(f gowemf.FontRequest) render.FontRequest {
	return render.FontRequest{Family: f.FaceName, Bold: f.Weight >= 600, Italic: f.Italic}
}

// face resolves a request once. A face the resolver cannot supply is
// reported, then left out; a limit or an invalid font stops the drawing.
func (be *backend) face(req render.FontRequest) (*shape.Face, error) {
	if f, ok := be.faces[req]; ok {
		return f, nil
	}
	if be.noFace[req] {
		return nil, nil
	}
	if be.opts.Fonts == nil {
		return nil, be.soft("text left out: no font resolver")
	}
	if len(be.faces)+len(be.noFace) >= be.maxFonts {
		return nil, fmt.Errorf("%w: metafile font requests", render.ErrLimit)
	}
	f, err := be.opts.Fonts(be.ctx, req)
	if cerr := be.ctx.Err(); cerr != nil {
		return nil, cerr
	}
	if err == nil && (f == nil || f.UnitsPerEm() <= 0) {
		err = errors.New("no font")
	}
	if err != nil {
		if errors.Is(err, render.ErrLimit) || errors.Is(err, render.ErrInvalid) {
			return nil, err
		}
		if be.noFace == nil {
			be.noFace = map[render.FontRequest]bool{}
		}
		be.noFace[req] = true
		return nil, be.soft("text in %q left out: %v", req.Family, err)
	}
	if be.faces == nil {
		be.faces = map[render.FontRequest]*shape.Face{}
	}
	be.faces[req] = f
	return f, nil
}

// realize sizes a logical font's face. It returns nil, after reporting why,
// when the text cannot be drawn.
func (be *backend) realize(f gowemf.FontRequest) (*realized, error) {
	if f.FaceName == "" {
		// Stock and unnamed fonts are realized by the system in faces this
		// renderer does not know.
		return nil, be.soft("text in a font without a face name left out")
	}
	if f.Height == 0 || !finite(f.Height) || !finite(f.Width) {
		return nil, be.soft("text in a font of default height left out")
	}
	face, err := be.face(fontRequest(f))
	if err != nil || face == nil {
		return nil, err
	}
	upem := float64(face.UnitsPerEm())
	desc := face.Descriptor()
	// GDI's tmAscent and tmDescent are the face's Windows metrics. Windows
	// will not load a TrueType face without them; for such a face the
	// ascent and descent it does state are all there is.
	asc, dsc := float64(desc.Ascent), float64(-desc.Descent)
	if desc.Has(shape.MetricWinMetrics) && desc.WinAscent+desc.WinDescent > 0 {
		asc, dsc = float64(desc.WinAscent), float64(desc.WinDescent)
	}
	em := math.Abs(f.Height)
	if f.Height > 0 {
		// A positive height is the cell height: the ascent and descent.
		if asc+dsc <= 0 {
			return nil, be.soft("text sized by a cell height without font metrics left out")
		}
		em = f.Height * upem / (asc + dsc)
	}
	k := em / upem
	rf := &realized{face: face, k: k, kx: k, ascent: asc * k, descent: dsc * k}
	if f.Width != 0 {
		// A width sets the average character width, which the face would
		// have to state for the glyphs to be stretched to it.
		if err = be.soft("text with a set character width drawn at the font's own width"); err != nil {
			return nil, err
		}
	}
	return rf, nil
}

// ownFace reports whether a face is the font a logical font names, by its
// family or its PostScript name. Glyph indexes are the font's own: in another
// face they are other glyphs.
func ownFace(face *shape.Face, f gowemf.FontRequest) bool {
	name := strings.TrimSpace(f.FaceName)
	return strings.EqualFold(strings.TrimSpace(face.Family()), name) || strings.EqualFold(strings.TrimSpace(face.Name()), name)
}

// elements returns the glyph of each element of a run, and false for the
// second unit of a surrogate pair. Characters the face lacks are -1: they are
// left out, without an advance.
func elements(run gowemf.TextRun, face *shape.Face) (gids []int, first []bool, missing bool) {
	n := len(run.Text)
	gids, first = make([]int, n), make([]bool, n)
	if run.Glyphs {
		for i, g := range run.Text {
			gids[i], first[i] = int(g), true
			if int(g) >= face.NumGlyphs() {
				gids[i], missing = -1, true
			}
		}
		return
	}
	for i := 0; i < n; i++ {
		u := run.Text[i]
		r := rune(u)
		first[i] = true
		if utf16.IsSurrogate(r) {
			if i+1 < n && u < 0xdc00 && run.Text[i+1] >= 0xdc00 && run.Text[i+1] < 0xe000 {
				r = utf16.DecodeRune(r, rune(run.Text[i+1]))
			} else {
				r = 0xfffd
			}
		}
		g, ok := face.GlyphID(r)
		if !ok {
			g = -1
			if r > ' ' {
				missing = true
			}
		}
		gids[i] = g
		if r > 0xffff {
			// The second unit belongs to the same character.
			i++
			gids[i], first[i] = g, false
		}
	}
	return
}

// MeasureText reports a run's ascent, descent and advances in text-space
// units. A run whose font cannot be realized measures as nothing and is left
// out when drawn.
func (be *backend) MeasureText(run gowemf.TextRun) (gowemf.TextMetrics, error) {
	if err := be.ctx.Err(); err != nil {
		return gowemf.TextMetrics{}, err
	}
	m := gowemf.TextMetrics{Advances: make([]float64, len(run.Text))}
	rf, err := be.realize(run.Font)
	if err != nil || rf == nil {
		return m, err
	}
	if run.Glyphs && !ownFace(rf.face, run.Font) {
		return m, nil
	}
	gids, first, _ := elements(run, rf.face)
	em := rf.k * float64(rf.face.UnitsPerEm())
	for i, g := range gids {
		if first[i] && g >= 0 {
			// Advances are in thousandths of the em.
			m.Advances[i] = rf.face.GlyphAdvance(g) / 1000 * em * rf.kx / rf.k
		}
	}
	m.Ascent, m.Descent = rf.ascent, rf.descent
	return m, nil
}

// DrawText draws a run's glyphs at their origins, and its underline and
// strikeout.
func (be *backend) DrawText(run gowemf.TextRun, cl gowemf.Clip) error {
	if err := be.begin(); err != nil {
		return err
	}
	rf, err := be.realize(run.Font)
	if err != nil || rf == nil {
		return err
	}
	if run.Glyphs && !ownFace(rf.face, run.Font) {
		return be.soft("text of glyph indexes in a substitute font left out")
	}
	if len(run.Origins) != len(run.Text) {
		return fmt.Errorf("%w: metafile: text origins", render.ErrInvalid)
	}
	if len(run.Text) > be.b.maxGlyphs-be.b.glyphs {
		return fmt.Errorf("%w: metafile glyphs", render.ErrLimit)
	}
	be.b.glyphs += len(run.Text)
	st, ok, err := be.fillStyle(run.Paint)
	if err != nil || !ok {
		return err
	}
	c, err := be.clip(cl)
	if err != nil {
		return err
	}
	toDest := fromMatrix(run.Transform)
	if !toDest.finite() {
		return errCoordinate
	}
	gids, first, missing := elements(run, rf.face)
	if missing {
		if err = be.soft("characters the font lacks left out"); err != nil {
			return err
		}
	}
	// Glyph space is the font's, y up; text space has y down from the
	// baseline. Orientation turns each glyph about its origin, counterclockwise
	// as displayed.
	sin, cos := math.Sincos(run.Font.Orientation)
	var contours [][]point
	for i, g := range gids {
		if !first[i] || g < 0 {
			continue
		}
		if err = be.ctx.Err(); err != nil {
			return err
		}
		o := run.Origins[i]
		at := func(p shape.Point) point {
			x, y := p.X*rf.kx, -p.Y*rf.k
			x, y = x*cos+y*sin, -x*sin+y*cos
			return toDest.apply(point{o.X + x, o.Y + y})
		}
		glyph, err := be.outline(rf.face, g, at)
		if err != nil {
			return err
		}
		contours = append(contours, glyph...)
	}
	if len(contours) > 0 {
		if err = be.r.fill(contours, false, st, c); err != nil {
			return err
		}
	}
	return be.decorate(run, rf, toDest, st, c)
}

// outline flattens a glyph's outline, placed by at.
func (be *backend) outline(face *shape.Face, gid int, at func(shape.Point) point) ([][]point, error) {
	var out [][]point
	var cur []point
	var last shape.Point
	var cbErr error
	flush := func() {
		if len(cur) >= 3 {
			out = append(out, cur)
		}
		cur = nil
	}
	err := face.GlyphOutline(gid, func(s shape.Segment) bool {
		switch s.Op {
		case shape.MoveTo:
			flush()
			cur = []point{at(s.Pts[0])}
			last = s.Pts[0]
		case shape.LineTo:
			cbErr = be.charge(1)
			cur = append(cur, at(s.Pts[0]))
			last = s.Pts[0]
		case shape.QuadTo:
			c1 := shape.Point{X: last.X + 2.0/3*(s.Pts[0].X-last.X), Y: last.Y + 2.0/3*(s.Pts[0].Y-last.Y)}
			c2 := shape.Point{X: s.Pts[1].X + 2.0/3*(s.Pts[0].X-s.Pts[1].X), Y: s.Pts[1].Y + 2.0/3*(s.Pts[0].Y-s.Pts[1].Y)}
			before := len(cur)
			cur = flattenCubic(cur, at(last), at(c1), at(c2), at(s.Pts[1]), be.tol)
			cbErr = be.charge(len(cur) - before)
			last = s.Pts[1]
		case shape.CubicTo:
			before := len(cur)
			cur = flattenCubic(cur, at(last), at(s.Pts[0]), at(s.Pts[1]), at(s.Pts[2]), be.tol)
			cbErr = be.charge(len(cur) - before)
			last = s.Pts[2]
		}
		return cbErr == nil
	})
	flush()
	if cbErr != nil {
		return nil, cbErr
	}
	if err != nil {
		return nil, be.soft("glyph outline left out: %v", err)
	}
	for _, c := range out {
		for _, q := range c {
			if !finite(q.x) || !finite(q.y) {
				return nil, errCoordinate
			}
		}
	}
	return out, nil
}

// decorate draws a run's underline and strikeout from its first origin over
// the sum of its advances, at the positions and thicknesses the font states.
func (be *backend) decorate(run gowemf.TextRun, rf *realized, toDest affine, st fillStyle, c *clip) error {
	if (!run.Font.Underline && !run.Font.StrikeOut) || len(run.Origins) == 0 {
		return nil
	}
	var width float64
	for _, a := range run.Advances {
		width += a
	}
	desc := rf.face.Descriptor()
	em := rf.k * float64(rf.face.UnitsPerEm())
	o := run.Origins[0]
	// A rule from top to bottom, in text space below the baseline at the
	// first origin.
	rule := func(top, bottom float64) []point {
		return []point{
			toDest.apply(point{o.X, o.Y + top}), toDest.apply(point{o.X + width, o.Y + top}),
			toDest.apply(point{o.X + width, o.Y + bottom}), toDest.apply(point{o.X, o.Y + bottom}),
		}
	}
	var contours [][]point
	if run.Font.Underline {
		pos, thick := -float64(desc.UnderlinePosition)*rf.k, float64(desc.UnderlineThickness)*rf.k
		if !desc.Has(shape.MetricUnderline) || thick <= 0 {
			if err := be.soft("text underline drawn without the font's underline metrics"); err != nil {
				return err
			}
			pos, thick = em/8, em/14
		}
		contours = append(contours, rule(pos, pos+thick))
	}
	if run.Font.StrikeOut {
		pos, size := -float64(desc.StrikeoutPosition)*rf.k, float64(desc.StrikeoutSize)*rf.k
		if !desc.Has(shape.MetricStrikeout) || size <= 0 {
			if err := be.soft("text strikeout drawn without the font's strikeout metrics"); err != nil {
				return err
			}
			pos, size = -em*0.3, em/14
		}
		// The strikeout position is the top of its stroke above the baseline.
		contours = append(contours, rule(pos, pos+size))
	}
	return be.r.fill(contours, false, st, c)
}
