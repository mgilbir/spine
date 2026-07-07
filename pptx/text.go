package pptx

import (
	"strings"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
)

// TextFrame represents a text frame within a shape.
type TextFrame struct {
	paragraphs []*Paragraph
	anchor     enum.TextAnchor
	wrap       enum.TextWrapping
	margins    TextMargins
}

// TextMargins specifies the text margins within a text frame.
type TextMargins struct {
	Left   dml.EMU
	Top    dml.EMU
	Right  dml.EMU
	Bottom dml.EMU
}

// NewTextFrame creates a new text frame.
func NewTextFrame() *TextFrame {
	return &TextFrame{
		paragraphs: make([]*Paragraph, 0),
		anchor:     enum.TextAnchorTop,
		wrap:       enum.TextWrappingSquare,
		margins: TextMargins{
			Left:   dml.Inches(0.1),
			Top:    dml.Inches(0.05),
			Right:  dml.Inches(0.1),
			Bottom: dml.Inches(0.05),
		},
	}
}

// Paragraphs returns all paragraphs in the text frame.
func (tf *TextFrame) Paragraphs() []*Paragraph {
	return tf.paragraphs
}

// AddParagraph adds a new paragraph to the text frame.
func (tf *TextFrame) AddParagraph() *Paragraph {
	p := NewParagraph()
	tf.paragraphs = append(tf.paragraphs, p)
	return p
}

// SetText sets the text content, replacing all existing paragraphs.
// Multiple paragraphs can be specified by separating with newlines.
func (tf *TextFrame) SetText(text string) {
	tf.paragraphs = tf.paragraphs[:0]
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		p := tf.AddParagraph()
		p.AddRun().SetText(line)
	}
}

// Text returns all text content joined by newlines.
func (tf *TextFrame) Text() string {
	var lines []string
	for _, p := range tf.paragraphs {
		lines = append(lines, p.Text())
	}
	return strings.Join(lines, "\n")
}

// Anchor returns the vertical anchor position.
func (tf *TextFrame) Anchor() enum.TextAnchor {
	return tf.anchor
}

// SetAnchor sets the vertical anchor position.
func (tf *TextFrame) SetAnchor(anchor enum.TextAnchor) {
	tf.anchor = anchor
}

// WordWrap returns the text wrapping mode.
func (tf *TextFrame) WordWrap() enum.TextWrapping {
	return tf.wrap
}

// SetWordWrap sets the text wrapping mode.
func (tf *TextFrame) SetWordWrap(wrap enum.TextWrapping) {
	tf.wrap = wrap
}

// Margins returns the text margins.
func (tf *TextFrame) Margins() TextMargins {
	return tf.margins
}

// SetMargins sets the text margins.
func (tf *TextFrame) SetMargins(margins TextMargins) {
	tf.margins = margins
}

// Paragraph represents a paragraph of text.
type Paragraph struct {
	runs       []*Run
	alignment  enum.TextAlign
	level      int
	lineSpacing int32 // in hundredths of a percent (100000 = 100%)
	spaceBefore dml.EMU
	spaceAfter  dml.EMU
	bulletType  BulletType
	bulletChar  string
}

// BulletType specifies the type of bullet for a paragraph.
type BulletType int

const (
	// BulletInherit is the zero value: the paragraph does not set any bullet
	// property and inherits it from the layout/master. This is distinct from
	// BulletNone, which explicitly suppresses an inherited bullet.
	BulletInherit BulletType = iota
	// BulletNone explicitly renders no bullet (emits <a:buNone/>).
	BulletNone
	// BulletAuto renders an automatic number.
	BulletAuto
	// BulletChar renders a literal bullet character (see SetBulletChar).
	BulletChar
	// BulletNumber renders an automatic number (arabicPeriod).
	BulletNumber
)

// NewParagraph creates a new paragraph.
func NewParagraph() *Paragraph {
	return &Paragraph{
		runs:        make([]*Run, 0),
		alignment:   enum.TextAlignLeft,
		lineSpacing: 100000, // 100%
	}
}

// Runs returns all runs in the paragraph.
func (p *Paragraph) Runs() []*Run {
	return p.runs
}

// AddRun adds a new run to the paragraph.
func (p *Paragraph) AddRun() *Run {
	r := NewRun()
	p.runs = append(p.runs, r)
	return r
}

// Text returns the text content of all runs.
func (p *Paragraph) Text() string {
	var sb strings.Builder
	for _, run := range p.runs {
		sb.WriteString(run.text)
	}
	return sb.String()
}

