package docxrender

import (
	"strconv"

	"github.com/mgilbir/forme/layout"
)

// wordLaidNotes are the footnotes of a laid out section: the pool of their
// text, laid out once at the section's width, and the separators.
type wordLaidNotes struct {
	pool *wordLaidSection
	// sepH and contH are the heights of the separator and the continuation
	// separator, with their display lists from the top left of the text area.
	sepH, contH     float64
	sepOps, contOps []layout.Op
}

// resetNotes forgets the placement of a previous pagination.
func (l *wordLaidSection) resetNotes() {
	if l.notes != nil && l.notes.pool != nil {
		l.notes.pool.chunks = nil
	}
	for _, m := range l.marks {
		m.ref.page = -1
	}
}

// wordMarkStyle tags text for wordLocateMarks. forme keeps the computed style
// on the text it lays out and has no other way to name the element a piece of
// text came from; z-index, which has no effect on an element that is not
// positioned, carries the mark's number (from 1) there. Text that is not a mark
// has the initial value, auto.
func wordMarkStyle(n int) string { return "z-index:" + strconv.Itoa(n) }

// wordLocateMarks finds the footnote references in the lines of a block and
// the unit each is in.
func wordLocateMarks(wrapper *layout.Fragment, bi int, lb *wordLaidBlock, marks []*wordMark) {
	stack := []*layout.Fragment{wrapper}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(f.Lines) > 0 {
			content := f.ContentRect()
			for _, l := range f.Lines {
				mid := content.Y.Add(l.Rect.Y).Px() + l.Rect.H.Px()/2
				for _, run := range l.Runs {
					if run.Box == nil {
						continue
					}
					k, err := strconv.Atoi(run.Box.Style.Get("z-index"))
					if err != nil || k < 1 || k > len(marks) || marks[k-1].found {
						continue
					}
					k--
					m := marks[k]
					m.found, m.block, m.unit = true, bi, wordUnitAt(lb.units, mid)
				}
			}
		}
		for i := len(f.Children) - 1; i >= 0; i-- {
			stack = append(stack, f.Children[i])
		}
	}
}

// wordUnitAt is the index of the unit holding y, the last one when none does.
func wordUnitAt(units []wordUnit, y float64) int {
	for i, u := range units {
		if y < u.bottom {
			return i
		}
	}
	return max(len(units)-1, 0)
}

// layoutNotes lays out the notes of a section: their text in a pool, the
// separators, and the units the references are in.
func (r *wordRenderer) layoutNotes(l *wordLaidSection, s *wordSection) error {
	if len(s.marks) == 0 {
		return nil
	}
	n := &wordLaidNotes{}
	var pool *wordLaidSection
	if len(s.pool) > 0 {
		var err error
		if pool, err = r.layoutSection(&wordSection{props: s.props, blocks: s.pool}); err != nil {
			return err
		}
		pool.pool = true
		n.pool = pool
	}
	for _, m := range s.marks {
		if !m.found {
			if err := r.approximate("footnote reference without a mark to locate it"); err != nil {
				return err
			}
			continue
		}
		ref := m.ref
		if pool != nil && ref.n > 0 {
			first, last := pool.blocks[ref.first], pool.blocks[ref.first+ref.n-1]
			ref.top, ref.bottom = first.top, last.bottom
			ref.lines = ref.lines[:0]
			for _, b := range pool.blocks[ref.first : ref.first+ref.n] {
				ref.lines = append(ref.lines, b.units...)
			}
		}
		lb := l.blocks[m.block]
		if lb.unitNotes == nil {
			lb.unitNotes = make([][]*wordNoteRef, len(lb.units))
		}
		lb.unitNotes[m.unit] = append(lb.unitNotes[m.unit], ref)
	}
	src, err := r.loadNotes(0)
	if err != nil {
		return err
	}
	sep := func(node *wordNode) (float64, []layout.Op, error) {
		if node == nil {
			return 0, nil, nil
		}
		blocks, err := r.noteBlocks(node, &wordNoteCtx{sep: true})
		if err != nil || len(blocks) == 0 {
			return 0, nil, err
		}
		sl, err := r.layoutSection(&wordSection{props: s.props, blocks: blocks})
		if err != nil {
			return 0, nil, err
		}
		return sl.blocks[len(sl.blocks)-1].bottom, sl.ops, nil
	}
	if n.sepH, n.sepOps, err = sep(src.separator); err != nil {
		return err
	}
	if n.contH, n.contOps, err = sep(src.continuation); err != nil {
		return err
	}
	l.notes = n
	return nil
}

// wordNotePart is a run of the lines of a note placed on one page.
type wordNotePart struct {
	sec      *wordLaidSection
	ref      *wordNoteRef
	from, to int
}

// top and bottom are the part's extent in pool pixels: a part that starts the
// note includes the space above its first line, and one that ends it the space
// below its last; the gap at a cut is not drawn.
func (p wordNotePart) top() float64 {
	if p.from == 0 {
		return p.ref.top
	}
	return p.ref.lines[p.from].top
}

func (p wordNotePart) bottom() float64 {
	if p.to >= len(p.ref.lines) {
		return p.ref.bottom
	}
	return p.ref.lines[p.to-1].bottom
}

func (p wordNotePart) height() float64 {
	if p.to <= p.from {
		return 0
	}
	return p.bottom() - p.top()
}

// fit is the most lines of the part, short of all of them, that fit in room.
func (p wordNotePart) fit(room float64) int {
	k := 0
	for t := p.from + 1; t < p.to; t++ {
		q := p
		q.to = t
		if q.height() > room+wordEps {
			break
		}
		k = t - p.from
	}
	return k
}

