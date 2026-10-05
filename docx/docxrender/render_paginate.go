package docxrender

import (
	"fmt"
	"math"
	"sort"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/render"
)

// Pagination.
//
// forme lays a document out on one sheet and does not fragment flow across
// pages. Each section is therefore laid out once at its content width with
// unbounded height, and pages are made by cutting that single layout at block
// and line-box boundaries. A page is a list of chunks: a chunk is a range of a
// section's layout, [y0, end), drawn at a vertical offset dest from the top
// margin. Cutting at a line box gives each text line to exactly one page, and
// leaving out the layout between two chunks is what removes the space around a
// page break (Word does not draw space before at the top of a page that a soft
// or a manual break started; after a section break it does).
//
// The display list of the whole section is then distributed to the pages: text
// and other marks go to the chunk holding their baseline or centre, and block
// level fills (backgrounds that may span a cut) are clipped to each chunk they
// reach. Ops are translated by the chunk's offset and the page margins.

// wordChunk is a range of a section's layout drawn on a page.
type wordChunk struct {
	sec  *wordLaidSection
	page *wordPage
	// y0 is where the range starts in section pixels; end is the bottom of
	// the content placed in it so far.
	y0, end float64
	// dest is where y0 is drawn, in pixels from the page's top margin.
	dest float64
}

// wordPage is one physical page.
type wordPage struct {
	w, h float64
	// sec is the section whose geometry the page has: the section that opened
	// it, or for a blank page the section whose break needed it.
	sec    *wordLaidSection
	chunks []*wordChunk
	ops    []layout.Op
	// num is the page number the page shows, in the section's number format;
	// first marks the first page a section opened (the page titlePg selects
	// the first-page header and footer for).
	num   int
	first bool
	// hdr and ftr are the page's header and footer, nil when it has none.
	hdr, ftr *wordHFLaid
	// bodyTop and bodyBottom are the text area's edges from the top of the
	// page: the margins, moved in by a header or footer that reaches past them.
	bodyTop, bodyBottom float64
	// notes is the footnote area at the bottom of the text area.
	notes wordPageNotes
}

// bodyH is the height of the page's text area.
func (p *wordPage) bodyH() float64 { return p.bodyBottom - p.bodyTop }

// wordPaginator places blocks on pages.
type wordPaginator struct {
	r        *wordRenderer
	pages    []*wordPage
	chunk    *wordChunk
	has      bool // content has been placed on the current page
	maxPages int
	env      *wordEnv
	// carry is the text of footnotes that continues on the next page.
	carry []wordNotePart
	// carryCapped and endSplit record approximations to report.
	carryCapped, endSplit, carried bool
}

const wordEps = 1.0 / 128

// wordPaginate cuts laid out sections into pages.
func (r *wordRenderer) paginate(secs []*wordLaidSection, env *wordEnv) ([]*wordPage, error) {
	pg := &wordPaginator{r: r, maxPages: r.limits.MaxOperations, env: env}
	var prev *wordLaidSection
	for _, sec := range secs {
		sec.chunks = nil
		sec.resetNotes()
	}
	for _, sec := range secs {
		if err := r.ctx.Err(); err != nil {
			return nil, err
		}
		if err := pg.startSection(sec, prev); err != nil {
			return nil, err
		}
		prev = sec
		chain := wordChains(sec)
		for i := range sec.blocks {
			if err := r.ctx.Err(); err != nil {
				return nil, err
			}
			if err := pg.block(sec, i, chain); err != nil {
				return nil, err
			}
		}
	}
	if len(pg.pages) == 0 {
		return nil, fmt.Errorf("%w: no pages", render.ErrInvalid)
	}
	for len(pg.carry) > 0 {
		// Footnote text still to place after the last page of the text.
		if _, err := pg.newPage(pg.pages[len(pg.pages)-1].sec, false); err != nil {
			return nil, err
		}
	}
	if err := pg.finishNotes(); err != nil {
		return nil, err
	}
	if pg.carried && r.notes.parts[0] != nil && r.notes.parts[0].notice {
		if err := r.approximate("footnote continuation notice"); err != nil {
			return nil, err
		}
	}
	if pg.carryCapped {
		if err := r.approximate("footnote text that takes more than half a page"); err != nil {
			return nil, err
		}
	}
	if pg.endSplit {
		if err := r.approximate("endnotes that continue on another page (no continuation separator)"); err != nil {
			return nil, err
		}
	}
	return pg.pages, nil
}

