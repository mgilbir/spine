package docx

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRevisionAcceptStaleHandleErrors pins C494: the transforms report false
// when the revision's block is no longer a direct child of its container —
// exactly the invalidation Revisions' godoc warns about after a prior Accept
// rebuilt the container. Accept discarded that bool, returned nil, and still
// called markModified, so a stale handle "succeeded" having done nothing.
func TestRevisionAcceptStaleHandleErrors(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:ins w:id="1" w:author="A" w:date="2024-01-01T00:00:00Z">` +
		`<w:r><w:t>one</w:t></w:r></w:ins>` +
		`<w:ins w:id="2" w:author="A" w:date="2024-01-01T00:00:00Z">` +
		`<w:r><w:t>two</w:t></w:r></w:ins>` +
		`</w:p></w:body>`
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, body))
	revs := doc.Revisions()
	if len(revs) != 2 {
		t.Fatalf("got %d revisions, want 2", len(revs))
	}
	if err := revs[0].Accept(); err != nil {
		t.Fatalf("first Accept: %v", err)
	}
	// The first Accept rebuilt the paragraph's child list; accepting the same
	// handle again resolves nothing.
	if err := revs[0].Accept(); !errors.Is(err, ErrRevisionStale) {
		t.Errorf("re-accepting a consumed revision: got %v, want ErrRevisionStale", err)
	}
	if err := revs[0].Reject(); !errors.Is(err, ErrRevisionStale) {
		t.Errorf("rejecting a consumed revision: got %v, want ErrRevisionStale", err)
	}
}

// TestRevisionStaleHandleDoesNotFlagHeader is the damaging half of C494: a
// no-op transform must not flag a header part for regeneration, which discards
// its preserved bytes for nothing.
func TestRevisionStaleHandleDoesNotFlagHeader(t *testing.T) {
	fixture := revisionHeaderFixture(t, `<w:p><w:ins w:id="4" w:author="A" w:date="2024-01-01T00:00:00Z">`+
		`<w:r><w:t xml:space="preserve">HDRINS</w:t></w:r></w:ins></w:p>`)
	doc := openFixture(t, fixture)
	revs := doc.Revisions()
	if len(revs) != 1 {
		t.Fatalf("got %d revisions, want 1", len(revs))
	}
	if err := revs[0].Accept(); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	doc.modifiedHdrFtrParts = nil
	if err := revs[0].Accept(); !errors.Is(err, ErrRevisionStale) {
		t.Fatalf("re-accept: got %v, want ErrRevisionStale", err)
	}
	if len(doc.modifiedHdrFtrParts) != 0 {
		t.Errorf("a no-op Accept flagged %v for regeneration", doc.modifiedHdrFtrParts)
	}
}

// TestRevisionsReportSectionAndSdtTableChanges pins C495: only the body-level
// w:sectPrChange was reported, so a mid-document section break edited under
// track changes was invisible; and structural table revisions inside a
// body-level content control were skipped by the enumeration though
// MaxRevisionID walked them for id allocation.
func TestRevisionsReportSectionAndSdtTableChanges(t *testing.T) {
	body := `<w:body>` +
		// mid-document section break with a tracked property change
		`<w:p><w:pPr><w:sectPr>` +
		`<w:sectPrChange w:id="11" w:author="Mid" w:date="2024-01-01T00:00:00Z"><w:sectPr/></w:sectPrChange>` +
		`</w:sectPr></w:pPr></w:p>` +
		// a tracked table revision inside a block-level content control
		`<w:sdt><w:sdtPr><w:tag w:val="wrap"/></w:sdtPr><w:sdtContent>` +
		`<w:tbl><w:tblPr>` +
		`<w:tblPrChange w:id="12" w:author="Sdt" w:date="2024-01-01T00:00:00Z"><w:tblPr/></w:tblPrChange>` +
		`</w:tblPr><w:tr><w:tc><w:p/></w:tc></w:tr></w:tbl>` +
		`</w:sdtContent></w:sdt>` +
		// the body-level section change, which was always reported
		`<w:sectPr>` +
		`<w:sectPrChange w:id="13" w:author="Body" w:date="2024-01-01T00:00:00Z"><w:sectPr/></w:sectPrChange>` +
		`</w:sectPr>` +
		`</w:body>`
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, body))
	byAuthor := map[string]RevisionType{}
	for _, r := range doc.Revisions() {
		byAuthor[r.Author()] = r.Type()
	}
	for author, want := range map[string]RevisionType{
		"Mid":  RevisionSectionFormat,
		"Sdt":  RevisionTableFormat,
		"Body": RevisionSectionFormat,
	} {
		if got, ok := byAuthor[author]; !ok {
			t.Errorf("Revisions() missed the change by %q (found %v)", author, byAuthor)
		} else if got != want {
			t.Errorf("Revisions() reported %q as %v, want %v", author, got, want)
		}
	}
}

// TestRevisionIDSeedCoversHeaders pins C496: the id seed scanned the body only,
// so authoring into a document whose header carries tracked changes reused
// those ids — even though Revisions() enumerates header revisions, so they are
// visibly in the same id space.
func TestRevisionIDSeedCoversHeaders(t *testing.T) {
	fixture := revisionHeaderFixture(t, `<w:p><w:ins w:id="500" w:author="A" w:date="2024-01-01T00:00:00Z">`+
		`<w:r><w:t xml:space="preserve">HDRINS</w:t></w:r></w:ins></w:p>`)
	doc := openFixture(t, fixture)
	doc.AddParagraph().AddInsertedRunWithDate("B", "body", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	// The authored insertion's w:id must be above every id already in the
	// package, the header's included.
	docXML := mustZipEntry(t, saveDoc(t, doc), "word/document.xml")
	id := authoredInsertionID(t, docXML)
	if id <= 500 {
		t.Errorf("authored revision id %d collides with the header's id 500", id)
	}
}

// authoredInsertionID returns the w:id of the w:ins element in document.xml.
func authoredInsertionID(t *testing.T, docXML string) int {
	t.Helper()
	const marker = `<w:ins w:id="`
	i := strings.Index(docXML, marker)
	if i < 0 {
		t.Fatalf("no w:ins in document.xml:\n%s", docXML)
	}
	rest := docXML[i+len(marker):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		t.Fatalf("malformed w:ins id in:\n%s", docXML)
	}
	id, err := strconv.Atoi(rest[:j])
	if err != nil {
		t.Fatalf("non-numeric w:ins id %q", rest[:j])
	}
	return id
}

// revisionHeaderFixture builds a docx whose default section references a header
// carrying the given body content.
func revisionHeaderFixture(t *testing.T, headerBody string) []byte {
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
	headerXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:hdr ` + fixtureWNS + `>` + headerBody + `</w:hdr>`

	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  fixtureRootRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": documentRels,
		"word/header1.xml":             headerXML,
	})
}
