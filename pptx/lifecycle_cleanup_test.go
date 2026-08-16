package pptx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempDeck saves deck bytes to a temp file and returns the path, for the
// path-only CreateFromTemplate API.
func writeTempDeck(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "template.pptx")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// C243/C303: CreateFromTemplate must not leave the template slides' relationships
// or owned parts behind. Otherwise a new AddSlide reuses slide1.xml and inherits
// the template slide's notesSlide (a data leak), and the removed template slides'
// content-type overrides dangle in [Content_Types].xml.
func TestCreateFromTemplate_ClearsSlideRelsAndNotes(t *testing.T) {
	// Build a template deck whose only slide carries secret speaker notes.
	tpl := Create()
	tpl.AddSlide().SetNotes("SECRET template notes")
	tplBytes, err := tpl.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// Sanity: the template really does carry the notes part.
	if _, ok := zipPartIfExists(t, tplBytes, "ppt/notesSlides/notesSlide1.xml"); !ok {
		t.Fatal("setup: template has no notesSlide1.xml")
	}
	templatePath := writeTempDeck(t, tplBytes)

	p, err := CreateFromTemplate(templatePath)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Add a fresh slide (which reuses the freed slide1.xml part name).
	newSlide := p.AddSlide()
	newSlide.AddTextBox().SetText("fresh")
	if newSlide.partName != "/ppt/slides/slide1.xml" {
		t.Fatalf("new slide part = %q, want the freed /ppt/slides/slide1.xml", newSlide.partName)
	}

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	reopened := openBytes(t, saved)
	defer func() { _ = reopened.Close() }()
	slides := reopened.Slides()
	if len(slides) != 1 {
		t.Fatalf("reopened deck has %d slides, want 1", len(slides))
	}
	if got := slides[0].Notes(); got != "" {
		t.Errorf("new slide inherited template notes: Notes() = %q, want empty", got)
	}

	if _, ok := zipPartIfExists(t, saved, "ppt/notesSlides/notesSlide1.xml"); ok {
		t.Error("orphan template notesSlide1.xml survives CreateFromTemplate")
	}
	ct := string(zipPart(t, saved, "[Content_Types].xml"))
	if strings.Contains(ct, "notesSlide1.xml") {
		t.Errorf("dangling content-type override for removed template notes part:\n%s", ct)
	}
	// The new slide must not carry a notesSlide relationship from the template.
	rels := string(zipPart(t, saved, "ppt/slides/_rels/slide1.xml.rels"))
	if strings.Contains(rels, "notesSlide") {
		t.Errorf("new slide inherited a template notesSlide rel:\n%s", rels)
	}
}
