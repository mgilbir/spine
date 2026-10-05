package docxrender

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/render"
)

// Footnotes and endnotes.
//
// Translation. The body is translated as before; a w:footnoteReference draws
// the note's mark (its number in the section's format, in the run's own
// formatting, which is how Word draws it) inside a span the layout can locate,
// and records the reference. When a section is closed the text of the notes it
// referenced is translated, by the body's paragraph code, into a pool of
// blocks that is laid out once at the section's width (render_notes_layout.go),
// so each note's height and lines are known. Endnotes become ordinary blocks
// at the end of the document or of their section.
//
// Pagination (render_paginate.go) puts a note on the page of the line that
// holds its reference: placing a line also places its notes at the bottom of
// the text area, under the separator, and the line fits only if both fit. A note
// that does not fit whole continues on the next page, under the continuation
// separator, as Word does.
//
// Numbers. A mark's number is known while the body is translated, except where
// numbering restarts on every page, which depends on pagination: those numbers
// are assumed (as if numbering were continuous), checked after pagination, and
// the document is translated again with the right ones, a bounded number of
// times.

// wordNoteRounds bounds how often the text is translated again for footnote
// numbers that depend on pagination.
const wordNoteRounds = 3

// wordNotePr is the numbering and position of one kind of note
// (w:footnotePr, w:endnotePr).
type wordNotePr struct {
	numFmt  string
	start   int
	restart string
	pos     string
}

func wordDefaultNotePr(endnote bool) wordNotePr {
	if endnote {
		return wordNotePr{numFmt: "lowerRoman", start: 1, restart: "continuous", pos: "docEnd"}
	}
	return wordNotePr{numFmt: "decimal", start: 1, restart: "continuous", pos: "pageBottom"}
}

// parseNotePr reads a w:footnotePr or w:endnotePr over the properties it
// overrides.
func parseNotePr(n *wordNode, base wordNotePr, endnote bool) (wordNotePr, error) {
	pr := base
	for _, c := range n.children {
		if c.space != nsW {
			continue
		}
		switch c.name {
		case "numFmt":
			pr.numFmt = c.val()
		case "numStart":
			v, err := strconv.Atoi(c.val())
			if err != nil || v < -(1<<30) || v > 1<<30 {
				return pr, fmt.Errorf("%w: w:numStart", render.ErrInvalid)
			}
			pr.start = v
		case "numRestart":
			switch v := c.val(); v {
			case "continuous", "eachSect":
				pr.restart = v
			case "eachPage":
				if endnote {
					return pr, fmt.Errorf("%w: w:numRestart", render.ErrInvalid)
				}
				pr.restart = v
			default:
				return pr, fmt.Errorf("%w: w:numRestart", render.ErrInvalid)
			}
		case "pos":
			switch v := c.val(); v {
			case "sectEnd", "docEnd":
				pr.pos = v
			case "pageBottom", "beneathText":
				if endnote {
					return pr, fmt.Errorf("%w: w:pos", render.ErrInvalid)
				}
				pr.pos = v
			default:
				return pr, fmt.Errorf("%w: w:pos", render.ErrInvalid)
			}
		}
	}
	return pr, nil
}

// wordSectNotePr are the note properties of one section.
type wordSectNotePr struct{ foot, end wordNotePr }

// wordNoteRef is one reference to a note and, once laid out, the note.
type wordNoteRef struct {
	id      string
	endnote bool
	// custom is a reference whose mark the following text supplies
	// (w:customMarkFollows).
	custom bool
	num    int
	text   string
	// page is the physical page (0-based) the reference is on, -1 before
	// pagination.
	page int
	// field is set instead of a note for a page number field in the body.
	field *wordFieldCheck
	// first and n are the note's blocks in the section's pool.
	first, n int
	// The note laid out: its extent in pool pixels and its lines.
	top, bottom float64
	lines       []wordUnit
}

// wordMark is something the layout must locate in a section's lines: a
// footnote reference, whose note goes to the page of its line.
type wordMark struct {
	ref *wordNoteRef
	// found, block and unit say where layout put it.
	found       bool
	block, unit int
}

// wordNoteCtx is set while the text of a note is translated.
type wordNoteCtx struct {
	// text is the note's mark, which a w:footnoteRef draws.
	text string
	// sep is the translation of a separator note, where w:separator and
	// w:continuationSeparator draw their rule.
	sep bool
}

// wordNoteSource is a footnotes or endnotes part.
type wordNoteSource struct {
	byID map[string]*wordNode
	// separator and continuation are the separator notes, nil when absent.
	separator, continuation *wordNode
	// notice says the part has a continuation notice with text, which Word
	// draws where a note continues on the next page.
	notice bool
}

