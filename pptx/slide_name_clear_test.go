package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// deckWithNamedSlide builds a deck whose slide's p:cSld carries an explicit
// name attribute, the form PowerPoint writes for a named slide.
func deckWithNamedSlide(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s := p.AddSlide()
	s.SetName("Introduction")
	s.AddTextBox().TextFrame().SetText("body")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), `name="Introduction"`) {
		t.Fatal("fixture did not get a cSld name")
	}
	return data
}

// C584: CommonSlideData.Name is omitempty, so SetName("") wrote a zero that was
// suppressed and then replaced by the source's name on replay — the setter was
// a silent no-op on any parsed slide.
func TestSlide_ClearingName_ReachesTheXML(t *testing.T) {
	deck := deckWithNamedSlide(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Slides()[0]
	if got := s.Name(); got != "Introduction" {
		t.Fatalf("cSld name did not parse: got %q, want %q", got, "Introduction")
	}
	s.SetName("")

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if strings.Contains(out, `name="Introduction"`) {
		t.Errorf("SetName(\"\") did not clear the slide name:\n%s", out)
	}

	// The cleared name must also be gone from the model on reopen.
	rp, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	if got := rp.Slides()[0].Name(); got != "" {
		t.Errorf("cleared name came back on reopen: %q", got)
	}
}

// Renaming (rather than clearing) still wins, and clearing the name leaves the
// rest of p:cSld alone.
func TestSlide_RenameAndClearLeaveTheRestOfCSldAlone(t *testing.T) {
	deck := deckWithNamedSlide(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	p.Slides()[0].SetName("Renamed")
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(out, `name="Renamed"`) {
		t.Errorf("rename did not reach the XML:\n%s", out)
	}
	if !strings.Contains(out, "<p:spTree>") {
		t.Errorf("clearing the name disturbed the shape tree:\n%s", out)
	}
}

// A slide whose name is untouched round-trips byte-identically: the clearing
// path must not fire on a document nobody edited.
func TestSlide_UntouchedNameRoundTripsUnchanged(t *testing.T) {
	deck := deckWithNamedSlide(t)
	before := zipPart(t, deck, "ppt/slides/slide1.xml")

	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if after := zipPart(t, saved, "ppt/slides/slide1.xml"); !bytes.Equal(before, after) {
		t.Errorf("untouched slide drifted\nbefore %s\nafter  %s", before, after)
	}
}
