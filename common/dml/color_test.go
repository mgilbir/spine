package dml

import (
	"testing"
)

func TestNewRGB(t *testing.T) {
	rgb := NewRGB(255, 128, 64)
	if rgb.R != 255 || rgb.G != 128 || rgb.B != 64 {
		t.Errorf("NewRGB(255, 128, 64) = {%d, %d, %d}, want {255, 128, 64}", rgb.R, rgb.G, rgb.B)
	}
}

func TestParseRGB(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		want    RGB
		wantErr bool
	}{
		{"basic hex", "FF8040", RGB{255, 128, 64}, false},
		{"with hash", "#FF8040", RGB{255, 128, 64}, false},
		{"lowercase", "ff8040", RGB{255, 128, 64}, false},
		{"mixed case", "Ff8040", RGB{255, 128, 64}, false},
		{"black", "000000", RGB{0, 0, 0}, false},
		{"white", "FFFFFF", RGB{255, 255, 255}, false},
		{"red", "FF0000", RGB{255, 0, 0}, false},
		{"green", "00FF00", RGB{0, 255, 0}, false},
		{"blue", "0000FF", RGB{0, 0, 255}, false},
		{"too short", "FFF", RGB{}, true},
		{"too long", "FFFFFFF", RGB{}, true},
		{"invalid char", "GGGGGG", RGB{}, true},
		{"empty", "", RGB{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRGB(tt.hex)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRGB(%q) error = %v, wantErr %v", tt.hex, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseRGB(%q) = %v, want %v", tt.hex, got, tt.want)
			}
		})
	}
}

func TestRGB_String(t *testing.T) {
	tests := []struct {
		rgb  RGB
		want string
	}{
		{RGB{255, 128, 64}, "FF8040"},
		{RGB{0, 0, 0}, "000000"},
		{RGB{255, 255, 255}, "FFFFFF"},
		{RGB{15, 15, 15}, "0F0F0F"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.rgb.String()
			if got != tt.want {
				t.Errorf("RGB.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRGB_ToColor(t *testing.T) {
	rgb := NewRGB(255, 0, 0)
	color := rgb.ToColor()

	if color.Type != ColorTypeRGB {
		t.Errorf("ToColor().Type = %v, want ColorTypeRGB", color.Type)
	}
	if color.RGB != rgb {
		t.Errorf("ToColor().RGB = %v, want %v", color.RGB, rgb)
	}
	if color.Alpha != 100000 {
		t.Errorf("ToColor().Alpha = %d, want 100000", color.Alpha)
	}
}

func TestThemeColor_String(t *testing.T) {
	tests := []struct {
		tc   ThemeColor
		want string
	}{
		{ThemeColorDark1, "dk1"},
		{ThemeColorLight1, "lt1"},
		{ThemeColorDark2, "dk2"},
		{ThemeColorLight2, "lt2"},
		{ThemeColorAccent1, "accent1"},
		{ThemeColorAccent2, "accent2"},
		{ThemeColorAccent3, "accent3"},
		{ThemeColorAccent4, "accent4"},
		{ThemeColorAccent5, "accent5"},
		{ThemeColorAccent6, "accent6"},
		{ThemeColorHyperlink, "hlink"},
		{ThemeColorFollowedHyperlink, "folHlink"},
		{ThemeColor(99), "theme99"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.tc.String()
			if got != tt.want {
				t.Errorf("ThemeColor.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThemeColor_ToColor(t *testing.T) {
	tc := ThemeColorAccent1
	color := tc.ToColor()

	if color.Type != ColorTypeTheme {
		t.Errorf("ToColor().Type = %v, want ColorTypeTheme", color.Type)
	}
	if color.Theme != tc {
		t.Errorf("ToColor().Theme = %v, want %v", color.Theme, tc)
	}
	if color.Alpha != 100000 {
		t.Errorf("ToColor().Alpha = %d, want 100000", color.Alpha)
	}
}

func TestColor_WithAlpha(t *testing.T) {
	original := ColorRed
	modified := original.WithAlpha(50)

	// Original should be unchanged
	if original.Alpha != 100000 {
		t.Error("WithAlpha() modified original color")
	}

	// Modified should have new alpha
	if modified.Alpha != 50000 {
		t.Errorf("WithAlpha(50).Alpha = %d, want 50000", modified.Alpha)
	}

	// Other properties should be the same
	if modified.Type != original.Type {
		t.Error("WithAlpha() changed Type")
	}
	if modified.RGB != original.RGB {
		t.Error("WithAlpha() changed RGB")
	}
}

func TestColor_WithTint(t *testing.T) {
	tests := []struct {
		name     string
		tint     float64
		expected float64
	}{
		{"positive tint", 0.5, 0.5},
		{"negative tint", -0.5, -0.5},
		{"zero tint", 0.0, 0.0},
		{"max tint", 1.0, 1.0},
		{"min tint", -1.0, -1.0},
		{"over max", 1.5, 1.0},
		{"under min", -1.5, -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := ColorBlue
			modified := original.WithTint(tt.tint)

			if original.Tint != 0 {
				t.Error("WithTint() modified original color")
			}

			if modified.Tint != tt.expected {
				t.Errorf("WithTint(%f).Tint = %f, want %f", tt.tint, modified.Tint, tt.expected)
			}
		})
	}
}

func TestPredefinedColors(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		rgb   RGB
	}{
		{"black", ColorBlack, RGB{0, 0, 0}},
		{"white", ColorWhite, RGB{255, 255, 255}},
		{"red", ColorRed, RGB{255, 0, 0}},
		{"green", ColorGreen, RGB{0, 255, 0}},
		{"blue", ColorBlue, RGB{0, 0, 255}},
		{"yellow", ColorYellow, RGB{255, 255, 0}},
		{"cyan", ColorCyan, RGB{0, 255, 255}},
		{"magenta", ColorMagenta, RGB{255, 0, 255}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.Type != ColorTypeRGB {
				t.Errorf("%s.Type = %v, want ColorTypeRGB", tt.name, tt.color.Type)
			}
			if tt.color.RGB != tt.rgb {
				t.Errorf("%s.RGB = %v, want %v", tt.name, tt.color.RGB, tt.rgb)
			}
			if tt.color.Alpha != 100000 {
				t.Errorf("%s.Alpha = %d, want 100000", tt.name, tt.color.Alpha)
			}
		})
	}
}

func TestColorType_Values(t *testing.T) {
	// Ensure color types are distinct
	types := []ColorType{ColorTypeRGB, ColorTypeTheme, ColorTypeSystem}
	for i, t1 := range types {
		for j, t2 := range types {
			if i != j && t1 == t2 {
				t.Errorf("ColorType values at %d and %d should be distinct", i, j)
			}
		}
	}
}

func TestParseRGB_RoundTrip(t *testing.T) {
	hexColors := []string{"FF8040", "000000", "FFFFFF", "123456", "ABCDEF"}

	for _, hex := range hexColors {
		rgb, err := ParseRGB(hex)
		if err != nil {
			t.Errorf("ParseRGB(%q) error = %v", hex, err)
			continue
		}
		back := rgb.String()
		if back != hex {
			t.Errorf("Round trip: %q -> %v -> %q", hex, rgb, back)
		}
	}
}
