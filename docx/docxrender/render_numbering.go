package docxrender

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mgilbir/spine/render"
)

// Numbering and lists.
//
// A paragraph is numbered by w:numPr (numId and ilvl), set directly or by its
// paragraph style. numId names a w:num of numbering.xml, which instantiates a
// w:abstractNum (through w:numStyleLink the levels are those of the numbering
// style's own list, bounded by wordMaxStyleDepth) and may override the start
// value or the whole definition of single levels.
//
// Counting follows Word. Counters belong to the abstract numbering, so several
// w:num of one abstractNum continue each other's count, unless a w:num overrides
// the start of a level: then the first paragraph of that w:num at the level
// restarts there. A level restarts, at its start value, after a paragraph of a
// higher level (or after the levels w:lvlRestart names, or never when it is
// zero). Paragraphs without numbering, and sections, do not affect counts. The
// counters advance in the order the paragraphs are translated, which is
// document order.
//
// The marker is the level's w:lvlText with %1 to %9 replaced by the counters
// of the levels, each in that level's format (in decimal under w:isLgl),
// formatted with the paragraph mark's run formatting overlaid by the level's
// w:rPr, then the suffix (a tab, a space or nothing). The tab is set by the tab
// machinery of render_tabs.go, so it reaches the hanging indent, a custom stop
// or the next default stop as in Word. The level's w:lvlJc aligns the marker on
// the position where the first line starts: it begins there, is centred on it
// or ends there. The level's w:pPr sets indents and tab stops under the
// paragraph's own: with the numbering set directly on the paragraph they
// replace the paragraph style's; with it set by the style, the style's win.

const (
	// wordListLevels is the number of levels of a list.
	wordListLevels = 9
	// wordMaxListValue bounds a counter, as Word does.
	wordMaxListValue = 32767
	// wordMaxLvlText bounds the length of a level's text in characters.
	wordMaxLvlText = 256
)

// wordLevel is one w:lvl of an abstract numbering or a w:lvlOverride.
type wordLevel struct {
	start     int
	numFmt    string
	restart   wordOpt[int]
	pStyle    string
	isLgl     bool
	suff      string
	text      string
	picBullet bool
	jc        string
	ppr       wordPPr
	rpr       wordRPr
	issues    []wordIssue
}

type wordAbstractNum struct {
	node         *wordNode
	parsed       bool
	numStyleLink string
	levels       [wordListLevels]*wordLevel
}

type wordLevelOverride struct {
	start wordOpt[int]
	lvl   *wordLevel
}

type wordNum struct {
	node       *wordNode
	parsed     bool
	abstractID string
	overrides  [wordListLevels]wordLevelOverride
}

// wordCounter is one level's count; a level that has not started yet counts
// from its start value.
type wordCounter struct {
	val     int
	started bool
}

type wordListState struct{ lv [wordListLevels]wordCounter }

type wordOverrideKey struct {
	num *wordNum
	lvl int
}

// wordLists is the numbering part and the counting state of one preparation.
type wordLists struct {
	r         *wordRenderer
	abstracts map[string]*wordAbstractNum
	nums      map[string]*wordNum
	pics      map[string]bool
	state     map[*wordAbstractNum]*wordListState
	// started records the (num, level) pairs whose start override has been
	// applied.
	started map[wordOverrideKey]bool
}

// wordListMark is the numbering of one paragraph.
type wordListMark struct {
	def *wordLevel
	// text is the formatted marker; for a bullet, the level's text.
	text   string
	bullet bool
}

// loadNumbering reads a numbering part. A nil root is a document without one:
// paragraphs that name a list then draw without it, as Word does.
func (r *wordRenderer) loadNumbering(root *wordNode) error {
	l := &wordLists{r: r, abstracts: map[string]*wordAbstractNum{}, nums: map[string]*wordNum{}, pics: map[string]bool{},
		state: map[*wordAbstractNum]*wordListState{}, started: map[wordOverrideKey]bool{}}
	r.lists = l
	if root == nil {
		return nil
	}
	for _, c := range root.children {
		if c.space != nsW {
			continue
		}
		switch c.name {
		case "abstractNum":
			id, ok := c.attr("abstractNumId")
			if !ok || l.abstracts[id] != nil {
				return fmt.Errorf("%w: w:abstractNumId", render.ErrInvalid)
			}
			l.abstracts[id] = &wordAbstractNum{node: c}
		case "num":
			id, ok := c.attr("numId")
			if !ok || l.nums[id] != nil {
				return fmt.Errorf("%w: w:numId", render.ErrInvalid)
			}
			l.nums[id] = &wordNum{node: c}
		case "numPicBullet":
			if id, ok := c.attr("numPicBulletId"); ok {
				l.pics[id] = true
			}
		}
	}
	return nil
}

