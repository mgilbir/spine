package pptx

import (
	"fmt"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
)

// TestAddSlideKeepsNameCacheWarm guards the linearity. Allocating a slide part
// name must not rebuild the set of taken names per slide: that rebuild walks
// every slide and every other part, which is the O(slides) cost the cache
// removes, and it would hand out every correct name while doing it.
func TestAddSlideKeepsNameCacheWarm(t *testing.T) {
	const slides = 400

	p := Create()
	before := p.partNameRebuilds
	for i := 0; i < slides; i++ {
		p.AddSlide()
	}
	if got := p.partNameRebuilds - before; got > 2 {
		t.Errorf("part name cache rebuilt %d times while adding %d slides (want <= 2)", got, slides)
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

// TestSetNotesResolvesNotesMasterOnce guards the cost that actually dominated
// SetNotes. notesMasterPartName walks every other part, and SetNotes adds one
// per slide, so running it per call made giving every slide speaker notes
// quadratic — 3200 slides took 2.14s while returning the right answer
// throughout.
func TestSetNotesResolvesNotesMasterOnce(t *testing.T) {
	const slides = 400

	p := Create()
	for i := 0; i < slides; i++ {
		s := p.AddSlide()
		s.SetNotes(fmt.Sprintf("notes %d", i))
	}
	if p.notesMasterResolves > 2 {
		t.Errorf("notes master resolved %d times across %d SetNotes calls (want <= 2)",
			p.notesMasterResolves, slides)
	}
	if p.partNameRebuilds > 4 {
		t.Errorf("part name caches rebuilt %d times across %d slides with notes (want <= 4)",
			p.partNameRebuilds, slides)
	}
	// Every slide must have kept its own notes.
	for _, probe := range []int{0, slides / 2, slides - 1} {
		if got, want := p.slides[probe].Notes(), fmt.Sprintf("notes %d", probe); got != want {
			t.Errorf("slide %d notes = %q, want %q", probe, got, want)
		}
	}
}

// TestNotesSlideNamesAreUniqueAndDense checks the notes allocator hands out
// distinct names. Two notes slides sharing a part makes the package unopenable.
func TestNotesSlideNamesAreUniqueAndDense(t *testing.T) {
	const slides = 50

	p := Create()
	for i := 0; i < slides; i++ {
		p.AddSlide().SetNotes(fmt.Sprintf("n%d", i))
	}
	seen := map[string]bool{}
	for name := range p.otherParts {
		if !strings.HasPrefix(name, "/ppt/notesSlides/") {
			continue
		}
		if seen[name] {
			t.Errorf("notes part %q appears twice", name)
		}
		seen[name] = true
	}
	if len(seen) != slides {
		t.Errorf("got %d notes parts, want %d", len(seen), slides)
	}
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("save: %v", err)
	}
}

// TestNotesMasterInvalidationIsSeen covers the one path that can change the
// answer: merge adding a notes master. A cache that kept its old answer would
// point every later notes slide at a master that is no longer the first.
func TestNotesMasterInvalidationIsSeen(t *testing.T) {
	p := Create()
	p.AddSlide().SetNotes("first")
	before := p.notesMasterPartName()

	// Add several notes masters, the way a merge carrying them would, and tell
	// the cache. More than one, and spanning the existing name on both sides,
	// so that "returns the lowest" is distinguishable from "returns whichever
	// the map yielded last".
	for _, n := range []string{"0", "3", "7", "9"} {
		p.otherParts["/ppt/notesMasters/notesMaster"+n+".xml"] = &coxml.RawPart{ContentType: "application/xml"}
	}
	p.invalidateNotesMaster()

	after := p.notesMasterPartName()
	if after == before {
		t.Errorf("notes master still resolves to %q after ones sorting ahead of it were added", after)
	}
	// Recompute the expected minimum from the live parts rather than hardcoding
	// it, so the assertion stays true whatever Create() starts with.
	want := ""
	for name := range p.otherParts {
		if !strings.HasPrefix(name, "/ppt/notesMasters/") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		if want == "" || name < want {
			want = name
		}
	}
	if after != want {
		t.Errorf("notes master = %q, want the lowest-named one %q", after, want)
	}
}
