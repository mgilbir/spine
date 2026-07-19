package pptx

import (
	"bytes"
	"testing"
)

// sectionExtFixture is a p14:sectionLst extension as PowerPoint writes it,
// referencing the single slide (id 256) of savedDeck.
const sectionExtFixture = `<p:extLst><p:ext uri="{521415D9-36F7-43E2-AB2F-B90AF26B5E84}">` +
	`<p14:sectionLst xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">` +
	`<p14:section name="Test Section" id="{124617AE-E5F0-462F-B980-77B306D58FBF}">` +
	`<p14:sldIdLst><p14:sldId id="256"/></p14:sldIdLst>` +
	`</p14:section></p14:sectionLst></p:ext></p:extLst>`

// An existing p14:sectionLst is read into the typed model and, when left
// unmodified, round-trips byte-identically on save.
func TestSections_ExistingRoundTripByteIdentical(t *testing.T) {
	data := rewriteZipPart(t, savedDeck(t), "ppt/presentation.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:presentation>"), []byte(sectionExtFixture+"</p:presentation>"), 1)
	})

	p := openBytes(t, data)
	secs := p.Sections()
	if len(secs) != 1 {
		t.Fatalf("Sections() = %d, want 1", len(secs))
	}
	if secs[0].Name() != "Test Section" {
		t.Errorf("Name() = %q, want %q", secs[0].Name(), "Test Section")
	}
	if secs[0].ID() != "{124617AE-E5F0-462F-B980-77B306D58FBF}" {
		t.Errorf("ID() = %q", secs[0].ID())
	}
	members := secs[0].Slides()
	if len(members) != 1 || members[0].Index() != 0 {
		t.Fatalf("Slides() = %v, want the deck's only slide", members)
	}

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	presXML, ok := zipPartIfExists(t, out, "ppt/presentation.xml")
	if !ok {
		t.Fatal("presentation.xml missing from saved deck")
	}
	if !bytes.Contains(presXML, []byte(sectionExtFixture)) {
		t.Errorf("unmodified section list not replayed verbatim:\n%s", presXML)
	}
}

// A deck without sections must not gain an empty sectionLst on save.
func TestSections_NoSectionsNoSectionLst(t *testing.T) {
	p := openBytes(t, savedDeck(t))
	if secs := p.Sections(); secs != nil {
		t.Fatalf("Sections() = %v, want nil for a deck without sections", secs)
	}
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	presXML, _ := zipPartIfExists(t, out, "ppt/presentation.xml")
	if bytes.Contains(presXML, []byte("sectionLst")) {
		t.Errorf("deck without sections gained a sectionLst:\n%s", presXML)
	}
}

// Sections created and populated through the public API round-trip through a
// save/open cycle, with membership tracked by slide id.
func TestSections_CreateAssignRoundTrip(t *testing.T) {
	p := Create()
	s0 := p.AddSlide()
	s1 := p.AddSlide()
	s2 := p.AddSlide()

	secA := p.AddSection("A")
	secA.AddSlide(s0)
	secA.AddSlide(s1)
	secB := p.AddSection("B")
	secB.AddSlide(s2)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	reopened := openBytes(t, data)
	secs := reopened.Sections()
	if len(secs) != 2 {
		t.Fatalf("Sections() = %d, want 2", len(secs))
	}
	if secs[0].Name() != "A" || secs[1].Name() != "B" {
		t.Fatalf("section names = %q, %q", secs[0].Name(), secs[1].Name())
	}
	a := secs[0].Slides()
	if len(a) != 2 || a[0].Index() != 0 || a[1].Index() != 1 {
		t.Errorf("section A members = %v, want slides 0 and 1", a)
	}
	b := secs[1].Slides()
	if len(b) != 1 || b[0].Index() != 2 {
		t.Errorf("section B members = %v, want slide 2", b)
	}
}

// Assigning a slide to a section removes it from any section it was in before:
// a slide belongs to at most one section.
func TestSections_MembershipIsExclusive(t *testing.T) {
	p := Create()
	s0 := p.AddSlide()
	secA := p.AddSection("A")
	secB := p.AddSection("B")

	secA.AddSlide(s0)
	if got := len(secA.Slides()); got != 1 {
		t.Fatalf("A members = %d, want 1", got)
	}
	secB.AddSlide(s0)
	if got := len(secA.Slides()); got != 0 {
		t.Errorf("A members after move = %d, want 0 (slide moved to B)", got)
	}
	if got := len(secB.Slides()); got != 1 {
		t.Errorf("B members = %d, want 1", got)
	}
}
