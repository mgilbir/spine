package docxrender

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/render"
)

// wordPara is the translation state of one w:p.
//
// A paragraph becomes one block, or several when a manual page break splits
// it: the text after the break is the continuation of the same paragraph on a
// new page, without a first-line indent or space before.
type wordPara struct {
	f *wordFlow
	r *wordRenderer
	// ppr is the effective paragraph formatting.
	ppr wordPPr
	// paraRPr is the run formatting a run inherits: document defaults and the
	// paragraph style.
	paraRPr wordRPr
	// mark is the paragraph mark's effective run formatting.
	mark wordRPr
	// styleID names the paragraph style in effect.
	styleID string

	blocks []*wordBlock
	sb     strings.Builder
	// lead is markup that opens the paragraph's block (floated pictures),
	// and pics the anchored pictures of the block being built.
	lead       strings.Builder
	pics       []*wordPicture
	hasContent bool
	cont       bool
	sawLineBr  bool
	// brAt is where the markup stood after the block's last line break, -1
	// when it has none; hasText says the block has text.
	brAt    int
	hasText bool
	// place is the line placement of the block's lines, from its tallest run
	// (tallest is that run's line height); misplaced says a run would need
	// another placement, or one layout cannot give.
	place     string
	tallest   float64
	misplaced bool
	// inlinePics says the block has an inline picture.
	inlinePics bool
	// tabMode says the paragraph sets its own tabs (render_tabs.go): tabs and
	// manual breaks become tokens, collected in tabItems for the current block.
	tabMode  bool
	tabItems []wordTabItem
	stops    []wordTab
	// list is the paragraph's numbering, drawn by startList
	// (render_numbering.go); marker is set when the marker is positioned by
	// its width.
	list   *wordListMark
	marker *wordMarker
}

// wordParagraphStarts are run before a paragraph's content is translated, in
// order. The numbering translator registers the one that writes the list
// marker into p.
var wordParagraphStarts []func(p *wordPara) error

// paragraph translates a w:p block.
func (f *wordFlow) paragraph(n *wordNode) error {
	r := f.r
	var direct wordPPr
	pPr := n.child("pPr")
	direct, err := r.parsePPr(pPr)
	if err != nil {
		return err
	}
	lv, err := r.styles.paragraphIn(direct.style, f.cellBase())
	if err != nil {
		return err
	}
	styleID := direct.style
	if styleID == "" || r.styles.byID[styleID] == nil {
		styleID = r.styles.defPara
	}
	ppr, list, err := r.listProps(direct, lv, styleID)
	if err != nil {
		return err
	}
	p := &wordPara{f: f, r: r, ppr: ppr, paraRPr: lv.rpr, list: list, styleID: styleID, brAt: -1}
	p.mark = p.ppr.mark.over(lv.rpr)
	if err = r.issues(p.ppr.issues); err != nil {
		return err
	}
	p.stops = wordEffectiveTabs(p.ppr.tabs.v)
	p.tabMode = wordNeedsTabLayout(p.stops, p.ppr.indLeft.v, r.defaultTab)
	for _, s := range p.stops {
		if s.val == "bar" {
			if err = r.approximate("bar tab stops"); err != nil {
				return err
			}
			break
		}
	}
	for _, hook := range wordParagraphStarts {
		if err = hook(p); err != nil {
			return err
		}
	}
	if err = p.children(n); err != nil {
		return err
	}
	if p.sawLineBr && (p.ppr.jc.v == "both" || p.ppr.jc.v == "distribute") {
		// Word stretches the line before a manual line break; the layout
		// engine sets it at its natural width.
		if err = r.approximate("justified line before a manual line break"); err != nil {
			return err
		}
	}
	// A page break that ends the paragraph keeps the mark with it, as Word's
	// PDF output shows: no empty line opens the next page, whose first block
	// is the next one. A cell's flow has no pages of its own.
	breakEnds := p.cont && !p.hasContent && f.cell == nil
	if !breakEnds {
		if err = p.finishBlock(true, nil); err != nil {
			return err
		}
	}
	last := len(p.blocks) - 1
	for i, b := range p.blocks {
		if i == 0 {
			b.before, b.beforeAuto = p.spacing(true)
		}
		if i == last {
			b.after, b.afterAuto = p.spacing(false)
		}
		if err = f.add(b); err != nil {
			return err
		}
	}
	f.breakPending = f.breakPending || breakEnds
	if pPr != nil && f.cell == nil {
		// Section properties in a paragraph of a table cell end nothing.
		if sp := pPr.child("sectPr"); sp != nil {
			return f.closeSection(sp)
		}
	}
	return nil
}

