package docx

import (
	"strings"
	"testing"
)

// TestRunAddCommentRefusesNestedRun pins C403: Run.AddComment discarded the
// bool from AddCommentAroundRun *after* addCommentModel had already appended to
// comments.xml, commentsExtended and people and flagged all three modified. A
// run from Hyperlink.Runs() is not a direct paragraph child, so the anchor
// silently failed and the document was saved carrying a comment Word never
// displays, with no range markers and no error to the caller.
func TestRunAddCommentRefusesNestedRun(t *testing.T) {
	body := `<w:body><w:p><w:hyperlink r:id="rIdX">` +
		`<w:r><w:t>linked</w:t></w:r></w:hyperlink></w:p></w:body>`
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, body))
	links := doc.Hyperlinks()
	if len(links) != 1 || len(links[0].Runs()) != 1 {
		t.Fatalf("fixture: got %d hyperlinks", len(links))
	}
	if c := links[0].Runs()[0].AddComment("A", "note"); c != nil {
		t.Error("Run.AddComment on a hyperlink run returned a handle; want nil")
	}
	if n := len(doc.Comments()); n != 0 {
		t.Errorf("comments model gained %d orphan entries; want 0", n)
	}
	saved := saveDoc(t, doc)
	if _, ok := zipEntry(t, saved, "word/comments.xml"); ok {
		t.Error("a comments part was written for a comment that was never anchored")
	}
	docXML := mustZipEntry(t, saved, "word/document.xml")
	if strings.Contains(docXML, "commentRangeStart") {
		t.Error("document.xml gained a comment range marker")
	}
}

// TestRunAddCommentOnDirectRunStillWorks guards that the C403 pre-check does
// not refuse the ordinary case.
func TestRunAddCommentOnDirectRunStillWorks(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddText("hello")
	c := r.AddComment("A", "note")
	if c == nil {
		t.Fatal("Run.AddComment on a direct child run returned nil")
	}
	if got := c.AnchorText(); got != "hello" {
		t.Errorf("AnchorText() = %q, want %q", got, "hello")
	}
}

// reversedRangeFixture is a two-paragraph body used by the C404 repros.
const reversedRangeFixture = `<w:body>` +
	`<w:p><w:r><w:t>first</w:t></w:r></w:p>` +
	`<w:p><w:r><w:t>second</w:t></w:r></w:p>` +
	`</w:body>`

// TestAddCommentOnRangeReversedEndpoints pins C404: passing the endpoints in
// reverse document order emitted commentRangeEnd before commentRangeStart —
// markup Validate() reports nothing about and AnchorText() reads back wrong.
func TestAddCommentOnRangeReversedEndpoints(t *testing.T) {
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, reversedRangeFixture))
	paras := doc.Paragraphs()
	if len(paras) != 2 {
		t.Fatalf("fixture: got %d paragraphs", len(paras))
	}
	// start in paragraph 2, end in paragraph 1 — the wrong way round.
	c := doc.AddCommentOnRange(paras[1].Runs()[0], paras[0].Runs()[0], "A", "note")
	if c == nil {
		t.Fatal("AddCommentOnRange returned nil for two resolvable endpoints")
	}
	if got, want := c.AnchorText(), "firstsecond"; got != want {
		t.Errorf("AnchorText() = %q, want %q", got, want)
	}
	docXML := mustZipEntry(t, saveDoc(t, doc), "word/document.xml")
	assertMarkerOrder(t, docXML, "commentRangeStart", "commentRangeEnd")
}

// TestAddBookmarkOnRangeReversedEndpoints is the bookmark half of C404.
func TestAddBookmarkOnRangeReversedEndpoints(t *testing.T) {
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, reversedRangeFixture))
	paras := doc.Paragraphs()
	b := doc.AddBookmarkOnRange("Span", paras[1].Runs()[0], paras[0].Runs()[0])
	if b == nil {
		t.Fatal("AddBookmarkOnRange returned nil for two resolvable endpoints")
	}
	if got, want := b.Text(), "firstsecond"; got != want {
		t.Errorf("Bookmark.Text() = %q, want %q", got, want)
	}
	docXML := mustZipEntry(t, saveDoc(t, doc), "word/document.xml")
	assertMarkerOrder(t, docXML, "bookmarkStart", "bookmarkEnd")
}

// TestAddCommentOnRangeForwardEndpointsUnchanged guards that the ordering check
// leaves a correctly ordered pair alone.
func TestAddCommentOnRangeForwardEndpointsUnchanged(t *testing.T) {
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, reversedRangeFixture))
	paras := doc.Paragraphs()
	c := doc.AddCommentOnRange(paras[0].Runs()[0], paras[1].Runs()[0], "A", "note")
	if c == nil {
		t.Fatal("AddCommentOnRange returned nil")
	}
	if got, want := c.AnchorText(), "firstsecond"; got != want {
		t.Errorf("AnchorText() = %q, want %q", got, want)
	}
}

// TestAddCommentOnRangeReversedWithinParagraph covers the same-paragraph
// inversion, which the paragraph-index comparison alone cannot catch.
func TestAddCommentOnRangeReversedWithinParagraph(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:t>alpha</w:t></w:r><w:r><w:t>beta</w:t></w:r>` +
		`</w:p></w:body>`
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, body))
	runs := doc.Paragraphs()[0].Runs()
	c := doc.AddCommentOnRange(runs[1], runs[0], "A", "note")
	if c == nil {
		t.Fatal("AddCommentOnRange returned nil")
	}
	if got, want := c.AnchorText(), "alphabeta"; got != want {
		t.Errorf("AnchorText() = %q, want %q", got, want)
	}
	docXML := mustZipEntry(t, saveDoc(t, doc), "word/document.xml")
	assertMarkerOrder(t, docXML, "commentRangeStart", "commentRangeEnd")
}

// assertMarkerOrder fails when first does not precede last in xml.
func assertMarkerOrder(t *testing.T, xml, first, last string) {
	t.Helper()
	i := strings.Index(xml, first)
	j := strings.Index(xml, last)
	if i < 0 || j < 0 {
		t.Fatalf("missing markers %q (%d) / %q (%d) in:\n%s", first, i, last, j, xml)
	}
	if i > j {
		t.Errorf("%s appears after %s:\n%s", first, last, xml)
	}
}
