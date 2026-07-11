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
