package docx

import (
	"fmt"
	"strconv"

	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Run represents a run of text with consistent formatting.
type Run struct {
	paragraph *Paragraph
	r         *oxml.CT_R
}

// Text returns the text content of the run.
func (r *Run) Text() string {
	text := ""
	for _, t := range r.r.T {
		text += t.Text
	}
	return text
}

// SetText sets the text content, replacing all existing text elements.
func (r *Run) SetText(text string) {
	r.touch()
	r.r.SetTexts([]*oxml.CT_Text{{Space: "preserve", Text: text}})
}

// Bold returns whether the run is bold.
func (r *Run) Bold() bool {
	if r.r.RPr == nil || r.r.RPr.B == nil {
		return false
	}
	return r.r.RPr.B.IsOn()
}

// SetBold sets the run's bold state explicitly. Unlike a plain toggle,
// SetBold(false) emits an explicit "off" (w:b w:val="false") so text that
// inherits bold from its style is actually un-bolded; use ClearBold to inherit
// the style's value instead.
func (r *Run) SetBold(bold bool) {
	r.ensureRPr()
	r.r.RPr.B = onOff(bold)
}

// ClearBold removes the run's explicit bold setting so it inherits from the
// paragraph/style.
func (r *Run) ClearBold() {
	r.touch()
	if r.r.RPr != nil {
		r.r.RPr.B = nil
	}
}

// onOff builds a CT_OnOff for an explicit on (val absent) or off (val="false").
func onOff(on bool) *oxml.CT_OnOff {
	if on {
		return &oxml.CT_OnOff{}
	}
	val := "false"
	return &oxml.CT_OnOff{Val: &val}
}

// Italic returns whether the run is italic.
func (r *Run) Italic() bool {
	if r.r.RPr == nil || r.r.RPr.I == nil {
		return false
	}
	return r.r.RPr.I.IsOn()
}

// SetItalic sets the run's italic state explicitly. SetItalic(false) emits an
// explicit "off"; use ClearItalic to inherit from the style.
func (r *Run) SetItalic(italic bool) {
	r.ensureRPr()
	r.r.RPr.I = onOff(italic)
}

// ClearItalic removes the run's explicit italic setting so it inherits.
func (r *Run) ClearItalic() {
	r.touch()
	if r.r.RPr != nil {
		r.r.RPr.I = nil
	}
}

// Underline returns whether the run is underlined.
func (r *Run) Underline() bool {
	if r.r.RPr == nil || r.r.RPr.U == nil {
		return false
	}
	return r.r.RPr.U.Val != "" && r.r.RPr.U.Val != "none"
}

// SetUnderline sets whether the run is underlined.
func (r *Run) SetUnderline(underline bool) {
	r.ensureRPr()
	if underline {
		r.r.RPr.U = &oxml.CT_Underline{Val: "single"}
	} else {
		r.r.RPr.U = nil
	}
}

// Strike returns whether the run has strikethrough.
func (r *Run) Strike() bool {
	if r.r.RPr == nil || r.r.RPr.Strike == nil {
		return false
	}
	return r.r.RPr.Strike.IsOn()
}

// SetStrike sets the run's strikethrough state explicitly. SetStrike(false)
// emits an explicit "off"; use ClearStrike to inherit from the style.
func (r *Run) SetStrike(strike bool) {
	r.ensureRPr()
	r.r.RPr.Strike = onOff(strike)
}

// ClearStrike removes the run's explicit strikethrough setting so it inherits.
func (r *Run) ClearStrike() {
	r.touch()
	if r.r.RPr != nil {
		r.r.RPr.Strike = nil
	}
}

// Font returns the font name (ASCII font).
func (r *Run) Font() string {
	if r.r.RPr == nil || r.r.RPr.RFonts == nil {
		return ""
	}
	return r.r.RPr.RFonts.Ascii
}

// SetFont sets the font name.
func (r *Run) SetFont(name string) {
	r.ensureRPr()
	if r.r.RPr.RFonts == nil {
		r.r.RPr.RFonts = &oxml.CT_Fonts{}
	}
	r.r.RPr.RFonts.Ascii = name
	r.r.RPr.RFonts.HAnsi = name
}

// FontSize returns the font size in points.
func (r *Run) FontSize() float64 {
	if r.r.RPr == nil || r.r.RPr.Sz == nil {
		return 0
	}
	// Sz.Val is in half-points
	hp, err := strconv.ParseFloat(r.r.RPr.Sz.Val, 64)
	if err != nil {
		return 0
	}
	return hp / 2
}

// SetFontSize sets the font size in points.
func (r *Run) SetFontSize(size float64) {
	r.ensureRPr()
	// Store as half-points
	hp := size * 2
	r.r.RPr.Sz = &oxml.CT_HpsMeasure{Val: fmt.Sprintf("%g", hp)}
}

// Color returns the text color as a hex string.
func (r *Run) Color() string {
	if r.r.RPr == nil || r.r.RPr.Color == nil {
		return ""
	}
	return r.r.RPr.Color.Val
}

// SetColor sets the text color as a hex string (e.g., "FF0000" for red).
func (r *Run) SetColor(color string) {
	r.ensureRPr()
	r.r.RPr.Color = &oxml.CT_Color{Val: color}
}

// AddBreak adds a line break to the run.
func (r *Run) AddBreak() {
	r.touch()
	r.r.AppendBr(&oxml.CT_Br{})
}

// AddTab adds a tab to the run.
func (r *Run) AddTab() {
	r.touch()
	r.r.AppendTab()
}

// UnderlineStyle names the line style of a run's underline (the w:u@val
// attribute, ST_Underline). The string values are the WordprocessingML tokens,
// so they serialize directly.
type UnderlineStyle string

const (
	UnderlineNone            UnderlineStyle = "none"
	UnderlineSingle          UnderlineStyle = "single"
	UnderlineWords           UnderlineStyle = "words"
	UnderlineDouble          UnderlineStyle = "double"
	UnderlineThick           UnderlineStyle = "thick"
	UnderlineDotted          UnderlineStyle = "dotted"
	UnderlineDottedHeavy     UnderlineStyle = "dottedHeavy"
	UnderlineDash            UnderlineStyle = "dash"
	UnderlineDashedHeavy     UnderlineStyle = "dashedHeavy"
	UnderlineDashLong        UnderlineStyle = "dashLong"
	UnderlineDashLongHeavy   UnderlineStyle = "dashLongHeavy"
	UnderlineDotDash         UnderlineStyle = "dotDash"
	UnderlineDashDotHeavy    UnderlineStyle = "dashDotHeavy"
	UnderlineDotDotDash      UnderlineStyle = "dotDotDash"
	UnderlineDashDotDotHeavy UnderlineStyle = "dashDotDotHeavy"
	UnderlineWave            UnderlineStyle = "wave"
	UnderlineWavyHeavy       UnderlineStyle = "wavyHeavy"
	UnderlineWavyDouble      UnderlineStyle = "wavyDouble"
)

// UnderlineStyle returns the run's underline line style (the w:u@val token), or
// an empty string when the run sets no underline.
func (r *Run) UnderlineStyle() UnderlineStyle {
	if r.r.RPr == nil || r.r.RPr.U == nil {
		return ""
	}
	return UnderlineStyle(r.r.RPr.U.Val)
}

// SetUnderlineStyle sets the run's underline line style (e.g. UnderlineDouble,
// UnderlineWavy). It is the richer counterpart to SetUnderline(bool): the plain
// boolean setter is preserved. Any underline color already set is kept.
func (r *Run) SetUnderlineStyle(style UnderlineStyle) {
	r.ensureRPr()
	if r.r.RPr.U == nil {
		r.r.RPr.U = &oxml.CT_Underline{}
	}
	r.r.RPr.U.Val = string(style)
}

// UnderlineColor returns the run's underline color as a hex string (w:u@color),
// or an empty string when none is set.
func (r *Run) UnderlineColor() string {
	if r.r.RPr == nil || r.r.RPr.U == nil {
		return ""
	}
	return r.r.RPr.U.Color
}

// SetUnderlineColor sets the run's underline color as a hex string (e.g.
// "FF0000"). It creates a single underline if the run has none yet, so the
// color has a line to apply to.
func (r *Run) SetUnderlineColor(color string) {
	r.ensureRPr()
	if r.r.RPr.U == nil {
		r.r.RPr.U = &oxml.CT_Underline{Val: "single"}
	}
	r.r.RPr.U.Color = color
}

// Highlight returns the run's highlight color name (w:highlight, a named
// ST_HighlightColor value such as "yellow"), or an empty string when none.
func (r *Run) Highlight() string {
	if r.r.RPr == nil || r.r.RPr.Highlight == nil {
		return ""
	}
	return r.r.RPr.Highlight.Val
}

// SetHighlight sets the run's highlight to a named color (e.g. "yellow",
// "green", "cyan"). Passing an empty string or "none" removes the highlight.
func (r *Run) SetHighlight(color string) {
	r.touch()
	if color == "" {
		if r.r.RPr != nil {
			r.r.RPr.Highlight = nil
		}
		return
	}
	r.ensureRPr()
	r.r.RPr.Highlight = &oxml.CT_Highlight{Val: color}
}

// VerticalAlign returns the run's vertical alignment (baseline, superscript, or
// subscript), or an empty string when the run sets none.
func (r *Run) VerticalAlign() enum.VerticalAlignRun {
	if r.r.RPr == nil || r.r.RPr.VertAlign == nil {
		return ""
	}
	return enum.VerticalAlignRun(r.r.RPr.VertAlign.Val)
}

// SetVerticalAlign sets the run as superscript, subscript, or baseline
// (w:vertAlign). Passing an empty string clears the setting so the run inherits
// its vertical alignment.
func (r *Run) SetVerticalAlign(align enum.VerticalAlignRun) {
	r.touch()
	if align == "" {
		if r.r.RPr != nil {
			r.r.RPr.VertAlign = nil
		}
		return
	}
	r.ensureRPr()
	r.r.RPr.VertAlign = &oxml.CT_VerticalAlignRun{Val: string(align)}
}

// Superscript reports whether the run is rendered as superscript.
func (r *Run) Superscript() bool {
	return r.VerticalAlign() == enum.VerticalAlignRunSuperscript
}

// SetSuperscript sets the run as superscript (on) or baseline (off).
func (r *Run) SetSuperscript(on bool) {
	if on {
		r.SetVerticalAlign(enum.VerticalAlignRunSuperscript)
	} else {
		r.SetVerticalAlign(enum.VerticalAlignRunBaseline)
	}
}

// Subscript reports whether the run is rendered as subscript.
func (r *Run) Subscript() bool {
	return r.VerticalAlign() == enum.VerticalAlignRunSubscript
}

// SetSubscript sets the run as subscript (on) or baseline (off).
func (r *Run) SetSubscript(on bool) {
	if on {
		r.SetVerticalAlign(enum.VerticalAlignRunSubscript)
	} else {
		r.SetVerticalAlign(enum.VerticalAlignRunBaseline)
	}
}

// Caps returns whether the run renders as all capitals (w:caps).
func (r *Run) Caps() bool {
	return r.r.RPr != nil && r.r.RPr.Caps.IsOn()
}

// SetCaps sets the run's all-capitals state explicitly. SetCaps(false) emits an
// explicit "off"; use ClearCaps to inherit from the style.
func (r *Run) SetCaps(caps bool) {
	r.ensureRPr()
	r.r.RPr.Caps = onOff(caps)
}

// ClearCaps removes the run's explicit all-capitals setting so it inherits.
func (r *Run) ClearCaps() {
	r.touch()
	if r.r.RPr != nil {
		r.r.RPr.Caps = nil
	}
}

// SmallCaps returns whether the run renders as small capitals (w:smallCaps).
func (r *Run) SmallCaps() bool {
	return r.r.RPr != nil && r.r.RPr.SmallCaps.IsOn()
}

// SetSmallCaps sets the run's small-capitals state explicitly.
// SetSmallCaps(false) emits an explicit "off"; use ClearSmallCaps to inherit.
func (r *Run) SetSmallCaps(smallCaps bool) {
	r.ensureRPr()
	r.r.RPr.SmallCaps = onOff(smallCaps)
}

// ClearSmallCaps removes the run's explicit small-capitals setting so it
// inherits from the style.
func (r *Run) ClearSmallCaps() {
	r.touch()
	if r.r.RPr != nil {
		r.r.RPr.SmallCaps = nil
	}
}

// CharacterSpacing returns the additional character spacing in points
// (w:spacing). Positive values expand, negative values condense. Returns 0 when
// unset.
func (r *Run) CharacterSpacing() float64 {
	if r.r.RPr == nil || r.r.RPr.Spacing == nil {
		return 0
	}
	return twipsToPoints(r.r.RPr.Spacing.Val)
}

// SetCharacterSpacing sets the additional character spacing in points
// (w:spacing). Positive values expand the spacing, negative values condense it.
func (r *Run) SetCharacterSpacing(points float64) {
	r.ensureRPr()
	r.r.RPr.Spacing = &oxml.CT_SignedTwipsMeasure{Val: pointsToTwips(points)}
}

// Position returns the run's vertical text position in points (w:position).
// Positive values raise the text, negative values lower it. Returns 0 when
// unset.
func (r *Run) Position() float64 {
	if r.r.RPr == nil || r.r.RPr.Position == nil {
		return 0
	}
	hp, err := strconv.ParseFloat(r.r.RPr.Position.Val, 64)
	if err != nil {
		return 0
	}
	return hp / 2
}

// SetPosition sets the run's vertical text position in points (w:position).
// Positive values raise the text above the baseline, negative values lower it.
func (r *Run) SetPosition(points float64) {
	r.ensureRPr()
	r.r.RPr.Position = &oxml.CT_SignedHpsMeasure{Val: fmt.Sprintf("%g", points*2)}
}

// Kerning returns the minimum font size in points at which kerning is applied
// (w:kern). Returns 0 when unset.
func (r *Run) Kerning() float64 {
	if r.r.RPr == nil || r.r.RPr.Kern == nil {
		return 0
	}
	hp, err := strconv.ParseFloat(r.r.RPr.Kern.Val, 64)
	if err != nil {
		return 0
	}
	return hp / 2
}

// SetKerning sets the minimum font size in points at which the run's characters
// are kerned (w:kern). A value of 0 disables kerning.
func (r *Run) SetKerning(points float64) {
	r.ensureRPr()
	r.r.RPr.Kern = &oxml.CT_HpsMeasure{Val: fmt.Sprintf("%g", points*2)}
}

// Style returns the run's character style id (w:rStyle), or "" when the run
// applies no character style.
func (r *Run) Style() string {
	if r.r.RPr == nil || r.r.RPr.RStyle == nil {
		return ""
	}
	return r.r.RPr.RStyle.Val
}

// SetStyle applies the character style with the given id to the run (w:rStyle),
// complementing the paragraph and style-definition APIs. Passing "" removes the
// character style so the run inherits from its paragraph style.
func (r *Run) SetStyle(id string) {
	r.touch()
	if id == "" {
		if r.r.RPr != nil {
			r.r.RPr.RStyle = nil
		}
		return
	}
	r.ensureRPr()
	r.r.RPr.RStyle = &oxml.CT_String{Val: id}
}

// AddSymbol appends a symbol glyph (w:sym) to the run: a single character drawn
// from a specific symbol font, addressed by the font name and the character's
// code point as a hex string (e.g. font "Wingdings", char "F0E0"). The 0xF000
// offset Word uses for symbol fonts is part of the stored value and is not added
// here.
func (r *Run) AddSymbol(font, char string) {
	r.touch()
	r.r.AppendSym(&oxml.CT_Sym{Font: font, Char: char})
}

// Clear removes all content from the run. Relationships referenced only by the
// removed content — a drawing's r:embed, an OLE object's r:id — are reclaimed,
// along with any media part added in this session that nothing else points at
// (C407).
func (r *Run) Clear() {
	r.touch()
	removed := make(map[string]bool)
	addRunRelRefs(removed, r.r)
	r.r.ClearContent()
	if r.paragraph != nil {
		r.paragraph.sweepRemovedRelRefs(removed)
	}
}

func (r *Run) ensureRPr() {
	// Every property setter funnels through here before mutating, so flagging
	// the owning header/footer part here covers all of them in one place: an
	// edit through a live handle into a reopened header/footer must regenerate
	// that part on save, not write the preserved raw bytes. A no-op for runs in
	// the main document part.
	r.touch()
	if r.r.RPr == nil {
		r.r.RPr = &oxml.CT_RPr{}
	}
}

// touch flags the header/footer part this run belongs to as modified, so an
// edit made through a live handle into a reopened header/footer is written back
// instead of being masked by the preserved original bytes. It resolves to a
// no-op for runs in the main document part (markHdrFtrModified only acts on a
// preserved header/footer part).
func (r *Run) touch() {
	if r != nil && r.paragraph != nil {
		r.paragraph.touch()
	}
}
