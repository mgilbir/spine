package docxrender

import (
	"fmt"
	"math"
	"strings"

	"github.com/mgilbir/spine/render"
)

// wordOpt is a property that a cascade level may or may not set.
type wordOpt[T any] struct {
	v   T
	set bool
}

func wordSome[T any](v T) wordOpt[T] { return wordOpt[T]{v, true} }

// or returns o when it is set and base otherwise.
func (o wordOpt[T]) or(base wordOpt[T]) wordOpt[T] {
	if o.set {
		return o
	}
	return base
}

// Font slots, in the order of the w:rFonts attributes.
const (
	wordSlotASCII = iota
	wordSlotHAnsi
	wordSlotEastAsia
	wordSlotCS
	wordSlots
)

// wordColor is a run colour: automatic (black on the profile's white page) or
// an explicit RGB value.
type wordColor struct {
	auto bool
	rgb  wordRGB
}

// wordUnderline is a resolved w:u.
type wordUnderline struct {
	style string
	color wordColor
}

// wordShading is a resolved solid fill; none is true for "no fill".
type wordShading struct {
	none bool
	fill wordRGB
}

// wordRPr is run formatting as one cascade level (a style, the document
// defaults or direct formatting) states it. Unset fields inherit.
type wordRPr struct {
	rStyle string

	fonts [wordSlots]wordOpt[string]
	hint  string

	// Toggle properties: combined across style levels by exclusive or (ECMA-376
	// 17.7.3), and absolute in direct formatting.
	bold, italic, boldCs, italicCs   wordOpt[bool]
	caps, smallCaps, strike, dstrike wordOpt[bool]
	vanish                           wordOpt[bool]

	size, sizeCs wordOpt[int] // half-points
	color        wordOpt[wordColor]
	underline    wordOpt[wordUnderline]
	highlight    wordOpt[wordShading]
	shading      wordOpt[wordShading]
	vertAlign    wordOpt[string]
	spacing      wordOpt[int] // twentieths of a point
	position     wordOpt[int] // half-points, raised when positive
	kern         wordOpt[int] // half-points: kerning from this size up
	rtl, cs      wordOpt[bool]

	// issues are the features of this level the profile does not draw.
	issues []wordIssue
}

// over returns r with every unset field taken from base. Issues accumulate.
func (r wordRPr) over(base wordRPr) wordRPr {
	out := r
	if out.rStyle == "" {
		out.rStyle = base.rStyle
	}
	for i := range out.fonts {
		out.fonts[i] = r.fonts[i].or(base.fonts[i])
	}
	if out.hint == "" {
		out.hint = base.hint
	}
	out.bold, out.italic = r.bold.or(base.bold), r.italic.or(base.italic)
	out.boldCs, out.italicCs = r.boldCs.or(base.boldCs), r.italicCs.or(base.italicCs)
	out.caps, out.smallCaps = r.caps.or(base.caps), r.smallCaps.or(base.smallCaps)
	out.strike, out.dstrike = r.strike.or(base.strike), r.dstrike.or(base.dstrike)
	out.vanish = r.vanish.or(base.vanish)
	out.size, out.sizeCs = r.size.or(base.size), r.sizeCs.or(base.sizeCs)
	out.color = r.color.or(base.color)
	out.underline = r.underline.or(base.underline)
	out.highlight = r.highlight.or(base.highlight)
	out.shading = r.shading.or(base.shading)
	out.vertAlign = r.vertAlign.or(base.vertAlign)
	out.spacing, out.position = r.spacing.or(base.spacing), r.position.or(base.position)
	out.kern = r.kern.or(base.kern)
	out.rtl, out.cs = r.rtl.or(base.rtl), r.cs.or(base.cs)
	out.issues = nil
	for _, i := range base.issues {
		out.issues = wordAddIssue(out.issues, i)
	}
	for _, i := range r.issues {
		out.issues = wordAddIssue(out.issues, i)
	}
	return out
}

