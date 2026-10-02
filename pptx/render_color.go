package pptx

import (
	"context"
	"fmt"
	"math"

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
}

var renderWhite = style.RGBA{R: 255, G: 255, B: 255, A: 1}

// renderColor is one color element. Exactly one choice must be present.
type renderColor struct {
	srgb   *dml.SrgbClr
	scheme *dml.SchemeClrTransform
	sys    *dml.SystemClr
	others int
}

func renderSolidColor(f *dml.SolidFill) renderColor {
	return renderColorOf(f.SrgbClr, f.SchemeClr, f.SysClr, f.ScRgbClr != nil, f.HslClr != nil, f.PrstClr != nil)
}

func renderChoiceColor(c *dml.ColorChoice) renderColor {
	return renderColorOf(c.SrgbClr, c.SchemeClr, c.SysClr, c.ScrgbClr != nil, c.HslClr != nil, c.PrstClr != nil)
}

func renderColorOf(srgb *dml.SrgbClr, scheme *dml.SchemeClrTransform, sys *dml.SystemClr, others ...bool) renderColor {
	c := renderColor{srgb: srgb, scheme: scheme, sys: sys}
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
	for _, present := range []bool{v.srgb != nil, v.scheme != nil, v.sys != nil} {
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

func renderRGB(val string) (style.RGBA, error) {
	rgb, err := dml.ParseRGB(val)
	if err != nil {
		return style.RGBA{}, fmt.Errorf("%w: RGB color", render.ErrInvalid)
	}
	return style.RGBA{R: float64(rgb.R), G: float64(rgb.G), B: float64(rgb.B), A: 1}, nil
}

// renderTransform applies luminance modulation and offset in HSL space,
// clamping after each step. Every other transform fails explicitly.
func renderTransform(c style.RGBA, steps []dml.ColorTransformStep) (style.RGBA, error) {
	if len(steps) == 0 {
		return c, nil
	}
	h, s, l := renderHSL(c)
	for _, step := range steps {
		v := float64(step.Val.Int32()) / 100000
		switch step.Name {
		case "lumMod":
			l *= v
		case "lumOff":
			l += v
		default:
			return style.RGBA{}, fmt.Errorf("%w: %s color transform", render.ErrUnsupported, step.Name)
		}
		l = math.Min(1, math.Max(0, l))
	}
	return renderFromHSL(h, s, l), nil
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

func renderFromHSL(h, s, l float64) style.RGBA {
	channel := func(v float64) float64 { return math.Round(math.Min(255, math.Max(0, v*255))) }
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
func (c *renderColors) background(bg *oxml.Background) (style.RGBA, error) {
	if bg.BwMode != "" || (bg.BgPr == nil) == (bg.BgRef == nil) {
		return style.RGBA{}, fmt.Errorf("%w: slide background", render.ErrUnsupported)
	}
	if v := bg.BgPr; v != nil {
		if v.GradFill != nil || v.BlipFill != nil || v.PattFill != nil || v.EffectLst != nil || v.ExtLst != nil {
			return style.RGBA{}, fmt.Errorf("%w: background fill/effect", render.ErrUnsupported)
		}
		if v.NoFill != nil && v.SolidFill == nil {
			return renderWhite, nil
		}
		return c.solid(v.SolidFill, nil)
	}
	ref := bg.BgRef
	if ref.Idx == 0 {
		return renderWhite, nil
	}
	color, err := c.color(renderColorOf(ref.SrgbClr, ref.SchemeClr, ref.SysClr, ref.ScrgbClr != nil, ref.HslClr != nil, ref.PrstClr != nil), nil)
	if err != nil {
		return style.RGBA{}, err
	}
	theme, err := c.loadTheme()
	if err != nil {
		return style.RGBA{}, err
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
		return style.RGBA{}, fmt.Errorf("%w: background style %d", render.ErrInvalid, ref.Idx)
	}
	if entry.NoFill != nil {
		return renderWhite, nil
	}
	if entry.SolidFill == nil {
		return style.RGBA{}, fmt.Errorf("%w: theme background fill", render.ErrUnsupported)
	}
	return c.solid(entry.SolidFill, &color)
}
