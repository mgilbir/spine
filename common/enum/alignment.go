// Package enum provides shared enumerations used across OOXML formats.
package enum

// TextAlign specifies horizontal text alignment.
type TextAlign string

const (
	TextAlignLeft      TextAlign = "l"
	TextAlignCenter    TextAlign = "ctr"
	TextAlignRight     TextAlign = "r"
	TextAlignJustify   TextAlign = "just"
	TextAlignDistribute TextAlign = "dist"
)

// VerticalAlign specifies vertical text alignment within a container.
type VerticalAlign string

const (
	VerticalAlignTop    VerticalAlign = "t"
	VerticalAlignMiddle VerticalAlign = "ctr"
	VerticalAlignBottom VerticalAlign = "b"
)

// TextAnchor specifies the anchoring position of text within a shape.
type TextAnchor string

const (
	TextAnchorTop     TextAnchor = "t"
	TextAnchorMiddle  TextAnchor = "ctr"
	TextAnchorBottom  TextAnchor = "b"
)

// TextWrapping specifies how text wraps within a container.
type TextWrapping string

const (
	TextWrappingNone   TextWrapping = "none"
	TextWrappingSquare TextWrapping = "square"
)

// FontStyle represents text styling options.
type FontStyle int

const (
	FontStyleNormal FontStyle = 0
	FontStyleBold   FontStyle = 1 << iota
	FontStyleItalic
	FontStyleUnderline
	FontStyleStrikethrough
)

// Has returns true if the style includes the specified flag.
func (fs FontStyle) Has(flag FontStyle) bool {
	return fs&flag != 0
}

// With returns a new FontStyle with the specified flag added.
func (fs FontStyle) With(flag FontStyle) FontStyle {
	return fs | flag
}

// Without returns a new FontStyle with the specified flag removed.
func (fs FontStyle) Without(flag FontStyle) FontStyle {
	return fs &^ flag
}

// UnderlineStyle specifies the type of underline.
type UnderlineStyle string

const (
	UnderlineNone          UnderlineStyle = "none"
	UnderlineSingle        UnderlineStyle = "sng"
	UnderlineDouble        UnderlineStyle = "dbl"
	UnderlineHeavy         UnderlineStyle = "heavy"
	UnderlineDotted        UnderlineStyle = "dotted"
	UnderlineDash          UnderlineStyle = "dash"
	UnderlineDashLong      UnderlineStyle = "dashLong"
	UnderlineDotDash       UnderlineStyle = "dotDash"
	UnderlineDotDotDash    UnderlineStyle = "dotDotDash"
	UnderlineWavy          UnderlineStyle = "wavy"
	UnderlineWavyHeavy     UnderlineStyle = "wavyHeavy"
	UnderlineWavyDouble    UnderlineStyle = "wavyDbl"
)

// StrikeStyle specifies the type of strikethrough.
type StrikeStyle string

const (
	StrikeNone   StrikeStyle = "noStrike"
	StrikeSingle StrikeStyle = "sngStrike"
	StrikeDouble StrikeStyle = "dblStrike"
)

// Orientation specifies content orientation.
type Orientation string

const (
	OrientationDefault   Orientation = "default"
	OrientationPortrait  Orientation = "portrait"
	OrientationLandscape Orientation = "landscape"
)
