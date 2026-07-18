package pptx

import (
	"bytes"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// reopen saves p to bytes and opens the result.
func reopen(t *testing.T, p *Presentation) *Presentation {
	t.Helper()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	rp, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	return rp
}

// injectPart adds a raw part and a slide relationship pointing at it.
func injectSlidePart(p *Presentation, s *Slide, partName, contentType, relType, body string) {
	p.otherParts[partName] = &coxml.RawPart{ContentType: contentType, Data: []byte(body)}
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         s.nextRelID(),
		Type:       relType,
		Target:     relativeTarget(s.partName, partName),
		TargetMode: opc.TargetModeInternal,
	})
}

const legacyAuthorsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:cmAuthorLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cmAuthor id="0" name="Ada Lovelace" initials="AL" lastIdx="1" clrIdx="0"/></p:cmAuthorLst>`

const legacyCommentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:cmLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cm authorId="0" dt="2011-02-09T11:06:58.492" idx="1"><p:pos x="684" y="1584"/><p:text>Looks great</p:text></p:cm></p:cmLst>`

const modernAuthorsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p188:authorLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main"><p188:author id="{7E013C82-7D75-1E69-2E91-2087A44DBE8C}" name="Grace Hopper" initials="GH" userId="u1" providerId="AD"/><p188:author id="{070468F7-CA25-9EB3-8E12-A41AF9434767}" name="Alan Turing" initials="AT" userId="u2" providerId="AD"/></p188:authorLst>`

const modernThreadXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p188:cmLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main"><p188:cm id="{1731B08D-2FF5-074E-A2EE-D3516BCF8095}" authorId="{7E013C82-7D75-1E69-2E91-2087A44DBE8C}" status="resolved" created="2025-01-30T11:09:28.599"><pc:sldMkLst xmlns:pc="http://schemas.microsoft.com/office/powerpoint/2013/main/command"><pc:docMk/><pc:sldMk cId="1836216607" sldId="256"/></pc:sldMkLst><p188:pos x="4572000" y="2286000"/><p188:replyLst><p188:reply id="{C720E66C-5F4D-41AB-B8D6-6EAE84414019}" authorId="{070468F7-CA25-9EB3-8E12-A41AF9434767}" created="2025-01-30T12:03:49.050"><p188:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="pt-PT"/><a:t>Solved!</a:t></a:r></a:p></p188:txBody></p188:reply></p188:replyLst><p188:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Please review the algorithm.</a:t></a:r></a:p></p188:txBody></p188:cm></p188:cmLst>`

func firstSlide(t *testing.T, p *Presentation) *Slide {
	t.Helper()
	s, err := p.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	return s
}

func TestReadLegacyComments(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := firstSlide(t, p)
	p.otherParts[legacyAuthorsPart] = &coxml.RawPart{ContentType: opc.ContentTypePresentationCommentAuthors, Data: []byte(legacyAuthorsXML)}
	injectSlidePart(p, s, "/ppt/comments/comment1.xml", opc.ContentTypePresentationComments, opc.RelTypeComments, legacyCommentXML)

	comments := s.Comments()
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	c := comments[0]
	if c.Author() != "Ada Lovelace" {
		t.Errorf("author = %q, want Ada Lovelace", c.Author())
	}
	if c.Text() != "Looks great" {
		t.Errorf("text = %q", c.Text())
	}
	if c.Date().IsZero() {
		t.Errorf("date should be parsed")
	}
	if x, y := c.Position(); x != 684 || y != 1584 {
		t.Errorf("pos = (%d,%d), want (684,1584)", x, y)
	}
	if c.Resolved() {
		t.Errorf("legacy comment should never be resolved")
	}
	if c.Slide() != s {
		t.Errorf("Slide() mismatch")
	}
	// Legacy comments cannot be replied to or resolved (documented no-ops).
	if c.Reply("Bob", "hi") != nil {
		t.Errorf("Reply on legacy comment should return nil")
	}
	c.Resolve()
	if c.Resolved() {
		t.Errorf("Resolve on legacy comment must be a no-op")
	}
}

