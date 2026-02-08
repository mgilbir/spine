package dml

import (
	"testing"
)

func TestLine_ApplyToSpPr(t *testing.T) {
	l := Line{
		Width: 2.0,
		Color: NewRGB(0, 0, 0).ToColor(),
	}

	spPr := &SpPr{}
	l.ApplyToSpPr(spPr)

	if spPr.Ln == nil {
		t.Fatal("expected Ln to be set")
	}
	if spPr.Ln.W == nil {
		t.Fatal("expected W to be set")
	}
	// 2.0 points * 12700 = 25400 EMUs
	if *spPr.Ln.W != 25400 {
		t.Errorf("W = %d, want 25400", *spPr.Ln.W)
	}
	if spPr.Ln.SolidFill == nil {
		t.Fatal("expected SolidFill to be set on line")
	}
	if spPr.Ln.SolidFill.SrgbClr == nil || spPr.Ln.SolidFill.SrgbClr.Val != "000000" {
		t.Error("expected line color to be 000000")
	}
	if spPr.Ln.PrstDash != nil {
		t.Error("expected no dash for default solid line")
	}
}

func TestLine_WithDash(t *testing.T) {
	l := Line{
		Width: 1.0,
		Color: NewRGB(255, 0, 0).ToColor(),
		Dash:  DashDot,
	}

	spPr := &SpPr{}
	l.ApplyToSpPr(spPr)

	if spPr.Ln.PrstDash == nil {
		t.Fatal("expected PrstDash to be set")
	}
	if spPr.Ln.PrstDash.Val != "dot" {
		t.Errorf("PrstDash.Val = %s, want dot", spPr.Ln.PrstDash.Val)
	}
}

func TestLine_SolidDashNotSet(t *testing.T) {
	l := Line{
		Width: 1.0,
		Color: ColorBlack,
		Dash:  DashSolid,
	}

	spPr := &SpPr{}
	l.ApplyToSpPr(spPr)

	// "solid" dash should not set PrstDash element
	if spPr.Ln.PrstDash != nil {
		t.Error("expected PrstDash to be nil for solid dash")
	}
}

func TestLine_ZeroWidth(t *testing.T) {
	l := Line{
		Width: 0,
		Color: ColorBlack,
	}

	spPr := &SpPr{}
	l.ApplyToSpPr(spPr)

	if spPr.Ln.W != nil {
		t.Error("expected W to be nil for zero width")
	}
}

func TestLine_AllDashStyles(t *testing.T) {
	styles := []struct {
		dash DashStyle
		val  string
	}{
		{DashDot, "dot"},
		{DashDash, "dash"},
		{DashDashDot, "dashDot"},
		{DashLongDash, "lgDash"},
	}

	for _, s := range styles {
		l := Line{Width: 1, Color: ColorBlack, Dash: s.dash}
		spPr := &SpPr{}
		l.ApplyToSpPr(spPr)

		if spPr.Ln.PrstDash == nil {
			t.Errorf("Dash %s: expected PrstDash to be set", s.dash)
			continue
		}
		if spPr.Ln.PrstDash.Val != s.val {
			t.Errorf("Dash %s: PrstDash.Val = %s, want %s", s.dash, spPr.Ln.PrstDash.Val, s.val)
		}
	}
}
