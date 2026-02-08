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

func colorToSolidFill(c Color) *SolidFill {
	sf := &SolidFill{}
	switch c.Type {
	case ColorTypeRGB:
		srgb := &SrgbClr{Val: c.RGB.String()}
		if c.Alpha > 0 && c.Alpha < 100000 {
			srgb.Alpha = &ColorTransform{Val: Percentage(c.Alpha)}
		}
		sf.SrgbClr = srgb
	case ColorTypeTheme:
		scheme := &SchemeClrTransform{Val: c.Theme.String()}
		if c.Tint != 0 {
			tintVal := int32(math.Round(c.Tint * 100000))
			if c.Tint > 0 {
				scheme.Tint = append(scheme.Tint, &ColorTransform{Val: Percentage(tintVal)})
			} else {
				scheme.Shade = append(scheme.Shade, &ColorTransform{Val: Percentage(-tintVal)})
			}
		}
		sf.SchemeClr = scheme
	}
	return sf
}

func colorToSrgbClr(c Color) *SrgbClr {
	if c.Type != ColorTypeRGB {
		return &SrgbClr{Val: "000000"}
	}
	srgb := &SrgbClr{Val: c.RGB.String()}
	if c.Alpha > 0 && c.Alpha < 100000 {
		srgb.Alpha = &ColorTransform{Val: Percentage(c.Alpha)}
	}
	return srgb
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
		gs := &Gs{
			Pos:     int32(math.Round(stop.Position * 100000)),
			SrgbClr: colorToSrgbClr(stop.Color),
		}
		gf.GsLst.Gs = append(gf.GsLst.Gs, gs)
	}
	return gf
}

func (p *patternFillDef) toXML() *PattFill {
	return &PattFill{
		Prst: p.pattern,
		FgClr: &ColorChoice{
			SrgbClr: colorToSrgbClr(p.fg),
		},
		BgClr: &ColorChoice{
			SrgbClr: colorToSrgbClr(p.bg),
		},
	}
}