// wordToggle combines a toggle property from a character style onto the value
// the paragraph style level resolved: both on is off.
func wordToggle(para, char wordOpt[bool]) wordOpt[bool] {
	if !char.set {
		return para
	}
	return wordSome(para.v != char.v)
}

// wordTab is one w:tab stop.
type wordTab struct {
	pos    float64 // pixels from the text column edge
	val    string  // clear, left, center, right, decimal, bar, num
	leader string
}

// wordLine is a resolved line spacing.
type wordLine struct {
	rule string  // auto, exact or atLeast
	val  float64 // multiple for auto, pixels otherwise
}

// wordPPr is paragraph formatting as one cascade level states it.
type wordPPr struct {
	style string

	keepNext, keepLines, pageBreakBefore, widow wordOpt[bool]
	contextual                                  wordOpt[bool]
	jc                                          wordOpt[string]
	indLeft, indRight                           wordOpt[float64] // pixels
	indFirst                                    wordOpt[float64] // pixels: firstLine positive, hanging negative
	before, after                               wordOpt[float64] // pixels
	beforeAuto, afterAuto                       wordOpt[bool]
	line                                        wordOpt[wordLine]
	tabs                                        wordOpt[[]wordTab]
	numbered                                    wordOpt[bool]
	bidi                                        wordOpt[bool]

	// mark is the paragraph mark's run formatting (w:pPr/w:rPr).
	mark wordRPr

	issues []wordIssue
}

func (p wordPPr) over(base wordPPr) wordPPr {
	out := p
	if out.style == "" {
		out.style = base.style
	}
	out.keepNext, out.keepLines = p.keepNext.or(base.keepNext), p.keepLines.or(base.keepLines)
	out.pageBreakBefore, out.widow = p.pageBreakBefore.or(base.pageBreakBefore), p.widow.or(base.widow)
	out.contextual = p.contextual.or(base.contextual)
	out.jc = p.jc.or(base.jc)
	out.indLeft, out.indRight, out.indFirst = p.indLeft.or(base.indLeft), p.indRight.or(base.indRight), p.indFirst.or(base.indFirst)
	out.before, out.after = p.before.or(base.before), p.after.or(base.after)
	out.beforeAuto, out.afterAuto = p.beforeAuto.or(base.beforeAuto), p.afterAuto.or(base.afterAuto)
	out.line = p.line.or(base.line)
	out.numbered = p.numbered.or(base.numbered)
	out.bidi = p.bidi.or(base.bidi)
	if p.tabs.set && base.tabs.set {
		out.tabs = wordSome(append(append([]wordTab(nil), base.tabs.v...), p.tabs.v...))
	} else {
		out.tabs = p.tabs.or(base.tabs)
	}
	out.mark = p.mark.over(base.mark)
	out.issues = nil
	for _, i := range base.issues {
		out.issues = wordAddIssue(out.issues, i)
	}
	for _, i := range p.issues {
		out.issues = wordAddIssue(out.issues, i)
	}
	return out
}

// wordMaxLength bounds any length read from the source, in pixels.
const wordMaxLength = 1 << 20

func wordLength(tw string, what string) (float64, error) {
	px, ok := wordRenderTwips(tw)
	if !ok || math.Abs(px) > wordMaxLength {
		return 0, fmt.Errorf("%w: %s", render.ErrInvalid, what)
	}
	return px, nil
}

func wordOnOff(n *wordNode, what string) (bool, error) {
	on, ok := n.on()
	if !ok {
		return false, fmt.Errorf("%w: %s", render.ErrInvalid, what)
	}
	return on, nil
}

