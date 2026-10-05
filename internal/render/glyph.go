package render

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
)

func (p *Page) fontFace(source *shape.Face, budget *prepareBudget) (*shape.Face, error) {
	if source == nil || source.UnitsPerEm() <= 0 {
		return nil, fmt.Errorf("%w: missing font", ErrInvalid)
	}
	if f := budget.faces[source]; f != nil {
		return f, nil
	}
	if len(budget.faces) >= p.limits.MaxFonts {
		return nil, fmt.Errorf("%w: font count", ErrLimit)
	}
	program := source.Program()
	if int64(len(program)) > p.limits.MaxFontBytes-budget.fontBytes {
		return nil, fmt.Errorf("%w: font bytes", ErrLimit)
	}
	budget.fontBytes += int64(len(program))
	if len(program) < 12 {
		return nil, fmt.Errorf("%w: font has no outline program", ErrUnsupported)
	}
	if err := checkFontDirectory(program); err != nil {
		return nil, err
	}
	f := source.Clone()
	if len(f.LayoutLimits()) > 0 {
		return nil, fmt.Errorf("%w: font layout was truncated", ErrLimit)
	}
	if budget.faces == nil {
		budget.faces = map[*shape.Face]*shape.Face{}
	}
	budget.faces[source] = f
	return f, nil
}

func (p *Page) collectGlyphs(ctx context.Context, v layout.DrawGlyphs, clips []*geometry, budget *prepareBudget) error {
	if len(v.Text) > p.limits.MaxTextBytes-budget.textBytes {
		return fmt.Errorf("%w: text bytes", ErrLimit)
	}
	budget.textBytes += len(v.Text)
	if v.Size <= 0 || !validColor(v.Color) {
		return fmt.Errorf("%w: glyph style", ErrInvalid)
	}
	f, err := p.fontFace(v.Face, budget)
	if err != nil {
		return err
	}
	return p.addGlyphs(ctx, v.At, v.Size, v.Color, v.Clip, v.Glyphs, f, v.Text, 1, clips, budget)
}

// outlineGeometry is a glyph's outline as a nonzero-filled geometry, each
// point of it placed by place, counted against the page's path segments.
func (p *Page) outlineGeometry(ctx context.Context, face *shape.Face, gid int, place func(shape.Point) (x, y float64), budget *prepareBudget) (*geometry, error) {
	g := &geometry{nonzero: true, curves: make([]curve, 0), bounds: rectangle{x0: math.Inf(1), y0: math.Inf(1), x1: math.Inf(-1), y1: math.Inf(-1)}}
	var callbackError error
	err := face.GlyphOutline(gid, func(s shape.Segment) bool {
		if err := ctx.Err(); err != nil {
			callbackError = err
			return false
		}
		if budget.segments >= p.limits.MaxPathSegments {
			callbackError = fmt.Errorf("%w: glyph segments", ErrLimit)
			return false
		}
		var c curve
		n := 1
		switch s.Op {
		case shape.MoveTo:
			c.op = 'M'
		case shape.LineTo:
			c.op = 'L'
		case shape.QuadTo:
			c.op = 'Q'
			n = 2
		case shape.CubicTo:
			c.op = 'C'
			n = 3
		default:
			callbackError = fmt.Errorf("%w: glyph segment", ErrUnsupported)
			return false
		}
		for i := 0; i < n; i++ {
			x, y := place(s.Pts[i])
			if !finite(x) || !finite(y) || math.Abs(x) > style.MaxUnit.Px() || math.Abs(y) > style.MaxUnit.Px() {
				callbackError = fmt.Errorf("%w: glyph coordinate range", ErrLimit)
				return false
			}
			c.pts[i] = point{x, y}
			g.bounds.x0, g.bounds.y0 = math.Min(g.bounds.x0, x), math.Min(g.bounds.y0, y)
			g.bounds.x1, g.bounds.y1 = math.Max(g.bounds.x1, x), math.Max(g.bounds.y1, y)
		}
		budget.segments++
		g.curves = append(g.curves, c)
		return true
	})
	if callbackError != nil {
		return nil, callbackError
	}
	if err != nil {
		return nil, fmt.Errorf("%w: glyph outline: %w", ErrUnsupported, err)
	}
	if len(g.curves) == 0 {
		g.bounds = rectangle{}
	}
	return g, nil
}

func (b *prepareBudget) fontID(face *shape.Face) string {
	if b.fontIDs == nil {
		b.fontIDs = map[*shape.Face]string{}
	}
	id := b.fontIDs[face]
	if id == "" {
		hash := sha256.Sum256(face.Program())
		id = fmt.Sprintf("%x", hash)
		b.fontIDs[face] = id
	}
	return id
}

