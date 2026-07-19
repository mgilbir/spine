package pptx

import (
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// This file exposes two presentation-level lists that the model already
// round-trips but had no public API: embedded fonts (p:embeddedFontLst) and
// custom shows (p:custShowLst).

// EmbeddedFont describes one embedded font (p:embeddedFont). The four style
// fields hold the relationship id (r:id) of the font-data part for that style,
// or an empty string when the deck does not embed that style. The parts they
// reference are preserved as-is across a round trip.
type EmbeddedFont struct {
	Typeface   string
	Regular    string
	Bold       string
	Italic     string
	BoldItalic string
}

// EmbeddedFonts returns the fonts embedded in the presentation
// (p:embeddedFontLst), in document order. It returns nil when none are embedded.
func (p *Presentation) EmbeddedFonts() []EmbeddedFont {
	if p.presentation == nil || p.presentation.EmbeddedFontLst == nil {
		return nil
	}
	var fonts []EmbeddedFont
	for _, ef := range p.presentation.EmbeddedFontLst.EmbeddedFont {
		font := EmbeddedFont{}
		if ef.Font != nil {
			font.Typeface = ef.Font.Typeface
		}
		if ef.Regular != nil {
			font.Regular = ef.Regular.RID
		}
		if ef.Bold != nil {
			font.Bold = ef.Bold.RID
		}
		if ef.Italic != nil {
			font.Italic = ef.Italic.RID
		}
		if ef.BoldItalic != nil {
			font.BoldItalic = ef.BoldItalic.RID
		}
		fonts = append(fonts, font)
	}
	return fonts
}

// SetEmbeddedFonts replaces the presentation's embedded-font list. Each style's
// relationship id must reference a font-data part that exists in the package
// (typically one read back from EmbeddedFonts on a deck that already embeds
// fonts); an empty id omits that style. Passing an empty slice clears the list.
func (p *Presentation) SetEmbeddedFonts(fonts []EmbeddedFont) {
	if p.presentation == nil {
		p.presentation = &oxml.Presentation{}
	}
	if len(fonts) == 0 {
		p.presentation.EmbeddedFontLst = nil
		return
	}
	lst := &oxml.EmbeddedFontList{EmbeddedFont: make([]oxml.EmbeddedFont, 0, len(fonts))}
	for _, f := range fonts {
		ef := oxml.EmbeddedFont{
			Font: &oxml.TextFont{Typeface: f.Typeface},
		}
		if f.Regular != "" {
			ef.Regular = &oxml.EmbeddedFontData{RID: f.Regular}
		}
		if f.Bold != "" {
			ef.Bold = &oxml.EmbeddedFontData{RID: f.Bold}
		}
		if f.Italic != "" {
			ef.Italic = &oxml.EmbeddedFontData{RID: f.Italic}
		}
		if f.BoldItalic != "" {
			ef.BoldItalic = &oxml.EmbeddedFontData{RID: f.BoldItalic}
		}
		lst.EmbeddedFont = append(lst.EmbeddedFont, ef)
	}
	p.presentation.EmbeddedFontLst = lst
}

// EmbedTrueTypeFonts reports whether the presentation is flagged to embed
// TrueType fonts (p:presentation/@embedTrueTypeFonts).
func (p *Presentation) EmbedTrueTypeFonts() bool {
	return p.presentation != nil && p.presentation.EmbedTrueTypeFonts != nil && *p.presentation.EmbedTrueTypeFonts
}

// SetEmbedTrueTypeFonts sets the embedTrueTypeFonts flag on the presentation.
func (p *Presentation) SetEmbedTrueTypeFonts(v bool) {
	if p.presentation == nil {
		p.presentation = &oxml.Presentation{}
	}
	p.presentation.EmbedTrueTypeFonts = &v
}

// CustomShow describes one custom slide show (p:custShow): a named, ordered
// subset of the presentation's slides identified by their presentation-level
// relationship ids (the r:id of each p:sldId, also reported by Slide.RelID).
type CustomShow struct {
	Name        string
	ID          uint32
	SlideRelIDs []string
}

// CustomShows returns the presentation's custom shows (p:custShowLst) in
// document order, or nil when there are none.
func (p *Presentation) CustomShows() []CustomShow {
	if p.presentation == nil || p.presentation.CustShowLst == nil {
		return nil
	}
	var shows []CustomShow
	for _, cs := range p.presentation.CustShowLst.CustShow {
		show := CustomShow{Name: cs.Name, ID: cs.ID}
		if cs.SldLst != nil {
			for _, sld := range cs.SldLst.Sld {
				show.SlideRelIDs = append(show.SlideRelIDs, sld.Id)
			}
		}
		shows = append(shows, show)
	}
	return shows
}

// SetCustomShows replaces the presentation's custom-show list. Each show's
// SlideRelIDs must be presentation-level slide relationship ids (see
// Slide.RelID). Passing an empty slice clears the list.
func (p *Presentation) SetCustomShows(shows []CustomShow) {
	if p.presentation == nil {
		p.presentation = &oxml.Presentation{}
	}
	if len(shows) == 0 {
		p.presentation.CustShowLst = nil
		return
	}
	lst := &oxml.CustomShowList{CustShow: make([]oxml.CustomShow, 0, len(shows))}
	for _, s := range shows {
		cs := oxml.CustomShow{
			Name:   s.Name,
			ID:     s.ID,
			SldLst: &oxml.SlideRelationshipList{},
		}
		for _, rid := range s.SlideRelIDs {
			cs.SldLst.Sld = append(cs.SldLst.Sld, oxml.RelationshipRef{Id: rid})
		}
		lst.CustShow = append(lst.CustShow, cs)
	}
	p.presentation.CustShowLst = lst
}

// AddCustomShow appends a custom show that plays the given slides in order,
// assigning it the next free custom-show id. The slides must already have
// presentation-level relationship ids (true for slides loaded from a file or
// added and saved at least once); slides without one are skipped. It returns
// the assigned id.
func (p *Presentation) AddCustomShow(name string, slides ...*Slide) uint32 {
	id := p.nextCustomShowID()
	show := CustomShow{Name: name, ID: id}
	for _, s := range slides {
		if s != nil && s.relID != "" {
			show.SlideRelIDs = append(show.SlideRelIDs, s.relID)
		}
	}
	p.SetCustomShows(append(p.CustomShows(), show))
	return id
}

// nextCustomShowID returns an id one greater than the highest existing custom
// show id (custom show ids start at 0).
func (p *Presentation) nextCustomShowID() uint32 {
	var next uint32
	for _, cs := range p.CustomShows() {
		if cs.ID >= next {
			next = cs.ID + 1
		}
	}
	return next
}
