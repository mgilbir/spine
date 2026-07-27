package dml

import (
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// ThemeEditor is a read/write handle over a parsed DrawingML theme part
// (a:theme). It is the single theme model behind docx.Document.Theme,
// xlsx.Workbook.Theme and pptx's Presentation.Theme / SlideMaster.Theme — the
// same a:theme lives at word/theme/theme1.xml, xl/theme/theme1.xml and
// ppt/theme/themeN.xml, so one type serves all three (C571). A modified editor
// re-serializes the theme part on save; an untouched one leaves the source
// bytes in place for a byte-identical round-trip.
type ThemeEditor struct {
	theme    *Theme
	raw      []byte // original part bytes, for prolog capture on re-marshal
	modified bool
}

// NewThemeEditor wraps a parsed theme together with its original part bytes.
// It returns nil when theme is nil, so callers can surface "no theme part" as
// a nil handle.
func NewThemeEditor(theme *Theme, raw []byte) *ThemeEditor {
	if theme == nil {
		return nil
	}
	return &ThemeEditor{theme: theme, raw: raw}
}

// Modified reports whether any setter changed the theme since it was loaded.
// A save only re-serializes the theme part when this is true.
func (e *ThemeEditor) Modified() bool { return e.modified }

// Name returns the theme name (the a:theme name attribute).
func (e *ThemeEditor) Name() string { return e.theme.Name }

// SetName sets the theme name.
func (e *ThemeEditor) SetName(name string) {
	if e.theme.Name != name {
		e.theme.Name = name
		e.modified = true
	}
}

// ColorScheme returns the color-scheme editor, or nil when the theme carries
// no clrScheme.
func (e *ThemeEditor) ColorScheme() *ThemeColorScheme {
	if e.theme.ThemeElements == nil || e.theme.ThemeElements.ClrScheme == nil {
		return nil
	}
	return &ThemeColorScheme{owner: e, cs: e.theme.ThemeElements.ClrScheme}
}

// FontScheme returns the font-scheme editor, or nil when the theme carries no
// fontScheme.
func (e *ThemeEditor) FontScheme() *ThemeFontScheme {
	if e.theme.ThemeElements == nil || e.theme.ThemeElements.FontScheme == nil {
		return nil
	}
	return &ThemeFontScheme{owner: e, fs: e.theme.ThemeElements.FontScheme}
}

// FormatScheme returns a read-only view of the theme's format scheme
// (a:fmtScheme), or nil when the theme carries none.
//
// It is read-only on purpose: a:fmtScheme's fill, line and effect style lists
// are positional (a:fillRef/@idx and friends index them), so an editing API
// that let a caller reorder or drop one would silently repoint every styled
// shape in the file — the defect C401 records. Read the ClrScheme/FmtScheme
// model directly for anything this view does not surface.
func (e *ThemeEditor) FormatScheme() *ThemeFormatScheme {
	if e.theme.ThemeElements == nil || e.theme.ThemeElements.FmtScheme == nil {
		return nil
	}
	return &ThemeFormatScheme{fs: e.theme.ThemeElements.FmtScheme}
}

// Marshal re-serializes the theme part, reproducing the source XML declaration
// and surrounding whitespace so a modified save differs only in the edited
// content. Callers use it only when Modified reports true; an unmodified theme
// round-trips from its preserved source bytes instead.
func (e *ThemeEditor) Marshal() ([]byte, error) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(xmlb.NSDrawingML, xmlb.PrefixDrawingML)
	b.SetCollapseEmptyElements(true)
	prolog := xmlb.CaptureProlog(e.raw)
	b.WriteProlog(prolog)
	b.SetRootEndTag(prolog.RootEnd)
	if e.theme.CapturedAttrs != nil {
		// Replay the source root attribute list verbatim (its declaration
		// order, any mc:Ignorable, any extension prefix declared up here for a
		// nested a:ext), with the modeled name winning when SetName changed it.
		// The fixed decl set below would both drop those and re-declare
		// xmlns:a a second time.
		var modeled []xmlb.Attr
		if e.theme.Name != "" {
			modeled = append(modeled, xmlb.StrAttr("name", e.theme.Name))
		}
		b.StartElementWithRootAttrsMerged(xmlb.NSDrawingML, "theme", e.theme.CapturedAttrs, modeled)
		b.MarshalChildren(xmlb.NSDrawingML, e.theme)
		b.EndElement(xmlb.NSDrawingML, "theme")
	} else {
		// Programmatically built theme: canonical declaration.
		b.MarshalRoot(xmlb.NSDrawingML, "theme", e.theme,
			[]xmlb.NSDecl{{Prefix: xmlb.PrefixDrawingML, URI: xmlb.NSDrawingML}})
	}
	b.WriteTrailer(prolog)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("dml: marshal theme: %w", err)
	}
	return b.Bytes(), nil
}

