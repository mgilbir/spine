package pptxrender

import (
	"context"
	"encoding/xml"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// renderTextStyles resolves inherited paragraph and run properties for
// non-placeholder text on one slide.
//
// Such text inherits from its own list style and then from document defaults.
// Whether PowerPoint consults the master's other-text style before the
// presentation default text style is not specified, and implementations
// differ, so every property is resolved both ways and must agree; agreement
// also covers consulting them in the other order. PowerPoint writes the two
// with the same content. A deck built in code has an other-text style that
// sets only sizes, margins and alignment, which the default style matches.
type renderTextStyles struct {
	ctx    context.Context
	slide  *renderSlide
	budget *core.SourceBudget
	colors *renderColors
	// masterErrs holds the first unsupported node the master source check
	// found in each of its text styles, by element name.
	masterErrs map[string]error
	// warn, in best-effort mode, receives a style problem once; the styles
	// are then used as parsed.
	warn func(error)

	loaded          bool
	err             error
	other, defaults *dml.LstStyle

	// fontScale and lnSpcReduction are the normal autofit of the body being
	// laid out, in thousandths of a percent; zero scale is none.
	fontScale, lnSpcReduction int32
}

// renderParaStyle is a paragraph's resolved layout.
type renderParaStyle struct {
	align       enum.TextAlign
	lineSpacing int32 // 100000 is 100%
	// lineFixed, when positive, is an exact line height that replaces the
	// percentage.
	lineFixed     dml.EMU
	before, after dml.EMU
	// beforePct and afterPct are spacing in percent of the paragraph's first
	// and last line heights (100000 is one line), added to before and after.
	beforePct, afterPct int32
	marL, marR          dml.EMU
	// tabSize is the default tab stop spacing; tabs are the explicit stops.
	tabSize dml.EMU
	tabs    *dml.TabLst
	indent  dml.EMU // first line offset from marL; negative hangs
	bullet  renderBullet
	// eaBreak is whether East Asian text wraps by East Asian line breaking
	// rules, and hangPunct whether East Asian stops and commas may hang past
	// the end of a line.
	eaBreak, hangPunct bool
	// rtl is a right-to-left paragraph: it starts at the right, its margins
	// and indent are from there, and alignment stays physical: left is the
	// left edge, as in PowerPoint.
	rtl bool
	// kashida is low kashida justification, which stretches Arabic by
	// elongating letters.
	kashida bool
}

// renderBullet is a paragraph's character bullet; char is empty for none.
type renderBullet struct {
	char string
	// autoNum is the numbering scheme of a numbered bullet, whose char the
	// layout sets; startAt is its first number.
	autoNum string
	startAt int32
	// font is the bullet font, or empty to follow the text's.
	font string
	// size is a percentage of the text size (100000 is 100%), or zero when
	// points is set.
	size, points int32
	// color is the bullet color, unless it follows the text's.
	color    style.RGBA
	ownColor bool
}

// renderRunStyle is a run's resolved appearance: the properties that shape it,
// and its paint.
type renderRunStyle struct {
	renderShaping
	color     style.RGBA
	highlight renderHighlight
	// eastAsian marks a run in an East Asian language, where PowerPoint may
	// draw symbols of ambiguous width with the East Asian font.
	eastAsian bool
	// ea is the run's East Asian font, which draws its East Asian
	// characters; empty where its styles and the theme name none. eaFault
	// is why it cannot be resolved, which a run with no East Asian
	// characters does not need.
	ea      string
	eaFault error
	// cs is the run's complex-script font, which draws its right-to-left
	// characters, as ea does East Asian ones. Where its styles name a theme
	// font the theme leaves empty, csList is the theme's font list, to be
	// searched by the script of the characters.
	cs      string
	csFault error
	csList  *dml.FontCollection
	// underline and strike are lines drawn, approximately, under and
	// through the run: 0 none, 1 single, 2 double.
	underline, strike int
	// caps draws the run's letters capitalized.
	caps bool
}

// renderShaping is the part of a run style that selects and shapes glyphs.
type renderShaping struct {
	font         string
	size         int32 // hundredths of a point
	bold, italic bool
	kern         int32 // smallest kerned size in hundredths of a point; 0 is off
	// spacing is added after each character, in hundredths of a point;
	// baseline raises (or, negative, lowers) the run by thousandths of a
	// percent of its size, drawing it smaller.
	spacing, baseline int32
}

// renderHighlight is a run's text highlight; the zero value is none.
type renderHighlight struct {
	on    bool
	color style.RGBA
}

// renderLayer is one inheritance level. PowerPoint may or may not consult an
// optional layer, so its value must agree with the value that applies without
// it.
type renderLayer[L any] struct {
	v        L
	optional bool
}

func (t *renderTextStyles) load() error {
	if !t.loaded {
		t.loaded = true
		t.err = t.loadLists()
		if t.err != nil && t.warn != nil && t.ctx.Err() == nil {
			t.warn(t.err)
			t.err = nil
		}
	}
	return t.err
}

func (t *renderTextStyles) loadLists() error {
	if l := t.slide.layout; l != nil && l.master != nil && l.master.masterXML != nil && l.master.masterXML.TxStyles != nil {
		t.other = l.master.masterXML.TxStyles.OtherStyle
	}
	// Load every list before reporting the first problem, so best effort can
	// go on with them.
	var first error
	if err := t.masterErrs["otherStyle"]; err != nil {
		first = fmt.Errorf("pptx: master other-text style: %w", err)
	}
	p := t.slide.presentation
	if pres := p.presentation; pres != nil && pres.DefaultTextStyle != nil {
		t.defaults = pres.DefaultTextStyle
		if len(pres.SourceXML) > 0 && first == nil {
			if err := t.budget.CheckXML(t.ctx, pres.SourceXML, renderDefaultTextStyleProfile); err != nil {
				first = fmt.Errorf("pptx: presentation default text style: %w", err)
			}
		}
	} else if p.reader == nil {
		// A deck built in code is saved with a synthesized default style.
		var err error
		if t.defaults, err = renderSynthesizedTextStyle(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// renderDefaultTextStyleProfile checks the presentation's default text style
// source with the slide text profile; other presentation content is not read.
func renderDefaultTextStyleProfile(node core.XMLNode) error {
	if len(node.Path) == 1 {
		if node.Name.Space != nsP || node.Name.Local != "presentation" {
			return fmt.Errorf("%w: presentation XML root", render.ErrInvalid)
		}
		return nil
	}
	if node.Path[1] != (xml.Name{Space: nsP, Local: "defaultTextStyle"}) {
		return nil
	}
	if len(node.Path) == 2 {
		if node.Occurrence > 1 {
			return fmt.Errorf("%w: repeated default text style", render.ErrInvalid)
		}
		return nil
	}
	return slideRenderNode(node)
}

var renderSynthesized struct {
	once  sync.Once
	style *dml.LstStyle
	err   error
}

// renderSynthesizedTextStyle parses the default text style a save writes for a
// deck built in code.
func renderSynthesizedTextStyle() (*dml.LstStyle, error) {
	renderSynthesized.once.Do(func() {
		b := xmlb.NewPresentationMLBuilder()
		marshalDefaultTextStyle(b)
		if err := b.Finish(); err != nil {
			renderSynthesized.err = err
			return
		}
		var wrapper struct {
			Style *dml.LstStyle `xml:"http://schemas.openxmlformats.org/presentationml/2006/main defaultTextStyle"`
		}
		data := `<w xmlns:a="` + nsA + `" xmlns:p="` + nsP + `">` + string(b.Bytes()) + `</w>`
		if err := xmlb.UnmarshalWithSource([]byte(data), &wrapper); err != nil {
			renderSynthesized.err = err
			return
		}
		renderSynthesized.style = wrapper.Style
	})
	if renderSynthesized.err != nil {
		return nil, fmt.Errorf("%w: default text style: %w", render.ErrInvalid, renderSynthesized.err)
	}
	return renderSynthesized.style, nil
}

func renderLevel(l *dml.LstStyle, level int) *dml.PPr {
	if l == nil {
		return nil
	}
	return [...]*dml.PPr{l.Lvl1pPr, l.Lvl2pPr, l.Lvl3pPr, l.Lvl4pPr, l.Lvl5pPr, l.Lvl6pPr, l.Lvl7pPr, l.Lvl8pPr, l.Lvl9pPr}[level]
}

func renderDefPPr(l *dml.LstStyle) *dml.PPr {
	if l == nil {
		return nil
	}
	return l.DefPPr
}

// renderListChain is where a text body's paragraphs inherit from beyond its
// own list style: the list styles in between, nearest first, and the
// candidate document defaults, every one of which must agree.
type renderListChain struct {
	inherited []*dml.LstStyle
	variants  [][]*dml.LstStyle
}

// shapeChain is the chain of non-placeholder text.
func (t *renderTextStyles) shapeChain() renderListChain {
	return renderListChain{variants: [][]*dml.LstStyle{{t.other, t.defaults}, {t.defaults}}}
}

// paragraphLayers returns, for each candidate source of document defaults, a
// paragraph's property layers, nearest first.
func (t *renderTextStyles) paragraphLayers(body *dml.TxBody, p *dml.P, chain renderListChain) ([][]renderLayer[*dml.PPr], error) {
	level := 0
	if p.PPr != nil && p.PPr.Lvl != nil {
		if *p.PPr.Lvl < 0 || *p.PPr.Lvl > 8 {
			return nil, fmt.Errorf("%w: paragraph level", render.ErrInvalid)
		}
		level = int(*p.PPr.Lvl)
	}
	list := func(l *dml.LstStyle) []renderLayer[*dml.PPr] {
		return []renderLayer[*dml.PPr]{{v: renderLevel(l, level)}, {v: renderDefPPr(l), optional: true}}
	}
	shape := append([]renderLayer[*dml.PPr]{{v: p.PPr}}, list(body.LstStyle)...)
	for _, l := range chain.inherited {
		shape = append(shape, list(l)...)
	}
	variants := make([][]renderLayer[*dml.PPr], 0, len(chain.variants))
	for _, v := range chain.variants {
		layers := append([]renderLayer[*dml.PPr]{}, shape...)
		for _, l := range v {
			layers = append(layers, list(l)...)
		}
		variants = append(variants, layers)
	}
	return variants, nil
}

// runLayers maps paragraph layers to run property layers below a run's own
// properties. PowerPoint may ignore the paragraph's own default run properties.
func runLayers(paragraph [][]renderLayer[*dml.PPr], own *dml.RPr) [][]renderLayer[*dml.RPr] {
	variants := make([][]renderLayer[*dml.RPr], 0, len(paragraph))
	for _, layers := range paragraph {
		out := make([]renderLayer[*dml.RPr], 0, len(layers)+1)
		out = append(out, renderLayer[*dml.RPr]{v: own})
		for i, l := range layers {
			var def *dml.RPr
			if l.v != nil {
				def = l.v.DefRPr
			}
			out = append(out, renderLayer[*dml.RPr]{v: def, optional: l.optional || i == 0})
		}
		variants = append(variants, out)
	}
	return variants
}

// renderInherit resolves one property: the nearest required layer that sets
// it, or fallback. Every optional layer consulted on the way and every variant
// must agree with the result.
//
// approx is the best-effort hook: where readings disagree and it accepts the
// problem, the first reading, of the nearest variant, is used.
func renderInherit[L any, T comparable](approx func(error) error, name string, variants [][]renderLayer[L], get func(L) (T, bool, error), fallback func() (T, error)) (T, error) {
	var (
		zero, out T
		have      bool
		disagree  bool
	)
	agree := func(v T) error {
		if have && v != out {
			if disagree {
				return nil
			}
			if err := approx(fmt.Errorf("%w: ambiguous inherited %s", render.ErrUnsupported, name)); err != nil {
				return err
			}
			disagree = true
			return nil
		}
		if !have {
			out, have = v, true
		}
		return nil
	}
	for _, layers := range variants {
		required := false
		for _, l := range layers {
			v, ok, err := get(l.v)
			if err != nil {
				return zero, err
			}
			if !ok {
				continue
			}
			if err = agree(v); err != nil {
				return zero, err
			}
			if !l.optional {
				required = true
				break
			}
		}
		if !required {
			v, err := fallback()
			if err != nil {
				return zero, err
			}
			if err = agree(v); err != nil {
				return zero, err
			}
		}
	}
	return out, nil
}

func renderBuiltin[T any](v T) func() (T, error) {
	return func() (T, error) { return v, nil }
}

// paragraph resolves a paragraph's layout properties and returns the layers
// its runs inherit through.
func (t *renderTextStyles) paragraph(body *dml.TxBody, p *dml.P, chain renderListChain) (renderParaStyle, [][]renderLayer[*dml.PPr], error) {
	var s renderParaStyle
	layers, err := t.paragraphLayers(body, p, chain)
	if err != nil {
		return s, nil, err
	}
	if s.align, err = renderInherit(t.colors.approximate, "alignment", layers, func(pp *dml.PPr) (enum.TextAlign, bool, error) {
		if pp == nil || pp.Algn == "" {
			return "", false, nil
		}
		return enum.TextAlign(pp.Algn), true, nil
	}, renderBuiltin(enum.TextAlignLeft)); err != nil {
		return s, nil, err
	}
	switch s.align {
	case enum.TextAlignLeft, enum.TextAlignCenter, enum.TextAlignRight, enum.TextAlignJustify, enum.TextAlignDistribute:
	case enum.TextAlignJustifyLow:
		// Low kashida justification differs only in Arabic.
		s.align, s.kashida = enum.TextAlignJustify, true
	case enum.TextAlignThaiDistribute:
		// Thai distribution differs only in Thai, likewise.
		s.align = enum.TextAlignDistribute
	default:
		if err = t.colors.approximate(fmt.Errorf("%w: unknown text alignment drawn left aligned", render.ErrUnsupported)); err != nil {
			return s, nil, err
		}
		s.align = enum.TextAlignLeft
	}
	// A line spacing is a percentage of the line or, with spcPts, a height.
	type lineSpacing struct {
		pct   int32
		fixed dml.EMU
	}
	ls, err := renderInherit(t.colors.approximate, "line spacing", layers, func(pp *dml.PPr) (lineSpacing, bool, error) {
		if pp == nil || pp.LnSpc == nil {
			return lineSpacing{}, false, nil
		}
		if pp.LnSpc.SpcPct == nil && pp.LnSpc.SpcPts != nil && pp.LnSpc.SpcPts.Val > 0 {
			// spcPts is in hundredths of a point; a point is 12700 EMU.
			return lineSpacing{pct: 100000, fixed: dml.EMU(pp.LnSpc.SpcPts.Val) * 127}, true, nil
		}
		if pp.LnSpc.SpcPct != nil && pp.LnSpc.SpcPts == nil {
			if v := pp.LnSpc.SpcPct.Val.Int32(); v > 0 {
				return lineSpacing{pct: v}, true, nil
			}
		}
		return lineSpacing{}, false, fmt.Errorf("%w: line spacing", render.ErrInvalid)
	}, renderBuiltin(lineSpacing{pct: 100000}))
	if err != nil {
		return s, nil, err
	}
	s.lineSpacing, s.lineFixed = ls.pct, ls.fixed
	if s.lineFixed > 0 {
		if err = t.colors.approximate(fmt.Errorf("%w: exact line spacing placed approximately", render.ErrUnsupported)); err != nil {
			return s, nil, err
		}
	}
	// A body scaled to fit reduces percentage line spacing.
	if s.lineFixed == 0 && t.lnSpcReduction > 0 {
		s.lineSpacing = max(s.lineSpacing-t.lnSpcReduction, 1000)
	}
	// A paragraph space is in points or, approximately, a percentage of a
	// line.
	type space struct {
		pts dml.EMU
		pct int32
	}
	spacing := func(name string, pick func(*dml.PPr) (*dml.SpcPct, *dml.SpcPts, bool)) (space, error) {
		v, err := renderInherit(t.colors.approximate, name, layers, func(pp *dml.PPr) (space, bool, error) {
			if pp == nil {
				return space{}, false, nil
			}
			pct, pts, set := pick(pp)
			switch {
			case !set:
				return space{}, false, nil
			case pct != nil && pts == nil && pct.Val.Int32() >= 0:
				return space{pct: pct.Val.Int32()}, true, nil
			case pct != nil || pts == nil || pts.Val < 0:
				return space{}, false, fmt.Errorf("%w: %s", render.ErrInvalid, name)
			}
			// spcPts is in hundredths of a point; a point is 12700 EMU.
			return space{pts: dml.EMU(pts.Val) * 127}, true, nil
		}, renderBuiltin(space{}))
		if err == nil && v.pct != 0 {
			err = t.colors.approximate(fmt.Errorf("%w: %s in percent of a line placed approximately", render.ErrUnsupported, name))
		}
		return v, err
	}
	before, err := spacing("space before", func(pp *dml.PPr) (*dml.SpcPct, *dml.SpcPts, bool) {
		if pp.SpcBef == nil {
			return nil, nil, false
		}
		return pp.SpcBef.SpcPct, pp.SpcBef.SpcPts, true
	})
	if err != nil {
		return s, nil, err
	}
	after, err := spacing("space after", func(pp *dml.PPr) (*dml.SpcPct, *dml.SpcPts, bool) {
		if pp.SpcAft == nil {
			return nil, nil, false
		}
		return pp.SpcAft.SpcPct, pp.SpcAft.SpcPts, true
	})
	if err != nil {
		return s, nil, err
	}
	s.before, s.beforePct, s.after, s.afterPct = before.pts, before.pct, after.pts, after.pct
	tab, err := renderInherit(t.colors.approximate, "tab size", layers, func(pp *dml.PPr) (int32, bool, error) {
		if pp == nil || pp.DefTabSz == nil {
			return 0, false, nil
		}
		if *pp.DefTabSz < 0 {
			return 0, false, fmt.Errorf("%w: tab size", render.ErrInvalid)
		}
		return *pp.DefTabSz, true, nil
	}, renderBuiltin(int32(914400)))
	if err != nil {
		return s, nil, err
	}
	s.tabSize = dml.EMU(tab)
	if s.tabs, err = renderInherit(t.colors.approximate, "tab stops", layers, func(pp *dml.PPr) (*dml.TabLst, bool, error) {
		if pp == nil || pp.TabLst == nil {
			return nil, false, nil
		}
		return pp.TabLst, true, nil
	}, renderBuiltin[*dml.TabLst](nil)); err != nil {
		return s, nil, err
	}
	margin := func(name string, pick func(*dml.PPr) *int32) (int32, error) {
		return renderInherit(t.colors.approximate, name, layers, func(pp *dml.PPr) (int32, bool, error) {
			if pp == nil || pick(pp) == nil {
				return 0, false, nil
			}
			return *pick(pp), true, nil
		}, renderBuiltin(int32(0)))
	}
	marL, err := margin("left margin", func(pp *dml.PPr) *int32 { return pp.MarL })
	if err != nil {
		return s, nil, err
	}
	marR, err := margin("right margin", func(pp *dml.PPr) *int32 { return pp.MarR })
	if err != nil {
		return s, nil, err
	}
	indent, err := margin("indent", func(pp *dml.PPr) *int32 { return pp.Indent })
	if err != nil {
		return s, nil, err
	}
	if marL < 0 || marR < 0 || marL+indent < 0 {
		return s, nil, fmt.Errorf("%w: paragraph indentation", render.ErrUnsupported)
	}
	s.marL, s.marR, s.indent = dml.EMU(marL), dml.EMU(marR), dml.EMU(indent)
	if s.bullet, err = t.bullet(layers); err != nil {
		return s, nil, err
	}
	flag := func(name string, pick func(*dml.PPr) *bool) (bool, error) {
		return renderInherit(t.colors.approximate, name, layers, func(pp *dml.PPr) (bool, bool, error) {
			if pp == nil || pick(pp) == nil {
				return false, false, nil
			}
			return *pick(pp), true, nil
		}, renderBuiltin(false))
	}
	rtl, err := flag("direction", func(pp *dml.PPr) *bool { return pp.Rtl })
	if err != nil {
		return s, nil, err
	}
	// PowerPoint writes both of these on every level of its own styles. An
	// absent value takes its application default: East Asian text wraps by
	// East Asian rules, and punctuation hangs.
	flagOr := func(name string, pick func(*dml.PPr) *bool) (bool, error) {
		return renderInherit(t.colors.approximate, name, layers, func(pp *dml.PPr) (bool, bool, error) {
			if pp == nil || pick(pp) == nil {
				return false, false, nil
			}
			return *pick(pp), true, nil
		}, renderBuiltin(true))
	}
	if s.eaBreak, err = flagOr("East Asian line breaking", func(pp *dml.PPr) *bool { return pp.EaLnBrk }); err != nil {
		return s, nil, err
	}
	if s.hangPunct, err = flagOr("hanging punctuation", func(pp *dml.PPr) *bool { return pp.HangingPunct }); err != nil {
		return s, nil, err
	}
	// Breaking Latin words anywhere would change wrapping.
	latinBreak, err := flag("Latin line breaking", func(pp *dml.PPr) *bool { return pp.LatinLnBrk })
	if err != nil {
		return s, nil, err
	}
	if latinBreak {
		return s, nil, fmt.Errorf("%w: Latin-break paragraph", render.ErrUnsupported)
	}
	s.rtl = rtl
	return s, layers, nil
}

// run resolves a run's appearance through the paragraph's layers.
func (t *renderTextStyles) run(paragraph [][]renderLayer[*dml.PPr], own *dml.RPr) (renderRunStyle, error) {
	var s renderRunStyle
	if own != nil && (own.Rtl != nil || own.ExtLst != nil) {
		return s, fmt.Errorf("%w: right-to-left or extended run", render.ErrUnsupported)
	}
	if own != nil && (own.HlinkClick != nil || own.HlinkMouseOver != nil) {
		if err := t.colors.approximate(fmt.Errorf("%w: hyperlink drawn as plain text", render.ErrUnsupported)); err != nil {
			return s, err
		}
	}
	layers := runLayers(paragraph, own)
	var err error
	if s.eastAsian, err = renderInherit(t.colors.approximate, "language", layers, func(r *dml.RPr) (bool, bool, error) {
		if r == nil || r.Lang == "" {
			return false, false, nil
		}
		return renderEastAsian(r.Lang), true, nil
	}, renderBuiltin(false)); err != nil {
		return s, err
	}
	if own != nil && renderEastAsian(own.AltLang) {
		s.eastAsian = true
	}
	// The language picks among the theme's fonts by script, for a font slot
	// the theme leaves empty. Layers that name different languages need not
	// agree: only a font slot that is empty reads it.
	script, err := renderInherit(func(error) error { return nil }, "language script", layers, func(r *dml.RPr) (string, bool, error) {
		if r == nil || r.Lang == "" {
			return "", false, nil
		}
		return renderLangScript(r.Lang), true, nil
	}, renderBuiltin(""))
	if err != nil {
		return s, err
	}
	if script == "" && own != nil {
		script = renderLangScript(own.AltLang)
	}
	// Painting properties this profile does not draw fail wherever a resolved
	// layer sets them.
	effects := false
	if _, err = renderInherit(t.colors.approximate, "text effects", layers, func(r *dml.RPr) (bool, bool, error) {
		if r != nil && (renderEffects(r.EffectLst) || r.EffectDag != nil || (r.Ln != nil && (r.Ln.NoFill == nil || r.Ln.SolidFill != nil || r.Ln.GradFill != nil || r.Ln.PattFill != nil))) {
			effects = true
		}
		return false, false, nil
	}, renderBuiltin(false)); err != nil {
		return s, err
	}
	if effects {
		if err = t.colors.approximate(fmt.Errorf("%w: text effects or outline left out", render.ErrUnsupported)); err != nil {
			return s, err
		}
	}
	if s.size, err = renderInherit(t.colors.approximate, "font size", layers, func(r *dml.RPr) (int32, bool, error) {
		if r == nil || r.Sz == 0 {
			return 0, false, nil
		}
		return r.Sz, true, nil
	}, renderBuiltin(int32(1800))); err != nil {
		return s, err
	}
	if t.fontScale > 0 && t.fontScale != 100000 {
		s.size = max(1, int32(math.Round(float64(s.size)*float64(t.fontScale)/100000)))
	}
	flag := func(name string, pick func(*dml.RPr) *bool) (bool, error) {
		return renderInherit(t.colors.approximate, name, layers, func(r *dml.RPr) (bool, bool, error) {
			if r == nil || pick(r) == nil {
				return false, false, nil
			}
			return *pick(r), true, nil
		}, renderBuiltin(false))
	}
	if s.bold, err = flag("bold", func(r *dml.RPr) *bool { return r.B }); err != nil {
		return s, err
	}
	if s.italic, err = flag("italic", func(r *dml.RPr) *bool { return r.I }); err != nil {
		return s, err
	}
	word := func(name, plain string, pick func(*dml.RPr) string) (string, error) {
		return renderInherit(t.colors.approximate, name, layers, func(r *dml.RPr) (string, bool, error) {
			if r == nil || pick(r) == "" {
				return "", false, nil
			}
			return pick(r), true, nil
		}, renderBuiltin(plain))
	}
	u, err := word("underline", "none", func(r *dml.RPr) string { return r.U })
	if err != nil {
		return s, err
	}
	strike, err := word("strikethrough", "noStrike", func(r *dml.RPr) string { return r.Strike })
	if err != nil {
		return s, err
	}
	capital, err := word("capitalization", "none", func(r *dml.RPr) string { return r.Cap })
	if err != nil {
		return s, err
	}
	// Lines are placed from the font size, not the font's own metrics;
	// styled underlines (dotted, wavy, …) draw solid.
	switch u {
	case "none":
	case "dbl", "wavyDbl":
		s.underline = 2
	default:
		s.underline = 1
	}
	switch strike {
	case "noStrike":
	case "dblStrike":
		s.strike = 2
	case "sngStrike":
		s.strike = 1
	default:
		return s, fmt.Errorf("%w: strikethrough", render.ErrInvalid)
	}
	if s.underline > 0 || s.strike > 0 {
		if err = t.colors.approximate(fmt.Errorf("%w: underline or strikethrough placed approximately", render.ErrUnsupported)); err != nil {
			return s, err
		}
	}
	switch capital {
	case "none":
	case "all":
		s.caps = true
	case "small":
		if err = t.colors.approximate(fmt.Errorf("%w: small capitals drawn as capitals", render.ErrUnsupported)); err != nil {
			return s, err
		}
		s.caps = true
	default:
		return s, fmt.Errorf("%w: capitalization", render.ErrInvalid)
	}
	number := func(name string, pick func(*dml.RPr) (int32, bool)) (int32, error) {
		return renderInherit(t.colors.approximate, name, layers, func(r *dml.RPr) (int32, bool, error) {
			if r == nil {
				return 0, false, nil
			}
			v, ok := pick(r)
			return v, ok, nil
		}, renderBuiltin(int32(0)))
	}
	baseline, err := number("baseline", func(r *dml.RPr) (int32, bool) {
		if r.Baseline == nil {
			return 0, false
		}
		return r.Baseline.Int32(), true
	})
	if err != nil {
		return s, err
	}
	spacing, err := number("character spacing", func(r *dml.RPr) (int32, bool) {
		if r.Spc == nil {
			return 0, false
		}
		return *r.Spc, true
	})
	if err != nil {
		return s, err
	}
	if spacing < -400000 || spacing > 400000 || baseline < -1000000 || baseline > 1000000 {
		return s, fmt.Errorf("%w: character spacing or baseline", render.ErrInvalid)
	}
	s.spacing, s.baseline = spacing, baseline
	if baseline != 0 {
		// PowerPoint's size for raised and lowered text is undocumented;
		// best effort uses LibreOffice's 58%.
		if err = t.colors.approximate(fmt.Errorf("%w: superscript or subscript size approximated", render.ErrUnsupported)); err != nil {
			return s, err
		}
	}
	if s.kern, err = number("kerning", func(r *dml.RPr) (int32, bool) {
		if r.Kern == nil {
			return 0, false
		}
		return *r.Kern, true
	}); err != nil {
		return s, err
	}
	if s.kern < 0 {
		return s, fmt.Errorf("%w: kerning size", render.ErrInvalid)
	}
	if s.color, err = renderInherit(t.colors.approximate, "text color", layers, func(r *dml.RPr) (style.RGBA, bool, error) {
		switch {
		case r == nil:
			return style.RGBA{}, false, nil
		case r.NoFill != nil && (r.SolidFill != nil || r.GradFill != nil || r.PattFill != nil):
			return style.RGBA{}, false, fmt.Errorf("%w: ambiguous text fill", render.ErrInvalid)
		case r.NoFill != nil:
			// Unfilled text is invisible.
			return style.RGBA{}, true, nil
		case r.BlipFill != nil || r.GrpFill != nil:
			return style.RGBA{}, false, fmt.Errorf("%w: picture or group text fill", render.ErrUnsupported)
		case r.SolidFill == nil && (r.GradFill != nil || r.PattFill != nil):
			c, err := t.colors.representative(r.GradFill, r.PattFill, nil)
			return c, err == nil, err
		case r.SolidFill != nil:
			c, err := t.colors.solid(r.SolidFill, nil)
			return c, err == nil, err
		}
		return style.RGBA{}, false, nil
	}, func() (style.RGBA, error) {
		return style.RGBA{}, fmt.Errorf("%w: text without a color", render.ErrUnsupported)
	}); err != nil {
		return s, err
	}
	if s.highlight, err = renderInherit(t.colors.approximate, "highlight", layers, func(r *dml.RPr) (renderHighlight, bool, error) {
		if r == nil || r.Highlight == nil {
			return renderHighlight{}, false, nil
		}
		c, err := t.colors.color(renderChoiceColor(r.Highlight), nil)
		return renderHighlight{on: true, color: c}, err == nil, err
	}, renderBuiltin(renderHighlight{})); err != nil {
		return s, err
	}
	if s.font, err = renderInherit(t.colors.approximate, "font", layers, func(r *dml.RPr) (string, bool, error) {
		if r == nil || r.Latin == nil {
			return "", false, nil
		}
		v, err := t.typeface(r.Latin.Typeface)
		return v, err == nil, err
	}, func() (string, error) {
		return "", fmt.Errorf("%w: text without a font", render.ErrUnsupported)
	}); err != nil {
		return s, err
	}
	// A problem with the East Asian font matters only to a run that has East
	// Asian characters, and is raised for them; see eaFault.
	fault := func(err error) error {
		if s.eaFault == nil {
			s.eaFault = err
		}
		return nil
	}
	if s.ea, err = renderInherit(fault, "East Asian font", layers, func(r *dml.RPr) (string, bool, error) {
		if r == nil || r.Ea == nil {
			return "", false, nil
		}
		v, _, err := t.slotTypeface(r.Ea.Typeface, "ea", script)
		if err != nil {
			return "", false, fault(err)
		}
		return v, true, nil
	}, renderBuiltin("")); err != nil {
		return s, err
	}
	csFault := func(err error) error {
		if s.csFault == nil {
			s.csFault = err
		}
		return nil
	}
	if s.cs, err = renderInherit(csFault, "complex-script font", layers, func(r *dml.RPr) (string, bool, error) {
		if r == nil || r.Cs == nil {
			return "", false, nil
		}
		v, list, err := t.slotTypeface(r.Cs.Typeface, "cs", script)
		if err != nil {
			return "", false, csFault(err)
		}
		if v == "" {
			s.csList = list
		}
		return v, true, nil
	}, renderBuiltin("")); err != nil {
		return s, err
	}
	return s, nil
}

// typeface resolves a theme font reference.
func (t *renderTextStyles) typeface(name string) (string, error) {
	if name == "+mn-lt" || name == "+mj-lt" {
		theme, err := t.colors.loadTheme()
		if err != nil {
			return "", err
		}
		var fonts *dml.FontCollection
		if e := theme.ThemeElements; e != nil && e.FontScheme != nil {
			fonts = e.FontScheme.MinorFont
			if name == "+mj-lt" {
				fonts = e.FontScheme.MajorFont
			}
		}
		if fonts == nil || fonts.Latin == nil {
			return "", fmt.Errorf("%w: theme font %s", render.ErrUnsupported, name)
		}
		name = fonts.Latin.Typeface
	}
	switch {
	case name == "" || strings.HasPrefix(name, "+"):
		return "", fmt.Errorf("%w: font family %q", render.ErrUnsupported, name)
	case len(name) > 1024:
		return "", fmt.Errorf("%w: font family name", render.ErrLimit)
	}
	return name, nil
}

// slotTypeface resolves a run's East Asian ("ea") or complex-script ("cs")
// font slot, which may be a theme reference. A slot the theme leaves empty
// takes the theme's font for the run's script, if the list names one, and
// otherwise is empty, with the theme's font list returned for a search by the
// script of the text.
func (t *renderTextStyles) slotTypeface(name, slot, script string) (string, *dml.FontCollection, error) {
	major := false
	switch {
	case name == "+mn-"+slot:
	case name == "+mj-"+slot:
		major = true
	case strings.HasPrefix(name, "+"):
		return "", nil, fmt.Errorf("%w: font family %q", render.ErrUnsupported, name)
	case len(name) > 1024:
		return "", nil, fmt.Errorf("%w: font family name", render.ErrLimit)
	default:
		return name, nil, nil
	}
	theme, err := t.colors.loadTheme()
	if err != nil {
		return "", nil, err
	}
	var fonts *dml.FontCollection
	if e := theme.ThemeElements; e != nil && e.FontScheme != nil {
		fonts = e.FontScheme.MinorFont
		if major {
			fonts = e.FontScheme.MajorFont
		}
	}
	if fonts == nil {
		return "", nil, fmt.Errorf("%w: theme font %s", render.ErrUnsupported, name)
	}
	face := fonts.Ea
	if slot == "cs" {
		face = fonts.Cs
	}
	if face != nil && face.Typeface != "" {
		if len(face.Typeface) > 1024 {
			return "", nil, fmt.Errorf("%w: font family name", render.ErrLimit)
		}
		return face.Typeface, nil, nil
	}
	v, err := renderScriptFont(fonts, slot, script)
	return v, fonts, err
}

// renderScriptFont is the typeface a theme's font list gives a script, for
// the slot it belongs to, or empty.
func renderScriptFont(fonts *dml.FontCollection, slot, script string) (string, error) {
	if script == "" || fonts == nil || (slot == "ea") != renderEastAsianScript(script) {
		return "", nil
	}
	for _, f := range fonts.Font {
		if f != nil && f.Script == script && f.Typeface != "" {
			if len(f.Typeface) > 1024 {
				return "", fmt.Errorf("%w: font family name", render.ErrLimit)
			}
			return f.Typeface, nil
		}
	}
	return "", nil
}

// renderEastAsianScript reports whether an ISO 15924 script, as a theme's font
// lists name it, belongs to the East Asian font slot rather than the
// complex-script one.
func renderEastAsianScript(script string) bool {
	switch script {
	case "Jpan", "Hang", "Hans", "Hant":
		return true
	}
	return false
}

// renderLangScript is the ISO 15924 script, as a theme's font lists name it,
// of the languages written in the scripts this profile draws with the East
// Asian and complex-script fonts; empty for any other.
func renderLangScript(tag string) string {
	primary, rest, _ := strings.Cut(strings.ToLower(tag), "-")
	switch primary {
	case "ja":
		return "Jpan"
	case "ko":
		return "Hang"
	case "zh":
		for _, sub := range strings.Split(rest, "-") {
			switch sub {
			case "hant", "tw", "hk", "mo":
				return "Hant"
			}
		}
		return "Hans"
	case "ar", "fa", "ur", "ps", "sd", "ug", "ckb", "ks", "azb":
		return "Arab"
	case "he", "yi":
		return "Hebr"
	case "dv":
		return "Thaa"
	case "syr":
		return "Syrc"
	case "nqo":
		return "Nkoo"
	}
	return ""
}

// renderEastAsian reports whether a language tag names Chinese, Japanese or
// Korean.
func renderEastAsian(tag string) bool {
	primary, _, _ := strings.Cut(strings.ToLower(tag), "-")
	switch primary {
	case "zh", "ja", "ko":
		return true
	}
	return false
}

// bullet resolves a paragraph's bullet. Each bullet property inherits on its
// own. Picture bullets fail, and best effort draws them as "•".
func (t *renderTextStyles) bullet(layers [][]renderLayer[*dml.PPr]) (renderBullet, error) {
	var b renderBullet
	type kind struct {
		char, autoNum string
		startAt       int32
	}
	k, err := renderInherit(t.colors.approximate, "bullet", layers, func(pp *dml.PPr) (kind, bool, error) {
		switch {
		case pp == nil:
			return kind{}, false, nil
		case pp.BuNone != nil:
			return kind{}, true, nil
		case pp.BuChar != nil:
			return kind{char: pp.BuChar.Char}, true, nil
		case pp.BuAutoNum != nil:
			start := pp.BuAutoNum.StartAt
			if start == 0 {
				start = 1
			}
			if start < 1 || start > 32767 {
				return kind{}, false, fmt.Errorf("%w: numbering start", render.ErrInvalid)
			}
			return kind{autoNum: pp.BuAutoNum.Type, startAt: start}, true, nil
		case pp.BuBlip != nil:
			if err := t.colors.approximate(fmt.Errorf("%w: picture bullet drawn as a dot", render.ErrUnsupported)); err != nil {
				return kind{}, false, err
			}
			return kind{char: "•"}, true, nil
		}
		return kind{}, false, nil
	}, renderBuiltin(kind{}))
	if err != nil || (k.char == "" && k.autoNum == "") {
		return b, err
	}
	b.char, b.autoNum, b.startAt = k.char, k.autoNum, k.startAt
	if b.font, err = renderInherit(t.colors.approximate, "bullet font", layers, func(pp *dml.PPr) (string, bool, error) {
		switch {
		case pp == nil:
			return "", false, nil
		case pp.BuFontTx != nil:
			return "", true, nil
		case pp.BuFont != nil:
			v, err := t.typeface(pp.BuFont.Typeface)
			return v, err == nil, err
		}
		return "", false, nil
	}, renderBuiltin("")); err != nil {
		return b, err
	}
	type size struct{ pct, pts int32 }
	sz, err := renderInherit(t.colors.approximate, "bullet size", layers, func(pp *dml.PPr) (size, bool, error) {
		switch {
		case pp == nil:
			return size{}, false, nil
		case pp.BuSzTx != nil:
			return size{pct: 100000}, true, nil
		case pp.BuSzPct != nil:
			if v := pp.BuSzPct.Val.Int32(); v > 0 {
				return size{pct: v}, true, nil
			}
			return size{}, false, fmt.Errorf("%w: bullet size", render.ErrInvalid)
		case pp.BuSzPts != nil:
			if pp.BuSzPts.Val > 0 {
				return size{pts: pp.BuSzPts.Val}, true, nil
			}
			return size{}, false, fmt.Errorf("%w: bullet size", render.ErrInvalid)
		}
		return size{}, false, nil
	}, renderBuiltin(size{pct: 100000}))
	if err != nil {
		return b, err
	}
	b.size, b.points = sz.pct, sz.pts
	type paint struct {
		own bool
		c   style.RGBA
	}
	color, err := renderInherit(t.colors.approximate, "bullet color", layers, func(pp *dml.PPr) (paint, bool, error) {
		switch {
		case pp == nil:
			return paint{}, false, nil
		case pp.BuClrTx != nil:
			return paint{}, true, nil
		case pp.BuClr != nil:
			c := pp.BuClr
			v, err := t.colors.color(renderColorOf(c.SrgbClr, c.SchemeClr, c.SysClr, c.PrstClr, c.ScRgbClr != nil, c.HslClr != nil), nil)
			return paint{own: true, c: v}, err == nil, err
		}
		return paint{}, false, nil
	}, renderBuiltin(paint{}))
	if err != nil {
		return b, err
	}
	b.ownColor, b.color = color.own, color.c
	return b, nil
}
