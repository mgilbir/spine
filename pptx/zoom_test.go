package pptx

import (
	"bytes"
	"testing"
)

// Zoom graphicFrame fragments in the byte form the marshaler re-emits (empty
// children self-closed, graphicData inner XML preserved verbatim). Each is a
// p:graphicFrame carrying a slide, section, or summary zoom.
const (
	slideZoomFrame = `<p:graphicFrame>` +
		`<p:nvGraphicFramePr><p:cNvPr id="10" name="Slide Zoom 9"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		`<p:xfrm><a:off x="1000000" y="1000000"/><a:ext cx="2000000" cy="1125000"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/powerpoint/2016/slidezoom">` +
		`<p14:sldZm xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2016/slidezoom"><p14:sldZmObj sldId="257"><p14:zmPr/></p14:sldZmObj></p14:sldZm>` +
		`</a:graphicData></a:graphic></p:graphicFrame>`

	sectionZoomFrame = `<p:graphicFrame>` +
		`<p:nvGraphicFramePr><p:cNvPr id="11" name="Section Zoom 10"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		`<p:xfrm><a:off x="3200000" y="1000000"/><a:ext cx="2000000" cy="1125000"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/powerpoint/2016/sectionzoom">` +
		`<p14:sectionZm xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2016/sectionzoom"><p14:sectionZmObj sectionId="{6E5F1A2B-0000-4000-8000-000000000001}"><p14:zmPr/></p14:sectionZmObj></p14:sectionZm>` +
		`</a:graphicData></a:graphic></p:graphicFrame>`

	summaryZoomFrame = `<p:graphicFrame>` +
		`<p:nvGraphicFramePr><p:cNvPr id="12" name="Summary Zoom 11"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		`<p:xfrm><a:off x="1000000" y="3000000"/><a:ext cx="4200000" cy="2362500"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/powerpoint/2016/summaryzoom">` +
		`<p14:summaryZm xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2016/summaryzoom">` +
		`<p14:summaryZmObj sectionId="{AAAA1111-0000-4000-8000-000000000002}"><p14:zmPr/></p14:summaryZmObj>` +
		`<p14:summaryZmObj sectionId="{BBBB2222-0000-4000-8000-000000000003}"><p14:zmPr/></p14:summaryZmObj>` +
		`<p14:gridLayout/></p14:summaryZm>` +
		`</a:graphicData></a:graphic></p:graphicFrame>`
)

// deckWithZooms builds a created deck and splices the given zoom graphicFrames
// into slide1's spTree.
func deckWithZooms(t *testing.T, frames ...string) []byte {
	t.Helper()
	p := Create()
	p.AddSlide()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, f := range frames {
		joined += f
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		if !bytes.Contains(xml, []byte("</p:spTree>")) {
			t.Fatal("slide1.xml has no spTree close tag")
		}
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(joined+"</p:spTree>"), 1)
	})
}

// TestZoomLinksReads confirms ZoomLinks enumerates each zoom kind and reports
// its target slide id or section ids.
func TestZoomLinksReads(t *testing.T) {
	deck := deckWithZooms(t, slideZoomFrame, sectionZoomFrame, summaryZoomFrame)

	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	links := p.Slides()[0].ZoomLinks()
	if len(links) != 3 {
		t.Fatalf("ZoomLinks returned %d links, want 3: %+v", len(links), links)
	}

	slide := links[0]
	if slide.Kind != ZoomKindSlide || slide.TargetSlideID != 257 {
		t.Errorf("slide zoom = %+v, want Kind=slide TargetSlideID=257", slide)
	}
	if slide.ShapeID != 10 || slide.ShapeName != "Slide Zoom 9" || slide.SourceSlideIndex != 0 {
		t.Errorf("slide zoom shape metadata = %+v", slide)
	}

	section := links[1]
	if section.Kind != ZoomKindSection ||
		len(section.TargetSectionIDs) != 1 ||
		section.TargetSectionIDs[0] != "{6E5F1A2B-0000-4000-8000-000000000001}" {
		t.Errorf("section zoom = %+v, want one section id", section)
	}

	summary := links[2]
	if summary.Kind != ZoomKindSummary || len(summary.TargetSectionIDs) != 2 {
		t.Fatalf("summary zoom = %+v, want two section ids", summary)
	}
	if summary.TargetSectionIDs[0] != "{AAAA1111-0000-4000-8000-000000000002}" ||
		summary.TargetSectionIDs[1] != "{BBBB2222-0000-4000-8000-000000000003}" {
		t.Errorf("summary zoom section ids = %v", summary.TargetSectionIDs)
	}

	// Presentation-level accessor aggregates the same links.
	if all := p.ZoomLinks(); len(all) != 3 {
		t.Errorf("Presentation.ZoomLinks returned %d, want 3", len(all))
	}
}

// TestZoomLinksPreservesBytes confirms that reading the zoom links does not
// perturb the lazy-slide passthrough: the slide part is written back exactly as
// it was, whether or not ZoomLinks was called.
func TestZoomLinksPreservesBytes(t *testing.T) {
	deck := deckWithZooms(t, slideZoomFrame, sectionZoomFrame, summaryZoomFrame)
	original := zipPart(t, deck, "ppt/slides/slide1.xml")

	// Passthrough: open and save without touching the slide.
	pPass, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	savedPass, err := pPass.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	passSlide := zipPart(t, savedPass, "ppt/slides/slide1.xml")
	if !bytes.Equal(original, passSlide) {
		t.Fatalf("passthrough did not preserve slide bytes:\n orig: %s\n got:  %s", original, passSlide)
	}

	// Read: open, call ZoomLinks (materializes the slide), then save.
	pRead, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	if got := pRead.Slides()[0].ZoomLinks(); len(got) != 3 {
		t.Fatalf("ZoomLinks returned %d, want 3", len(got))
	}
	savedRead, err := pRead.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	readSlide := zipPart(t, savedRead, "ppt/slides/slide1.xml")
	if !bytes.Equal(original, readSlide) {
		t.Fatalf("ZoomLinks perturbed the slide round-trip:\n orig: %s\n got:  %s", original, readSlide)
	}
}
