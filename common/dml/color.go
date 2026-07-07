package dml

import (
	"fmt"
	"strconv"
	"strings"
)

// Color represents a color in DrawingML.
// Colors can be specified as RGB, theme colors, or system colors.
type Color struct {
	// Type indicates how the color is specified.
	Type ColorType

	// RGB is the RGB value when Type is ColorTypeRGB.
	RGB RGB

	// Theme is the theme color index when Type is ColorTypeTheme.
	Theme ThemeColor

	// Tint adjusts the color luminance (-1.0 to 1.0).
	// Positive values lighten, negative values darken.
	Tint float64

	// Alpha is the opacity (0-100000, where 100000 = 100%).
	Alpha int
}

// ColorType indicates how a color is specified.
type ColorType int

const (
	// ColorTypeRGB indicates an RGB color value.
	ColorTypeRGB ColorType = iota

	// ColorTypeTheme indicates a theme color reference.
	ColorTypeTheme

	// ColorTypeSystem indicates a system color reference.
	ColorTypeSystem
)

// RGB represents an RGB color value.
type RGB struct {
	R uint8
	G uint8
	B uint8
}

// NewRGB creates an RGB color from red, green, blue components (0-255).
func NewRGB(r, g, b uint8) RGB {
	return RGB{R: r, G: g, B: b}
}

// ParseRGB parses a hex color string (with or without # prefix).
func ParseRGB(hex string) (RGB, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return RGB{}, fmt.Errorf("invalid hex color: %s", hex)
	}

	r, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid red component: %w", err)
	}

	g, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid green component: %w", err)
	}

	b, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid blue component: %w", err)
	}

	return RGB{R: uint8(r), G: uint8(g), B: uint8(b)}, nil
}

// String returns the hex representation of the RGB color.
func (rgb RGB) String() string {
	return fmt.Sprintf("%02X%02X%02X", rgb.R, rgb.G, rgb.B)
}

// ToColor converts RGB to a Color.
func (rgb RGB) ToColor() Color {
	return Color{
		Type:  ColorTypeRGB,
		RGB:   rgb,
		Alpha: 100000, // Fully opaque
	}
}

// ThemeColor represents a reference to a theme color.
type ThemeColor int

// Theme color constants matching OOXML theme color indices.
const (
	ThemeColorDark1     ThemeColor = 0  // Usually black
	ThemeColorLight1    ThemeColor = 1  // Usually white
	ThemeColorDark2     ThemeColor = 2  // Secondary dark
	ThemeColorLight2    ThemeColor = 3  // Secondary light
	ThemeColorAccent1   ThemeColor = 4
	ThemeColorAccent2   ThemeColor = 5
	ThemeColorAccent3   ThemeColor = 6
	ThemeColorAccent4   ThemeColor = 7
	ThemeColorAccent5   ThemeColor = 8
	ThemeColorAccent6   ThemeColor = 9
	ThemeColorHyperlink ThemeColor = 10
	ThemeColorFollowedHyperlink ThemeColor = 11
)

// String returns the name of the theme color.
func (tc ThemeColor) String() string {
	names := map[ThemeColor]string{
		ThemeColorDark1:     "dk1",
		ThemeColorLight1:    "lt1",
		ThemeColorDark2:     "dk2",
		ThemeColorLight2:    "lt2",
		ThemeColorAccent1:   "accent1",
		ThemeColorAccent2:   "accent2",
		ThemeColorAccent3:   "accent3",
		ThemeColorAccent4:   "accent4",
		ThemeColorAccent5:   "accent5",
		ThemeColorAccent6:   "accent6",
		ThemeColorHyperlink: "hlink",
		ThemeColorFollowedHyperlink: "folHlink",
	}
	if name, ok := names[tc]; ok {
		return name
	}
	return fmt.Sprintf("theme%d", tc)
}

// ToColor converts ThemeColor to a Color.
func (tc ThemeColor) ToColor() Color {
	return Color{
		Type:  ColorTypeTheme,
		Theme: tc,
		Alpha: 100000, // Fully opaque
	}
}

// Predefined colors
var (
	ColorBlack   = NewRGB(0, 0, 0).ToColor()
	ColorWhite   = NewRGB(255, 255, 255).ToColor()
	ColorRed     = NewRGB(255, 0, 0).ToColor()
	ColorGreen   = NewRGB(0, 255, 0).ToColor()
	ColorBlue    = NewRGB(0, 0, 255).ToColor()
	ColorYellow  = NewRGB(255, 255, 0).ToColor()
	ColorCyan    = NewRGB(0, 255, 255).ToColor()
	ColorMagenta = NewRGB(255, 0, 255).ToColor()
)

// WithAlpha returns a copy of the color with the specified alpha percentage,
// clamped to the valid 0-100 range so out-of-range inputs cannot produce an
// invalid opacity value.
func (c Color) WithAlpha(alphaPercent int) Color {
	if alphaPercent < 0 {
		alphaPercent = 0
	} else if alphaPercent > 100 {
		alphaPercent = 100
	}
	copy := c
	copy.Alpha = alphaPercent * 1000
	return copy
}

// WithTint returns a copy of the color with the specified tint (-1.0 to 1.0).
func (c Color) WithTint(tint float64) Color {
	copy := c
	if tint < -1.0 {
		tint = -1.0
	} else if tint > 1.0 {
		tint = 1.0
	}
	copy.Tint = tint
	return copy
}
