package dml

import "math"

// FillType identifies the kind of fill.
type FillType int

const (
	FillTypeNone     FillType = iota
	FillTypeSolid
	FillTypeGradient
	FillTypePattern
)

// Fill represents a fill definition for shapes.
type Fill struct {
	typ      FillType
	solid    *solidFillDef
	gradient *gradientFillDef
	pattern  *patternFillDef
}

type solidFillDef struct {
	color Color
}

type gradientFillDef struct {
	angleDeg float64
	stops    []GradientStop
}

type patternFillDef struct {
	pattern string
	fg, bg  Color
}

// GradientStop defines a color stop in a gradient fill.
type GradientStop struct {
	Position float64 // 0.0 to 1.0
	Color    Color
}

// NoFill creates a fill that removes any fill from the shape.
func NewNoFill() Fill {
	return Fill{typ: FillTypeNone}
}

// SolidFill creates a solid color fill.
func NewSolidFill(c Color) Fill {
	return Fill{
		typ:   FillTypeSolid,
		solid: &solidFillDef{color: c},
	}
}

// GradientFill creates a linear gradient fill.
// angleDeg is the angle in degrees (0 = left-to-right, 90 = top-to-bottom).
func NewGradientFill(angleDeg float64, stops ...GradientStop) Fill {
	return Fill{
		typ:      FillTypeGradient,
		gradient: &gradientFillDef{angleDeg: angleDeg, stops: stops},
	}
}

// PatternFill creates a pattern fill.
func NewPatternFill(pattern string, fg, bg Color) Fill {
	return Fill{
		typ:     FillTypePattern,
		pattern: &patternFillDef{pattern: pattern, fg: fg, bg: bg},
	}
}

// Type returns the fill type.
func (f Fill) Type() FillType { return f.typ }

// ApplyToSpPr sets the fill on a SpPr, clearing any existing fill.
func (f Fill) ApplyToSpPr(spPr *SpPr) {
	// Clear all fills
	spPr.NoFill = nil
	spPr.SolidFill = nil
	spPr.GradFill = nil
	spPr.PattFill = nil
	spPr.BlipFill = nil
	spPr.GrpFill = nil

	switch f.typ {
	case FillTypeNone:
		spPr.NoFill = &NoFillXML{}
	case FillTypeSolid:
		spPr.SolidFill = colorToSolidFill(f.solid.color)
	case FillTypeGradient:
		spPr.GradFill = f.gradient.toXML()
	case FillTypePattern:
		spPr.PattFill = f.pattern.toXML()
	}
}

// colorAlpha returns an alpha transform for partial opacity, or nil when the
// color is fully opaque or unset (Alpha 0 is treated as "not specified", the
// zero value). The value is clamped to the valid 0..100000 range.
func colorAlpha(c Color) *ColorTransform {
	if c.Alpha <= 0 || c.Alpha >= 100000 {
		return nil
	}
	return &ColorTransform{Val: Percentage(c.Alpha)}
}

// colorToSrgbClr builds an <a:srgbClr> from a color's RGB value and opacity.
// It is used for RGB colors and as a best-effort fallback for colors whose
// kind the Color model cannot otherwise represent as its own element.
func colorToSrgbClr(c Color) *SrgbClr {
	srgb := &SrgbClr{Val: c.RGB.String()}
	srgb.Alpha = colorAlpha(c)
	return srgb
}

// colorToSchemeClr builds an <a:schemeClr> from a theme color, carrying its
// tint/shade and any partial opacity.
func colorToSchemeClr(c Color) *SchemeClrTransform {
	scheme := &SchemeClrTransform{Val: c.Theme.String()}
	if c.Tint != 0 {
		tintVal := int32(math.Round(c.Tint * 100000))
		if c.Tint > 0 {
			scheme.Tint = append(scheme.Tint, &ColorTransform{Val: Percentage(tintVal)})
		} else {
			scheme.Shade = append(scheme.Shade, &ColorTransform{Val: Percentage(-tintVal)})
		}
	}
	if a := colorAlpha(c); a != nil {
		scheme.Alpha = append(scheme.Alpha, a)
	}
	return scheme
}

func colorToSolidFill(c Color) *SolidFill {
	sf := &SolidFill{}
	if c.Type == ColorTypeTheme {
		sf.SchemeClr = colorToSchemeClr(c)
	} else {
		// RGB, and system colors (which the model cannot name) degrade to
		// their concrete RGB value — never an empty, schema-invalid solidFill.
		sf.SrgbClr = colorToSrgbClr(c)
	}
	return sf
}

// colorToColorChoice maps a color to a single EG_ColorChoice element, used
// where a fill accepts any color kind (pattern fg/bg).
func colorToColorChoice(c Color) *ColorChoice {
	cc := &ColorChoice{}
	if c.Type == ColorTypeTheme {
		cc.SchemeClr = colorToSchemeClr(c)
	} else {
		cc.SrgbClr = colorToSrgbClr(c)
	}
	return cc
}

func (g *gradientFillDef) toXML() *GradFill {
	gf := &GradFill{
		GsLst: &GsLst{},
		Lin: &Lin{
			Ang:    int32(math.Round(g.angleDeg * 60000)), // degrees to 60000ths
			Scaled: true,
		},
	}
	for _, stop := range g.stops {
		gs := &Gs{Pos: int32(math.Round(stop.Position * 100000))}
		// Route the stop color to the correct color-choice element so theme
		// colors are not silently rendered as black.
		if stop.Color.Type == ColorTypeTheme {
			gs.SchemeClr = colorToSchemeClr(stop.Color)
		} else {
			gs.SrgbClr = colorToSrgbClr(stop.Color)
		}
		gf.GsLst.Gs = append(gf.GsLst.Gs, gs)
	}
	return gf
}

func (p *patternFillDef) toXML() *PattFill {
	return &PattFill{
		Prst:  p.pattern,
		FgClr: colorToColorChoice(p.fg),
		BgClr: colorToColorChoice(p.bg),
	}
}
