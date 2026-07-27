package dml

import "math"

// FillType identifies the kind of fill.
type FillType int

const (
	FillTypeNone FillType = iota
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

// NewNoFill creates a fill that removes any fill from the shape.
func NewNoFill() Fill {
	return Fill{typ: FillTypeNone}
}

// NewSolidFill creates a solid color fill.
func NewSolidFill(c Color) Fill {
	return Fill{
		typ:   FillTypeSolid,
		solid: &solidFillDef{color: c},
	}
}

// NewGradientFill creates a linear gradient fill.
// angleDeg is the angle in degrees (0 = left-to-right, 90 = top-to-bottom).
// Angles outside [0, 360) are normalized into that range.
func NewGradientFill(angleDeg float64, stops ...GradientStop) Fill {
	return Fill{
		typ:      FillTypeGradient,
		gradient: &gradientFillDef{angleDeg: angleDeg, stops: stops},
	}
}

// NewPatternFill creates a pattern fill.
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
// color is fully opaque or the opacity was never specified. A color built via
// WithAlpha carries an explicit set-flag, so WithAlpha(0) (fully transparent)
// emits <a:alpha val="0"/> instead of being confused with the unset zero
// value. The value is clamped to the valid 0..100000 range.
func colorAlpha(c Color) *ColorTransform {
	a := c.Alpha
	if !c.alphaSet && a <= 0 {
		// Zero without the set-flag is the unset zero value, not transparency.
		return nil
	}
	if a >= 100000 {
		return nil // fully opaque: no transform needed
	}
	if a < 0 {
		a = 0
	}
	return &ColorTransform{Val: NewPercentage(int32(a))}
}

// colorToSrgbClr builds an <a:srgbClr> from a color's RGB value, tint/shade
// and opacity. It is used for RGB colors and as a best-effort fallback for
// colors whose kind the Color model cannot otherwise represent as its own
// element.
//
// a:tint / a:shade are EG_ColorTransform members and apply to any color kind,
// so WithTint means the same thing on an RGB color as on a theme color; only
// emitting it under a:schemeClr made WithTint a silent no-op for the whole
// NewSolidFill/gradient-stop/line-color/shadow-color surface (C479).
func colorToSrgbClr(c Color) *SrgbClr {
	srgb := &SrgbClr{Val: c.RGB.String()}
	if tint, shade := colorTintShade(c); tint != nil {
		srgb.Tint = tint
	} else if shade != nil {
		srgb.Shade = shade
	}
	srgb.Alpha = colorAlpha(c)
	return srgb
}

// colorToSchemeClr builds an <a:schemeClr> from a theme color, carrying its
// tint/shade and any partial opacity.
func colorToSchemeClr(c Color) *SchemeClrTransform {
	scheme := &SchemeClrTransform{Val: c.Theme.String()}
	if tint, shade := colorTintShade(c); tint != nil {
		scheme.Tint = append(scheme.Tint, tint)
	} else if shade != nil {
		scheme.Shade = append(scheme.Shade, shade)
	}
	if a := colorAlpha(c); a != nil {
		scheme.Alpha = append(scheme.Alpha, a)
	}
	return scheme
}

// colorTintShade renders a color's tint as the transform it maps to: a
// positive tint is a:tint, a negative one a:shade with the magnitude. At most
// one of the two is non-nil; both are nil when no tint was set.
func colorTintShade(c Color) (tint, shade *ColorTransform) {
	if c.Tint == 0 {
		return nil, nil
	}
	val := int32(math.Round(c.Tint * 100000))
	if c.Tint > 0 {
		return &ColorTransform{Val: NewPercentage(val)}, nil
	}
	return nil, &ColorTransform{Val: NewPercentage(-val)}
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
	// ang is ST_PositiveFixedAngle ([0, 21600000)): normalize the angle into
	// [0, 360) first so a negative input cannot emit a schema-invalid
	// negative ang.
	ang := math.Mod(g.angleDeg, 360)
	if ang < 0 {
		ang += 360
	}
	scaled := true
	linAng := int32(math.Round(ang*60000)) % 21600000 // degrees to 60000ths
	gf := &GradFill{
		GsLst: &GsLst{},
		Lin: &Lin{
			Ang:    &linAng,
			Scaled: &scaled,
		},
	}
	for _, stop := range g.stops {
		gs := &Gs{Pos: NewPercentage(int32(math.Round(stop.Position * 100000)))}
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
