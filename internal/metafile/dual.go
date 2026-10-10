package metafile

import (
	"context"
	"errors"

	"github.com/mgilbir/gowemf"
)

// probe is a backend that draws nothing, for learning whether Play can draw a
// stream completely.
type probe struct{ ctx context.Context }

func (p probe) FillPath(gowemf.Path, gowemf.FillRule, gowemf.Paint, gowemf.Clip) error {
	return p.ctx.Err()
}
func (p probe) StrokePath(gowemf.Path, gowemf.Stroke, gowemf.Clip) error { return p.ctx.Err() }
func (p probe) DrawImage(gowemf.ImageDraw, gowemf.Clip) error            { return p.ctx.Err() }
func (p probe) FillGradient([]gowemf.GradientTriangle, gowemf.Clip) error {
	return p.ctx.Err()
}
func (p probe) DrawText(gowemf.TextRun, gowemf.Clip) error { return p.ctx.Err() }
func (p probe) MeasureText(run gowemf.TextRun) (gowemf.TextMetrics, error) {
	return gowemf.TextMetrics{Advances: make([]float64, len(run.Text))}, p.ctx.Err()
}

// complete reports whether Play draws the stream po selects without leaving
// anything out. The error is Play's: a stream it cannot read, or a limit.
func complete(ctx context.Context, data []byte, po gowemf.PlayOptions) (bool, error) {
	clean := true
	po.Unsupported = func(gowemf.UnsupportedOperation) error {
		clean = false
		return nil
	}
	_, err := gowemf.Play(data, po, probe{ctx})
	if cerr := ctx.Err(); cerr != nil {
		return false, cerr
	}
	return clean && err == nil, err
}

// preferGDI chooses the records a Dual EMF+ file is drawn from. Its EMF+
// records are the picture as GDI+ draws it, and are drawn when they can be
// drawn completely. The GDI records are there for readers that cannot: they
// are drawn instead, and reported, when the EMF+ records cannot be read, or
// when some cannot be drawn and the GDI records can be drawn completely.
// Otherwise the EMF+ records are drawn, leaving out what cannot be.
func (be *backend) preferGDI(data []byte, po gowemf.PlayOptions) (bool, error) {
	ok, err := complete(be.ctx, data, po)
	switch {
	case ok:
		return false, nil
	case be.ctx.Err() != nil:
		return false, be.ctx.Err()
	case err != nil && !errors.Is(err, gowemf.ErrUnsupported) && !errors.Is(err, gowemf.ErrMalformed):
		return false, wrapParse(err)
	case err != nil:
		// The EMF+ records cannot be read past a point.
		return true, be.soft("EMF+ file drawn from its GDI records: %v", err)
	case be.opts.Approximate == nil:
		// Either set of records would be drawn approximately.
		return false, be.soft("EMF+ file not drawn exactly")
	}
	gdi := po
	gdi.Stream.PreferGDI = true
	ok, err = complete(be.ctx, data, gdi)
	if cerr := be.ctx.Err(); cerr != nil {
		return false, cerr
	}
	if !ok || err != nil {
		return false, nil
	}
	return true, be.soft("EMF+ file drawn from its GDI records, as some of its EMF+ records cannot be drawn")
}
