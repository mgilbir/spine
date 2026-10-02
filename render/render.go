// Package render provides immutable page snapshots with native PNG and SVG
// writers. Format-specific preparation lives in docx, pptx and xlsx. Rendering
// uses only Go, Forme and the standard library; it loads no external resources.
package render

import (
	"context"
	"io"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/common/dml"
	core "github.com/mgilbir/spine/internal/render"
)

// Limits bounds snapshot preparation and painting. Zero fields select the
// documented defaults; negative fields are invalid. See docs/rendering.md.
type Limits = core.Limits

var (
	ErrInvalid     = core.ErrInvalid
	ErrLimit       = core.ErrLimit
	ErrUnsupported = core.ErrUnsupported
)

// FontRequest identifies the Office typeface and emphasis the adapter resolved.
// The caller controls any substitution by returning a different font explicitly.
type FontRequest struct {
	Family       string
	Bold, Italic bool
}

// FontResolver supplies an already loaded immutable font. It must honour ctx
// itself; the renderer cannot interrupt a blocking callback. Font loading and
// its input budgets belong to the provider. No host-font or network lookup is
// performed by this package.
type FontResolver func(ctx context.Context, request FontRequest) (*shape.Face, error)

// Options controls format preparation. Source limits are separate from drawing
// and pixel limits because unsupported/raw content must be checked before paint.
// Zero MaxSourceBytes and MaxLayoutNodes select 16 MiB and 100000 respectively.
// Formats may impose further capability-specific bounds.
type Options struct {
	Limits         Limits
	Fonts          FontResolver
	MaxSourceBytes int64
	MaxLayoutNodes int
	// Warn, when set, makes preparation best effort: content an adapter
	// cannot draw is reported to Warn and left out, and preparation goes on
	// with the rest. The page is then incomplete. Errors that stop
	// preparation as a whole (cancellation, invalid options, malformed
	// parts, page-wide limits) are still returned. Adapters that do not
	// support best effort ignore it.
	Warn func(error)
}

// Page owns its prepared geometry and normalized raster data. It can be rendered
// concurrently and remains independent of subsequent source edits. Its zero
// value is invalid.
type Page struct{ page *core.Page }

// Prepare copies supported Forme operations in painter order onto an EMU-sized
// page. It performs no Office layout or font lookup. Unsupported operations,
// missing glyphs and resource exhaustion return no page. Caller-owned fonts and
// image buffers must remain immutable during this call.
func Prepare(ctx context.Context, width, height dml.EMU, ops []layout.Op, limits Limits) (*Page, error) {
	p, err := core.Prepare(ctx, width, height, ops, limits)
	if err != nil {
		return nil, err
	}
	return &Page{page: p}, nil
}
func (p *Page) prepared() *core.Page {
	if p == nil {
		return nil
	}
	return p.page
}

// Size returns pixel dimensions rounded upwards at dpi; zero dpi selects 96.
// Allocation limits and address-space overflow are checked before painting.
func (p *Page) Size(dpi float64) (int, int, error) { return p.prepared().Size(dpi) }

// WritePNG paints an eight-bit PNG directly onto transparent black, using sRGB
// source-over composition. Image sampling is nearest-neighbour and path edges
// use bounded numerical antialiasing. Errors can leave partial output. A blocking
// writer must provide its own cancellation; see docs/rendering.md.
func (p *Page) WritePNG(ctx context.Context, w io.Writer, dpi float64) error {
	return p.prepared().WritePNG(ctx, w, dpi)
}

// WriteSVG serializes generated geometry and normalized embedded PNG data.
// It emits no scripts, external references, font references or source markup.
// Errors can leave partial output; blocking writers need their own cancellation.
func (p *Page) WriteSVG(ctx context.Context, w io.Writer, dpi float64) error {
	return p.prepared().WriteSVG(ctx, w, dpi)
}
