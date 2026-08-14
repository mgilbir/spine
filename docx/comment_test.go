package docx

import (
	"bytes"
	"strings"
	"testing"
)

// fixtureCommentCT declares the content types for a comment-bearing package,
// including the commentsExtended override.
const fixtureCommentCT = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/comments.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"/><Override PartName="/word/commentsExtended.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.commentsExtended+xml"/></Types>`

const fixtureCommentDocRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/><Relationship Id="rId2" Type="http://schemas.microsoft.com/office/2011/relationships/commentsExtended" Target="commentsExtended.xml"/></Relationships>`

const fixtureCommentNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml"`

const fixtureCommentDocument = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<w:document ` + fixtureCommentNS + `><w:body><w:p w14:paraId="00000001" w14:textId="00000001">` +
	`<w:commentRangeStart w:id="0"/><w:commentRangeStart w:id="1"/>` +
	`<w:r><w:t>commented text</w:t></w:r>` +
	`<w:commentRangeEnd w:id="1"/><w:commentRangeEnd w:id="0"/>` +
	`<w:r><w:rPr><w:rStyle w:val="CommentReference"/></w:rPr><w:commentReference w:id="0"/></w:r>` +
	`<w:r><w:rPr><w:rStyle w:val="CommentReference"/></w:rPr><w:commentReference w:id="1"/></w:r>` +
	`</w:p></w:body></w:document>`

const fixtureCommentComments = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:comments ` + fixtureCommentNS + `><w:comment w:id="0" w:author="Alice" w:date="2021-01-01T00:00:00Z" w:initials="A"><w:p w14:paraId="0000A001"><w:pPr><w:pStyle w:val="CommentText"/></w:pPr><w:r><w:t>First comment</w:t></w:r></w:p></w:comment><w:comment w:id="1" w:author="Bob" w:date="2021-01-02T00:00:00Z" w:initials="B"><w:p w14:paraId="0000A002"><w:pPr><w:pStyle w:val="CommentText"/></w:pPr><w:r><w:t>A reply</w:t></w:r></w:p></w:comment></w:comments>`

