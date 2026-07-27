package pptx

import "testing"

// C255: saveNew reassigned every slide's relationship id by slice order, so a
// MoveSlide between AddCustomShow and Save left the stored custom-show reference
// pointing at whatever slide landed at the originally-referenced id. Eager ids
// are now kept stable, so the reference still resolves to the intended slide.
func TestCustomShow_RelIDStableAcrossReorderBeforeSave(t *testing.T) {
	p := Create()
	s1 := p.AddSlide()
	s1.SetName("ALPHA")
	s2 := p.AddSlide()
	s2.SetName("BETA")

	if s1.RelID() == "" {
		t.Fatal("AddSlide did not assign an eager relID")
	}

	p.AddCustomShow("show", s1)

	// Reorder the slides after recording the show but before the first save.
	if err := p.MoveSlide(0, 1); err != nil {
		t.Fatal(err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	reopened := openBytes(t, data)
	shows := reopened.CustomShows()
	if len(shows) != 1 || len(shows[0].SlideRelIDs) != 1 {
		t.Fatalf("custom shows = %+v", shows)
	}
	ref := shows[0].SlideRelIDs[0]

	var resolved string
	for _, s := range reopened.Slides() {
		if s.RelID() == ref {
			resolved = s.Name()
			break
		}
	}
	if resolved != "ALPHA" {
		t.Errorf("custom show ref %q resolves to slide %q, want ALPHA (the originally-referenced slide)", ref, resolved)
	}
}

// C304: a created slide reports a provisional relationship id from the moment it
// is added (not the empty string the old godoc claimed), and after save still
// reports a resolvable, non-empty id.
func TestSlideRelID_ProvisionalThenResolvable(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	if s.RelID() == "" {
		t.Fatal("AddSlide slide reports empty RelID; expected a provisional id")
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if s.RelID() == "" {
		t.Error("slide reports empty RelID after save")
	}

	// The saved id resolves to a sldId entry on reopen.
	reopened := openBytes(t, data)
	if got := reopened.Slides(); len(got) != 1 || got[0].RelID() == "" {
		t.Errorf("reopened slide relID = %q", got[0].RelID())
	}
}
