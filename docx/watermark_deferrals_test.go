package docx

import (
	"strings"
	"testing"
)

// TestSectionTextWatermarkDetected stamps a watermark on a specific section and
// verifies it is detected and its text carried.
func TestSectionTextWatermarkDetected(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	if err := doc.SetSectionTextWatermark(sec, "CONFIDENTIAL", WatermarkOptions{Diagonal: true}); err != nil {
		t.Fatal(err)
	}
	wm := doc.Watermark()
	if wm == nil || wm.Type != WatermarkText || wm.Text != "CONFIDENTIAL" {
		t.Fatalf("section watermark not detected: %+v", wm)
	}
}

// TestPerSectionDistinctWatermarks applies different watermarks to two sections
// and verifies each section's default header carries its own watermark text.
func TestPerSectionDistinctWatermarks(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("first section")
	doc.AddSectionBreak() // first section's props move onto a paragraph
	doc.AddParagraphWithText("second section")

	secs := doc.Sections()
	if len(secs) != 2 {
		t.Fatalf("want 2 sections, got %d", len(secs))
	}
	if err := doc.SetSectionTextWatermark(secs[0], "DRAFT", WatermarkOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetSectionTextWatermark(secs[1], "FINAL", WatermarkOptions{}); err != nil {
		t.Fatal(err)
	}

	// Watermark text lives in the header parts, not document.xml; reopen and
	// scan every header part.
	fresh := reopenDoc(t, doc)
	texts := map[string]bool{}
	for _, hdr := range fresh.watermarkHeaders() {
		for _, p := range hdr.Paragraphs() {
			for _, raw := range paragraphPictContents(p) {
				if wm := classifyWatermark(raw); wm != nil {
					texts[wm.Text] = true
				}
			}
		}
	}
	if !texts["DRAFT"] || !texts["FINAL"] {
		t.Fatalf("expected both DRAFT and FINAL watermarks across sections, got %v", texts)
	}
}

// TestDrawingMLTextWatermark emits a DrawingML text watermark and verifies the
// saved header carries an mc:AlternateContent with a wps DrawingML choice and a
// VML fallback, and that detection still recognizes it.
func TestDrawingMLTextWatermark(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	if err := doc.SetTextWatermark("SECRET", WatermarkOptions{DrawingML: true, Diagonal: true}); err != nil {
		t.Fatal(err)
	}

	// Detection works on the in-memory model.
	if wm := doc.Watermark(); wm == nil || wm.Type != WatermarkText || wm.Text != "SECRET" {
		t.Fatalf("DrawingML watermark not detected: %+v", wm)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	data, ok := zipEntry(t, saved, "word/header1.xml")
	if !ok {
		t.Fatal("word/header1.xml missing from saved package")
	}
	headerXML := string(data)
	for _, want := range []string{
		"mc:AlternateContent", `Requires="wps"`, "wps:txbx", "mc:Fallback", "w:pict", "SECRET",
	} {
		if !strings.Contains(headerXML, want) {
			t.Errorf("DrawingML watermark header missing %q:\n%s", want, headerXML)
		}
	}

	// It reopens and is still detected as a text watermark.
	fresh := reopenDoc(t, doc)
	if wm := fresh.Watermark(); wm == nil || wm.Text != "SECRET" {
		t.Fatalf("DrawingML watermark not detected after round-trip: %+v", wm)
	}
}