// wordHasText reports whether any w:t under n has text.
func wordHasText(n *wordNode) bool {
	stack := []*wordNode{n}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if c.is("t") && strings.TrimSpace(c.text) != "" {
			return true
		}
		stack = append(stack, c.children...)
	}
	return false
}

// wordNoteCounter numbers the references of one kind of note.
type wordNoteCounter struct {
	n       int
	started bool
	sec     int
}

// next is the number of the next reference, in section sec.
func (c *wordNoteCounter) next(pr wordNotePr, sec int) int {
	if !c.started || (pr.restart == "eachSect" && sec != c.sec) {
		c.n = pr.start
	} else {
		c.n++
	}
	c.started, c.sec = true, sec
	return c.n
}

// wordNotes is the notes state of one preparation.
type wordNotes struct {
	// defaults come from the settings part.
	foot, end wordNotePr
	// source reads the parts: footnotes then endnotes.
	source [2]func() ([]byte, error)
	parts  [2]*wordNoteSource
	// sects are the sections' note properties, from a scan of the body.
	sects []wordSectNotePr
	// Translation state, reset for each pass.
	counters [2]wordNoteCounter
	marks    []*wordMark
	// endRefs are the endnote references by section.
	endRefs [][]*wordNoteRef
	// nums are the footnote numbers drawn in order, and assume those a pass
	// is to draw for pages that restart numbering.
	nums, assume []int
	// eachPage is set when numbering restarts on every page somewhere.
	eachPage bool
}

// newWordNotes returns the notes state with the default properties.
func newWordNotes() *wordNotes {
	return &wordNotes{foot: wordDefaultNotePr(false), end: wordDefaultNotePr(true)}
}

// reset clears the translation state for a new pass.
func (n *wordNotes) reset() {
	n.counters = [2]wordNoteCounter{}
	n.marks, n.endRefs, n.nums = nil, nil, nil
}

// secPr is the properties of section i.
func (n *wordNotes) secPr(i int) wordSectNotePr {
	if len(n.sects) == 0 {
		return wordSectNotePr{n.foot, n.end}
	}
	if i >= len(n.sects) {
		i = len(n.sects) - 1
	}
	return n.sects[i]
}

// scan collects the section properties of the body in order and reads their
// note properties.
func (r *wordRenderer) scanNotes(body *wordNode) error {
	var sects []*wordNode
	var walk func(n *wordNode, depth int)
	walk = func(n *wordNode, depth int) {
		if depth > 64 {
			return
		}
		for _, c := range n.children {
			switch {
			case c.is("p"):
				if sp := c.child("pPr").child("sectPr"); sp != nil {
					sects = append(sects, sp)
				}
			case c.is("sdt"):
				if content := c.child("sdtContent"); content != nil {
					walk(content, depth+1)
				}
			case c.is("customXml"):
				walk(c, depth+1)
			}
		}
	}
	walk(body, 0)
	if sp := body.child("sectPr"); sp != nil {
		sects = append(sects, sp)
	}
	n := r.notes
	n.sects = n.sects[:0]
	for _, sp := range sects {
		s := wordSectNotePr{n.foot, n.end}
		var err error
		if c := sp.child("footnotePr"); c != nil {
			if s.foot, err = parseNotePr(c, n.foot, false); err != nil {
				return err
			}
		}
		if c := sp.child("endnotePr"); c != nil {
			if s.end, err = parseNotePr(c, n.end, true); err != nil {
				return err
			}
		}
		if s.foot.restart == "eachPage" {
			n.eachPage = true
		}
		n.sects = append(n.sects, s)
	}
	return nil
}

// load reads and indexes a notes part (0 footnotes, 1 endnotes) on first use.
func (r *wordRenderer) loadNotes(kind int) (*wordNoteSource, error) {
	n := r.notes
	if n.parts[kind] != nil {
		return n.parts[kind], nil
	}
	src := &wordNoteSource{byID: map[string]*wordNode{}}
	n.parts[kind] = src
	if n.source[kind] == nil {
		return src, nil
	}
	data, err := n.source[kind]()
	if err != nil || data == nil {
		return src, err
	}
	name, child := "footnotes", "footnote"
	if kind == 1 {
		name, child = "endnotes", "endnote"
	}
	root, err := wordRenderParse(r.ctx, data, r.budget, nsW, name)
	if err != nil {
		return nil, fmt.Errorf("docx: %s part: %w", name, err)
	}
	for _, c := range root.children {
		if !c.is(child) {
			continue
		}
		switch c.attrOr("type") {
		case "separator":
			if src.separator == nil {
				src.separator = c
			}
		case "continuationSeparator":
			if src.continuation == nil {
				src.continuation = c
			}
		case "continuationNotice":
			src.notice = src.notice || wordHasText(c)
		case "", "normal":
			if id := c.attrOr("id"); id != "" && src.byID[id] == nil {
				src.byID[id] = c
			}
		}
	}
	return src, nil
}