// wordPageNotes is the footnote area of a page.
type wordPageNotes struct {
	// h is the area's height, separator included, and any says it exists.
	h   float64
	any bool
	// cont says it opens with the continuation separator.
	cont  bool
	parts []wordNotePart
	// sec is the section whose separator is drawn.
	sec *wordLaidSection
	// sepOps are the separator's display list at its position in the page.
	sepOps     []layout.Op
	sepX, sepY float64
}

// fitNotes places the notes whose references are in a unit that ends pb below
// the top of the text area, in the note area st. A note that does not fit whole
// is cut at a line and its rest carried to the next page; ok is false when not
// even the first line of the first note that does not fit would, unless force
// (a unit that has to be placed on a page whatever it holds) carries the whole
// note. With apply the parts and the carry are recorded; otherwise only st's
// height is.
func (pg *wordPaginator) fitNotes(st *wordPageNotes, sec *wordLaidSection, refs []*wordNoteRef, pb float64, apply, force bool) (ok, partial bool) {
	page := pg.chunk.page
	sepH := 0.0
	if !st.any && sec.notes != nil {
		sepH = sec.notes.sepH
	}
	room := page.bodyH() - pb - st.h - sepH
	var placed, carried []wordNotePart
	carrying := false
	for _, ref := range refs {
		n := len(ref.lines)
		if n == 0 {
			continue
		}
		whole := wordNotePart{sec, ref, 0, n}
		if carrying {
			carried = append(carried, whole)
			continue
		}
		if h := whole.height(); h <= room+wordEps {
			placed = append(placed, whole)
			room -= h
			continue
		}
		k := whole.fit(room)
		if k == 0 {
			if !force {
				return false, false
			}
			carrying, partial = true, true
			carried = append(carried, whole)
			continue
		}
		placed = append(placed, wordNotePart{sec, ref, 0, k})
		carried = append(carried, wordNotePart{sec, ref, k, n})
		carrying, partial = true, true
	}
	if len(placed) > 0 {
		if !st.any {
			st.any, st.h, st.sec = true, st.h+sepH, sec
		}
		for _, p := range placed {
			st.h += p.height()
		}
	}
	if apply {
		pg.carried = pg.carried || len(carried) > 0
		st.parts = append(st.parts, placed...)
		pg.carry = append(pg.carry, carried...)
		for _, ref := range refs {
			ref.page = len(pg.pages) - 1
		}
	}
	return true, partial
}

// commitNotes records the notes of the units start to start+take of a block as
// placed on the current page.
func (pg *wordPaginator) commitNotes(sec *wordLaidSection, b *wordLaidBlock, start, take int, force bool) {
	if b.unitNotes == nil {
		return
	}
	page := pg.chunk.page
	for j := start; j < start+take; j++ {
		if len(b.unitNotes[j]) == 0 {
			continue
		}
		pb := pg.chunk.dest + b.units[j].bottom - pg.chunk.y0
		pg.fitNotes(&page.notes, sec, b.unitNotes[j], pb, true, force)
	}
}

// startNotes opens a page's note area with the notes carried from the page
// before, under the continuation separator. Carried text takes at most half
// the text area; the rest continues on the next page.
func (pg *wordPaginator) startNotes(page *wordPage) {
	if len(pg.carry) == 0 {
		return
	}
	carry := pg.carry
	pg.carry = nil
	st := &page.notes
	sec := carry[0].sec
	st.any, st.cont, st.sec = true, true, sec
	st.h = sec.notes.contH
	limit := page.bodyH() / 2
	for i, p := range carry {
		if h := p.height(); st.h+h <= limit+wordEps {
			st.parts = append(st.parts, p)
			st.h += h
			continue
		}
		k := p.fit(limit - st.h)
		if k == 0 && len(st.parts) == 0 {
			k = 1
		}
		if k > 0 && k < p.to-p.from {
			placed := p
			placed.to = p.from + k
			st.parts = append(st.parts, placed)
			st.h += placed.height()
			p.from += k
		}
		pg.carry = append(pg.carry, p)
		pg.carry = append(pg.carry, carry[i+1:]...)
		pg.carryCapped = true
		return
	}
}

// finishNotes gives the parts of the note areas their place on the pages: the
// pool of a section is drawn from the chunks made here.
func (pg *wordPaginator) finishNotes() error {
	for _, page := range pg.pages {
		st := &page.notes
		if !st.any {
			continue
		}
		n := st.sec.notes
		sepH, ops := n.sepH, n.sepOps
		if st.cont {
			sepH, ops = n.contH, n.contOps
		}
		top := page.bodyH() - st.h
		st.sepOps, st.sepX, st.sepY = ops, st.sec.props.left, page.bodyTop+top
		off := top + sepH
		for _, p := range st.parts {
			pool := p.sec.notes.pool
			pool.chunks = append(pool.chunks, &wordChunk{sec: pool, page: page, y0: p.top(), end: p.bottom(), dest: off})
			off += p.height()
		}
	}
	return nil
}

func init() {
	wordPageDecorators = append(wordPageDecorators, func(r *wordRenderer, _ []*wordLaidSection, pages []*wordPage) error {
		for _, page := range pages {
			if len(page.notes.sepOps) == 0 {
				continue
			}
			if err := page.place(page.notes.sepOps, page.notes.sepX, page.notes.sepY); err != nil {
				return err
			}
		}
		return r.ctx.Err()
	})
}
