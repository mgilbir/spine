package pptx

import (
	"regexp"
	"strings"
	"testing"
)

// C379: removing a *group* that contains auto-play media must prune exactly what
// removing the same media at top level prunes — the generated timing tree
// targeting its spid, the media/video relationships, and the media part itself.
// collectRemovedPicRefs only matched oxml.ChildPic, so a removed group was never
// descended into and every one of those survived.
func TestLoadedSlide_RemoveGroupWithAutoplayVideo_PrunesTimingAndRels(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]

	g := NewGroupShape()
	g.SetName("Grp1")
	v := NewVideo([]byte("vid-one"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)
	v.SetName("Vid1")
	if err := g.AddChild(v); err != nil {
		t.Fatalf("AddChild: %v", err)
	}
	if err := s.AddShape(g); err != nil {
		t.Fatalf("AddShape: %v", err)
	}

	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml1 := string(zipPart(t, data1, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml1, "<p:timing>") {
		t.Fatalf("setup: no timing tree after adding an autoplay video inside a group\n%s", xml1)
	}
	if !strings.Contains(xml1, "<p:grpSp>") {
		t.Fatalf("setup: the group was not written\n%s", xml1)
	}

	s.RemoveShape(g)
	data2, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after RemoveShape: %v", err)
	}

	xml := string(zipPart(t, data2, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, "<p:grpSp>") {
		t.Fatalf("the group survived RemoveShape\n%s", xml)
	}
	if strings.Contains(xml, "<p:timing>") {
		t.Errorf("timing tree survives removal of the group holding its only media\n%s", xml)
	}
	rels := string(zipPart(t, data2, "ppt/slides/_rels/slide1.xml.rels"))
	if strings.Contains(rels, "media") {
		t.Errorf("media relationships linger after the group holding the media was removed\n%s", rels)
	}
	if _, ok := zipParts(t, data2)["/ppt/media/media1.mp4"]; ok {
		t.Error("/ppt/media/media1.mp4 survives as an orphan part")
	}
	for _, m := range regexp.MustCompile(`r:(?:embed|link)="([^"]+)"`).FindAllStringSubmatch(xml, -1) {
		if !strings.Contains(rels, `Id="`+m[1]+`"`) {
			t.Errorf("slide references %s but the rels part lacks it", m[1])
		}
	}
}

// C379, the ordinary-picture half: a plain image inside a removed group leaks
// its image relationship the same way.
func TestLoadedSlide_RemoveGroupWithPicture_PrunesImageRel(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]

	g := NewGroupShape()
	g.SetName("Grp1")
	pic := NewPicture()
	pic.SetImageData(createMinimalPNG(), "image/png")
	pic.SetName("Pic1")
	if err := g.AddChild(pic); err != nil {
		t.Fatalf("AddChild: %v", err)
	}
	if err := s.AddShape(g); err != nil {
		t.Fatalf("AddShape: %v", err)
	}
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	s.RemoveShape(g)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after RemoveShape: %v", err)
	}
	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	if strings.Contains(rels, "/image") {
		t.Errorf("image relationship lingers after the group holding the picture was removed\n%s", rels)
	}
}

// C379, nested: media two levels down (group inside group) must be swept too —
// the descent has to recurse, not just look one level in.
func TestLoadedSlide_RemoveNestedGroupWithAutoplayVideo_PrunesTimingAndRels(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]

	outer := NewGroupShape()
	outer.SetName("Outer")
	inner := NewGroupShape()
	inner.SetName("Inner")
	v := NewVideo([]byte("vid-one"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)
	v.SetName("Vid1")
	if err := inner.AddChild(v); err != nil {
		t.Fatalf("AddChild(video): %v", err)
	}
	if err := outer.AddChild(inner); err != nil {
		t.Fatalf("AddChild(group): %v", err)
	}
	if err := s.AddShape(outer); err != nil {
		t.Fatalf("AddShape: %v", err)
	}
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	s.RemoveShape(outer)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after RemoveShape: %v", err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, "<p:timing>") {
		t.Errorf("timing tree survives removal of the outer group\n%s", xml)
	}
	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	if strings.Contains(rels, "media") {
		t.Errorf("media relationships linger after the outer group was removed\n%s", rels)
	}
}