// noteBlocks translates the content of a note into blocks.
func (r *wordRenderer) noteBlocks(n *wordNode, ctx *wordNoteCtx) ([]*wordBlock, error) {
	nf := &wordFlow{r: r, note: ctx}
	if err := nf.blocksOf(n); err != nil {
		return nil, err
	}
	var out []*wordBlock
	for _, s := range nf.secs {
		out = append(out, s.blocks...)
	}
	return append(out, nf.cur...), nil
}

// noteReference translates a w:footnoteReference or w:endnoteReference: the
// mark is drawn and the reference recorded.
func (rn *wordRun) noteReference(n *wordNode, endnote bool) error {
	f, r := rn.p.f, rn.p.r
	if rn.hidden() {
		return nil
	}
	if f.hf != nil || f.note != nil {
		return r.leaveOut("notes in headers, footers and notes")
	}
	notes := r.notes
	kind := 0
	if endnote {
		kind = 1
	}
	sec := len(f.secs)
	pr := notes.secPr(sec).foot
	if endnote {
		pr = notes.secPr(sec).end
	}
	if !endnote && pr.pos != "pageBottom" {
		if err := r.approximate("footnotes placed other than at the bottom of the page"); err != nil {
			return err
		}
	}
	ref := &wordNoteRef{id: n.attrOr("id"), endnote: endnote, page: -1}
	if v := n.attrOr("customMarkFollows"); v == "1" || v == "true" || v == "on" {
		ref.custom = true
	}
	if !ref.custom {
		switch {
		case !endnote && pr.restart == "eachPage":
			notes.eachPage = true
			if i := len(notes.nums); i < len(notes.assume) {
				ref.num = notes.assume[i]
			} else {
				ref.num = notes.counters[kind].next(wordNotePr{start: pr.start, restart: "continuous"}, sec)
			}
		default:
			ref.num = notes.counters[kind].next(pr, sec)
		}
		var exact bool
		ref.text, exact = wordNumberText(ref.num, pr.numFmt)
		if !exact {
			if err := r.approximate("note number format"); err != nil {
				return err
			}
		}
	}
	if endnote {
		for len(notes.endRefs) <= sec {
			notes.endRefs = append(notes.endRefs, nil)
		}
		notes.endRefs[sec] = append(notes.endRefs[sec], ref)
	} else {
		if len(notes.marks) >= r.limits.MaxOperations {
			return render.ErrLimit
		}
		if !ref.custom {
			notes.nums = append(notes.nums, ref.num)
		}
		notes.marks = append(notes.marks, &wordMark{ref: ref})
		rn.mark = len(notes.marks)
	}
	if ref.custom {
		// The text that follows in the run is the mark: the first of it is
		// what locates the reference.
		if endnote {
			rn.mark = 0
		}
		return nil
	}
	err := rn.emitString(ref.text)
	rn.mark = 0
	return err
}

// closeNotes ends a section's notes: the footnotes it referenced are
// translated into its pool.
func (r *wordRenderer) closeNotes(ws *wordSection) error {
	notes := r.notes
	ws.marks, notes.marks = notes.marks, nil
	for _, m := range ws.marks {
		if m.ref.field != nil {
			continue
		}
		src, err := r.loadNotes(0)
		if err != nil {
			return err
		}
		node := src.byID[m.ref.id]
		if node == nil {
			if err = r.leaveOut("footnote that the document does not have"); err != nil {
				return err
			}
			continue
		}
		blocks, err := r.noteBlocks(node, &wordNoteCtx{text: m.ref.text})
		if err != nil {
			return err
		}
		m.ref.first, m.ref.n = len(ws.pool), len(blocks)
		ws.pool = append(ws.pool, blocks...)
	}
	return nil
}

// placeEndnotes appends the endnotes to the sections they are printed at: the
// end of their section, or the end of the document.
func (r *wordRenderer) placeEndnotes(secs []*wordSection) error {
	notes := r.notes
	var doc []*wordNoteRef
	for i, s := range secs {
		var refs []*wordNoteRef
		if i < len(notes.endRefs) {
			refs = notes.endRefs[i]
		}
		if len(refs) == 0 {
			continue
		}
		if notes.secPr(i).end.pos == "sectEnd" {
			if err := r.appendEndnotes(s, refs); err != nil {
				return err
			}
			continue
		}
		doc = append(doc, refs...)
	}
	if len(doc) == 0 {
		return nil
	}
	return r.appendEndnotes(secs[len(secs)-1], doc)
}