func TestReadModernComments(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := firstSlide(t, p)
	p.otherParts[modernAuthorsPart] = &coxml.RawPart{ContentType: opc.ContentTypeAuthors, Data: []byte(modernAuthorsXML)}
	injectSlidePart(p, s, "/ppt/comments/modernComment1.xml", opc.ContentTypeModernComments, opc.RelTypeModernComments, modernThreadXML)

	comments := s.Comments()
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	top := comments[0]
	if top.Author() != "Grace Hopper" {
		t.Errorf("author = %q, want Grace Hopper", top.Author())
	}
	if top.Text() != "Please review the algorithm." {
		t.Errorf("text = %q", top.Text())
	}
	if !top.Resolved() {
		t.Errorf("thread should be resolved")
	}
	if top.Date().IsZero() {
		t.Errorf("created date should parse")
	}
	if x, y := top.Position(); x != 4572000 || y != 2286000 {
		t.Errorf("pos = (%d,%d)", x, y)
	}
	if len(top.Replies()) != 1 {
		t.Fatalf("got %d replies, want 1", len(top.Replies()))
	}
	reply := top.Replies()[0]
	if reply.Author() != "Alan Turing" || reply.Text() != "Solved!" {
		t.Errorf("reply = %q by %q", reply.Text(), reply.Author())
	}
	if reply.Parent() != top {
		t.Errorf("reply.Parent() mismatch")
	}
	if !reply.Resolved() {
		t.Errorf("reply should inherit resolved state")
	}
}

func TestAddModernCommentRoundTrip(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := firstSlide(t, p)
	if len(s.Comments()) != 0 {
		t.Fatalf("expected no comments initially")
	}

	c := s.AddCommentAt(1000, 2000, "Ada Lovelace", "First note")
	if c == nil {
		t.Fatal("AddCommentAt returned nil")
	}
	reply := c.Reply("Grace Hopper", "Agreed")
	if reply == nil {
		t.Fatal("Reply returned nil")
	}
	c.Resolve()

	rp := reopen(t, p)
	rs := firstSlide(t, rp)
	comments := rs.Comments()
	if len(comments) != 1 {
		t.Fatalf("after reopen got %d comments, want 1", len(comments))
	}
	got := comments[0]
	if got.Author() != "Ada Lovelace" || got.Text() != "First note" {
		t.Errorf("comment = %q by %q", got.Text(), got.Author())
	}
	if !got.Resolved() {
		t.Errorf("comment should be resolved after reopen")
	}
	if x, y := got.Position(); x != 1000 || y != 2000 {
		t.Errorf("pos = (%d,%d), want (1000,2000)", x, y)
	}
	if len(got.Replies()) != 1 {
		t.Fatalf("got %d replies, want 1", len(got.Replies()))
	}
	if got.Replies()[0].Author() != "Grace Hopper" || got.Replies()[0].Text() != "Agreed" {
		t.Errorf("reply = %q by %q", got.Replies()[0].Text(), got.Replies()[0].Author())
	}

	// Authors part and relationships must exist after reopen.
	if rp.rawPartData(modernAuthorsPart) == nil {
		t.Errorf("authors.xml missing after reopen")
	}
	names := rp.modernAuthorNames()
	if len(names) != 2 {
		t.Errorf("expected 2 registered authors, got %d", len(names))
	}
	foundAuthorsRel := false
	for _, rel := range rp.relationships["/ppt/presentation.xml"] {
		if rel.Type == opc.RelTypeAuthors {
			foundAuthorsRel = true
		}
	}
	if !foundAuthorsRel {
		t.Errorf("presentation -> authors relationship missing")
	}
}

// TestAuthorDedup verifies a repeated author name reuses one author entry.
func TestAuthorDedup(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := firstSlide(t, p)
	c1 := s.AddComment("Ada Lovelace", "one")
	s.AddComment("Ada Lovelace", "two")
	c1.Reply("Ada Lovelace", "three")

	names := p.modernAuthorNames()
	if len(names) != 1 {
		t.Errorf("expected 1 deduped author, got %d: %v", len(names), names)
	}
	if p.SlideCount() == 0 {
		t.Fatal("no slides")
	}
	if len(s.Comments()) != 2 {
		t.Errorf("expected 2 top-level comments, got %d", len(s.Comments()))
	}
}

