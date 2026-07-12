package docx

import (
	"bytes"
	"strings"
	"testing"
)

const fixtureNumberingCT = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/></Types>`

const fixtureSettingsCT = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/></Types>`

func openFixture(t *testing.T, fixture []byte) *Document {
	t.Helper()
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func saveDoc(t *testing.T, doc *Document) []byte {
	t.Helper()
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenReader(bytes.NewReader(saved), int64(len(saved))); err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
	return saved
}

func mustZipEntry(t *testing.T, data []byte, name string) string {
	t.Helper()
	b, ok := zipEntry(t, data, name)
	if !ok {
		t.Fatalf("%s missing from saved package", name)
	}
	return string(b)
}

// C26: a list added to an OPENED document produced numPr references with no
// numbering part, relationship, or content-type override.
func TestOpenedDocAddListWritesNumberingPart(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p><w:r><w:t>hello</w:t></w:r></w:p></w:body>`)
	doc := openFixture(t, fixture)

	list := doc.AddBulletList()
	doc.AddParagraphWithText("item one").SetListStyle(list, 0)

	saved := saveDoc(t, doc)

	numbering := mustZipEntry(t, saved, "word/numbering.xml")
	if !strings.Contains(numbering, `<w:abstractNum`) || !strings.Contains(numbering, `<w:num `) {
		t.Fatalf("numbering.xml lacks definitions:\n%s", numbering)
	}
	rels := mustZipEntry(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, `relationships/numbering`) || !strings.Contains(rels, `Target="numbering.xml"`) {
		t.Fatalf("document rels lack numbering relationship:\n%s", rels)
	}
	ct := mustZipEntry(t, saved, "[Content_Types].xml")
	if !strings.Contains(ct, `PartName="/word/numbering.xml"`) {
		t.Fatalf("[Content_Types].xml lacks numbering override:\n%s", ct)
	}
}

// C26: a document that already HAS a numbering part must keep its original
// definitions verbatim (including children and attributes the model does not
// type) and gain the new ones, with fresh non-colliding IDs.
func TestOpenedDocWithNumberingParseAndExtend(t *testing.T) {
	origNumbering := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml">` +
		`<w:numPicBullet w:numPicBulletId="0"><w:drawing/></w:numPicBullet>` +
		`<w:abstractNum w:abstractNumId="3" w15:restartNumberingAfterBreak="0"><w:multiLevelType w:val="hybridMultilevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/></w:lvl></w:abstractNum>` +
		`<w:num w:numId="7"><w:abstractNumId w:val="3"/></w:num>` +
		`</w:numbering>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureNumberingCT,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/></Relationships>`,
		"word/numbering.xml": origNumbering,
	})
	doc := openFixture(t, fixture)

	list := doc.AddNumberedList()
	if list.numID <= 7 {
		t.Fatalf("new numId %d collides with parsed definitions", list.numID)
	}
	doc.AddParagraphWithText("numbered").SetListStyle(list, 0)

	saved := saveDoc(t, doc)
	numbering := mustZipEntry(t, saved, "word/numbering.xml")

	// Original definitions preserved verbatim, including the untyped
	// numPicBullet child and the w15 extension attribute.
	for _, want := range []string{
		`<w:numPicBullet w:numPicBulletId="0"><w:drawing/></w:numPicBullet>`,
		`<w:abstractNum w:abstractNumId="3" w15:restartNumberingAfterBreak="0">`,
		`<w:num w:numId="7"><w:abstractNumId w:val="3"/></w:num>`,
	} {
		if !strings.Contains(numbering, want) {
			t.Errorf("original numbering content lost: %q\n%s", want, numbering)
		}
	}
	// New definitions present with non-colliding IDs.
	if !strings.Contains(numbering, `w:abstractNumId="4"`) {
		t.Errorf("new abstract definition missing:\n%s", numbering)
	}
	if !strings.Contains(numbering, `<w:num w:numId="8">`) {
		t.Errorf("new num instance missing:\n%s", numbering)
	}
	// Schema order: every abstractNum before every num.
	lastAbs := strings.LastIndex(numbering, "<w:abstractNum ")
	firstNum := strings.Index(numbering, "<w:num ")
	if lastAbs > firstNum {
		t.Errorf("abstractNum emitted after num (schema order violated):\n%s", numbering)
	}
	// The document relationship must not be duplicated.
	rels := mustZipEntry(t, saved, "word/_rels/document.xml.rels")
	if strings.Count(rels, "relationships/numbering") != 1 {
		t.Errorf("numbering relationship duplicated:\n%s", rels)
	}
}

// A zero-modification save keeps an existing numbering part byte-identical.
func TestNumberingByteIdenticalWithoutModification(t *testing.T) {
	origNumbering := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num></w:numbering>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureNumberingCT,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/numbering.xml": origNumbering,
	})
	doc := openFixture(t, fixture)
	saved := saveDoc(t, doc)
	if got := mustZipEntry(t, saved, "word/numbering.xml"); got != origNumbering {
		t.Fatalf("numbering.xml changed on zero-modification save:\n%s", got)
	}
}

