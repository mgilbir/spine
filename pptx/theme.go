package pptx

import (
	"github.com/mgilbir/spine/common/dml"
)

// Theme represents a presentation theme.
type Theme struct {
	name         string
	colorScheme  *ColorScheme
	fontScheme   *FontScheme
	formatScheme *FormatScheme
}

// Name returns the theme name.
func (t *Theme) Name() string {
	return t.name
}

// SetName sets the theme name.
func (t *Theme) SetName(name string) {
	t.name = name
}

// ColorScheme returns the color scheme.
func (t *Theme) ColorScheme() *ColorScheme {
	return t.colorScheme
}

// FontScheme returns the font scheme.
func (t *Theme) FontScheme() *FontScheme {
	return t.fontScheme
}

// FormatScheme returns the format scheme.
func (t *Theme) FormatScheme() *FormatScheme {
	return t.formatScheme
}

// ColorScheme defines the colors for a theme.
type ColorScheme struct {
	name     string
	dark1    dml.Color
	light1   dml.Color
	dark2    dml.Color
	light2   dml.Color
	accent1  dml.Color
	accent2  dml.Color
	accent3  dml.Color
	accent4  dml.Color
	accent5  dml.Color
	accent6  dml.Color
	hyperlink dml.Color
	followedHyperlink dml.Color
}

// Name returns the color scheme name.
func (cs *ColorScheme) Name() string {
	return cs.name
}

// Dark1 returns the first dark color (typically black).
func (cs *ColorScheme) Dark1() dml.Color {
	return cs.dark1
}

// SetDark1 sets the first dark color.
func (cs *ColorScheme) SetDark1(c dml.Color) {
	cs.dark1 = c
}

// Light1 returns the first light color (typically white).
func (cs *ColorScheme) Light1() dml.Color {
	return cs.light1
}

// SetLight1 sets the first light color.
func (cs *ColorScheme) SetLight1(c dml.Color) {
	cs.light1 = c
}

// Dark2 returns the second dark color.
func (cs *ColorScheme) Dark2() dml.Color {
	return cs.dark2
}

// SetDark2 sets the second dark color.
func (cs *ColorScheme) SetDark2(c dml.Color) {
	cs.dark2 = c
}

// Light2 returns the second light color.
func (cs *ColorScheme) Light2() dml.Color {
	return cs.light2
}

// SetLight2 sets the second light color.
func (cs *ColorScheme) SetLight2(c dml.Color) {
	cs.light2 = c
}

// Accent1 returns the first accent color.
func (cs *ColorScheme) Accent1() dml.Color {
	return cs.accent1
}

// SetAccent1 sets the first accent color.
func (cs *ColorScheme) SetAccent1(c dml.Color) {
	cs.accent1 = c
}

// Accent2 returns the second accent color.
func (cs *ColorScheme) Accent2() dml.Color {
	return cs.accent2
}

// SetAccent2 sets the second accent color.
func (cs *ColorScheme) SetAccent2(c dml.Color) {
	cs.accent2 = c
}

// Accent3 returns the third accent color.
func (cs *ColorScheme) Accent3() dml.Color {
	return cs.accent3
}

// SetAccent3 sets the third accent color.
func (cs *ColorScheme) SetAccent3(c dml.Color) {
	cs.accent3 = c
}

// Accent4 returns the fourth accent color.
func (cs *ColorScheme) Accent4() dml.Color {
	return cs.accent4
}

// SetAccent4 sets the fourth accent color.
func (cs *ColorScheme) SetAccent4(c dml.Color) {
	cs.accent4 = c
}

// Accent5 returns the fifth accent color.
func (cs *ColorScheme) Accent5() dml.Color {
	return cs.accent5
}

// SetAccent5 sets the fifth accent color.
func (cs *ColorScheme) SetAccent5(c dml.Color) {
	cs.accent5 = c
}

// Accent6 returns the sixth accent color.
func (cs *ColorScheme) Accent6() dml.Color {
	return cs.accent6
}

// SetAccent6 sets the sixth accent color.
func (cs *ColorScheme) SetAccent6(c dml.Color) {
	cs.accent6 = c
}