// ThemeColorScheme is a read/write view of a theme's color scheme
// (a:clrScheme). Getters resolve a slot to a concrete color (srgbClr directly,
// sysClr via its lastClr rendering); setters replace the slot with a plain
// srgbClr, since a theme slot defines a color rather than referencing one.
type ThemeColorScheme struct {
	owner *ThemeEditor
	cs    *ClrScheme
}

// Name returns the color scheme name.
func (c *ThemeColorScheme) Name() string { return c.cs.Name }

// SetName sets the color scheme name.
func (c *ThemeColorScheme) SetName(name string) {
	if c.cs.Name != name {
		c.cs.Name = name
		c.owner.modified = true
	}
}

// Dark1 returns the first dark color (dk1, typically black).
func (c *ThemeColorScheme) Dark1() Color { return themeSlotColor(c.cs.Dk1) }

// SetDark1 sets the first dark color (dk1).
func (c *ThemeColorScheme) SetDark1(col Color) {
	c.cs.Dk1 = themeSlotChoice(col)
	c.owner.modified = true
}

// Light1 returns the first light color (lt1, typically white).
func (c *ThemeColorScheme) Light1() Color { return themeSlotColor(c.cs.Lt1) }

// SetLight1 sets the first light color (lt1).
func (c *ThemeColorScheme) SetLight1(col Color) {
	c.cs.Lt1 = themeSlotChoice(col)
	c.owner.modified = true
}

// Dark2 returns the second dark color (dk2).
func (c *ThemeColorScheme) Dark2() Color { return themeSlotColor(c.cs.Dk2) }

// SetDark2 sets the second dark color (dk2).
func (c *ThemeColorScheme) SetDark2(col Color) {
	c.cs.Dk2 = themeSlotChoice(col)
	c.owner.modified = true
}

// Light2 returns the second light color (lt2).
func (c *ThemeColorScheme) Light2() Color { return themeSlotColor(c.cs.Lt2) }

// SetLight2 sets the second light color (lt2).
func (c *ThemeColorScheme) SetLight2(col Color) {
	c.cs.Lt2 = themeSlotChoice(col)
	c.owner.modified = true
}

// Accent1 returns the first accent color.
func (c *ThemeColorScheme) Accent1() Color { return themeSlotColor(c.cs.Accent1) }

// SetAccent1 sets the first accent color.
func (c *ThemeColorScheme) SetAccent1(col Color) {
	c.cs.Accent1 = themeSlotChoice(col)
	c.owner.modified = true
}

// Accent2 returns the second accent color.
func (c *ThemeColorScheme) Accent2() Color { return themeSlotColor(c.cs.Accent2) }

// SetAccent2 sets the second accent color.
func (c *ThemeColorScheme) SetAccent2(col Color) {
	c.cs.Accent2 = themeSlotChoice(col)
	c.owner.modified = true
}

// Accent3 returns the third accent color.
func (c *ThemeColorScheme) Accent3() Color { return themeSlotColor(c.cs.Accent3) }

// SetAccent3 sets the third accent color.
func (c *ThemeColorScheme) SetAccent3(col Color) {
	c.cs.Accent3 = themeSlotChoice(col)
	c.owner.modified = true
}