// C64: Create()'d documents referenced Heading styles with no styles.xml.
func TestCreatedDocWritesDefaultStyles(t *testing.T) {
	doc := Create()
	doc.AddHeading("Title", 1)
	saved := saveDoc(t, doc)

	styles := mustZipEntry(t, saved, "word/styles.xml")
	for _, want := range []string{
		`w:styleId="Normal"`, `w:styleId="Heading1"`, `w:styleId="Heading9"`,
		`<w:docDefaults>`, `<w:outlineLvl w:val="0"/>`,
	} {
		if !strings.Contains(styles, want) {
			t.Errorf("styles.xml missing %q:\n%s", want, styles)
		}
	}
	rels := mustZipEntry(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, `relationships/styles`) {
		t.Fatalf("document rels lack styles relationship:\n%s", rels)
	}
	ct := mustZipEntry(t, saved, "[Content_Types].xml")
	if !strings.Contains(ct, `PartName="/word/styles.xml"`) {
		t.Fatalf("[Content_Types].xml lacks styles override:\n%s", ct)
	}

	// Reopen through the library: the heading's style reference must resolve.
	doc2 := openFixture(t, saved)
	styleID := doc2.Paragraphs()[0].Style()
	if styleID != "Heading1" {
		t.Fatalf("expected Heading1 style reference, got %q", styleID)
	}
	found := false
	for _, st := range doc2.styles.Style {
		if st.StyleId == styleID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("style %q not defined in parsed styles part", styleID)
	}
}

// C197: AddHeader(HeaderEven) on a created document must produce settings.xml
// carrying w:evenAndOddHeaders (plus relationship and content-type override).
func TestCreatedDocEvenHeaderWritesSettings(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.AddHeader(HeaderEven).AddParagraphWithText("even header")
	saved := saveDoc(t, doc)

	settings := mustZipEntry(t, saved, "word/settings.xml")
	if !strings.Contains(settings, `<w:evenAndOddHeaders/>`) {
		t.Fatalf("settings.xml lacks evenAndOddHeaders:\n%s", settings)
	}
	rels := mustZipEntry(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, `relationships/settings`) {
		t.Fatalf("document rels lack settings relationship:\n%s", rels)
	}
	ct := mustZipEntry(t, saved, "[Content_Types].xml")
	if !strings.Contains(ct, `PartName="/word/settings.xml"`) {
		t.Fatalf("[Content_Types].xml lacks settings override:\n%s", ct)
	}
}

// C197: on an opened document whose settings.xml exists, the flag is inserted
// at its schema position and every other child round-trips verbatim.
func TestOpenedDocEvenFooterPatchesSettings(t *testing.T) {
	origSettings := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">` +
		`<w:zoom w:percent="100"/>` +
		`<w:defaultTabStop w:val="708"/>` +
		`<w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat>` +
		`<w14:docId w14:val="1DEAB552"/>` +
		`</w:settings>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureSettingsCT,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/settings.xml": origSettings,
	})
	doc := openFixture(t, fixture)
	doc.AddFooter(FooterEven).AddParagraphWithText("even footer")
	saved := saveDoc(t, doc)

	settings := mustZipEntry(t, saved, "word/settings.xml")
	// Inserted in schema position: after defaultTabStop, before compat.
	want := `<w:zoom w:percent="100"/><w:defaultTabStop w:val="708"/><w:evenAndOddHeaders/><w:compat>`
	if !strings.Contains(settings, want) {
		t.Fatalf("evenAndOddHeaders not inserted in schema position:\n%s", settings)
	}
	// The other children (including the extension element) survive verbatim.
	for _, keep := range []string{
		`<w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/>`,
		`<w14:docId w14:val="1DEAB552"/>`,
	} {
		if !strings.Contains(settings, keep) {
			t.Errorf("settings child lost: %q\n%s", keep, settings)
		}
	}
}

// C197: an opened document with no settings part gets a fresh one.
func TestOpenedDocEvenHeaderCreatesSettings(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body><w:p/></w:body>`)
	doc := openFixture(t, fixture)
	doc.AddHeader(HeaderEven).AddParagraphWithText("even header")
	saved := saveDoc(t, doc)

	settings := mustZipEntry(t, saved, "word/settings.xml")
	if !strings.Contains(settings, `<w:evenAndOddHeaders/>`) {
		t.Fatalf("settings.xml lacks evenAndOddHeaders:\n%s", settings)
	}
	rels := mustZipEntry(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, `relationships/settings`) {
		t.Fatalf("document rels lack settings relationship:\n%s", rels)
	}
	ct := mustZipEntry(t, saved, "[Content_Types].xml")
	if !strings.Contains(ct, `PartName="/word/settings.xml"`) {
		t.Fatalf("[Content_Types].xml lacks settings override:\n%s", ct)
	}
}

// A zero-modification save keeps an existing settings part byte-identical.
func TestSettingsByteIdenticalWithoutModification(t *testing.T) {
	origSettings := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:zoom w:percent="100"/><w:evenAndOddHeaders/><w:compat/></w:settings>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureSettingsCT,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/settings.xml": origSettings,
	})
	doc := openFixture(t, fixture)
	saved := saveDoc(t, doc)
	if got := mustZipEntry(t, saved, "word/settings.xml"); got != origSettings {
		t.Fatalf("settings.xml changed on zero-modification save:\ngot:  %s\nwant: %s", got, origSettings)
	}

	// A flag that is already present must not mark the settings modified.
	doc2 := openFixture(t, fixture)
	doc2.AddHeader(HeaderEven)
	saved2 := saveDoc(t, doc2)
	got2 := mustZipEntry(t, saved2, "word/settings.xml")
	if got2 != origSettings {
		t.Fatalf("settings.xml regenerated although evenAndOddHeaders already present:\n%s", got2)
	}
}
