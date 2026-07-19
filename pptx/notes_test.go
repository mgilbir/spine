package pptx

import (
	"bytes"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// TestNotes_NoNotesSlideReturnsEmpty: a slide without a notes slide reports no
// notes text.
func TestNotes_NoNotesSlideReturnsEmpty(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	if got := s.Notes(); got != "" {
		t.Fatalf("Notes() on a slide with no notes = %q, want empty", got)
	}
}

// TestSetNotes_CreateRoundTrip: SetNotes on a created deck, then save and
// reopen, yields the same notes text back.
func TestSetNotes_CreateRoundTrip(t *testing.T) {
	p := Create()
	s := p.AddSlide()

	const want = "First line of notes\nSecond line"
	s.SetNotes(want)
	if got := s.Notes(); got != want {
		t.Fatalf("Notes() right after SetNotes = %q, want %q", got, want)
	}

	var buf bytes.Buffer
	if err := p.SaveTo(&buf); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	slides := reopened.Slides()
	if len(slides) != 1 {
		t.Fatalf("reopened deck has %d slides, want 1", len(slides))
	}
	if got := slides[0].Notes(); got != want {
		t.Fatalf("Notes() after reopen = %q, want %q", got, want)
	}
}

// TestSetNotes_ReplaceExisting: SetNotes twice replaces the body text rather
// than accumulating notes parts.
func TestSetNotes_ReplaceExisting(t *testing.T) {
	p := Create()
	s := p.AddSlide()

	s.SetNotes("original")
	before := len(notesParts(p))
	s.SetNotes("updated")
	after := len(notesParts(p))

	if before != 1 || after != 1 {
		t.Fatalf("notes part count = (%d then %d), want (1 then 1)", before, after)
	}
	if got := s.Notes(); got != "updated" {
		t.Fatalf("Notes() = %q, want %q", got, "updated")
	}
}

// TestNotes_ReadFromExistingNotesSlide: notes are read from a pre-existing
// notes-bearing part wired to the slide.
func TestNotes_ReadFromExistingNotesSlide(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.partName = "/ppt/slides/slide1.xml"

	const notesPart = "/ppt/notesSlides/notesSlide1.xml"
	p.otherParts[notesPart] = &coxml.RawPart{
		ContentType: opc.ContentTypeNotesSlide,
		Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
			`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:cSld><p:spTree>` +
			`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
			`<p:grpSpPr/>` +
			`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder 1"/>` +
			`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
			`<p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>` +
			`<p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>` +
			`<a:p><a:r><a:t>Hello</a:t></a:r></a:p>` +
			`<a:p><a:r><a:t>World</a:t></a:r></a:p>` +
			`</p:txBody></p:sp>` +
			`</p:spTree></p:cSld></p:notes>`),
	}
	p.relationships[s.partName] = []*opc.Relationship{
		{ID: "rId1", Type: opc.RelTypeNotesSlide, Target: "../notesSlides/notesSlide1.xml", TargetMode: opc.TargetModeInternal},
	}

	if got := s.Notes(); got != "Hello\nWorld" {
		t.Fatalf("Notes() = %q, want %q", got, "Hello\nWorld")
	}
}

// TestSetNotes_EditExistingPreservesPart: editing notes on a slide that already
// has a notes slide rewrites that part in place (no new part, same rel).
func TestSetNotes_EditExistingPreservesPart(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.partName = "/ppt/slides/slide1.xml"

	const notesPart = "/ppt/notesSlides/notesSlide1.xml"
	p.otherParts[notesPart] = &coxml.RawPart{
		ContentType: opc.ContentTypeNotesSlide,
		Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
			`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:cSld><p:spTree>` +
			`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
			`<p:grpSpPr/>` +
			`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder 1"/>` +
			`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
			`<p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>` +
			`<p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>` +
			`<a:p><a:r><a:t>old text</a:t></a:r></a:p>` +
			`</p:txBody></p:sp>` +
			`</p:spTree></p:cSld></p:notes>`),
	}
	p.relationships[s.partName] = []*opc.Relationship{
		{ID: "rId1", Type: opc.RelTypeNotesSlide, Target: "../notesSlides/notesSlide1.xml", TargetMode: opc.TargetModeInternal},
	}

	s.SetNotes("brand new text")

	if n := len(notesParts(p)); n != 1 {
		t.Fatalf("notes part count = %d, want 1 (edit must not add a part)", n)
	}
	if got := s.Notes(); got != "brand new text" {
		t.Fatalf("Notes() = %q, want %q", got, "brand new text")
	}
	// The rewritten part must still be valid PresentationML notes.
	data := p.otherParts[notesPart].Data
	if !bytes.Contains(data, []byte("<p:notes")) || !bytes.Contains(data, []byte("brand new text")) {
		t.Fatalf("rewritten notes part not as expected: %s", data)
	}
}

// notesParts returns the notesSlide part names currently registered.
func notesParts(p *Presentation) []string {
	var out []string
	for name := range p.otherParts {
		if strings.HasPrefix(name, "/ppt/notesSlides/") {
			out = append(out, name)
		}
	}
	return out
}