// Accent4 returns the fourth accent color.
func (c *ThemeColorScheme) Accent4() Color { return themeSlotColor(c.cs.Accent4) }

// SetAccent4 sets the fourth accent color.
func (c *ThemeColorScheme) SetAccent4(col Color) {
	c.cs.Accent4 = themeSlotChoice(col)
	c.owner.modified = true
}

// Accent5 returns the fifth accent color.
func (c *ThemeColorScheme) Accent5() Color { return themeSlotColor(c.cs.Accent5) }

// SetAccent5 sets the fifth accent color.
func (c *ThemeColorScheme) SetAccent5(col Color) {
	c.cs.Accent5 = themeSlotChoice(col)
	c.owner.modified = true
}

// Accent6 returns the sixth accent color.
func (c *ThemeColorScheme) Accent6() Color { return themeSlotColor(c.cs.Accent6) }

// SetAccent6 sets the sixth accent color.
func (c *ThemeColorScheme) SetAccent6(col Color) {
	c.cs.Accent6 = themeSlotChoice(col)
	c.owner.modified = true
}

// Hyperlink returns the hyperlink color (hlink).
func (c *ThemeColorScheme) Hyperlink() Color { return themeSlotColor(c.cs.Hlink) }

// SetHyperlink sets the hyperlink color (hlink).
func (c *ThemeColorScheme) SetHyperlink(col Color) {
	c.cs.Hlink = themeSlotChoice(col)
	c.owner.modified = true
}

// FollowedHyperlink returns the followed-hyperlink color (folHlink).
func (c *ThemeColorScheme) FollowedHyperlink() Color { return themeSlotColor(c.cs.FolHlink) }

// SetFollowedHyperlink sets the followed-hyperlink color (folHlink).
func (c *ThemeColorScheme) SetFollowedHyperlink(col Color) {
	c.cs.FolHlink = themeSlotChoice(col)
	c.owner.modified = true
}

// themeSlotColor resolves a color slot to a concrete color: srgbClr directly,
// sysClr via its lastClr rendering, scrgbClr by converting its r/g/b
// percentages, and schemeClr as the theme reference it is.
//
// The remaining kinds (hslClr, prstClr) and a slot whose value does not parse
// yield the zero Color, which is indistinguishable from opaque black — callers
// that must tell "black" from "unresolvable" should read the slot off
// ClrScheme directly. Previously *every* kind but srgbClr/sysClr collapsed that
// way, so a scheme whose dk1 was an scrgbClr or an alias reported black.
func themeSlotColor(cc *ColorChoice) Color {
	if cc == nil {
		return Color{}
	}
	switch {
	case cc.SrgbClr != nil:
		if rgb, err := ParseRGB(cc.SrgbClr.Val); err == nil {
			return rgb.ToColor()
		}
	case cc.SysClr != nil:
		if cc.SysClr.LastClr != "" {
			if rgb, err := ParseRGB(cc.SysClr.LastClr); err == nil {
				return rgb.ToColor()
			}
		}
	case cc.ScrgbClr != nil:
		return NewRGB(
			pctToByte(cc.ScrgbClr.R), pctToByte(cc.ScrgbClr.G), pctToByte(cc.ScrgbClr.B),
		).ToColor()
	case cc.SchemeClr != nil:
		if tc, ok := parseThemeColorName(cc.SchemeClr.Val); ok {
			return tc.ToColor()
		}
	}
	return Color{}
}

// pctToByte converts an ST_Percentage channel value (0..100000) to an 8-bit
// channel, clamping out-of-range input.
func pctToByte(p Percentage) uint8 {
	v := p.Int32()
	if v <= 0 {
		return 0
	}
	if v >= 100000 {
		return 255
	}
	return uint8((int64(v)*255 + 50000) / 100000)
}

