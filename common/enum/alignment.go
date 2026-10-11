// Package enum provides shared enumerations used across OOXML formats.
package enum

// TextAlign specifies horizontal text alignment.
type TextAlign string

// Paragraph alignments. The values are the DrawingML ST_TextAlignType tokens
// (a:pPr@algn), as used by pptx Paragraph.SetAlignment.
const (
	TextAlignLeft           TextAlign = "l"
	TextAlignCenter         TextAlign = "ctr"
	TextAlignRight          TextAlign = "r"
	TextAlignJustify        TextAlign = "just"
	TextAlignJustifyLow     TextAlign = "justLow"
	TextAlignDistribute     TextAlign = "dist"
	TextAlignThaiDistribute TextAlign = "thaiDist"
)

// VerticalAlign specifies vertical text alignment within a container.
type VerticalAlign string

// Vertical alignments, as used by pptx TableCell.SetVerticalAlign. The values
// are DrawingML ST_TextAnchoringType tokens (a:tcPr@anchor).
const (
	VerticalAlignTop    VerticalAlign = "t"
	VerticalAlignMiddle VerticalAlign = "ctr"
	VerticalAlignBottom VerticalAlign = "b"
)

// TextAnchor specifies the anchoring position of text within a shape.
type TextAnchor string

// Text anchors for a shape's text frame (pptx TextFrame.SetAnchor). The values
// are DrawingML ST_TextAnchoringType tokens (a:bodyPr@anchor).
const (
	TextAnchorTop    TextAnchor = "t"
	TextAnchorMiddle TextAnchor = "ctr"
	TextAnchorBottom TextAnchor = "b"
)

// TextWrapping specifies how text wraps within a container.
type TextWrapping string

// Text wrapping modes for a text frame (pptx TextFrame.SetWordWrap):
// TextWrappingSquare wraps lines at the frame's width, TextWrappingNone lets
// them run past it. The values are DrawingML ST_TextWrappingType tokens
// (a:bodyPr@wrap).
const (
	TextWrappingNone   TextWrapping = "none"
	TextWrappingSquare TextWrapping = "square"
)

// FontStyle represents text styling options.
type FontStyle int

// FontStyleNormal is the zero value: no styling flags set.
const FontStyleNormal FontStyle = 0

// Font style flags, combined with | or With and tested with Has.
const (
	// FontStyleBold is bit 0. The flags start at 1<<0 so no bit is wasted;
	// they are never serialized as their integer value (they map to boolean
	// XML attributes), so the specific bit positions are an implementation
	// detail callers reach only through the named constants.
	FontStyleBold FontStyle = 1 << iota
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

// Underline styles. The values are the DrawingML ST_TextUnderlineType tokens
// (a:rPr@u); they differ from WordprocessingML's ("sng" rather than
// "single"), so docx.UnderlineStyle is a separate type.
const (
	UnderlineNone            UnderlineStyle = "none"
	UnderlineWords           UnderlineStyle = "words"
	UnderlineSingle          UnderlineStyle = "sng"
	UnderlineDouble          UnderlineStyle = "dbl"
	UnderlineHeavy           UnderlineStyle = "heavy"
	UnderlineDotted          UnderlineStyle = "dotted"
	UnderlineDottedHeavy     UnderlineStyle = "dottedHeavy"
	UnderlineDash            UnderlineStyle = "dash"
	UnderlineDashHeavy       UnderlineStyle = "dashHeavy"
	UnderlineDashLong        UnderlineStyle = "dashLong"
	UnderlineDashLongHeavy   UnderlineStyle = "dashLongHeavy"
	UnderlineDotDash         UnderlineStyle = "dotDash"
	UnderlineDotDashHeavy    UnderlineStyle = "dotDashHeavy"
	UnderlineDotDotDash      UnderlineStyle = "dotDotDash"
	UnderlineDotDotDashHeavy UnderlineStyle = "dotDotDashHeavy"
	UnderlineWavy            UnderlineStyle = "wavy"
	UnderlineWavyHeavy       UnderlineStyle = "wavyHeavy"
	UnderlineWavyDouble      UnderlineStyle = "wavyDbl"
)

// StrikeStyle specifies the type of strikethrough.
type StrikeStyle string

// Strikethrough styles. The values are the DrawingML ST_TextStrikeType tokens
// (a:rPr@strike).
const (
	StrikeNone   StrikeStyle = "noStrike"
	StrikeSingle StrikeStyle = "sngStrike"
	StrikeDouble StrikeStyle = "dblStrike"
)

// VerticalAlignRun specifies the vertical alignment (baseline offset) of a run
// of characters: normal baseline, superscript, or subscript. The string values
// are the ST_VerticalAlignRun tokens shared by WordprocessingML (w:vertAlign)
// and SpreadsheetML (x:vertAlign), so they serialize directly in both formats.
type VerticalAlignRun string

// Run vertical alignments. VerticalAlignRunBaseline is the explicit form of
// "neither superscript nor subscript".
const (
	VerticalAlignRunBaseline    VerticalAlignRun = "baseline"
	VerticalAlignRunSuperscript VerticalAlignRun = "superscript"
	VerticalAlignRunSubscript   VerticalAlignRun = "subscript"
)

// Orientation specifies content orientation.
type Orientation string

// Content orientations. No spine API currently takes or returns an
// Orientation; docx has its own Orientation type and xlsx its own
// page-orientation constants.
const (
	OrientationDefault   Orientation = "default"
	OrientationPortrait  Orientation = "portrait"
	OrientationLandscape Orientation = "landscape"
)
