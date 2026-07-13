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

// C220: SetPlayMode after the shape was synced must not be a silent no-op —
// switching to autoplay builds the timing tree on the next save, switching
// back drops it.
func TestLoadedSlide_SetPlayModeAfterSync(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]
	v := s.AddVideo([]byte("vid-one"), "video/mp4")
	v.SetName("Vid1")
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}

	v.SetPlayMode(PlayAutomatically)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "<p:timing>") {
		t.Fatal("post-sync SetPlayMode(PlayAutomatically) produced no timing tree")
	}
	id := shapeIDByName(t, xml, "Vid1")
	for _, m := range spidRE.FindAllStringSubmatch(xml, -1) {
		if m[1] != id {
			t.Errorf("timing targets spid %s, video id is %s", m[1], id)
		}
	}

	v.SetPlayMode(PlayOnClick)
	data, err = p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml = string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, "<p:timing>") {
		t.Error("post-sync SetPlayMode(PlayOnClick) left the timing tree behind")
	}
}

// C220: SetPoster after the shape was synced swaps the poster blip on the
// next save; the new rel resolves and the old placeholder rel is collected.
func TestLoadedSlide_SetPosterAfterSync(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]
	v := s.AddVideo([]byte("vid-one"), "video/mp4")
	v.SetName("Vid1")
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	embedRE := regexp.MustCompile(`<p:blipFill><a:blip r:embed="([^"]+)"`)
	oldEmbed := embedRE.FindStringSubmatch(string(zipPart(t, data1, "ppt/slides/slide1.xml")))
	if oldEmbed == nil {
		t.Fatal("setup: no poster blip after first save")
	}

	v.SetPoster([]byte("real-poster-bytes"), "image/png")
	data2, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data2, "ppt/slides/slide1.xml"))
	newEmbed := embedRE.FindStringSubmatch(xml)
	if newEmbed == nil {
		t.Fatal("no poster blip after SetPoster save")
	}
	if newEmbed[1] == oldEmbed[1] {
		t.Errorf("poster blip still %s after SetPoster", oldEmbed[1])
	}
	rels := string(zipPart(t, data2, "ppt/slides/_rels/slide1.xml.rels"))
	if !strings.Contains(rels, `Id="`+newEmbed[1]+`"`) {
		t.Error("new poster relationship missing from rels part")
	}
	if strings.Contains(rels, `Id="`+oldEmbed[1]+`"`) {
		t.Error("old poster relationship was not collected")
	}
	// The new poster part must carry the new bytes. The old placeholder
	// poster part is no longer referenced by anything, so the save-time media
	// GC (C221) may drop it — probe parts individually instead of requiring
	// both to exist.
	found := false
	for _, name := range []string{"ppt/media/image1.png", "ppt/media/image2.png"} {
		if data, ok := zipPartIfExists(t, data2, name); ok && strings.Contains(string(data), "real-poster-bytes") {
			found = true
		}
	}
	if !found {
		t.Error("new poster bytes not stored in any media part")
	}
}

// C221: removing a media shape garbage-collects its media (and poster) parts
// at save time once nothing references them anymore.
func TestRemoveVideoShape_GCsMediaParts(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]
	v := s.AddVideo([]byte("gc-video-bytes"), "video/mp4")
	v.SetName("Vid1")
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zipPartIfExists(t, data1, "ppt/media/media1.mp4"); !ok {
		t.Fatal("setup: media part missing after first save")
	}

	s.RemoveShape(v)
	data2, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zipPartIfExists(t, data2, "ppt/media/media1.mp4"); ok {
		t.Error("media part lingers after its only shape was removed")
	}
	if _, ok := zipPartIfExists(t, data2, "ppt/media/image1.png"); ok {
		t.Error("poster part lingers after its only shape was removed")
	}
}

// C221: a media part shared by shapes on two slides survives the removal of
// one of the shapes.
func TestRemoveSharedMediaShape_KeepsPart(t *testing.T) {
	p := Create()
	s1 := p.AddSlide()
	s2 := p.AddSlide()
	s1.AddVideo([]byte("shared-video-bytes"), "video/mp4")
	s2.AddVideo([]byte("shared-video-bytes"), "video/mp4")
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zipPartIfExists(t, data1, "ppt/media/media1.mp4"); !ok {
		t.Fatal("setup: shared media part missing")
	}

	// Reopen and remove the media shape from the first slide only.
	p2 := openBytes(t, data1)
	slide1 := p2.Slides()[0]
	var mediaShape Shape
	for _, sh := range slide1.Shapes() {
		if _, ok := sh.(*Picture); ok {
			mediaShape = sh
			break
		}
	}
	if mediaShape == nil {
		t.Fatal("setup: media pic not materialized on slide 1")
	}
	slide1.RemoveShape(mediaShape)

	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zipPartIfExists(t, data2, "ppt/media/media1.mp4"); !ok {
		t.Error("shared media part was garbage-collected while slide 2 still references it")
	}
	rels1 := string(zipPart(t, data2, "ppt/slides/_rels/slide1.xml.rels"))
	if strings.Contains(rels1, "media1.mp4") {
		t.Errorf("slide 1 keeps media rels for the removed shape:\n%s", rels1)
	}
	rels2 := string(zipPart(t, data2, "ppt/slides/_rels/slide2.xml.rels"))
	if !strings.Contains(rels2, "media1.mp4") {
		t.Errorf("slide 2 lost its media rels:\n%s", rels2)
	}
}

// C221: a zero-modification save never garbage-collects, even media parts
// that were already unreferenced when the package was opened.
func TestZeroModSave_KeepsUnreferencedMedia(t *testing.T) {
	deck := addZipParts(t, savedDeck(t), map[string][]byte{
		"ppt/media/orphan.png": []byte("orphan-image-bytes"),
	})

	p := openBytes(t, deck)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zipPartIfExists(t, data, "ppt/media/orphan.png"); !ok {
		t.Error("zero-modification save dropped a pre-existing unreferenced media part")
	}
}
