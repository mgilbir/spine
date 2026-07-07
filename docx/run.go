package docx

import (
	"fmt"
	"strconv"

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
	r.r.T = []*oxml.CT_Text{{Space: "preserve", Text: text}}
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
	r.r.AppendBr(&oxml.CT_Br{})
}

// AddTab adds a tab to the run.
func (r *Run) AddTab() {
	r.r.AppendTab()
}

// Clear removes all content from the run.
func (r *Run) Clear() {
	r.r.T = nil
	r.r.Br = nil
	r.r.Tab = nil
	r.r.Cr = nil
	r.r.Sym = nil
	r.r.Drawing = nil
	r.r.FtnRef = nil
	r.r.EndnoteRef = nil
	r.r.LastRenderedPageBreak = nil
	r.r.NoBreakHyphen = nil
	r.r.SoftHyphen = nil
	r.r.FldChar = nil
	r.r.InstrText = nil
}

func (r *Run) ensureRPr() {
	if r.r.RPr == nil {
		r.r.RPr = &oxml.CT_RPr{}
	}
}
