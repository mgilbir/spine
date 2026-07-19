package dml

import "math"

// Shadow represents an outer shadow effect for shapes.
type Shadow struct {
	Color    Color
	BlurRad  float64 // points
	Distance float64 // points
	Angle    float64 // degrees
}

// ApplyToSpPr sets the shadow on a SpPr's effect list.
func (s Shadow) ApplyToSpPr(spPr *SpPr) {
	outerShdw := &OuterShdw{}

	// BlurRad in EMUs
	if s.BlurRad > 0 {
		r := int64(math.Round(s.BlurRad * 12700))
		outerShdw.BlurRad = &r
	}

	// Distance in EMUs
	if s.Distance > 0 {
		d := int64(math.Round(s.Distance * 12700))
		outerShdw.Dist = &d
	}

	// Direction in 60000ths of a degree. dir is ST_PositiveFixedAngle
	// ([0, 21600000)), so the angle is normalized into [0, 360) first: a
	// negative Angle must not emit a schema-invalid negative dir.
	ang := math.Mod(s.Angle, 360)
	if ang < 0 {
		ang += 360
	}
	dir := int32(math.Round(ang*60000)) % 21600000
	outerShdw.Dir = &dir

	// Route the color to the correct color-choice element so theme colors
	// are not silently rendered as black.
	if s.Color.Type == ColorTypeTheme {
		outerShdw.SchemeClr = colorToSchemeClr(s.Color)
	} else {
		outerShdw.SrgbClr = colorToSrgbClr(s.Color)
	}

	if spPr.EffectLst == nil {
		spPr.EffectLst = &EffectLst{}
	}
	spPr.EffectLst.OuterShdw = outerShdw
}

// effectColorChoice routes a color to the scheme or srgb color-choice element
// on the given setters, so theme colors are not silently rendered as black
// (mirrors the routing Shadow.ApplyToSpPr uses).
func effectColorChoice(c Color, setScheme func(*SchemeClrTransform), setSrgb func(*SrgbClr)) {
	if c.Type == ColorTypeTheme {
		setScheme(colorToSchemeClr(c))
	} else {
		setSrgb(colorToSrgbClr(c))
	}
}

// ensureEffectLst returns spPr's effect list, allocating it if absent.
func ensureEffectLst(spPr *SpPr) *EffectLst {
	if spPr.EffectLst == nil {
		spPr.EffectLst = &EffectLst{}
	}
	return spPr.EffectLst
}

// Glow represents a glow effect (a:glow): a colored halo of the given radius
// around a shape.
type Glow struct {
	Color  Color
	Radius float64 // points
}

// ApplyToSpPr sets the glow on a SpPr's effect list, leaving any other effects
// (shadow, reflection, soft edge) already present untouched.
func (g Glow) ApplyToSpPr(spPr *SpPr) {
	glow := &GlowXML{}
	if g.Radius > 0 {
		glow.Rad = int64(math.Round(g.Radius * 12700))
	}
	effectColorChoice(g.Color,
		func(s *SchemeClrTransform) { glow.SchemeClr = s },
		func(s *SrgbClr) { glow.SrgbClr = s })
	ensureEffectLst(spPr).Glow = glow
}

// SoftEdge represents a soft-edge effect (a:softEdge): a feathered blur applied
// to a shape's edges over the given radius.
type SoftEdge struct {
	Radius float64 // points
}

// ApplyToSpPr sets the soft edge on a SpPr's effect list.
func (s SoftEdge) ApplyToSpPr(spPr *SpPr) {
	var rad int64
	if s.Radius > 0 {
		rad = int64(math.Round(s.Radius * 12700))
	}
	ensureEffectLst(spPr).SoftEdge = &SoftEdgeXML{Rad: rad}
}