// newPage appends a page of the section's geometry, numbered and furnished
// with its header and footer. first marks the first page of a section.
func (pg *wordPaginator) newPage(sec *wordLaidSection, first bool) (*wordPage, error) {
	if len(pg.pages) >= pg.maxPages {
		return nil, render.ErrLimit
	}
	page := &wordPage{w: sec.props.w, h: sec.props.h, sec: sec, first: first}
	page.num = pg.nextNumber(sec, first)
	if err := pg.r.furnish(page, pg.env); err != nil {
		return nil, err
	}
	pg.pages = append(pg.pages, page)
	pg.startNotes(page)
	return page, nil
}

// nextNumber is the number of the page about to be added: a section that
// restarts numbering starts at its value, and other pages follow the page
// before (the first page of a document is 1).
func (pg *wordPaginator) nextNumber(sec *wordLaidSection, first bool) int {
	switch {
	case first && sec.props.hasStart:
		return sec.props.pgStart
	case len(pg.pages) == 0:
		return 1
	}
	return pg.pages[len(pg.pages)-1].num + 1
}

// openPage starts a new page whose first chunk draws section layout from y0.
func (pg *wordPaginator) openPage(sec *wordLaidSection, y0 float64, first bool) error {
	if !first && sec.hasEnd && y0 > sec.endTop+wordEps {
		pg.endSplit = true
	}
	page, err := pg.newPage(sec, first)
	if err != nil {
		return err
	}
	pg.addChunk(sec, page, y0, 0)
	pg.has = false
	return nil
}

func (pg *wordPaginator) addChunk(sec *wordLaidSection, page *wordPage, y0, dest float64) {
	c := &wordChunk{sec: sec, page: page, y0: y0, end: y0, dest: dest}
	page.chunks = append(page.chunks, c)
	sec.chunks = append(sec.chunks, c)
	pg.chunk = c
}

// avail is the height of the current page's text area.
func (pg *wordPaginator) avail() float64 { return pg.chunk.page.bodyH() - pg.chunk.page.notes.h }

// blankPage appends a page with nothing on it.
func (pg *wordPaginator) blankPage(sec *wordLaidSection) error {
	_, err := pg.newPage(sec, false)
	return err
}

// startSection begins a section according to its break type.
func (pg *wordPaginator) startSection(sec, prev *wordLaidSection) error {
	typ := sec.props.typ
	if prev == nil {
		return pg.openPage(sec, 0, true)
	}
	if typ == "continuous" {
		p, c := prev.props, sec.props
		same := p.w == c.w && p.h == c.h && p.top == c.top && p.bottom == c.bottom
		if !same {
			if err := pg.r.approximate("continuous section break between different page geometries"); err != nil {
				return err
			}
		} else if used := pg.chunk.dest + pg.chunk.end - pg.chunk.y0; used < pg.avail()-wordEps {
			pg.addChunk(sec, pg.chunk.page, 0, used)
			return nil
		}
		typ = "nextPage"
	}
	// The number the next page shows decides whether an even or odd page
	// break needs a blank page first.
	next := pg.nextNumber(sec, true)
	if (typ == "oddPage" && next%2 == 0) || (typ == "evenPage" && next%2 == 1) {
		if err := pg.blankPage(sec); err != nil {
			return err
		}
	}
	return pg.openPage(sec, 0, true)
}

// dest maps a section y to the page y of the current chunk.
func (pg *wordPaginator) limitY() float64 { return pg.chunk.y0 + (pg.avail() - pg.chunk.dest) }

func (pg *wordPaginator) place(upTo float64) {
	if upTo > pg.chunk.end {
		pg.chunk.end = upTo
	}
	pg.has = true
}

// wordChain is the keep-with-next requirement of each block of a section.
type wordChain struct {
	// need is the section y a block and the blocks it must stay with reach: for
	// a keep-with-next block the first lines of the next block, after the whole
	// chain of keep-with-next blocks that follows it.
	need []float64
	// head is the top of the first block of the chain the block is in. A chain
	// taller than a page cannot be kept together and is not.
	head []float64
}

func wordChains(sec *wordLaidSection) wordChain {
	n := len(sec.blocks)
	chain := make([]float64, n)
	head := make([]float64, n)
	for i, b := range sec.blocks {
		head[i] = b.top
		if i > 0 && sec.blocks[i-1].keepNext && !b.pageBreakBefore {
			head[i] = head[i-1]
		}
	}
	for i := n - 1; i >= 0; i-- {
		b := sec.blocks[i]
		chain[i] = b.bottom
		if !b.keepNext || i+1 >= n || sec.blocks[i+1].pageBreakBefore {
			continue
		}
		next := sec.blocks[i+1]
		if next.keepNext && i+2 < n && !sec.blocks[i+2].pageBreakBefore {
			chain[i] = math.Max(chain[i], chain[i+1])
		} else {
			chain[i] = math.Max(chain[i], wordMinBottom(next))
		}
	}
	return wordChain{need: chain, head: head}
}