// parseRPr reads a w:rPr element. A nil node is the empty level.
func (r *wordRenderer) parseRPr(n *wordNode) (wordRPr, error) {
	var p wordRPr
	if n == nil {
		return p, nil
	}
	omit := func(what string) { p.issues = wordAddIssue(p.issues, wordIssue{wordLeaveOut, "run property " + what}) }
	approx := func(what string) { p.issues = wordAddIssue(p.issues, wordIssue{wordApproximate, "run property " + what}) }
	toggle := func(c *wordNode, dst *wordOpt[bool]) error {
		on, err := wordOnOff(c, "w:"+c.name)
		if err != nil {
			return err
		}
		*dst = wordSome(on)
		return nil
	}
	for _, c := range n.children {
		if c.space != nsW {
			omit(c.name)
			continue
		}
		var err error
		switch c.name {
		case "rStyle":
			p.rStyle = c.val()
		case "rFonts":
			p.parseFonts(r, c)
		case "b":
			err = toggle(c, &p.bold)
		case "bCs":
			err = toggle(c, &p.boldCs)
		case "i":
			err = toggle(c, &p.italic)
		case "iCs":
			err = toggle(c, &p.italicCs)
		case "caps":
			err = toggle(c, &p.caps)
		case "smallCaps":
			err = toggle(c, &p.smallCaps)
			if err == nil && p.smallCaps.v {
				approx("w:smallCaps")
			}
		case "strike":
			err = toggle(c, &p.strike)
		case "dstrike":
			err = toggle(c, &p.dstrike)
		case "vanish":
			err = toggle(c, &p.vanish)
		case "webHidden", "noProof", "snapToGrid", "lang", "specVanish", "oMath":
			// No effect on a drawn page, or handled by the paragraph walker.
		case "sz", "szCs":
			v, ok := wordRenderInt(c.val())
			if !ok || v <= 0 || v > 4096 {
				return p, fmt.Errorf("%w: w:%s", render.ErrInvalid, c.name)
			}
			if c.name == "sz" {
				p.size = wordSome(v)
			} else {
				p.sizeCs = wordSome(v)
			}
		case "color":
			col, e := r.parseColor(c, "val")
			if e != nil {
				return p, e
			}
			p.color = wordSome(col)
		case "u":
			u := wordUnderline{style: c.val(), color: wordColor{auto: true}}
			if u.style == "" {
				return p, fmt.Errorf("%w: w:u", render.ErrInvalid)
			}
			if _, ok := c.attr("color"); ok {
				col, e := r.parseColor(c, "color")
				if e != nil {
					return p, e
				}
				u.color = col
			}
			p.underline = wordSome(u)
		case "highlight":
			v := c.val()
			if v == "none" {
				p.highlight = wordSome(wordShading{none: true})
			} else if rgb, ok := wordHighlight[v]; ok {
				p.highlight = wordSome(wordShading{fill: rgb})
			} else {
				return p, fmt.Errorf("%w: w:highlight", render.ErrInvalid)
			}
		case "shd":
			s, ok, e := r.parseShading(c)
			if e != nil {
				return p, e
			}
			if !ok {
				approx("w:shd pattern")
			} else {
				p.shading = wordSome(s)
			}
		case "vertAlign":
			switch c.val() {
			case "baseline", "superscript", "subscript":
				p.vertAlign = wordSome(c.val())
			default:
				return p, fmt.Errorf("%w: w:vertAlign", render.ErrInvalid)
			}
		case "spacing":
			v, ok := wordRenderInt(c.val())
			if !ok || v < -31680 || v > 31680 {
				return p, fmt.Errorf("%w: w:spacing", render.ErrInvalid)
			}
			p.spacing = wordSome(v)
		case "position":
			v, ok := wordRenderInt(c.val())
			if !ok || v < -4096 || v > 4096 {
				return p, fmt.Errorf("%w: w:position", render.ErrInvalid)
			}
			p.position = wordSome(v)
		case "kern":
			v, ok := wordRenderInt(c.val())
			if !ok || v < 0 {
				return p, fmt.Errorf("%w: w:kern", render.ErrInvalid)
			}
			p.kern = wordSome(v)
		case "rtl":
			err = toggle(c, &p.rtl)
			if err == nil && p.rtl.v {
				approx("w:rtl")
			}
		case "cs":
			err = toggle(c, &p.cs)
		case "outline", "shadow", "emboss", "imprint":
			var on wordOpt[bool]
			if err = toggle(c, &on); err == nil && on.v {
				omit("w:" + c.name)
			}
		case "effect", "em":
			if c.val() != "none" {
				omit("w:" + c.name)
			}
		case "bdr":
			omit("w:bdr")
		case "w":
			if v := c.val(); v != "" && v != "100" {
				omit("w:w")
			}
		case "fitText":
			omit("w:fitText")
		case "eastAsianLayout":
			omit("w:eastAsianLayout")
		case "rPrChange", "ins", "del", "moveFrom", "moveTo":
			approx("tracked formatting change")
		default:
			omit("w:" + c.name)
		}
		if err != nil {
			return p, err
		}
	}
	return p, nil
}