// Reflection represents a reflection effect (a:reflection): a mirrored, fading
// copy of the shape below it. The zero value produces PowerPoint's default
// "tight reflection" geometry.
type Reflection struct {
	BlurRadius    float64 // points
	Distance      float64 // points
	Direction     float64 // degrees
	FadeDirection float64 // degrees, direction the reflection fades toward
	StartOpacity  float64 // 0..1, opacity where the reflection starts
	StartPosition float64 // 0..1, start of the alpha gradient
	EndOpacity    float64 // 0..1, opacity where the reflection ends
	EndPosition   float64 // 0..1, end of the alpha gradient
}

// ApplyToSpPr sets the reflection on a SpPr's effect list. Values left at zero
// are omitted so the reflection inherits PowerPoint's defaults for them.
func (r Reflection) ApplyToSpPr(spPr *SpPr) {
	rf := &ReflectionXML{}
	if r.BlurRadius > 0 {
		v := int64(math.Round(r.BlurRadius * 12700))
		rf.BlurRad = &v
	}
	if r.Distance > 0 {
		v := int64(math.Round(r.Distance * 12700))
		rf.Dist = &v
	}
	if r.Direction != 0 {
		v := positiveFixedAngle(r.Direction)
		rf.Dir = &v
	}
	if r.FadeDirection != 0 {
		v := positiveFixedAngle(r.FadeDirection)
		rf.FadeDir = &v
	}
	if r.StartOpacity != 0 {
		p := NewPercentage(clampPct(r.StartOpacity))
		rf.StA = &p
	}
	if r.StartPosition != 0 {
		p := NewPercentage(clampPct(r.StartPosition))
		rf.StPos = &p
	}
	if r.EndOpacity != 0 {
		p := NewPercentage(clampPct(r.EndOpacity))
		rf.EndA = &p
	}
	if r.EndPosition != 0 {
		p := NewPercentage(clampPct(r.EndPosition))
		rf.EndPos = &p
	}
	ensureEffectLst(spPr).Reflection = rf
}

// Bevel3D represents a basic 3D top bevel (a:sp3d/a:bevelT): a raised edge of
// the given preset shape and dimensions. It is named Bevel3D to avoid colliding
// with Bevel (CT_LineJoinBevel, the line-join element).
type Bevel3D struct {
	Preset string  // bevel preset (e.g. "circle", "relaxedInset", "coolSlant")
	Width  float64 // points
	Height float64 // points
}

// Common 3D bevel presets (a:bevelT/@prst values).
const (
	BevelCircle       = "circle"
	BevelRelaxedInset = "relaxedInset"
	BevelSlope        = "slope"
	BevelCross        = "cross"
	BevelAngle        = "angle"
	BevelSoftRound    = "softRound"
	BevelConvex       = "convex"
	BevelCoolSlant    = "coolSlant"
	BevelDivot        = "divot"
	BevelRiblet       = "riblet"
	BevelHardEdge     = "hardEdge"
	BevelArtDeco      = "artDeco"
)

// ApplyToSpPr sets the top bevel on a SpPr's shape 3D properties, leaving any
// other 3D properties already present untouched.
func (b Bevel3D) ApplyToSpPr(spPr *SpPr) {
	if spPr.Sp3d == nil {
		spPr.Sp3d = &Sp3d{}
	}
	bev := &Bevel3d{Prst: b.Preset}
	if b.Width > 0 {
		bev.W = int64(math.Round(b.Width * 12700))
	}
	if b.Height > 0 {
		bev.H = int64(math.Round(b.Height * 12700))
	}
	spPr.Sp3d.BevelT = bev
}

// positiveFixedAngle normalizes an angle in degrees into the ST_PositiveFixedAngle
// range ([0, 21600000) in 60000ths of a degree), so a negative input never emits
// a schema-invalid negative angle.
func positiveFixedAngle(deg float64) int32 {
	a := math.Mod(deg, 360)
	if a < 0 {
		a += 360
	}
	return int32(math.Round(a*60000)) % 21600000
}

// clampPct maps a 0..1 fraction to a 0..100000 ST_Percentage value.
func clampPct(f float64) int32 {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return int32(math.Round(f * 100000))
}
