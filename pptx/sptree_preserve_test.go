package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// spTreeACFragment is an mc:AlternateContent spTree child in the byte form
// the marshaler emits (declarations on the AlternateContent element, fallback
// with xmlns=""). PowerPoint uses this construct for ink and 2010+ shapes.
const spTreeACFragment = `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">` +
	`<mc:Choice Requires="p14"><p:contentPart r:id="rId9"/></mc:Choice>` +
	`<mc:Fallback xmlns="">` +
	`<p:sp xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:nvSpPr><p:cNvPr id="20" name="Ink Fallback 20"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr/></p:sp>` +
	`</mc:Fallback></mc:AlternateContent>`

// spTreeContentPart is a bare p:contentPart spTree child (no mc wrapper).
const spTreeContentPart = `<p:contentPart r:id="rId8"/>`

// deckWithSpTreeAlternateContent builds a deck via the API and splices an
// mc:AlternateContent plus a bare p:contentPart into the slide's spTree.
func deckWithSpTreeAlternateContent(t *testing.T, boxes ...string) []byte {
	t.Helper()
	p := Create()
	slide := p.AddSlide()
	for _, text := range boxes {
		box := slide.AddTextBox()
		box.TextFrame().SetText(text)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		if !bytes.Contains(xml, []byte("</p:spTree>")) {
			t.Fatal("slide1.xml has no spTree close tag")
		}
		return bytes.Replace(xml, []byte("</p:spTree>"),
			[]byte(spTreeACFragment+spTreeContentPart+"</p:spTree>"), 1)
	})
}

// C32: a zero-modification Open+Save must not delete mc:AlternateContent or
// p:contentPart children of the shape tree, and must re-emit them
// byte-faithfully in position.
func TestZeroModSavePreservesSpTreeAlternateContent(t *testing.T) {
	deck := deckWithSpTreeAlternateContent(t, "keeper")

	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if !strings.Contains(slideXML, spTreeACFragment) {
		t.Errorf("spTree AlternateContent not preserved byte-faithfully:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, spTreeContentPart) {
		t.Errorf("spTree contentPart not preserved:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, "keeper") {
		t.Errorf("existing shape lost:\n%s", slideXML)
	}
}

// C32: appending a shape to a loaded slide keeps the preserved children in
// their original position (before the appended shape).
func TestAddShapePreservesSpTreeAlternateContent(t *testing.T) {
	deck := deckWithSpTreeAlternateContent(t, "keeper")

	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	slide := p.Slides()[0]
	box := slide.AddTextBox()
	box.TextFrame().SetText("appended")

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	acIdx := strings.Index(slideXML, spTreeACFragment)
	if acIdx < 0 {
		t.Fatalf("spTree AlternateContent lost after AddTextBox:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, spTreeContentPart) {
		t.Errorf("spTree contentPart lost after AddTextBox:\n%s", slideXML)
	}
	addedIdx := strings.Index(slideXML, "appended")
	if addedIdx < 0 {
		t.Fatalf("appended shape missing:\n%s", slideXML)
	}
	if acIdx > addedIdx {
		t.Errorf("AlternateContent moved after the appended shape (position lost)")
	}
}

// C32 + C150 interplay: surgical multi-cycle removals must keep deleting the
// right shapes while never touching the preserved raw children.
func TestRemoveShapesPreservesSpTreeAlternateContentMultiCycle(t *testing.T) {
	deck := deckWithSpTreeAlternateContent(t, "Box1", "Box2", "Box3")

	removeBox := func(data []byte, text string) []byte {
		t.Helper()
		p, err := OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		slide := p.Slides()[0]
		for _, shape := range slide.Shapes() {
			tb, ok := shape.(*TextBox)
			if !ok {
				continue
			}
			if tb.TextFrame().Text() == text {
				slide.RemoveShape(tb)
				saved, err := p.SaveBytes()
				if err != nil {
					t.Fatal(err)
				}
				return saved
			}
		}
		t.Fatalf("box %q not found", text)
		return nil
	}

	saved := removeBox(deck, "Box1")
	saved = removeBox(saved, "Box2")

	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(slideXML, "Box1") || strings.Contains(slideXML, "Box2") {
		t.Errorf("removed boxes still present:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, "Box3") {
		t.Errorf("wrong shape removed (Box3 gone):\n%s", slideXML)
	}
	if !strings.Contains(slideXML, spTreeACFragment) {
		t.Errorf("spTree AlternateContent lost across removal cycles:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, spTreeContentPart) {
		t.Errorf("spTree contentPart lost across removal cycles:\n%s", slideXML)
	}
}

// C32: unmodeled children inside a group shape (p:grpSp) are preserved too.
func TestGroupShapeUnmodeledChildPreserved(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	box := slide.AddTextBox()
	box.TextFrame().SetText("keeper")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	group := `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="30" name="Group 30"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` +
		spTreeContentPart +
		`</p:grpSp>`
	deck := rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(group+"</p:spTree>"), 1)
	})

	p2, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(slideXML, group) {
		t.Errorf("group contentPart not preserved byte-faithfully:\n%s", slideXML)
	}
}
