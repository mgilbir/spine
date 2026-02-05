package docx

// Run represents a run of text with consistent formatting.
type Run struct {
	paragraph *Paragraph
	text      string
	bold      bool
	italic    bool
	underline bool
	strike    bool
	fontName  string
	fontSize  float64
	color     string // hex color
}

// Text returns the text content of the run.
func (r *Run) Text() string {
	return r.text
}

// SetText sets the text content.
func (r *Run) SetText(text string) {
	r.text = text
}

// Bold returns whether the run is bold.
func (r *Run) Bold() bool {
	return r.bold
}

// SetBold sets whether the run is bold.
func (r *Run) SetBold(bold bool) {
	r.bold = bold
}

// Italic returns whether the run is italic.
func (r *Run) Italic() bool {
	return r.italic
}

// SetItalic sets whether the run is italic.
func (r *Run) SetItalic(italic bool) {
	r.italic = italic
}

// Underline returns whether the run is underlined.
func (r *Run) Underline() bool {
	return r.underline
}

// SetUnderline sets whether the run is underlined.
func (r *Run) SetUnderline(underline bool) {
	r.underline = underline
}

// Strike returns whether the run has strikethrough.
func (r *Run) Strike() bool {
	return r.strike
}

// SetStrike sets whether the run has strikethrough.
func (r *Run) SetStrike(strike bool) {
	r.strike = strike
}

// Font returns the font name.
func (r *Run) Font() string {
	return r.fontName
}

// SetFont sets the font name.
func (r *Run) SetFont(name string) {
	r.fontName = name
}

// FontSize returns the font size in points.
func (r *Run) FontSize() float64 {
	return r.fontSize
}

// SetFontSize sets the font size in points.
func (r *Run) SetFontSize(size float64) {
	r.fontSize = size
}

// Color returns the text color as a hex string.
func (r *Run) Color() string {
	return r.color
}

// SetColor sets the text color as a hex string (e.g., "FF0000" for red).
func (r *Run) SetColor(color string) {
	r.color = color
}

// AddText appends text to the run.
func (r *Run) AddText(text string) {
	r.text += text
}

// AddBreak adds a line break to the run.
func (r *Run) AddBreak() {
	r.text += "\n"
}

// AddTab adds a tab character to the run.
func (r *Run) AddTab() {
	r.text += "\t"
}

// Clear removes all text from the run.
func (r *Run) Clear() {
	r.text = ""
}

// Clone creates a copy of the run with the same formatting.
func (r *Run) Clone() *Run {
	return &Run{
		paragraph: r.paragraph,
		text:      r.text,
		bold:      r.bold,
		italic:    r.italic,
		underline: r.underline,
		strike:    r.strike,
		fontName:  r.fontName,
		fontSize:  r.fontSize,
		color:     r.color,
	}
}
