package pptx

import (
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// TransitionType represents the type of slide transition.
type TransitionType int

const (
	TransitionNone     TransitionType = iota
	TransitionFade                    // Fade
	TransitionPush                    // Push from a side
	TransitionWipe                    // Wipe from a side
	TransitionSplit                   // Split in/out
	TransitionCover                   // Cover from a direction
	TransitionDissolve                // Dissolve
	TransitionBlind                   // Blinds
	TransitionChecker                 // Checkerboard
	TransitionWheel                   // Wheel (clock-like)
	TransitionRandom                  // Random transition
	TransitionCut                     // Cut
	TransitionDiamond                 // Diamond
	TransitionPlus                    // Plus sign
)

// Transition represents slide transition settings.
type Transition struct {
	Type TransitionType
	// Duration is the transition speed in seconds. The base OOXML schema only
	// stores a coarse speed (fast/med/slow), so the value snaps to 0.5, 1.0, or
	// 2.0 on a round-trip; exact durations require the p14 extension, which is
	// not yet modeled.
	Duration       float64
	AdvanceOnClick bool
	AdvanceAfter   float64 // seconds, 0 = disabled
}

// SetTransition sets the slide transition.
func (s *Slide) SetTransition(t Transition) {
	if s.slideXML == nil {
		s.slideXML = newSlideXML()
	}

	if t.Type == TransitionNone {
		s.slideXML.Transition = nil
		return
	}

	// Always set advClick explicitly so AdvanceOnClick=false is emitted
	// (advClick="0") rather than omitted and read back as the default true.
	advClick := t.AdvanceOnClick
	tr := &oxml.Transition{
		AdvClick: &advClick,
	}

	// Convert duration to speed attribute
	if t.Duration > 0 {
		switch {
		case t.Duration <= 0.5:
			tr.Spd = "fast"
		case t.Duration <= 1.0:
			tr.Spd = "med"
		default:
			tr.Spd = "slow"
		}
	}

	// Convert advance after to milliseconds
	if t.AdvanceAfter > 0 {
		advTm := uint32(t.AdvanceAfter * 1000)
		tr.AdvTm = &advTm
	}

	// Set transition type
	switch t.Type {
	case TransitionFade:
		tr.Fade = &oxml.OptionalBlackTransition{}
	case TransitionPush:
		tr.Push = &oxml.SideDirectionTransition{}
	case TransitionWipe:
		tr.Wipe = &oxml.SideDirectionTransition{}
	case TransitionSplit:
		tr.Split = &oxml.SplitTransition{}
	case TransitionCover:
		tr.Cover = &oxml.EightDirectionTransition{}
	case TransitionDissolve:
		tr.Dissolve = &oxml.EmptyTransition{}
	case TransitionBlind:
		tr.Blinds = &oxml.OrientationTransition{}
	case TransitionChecker:
		tr.Checker = &oxml.OrientationTransition{}
	case TransitionWheel:
		tr.Wheel = &oxml.WheelTransition{Spokes: 4}
	case TransitionRandom:
		tr.Random = &oxml.EmptyTransition{}
	case TransitionCut:
		tr.Cut = &oxml.OptionalBlackTransition{}
	case TransitionDiamond:
		tr.Diamond = &oxml.EmptyTransition{}
	case TransitionPlus:
		tr.Plus = &oxml.EmptyTransition{}
	}

	s.slideXML.Transition = tr
}

// Transition returns the current slide transition, or nil if none is set.
func (s *Slide) Transition() *Transition {
	if s.slideXML == nil || s.slideXML.Transition == nil {
		return nil
	}

	tr := s.slideXML.Transition
	t := &Transition{
		// advClick defaults to true when the attribute is absent.
		AdvanceOnClick: tr.AdvClick == nil || *tr.AdvClick,
	}

	// Convert speed to approximate duration
	switch tr.Spd {
	case "fast":
		t.Duration = 0.5
	case "med":
		t.Duration = 1.0
	case "slow":
		t.Duration = 2.0
	default:
		t.Duration = 1.0 // default
	}

	// Convert advance time from ms to seconds
	if tr.AdvTm != nil && *tr.AdvTm > 0 {
		t.AdvanceAfter = float64(*tr.AdvTm) / 1000.0
	}

	// Detect type
	switch {
	case tr.Fade != nil:
		t.Type = TransitionFade
	case tr.Push != nil:
		t.Type = TransitionPush
	case tr.Wipe != nil:
		t.Type = TransitionWipe
	case tr.Split != nil:
		t.Type = TransitionSplit
	case tr.Cover != nil:
		t.Type = TransitionCover
	case tr.Dissolve != nil:
		t.Type = TransitionDissolve
	case tr.Blinds != nil:
		t.Type = TransitionBlind
	case tr.Checker != nil:
		t.Type = TransitionChecker
	case tr.Wheel != nil:
		t.Type = TransitionWheel
	case tr.Random != nil:
		t.Type = TransitionRandom
	case tr.Cut != nil:
		t.Type = TransitionCut
	case tr.Diamond != nil:
		t.Type = TransitionDiamond
	case tr.Plus != nil:
		t.Type = TransitionPlus
	}

	return t
}
