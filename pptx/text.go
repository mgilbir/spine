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

	autofit AutofitType
	// autofitDirty is set only by SetAutofit so an unmodified (or merely
	// anchor/wrap/margin-edited) text body leaves its parsed autofit element
	// untouched — reading the autofit for a getter must not force a rewrite.
	autofitDirty bool
	// marginsDirty is set only by SetMargins so an anchor/wrap-only edit does
	// not write the four insets. A parsed body without inset attributes
	// materializes zero-value margins that are indistinguishable from an
	// explicit zero; writing them would replace the inherited ~91440/45720
	// defaults with zeros and shift the text.
	marginsDirty bool

	// bodyDirty is set when anchor/wrap/margins change; contentDirty when the
	// paragraph list changes. Together with the per-paragraph and per-run
	// flags they let the shape sync update only what the caller touched (see
	// updateTxBody), leaving unmodeled parsed content alone.
	bodyDirty    bool
	contentDirty bool
}

// AutofitType selects how a text body resizes text and shape to fit each other
// (the a:bodyPr autofit child).
type AutofitType int

const (
	// AutofitInherit is the zero value: the text frame emits no autofit element
	// and inherits the setting from its placeholder/layout/master.
	AutofitInherit AutofitType = iota
	// AutofitNone disables autofit (a:noAutofit): text is neither shrunk nor is
	// the shape resized.
	AutofitNone
	// AutofitShape resizes the shape to fit its text (a:spAutoFit).
	AutofitShape
	// AutofitNormal shrinks the text so it fits the shape (a:normAutofit).
	AutofitNormal
)

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
	tf.contentDirty = true
	return p
}

// SetText sets the text content, replacing all existing paragraphs.
// Multiple paragraphs can be specified by separating with newlines.
func (tf *TextFrame) SetText(text string) {
	tf.paragraphs = tf.paragraphs[:0]
	tf.contentDirty = true
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
	tf.bodyDirty = true
}

// WordWrap returns the text wrapping mode.
func (tf *TextFrame) WordWrap() enum.TextWrapping {
	return tf.wrap
}

// SetWordWrap sets the text wrapping mode.
func (tf *TextFrame) SetWordWrap(wrap enum.TextWrapping) {
	tf.wrap = wrap
	tf.bodyDirty = true
}

// Autofit returns the text body's autofit mode.
func (tf *TextFrame) Autofit() AutofitType {
	return tf.autofit
}

// SetAutofit sets the text body's autofit mode (a:noAutofit / a:spAutoFit /
// a:normAutofit). AutofitInherit clears the setting so the frame inherits its
// autofit from the placeholder/layout/master.
func (tf *TextFrame) SetAutofit(autofit AutofitType) {
	tf.autofit = autofit
	tf.autofitDirty = true
	tf.bodyDirty = true
}

// Margins returns the text margins.
func (tf *TextFrame) Margins() TextMargins {
	return tf.margins
}

// SetMargins sets the text margins.
func (tf *TextFrame) SetMargins(margins TextMargins) {
	tf.margins = margins
	tf.marginsDirty = true
	tf.bodyDirty = true
}

// isContentDirty reports whether the paragraph content changed since the last
// sync (paragraph list, paragraph properties, or any run).
func (tf *TextFrame) isContentDirty() bool {
	if tf == nil {
		return false
	}
	if tf.contentDirty {
		return true
	}
	for _, p := range tf.paragraphs {
		if p.isDirty() {
			return true
		}
	}
	return false
}

// isDirty reports whether any part of the text frame changed since the last sync.
func (tf *TextFrame) isDirty() bool {
	return tf != nil && (tf.bodyDirty || tf.isContentDirty())
}

// clearDirty resets all modification flags after a sync flushed them.
func (tf *TextFrame) clearDirty() {
	if tf == nil {
		return
	}
	tf.bodyDirty = false
	tf.contentDirty = false
	tf.autofitDirty = false
	tf.marginsDirty = false
	for _, p := range tf.paragraphs {
		p.dirty = false
		for _, r := range p.runs {
			r.dirty = false
		}
	}
}

