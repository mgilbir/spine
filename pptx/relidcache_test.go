package pptx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// relIDsIn collects the relationship id numbers in one part's scope.
func relIDsIn(p *Presentation, partName string) []string {
	var out []string
	for _, rel := range p.relationships[partName] {
		if rel != nil {
			out = append(out, rel.ID)
		}
	}
	return out
}

func assertScopeRelIDsUnique(t *testing.T, p *Presentation, partName string) {
	t.Helper()
	seen := map[string]bool{}
	for _, id := range relIDsIn(p, partName) {
		if seen[id] {
			t.Errorf("relationship id %q appears twice in %s — the C363 defect: the save "+
				"keeps one and drops the other as a duplicate", id, partName)
		}
		seen[id] = true
	}
}

// reopened round-trips a presentation so its masters carry real part names and
// relationships, which is the shape where nextLayoutRelIDNum consults the
// master's own scope as well as its layouts.
func reopened(t *testing.T, p *Presentation) *Presentation {
	t.Helper()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return out
}

// TestAddLayoutKeepsRelIDCachesWarm guards the linearity. Both halves of the id
// allocation — the sibling layouts and the master's own relationship scope —
// grow by one per AddLayout, so a rescan per add is the quadratic this removed.
func TestAddLayoutKeepsRelIDCachesWarm(t *testing.T) {
	const layouts = 300

	p := reopened(t, Create())
	sm := p.SlideMasters()[0]
	beforeMaster := sm.layoutRelIDRescans
	beforeScope := p.relIDRescans

	for i := 0; i < layouts; i++ {
		sm.AddLayout(LayoutTitleAndContent)
	}

	if got := sm.layoutRelIDRescans - beforeMaster; got > 2 {
		t.Errorf("sibling-layout scan ran %d times across %d AddLayout calls (want <= 2)", got, layouts)
	}
	// One rescan per NEW layout part scope is expected (each layout part is a
	// fresh scope); what must not grow is rescans of the master's own scope.
	if got := p.relIDRescans - beforeScope; got > layouts+2 {
		t.Errorf("relationship scope rescanned %d times across %d AddLayout calls (want <= %d)",
			got, layouts, layouts+2)
	}
	assertScopeRelIDsUnique(t, p, sm.partName)
}

// TestAddLayoutIDsAreUniqueAndSaveable is the correctness half, in the
// direction that corrupts: an id handed out twice makes the package reference
// the wrong part.
func TestAddLayoutIDsAreUniqueAndSaveable(t *testing.T) {
	p := reopened(t, Create())
	sm := p.SlideMasters()[0]

	seen := map[string]bool{}
	for _, l := range sm.layouts {
		seen[l.relID] = true
	}
	for i := 0; i < 40; i++ {
		l := sm.AddLayout(LayoutTitleAndContent)
		if l.relID == "" {
			t.Fatalf("layout %d got an empty relationship id", i)
		}
		if seen[l.relID] {
			t.Fatalf("layout %d reused relationship id %q", i, l.relID)
		}
		seen[l.relID] = true
	}
	assertScopeRelIDsUnique(t, p, sm.partName)
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("save after adding layouts: %v", err)
	}
	// And the result must reopen with every layout still resolvable.
	again := reopened(t, p)
	if got, want := len(again.slideLayouts), len(p.slideLayouts); got != want {
		t.Errorf("reopened deck has %d layouts, want %d", got, want)
	}
}

// TestRelIDCacheSeesRelationshipsAddedDirectly is the staleness direction that
// matters. A relationship appended without going through appendRelationship
// must still raise the maximum, or the next allocation hands out an id that
// relationship already holds.
func TestRelIDCacheSeesRelationshipsAddedDirectly(t *testing.T) {
	p := reopened(t, Create())
	sm := p.SlideMasters()[0]

	// Warm the cache.
	sm.AddLayout(LayoutTitleAndContent)

	// Append a relationship with a far higher id, bypassing the helper.
	p.relationships[sm.partName] = append(p.relationships[sm.partName], &opc.Relationship{
		ID:         "rId9000",
		Type:       opc.RelTypeTheme,
		Target:     "../theme/theme1.xml",
		TargetMode: opc.TargetModeInternal,
	})

	l := sm.AddLayout(LayoutTitleAndContent)
	if l.relID == "rId9000" {
		t.Fatalf("new layout took rId9000, which a relationship added directly already holds")
	}
	if n, ok := relIDNum(l.relID); !ok || n <= 9000 {
		t.Errorf("new layout relID = %q; want a number above the 9000 already in the scope", l.relID)
	}
	assertScopeRelIDsUnique(t, p, sm.partName)
}

// TestAddLayoutIDsUniqueOnCreatedDeck is the same property on a deck that was
// never saved, which takes the saveNew path rather than the round-trip one.
//
// It does NOT isolate either cache: a created deck's master already carries a
// part name, so registerLayoutRelationships runs and both maxima are in play
// here too. The two are mutually redundant on purpose — remove either and the
// other still prevents a reused id; remove both and
// TestAddLayoutIDsUniqueAndSaveable fails. That pair is what is load-bearing,
// and no single-cache configuration exists to test.
func TestAddLayoutIDsUniqueOnCreatedDeck(t *testing.T) {
	p := Create()
	sm := p.SlideMasters()[0]

	seen := map[string]bool{}
	for _, l := range sm.layouts {
		if l.relID != "" {
			seen[l.relID] = true
		}
	}
	for i := 0; i < 50; i++ {
		l := sm.AddLayout(LayoutTitleAndContent)
		if l.relID == "" {
			t.Fatalf("layout %d got an empty relationship id", i)
		}
		if seen[l.relID] {
			t.Fatalf("layout %d reused relationship id %q on a created deck", i, l.relID)
		}
		seen[l.relID] = true
	}
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("save: %v", err)
	}
}
