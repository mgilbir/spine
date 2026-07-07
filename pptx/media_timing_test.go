package pptx

import (
	"bytes"
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
