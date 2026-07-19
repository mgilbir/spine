package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// AddAnimation builds a valid p:timing main sequence: a tmRoot par, a mainSeq,
// and a click group holding the effect node with the effect-specific body.
func TestAddAnimation_BuildsTimingTree(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("hello")
	s.AddAnimation(2, EffectFadeIn, TriggerOnClick)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	for _, want := range []string{
		`<p:timing>`,
		`nodeType="tmRoot"`,
		`nodeType="mainSeq"`,
		`presetClass="entr"`,
		`presetID="10"`,
		`nodeType="clickEffect"`,
		`<p:animEffect transition="in" filter="fade">`,
		`<p:spTgt spid="2"/>`,
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("timing tree missing %q\n%s", want, xml)
		}
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("saved deck does not reopen: %v", err)
	}
}

// Each supported effect emits its distinguishing preset class/id and body.
func TestAddAnimation_EffectBodies(t *testing.T) {
	cases := []struct {
		effect AnimationEffect
		wants  []string
	}{
		{EffectAppear, []string{`presetClass="entr"`, `presetID="1"`, `<p:strVal val="visible"/>`}},
		{EffectFadeIn, []string{`presetClass="entr"`, `filter="fade"`, `transition="in"`}},
		{EffectFlyIn, []string{`presetClass="entr"`, `presetID="2"`, `<p:attrName>ppt_y</p:attrName>`, `1+#ppt_h/2`}},
		{EffectWipe, []string{`presetClass="entr"`, `filter="wipe(up)"`}},
		{EffectZoom, []string{`presetClass="entr"`, `filter="zoom"`}},
		{EffectPulse, []string{`presetClass="emph"`, `<p:animScale>`, `<p:by x="110000" y="110000"/>`, `autoRev="1"`}},
		{EffectSpin, []string{`presetClass="emph"`, `<p:animRot by="21600000">`, `<p:attrName>r</p:attrName>`}},
		{EffectGrowShrink, []string{`presetClass="emph"`, `<p:by x="150000" y="150000"/>`}},
		{EffectDisappear, []string{`presetClass="exit"`, `<p:strVal val="hidden"/>`}},
		{EffectFadeOut, []string{`presetClass="exit"`, `transition="out"`, `filter="fade"`}},
		{EffectFlyOut, []string{`presetClass="exit"`, `<p:attrName>ppt_y</p:attrName>`, `<p:strVal val="hidden"/>`}},
	}
	for _, tc := range cases {
		t.Run(tc.effect.String(), func(t *testing.T) {
			p := Create()
			s := p.AddSlide()
			s.AddTextBox().TextFrame().SetText("x")
			s.AddAnimation(2, tc.effect, TriggerOnClick)
			data, err := p.SaveBytes()
			if err != nil {
				t.Fatal(err)
			}
			xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
			for _, w := range tc.wants {
				if !strings.Contains(xml, w) {
					t.Errorf("%s: missing %q\n%s", tc.effect, w, xml)
				}
			}
			if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
				t.Fatalf("%s: saved deck does not reopen: %v", tc.effect, err)
			}
		})
	}
}

// on-click starts a new click group; with/after-previous join the current one,
// so three animations chained this way share one click group.
func TestAddAnimation_Triggers(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("a")
	s.AddTextBox().TextFrame().SetText("b")
	s.AddAnimation(2, EffectFadeIn, TriggerOnClick)
	s.AddAnimation(3, EffectFlyIn, TriggerAfterPrevious)
	s.AddAnimation(2, EffectSpin, TriggerWithPrevious)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if c := strings.Count(xml, `nodeType="clickEffect"`); c != 1 {
		t.Errorf("want 1 clickEffect got %d", c)
	}
	if !strings.Contains(xml, `nodeType="afterEffect"`) || !strings.Contains(xml, `nodeType="withEffect"`) {
		t.Errorf("missing after/with effect node types\n%s", xml)
	}
}

// A timing tree the caller never touches survives a reopen and re-save byte
// for byte (the critical fidelity guarantee).
func TestAddAnimation_UntouchedTimingByteIdentical(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("hi")
	s.AddAnimation(2, EffectGrowShrink, TriggerOnClick)
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	orig := zipPart(t, data1, "ppt/slides/slide1.xml")

	p2, err := OpenReader(bytes.NewReader(data1), int64(len(data1)))
	if err != nil {
		t.Fatal(err)
	}
	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if again := zipPart(t, data2, "ppt/slides/slide1.xml"); !bytes.Equal(orig, again) {
		t.Errorf("untouched timing not byte-identical\norig:  %s\nagain: %s", orig, again)
	}
}