// wordAutoSpacing is the space HTML-style automatic spacing stands for: 14 pt.
const wordAutoSpacing = 14 * 96.0 / 72

// spacing returns the space before or after the paragraph in pixels.
func (p *wordPara) spacing(before bool) (float64, bool) {
	v, auto := p.ppr.after, p.ppr.afterAuto
	if before {
		v, auto = p.ppr.before, p.ppr.beforeAuto
	}
	if auto.v {
		return wordAutoSpacing, true
	}
	return v.v, false
}

// children translates the content children of a paragraph or of a container
// inside one.
func (p *wordPara) children(n *wordNode) error {
	for _, c := range n.children {
		if err := p.r.ctx.Err(); err != nil {
			return err
		}
		if c.space == nsW && c.name == "pPr" {
			continue
		}
		fn := wordInlineKinds[wordKey(c)]
		if fn == nil {
			if err := p.r.leaveOut("paragraph element " + wordKey(c)); err != nil {
				return err
			}
			continue
		}
		if err := fn(p, c); err != nil {
			return err
		}
	}
	return nil
}

// wordInlineFunc translates one paragraph-level element.
type wordInlineFunc func(p *wordPara, n *wordNode) error

var wordInlineKinds = map[string]wordInlineFunc{}

// wordRegisterInline registers the translator of a paragraph-level element.
func wordRegisterInline(key string, fn wordInlineFunc) {
	if _, dup := wordInlineKinds[key]; dup {
		panic("docxrender: duplicate inline kind " + key)
	}
	wordInlineKinds[key] = fn
}

func init() {
	wordRegisterInline("r", (*wordPara).run)
	for _, k := range []string{"hyperlink", "smartTag", "customXml"} {
		wordRegisterInline(k, func(p *wordPara, n *wordNode) error { return p.children(n) })
	}
	wordRegisterInline("fldSimple", (*wordPara).simpleField)
	wordRegisterInline("sdt", func(p *wordPara, n *wordNode) error {
		if c := n.child("sdtContent"); c != nil {
			return p.children(c)
		}
		return nil
	})
	// Tracked changes: the final text is drawn (insertions kept, deletions
	// dropped), which is an approximation of Word's markup view.
	for _, k := range []string{"ins", "moveTo"} {
		wordRegisterInline(k, func(p *wordPara, n *wordNode) error {
			if err := p.r.approximate("tracked insertions"); err != nil {
				return err
			}
			return p.children(n)
		})
	}
	for _, k := range []string{"del", "moveFrom"} {
		wordRegisterInline(k, func(p *wordPara, _ *wordNode) error { return p.r.approximate("tracked deletions") })
	}
	for _, k := range []string{"bookmarkStart", "bookmarkEnd", "proofErr", "permStart", "permEnd", "moveFromRangeStart", "moveFromRangeEnd", "moveToRangeStart", "moveToRangeEnd"} {
		wordRegisterInline(k, func(*wordPara, *wordNode) error { return nil })
	}
}

// simpleField translates a w:fldSimple: its content is the result.
func (p *wordPara) simpleField(n *wordNode) error {
	f := p.f
	if len(f.fields) >= 64 {
		return fmt.Errorf("%w: nested fields", render.ErrLimit)
	}
	instr, _ := n.attr("instr")
	if len(instr) > wordMaxInstr {
		instr = instr[:wordMaxInstr]
	}
	s := wordFieldState{instr: instr}
	if err := f.startFieldResult(&s); err != nil {
		return err
	}
	s.result = true
	f.fields = append(f.fields, s)
	err := p.children(n)
	s = f.fields[len(f.fields)-1]
	f.fields = f.fields[:len(f.fields)-1]
	if err != nil {
		return err
	}
	if s.subst && !s.done && f.visible() {
		// No result runs: draw the value in the paragraph's formatting.
		eff, err := p.r.styles.runProps(p.paraRPr, wordRPr{})
		if err != nil {
			return err
		}
		if !eff.vanish.v {
			rn := &wordRun{p: p, rp: eff, cur: -1}
			if err = rn.emitString(s.text); err != nil {
				return err
			}
			return rn.closeSpan()
		}
	}
	return nil
}

