package pptx

import (
	"encoding/xml"
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

// MorphOption is an open string type; a value with XML metacharacters must be
// escaped into the synthesized morph Choice, not concatenated verbatim (which
// produced a malformed part).
func TestSetTransitionMorphOptionEscaped(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()

	slide.SetTransition(Transition{
		Type:        TransitionMorph,
		Duration:    1.0,
		MorphOption: MorphOption(`byObject" x="<&`),
	})

	out, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	sx := zipPart(t, out, "ppt/slides/slide1.xml")

	// The slide part must be well-formed XML (a verbatim concatenation would
	// break parsing at the injected quote/angle brackets).
	dec := xml.NewDecoder(strings.NewReader(string(sx)))
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("emitted malformed slide XML: %v\n%s", err, sx)
		}
	}

	// The raw metacharacters must be escaped, not present literally in the attr.
	if strings.Contains(string(sx), `option="byObject" x="<&`) {
		t.Errorf("morph option written unescaped\n%s", sx)
	}
	if !strings.Contains(string(sx), "&amp;") {
		t.Errorf("expected escaped ampersand in morph option\n%s", sx)
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

// C516: SetTransition carries the sound into the morph AlternateContent (the
// C312 fix above), but Transition() never parsed p:sndAc back out of it. A
// read-modify-write — the natural way to tweak one field of an existing
// transition — therefore stripped the sound and its embedded clip reference,
// while the base-schema path round-tripped it correctly through
// soundActionFromOxml. The two paths are now coherent.
func TestMorphTransitionReadModifyWriteKeepsSound(t *testing.T) {
	base := Create()
	base.AddSlide()
	data, err := base.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	pres := openBytes(t, data)
	pres.Slides()[0].SetTransition(Transition{
		Type:        TransitionMorph,
		MorphOption: MorphByWord,
		Duration:    1.25,
		Sound: &TransitionSound{
			StartSoundData:        []byte("RIFF....WAVEfmt "),
			StartSoundContentType: "audio/wav",
			StartSoundName:        "chime",
			StartSoundLoop:        true,
		},
	})
	saved, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// Reopen, read the transition back, and write it straight out again.
	pres2 := openBytes(t, saved)
	slide := pres2.Slides()[0]
	got := slide.Transition()
	if got == nil {
		t.Fatal("Transition() reported no transition for a morph")
	}
	if got.Sound == nil {
		t.Fatal("Transition() dropped the morph's sound, so a read-modify-write deletes it")
	}
	if got.Sound.StartSoundName != "chime" {
		t.Errorf("start sound name = %q, want chime", got.Sound.StartSoundName)
	}
	if !got.Sound.StartSoundLoop {
		t.Error("start sound loop flag was not read back")
	}

	slide.SetTransition(*got)
	out, err := pres2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	sx := string(zipPart(t, out, "ppt/slides/slide1.xml"))

	if n := strings.Count(sx, "<p:stSnd"); n != 2 {
		t.Errorf("after a read-modify-write %d p:stSnd remain, want 2\n%s", n, sx)
	}
	if !strings.Contains(sx, `name="chime"`) {
		t.Errorf("read-modify-write lost the sound name\n%s", sx)
	}
	if !strings.Contains(sx, `loop="1"`) {
		t.Errorf("read-modify-write lost the sound loop flag\n%s", sx)
	}
	// The clip must still be referenced, not dropped or re-embedded.
	if !strings.Contains(sx, `r:embed=`) {
		t.Errorf("read-modify-write lost the embedded start-sound reference\n%s", sx)
	}
}

// C516: a morph with no sound must read back as having none, and one carrying
// only p:endSnd (which needs no audio part) must round-trip that.
func TestMorphTransitionSoundReadBackEdgeCases(t *testing.T) {
	t.Run("no sound", func(t *testing.T) {
		pres := Create()
		pres.AddSlide().SetTransition(Transition{Type: TransitionMorph, Duration: 0.5})
		data, err := pres.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		got := openBytes(t, data).Slides()[0].Transition()
		if got == nil {
			t.Fatal("Transition() reported no transition for a morph")
		}
		if got.Sound != nil {
			t.Errorf("a soundless morph read back a sound: %+v", got.Sound)
		}
	})

	t.Run("endSnd only", func(t *testing.T) {
		pres := Create()
		pres.AddSlide().SetTransition(Transition{
			Type:     TransitionMorph,
			Duration: 0.5,
			Sound:    &TransitionSound{StopPreviousSound: true},
		})
		data, err := pres.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		got := openBytes(t, data).Slides()[0].Transition()
		if got == nil || got.Sound == nil {
			t.Fatalf("the endSnd-only sound was not read back: %+v", got)
		}
		if !got.Sound.StopPreviousSound {
			t.Error("StopPreviousSound was not read back")
		}
		if got.Sound.StartSoundName != "" {
			t.Errorf("invented a start sound name: %q", got.Sound.StartSoundName)
		}
	})
}
