package docx

import (
	"bytes"
	"testing"
)

// TestAddWordArtRoundTrip creates a warped WordArt shape, saves, reopens, and
// reads the text back out of the regenerated drawing.
func TestAddWordArtRoundTrip(t *testing.T) {
	doc := Create()
	wa := doc.AddWordArt("Fancy", WordArtOptions{
		Warp:      WarpArchUp,
		FillColor: "FF0000",
		Bold:      true,
	})
	if wa == nil {
		t.Fatal("AddWordArt returned nil")
	}
	if wa.Text() != "Fancy" {
		t.Errorf("Text() = %q, want %q", wa.Text(), "Fancy")
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, ok := zipEntry(t, saved, "word/document.xml")
	if !ok {
		t.Fatal("document.xml missing")
	}
	for _, want := range []string{
		`<a:prstTxWarp prst="textArchUp">`,
		`<a:noFill/>`,
		`<w:color w:val="FF0000"/>`,
		"<w:txbxContent>",
		"Fancy",
	} {
		if !bytes.Contains(docXML, []byte(want)) {
			t.Errorf("saved document.xml missing %q", want)
		}
	}

	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	boxes := reopened.TextBoxes()
	if len(boxes) != 1 || boxes[0].Text() != "Fancy" {
		t.Fatalf("reopened TextBoxes() = %+v, want one box with text Fancy", boxes)
	}
}

// TestAddWordArtNoWarp confirms a WordArt without a warp still emits a wps shape
// and omits the prstTxWarp element.
func TestAddWordArtNoWarp(t *testing.T) {
	doc := Create()
	doc.AddWordArt("Plain", WordArtOptions{})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, _ := zipEntry(t, saved, "word/document.xml")
	if bytes.Contains(docXML, []byte("prstTxWarp")) {
		t.Error("un-warped WordArt should not emit prstTxWarp")
	}
	if !bytes.Contains(docXML, []byte("<wps:wsp>")) {
		t.Error("WordArt should emit a wps shape")
	}
}

// TestAddWordArtCaptionEscapesCR verifies a carriage return in a WordArt caption
// is emitted as a &#xD; character reference rather than a raw CR, which XML
// §2.11 end-of-line handling would silently normalize to a newline on reparse.
// Regression test for C349.
func TestAddWordArtCaptionEscapesCR(t *testing.T) {
	doc := Create()
	doc.AddWordArt("before\rafter", WordArtOptions{})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, ok := zipEntry(t, saved, "word/document.xml")
	if !ok {
		t.Fatal("document.xml missing")
	}
	if !bytes.Contains(docXML, []byte("before&#xD;after")) {
		t.Errorf("document.xml did not escape the CR as &#xD;; got %q", docXML)
	}
	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	boxes := reopened.TextBoxes()
	if len(boxes) != 1 {
		t.Fatalf("TextBoxes() = %d, want 1", len(boxes))
	}
	if got := boxes[0].Text(); got != "before\rafter" {
		t.Errorf("reopened Text() = %q, want %q (CR lost to EOL normalization)", got, "before\rafter")
	}
}
