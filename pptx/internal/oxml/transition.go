// Package oxml provides PresentationML transition types from pml.xsd.
// These types implement the p: namespace slide transition elements.
package oxml

import "github.com/mgilbir/spine/common/dml"

// Transition represents CT_SlideTransition (p:transition)
type Transition struct {
	Spd         string `xml:"spd,attr,omitempty"`       // slow, med, fast
	// AdvClick defaults to true when absent, so it is a pointer: nil means
	// "unspecified" (advance-on-click enabled), and an explicit false must be
	// emitted as advClick="0" rather than omitted (which readers treat as true).
	AdvClick    *bool  `xml:"advClick,attr,omitempty"`
	AdvTm       uint32 `xml:"advTm,attr,omitempty"`     // advance time in ms
	// Choice of transition type
	Blinds      *OrientationTransition    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main blinds,omitempty"`
	Checker     *OrientationTransition    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main checker,omitempty"`
	Circle      *EmptyTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main circle,omitempty"`
	Dissolve    *EmptyTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main dissolve,omitempty"`
	Comb        *OrientationTransition    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main comb,omitempty"`
	Cover       *EightDirectionTransition `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cover,omitempty"`
	Cut         *OptionalBlackTransition  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cut,omitempty"`
	Diamond     *EmptyTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main diamond,omitempty"`
	Fade        *OptionalBlackTransition  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main fade,omitempty"`
	Newsflash   *EmptyTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main newsflash,omitempty"`
	Plus        *EmptyTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main plus,omitempty"`
	Pull        *EightDirectionTransition `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pull,omitempty"`
	Push        *SideDirectionTransition  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main push,omitempty"`
	Random      *EmptyTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main random,omitempty"`
	RandomBar   *OrientationTransition    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main randomBar,omitempty"`
	Split       *SplitTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main split,omitempty"`
	Strips      *CornerDirectionTransition `xml:"http://schemas.openxmlformats.org/presentationml/2006/main strips,omitempty"`
	Wedge       *EmptyTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main wedge,omitempty"`
	Wheel       *WheelTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main wheel,omitempty"`
	Wipe        *SideDirectionTransition  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main wipe,omitempty"`
	Zoom        *InOutTransition          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main zoom,omitempty"`
	// Sound action
	SndAc       *TransitionSoundAction    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sndAc,omitempty"`
	ExtLst      *dml.ExtLst               `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// EmptyTransition represents CT_Empty for transitions with no parameters
type EmptyTransition struct{}

// OrientationTransition represents CT_OrientationTransition (p:blinds, p:checker, p:comb, p:randomBar)
type OrientationTransition struct {
	Dir string `xml:"dir,attr,omitempty"` // horz, vert (default horz)
}

// SideDirectionTransition represents CT_SideDirectionTransition (p:push, p:wipe)
type SideDirectionTransition struct {
	Dir string `xml:"dir,attr,omitempty"` // l, u, r, d (default l)
}

// CornerDirectionTransition represents CT_CornerDirectionTransition (p:strips)
type CornerDirectionTransition struct {
	Dir string `xml:"dir,attr,omitempty"` // lu, ru, ld, rd (default lu)
}

// EightDirectionTransition represents CT_EightDirectionTransition (p:cover, p:pull)
type EightDirectionTransition struct {
	Dir string `xml:"dir,attr,omitempty"` // l, u, r, d, lu, ru, ld, rd (default l)
}

// OptionalBlackTransition represents CT_OptionalBlackTransition (p:cut, p:fade)
type OptionalBlackTransition struct {
	ThruBlk bool `xml:"thruBlk,attr,omitempty"` // through black
}

// SplitTransition represents CT_SplitTransition (p:split)
type SplitTransition struct {
	Orient string `xml:"orient,attr,omitempty"` // horz, vert (default horz)
	Dir    string `xml:"dir,attr,omitempty"`    // in, out (default out)
}

// WheelTransition represents CT_WheelTransition (p:wheel)
type WheelTransition struct {
	Spokes uint32 `xml:"spokes,attr,omitempty"` // 1, 2, 3, 4, 8 (default 4)
}

// InOutTransition represents CT_InOutTransition (p:zoom)
type InOutTransition struct {
	Dir string `xml:"dir,attr,omitempty"` // in, out (default out)
}

// TransitionSoundAction represents CT_TransitionSoundAction (p:sndAc)
type TransitionSoundAction struct {
	StSnd  *TransitionStartSoundAction `xml:"http://schemas.openxmlformats.org/presentationml/2006/main stSnd,omitempty"`
	EndSnd *EmptyTransition            `xml:"http://schemas.openxmlformats.org/presentationml/2006/main endSnd,omitempty"`
}

// TransitionStartSoundAction represents CT_TransitionStartSoundAction (p:stSnd)
type TransitionStartSoundAction struct {
	Loop  bool              `xml:"loop,attr,omitempty"`
	Snd   *dml.EmbeddedWAVXML `xml:"http://schemas.openxmlformats.org/presentationml/2006/main snd,omitempty"`
}

// --- Office 2010+ Transition Extensions ---

// TransitionPrism represents p14:prism transition
type TransitionPrism struct {
	Dir     string `xml:"dir,attr,omitempty"`     // l, r, u, d
	IsContent bool `xml:"isContent,attr,omitempty"`
	IsInverted bool `xml:"isInverted,attr,omitempty"`
}

// TransitionRipple represents p14:ripple transition
type TransitionRipple struct {
	Dir string `xml:"dir,attr,omitempty"` // center, l, r, u, d
}

// TransitionHoneycomb represents p14:honeycomb transition (no params)
type TransitionHoneycomb struct{}

// TransitionFlash represents p14:flash transition (no params)
type TransitionFlash struct{}

// TransitionVortex represents p14:vortex transition
type TransitionVortex struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r, u, d
}

// TransitionShred represents p14:shred transition
type TransitionShred struct {
	Pattern string `xml:"pattern,attr,omitempty"` // strip, rectangle
	Dir     string `xml:"dir,attr,omitempty"`     // in, out
}

// TransitionSwitch represents p14:switch transition
type TransitionSwitch struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r
}

// TransitionFlip represents p14:flip transition
type TransitionFlip struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r
}

// TransitionGallery represents p14:gallery transition
type TransitionGallery struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r
}

// TransitionCube represents p14:cube transition
type TransitionCube struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r, u, d
}

// TransitionDoors represents p14:doors transition
type TransitionDoors struct {
	Dir string `xml:"dir,attr,omitempty"` // horz, vert
}

// TransitionBox represents p14:box transition
type TransitionBox struct {
	Dir string `xml:"dir,attr,omitempty"` // in, out
}

// TransitionComb represents p14:comb transition
type TransitionComb struct {
	Dir string `xml:"dir,attr,omitempty"` // horz, vert
}

// TransitionZoom represents p14:zoom transition
type TransitionZoom struct {
	Dir string `xml:"dir,attr,omitempty"` // in, out
}

// TransitionPan represents p14:pan transition
type TransitionPan struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r, u, d
}

// TransitionFerris represents p14:ferris transition
type TransitionFerris struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r
}

// TransitionReveal represents p14:reveal transition
type TransitionReveal struct {
	ThruBlk bool   `xml:"thruBlk,attr,omitempty"`
	Dir     string `xml:"dir,attr,omitempty"` // l, r
}

// TransitionWarp represents p14:warp transition
type TransitionWarp struct {
	Dir string `xml:"dir,attr,omitempty"` // in, out
}

// TransitionFlythrough represents p14:flythrough transition
type TransitionFlythrough struct {
	Dir     string `xml:"dir,attr,omitempty"` // in, out
	HasBounce bool `xml:"hasBounce,attr,omitempty"`
}

// TransitionWheel represents p14:wheelReverse transition
type TransitionWheelReverse struct {
	Spokes uint32 `xml:"spokes,attr,omitempty"`
}

// TransitionConvey represents p14:conveyor transition
type TransitionConvey struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r
}

// TransitionGlitter represents p14:glitter transition
type TransitionGlitter struct {
	Dir     string `xml:"dir,attr,omitempty"` // l, r, u, d
	Pattern string `xml:"pattern,attr,omitempty"` // diamond, hexagon
}

// TransitionWindow represents p14:window transition
type TransitionWindow struct {
	Dir string `xml:"dir,attr,omitempty"` // horz, vert
}

// TransitionOrbit represents p14:orbit transition
type TransitionOrbit struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r, u, d
}

// TransitionDrape represents p14:drape transition
type TransitionDrape struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r
}

// TransitionFallOver represents p14:fallOver transition
type TransitionFallOver struct {
	Dir string `xml:"dir,attr,omitempty"` // l, r
}
