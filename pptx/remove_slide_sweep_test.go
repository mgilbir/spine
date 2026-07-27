package pptx

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/opc"
)

// deckWithSlideJump returns a saved three-slide deck whose slide 1 carries a
// text run jumping to slide 3. Saving allocates the RelTypeSlide relationship,
// so reopening the bytes gives a deck where the inbound reference is a real
// relationship on slide1.xml — the shape C364 needs.
func deckWithSlideJump(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s1 := p.AddSlide()
	p.AddSlide()
	p.AddSlide()
	run := s1.AddTextBox().TextFrame().AddParagraph().AddRun()
	run.SetText("jump")
	run.SetHyperlinkToSlide(2) // 0-based: the third slide
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return out
}

// relsTargeting returns the ids of relationships in the given .rels part whose
// target ends with suffix.
func relsTargeting(t *testing.T, relsXML []byte, suffix string) []string {
	t.Helper()
	rels, err := opc.UnmarshalRelationships(relsXML)
	if err != nil {
		t.Fatalf("UnmarshalRelationships: %v", err)
	}
	var ids []string
	for _, rel := range rels {
		if rel != nil && strings.HasSuffix(rel.Target, suffix) {
			ids = append(ids, rel.ID)
		}
	}
	return ids
}

// C364: RemoveSlide cleaned the removed slide's own edges but never the inbound
// ones, so a surviving slide kept a RelTypeSlide relationship targeting a part
// no longer in the package — OPC-invalid — plus the hlinksldjump action that
// used it.
func TestRemoveSlideStripsInboundSlideJump(t *testing.T) {
	deck := deckWithSlideJump(t)
	p := openBytes(t, deck)

	// Sanity: the jump relationship exists before the removal.
	if got := relsTargeting(t, zipPart(t, deck, "ppt/slides/_rels/slide1.xml.rels"), "slide3.xml"); len(got) != 1 {
		t.Fatalf("fixture setup failed: slide1 rels targeting slide3.xml = %v, want exactly one", got)
	}

	if err := p.RemoveSlide(2); err != nil {
		t.Fatalf("RemoveSlide: %v", err)
	}
	if rep := p.Validate(); rep.HasErrors() {
		t.Fatalf("Validate reported errors after RemoveSlide: %v", rep)
	}
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	parts := zipParts(t, out)
	if _, ok := parts["/ppt/slides/slide3.xml"]; ok {
		t.Fatal("slide3.xml is still in the package after RemoveSlide")
	}
	if ids := relsTargeting(t, parts["/ppt/slides/_rels/slide1.xml.rels"], "slide3.xml"); len(ids) != 0 {
		t.Errorf("slide1.xml.rels still targets the removed slide3.xml (%v)", ids)
	}
	if bytes.Contains(parts["/ppt/slides/slide1.xml"], []byte("hlinksldjump")) {
		t.Error("slide1.xml still carries a ppaction://hlinksldjump for the removed slide")
	}

	// The saved package must reopen and re-save without complaint.
	reopened := openBytes(t, out)
	if rep := reopened.Validate(); rep.HasErrors() {
		t.Errorf("reopened deck fails Validate: %v", rep)
	}
}

// C364, output-set Validate: the dangling inbound relationship must be visible
// to the pre-save gate. Before the fix partExists resolved against the source
// reader, where the removed part is still present, so the whole
// deletion-induced dangling class was invisible by construction (tension T-A).
func TestValidateSeesDanglingRelAfterPartRemoval(t *testing.T) {
	deck := deckWithSlideJump(t)
	p := openBytes(t, deck)

	// Remove the slide part without the reference sweep, the state the C364 bug
	// produced, and assert Validate now reports it.
	removed := p.slides[2]
	p.slides = p.slides[:2]
	delete(p.relationships, removed.partName)
	p.markPartRemoved(removed.partName)

	rep := p.Validate()
	found := false
	for _, f := range rep {
		if f.Code == validate.CodeRelTargetMissing && strings.Contains(f.Detail, "slide3.xml") {
			found = true
		}
	}
	if !found {
		t.Errorf("Validate did not report the dangling slide3.xml relationship target: %v", rep)
	}
}

