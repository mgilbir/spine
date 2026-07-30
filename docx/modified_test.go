package docx

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// dcterms:modified is stamped if and only if the session changed the content.
// Both halves have to be pinned: a suite that only checks one direction does
// not notice the sense being inverted, and the two behaviours it protects —
// an accurate write time and a reproducible save — pull in opposite directions
// whenever a save happens to cross a second boundary.
//
// The determinism tests sleep past a second boundary on purpose:
// dcterms:modified has one-second resolution, so a wall-clock stamp is
// invisible to two saves made in the same second. That is exactly how the
// equivalent pptx defect survived three audits as "flaky, ~1/300".

// fixtureModified is the dcterms:modified value in the fixtures built below.
const fixtureModified = "2020-01-02T03:04:05Z"

const fixtureCorePropsCT = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/></Types>`

const fixtureCorePropsRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/></Relationships>`

const fixtureCoreXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>Fixture</dc:title><dcterms:created xsi:type="dcterms:W3CDTF">2019-01-01T00:00:00Z</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">` + fixtureModified + `</dcterms:modified></cp:coreProperties>`

// fixtureWithCoreProps builds a minimal docx that carries docProps/core.xml,
// so the stamping path is exercised (a package without core properties never
// gains them; see stampModified).
func fixtureWithCoreProps(t *testing.T, body string) []byte {
	t.Helper()
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureCorePropsCT,
		"_rels/.rels":         fixtureCorePropsRels,
		"docProps/core.xml":   fixtureCoreXML,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `>` + body + `</w:document>`,
	})
}

