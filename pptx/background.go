package pptx

import (
	"fmt"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// This file exposes slide and slide-master background fills (p:cSld/p:bg/p:bgPr).
// A background reuses the same dml.Fill values as shape fills, so a solid,
// gradient, or pattern fill can be applied as a background; an image (blip)
// background embeds a media part and references it from the owning part's
// relationships.

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

// blipToBackgroundProps builds a p:bgPr whose fill is a stretched image
// (a:blipFill) referencing the embedded image part at relID.
func blipToBackgroundProps(relID string) *oxml.BackgroundProps {
	return &oxml.BackgroundProps{
		BlipFill: &dml.BlipFillXML{
			Blip:    &dml.BlipXML{Embed: relID},
			Stretch: &dml.StretchXML{FillRect: &dml.RelRect{}},
		},
	}
}

// validateBackgroundImage rejects an image background that cannot be embedded:
// no owning package, no bytes, or no content type.
func validateBackgroundImage(pres *Presentation, partName string, data []byte, contentType string) error {
	if pres == nil || partName == "" {
		return fmt.Errorf("pptx: background image needs a saved package association")
	}
	if len(data) == 0 {
		return fmt.Errorf("pptx: background image has no data")
	}
	if contentType == "" {
		return fmt.Errorf("pptx: background image has no content type")
	}
	return nil
}

// backgroundBlipEmbed returns the r:embed of a p:bg's image fill (a:blipFill),
// or "" when the background is absent or not an image fill. Used to garbage
// collect the rel + media part a replaced background image referenced.
func backgroundBlipEmbed(bg *oxml.Background) string {
	if bg == nil || bg.BgPr == nil || bg.BgPr.BlipFill == nil || bg.BgPr.BlipFill.Blip == nil {
		return ""
	}
	return bg.BgPr.BlipFill.Blip.Embed
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
	m := s.ensureModel()
	if m.CSld == nil {
		m.CSld = &oxml.CommonSlideData{}
	}
	return m.CSld
}

// SetBackgroundFill sets the slide's background to the given fill (solid,
// gradient, or pattern). It replaces any existing background.
func (s *Slide) SetBackgroundFill(fill dml.Fill) {
	s.ensureCSld().Bg = &oxml.Background{BgPr: fillToBackgroundProps(fill)}
}

// SetBackgroundImage sets the slide's background to a stretched image, embedding
// the bytes as a media part referenced from the slide and pointing an a:blipFill
// at it. contentType is the image MIME type (e.g. "image/png"). It replaces any
// existing background and returns an error when the image cannot be embedded (no
// package association, no data, or no content type).
func (s *Slide) SetBackgroundImage(data []byte, contentType string) error {
	if err := validateBackgroundImage(s.presentation, s.partName, data, contentType); err != nil {
		return err
	}
	oldRelID := backgroundBlipEmbed(s.ensureCSld().Bg)
	relID := s.presentation.embedImageForPart(s.partName, data, contentType)
	s.ensureCSld().Bg = &oxml.Background{BgPr: blipToBackgroundProps(relID)}
	// A prior background image is now orphaned (e.g. SetBackgroundImage called
	// twice); drop its rel + media part unless another node still references it
	// (C314).
	if oldRelID != "" && oldRelID != relID {
		s.gcSlideRels([]string{oldRelID})
	}
	return nil
}

// ClearBackground removes the slide's explicit background, so it inherits from
// the layout and master.
func (s *Slide) ClearBackground() {
	if s.sx() != nil && s.sx().CSld != nil {
		s.presentation.markModelEdited()
		s.sx().CSld.Bg = nil
	}
}

// HasBackground reports whether the slide carries an explicit background fill.
func (s *Slide) HasBackground() bool {
	if s.sx() == nil || s.sx().CSld == nil {
		return false
	}
	return hasBackgroundFill(s.sx().CSld.Bg)
}

// BackgroundColor returns the slide's solid background color and true, or a
// zero color and false when the background is absent or not a solid fill.
func (s *Slide) BackgroundColor() (dml.Color, bool) {
	if s.sx() == nil || s.sx().CSld == nil {
		return dml.Color{}, false
	}
	return backgroundColor(s.sx().CSld.Bg)
}

// --- Slide master background ---

// ensureCSld returns the master's common slide data, allocating the master XML
// and cSld as needed. Only the background setters call it, and each is about to
// write, so it also records the edit: the master part is regenerated on every
// save, which is why these mutators need no flag to persist and why nothing
// else would notice that the deck changed.
func (sm *SlideMaster) ensureCSld() *oxml.CommonSlideData {
	sm.presentation.markModelEdited()
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

// SetBackgroundImage sets the master's background to a stretched image,
// embedding the bytes as a media part referenced from the master. contentType is
// the image MIME type (e.g. "image/png"). It replaces any existing background
// and returns an error when the image cannot be embedded.
func (sm *SlideMaster) SetBackgroundImage(data []byte, contentType string) error {
	if err := validateBackgroundImage(sm.presentation, sm.partName, data, contentType); err != nil {
		return err
	}
	p := sm.presentation
	oldRelID := backgroundBlipEmbed(sm.ensureCSld().Bg)
	// A master's relationship id space is shared with its layout relationships,
	// which the master body references by id (p:sldLayoutId) and which the
	// from-scratch save path assigns from the layout order rather than from
	// p.relationships. Allocate the image rel past both so the two never collide.
	mediaName := p.embedImagePart(data, contentType)
	relID := sm.nextBackgroundRelID()
	p.addImageRel(sm.partName, mediaName, relID)
	sm.ensureCSld().Bg = &oxml.Background{BgPr: blipToBackgroundProps(relID)}
	// Drop a prior background image's now-orphaned rel + media part (C314).
	if oldRelID != "" && oldRelID != relID {
		if partXML, err := marshalSlideMaster(sm.masterXML); err == nil {
			p.gcPartRels(sm.partName, partXML, []string{oldRelID})
		}
	}
	return nil
}

// nextBackgroundRelID returns an rId free across both the master's own
// relationships and its layout relationships (see SetBackgroundImage).
func (sm *SlideMaster) nextBackgroundRelID() string {
	max := 0
	consider := func(id string) {
		if len(id) > 3 && id[:3] == "rId" {
			var n int
			if _, err := fmt.Sscanf(id, "rId%d", &n); err == nil && n > max {
				max = n
			}
		}
	}
	for _, rel := range sm.presentation.relationships[sm.partName] {
		if rel != nil {
			consider(rel.ID)
		}
	}
	for _, l := range sm.layouts {
		consider(l.relID)
	}
	return fmt.Sprintf("rId%d", max+1)
}

// ClearBackground removes the master's explicit background.
func (sm *SlideMaster) ClearBackground() {
	if sm.masterXML != nil && sm.masterXML.CSld != nil {
		sm.presentation.markModelEdited()
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
// and cSld as needed. See SlideMaster.ensureCSld for why it records the edit.
func (sl *SlideLayout) ensureCSld() *oxml.CommonSlideData {
	sl.presentation.markModelEdited()
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

// SetBackgroundImage sets the layout's background to a stretched image,
// embedding the bytes as a media part referenced from the layout. contentType is
// the image MIME type (e.g. "image/png"). It replaces any existing background
// and returns an error when the image cannot be embedded.
func (sl *SlideLayout) SetBackgroundImage(data []byte, contentType string) error {
	if err := validateBackgroundImage(sl.presentation, sl.partName, data, contentType); err != nil {
		return err
	}
	oldRelID := backgroundBlipEmbed(sl.ensureCSld().Bg)
	relID := sl.presentation.embedImageForPart(sl.partName, data, contentType)
	sl.ensureCSld().Bg = &oxml.Background{BgPr: blipToBackgroundProps(relID)}
	// Drop a prior background image's now-orphaned rel + media part (C314).
	if oldRelID != "" && oldRelID != relID {
		if partXML, err := marshalSlideLayout(sl.layoutXML); err == nil {
			sl.presentation.gcPartRels(sl.partName, partXML, []string{oldRelID})
		}
	}
	return nil
}

// ClearBackground removes the layout's explicit background, so it inherits from
// the master.
func (sl *SlideLayout) ClearBackground() {
	if sl.layoutXML != nil && sl.layoutXML.CSld != nil {
		sl.presentation.markModelEdited()
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