func (l *wordLists) parseAbstract(a *wordAbstractNum) error {
	if a.parsed {
		return nil
	}
	for _, c := range a.node.children {
		if c.space != nsW {
			continue
		}
		switch c.name {
		case "numStyleLink":
			a.numStyleLink = c.val()
		case "lvl":
			lv, ilvl, err := l.parseLevel(c)
			if err != nil {
				return err
			}
			if a.levels[ilvl] != nil {
				return fmt.Errorf("%w: duplicate numbering level", render.ErrInvalid)
			}
			a.levels[ilvl] = lv
		case "nsid", "tmpl", "name", "multiLevelType", "styleLink":
			// Identify the list; no effect on how it is drawn.
		default:
			if err := l.r.leaveOut("numbering definition property w:" + c.name); err != nil {
				return err
			}
		}
	}
	a.parsed = true
	return nil
}

func (l *wordLists) parseNum(n *wordNum) error {
	if n.parsed {
		return nil
	}
	for _, c := range n.node.children {
		if c.space != nsW {
			continue
		}
		switch c.name {
		case "abstractNumId":
			n.abstractID = c.val()
		case "lvlOverride":
			ilvl, ok := wordRenderInt(attrOrEmpty(c, "ilvl"))
			if !ok || ilvl < 0 || ilvl >= wordListLevels || n.overrides[ilvl].lvl != nil || n.overrides[ilvl].start.set {
				return fmt.Errorf("%w: w:lvlOverride", render.ErrInvalid)
			}
			ov := &n.overrides[ilvl]
			if so := c.child("startOverride"); so != nil {
				v, err := wordListStart(so)
				if err != nil {
					return err
				}
				ov.start = wordSome(v)
			}
			if lv := c.child("lvl"); lv != nil {
				def, at, err := l.parseLevel(lv)
				if err != nil {
					return err
				}
				if at != ilvl {
					return fmt.Errorf("%w: w:lvlOverride level", render.ErrInvalid)
				}
				ov.lvl = def
			}
		}
	}
	n.parsed = true
	return nil
}

func wordListStart(n *wordNode) (int, error) {
	v, ok := wordRenderInt(n.val())
	if !ok || v < 0 || v > wordMaxListValue {
		return 0, fmt.Errorf("%w: numbering start", render.ErrInvalid)
	}
	return v, nil
}

// parseLevel reads a w:lvl and returns it with its level.
func (l *wordLists) parseLevel(n *wordNode) (*wordLevel, int, error) {
	r := l.r
	ilvl, ok := wordRenderInt(attrOrEmpty(n, "ilvl"))
	if !ok || ilvl < 0 || ilvl >= wordListLevels {
		return nil, 0, fmt.Errorf("%w: w:ilvl", render.ErrInvalid)
	}
	lv := &wordLevel{numFmt: "decimal", suff: "tab", jc: "left"}
	omit := func(what string) {
		lv.issues = wordAddIssue(lv.issues, wordIssue{wordLeaveOut, "numbering level property " + what})
	}
	approx := func(what string) {
		lv.issues = wordAddIssue(lv.issues, wordIssue{wordApproximate, "numbering level " + what})
	}
	for _, c := range n.children {
		if c.space != nsW {
			omit(c.name)
			continue
		}
		var err error
		switch c.name {
		case "start":
			lv.start, err = wordListStart(c)
		case "numFmt":
			lv.numFmt = c.val()
			if lv.numFmt == "" || len(lv.numFmt) > 64 {
				return nil, 0, fmt.Errorf("%w: w:numFmt", render.ErrInvalid)
			}
		case "lvlRestart":
			v, ok := wordRenderInt(c.val())
			if !ok || v < 0 || v > wordListLevels {
				return nil, 0, fmt.Errorf("%w: w:lvlRestart", render.ErrInvalid)
			}
			lv.restart = wordSome(v)
		case "pStyle":
			lv.pStyle = c.val()
		case "isLgl":
			lv.isLgl, err = wordOnOff(c, "w:isLgl")
		case "suff":
			switch v := c.val(); v {
			case "tab", "space", "nothing":
				lv.suff = v
			default:
				return nil, 0, fmt.Errorf("%w: w:suff", render.ErrInvalid)
			}
		case "lvlText":
			lv.text = attrOrEmpty(c, "val")
			if utf8.RuneCountInString(lv.text) > wordMaxLvlText || !utf8.ValidString(lv.text) {
				return nil, 0, fmt.Errorf("%w: w:lvlText", render.ErrInvalid)
			}
		case "lvlPicBulletId":
			lv.picBullet = true
		case "lvlJc":
			switch v := c.val(); v {
			case "left", "start":
				lv.jc = "left"
			case "center":
				lv.jc = "center"
			case "right", "end":
				lv.jc = "right"
			default:
				return nil, 0, fmt.Errorf("%w: w:lvlJc", render.ErrInvalid)
			}
		case "pPr":
			lv.ppr, err = r.parsePPr(c)
		case "rPr":
			lv.rpr, err = r.parseRPr(c)
		case "legacy":
			if on, e := wordBoolAttr(attrOrEmpty(c, "legacy")); e == nil && on {
				approx("legacy spacing")
			}
		default:
			omit("w:" + c.name)
		}
		if err != nil {
			return nil, 0, err
		}
	}
	if lv.picBullet {
		// The picture is not drawn; a bullet character stands for it.
		approx("picture bullet")
	}
	return lv, ilvl, nil
}

