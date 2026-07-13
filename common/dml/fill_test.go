package dml

import (
	"testing"
)

func TestNewNoFill(t *testing.T) {
	f := NewNoFill()
	if f.Type() != FillTypeNone {
		t.Errorf("Type() = %v, want FillTypeNone", f.Type())
	}

	spPr := &SpPr{}
	f.ApplyToSpPr(spPr)

	if spPr.NoFill == nil {
		t.Fatal("expected NoFill to be set")
	}
	if spPr.SolidFill != nil {
		t.Error("expected SolidFill to be nil")
	}
	if spPr.GradFill != nil {
		t.Error("expected GradFill to be nil")
	}
	if spPr.PattFill != nil {
		t.Error("expected PattFill to be nil")
	}
}

func TestNewSolidFill_RGB(t *testing.T) {
	c := NewRGB(68, 114, 196).ToColor()
	f := NewSolidFill(c)
	if f.Type() != FillTypeSolid {
		t.Errorf("Type() = %v, want FillTypeSolid", f.Type())
	}

	spPr := &SpPr{}
	f.ApplyToSpPr(spPr)

	if spPr.SolidFill == nil {
		t.Fatal("expected SolidFill to be set")
	}
	if spPr.NoFill != nil {
		t.Error("expected NoFill to be nil")
	}
	if spPr.SolidFill.SrgbClr == nil {
		t.Fatal("expected SrgbClr to be set")
	}
	if spPr.SolidFill.SrgbClr.Val != "4472C4" {
		t.Errorf("SrgbClr.Val = %s, want 4472C4", spPr.SolidFill.SrgbClr.Val)
	}
}

func TestNewSolidFill_Theme(t *testing.T) {
	c := ThemeColorAccent1.ToColor()
	f := NewSolidFill(c)

	spPr := &SpPr{}
	f.ApplyToSpPr(spPr)

	if spPr.SolidFill == nil {
		t.Fatal("expected SolidFill to be set")
	}
	if spPr.SolidFill.SchemeClr == nil {
		t.Fatal("expected SchemeClr to be set")
	}
	if spPr.SolidFill.SchemeClr.Val != "accent1" {
		t.Errorf("SchemeClr.Val = %s, want accent1", spPr.SolidFill.SchemeClr.Val)
	}
}

func TestNewSolidFill_ThemeWithTint(t *testing.T) {
	c := ThemeColorAccent1.ToColor().WithTint(0.5)
	f := NewSolidFill(c)

	spPr := &SpPr{}
	f.ApplyToSpPr(spPr)

	if spPr.SolidFill.SchemeClr == nil {
		t.Fatal("expected SchemeClr to be set")
	}
	if len(spPr.SolidFill.SchemeClr.Tint) != 1 {
		t.Fatalf("expected 1 tint transform, got %d", len(spPr.SolidFill.SchemeClr.Tint))
	}
	if spPr.SolidFill.SchemeClr.Tint[0].Val != 50000 {
		t.Errorf("Tint.Val = %d, want 50000", spPr.SolidFill.SchemeClr.Tint[0].Val)
	}
}

func TestNewSolidFill_ThemeWithShade(t *testing.T) {
	c := ThemeColorAccent1.ToColor().WithTint(-0.25)
	f := NewSolidFill(c)

	spPr := &SpPr{}
	f.ApplyToSpPr(spPr)

	if spPr.SolidFill.SchemeClr == nil {
		t.Fatal("expected SchemeClr to be set")
	}
	if len(spPr.SolidFill.SchemeClr.Shade) != 1 {
		t.Fatalf("expected 1 shade transform, got %d", len(spPr.SolidFill.SchemeClr.Shade))
	}
	if spPr.SolidFill.SchemeClr.Shade[0].Val != 25000 {
		t.Errorf("Shade.Val = %d, want 25000", spPr.SolidFill.SchemeClr.Shade[0].Val)
	}
}

func TestNewSolidFill_WithAlpha(t *testing.T) {
	c := NewRGB(255, 0, 0).ToColor().WithAlpha(50)
	f := NewSolidFill(c)

	spPr := &SpPr{}
	f.ApplyToSpPr(spPr)

	if spPr.SolidFill.SrgbClr == nil {
		t.Fatal("expected SrgbClr to be set")
	}
	if spPr.SolidFill.SrgbClr.Alpha == nil {
		t.Fatal("expected Alpha to be set")
	}
	if spPr.SolidFill.SrgbClr.Alpha.Val != 50000 {
		t.Errorf("Alpha.Val = %d, want 50000", spPr.SolidFill.SrgbClr.Alpha.Val)
	}
}

