package pptx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// addZipParts returns a copy of the package with the given entries appended.
func addZipParts(t *testing.T, data []byte, parts map[string][]byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	copyEntry := func(name string, content []byte) {
		dst, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range reader.File {
		src, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var content bytes.Buffer
		if _, err := content.ReadFrom(src); err != nil {
			t.Fatal(err)
		}
		_ = src.Close()
		copyEntry(file.Name, content.Bytes())
	}
	for name, content := range parts {
		copyEntry(name, content)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// deckWithNotes returns a saved one-slide deck whose slide1 carries a notes
// slide part with a slide back-reference, wired through slide rels and a
// content-type override, like PowerPoint writes it.
func deckWithNotes(t *testing.T) []byte {
	t.Helper()
	deck := savedDeck(t)
	deck = addZipParts(t, deck, map[string][]byte{
		"ppt/notesSlides/notesSlide1.xml":            []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:notes>`),
		"ppt/notesSlides/_rels/notesSlide1.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="../slides/slide1.xml"/></Relationships>`),
	})
	deck = rewriteZipPart(t, deck, "ppt/slides/_rels/slide1.xml.rels", func(rels []byte) []byte {
		add := `<Relationship Id="rId99" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide1.xml"/>`
		return bytes.Replace(rels, []byte("</Relationships>"), []byte(add+"</Relationships>"), 1)
	})
	deck = rewriteZipPart(t, deck, "[Content_Types].xml", func(ct []byte) []byte {
		add := `<Override PartName="/ppt/notesSlides/notesSlide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"/>`
		return bytes.Replace(ct, []byte("</Types>"), []byte(add+"</Types>"), 1)
	})
	return deck
}

// C88: RemoveSlide removes the slide's notes part along with its
// relationships and content-type override, so no orphan notes slide (with a
// back-reference to a freed part name) survives in the package.
func TestRemoveSlide_RemovesNotesSlidePart(t *testing.T) {
	p := openBytes(t, deckWithNotes(t))
	if _, ok := p.otherParts["/ppt/notesSlides/notesSlide1.xml"]; !ok {
		t.Fatal("setup: notes part not loaded")
	}
	if err := p.RemoveSlide(0); err != nil {
		t.Fatal(err)
	}
	// The freed part name is reused by the next added slide.
	newSlide := p.AddSlide()
	newSlide.AddTextBox().SetText("fresh")
	if newSlide.partName != "/ppt/slides/slide1.xml" {
		t.Fatalf("new slide part = %q, want the freed /ppt/slides/slide1.xml", newSlide.partName)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zipPartIfExists(t, data, "ppt/notesSlides/notesSlide1.xml"); ok {
		t.Error("orphan notesSlide part survives RemoveSlide")
	}
	if _, ok := zipPartIfExists(t, data, "ppt/notesSlides/_rels/notesSlide1.xml.rels"); ok {
		t.Error("orphan notesSlide rels survive RemoveSlide")
	}
	ct := string(zipPart(t, data, "[Content_Types].xml"))
	if strings.Contains(ct, "notesSlide1.xml") {
		t.Errorf("content-type override for the removed notes part lingers:\n%s", ct)
	}
	// The reused slide part name must get its override re-registered.
	if !strings.Contains(ct, `PartName="/ppt/slides/slide1.xml"`) {
		t.Errorf("reused slide part lost its content-type override:\n%s", ct)
	}
	// The new slide reusing the freed name must not inherit phantom notes.
	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	if strings.Contains(rels, "notesSlide") {
		t.Errorf("new slide reusing the part name inherited a notesSlide rel:\n%s", rels)
	}
}

// C88: a notes part referenced by another slide (should not happen in a valid
// package, but be conservative) is not removed with the slide.
func TestRemoveSlide_KeepsSharedNotesPart(t *testing.T) {
	p := openBytes(t, deckWithNotes(t))
	// Simulate a second slide sharing the notes part.
	s2 := p.AddSlide()
	p.relationships[s2.partName] = append(p.relationships[s2.partName],
		p.relationships["/ppt/slides/slide1.xml"]...)

	if err := p.RemoveSlide(0); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.otherParts["/ppt/notesSlides/notesSlide1.xml"]; !ok {
		t.Error("notes part referenced by another slide was removed")
	}
}

// C118: a slide part present in the package but not referenced by
// presentation.xml round-trips verbatim instead of vanishing, and AddSlide
// does not collide with its part name.
func TestUnreferencedSlidePart_RoundTripsAndBlocksName(t *testing.T) {
	base := savedDeck(t)
	slide1XML := zipPart(t, base, "ppt/slides/slide1.xml")
	unrefXML := bytes.Replace(slide1XML, []byte(">content<"), []byte(">unreferenced<"), 1)

	deck := addZipParts(t, base, map[string][]byte{
		"ppt/slides/slide2.xml":            unrefXML,
		"ppt/slides/_rels/slide2.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`),
	})
	deck = rewriteZipPart(t, deck, "[Content_Types].xml", func(ct []byte) []byte {
		add := `<Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`
		return bytes.Replace(ct, []byte("</Types>"), []byte(add+"</Types>"), 1)
	})

	p := openBytes(t, deck)
	if got := p.SlideCount(); got != 1 {
		t.Fatalf("SlideCount = %d, want 1 (unreferenced slide must not join the slide list)", got)
	}

	// AddSlide must not collide with the preserved slide2.xml.
	newSlide := p.AddSlide()
	newSlide.AddTextBox().SetText("added")
	if newSlide.partName == "/ppt/slides/slide2.xml" {
		t.Fatalf("AddSlide reused the preserved unreferenced slide part name %q", newSlide.partName)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := zipPartIfExists(t, data, "ppt/slides/slide2.xml")
	if !ok {
		t.Fatal("unreferenced slide part vanished on save")
	}
	if !bytes.Equal(got, unrefXML) {
		t.Error("unreferenced slide part was not preserved verbatim")
	}
	if _, ok := zipPartIfExists(t, data, "ppt/slides/_rels/slide2.xml.rels"); !ok {
		t.Error("unreferenced slide's rels vanished on save")
	}
	// presentation.xml still references only the two real slides.
	pres := string(zipPart(t, data, "ppt/presentation.xml"))
	if strings.Count(pres, "<p:sldId ") != 2 {
		t.Errorf("presentation.xml slide count changed:\n%s", pres)
	}
}
