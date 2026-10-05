package render

import (
	"context"
	"errors"
	"fmt"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
)

func (p *Page) collectText(ctx context.Context, v layout.DrawText, clips []*geometry, budget *prepareBudget) error {
	if v.Sideways || v.Anticlockwise || v.Upright || v.Features.Vertical || v.CharSpacing != 0 {
		return fmt.Errorf("%w: vertical text or letter spacing", ErrUnsupported)
	}
	scale := v.WidthScale
	if scale == 0 {
		scale = 1
	}
	if !finite(scale) || scale <= 0 || scale > 1 || v.Size <= 0 || !validColor(v.Color) {
		return fmt.Errorf("%w: text style", ErrInvalid)
	}
	if len(v.Text) > p.limits.MaxTextBytes-budget.textBytes {
		return fmt.Errorf("%w: text bytes", ErrLimit)
	}
	budget.textBytes += len(v.Text)
	face, err := p.fontFace(v.Face, budget)
	if err != nil {
		return err
	}
	remaining := p.limits.MaxShapeWork - budget.shapeWork
	remainingGlyphs := p.limits.MaxGlyphs - budget.glyphs
	// Forme treats zero as a default, so exhausted page budgets must fail here.
	if remaining <= 0 || remainingGlyphs <= 0 {
		return fmt.Errorf("%w: shaping budget", ErrLimit)
	}
	result, err := face.ShapeGlyphsContext(ctx, shape.RunInput{
		Text: layout.ShapedText(v), Before: v.PreContext, After: v.PostContext, MergeBefore: v.MergePre, MergeAfter: v.MergePost, Kerns: v.ContextKerns, Features: face.FeaturesAt(v.Features, v.Size.Px()),
	}, shape.RunLimits{MaxInputBytes: p.limits.MaxRunBytes, MaxGlyphs: remainingGlyphs, MaxWork: remaining})
	if err != nil {
		if errors.Is(err, shape.ErrRunLimit) {
			return fmt.Errorf("%w: shaping: %w", ErrLimit, err)
		}
		return err
	}
	budget.shapeWork += result.Work
	if result.Missing != 0 {
		return fmt.Errorf("%w: %d missing text characters", ErrUnsupported, result.Missing)
	}
	return p.addGlyphs(ctx, v.At, v.Size, v.Color, v.Clip, result.Glyphs, face, v.Text, scale, clips, budget)
}
