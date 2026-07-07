package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// C1 (removal residual): removing a shape from a loaded slide deletes only that
// shape and preserves content the domain model cannot represent (here, a group
// shape), instead of rebuilding the whole tree and dropping it.
func TestRemoveShapeOnLoadedSlidePreservesGroup(t *testing.T) {
	deck := deckWithGroupShape(t) // a textbox "prototype" + a spliced-in group
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	slide := p.Slides()[0]

	removed := false
	for _, shape := range slide.Shapes() {
		if tb, ok := shape.(*TextBox); ok {
			slide.RemoveShape(tb)
			removed = true
			break
		}
	}
	if !removed {
		t.Fatal("no textbox found to remove")
	}

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if strings.Contains(slideXML, "prototype") {
		t.Error("removed textbox is still present")
	}
	if !strings.Contains(slideXML, "<p:grpSp>") ||
		!strings.Contains(slideXML, "Group 9") ||
		!strings.Contains(slideXML, "Grouped 10") {
		t.Errorf("group shape was dropped by the removal rebuild:\n%s", slideXML)
	}
}
