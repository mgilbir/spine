package pptx

import (
	"context"
	"encoding/xml"
	"fmt"
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
	slide  *Slide
	budget *core.SourceBudget
	colors *renderColors
	// masterErrs holds the first unsupported node the master source check
	// found in each of its text styles, by element name.
	masterErrs map[string]error

	loaded          bool
	err             error
	other, defaults *dml.LstStyle
}

// renderParaStyle is a paragraph's resolved layout.
type renderParaStyle struct {
	align         enum.TextAlign
	lineSpacing   int32 // 100000 is 100%
	before, after dml.EMU
	marL, marR    dml.EMU
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
}

// renderShaping is the part of a run style that selects and shapes glyphs.
type renderShaping struct {
	font         string
	size         int32 // hundredths of a point
	bold, italic bool
	kern         int32 // smallest kerned size in hundredths of a point; 0 is off
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
	}
	return t.err
}

func (t *renderTextStyles) loadLists() error {
	if err := t.masterErrs["otherStyle"]; err != nil {
		return fmt.Errorf("pptx: master other-text style: %w", err)
	}
	if l := t.slide.layout; l != nil && l.master != nil && l.master.masterXML != nil && l.master.masterXML.TxStyles != nil {
		t.other = l.master.masterXML.TxStyles.OtherStyle
	}
	p := t.slide.presentation
	if pres := p.presentation; pres != nil && pres.DefaultTextStyle != nil {
		t.defaults = pres.DefaultTextStyle
		if len(pres.SourceXML) > 0 {
			if err := t.budget.CheckXML(t.ctx, pres.SourceXML, renderDefaultTextStyleProfile); err != nil {
				return fmt.Errorf("pptx: presentation default text style: %w", err)
			}
		}
	} else if p.reader == nil {
		// A deck built in code is saved with a synthesized default style.
		var err error
		if t.defaults, err = renderSynthesizedTextStyle(); err != nil {
			return err
		}
	}
	return nil
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
func renderInherit[L any, T comparable](name string, variants [][]renderLayer[L], get func(L) (T, bool, error), fallback func() (T, error)) (T, error) {
	var (
		zero, out T
		have      bool
	)
	agree := func(v T) error {
		if have && v != out {
			return fmt.Errorf("%w: ambiguous inherited %s", render.ErrUnsupported, name)
		}
		out, have = v, true
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
	if s.align, err = renderInherit("alignment", layers, func(pp *dml.PPr) (enum.TextAlign, bool, error) {
		if pp == nil || pp.Algn == "" {
			return "", false, nil
		}
		return enum.TextAlign(pp.Algn), true, nil
	}, renderBuiltin(enum.TextAlignLeft)); err != nil {
		return s, nil, err
	}
	if s.align != enum.TextAlignLeft && s.align != enum.TextAlignCenter && s.align != enum.TextAlignRight {
		return s, nil, fmt.Errorf("%w: paragraph alignment", render.ErrUnsupported)
	}
	if s.lineSpacing, err = renderInherit("line spacing", layers, func(pp *dml.PPr) (int32, bool, error) {
		if pp == nil || pp.LnSpc == nil {
			return 0, false, nil
		}
		if pp.LnSpc.SpcPct == nil {
			return 0, false, fmt.Errorf("%w: fixed line spacing", render.ErrUnsupported)
		}
		if v := pp.LnSpc.SpcPct.Val.Int32(); v > 0 {
			return v, true, nil
		}
		return 0, false, fmt.Errorf("%w: line spacing", render.ErrInvalid)
	}, renderBuiltin(int32(100000))); err != nil {
		return s, nil, err
	}
	spacing := func(name string, pick func(*dml.PPr) (*dml.SpcPct, *dml.SpcPts, bool)) (dml.EMU, error) {
		return renderInherit(name, layers, func(pp *dml.PPr) (dml.EMU, bool, error) {
			if pp == nil {
				return 0, false, nil
			}
			pct, pts, set := pick(pp)
			switch {
			case !set:
				return 0, false, nil
			case pct != nil && pts == nil && pct.Val.Int32() == 0:
				// A percentage of the line is undocumented, but none is none.
				return 0, true, nil
			case pct != nil || pts == nil:
				return 0, false, fmt.Errorf("%w: percentage %s", render.ErrUnsupported, name)
			case pts.Val < 0:
				return 0, false, fmt.Errorf("%w: %s", render.ErrInvalid, name)
			}
			// spcPts is in hundredths of a point; a point is 12700 EMU.
			return dml.EMU(pts.Val) * 127, true, nil
		}, renderBuiltin(dml.EMU(0)))
	}
	if s.before, err = spacing("space before", func(pp *dml.PPr) (*dml.SpcPct, *dml.SpcPts, bool) {
		if pp.SpcBef == nil {
			return nil, nil, false
		}
		return pp.SpcBef.SpcPct, pp.SpcBef.SpcPts, true
	}); err != nil {
		return s, nil, err
	}
	if s.after, err = spacing("space after", func(pp *dml.PPr) (*dml.SpcPct, *dml.SpcPts, bool) {
		if pp.SpcAft == nil {
			return nil, nil, false
		}
		return pp.SpcAft.SpcPct, pp.SpcAft.SpcPts, true
	}); err != nil {
		return s, nil, err
	}
	margin := func(name string, pick func(*dml.PPr) *int32) (int32, error) {
		return renderInherit(name, layers, func(pp *dml.PPr) (int32, bool, error) {
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
	if marL < 0 || marR < 0 || indent != 0 {
		return s, nil, fmt.Errorf("%w: paragraph indentation", render.ErrUnsupported)
	}
	s.marL, s.marR = dml.EMU(marL), dml.EMU(marR)
	bullet, err := renderInherit("bullet", layers, func(pp *dml.PPr) (bool, bool, error) {
		switch {
		case pp == nil:
			return false, false, nil
		case pp.BuNone != nil:
			return false, true, nil
		case pp.BuChar != nil || pp.BuAutoNum != nil || pp.BuBlip != nil:
			return true, true, nil
		}
		return false, false, nil
	}, renderBuiltin(false))
	if err != nil {
		return s, nil, err
	}
	flag := func(name string, pick func(*dml.PPr) *bool) (bool, error) {
		return renderInherit(name, layers, func(pp *dml.PPr) (bool, bool, error) {
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
	// Breaking Latin words anywhere would change wrapping.
	latinBreak, err := flag("Latin line breaking", func(pp *dml.PPr) *bool { return pp.LatinLnBrk })
	if err != nil {
		return s, nil, err
	}
	if bullet || rtl || latinBreak {
		return s, nil, fmt.Errorf("%w: bullet, right-to-left or Latin-break paragraph", render.ErrUnsupported)
	}
	return s, layers, nil
}

// run resolves a run's appearance through the paragraph's layers.
func (t *renderTextStyles) run(paragraph [][]renderLayer[*dml.PPr], own *dml.RPr) (renderRunStyle, error) {
	var s renderRunStyle
	if own != nil && (own.HlinkClick != nil || own.HlinkMouseOver != nil || own.Rtl != nil || own.ExtLst != nil) {
		return s, fmt.Errorf("%w: hyperlink, right-to-left or extended run", render.ErrUnsupported)
	}
	layers := runLayers(paragraph, own)
	var err error
	if s.eastAsian, err = renderInherit("language", layers, func(r *dml.RPr) (bool, bool, error) {
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
	// Painting properties this profile does not draw fail wherever a resolved
	// layer sets them.
	if _, err = renderInherit("text effects", layers, func(r *dml.RPr) (bool, bool, error) {
		if r != nil && (renderEffects(r.EffectLst) || r.EffectDag != nil || (r.Ln != nil && (r.Ln.NoFill == nil || r.Ln.SolidFill != nil || r.Ln.GradFill != nil || r.Ln.PattFill != nil))) {
			return false, false, fmt.Errorf("%w: text effect or outline", render.ErrUnsupported)
		}
		return false, false, nil
	}, renderBuiltin(false)); err != nil {
		return s, err
	}
	if s.size, err = renderInherit("font size", layers, func(r *dml.RPr) (int32, bool, error) {
		if r == nil || r.Sz == 0 {
			return 0, false, nil
		}
		return r.Sz, true, nil
	}, renderBuiltin(int32(1800))); err != nil {
		return s, err
	}
	flag := func(name string, pick func(*dml.RPr) *bool) (bool, error) {
		return renderInherit(name, layers, func(r *dml.RPr) (bool, bool, error) {
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
	word := func(name, plain string, pick func(*dml.RPr) string) error {
		v, err := renderInherit(name, layers, func(r *dml.RPr) (string, bool, error) {
			if r == nil || pick(r) == "" {
				return "", false, nil
			}
			return pick(r), true, nil
		}, renderBuiltin(plain))
		if err == nil && v != plain {
			err = fmt.Errorf("%w: %s", render.ErrUnsupported, name)
		}
		return err
	}
	if err = word("underline", "none", func(r *dml.RPr) string { return r.U }); err != nil {
		return s, err
	}
	if err = word("strikethrough", "noStrike", func(r *dml.RPr) string { return r.Strike }); err != nil {
		return s, err
	}
	if err = word("capitalization", "none", func(r *dml.RPr) string { return r.Cap }); err != nil {
		return s, err
	}
	number := func(name string, pick func(*dml.RPr) (int32, bool)) (int32, error) {
		return renderInherit(name, layers, func(r *dml.RPr) (int32, bool, error) {
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
	if baseline != 0 || spacing != 0 {
		return s, fmt.Errorf("%w: baseline shift or character spacing", render.ErrUnsupported)
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
	if s.color, err = renderInherit("text color", layers, func(r *dml.RPr) (style.RGBA, bool, error) {
		switch {
		case r == nil:
			return style.RGBA{}, false, nil
		case r.NoFill != nil || r.GradFill != nil || r.BlipFill != nil || r.PattFill != nil || r.GrpFill != nil:
			return style.RGBA{}, false, fmt.Errorf("%w: text fill", render.ErrUnsupported)
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
	if s.highlight, err = renderInherit("highlight", layers, func(r *dml.RPr) (renderHighlight, bool, error) {
		if r == nil || r.Highlight == nil {
			return renderHighlight{}, false, nil
		}
		c, err := t.colors.color(renderChoiceColor(r.Highlight), nil)
		return renderHighlight{on: true, color: c}, err == nil, err
	}, renderBuiltin(renderHighlight{})); err != nil {
		return s, err
	}
	if s.font, err = renderInherit("font", layers, func(r *dml.RPr) (string, bool, error) {
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
