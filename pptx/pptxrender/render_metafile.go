package pptxrender

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/internal/imagesniff"
	"github.com/mgilbir/spine/internal/metafile"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// renderMetafileScale is how many pixels a metafile is drawn at for each CSS
// pixel it covers, as far as the image budget allows; it is never drawn below
// half that of the page.
const (
	renderMetafileScale    = 2.0
	renderMetafileMinScale = 0.5
)

// renderIsMetafile reports whether picture bytes are an EMF or a WMF.
func renderIsMetafile(data []byte) bool {
	return imagesniff.Detect(data).In(imagesniff.EMF, imagesniff.WMF)
}

// renderMetafilePlan is the raster a metafile is drawn into.
type renderMetafilePlan struct{ w, h int }

// renderPlanMetafile sizes the raster of a metafile drawn over a box of w by h
// CSS pixels (zero for its natural size) within the pixels left in the image
// budget.
func renderPlanMetafile(data []byte, w, h float64, remaining int64, maxDim int) (renderMetafilePlan, error) {
	info, err := metafile.Inspect(data)
	if err != nil {
		return renderMetafilePlan{}, err
	}
	if !(w > 0) || !(h > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) {
		w, h = info.Width, info.Height
		if !(w > 0) || !(h > 0) {
			w, h = 96, 96/info.Aspect
		}
	}
	scale := renderMetafileScale
	if area := w * h; area*scale*scale > float64(remaining) {
		scale = math.Sqrt(float64(remaining) / area)
	}
	if side := math.Max(w, h); side*scale > float64(maxDim) {
		scale = float64(maxDim) / side
	}
	if !(scale >= renderMetafileMinScale) {
		return renderMetafilePlan{}, fmt.Errorf("%w: slide image budget for a metafile", render.ErrLimit)
	}
	pw, ph := max(1, int(math.Round(w*scale))), max(1, int(math.Round(h*scale)))
	if int64(pw)*int64(ph) > remaining || pw > maxDim || ph > maxDim {
		return renderMetafilePlan{}, fmt.Errorf("%w: slide image budget for a metafile", render.ErrLimit)
	}
	return renderMetafilePlan{pw, ph}, nil
}

// renderDrawMetafile draws a metafile into the planned raster. Strict mode
// refuses anything it cannot draw exactly; best effort reports it through the
// shape's approximation reporter.
func renderDrawMetafile(ctx context.Context, data []byte, plan renderMetafilePlan, limits render.Limits, colors *renderColors, fonts *slideRenderFonts) (image.Image, error) {
	opts := metafile.Options{Limits: limits}
	if colors != nil && colors.approx != nil {
		opts.Approximate = colors.approximate
	}
	if fonts != nil && fonts.opts.Fonts != nil {
		opts.Fonts = func(ctx context.Context, req render.FontRequest) (*shape.Face, error) {
			return fonts.resolve(ctx, req.Family, req.Bold, req.Italic)
		}
	}
	return metafile.Render(ctx, data, plan.w, plan.h, opts)
}

// renderDecodePicture decodes picture bytes: a raster picture, or a metafile
// drawn for a box of about w by h CSS pixels.
func renderDecodePicture(ctx context.Context, data []byte, limits render.Limits, w, h float64, colors *renderColors, fonts *slideRenderFonts) (image.Image, error) {
	if !renderIsMetafile(data) {
		return core.DecodeImage(ctx, data, limits)
	}
	resolved, err := core.ResolveLimits(limits)
	if err != nil {
		return nil, err
	}
	plan, err := renderPlanMetafile(data, w, h, resolved.MaxImagePixels, resolved.MaxDimension)
	if err != nil {
		return nil, err
	}
	return renderDrawMetafile(ctx, data, plan, limits, colors, fonts)
}
