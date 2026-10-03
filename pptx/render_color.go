package pptx

import (
	"context"
	"fmt"
	"math"
	"strings"

	csstok "github.com/mgilbir/forme/css"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// renderColors resolves DrawingML colors for one slide preparation. Every
// scheme color, including one inherited from the layout or master, resolves
// through the slide's effective color map, as PowerPoint shows the slide. The
// master's theme is parsed at most once, under the preparation's source budget.
type renderColors struct {
	ctx         context.Context
	slide       *Slide
	budget      *core.SourceBudget
	theme       *dml.Theme
	themeErr    error
	themeLoaded bool
	// approx, in best-effort mode, reports a detail of the shape being drawn
	// that is drawn approximately or left out; nil in strict mode.
	approx func(error)
}

// approximate reports err and returns nil in best-effort mode, and returns err
// in strict mode.
func (c *renderColors) approximate(err error) error {
	if err == nil || c.approx == nil {
		return err
	}
	c.approx(err)
	return nil
}

var renderWhite = style.RGBA{R: 255, G: 255, B: 255, A: 1}

// renderColor is one color element. Exactly one choice must be present.
type renderColor struct {
	srgb   *dml.SrgbClr
	scheme *dml.SchemeClrTransform
	sys    *dml.SystemClr
	prst   *dml.PrstClr
	others int
}

func renderSolidColor(f *dml.SolidFill) renderColor {
	return renderColorOf(f.SrgbClr, f.SchemeClr, f.SysClr, f.PrstClr, f.ScRgbClr != nil, f.HslClr != nil)
}

func renderChoiceColor(c *dml.ColorChoice) renderColor {
	return renderColorOf(c.SrgbClr, c.SchemeClr, c.SysClr, c.PrstClr, c.ScrgbClr != nil, c.HslClr != nil)
}

func renderColorOf(srgb *dml.SrgbClr, scheme *dml.SchemeClrTransform, sys *dml.SystemClr, prst *dml.PrstClr, others ...bool) renderColor {
	c := renderColor{srgb: srgb, scheme: scheme, sys: sys, prst: prst}
	for _, set := range others {
		if set {
			c.others++
		}
	}
	return c
}

// solid resolves a solid fill. placeholder is the style reference color that a
// theme style's phClr names; it is nil outside theme styles.
func (c *renderColors) solid(f *dml.SolidFill, placeholder *style.RGBA) (style.RGBA, error) {
	if f == nil {
		return style.RGBA{}, fmt.Errorf("%w: solid fill color", render.ErrUnsupported)
	}
	return c.color(renderSolidColor(f), placeholder)
}

func (c *renderColors) color(v renderColor, placeholder *style.RGBA) (style.RGBA, error) {
	set := v.others
	for _, present := range []bool{v.srgb != nil, v.scheme != nil, v.sys != nil, v.prst != nil} {
		if present {
			set++
		}
	}
	if set != 1 {
		return style.RGBA{}, fmt.Errorf("%w: one color choice required", render.ErrInvalid)
	}
	var (
		base  style.RGBA
		steps []dml.ColorTransformStep
		ok    bool
		err   error
	)
	switch {
	case v.srgb != nil:
		base, err = renderRGB(v.srgb.Val)
		steps, ok = v.srgb.Transforms()
	case v.sys != nil:
		// lastClr is the system color PowerPoint recorded when saving.
		if v.sys.LastClr == "" {
			return style.RGBA{}, fmt.Errorf("%w: system color without a recorded value", render.ErrUnsupported)
		}
		base, err = renderRGB(v.sys.LastClr)
		steps, ok = v.sys.Transforms()
	case v.scheme != nil:
		base, err = c.scheme(v.scheme.Val, placeholder)
		steps, ok = v.scheme.Transforms()
	case v.prst != nil:
		base, err = renderPresetColor(v.prst.Val)
		steps, ok = v.prst.Transforms()
	default:
		return style.RGBA{}, fmt.Errorf("%w: color kind", render.ErrUnsupported)
	}
	if err != nil {
		return style.RGBA{}, err
	}
	if !ok {
		return style.RGBA{}, fmt.Errorf("%w: unrecognized color transform", render.ErrUnsupported)
	}
	return renderTransform(base, steps)
}

// renderPresetColor resolves an ST_PresetColorVal name, which is a CSS color
// name in camel case, its "dk", "lt" and "med" prefixes abbreviating "dark",
// "light" and "medium".
func renderPresetColor(name string) (style.RGBA, error) {
	css := name
	for _, p := range [...]struct{ short, long string }{{"dk", "dark"}, {"lt", "light"}, {"med", "medium"}} {
		if rest, ok := strings.CutPrefix(name, p.short); ok && rest != "" && rest[0] >= 'A' && rest[0] <= 'Z' {
			css = p.long + rest
			break
		}
	}
	css = strings.ToLower(css)
	for _, r := range css {
		if r < 'a' || r > 'z' {
			return style.RGBA{}, fmt.Errorf("%w: preset color %q", render.ErrInvalid, name)
		}
	}
	vals, errs := csstok.ParseComponentValues(css)
	c, ok := style.ParseColor(vals)
	if len(errs) > 0 || !ok || css == "transparent" || css == "currentcolor" {
		return style.RGBA{}, fmt.Errorf("%w: preset color %q", render.ErrInvalid, name)
	}
	return c, nil
}

func renderRGB(val string) (style.RGBA, error) {
	rgb, err := dml.ParseRGB(val)
	if err != nil {
		return style.RGBA{}, fmt.Errorf("%w: RGB color", render.ErrInvalid)
	}
	return style.RGBA{R: float64(rgb.R), G: float64(rgb.G), B: float64(rgb.B), A: 1}, nil
}

// renderTransform applies a color's transforms in order, clamping after each.
// HSL transforms work on hue, saturation and luminance; tint, shade, the
// channel transforms and gray work on linear RGB, as LibreOffice's import does;
// alpha transforms on opacity. Channels round once, at the end.
func renderTransform(c style.RGBA, steps []dml.ColorTransformStep) (style.RGBA, error) {
	if len(steps) == 0 {
		return c, nil
	}
	clamp := func(v float64) float64 { return math.Min(1, math.Max(0, v)) }
	// The color is held in one space at a time: sRGB channels in 0-1, or HSL.
	r, g, b, a := c.R/255, c.G/255, c.B/255, c.A
	var h, sat, l float64
	inHSL := false
	toHSL := func() {
		if !inHSL {
			h, sat, l = renderHSL(style.RGBA{R: r * 255, G: g * 255, B: b * 255})
			inHSL = true
		}
	}
	toRGB := func() {
		if inHSL {
			v := renderFromHSLExact(h, sat, l)
			r, g, b, inHSL = v[0], v[1], v[2], false
		}
	}
	linear := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	encode := func(v float64) float64 {
		if v <= 0.0031308 {
			return v * 12.92
		}
		return 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	// inLinear applies f to each channel in linear RGB.
	inLinear := func(f func(float64) float64) {
		toRGB()
		r, g, b = clamp(encode(clamp(f(linear(r))))), clamp(encode(clamp(f(linear(g))))), clamp(encode(clamp(f(linear(b)))))
	}
	channel := func(ch *float64, f func(float64) float64) {
		toRGB()
		*ch = clamp(encode(clamp(f(linear(*ch)))))
	}
	for _, step := range steps {
		v := float64(step.Val.Int32()) / 100000
		// Hue values are angles in 60000ths of a degree.
		turn := float64(step.Val.Int32()) / 21600000
		switch step.Name {
		case "lumMod":
			toHSL()
			l = clamp(l * v)
		case "lumOff":
			toHSL()
			l = clamp(l + v)
		case "lum":
			toHSL()
			l = clamp(v)
		case "satMod":
			toHSL()
			sat = clamp(sat * v)
		case "satOff":
			toHSL()
			sat = clamp(sat + v)
		case "sat":
			toHSL()
			sat = clamp(v)
		case "hue":
			toHSL()
			h = math.Mod(math.Max(0, turn), 1)
		case "hueOff":
			toHSL()
			h = math.Mod(h+turn+1, 1)
		case "hueMod":
			toHSL()
			h = math.Mod(h*v, 1)
		case "comp":
			toHSL()
			h = math.Mod(h+0.5, 1)
		case "inv":
			toRGB()
			r, g, b = 1-r, 1-g, 1-b
		case "gray":
			toRGB()
			y := 0.299*r + 0.587*g + 0.114*b
			r, g, b = y, y, y
		case "tint":
			// 0% is white, 100% the color.
			inLinear(func(x float64) float64 { return 1 - (1-x)*v })
		case "shade":
			// 0% is black, 100% the color.
			inLinear(func(x float64) float64 { return x * v })
		case "red":
			channel(&r, func(float64) float64 { return v })
		case "redMod":
			channel(&r, func(x float64) float64 { return x * v })
		case "redOff":
			channel(&r, func(x float64) float64 { return x + v })
		case "green":
			channel(&g, func(float64) float64 { return v })
		case "greenMod":
			channel(&g, func(x float64) float64 { return x * v })
		case "greenOff":
			channel(&g, func(x float64) float64 { return x + v })
		case "blue":
			channel(&b, func(float64) float64 { return v })
		case "blueMod":
			channel(&b, func(x float64) float64 { return x * v })
		case "blueOff":
			channel(&b, func(x float64) float64 { return x + v })
		case "gamma":
			toRGB()
			r, g, b = encode(r), encode(g), encode(b)
		case "invGamma":
			toRGB()
			r, g, b = linear(r), linear(g), linear(b)
		case "alpha":
			a = clamp(v)
		case "alphaMod":
			a = clamp(a * v)
		case "alphaOff":
			a = clamp(a + v)
		default:
			return style.RGBA{}, fmt.Errorf("%w: %s color transform", render.ErrUnsupported, step.Name)
		}
	}
	toRGB()
	round := func(v float64) float64 { return math.Round(clamp(v) * 255) }
	return style.RGBA{R: round(r), G: round(g), B: round(b), A: a}, nil
}

func renderHSL(c style.RGBA) (h, s, l float64) {
	r, g, b := c.R/255, c.G/255, c.B/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (hi + lo) / 2
	d := hi - lo
	if d == 0 {
		return 0, 0, l
	}
	if l > 0.5 {
		s = d / (2 - hi - lo)
	} else {
		s = d / (hi + lo)
	}
	switch hi {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

// renderFromHSLExact converts HSL to sRGB channels in 0-1, unrounded.
func renderFromHSLExact(h, s, l float64) [3]float64 {
	c := renderFromHSLWith(h, s, l, func(v float64) float64 { return math.Min(1, math.Max(0, v)) })
	return [3]float64{c.R, c.G, c.B}
}

func renderFromHSLWith(h, s, l float64, channel func(float64) float64) style.RGBA {
	if s == 0 {
		return style.RGBA{R: channel(l), G: channel(l), B: channel(l), A: 1}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		switch {
		case t < 0:
			t++
		case t > 1:
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return style.RGBA{R: channel(hue(h + 1.0/3)), G: channel(hue(h)), B: channel(hue(h - 1.0/3)), A: 1}
}

// scheme resolves a scheme color name through the slide's color map and the
// theme's color scheme.
func (c *renderColors) scheme(name string, placeholder *style.RGBA) (style.RGBA, error) {
	if name == "phClr" {
		if placeholder == nil {
			return style.RGBA{}, fmt.Errorf("%w: placeholder color outside a theme style", render.ErrInvalid)
		}
		return *placeholder, nil
	}
	slot := name
	if renderMappedScheme(name) {
		m, err := c.colorMap()
		if err != nil {
			return style.RGBA{}, err
		}
		slot = renderMapLookup(m, name)
	}
	theme, err := c.loadTheme()
	if err != nil {
		return style.RGBA{}, err
	}
	var scheme *dml.ClrScheme
	if theme.ThemeElements != nil {
		scheme = theme.ThemeElements.ClrScheme
	}
	choice := renderSchemeSlot(scheme, slot)
	if choice == nil {
		return style.RGBA{}, fmt.Errorf("%w: theme color %s", render.ErrUnsupported, name)
	}
	v := renderChoiceColor(choice)
	if v.scheme != nil {
		return style.RGBA{}, fmt.Errorf("%w: theme color %s refers to the scheme", render.ErrInvalid, name)
	}
	return c.color(v, nil)
}

func renderMappedScheme(name string) bool {
	switch name {
	case "bg1", "tx1", "bg2", "tx2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hlink", "folHlink":
		return true
	}
	return false
}

func renderMapLookup(m *oxml.ColorMap, name string) string {
	switch name {
	case "bg1":
		return m.Bg1
	case "tx1":
		return m.Tx1
	case "bg2":
		return m.Bg2
	case "tx2":
		return m.Tx2
	case "accent1":
		return m.Accent1
	case "accent2":
		return m.Accent2
	case "accent3":
		return m.Accent3
	case "accent4":
		return m.Accent4
	case "accent5":
		return m.Accent5
	case "accent6":
		return m.Accent6
	case "hlink":
		return m.Hlink
	case "folHlink":
		return m.FolHlink
	}
	return ""
}

func renderSchemeSlot(s *dml.ClrScheme, slot string) *dml.ColorChoice {
	if s == nil {
		return nil
	}
	switch slot {
	case "dk1":
		return s.Dk1
	case "lt1":
		return s.Lt1
	case "dk2":
		return s.Dk2
	case "lt2":
		return s.Lt2
	case "accent1":
		return s.Accent1
	case "accent2":
		return s.Accent2
	case "accent3":
		return s.Accent3
	case "accent4":
		return s.Accent4
	case "accent5":
		return s.Accent5
	case "accent6":
		return s.Accent6
	case "hlink":
		return s.Hlink
	case "folHlink":
		return s.FolHlink
	}
	return nil
}

// colorMap returns the slide's effective color map: the master's map, replaced
// in turn by any layout and slide override.
func (c *renderColors) colorMap() (*oxml.ColorMap, error) {
	l := c.slide.layout
	if l == nil || l.master == nil || l.master.masterXML == nil || l.master.masterXML.ClrMap == nil {
		return nil, fmt.Errorf("%w: missing master color map", render.ErrUnsupported)
	}
	m := l.master.masterXML.ClrMap
	var overrides []*oxml.ColorMapOverride
	if l.layoutXML != nil {
		overrides = append(overrides, l.layoutXML.ClrMapOvr)
	}
	if model := c.slide.sx(); model != nil {
		overrides = append(overrides, model.ClrMapOvr)
	}
	for _, o := range overrides {
		if o == nil {
			continue
		}
		if o.MasterClrMapping != nil && o.OverrideClrMapping != nil {
			return nil, fmt.Errorf("%w: ambiguous color map override", render.ErrInvalid)
		}
		if o.OverrideClrMapping != nil {
			m = o.OverrideClrMapping
		}
	}
	return m, nil
}

// loadTheme parses the master's theme, including unsaved theme edits.
func (c *renderColors) loadTheme() (*dml.Theme, error) {
	if c.themeLoaded {
		return c.theme, c.themeErr
	}
	c.themeLoaded = true
	c.theme, c.themeErr = c.parseTheme()
	return c.theme, c.themeErr
}

func (c *renderColors) parseTheme() (*dml.Theme, error) {
	l := c.slide.layout
	if l == nil || l.master == nil || l.master.resolvedThemePart == "" {
		return nil, fmt.Errorf("%w: missing theme", render.ErrUnsupported)
	}
	p, part := c.slide.presentation, l.master.resolvedThemePart
	data := p.themeData[part]
	// Read the editor cache without filling it: preparation does not mutate
	// presentation state.
	if ed := p.themeEditors[part]; ed != nil && ed.Modified() {
		var err error
		if data, err = ed.Marshal(); err != nil {
			return nil, fmt.Errorf("%w: theme: %w", render.ErrInvalid, err)
		}
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: missing theme", render.ErrUnsupported)
	}
	if err := c.budget.CheckXML(c.ctx, data, renderThemeRoot); err != nil {
		return nil, fmt.Errorf("pptx: theme: %w", err)
	}
	var theme dml.Theme
	if err := xmlb.UnmarshalWithSource(data, &theme); err != nil {
		return nil, fmt.Errorf("%w: theme: %w", render.ErrInvalid, err)
	}
	return &theme, nil
}

// renderThemeRoot bounds theme nodes and depth before parsing. Theme content is
// validated where it is used.
func renderThemeRoot(node core.XMLNode) error {
	if len(node.Path) == 1 && !node.Text && (node.Name.Space != nsA || node.Name.Local != "theme") {
		return fmt.Errorf("%w: theme XML root", render.ErrInvalid)
	}
	return nil
}

// background resolves an explicit or theme-referenced slide background.
func (c *renderColors) background(bg *oxml.Background, w, h float64) (renderPaint, error) {
	white := renderPaint{color: renderWhite}
	if (bg.BgPr == nil) == (bg.BgRef == nil) {
		return white, fmt.Errorf("%w: slide background", render.ErrUnsupported)
	}
	if v := bg.BgPr; v != nil {
		if v.BlipFill != nil || v.PattFill != nil || renderEffects(v.EffectLst) || v.ExtLst != nil {
			return white, fmt.Errorf("%w: background picture, pattern or effect", render.ErrUnsupported)
		}
		if v.NoFill != nil && v.SolidFill == nil && v.GradFill == nil {
			return white, nil
		}
		return c.fillPaint(v.SolidFill, v.GradFill, nil, w, h)
	}
	ref := bg.BgRef
	if ref.Idx == 0 {
		return white, nil
	}
	color, err := c.color(renderColorOf(ref.SrgbClr, ref.SchemeClr, ref.SysClr, ref.PrstClr, ref.ScrgbClr != nil, ref.HslClr != nil), nil)
	if err != nil {
		return white, err
	}
	theme, err := c.loadTheme()
	if err != nil {
		return white, err
	}
	var format *dml.FmtScheme
	if theme.ThemeElements != nil {
		format = theme.ThemeElements.FmtScheme
	}
	var (
		entry dml.StyleFill
		found bool
	)
	switch {
	case format == nil:
	case ref.Idx >= 1 && ref.Idx <= 999:
		entry, found = format.FillStyleLst.Entry(int(ref.Idx) - 1)
	case ref.Idx >= 1001:
		entry, found = format.BgFillStyleLst.Entry(int(ref.Idx - 1001))
	}
	if !found {
		return white, fmt.Errorf("%w: background style %d", render.ErrInvalid, ref.Idx)
	}
	if entry.NoFill != nil {
		return white, nil
	}
	if entry.SolidFill == nil && entry.GradFill == nil {
		return white, fmt.Errorf("%w: theme background fill", render.ErrUnsupported)
	}
	return c.fillPaint(entry.SolidFill, entry.GradFill, &color, w, h)
}
