package docx

import (
	"bytes"
	"strings"
	"testing"
)

// TestAddTextBoxRoundTrip creates a text box, saves the document, reopens it,
// and reads the text back out of the regenerated drawing.
func TestAddTextBoxRoundTrip(t *testing.T) {
	doc := Create()
	tb := doc.AddTextBox("Hello box", TextBoxOptions{WidthEMU: 1828800, HeightEMU: 914400})
	if tb == nil {
		t.Fatal("AddTextBox returned nil")
	}
	if got := tb.Text(); got != "Hello box" {
		t.Errorf("Text() = %q, want %q", got, "Hello box")
	}
	if tb.WidthEMU() != 1828800 || tb.HeightEMU() != 914400 {
		t.Errorf("size = %dx%d, want 1828800x914400", tb.WidthEMU(), tb.HeightEMU())
	}
	if tb.Shape() != ShapeRectangle {
		t.Errorf("Shape() = %q, want rect", tb.Shape())
	}

	// The in-memory model already reports the box.
	if boxes := doc.TextBoxes(); len(boxes) != 1 || boxes[0].Text() != "Hello box" {
		t.Fatalf("TextBoxes() before save = %+v", boxes)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// The drawing must carry a wps shape with a txbxContent body.
	docXML, ok := zipEntry(t, saved, "word/document.xml")
	if !ok {
		t.Fatal("document.xml missing")
	}
	for _, want := range []string{"<wps:wsp>", `<wps:cNvSpPr txBox="1"/>`, "<w:txbxContent>", "Hello box"} {
		if !bytes.Contains(docXML, []byte(want)) {
			t.Errorf("saved document.xml missing %q", want)
		}
	}

	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	boxes := reopened.TextBoxes()
	if len(boxes) != 1 {
		t.Fatalf("TextBoxes() after reopen = %d, want 1", len(boxes))
	}
	if got := boxes[0].Text(); got != "Hello box" {
		t.Errorf("reopened Text() = %q, want %q", got, "Hello box")
	}
	if boxes[0].WidthEMU() != 1828800 || boxes[0].HeightEMU() != 914400 {
		t.Errorf("reopened size = %dx%d", boxes[0].WidthEMU(), boxes[0].HeightEMU())
	}
}

// TestAddTextBoxMultiline splits newline-separated text into paragraphs and
// reads them back joined by newlines.
func TestAddTextBoxMultiline(t *testing.T) {
	doc := Create()
	doc.AddTextBox("line one\nline two", TextBoxOptions{})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	boxes := reopened.TextBoxes()
	if len(boxes) != 1 {
		t.Fatalf("TextBoxes() = %d, want 1", len(boxes))
	}
	if got := boxes[0].Text(); got != "line one\nline two" {
		t.Errorf("Text() = %q, want %q", got, "line one\nline two")
	}
}

// TestAddFloatingTextBox anchors a text box and confirms the anchor markup and
// the Floating flag survive a round trip.
func TestAddFloatingTextBox(t *testing.T) {
	doc := Create()
	tb := doc.AddTextBox("floating", TextBoxOptions{
		Floating: true,
		Anchor:   Anchor{RelativeToPage: true, X: 72, Y: 144},
	})
	if !tb.Floating() {
		t.Error("Floating() = false, want true")
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, _ := zipEntry(t, saved, "word/document.xml")
	if !bytes.Contains(docXML, []byte("<wp:anchor")) {
		t.Error("saved document.xml missing wp:anchor")
	}
	reopened, _ := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	boxes := reopened.TextBoxes()
	if len(boxes) != 1 || !boxes[0].Floating() {
		t.Fatalf("reopened floating box not found: %+v", boxes)
	}
}

// TestAddShapeNoText creates a basic shape with fill/border options.
func TestAddShapeNoText(t *testing.T) {
	doc := Create()
	doc.AddShape("", TextBoxOptions{
		Shape:       ShapeEllipse,
		FillColor:   "FF0000",
		BorderColor: "0000FF",
	})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, _ := zipEntry(t, saved, "word/document.xml")
	for _, want := range []string{`prst="ellipse"`, `<a:srgbClr val="FF0000"/>`, `<a:srgbClr val="0000FF"/>`} {
		if !bytes.Contains(docXML, []byte(want)) {
			t.Errorf("saved document.xml missing %q", want)
		}
	}
}

// TestReadDrawingMLTextBox reads a wps text box from an existing document.
func TestReadDrawingMLTextBox(t *testing.T) {
	body := `<w:body><w:p><w:r><w:drawing>` +
		`<wp:inline distT="0" distB="0" distL="0" distR="0" ` +
		`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<wp:extent cx="1828800" cy="914400"/>` +
		`<wp:docPr id="1" name="Text Box 1"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<wps:wsp><wps:cNvSpPr txBox="1"/><wps:spPr>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></wps:spPr>` +
		`<wps:txbx><w:txbxContent><w:p><w:r><w:t>Existing text</w:t></w:r></w:p></w:txbxContent></wps:txbx>` +
		`<wps:bodyPr/></wps:wsp></a:graphicData></a:graphic>` +
		`</wp:inline></w:drawing></w:r></w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS+
		` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"`+
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`+
		` xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"`, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	boxes := doc.TextBoxes()
	if len(boxes) != 1 {
		t.Fatalf("TextBoxes() = %d, want 1", len(boxes))
	}
	if got := boxes[0].Text(); got != "Existing text" {
		t.Errorf("Text() = %q, want %q", got, "Existing text")
	}
	if boxes[0].WidthEMU() != 1828800 || boxes[0].HeightEMU() != 914400 {
		t.Errorf("size = %dx%d", boxes[0].WidthEMU(), boxes[0].HeightEMU())
	}
	if boxes[0].IsVML() {
		t.Error("IsVML() = true, want false")
	}
}

// TestReadVMLTextBox reads a legacy VML text box.
func TestReadVMLTextBox(t *testing.T) {
	body := `<w:body><w:p><w:r><w:pict>` +
		`<v:shape xmlns:v="urn:schemas-microsoft-com:vml" id="s1" style="width:100pt;height:50pt">` +
		`<v:textbox><w:txbxContent><w:p><w:r><w:t>VML box</w:t></w:r></w:p></w:txbxContent></v:textbox>` +
		`</v:shape></w:pict></w:r></w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	boxes := doc.TextBoxes()
	if len(boxes) != 1 {
		t.Fatalf("TextBoxes() = %d, want 1", len(boxes))
	}
	if got := boxes[0].Text(); got != "VML box" {
		t.Errorf("Text() = %q, want %q", got, "VML box")
	}
	if !boxes[0].IsVML() {
		t.Error("IsVML() = false, want true")
	}
	// 100pt x 50pt in EMU (12700 EMU/pt).
	if boxes[0].WidthEMU() != 1270000 || boxes[0].HeightEMU() != 635000 {
		t.Errorf("size = %dx%d, want 1270000x635000", boxes[0].WidthEMU(), boxes[0].HeightEMU())
	}
}

// TestReadAlternateContentTextBoxOnce reads a text box wrapped in
// mc:AlternateContent (wps Choice + VML Fallback) and confirms the text is
// reported exactly once, not duplicated across the choice and fallback.
func TestReadAlternateContentTextBoxOnce(t *testing.T) {
	body := `<w:body><w:p><w:r><mc:AlternateContent>` +
		`<mc:Choice Requires="wps"><w:drawing><wp:inline ` +
		`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<wp:extent cx="914400" cy="457200"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<wps:wsp><wps:txbx><w:txbxContent><w:p><w:r><w:t>AC box</w:t></w:r></w:p></w:txbxContent></wps:txbx></wps:wsp>` +
		`</a:graphicData></a:graphic></wp:inline></w:drawing></mc:Choice>` +
		`<mc:Fallback><w:pict><v:shape xmlns:v="urn:schemas-microsoft-com:vml" id="s1" style="width:72pt;height:36pt">` +
		`<v:textbox><w:txbxContent><w:p><w:r><w:t>AC box</w:t></w:r></w:p></w:txbxContent></v:textbox>` +
		`</v:shape></w:pict></mc:Fallback>` +
		`</mc:AlternateContent></w:r></w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS+
		` xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"`, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	boxes := doc.TextBoxes()
	if len(boxes) != 1 {
		t.Fatalf("TextBoxes() = %d, want 1 (choice+fallback must not double-count)", len(boxes))
	}
	if got := boxes[0].Text(); got != "AC box" {
		t.Errorf("Text() = %q, want %q", got, "AC box")
	}
}

// TestExistingTextBoxByteIdentical confirms a document whose text box we do not
// touch is regenerated with the drawing preserved verbatim.
func TestExistingTextBoxByteIdentical(t *testing.T) {
	drawing := `<w:drawing>` +
		`<wp:inline distT="0" distB="0" distL="0" distR="0" ` +
		`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<wp:extent cx="1828800" cy="914400"/>` +
		`<wp:docPr id="1" name="Text Box 1"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<wps:wsp><wps:cNvSpPr txBox="1"/>` +
		`<wps:txbx><w:txbxContent><w:p><w:r><w:t>Keep me</w:t></w:r></w:p></w:txbxContent></wps:txbx>` +
		`<wps:bodyPr/></wps:wsp></a:graphicData></a:graphic>` +
		`</wp:inline></w:drawing>`
	body := `<w:body><w:p><w:r>` + drawing + `</w:r></w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)

	saved := openSave(t, fixture)
	if !strings.Contains(saved, drawing) {
		t.Errorf("saved document.xml did not preserve the drawing verbatim:\n%s", saved)
	}
}
