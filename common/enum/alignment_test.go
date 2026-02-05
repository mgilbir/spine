package enum

import (
	"testing"
)

func TestTextAlign_Values(t *testing.T) {
	tests := []struct {
		align TextAlign
		want  string
	}{
		{TextAlignLeft, "l"},
		{TextAlignCenter, "ctr"},
		{TextAlignRight, "r"},
		{TextAlignJustify, "just"},
		{TextAlignDistribute, "dist"},
	}

	for _, tt := range tests {
		if string(tt.align) != tt.want {
			t.Errorf("TextAlign value = %q, want %q", tt.align, tt.want)
		}
	}
}

func TestVerticalAlign_Values(t *testing.T) {
	tests := []struct {
		align VerticalAlign
		want  string
	}{
		{VerticalAlignTop, "t"},
		{VerticalAlignMiddle, "ctr"},
		{VerticalAlignBottom, "b"},
	}

	for _, tt := range tests {
		if string(tt.align) != tt.want {
			t.Errorf("VerticalAlign value = %q, want %q", tt.align, tt.want)
		}
	}
}

func TestTextAnchor_Values(t *testing.T) {
	tests := []struct {
		anchor TextAnchor
		want   string
	}{
		{TextAnchorTop, "t"},
		{TextAnchorMiddle, "ctr"},
		{TextAnchorBottom, "b"},
	}

	for _, tt := range tests {
		if string(tt.anchor) != tt.want {
			t.Errorf("TextAnchor value = %q, want %q", tt.anchor, tt.want)
		}
	}
}

func TestTextWrapping_Values(t *testing.T) {
	tests := []struct {
		wrap TextWrapping
		want string
	}{
		{TextWrappingNone, "none"},
		{TextWrappingSquare, "square"},
	}

	for _, tt := range tests {
		if string(tt.wrap) != tt.want {
			t.Errorf("TextWrapping value = %q, want %q", tt.wrap, tt.want)
		}
	}
}

func TestFontStyle_Has(t *testing.T) {
	style := FontStyleBold | FontStyleItalic

	if !style.Has(FontStyleBold) {
		t.Error("Has(FontStyleBold) should return true")
	}
	if !style.Has(FontStyleItalic) {
		t.Error("Has(FontStyleItalic) should return true")
	}
	if style.Has(FontStyleUnderline) {
		t.Error("Has(FontStyleUnderline) should return false")
	}
	if style.Has(FontStyleStrikethrough) {
		t.Error("Has(FontStyleStrikethrough) should return false")
	}
}

func TestFontStyle_With(t *testing.T) {
	style := FontStyleNormal

	style = style.With(FontStyleBold)
	if !style.Has(FontStyleBold) {
		t.Error("With(FontStyleBold) should add bold")
	}

	style = style.With(FontStyleItalic)
	if !style.Has(FontStyleBold) || !style.Has(FontStyleItalic) {
		t.Error("With(FontStyleItalic) should preserve bold and add italic")
	}
}

func TestFontStyle_Without(t *testing.T) {
	style := FontStyleBold | FontStyleItalic | FontStyleUnderline

	style = style.Without(FontStyleItalic)
	if style.Has(FontStyleItalic) {
		t.Error("Without(FontStyleItalic) should remove italic")
	}
	if !style.Has(FontStyleBold) || !style.Has(FontStyleUnderline) {
		t.Error("Without(FontStyleItalic) should preserve other styles")
	}
}

func TestFontStyle_Normal(t *testing.T) {
	style := FontStyleNormal

	if style.Has(FontStyleBold) {
		t.Error("FontStyleNormal should not have bold")
	}
	if style.Has(FontStyleItalic) {
		t.Error("FontStyleNormal should not have italic")
	}
	if style.Has(FontStyleUnderline) {
		t.Error("FontStyleNormal should not have underline")
	}
	if style.Has(FontStyleStrikethrough) {
		t.Error("FontStyleNormal should not have strikethrough")
	}
}

func TestUnderlineStyle_Values(t *testing.T) {
	tests := []struct {
		style UnderlineStyle
		want  string
	}{
		{UnderlineNone, "none"},
		{UnderlineSingle, "sng"},
		{UnderlineDouble, "dbl"},
		{UnderlineHeavy, "heavy"},
		{UnderlineDotted, "dotted"},
		{UnderlineDash, "dash"},
		{UnderlineDashLong, "dashLong"},
		{UnderlineDotDash, "dotDash"},
		{UnderlineDotDotDash, "dotDotDash"},
		{UnderlineWavy, "wavy"},
		{UnderlineWavyHeavy, "wavyHeavy"},
		{UnderlineWavyDouble, "wavyDbl"},
	}

	for _, tt := range tests {
		if string(tt.style) != tt.want {
			t.Errorf("UnderlineStyle value = %q, want %q", tt.style, tt.want)
		}
	}
}

func TestStrikeStyle_Values(t *testing.T) {
	tests := []struct {
		style StrikeStyle
		want  string
	}{
		{StrikeNone, "noStrike"},
		{StrikeSingle, "sngStrike"},
		{StrikeDouble, "dblStrike"},
	}

	for _, tt := range tests {
		if string(tt.style) != tt.want {
			t.Errorf("StrikeStyle value = %q, want %q", tt.style, tt.want)
		}
	}
}

func TestOrientation_Values(t *testing.T) {
	tests := []struct {
		orient Orientation
		want   string
	}{
		{OrientationDefault, "default"},
		{OrientationPortrait, "portrait"},
		{OrientationLandscape, "landscape"},
	}

	for _, tt := range tests {
		if string(tt.orient) != tt.want {
			t.Errorf("Orientation value = %q, want %q", tt.orient, tt.want)
		}
	}
}

func TestFontStyle_Combinations(t *testing.T) {
	// Test combining multiple styles
	style := FontStyleNormal.With(FontStyleBold).With(FontStyleItalic).With(FontStyleUnderline)

	if !style.Has(FontStyleBold) {
		t.Error("Combined style should have bold")
	}
	if !style.Has(FontStyleItalic) {
		t.Error("Combined style should have italic")
	}
	if !style.Has(FontStyleUnderline) {
		t.Error("Combined style should have underline")
	}

	// Remove one style
	style = style.Without(FontStyleItalic)
	if style.Has(FontStyleItalic) {
		t.Error("After Without, should not have italic")
	}
	if !style.Has(FontStyleBold) || !style.Has(FontStyleUnderline) {
		t.Error("After Without(Italic), should still have bold and underline")
	}
}