// numStyleNum resolves a numbering style to the w:num its numPr names.
func (l *wordLists) numStyleNum(styleID string) (*wordNum, error) {
	st := l.r.styles.byID[styleID]
	if st == nil || st.typ != "numbering" {
		return nil, nil
	}
	chain, err := l.r.styles.chain(styleID)
	if err != nil {
		return nil, err
	}
	var ppr wordPPr
	for _, s := range chain {
		if err = l.r.styles.parse(s); err != nil {
			return nil, err
		}
		ppr = s.ppr.over(ppr)
	}
	if !ppr.numID.set {
		return nil, nil
	}
	return l.nums[ppr.numID.v], nil
}

// resolve finds the num and the abstract numbering whose levels it uses, or
// nil when the list is not defined, which Word draws as no numbering.
func (l *wordLists) resolve(id string) (*wordNum, *wordAbstractNum, error) {
	n := l.nums[id]
	if n == nil {
		return nil, nil, nil
	}
	if err := l.parseNum(n); err != nil {
		return nil, nil, err
	}
	a := l.abstracts[n.abstractID]
	for depth := 0; a != nil; depth++ {
		if err := l.parseAbstract(a); err != nil {
			return nil, nil, err
		}
		if a.numStyleLink == "" {
			return n, a, nil
		}
		if depth >= wordMaxStyleDepth {
			return nil, nil, fmt.Errorf("%w: numbering style links", render.ErrInvalid)
		}
		linked, err := l.numStyleNum(a.numStyleLink)
		if err != nil || linked == nil {
			return nil, nil, err
		}
		if err = l.parseNum(linked); err != nil {
			return nil, nil, err
		}
		a = l.abstracts[linked.abstractID]
	}
	return nil, nil, nil
}

// level is the definition of level k of a num: its own override, else the
// abstract numbering's.
func (n *wordNum) level(a *wordAbstractNum, k int) *wordLevel {
	if lv := n.overrides[k].lvl; lv != nil {
		return lv
	}
	return a.levels[k]
}

func (n *wordNum) start(a *wordAbstractNum, k int) int {
	if n.overrides[k].start.set {
		return n.overrides[k].start.v
	}
	if lv := n.level(a, k); lv != nil {
		return lv.start
	}
	return 0
}

// restartsAfter reports whether level k, defined by lv, restarts when a
// paragraph of level j (j below k) occurs.
func wordRestartsAfter(lv *wordLevel, k, j int) bool {
	last := k - 1
	if lv != nil && lv.restart.set {
		if lv.restart.v == 0 {
			return false
		}
		last = min(last, lv.restart.v-1)
	}
	return j <= last
}

// listProps resolves a paragraph's numbering, counts the paragraph and returns
// its effective formatting with the level's indents and tab stops in place.
// The paragraph is not numbered (nil mark) when it has no list or the list is
// not defined.
func (r *wordRenderer) listProps(direct wordPPr, lv *wordParaLevel, styleID string) (wordPPr, *wordListMark, error) {
	plain := direct.over(lv.ppr)
	if !plain.numID.set || plain.numID.v == "0" || r.lists == nil {
		return plain, nil, nil
	}
	l := r.lists
	n, a, err := l.resolve(plain.numID.v)
	if err != nil || n == nil {
		return plain, nil, err
	}
	fromStyle := !direct.numID.set
	ilvl := 0
	switch {
	case plain.ilvl.set:
		ilvl = plain.ilvl.v
	case fromStyle:
		ilvl, err = l.linkedLevel(n, a, styleID)
		if err != nil {
			return plain, nil, err
		}
	}
	def := n.level(a, ilvl)
	if def == nil {
		return plain, nil, nil
	}
	ppr := direct.over(def.ppr.over(lv.ppr))
	if fromStyle {
		ppr = direct.over(lv.own.over(def.ppr.over(r.styles.docPPr)))
	}
	if err = l.count(n, a, ilvl); err != nil {
		return plain, nil, err
	}
	mark := &wordListMark{def: def, bullet: def.numFmt == "bullet" || def.picBullet}
	switch {
	case def.picBullet:
		mark.text = "\u2022"
	case mark.bullet:
		mark.text = def.text
	default:
		mark.text, err = l.format(n, a, def, ilvl)
	}
	if err != nil {
		return plain, nil, err
	}
	return ppr, mark, nil
}