func TestNewGradientFill(t *testing.T) {
	f := NewGradientFill(90,
		GradientStop{Position: 0, Color: NewRGB(68, 114, 196).ToColor()},
		GradientStop{Position: 1, Color: NewRGB(255, 255, 255).ToColor()},
	)
	if f.Type() != FillTypeGradient {
		t.Errorf("Type() = %v, want FillTypeGradient", f.Type())
	}

	spPr := &SpPr{}
	f.ApplyToSpPr(spPr)

	if spPr.GradFill == nil {
		t.Fatal("expected GradFill to be set")
	}
	if spPr.SolidFill != nil {
		t.Error("expected SolidFill to be nil")
	}

	gf := spPr.GradFill
	if gf.Lin == nil {
		t.Fatal("expected Lin to be set")
	}
	// 90 degrees * 60000 = 5400000
	if gf.Lin.Ang != 5400000 {
		t.Errorf("Lin.Ang = %d, want 5400000", gf.Lin.Ang)
	}
	if gf.Lin.Scaled == nil || !*gf.Lin.Scaled {
		t.Error("expected Scaled to be true")
	}

	if gf.GsLst == nil {
		t.Fatal("expected GsLst to be set")
	}
	if len(gf.GsLst.Gs) != 2 {
		t.Fatalf("expected 2 gradient stops, got %d", len(gf.GsLst.Gs))
	}

	// First stop: position 0
	if gf.GsLst.Gs[0].Pos != 0 {
		t.Errorf("stop 0 Pos = %d, want 0", gf.GsLst.Gs[0].Pos)
	}
	if gf.GsLst.Gs[0].SrgbClr == nil || gf.GsLst.Gs[0].SrgbClr.Val != "4472C4" {
		t.Errorf("stop 0 color incorrect")
	}

	// Second stop: position 100000 (1.0 * 100000)
	if gf.GsLst.Gs[1].Pos != 100000 {
		t.Errorf("stop 1 Pos = %d, want 100000", gf.GsLst.Gs[1].Pos)
	}
	if gf.GsLst.Gs[1].SrgbClr == nil || gf.GsLst.Gs[1].SrgbClr.Val != "FFFFFF" {
		t.Errorf("stop 1 color incorrect")
	}
}

func TestNewPatternFill(t *testing.T) {
	f := NewPatternFill("pct50",
		NewRGB(0, 0, 0).ToColor(),
		NewRGB(255, 255, 255).ToColor(),
	)
	if f.Type() != FillTypePattern {
		t.Errorf("Type() = %v, want FillTypePattern", f.Type())
	}

	spPr := &SpPr{}
	f.ApplyToSpPr(spPr)

	if spPr.PattFill == nil {
		t.Fatal("expected PattFill to be set")
	}
	if spPr.PattFill.Prst != "pct50" {
		t.Errorf("Prst = %s, want pct50", spPr.PattFill.Prst)
	}
	if spPr.PattFill.FgClr == nil || spPr.PattFill.FgClr.SrgbClr == nil {
		t.Fatal("expected FgClr.SrgbClr to be set")
	}
	if spPr.PattFill.FgClr.SrgbClr.Val != "000000" {
		t.Errorf("FgClr = %s, want 000000", spPr.PattFill.FgClr.SrgbClr.Val)
	}
	if spPr.PattFill.BgClr == nil || spPr.PattFill.BgClr.SrgbClr == nil {
		t.Fatal("expected BgClr.SrgbClr to be set")
	}
	if spPr.PattFill.BgClr.SrgbClr.Val != "FFFFFF" {
		t.Errorf("BgClr = %s, want FFFFFF", spPr.PattFill.BgClr.SrgbClr.Val)
	}
}

func TestFill_ClearsExistingFill(t *testing.T) {
	spPr := &SpPr{
		SolidFill: &SolidFill{SrgbClr: &SrgbClr{Val: "FF0000"}},
		Ln:        &Ln{}, // should not be cleared
	}

	// Apply NoFill should clear SolidFill
	NewNoFill().ApplyToSpPr(spPr)

	if spPr.SolidFill != nil {
		t.Error("expected SolidFill to be cleared")
	}
	if spPr.NoFill == nil {
		t.Error("expected NoFill to be set")
	}
	if spPr.Ln == nil {
		t.Error("Ln should not be cleared by fill change")
	}
}
