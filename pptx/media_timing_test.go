package pptx

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// Auto-play media builds a p:timing tree that plays each media shape when the
// slide appears, and the tree survives a round-trip.
func TestAutoPlayMedia_TimingTree(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	v := slide.AddVideo([]byte("vid"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)
	a := slide.AddAudio([]byte("aud"), "audio/mpeg")
	a.SetPlayMode(PlayAutomatically)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))

	for _, want := range []string{
		"<p:timing>",
		`nodeType="mainSeq"`,
		`presetClass="mediacall"`,
		"playFrom(0.0)",
		"<p:video>",
		"<p:audio>",
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("timing tree missing %q\n%s", want, xml)
		}
	}
	// The media nodes must target the two media shapes (ids 2 and 3).
	if !strings.Contains(xml, `<p:spTgt spid="2"/>`) || !strings.Contains(xml, `<p:spTgt spid="3"/>`) {
		t.Errorf("timing does not target both media shapes\n%s", xml)
	}

	// The deck reopens and the timing tree survives a re-save.
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("saved deck does not reopen: %v", err)
	}
	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data2, "ppt/slides/slide1.xml")), "<p:timing>") {
		t.Error("timing tree was lost on round-trip")
	}
}

// Click-to-play media (the default) emits no timing tree.
func TestPlayOnClick_NoTiming(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	slide.AddVideo([]byte("vid"), "video/mp4") // default PlayOnClick

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), "<p:timing>") {
		t.Error("click-to-play media should not emit a timing tree")
	}
}

var cNvPrIDRE = regexp.MustCompile(`<p:cNvPr id="(\d+)" name="([^"]*)"`)
var spidRE = regexp.MustCompile(`spid="(\d+)"`)

// shapeIDByName returns the cNvPr id of the named shape in slide XML.
func shapeIDByName(t *testing.T, slideXML, name string) string {
	t.Helper()
	for _, m := range cNvPrIDRE.FindAllStringSubmatch(slideXML, -1) {
		if m[2] == name {
			return m[1]
		}
	}
	t.Fatalf("shape %q not found in slide XML\n%s", name, slideXML)
	return ""
}

