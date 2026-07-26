package pptx

import (
	"strings"
	"testing"
)

// C312: a morph transition must carry its Transition.Sound. The morph branch
// returned before soundActionToOxml ran, so the emitted mc:AlternateContent
// carried no p:sndAc in either its Choice or its Fallback.
func TestSetTransitionMorphCarriesSound(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()

	slide.SetTransition(Transition{
		Type:           TransitionMorph,
		Duration:       1.0,
		AdvanceOnClick: true,
		Sound:          &TransitionSound{StopPreviousSound: true},
	})

	if pres.Validate().HasErrors() {
		t.Fatalf("validation errors: %v", pres.Validate())
	}

	out, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	sx := string(zipPart(t, out, "ppt/slides/slide1.xml"))

	// The morph transition is an mc:AlternateContent with a Choice (the morph)
	// and a Fallback (a fade); the sound action must appear in both.
	if n := strings.Count(sx, "<p:sndAc>"); n != 2 {
		t.Fatalf("emitted %d p:sndAc, want 2 (one in the morph Choice and one in the Fallback); sound was dropped\n%s", n, sx)
	}
	if n := strings.Count(sx, "<p:endSnd/>"); n != 2 {
		t.Errorf("emitted %d p:endSnd, want 2", n)
	}
}

// A start-sound clip on a morph transition is embedded and referenced by
// r:embed inside the p:sndAc of both fragments.
func TestSetTransitionMorphEmbedsStartSound(t *testing.T) {
	// Start from a saved deck so the slide has a part name; embedding a start
	// sound requires it.
	base := Create()
	base.AddSlide()
	data, err := base.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	pres := openBytes(t, data)
	slide := pres.Slides()[0]

	slide.SetTransition(Transition{
		Type:     TransitionMorph,
		Duration: 1.0,
		Sound: &TransitionSound{
			StartSoundData:        []byte("RIFF....WAVEfmt "),
			StartSoundContentType: "audio/wav",
			StartSoundName:        "chime",
		},
	})

	out, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	sx := string(zipPart(t, out, "ppt/slides/slide1.xml"))
	if n := strings.Count(sx, "<p:stSnd"); n != 2 {
		t.Fatalf("emitted %d p:stSnd, want 2 (Choice + Fallback); start sound dropped\n%s", n, sx)
	}
	if !strings.Contains(sx, `r:embed=`) {
		t.Errorf("p:snd missing r:embed reference to the embedded clip")
	}
	if !strings.Contains(sx, `name="chime"`) {
		t.Errorf("p:snd missing sound name")
	}
}