const fixtureCommentExtended = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w15:commentsEx ` + fixtureCommentNS + `><w15:commentEx w15:paraId="0000A001" w15:done="0"/><w15:commentEx w15:paraId="0000A002" w15:paraIdParent="0000A001" w15:done="1"/></w15:commentsEx>`

func fixtureCommentDocx(t *testing.T) []byte {
	t.Helper()
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          fixtureCommentCT,
		"_rels/.rels":                  fixtureRootRels,
		"word/_rels/document.xml.rels": fixtureCommentDocRels,
		"word/document.xml":            fixtureCommentDocument,
		"word/comments.xml":            fixtureCommentComments,
		"word/commentsExtended.xml":    fixtureCommentExtended,
	})
}

// TestCommentsRead verifies reading authors, text, thread links, resolved
// state, and anchor text from a crafted comment-bearing document.
func TestCommentsRead(t *testing.T) {
	doc := openFixture(t, fixtureCommentDocx(t))
	defer func() { _ = doc.Close() }()

	comments := doc.Comments()
	if len(comments) != 1 {
		t.Fatalf("Comments() = %d, want 1 (top-level thread root)", len(comments))
	}

	parent := comments[0]
	if len(parent.Replies()) != 1 {
		t.Fatalf("parent.Replies() = %d, want 1", len(parent.Replies()))
	}
	reply := parent.Replies()[0]
	if parent.Author() != "Alice" || parent.Text() != "First comment" {
		t.Errorf("parent = %q/%q, want Alice/First comment", parent.Author(), parent.Text())
	}
	if parent.Initials() != "A" {
		t.Errorf("parent initials = %q, want A", parent.Initials())
	}
	if parent.Resolved() {
		t.Errorf("parent should not be resolved")
	}
	if parent.Parent() != nil {
		t.Errorf("parent.Parent() should be nil")
	}
	if got := parent.Date(); got.IsZero() {
		t.Errorf("parent.Date() is zero, want parsed timestamp")
	}
	if replies := parent.Replies(); len(replies) != 1 || replies[0].ID() != reply.ID() {
		t.Errorf("parent.Replies() = %v, want [reply]", replies)
	}
	if got := parent.AnchorText(); got != "commented text" {
		t.Errorf("parent.AnchorText() = %q, want %q", got, "commented text")
	}

	if reply.Author() != "Bob" || reply.Text() != "A reply" {
		t.Errorf("reply = %q/%q, want Bob/A reply", reply.Author(), reply.Text())
	}
	if !reply.Resolved() {
		t.Errorf("reply should be resolved (done=1)")
	}
	if p := reply.Parent(); p == nil || p.ID() != parent.ID() {
		t.Errorf("reply.Parent() = %v, want parent", p)
	}
	if got := reply.AnchorText(); got != "commented text" {
		t.Errorf("reply.AnchorText() = %q, want %q", got, "commented text")
	}
}

// TestCommentsRoundTripByteIdentical verifies a zero-modification save of a
// comment-bearing document leaves every comment part and the document byte
// identical.
func TestCommentsRoundTripByteIdentical(t *testing.T) {
	fixture := fixtureCommentDocx(t)
	doc := openFixture(t, fixture)
	saved := saveDoc(t, doc)
	_ = doc.Close()

	for _, part := range []string{
		"word/document.xml",
		"word/comments.xml",
		"word/commentsExtended.xml",
	} {
		orig, ok1 := zipEntry(t, fixture, part)
		got, ok2 := zipEntry(t, saved, part)
		if !ok1 || !ok2 {
			t.Fatalf("%s missing (orig=%v saved=%v)", part, ok1, ok2)
		}
		if !bytes.Equal(orig, got) {
			t.Errorf("%s not byte-identical after zero-mod save:\n--- orig ---\n%s\n--- got ---\n%s", part, orig, got)
		}
	}
}

// TestCommentsAddReplyResolve creates a document, adds a comment, replies, and
// resolves, then reopens the saved package and asserts the full structure.
func TestCommentsAddReplyResolve(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("The quick brown fox")
	c := p.AddComment("Reviewer One", "Please rephrase this.")
	reply, err := c.Reply("Reviewer Two", "Agreed, will fix.")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if err := reply.Resolve(); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	saved := saveDoc(t, doc)

	// Parts, relationships, and content types must all be present.
	comments := mustZipEntry(t, saved, "word/comments.xml")
	if !strings.Contains(comments, "Please rephrase this.") || !strings.Contains(comments, "Agreed, will fix.") {
		t.Fatalf("comments.xml missing bodies:\n%s", comments)
	}
	ext := mustZipEntry(t, saved, "word/commentsExtended.xml")
	if !strings.Contains(ext, "paraIdParent") {
		t.Fatalf("commentsExtended.xml missing threading:\n%s", ext)
	}
	if !strings.Contains(ext, `w15:done="1"`) {
		t.Fatalf("commentsExtended.xml missing resolved state:\n%s", ext)
	}
	people := mustZipEntry(t, saved, "word/people.xml")
	if !strings.Contains(people, "Reviewer One") || !strings.Contains(people, "Reviewer Two") {
		t.Fatalf("people.xml missing authors:\n%s", people)
	}
	rels := mustZipEntry(t, saved, "word/_rels/document.xml.rels")
	for _, want := range []string{"comments.xml", "commentsExtended.xml", "people.xml"} {
		if !strings.Contains(rels, want) {
			t.Fatalf("document rels missing %s:\n%s", want, rels)
		}
	}
	ct := mustZipEntry(t, saved, "[Content_Types].xml")
	for _, want := range []string{"commentsExtended", "people"} {
		if !strings.Contains(ct, want) {
			t.Fatalf("[Content_Types].xml missing %s override:\n%s", want, ct)
		}
	}
	docXML := mustZipEntry(t, saved, "word/document.xml")
	if !strings.Contains(docXML, "commentRangeStart") || !strings.Contains(docXML, "commentReference") {
		t.Fatalf("document.xml missing range markers:\n%s", docXML)
	}

	// Reopen and assert the structure.
	doc2 := openFixture(t, saved)
	defer func() { _ = doc2.Close() }()
	got := doc2.Comments()
	if len(got) != 1 {
		t.Fatalf("reopened Comments() = %d, want 1 (top-level thread root)", len(got))
	}
	root := got[0]
	if root.Author() != "Reviewer One" || root.Text() != "Please rephrase this." {
		t.Errorf("root = %q/%q", root.Author(), root.Text())
	}
	if root.Initials() != "RO" {
		t.Errorf("derived initials = %q, want RO", root.Initials())
	}
	replies := root.Replies()
	if len(replies) != 1 {
		t.Fatalf("root.Replies() = %d, want 1", len(replies))
	}
	if replies[0].Author() != "Reviewer Two" {
		t.Errorf("reply author = %q, want Reviewer Two", replies[0].Author())
	}
	if !replies[0].Resolved() || !root.Resolved() {
		t.Errorf("thread should be resolved (root=%v reply=%v)", root.Resolved(), replies[0].Resolved())
	}
	if got := root.AnchorText(); got != "The quick brown fox" {
		t.Errorf("root.AnchorText() = %q, want whole paragraph", got)
	}
}

// TestCommentChildOrderPreserved verifies that adding a comment to a paragraph
// that already has runs and a hyperlink keeps every child in order and
// serialized exactly once.
func TestCommentChildOrderPreserved(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:t>alpha </w:t></w:r>` +
		`<w:hyperlink r:id="rIdX"><w:r><w:t>beta</w:t></w:r></w:hyperlink>` +
		`<w:r><w:t> gamma</w:t></w:r>` +
		`</w:p></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openFixture(t, fixture)

	paras := doc.Paragraphs()
	if len(paras) != 1 {
		t.Fatalf("got %d paragraphs", len(paras))
	}
	paras[0].AddComment("Ann", "note")

	saved := saveDoc(t, doc)
	docXML := mustZipEntry(t, saved, "word/document.xml")

	// The original content survives in order, bracketed by the range markers.
	for _, want := range []string{"alpha ", "beta", " gamma"} {
		if strings.Count(docXML, ">"+want+"<") > 1 {
			t.Errorf("%q serialized more than once:\n%s", want, docXML)
		}
	}
	startIdx := strings.Index(docXML, "commentRangeStart")
	alphaIdx := strings.Index(docXML, "alpha")
	endIdx := strings.Index(docXML, "commentRangeEnd")
	if startIdx < 0 || endIdx < 0 || startIdx >= alphaIdx || alphaIdx >= endIdx {
		t.Fatalf("range markers not bracketing content:\n%s", docXML)
	}

	// Anchor text spans the whole paragraph including the hyperlink.
	doc2 := openFixture(t, saved)
	defer func() { _ = doc2.Close() }()
	comments := doc2.Comments()
	if len(comments) != 1 {
		t.Fatalf("reopened Comments() = %d, want 1", len(comments))
	}
	if got := comments[0].AnchorText(); got != "alpha beta gamma" {
		t.Errorf("AnchorText() = %q, want %q", got, "alpha beta gamma")
	}
}
