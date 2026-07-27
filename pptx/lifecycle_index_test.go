package pptx

import (
	"regexp"
	"strings"
	"testing"
)

// C301: a dangling sldId entry (rel/part missing) between valid entries must not
// misalign Slide.index from its p.slides slot. Slide(1) must be the real second
// slide with Index()==1, and deleting it must remove the right slide.
func TestLoadSlides_DanglingSldIDKeepsIndexAligned(t *testing.T) {
	// Build a 3-slide deck with distinguishable names.
	build := Create()
	for _, name := range []string{"S1", "S2", "S3"} {
		build.AddSlide().SetName(name)
	}
	deck, err := build.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// Inject a bogus sldId (rId999 has no matching relationship) after the first
	// real entry, yielding sldIdLst = [s1, bogus, s2, s3].
	deck = rewriteZipPart(t, deck, "ppt/presentation.xml", func(xml []byte) []byte {
		loc := regexp.MustCompile(`<p:sldId[^>]*>`).FindIndex(xml)
		if loc == nil {
			t.Fatal("setup: no p:sldId element in presentation.xml")
		}
		bogus := []byte(`<p:sldId id="999" r:id="rId999"/>`)
		out := make([]byte, 0, len(xml)+len(bogus))
		out = append(out, xml[:loc[1]]...)
		out = append(out, bogus...)
		out = append(out, xml[loc[1]:]...)
		return out
	})

	p := openBytes(t, deck)
	defer func() { _ = p.Close() }()

	if p.SlideCount() != 3 {
		t.Fatalf("SlideCount() = %d, want 3 (bogus sldId skipped)", p.SlideCount())
	}
	s2, err := p.Slide(1)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Name(); got != "S2" {
		t.Errorf("Slide(1).Name() = %q, want S2", got)
	}
	if got := s2.Index(); got != 1 {
		t.Errorf("Slide(1).Index() = %d, want 1", got)
	}

	// Deleting the real second slide must remove S2, not a misaligned neighbor.
	if err := s2.Delete(); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range p.Slides() {
		names = append(names, s.Name())
	}
	if strings.Join(names, ",") != "S1,S3" {
		t.Errorf("after deleting Slide(1), remaining slides = %v, want [S1 S3]", names)
	}
}