// run translates a w:r.
func (p *wordPara) run(n *wordNode) error {
	r := p.r
	direct, err := r.parseRPr(n.child("rPr"))
	if err != nil {
		return err
	}
	eff, err := r.styles.runProps(p.paraRPr, direct)
	if err != nil {
		return err
	}
	rn := &wordRun{p: p, rp: eff, cur: -1}
	reported := false
	for _, c := range n.children {
		if err = r.ctx.Err(); err != nil {
			return err
		}
		if c.is("rPr") {
			continue
		}
		fn := wordRunKinds[wordKey(c)]
		if fn == nil {
			if p.f.visible() {
				if err = r.leaveOut("run element " + wordKey(c)); err != nil {
					return err
				}
			}
			continue
		}
		if !reported && p.f.visible() && !eff.vanish.v && wordRunDraws(c) {
			reported = true
			if err = r.issues(eff.issues); err != nil {
				return err
			}
		}
		if err = fn(rn, c); err != nil {
			return err
		}
	}
	return rn.closeSpan()
}

// wordRunDraws reports whether a run child puts ink or space on the page.
func wordRunDraws(c *wordNode) bool {
	if c.space != nsW {
		return false
	}
	switch c.name {
	case "t", "tab", "br", "cr", "noBreakHyphen", "softHyphen":
		return true
	}
	return false
}

// wordRun is one run being emitted: its effective formatting and the span
// that is open.
type wordRun struct {
	p   *wordPara
	rp  wordRPr
	cur int // font slot of the open span, or -1
	// mark, when positive, tags the next text drawn so the layout can find
	// its line (a note reference's mark): see wordMarkStyle.
	mark int
	// closing is the markup that ends the open span.
	closing string
	spans   [wordSlots]wordSpanStyle
}

// wordSpanStyle caches a slot's opening and closing markup.
type wordSpanStyle struct {
	ok          bool
	open, close string
	nodes       int
}

// hidden reports whether the run's text is not drawn.
func (rn *wordRun) hidden() bool { return rn.rp.vanish.v || !rn.p.f.visible() }

type wordRunFunc func(rn *wordRun, n *wordNode) error

var wordRunKinds = map[string]wordRunFunc{}

// wordRegisterRun registers the translator of a w:r child element.
func wordRegisterRun(key string, fn wordRunFunc) {
	if _, dup := wordRunKinds[key]; dup {
		panic("docxrender: duplicate run kind " + key)
	}
	wordRunKinds[key] = fn
}

func init() {
	wordRegisterRun("t", (*wordRun).text)
	wordRegisterRun("tab", func(rn *wordRun, _ *wordNode) error {
		if rn.hidden() {
			return nil
		}
		if rn.p.tabMode {
			return rn.tabToken('T')
		}
		return rn.put(rn.slotForPiece(), "\t", false)
	})
	wordRegisterRun("br", (*wordRun).lineBreak)
	wordRegisterRun("cr", func(rn *wordRun, _ *wordNode) error { return rn.softBreak() })
	wordRegisterRun("noBreakHyphen", func(rn *wordRun, _ *wordNode) error {
		if rn.hidden() {
			return nil
		}
		return rn.put(rn.slotForPiece(), "\u2011", false)
	})
	wordRegisterRun("softHyphen", func(rn *wordRun, _ *wordNode) error {
		if rn.hidden() {
			return nil
		}
		return rn.put(rn.slotForPiece(), "\u00AD", false)
	})
	wordRegisterRun("fldChar", (*wordRun).fieldChar)
	for _, k := range []string{"delText", "lastRenderedPageBreak", "fldData"} {
		wordRegisterRun(k, func(*wordRun, *wordNode) error { return nil })
	}
	wordRegisterRun("instrText", func(rn *wordRun, n *wordNode) error {
		f := rn.p.f
		if len(f.fields) == 0 {
			return nil
		}
		if s := &f.fields[len(f.fields)-1]; !s.result && len(s.instr) < wordMaxInstr {
			s.instr += n.text
		}
		return nil
	})
}

// slotForPiece is the font slot for a character that has no script, such as a
// tab: the slot of the open span, or ASCII.
func (rn *wordRun) slotForPiece() int {
	if rn.cur >= 0 {
		return rn.cur
	}
	return wordSlotASCII
}

