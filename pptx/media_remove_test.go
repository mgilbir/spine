package pptx

import (
	"regexp"
	"strings"
	"testing"
)

// loadedDeckSlide returns an opened one-slide deck (the surgical sync path).
func loadedDeck(t *testing.T) *Presentation {
	t.Helper()
	return openBytes(t, savedDeck(t))
}

// C191: removing an auto-play video from a loaded slide must drop the
// generated p:timing tree targeting its spid and the media relationships only
// it referenced.
func TestLoadedSlide_RemoveAutoplayVideo_PrunesTimingAndRels(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]
	v := s.AddVideo([]byte("vid-one"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)
	v.SetName("Vid1")
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data1, "ppt/slides/slide1.xml")), "<p:timing>") {
		t.Fatal("setup: no timing tree after adding autoplay video")
	}

	s.RemoveShape(v)
	data2, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data2, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, "<p:timing>") {
		t.Errorf("timing tree survives removal of its only media\n%s", xml)
	}
	rels := string(zipPart(t, data2, "ppt/slides/_rels/slide1.xml.rels"))
	if strings.Contains(rels, "media") {
		t.Errorf("media relationships linger after the media shape was removed\n%s", rels)
	}
	// No relationship id referenced by the slide may be missing from the rels
	// part, and vice versa no media rel may be unreferenced.
	for _, m := range regexp.MustCompile(`r:(?:embed|link)="([^"]+)"`).FindAllStringSubmatch(xml, -1) {
		if !strings.Contains(rels, `Id="`+m[1]+`"`) {
			t.Errorf("slide references %s but the rels part lacks it", m[1])
		}
	}
}

// C191: with two auto-play videos, removing one keeps the other's timing and
// relationships intact.
func TestLoadedSlide_RemoveOneOfTwoVideos_KeepsOther(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]
	v1 := s.AddVideo([]byte("vid-one"), "video/mp4")
	v1.SetPlayMode(PlayAutomatically)
	v1.SetName("Vid1")
	v2 := s.AddVideo([]byte("vid-two"), "video/mp4")
	v2.SetPlayMode(PlayAutomatically)
	v2.SetName("Vid2")
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}

	s.RemoveShape(v1)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "<p:timing>") {
		t.Fatal("surviving video lost its timing tree")
	}
	keptID := shapeIDByName(t, xml, "Vid2")
	for _, m := range spidRE.FindAllStringSubmatch(xml, -1) {
		if m[1] != keptID {
			t.Errorf("timing targets spid %s; surviving video id is %s", m[1], keptID)
		}
	}
	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	if !strings.Contains(rels, "media2") {
		t.Errorf("surviving video's media relationship lost\n%s", rels)
	}
	if strings.Contains(rels, "media1.") {
		t.Errorf("removed video's media relationship lingers\n%s", rels)
	}
	for _, m := range regexp.MustCompile(`r:(?:embed|link)="([^"]+)"`).FindAllStringSubmatch(xml, -1) {
		if !strings.Contains(rels, `Id="`+m[1]+`"`) {
			t.Errorf("slide references %s but the rels part lacks it", m[1])
		}
	}
}

// C191 multi-cycle: remove -> save -> remove -> save stays consistent.
func TestLoadedSlide_RemoveVideosAcrossCycles(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]
	v1 := s.AddVideo([]byte("vid-one"), "video/mp4")
	v1.SetPlayMode(PlayAutomatically)
	v1.SetName("Vid1")
	v2 := s.AddVideo([]byte("vid-two"), "video/mp4")
	v2.SetPlayMode(PlayAutomatically)
	v2.SetName("Vid2")
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}

	s.RemoveShape(v1)
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}
	s.RemoveShape(v2)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, "<p:timing>") {
		t.Error("timing tree survives removal of all media")
	}
	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	if strings.Contains(rels, "media") {
		t.Errorf("media relationships linger after all media removed\n%s", rels)
	}
}

// C192: a video added to a loaded slide without SetSize gets the 4x3 default
// in the XML; the domain shape must carry the same size, so a later
// SetName/SetPosition flush does not collapse the frame to 0x0.
func TestLoadedSlide_MediaDefaultSizeSurvivesLaterFlush(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]
	v := s.AddVideo([]byte("vid-one"), "video/mp4")
	v.SetName("Vid1")
	// no SetSize
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}
	if w, h := v.Size(); w != defaultMediaWidth || h != defaultMediaHeight {
		t.Errorf("domain size after save = %dx%d, want the %dx%d default", w, h, defaultMediaWidth, defaultMediaHeight)
	}

	v.SetPosition(914400, 914400)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	pic := xml[strings.Index(xml, "<p:pic>"):]
	if strings.Contains(pic, `<a:ext cx="0" cy="0"/>`) {
		t.Errorf("media frame collapsed to 0x0 after SetPosition flush\n%s", pic[:strings.Index(pic, "</p:pic>")])
	}
	if !strings.Contains(pic, `<a:ext cx="3657600" cy="2743200"/>`) {
		t.Errorf("media frame does not keep the 4x3 default size\n%s", pic[:strings.Index(pic, "</p:pic>")])
	}
}
