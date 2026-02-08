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

	// Direction in 60000ths of a degree
	dir := int32(math.Round(s.Angle * 60000))
	outerShdw.Dir = &dir

	// Color
	outerShdw.SrgbClr = colorToSrgbClr(s.Color)

	if spPr.EffectLst == nil {
		spPr.EffectLst = &EffectLst{}
	}
	spPr.EffectLst.OuterShdw = outerShdw
}