// TestAddCommentPreservesContent ensures adding a comment does not clobber
// existing slide shapes or notes.
func TestAddCommentPreservesContent(t *testing.T) {
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := firstSlide(t, p)
	shapesBefore := len(s.Shapes())

	before, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	notesBefore := bytes.Contains(before, []byte("notesSlide"))

	s.AddComment("Reviewer", "please expand this")
	rp := reopen(t, p)
	rs := firstSlide(t, rp)

	if got := len(rs.Shapes()); got != shapesBefore {
		t.Errorf("shape count changed: before %d after %d", shapesBefore, got)
	}
	after, err := rp.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if notesBefore && !bytes.Contains(after, []byte("notesSlide")) {
		t.Errorf("notes slide clobbered by comment add")
	}
	if len(rs.Comments()) != 1 {
		t.Errorf("expected 1 comment after reopen, got %d", len(rs.Comments()))
	}
}

// TestModernCommentByteIdentical verifies a deck carrying modern comments saves
// byte-identically on a zero-modification round-trip.
func TestModernCommentByteIdentical(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := firstSlide(t, p)
	c := s.AddComment("Ada Lovelace", "note")
	c.Reply("Grace Hopper", "reply")

	b1, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	// Reopen and save again with no modifications: must be byte-identical.
	rp, err := OpenReader(bytes.NewReader(b1), int64(len(b1)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	b2, err := rp.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes 2: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Errorf("zero-modification round-trip of a modern-comment deck not byte-identical (%d vs %d bytes)", len(b1), len(b2))
	}
}

// TestModifyModernThreadPreservesRawChildren verifies that resolving/replying to
// a rich modern comment read from disk preserves the data this library does not
// model (anchor marker list, position, existing reply body).
func TestModifyModernThreadPreservesRawChildren(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := firstSlide(t, p)
	p.otherParts[modernAuthorsPart] = &coxml.RawPart{ContentType: opc.ContentTypeAuthors, Data: []byte(modernAuthorsXML)}
	injectSlidePart(p, s, "/ppt/comments/modernComment1.xml", opc.ContentTypeModernComments, opc.RelTypeModernComments, modernThreadXML)

	top := s.Comments()[0]
	top.SetResolved(false)            // was resolved in the fixture
	top.Reply("Grace Hopper", "Ping") // add a second reply

	raw := string(p.otherParts["/ppt/comments/modernComment1.xml"].Data)
	for _, want := range []string{
		"pc:sldMkLst", `sldId="256"`, // anchor marker preserved
		`<p188:pos x="4572000" y="2286000"/>`, // position preserved
		"Solved!",                             // original reply body preserved
		"Ping",                                // new reply added
	} {
		if !bytes.Contains([]byte(raw), []byte(want)) {
			t.Errorf("regenerated thread missing %q\n%s", want, raw)
		}
	}
	if bytes.Contains([]byte(raw), []byte(`status="resolved"`)) {
		t.Errorf("status should be cleared after SetResolved(false)")
	}

	rp := reopen(t, p)
	got := firstSlide(t, rp).Comments()[0]
	if got.Resolved() {
		t.Errorf("comment should be unresolved after reopen")
	}
	if len(got.Replies()) != 2 {
		t.Errorf("expected 2 replies after reopen, got %d", len(got.Replies()))
	}
}

// TestValidateCommentAuthor flags a comment whose author id resolves to nothing.
func TestValidateCommentAuthor(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := firstSlide(t, p)
	// Legacy comment referencing author id 0, but no commentAuthors part.
	injectSlidePart(p, s, "/ppt/comments/comment1.xml", opc.ContentTypePresentationComments, opc.RelTypeComments, legacyCommentXML)

	report := p.Validate()
	found := false
	for _, f := range report {
		if f.Code == codeCommentNoAuthor {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a %q warning, got %+v", codeCommentNoAuthor, report)
	}
}

func TestPresentationComments(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := p.Comments(); len(got) != 0 {
		t.Fatalf("expected no comments, got %d", len(got))
	}
	for i := 0; i < p.SlideCount(); i++ {
		s, _ := p.Slide(i)
		s.AddComment("Author", "c")
	}
	if got := len(p.Comments()); got != p.SlideCount() {
		t.Errorf("Presentation.Comments() = %d, want %d", got, p.SlideCount())
	}
}
