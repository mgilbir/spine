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

// C60: same for a referenced-but-missing footer and numbering part.
func TestOpenErrorsOnMissingReferencedFooterAndNumbering(t *testing.T) {
	for _, tc := range []struct{ relType, target, wantPart string }{
		{"footer", "footer2.xml", "/word/footer2.xml"},
		{"numbering", "numbering.xml", "/word/numbering.xml"},
	} {
		fixture := fixtureWithDocRels(t,
			`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/`+tc.relType+`" Target="`+tc.target+`"/>`,
			nil)
		_, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
		if err == nil {
			t.Fatalf("Open succeeded on a document referencing a missing %s part", tc.relType)
		}
		if !strings.Contains(err.Error(), tc.wantPart) {
			t.Errorf("%s: error does not name the missing part: %v", tc.relType, err)
		}
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
