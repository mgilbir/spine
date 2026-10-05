package docxrender

import (
	"fmt"
	"math"
	"strconv"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/render"
)

// Headers and footers.
//
// A section references up to three headers and three footers: the default
// one, the one for its first page (when w:titlePg is set) and the one for even
// pages (when the settings part has w:evenAndOddHeaders). A reference a section
// does not make is inherited from the previous section's; a type nobody
// defines draws nothing. A page's type comes from the page that opened it: the
// first page of its section, an even displayed page number, or the default.
//
// The content is paragraphs (and whatever other block kinds are registered),
// translated by the same code as the body, laid out at the section's text
// width, and drawn from the header or footer distance of the page edge. A
// header that reaches past the top margin moves the text area down, and a
// footer past the bottom margin moves it up, so the heights are needed while
// the body is paginated: wordRenderer.furnish gives each page its header and
// footer as it is opened.
//
// PAGE, NUMPAGES, SECTIONPAGES and SECTION show real values (render_fields.go).
// A part is translated and laid out once for each distinct set of the values it
// shows, and the layouts are cached. NUMPAGES and SECTIONPAGES are known only
// after pagination, so Prepare paginates with a guess and repeats while the
// header or footer heights it assumed turn out different (wordEnv).

// The header and footer types.
const (
	wordHFDefault = iota
	wordHFFirst
	wordHFEven
	wordHFTypes
)

// wordHFRounds bounds how often pagination repeats because the page count
// changed the heights of headers and footers.
const wordHFRounds = 4

// addHFRef reads a w:headerReference or w:footerReference.
func (s *wordSectProps) addHFRef(c *wordNode) error {
	var rid string
	for _, a := range c.attrs {
		if a.space == nsRelat && a.name == "id" {
			rid = a.value
		}
	}
	var typ int
	switch v, _ := c.attr("type"); v {
	case "default":
		typ = wordHFDefault
	case "first":
		typ = wordHFFirst
	case "even":
		typ = wordHFEven
	default:
		return fmt.Errorf("%w: w:%s type", render.ErrInvalid, c.name)
	}
	if rid == "" {
		return fmt.Errorf("%w: w:%s without r:id", render.ErrInvalid, c.name)
	}
	dst := &s.header
	if c.name == "footerReference" {
		dst = &s.footer
	}
	if dst[typ] != "" {
		return fmt.Errorf("%w: duplicate w:%s", render.ErrInvalid, c.name)
	}
	dst[typ] = rid
	return nil
}

// parsePgNumType reads w:pgNumType.
func (r *wordRenderer) parsePgNumType(s *wordSectProps, c *wordNode) error {
	if v, ok := c.attr("start"); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < -(1<<30) || n > 1<<30 {
			return fmt.Errorf("%w: w:pgNumType start", render.ErrInvalid)
		}
		s.hasStart, s.pgStart = true, n
	}
	s.pgFmt = attrOrEmpty(c, "fmt")
	if _, ok := c.attr("chapStyle"); ok {
		return r.approximate("chapter numbers in page numbers")
	}
	return nil
}

// wordInheritHF fills the header and footer references a section does not
// make from the previous section's.
func wordInheritHF(secs []*wordSection) {
	for i := 1; i < len(secs); i++ {
		cur, prev := &secs[i].props, &secs[i-1].props
		for t := 0; t < wordHFTypes; t++ {
			if cur.header[t] == "" {
				cur.header[t] = prev.header[t]
			}
			if cur.footer[t] == "" {
				cur.footer[t] = prev.footer[t]
			}
		}
	}
}

// wordEnv is what pagination assumes about the document's page count while
// the headers and footers are sized: the number of pages, and of those of each
// section.
type wordEnv struct {
	total int
	sec   []int
}

func (e *wordEnv) secPages(i int) int {
	if i >= 0 && i < len(e.sec) {
		return e.sec[i]
	}
	return 0
}

func (e *wordEnv) equal(o *wordEnv) bool {
	if e.total != o.total || len(e.sec) != len(o.sec) {
		return false
	}
	for i, n := range e.sec {
		if o.sec[i] != n {
			return false
		}
	}
	return true
}

// measureEnv counts the pages of a pagination and of each section. A section's
// pages are those it opened and those it has body content on.
func measureEnv(secs []*wordLaidSection, pages []*wordPage) *wordEnv {
	e := &wordEnv{total: len(pages), sec: make([]int, len(secs))}
	last := make([]int, len(secs))
	for i, p := range pages {
		count := func(s *wordLaidSection) {
			if last[s.props.idx] != i+1 {
				last[s.props.idx] = i + 1
				e.sec[s.props.idx]++
			}
		}
		count(p.sec)
		for _, c := range p.chunks {
			count(c.sec)
		}
	}
	return e
}

