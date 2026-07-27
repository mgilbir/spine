package pptx

import (
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Section is a named group of slides — the sections shown in PowerPoint's
// thumbnail pane. Membership is tracked by slide id and kept consistent with
// the presentation's slide list; a slide belongs to at most one section.
type Section struct {
	pres *Presentation
	s    *oxml.P14Section
}

// Name returns the section's display name.
func (s *Section) Name() string { return s.s.Name }

// SetName renames the section.
func (s *Section) SetName(name string) {
	if s.s.Name == name {
		return
	}
	s.s.Name = name
	if sl := s.pres.sectionLst(); sl != nil {
		sl.MarkDirty()
	}
}

// ID returns the section's GUID (PowerPoint's brace-wrapped uppercase form).
func (s *Section) ID() string { return s.s.ID }

// Slides returns the section's member slides in order. Member ids that no
// longer resolve to a slide in the presentation are skipped.
func (s *Section) Slides() []*Slide {
	out := make([]*Slide, 0, len(s.s.SldId))
	for _, id := range s.s.SldId {
		if sl := s.pres.slideByID(id); sl != nil {
			out = append(out, sl)
		}
	}
	return out
}

// AddSlide assigns slide to this section, removing it from any other section
// first (a slide belongs to at most one section).
func (s *Section) AddSlide(slide *Slide) {
	s.pres.MoveSlideToSection(slide, s)
}

// Sections returns the presentation's slide sections in order, or nil when the
// deck defines none.
func (p *Presentation) Sections() []*Section {
	sl := p.sectionLst()
	if sl == nil {
		return nil
	}
	out := make([]*Section, 0, len(sl.Section))
	for _, s := range sl.Section {
		out = append(out, &Section{pres: p, s: s})
	}
	return out
}

// AddSection appends a new, empty section with the given name and returns it.
// The section is written into the presentation's p14 extension list on save.
//
// Section membership is tracked by slide id, independently of slide order.
// Two consequences are worth knowing, because PowerPoint's own UI maintains
// invariants this package does not enforce:
//
//   - Sections need not cover every slide. PowerPoint's UI always partitions
//     the whole deck, so a slide left in no section is a state its editor does
//     not produce.
//   - Presentation.MoveSlide reorders the slide list without reassigning
//     sections, so a slide moved across a section boundary keeps its old
//     section and the sections stop being contiguous runs of the slide order.
//     Call MoveSlideToSection afterwards to reassign it.
//
// Neither is enforced here: the schema permits both, and how PowerPoint reacts
// to a deck that violates them is unverified.
func (p *Presentation) AddSection(name string) *Section {
	sl := p.ensureSectionLst()
	sec := &oxml.P14Section{Name: name, ID: newGUID()}
	sl.Section = append(sl.Section, sec)
	sl.MarkDirty()
	return &Section{pres: p, s: sec}
}

// MoveSlideToSection assigns slide to section, removing its id from every other
// section so a slide belongs to at most one section (consistent with the
// sldIdLst). A nil section removes the slide from all sections.
func (p *Presentation) MoveSlideToSection(slide *Slide, section *Section) {
	if slide == nil {
		return
	}
	sl := p.sectionLst()
	if sl == nil {
		if section == nil {
			return
		}
		sl = p.ensureSectionLst()
	}
	id := slide.id
	for _, s := range sl.Section {
		s.SldId = removeSlideID(s.SldId, id)
	}
	if section != nil {
		section.s.SldId = append(section.s.SldId, id)
	}
	sl.MarkDirty()
}

// sectionLst returns the parsed p14:sectionLst extension, or nil if absent.
func (p *Presentation) sectionLst() *oxml.P14SectionLst {
	if p.presentation == nil || p.presentation.ExtLst == nil {
		return nil
	}
	for i := range p.presentation.ExtLst.Ext {
		if sl := p.presentation.ExtLst.Ext[i].SectionLst; sl != nil {
			return sl
		}
	}
	return nil
}

// ensureSectionLst returns the presentation's p14:sectionLst, creating the
// extension (and the enclosing extLst) when the deck has none.
func (p *Presentation) ensureSectionLst() *oxml.P14SectionLst {
	if sl := p.sectionLst(); sl != nil {
		return sl
	}
	if p.presentation.ExtLst == nil {
		p.presentation.ExtLst = &oxml.ExtensionList{}
	}
	sl := &oxml.P14SectionLst{}
	sl.MarkDirty()
	p.presentation.ExtLst.Ext = append(p.presentation.ExtLst.Ext, oxml.Extension{
		URI:        xmlb.ExtURISectionLst,
		SectionLst: sl,
	})
	return sl
}

// removeSlideFromSections strips id from every section's member list, marking
// the section list dirty when the id was actually present. RemoveSlide calls
// it so a removed slide leaves no dangling p14:sldId reference — a section
// member id with no matching entry in the presentation's sldIdLst — on save.
func (p *Presentation) removeSlideFromSections(id uint32) {
	sl := p.sectionLst()
	if sl == nil {
		return
	}
	changed := false
	for _, s := range sl.Section {
		n := len(s.SldId)
		s.SldId = removeSlideID(s.SldId, id)
		if len(s.SldId) != n {
			changed = true
		}
	}
	if changed {
		sl.MarkDirty()
	}
}

// slideByID returns the slide with the given presentation slide id, or nil.
func (p *Presentation) slideByID(id uint32) *Slide {
	for _, s := range p.slides {
		if s.id == id {
			return s
		}
	}
	return nil
}

// removeSlideID returns ids with the first occurrence of id removed, preserving
// order.
func removeSlideID(ids []uint32, id uint32) []uint32 {
	for i, v := range ids {
		if v == id {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}
