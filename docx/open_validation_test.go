package docx

import (
	"bytes"
	"strings"
	"testing"
)

// fixtureWithDocRels builds a docx whose main part carries the given
// relationships and whose zip contains the given extra parts.
func fixtureWithDocRels(t *testing.T, rels string, extra map[string]string) []byte {
	t.Helper()
	parts := map[string]string{
		"[Content_Types].xml": fixtureContentTypes,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + rels + `</Relationships>`,
	}
	for name, data := range extra {
		parts[name] = data
	}
	return buildFixtureDocx(t, parts)
}

const fixtureHdrXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p/></w:hdr>`

// C60: a header referenced from document.xml but absent from the package must
// fail Open, naming the part; the reference means the document displays that
// content.
func TestOpenErrorsOnMissingReferencedHeader(t *testing.T) {
	fixture := fixtureWithDocRels(t,
		`<Relationship Id="rId7" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>`,
		nil)
	_, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err == nil {
		t.Fatal("Open succeeded on a document referencing a missing header part")
	}
	if !strings.Contains(err.Error(), "/word/header1.xml") || !strings.Contains(err.Error(), "rId7") {
		t.Errorf("error does not name the missing part and relationship: %v", err)
	}
}

// C60: a referenced-but-missing footer part is still an Open error (footers,
// like headers, render visible page furniture and stay essential). Numbering is
// covered separately as an OPTIONAL part that is now tolerated — see
// TestOpenToleratesMissingNumberingPart.
func TestOpenErrorsOnMissingReferencedFooter(t *testing.T) {
	fixture := fixtureWithDocRels(t,
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer2.xml"/>`,
		nil)
	_, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err == nil {
		t.Fatal("Open succeeded on a document referencing a missing footer part")
	}
	if !strings.Contains(err.Error(), "/word/footer2.xml") {
		t.Errorf("error does not name the missing part: %v", err)
	}
}

// C60: a referenced header that exists still opens (the check must not turn
// present parts into errors), and an unreferenced absence stays tolerated.
func TestOpenReferencedHeaderPresentSucceeds(t *testing.T) {
	fixture := fixtureWithDocRels(t,
		`<Relationship Id="rId7" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>`,
		map[string]string{"word/header1.xml": fixtureHdrXML})
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("Open failed with the referenced header present: %v", err)
	}
	if _, ok := doc.headers["/word/header1.xml"]; !ok {
		t.Error("referenced header part not loaded")
	}

	// External-mode and unrelated relationship types are not checked.
	fixture = fixtureWithDocRels(t,
		`<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com/" TargetMode="External"/>`,
		nil)
	if _, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture))); err != nil {
		t.Fatalf("Open failed on an external hyperlink relationship: %v", err)
	}
}

// saveNew dedup-rel gap: adding the same image bytes twice to a NEW document
// dedupes to one media part, but the second placement's relationship was
// registered only in d.relationships, which saveNew did not emit — leaving
// the second r:embed dangling.
func TestNewDocumentDuplicateImageRelsResolve(t *testing.T) {
	doc := Create()
	png := append([]byte(nil), minimalTransparentPNG...)

	img1, err := doc.AddParagraph().AddRun().AddImageFromBytes(png, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	img2, err := doc.AddParagraph().AddRun().AddImageFromBytes(png, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if img1.relID == img2.relID {
		t.Fatalf("both placements share relationship %s; want distinct ids", img1.relID)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// One deduplicated media part...
	if n := zipEntryCount(t, saved, "word/media/image1.png"); n != 1 {
		t.Fatalf("media part count = %d, want 1", n)
	}

	// ...but both r:embed ids resolve through document.xml.rels.
	rels := string(readZipPart(t, saved, "word/_rels/document.xml.rels"))
	for _, id := range []string{img1.relID, img2.relID} {
		if !strings.Contains(rels, `Id="`+id+`"`) {
			t.Errorf("relationship %s missing from document rels:\n%s", id, rels)
		}
	}

	// Both drawings reference their own relationship id.
	docXML := string(readZipPart(t, saved, "word/document.xml"))
	for _, id := range []string{img1.relID, img2.relID} {
		if !strings.Contains(docXML, `r:embed="`+id+`"`) {
			t.Errorf("document.xml missing r:embed %s", id)
		}
	}

	// The saved package reopens cleanly.
	if _, err := OpenReader(bytes.NewReader(saved), int64(len(saved))); err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
}
