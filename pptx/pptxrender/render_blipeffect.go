package pptxrender

import (
	"image"

	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	core "github.com/mgilbir/spine/internal/render"
)

// blipEffects applies a picture's color effects to a copy of its image; see
// core.BlipEffects for what is drawn exactly, approximately and left out.
func (c *renderColors) blipEffects(img image.Image, effects []*dml.BlipEffect, emuPerPixel float64) (image.Image, error) {
	return core.BlipEffects(c.ctx, renderBlipColors{c}, img, effects, emuPerPixel)
}

// renderBlipColors resolves a picture effect's colors as the slide does.
type renderBlipColors struct{ c *renderColors }

func (b renderBlipColors) Color(choice dml.ColorChoice) (style.RGBA, error) {
	return b.c.color(renderChoiceColor(&choice), nil)
}

func (b renderBlipColors) Solid(f *dml.SolidFill) (style.RGBA, error) { return b.c.solid(f, nil) }

func (b renderBlipColors) Gradient(g *dml.GradFill) (style.RGBA, error) {
	return b.c.representative(g, nil, nil)
}

func (b renderBlipColors) Approximate(err error) error { return b.c.approximate(err) }
