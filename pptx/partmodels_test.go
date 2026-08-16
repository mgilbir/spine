package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// flushForInspection serializes the deferred notes/comment models so a test can
// assert on the bytes the save would write.
//
// The setters no longer serialize on the spot — they edit a model that the save
// flushes (see partmodels.go) — so a test reading a part straight after a setter
// reads the state from before the edit. That reads as a passing test rather than
// a failing one when the stale bytes happen to satisfy the assertion, which is
// why the tests that inspect a rewritten part call this rather than being
// rewritten to assert through a save: the assertion stays on the same bytes it
// was written for, and the flush is visible at the point it matters.
func flushForInspection(tb testing.TB, p *Presentation) {
	tb.Helper()
	if err := p.flushPendingParts(); err != nil {
		tb.Fatalf("flushing the deferred notes/comment models: %v", err)
	}
}

// TestFlushIsRequiredToSerializeADeferredEdit pins the invariant the helper
// above exists for, so the deferral is not quietly undone by a future change
// that goes back to serializing inside the setter.
//
// It also covers the other half: the flush must actually produce the edit. A
// deferral that never flushed would pass the first assertion and lose the data.
func TestFlushIsRequiredToSerializeADeferredEdit(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.SetNotes("deferred until the flush")

	part := s.notesSlidePartName()
	if part == "" {
		t.Fatal("SetNotes did not wire a notes slide relationship")
	}
	if data := p.rawPartData(part); len(data) != 0 {
		t.Errorf("SetNotes serialized on the spot; the edit should be deferred:\n%s", data)
	}
	// The edit is readable through the model regardless of serialization.
	if got := s.Notes(); got != "deferred until the flush" {
		t.Errorf("Notes() before the flush = %q, want the text just set", got)
	}

	flushForInspection(t, p)
	if got := p.rawPartData(part); len(got) == 0 {
		t.Fatal("the flush produced no bytes for the edited notes part")
	} else if !bytes.Contains(got, []byte("deferred until the flush")) {
		t.Errorf("flushed notes part does not carry the text:\n%s", got)
	}
}

// A comment part carrying an attribute named "a:" — a bound prefix with an
// empty local part. It is well-formed XML and its prefix resolves, so it passes
// both the parse and the namespace check; it is not a QName, so the Builder
// refuses to write it. That combination is what makes it the right fixture:
// the failure is reachable only at serialization time.
//
// The attribute the corpus actually produced, cre0:0ated, is no longer usable
// here — the prefix it names is unbound, and this branch rejects that at parse.
const unwritableThreadXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<p188:cmLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main">` +
	`<p188:cm id="{1}" authorId="{7E013C82-7D75-1E69-2E91-2087A44DBE8C}" a:="2024-01-01T00:00:00.000">` +
	`<p188:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>body</a:t></a:r></a:p></p188:txBody>` +
	`</p188:cm></p188:cmLst>`

// TestAnUnwritablePartFailsTheSaveRatherThanTheSetter is why the setters carry no
// error return.
//
// The failure is real and it is not swallowed — it moved to Save, which is the
// only place it can be acted on, and which now refuses to write the package.
// Under the setter-returns-error shape the same deck saved *successfully*: the
// setter refused, left the part's original bytes in place, and a caller who did
// not check got a file that was missing the edit and said nothing about it. The
// second half of this test pins that difference.
func TestAnUnwritablePartFailsTheSaveRatherThanTheSetter(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	injectSlidePart(p, s, modernAuthorsPart, opc.ContentTypeAuthors, opc.RelTypeAuthors, modernAuthorsXML)
	injectSlidePart(p, s, "/ppt/comments/modernComment1.xml",
		opc.ContentTypeModernComments, opc.RelTypeModernComments, unwritableThreadXML)

	comments := s.Comments()
	if len(comments) != 1 {
		t.Fatalf("fixture setup failed: slide has %d comments, want 1", len(comments))
	}
	// The setter cannot fail: it edits a model and returns nothing.
	comments[0].SetResolved(true)

	// The save is where it surfaces, and it declines to write anything.
	if _, err := p.SaveBytes(); err == nil {
		t.Fatal("saving a deck whose edited comment part cannot be serialized reported no error")
	} else if !strings.Contains(err.Error(), `"a:"`) {
		t.Errorf("save error does not name the offending attribute: %v", err)
	}

	// And it keeps failing: the model stays dirty, so the failure cannot be
	// walked past by simply saving again.
	if _, err := p.SaveBytes(); err == nil {
		t.Fatal("the second save of the same deck reported no error")
	}
}

