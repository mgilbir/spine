package docx

import (
	"strings"
	"testing"
)

// TestRestartedListStyleWritesStartOverride covers the §5 numbering gap:
// CT_NumLvl/StartOverride were modeled but no API reached them, so "restart
// this list at 1" — the most-asked numbering feature — was unexpressible.
func TestRestartedListStyleWritesStartOverride(t *testing.T) {
	doc := Create()
	def := doc.Numbering().AddDefinition()
	def.SetLevel(0, NumberFormatDecimal, "%1.")
	first := def.ListStyle()
	second := def.RestartedListStyle(0, 1)

	if first.numID == second.numID {
		t.Fatal("RestartedListStyle reused the base numbering instance; the two counters would be one")
	}
	doc.AddParagraphWithText("one").SetListStyle(first, 0)
	doc.AddParagraphWithText("two").SetListStyle(first, 0)
	doc.AddParagraphWithText("one again").SetListStyle(second, 0)

	saved := saveDoc(t, doc)
	numbering := mustZipEntry(t, saved, "word/numbering.xml")
	if !strings.Contains(numbering, "<w:lvlOverride") {
		t.Fatalf("numbering.xml carries no lvlOverride:\n%s", numbering)
	}
	if !strings.Contains(numbering, `<w:startOverride w:val="1"/>`) {
		t.Errorf("numbering.xml carries no startOverride:\n%s", numbering)
	}
	// Both instances must point at the same abstract definition.
	if got := strings.Count(numbering, `<w:abstractNumId w:val="`+itoa(def.AbstractNumID())+`"/>`); got != 2 {
		t.Errorf("got %d instances of abstract definition %d, want 2:\n%s", got, def.AbstractNumID(), numbering)
	}
	// And the document must still validate and reopen.
	reopened := openFixture(t, saved)
	if rep := reopened.Validate(); rep.HasErrors() {
		t.Errorf("the restarted list does not validate: %v", rep)
	}
}

// TestRestartAtOnSessionInstance covers the in-place override on an instance
// this session created.
func TestRestartAtOnSessionInstance(t *testing.T) {
	doc := Create()
	ls := doc.AddNumberedList()
	ls.RestartAt(0, 5)
	ls.RestartAt(0, 7) // replaces, does not duplicate
	ls.RestartAt(1, 3)
	doc.AddParagraphWithText("item").SetListStyle(ls, 0)

	numbering := mustZipEntry(t, saveDoc(t, doc), "word/numbering.xml")
	if got := strings.Count(numbering, "<w:lvlOverride"); got != 2 {
		t.Errorf("got %d lvlOverride elements, want 2 (one per level):\n%s", got, numbering)
	}
	if !strings.Contains(numbering, `<w:startOverride w:val="7"/>`) {
		t.Errorf("the second RestartAt for level 0 did not replace the first:\n%s", numbering)
	}
	if strings.Contains(numbering, `<w:startOverride w:val="5"/>`) {
		t.Errorf("the replaced override is still present:\n%s", numbering)
	}
	if !strings.Contains(numbering, `<w:startOverride w:val="3"/>`) {
		t.Errorf("the level-1 override is missing:\n%s", numbering)
	}
}

// TestRestartAtIgnoresParsedInstance pins the documented limit: an instance
// that came from an opened package is round-tripped as raw XML, so RestartAt is
// a no-op on it rather than silently writing an override into a fresh model
// that would never be emitted.
func TestRestartAtIgnoresParsedInstance(t *testing.T) {
	doc := openFixture(t, numberingFixture(t))
	ls := &ListStyle{document: doc, numID: 1}
	ls.RestartAt(0, 4)
	if doc.numberingModified {
		t.Error("RestartAt on a parsed instance flagged numbering.xml modified for a change it cannot make")
	}
}

// numberingFixture builds a docx with one parsed numbering instance (numId 1).
func numberingFixture(t *testing.T) []byte {
	t.Helper()
	const ct = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/></Types>`
	const rels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/></Relationships>`
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": ct,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels": rels,
		"word/numbering.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:numbering ` + fixtureWNS + `>` +
			`<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl></w:abstractNum>` +
			`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>` +
			`</w:numbering>`,
	})
}
