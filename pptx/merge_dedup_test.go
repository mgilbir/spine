package pptx

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// Merging decks that share a template, and slide-jump links inside groups.
//
// Both paths were never executed by any test. That matters more here than
// elsewhere: merge is where this package's worst defects have been — duplicate
// relationship ids (C236), master and layout image relationships dropped
// (C237), a slide id bound to the wrong master (C363) — and both paths are on
// the ordinary route through it.
//
// Neither is reachable from a deck built by Create alone. A created deck's
// slides carry no modeled layout, so importLayout returns before it can look
// for an equivalent one; the layout wiring only exists after a save and reopen.
// That is why merging two Create() decks — which several tests do — left the
// deduplication path dark, and why these tests round-trip first.

func deckWithText(t *testing.T, text string) *Presentation {
	t.Helper()
	p := Create()
	s := p.AddSlide()
	tb := NewTextBox()
	tb.SetText(text)
	if err := s.AddShape(tb); err != nil {
		t.Fatalf("AddShape: %v", err)
	}
	return p
}

// A merge between decks built from the same template reuses the destination's
// master and the layouts under it, rather than carrying in a second copy of
// furniture that is byte-for-byte the same.
//
// The cost of getting this wrong is not cosmetic: a duplicated master brings a
// duplicated layout set and theme with it, so a deck merged from four sources
// of the same template carries four identical masters, and PowerPoint shows
// four copies of every layout in its gallery.
func TestMergeReusesAnIdenticalMasterAndItsLayouts(t *testing.T) {
	dst := saveReopen(t, deckWithText(t, "destination"))
	src := saveReopen(t, deckWithText(t, "source"))

	mastersBefore := len(dst.slideMasters)
	layoutsBefore := len(dst.slideMasters[0].layouts)

	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}

	if got := len(dst.slideMasters); got != mastersBefore {
		t.Errorf("masters after merging an identical template = %d, want %d — the master was duplicated", got, mastersBefore)
	}
	if got := len(dst.slideMasters[0].layouts); got != layoutsBefore {
		t.Errorf("layouts under the master = %d, want %d — the layout set was duplicated", got, layoutsBefore)
	}
	if got := len(dst.Slides()); got != 2 {
		t.Fatalf("slides after merge = %d, want 2", got)
	}

	// The reuse has to survive the save, which is where the parts are actually
	// written: counting in memory alone would pass a merge that reused the
	// model and then wrote both copies out.
	out, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	names := unzipAll(t, out)
	if n := countPrefix(names, "ppt/slideMasters/slideMaster"); n != 1 {
		t.Errorf("saved package holds %d slide masters, want 1", n)
	}
	if n := countPrefix(names, "ppt/slideLayouts/slideLayout"); n != layoutsBefore {
		t.Errorf("saved package holds %d layouts, want %d", n, layoutsBefore)
	}
	if n := countPrefix(names, "ppt/theme/theme"); n != 1 {
		t.Errorf("saved package holds %d themes, want 1", n)
	}

	// And the merged slide has to still resolve to a layout under that master,
	// not to nothing: a dedup that dropped the wiring reads as a slide with no
	// layout, which PowerPoint renders with default furniture.
	back, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen after merge: %v", err)
	}
	for i, s := range back.Slides() {
		if s.layout == nil {
			t.Errorf("slide %d has no layout after the merge", i+1)
			continue
		}
		if s.layout.master == nil {
			t.Errorf("slide %d's layout belongs to no master", i+1)
		}
	}
	if got := textOfSlides(back); !strings.Contains(got, "destination") || !strings.Contains(got, "source") {
		t.Errorf("merged deck lost text: %q", got)
	}
}