// parseThemeColorName maps an ST_SchemeColorVal to its ThemeColor, reporting
// whether the name is one the model can represent (phClr, bg1/tx1/bg2/tx2 and
// anything unknown are not).
func parseThemeColorName(name string) (ThemeColor, bool) {
	switch name {
	case "dk1":
		return ThemeColorDark1, true
	case "lt1":
		return ThemeColorLight1, true
	case "dk2":
		return ThemeColorDark2, true
	case "lt2":
		return ThemeColorLight2, true
	case "accent1":
		return ThemeColorAccent1, true
	case "accent2":
		return ThemeColorAccent2, true
	case "accent3":
		return ThemeColorAccent3, true
	case "accent4":
		return ThemeColorAccent4, true
	case "accent5":
		return ThemeColorAccent5, true
	case "accent6":
		return ThemeColorAccent6, true
	case "hlink":
		return ThemeColorHyperlink, true
	case "folHlink":
		return ThemeColorFollowedHyperlink, true
	}
	return 0, false
}

// themeSlotChoice builds a color slot from a concrete color as a plain
// srgbClr.
func themeSlotChoice(col Color) *ColorChoice {
	return &ColorChoice{SrgbClr: colorToSrgbClr(col)}
}

// ThemeFontScheme is a read/write view of a theme's font scheme
// (a:fontScheme), exposing the major (heading) and minor (body) Latin
// typefaces.
type ThemeFontScheme struct {
	owner *ThemeEditor
	fs    *FontScheme
}

// Name returns the font scheme name.
func (f *ThemeFontScheme) Name() string { return f.fs.Name }

// SetName sets the font scheme name.
func (f *ThemeFontScheme) SetName(name string) {
	if f.fs.Name != name {
		f.fs.Name = name
		f.owner.modified = true
	}
}

// MajorLatin returns the major (heading) Latin typeface.
func (f *ThemeFontScheme) MajorLatin() string { return fontLatin(f.fs.MajorFont) }

// SetMajorLatin sets the major (heading) Latin typeface.
func (f *ThemeFontScheme) SetMajorLatin(typeface string) {
	setFontLatin(&f.fs.MajorFont, typeface)
	f.owner.modified = true
}

// MinorLatin returns the minor (body) Latin typeface.
func (f *ThemeFontScheme) MinorLatin() string { return fontLatin(f.fs.MinorFont) }

// SetMinorLatin sets the minor (body) Latin typeface.
func (f *ThemeFontScheme) SetMinorLatin(typeface string) {
	setFontLatin(&f.fs.MinorFont, typeface)
	f.owner.modified = true
}

// The East Asian and complex-script slots. A theme names three typefaces per
// font collection (a:latin, a:ea, a:cs), and pptx's read-only theme surfaced
// all three while this editor surfaced only the Latin one; converging the two
// (C571) meant the editor had to carry the full set rather than the merge
// dropping a capability.

// MajorEastAsia returns the major (heading) East Asian typeface.
func (f *ThemeFontScheme) MajorEastAsia() string { return fontSlot(f.fs.MajorFont, slotEA) }

// SetMajorEastAsia sets the major (heading) East Asian typeface.
func (f *ThemeFontScheme) SetMajorEastAsia(typeface string) {
	setFontSlot(&f.fs.MajorFont, slotEA, typeface)
	f.owner.modified = true
}

// MinorEastAsia returns the minor (body) East Asian typeface.
func (f *ThemeFontScheme) MinorEastAsia() string { return fontSlot(f.fs.MinorFont, slotEA) }

// SetMinorEastAsia sets the minor (body) East Asian typeface.
func (f *ThemeFontScheme) SetMinorEastAsia(typeface string) {
	setFontSlot(&f.fs.MinorFont, slotEA, typeface)
	f.owner.modified = true
}

// MajorComplexScript returns the major (heading) complex-script typeface.
func (f *ThemeFontScheme) MajorComplexScript() string { return fontSlot(f.fs.MajorFont, slotCS) }

// SetMajorComplexScript sets the major (heading) complex-script typeface.
func (f *ThemeFontScheme) SetMajorComplexScript(typeface string) {
	setFontSlot(&f.fs.MajorFont, slotCS, typeface)
	f.owner.modified = true
}

// MinorComplexScript returns the minor (body) complex-script typeface.
func (f *ThemeFontScheme) MinorComplexScript() string { return fontSlot(f.fs.MinorFont, slotCS) }

