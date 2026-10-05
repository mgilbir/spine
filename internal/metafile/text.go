package metafile

import (
	"errors"
	"fmt"
	"math"
	"unicode/utf16"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

// Text alignment flags.
const (
	taUpdateCP = 1
	taRight    = 2
	taCenter   = 6
	taBottom   = 8
	taBaseline = 24
)

// cp1252 maps the bytes 0x80 to 0x9f of Windows-1252; zero is undefined.
var cp1252 = [32]rune{
	0x20ac, 0, 0x201a, 0x0192, 0x201e, 0x2026, 0x2020, 0x2021, 0x02c6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017d, 0,
	0, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014, 0x02dc, 0x2122, 0x0161, 0x203a, 0x0153, 0, 0x017e, 0x0178,
}

// face resolves the font a text uses, once for each request.
func (it *interp) face(req render.FontRequest) (*shape.Face, error) {
	if f, ok := it.faces[req]; ok {
		return f, nil
	}
	if len(it.faces) >= it.maxFonts {
		return nil, fmt.Errorf("%w: metafile font requests", render.ErrLimit)
	}
	f, err := it.opts.Fonts(it.ctx, req)
	if cerr := it.ctx.Err(); cerr != nil {
		return nil, cerr
	}
	if err != nil {
		if errors.Is(err, render.ErrLimit) || errors.Is(err, render.ErrInvalid) {
			return nil, err
		}
		return nil, it.soft("text in %q left out: %v", req.Family, err)
	}
	if f == nil || f.UnitsPerEm() <= 0 {
		return nil, it.soft("text in %q left out: no font", req.Family)
	}
	if it.faces == nil {
		it.faces = map[render.FontRequest]*shape.Face{}
	}
	it.faces[req] = f
	return f, nil
}

func faceName(f *gowemf.Font) string {
	b := f.FaceName
	if f.Unicode {
		var u []uint16
		for i := 0; i+1 < len(b); i += 2 {
			c := uint16(b[i]) | uint16(b[i+1])<<8
			if c == 0 {
				break
			}
			u = append(u, c)
		}
		return string(utf16.Decode(u))
	}
	for i, c := range b {
		if c == 0 {
			b = b[:i]
			break
		}
	}
	return string(b)
}

// runes decodes the characters of a text. ok is false when it cannot, after
// reporting why.
func (it *interp) runes(t gowemf.Text, f *gowemf.Font) ([]rune, bool, error) {
	if t.Options&0x10 != 0 {
		return nil, false, it.soft("text of glyph indexes left out")
	}
	var out []rune
	switch {
	case t.SmallChars:
		for _, c := range t.Bytes {
			out = append(out, rune(c))
		}
	case t.Unicode:
		u := make([]uint16, 0, len(t.Bytes)/2)
		for i := 0; i+1 < len(t.Bytes); i += 2 {
			u = append(u, uint16(t.Bytes[i])|uint16(t.Bytes[i+1])<<8)
		}
		out = make([]rune, 0, len(u))
		for _, c := range u {
			if c >= 0xd800 && c < 0xe000 {
				return nil, false, it.soft("text with surrogate pairs left out")
			}
			out = append(out, rune(c))
		}
	default:
		for _, c := range t.Bytes {
			switch {
			case c < 0x80:
				out = append(out, rune(c))
			case f.CharSet <= 1 && c < 0xa0 && cp1252[c-0x80] != 0:
				out = append(out, cp1252[c-0x80])
			case f.CharSet <= 1 && c >= 0xa0:
				out = append(out, rune(c))
			default:
				return nil, false, it.soft("text in a character set other than Windows-1252 left out")
			}
		}
	}
	return out, true, nil
}

// text draws the characters of a text record.
func (it *interp) text(t gowemf.Text) error {
	if len(t.Bytes) == 0 && (t.Options&2 == 0 || !t.HasRectangle) {
		return nil
	}
	if it.path != nil {
		return it.soft("text in a path left out")
	}
	d := &it.dc
	if it.opts.Fonts == nil {
		return it.soft("text left out: no font resolver")
	}
	f := d.font
	if f == nil || len(f.FaceName) == 0 || faceName(f) == "" {
		return it.soft("text in a font without a face name left out")
	}
	if err := it.b.op(); err != nil {
		return err
	}
	runes, ok, err := it.runes(t, f)
	if err != nil || !ok {
		return err
	}
	if len(runes) > it.b.maxGlyphs-it.b.glyphs {
		return fmt.Errorf("%w: metafile glyphs", render.ErrLimit)
	}
	it.b.glyphs += len(runes)
	// A background rectangle or clip comes first.
	if t.Options&2 != 0 && t.HasRectangle {
		bk, err := it.color(d.bkColor)
		if err != nil {
			return err
		}
		l, tp, r, b := rectOf(t.Rectangle)
		if err = it.r.fill([][]point{it.rectContour(l, tp, r, b)}, false, fillStyle{solid: bk}, d.clip); err != nil {
			return err
		}
	}
	if len(runes) == 0 {
		return nil
	}
	saved := d.clip
	if t.Options&4 != 0 && t.HasRectangle {
		l, tp, r, b := rectOf(t.Rectangle)
		if err = it.clipRect(l, tp, r, b, clipAnd); err != nil {
			return err
		}
		defer func() { it.dc.clip = saved }()
	}
	req := render.FontRequest{Family: faceName(f), Bold: f.Weight >= 700, Italic: f.Italic != 0}
	face, err := it.face(req)
	if err != nil || face == nil {
		return err
	}
	return it.drawRunes(t, f, face, runes)
}

func (it *interp) drawRunes(t gowemf.Text, f *gowemf.Font, face *shape.Face, runes []rune) error {
	d := &it.dc
	upem := float64(face.UnitsPerEm())
	desc := face.Descriptor()
	em := math.Abs(float64(f.Height))
	if f.Height == 0 {
		return it.soft("text in a font of default height left out")
	}
	if f.Height > 0 {
		cell := float64(desc.Ascent - desc.Descent)
		if cell <= 0 {
			return it.soft("text sized by a cell height without font metrics left out")
		}
		if err := it.soft("text sized by its cell height from the font's metrics"); err != nil {
			return err
		}
		em = em * upem / cell
	}
	k := em / upem // logical units per font unit
	m := it.matrix()
	big, small := m.singular()
	if big > 0 && (big-small)/big > 0.01 {
		if err := it.soft("text under a non-uniform transform stretched with it"); err != nil {
			return err
		}
	}
	if f.Escapement != f.Orientation && f.Orientation != 0 {
		if err := it.soft("text with an orientation other than its escapement drawn at the escapement"); err != nil {
			return err
		}
	}
	theta := float64(f.Escapement) / 10 * math.Pi / 180
	cos, sin := math.Cos(theta), math.Sin(theta)
	ky := -1.0
	if m.a*m.d-m.b*m.c < 0 {
		ky = 1
	}
	// Advances along the baseline, in logical units.
	n := len(runes)
	adv := make([]float64, n)
	pdy := t.Options&0x2000 != 0
	switch {
	case t.Advances.Len() >= n:
		if pdy {
			if err := it.soft("text with vertical advances drawn without them"); err != nil {
				return err
			}
		}
		for i := 0; i < n; i++ {
			if pdy {
				adv[i] = float64(t.Advances.SignedAt(2 * i))
			} else {
				adv[i] = float64(t.Advances.SignedAt(i))
			}
		}
	default:
		if err := it.soft("text without character advances spaced by the font's own widths"); err != nil {
			return err
		}
		for i, r := range runes {
			if gid, ok := face.GlyphID(r); ok {
				adv[i] = face.GlyphAdvance(gid) / 1000 * em
			}
		}
	}
	if d.charExtra != 0 || d.justExtra != 0 {
		if err := it.soft("text with extra character or break spacing drawn without it"); err != nil {
			return err
		}
	}
	var width float64
	for _, a := range adv {
		width += a
	}
	// The reference point, aligned.
	ref := t.Reference
	origin := point{ref.X, ref.Y}
	if d.textAlign&taUpdateCP != 0 {
		origin = d.pos
	}
	var x0 float64
	switch d.textAlign & taCenter {
	case taRight:
		x0 = -width
	case taCenter:
		x0 = -width / 2
	}
	asc, dsc := float64(desc.Ascent)*k, float64(-desc.Descent)*k
	var y0 float64 // the baseline, in text space with y up
	switch d.textAlign & taBaseline {
	case taBaseline:
	case taBottom:
		if err := it.soft("text aligned to its bottom uses the font's descent"); err != nil {
			return err
		}
		y0 = dsc
	default:
		if err := it.soft("text aligned to its top uses the font's ascent"); err != nil {
			return err
		}
		y0 = -asc
	}
	// toLogical places a text-space point (y up, relative to the start of the
	// baseline) in logical coordinates.
	toLogical := func(tx, ty float64) point {
		tx, ty = tx+x0, ty+y0
		rx, ry := tx*cos-ty*sin, tx*sin+ty*cos
		return point{origin.x + rx, origin.y + ky*ry}
	}
	toDev := func(tx, ty float64) point {
		p := toLogical(tx, ty)
		return m.apply(p)
	}
	if d.textAlign&taUpdateCP != 0 {
		end := toLogical(width, 0)
		d.pos = point{end.x, end.y}
	}
	// The cell behind the characters, in opaque mode.
	if d.bkMode == 2 && t.Options&2 == 0 {
		if err := it.soft("opaque text background uses the font's ascent and descent"); err != nil {
			return err
		}
		bk, err := it.color(d.bkColor)
		if err != nil {
			return err
		}
		cell := []point{toDev(0, asc), toDev(width, asc), toDev(width, -dsc), toDev(0, -dsc)}
		if err = it.r.fill([][]point{cell}, false, fillStyle{solid: bk}, d.clip); err != nil {
			return err
		}
	}
	col, err := it.color(d.textColor)
	if err != nil {
		return err
	}
	var contours [][]point
	var penX float64
	for i, r := range runes {
		if err = it.ctx.Err(); err != nil {
			return err
		}
		gid, ok := face.GlyphID(r)
		if !ok {
			if r != ' ' && r != '\t' && r > 0x20 {
				if err = it.soft("characters the font lacks left out"); err != nil {
					return err
				}
			}
			penX += adv[i]
			continue
		}
		var cur []point
		var last shape.Point
		var cbErr error
		flush := func() {
			if len(cur) >= 3 {
				contours = append(contours, cur)
			}
			cur = nil
		}
		at := func(p shape.Point) point { return toDev(penX+p.X*k, p.Y*k) }
		oerr := face.GlyphOutline(gid, func(s shape.Segment) bool {
			if cbErr = it.ctx.Err(); cbErr != nil {
				return false
			}
			switch s.Op {
			case shape.MoveTo:
				flush()
				cur = []point{at(s.Pts[0])}
				last = s.Pts[0]
			case shape.LineTo:
				cbErr = it.charge(1)
				cur = append(cur, at(s.Pts[0]))
				last = s.Pts[0]
			case shape.QuadTo:
				c1 := shape.Point{X: last.X + 2.0/3*(s.Pts[0].X-last.X), Y: last.Y + 2.0/3*(s.Pts[0].Y-last.Y)}
				c2 := shape.Point{X: s.Pts[1].X + 2.0/3*(s.Pts[0].X-s.Pts[1].X), Y: s.Pts[1].Y + 2.0/3*(s.Pts[0].Y-s.Pts[1].Y)}
				before := len(cur)
				cur = flattenCubic(cur, at(last), at(c1), at(c2), at(s.Pts[1]), it.tol)
				cbErr = it.charge(len(cur) - before)
				last = s.Pts[1]
			case shape.CubicTo:
				before := len(cur)
				cur = flattenCubic(cur, at(last), at(s.Pts[0]), at(s.Pts[1]), at(s.Pts[2]), it.tol)
				cbErr = it.charge(len(cur) - before)
				last = s.Pts[2]
			}
			return cbErr == nil
		})
		flush()
		if cbErr != nil {
			return cbErr
		}
		if oerr != nil {
			if err = it.soft("glyph outline left out: %v", oerr); err != nil {
				return err
			}
		}
		penX += adv[i]
	}
	if len(contours) > 0 {
		if err = it.r.fill(contours, false, fillStyle{solid: col}, d.clip); err != nil {
			return err
		}
	}
	// Underline and strikeout, as thin rectangles along the baseline.
	if f.Underline != 0 || f.StrikeOut != 0 {
		contours = nil
		if err = it.soft("text underline or strikeout drawn from the font's metrics"); err != nil {
			return err
		}
		thick := float64(desc.UnderlineThickness) * k
		if thick <= 0 {
			thick = em / 14
		}
		if f.Underline != 0 {
			pos := float64(desc.UnderlinePosition) * k
			if pos == 0 {
				pos = -em / 8
			}
			contours = append(contours, []point{toDev(0, pos), toDev(width, pos), toDev(width, pos-thick), toDev(0, pos-thick)})
		}
		if f.StrikeOut != 0 {
			pos := float64(desc.StrikeoutPosition) * k
			sz := float64(desc.StrikeoutSize) * k
			if sz <= 0 {
				sz = em / 14
			}
			if pos == 0 {
				pos = em * 0.3
			}
			contours = append(contours, []point{toDev(0, pos+sz), toDev(width, pos+sz), toDev(width, pos), toDev(0, pos)})
		}
	}
	if len(contours) == 0 {
		return nil
	}
	return it.r.fill(contours, false, fillStyle{solid: col}, d.clip)
}