// text appends a w:t.
func (rn *wordRun) text(n *wordNode) error {
	if rn.hidden() {
		return nil
	}
	if s := rn.p.f.substField(); s != nil {
		// The result of a page-dependent field is replaced by its value, drawn
		// once.
		if s.done {
			return nil
		}
		s.done = true
		return rn.emitString(s.text)
	}
	s := n.text
	preserve := false
	for _, a := range n.attrs {
		if a.space == nsXML && a.name == "space" && a.value == "preserve" {
			preserve = true
		}
	}
	if !preserve {
		s = strings.Trim(s, " \t\r\n")
	}
	s = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(s)
	if cf := rn.p.f.checkField(); cf != nil && s != "" {
		// The saved result of a page number field in the body: tag its first
		// text and keep the result.
		if cf.check.cached == "" && rn.mark == 0 {
			rn.mark = cf.mark
			defer func() { rn.mark = 0 }()
		}
		cf.check.cached += s
	}
	return rn.emitString(s)
}

// emitString draws text in the run's formatting, by font slot.
func (rn *wordRun) emitString(s string) error {
	if s == "" {
		return nil
	}
	start, slot := 0, -1
	for i, c := range s {
		cs := wordSlotOf(c, &rn.rp)
		if slot >= 0 && cs != slot {
			if err := rn.put(slot, s[start:i], false); err != nil {
				return err
			}
			start = i
		}
		slot = cs
	}
	return rn.put(slot, s[start:], false)
}

// lineBreak handles w:br.
func (rn *wordRun) lineBreak(n *wordNode) error {
	if rn.hidden() {
		return nil
	}
	switch n.attrOr("type") {
	case "", "textWrapping":
		return rn.softBreak()
	case "page", "column":
		return rn.p.pageBreak(rn)
	}
	return fmt.Errorf("%w: w:br type", render.ErrInvalid)
}

func (rn *wordRun) softBreak() error {
	if rn.hidden() {
		return nil
	}
	rn.p.sawLineBr = true
	if rn.p.tabMode {
		return rn.tabToken('B')
	}
	return rn.put(rn.slotForPiece(), "", true)
}

// fieldChar tracks complex fields. Their instruction is not drawn; their
// cached result is.
func (rn *wordRun) fieldChar(n *wordNode) error {
	f := rn.p.f
	switch n.attrOr("fldCharType") {
	case "begin":
		if len(f.fields) >= 64 {
			return fmt.Errorf("%w: nested fields", render.ErrLimit)
		}
		if n.child("ffData") != nil {
			if err := rn.p.r.leaveOut("form fields"); err != nil {
				return err
			}
		}
		f.fields = append(f.fields, wordFieldState{})
	case "separate":
		if len(f.fields) > 0 {
			s := &f.fields[len(f.fields)-1]
			if !s.result {
				s.result = true
				if err := f.startFieldResult(s); err != nil {
					return err
				}
			}
		}
	case "end":
		if len(f.fields) > 0 {
			s := f.fields[len(f.fields)-1]
			f.fields = f.fields[:len(f.fields)-1]
			if !s.result {
				// A field without a result: a page-dependent one still
				// draws its value, in the formatting of the closing run.
				if err := f.startFieldResult(&s); err != nil {
					return err
				}
			}
			if s.subst && !s.done && !rn.hidden() {
				return rn.emitString(s.text)
			}
		}
	default:
		return fmt.Errorf("%w: w:fldChar", render.ErrInvalid)
	}
	return nil
}

func (n *wordNode) attrOr(local string) string { v, _ := n.attr(local); return v }

// pageBreak ends the current block at a manual page break and starts the
// continuation.
func (p *wordPara) pageBreak(rn *wordRun) error {
	if err := rn.closeSpan(); err != nil {
		return err
	}
	if err := p.finishBlock(false, rn); err != nil {
		return err
	}
	p.cont = true
	return nil
}

// openSpan makes the span of a font slot the open one.
func (rn *wordRun) openSpan(slot int) error {
	if rn.cur == slot {
		return nil
	}
	if err := rn.closeSpan(); err != nil {
		return err
	}
	open, closing, err := rn.span(slot)
	if err != nil {
		return err
	}
	if err = rn.p.r.charge(rn.spans[slot].nodes); err != nil {
		return err
	}
	rn.p.sb.WriteString(open)
	rn.closing = closing
	rn.cur = slot
	return nil
}

// putMarkup appends generated markup of nodes elements (an inline picture) in
// the run's span, so the line takes the run's font metrics.
func (rn *wordRun) putMarkup(html string, nodes int) error {
	if err := rn.openSpan(rn.slotForPiece()); err != nil {
		return err
	}
	if err := rn.p.r.charge(nodes); err != nil {
		return err
	}
	rn.p.sb.WriteString(html)
	rn.p.hasContent = true
	return nil
}

