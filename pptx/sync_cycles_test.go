package pptx

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// Multi-cycle (mutate → save → mutate → save) regression tests for the shape
// sync bookkeeping: shapeRefs/syncedShapes must survive the compaction and
// appends performed by each sync, so the next cycle still targets the right
// parsed nodes.

var runTextRe = regexp.MustCompile(`<a:t>([^<]*)</a:t>`)

func slideTexts(t *testing.T, deck []byte, part string) []string {
	t.Helper()
	var out []string
	for _, m := range runTextRe.FindAllStringSubmatch(string(zipPart(t, deck, part)), -1) {
		out = append(out, m[1])
	}
	return out
}

// deckWithTextBoxes builds a deck with the given texts (one textbox each),
// splices in a group shape the domain model cannot represent, and returns the
// saved bytes for reopening.
func deckWithTextBoxes(t *testing.T, texts ...string) []byte {
	t.Helper()
	p := Create()
	slide := p.AddSlide()
	for _, txt := range texts {
		slide.AddTextBox().TextFrame().SetText(txt)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(grpSpFragment+"</p:spTree>"), 1)
	})
}

// C150: shapeRefs must be re-indexed after a surgical removal compacts the
// parsed tree. Before the fix, remove(A) → save → remove(B) → save deleted C
// and kept B.
func TestRemoveShapesAcrossSaveCycles(t *testing.T) {
	deck := deckWithTextBoxes(t, "First", "Second", "Third")
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	slide := p.Slides()[0]

	slide.RemoveShape(slide.Shapes()[0]) // First
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if got := slideTexts(t, saved, "ppt/slides/slide1.xml"); len(got) != 2 || got[0] != "Second" || got[1] != "Third" {
		t.Fatalf("after first removal: got %v, want [Second Third]", got)
	}

	slide.RemoveShape(slide.Shapes()[0]) // Second
	saved, err = p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	got := slideTexts(t, saved, "ppt/slides/slide1.xml")
	if len(got) != 1 || got[0] != "Third" {
		t.Fatalf("after second removal: got %v, want [Third]", got)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(slideXML, "Group 9") || !strings.Contains(slideXML, "Grouped 10") {
		t.Errorf("group shape was dropped across removal cycles:\n%s", slideXML)
	}

	// A third cycle for good measure: the remaining textbox must still be the
	// one the refs claim it is.
	slide.RemoveShape(slide.Shapes()[0]) // Third
	saved, err = p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if got := slideTexts(t, saved, "ppt/slides/slide1.xml"); len(got) != 0 {
		t.Fatalf("after third removal: got %v, want []", got)
	}
	slideXML = string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(slideXML, "Group 9") {
		t.Error("group shape was dropped when the last textbox was removed")
	}
}

// C158: a shape appended in a previous save cycle must extend shapeRefs, so
// removing it later stays surgical. Before the fix the removal fell into the
// full rebuild, which dropped the group shape and renumbered ids.
func TestRemoveAppendedShapeAfterSaveCycle(t *testing.T) {
	deck := deckWithGroupShape(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	slide := p.Slides()[0]

	added := slide.AddTextBox()
	added.TextFrame().SetText("temp")
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, saved, "ppt/slides/slide1.xml")), "temp") {
		t.Fatal("appended textbox missing after first save")
	}

	slide.RemoveShape(added)
	saved, err = p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(slideXML, "temp") {
		t.Error("removed appended textbox is still present")
	}
	if !strings.Contains(slideXML, "prototype") {
		t.Error("parsed textbox was dropped by the removal")
	}
	if !strings.Contains(slideXML, `<p:cNvPr id="9" name="Group 9"/>`) ||
		!strings.Contains(slideXML, `<p:cNvPr id="10" name="Grouped 10"/>`) {
		t.Errorf("group shape was dropped or renumbered by the removal:\n%s", slideXML)
	}

	// Next cycle must still be surgical: removing the parsed textbox keeps the
	// group intact.
	for _, shape := range slide.Shapes() {
		if tb, ok := shape.(*TextBox); ok {
			slide.RemoveShape(tb)
			break
		}
	}
	saved, err = p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML = string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(slideXML, "prototype") {
		t.Error("removed parsed textbox is still present")
	}
	if !strings.Contains(slideXML, "Group 9") {
		t.Error("group shape was dropped in the second removal cycle")
	}
}