// wordHFLaid is a header or footer laid out: its height and display list, in
// coordinates from the top left of the text area.
type wordHFLaid struct {
	h   float64
	ops []layout.Op
}

// wordHFPart is a header or footer part and the layouts made from it.
type wordHFPart struct {
	root *wordNode
	// uses is which page-dependent values the part shows, known after the first
	// translation; the layouts are cached by those values and the width.
	uses   uint8
	probed bool
	cache  map[string]*wordHFLaid
}

// wordHF is the header and footer parts read so far, by relationship id.
type wordHF struct {
	source func(rid string) ([]byte, error)
	parts  map[string]*wordHFPart
	// totals is set when a part shows NUMPAGES or SECTIONPAGES: pagination
	// then depends on its own page count.
	totals bool
}

// part returns the header or footer part for a relationship id, nil when the
// document has none.
func (r *wordRenderer) hfPart(footer bool, rid string) (*wordHFPart, error) {
	key, root := "h"+rid, "hdr"
	if footer {
		key, root = "f"+rid, "ftr"
	}
	if p, ok := r.hf.parts[key]; ok {
		return p, nil
	}
	var part *wordHFPart
	if r.hf.source != nil {
		data, err := r.hf.source(rid)
		if err != nil {
			return nil, err
		}
		if data != nil {
			n, err := wordRenderParse(r.ctx, data, r.budget, nsW, root)
			if err != nil {
				return nil, fmt.Errorf("docx: %s part: %w", root, err)
			}
			part = &wordHFPart{root: n, cache: map[string]*wordHFLaid{}}
		}
	}
	r.hf.parts[key] = part
	if part == nil {
		return nil, r.leaveOut("header or footer part that is missing")
	}
	return part, nil
}

// hfType is the type of header and footer a page shows.
func (r *wordRenderer) hfType(page *wordPage) int {
	switch {
	case page.sec.props.titlePg && page.first:
		return wordHFFirst
	case r.evenOdd && page.num&1 == 0:
		return wordHFEven
	}
	return wordHFDefault
}

// furnish sets a page's header, footer and text area. It is called as the page
// is opened, once its number is known.
func (r *wordRenderer) furnish(page *wordPage, env *wordEnv) error {
	p := page.sec.props
	page.bodyTop, page.bodyBottom = p.top, p.h-p.bottom
	page.hdr, page.ftr = nil, nil
	t := r.hfType(page)
	vals := wordPageVals{page: page.num, pageFmt: p.pgFmt, total: env.total, secPages: env.secPages(p.idx), sec: p.idx + 1}
	top, bottom := p.top, p.bottom
	for i, rid := range [2]string{p.header[t], p.footer[t]} {
		if rid == "" {
			continue
		}
		l, err := r.hfLaid(i == 1, rid, p, vals)
		if err != nil {
			return err
		}
		if l == nil || l.h <= 0 {
			continue
		}
		if i == 0 {
			page.hdr = l
			top = math.Max(top, p.hdrDist+l.h)
		} else {
			page.ftr = l
			bottom = math.Max(bottom, p.ftrDist+l.h)
		}
	}
	if p.h-top-bottom < 1 {
		// Header and footer leave no room for text: draw the text area where
		// the margins put it, over them.
		if err := r.approximate("header or footer taller than the page"); err != nil {
			return err
		}
		return nil
	}
	page.bodyTop, page.bodyBottom = top, p.h-bottom
	return nil
}

// refurnish gives pages the headers and footers of a new page count, when
// that changes none of their heights, and reports whether it did. The
// pagination then stands as it is.
func (r *wordRenderer) refurnish(pages []*wordPage, env *wordEnv) (bool, error) {
	type pair struct{ hdr, ftr *wordHFLaid }
	out := make([]pair, len(pages))
	for i, page := range pages {
		p := page.sec.props
		t := r.hfType(page)
		vals := wordPageVals{page: page.num, pageFmt: p.pgFmt, total: env.total, secPages: env.secPages(p.idx), sec: p.idx + 1}
		for k, rid := range [2]string{p.header[t], p.footer[t]} {
			var l *wordHFLaid
			if rid != "" {
				var err error
				if l, err = r.hfLaid(k == 1, rid, p, vals); err != nil {
					return false, err
				}
				if l != nil && l.h <= 0 {
					l = nil
				}
			}
			old := page.hdr
			if k == 1 {
				old = page.ftr
			}
			if (l == nil) != (old == nil) || (l != nil && l.h != old.h) {
				return false, nil
			}
			if k == 0 {
				out[i].hdr = l
			} else {
				out[i].ftr = l
			}
		}
	}
	for i, page := range pages {
		page.hdr, page.ftr = out[i].hdr, out[i].ftr
	}
	return true, nil
}