// put appends text (or a line break) in a font slot, opening a span for the
// slot when it is not the open one.
func (rn *wordRun) put(slot int, text string, br bool) error {
	if err := rn.openSpan(slot); err != nil {
		return err
	}
	if br {
		rn.p.sb.WriteString("<br>")
		rn.p.brAt = rn.p.sb.Len()
	} else {
		if err := rn.p.r.chargeText(len(text)); err != nil {
			return err
		}
		if rn.mark > 0 {
			if err := rn.p.r.charge(1); err != nil {
				return err
			}
			rn.p.sb.WriteString(`<span style="` + wordMarkStyle(rn.mark) + `">`)
			wordEscape(&rn.p.sb, text)
			rn.p.sb.WriteString("</span>")
			rn.mark = 0
		} else {
			wordEscape(&rn.p.sb, text)
		}
		rn.p.hasText = rn.p.hasText || text != ""
	}
	rn.p.hasContent = true
	return nil
}

func (rn *wordRun) closeSpan() error {
	if rn.cur >= 0 {
		rn.p.sb.WriteString(rn.closing)
		rn.cur, rn.closing = -1, ""
	}
	return nil
}

// finishBlock ends the block being built. At the paragraph's end (final) it
// adds the paragraph mark, whose formatting sizes an empty paragraph and the
// last line; at a manual page break it sizes an otherwise empty line with the
// breaking run.
func (p *wordPara) finishBlock(final bool, breaker *wordRun) error {
	sized := &wordRun{p: p, rp: p.mark, cur: -1}
	if !final && breaker != nil {
		sized = &wordRun{p: p, rp: breaker.rp, cur: -1}
	}
	// The mark sizes the last line only when it has no text: an empty
	// paragraph, the line after a manual line break that ends it, or a line
	// of pictures. A larger or smaller mark does not change a line with
	// text, as Word's PDF output shows.
	trailingBr := p.brAt >= 0 && strings.ReplaceAll(p.sb.String()[p.brAt:], "</span>", "") == ""
	if !p.hasContent || final && (!p.hasText || trailingBr) {
		if err := p.mark1(sized, !p.hasContent || trailingBr); err != nil {
			return err
		}
	}
	css := p.blockCSS()
	if p.inlinePics && p.place != "" && p.place != "top" && p.place != "bottom" && p.place != "0.8" {
		p.misplaced = true
	}
	if p.misplaced {
		if err := p.r.approximate("line placement of mixed fonts, of a font whose line metrics differ from its Windows metrics beside an inline picture or at an at-least line height"); err != nil {
			return err
		}
	}
	p.place, p.tallest, p.misplaced, p.inlinePics = "", 0, false, false
	inner := `<div style="` + css + `">` + p.lead.String() + p.sb.String() + `</div>`
	if err := p.r.charge(1); err != nil {
		return err
	}
	var tab *wordTabBlock
	if len(p.tabItems) > 0 || p.marker != nil {
		first := p.ppr.indFirst.v
		if p.cont {
			first = 0
		}
		tab = &wordTabBlock{open: `<div style="` + css + `">`, lead: p.lead.String(), content: p.sb.String(), items: p.tabItems,
			left: p.ppr.indLeft.v, first: first, firstLine: !p.cont, stops: p.stops, defaultTab: p.r.defaultTab}
		if p.marker != nil {
			tab.markerHTML, tab.markerAlign = p.marker.html, p.marker.align
			tab.openAt = func(first float64) string { return `<div style="` + p.blockCSSAt(first) + `">` }
		}
		p.tabItems, p.marker = nil, nil
		inner = ""
	}
	b := &wordBlock{
		kind: "p", inner: inner, tab: tab,
		styleID:         p.styleID,
		contextual:      p.ppr.contextual.v,
		keepNext:        p.ppr.keepNext.v && final,
		keepLines:       p.ppr.keepLines.v,
		widow:           !p.ppr.widow.set || p.ppr.widow.v,
		pageBreakBefore: p.ppr.pageBreakBefore.v || p.cont,
		continuation:    p.cont,
		pics:            p.pics,
	}
	p.blocks = append(p.blocks, b)
	p.pics = nil
	p.lead.Reset()
	p.sb.Reset()
	p.brAt, p.hasText = -1, false
	p.hasContent = false
	return nil
}

