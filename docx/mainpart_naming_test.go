package docx

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// C61: with the main part at a non-standard name (e.g. /word/document2.xml),
// save wrote the regenerated body to a hardcoded /word/document.xml while the
// stale original part and the rels pointing at it were preserved — every edit
// was silently lost.
func TestNonStandardMainPartName(t *testing.T) {
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document2.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document2.xml"/></Relationships>`,
		"word/document2.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>original</w:t></w:r></w:p></w:body></w:document>`,
	})
	doc := openFixture(t, fixture)
	doc.AddParagraphWithText("edited content")
	saved := saveDoc(t, doc)

	main := mustZipEntry(t, saved, "word/document2.xml")
	if !strings.Contains(main, "edited content") {
		t.Fatalf("edit lost: document2.xml not regenerated:\n%s", main)
	}
	if !strings.Contains(main, "original") {
		t.Fatalf("original content lost:\n%s", main)
	}
	if _, ok := zipEntry(t, saved, "word/document.xml"); ok {
		t.Fatal("orphan /word/document.xml written alongside document2.xml")
	}
	rootRels := mustZipEntry(t, saved, "_rels/.rels")
	if !strings.Contains(rootRels, `Target="word/document2.xml"`) {
		t.Fatalf("root relationship no longer points at document2.xml:\n%s", rootRels)
	}

	// The edit must be visible after reopening.
	doc2 := openFixture(t, saved)
	if body := doc2.Body(); !strings.Contains(body, "edited content") {
		t.Fatalf("edit not visible after reopen: %q", body)
	}
}

// C61: relationships added on a non-standard main part (e.g. by a list) must
// land in that part's rels, not /word/_rels/document.xml.rels.
func TestNonStandardMainPartRelationships(t *testing.T) {
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document2.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document2.xml"/></Relationships>`,
		"word/document2.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
	})
	doc := openFixture(t, fixture)
	list := doc.AddBulletList()
	doc.AddParagraphWithText("item").SetListStyle(list, 0)
	saved := saveDoc(t, doc)

	rels := mustZipEntry(t, saved, "word/_rels/document2.xml.rels")
	if !strings.Contains(rels, `Target="numbering.xml"`) {
		t.Fatalf("numbering relationship missing from document2 rels:\n%s", rels)
	}
	if _, ok := zipEntry(t, saved, "word/_rels/document.xml.rels"); ok {
		t.Fatal("rels written for the wrong main part name")
	}
}

// Wave-2 finding: preserved parts were written in map-iteration order, so
// zip entry order (and therefore whole-archive bytes) differed between runs.
//
// Modified is pinned because each build() is a separate *edited* session, and an
// edit stamps the write time: without it the subject of the test would be the
// wall clock, and two builds either side of a second boundary would differ for a
// reason that has nothing to do with entry order. An explicit assignment wins
// over the automatic stamp. Idempotent saving is pinned by
// TestSavesAfterEditAreByteIdentical, which is where it belongs.
func TestSaveIsDeterministic(t *testing.T) {
	pinned := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	build := func() []byte {
		doc, err := Open("testdata/svg_test.docx")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = doc.Close() }()
		doc.Properties.Modified = pinned
		doc.AddParagraphWithText("determinism probe")
		saved, err := doc.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		return saved
	}
	first := build()
	for i := 0; i < 4; i++ {
		if !bytes.Equal(first, build()) {
			t.Fatalf("save %d produced different archive bytes", i+2)
		}
	}
}