// hfLaid returns the laid out header or footer for the values of a page.
func (r *wordRenderer) hfLaid(footer bool, rid string, p wordSectProps, vals wordPageVals) (*wordHFLaid, error) {
	part, err := r.hfPart(footer, rid)
	if err != nil || part == nil {
		return nil, err
	}
	key := func() string { return strconv.FormatFloat(p.contentW(), 'f', 3, 64) + "|" + vals.key(part.uses) }
	if part.probed {
		if l, ok := part.cache[key()]; ok {
			return l, nil
		}
	}
	if len(part.cache) >= r.maxPages() {
		return nil, render.ErrLimit
	}
	l, uses, err := r.layoutHF(part, p, vals)
	if err != nil {
		return nil, err
	}
	if !part.probed {
		part.probed, part.uses = true, uses
		if uses&(wordUseTotal|wordUseSecPages) != 0 {
			r.hf.totals = true
		}
	}
	part.cache[key()] = l
	return l, nil
}

func (r *wordRenderer) maxPages() int { return r.limits.MaxOperations }

// layoutHF translates and lays out a header or footer part for a page's
// values, reporting which values it showed.
func (r *wordRenderer) layoutHF(part *wordHFPart, p wordSectProps, vals wordPageVals) (*wordHFLaid, uint8, error) {
	f := &wordFlow{r: r, hf: &wordHFCtx{vals: vals}}
	if err := f.blocksOf(part.root); err != nil {
		return nil, 0, err
	}
	var blocks []*wordBlock
	for _, s := range f.secs {
		blocks = append(blocks, s.blocks...)
	}
	blocks = append(blocks, f.cur...)
	uses := f.hf.uses
	if len(blocks) == 0 {
		return &wordHFLaid{}, uses, nil
	}
	for _, b := range blocks {
		if b.pageBreakBefore {
			if err := r.approximate("page break in a header or footer"); err != nil {
				return nil, 0, err
			}
			break
		}
	}
	laid, err := r.layoutSection(&wordSection{props: p, blocks: blocks})
	if err != nil {
		return nil, 0, err
	}
	l := &wordHFLaid{ops: laid.ops}
	if n := len(laid.blocks); n > 0 {
		l.h = laid.blocks[n-1].bottom
	}
	return l, uses, nil
}

func init() {
	wordPageDecorators = append(wordPageDecorators, wordDrawHF)
	wordRegisterRun("pgNum", func(rn *wordRun, _ *wordNode) error {
		f := rn.p.f
		if f.hf == nil {
			if !f.visible() {
				return nil
			}
			return rn.p.r.leaveOut("page number placeholders")
		}
		if rn.hidden() {
			return nil
		}
		text, exact := f.hf.pageText()
		if !exact {
			if err := rn.p.r.approximate("page number format"); err != nil {
				return err
			}
		}
		return rn.emitString(text)
	})
}

// wordDrawHF adds each page's header and footer to its display list.
func wordDrawHF(r *wordRenderer, _ []*wordLaidSection, pages []*wordPage) error {
	for _, page := range pages {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		p := page.sec.props
		if page.hdr != nil {
			if err := page.place(page.hdr.ops, p.left, p.hdrDist); err != nil {
				return err
			}
		}
		if page.ftr != nil {
			if err := page.place(page.ftr.ops, p.left, page.h-p.ftrDist-page.ftr.h); err != nil {
				return err
			}
		}
	}
	return nil
}

// place appends display list operations translated by (dx, dy) pixels.
func (page *wordPage) place(ops []layout.Op, dx, dy float64) error {
	x, ok := style.FromPx(dx)
	y, ok2 := style.FromPx(dy)
	if !ok || !ok2 {
		return render.ErrLimit
	}
	t := wordShift{x, y}
	for _, op := range ops {
		switch v := op.(type) {
		case layout.Link:
		case layout.DrawText:
			if v.Clip.Active {
				return fmt.Errorf("%w: clipped text", render.ErrUnsupported)
			}
			v.At = layout.Point{X: v.At.X.Add(t.x), Y: v.At.Y.Add(t.y)}
			page.ops = append(page.ops, v)
		case layout.FillRect:
			v.Rect.X, v.Rect.Y = v.Rect.X.Add(t.x), v.Rect.Y.Add(t.y)
			page.ops = append(page.ops, v)
		case layout.FillPath:
			v.Path = wordTranslatePath(v.Path, t)
			page.ops = append(page.ops, v)
		default:
			return fmt.Errorf("%w: display list operation %T", render.ErrUnsupported, op)
		}
	}
	return nil
}
