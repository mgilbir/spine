package dml

import (
	"math"
	"testing"
)

func TestEMUConversions(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		convert  func(float64) EMU
		expected EMU
	}{
		{"1 inch", 1.0, Inches, 914400},
		{"0.5 inches", 0.5, Inches, 457200},
		{"10 points", 10.0, Points, 127000},
		{"72 points (1 inch)", 72.0, Points, 914400},
		{"1 centimeter", 1.0, Centimeters, 360000},
		{"2.54 centimeters (1 inch)", 2.54, Centimeters, 914400},
		{"1 millimeter", 1.0, Millimeters, 36000},
		{"25.4 millimeters (1 inch)", 25.4, Millimeters, 914400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.convert(tt.input)
			if got != tt.expected {
				t.Errorf("%s = %d EMU, want %d", tt.name, got, tt.expected)
			}
		})
	}
}

func TestPixels(t *testing.T) {
	// At 96 DPI, 1 inch = 96 pixels = 914400 EMU
	got := Pixels(96)
	expected := EMU(914400)
	if got != expected {
		t.Errorf("Pixels(96) = %d, want %d", got, expected)
	}

	got = Pixels(1)
	if got != EMUsPerPixel {
		t.Errorf("Pixels(1) = %d, want %d", got, EMUsPerPixel)
	}
}

func TestEMU_ToInches(t *testing.T) {
	tests := []struct {
		name     string
		emu      EMU
		expected float64
	}{
		{"1 inch", 914400, 1.0},
		{"0.5 inches", 457200, 0.5},
		{"0 inches", 0, 0.0},
		{"2 inches", 1828800, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.emu.ToInches()
			if math.Abs(got-tt.expected) > 0.0001 {
				t.Errorf("EMU(%d).ToInches() = %f, want %f", tt.emu, got, tt.expected)
			}
		})
	}
}

func TestEMU_ToPoints(t *testing.T) {
	got := EMU(914400).ToPoints() // 1 inch = 72 points
	if math.Abs(got-72.0) > 0.0001 {
		t.Errorf("ToPoints() = %f, want 72", got)
	}
}

func TestEMU_ToCentimeters(t *testing.T) {
	got := EMU(914400).ToCentimeters() // 1 inch = 2.54 cm
	if math.Abs(got-2.54) > 0.01 {
		t.Errorf("ToCentimeters() = %f, want ~2.54", got)
	}
}

func TestEMU_ToMillimeters(t *testing.T) {
	got := EMU(914400).ToMillimeters() // 1 inch = 25.4 mm
	if math.Abs(got-25.4) > 0.1 {
		t.Errorf("ToMillimeters() = %f, want ~25.4", got)
	}
}

func TestEMU_ToPixels(t *testing.T) {
	got := EMU(914400).ToPixels() // 1 inch at 96 DPI = 96 pixels
	if got != 96 {
		t.Errorf("ToPixels() = %d, want 96", got)
	}
}

func TestEMU_RoundTrip(t *testing.T) {
	// Test that converting to and from EMU preserves values
	originalInches := 3.5
	emu := Inches(originalInches)
	backToInches := emu.ToInches()
	if math.Abs(backToInches-originalInches) > 0.0001 {
		t.Errorf("Round trip: %f -> %d -> %f", originalInches, emu, backToInches)
	}
}

func TestNewPoint(t *testing.T) {
	p := NewPoint(100, 200)
	if p.X != 100 || p.Y != 200 {
		t.Errorf("NewPoint(100, 200) = {%d, %d}, want {100, 200}", p.X, p.Y)
	}
}

func TestNewRect(t *testing.T) {
	r := NewRect(10, 20, 100, 50)
	if r.X != 10 || r.Y != 20 || r.Width != 100 || r.Height != 50 {
		t.Errorf("NewRect() returned unexpected values")
	}
}

func TestRect_Right(t *testing.T) {
	r := NewRect(10, 20, 100, 50)
	if r.Right() != 110 {
		t.Errorf("Right() = %d, want 110", r.Right())
	}
}

func TestRect_Bottom(t *testing.T) {
	r := NewRect(10, 20, 100, 50)
	if r.Bottom() != 70 {
		t.Errorf("Bottom() = %d, want 70", r.Bottom())
	}
}

func TestRect_Center(t *testing.T) {
	r := NewRect(0, 0, 100, 200)
	center := r.Center()
	if center.X != 50 || center.Y != 100 {
		t.Errorf("Center() = {%d, %d}, want {50, 100}", center.X, center.Y)
	}
}

func TestRect_Contains(t *testing.T) {
	r := NewRect(10, 20, 100, 50)

	tests := []struct {
		name     string
		point    Point
		expected bool
	}{
		{"inside", Point{50, 40}, true},
		{"top-left corner", Point{10, 20}, true},
		{"bottom-right corner", Point{110, 70}, true},
		{"outside left", Point{5, 40}, false},
		{"outside right", Point{115, 40}, false},
		{"outside top", Point{50, 15}, false},
		{"outside bottom", Point{50, 75}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Contains(tt.point)
			if got != tt.expected {
				t.Errorf("Contains(%v) = %v, want %v", tt.point, got, tt.expected)
			}
		})
	}
}

func TestNewSize(t *testing.T) {
	s := NewSize(100, 200)
	if s.Width != 100 || s.Height != 200 {
		t.Errorf("NewSize(100, 200) = {%d, %d}, want {100, 200}", s.Width, s.Height)
	}
}

func TestNewOffset(t *testing.T) {
	o := NewOffset(50, 75)
	if o.X != 50 || o.Y != 75 {
		t.Errorf("NewOffset(50, 75) = {%d, %d}, want {50, 75}", o.X, o.Y)
	}
}

func TestNewExtents(t *testing.T) {
	e := NewExtents(300, 400)
	if e.Cx != 300 || e.Cy != 400 {
		t.Errorf("NewExtents(300, 400) = {%d, %d}, want {300, 400}", e.Cx, e.Cy)
	}
}

func TestEMUConstants(t *testing.T) {
	// Verify relationships between constants
	if EMUsPerInch != 914400 {
		t.Errorf("EMUsPerInch = %d, want 914400", EMUsPerInch)
	}

	// 1 inch = 72 points
	if EMUsPerPoint*72 != EMUsPerInch {
		t.Errorf("EMUsPerPoint*72 = %d, want %d", EMUsPerPoint*72, EMUsPerInch)
	}

	// 1 inch = 2.54 cm
	expectedCm := EMU(float64(EMUsPerInch) / 2.54)
	if math.Abs(float64(EMUsPerCentimeter-expectedCm)) > 1 {
		t.Errorf("EMUsPerCentimeter = %d, want ~%d", EMUsPerCentimeter, expectedCm)
	}

	// 1 cm = 10 mm
	if EMUsPerCentimeter != EMUsPerMillimeter*10 {
		t.Errorf("EMUsPerCentimeter = %d, want %d", EMUsPerCentimeter, EMUsPerMillimeter*10)
	}
}
