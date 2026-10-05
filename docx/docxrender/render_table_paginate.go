package docxrender

import (
	"sort"
	"strconv"

	"github.com/mgilbir/forme/layout"
)

// Table pagination.
//
// A table is cut between rows, and inside a row only where Word does: a row
// that may split (no w:cantSplit) is cut between the lines of its cells, at a
// place no line of any cell crosses. A row that cannot split moves to the next
// page whole, and splits anyway only where it is taller than a page. Rows that
// a vertical merge joins cannot be cut between and are one group. Header rows
// (w:tblHeader, from the first row) are drawn again at the top of each page the
// table continues on, as a repeated chunk (wordChunk.repeat).

// wordRowGroup is a row, or rows that cannot be cut between, located in the
// section's layout.
type wordRowGroup struct {
	top, bottom float64
	// units are the places the group may be cut, in order.
	units []wordUnit
	// noSplit keeps the group on one page when it fits on one (cantSplit, or
	// vertical alignment, which the cut would not follow).
	noSplit bool
	header  bool
	// keepNext keeps the group with the one after it.
	keepNext bool
}

// wordTablePlan is where a table's rows are, known after layout.
type wordTablePlan struct {
	groups []wordRowGroup
	// header is the number of leading groups that repeat on each page.
	header int
	// flat is every unit of the table, for the keep rules of the blocks around.
	flat []wordUnit
}

// wordFragIndex locates the generated table elements of a layout by id.
type wordFragIndex map[string]*layout.Fragment

// wordIndexFragments indexes the cells ("c"), rows ("r") and tab paragraphs
// ("t") under a fragment.
func wordIndexFragments(root *layout.Fragment) wordFragIndex {
	ix := wordFragIndex{}
	stack := []*layout.Fragment{root}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if f.Box != nil && f.Box.Element != nil {
			if id, ok := f.Box.Element.Attr("id"); ok && len(id) > 1 && (id[0] == 'c' || id[0] == 'r' || id[0] == 't') {
				if _, dup := ix[id]; !dup {
					ix[id] = f
				}
			}
		}
		stack = append(stack, f.Children...)
	}
	return ix
}

// wordLineExtents appends the vertical extent of every line box under f.
func wordLineExtents(f *layout.Fragment, out *[]wordUnit) {
	stack := []*layout.Fragment{f}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(cur.Lines) > 0 {
			content := cur.ContentRect()
			for _, l := range cur.Lines {
				top := content.Y.Add(l.Rect.Y).Px()
				*out = append(*out, wordUnit{top, top + l.Rect.H.Px()})
			}
		}
		stack = append(stack, cur.Children...)
	}
}

// inspect checks the table against its layout and, for a table of the body,
// locates its rows.
func (t *wordTable) inspect(wrapper *layout.Fragment, ix wordFragIndex) error {
	r := t.r
	for _, row := range t.rows {
		rf := ix["r"+strconv.Itoa(row.id)]
		if rf == nil {
			return wordErrTable("layout dropped a row")
		}
		if row.pr.rule == "exact" && rf.BorderRect.H.Px() > row.pr.height+0.5 {
			if err := r.approximate("text taller than an exact row height (not clipped)"); err != nil {
				return err
			}
		}
		for _, c := range row.cells {
			for _, b := range c.blocks {
				if b.tab == nil || b.expectLines == 0 {
					continue
				}
				f := ix["t"+strconv.Itoa(b.id)]
				if f == nil {
					return wordErrTable("layout dropped a paragraph")
				}
				if len(wordParagraphUnits(f)) != b.expectLines {
					if err := r.approximate("tab stops in a paragraph that wraps"); err != nil {
						return err
					}
				}
			}
		}
	}
	if t.nested {
		return nil
	}
	return t.buildPlan(wrapper, ix)
}

