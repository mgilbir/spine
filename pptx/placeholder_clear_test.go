package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// deckWithIndexedPlaceholder builds a deck whose slide carries a body
// placeholder with an explicit idx="3", the form PowerPoint writes for the
// third placeholder of a layout.
func deckWithIndexedPlaceholder(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("body")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		// Turn the text box into a placeholder carrying idx="3". The LAST
		// <p:nvPr/> is the shape's; the first belongs to the spTree's group.
		old := []byte("<p:nvPr/>")
		at := bytes.LastIndex(xml, old)
		if at < 0 {
			t.Fatalf("no p:nvPr to convert:\n%s", xml)
		}
		out := append([]byte{}, xml[:at]...)
		out = append(out, `<p:nvPr><p:ph type="body" idx="3"/></p:nvPr>`...)
		return append(out, xml[at+len(old):]...)
	})
}

// C585: Placeholder.Idx is omitempty, so SetIndex(0) wrote a zero that was
// suppressed and then replaced by the source's idx="3" on replay — the setter
// was a silent no-op on any parsed placeholder.
func TestPlaceholder_ClearingIndex_ReachesTheXML(t *testing.T) {
	deck := deckWithIndexedPlaceholder(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	phs := p.Slides()[0].Placeholders()
	if len(phs) == 0 {
		t.Fatal("no placeholder in the reopened deck")
	}
	if got := phs[0].Index(); got != 3 {
		t.Fatalf("placeholder idx did not parse: got %d, want 3", got)
	}
	phs[0].SetIndex(0)

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if strings.Contains(out, `idx="3"`) {
		t.Errorf("SetIndex(0) did not reach the XML:\n%s", out)
	}
	// The placeholder itself must survive; only its index was cleared.
	if !strings.Contains(out, `type="body"`) {
		t.Errorf("clearing the index removed the placeholder type:\n%s", out)
	}
}

// C585: setting a nonzero index still works, and an untouched placeholder keeps
// its parsed idx.
func TestPlaceholder_SetIndex_Nonzero(t *testing.T) {
	deck := deckWithIndexedPlaceholder(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	p.Slides()[0].Placeholders()[0].SetIndex(7)

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(out, `idx="7"`) {
		t.Errorf("SetIndex(7) did not reach the XML:\n%s", out)
	}
}

// C585: a zero-modification round trip must still replay the parsed idx.
func TestPlaceholder_UntouchedIndexSurvives(t *testing.T) {
	deck := deckWithIndexedPlaceholder(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(out, `idx="3"`) {
		t.Errorf("an untouched placeholder lost its parsed idx:\n%s", out)
	}
}
