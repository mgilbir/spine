package docx

import (
	"strings"
	"testing"
)

// peopleFixture builds a docx with a comments part, a commentsExtended part and
// a people.xml whose w15:person carries an attribute the model does not type.
func peopleFixture(t *testing.T) []byte {
	t.Helper()
	const ct = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/comments.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"/><Override PartName="/word/commentsExtended.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.commentsExtended+xml"/><Override PartName="/word/people.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.people+xml"/></Types>`
	const rels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/><Relationship Id="rId2" Type="http://schemas.microsoft.com/office/2011/relationships/commentsExtended" Target="commentsExtended.xml"/><Relationship Id="rId3" Type="http://schemas.microsoft.com/office/2011/relationships/people" Target="people.xml"/></Relationships>`
	const w15 = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml" ` +
		`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`
	const documentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `><w:body>` +
		`<w:p w14:paraId="0000AAAA" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">` +
		`<w:commentRangeStart w:id="1"/><w:r><w:t>anchored</w:t></w:r><w:commentRangeEnd w:id="1"/>` +
		`<w:r><w:commentReference w:id="1"/></w:r></w:p>` +
		`</w:body></w:document>`
	const comments = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:comments ` + w15 + `>` +
		`<w:comment w:id="1" w:author="Ann" w:initials="A" w:date="2024-01-01T00:00:00Z">` +
		`<w:p w14:paraId="0000BBBB"><w:r><w:t>root</w:t></w:r></w:p></w:comment>` +
		`</w:comments>`
	const commentsExt = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w15:commentsEx ` + w15 + `><w15:commentEx w15:paraId="0000BBBB" w15:done="0"/></w15:commentsEx>`
	const people = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w15:people ` + w15 + ` mc:Ignorable="w14 w15" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006">` +
		`<w15:person w15:author="Ann" w15:custom="keep-me">` +
		`<w15:presenceInfo w15:providerId="AD" w15:userId="ann@example.com"/>` +
		`</w15:person></w15:people>`

	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          ct,
		"_rels/.rels":                  fixtureRootRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": rels,
		"word/comments.xml":            comments,
		"word/commentsExtended.xml":    commentsExt,
		"word/people.xml":              people,
	})
}

// TestReplyByKnownAuthorLeavesPeopleAlone pins the second half of C500:
// addCommentModel flagged people.xml modified unconditionally, so a reply by an
// author already in the registry regenerated the part for nothing.
func TestReplyByKnownAuthorLeavesPeopleAlone(t *testing.T) {
	fixture := peopleFixture(t)
	doc := openFixture(t, fixture)
	comments := doc.Comments()
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	if c, err := comments[0].Reply("Ann", "reply text"); err != nil || c == nil {
		t.Fatalf("Reply: %v (comment %v)", err, c)
	}
	saved := saveDoc(t, doc)
	orig := mustZipEntry(t, fixture, "word/people.xml")
	if got := mustZipEntry(t, saved, "word/people.xml"); got != orig {
		t.Errorf("people.xml was regenerated for a reply by a known author:\n got: %s\nwant: %s", got, orig)
	}
}

// TestNewAuthorPreservesCapturedPersonAttrs pins the first half of C500: when
// people.xml *does* have to be regenerated, the existing entries must keep the
// attributes the model does not type — CT_Person.CapturedAttrs was captured at
// parse and then never replayed.
func TestNewAuthorPreservesCapturedPersonAttrs(t *testing.T) {
	doc := openFixture(t, peopleFixture(t))
	if c, err := doc.Comments()[0].Reply("Bob", "reply text"); err != nil || c == nil {
		t.Fatalf("Reply: %v (comment %v)", err, c)
	}
	people := mustZipEntry(t, saveDoc(t, doc), "word/people.xml")
	if !strings.Contains(people, `w15:custom="keep-me"`) {
		t.Errorf("regenerated people.xml dropped the unmodeled person attribute:\n%s", people)
	}
	if !strings.Contains(people, `w15:author="Bob"`) {
		t.Errorf("regenerated people.xml is missing the new author:\n%s", people)
	}
	if !strings.Contains(people, `w15:userId="ann@example.com"`) {
		t.Errorf("regenerated people.xml lost the existing presenceInfo:\n%s", people)
	}
}

// TestNextParaIDSeesTableCellParagraphs pins C499: the uniqueness scan used the
// top-level-only walk while anchoring and threading were moved to the
// descend-into-tables walk by C267, so a table-cell paragraph's existing
// w14:paraId was not in the used set.
func TestNextParaIDSeesTableCellParagraphs(t *testing.T) {
	body := `<w:body><w:tbl><w:tr><w:tc>` +
		`<w:p w14:paraId="0000CCCC" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">` +
		`<w:r><w:t>cell</w:t></w:r></w:p>` +
		`</w:tc></w:tr></w:tbl></w:body>`
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, body))
	if !doc.usedParaIDs()["0000CCCC"] {
		t.Error("the paraId uniqueness scan does not see table-cell paragraphs")
	}
}

// TestNextParaIDSeesHeaderParagraphs extends the C499 scan to header and footer
// paragraphs, which carry w14:paraId just as body paragraphs do and which the
// mutators reachable through Header.Paragraphs() can now add comments to.
func TestNextParaIDSeesHeaderParagraphs(t *testing.T) {
	const w14 = ` xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`
	doc := openFixture(t, revisionHeaderFixture(t,
		`<w:p w14:paraId="0000DDDD"`+w14+`><w:r><w:t>HEADER</w:t></w:r></w:p>`))
	if !doc.usedParaIDs()["0000DDDD"] {
		t.Error("the paraId uniqueness scan does not see header paragraphs")
	}
}
