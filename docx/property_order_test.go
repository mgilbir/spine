package docx

import (
	"bytes"
	"strings"
	"testing"
)

// C329 at the public API. The audit reported the defect through Run.SetBold and
// Paragraph.SetAlignment on an *opened* document: the property landed after
// every child the source had written, which for w:pPr's xsd:sequence is
// schema-invalid. These exercise the same entry points end to end — open, set,
// save, read back the regenerated document.xml.

// TestSetBoldOnParsedRunEmitsSchemaOrder covers the run-property shape: the
// added w:b must precede the w:sz the source already carried.
func TestSetBoldOnParsedRunEmitsSchemaOrder(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p><w:r><w:rPr><w:sz w:val="24"/></w:rPr><w:t>hi</w:t></w:r></w:p></w:body>`)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	doc.Paragraphs()[0].Runs()[0].SetBold(true)

	out := saveDocumentXML(t, doc)
	iB, iSz := strings.Index(out, "<w:b/>"), strings.Index(out, "<w:sz ")
	if iB < 0 || iSz < 0 {
		t.Fatalf("missing w:b or w:sz in %s", out)
	}
	if iB > iSz {
		t.Errorf("SetBold appended w:b after the captured w:sz: %s", out)
	}
}

// TestSetAlignmentOnParsedSectionParagraphEmitsSchemaOrder covers the
// schema-invalid shape: a paragraph whose w:pPr carries the section's w:sectPr,
// given an alignment after parse, emitted <w:sectPr/><w:jc/>. CT_PPr's content
// model puts sectPr after every pPrBase child, so w:jc must come first.
func TestSetAlignmentOnParsedSectionParagraphEmitsSchemaOrder(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p><w:pPr><w:sectPr><w:type w:val="nextPage"/></w:sectPr></w:pPr>`+
			`<w:r><w:t>hi</w:t></w:r></w:p></w:body>`)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	doc.Paragraphs()[0].SetAlignment(AlignmentCenter)

	out := saveDocumentXML(t, doc)
	iJc, iSect := strings.Index(out, "<w:jc "), strings.Index(out, "<w:sectPr>")
	if iJc < 0 || iSect < 0 {
		t.Fatalf("missing w:jc or w:sectPr in %s", out)
	}
	if iJc > iSect {
		t.Errorf("SetAlignment emitted w:jc after w:sectPr (schema-invalid): %s", out)
	}
}

// TestUnmodifiedDocumentWithOutOfOrderPropertiesIsUnchanged is the byte-identity
// side of the same change. Real producers write property children out of schema
// order and Word tolerates it; a zero-modification open→save must reproduce the
// source exactly rather than "correcting" it, so the rank ordering above may
// only apply to properties this library adds.
func TestUnmodifiedDocumentWithOutOfOrderPropertiesIsUnchanged(t *testing.T) {
	body := `<w:body><w:p><w:pPr><w:sectPr><w:type w:val="nextPage"/></w:sectPr>` +
		`<w:jc w:val="center"/><w:pStyle w:val="Body"/></w:pPr>` +
		`<w:r><w:rPr><w:sz w:val="24"/><w:b/><w:rFonts w:ascii="Arial"/></w:rPr>` +
		`<w:t>hi</w:t></w:r></w:p></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	out := saveDocumentXML(t, doc)
	if !strings.Contains(out, body) {
		t.Errorf("zero-modification save reordered the source's property children\n got: %s\nwant to contain: %s", out, body)
	}
}

// saveDocumentXML saves doc and returns the regenerated word/document.xml.
func saveDocumentXML(t *testing.T, doc *Document) string {
	t.Helper()
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	data, ok := zipEntry(t, saved, "word/document.xml")
	if !ok {
		t.Fatal("word/document.xml missing from saved package")
	}
	return string(data)
}
