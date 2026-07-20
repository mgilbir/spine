package docx

import (
	"bytes"
	"testing"
)

// TestAddShapeGroupRoundTrip groups two shapes and confirms the wpg group markup
// and the members survive a save/reopen.
func TestAddShapeGroupRoundTrip(t *testing.T) {
	doc := Create()
	g := doc.AddShapeGroup(GroupOptions{},
		GroupMember{
			Text: "First", Shape: ShapeRectangle,
			XEMU: 0, YEMU: 0, WidthEMU: 914400, HeightEMU: 457200,
			FillColor: "FFCC00",
		},
		GroupMember{
			Shape: ShapeEllipse,
			XEMU:  914400, YEMU: 457200, WidthEMU: 914400, HeightEMU: 457200,
			BorderColor: "0000FF",
		},
	)
	if g == nil {
		t.Fatal("AddShapeGroup returned nil")
	}
	// The group extent is the bounding box of the members.
	if g.WidthEMU() != 1828800 || g.HeightEMU() != 914400 {
		t.Errorf("group extent = %dx%d, want 1828800x914400", g.WidthEMU(), g.HeightEMU())
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, _ := zipEntry(t, saved, "word/document.xml")
	for _, want := range []string{
		"<wpg:wgp>",
		"<wpg:grpSpPr>",
		`<a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingGroup">`,
		`prst="ellipse"`,
		`<a:srgbClr val="FFCC00"/>`,
		"First",
	} {
		if !bytes.Contains(docXML, []byte(want)) {
			t.Errorf("saved document.xml missing %q", want)
		}
	}

	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	// The group is one drawing; the first text member is reported.
	boxes := reopened.TextBoxes()
	if len(boxes) != 1 || boxes[0].Text() != "First" {
		t.Fatalf("reopened TextBoxes() = %+v, want one box with text First", boxes)
	}
}

// TestAddShapeGroupFloating anchors a group and confirms the anchor markup.
func TestAddShapeGroupFloating(t *testing.T) {
	doc := Create()
	doc.AddShapeGroup(GroupOptions{Floating: true, Anchor: Anchor{X: 36, Y: 72}},
		GroupMember{Text: "A", WidthEMU: 914400, HeightEMU: 457200})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, _ := zipEntry(t, saved, "word/document.xml")
	if !bytes.Contains(docXML, []byte("<wp:anchor")) {
		t.Error("floating group missing wp:anchor")
	}
	if !bytes.Contains(docXML, []byte("<wpg:wgp>")) {
		t.Error("floating group missing wpg:wgp")
	}
}
