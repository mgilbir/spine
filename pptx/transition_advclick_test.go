package pptx

import (
	"bytes"
	"testing"
)

// C140: AdvanceOnClick=false is representable and survives a save round-trip
// (previously false was omitted and read back as the default true).
func TestSetTransition_AdvanceOnClickFalseRoundTrips(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.SetTransition(Transition{Type: TransitionFade, AdvanceOnClick: false})

	if s.Transition().AdvanceOnClick {
		t.Fatal("AdvanceOnClick set false but read back true before save")
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Slides()[0].Transition()
	if got == nil {
		t.Fatal("transition lost on round-trip")
	}
	if got.AdvanceOnClick {
		t.Error("AdvanceOnClick=false became true after save/reload (zero-value trap)")
	}

	// And true still round-trips.
	s.SetTransition(Transition{Type: TransitionFade, AdvanceOnClick: true})
	if !s.Transition().AdvanceOnClick {
		t.Error("AdvanceOnClick=true read back false")
	}
}