// Alignment returns the paragraph alignment.
func (p *Paragraph) Alignment() enum.TextAlign {
	return p.alignment
}

// SetAlignment sets the paragraph alignment.
func (p *Paragraph) SetAlignment(align enum.TextAlign) {
	p.alignment = align
}

// Level returns the indentation level (0-8).
func (p *Paragraph) Level() int {
	return p.level
}

// SetLevel sets the indentation level (0-8).
func (p *Paragraph) SetLevel(level int) {
	if level < 0 {
		level = 0
	} else if level > 8 {
		level = 8
	}
	p.level = level
}

// LineSpacing returns the line spacing in hundredths of a percent.
func (p *Paragraph) LineSpacing() int32 {
	return p.lineSpacing
}

// SetLineSpacing sets the line spacing in hundredths of a percent.
// 100000 = 100% (single spacing), 200000 = 200% (double spacing)
func (p *Paragraph) SetLineSpacing(spacing int32) {
	p.lineSpacing = spacing
}

// SpaceBefore returns the space before the paragraph.
func (p *Paragraph) SpaceBefore() dml.EMU {
	return p.spaceBefore
}

// SetSpaceBefore sets the space before the paragraph.
func (p *Paragraph) SetSpaceBefore(space dml.EMU) {
	p.spaceBefore = space
}

// SpaceAfter returns the space after the paragraph.
func (p *Paragraph) SpaceAfter() dml.EMU {
	return p.spaceAfter
}

// SetSpaceAfter sets the space after the paragraph.
func (p *Paragraph) SetSpaceAfter(space dml.EMU) {
	p.spaceAfter = space
}

// Bullet returns the bullet type.
func (p *Paragraph) Bullet() BulletType {
	return p.bulletType
}

// SetBullet sets the bullet type.
func (p *Paragraph) SetBullet(bulletType BulletType) {
	p.bulletType = bulletType
}

// BulletChar returns the bullet character.
func (p *Paragraph) BulletChar() string {
	return p.bulletChar
}

// SetBulletChar sets the bullet character.
func (p *Paragraph) SetBulletChar(char string) {
	p.bulletChar = char
	p.bulletType = BulletChar
}

// Run represents a run of text with consistent formatting.
type Run struct {
	text       string
	fontName   string
	fontSize   float64 // in points
	bold       bool
	italic     bool
	underline  enum.UnderlineStyle
	strike     enum.StrikeStyle
	color      *dml.Color
	highlight  *dml.Color
	baseline   int32 // percentage, positive for superscript, negative for subscript
}

// NewRun creates a new run. The font size defaults to 0 (unset) so a run added
// to a placeholder inherits the placeholder/layout size instead of being
// clobbered with an explicit sz; set it explicitly with SetFontSize when needed.
func NewRun() *Run {
	return &Run{
		underline: enum.UnderlineNone,
		strike:    enum.StrikeNone,
	}
}

// Text returns the text content.
func (r *Run) Text() string {
	return r.text
}

// SetText sets the text content.
func (r *Run) SetText(text string) {
	r.text = text
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

// Underline returns the underline style.
func (r *Run) Underline() enum.UnderlineStyle {
	return r.underline
}

// SetUnderline sets the underline style.
func (r *Run) SetUnderline(style enum.UnderlineStyle) {
	r.underline = style
}

// Strike returns the strikethrough style.
func (r *Run) Strike() enum.StrikeStyle {
	return r.strike
}

// SetStrike sets the strikethrough style.
func (r *Run) SetStrike(style enum.StrikeStyle) {
	r.strike = style
}

// Color returns the text color.
func (r *Run) Color() *dml.Color {
	return r.color
}

// SetColor sets the text color.
func (r *Run) SetColor(color dml.Color) {
	r.color = &color
}

// Highlight returns the highlight color.
func (r *Run) Highlight() *dml.Color {
	return r.highlight
}

// SetHighlight sets the highlight color.
func (r *Run) SetHighlight(color dml.Color) {
	r.highlight = &color
}

// Baseline returns the baseline offset percentage.
// Positive values create superscript, negative values create subscript.
func (r *Run) Baseline() int32 {
	return r.baseline
}

// SetBaseline sets the baseline offset percentage.
func (r *Run) SetBaseline(baseline int32) {
	r.baseline = baseline
}

// SetSuperscript configures the run as superscript.
func (r *Run) SetSuperscript() {
	r.baseline = 30000 // 30%
}

// SetSubscript configures the run as subscript.
func (r *Run) SetSubscript() {
	r.baseline = -30000 // -30%
}