func (p *wordRPr) parseFonts(r *wordRenderer, n *wordNode) {
	slotAttr := [wordSlots]struct{ name, theme string }{
		{"ascii", "asciiTheme"}, {"hAnsi", "hAnsiTheme"}, {"eastAsia", "eastAsiaTheme"}, {"cs", "cstheme"},
	}
	for i, a := range slotAttr {
		if th, ok := n.attr(a.theme); ok && th != "" {
			if f, ok := r.theme.themeFont(th); ok {
				// An empty typeface leaves the slot to the explicit name or to
				// the other slots' families.
				if f != "" {
					p.fonts[i] = wordSome(f)
					continue
				}
			} else {
				p.issues = wordAddIssue(p.issues, wordIssue{wordApproximate, "theme font without a theme part"})
			}
		}
		if v, ok := n.attr(a.name); ok && v != "" {
			p.fonts[i] = wordSome(v)
		}
	}
	if h, ok := n.attr("hint"); ok {
		p.hint = h
	}
}

// parseColor reads a colour held in the attribute named val of n, with the
// optional themeColor, themeTint and themeShade attributes.
func (r *wordRenderer) parseColor(n *wordNode, val string) (wordColor, error) {
	if th, ok := n.attr("themeColor"); ok && th != "" {
		tint, _ := n.attr("themeTint")
		shade, _ := n.attr("themeShade")
		if rgb, ok := r.theme.themeColor(th, tint, shade, r.clrMap()); ok {
			return wordColor{rgb: rgb}, nil
		}
	}
	v := attrOrEmpty(n, val)
	if v == "auto" {
		return wordColor{auto: true}, nil
	}
	rgb, ok := wordParseHex(v)
	if !ok {
		return wordColor{}, fmt.Errorf("%w: colour", render.ErrInvalid)
	}
	return wordColor{rgb: rgb}, nil
}

// parseShading reads a w:shd. Only "clear" and "solid" patterns are exact:
// ok is false for others, which Word draws by mixing fill and colour.
func (r *wordRenderer) parseShading(n *wordNode) (s wordShading, ok bool, err error) {
	pattern := n.val()
	fill, hasFill := n.attr("fill")
	if th, has := n.attr("themeFill"); has && th != "" {
		tint, _ := n.attr("themeFillTint")
		shade, _ := n.attr("themeFillShade")
		if rgb, found := r.theme.themeColor(th, tint, shade, r.clrMap()); found {
			fill, hasFill = fmt.Sprintf("%02X%02X%02X", rgb.r, rgb.g, rgb.b), true
		}
	}
	switch pattern {
	case "nil", "none":
		return wordShading{none: true}, true, nil
	case "clear":
		if !hasFill || fill == "auto" {
			return wordShading{none: true}, true, nil
		}
		rgb, valid := wordParseHex(fill)
		if !valid {
			return s, false, fmt.Errorf("%w: w:shd fill", render.ErrInvalid)
		}
		return wordShading{fill: rgb}, true, nil
	case "solid":
		// Solid shading paints the colour attribute, not the fill.
		col, has := n.attr("color")
		if th, found := n.attr("themeColor"); found && th != "" {
			tint, _ := n.attr("themeTint")
			shade, _ := n.attr("themeShade")
			if rgb, ok := r.theme.themeColor(th, tint, shade, r.clrMap()); ok {
				return wordShading{fill: rgb}, true, nil
			}
		}
		if !has || col == "auto" {
			return wordShading{fill: wordRGB{}}, true, nil
		}
		rgb, valid := wordParseHex(col)
		if !valid {
			return s, false, fmt.Errorf("%w: w:shd colour", render.ErrInvalid)
		}
		return wordShading{fill: rgb}, true, nil
	}
	return s, false, nil
}