// C163: a full shape rebuild renumbers shape ids, but the auto-generated
// timing tree used to be kept as-is (applyMediaTiming skipped when Timing !=
// nil), leaving it targeting a nonexistent spid. The generated tree must be
// rebuilt against the new ids.
func TestAutoPlayTiming_RegeneratedAfterShapeRenumber(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	tb.TextFrame().SetText("hello")
	v := s.AddVideo([]byte("vid"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)

	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml1 := string(zipPart(t, data1, "ppt/slides/slide1.xml"))
	id1 := shapeIDByName(t, xml1, "Video")
	for _, m := range spidRE.FindAllStringSubmatch(xml1, -1) {
		if m[1] != id1 {
			t.Errorf("first save: timing targets spid %s, video id is %s", m[1], id1)
		}
	}

	// Removing the textbox forces a rebuild that renumbers the video's id.
	s.RemoveShape(tb)
	data2, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml2 := string(zipPart(t, data2, "ppt/slides/slide1.xml"))
	id2 := shapeIDByName(t, xml2, "Video")
	spids := spidRE.FindAllStringSubmatch(xml2, -1)
	if len(spids) == 0 {
		t.Fatalf("timing tree lost after rebuild\n%s", xml2)
	}
	for _, m := range spids {
		if m[1] != id2 {
			t.Errorf("after rebuild: timing targets spid %s, video id is %s\n%s", m[1], id2, xml2)
		}
	}
	if _, err := OpenReader(bytes.NewReader(data2), int64(len(data2))); err != nil {
		t.Fatalf("saved deck does not reopen: %v", err)
	}
}

// C163: auto-play media added to a created deck after its first save must
// still get a timing tree (the old code skipped building one because the
// first save had already installed a tree — even an unrelated empty one — or
// because none existed and the media came later).
func TestAutoPlayTiming_MediaAddedAfterFirstSave(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("first")
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}

	v := s.AddVideo([]byte("vid"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "<p:timing>") {
		t.Fatalf("no timing tree for media added after first save\n%s", xml)
	}
	id := shapeIDByName(t, xml, "Video")
	for _, m := range spidRE.FindAllStringSubmatch(xml, -1) {
		if m[1] != id {
			t.Errorf("timing targets spid %s, video id is %s", m[1], id)
		}
	}
}

// C163: removing the only auto-play media drops the generated timing tree
// instead of leaving one targeting a removed spid.
func TestAutoPlayTiming_DroppedWhenMediaRemoved(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	v := s.AddVideo([]byte("vid"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}

	s.RemoveShape(v)
	s.AddTextBox().TextFrame().SetText("left behind")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, "<p:timing>") {
		t.Errorf("timing tree kept after its media was removed\n%s", xml)
	}
}

// C270: once an authored animation (AddAnimation) is appended to a
// library-generated autoplay timing tree, a later autoplay rebuild — adding
// another auto-play medium and re-saving, which regenerates the tree from
// mediacall nodes only — must not replace the tree and drop the animation.
func TestAutoPlayTiming_AddedAnimationSurvivesRebuild(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("title") // shape id 2
	v1 := s.AddVideo([]byte("vid1"), "video/mp4")
	v1.SetPlayMode(PlayAutomatically)
	v2 := s.AddVideo([]byte("vid2"), "video/mp4")
	v2.SetPlayMode(PlayAutomatically)
	s.AddAnimation(2, EffectFadeIn, TriggerOnClick)

	// First save: the generated tree gets two mediacalls, then the entrance
	// animation is appended (and the tree frozen).
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}

	// A second autoplay medium then a re-save used to regenerate the whole
	// timing tree, dropping the animation appended above.
	v3 := s.AddVideo([]byte("vid3"), "video/mp4")
	v3.SetPlayMode(PlayAutomatically)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, `presetClass="entr"`) {
		t.Errorf("entrance animation dropped by autoplay rebuild:\n%s", xml)
	}
	if n := strings.Count(xml, `presetClass="mediacall"`); n != 2 {
		t.Errorf("mediacall count = %d, want 2 (the media present when the tree froze)", n)
	}
}

// C163 guard: a timing tree parsed from an opened file is never regenerated or
// clobbered, even when new auto-play media is added to that slide.
func TestAutoPlayTiming_ParsedTimingNotClobbered(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	v := s.AddVideo([]byte("vid"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	timingRE := regexp.MustCompile(`<p:timing>.*</p:timing>`)
	timing1 := timingRE.FindString(string(zipPart(t, data1, "ppt/slides/slide1.xml")))
	if timing1 == "" {
		t.Fatal("first save produced no timing tree")
	}

	p2, err := OpenReader(bytes.NewReader(data1), int64(len(data1)))
	if err != nil {
		t.Fatal(err)
	}
	s2 := p2.Slides()[0]
	v2 := s2.AddVideo([]byte("vid-two"), "video/mp4")
	v2.SetPlayMode(PlayAutomatically)
	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml2 := string(zipPart(t, data2, "ppt/slides/slide1.xml"))
	if got := timingRE.FindString(xml2); got != timing1 {
		t.Errorf("parsed timing tree was modified:\nbefore: %s\nafter:  %s", timing1, got)
	}
}

// C193: duplicating a slide with auto-play media before the first save must
// give BOTH slides a valid timing tree targeting their own shape ids (timing
// used to be built only at save, after Duplicate snapshotted the XML).
func TestAutoPlayTiming_SurvivesDuplicateBeforeSave(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	v := s.AddVideo([]byte("vid"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)
	v.SetName("Vid1")
	s.Duplicate()

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"ppt/slides/slide1.xml", "ppt/slides/slide2.xml"} {
		xml := string(zipPart(t, data, part))
		if !strings.Contains(xml, "<p:timing>") {
			t.Errorf("%s has no timing tree", part)
			continue
		}
		id := shapeIDByName(t, xml, "Vid1")
		for _, m := range spidRE.FindAllStringSubmatch(xml, -1) {
			if m[1] != id {
				t.Errorf("%s: timing targets spid %s, media id is %s", part, m[1], id)
			}
		}
	}
}
