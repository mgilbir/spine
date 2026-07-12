package pptx

import (
	"testing"
)

func TestSetTransitionFade(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()

	slide.SetTransition(Transition{
		Type:           TransitionFade,
		Duration:       0.5,
		AdvanceOnClick: true,
		AdvanceAfter:   5.0,
	})

	tr := slide.Transition()
	if tr == nil {
		t.Fatal("expected transition to be set")
	}
	if tr.Type != TransitionFade {
		t.Errorf("Type = %d, want TransitionFade", tr.Type)
	}
	if tr.Duration != 0.5 {
		t.Errorf("Duration = %f, want 0.5", tr.Duration)
	}
	if !tr.AdvanceOnClick {
		t.Error("expected AdvanceOnClick to be true")
	}
	if tr.AdvanceAfter != 5.0 {
		t.Errorf("AdvanceAfter = %f, want 5.0", tr.AdvanceAfter)
	}
}

func TestSetTransitionPush(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()

	slide.SetTransition(Transition{
		Type:     TransitionPush,
		Duration: 1.0,
	})

	tr := slide.Transition()
	if tr == nil {
		t.Fatal("expected transition to be set")
	}
	if tr.Type != TransitionPush {
		t.Errorf("Type = %d, want TransitionPush", tr.Type)
	}
}

func TestSetTransitionNone(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()

	// Set then clear
	slide.SetTransition(Transition{
		Type: TransitionFade,
	})
	slide.SetTransition(Transition{
		Type: TransitionNone,
	})

	tr := slide.Transition()
	if tr != nil {
		t.Error("expected transition to be nil after setting TransitionNone")
	}
}

func TestNoTransition(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()

	tr := slide.Transition()
	if tr != nil {
		t.Error("expected no transition on new slide")
	}
}

func TestAllTransitionTypes(t *testing.T) {
	types := []struct {
		typ  TransitionType
		name string
	}{
		{TransitionFade, "Fade"},
		{TransitionPush, "Push"},
		{TransitionWipe, "Wipe"},
		{TransitionSplit, "Split"},
		{TransitionCover, "Cover"},
		{TransitionDissolve, "Dissolve"},
		{TransitionBlind, "Blind"},
		{TransitionChecker, "Checker"},
		{TransitionWheel, "Wheel"},
		{TransitionRandom, "Random"},
		{TransitionCut, "Cut"},
		{TransitionDiamond, "Diamond"},
		{TransitionPlus, "Plus"},
	}

	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			pres := Create()
			slide := pres.AddSlide()

			slide.SetTransition(Transition{
				Type:           tc.typ,
				AdvanceOnClick: true,
			})

			tr := slide.Transition()
			if tr == nil {
				t.Fatal("expected transition to be set")
			}
			if tr.Type != tc.typ {
				t.Errorf("Type = %d, want %d", tr.Type, tc.typ)
			}
		})
	}
}

func TestTransitionAdvanceAfterMs(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()

	slide.SetTransition(Transition{
		Type:         TransitionFade,
		AdvanceAfter: 3.5,
	})

	// Check internal representation
	if slide.slideXML.Transition.AdvTm == nil || *slide.slideXML.Transition.AdvTm != 3500 {
		t.Errorf("AdvTm = %v, want 3500", slide.slideXML.Transition.AdvTm)
	}

	// Check public API round-trip
	tr := slide.Transition()
	if tr.AdvanceAfter != 3.5 {
		t.Errorf("AdvanceAfter = %f, want 3.5", tr.AdvanceAfter)
	}
}

func TestTransitionSpeedMapping(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()

	// Fast (<=0.5s)
	slide.SetTransition(Transition{Type: TransitionFade, Duration: 0.3})
	if slide.slideXML.Transition.Spd != "fast" {
		t.Errorf("Spd = %s, want fast", slide.slideXML.Transition.Spd)
	}

	// Med (0.5-1.0s)
	slide.SetTransition(Transition{Type: TransitionFade, Duration: 0.8})
	if slide.slideXML.Transition.Spd != "med" {
		t.Errorf("Spd = %s, want med", slide.slideXML.Transition.Spd)
	}

	// Slow (>1.0s)
	slide.SetTransition(Transition{Type: TransitionFade, Duration: 2.0})
	if slide.slideXML.Transition.Spd != "slow" {
		t.Errorf("Spd = %s, want slow", slide.slideXML.Transition.Spd)
	}
}