// parsePPr reads a w:pPr element (without its w:sectPr, which the section
// walker reads). A nil node is the empty level.
func (r *wordRenderer) parsePPr(n *wordNode) (wordPPr, error) {
	var p wordPPr
	if n == nil {
		return p, nil
	}
	omit := func(what string) { p.issues = wordAddIssue(p.issues, wordIssue{wordLeaveOut, "paragraph property " + what}) }
	approx := func(what string) { p.issues = wordAddIssue(p.issues, wordIssue{wordApproximate, "paragraph property " + what}) }
	toggle := func(c *wordNode, dst *wordOpt[bool]) error {
		on, err := wordOnOff(c, "w:"+c.name)
		if err != nil {
			return err
		}
		*dst = wordSome(on)
		return nil
	}
	for _, c := range n.children {
		if c.space != nsW {
			omit(c.name)
			continue
		}
		var err error
		switch c.name {
		case "pStyle":
			p.style = c.val()
		case "keepNext":
			err = toggle(c, &p.keepNext)
		case "keepLines":
			err = toggle(c, &p.keepLines)
		case "pageBreakBefore":
			err = toggle(c, &p.pageBreakBefore)
		case "widowControl":
			err = toggle(c, &p.widow)
		case "contextualSpacing":
			err = toggle(c, &p.contextual)
		case "bidi":
			approx("w:bidi")
			err = toggle(c, &p.bidi)
		case "jc":
			switch v := c.val(); v {
			case "left", "start", "center", "right", "end", "both", "distribute":
				p.jc = wordSome(v)
			case "mediumKashida", "highKashida", "lowKashida", "thaiDistribute":
				approx("w:jc " + v)
				p.jc = wordSome("both")
			default:
				return p, fmt.Errorf("%w: w:jc", render.ErrInvalid)
			}
		case "ind":
			err = p.parseInd(c, approx)
		case "spacing":
			err = p.parseSpacing(c, approx)
		case "tabs":
			var tabs []wordTab
			for _, t := range c.children {
				if !t.is("tab") {
					omit("w:tabs child")
					continue
				}
				pos, e := wordLength(attrOrEmpty(t, "pos"), "w:tab")
				if e != nil {
					return p, e
				}
				tabs = append(tabs, wordTab{pos: pos, val: t.val(), leader: attrOrEmpty(t, "leader")})
				if len(tabs) > 64 {
					return p, fmt.Errorf("%w: w:tabs", render.ErrLimit)
				}
			}
			p.tabs = wordSome(tabs)
		case "numPr":
			id := c.child("numId")
			if id == nil {
				// Only an inherited numbering property level (ilvl) is set.
				break
			}
			if id.val() == "0" {
				p.numbered = wordSome(false)
			} else {
				p.numbered = wordSome(true)
			}
		case "rPr":
			p.mark, err = r.parseRPr(c)
		case "sectPr":
			// Read by the section walker.
		case "pBdr":
			if len(c.children) > 0 {
				omit("w:pBdr")
			}
		case "shd":
			omit("w:shd")
		case "framePr":
			omit("w:framePr")
		case "textDirection":
			if c.val() != "lrTb" && c.val() != "lr" {
				omit("w:textDirection")
			}
		case "textAlignment":
			if v := c.val(); v != "auto" && v != "baseline" {
				approx("w:textAlignment")
			}
		case "mirrorIndents":
			if on, e := wordOnOff(c, "w:mirrorIndents"); e != nil {
				return p, e
			} else if on {
				omit("w:mirrorIndents")
			}
		case "pPrChange":
			approx("tracked formatting change")
		case "suppressAutoHyphens", "suppressLineNumbers", "suppressOverlap", "outlineLvl", "snapToGrid",
			"adjustRightInd", "kinsoku", "wordWrap", "overflowPunct", "topLinePunct", "autoSpaceDE", "autoSpaceDN",
			"divId", "cnfStyle":
			// No effect on the profile's layout.
		default:
			omit("w:" + c.name)
		}
		if err != nil {
			return p, err
		}
	}
	return p, nil
}