// SetMinorComplexScript sets the minor (body) complex-script typeface.
func (f *ThemeFontScheme) SetMinorComplexScript(typeface string) {
	setFontSlot(&f.fs.MinorFont, slotCS, typeface)
	f.owner.modified = true
}

// fontSlotKind names one of a font collection's three typeface slots.
type fontSlotKind int

const (
	slotLatin fontSlotKind = iota
	slotEA
	slotCS
)

// fontSlotPtr returns the address of the requested slot within a (possibly nil)
// font collection, allocating nothing.
func fontSlotPtr(fc *FontCollection, kind fontSlotKind) **TextFont {
	if fc == nil {
		return nil
	}
	switch kind {
	case slotLatin:
		return &fc.Latin
	case slotEA:
		return &fc.Ea
	default:
		return &fc.Cs
	}
}

// fontSlot returns the typeface of one slot of a font collection, or "".
func fontSlot(fc *FontCollection, kind fontSlotKind) string {
	p := fontSlotPtr(fc, kind)
	if p == nil || *p == nil {
		return ""
	}
	return (*p).Typeface
}

// setFontSlot sets the typeface of one slot, allocating the collection and the
// slot as needed.
func setFontSlot(fc **FontCollection, kind fontSlotKind, typeface string) {
	if *fc == nil {
		*fc = &FontCollection{}
	}
	p := fontSlotPtr(*fc, kind)
	if *p == nil {
		*p = &TextFont{}
	}
	(*p).Typeface = typeface
}

// fontLatin returns the Latin typeface of a font collection, or "".
func fontLatin(fc *FontCollection) string { return fontSlot(fc, slotLatin) }

// setFontLatin sets the Latin typeface of a font collection, allocating the
// collection and its latin slot as needed.
func setFontLatin(fc **FontCollection, typeface string) {
	setFontSlot(fc, slotLatin, typeface)
}

// ThemeFormatScheme is a read-only view of a theme's format scheme
// (a:fmtScheme). See ThemeEditor.FormatScheme for why it has no setters.
type ThemeFormatScheme struct {
	fs *FmtScheme
}

// Name returns the format scheme name.
func (f *ThemeFormatScheme) Name() string { return f.fs.Name }

// ThemeLineStyle is one entry of a format scheme's a:lnStyleLst, in list order
// (a shape's a:lnRef/@idx is a 1-based index into it).
type ThemeLineStyle struct {
	// Width is the line width in EMU; zero when the entry declares none.
	Width EMU
	// Color is the line's solid fill resolved to a concrete color, or nil when
	// the entry has no solid fill (a gradient or pattern line).
	Color *Color
	// Dash is the preset dash name (a:prstDash/@val), or "" for a solid line.
	Dash string
}

// LineStyles returns the format scheme's line styles in list order. The fill
// and effect style lists are not surfaced: they are positional too, and
// flattening them to a value type would lose the gradient/pattern detail that
// makes them useful — read FmtScheme directly for those.
func (f *ThemeFormatScheme) LineStyles() []ThemeLineStyle {
	if f.fs.LnStyleLst == nil {
		return nil
	}
	out := make([]ThemeLineStyle, 0, len(f.fs.LnStyleLst.Ln))
	for _, ln := range f.fs.LnStyleLst.Ln {
		if ln == nil {
			continue
		}
		style := ThemeLineStyle{}
		if ln.W != nil {
			style.Width = EMU(*ln.W)
		}
		if ln.PrstDash != nil {
			style.Dash = ln.PrstDash.Val
		}
		if sf := ln.SolidFill; sf != nil {
			c := themeSlotColor(&ColorChoice{
				ScrgbClr:  sf.ScRgbClr,
				SrgbClr:   sf.SrgbClr,
				HslClr:    sf.HslClr,
				SysClr:    sf.SysClr,
				SchemeClr: sf.SchemeClr,
				PrstClr:   sf.PrstClr,
			})
			style.Color = &c
		}
		out = append(out, style)
	}
	return out
}
