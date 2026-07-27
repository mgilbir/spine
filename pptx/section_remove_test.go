package pptx

import (
	"bytes"
	"testing"
)

// C307: RemoveSlide must strip the removed slide's id from every section so the
// emitted p14:sectionLst carries no dangling p14:sldId (a member id with no
// matching entry in the presentation's sldIdLst).
func TestSections_RemoveSlideDropsDanglingID(t *testing.T) {
	p := Create()
	s0 := p.AddSlide()
	s1 := p.AddSlide()

	sec := p.AddSection("A")
	sec.AddSlide(s0)
	sec.AddSlide(s1)

	if err := p.RemoveSlide(0); err != nil {
		t.Fatalf("RemoveSlide: %v", err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// The emitted section list must reference exactly one member (the surviving
	// slide), not two — a stale second p14:sldId is the dangling reference.
	pres := zipPart(t, data, "ppt/presentation.xml")
	if n := countSectionSldIds(pres); n != 1 {
		t.Fatalf("section list emitted %d p14:sldId entries, want 1 (dangling reference to removed slide)", n)
	}

	// And the surviving slide is still a member of the section on reopen.
	reopened := openBytes(t, data)
	secs := reopened.Sections()
	if len(secs) != 1 {
		t.Fatalf("Sections() = %d, want 1", len(secs))
	}
	if got := len(secs[0].Slides()); got != 1 {
		t.Fatalf("section members after remove = %d, want 1", got)
	}
}

// countSectionSldIds counts the p14:sldId member elements inside the
// p14:sectionLst of a presentation.xml part.
func countSectionSldIds(pres []byte) int {
	i := bytes.Index(pres, []byte("sectionLst"))
	if i < 0 {
		return 0
	}
	// Each member is emitted as <p14:sldId id="…"/>; the enclosing sldIdLst
	// element never matches "sldId id=".
	return bytes.Count(pres[i:], []byte("sldId id="))
}
