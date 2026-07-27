package dml

import (
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// ThemeEditor is a read/write handle over a parsed DrawingML theme part
// (a:theme). It is the shared model behind docx.Document.Theme and
// xlsx.Workbook.Theme (the same a:theme lives at word/theme/theme1.xml and
// xl/theme/theme1.xml), mirroring the accessor shape of pptx's read-only Theme
// but persisting edits: a modified editor re-serializes the theme part on
// save, while an untouched one leaves the source bytes in place for a
// byte-identical round-trip.
type ThemeEditor struct {
	theme    *Theme
	raw      []byte // original part bytes, for prolog capture on re-marshal
	modified bool
}

// NewThemeEditor wraps a parsed theme together with its original part bytes.
// It returns nil when theme is nil, so callers can surface "no theme part" as
// a nil handle (matching pptx's Theme accessors).
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
// sysClr via its lastClr rendering. An empty or unresolved slot yields the
// zero Color.
func themeSlotColor(cc *ColorChoice) Color {
	if cc == nil {
		return Color{}
	}
	if cc.SrgbClr != nil {
		if rgb, err := ParseRGB(cc.SrgbClr.Val); err == nil {
			return rgb.ToColor()
		}
	}
	if cc.SysClr != nil && cc.SysClr.LastClr != "" {
		if rgb, err := ParseRGB(cc.SysClr.LastClr); err == nil {
			return rgb.ToColor()
		}
	}
	return Color{}
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

// fontLatin returns the Latin typeface of a font collection, or "".
func fontLatin(fc *FontCollection) string {
	if fc == nil || fc.Latin == nil {
		return ""
	}
	return fc.Latin.Typeface
}

// setFontLatin sets the Latin typeface of a font collection, allocating the
// collection and its latin slot as needed.
func setFontLatin(fc **FontCollection, typeface string) {
	if *fc == nil {
		*fc = &FontCollection{}
	}
	if (*fc).Latin == nil {
		(*fc).Latin = &TextFont{}
	}
	(*fc).Latin.Typeface = typeface
}
