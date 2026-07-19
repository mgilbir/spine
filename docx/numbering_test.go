package docx

import (
	"strings"
	"testing"
)

// A custom numbering definition built through the manager writes an abstractNum
// with the configured levels and a num instance, and drives a list paragraph.
func TestNumberingManagerCustomDefinition(t *testing.T) {
	doc := Create()

	def := doc.Numbering().AddDefinition()
	def.SetLevel(0, NumberFormatUpperRoman, "%1.").SetStart(1)
	def.SetLevel(1, NumberFormatLowerLetter, "%2)").SetIndent(48).SetHanging(24)
	list := def.ListStyle()

	doc.AddParagraphWithText("Chapter").SetListStyle(list, 0)
	doc.AddParagraphWithText("Point").SetListStyle(list, 1)

	saved := saveDoc(t, doc)
	numbering := mustZipEntry(t, saved, "word/numbering.xml")

	for _, want := range []string{
		`<w:abstractNum w:abstractNumId="0">`,
		`<w:lvl w:ilvl="0">`,
		`<w:numFmt w:val="upperRoman"/>`,
		`<w:lvlText w:val="%1."/>`,
		`<w:lvl w:ilvl="1">`,
		`<w:numFmt w:val="lowerLetter"/>`,
		`<w:lvlText w:val="%2)"/>`,
		`<w:ind w:left="960" w:hanging="480"/>`,
		`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`,
	} {
		if !strings.Contains(numbering, want) {
			t.Errorf("numbering.xml missing %q\n%s", want, numbering)
		}
	}

	// The list paragraphs reference the definition's num id.
	document := mustZipEntry(t, saved, "word/document.xml")
	if !strings.Contains(document, `<w:numId w:val="1"/>`) {
		t.Errorf("document.xml does not reference the custom list numId:\n%s", document)
	}
}

// A custom bullet definition: a level with the bullet format, a glyph, and a
// bullet font round-trips.
func TestNumberingManagerCustomBullet(t *testing.T) {
	doc := Create()
	def := doc.Numbering().AddDefinition()
	def.Level(0).SetFormat(NumberFormatBullet).SetText("").SetFont("Symbol")
	list := def.ListStyle()
	doc.AddParagraphWithText("bullet").SetListStyle(list, 0)

	saved := saveDoc(t, doc)
	numbering := mustZipEntry(t, saved, "word/numbering.xml")
	for _, want := range []string{
		`<w:numFmt w:val="bullet"/>`,
		`<w:rFonts w:ascii="Symbol" w:hAnsi="Symbol"/>`,
	} {
		if !strings.Contains(numbering, want) {
			t.Errorf("numbering.xml missing %q\n%s", want, numbering)
		}
	}
}

// ListStyle is stable: repeated calls return the same numbering instance rather
// than allocating a new num each time.
func TestNumberingListStyleStable(t *testing.T) {
	doc := Create()
	def := doc.Numbering().AddDefinition()
	def.SetLevel(0, NumberFormatDecimal, "%1.")
	if a, b := def.ListStyle(), def.ListStyle(); a.numID != b.numID {
		t.Fatalf("ListStyle allocated distinct num ids: %d vs %d", a.numID, b.numID)
	}
}

// A custom definition added to a document that already has a numbering part
// keeps the original definitions verbatim and gets non-colliding IDs.
func TestNumberingManagerExtendsExisting(t *testing.T) {
	origNumbering := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:abstractNum w:abstractNumId="5"><w:multiLevelType w:val="hybridMultilevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/></w:lvl></w:abstractNum>` +
		`<w:num w:numId="9"><w:abstractNumId w:val="5"/></w:num>` +
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

	def := doc.Numbering().AddDefinition()
	if def.AbstractNumID() <= 5 {
		t.Fatalf("new abstractNumId %d collides with parsed definition", def.AbstractNumID())
	}
	def.SetLevel(0, NumberFormatDecimal, "%1.")
	list := def.ListStyle()
	if list.numID <= 9 {
		t.Fatalf("new numId %d collides with parsed definition", list.numID)
	}
	doc.AddParagraphWithText("x").SetListStyle(list, 0)

	saved := saveDoc(t, doc)
	numbering := mustZipEntry(t, saved, "word/numbering.xml")
	if !strings.Contains(numbering, `<w:abstractNum w:abstractNumId="5">`) ||
		!strings.Contains(numbering, `<w:num w:numId="9"><w:abstractNumId w:val="5"/></w:num>`) {
		t.Errorf("original numbering definitions lost:\n%s", numbering)
	}
}
