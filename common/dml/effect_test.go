package dml

import (
	"strings"
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
	if os.SrgbClr.Alpha.Val.Int32() != 50000 {
		t.Errorf("Alpha.Val = %d, want 50000", os.SrgbClr.Alpha.Val.Int32())
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

// C179 (the shadow leg of C40): a theme-colored shadow maps to a schemeClr
// with alpha carried, not a literal black srgbClr, and a negative angle is
// normalized into the valid ST_PositiveFixedAngle range.
func TestShadow_ThemeColorAndAngleNormalization(t *testing.T) {
	s := Shadow{
		Color:    ThemeColorAccent1.ToColor().WithAlpha(50),
		BlurRad:  4,
		Distance: 3,
		Angle:    -45, // must normalize to 315 degrees
	}

	spPr := &SpPr{}
	s.ApplyToSpPr(spPr)
	os := spPr.EffectLst.OuterShdw

	if os.SrgbClr != nil {
		t.Errorf("theme shadow wrongly rendered as srgb: %+v", os.SrgbClr)
	}
	if os.SchemeClr == nil {
		t.Fatalf("theme shadow color lost: SchemeClr is nil")
	}
	if os.SchemeClr.Val != "accent1" {
		t.Errorf("SchemeClr.Val = %q, want accent1", os.SchemeClr.Val)
	}
	if len(os.SchemeClr.Alpha) != 1 || os.SchemeClr.Alpha[0].Val.Int32() != 50000 {
		t.Errorf("alpha not carried on schemeClr: %+v", os.SchemeClr.Alpha)
	}

	// -45 degrees -> 315 degrees -> 18900000 sixtieths
	if os.Dir == nil || *os.Dir != 18900000 {
		t.Errorf("Dir = %v, want 18900000 (negative angle normalized)", os.Dir)
	}

	// Through the production Builder the shadow must carry the scheme color.
	out := buildFragment(t, "effectLst", spPr.EffectLst)
	if !strings.Contains(out, `<a:schemeClr val="accent1">`) {
		t.Errorf("Builder output missing schemeClr: %s", out)
	}
	if strings.Contains(out, `srgbClr val="000000"`) {
		t.Errorf("Builder output degraded theme color to black: %s", out)
	}
}

// C179: angle normalization keeps dir inside [0, 21600000).
func TestShadow_AngleNormalization(t *testing.T) {
	cases := []struct {
		angle float64
		want  int32
	}{
		{0, 0},
		{360, 0},
		{-45, 18900000},
		{-360, 0},
		{405, 2700000},
		{315, 18900000},
	}
	for _, c := range cases {
		spPr := &SpPr{}
		Shadow{Color: ColorBlack, Angle: c.angle}.ApplyToSpPr(spPr)
		got := *spPr.EffectLst.OuterShdw.Dir
		if got != c.want {
			t.Errorf("Angle %v: Dir = %d, want %d", c.angle, got, c.want)
		}
		if got < 0 || got >= 21600000 {
			t.Errorf("Angle %v: Dir %d outside ST_PositiveFixedAngle range", c.angle, got)
		}
	}
}
