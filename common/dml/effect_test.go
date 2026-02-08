package dml

import (
	"testing"
)

func TestShadow_ApplyToSpPr(t *testing.T) {
	s := Shadow{
		Color:    NewRGB(0, 0, 0).ToColor().WithAlpha(50),
		BlurRad:  4,
		Distance: 3,
		Angle:    315,
	}

	spPr := &SpPr{}
	s.ApplyToSpPr(spPr)

	if spPr.EffectLst == nil {
		t.Fatal("expected EffectLst to be set")
	}
	if spPr.EffectLst.OuterShdw == nil {
		t.Fatal("expected OuterShdw to be set")
	}

	os := spPr.EffectLst.OuterShdw

	// BlurRad: 4 points * 12700 = 50800
	if os.BlurRad == nil {
		t.Fatal("expected BlurRad to be set")
	}
	if *os.BlurRad != 50800 {
		t.Errorf("BlurRad = %d, want 50800", *os.BlurRad)
	}

	// Distance: 3 points * 12700 = 38100
	if os.Dist == nil {
		t.Fatal("expected Dist to be set")
	}
	if *os.Dist != 38100 {
		t.Errorf("Dist = %d, want 38100", *os.Dist)
	}

	// Direction: 315 degrees * 60000 = 18900000
	if os.Dir == nil {
		t.Fatal("expected Dir to be set")
	}
	if *os.Dir != 18900000 {
		t.Errorf("Dir = %d, want 18900000", *os.Dir)
	}

	// Color
	if os.SrgbClr == nil {
		t.Fatal("expected SrgbClr to be set")
	}
	if os.SrgbClr.Val != "000000" {
		t.Errorf("SrgbClr.Val = %s, want 000000", os.SrgbClr.Val)
	}
	if os.SrgbClr.Alpha == nil {
		t.Fatal("expected Alpha to be set")
	}
	if os.SrgbClr.Alpha.Val != 50000 {
		t.Errorf("Alpha.Val = %d, want 50000", os.SrgbClr.Alpha.Val)
	}
}

func TestShadow_ZeroBlurAndDistance(t *testing.T) {
	s := Shadow{
		Color: ColorBlack,
		Angle: 0,
	}

	spPr := &SpPr{}
	s.ApplyToSpPr(spPr)

	os := spPr.EffectLst.OuterShdw
	if os.BlurRad != nil {
		t.Error("expected BlurRad to be nil for zero value")
	}
	if os.Dist != nil {
		t.Error("expected Dist to be nil for zero value")
	}
	if os.Dir == nil || *os.Dir != 0 {
		t.Error("expected Dir to be 0")
	}
}

func TestShadow_PreservesExistingEffectLst(t *testing.T) {
	spPr := &SpPr{
		EffectLst: &EffectLst{},
	}

	s := Shadow{
		Color:    ColorBlack,
		BlurRad:  2,
		Distance: 1,
		Angle:    270,
	}
	s.ApplyToSpPr(spPr)

	if spPr.EffectLst.OuterShdw == nil {
		t.Fatal("expected OuterShdw to be set")
	}
}