// buildPlan locates the row groups of a table and where each may be cut.
func (t *wordTable) buildPlan(wrapper *layout.Fragment, ix wordFragIndex) error {
	const eps = 1.0 / 64
	top, bottom := wrapper.BorderRect.Y.Px(), wrapper.BorderRect.Bottom().Px()
	n := len(t.rows)
	// bound[k] is the bottom of row k, where the cut after it would be.
	bound := make([]float64, n)
	var lines []wordUnit
	for k, row := range t.rows {
		rf := ix["r"+strconv.Itoa(row.id)]
		bound[k] = rf.BorderRect.Bottom().Px()
		if k > 0 && bound[k] < bound[k-1] {
			bound[k] = bound[k-1]
		}
		for _, c := range row.cells {
			cf := ix["c"+strconv.Itoa(c.id)]
			if cf == nil {
				return wordErrTable("layout dropped a cell")
			}
			wordLineExtents(cf, &lines)
		}
	}
	bound[n-1] = bottom
	// Merge the line extents into runs no cut may enter.
	sort.Slice(lines, func(i, j int) bool { return lines[i].top < lines[j].top })
	var runs []wordUnit
	for _, l := range lines {
		if k := len(runs) - 1; k >= 0 && l.top < runs[k].bottom-eps {
			runs[k].bottom = max(runs[k].bottom, l.bottom)
			continue
		}
		runs = append(runs, l)
	}
	// crossed reports whether a line extent contains y, so no cut may be there.
	crossed := func(y float64) bool {
		i := sort.Search(len(runs), func(i int) bool { return runs[i].bottom > y+eps })
		return i < len(runs) && runs[i].top < y-eps
	}
	plan := t.plan
	start := 0
	gtop := top
	for k := range n {
		last := k == n-1
		if !last && crossed(bound[k]) {
			continue
		}
		g := wordRowGroup{top: gtop, bottom: bound[k], header: true}
		for _, row := range t.rows[start : k+1] {
			g.noSplit = g.noSplit || row.pr.cantSplit
			g.header = g.header && row.pr.header
			for _, c := range row.cells {
				g.noSplit = g.noSplit || c.vAlign != "top"
			}
		}
		g.keepNext = t.rows[k].keepNext
		g.units = wordCuts(g.top, g.bottom, runs, eps)
		plan.groups = append(plan.groups, g)
		plan.flat = append(plan.flat, g.units...)
		start, gtop = k+1, bound[k]
	}
	for _, g := range plan.groups {
		if !g.header {
			break
		}
		plan.header++
	}
	if plan.header == len(plan.groups) {
		// A table of header rows only repeats nothing.
		plan.header = 0
	}
	return nil
}

// wordCuts splits [top, bottom] at the gaps between the line runs inside it.
func wordCuts(top, bottom float64, runs []wordUnit, eps float64) []wordUnit {
	i := sort.Search(len(runs), func(i int) bool { return runs[i].top >= top-eps })
	var units []wordUnit
	start := top
	for ; i+1 < len(runs) && runs[i+1].top < bottom-eps; i++ {
		if runs[i].bottom <= start+eps {
			continue
		}
		units = append(units, wordUnit{start, runs[i].bottom})
		start = runs[i+1].top
	}
	return append(units, wordUnit{start, bottom})
}