// linkedLevel finds the level a paragraph style is linked to by the level's
// w:pStyle, looking from the paragraph's style to its bases; the first level
// when there is none.
func (l *wordLists) linkedLevel(n *wordNum, a *wordAbstractNum, styleID string) (int, error) {
	chain, err := l.r.styles.chain(styleID)
	if err != nil {
		return 0, err
	}
	for i := len(chain) - 1; i >= 0; i-- {
		for k := range wordListLevels {
			if lv := n.level(a, k); lv != nil && lv.pStyle == chain[i].id {
				return k, nil
			}
		}
	}
	return 0, nil
}

// count advances the counters for a paragraph of a level.
func (l *wordLists) count(n *wordNum, a *wordAbstractNum, ilvl int) error {
	st := l.state[a]
	if st == nil {
		st = &wordListState{}
		l.state[a] = st
	}
	key := wordOverrideKey{n, ilvl}
	c := &st.lv[ilvl]
	if n.overrides[ilvl].start.set && !l.started[key] {
		c.started = false
	}
	l.started[key] = true
	if !c.started {
		c.val, c.started = n.start(a, ilvl), true
	} else {
		c.val++
		if c.val > wordMaxListValue {
			return fmt.Errorf("%w: list counter", render.ErrLimit)
		}
	}
	for k := ilvl + 1; k < wordListLevels; k++ {
		if wordRestartsAfter(n.level(a, k), k, ilvl) {
			st.lv[k].started = false
		}
	}
	return nil
}

// format builds the marker text of a level from its w:lvlText.
func (l *wordLists) format(n *wordNum, a *wordAbstractNum, def *wordLevel, ilvl int) (string, error) {
	st := l.state[a]
	var sb strings.Builder
	runes := []rune(def.text)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c != '%' || i+1 >= len(runes) || runes[i+1] < '1' || runes[i+1] > '9' {
			sb.WriteRune(c)
			continue
		}
		i++
		k := int(runes[i] - '1')
		kd := n.level(a, k)
		if kd == nil {
			continue
		}
		if k > ilvl {
			if err := l.r.approximate("numbering text that refers to a deeper level"); err != nil {
				return "", err
			}
		}
		v := n.start(a, k)
		if st.lv[k].started {
			v = st.lv[k].val
		}
		format := kd.numFmt
		if def.isLgl && format != "bullet" && format != "none" {
			format = "decimal"
		}
		s, exact := wordFormatNumber(format, v)
		if !exact {
			if err := l.r.approximate("number format " + format); err != nil {
				return "", err
			}
		}
		sb.WriteString(s)
	}
	return sb.String(), nil
}

// Bullet symbols. Word draws bullets in symbol fonts by private-use code
// points (U+F020 to U+F0FF stand for the font's own code 0x20 to 0xFF). The
// glyph is drawn from the font when the host provides one; otherwise a
// Unicode character of similar look is drawn in the paragraph's font, which is
// an approximation.

var wordSymbolBullets = map[string]map[int]rune{
	"symbol":      {0xB7: 0x2022, 0xA8: 0x2666, 0xB0: 0x00B0, 0xD7: 0x00D7},
	"wingdings":   {0xA7: 0x25AA, 0x6C: 0x25CF, 0x6E: 0x25A0, 0x71: 0x274F, 0x76: 0x2756, 0xD8: 0x27A2, 0xFC: 0x2713, 0xFB: 0x2717, 0xA8: 0x25FB, 0x9F: 0x2022, 0x6F: 0x25A1, 0x77: 0x2B25, 0xE8: 0x2794, 0x70: 0x274D, 0xFE: 0x2611},
	"wingdings 2": {0x97: 0x25AA, 0xA2: 0x25CB},
	"wingdings 3": {0x7D: 0x25B6},
}