// appendEndnotes adds a separator and the notes to a section's flow.
func (r *wordRenderer) appendEndnotes(s *wordSection, refs []*wordNoteRef) error {
	src, err := r.loadNotes(1)
	if err != nil {
		return err
	}
	var blocks []*wordBlock
	if src.separator != nil {
		if blocks, err = r.noteBlocks(src.separator, &wordNoteCtx{sep: true}); err != nil {
			return err
		}
	}
	for _, ref := range refs {
		node := src.byID[ref.id]
		if node == nil {
			if err = r.leaveOut("endnote that the document does not have"); err != nil {
				return err
			}
			continue
		}
		nb, err := r.noteBlocks(node, &wordNoteCtx{text: ref.text})
		if err != nil {
			return err
		}
		blocks = append(blocks, nb...)
	}
	if len(blocks) == 0 {
		return nil
	}
	if !s.hasEndnotes {
		s.hasEndnotes, s.endnotesFrom = true, len(s.blocks)
	}
	s.blocks = append(s.blocks, blocks...)
	return r.charge(len(blocks))
}

// pageNoteNumbers are the footnote numbers pagination gives, in order.
func (r *wordRenderer) pageNoteNumbers(laid []*wordLaidSection) []int {
	notes := r.notes
	var out []int
	var c wordNoteCounter
	lastPage := -1
	for _, s := range laid {
		pr := notes.secPr(s.props.idx).foot
		for _, m := range s.marks {
			if m.ref.custom || m.ref.field != nil {
				continue
			}
			page := m.ref.page
			if page < 0 {
				page = lastPage
			}
			restart := !c.started || (pr.restart == "eachSect" && s.props.idx != c.sec) || (pr.restart == "eachPage" && page != lastPage)
			n := c.n + 1
			if restart {
				n = pr.start
			}
			c.n, c.started, c.sec = n, true, s.props.idx
			lastPage = page
			out = append(out, n)
		}
	}
	return out
}

// verifyFields compares the saved result of each page number field in the
// body with the value the field has on the page it landed on. Word saves
// the result it computed, so a document that was last laid out as it is now
// has them equal; one that is stale or laid out differently is drawn with the
// saved result and reported.
func (r *wordRenderer) verifyFields(laid []*wordLaidSection, pages []*wordPage) error {
	env := measureEnv(laid, pages)
	for _, s := range laid {
		for _, m := range s.marks {
			f := m.ref.field
			if f == nil {
				continue
			}
			if !m.found || m.ref.page < 0 || m.ref.page >= len(pages) {
				if err := r.approximate("page number field without a saved result to check"); err != nil {
					return err
				}
				continue
			}
			ctx := wordHFCtx{vals: wordPageVals{page: pages[m.ref.page].num, pageFmt: s.props.pgFmt, total: env.total, secPages: env.secPages(s.props.idx), sec: s.props.idx + 1}}
			want, _ := ctx.fieldValue(f.info)
			if strings.TrimSpace(f.cached) != want {
				if err := r.approximate("page number field whose saved result differs from the page it is on"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func init() {
	wordRegisterRun("footnoteReference", func(rn *wordRun, n *wordNode) error { return rn.noteReference(n, false) })
	wordRegisterRun("endnoteReference", func(rn *wordRun, n *wordNode) error { return rn.noteReference(n, true) })
	for _, k := range []string{"footnoteRef", "endnoteRef"} {
		what := "footnotes"
		if k == "endnoteRef" {
			what = "endnotes"
		}
		wordRegisterRun(k, func(rn *wordRun, _ *wordNode) error {
			f := rn.p.f
			if rn.hidden() {
				return nil
			}
			if f.note == nil || f.note.sep {
				return rn.p.r.leaveOut(what)
			}
			return rn.emitString(f.note.text)
		})
	}
	for _, k := range []string{"separator", "continuationSeparator"} {
		cont := k == "continuationSeparator"
		wordRegisterRun(k, func(rn *wordRun, _ *wordNode) error {
			f := rn.p.f
			if rn.hidden() {
				return nil
			}
			if f.note == nil || !f.note.sep {
				return rn.p.r.leaveOut("footnotes")
			}
			return rn.rule(cont)
		})
	}
}

// rule draws a note separator: a rule 144 pt long from the start of the line,
// as Word draws it, or the whole width for a continuation separator. Both
// are 0.75 pt thick.
func (rn *wordRun) rule(full bool) error {
	if err := rn.closeSpan(); err != nil {
		return err
	}
	size := 12.0
	if rn.rp.size.set {
		size = float64(rn.rp.size.v) * 2 / 3
	}
	var c wordCSS
	c.add("display", "inline-block")
	if full {
		c.add("width", "100%")
	} else {
		c.px("width", 192)
		c.add("max-width", "100%")
	}
	c.px("height", 1)
	c.add("background-color", "rgb(0,0,0)")
	c.px("vertical-align", size*0.3)
	if err := rn.p.r.charge(1); err != nil {
		return err
	}
	rn.p.sb.WriteString(`<span style="` + c.String() + `"></span>`)
	rn.p.hasContent = true
	return nil
}