// mark1 writes the paragraph mark span: a space when the line would otherwise
// be empty, nothing but the mark's own line height otherwise.
func (p *wordPara) mark1(rn *wordRun, empty bool) error {
	open, closing, err := rn.strut()
	if err != nil {
		return err
	}
	p.sb.WriteString(open)
	if empty {
		p.sb.WriteByte(' ')
	}
	p.sb.WriteString(closing)
	return nil
}

// strut is a span that sizes a line without drawing anything of its own: the
// paragraph mark, or the line holding a manual page break.
func (rn *wordRun) strut() (open, closing string, err error) {
	rp := &rn.rp
	half := 20
	bold, italic := rp.bold.v, rp.italic.v
	if rp.size.set {
		half = rp.size.v
	}
	face, err := rn.p.r.fonts.get(wordFamily(rp, wordSlotASCII), bold, italic)
	if err != nil {
		return "", "", err
	}
	size := float64(half) * 2 / 3
	var c wordCSS
	c.add("font-family", face.css)
	c.px("font-size", size)
	c.px("line-height", rn.p.lineHeight(face, size))
	if err = rn.p.r.charge(1); err != nil {
		return "", "", err
	}
	return `<span style="` + c.String() + `">`, "</span>", nil
}

// blockCSS is the style of the element holding the paragraph's lines. Its own
// font and line height are zero-sized so lines are sized only by their runs
// and the paragraph mark, as Word sizes them.
func (p *wordPara) blockCSS() string {
	first := p.ppr.indFirst.v
	if p.cont {
		first = 0
	}
	return p.blockCSSAt(first)
}

// blockCSSAt is blockCSS with the first line's offset from the left indent
// given.
func (p *wordPara) blockCSSAt(first float64) string {
	var c wordCSS
	left, right := p.ppr.indLeft.v, p.ppr.indRight.v
	c.add("margin", "0 "+wordPx(right)+" 0 "+wordPx(left))
	c.px("text-indent", first)
	switch p.ppr.jc.v {
	case "center":
		c.add("text-align", "center")
	case "right", "end":
		c.add("text-align", "right")
	case "both":
		c.add("text-align", "justify")
	case "distribute":
		c.add("text-align", "justify")
		c.add("text-align-last", "justify")
	default:
		c.add("text-align", "left")
	}
	if p.ppr.bidi.v {
		c.add("direction", "rtl")
	}
	c.add("font-size", "1px")
	c.add("line-height", "0")
	if p.place != "" {
		c.add("-forme-line-placement", p.place)
	}
	c.add("white-space", "pre-wrap")
	if p.f.cell != nil && p.f.cell.autofit {
		c.add("overflow-wrap", "normal")
	} else {
		c.add("overflow-wrap", "anywhere")
	}
	c.px("tab-size", p.r.defaultTab)
	return c.String()
}

// wordSlotOf classifies a character into the w:rFonts slot that styles it.
func wordSlotOf(c rune, rp *wordRPr) int {
	if rp.cs.v {
		return wordSlotCS
	}
	switch {
	case c < 0x80:
		return wordSlotASCII
	case wordIsEastAsian(c):
		return wordSlotEastAsia
	case wordIsComplexScript(c):
		return wordSlotCS
	}
	return wordSlotHAnsi
}

func wordIsEastAsian(c rune) bool {
	switch {
	case c >= 0x1100 && c <= 0x11FF, c >= 0x2E80 && c <= 0x303F, c >= 0x3040 && c <= 0x30FF,
		c >= 0x3100 && c <= 0x312F, c >= 0x3130 && c <= 0x318F, c >= 0x31A0 && c <= 0x31FF,
		c >= 0x3200 && c <= 0x4DBF, c >= 0x4E00 && c <= 0x9FFF, c >= 0xA960 && c <= 0xA97F,
		c >= 0xAC00 && c <= 0xD7FF, c >= 0xF900 && c <= 0xFAFF, c >= 0xFE30 && c <= 0xFE4F,
		c >= 0xFF00 && c <= 0xFFEF, c >= 0x20000 && c <= 0x2FA1F:
		return true
	}
	return false
}

func wordIsComplexScript(c rune) bool {
	switch {
	case c >= 0x0590 && c <= 0x08FF, c >= 0x0900 && c <= 0x0DFF, c >= 0x0E00 && c <= 0x0E7F,
		c >= 0x0E80 && c <= 0x0FFF, c >= 0xFB1D && c <= 0xFDFF, c >= 0xFE70 && c <= 0xFEFF:
		return true
	}
	return false
}