// tableBlock places a table block: row groups in order, cutting inside a group
// only where it does not fit.
func (pg *wordPaginator) tableBlock(sec *wordLaidSection, b *wordLaidBlock) error {
	plan := b.plan
	groups := plan.groups
	// need[k] is where group k and the groups it must stay with end.
	need := make([]float64, len(groups))
	head := make([]float64, len(groups))
	for k, g := range groups {
		head[k] = g.top
		if k > 0 && groups[k-1].keepNext {
			head[k] = head[k-1]
		}
	}
	for k := len(groups) - 1; k >= 0; k-- {
		need[k] = groups[k].bottom
		if groups[k].keepNext && k+1 < len(groups) {
			need[k] = max(need[k], need[k+1])
			if !groups[k+1].keepNext {
				need[k] = max(need[k], groups[k+1].units[0].bottom)
			}
		}
	}
	hdr := plan.header
	var hdrTop, hdrBottom float64
	if hdr > 0 {
		hdrTop, hdrBottom = groups[0].top, groups[hdr-1].bottom
		if hdrBottom-hdrTop+groups[hdr].units[0].bottom-groups[hdr].top > pg.avail() {
			// No room beside the header rows: they are not repeated.
			hdr = 0
		}
	}
	// A page for the rest of the table, with the header rows drawn again.
	next := func(k int, y0 float64) error {
		if hdr > 0 && k >= hdr {
			return pg.openPageRepeat(sec, y0, hdrTop, hdrBottom)
		}
		return pg.openPage(sec, y0, false)
	}
	if plan.header > 0 && pg.has {
		// The header rows are not left alone at the foot of a page.
		if first := groups[plan.header].units[0].bottom; first > pg.limitY()+wordEps {
			if err := pg.openPage(sec, groups[0].top, false); err != nil {
				return err
			}
		}
	}
	for k, g := range groups {
		units := g.units
		start := 0
		for start < len(units) {
			limit := pg.limitY()
			fit := 0
			for j := start; j < len(units) && units[j].bottom <= limit+wordEps; j++ {
				fit++
			}
			if fit == len(units)-start {
				if start == 0 && g.keepNext && need[k] > limit+wordEps && pg.has && need[k]-head[k] <= pg.avail() {
					if err := next(k, units[0].top); err != nil {
						return err
					}
					continue
				}
				pg.place(g.bottom)
				break
			}
			if start == 0 && g.noSplit && pg.has {
				if err := next(k, units[0].top); err != nil {
					return err
				}
				continue
			}
			take := fit
			if take == 0 {
				if pg.has {
					if err := next(k, units[start].top); err != nil {
						return err
					}
					continue
				}
				// Nothing fits on a fresh page: place one unit so pagination
				// always advances.
				take = 1
			}
			pg.place(units[start+take-1].bottom)
			start += take
			if start < len(units) {
				if err := next(k, units[start].top); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// openPageRepeat starts a page that draws the header rows of a table (the range
// [hdrTop, hdrBottom) of the section layout) and then continues from y0.
func (pg *wordPaginator) openPageRepeat(sec *wordLaidSection, y0, hdrTop, hdrBottom float64) error {
	if err := pg.openPage(sec, y0, false); err != nil {
		return err
	}
	page := pg.chunk.page
	rep := &wordChunk{sec: sec, page: page, y0: hdrTop, end: hdrBottom, dest: 0, repeat: true}
	body := pg.chunk
	body.dest = hdrBottom - hdrTop
	// The repeated chunk is drawn first; the body chunk keeps its place after it.
	page.chunks = append([]*wordChunk{rep}, page.chunks...)
	sec.repeats = wordAddRepeat(sec.repeats, rep)
	return nil
}

// wordRepeatGroup is the chunks that draw one range of a section again.
type wordRepeatGroup struct {
	y0, end float64
	chunks  []*wordChunk
}

func wordAddRepeat(groups []wordRepeatGroup, c *wordChunk) []wordRepeatGroup {
	if n := len(groups); n > 0 && groups[n-1].y0 == c.y0 && groups[n-1].end == c.end {
		groups[n-1].chunks = append(groups[n-1].chunks, c)
		return groups
	}
	return append(groups, wordRepeatGroup{y0: c.y0, end: c.end, chunks: []*wordChunk{c}})
}

// repeatsAt returns the chunks that draw the section layout at y again.
func (s *wordLaidSection) repeatsAt(y float64) []*wordChunk {
	i := sort.Search(len(s.repeats), func(i int) bool { return s.repeats[i].y0 > y+wordEps }) - 1
	if i < 0 || y >= s.repeats[i].end-wordEps {
		return nil
	}
	return s.repeats[i].chunks
}
