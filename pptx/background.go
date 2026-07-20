package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// This file exposes slide and slide-master background fills (p:cSld/p:bg/p:bgPr).
// A background reuses the same dml.Fill values as shape fills, so a solid,
// gradient, or pattern fill can be applied as a background.

// fillToBackgroundProps builds a p:bgPr from a dml.Fill, reusing Fill's own
// SpPr routing so the fill kind (solid/gradient/pattern/none) and its color
// handling stay identical to shape fills.
func fillToBackgroundProps(f dml.Fill) *oxml.BackgroundProps {
	var sp dml.SpPr
	f.ApplyToSpPr(&sp)
	return &oxml.BackgroundProps{
		NoFill:    sp.NoFill,
		SolidFill: sp.SolidFill,
		GradFill:  sp.GradFill,
		PattFill:  sp.PattFill,
	}
}

// backgroundColor returns the solid background color of a p:bg, when the
// background is a solid fill.
func backgroundColor(bg *oxml.Background) (dml.Color, bool) {
	if bg == nil || bg.BgPr == nil || bg.BgPr.SolidFill == nil {
		return dml.Color{}, false
	}
	if c := oxmlToColor(bg.BgPr.SolidFill); c != nil {
		return *c, true
	}
	return dml.Color{}, false
}

// hasBackgroundFill reports whether a p:bg carries an explicit fill (rather than
// a style reference or nothing).
func hasBackgroundFill(bg *oxml.Background) bool {
	if bg == nil || bg.BgPr == nil {
		return false
	}
	p := bg.BgPr
	return p.SolidFill != nil || p.GradFill != nil || p.PattFill != nil ||
		p.BlipFill != nil || p.NoFill != nil
}

// --- Slide background ---

// ensureCSld returns the slide's common slide data, allocating the slide XML
// and cSld as needed.
func (s *Slide) ensureCSld() *oxml.CommonSlideData {
	if s.slideXML == nil {
		s.slideXML = newSlideXML()
	}
	if s.slideXML.CSld == nil {
		s.slideXML.CSld = &oxml.CommonSlideData{}
	}
	return s.slideXML.CSld
}

// SetBackgroundFill sets the slide's background to the given fill (solid,
// gradient, or pattern). It replaces any existing background.
func (s *Slide) SetBackgroundFill(fill dml.Fill) {
	s.ensureCSld().Bg = &oxml.Background{BgPr: fillToBackgroundProps(fill)}
}

// ClearBackground removes the slide's explicit background, so it inherits from
// the layout and master.
func (s *Slide) ClearBackground() {
	if s.slideXML != nil && s.slideXML.CSld != nil {
		s.slideXML.CSld.Bg = nil
	}
}

// HasBackground reports whether the slide carries an explicit background fill.
func (s *Slide) HasBackground() bool {
	if s.slideXML == nil || s.slideXML.CSld == nil {
		return false
	}
	return hasBackgroundFill(s.slideXML.CSld.Bg)
}

// BackgroundColor returns the slide's solid background color and true, or a
// zero color and false when the background is absent or not a solid fill.
func (s *Slide) BackgroundColor() (dml.Color, bool) {
	if s.slideXML == nil || s.slideXML.CSld == nil {
		return dml.Color{}, false
	}
	return backgroundColor(s.slideXML.CSld.Bg)
}

// --- Slide master background ---

// ensureCSld returns the master's common slide data, allocating the master XML
// and cSld as needed.
func (sm *SlideMaster) ensureCSld() *oxml.CommonSlideData {
	if sm.masterXML == nil {
		sm.masterXML = &oxml.SlideMaster{}
	}
	if sm.masterXML.CSld == nil {
		sm.masterXML.CSld = &oxml.CommonSlideData{}
	}
	return sm.masterXML.CSld
}

// SetBackgroundFill sets the master's background to the given fill (solid,
// gradient, or pattern). It replaces any existing background.
func (sm *SlideMaster) SetBackgroundFill(fill dml.Fill) {
	sm.ensureCSld().Bg = &oxml.Background{BgPr: fillToBackgroundProps(fill)}
}

// ClearBackground removes the master's explicit background.
func (sm *SlideMaster) ClearBackground() {
	if sm.masterXML != nil && sm.masterXML.CSld != nil {
		sm.masterXML.CSld.Bg = nil
	}
}

// HasBackground reports whether the master carries an explicit background fill.
func (sm *SlideMaster) HasBackground() bool {
	if sm.masterXML == nil || sm.masterXML.CSld == nil {
		return false
	}
	return hasBackgroundFill(sm.masterXML.CSld.Bg)
}

// BackgroundColor returns the master's solid background color and true, or a
// zero color and false when the background is absent or not a solid fill.
func (sm *SlideMaster) BackgroundColor() (dml.Color, bool) {
	if sm.masterXML == nil || sm.masterXML.CSld == nil {
		return dml.Color{}, false
	}
	return backgroundColor(sm.masterXML.CSld.Bg)
}

// --- Slide layout background ---

// ensureCSld returns the layout's common slide data, allocating the layout XML
// and cSld as needed.
func (sl *SlideLayout) ensureCSld() *oxml.CommonSlideData {
	if sl.layoutXML == nil {
		sl.layoutXML = newLayoutXML(sl.layoutType)
	}
	if sl.layoutXML.CSld == nil {
		sl.layoutXML.CSld = &oxml.CommonSlideData{}
	}
	return sl.layoutXML.CSld
}

// SetBackgroundFill sets the layout's background to the given fill (solid,
// gradient, or pattern). It replaces any existing background.
func (sl *SlideLayout) SetBackgroundFill(fill dml.Fill) {
	sl.ensureCSld().Bg = &oxml.Background{BgPr: fillToBackgroundProps(fill)}
}

// ClearBackground removes the layout's explicit background, so it inherits from
// the master.
func (sl *SlideLayout) ClearBackground() {
	if sl.layoutXML != nil && sl.layoutXML.CSld != nil {
		sl.layoutXML.CSld.Bg = nil
	}
}

// HasBackground reports whether the layout carries an explicit background fill.
func (sl *SlideLayout) HasBackground() bool {
	if sl.layoutXML == nil || sl.layoutXML.CSld == nil {
		return false
	}
	return hasBackgroundFill(sl.layoutXML.CSld.Bg)
}

// BackgroundColor returns the layout's solid background color and true, or a
// zero color and false when the background is absent or not a solid fill.
func (sl *SlideLayout) BackgroundColor() (dml.Color, bool) {
	if sl.layoutXML == nil || sl.layoutXML.CSld == nil {
		return dml.Color{}, false
	}
	return backgroundColor(sl.layoutXML.CSld.Bg)
}