// Paragraph represents a paragraph of text.
type Paragraph struct {
	runs        []*Run
	alignment   enum.TextAlign
	level       int
	lineSpacing int32 // in hundredths of a percent (100000 = 100%); 0 = unset, inherit
	spaceBefore dml.EMU
	spaceAfter  dml.EMU
	bulletType  BulletType
	bulletChar  string
	// Bullet styling (a:buClr / a:buSzPct / a:buFont) applies to whichever
	// bullet kind the paragraph carries. bulletSizePct is in the dml.Percentage
	// unit scale (100000 = 100%); 0 means unset.
	bulletColor          *dml.Color
	bulletSizePct        int32
	bulletFont           string
	bulletAutoNumType    AutoNumberScheme
	bulletAutoNumStartAt int32
	// marL / indent (a:pPr@marL/@indent) in EMU; nil means unset. tabStops maps
	// a:tabLst.
	marL     *int32
	indent   *int32
	tabStops []TabStop
	dirty    bool
}

// AutoNumberScheme names an automatic-numbering bullet scheme (a:buAutoNum@type,
// ST_TextAutonumberScheme). The string values are the DrawingML tokens.
type AutoNumberScheme string

const (
	AutoNumberArabicPeriod  AutoNumberScheme = "arabicPeriod"
	AutoNumberArabicParenR  AutoNumberScheme = "arabicParenR"
	AutoNumberAlphaLcPeriod AutoNumberScheme = "alphaLcPeriod"
	AutoNumberAlphaUcPeriod AutoNumberScheme = "alphaUcPeriod"
	AutoNumberRomanLcPeriod AutoNumberScheme = "romanLcPeriod"
	AutoNumberRomanUcPeriod AutoNumberScheme = "romanUcPeriod"
)

// TabAlign names the alignment of a paragraph tab stop (a:tab@algn,
// ST_TextTabAlignType).
type TabAlign string

const (
	TabAlignLeft    TabAlign = "l"
	TabAlignCenter  TabAlign = "ctr"
	TabAlignRight   TabAlign = "r"
	TabAlignDecimal TabAlign = "dec"
)