// wordSymbolFont reports whether a family is one of the symbol fonts whose
// characters are addressed by code.
func wordSymbolFont(family string) (string, bool) {
	f := strings.ToLower(strings.TrimSpace(family))
	switch f {
	case "symbol", "wingdings", "wingdings 2", "wingdings 3", "webdings":
		return f, true
	}
	return "", false
}

// startList writes the marker of a numbered paragraph before its content.
func (p *wordPara) startList() error {
	m := p.list
	if m == nil {
		return nil
	}
	r := p.r
	def := m.def
	if err := r.issues(def.issues); err != nil {
		return err
	}
	if err := r.issues(def.rpr.issues); err != nil {
		return err
	}
	eff, err := r.styles.runProps(p.mark, def.rpr)
	if err != nil {
		return err
	}
	if eff.vanish.v {
		return nil
	}
	rn := &wordRun{p: p, rp: eff, cur: -1}
	tr := rn
	switch {
	case m.bullet:
		if tr, err = p.bulletMarker(rn, m.text); err != nil {
			return err
		}
	case m.text != "":
		if err = rn.emitString(m.text); err != nil {
			return err
		}
	}
	if m.text != "" && def.jc != "left" {
		// The marker is placed relative to where the first line starts, which
		// needs its width: the tab machinery measures it.
		markup := p.sb.String()
		if rn.cur >= 0 {
			markup += rn.closing
		}
		p.marker = &wordMarker{html: markup, align: def.jc}
		p.tabMode = true
	}
	switch def.suff {
	case "tab":
		p.tabMode = true
		if err = tr.tabToken('T'); err != nil {
			return err
		}
		p.tabItems[len(p.tabItems)-1].marker = true
	case "space":
		if err = tr.put(tr.slotForPiece(), " ", false); err != nil {
			return err
		}
	}
	return tr.closeSpan()
}

// wordMarker is a list marker that is positioned by its width.
type wordMarker struct {
	html  string
	align string
}

// bulletMarker draws a bullet level's text and returns the run whose span
// formats what follows the marker.
func (p *wordPara) bulletMarker(rn *wordRun, text string) (*wordRun, error) {
	family := wordFamily(&rn.rp, wordSlotASCII)
	symbol, ok := wordSymbolFont(family)
	if !ok {
		return rn, rn.emitString(text)
	}
	// Symbol fonts are addressed through the ASCII slot whatever the
	// character's script.
	sym := rn.rp
	for i := range sym.fonts {
		sym.fonts[i] = wordSome(family)
	}
	plain := rn.rp
	plain.fonts = p.mark.fonts
	symRun := &wordRun{p: p, rp: sym, cur: -1}
	plainRun := &wordRun{p: p, rp: plain, cur: -1}
	for _, c := range text {
		code := -1
		switch {
		case c >= 0xF020 && c <= 0xF0FF:
			code = int(c - 0xF000)
		case c >= 0x20 && c <= 0xFF:
			code = int(c)
		}
		if code < 0 {
			if err := plainRun.emitString(string(c)); err != nil {
				return nil, err
			}
			if err := plainRun.closeSpan(); err != nil {
				return nil, err
			}
			continue
		}
		glyph := rune(0xF000 + code)
		has, err := p.r.fonts.hasGlyph(family, sym.bold.v, sym.italic.v, glyph)
		if err != nil {
			return nil, err
		}
		if has {
			err = symRun.put(wordSlotASCII, string(glyph), false)
			if err == nil {
				err = symRun.closeSpan()
			}
			if err != nil {
				return nil, err
			}
			continue
		}
		sub, known := wordSymbolBullets[symbol][code]
		if !known {
			sub = 0x2022
		}
		if err = p.r.approximate("symbol font bullet drawn with a substitute character"); err != nil {
			return nil, err
		}
		if err = plainRun.emitString(string(sub)); err != nil {
			return nil, err
		}
		if err = plainRun.closeSpan(); err != nil {
			return nil, err
		}
	}
	return plainRun, nil
}

// hasGlyph reports whether the face resolved for a family has a glyph for a
// character. A family the host cannot resolve has none.
func (f *wordFonts) hasGlyph(family string, bold, italic bool, c rune) (bool, error) {
	w, err := f.get(family, bold, italic)
	if err != nil {
		if errors.Is(err, render.ErrUnsupported) {
			return false, nil
		}
		return false, err
	}
	_, ok := w.face.GlyphID(c)
	return ok, nil
}

func init() {
	wordParagraphStarts = append(wordParagraphStarts, (*wordPara).startList)
}