// wordScale is the size of superscript and subscript text relative to its
// run, and the baseline shift as a fraction of the run's size.
const (
	wordScriptScale = 0.65
	wordSuperRaise  = 0.33
	wordSubLower    = 0.12
)

// span builds (and caches) the markup that opens and closes a span of the run
// in a font slot.
func (rn *wordRun) span(slot int) (open, closing string, err error) {
	if s := rn.spans[slot]; s.ok {
		return s.open, s.close, nil
	}
	p, r, rp := rn.p, rn.p.r, &rn.rp
	cs := slot == wordSlotCS
	family := wordFamily(rp, slot)
	bold, italic, half := rp.bold.v, rp.italic.v, 20
	if rp.size.set {
		half = rp.size.v
	}
	if cs {
		bold, italic = rp.boldCs.v, rp.italicCs.v
		if rp.sizeCs.set {
			half = rp.sizeCs.v
		}
	}
	face, err := r.fonts.get(family, bold, italic)
	if err != nil {
		return "", "", err
	}
	size := float64(half) * 2 / 3
	var c wordCSS
	vertical := ""
	switch rp.vertAlign.v {
	case "superscript":
		vertical = wordPx(size * wordSuperRaise)
		size *= wordScriptScale
	case "subscript":
		vertical = wordPx(-size * wordSubLower)
		size *= wordScriptScale
	}
	if rp.position.set && rp.position.v != 0 {
		vertical = wordPx(float64(rp.position.v) * 2 / 3)
	}
	if rp.spacing.set && rp.spacing.v != 0 {
		if err = r.approximate("character spacing"); err != nil {
			return "", "", err
		}
	}
	c.add("font-family", face.css)
	c.px("font-size", size)
	c.px("line-height", p.lineHeight(face, size))
	if vertical != "" {
		c.add("vertical-align", vertical)
	}
	col := wordRGB{}
	if rp.color.set && !rp.color.v.auto {
		col = rp.color.v.rgb
	}
	switch {
	case rp.caps.v:
		c.add("text-transform", "uppercase")
	case rp.smallCaps.v:
		c.add("font-variant-caps", "small-caps")
	}
	if rp.kern.set && rp.kern.v > 0 && half >= rp.kern.v {
		c.add("font-kerning", "normal")
	}
	switch {
	case rp.highlight.set && !rp.highlight.v.none:
		c.rgb("background-color", rp.highlight.v.fill)
	case rp.highlight.set:
		// Highlight "none" shows no background even over shading.
	case rp.shading.set && !rp.shading.v.none:
		c.rgb("background-color", rp.shading.v.fill)
	}
	// Decorations. forme draws a decoration in the colour of the box that
	// declares it and only as a solid line, so an underline in its own colour
	// is declared on an outer span and the text is coloured by an inner one, and
	// a double underline is two declared lines at different offsets.
	underline := rp.underline.set && rp.underline.v.style != "none" && rp.underline.v.style != ""
	strike := rp.strike.v || rp.dstrike.v
	textCol := col
	underCol := col
	double, thick := false, false
	if underline {
		kind := wordUnderlineKind(rp.underline.v.style)
		if !kind.exact {
			if err = r.approximate("underline style " + rp.underline.v.style); err != nil {
				return "", "", err
			}
		}
		double, thick = kind.double, kind.thick
		if uc := rp.underline.v.color; !uc.auto {
			underCol = uc.rgb
		}
	}
	if rp.dstrike.v {
		if err = r.approximate("double strikethrough"); err != nil {
			return "", "", err
		}
	}
	type layer struct {
		css   wordCSS
		color wordRGB
	}
	var layers []*layer
	base := &layer{color: textCol}
	if underline {
		base.color = underCol
	}
	base.css = c
	layers = append(layers, base)
	strikeDone := false
	if underline {
		switch {
		case double:
			base.css.add("text-decoration-line", "underline")
			base.css.add("text-decoration-thickness", "0.06em")
			base.css.add("text-underline-offset", "0.08em")
			second := &layer{color: underCol}
			second.css.add("text-decoration-line", "underline")
			second.css.add("text-decoration-thickness", "0.06em")
			second.css.add("text-underline-offset", "0.20em")
			layers = append(layers, second)
		case thick:
			base.css.add("text-decoration-line", "underline")
			base.css.add("text-decoration-thickness", "0.1em")
		default:
			base.css.add("text-decoration-line", "underline")
		}
		if strike && underCol == textCol {
			// The line-through shares the box of the last underline layer.
			last := layers[len(layers)-1]
			last.css.add("text-decoration-line", "underline line-through")
			strikeDone = true
		}
		if underCol != textCol {
			layers = append(layers, &layer{color: textCol})
		}
	}
	if strike && !strikeDone {
		last := layers[len(layers)-1]
		if underline && last.color != textCol {
			last.color = textCol
		}
		last.css.add("text-decoration-line", "line-through")
	}
	var ob, cb strings.Builder
	for _, l := range layers {
		ob.WriteString(`<span style="`)
		ob.WriteString(l.css.String())
		ob.WriteString(wordColorCSS(l.color))
		ob.WriteString(`">`)
		cb.WriteString("</span>")
	}
	open, closing = ob.String(), cb.String()
	rn.spans[slot] = wordSpanStyle{true, open, closing, len(layers)}
	return open, closing, nil
}