// TabStop describes a single paragraph tab stop (a:tab).
type TabStop struct {
	// Position is the tab stop position in EMU from the text region's left edge.
	Position dml.EMU
	// Align is the tab stop alignment. The empty value is treated as left.
	Align TabAlign
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

// NewParagraph creates a new paragraph. Alignment and line spacing default to
// their unset values so the paragraph inherits them from its
// placeholder/layout/master instead of clobbering them with an explicit
// algn="l" / 100% (a plain SetText on a centered title must not left-align it);
// set them explicitly with SetAlignment / SetLineSpacing when needed.
func NewParagraph() *Paragraph {
	return &Paragraph{
		runs: make([]*Run, 0),
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
	p.dirty = true
	return r
}

// isDirty reports whether the paragraph or any of its runs changed since the
// last sync.
func (p *Paragraph) isDirty() bool {
	if p.dirty {
		return true
	}
	for _, r := range p.runs {
		if r.dirty {
			return true
		}
	}
	return false
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
	p.dirty = true
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
	p.dirty = true
}

// LineSpacing returns the line spacing in hundredths of a percent.
// 0 means unset: the paragraph inherits the placeholder/layout/master spacing.
func (p *Paragraph) LineSpacing() int32 {
	return p.lineSpacing
}

// SetLineSpacing sets the line spacing in hundredths of a percent.
// 100000 = 100% (single spacing), 200000 = 200% (double spacing).
// 0 restores the default: inherit from the placeholder/layout/master.
func (p *Paragraph) SetLineSpacing(spacing int32) {
	p.lineSpacing = spacing
	p.dirty = true
}

// SpaceBefore returns the space before the paragraph.
func (p *Paragraph) SpaceBefore() dml.EMU {
	return p.spaceBefore
}

// SetSpaceBefore sets the space before the paragraph.
func (p *Paragraph) SetSpaceBefore(space dml.EMU) {
	p.spaceBefore = space
	p.dirty = true
}

// SpaceAfter returns the space after the paragraph.
func (p *Paragraph) SpaceAfter() dml.EMU {
	return p.spaceAfter
}

// SetSpaceAfter sets the space after the paragraph.
func (p *Paragraph) SetSpaceAfter(space dml.EMU) {
	p.spaceAfter = space
	p.dirty = true
}

// Bullet returns the bullet type.
func (p *Paragraph) Bullet() BulletType {
	return p.bulletType
}

// SetBullet sets the bullet type.
func (p *Paragraph) SetBullet(bulletType BulletType) {
	p.bulletType = bulletType
	p.dirty = true
}

// BulletChar returns the bullet character.
func (p *Paragraph) BulletChar() string {
	return p.bulletChar
}

// SetBulletChar sets the bullet character.
func (p *Paragraph) SetBulletChar(char string) {
	p.bulletChar = char
	p.bulletType = BulletChar
	p.dirty = true
}

// SetBulletAutoNumber sets the paragraph to an automatically numbered bullet
// using the given scheme, starting at startAt (1-based; pass 0 to omit the
// start attribute and begin at the scheme default). An empty scheme defaults to
// AutoNumberArabicPeriod.
func (p *Paragraph) SetBulletAutoNumber(scheme AutoNumberScheme, startAt int32) {
	if scheme == "" {
		scheme = AutoNumberArabicPeriod
	}
	p.bulletType = BulletNumber
	p.bulletAutoNumType = scheme
	p.bulletAutoNumStartAt = startAt
	p.dirty = true
}

// BulletAutoNumberScheme returns the auto-numbering scheme, or an empty string
// when the paragraph is not auto-numbered.
func (p *Paragraph) BulletAutoNumberScheme() AutoNumberScheme {
	return p.bulletAutoNumType
}

// BulletAutoNumberStart returns the auto-numbering start value, or 0 when unset.
func (p *Paragraph) BulletAutoNumberStart() int32 {
	return p.bulletAutoNumStartAt
}

// BulletColor returns the bullet color (a:buClr), or nil when unset.
func (p *Paragraph) BulletColor() *dml.Color {
	return p.bulletColor
}

// SetBulletColor sets the bullet color (a:buClr), independent of the run text
// color.
func (p *Paragraph) SetBulletColor(color dml.Color) {
	p.bulletColor = &color
	p.dirty = true
}

// BulletSizePercent returns the bullet size as a percentage of the text size in
// the dml.Percentage unit scale (100000 = 100%), or 0 when unset.
func (p *Paragraph) BulletSizePercent() int32 {
	return p.bulletSizePct
}

// SetBulletSizePercent sets the bullet size as a percentage of the text
// (a:buSzPct), in the dml.Percentage unit scale (100000 = 100%).
func (p *Paragraph) SetBulletSizePercent(pct int32) {
	p.bulletSizePct = pct
	p.dirty = true
}

// BulletFont returns the bullet font typeface (a:buFont), or an empty string
// when unset.
func (p *Paragraph) BulletFont() string {
	return p.bulletFont
}

// SetBulletFont sets the bullet font typeface (a:buFont), used by character and
// numbered bullets.
func (p *Paragraph) SetBulletFont(typeface string) {
	p.bulletFont = typeface
	p.dirty = true
}

// MarginLeft returns the paragraph left margin in EMU (a:pPr@marL), or 0 when
// unset.
func (p *Paragraph) MarginLeft() dml.EMU {
	if p.marL == nil {
		return 0
	}
	return dml.EMU(*p.marL)
}

// SetMarginLeft sets the paragraph left margin in EMU (a:pPr@marL).
func (p *Paragraph) SetMarginLeft(marL dml.EMU) {
	v := int32(marL)
	p.marL = &v
	p.dirty = true
}

// Indent returns the first-line indent in EMU (a:pPr@indent), or 0 when unset.
// A negative indent produces a hanging indent.
func (p *Paragraph) Indent() dml.EMU {
	if p.indent == nil {
		return 0
	}
	return dml.EMU(*p.indent)
}

// SetIndent sets the first-line indent in EMU (a:pPr@indent). Negative values
// produce a hanging indent.
func (p *Paragraph) SetIndent(indent dml.EMU) {
	v := int32(indent)
	p.indent = &v
	p.dirty = true
}

// TabStops returns the paragraph's explicit tab stops (a:tabLst), in order.
func (p *Paragraph) TabStops() []TabStop {
	return p.tabStops
}

// SetTabStops replaces the paragraph's explicit tab stops (a:tabLst).
func (p *Paragraph) SetTabStops(stops []TabStop) {
	p.tabStops = stops
	p.dirty = true
}

// AddTabStop appends a single tab stop to the paragraph (a:tabLst).
func (p *Paragraph) AddTabStop(stop TabStop) {
	p.tabStops = append(p.tabStops, stop)
	p.dirty = true
}

// Run represents a run of text with consistent formatting.
type Run struct {
	text      string
	fontName  string
	fontSize  float64 // in points
	bold      bool
	italic    bool
	underline enum.UnderlineStyle
	strike    enum.StrikeStyle
	color     *dml.Color
	highlight *dml.Color
	baseline  int32 // percentage, positive for superscript, negative for subscript
	hyperlink *Hyperlink
	dirty     bool
}

// NewRun creates a new run. Font size, underline, and strike default to their
// zero values (unset) so a run added to a placeholder inherits the
// placeholder/layout formatting instead of being clobbered with explicit
// attributes; set them explicitly with SetFontSize/SetUnderline/SetStrike when
// needed (SetUnderline(enum.UnderlineNone) and SetStrike(enum.StrikeNone)
// explicitly suppress inherited underline/strike).
func NewRun() *Run {
	return &Run{}
}

// Text returns the text content.
func (r *Run) Text() string {
	return r.text
}

// SetText sets the text content.
func (r *Run) SetText(text string) {
	r.text = text
	r.dirty = true
}

// Font returns the font name.
func (r *Run) Font() string {
	return r.fontName
}

// SetFont sets the font name.
func (r *Run) SetFont(name string) {
	r.fontName = name
	r.dirty = true
}

// FontSize returns the font size in points.
func (r *Run) FontSize() float64 {
	return r.fontSize
}

// SetFontSize sets the font size in points.
func (r *Run) SetFontSize(size float64) {
	r.fontSize = size
	r.dirty = true
}

// Bold returns whether the run is bold.
func (r *Run) Bold() bool {
	return r.bold
}

// SetBold sets whether the run is bold.
func (r *Run) SetBold(bold bool) {
	r.bold = bold
	r.dirty = true
}

// Italic returns whether the run is italic.
func (r *Run) Italic() bool {
	return r.italic
}

// SetItalic sets whether the run is italic.
func (r *Run) SetItalic(italic bool) {
	r.italic = italic
	r.dirty = true
}

// Underline returns the underline style. The empty string means unset: the
// run inherits the underline of its placeholder/layout/master.
func (r *Run) Underline() enum.UnderlineStyle {
	return r.underline
}

// SetUnderline sets the underline style. Use enum.UnderlineNone to explicitly
// suppress an inherited underline; the empty string restores inheritance.
func (r *Run) SetUnderline(style enum.UnderlineStyle) {
	r.underline = style
	r.dirty = true
}

// Strike returns the strikethrough style. The empty string means unset: the
// run inherits the strike of its placeholder/layout/master.
func (r *Run) Strike() enum.StrikeStyle {
	return r.strike
}

// SetStrike sets the strikethrough style. Use enum.StrikeNone to explicitly
// suppress an inherited strike; the empty string restores inheritance.
func (r *Run) SetStrike(style enum.StrikeStyle) {
	r.strike = style
	r.dirty = true
}

// Color returns the text color.
func (r *Run) Color() *dml.Color {
	return r.color
}

// SetColor sets the text color.
func (r *Run) SetColor(color dml.Color) {
	r.color = &color
	r.dirty = true
}

// Highlight returns the highlight color.
func (r *Run) Highlight() *dml.Color {
	return r.highlight
}

// SetHighlight sets the highlight color.
func (r *Run) SetHighlight(color dml.Color) {
	r.highlight = &color
	r.dirty = true
}

// Baseline returns the baseline offset percentage.
// Positive values create superscript, negative values create subscript.
func (r *Run) Baseline() int32 {
	return r.baseline
}

// SetBaseline sets the baseline offset percentage.
func (r *Run) SetBaseline(baseline int32) {
	r.baseline = baseline
	r.dirty = true
}

// SetSuperscript configures the run as superscript.
func (r *Run) SetSuperscript() {
	r.baseline = 30000 // 30%
	r.dirty = true
}

// SetSubscript configures the run as subscript.
func (r *Run) SetSubscript() {
	r.baseline = -30000 // -30%
	r.dirty = true
}

// Hyperlink returns the run's hyperlink (an a:hlinkClick on the run properties),
// or nil when the run carries none.
func (r *Run) Hyperlink() *Hyperlink {
	return r.hyperlink
}

// SetHyperlink attaches an external-URL hyperlink to the run and returns it. The
// External relationship is allocated in the slide's rels on save.
func (r *Run) SetHyperlink(url string) *Hyperlink {
	r.hyperlink = newExternalHyperlink(url, func() { r.dirty = true })
	r.dirty = true
	return r.hyperlink
}

// SetActionHyperlink attaches a slide-show action hyperlink (e.g. ActionNextSlide)
// to the run and returns it. Action verbs need no relationship.
func (r *Run) SetActionHyperlink(action string) *Hyperlink {
	r.hyperlink = newActionHyperlink(action, func() { r.dirty = true })
	r.dirty = true
	return r.hyperlink
}

// SetHyperlinkToSlide attaches an internal jump to the slide at the given 0-based
// index and returns it. The RelTypeSlide relationship is allocated on save.
func (r *Run) SetHyperlinkToSlide(index int) *Hyperlink {
	r.hyperlink = newSlideJumpHyperlink(index, func() { r.dirty = true })
	r.dirty = true
	return r.hyperlink
}