func (p *Page) addGlyphs(ctx context.Context, at layout.Point, size style.Unit, color style.RGBA, clip layout.Clip, glyphs []shape.Glyph, face *shape.Face, text string, widthScale float64, clips []*geometry, budget *prepareBudget) error {
	if len(glyphs) > p.limits.MaxGlyphs-budget.glyphs {
		return fmt.Errorf("%w: glyph count", ErrLimit)
	}
	budget.glyphs += len(glyphs)
	pen := 0.0
	ppem := glyphPPEM(size)
	region := meet(rectangle{x0: math.Inf(-1), y0: math.Inf(-1), x1: math.Inf(1), y1: math.Inf(1)}, rectangle{0, 0, p.width, p.height, style.RGBA{}})
	if clip.Active && len(glyphs) > 0 {
		if clip.Rect.W < 0 || clip.Rect.H < 0 {
			return fmt.Errorf("%w: glyph clip", ErrInvalid)
		}
		c := clip.Rect
		region = meet(region, rectangle{c.X.Px(), c.Y.Px(), c.X.Px() + c.W.Px(), c.Y.Px() + c.H.Px(), style.RGBA{}})
	}
	for index, glyph := range glyphs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if glyph.GID <= 0 || glyph.GID >= face.NumGlyphs() {
			return fmt.Errorf("%w: invalid/missing glyph %d", ErrUnsupported, glyph.GID)
		}
		if !finite(glyph.XAdvance) || !finite(glyph.XOffset) || !finite(glyph.YOffset) || glyph.YAdvance != 0 || glyph.VOriginX != 0 || glyph.VOriginY != 0 {
			return fmt.Errorf("%w: glyph placement", ErrInvalid)
		}
		logical := ""
		if index == 0 {
			logical = text
		}
		if face.GlyphColour(glyph.GID, ppem) != shape.ColourNone {
			upem := float64(face.UnitsPerEm())
			run := colorRun{
				face: face, gid: glyph.GID, ppem: ppem, color: color, region: region, clips: clips,
				base: affine{
					a: size.Px() / upem * widthScale, d: -size.Px() / upem,
					e: at.X.Px() + (pen+glyph.XOffset)*size.Px()/1000*widthScale, f: at.Y.Px() - glyph.YOffset*size.Px()/1000,
				},
				text: logical, fontID: budget.fontID(face),
			}
			drawn, err := p.paintColorGlyph(ctx, run, budget)
			if err != nil {
				return err
			}
			if drawn {
				pen += glyph.XAdvance
				if !finite(pen) {
					return fmt.Errorf("%w: glyph advance", ErrLimit)
				}
				continue
			}
		}
		ox, oy := at.X.Px(), at.Y.Px()
		var g *geometry
		if _, _, w, h, ok := face.GlyphExtents(glyph.GID); face.BitmapOnly() && (!ok || w == 0 || h == 0) {
			// A glyph of a bitmap font that has no ink, a space, draws nothing
			// and, having no outline either, is not drawn as one.
			g = &geometry{nonzero: true, curves: make([]curve, 0)}
		} else {
			var err error
			g, err = p.outlineGeometry(ctx, face, glyph.GID, func(pt shape.Point) (float64, float64) {
				x := ox + ((pen+glyph.XOffset)*size.Px()/1000+pt.X*size.Px()/float64(face.UnitsPerEm()))*widthScale
				y := oy - glyph.YOffset*size.Px()/1000 - pt.Y*size.Px()/float64(face.UnitsPerEm())
				return x, y
			}, budget)
			if err != nil {
				return err
			}
		}
		r := meet(g.bounds, region)
		r.color = color
		p.draws = append(p.draws, drawing{rect: r, path: g, clips: clips, text: logical, fontID: budget.fontID(face)})
		pen += glyph.XAdvance
		if !finite(pen) {
			return fmt.Errorf("%w: glyph advance", ErrLimit)
		}
	}
	if len(face.LayoutLimits()) > 0 {
		return fmt.Errorf("%w: font layout was truncated", ErrLimit)
	}

	return nil
}

// checkFontDirectory refuses a font program whose table directory does not fit
// it. Glyphs are drawn as the face has them: outlines, or, for COLR, SVG, CBDT,
// sbix, EBDT and bdat glyphs, one at a time by paintColorGlyph. A font with
// outlines draws from them whatever bitmap strikes it carries; one without
// draws its glyphs from its strikes.
func checkFontDirectory(program []byte) error {
	if int(binary.BigEndian.Uint16(program[4:6])) > (len(program)-12)/16 {
		return fmt.Errorf("%w: font directory", ErrInvalid)
	}
	return nil
}