// C365: RemoveSlide dropped the removed slide's presentation relationship but
// left every p:custShow entry naming it, producing an ST_RelationshipId that
// resolves to nothing — PowerPoint flags the deck for repair.
func TestRemoveSlideStripsCustomShowMembership(t *testing.T) {
	p := Create()
	p.AddSlide()
	p.AddSlide()
	p.AddSlide()
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	p2 := openBytes(t, saved)
	slides := p2.Slides()
	goneRelID := slides[2].RelID()
	keptRelID := slides[0].RelID()
	if goneRelID == "" || keptRelID == "" {
		t.Fatalf("fixture setup failed: rel ids %q %q", keptRelID, goneRelID)
	}
	p2.AddCustomShow("show", slides[0], slides[2])

	if err := p2.RemoveSlide(2); err != nil {
		t.Fatalf("RemoveSlide: %v", err)
	}

	shows := p2.CustomShows()
	if len(shows) != 1 {
		t.Fatalf("CustomShows() = %d shows, want 1", len(shows))
	}
	for _, id := range shows[0].SlideRelIDs {
		if id == goneRelID {
			t.Errorf("custom show still lists the removed slide's rel id %q (%v)", goneRelID, shows[0].SlideRelIDs)
		}
	}
	if len(shows[0].SlideRelIDs) != 1 || shows[0].SlideRelIDs[0] != keptRelID {
		t.Errorf("custom show membership = %v, want [%s]", shows[0].SlideRelIDs, keptRelID)
	}

	if rep := p2.Validate(); rep.HasErrors() {
		t.Fatalf("Validate reported errors: %v", rep)
	}
	out, err := p2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after RemoveSlide: %v", err)
	}

	pres := zipPart(t, out, "ppt/presentation.xml")
	presRels := zipPart(t, out, "ppt/_rels/presentation.xml.rels")
	ids := relIDSet(mustRels(t, presRels))
	for _, ref := range custShowRefs(t, pres) {
		if !ids[ref] {
			t.Errorf("custShow references %q, absent from presentation.xml.rels", ref)
		}
	}
}

// C365: a custom show that loses every member must not be emitted as an empty
// p:sldLst — CT_CustomShow requires at least one p:sld.
func TestRemoveSlideDropsEmptiedCustomShow(t *testing.T) {
	p := Create()
	p.AddSlide()
	p.AddSlide()
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	p2 := openBytes(t, saved)
	slides := p2.Slides()
	p2.AddCustomShow("only-slide-2", slides[1])
	p2.AddCustomShow("keeps-slide-1", slides[0])

	if err := p2.RemoveSlide(1); err != nil {
		t.Fatalf("RemoveSlide: %v", err)
	}
	shows := p2.CustomShows()
	if len(shows) != 1 || shows[0].Name != "keeps-slide-1" {
		t.Fatalf("CustomShows() = %+v, want only keeps-slide-1", shows)
	}

	out, err := p2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if bytes.Contains(zipPart(t, out, "ppt/presentation.xml"), []byte("only-slide-2")) {
		t.Error("presentation.xml still carries the emptied custom show")
	}
}

// C364, the zoom half: a Slide Zoom binds to its target by numeric p:sldId, not
// by relationship, so the relationship sweep cannot reach it and the id lives in
// preserved raw graphicData. Validate must at least report the dangling zoom.
func TestValidateReportsZoomTargetingRemovedSlide(t *testing.T) {
	base := Create()
	base.AddSlide()
	base.AddSlide()
	saved, err := base.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// Point a Slide Zoom on slide 1 at slide 2's id.
	victim := openBytes(t, saved).Slides()[1]
	frame := strings.Replace(slideZoomFrame, `sldId="257"`, fmt.Sprintf(`sldId="%d"`, victim.id), 1)
	deck := rewriteZipPart(t, saved, "ppt/slides/slide1.xml", func(x []byte) []byte {
		return bytes.Replace(x, []byte("</p:spTree>"), []byte(frame+"</p:spTree>"), 1)
	})

	p := openBytes(t, deck)
	if links := p.Slides()[0].ZoomLinks(); len(links) != 1 || links[0].TargetSlideID != victim.id {
		t.Fatalf("fixture setup failed: zoom links = %+v", links)
	}
	if rep := p.Validate(); len(rep) != 0 {
		t.Fatalf("Validate is not clean before the removal: %v", rep)
	}

	if err := p.RemoveSlide(1); err != nil {
		t.Fatalf("RemoveSlide: %v", err)
	}
	rep := p.Validate()
	found := false
	for _, f := range rep {
		if f.Code == codeZoomNoTarget {
			found = true
		}
	}
	if !found {
		t.Errorf("Validate did not report the zoom targeting the removed slide: %v", rep)
	}
	if rep.HasErrors() {
		t.Errorf("a dangling zoom must be a warning, not a save-blocking error: %v", rep.Errors())
	}
}

// mustRels parses a .rels part or fails the test.
func mustRels(t *testing.T, data []byte) []*opc.Relationship {
	t.Helper()
	rels, err := opc.UnmarshalRelationships(data)
	if err != nil {
		t.Fatalf("UnmarshalRelationships: %v", err)
	}
	return rels
}

// custShowRefs returns every r:id referenced by a p:custShow/p:sldLst/p:sld in
// presentation.xml, in document order.
func custShowRefs(t *testing.T, pres []byte) []string {
	t.Helper()
	var out []string
	rest := pres
	for {
		i := bytes.Index(rest, []byte("<p:sld r:id=\""))
		if i < 0 {
			return out
		}
		rest = rest[i+len("<p:sld r:id=\""):]
		j := bytes.IndexByte(rest, '"')
		if j < 0 {
			return out
		}
		out = append(out, string(rest[:j]))
		rest = rest[j:]
	}
}
