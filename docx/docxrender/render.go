// Package docxrender prepares physical pages of a Word document for the native
// preview renderer. See docs/rendering.md for the profile and render_flow.go
// for how the translation is organised and extended.
package docxrender

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/docx/internal/view"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// ErrPageOutOfRange identifies a page beyond the laid-out document.
var ErrPageOutOfRange = errors.New("render page out of range")

// Pages is a document laid out and paginated once. It is immutable and safe
// for concurrent use; it holds no reference to the document it was made from.
type Pages struct {
	pages  []*wordPage
	limits render.Limits
}

// Count is the number of physical pages.
func (p *Pages) Count() int { return len(p.pages) }

// Page prepares a 1-based physical page. A page beyond Count returns
// render.ErrInvalid wrapping ErrPageOutOfRange.
func (p *Pages) Page(ctx context.Context, page int) (*render.Page, error) {
	if ctx == nil || page < 1 {
		return nil, fmt.Errorf("%w: page", render.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if page > len(p.pages) {
		return nil, fmt.Errorf("%w: %w: page %d beyond %d pages", render.ErrInvalid, ErrPageOutOfRange, page, len(p.pages))
	}
	pg := p.pages[page-1]
	return render.Prepare(ctx, dml.EMU(math.Round(pg.w*float64(dml.EMUsPerPixel))), dml.EMU(math.Round(pg.h*float64(dml.EMUsPerPixel))), pg.ops, p.limits)
}

// PreparePage prepares a 1-based physical page of the document. It lays out
// and paginates the complete document, so rendering several pages should use
// Prepare once instead. Unsupported content fails the call even when it is on
// another page: with opts.Warn unset (strict mode) anything that cannot be
// drawn exactly returns render.ErrUnsupported; with opts.Warn set it is left
// out or approximated and reported there. Native line metrics do not promise
// identical Word pagination. See docs/rendering.md.
func PreparePage(ctx context.Context, document *docx.Document, page int, opts render.Options) (*render.Page, error) {
	if ctx == nil || document == nil || page < 1 {
		return nil, fmt.Errorf("%w: document/page", render.ErrInvalid)
	}
	pages, err := Prepare(ctx, document, opts)
	if err != nil {
		return nil, err
	}
	return pages.Page(ctx, page)
}

// Prepare lays out and paginates a document: sections, styles, paragraphs and
// runs are translated into one forme document per section, laid out once at the
// section's text width, and cut into pages. It never saves or mutates the
// document, and the result includes unsaved edits.
func Prepare(ctx context.Context, document *docx.Document, opts render.Options) (*Pages, error) {
	if ctx == nil || document == nil {
		return nil, fmt.Errorf("%w: document", render.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limits, err := core.ResolveLimits(opts.Limits)
	if err != nil {
		return nil, err
	}
	budget, err := core.NewSourceBudget(opts.MaxSourceBytes, opts.MaxLayoutNodes)
	if err != nil {
		return nil, err
	}
	r := &wordRenderer{
		ctx: ctx, opts: opts, limits: limits, budget: budget,
		lenient: opts.Warn != nil, reported: map[string]bool{},
		maxTextBytes: limits.MaxTextBytes * 8,
	}
	r.fonts = newWordFonts(r)
	d := view.DocumentOf(document)
	if d == nil || d.MainXML == nil {
		return nil, fmt.Errorf("%w: document", render.ErrInvalid)
	}
	if err = r.loadParts(d); err != nil {
		return nil, err
	}
	main, err := d.MainXML()
	if err != nil {
		return nil, err
	}
	root, err := wordRenderParse(ctx, main, budget, nsW, "document")
	if err != nil {
		return nil, fmt.Errorf("docx: main part: %w", err)
	}
	body := root.child("body")
	if body == nil {
		return nil, fmt.Errorf("%w: missing body", render.ErrInvalid)
	}
	for _, c := range root.children {
		if c.is("background") {
			if err = r.leaveOut("page background"); err != nil {
				return nil, err
			}
		} else if !c.is("body") {
			if err = r.leaveOut("document element " + wordKey(c)); err != nil {
				return nil, err
			}
		}
	}
	secs, err := r.translateBody(body)
	if err != nil {
		return nil, err
	}
	laid := make([]*wordLaidSection, 0, len(secs))
	for _, s := range secs {
		l, e := r.layoutSection(s)
		if e != nil {
			return nil, e
		}
		laid = append(laid, l)
	}
	pages, err := r.paginate(laid)
	if err != nil {
		return nil, err
	}
	if err = r.distribute(laid, pages); err != nil {
		return nil, err
	}
	for _, decorate := range wordPageDecorators {
		if err = decorate(r, laid, pages); err != nil {
			return nil, err
		}
	}
	return &Pages{pages: pages, limits: limits}, nil
}

// loadParts reads the theme, settings and styles parts.
func (r *wordRenderer) loadParts(d *view.Document) error {
	read := func(get func() ([]byte, error), space, name string) (*wordNode, error) {
		if get == nil {
			return nil, nil
		}
		data, err := get()
		if err != nil || data == nil {
			return nil, err
		}
		n, err := wordRenderParse(r.ctx, data, r.budget, space, name)
		if err != nil {
			return nil, fmt.Errorf("docx: %s part: %w", name, err)
		}
		return n, nil
	}
	theme, err := read(d.ThemeXML, nsA, "theme")
	if err != nil {
		return err
	}
	r.theme = wordRenderTheme(theme)
	settings, err := read(d.SettingsXML, nsW, "settings")
	if err != nil {
		return err
	}
	if err = r.loadSettings(settings); err != nil {
		return err
	}
	styles, err := read(d.StylesXML, nsW, "styles")
	if err != nil {
		return err
	}
	return r.loadStyles(styles)
}