// wordMinBottom is the bottom of the least part of a block that has to share a
// page with the block before it: its first two lines under widow control.
func wordMinBottom(b *wordLaidBlock) float64 {
	n := len(b.units)
	if b.keepLines || n <= 1 {
		return b.bottom
	}
	k := 1
	if b.widow {
		k = 2
	}
	if k >= n {
		return b.bottom
	}
	return b.units[k-1].bottom
}

// block places block i of a section.
func (pg *wordPaginator) block(sec *wordLaidSection, i int, chain wordChain) error {
	b := sec.blocks[i]
	if b.pageBreakBefore && pg.has {
		// A manual break drops the space before at the top of the new page,
		// as Word's PDF output shows.
		if err := pg.openPage(sec, b.units[0].top, false); err != nil {
			return err
		}
	}
	n := len(b.units)
	start := 0
	for start < n {
		page := pg.chunk.page
		// fit counts the units that fit with their notes: st is the page's
		// note area as it would be after them.
		st := page.notes
		fit := 0
		for j := start; j < n; j++ {
			pb := pg.chunk.dest + b.units[j].bottom - pg.chunk.y0
			var refs []*wordNoteRef
			if b.unitNotes != nil {
				refs = b.unitNotes[j]
			}
			if len(refs) == 0 {
				if pb+st.h > page.bodyH()+wordEps {
					break
				}
				fit++
				continue
			}
			ok, partial := pg.fitNotes(&st, sec, refs, pb, false, false)
			if !ok {
				break
			}
			fit++
			if partial {
				break
			}
		}
		remaining := n - start
		if fit == remaining {
			limit := pg.limitY() - (st.h - page.notes.h)
			if start == 0 && b.keepNext && chain.need[i] > limit+wordEps && pg.has && chain.need[i]-chain.head[i] <= pg.avail() {
				if err := pg.openPage(sec, b.units[0].top, false); err != nil {
					return err
				}
				continue
			}
			pg.commitNotes(sec, b, start, remaining, false)
			pg.place(b.bottom)
			return nil
		}
		if start == 0 && b.keepLines && pg.has {
			if err := pg.openPage(sec, b.units[0].top, false); err != nil {
				return err
			}
			continue
		}
		take := fit
		if b.widow {
			if remaining-take == 1 {
				take--
			}
			if start == 0 && take < 2 {
				take = 0
			}
		}
		forced := false
		if take == 0 {
			if pg.has {
				if err := pg.openPage(sec, b.units[start].top, false); err != nil {
					return err
				}
				continue
			}
			// Nothing fits on a fresh page: place at least one unit so
			// pagination always advances.
			take = max(fit, 1)
			forced = fit == 0
		}
		pg.commitNotes(sec, b, start, take, forced)
		pg.place(b.units[start+take-1].bottom)
		start += take
		if start < n {
			if err := pg.openPage(sec, b.units[start].top, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// wordPageDecorators run after the body is distributed to pages, in order, and
// may append to a page's operations: headers, footers and page borders belong
// here. wordPage.sec is the page's section, and wordSectProps.node holds the
// section's w:sectPr for the header and footer references.
var wordPageDecorators []func(r *wordRenderer, secs []*wordLaidSection, pages []*wordPage) error

// distribute assigns the sections' display lists to pages.
func (r *wordRenderer) distribute(secs []*wordLaidSection, pages []*wordPage) error {
	for _, p := range pages {
		w, ok := style.FromPx(p.w)
		h, ok2 := style.FromPx(p.h)
		if !ok || !ok2 {
			return render.ErrLimit
		}
		p.ops = append(p.ops, layout.FillRect{Rect: layout.Rect{W: w, H: h}, Color: style.RGBA{R: 255, G: 255, B: 255, A: 1}})
	}
	total := 0
	for _, sec := range secs {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		if len(sec.chunks) == 0 {
			continue
		}
		starts := make([]float64, len(sec.chunks))
		for i, c := range sec.chunks {
			starts[i] = c.y0
		}
		// chunkAt is the last chunk starting at or above y.
		chunkAt := func(y float64) *wordChunk {
			i := sort.Search(len(starts), func(i int) bool { return starts[i] > y+wordEps }) - 1
			if i < 0 {
				i = 0
			}
			return sec.chunks[i]
		}
		for _, op := range sec.ops {
			if total++; total > 64*r.limits.MaxOperations {
				return render.ErrLimit
			}
			switch v := op.(type) {
			case layout.Link:
				// A link area draws nothing.
			case layout.DrawText:
				c := chunkAt(v.At.Y.Px())
				if sec.pool && !c.holds(v.At.Y.Px()) {
					continue
				}
				t, err := c.shift()
				if err != nil {
					return err
				}
				v.At = layout.Point{X: v.At.X.Add(t.x), Y: v.At.Y.Add(t.y)}
				if v.Clip.Active {
					return fmt.Errorf("%w: clipped text", render.ErrUnsupported)
				}
				c.page.ops = append(c.page.ops, v)
			case layout.FillRect:
				if err := r.distributeRect(sec, chunkAt, v); err != nil {
					return err
				}
			case layout.FillPath:
				c := chunkAt(wordPathCenter(v.Path))
				if sec.pool && !c.holds(wordPathCenter(v.Path)) {
					continue
				}
				t, err := c.shift()
				if err != nil {
					return err
				}
				v.Path = wordTranslatePath(v.Path, t)
				c.page.ops = append(c.page.ops, v)
			default:
				return fmt.Errorf("%w: display list operation %T", render.ErrUnsupported, op)
			}
		}
	}
	return nil
}

// holds reports whether y is within the range of the section a chunk draws.
func (c *wordChunk) holds(y float64) bool { return y >= c.y0-wordEps && y <= c.end+wordEps }

// wordShift is a translation in layout units.
type wordShift struct{ x, y style.Unit }

// shift is the translation taking section coordinates in the chunk to page
// coordinates.
func (c *wordChunk) shift() (wordShift, error) {
	x, ok := style.FromPx(c.sec.props.left)
	y, ok2 := style.FromPx(c.page.bodyTop + c.dest - c.y0)
	if !ok || !ok2 {
		return wordShift{}, render.ErrLimit
	}
	return wordShift{x, y}, nil
}

// distributeRect places a filled rectangle. Marks layout flags as overhanging
// (text decorations, inline backgrounds) belong to the line they decorate;
// block fills may span a cut and are clipped to each chunk they reach.
func (r *wordRenderer) distributeRect(sec *wordLaidSection, chunkAt func(float64) *wordChunk, v layout.FillRect) error {
	if v.Overhang {
		c := chunkAt(v.Rect.Y.Px() + v.Rect.H.Px()/2)
		if sec.pool && !c.holds(v.Rect.Y.Px()+v.Rect.H.Px()/2) {
			return nil
		}
		t, err := c.shift()
		if err != nil {
			return err
		}
		v.Rect.X, v.Rect.Y = v.Rect.X.Add(t.x), v.Rect.Y.Add(t.y)
		c.page.ops = append(c.page.ops, v)
		return nil
	}
	top, bottom := v.Rect.Y.Px(), v.Rect.Y.Px()+v.Rect.H.Px()
	for k, c := range sec.chunks {
		upper := math.Inf(1)
		if sec.pool {
			upper = c.end
		} else if k+1 < len(sec.chunks) {
			upper = sec.chunks[k+1].y0
		}
		lo, hi := math.Max(top, c.y0), math.Min(bottom, upper)
		if hi <= lo {
			continue
		}
		t, err := c.shift()
		if err != nil {
			return err
		}
		clipTop, ok := style.FromPx(lo)
		clipBottom, ok2 := style.FromPx(hi)
		if !ok || !ok2 {
			return render.ErrLimit
		}
		out := v
		out.Rect.Y, out.Rect.H = clipTop.Add(t.y), clipBottom.Sub(clipTop)
		out.Rect.X = v.Rect.X.Add(t.x)
		c.page.ops = append(c.page.ops, out)
	}
	return nil
}

// wordPathCenter is the vertical centre of a path's control points.
func wordPathCenter(p layout.Path) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range p {
		for _, y := range []float64{s.Point.Y.Px(), s.Center.Y.Px() - s.RadiusY.Px(), s.Center.Y.Px() + s.RadiusY.Px()} {
			if s.Op == layout.ArcTo || y == s.Point.Y.Px() {
				lo, hi = math.Min(lo, y), math.Max(hi, y)
			}
		}
	}
	if lo > hi {
		return 0
	}
	return (lo + hi) / 2
}

func wordTranslatePath(p layout.Path, t wordShift) layout.Path {
	out := make(layout.Path, len(p))
	for i, s := range p {
		s.Point = layout.Point{X: s.Point.X.Add(t.x), Y: s.Point.Y.Add(t.y)}
		s.Center = layout.Point{X: s.Center.X.Add(t.x), Y: s.Center.Y.Add(t.y)}
		out[i] = s
	}
	return out
}