// A slide-jump hyperlink whose target slide is not carried over must be
// cleared, including when it sits inside a group shape.
//
// A dangling hlinksldjump is not inert: PowerPoint reports the file as needing
// repair. The shape tree walk clears them at the top level, and the group walk
// that mirrors it was never exercised — the case where the link is one level
// down is exactly the one a hand-written walker forgets.
func TestMergeClearsSlideJumpInsideAGroup(t *testing.T) {
	src := Create()
	first := src.AddSlide()
	target := src.AddSlide() // the slide the link points at
	tb := NewTextBox()
	tb.SetText("target slide")
	if err := target.AddShape(tb); err != nil {
		t.Fatal(err)
	}

	group := NewGroupShape()
	linked := NewAutoShape("rect")
	linked.SetHyperlinkToSlide(1) // jump to the second slide, by index
	if err := group.AddChild(linked); err != nil {
		t.Fatalf("AddChild: %v", err)
	}
	if err := first.AddShape(group); err != nil {
		t.Fatalf("AddShape(group): %v", err)
	}

	// The jump has to be there before the merge, or the assertion below passes
	// for the wrong reason. It is read from the saved package rather than from
	// the in-memory model, which is where a reader would find it.
	srcBytes, err := src.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	srcSlide := string(unzipAll(t, srcBytes)["ppt/slides/slide1.xml"])
	if !strings.Contains(srcSlide, "hlinksldjump") {
		t.Fatalf("the source slide carries no slide jump to strip:\n%s", srcSlide)
	}
	if !strings.Contains(srcSlide, "<p:grpSp>") {
		t.Fatalf("the jump is not inside a group, so this test would not reach the group walk:\n%s", srcSlide)
	}

	// Reopen and drop the target, so the jump has nowhere to land.
	src = saveReopen(t, src)
	if err := src.RemoveSlide(1); err != nil {
		t.Fatalf("RemoveSlide: %v", err)
	}

	dst := Create()
	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	out, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	for name, part := range unzipAll(t, out) {
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		if strings.Contains(string(part), "hlinksldjump") {
			t.Errorf("a slide jump inside a group survived a merge that left its target behind:\n%s\n%s",
				name, part)
		}
	}
	// And nothing may point at a slide relationship that is not there: a
	// dangling r:id makes PowerPoint offer to repair the file.
	assertNoDanglingRels(t, out)
}

// assertNoDanglingRels checks every r:id a slide names against its own
// relationship part.
func assertNoDanglingRels(t *testing.T, pkg []byte) {
	t.Helper()
	parts := unzipAll(t, pkg)
	rid := regexp.MustCompile(`r:(?:id|embed|link)="([^"]+)"`)
	declared := regexp.MustCompile(`Id="([^"]+)"`)
	for name, data := range parts {
		if !strings.HasPrefix(name, "ppt/slides/slide") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		relName := "ppt/slides/_rels/" + strings.TrimPrefix(name, "ppt/slides/") + ".rels"
		have := map[string]bool{}
		for _, m := range declared.FindAllStringSubmatch(string(parts[relName]), -1) {
			have[m[1]] = true
		}
		for _, m := range rid.FindAllStringSubmatch(string(data), -1) {
			if m[1] != "" && !have[m[1]] {
				t.Errorf("%s references %s, which %s does not declare", name, m[1], relName)
			}
		}
	}
}

// countPrefix counts the saved parts whose name starts with prefix.
func countPrefix(parts map[string][]byte, prefix string) int {
	n := 0
	for name := range parts {
		if strings.HasPrefix(name, prefix) {
			n++
		}
	}
	return n
}

func textOfSlides(p *Presentation) string {
	var b strings.Builder
	for i := range p.Slides() {
		s, err := p.Slide(i)
		if err != nil {
			continue
		}
		for _, sh := range s.Shapes() {
			if tb, ok := sh.(*TextBox); ok {
				b.WriteString(tb.Text())
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// findEquivalentLayout has to return the layout that matches byte for byte, not
// merely one that looks similar or the first under the master.
//
// The integration test above cannot tell: when a master is deduplicated, the
// caller falls back to matching a layout by type, and for a template where each
// type appears once the two agree on every deck a test can build with the public
// API. The difference shows on a real template with two layouts of the same
// type, where matching by type picks whichever comes first — so the contract is
// pinned here, directly.
func TestFindEquivalentLayoutMatchesTheIdenticalOne(t *testing.T) {
	deck := saveReopen(t, deckWithText(t, "x"))
	master := deck.slideMasters[0]
	if len(master.layouts) < 3 {
		t.Fatalf("the template has %d layouts; this test needs several to tell them apart", len(master.layouts))
	}

	// Every layout must find itself, not its neighbour.
	for i, want := range master.layouts {
		got := findEquivalentLayout(master, want)
		if got == nil {
			t.Errorf("layout %d (%s) does not match itself", i, want.partName)
			continue
		}
		if got != want {
			t.Errorf("layout %d (%s) matched %s instead of itself", i, want.partName, got.partName)
		}
	}

	// A layout that is not byte-identical to any of them matches none. The
	// widescreen template sizes its placeholders to a different slide, so its
	// layouts differ from the 4:3 ones throughout.
	other := saveReopen(t, func() *Presentation {
		p := CreateWidescreen()
		p.AddSlide()
		return p
	}())
	foreign := other.slideMasters[0].layouts[0]
	if got := findEquivalentLayout(master, foreign); got != nil {
		t.Errorf("a layout from a differently-sized template matched %s; the comparison is not on bytes", got.partName)
	}
}