// TestRemovingASlideDropsItsPendingParts covers the hazard the deferral
// introduces at the other end: a model that outlives the part it belongs to.
//
// Deleting a slide deletes the notes and comment parts only it referenced. If
// their models stayed in the registry still marked dirty, the flush at save
// would write the parts straight back — as orphans, since the relationships
// that named them went with the slide.
func TestRemovingASlideDropsItsPendingParts(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.SetNotes("notes on a slide about to be removed")
	s.AddComment("Ada Lovelace", "a comment on the same slide")
	p.AddSlide() // a survivor, so the deck still has a slide to save

	notesPart := s.notesSlidePartName()
	if notesPart == "" {
		t.Fatal("SetNotes did not wire a notes slide")
	}
	commentPart := s.Comments()[0].partName

	if err := p.RemoveSlide(0); err != nil {
		t.Fatalf("RemoveSlide: %v", err)
	}

	flushForInspection(t, p)
	if _, ok := p.otherParts[notesPart]; ok {
		t.Errorf("the flush resurrected the removed slide's notes part %s", notesPart)
	}
	if _, ok := p.otherParts[commentPart]; ok {
		t.Errorf("the flush resurrected the removed slide's comment part %s", commentPart)
	}

	// And the saved package must not carry them either.
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	names := zipNames(t, data)
	for _, part := range []string{notesPart, commentPart} {
		if entry := strings.TrimPrefix(part, "/"); names[entry] {
			t.Errorf("saved package contains the removed slide's part %q", entry)
		}
	}
}

// Notes and comment parts in a producer's rendering rather than this library's:
// indented, with an XML comment, and with empty elements written in long form.
// The fidelity capture does not reproduce any of those, so re-serializing these
// parts changes their bytes — which is what makes them able to fail the test
// below. A part this library generated re-serializes byte-identically, so
// building the fixture with SetNotes/AddComment would have made the assertion
// hold whether or not reads mark models dirty.
const producerNotesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <!-- written by some other producer -->
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""></p:cNvPr><p:cNvGrpSpPr></p:cNvGrpSpPr><p:nvPr></p:nvPr></p:nvGrpSpPr>
      <p:grpSpPr></p:grpSpPr>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder 1"></p:cNvPr><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
        <p:spPr></p:spPr>
        <p:txBody><a:bodyPr></a:bodyPr><a:lstStyle></a:lstStyle><a:p><a:r><a:t>notes from a producer</a:t></a:r></a:p></p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:notes>`

// TestFlushLeavesAnUntouchedPartAlone is the byte-identity half: a part that was
// only read must not be re-serialized, because re-serializing it is what turns a
// round trip into a diff.
func TestFlushLeavesAnUntouchedPartAlone(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	const notesPart = "/ppt/notesSlides/notesSlide1.xml"
	const commentPart = "/ppt/comments/modernComment1.xml"
	injectSlidePart(p, s, notesPart, opc.ContentTypeNotesSlide, opc.RelTypeNotesSlide, producerNotesXML)
	injectSlidePart(p, s, modernAuthorsPart, opc.ContentTypeAuthors, opc.RelTypeAuthors, modernAuthorsXML)
	injectSlidePart(p, s, commentPart, opc.ContentTypeModernComments, opc.RelTypeModernComments, modernThreadXML)

	notesBefore := string(p.rawPartData(notesPart))
	commentBefore := string(p.rawPartData(commentPart))

	// Reads only. Each parses and caches a model; none of them may mark it dirty.
	if got := s.Notes(); got != "notes from a producer" {
		t.Fatalf("Notes() = %q, so the fixture was not read through the model", got)
	}
	if len(s.Comments()) != 1 {
		t.Fatalf("Comments() found %d comments, want 1", len(s.Comments()))
	}
	_ = p.Text()

	flushForInspection(t, p)
	if got := string(p.rawPartData(notesPart)); got != notesBefore {
		t.Errorf("reading the notes rewrote %s:\nbefore: %s\nafter:  %s", notesPart, notesBefore, got)
	}
	if got := string(p.rawPartData(commentPart)); got != commentBefore {
		t.Errorf("reading the comments rewrote %s:\nbefore: %s\nafter:  %s", commentPart, commentBefore, got)
	}
}
