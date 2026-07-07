package pptx

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

var slideRelRefRE = regexp.MustCompile(`r:(?:embed|link|id)="(rId\d+)"`)

// assertSlideRefsResolve fails the test when the slide part references a
// relationship ID that its .rels file does not declare.
func assertSlideRefsResolve(t *testing.T, data []byte, slideName string) {
	t.Helper()
	slideXML := string(zipPart(t, data, slideName))
	relsName := strings.Replace(slideName, "slides/", "slides/_rels/", 1) + ".rels"
	rels := string(zipPart(t, data, relsName))
	for _, m := range slideRelRefRE.FindAllStringSubmatch(slideXML, -1) {
		if !strings.Contains(rels, `Id="`+m[1]+`"`) {
			t.Errorf("%s references %s which is missing from %s\n%s", slideName, m[1], relsName, rels)
		}
	}
}

// C151: media relationships are keyed by slide part name, and saveNew used to
// reassign part names by presentation index — after a MoveSlide the media rels
// attached to the wrong slide, leaving the media slide's p:pic pointing at
// r:ids absent from its .rels. Part names are stable now: the rels must follow
// the slide with the video across save → move → save → reopen cycles.
func TestMediaRels_FollowSlideAcrossMove(t *testing.T) {
	p := Create()
	s1 := p.AddSlide()
	s1.AddTextBox().TextFrame().SetText("HasVideo")
	s1.AddVideo([]byte("fake-mp4-bytes"), "video/mp4")
	s2 := p.AddSlide()
	s2.AddTextBox().TextFrame().SetText("Plain")

	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}
	if err := p.MoveSlide(0, 1); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// Locate the slide part that carries the video after the move.
	slideParts := []string{"ppt/slides/slide1.xml", "ppt/slides/slide2.xml"}
	mediaSlide, plainSlide := "", ""
	for _, name := range slideParts {
		if strings.Contains(string(zipPart(t, data, name)), "videoFile") {
			mediaSlide = name
		} else {
			plainSlide = name
		}
	}
	if mediaSlide == "" || plainSlide == "" {
		t.Fatalf("expected exactly one slide with a video, got mediaSlide=%q plainSlide=%q", mediaSlide, plainSlide)
	}

	// Every r:id used on each slide must resolve in that slide's rels, and the
	// media relationships must sit on the slide that shows the video.
	for _, name := range slideParts {
		assertSlideRefsResolve(t, data, name)
	}
	mediaRels := string(zipPart(t, data, strings.Replace(mediaSlide, "slides/", "slides/_rels/", 1)+".rels"))
	if !strings.Contains(mediaRels, "office/2007/relationships/media") {
		t.Errorf("media relationship missing from %s rels\n%s", mediaSlide, mediaRels)
	}
	plainRels := string(zipPart(t, data, strings.Replace(plainSlide, "slides/", "slides/_rels/", 1)+".rels"))
	if strings.Contains(plainRels, "office/2007/relationships/media") {
		t.Errorf("media relationship leaked onto %s rels\n%s", plainSlide, plainRels)
	}

	// The move must be reflected in presentation order: the reopened deck shows
	// the plain slide first, then the video slide — and a further re-save keeps
	// every reference resolvable.
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("saved deck does not reopen: %v", err)
	}
	first := p2.Slides()[0]
	for _, shape := range first.Shapes() {
		if shape.ShapeType() == ShapeTypeVideo {
			t.Error("video slide is still first after MoveSlide")
		}
	}
	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range slideParts {
		assertSlideRefsResolve(t, data2, name)
	}
}

// C151: media relationships survive removing an unrelated slide before the
// media slide, across two save cycles.
func TestMediaRels_SurviveRemoveSlide(t *testing.T) {
	p := Create()
	p.AddSlide().AddTextBox().TextFrame().SetText("doomed")
	s2 := p.AddSlide()
	s2.AddVideo([]byte("fake-mp4-bytes"), "video/mp4")

	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveSlide(0); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	names := zipNames(t, data)
	var mediaSlide string
	for name := range names {
		if strings.HasPrefix(name, "ppt/slides/slide") && !strings.Contains(name, "_rels") {
			if strings.Contains(string(zipPart(t, data, name)), "videoFile") {
				mediaSlide = name
			}
		}
	}
	if mediaSlide == "" {
		t.Fatal("surviving slide lost its video")
	}
	assertSlideRefsResolve(t, data, mediaSlide)
	rels := string(zipPart(t, data, strings.Replace(mediaSlide, "slides/", "slides/_rels/", 1)+".rels"))
	if !strings.Contains(rels, "office/2007/relationships/media") {
		t.Errorf("media relationship missing after RemoveSlide\n%s", rels)
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("saved deck does not reopen: %v", err)
	}
}

// C152: ReplaceText on a created deck triggers a shape sync before the first
// save. Media relationships created during that sync used to be stored under
// the empty part name (slides only got a part name at save time) and were
// never written, leaving the p:pic referencing r:ids absent from the rels.
func TestMediaRels_ReplaceTextBeforeFirstSave(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("{{a}}")
	s.AddVideo([]byte("vid-a"), "video/mp4")
	s.ReplaceText(map[string]string{"{{a}}": "x"})

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	assertSlideRefsResolve(t, data, "ppt/slides/slide1.xml")
	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	if !strings.Contains(rels, "office/2007/relationships/media") {
		t.Errorf("media relationship missing after pre-save ReplaceText\n%s", rels)
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("saved deck does not reopen: %v", err)
	}
}

// C152: Duplicate before the first save also triggers an early sync; both the
// original and the duplicate must carry resolvable media relationships.
func TestMediaRels_DuplicateBeforeFirstSave(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddVideo([]byte("vid-b"), "video/mp4")
	s.Duplicate()

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ppt/slides/slide1.xml", "ppt/slides/slide2.xml"} {
		assertSlideRefsResolve(t, data, name)
		rels := string(zipPart(t, data, strings.Replace(name, "slides/", "slides/_rels/", 1)+".rels"))
		if !strings.Contains(rels, "office/2007/relationships/media") {
			t.Errorf("media relationship missing from %s rels\n%s", name, rels)
		}
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("saved deck does not reopen: %v", err)
	}
}