// wordColorCSS is a color declaration.
func wordColorCSS(col wordRGB) string {
	var c wordCSS
	c.rgb("color", col)
	return c.String()
}

type wordUnderlineKindInfo struct{ double, thick, exact bool }

// wordUnderlineKind classifies a w:u style. forme draws solid lines only:
// single, double (two lines) and thick are drawn as such; every other style is
// drawn as a single solid line, an approximation.
func wordUnderlineKind(v string) wordUnderlineKindInfo {
	switch v {
	case "single":
		return wordUnderlineKindInfo{exact: true}
	case "double":
		return wordUnderlineKindInfo{double: true, exact: true}
	case "thick":
		return wordUnderlineKindInfo{thick: true, exact: true}
	}
	return wordUnderlineKindInfo{}
}

// wordFamily is the family for a slot, falling back through the other slots
// and finally to Word's own default.
func wordFamily(rp *wordRPr, slot int) string {
	order := [wordSlots][]int{
		wordSlotASCII:    {wordSlotASCII, wordSlotHAnsi},
		wordSlotHAnsi:    {wordSlotHAnsi, wordSlotASCII},
		wordSlotEastAsia: {wordSlotEastAsia, wordSlotHAnsi, wordSlotASCII},
		wordSlotCS:       {wordSlotCS, wordSlotHAnsi, wordSlotASCII},
	}
	for _, s := range order[slot] {
		if rp.fonts[s].set {
			return rp.fonts[s].v
		}
	}
	return "Times New Roman"
}

// placement is where Word puts a face's text in its lines, as layout's line
// placement, or "" when layout cannot put it there. Word's PDF output shows a
// multiple's extra space below the text (the baseline at the face's Word
// ascent: a fixed fraction of the line, whatever the size), an at-least
// height's above it (the lowest text at the bottom) and an exact height's
// baseline four fifths of the way down. Layout places each line by its own
// tallest text, so sizes of one face are placed line by line.
func (p *wordPara) placement(face *wordFace) string {
	mult := 1.0
	if ln := p.ppr.line; ln.set {
		switch ln.v.rule {
		case "exact":
			return "0.8"
		case "atLeast":
			if !face.bottomPlaced {
				return ""
			}
			return "bottom"
		}
		mult = ln.v.val
	}
	if face.topPlaced {
		// The highest text's ascent at the top: right for pictures on the
		// line too.
		return "top"
	}
	if face.natural <= 0 || mult <= 0 {
		return ""
	}
	// The baseline a fixed fraction down: right for the face's text, not
	// for a picture on the line (finishBlock reports that).
	return strconv.FormatFloat(math.Min(1, face.ascent/(face.natural*mult)), 'f', 6, 64)
}

// lineHeight is the line height in pixels a span of a face and size
// contributes under the paragraph's line spacing.
func (p *wordPara) lineHeight(face *wordFace, size float64) float64 {
	natural := face.natural * size
	h := natural
	if ln := p.ppr.line; ln.set {
		switch ln.v.rule {
		case "exact":
			h = ln.v.val
		case "atLeast":
			h = math.Max(ln.v.val, natural)
		default:
			h = natural * ln.v.val
		}
	}
	place := p.placement(face)
	switch {
	case place == "":
		p.misplaced = true
	case p.place == "" || h > p.tallest:
		if p.place != "" && p.place != place {
			p.misplaced = true
		}
		p.place, p.tallest = place, h
	case place != p.place:
		p.misplaced = true
	}
	return h
}
