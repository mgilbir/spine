package docx

import (
	"strings"
	"testing"
)

// goBackFixture builds a docx whose body carries a bookmark with the given name
// and id around one paragraph, plus a second named bookmark so the collision is
// observable independently.
func goBackFixture(t *testing.T, name, id, text string) []byte {
	body := `<w:body>` +
		`<w:p><w:bookmarkStart w:id="` + id + `" w:name="` + name + `"/>` +
		`<w:r><w:t>` + text + `</w:t></w:r>` +
		`<w:bookmarkEnd w:id="` + id + `"/></w:p>` +
		`</w:body>`
	return fixtureWithDocument(t, fixtureWNS, body)
}

// TestAppendRemapsBookmarkIDsAndNames pins C503: Append copied the source's
// bookmark markers verbatim, so two documents both carrying Word's _GoBack
// (very often id 0) produced two bookmarkStart/bookmarkEnd pairs sharing an id
// — which mispairs — and two bookmarks sharing a name, which makes an internal
// hyperlink ambiguous.
func TestAppendRemapsBookmarkIDsAndNames(t *testing.T) {
	dst := openFixture(t, goBackFixture(t, "_GoBack", "0", "destination"))
	src := openFixture(t, goBackFixture(t, "_GoBack", "0", "source"))
	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}

	marks := dst.Bookmarks()
	if len(marks) != 2 {
		t.Fatalf("got %d bookmarks after Append, want 2: %v", len(marks), bookmarkSummary(marks))
	}
	ids := map[string]bool{}
	names := map[string]bool{}
	for _, b := range marks {
		if ids[b.id] {
			t.Errorf("two bookmarks share id %q: %v", b.id, bookmarkSummary(marks))
		}
		if names[b.Name()] {
			t.Errorf("two bookmarks share name %q: %v", b.Name(), bookmarkSummary(marks))
		}
		ids[b.id] = true
		names[b.Name()] = true
	}

	// Each pair must still bracket its own text after the renumbering.
	texts := map[string]string{}
	for _, b := range marks {
		texts[b.Name()] = b.Text()
	}
	if texts["_GoBack"] != "destination" {
		t.Errorf("the destination bookmark now spans %q, want %q", texts["_GoBack"], "destination")
	}
	found := false
	for name, text := range texts {
		if name != "_GoBack" && text == "source" {
			found = true
		}
	}
	if !found {
		t.Errorf("the imported bookmark does not span its own text: %v", texts)
	}

	// The saved markup must carry matched pairs.
	docXML := mustZipEntry(t, saveDoc(t, dst), "word/document.xml")
	if got := strings.Count(docXML, "<w:bookmarkStart"); got != 2 {
		t.Errorf("got %d bookmarkStart markers, want 2:\n%s", got, docXML)
	}
	if got := strings.Count(docXML, "<w:bookmarkEnd"); got != 2 {
		t.Errorf("got %d bookmarkEnd markers, want 2:\n%s", got, docXML)
	}
}

// TestAppendKeepsDistinctBookmarkNames guards that a source bookmark whose name
// is free in the destination keeps it.
func TestAppendKeepsDistinctBookmarkNames(t *testing.T) {
	dst := openFixture(t, goBackFixture(t, "Alpha", "0", "destination"))
	src := openFixture(t, goBackFixture(t, "Beta", "0", "source"))
	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	names := map[string]bool{}
	for _, b := range dst.Bookmarks() {
		names[b.Name()] = true
	}
	if !names["Alpha"] || !names["Beta"] {
		t.Errorf("a non-colliding bookmark name was renamed: %v", names)
	}
}

// TestAppendRemapsCollidingCommentParaIDs pins the second half of C503:
// commentsExtended is keyed on w14:paraId and the source's threading metadata
// is deliberately not merged, so an imported comment whose paraId matches one of
// the destination's keys read back as resolved (or threaded under a local
// comment) that it has nothing to do with.
func TestAppendRemapsCollidingCommentParaIDs(t *testing.T) {
	const shared = "0000BBBB"
	dst := openFixture(t, commentParaIDFixture(t, shared, "local", "1"))
	src := openFixture(t, commentParaIDFixture(t, shared, "foreign", "1"))
	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}

	byText := map[string]*Comment{}
	for _, c := range dst.Comments() {
		byText[c.Text()] = c
	}
	if len(byText) != 2 {
		t.Fatalf("got %d distinct comments after Append, want 2 (%v)", len(byText), byText)
	}
	if byText["foreign"] == nil {
		t.Fatal("the imported comment is not reported as a top-level comment")
	}
	if byText["foreign"].Resolved() {
		t.Error("the imported comment reads back as resolved, inheriting the destination's commentsExtended entry")
	}
	if !byText["local"].Resolved() {
		t.Error("the destination's own comment lost its resolved state")
	}
}

// bookmarkSummary renders bookmarks as name=id pairs for failure messages.
func bookmarkSummary(marks []*Bookmark) []string {
	out := make([]string, 0, len(marks))
	for _, b := range marks {
		out = append(out, b.Name()+"="+b.id)
	}
	return out
}

// commentParaIDFixture builds a docx with one resolved comment whose body
// paragraph carries the given w14:paraId.
func commentParaIDFixture(t *testing.T, paraID, text, commentID string) []byte {
	t.Helper()
	const ct = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/comments.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"/><Override PartName="/word/commentsExtended.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.commentsExtended+xml"/></Types>`
	const rels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/><Relationship Id="rId2" Type="http://schemas.microsoft.com/office/2011/relationships/commentsExtended" Target="commentsExtended.xml"/></Relationships>`
	const w15 = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml" ` +
		`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": ct,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p>` +
			`<w:commentRangeStart w:id="` + commentID + `"/><w:r><w:t>` + text + `</w:t></w:r>` +
			`<w:commentRangeEnd w:id="` + commentID + `"/>` +
			`<w:r><w:commentReference w:id="` + commentID + `"/></w:r>` +
			`</w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": rels,
		"word/comments.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:comments ` + w15 + `><w:comment w:id="` + commentID +
			`" w:author="A" w:initials="A" w:date="2024-01-01T00:00:00Z">` +
			`<w:p w14:paraId="` + paraID + `"><w:r><w:t>` + text + `</w:t></w:r></w:p>` +
			`</w:comment></w:comments>`,
		"word/commentsExtended.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w15:commentsEx ` + w15 + `><w15:commentEx w15:paraId="` + paraID + `" w15:done="1"/></w15:commentsEx>`,
	})
}
