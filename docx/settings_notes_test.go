package docx

import (
	"bytes"
	"strings"
	"testing"
)

// openDocWithSettings builds a docx whose document.xml references a settings
// part carrying the given settings.xml content, and opens it.
func openDocWithSettings(t *testing.T, settingsXML string) *Document {
	t.Helper()
	const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/></Types>`
	const documentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>body</w:t></w:r></w:p></w:body></w:document>`
	const documentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId20" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/></Relationships>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  fixtureRootRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": documentRels,
		"word/settings.xml":            settingsXML,
	})
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open settings fixture: %v", err)
	}
	return doc
}

// TestDocumentFootnotePropertiesRoundTrip sets document-level footnote numbering
// and verifies it is read back after a save/reopen.
func TestDocumentFootnotePropertiesRoundTrip(t *testing.T) {
	doc := Create()
	start := 2
	doc.SetFootnoteProperties(NoteProperties{
		Position:     "pageBottom",
		NumberFormat: "lowerRoman",
		NumberStart:  &start,
		Restart:      "eachSect",
	})

	if _, ok := doc.FootnoteProperties(); !ok {
		t.Fatal("footnote properties not present after set")
	}

	fresh := reopenDoc(t, doc)
	np, ok := fresh.FootnoteProperties()
	if !ok {
		t.Fatal("footnote properties lost across round-trip")
	}
	if np.Position != "pageBottom" || np.NumberFormat != "lowerRoman" || np.Restart != "eachSect" {
		t.Errorf("footnote properties wrong: %+v", np)
	}
	if np.NumberStart == nil || *np.NumberStart != 2 {
		t.Errorf("footnote numberStart wrong: %+v", np.NumberStart)
	}
}

// TestDocumentEndnotePropertiesRoundTrip does the same for endnotes.
func TestDocumentEndnotePropertiesRoundTrip(t *testing.T) {
	doc := Create()
	doc.SetEndnoteProperties(NoteProperties{Position: "docEnd", NumberFormat: "upperRoman"})

	fresh := reopenDoc(t, doc)
	np, ok := fresh.EndnoteProperties()
	if !ok {
		t.Fatal("endnote properties lost across round-trip")
	}
	if np.Position != "docEnd" || np.NumberFormat != "upperRoman" {
		t.Errorf("endnote properties wrong: %+v", np)
	}
}

// TestDocumentFootnotePropertiesMarkup checks the emitted settings.xml carries
// the numbering children under w:footnotePr.
func TestDocumentFootnotePropertiesMarkup(t *testing.T) {
	doc := Create()
	doc.SetFootnoteProperties(NoteProperties{NumberFormat: "decimal"})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	data, ok := zipEntry(t, saved, "word/settings.xml")
	if !ok {
		t.Fatal("word/settings.xml missing")
	}
	s := string(data)
	if !strings.Contains(s, `<w:footnotePr><w:numFmt w:val="decimal"/></w:footnotePr>`) {
		t.Errorf("footnotePr markup wrong:\n%s", s)
	}
}

// TestDocumentFootnotePropertiesPreservesSeparators verifies that changing the
// numbering keeps any separator w:footnote references already present.
func TestDocumentFootnotePropertiesPreservesSeparators(t *testing.T) {
	const settings = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:footnotePr><w:numFmt w:val="decimal"/>` +
		`<w:footnote w:type="separator" w:id="-1"/>` +
		`<w:footnote w:type="continuationSeparator" w:id="0"/>` +
		`</w:footnotePr></w:settings>`
	doc := openDocWithSettings(t, settings)

	np, ok := doc.FootnoteProperties()
	if !ok || np.NumberFormat != "decimal" {
		t.Fatalf("footnote props not read: %+v ok=%v", np, ok)
	}

	doc.SetFootnoteProperties(NoteProperties{NumberFormat: "lowerLetter"})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := zipEntry(t, saved, "word/settings.xml")
	s := string(data)
	if !strings.Contains(s, `<w:numFmt w:val="lowerLetter"/>`) {
		t.Errorf("numbering not updated:\n%s", s)
	}
	if !strings.Contains(s, `w:type="separator"`) || !strings.Contains(s, `w:type="continuationSeparator"`) {
		t.Errorf("separator references dropped:\n%s", s)
	}
}

// TestDocumentClearFootnoteProperties removes the element.
func TestDocumentClearFootnoteProperties(t *testing.T) {
	doc := Create()
	doc.SetFootnoteProperties(NoteProperties{NumberFormat: "decimal"})
	if !doc.ClearFootnoteProperties() {
		t.Fatal("ClearFootnoteProperties reported nothing removed")
	}
	if _, ok := doc.FootnoteProperties(); ok {
		t.Fatal("footnote properties still present after clear")
	}
}
