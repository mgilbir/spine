package pptx

import (
	"fmt"
	"testing"
)

// TestAddSlideKeepsNameCacheWarm guards the linearity. Allocating a slide part
// name must not rebuild the set of taken names per slide: that rebuild walks
// every slide and every other part, which is the O(slides) cost the cache
// removes, and it would hand out every correct name while doing it.
func TestAddSlideKeepsNameCacheWarm(t *testing.T) {
	const slides = 400

	p := Create()
	before := p.slideNameRebuilds
	for i := 0; i < slides; i++ {
		p.AddSlide()
	}
	if got := p.slideNameRebuilds - before; got > 2 {
		t.Errorf("slide name cache rebuilt %d times while adding %d slides (want <= 2)", got, slides)
	}

	seen := map[string]bool{}
	for i, s := range p.slides {
		if s.partName == "" {
			t.Fatalf("slide %d has no part name", i)
		}
		if seen[s.partName] {
			t.Fatalf("part name %q used twice", s.partName)
		}
		seen[s.partName] = true
	}
	// Names are still allocated densely from 1.
	last := p.slides[len(p.slides)-1].partName
	if want := fmt.Sprintf("/ppt/slides/slide%d.xml", len(p.slides)); last != want {
		t.Errorf("last slide part name = %q, want %q", last, want)
	}
}

// TestSlideNameReusedAfterRemove pins the allocation semantics the cache must
// not change: the name handed out is the lowest-numbered free one, so a name
// freed by removing a slide is reused rather than skipped.
func TestSlideNameReusedAfterRemove(t *testing.T) {
	p := Create()
	for i := 0; i < 3; i++ {
		p.AddSlide()
	}
	freed := p.slides[1].partName
	if err := p.RemoveSlide(1); err != nil {
		t.Fatal(err)
	}

	added := p.AddSlide()
	if added.partName != freed {
		t.Errorf("new slide took %q; the name freed by the removal (%q) was still available",
			added.partName, freed)
	}
}

// TestSlideNamesStayUniqueAcrossRemoveAndAdd is the safety direction: a name
// handed out twice makes two slides collide on one part, which fails the save.
func TestSlideNamesStayUniqueAcrossRemoveAndAdd(t *testing.T) {
	p := Create()
	for i := 0; i < 6; i++ {
		p.AddSlide()
	}
	if err := p.RemoveSlide(4); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveSlide(1); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		p.AddSlide()
	}

	seen := map[string]int{}
	for i, s := range p.slides {
		if prev, dup := seen[s.partName]; dup {
			t.Errorf("part name %q used by slides %d and %d", s.partName, prev, i)
		}
		seen[s.partName] = i
	}
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("save after remove/add churn: %v", err)
	}
}
