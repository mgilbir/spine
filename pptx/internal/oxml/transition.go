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
	// AdvTm has no XSD default: an explicit advTm="0" (advance immediately)
	// is meaningful, so the field is a pointer — plain uint32,omitempty
	// deleted it on every save (C29).
	AdvTm       *uint32 `xml:"advTm,attr,omitempty"` // advance time in ms
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
	ExtLst      *ExtensionList               `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
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
