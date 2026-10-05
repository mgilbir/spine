package docxrender

import (
	"image"
	"strings"

	"github.com/mgilbir/spine/internal/imagesniff"
	"github.com/mgilbir/spine/internal/metafile"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// decode decodes picture bytes: a raster picture, or an EMF or WMF played onto
// a raster for a frame of about w by h CSS pixels. Strict mode refuses what a
// metafile cannot draw exactly; best effort reports it and draws the rest.
func (r *wordRenderer) decode(data []byte, lim render.Limits, w, h float64) (image.Image, error) {
	if !imagesniff.Detect(data).In(imagesniff.EMF, imagesniff.WMF) {
		return core.DecodeImage(r.ctx, data, lim)
	}
	resolved, err := core.ResolveLimits(lim)
	if err != nil {
		return nil, err
	}
	pw, ph, err := metafile.Plan(data, w, h, resolved.MaxImagePixels, resolved.MaxDimension)
	if err != nil {
		return nil, err
	}
	opts := metafile.Options{Limits: lim, Fonts: r.opts.Fonts}
	if r.lenient {
		opts.Approximate = func(err error) error {
			return r.approximate("metafile " + strings.TrimPrefix(err.Error(), render.ErrUnsupported.Error()+": metafile: "))
		}
	}
	return metafile.Render(r.ctx, data, pw, ph, opts)
}
