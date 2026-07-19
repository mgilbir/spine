package docx

import (
	"strings"
	"testing"
)

const fixtureStylesCT = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`

// fixtureWithStyles builds a docx carrying a styles.xml part (referenced from
// document.xml.rels) with the given styles-part body.
func fixtureWithStyles(t *testing.T, stylesXML string) []byte {
	t.Helper()
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureStylesCT,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"word/styles.xml": stylesXML,
	})
}

const fixtureStylesBody = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults>` +
	`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>` +
	`<w:style w:type="character" w:styleId="Emphasis"><w:name w:val="Emphasis"/><w:rPr><w:i/></w:rPr></w:style>` +
	`</w:styles>`

// An unmodified styles part must round-trip byte-for-byte: merely reading
// through the style manager (or not touching it at all) leaves the preserved
// bytes untouched.
func TestStylesByteIdenticalWhenUnmodified(t *testing.T) {
	fixture := fixtureWithStyles(t, fixtureStylesBody)
	doc := openFixture(t, fixture)

	// Read through the manager: this must NOT mark the part modified.
	styles := doc.Styles()
	if len(styles.List()) == 0 {
		t.Fatal("expected styles to be listed")
	}
	if s := styles.Style("Emphasis"); s == nil || s.Type() != StyleTypeCharacter {
		t.Fatalf("Emphasis style not fetched as character style: %+v", s)
	}

	saved := saveDoc(t, doc)
	if got := mustZipEntry(t, saved, "word/styles.xml"); got != fixtureStylesBody {
		t.Fatalf("styles.xml not byte-identical after read-only round trip.\nwant:\n%s\ngot:\n%s", fixtureStylesBody, got)
	}
}

// Adding a paragraph style to a created document writes it into styles.xml and
// the document reopens cleanly with the style present and its properties intact.
func TestAddParagraphStyleCreated(t *testing.T) {
	doc := Create()
	doc.Styles().
		AddParagraphStyle("Quote", "Quote").
		SetBasedOn("Normal").
		SetNext("Normal").
		SetQuickFormat(true).
		SetAlignment(AlignmentCenter).
		SetSpaceBefore(6).
		SetSpaceAfter(12).
		SetIndentLeft(36).
		SetFont("Georgia").
		SetFontSize(13).
		SetItalic(true).
		SetColor("404040")

	saved := saveDoc(t, doc)
	styles := mustZipEntry(t, saved, "word/styles.xml")

	for _, want := range []string{
		`<w:style w:type="paragraph" w:styleId="Quote">`,
		`<w:name w:val="Quote"/>`,
		`<w:basedOn w:val="Normal"/>`,
		`<w:next w:val="Normal"/>`,
		`<w:qFormat/>`,
		`<w:jc w:val="center"/>`,
		`<w:spacing w:before="120" w:after="240"/>`,
		`<w:ind w:left="720"/>`,
		`<w:rFonts w:ascii="Georgia" w:hAnsi="Georgia"/>`,
		`<w:i/>`,
		`<w:sz w:val="26"/>`,
		`<w:color w:val="404040"/>`,
	} {
		if !strings.Contains(styles, want) {
			t.Errorf("styles.xml missing %q\n%s", want, styles)
		}
	}

	// Reopen and confirm the style is fetchable through the manager.
	reopened := openFixture(t, saved)
	defer func() { _ = reopened.Close() }()
	s := reopened.Styles().Style("Quote")
	if s == nil {
		t.Fatal("Quote style missing after reopen")
	}
	if s.Name() != "Quote" || s.Type() != StyleTypeParagraph {
		t.Fatalf("Quote style metadata wrong: name=%q type=%q", s.Name(), s.Type())
	}
}

// Modifying a style in a document that already has a styles part rewrites the
// part with the change while keeping the untouched styles.
func TestModifyExistingStyle(t *testing.T) {
	fixture := fixtureWithStyles(t, fixtureStylesBody)
	doc := openFixture(t, fixture)

	s := doc.Styles().Style("Emphasis")
	if s == nil {
		t.Fatal("Emphasis style not found")
	}
	s.SetBold(true).SetColor("FF0000")

	saved := saveDoc(t, doc)
	styles := mustZipEntry(t, saved, "word/styles.xml")

	if !strings.Contains(styles, `<w:style w:type="character" w:styleId="Emphasis">`) {
		t.Errorf("Emphasis style lost:\n%s", styles)
	}
	if !strings.Contains(styles, `<w:b/>`) || !strings.Contains(styles, `<w:color w:val="FF0000"/>`) {
		t.Errorf("Emphasis modifications missing:\n%s", styles)
	}
	// The pre-existing Normal style survives the regeneration.
	if !strings.Contains(styles, `w:styleId="Normal"`) {
		t.Errorf("Normal style dropped on regeneration:\n%s", styles)
	}
}

// A style added to an opened document that has no styles part creates the part,
// its document relationship, and its content-type override.
func TestOpenedDocAddStyleWritesStylesPart(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p><w:r><w:t>hello</w:t></w:r></w:p></w:body>`)
	doc := openFixture(t, fixture)

	doc.Styles().AddParagraphStyle("Callout", "Callout").SetBold(true)

	saved := saveDoc(t, doc)

	styles := mustZipEntry(t, saved, "word/styles.xml")
	if !strings.Contains(styles, `w:styleId="Callout"`) {
		t.Fatalf("styles.xml lacks the added style:\n%s", styles)
	}
	rels := mustZipEntry(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, `relationships/styles`) || !strings.Contains(rels, `Target="styles.xml"`) {
		t.Fatalf("document rels lack styles relationship:\n%s", rels)
	}
	ct := mustZipEntry(t, saved, "[Content_Types].xml")
	if !strings.Contains(ct, `PartName="/word/styles.xml"`) {
		t.Fatalf("[Content_Types].xml lacks styles override:\n%s", ct)
	}
}

// AddParagraphStyle is idempotent: a second call with the same id returns the
// existing builder rather than emitting a duplicate style.
func TestAddStyleIdempotent(t *testing.T) {
	doc := Create()
	m := doc.Styles()
	a := m.AddParagraphStyle("Custom", "Custom")
	b := m.AddParagraphStyle("Custom", "Different Name")
	if a.s != b.s {
		t.Fatal("AddParagraphStyle created a duplicate for an existing id")
	}
	if b.Name() != "Custom" {
		t.Fatalf("existing style name overwritten: %q", b.Name())
	}
	count := 0
	for _, s := range m.List() {
		if s.ID() == "Custom" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one Custom style, got %d", count)
	}
}
