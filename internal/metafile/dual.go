package metafile

import (
	"context"
	"errors"
	"fmt"

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
// drawn completely, within the limits. Otherwise the GDI records, which are
// there for readers that cannot, are drawn instead, and reported, when they
// can be read and drawn within the limits, however much of them can be
// drawn. When they cannot, what can be drawn of the EMF+ records is.
func (be *backend) preferGDI(data []byte, po gowemf.PlayOptions) (bool, error) {
	ok, err := complete(be.ctx, data, po)
	if cerr := be.ctx.Err(); cerr != nil {
		return false, cerr
	}
	unreadable := errors.Is(err, gowemf.ErrUnsupported) || errors.Is(err, gowemf.ErrMalformed)
	limit := errors.Is(err, gowemf.ErrLimit)
	switch {
	case ok:
		return false, nil
	case err != nil && !unreadable && !limit:
		return false, wrapParse(err)
	case be.opts.Approximate == nil:
		// Drawing it exactly is past the limits, or not possible.
		if limit {
			return false, wrapParse(err)
		}
		return false, be.soft("EMF+ file not drawn exactly")
	}
	why := "some of its EMF+ records cannot be drawn"
	switch {
	case unreadable:
		why = fmt.Sprintf("its EMF+ records cannot be read: %v", err)
	case limit:
		why = fmt.Sprintf("its EMF+ records are past the limits: %v", err)
	}
	gdi := po
	gdi.Stream.PreferGDI = true
	_, gerr := complete(be.ctx, data, gdi)
	if cerr := be.ctx.Err(); cerr != nil {
		return false, cerr
	}
	switch {
	case gerr == nil:
		return true, be.soft("EMF+ file drawn from its GDI records, as %s", why)
	case limit:
		return false, wrapParse(err)
	case unreadable:
		// Neither can be read through; the GDI records are the fallback.
		return true, be.soft("EMF+ file drawn from its GDI records, as %s", why)
	}
	return false, nil
}
