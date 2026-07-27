// This file provides PresentationML transition types from pml.xsd.
// These types implement the p: namespace slide transition elements.

package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// Transition represents CT_SlideTransition (p:transition)
type Transition struct {
	Spd string `xml:"spd,attr,omitempty"` // slow, med, fast
	// AdvClick defaults to true when absent, so it is a pointer: nil means
	// "unspecified" (advance-on-click enabled), and an explicit false must be
	// emitted as advClick="0" rather than omitted (which readers treat as true).
	AdvClick *bool `xml:"advClick,attr,omitempty"`
	// AdvTm has no XSD default: an explicit advTm="0" (advance immediately)
	// is meaningful, so the field is a pointer — plain uint32,omitempty
	// deleted it on every save (C29).
	AdvTm *uint32 `xml:"advTm,attr,omitempty"` // advance time in ms
	// Choice of transition type
	Blinds    *OrientationTransition     `xml:"http://schemas.openxmlformats.org/presentationml/2006/main blinds,omitempty"`
	Checker   *OrientationTransition     `xml:"http://schemas.openxmlformats.org/presentationml/2006/main checker,omitempty"`
	Circle    *EmptyTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main circle,omitempty"`
	Dissolve  *EmptyTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main dissolve,omitempty"`
	Comb      *OrientationTransition     `xml:"http://schemas.openxmlformats.org/presentationml/2006/main comb,omitempty"`
	Cover     *EightDirectionTransition  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cover,omitempty"`
	Cut       *OptionalBlackTransition   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cut,omitempty"`
	Diamond   *EmptyTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main diamond,omitempty"`
	Fade      *OptionalBlackTransition   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main fade,omitempty"`
	Newsflash *EmptyTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main newsflash,omitempty"`
	Plus      *EmptyTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main plus,omitempty"`
	Pull      *EightDirectionTransition  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pull,omitempty"`
	Push      *SideDirectionTransition   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main push,omitempty"`
	Random    *EmptyTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main random,omitempty"`
	RandomBar *OrientationTransition     `xml:"http://schemas.openxmlformats.org/presentationml/2006/main randomBar,omitempty"`
	Split     *SplitTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main split,omitempty"`
	Strips    *CornerDirectionTransition `xml:"http://schemas.openxmlformats.org/presentationml/2006/main strips,omitempty"`
	Wedge     *EmptyTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main wedge,omitempty"`
	Wheel     *WheelTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main wheel,omitempty"`
	Wipe      *SideDirectionTransition   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main wipe,omitempty"`
	Zoom      *InOutTransition           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main zoom,omitempty"`
	// Sound action
	SndAc  *TransitionSoundAction `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sndAc,omitempty"`
	ExtLst *ExtensionList         `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`

	// CapturedAttrs preserves the verbatim source attribute list (attribute
	// order and inline xmlns declarations such as xmlns:p14 on p:transition);
	// replayed by the reflection marshaler.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// through the struct tags.
func (t *Transition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	t.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Transition
	return d.DecodeElement((*alias)(t), &start)
}

// EmptyTransition represents CT_Empty for transitions with no parameters
type EmptyTransition struct{}

// The transition-choice types below all carry XSD-defaulted attributes
// (dir/orient/spokes default to a named value, thruBlk and loop default to
// false), which omitempty deletes when a producer writes the default
// explicitly. Slides, layouts and masters re-marshal their p:transition on
// every save, so each of these needs the CapturedAttrs convention to keep an
// explicit thruBlk="0" / loop="0" / spokes="0" and any unmodeled attribute
// (C420). nil means "built programmatically" and takes the canonical emission.

// OrientationTransition represents CT_OrientationTransition (p:blinds, p:checker, p:comb, p:randomBar)
type OrientationTransition struct {
	Dir           string          `xml:"dir,attr,omitempty"` // horz, vert (default horz)
	CapturedAttrs []xmlb.RootAttr `xml:"-"`                  // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// through the struct tags; the reflection marshaler replays it.
func (v *OrientationTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias OrientationTransition
	return d.DecodeElement((*alias)(v), &start)
}

// SideDirectionTransition represents CT_SideDirectionTransition (p:push, p:wipe)
type SideDirectionTransition struct {
	Dir           string          `xml:"dir,attr,omitempty"` // l, u, r, d (default l)
	CapturedAttrs []xmlb.RootAttr `xml:"-"`                  // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (see
// OrientationTransition.UnmarshalXML).
func (v *SideDirectionTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias SideDirectionTransition
	return d.DecodeElement((*alias)(v), &start)
}

// CornerDirectionTransition represents CT_CornerDirectionTransition (p:strips)
type CornerDirectionTransition struct {
	Dir           string          `xml:"dir,attr,omitempty"` // lu, ru, ld, rd (default lu)
	CapturedAttrs []xmlb.RootAttr `xml:"-"`                  // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (see
// OrientationTransition.UnmarshalXML).
func (v *CornerDirectionTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CornerDirectionTransition
	return d.DecodeElement((*alias)(v), &start)
}

// EightDirectionTransition represents CT_EightDirectionTransition (p:cover, p:pull)
type EightDirectionTransition struct {
	Dir           string          `xml:"dir,attr,omitempty"` // l, u, r, d, lu, ru, ld, rd (default l)
	CapturedAttrs []xmlb.RootAttr `xml:"-"`                  // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (see
// OrientationTransition.UnmarshalXML).
func (v *EightDirectionTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias EightDirectionTransition
	return d.DecodeElement((*alias)(v), &start)
}

// OptionalBlackTransition represents CT_OptionalBlackTransition (p:cut, p:fade)
type OptionalBlackTransition struct {
	ThruBlk       bool            `xml:"thruBlk,attr,omitempty"` // through black
	CapturedAttrs []xmlb.RootAttr `xml:"-"`                      // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list, so an explicit
// thruBlk="0" survives the re-marshal (see OrientationTransition.UnmarshalXML).
func (v *OptionalBlackTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias OptionalBlackTransition
	return d.DecodeElement((*alias)(v), &start)
}

// SplitTransition represents CT_SplitTransition (p:split)
type SplitTransition struct {
	Orient        string          `xml:"orient,attr,omitempty"` // horz, vert (default horz)
	Dir           string          `xml:"dir,attr,omitempty"`    // in, out (default out)
	CapturedAttrs []xmlb.RootAttr `xml:"-"`                     // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (see
// OrientationTransition.UnmarshalXML).
func (v *SplitTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias SplitTransition
	return d.DecodeElement((*alias)(v), &start)
}

// WheelTransition represents CT_WheelTransition (p:wheel)
type WheelTransition struct {
	Spokes        uint32          `xml:"spokes,attr,omitempty"` // 1, 2, 3, 4, 8 (default 4)
	CapturedAttrs []xmlb.RootAttr `xml:"-"`                     // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list, so an explicit
// spokes="0" is not deleted and silently re-read as the schema default 4 (see
// OrientationTransition.UnmarshalXML).
func (v *WheelTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias WheelTransition
	return d.DecodeElement((*alias)(v), &start)
}

// InOutTransition represents CT_InOutTransition (p:zoom)
type InOutTransition struct {
	Dir           string          `xml:"dir,attr,omitempty"` // in, out (default out)
	CapturedAttrs []xmlb.RootAttr `xml:"-"`                  // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (see
// OrientationTransition.UnmarshalXML).
func (v *InOutTransition) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias InOutTransition
	return d.DecodeElement((*alias)(v), &start)
}

// TransitionSoundAction represents CT_TransitionSoundAction (p:sndAc)
type TransitionSoundAction struct {
	StSnd  *TransitionStartSoundAction `xml:"http://schemas.openxmlformats.org/presentationml/2006/main stSnd,omitempty"`
	EndSnd *EmptyTransition            `xml:"http://schemas.openxmlformats.org/presentationml/2006/main endSnd,omitempty"`
}

// TransitionStartSoundAction represents CT_TransitionStartSoundAction (p:stSnd)
type TransitionStartSoundAction struct {
	Loop          bool                `xml:"loop,attr,omitempty"`
	Snd           *dml.EmbeddedWAVXML `xml:"http://schemas.openxmlformats.org/presentationml/2006/main snd,omitempty"`
	CapturedAttrs []xmlb.RootAttr     `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list, so an explicit
// loop="0" survives the re-marshal (see OrientationTransition.UnmarshalXML).
func (v *TransitionStartSoundAction) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias TransitionStartSoundAction
	return d.DecodeElement((*alias)(v), &start)
}