// Adding an animation to a slide that already has a main sequence appends into
// it (one tmRoot, one mainSeq, non-colliding ids) rather than starting a new one.
func TestAddAnimation_AppendsToExistingMainSeq(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("a")
	s.AddTextBox().TextFrame().SetText("b")
	s.AddAnimation(2, EffectAppear, TriggerOnClick)
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	p2, err := OpenReader(bytes.NewReader(data1), int64(len(data1)))
	if err != nil {
		t.Fatal(err)
	}
	s2 := p2.Slides()[0]
	if got := len(s2.Animations()); got != 1 {
		t.Fatalf("want 1 existing animation, got %d", got)
	}
	s2.AddAnimation(3, EffectFadeOut, TriggerOnClick)
	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data2, "ppt/slides/slide1.xml"))
	if c := strings.Count(xml, `nodeType="tmRoot"`); c != 1 {
		t.Errorf("want single tmRoot, got %d\n%s", c, xml)
	}
	if c := strings.Count(xml, `nodeType="mainSeq"`); c != 1 {
		t.Errorf("want single mainSeq, got %d", c)
	}
	if c := strings.Count(xml, `nodeType="clickEffect"`); c != 2 {
		t.Errorf("want 2 clickEffects, got %d\n%s", c, xml)
	}
	p3, err := OpenReader(bytes.NewReader(data2), int64(len(data2)))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(p3.Slides()[0].Animations()); got != 2 {
		t.Errorf("want 2 animations after append, got %d", got)
	}
}

// Build-by-paragraph animates a multi-paragraph text one paragraph at a time:
// a bldP entry plus one clickEffect per paragraph, each targeting a pRg.
func TestAddAnimation_ByParagraph(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	tb.TextFrame().SetText("one")
	tb.TextFrame().AddParagraph().AddRun().SetText("two")
	tb.TextFrame().AddParagraph().AddRun().SetText("three")
	s.AddAnimation(2, EffectFadeIn, TriggerOnClick).SetByParagraph(true)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, `<p:bldP spid="2"`) || !strings.Contains(xml, `build="p"`) {
		t.Errorf("missing build-by-paragraph bldP\n%s", xml)
	}
	if c := strings.Count(xml, `nodeType="clickEffect"`); c != 3 {
		t.Errorf("want 3 per-paragraph clickEffects, got %d\n%s", c, xml)
	}
	for _, w := range []string{`<p:pRg st="0" end="0"/>`, `<p:pRg st="1" end="1"/>`, `<p:pRg st="2" end="2"/>`} {
		if !strings.Contains(xml, w) {
			t.Errorf("missing paragraph range %q\n%s", w, xml)
		}
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	anims := p2.Slides()[0].Animations()
	if len(anims) != 3 {
		t.Fatalf("want 3 per-paragraph animations, got %d", len(anims))
	}
	if !anims[0].ByParagraph() {
		t.Error("read-back animation should report ByParagraph")
	}
}

// Animations round-trips effect, trigger, and target for each supported effect.
func TestAnimations_ReadBack(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("a")
	s.AddTextBox().TextFrame().SetText("b")
	s.AddAnimation(2, EffectZoom, TriggerOnClick)
	s.AddAnimation(3, EffectDisappear, TriggerAfterPrevious)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	anims := p2.Slides()[0].Animations()
	if len(anims) != 2 {
		t.Fatalf("want 2 animations, got %d", len(anims))
	}
	if anims[0].Effect() != EffectZoom || anims[0].Trigger() != TriggerOnClick || anims[0].ShapeID() != 2 {
		t.Errorf("anim0 = %v/%v/%d", anims[0].Effect(), anims[0].Trigger(), anims[0].ShapeID())
	}
	if anims[1].Effect() != EffectDisappear || anims[1].Trigger() != TriggerAfterPrevious || anims[1].ShapeID() != 3 {
		t.Errorf("anim1 = %v/%v/%d", anims[1].Effect(), anims[1].Trigger(), anims[1].ShapeID())
	}
}

// Animations reflects pending, not-yet-saved animations too.
func TestAnimations_PendingBeforeSave(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("x")
	a := s.AddAnimation(2, EffectSpin, TriggerWithPrevious)
	got := s.Animations()
	if len(got) != 1 || got[0] != a {
		t.Fatalf("Animations should include the pending animation")
	}
}

// A slide with no authored animation never gains a timing tree.
func TestAddAnimation_NoneLeavesTimingAbsent(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("x")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), "<p:timing>") {
		t.Error("slide without animations should not emit a timing tree")
	}
}

// A shape loaded from a file reports its cNvPr id via ID, usable as an
// AddAnimation target.
func TestShapeID_RoundTrips(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	tb.SetName("Target")
	tb.TextFrame().SetText("x")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	shape := p2.Slides()[0].ShapeByName("Target")
	if shape == nil {
		t.Fatal("shape not found after reopen")
	}
	if id := baseShapeOf(shape).ID(); id != 2 {
		t.Errorf("want shape ID 2, got %d", id)
	}
}
