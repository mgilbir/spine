package docx

import (
	"bytes"
	"strings"
	"testing"
)

// hdrFtrRevisionFixture builds a docx whose default section references a header
// part that carries a tracked insertion, so header revisions can be exercised.
func hdrFtrRevisionFixture(t *testing.T) []byte {
	t.Helper()
	const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`
	const documentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `><w:body>` +
		`<w:p><w:r><w:t xml:space="preserve">main body</w:t></w:r></w:p>` +
		`<w:sectPr><w:headerReference w:type="default" r:id="rId10"/></w:sectPr>` +
		`</w:body></w:document>`
	const documentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/></Relationships>`
	const headerXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:hdr ` + fixtureWNS + `><w:p><w:r><w:t xml:space="preserve">keep </w:t></w:r>` +
		`<w:ins w:id="1" w:author="Ann" w:date="2021-01-02T03:04:05Z"><w:r><w:t>added</w:t></w:r></w:ins>` +
		`</w:p></w:hdr>`
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  fixtureRootRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": documentRels,
		"word/header1.xml":             headerXML,
	})
}

func openHdrFtrRevisionDoc(t *testing.T) *Document {
	t.Helper()
	fixture := hdrFtrRevisionFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open header fixture: %v", err)
	}
	return doc
}

// TestHeaderRevisionEnumeration verifies a tracked change in a header part is
// enumerated by Document.Revisions.
func TestHeaderRevisionEnumeration(t *testing.T) {
	doc := openHdrFtrRevisionDoc(t)
	revs := doc.Revisions()
	if len(revs) != 1 {
		t.Fatalf("want 1 header revision, got %d: %+v", len(revs), revs)
	}
	r := revs[0]
	if r.Type() != RevisionInsertion || r.Author() != "Ann" || r.Text() != "added" {
		t.Fatalf("header revision wrong: (%s,%s,%q)", r.Type(), r.Author(), r.Text())
	}
}

// TestHeaderRevisionAcceptRegeneratesPart accepts the header revision and
// verifies the saved header part drops the w:ins marker while keeping the text.
func TestHeaderRevisionAcceptRegeneratesPart(t *testing.T) {
	doc := openHdrFtrRevisionDoc(t)
	if err := doc.AcceptAllRevisions(); err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	hdr, ok := zipEntry(t, saved, "word/header1.xml")
	if !ok {
		t.Fatal("word/header1.xml missing from saved package")
	}
	s := string(hdr)
	if strings.Contains(s, "w:ins") {
		t.Errorf("accepted header insertion still wrapped:\n%s", s)
	}
	if !strings.Contains(s, "added") {
		t.Errorf("accepted header text lost:\n%s", s)
	}
}

// TestHeaderRevisionRejectRemovesContent rejects the header revision and
// verifies the inserted text is removed from the saved header part.
func TestHeaderRevisionRejectRemovesContent(t *testing.T) {
	doc := openHdrFtrRevisionDoc(t)
	revs := doc.Revisions()
	if len(revs) != 1 {
		t.Fatalf("want 1 header revision, got %d", len(revs))
	}
	if err := revs[0].Reject(); err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	hdr, _ := zipEntry(t, saved, "word/header1.xml")
	s := string(hdr)
	if strings.Contains(s, "w:ins") || strings.Contains(s, "added") {
		t.Errorf("rejected header insertion not removed:\n%s", s)
	}
	if !strings.Contains(s, "keep ") {
		t.Errorf("rejected header lost surrounding text:\n%s", s)
	}
}