func attrOrEmpty(n *wordNode, local string) string { v, _ := n.attr(local); return v }

func (p *wordPPr) parseInd(c *wordNode, approx func(string)) error {
	for _, a := range c.attrs {
		if a.space != nsW {
			continue
		}
		switch a.name {
		case "left", "start":
			v, err := wordLength(a.value, "w:ind")
			if err != nil {
				return err
			}
			p.indLeft = wordSome(v)
		case "right", "end":
			v, err := wordLength(a.value, "w:ind")
			if err != nil {
				return err
			}
			p.indRight = wordSome(v)
		case "firstLine":
			v, err := wordLength(a.value, "w:ind")
			if err != nil {
				return err
			}
			if _, hang := c.attr("hanging"); !hang {
				p.indFirst = wordSome(v)
			}
		case "hanging":
			v, err := wordLength(a.value, "w:ind")
			if err != nil {
				return err
			}
			p.indFirst = wordSome(-v)
		case "leftChars", "startChars", "rightChars", "endChars", "firstLineChars", "hangingChars":
			approx("w:ind in character units")
		default:
			approx("w:ind@" + a.name)
		}
	}
	return nil
}

func (p *wordPPr) parseSpacing(c *wordNode, approx func(string)) error {
	var rule string
	var lineVal string
	var hasLine bool
	for _, a := range c.attrs {
		if a.space != nsW {
			continue
		}
		switch a.name {
		case "before":
			v, err := wordLength(a.value, "w:spacing")
			if err != nil {
				return err
			}
			p.before = wordSome(math.Max(v, 0))
		case "after":
			v, err := wordLength(a.value, "w:spacing")
			if err != nil {
				return err
			}
			p.after = wordSome(math.Max(v, 0))
		case "beforeAutospacing":
			on, err := wordBoolAttr(a.value)
			if err != nil {
				return err
			}
			p.beforeAuto = wordSome(on)
		case "afterAutospacing":
			on, err := wordBoolAttr(a.value)
			if err != nil {
				return err
			}
			p.afterAuto = wordSome(on)
		case "line":
			lineVal, hasLine = a.value, true
		case "lineRule":
			rule = a.value
		case "beforeLines", "afterLines":
			if a.value != "0" {
				approx("w:spacing in line units")
			}
		default:
			approx("w:spacing@" + a.name)
		}
	}
	if hasLine {
		switch rule {
		case "", "auto":
			v, ok := wordRenderInt(lineVal)
			if !ok || v < 0 || v > 1<<20 {
				return fmt.Errorf("%w: w:spacing line", render.ErrInvalid)
			}
			p.line = wordSome(wordLine{rule: "auto", val: float64(v) / 240})
		case "exact", "atLeast":
			v, err := wordLength(lineVal, "w:spacing line")
			if err != nil {
				return err
			}
			p.line = wordSome(wordLine{rule: rule, val: v})
		default:
			return fmt.Errorf("%w: w:spacing lineRule", render.ErrInvalid)
		}
	}
	return nil
}

func wordBoolAttr(v string) (bool, error) {
	switch strings.TrimSpace(v) {
	case "1", "true", "on":
		return true, nil
	case "0", "false", "off", "":
		return false, nil
	}
	return false, fmt.Errorf("%w: boolean attribute", render.ErrInvalid)
}