const modifiedFixtureBody = `<w:body>` +
	`<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="4680"/></w:tblGrid>` +
	`<w:tr><w:tc><w:tcPr/><w:p><w:r><w:t>CELL</w:t></w:r></w:p></w:tc></w:tr></w:tbl>` +
	`<w:sdt><w:sdtPr><w:tag w:val="t1"/></w:sdtPr>` +
	`<w:sdtContent><w:p><w:r><w:t>SDT</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
	`<w:p><w:r><w:t>BODY</w:t></w:r></w:p>` +
	`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>` +
	`</w:body>`

// baseModified is the fixture's dcterms:modified as a time.
func baseModified(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, fixtureModified)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// pastSecondBoundary waits long enough that a wall-clock stamp taken after it
// cannot serialize to the same dcterms:modified as one taken before.
func pastSecondBoundary() { time.Sleep(1100 * time.Millisecond) }

// TestUntouchedSaveDoesNotStampModified: opening a document and saving it
// changes neither the recorded write time nor a single byte, however long the
// two saves are apart.
func TestUntouchedSaveDoesNotStampModified(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fixture func(t *testing.T) []byte
	}{
		{"synthetic", func(t *testing.T) []byte { return fixtureWithCoreProps(t, modifiedFixtureBody) }},
		{"chart.docx", func(t *testing.T) []byte {
			raw, err := os.ReadFile("testdata/chart.docx")
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.fixture(t)
			doc := openFixture(t, raw)
			before := doc.Properties.Modified

			first := saveDoc(t, doc)
			pastSecondBoundary()
			second := saveDoc(t, doc)

			if !doc.Properties.Modified.Equal(before) {
				t.Errorf("an untouched save moved Properties.Modified from %s to %s",
					before, doc.Properties.Modified)
			}
			if !bytes.Equal(first, second) {
				t.Errorf("two untouched saves a second apart differ (%d vs %d bytes): "+
					"SaveBytes is not idempotent", len(first), len(second))
			}
			if !bytes.Equal(first, saveDoc(t, openFixture(t, raw))) {
				t.Error("an untouched save differs from the save of a freshly opened copy")
			}
		})
	}
}

// TestReadOnlyAccessDoesNotStampModified: reading a document end to end — every
// accessor that materializes a model on the way — is not editing it. Keying the
// stamp off "will this part be regenerated" would fail here, because reading
// the body materializes docModel and so regenerates document.xml.
func TestReadOnlyAccessDoesNotStampModified(t *testing.T) {
	raw, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc := openFixture(t, raw)
	before := doc.Properties.Modified

	readEverything(t, doc)

	pastSecondBoundary()
	saved := saveDoc(t, doc)

	if !doc.Properties.Modified.Equal(before) {
		t.Errorf("reading the document moved Properties.Modified from %s to %s",
			before, doc.Properties.Modified)
	}
	if doc.modelEdits != 0 {
		t.Errorf("reading the document recorded %d content edits", doc.modelEdits)
	}
	if untouched := saveDoc(t, openFixture(t, raw)); !bytes.Equal(saved, untouched) {
		t.Errorf("saving a document that was only read differs from saving an untouched one "+
			"(%d vs %d bytes)", len(saved), len(untouched))
	}
}

// readEverything walks the read API, including the accessors that build a model
// as a side effect of the read.
func readEverything(t *testing.T, doc *Document) {
	t.Helper()
	doc.Text()
	for _, p := range doc.Paragraphs() {
		p.Text()
		p.Style()
		p.Alignment()
		for _, r := range p.Runs() {
			r.Text()
			r.Bold()
		}
	}
	for _, tbl := range doc.Tables() {
		tbl.Style()
		for _, row := range tbl.Rows() {
			for _, cell := range row.Cells() {
				cell.Text()
				cell.Paragraphs()
			}
		}
	}
	for _, s := range doc.Sections() {
		s.PageSize()
		s.Margins()
		s.Orientation()
		s.Columns()
		s.PageNumbering()
		s.TitlePage()
		s.SectionType()
		s.PageBorders()
		s.LineNumbering()
		s.VerticalAlignment()
		s.PaperSource()
		s.DocumentGrid()
		s.FootnoteProperties()
		s.EndnoteProperties()
	}
	for _, c := range doc.ContentControls() {
		c.Tag()
		c.Alias()
		c.Value()
		c.Type()
		c.Options()
		c.Checked()
		c.DateFormat()
		c.DataBinding()
	}
	for _, h := range doc.Headers() {
		h.Paragraphs()
	}
	for _, f := range doc.Footers() {
		f.Paragraphs()
	}
	doc.Styles().List()
	doc.Numbering()
	doc.Comments()
	doc.Footnotes()
	doc.Endnotes()
	doc.Sources()
	doc.Revisions()
	doc.Images()
	doc.Hyperlinks()
	doc.Watermark()
	doc.Protection()
	doc.CustomXMLParts()
	doc.Bookmarks()
}

// TestContentEditStampsModified: a real edit records the time it was made, for
// every kind of part the package writes — the body, a header, each flag-gated
// metadata part, and the main-part handles that carry no flag at all.
func TestContentEditStampsModified(t *testing.T) {
	raw, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(t *testing.T, doc *Document)
	}{
		{"body run", func(t *testing.T, doc *Document) {
			doc.Paragraphs()[0].Runs()[0].SetText("EDITED")
		}},
		{"add paragraph", func(t *testing.T, doc *Document) {
			doc.AddParagraph()
		}},
		{"add table", func(t *testing.T, doc *Document) {
			doc.AddTable(2, 2)
		}},
		{"table cell", func(t *testing.T, doc *Document) {
			doc.Tables()[0].Rows()[0].Cells()[0].SetShading("FF0000")
		}},
		{"section layout", func(t *testing.T, doc *Document) {
			doc.Sections()[0].SetPageSize(200, 400)
		}},
		{"content control", func(t *testing.T, doc *Document) {
			ccs := doc.ContentControls()
			if len(ccs) == 0 {
				t.Skip("fixture has no content control")
			}
			ccs[0].SetValue("NEW")
		}},
		{"header", func(t *testing.T, doc *Document) {
			hs := doc.Headers()
			if len(hs) == 0 {
				t.Skip("fixture has no header")
			}
			hs[0].Paragraphs()[0].AddRun().SetText("HDR")
		}},
		{"styles", func(t *testing.T, doc *Document) {
			doc.Styles().AddStyle(StyleTypeParagraph, "MyStyle", "My Style")
		}},
		{"numbering", func(t *testing.T, doc *Document) {
			doc.AddBulletList()
		}},
		{"settings", func(t *testing.T, doc *Document) {
			doc.Protect(DocumentProtectionOptions{Edit: EditReadOnly})
		}},
		{"footnote", func(t *testing.T, doc *Document) {
			doc.Paragraphs()[0].AddRun().AddFootnote("note")
		}},
		{"theme", func(t *testing.T, doc *Document) {
			theme := doc.Theme()
			if theme == nil {
				t.Skip("fixture has no theme part")
			}
			theme.SetName("Renamed")
		}},
		{"custom xml", func(t *testing.T, doc *Document) {
			if _, err := doc.AddCustomXMLPart([]byte(`<root xmlns="urn:x"/>`)); err != nil {
				t.Fatal(err)
			}
		}},
		{"bibliography", func(t *testing.T, doc *Document) {
			if err := doc.AddSource(Source{Tag: "SRC1", Type: "Book", Title: "T"}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := openFixture(t, raw)
			before := doc.Properties.Modified
			start := time.Now().Add(-time.Second)

			tc.mutate(t, doc)
			saved := saveDoc(t, doc)

			if doc.Properties.Modified.Equal(before) {
				t.Fatalf("editing the document left Properties.Modified at %s", before)
			}
			if doc.Properties.Modified.Before(start) {
				t.Errorf("Properties.Modified is %s, which predates the edit", doc.Properties.Modified)
			}
			// And the stamp is in the written package, not just in memory.
			// dcterms:modified has one-second resolution, so the round-tripped
			// value is the stamp truncated, not the stamp.
			reopened := openFixture(t, saved)
			if drift := doc.Properties.Modified.Sub(reopened.Properties.Modified); drift < 0 || drift >= time.Second {
				t.Errorf("saved docProps/core.xml carries %s, want %s (within a second)",
					reopened.Properties.Modified, doc.Properties.Modified)
			}
			if reopened.Properties.Created.IsZero() {
				t.Error("saved docProps/core.xml lost dcterms:created")
			}
		})
	}
}

// TestSavesAfterEditAreByteIdentical: the stamp is taken once per change, not
// once per save. Two saves of the same edited document a second apart must
// agree, or a save-twice pipeline sees two different packages for one edit.
func TestSavesAfterEditAreByteIdentical(t *testing.T) {
	doc := openFixture(t, fixtureWithCoreProps(t, modifiedFixtureBody))
	doc.AddParagraphWithText("EDIT")

	first := saveDoc(t, doc)
	stamped := doc.Properties.Modified
	pastSecondBoundary()
	second := saveDoc(t, doc)

	if !doc.Properties.Modified.Equal(stamped) {
		t.Errorf("a second save of the same edit re-stamped Properties.Modified: %s -> %s",
			stamped, doc.Properties.Modified)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("two saves of one edit differ (%d vs %d bytes)", len(first), len(second))
	}
}

// TestEditAfterSaveStampsAgain: the counter is baselined at each save rather
// than latched, so a second round of edits is distinguishable from the first
// one saved twice.
func TestEditAfterSaveStampsAgain(t *testing.T) {
	doc := openFixture(t, fixtureWithCoreProps(t, modifiedFixtureBody))
	doc.AddParagraphWithText("FIRST")
	saveDoc(t, doc)
	first := doc.Properties.Modified

	pastSecondBoundary()
	doc.AddParagraphWithText("SECOND")
	saveDoc(t, doc)

	if !doc.Properties.Modified.After(first) {
		t.Errorf("an edit made after a save left Properties.Modified at %s (want later than %s)",
			doc.Properties.Modified, first)
	}
}

// TestEditedSaveReopensToTheSameBytes: an edited document, reopened and saved
// untouched, reproduces its own bytes. This is what catches regenerated core
// properties being appended by Close instead of written where the source part
// was: the content is identical, only the archive order would differ.
func TestEditedSaveReopensToTheSameBytes(t *testing.T) {
	raw, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc := openFixture(t, raw)
	doc.AddParagraphWithText("EDIT")
	edited := saveDoc(t, doc)

	resaved := saveDoc(t, openFixture(t, edited))
	if !bytes.Equal(edited, resaved) {
		t.Errorf("reopening an edited document and saving it untouched produced different bytes "+
			"(%d vs %d): docProps/core.xml has moved within the archive", len(edited), len(resaved))
	}
}

// TestExplicitModifiedSurvivesSave: setting Properties.Modified is itself a
// property edit, and the caller's value wins over the stamp.
func TestExplicitModifiedSurvivesSave(t *testing.T) {
	want := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

	t.Run("opened", func(t *testing.T) {
		doc := openFixture(t, fixtureWithCoreProps(t, modifiedFixtureBody))
		doc.Properties.Modified = want
		doc.AddParagraphWithText("EDIT")

		saved := saveDoc(t, doc)
		if !doc.Properties.Modified.Equal(want) {
			t.Errorf("the save overwrote an explicit Properties.Modified: got %s, want %s",
				doc.Properties.Modified, want)
		}
		if got := openFixture(t, saved).Properties.Modified; !got.Equal(want) {
			t.Errorf("saved docProps/core.xml carries %s, want %s", got, want)
		}
	})

	t.Run("created", func(t *testing.T) {
		doc := Create()
		doc.Properties.Modified = want
		doc.AddParagraphWithText("EDIT")

		saved := saveDoc(t, doc)
		if got := openFixture(t, saved).Properties.Modified; !got.Equal(want) {
			t.Errorf("saved docProps/core.xml carries %s, want %s", got, want)
		}
	})
}

// TestPropertiesOnlyEditDoesNotStamp: authoring the metadata is not a content
// change; the caller's core properties are written back as given.
func TestPropertiesOnlyEditDoesNotStamp(t *testing.T) {
	doc := openFixture(t, fixtureWithCoreProps(t, modifiedFixtureBody))
	before := doc.Properties.Modified
	if !before.Equal(baseModified(t)) {
		t.Fatalf("fixture parsed dcterms:modified as %s, want %s", before, fixtureModified)
	}
	doc.Properties.Title = "Retitled"

	saved := saveDoc(t, doc)
	if !doc.Properties.Modified.Equal(before) {
		t.Errorf("a properties-only edit stamped Properties.Modified: %s -> %s",
			before, doc.Properties.Modified)
	}
	core := mustZipEntry(t, saved, "docProps/core.xml")
	if !strings.Contains(core, "Retitled") {
		t.Errorf("the title edit is missing from docProps/core.xml:\n%s", core)
	}
	if !strings.Contains(core, fixtureModified) {
		t.Errorf("docProps/core.xml no longer carries the source dcterms:modified:\n%s", core)
	}
}

// TestCreatedDocumentStampsWhenEdited: a document built from scratch records
// the time its content was authored, and an empty one saved twice is still
// reproducible.
func TestCreatedDocumentStampsWhenEdited(t *testing.T) {
	t.Run("with content", func(t *testing.T) {
		start := time.Now().Add(-time.Second)
		doc := Create()
		doc.AddParagraphWithText("hello")

		saved := saveDoc(t, doc)
		got := openFixture(t, saved).Properties.Modified
		if got.IsZero() {
			t.Fatal("a created document with content saved with no dcterms:modified")
		}
		if got.Before(start) {
			t.Errorf("dcterms:modified is %s, which predates the edit", got)
		}
	})

	t.Run("repeat save", func(t *testing.T) {
		doc := Create()
		doc.AddParagraphWithText("hello")

		first := saveDoc(t, doc)
		pastSecondBoundary()
		second := saveDoc(t, doc)
		if !bytes.Equal(first, second) {
			t.Errorf("two saves of a created document differ (%d vs %d bytes)",
				len(first), len(second))
		}
	})
}
