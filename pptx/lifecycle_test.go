package pptx

import (
	"bytes"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// C82: a group shape's reported position/size come from off/ext (its on-slide
// placement), not chOff/chExt (the child coordinate space).
func TestGroupShapePositionUsesOffExt(t *testing.T) {
	gs := &oxml.GroupShape{
		GrpSpPr: &oxml.GrpSpPr{Xfrm: &dml.GrpXfrm{
			Off:   &dml.OffXML{X: 100, Y: 200},
			Ext:   &dml.ExtXML{Cx: 300, Cy: 400},
			ChOff: &dml.OffXML{X: 5, Y: 6},
			ChExt: &dml.ExtXML{Cx: 7, Cy: 8},
		}},
	}
	g := oxmlGroupShapeToGoGroupShape(gs, nil)
	if g.x != 100 || g.y != 200 || g.width != 300 || g.height != 400 {
		t.Errorf("group placement = (%d,%d %dx%d), want (100,200 300x400) — read child space?",
			g.x, g.y, g.width, g.height)
	}
}

// C16: RemoveSlide followed by AddSlide on a loaded deck must save without a
// duplicate-part error.
func TestRemoveThenAddSlideOnLoadedDeckSaves(t *testing.T) {
	p := Create()
	for p.SlideCount() < 2 {
		p.AddSlide()
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.RemoveSlide(0); err != nil {
		t.Fatal(err)
	}
	loaded.AddSlide()

	out, err := loaded.SaveBytes()
	if err != nil {
		t.Fatalf("save after RemoveSlide+AddSlide failed (duplicate part?): %v", err)
	}
	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen after save: %v", err)
	}
	if reopened.SlideCount() != 2 {
		t.Errorf("slide count = %d, want 2", reopened.SlideCount())
	}
}

// C17: AddLayout assigns a relationship id and part name and registers the
// layout, so it is not emitted with an empty r:id / Relationship Id.
func TestAddLayoutAssignsRelIDAndPart(t *testing.T) {
	p := Create()
	master := p.SlideMasters()[0]
	before := len(p.slideLayouts)

	layout := master.AddLayout(LayoutBlank)
	if layout.relID == "" {
		t.Error("AddLayout left relID empty")
	}
	if layout.partName == "" {
		t.Error("AddLayout left partName empty")
	}
	if len(p.slideLayouts) != before+1 {
		t.Errorf("layout not registered with presentation: %d -> %d", before, len(p.slideLayouts))
	}
}

// C17 (residual): AddLayout on an opened deck must allocate its relationship
// id across ALL of the master's relationships, not just the sibling layouts.
// The master's rels already hold a theme rel at the next id after the layouts,
// so the sibling-only scan handed the new layout the theme's rId: the
// <p:sldLayoutId r:id> resolved to theme1.xml, no relationship was written for
// the layout, and it was silently lost on reopen.
func TestAddLayoutOnOpenedDeckSurvivesReopen(t *testing.T) {
	p, err := Open("testdata/test.pptx") // master rels: slideLayout rId1-11, theme rId12
	if err != nil {
		t.Fatal(err)
	}
	master := p.SlideMasters()[0]
	before := len(master.Layouts())

	layout := master.AddLayout(LayoutTitleAndContent)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	masterPart := strings.TrimPrefix(master.partName, "/")
	relsName := path.Dir(masterPart) + "/_rels/" + path.Base(masterPart) + ".rels"
	rels := string(zipPart(t, data, relsName))

	// No rel id may be used twice, and the new layout's id must be present as
	// a slideLayout relationship (not shadowed by the theme rel).
	ids := regexp.MustCompile(`Id="(rId\d+)"`).FindAllStringSubmatch(rels, -1)
	seen := make(map[string]bool, len(ids))
	for _, m := range ids {
		if seen[m[1]] {
			t.Errorf("duplicate relationship id %s in master rels:\n%s", m[1], rels)
		}
		seen[m[1]] = true
	}
	if !regexp.MustCompile(`Id="` + layout.relID + `" Type="[^"]*/slideLayout"`).MatchString(rels) {
		t.Errorf("new layout rel %s missing or not a slideLayout rel:\n%s", layout.relID, rels)
	}

	// Every sldLayoutId r:id in the master must resolve to a slideLayout rel.
	masterXML := string(zipPart(t, data, masterPart))
	for _, m := range regexp.MustCompile(`<p:sldLayoutId [^>]*r:id="(rId\d+)"`).FindAllStringSubmatch(masterXML, -1) {
		if !regexp.MustCompile(`Id="` + m[1] + `" Type="[^"]*/slideLayout"`).MatchString(rels) {
			t.Errorf("sldLayoutId %s does not resolve to a slideLayout relationship:\n%s", m[1], rels)
		}
	}

	// The layout part itself needs a rel back to its master.
	layoutPart := strings.TrimPrefix(layout.partName, "/")
	layoutRels := zipPart(t, data, path.Dir(layoutPart)+"/_rels/"+path.Base(layoutPart)+".rels")
	if !bytes.Contains(layoutRels, []byte("slideMaster")) {
		t.Errorf("added layout %s has no slideMaster relationship:\n%s", layout.partName, layoutRels)
	}

	// The layout must survive a reopen.
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reopened.SlideMasters()[0].Layouts()); got != before+1 {
		t.Errorf("layouts after reopen = %d, want %d (added layout lost)", got, before+1)
	}
}

// C18: a slide added to a loaded deck gets a slideLayout relationship, so the
// saved package does not trigger a repair prompt.
func TestAddSlideOnLoadedDeckGetsLayoutRel(t *testing.T) {
	p := Create()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	newSlide := loaded.AddSlide()

	out, err := loaded.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	pn := strings.TrimPrefix(newSlide.partName, "/")
	relsName := path.Dir(pn) + "/_rels/" + path.Base(pn) + ".rels"
	rels := zipPart(t, out, relsName)
	if !bytes.Contains(rels, []byte("slideLayout")) {
		t.Errorf("added slide %s has no slideLayout relationship:\n%s", newSlide.partName, rels)
	}
}
