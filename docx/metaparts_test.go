package docx

import (
	"strings"
	"testing"
)

// nonstandardMetaFixture builds a docx whose styles, numbering and settings
// relationships point at nonstandard part names — legal under OPC, which binds
// a part to its role through the relationship rather than the name.
func nonstandardMetaFixture(t *testing.T) []byte {
	t.Helper()
	const ct = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles2.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/word/numbering2.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/><Override PartName="/word/settings2.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/></Types>`
	const rels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles2.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering2.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings2.xml"/></Relationships>`
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": ct,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels": rels,
		"word/styles2.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:styles ` + fixtureWNS + `><w:style w:type="paragraph" w:styleId="Existing"><w:name w:val="Existing"/></w:style></w:styles>`,
		"word/numbering2.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:numbering ` + fixtureWNS + `/>`,
		"word/settings2.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:settings ` + fixtureWNS + `/>`,
	})
}

// TestNonstandardStylesPartIsParsed pins the read half of C502: matching the
// styles part by hardcoded name left d.styles nil, so the document's existing
// styles were invisible.
func TestNonstandardStylesPartIsParsed(t *testing.T) {
	doc := openFixture(t, nonstandardMetaFixture(t))
	if s := doc.Styles().Style("Existing"); s == nil {
		t.Fatal("the styles part at a nonstandard name was not parsed: Style(\"Existing\") = nil")
	}
}

// TestNonstandardStylesPartEditLandsInPlace pins the write half of C502: the
// edit used to be written to a fresh /word/styles.xml while ensureDocRelationship
// (which matches on relationship *type*) saw the existing styles relationship
// and added none — an orphan part and a style that never took effect.
func TestNonstandardStylesPartEditLandsInPlace(t *testing.T) {
	doc := openFixture(t, nonstandardMetaFixture(t))
	doc.Styles().AddParagraphStyle("Added", "Added").SetBold(true)
	saved := saveDoc(t, doc)

	if _, ok := zipEntry(t, saved, "word/styles.xml"); ok {
		t.Error("a second, orphaned /word/styles.xml was written")
	}
	styles := mustZipEntry(t, saved, "word/styles2.xml")
	if !strings.Contains(styles, `w:styleId="Added"`) {
		t.Errorf("the new style did not land in the part the document points at:\n%s", styles)
	}
	if !strings.Contains(styles, `w:styleId="Existing"`) {
		t.Errorf("the existing style was lost:\n%s", styles)
	}

	// Reopening must find the style through the same relationship.
	reopened := openFixture(t, saved)
	if s := reopened.Styles().Style("Added"); s == nil {
		t.Error("the added style is not visible after a round trip")
	}
}

// TestNonstandardNumberingAndSettingsEditsLandInPlace covers the other two
// metadata parts the save path can regenerate.
func TestNonstandardNumberingAndSettingsEditsLandInPlace(t *testing.T) {
	doc := openFixture(t, nonstandardMetaFixture(t))
	doc.AddParagraph().SetListStyle(doc.AddBulletList(), 0)
	doc.SetMailMerge(&MailMerge{MainDocumentType: MailMergeFormLetters})
	saved := saveDoc(t, doc)

	for _, orphan := range []string{"word/numbering.xml", "word/settings.xml"} {
		if _, ok := zipEntry(t, saved, orphan); ok {
			t.Errorf("a second, orphaned /%s was written", orphan)
		}
	}
	if got := mustZipEntry(t, saved, "word/numbering2.xml"); !strings.Contains(got, "abstractNum") {
		t.Errorf("the list definition did not land in numbering2.xml:\n%s", got)
	}
	if got := mustZipEntry(t, saved, "word/settings2.xml"); !strings.Contains(got, "mailMerge") {
		t.Errorf("the mail-merge configuration did not land in settings2.xml:\n%s", got)
	}
}

// TestConventionalMetaPartNamesUnchanged guards that resolving through
// relationships leaves an ordinary package — and a created document — writing
// the conventional names.
func TestConventionalMetaPartNamesUnchanged(t *testing.T) {
	doc := Create()
	doc.Styles().AddParagraphStyle("Added", "Added")
	doc.AddParagraph().SetListStyle(doc.AddBulletList(), 0)
	saved := saveDoc(t, doc)
	for _, want := range []string{"word/styles.xml", "word/numbering.xml"} {
		if _, ok := zipEntry(t, saved, want); !ok {
			t.Errorf("a created document did not write /%s", want)
		}
	}
}