// Hyperlink returns the hyperlink color.
func (cs *ColorScheme) Hyperlink() dml.Color {
	return cs.hyperlink
}

// SetHyperlink sets the hyperlink color.
func (cs *ColorScheme) SetHyperlink(c dml.Color) {
	cs.hyperlink = c
}

// FollowedHyperlink returns the followed hyperlink color.
func (cs *ColorScheme) FollowedHyperlink() dml.Color {
	return cs.followedHyperlink
}

// SetFollowedHyperlink sets the followed hyperlink color.
func (cs *ColorScheme) SetFollowedHyperlink(c dml.Color) {
	cs.followedHyperlink = c
}

// FontScheme defines the fonts for a theme.
type FontScheme struct {
	name       string
	majorLatin string
	majorEastAsia string
	majorComplexScript string
	minorLatin string
	minorEastAsia string
	minorComplexScript string
}

// Name returns the font scheme name.
func (fs *FontScheme) Name() string {
	return fs.name
}

// MajorLatin returns the major (heading) Latin font.
func (fs *FontScheme) MajorLatin() string {
	return fs.majorLatin
}

// SetMajorLatin sets the major Latin font.
func (fs *FontScheme) SetMajorLatin(font string) {
	fs.majorLatin = font
}

// MinorLatin returns the minor (body) Latin font.
func (fs *FontScheme) MinorLatin() string {
	return fs.minorLatin
}

// SetMinorLatin sets the minor Latin font.
func (fs *FontScheme) SetMinorLatin(font string) {
	fs.minorLatin = font
}

// MajorEastAsia returns the major East Asian font.
func (fs *FontScheme) MajorEastAsia() string {
	return fs.majorEastAsia
}

// SetMajorEastAsia sets the major East Asian font.
func (fs *FontScheme) SetMajorEastAsia(font string) {
	fs.majorEastAsia = font
}

// MinorEastAsia returns the minor East Asian font.
func (fs *FontScheme) MinorEastAsia() string {
	return fs.minorEastAsia
}

// SetMinorEastAsia sets the minor East Asian font.
func (fs *FontScheme) SetMinorEastAsia(font string) {
	fs.minorEastAsia = font
}

// MajorComplexScript returns the major complex script font.
func (fs *FontScheme) MajorComplexScript() string {
	return fs.majorComplexScript
}

// SetMajorComplexScript sets the major complex script font.
func (fs *FontScheme) SetMajorComplexScript(font string) {
	fs.majorComplexScript = font
}

// MinorComplexScript returns the minor complex script font.
func (fs *FontScheme) MinorComplexScript() string {
	return fs.minorComplexScript
}

// SetMinorComplexScript sets the minor complex script font.
func (fs *FontScheme) SetMinorComplexScript(font string) {
	fs.minorComplexScript = font
}

// FormatScheme defines fills, lines, and effects for a theme.
type FormatScheme struct {
	name        string
	fillStyles  []FillStyle
	lineStyles  []LineStyle
	effectStyles []EffectStyle
	bgFillStyles []FillStyle
}

// Name returns the format scheme name.
func (fs *FormatScheme) Name() string {
	return fs.name
}

// FillStyles returns the fill styles.
func (fs *FormatScheme) FillStyles() []FillStyle {
	return fs.fillStyles
}

// LineStyles returns the line styles.
func (fs *FormatScheme) LineStyles() []LineStyle {
	return fs.lineStyles
}

// EffectStyles returns the effect styles.
func (fs *FormatScheme) EffectStyles() []EffectStyle {
	return fs.effectStyles
}

// BackgroundFillStyles returns the background fill styles.
func (fs *FormatScheme) BackgroundFillStyles() []FillStyle {
	return fs.bgFillStyles
}

// FillStyle represents a fill style.
type FillStyle struct {
	Type  FillType
	Color *dml.Color
}

// FillType specifies the type of fill.
type FillType int

const (
	FillTypeNone FillType = iota
	FillTypeSolid
	FillTypeGradient
	FillTypePattern
	FillTypePicture
)

// LineStyle represents a line style.
type LineStyle struct {
	Width dml.EMU
	Color *dml.Color
	Dash  string
}

// EffectStyle represents an effect style.
type EffectStyle struct {
	// Placeholder for effect properties
}
