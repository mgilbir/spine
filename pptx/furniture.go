package pptx

// Slide furniture: footer text, slide numbers, and the date placeholder.
//
// These are the three "Header and Footer" items PowerPoint places at the
// bottom of slides. Each is a placeholder shape of a fixed type (ftr, sldNum,
// dt) that omits its own geometry so it inherits position and size from the
// slide's layout — which is why the deck-wide setters below work against
// template decks whose layouts already define these placeholders.
//
// The setters are idempotent: an existing placeholder of the matching type
// (whether it came from the template or a previous call) is updated in place
// rather than duplicated.

// SetSlideFooter sets the footer text shown on every slide, adding a footer
// placeholder to slides that lack one and updating those that already have
// one. An empty string clears the footer text but keeps the placeholder;
// use ClearSlideFooter to remove it entirely.
func (p *Presentation) SetSlideFooter(text string) {
	for _, s := range p.Slides() {
		if ph := s.GetPlaceholder(PlaceholderFooter); ph != nil {
			ph.fieldType = ""
			ph.SetText(text)
			ph.markDirty()
			continue
		}
		ph := newFurniturePlaceholder(PlaceholderFooter)
		ph.SetText(text)
		s.addShape(ph)
	}
}

// ClearSlideFooter removes the footer placeholder from every slide.
func (p *Presentation) ClearSlideFooter() {
	p.removeFurniture(PlaceholderFooter)
}

// ShowSlideNumbers turns the automatic slide-number field on or off for every
// slide. When enabled, each slide gets a slide-number placeholder whose field
// PowerPoint renders as the slide's position; when disabled, any such
// placeholder is removed.
func (p *Presentation) ShowSlideNumbers(show bool) {
	if !show {
		p.removeFurniture(PlaceholderSlideNumber)
		return
	}
	for _, s := range p.Slides() {
		if ph := s.GetPlaceholder(PlaceholderSlideNumber); ph != nil {
			ph.fieldType = "slidenum"
			ph.fieldText = slideNumberGlyph
			ph.markDirty()
			continue
		}
		ph := newFurniturePlaceholder(PlaceholderSlideNumber)
		ph.fieldType = "slidenum"
		ph.fieldText = slideNumberGlyph
		s.addShape(ph)
	}
}

// SetSlideDate sets a fixed date/time string shown on every slide (added to
// slides without a date placeholder, updated on those that have one). For an
// auto-updating date that PowerPoint refreshes, use SetSlideDateAuto.
func (p *Presentation) SetSlideDate(text string) {
	for _, s := range p.Slides() {
		if ph := s.GetPlaceholder(PlaceholderDateTime); ph != nil {
			ph.fieldType = ""
			ph.SetText(text)
			ph.markDirty()
			continue
		}
		ph := newFurniturePlaceholder(PlaceholderDateTime)
		ph.SetText(text)
		s.addShape(ph)
	}
}

// SetSlideDateAuto places an auto-updating date field on every slide;
// PowerPoint refreshes it to the current date when the deck is opened.
func (p *Presentation) SetSlideDateAuto() {
	for _, s := range p.Slides() {
		if ph := s.GetPlaceholder(PlaceholderDateTime); ph != nil {
			ph.fieldType = "datetime"
			ph.fieldText = ""
			ph.textFrame = nil
			ph.markDirty()
			continue
		}
		ph := newFurniturePlaceholder(PlaceholderDateTime)
		ph.fieldType = "datetime"
		s.addShape(ph)
	}
}

// ClearSlideDate removes the date placeholder from every slide.
func (p *Presentation) ClearSlideDate() {
	p.removeFurniture(PlaceholderDateTime)
}

// SlideFooter returns the footer text shown on slides and whether any slide
// carries a footer placeholder. The deck-wide setters keep every slide's footer
// in sync, so the first slide with one is representative. It complements
// SetSlideFooter / ClearSlideFooter.
func (p *Presentation) SlideFooter() (string, bool) {
	for _, s := range p.Slides() {
		if ph := s.GetPlaceholder(PlaceholderFooter); ph != nil {
			return ph.Text(), true
		}
	}
	return "", false
}

// SlideNumbersVisible reports whether slides carry an automatic slide-number
// placeholder, complementing ShowSlideNumbers.
func (p *Presentation) SlideNumbersVisible() bool {
	for _, s := range p.Slides() {
		if s.GetPlaceholder(PlaceholderSlideNumber) != nil {
			return true
		}
	}
	return false
}

// SlideDate returns the fixed date/time text shown on slides and whether any
// slide carries a date placeholder. For an auto-updating date field (set via
// SetSlideDateAuto) the text is empty; use SlideDateIsAuto to distinguish it.
// It complements SetSlideDate / SetSlideDateAuto / ClearSlideDate.
func (p *Presentation) SlideDate() (string, bool) {
	for _, s := range p.Slides() {
		if ph := s.GetPlaceholder(PlaceholderDateTime); ph != nil {
			return ph.Text(), true
		}
	}
	return "", false
}

// SlideDateIsAuto reports whether the slide date placeholder is an
// auto-updating field (SetSlideDateAuto) rather than fixed text. This reflects
// the in-memory model set during the current session; the auto/fixed field
// state is not repopulated when a deck is reopened from disk.
func (p *Presentation) SlideDateIsAuto() bool {
	for _, s := range p.Slides() {
		if ph := s.GetPlaceholder(PlaceholderDateTime); ph != nil {
			return ph.fieldType == "datetime"
		}
	}
	return false
}

// markDirty flags a furniture placeholder so an in-place field/text change on
// a placeholder parsed from a template is flushed on the next sync (setting a
// field type directly bypasses the text-frame dirty flag).
func (p *PlaceholderShape) markDirty() { p.dirty = true }

// slideNumberGlyph is the fallback text a slide-number field shows until
// PowerPoint recalculates it (the "‹#›" marker PowerPoint itself writes).
const slideNumberGlyph = "‹#›"

// newFurniturePlaceholder builds a geometry-inheriting placeholder of the
// given furniture type with the size hint PowerPoint uses for it.
func newFurniturePlaceholder(t PlaceholderType) *PlaceholderShape {
	ph := NewPlaceholderShape(t)
	ph.inheritGeometry = true
	// Match the size hints PowerPoint writes for these placeholders so the
	// layout's placeholder of the same (type, sz) binds for geometry.
	switch t {
	case PlaceholderDateTime:
		ph.size = PlaceholderSizeHalf
	case PlaceholderFooter, PlaceholderSlideNumber:
		ph.size = PlaceholderSizeQuarter
	}
	return ph
}

// removeFurniture removes every placeholder of the given type from all slides.
func (p *Presentation) removeFurniture(t PlaceholderType) {
	for _, s := range p.Slides() {
		if ph := s.GetPlaceholder(t); ph != nil {
			s.RemoveShape(ph)
		}
	}
}
