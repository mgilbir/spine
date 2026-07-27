package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// C416: an animation authored against a shape that is then removed before the
// save must not emit a p:spTgt naming it. The spid guard rejected only 0, and
// pruneAutoTiming covers the generated autoplay refs, never pendingAnims — so
// the deck shipped a timing tree targeting a shape it did not contain.
func TestAddAnimation_TargetRemovedBeforeSave_IsDropped(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	keep := s.AddTextBox()
	keep.TextFrame().SetText("keep")
	victim := s.AddTextBox()
	victim.TextFrame().SetText("victim")

	// Resolve shape ids: Shape.ID reports 0 until the deck is first saved.
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}
	victimID := victim.ID()
	if victimID == 0 {
		t.Fatal("shape id still unresolved after a save")
	}

	s.AddAnimation(victimID, EffectFadeIn, TriggerOnClick)
	s.RemoveShape(victim)

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))

	if strings.Contains(xml, "victim") {
		t.Fatalf("the shape was not actually removed:\n%s", xml)
	}
	for _, spid := range spidRE.FindAllStringSubmatch(xml, -1) {
		got, err := parseUint32(spid[1])
		if err != nil {
			t.Fatal(err)
		}
		if got == victimID {
			t.Errorf("timing still targets removed shape spid=%d:\n%s", victimID, xml)
		}
	}
}

// C416: an animation whose target survives must still be emitted — the guard
// must not drop everything.
func TestAddAnimation_SurvivingTargetStillEmitted(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	box := s.AddTextBox()
	box.TextFrame().SetText("keep")

	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}
	s.AddAnimation(box.ID(), EffectFadeIn, TriggerOnClick)

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, `presetClass="entr"`) {
		t.Errorf("the animation for a live target was dropped:\n%s", xml)
	}
}

// C520 (claim A, corpus-verified): a slide whose FIRST animation is marked
// with-previous must start when the slide appears, not on the next click. The
// builder opened every click group with a bare delay="indefinite" regardless of
// trigger.
//
// PowerPoint expresses "also start when the sequence begins" by keeping
// delay="indefinite" and adding a second condition keyed to the mainSeq's own
// time node. 799 of 799 real groups leading with a with/after-previous effect
// carry exactly this pair, and a lone delay="0" on a click group occurs zero
// times in 1200 decks — so this asserts the real form, not the audit's
// suggested one.
func TestAddAnimation_LeadingWithPrevious_StartsWithSlide(t *testing.T) {
	for _, trigger := range []AnimationTrigger{TriggerWithPrevious, TriggerAfterPrevious} {
		t.Run(trigger.String(), func(t *testing.T) {
			p := Create()
			s := p.AddSlide()
			s.AddTextBox().TextFrame().SetText("x")
			s.AddAnimation(2, EffectFadeIn, trigger)

			data, err := p.SaveBytes()
			if err != nil {
				t.Fatal(err)
			}
			xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))

			// The mainSeq's cTn id is what the onBegin condition must reference.
			const wantCond = `<p:cond evt="onBegin" delay="0"><p:tn val="2"/></p:cond>`
			if !strings.Contains(xml, wantCond) {
				t.Errorf("a leading %s effect still waits only for a click; want %s\n%s",
					trigger, wantCond, xml)
			}
			// ...alongside the indefinite, not instead of it.
			if !strings.Contains(xml, `<p:cond delay="indefinite"/>`) {
				t.Errorf("the click condition was replaced rather than supplemented:\n%s", xml)
			}
		})
	}
}

// C520: an on-click effect must keep the bare indefinite condition — real decks
// never give a click-led group the onBegin pair (0 of 6292).
func TestAddAnimation_OnClickGroup_HasNoOnBeginCondition(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("x")
	s.AddAnimation(2, EffectFadeIn, TriggerOnClick)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, `evt="onBegin"`) {
		t.Errorf("a click-triggered group gained an onBegin start condition:\n%s", xml)
	}
}

// C519: Animations returns two identical-looking handle kinds with different
// semantics — a pending one from AddAnimation whose mutators work, and a
// reconstruction from the timing tree whose mutators do not. Editable
// distinguishes them.
func TestAnimationHandle_EditableDistinguishesPendingFromParsed(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("a\nb")

	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}
	pending := s.AddAnimation(2, EffectFadeIn, TriggerOnClick)
	if !pending.Editable() {
		t.Error("a handle from AddAnimation must report Editable")
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	got := p2.Slides()[0].Animations()
	if len(got) == 0 {
		t.Fatal("the saved animation did not read back")
	}
	for _, a := range got {
		if a.Editable() {
			t.Errorf("a parsed animation handle claims to be editable: %+v", a)
		}
	}
}

// C519: AddAnimation silently drops an effect it cannot emit — notably
// EffectUnknown, which is exactly what Animations reports for an unrecognized
// preset, so echoing a read-back value loses it. Supported makes that checkable.
func TestAnimationEffect_Supported(t *testing.T) {
	if EffectUnknown.Supported() {
		t.Error("EffectUnknown must not report Supported: AddAnimation cannot emit it")
	}
	for _, e := range []AnimationEffect{
		EffectAppear, EffectFadeIn, EffectFlyIn, EffectWipe, EffectZoom,
		EffectPulse, EffectSpin, EffectGrowShrink,
		EffectDisappear, EffectFadeOut, EffectFlyOut,
	} {
		if !e.Supported() {
			t.Errorf("%s must report Supported", e)
		}
	}
}
