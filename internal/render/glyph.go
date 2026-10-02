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
	count := int(binary.BigEndian.Uint16(program[4:6]))
	if count > (len(program)-12)/16 {
		return nil, fmt.Errorf("%w: font directory", ErrInvalid)
	}
	for i := 0; i < count; i++ {
		tag := string(program[12+16*i : 16+16*i])
		switch tag {
		case "COLR", "CPAL", "SVG ", "sbix", "CBDT", "CBLC", "bdat", "bloc":
			return nil, fmt.Errorf("%w: color/bitmap font table %s", ErrUnsupported, tag)
		}
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

func (p *Page) addGlyphs(ctx context.Context, at layout.Point, size style.Unit, color style.RGBA, clip layout.Clip, glyphs []shape.Glyph, face *shape.Face, text string, widthScale float64, clips []*geometry, budget *prepareBudget) error {
	if len(glyphs) > p.limits.MaxGlyphs-budget.glyphs {
		return fmt.Errorf("%w: glyph count", ErrLimit)
	}
	budget.glyphs += len(glyphs)
	pen := 0.0
	for index, glyph := range glyphs {
		g := &geometry{nonzero: true, curves: make([]curve, 0), bounds: rectangle{x0: math.Inf(1), y0: math.Inf(1), x1: math.Inf(-1), y1: math.Inf(-1)}}
		if err := ctx.Err(); err != nil {
			return err
		}
		if glyph.GID <= 0 || glyph.GID >= face.NumGlyphs() {
			return fmt.Errorf("%w: invalid/missing glyph %d", ErrUnsupported, glyph.GID)
		}
		if !finite(glyph.XAdvance) || !finite(glyph.XOffset) || !finite(glyph.YOffset) || glyph.YAdvance != 0 || glyph.VOriginX != 0 || glyph.VOriginY != 0 {
			return fmt.Errorf("%w: glyph placement", ErrInvalid)
		}
		var callbackError error
		err := face.GlyphOutline(glyph.GID, func(s shape.Segment) bool {
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
				x := at.X.Px() + ((pen+glyph.XOffset)*size.Px()/1000+s.Pts[i].X*size.Px()/float64(face.UnitsPerEm()))*widthScale
				y := at.Y.Px() - glyph.YOffset*size.Px()/1000 - s.Pts[i].Y*size.Px()/float64(face.UnitsPerEm())
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
			return callbackError
		}
		if err != nil {
			return fmt.Errorf("%w: glyph outline: %w", ErrUnsupported, err)
		}
		if len(g.curves) == 0 {
			g.bounds = rectangle{}
		}
		r := meet(g.bounds, rectangle{0, 0, p.width, p.height, style.RGBA{}})
		if clip.Active {
			if clip.Rect.W < 0 || clip.Rect.H < 0 {
				return fmt.Errorf("%w: glyph clip", ErrInvalid)
			}
			c := clip.Rect
			r = meet(r, rectangle{c.X.Px(), c.Y.Px(), c.X.Px() + c.W.Px(), c.Y.Px() + c.H.Px(), style.RGBA{}})
		}
		r.color = color
		if budget.fontIDs == nil {
			budget.fontIDs = map[*shape.Face]string{}
		}
		id := budget.fontIDs[face]
		if id == "" {
			hash := sha256.Sum256(face.Program())
			id = fmt.Sprintf("%x", hash)
			budget.fontIDs[face] = id
		}
		logical := ""
		if index == 0 {
			logical = text
		}
		p.draws = append(p.draws, drawing{rect: r, path: g, clips: clips, text: logical, fontID: id})
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
