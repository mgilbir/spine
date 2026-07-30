package docx

import (
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/common/omml"
)

// featureTouchFixture builds a docx whose default section references a header
// part carrying one plain paragraph and one hyperlink paragraph. The header is
// round-tripped from preserved raw bytes unless it is flagged modified, so a
// mutation through a live handle into it is masked on save unless the mutator
// calls touch() (C266/C406).
func featureTouchFixture(t *testing.T) []byte {
	return featureTouchFixtureWith(t, "")
}

// featureTouchFixtureWith is featureTouchFixture with extra properties on the
// header's first paragraph (a w:pPr fragment), so a removal-shaped mutator has
// something to remove.
func featureTouchFixtureWith(t *testing.T, firstParaPPr string) []byte {
	t.Helper()
	return featureTouchFixtureNS(t, firstParaPPr, "")
}

// featureTouchFixtureNS is featureTouchFixtureWith plus extra namespace
// declarations on the header root, for mutators that write content in a
// namespace the base fixture's w:hdr does not declare (m: for math). The header
// replays its captured root attributes verbatim, so a mutator that emits a
// foreign prefix needs the declaration to already be there.
func featureTouchFixtureNS(t *testing.T, firstParaPPr, hdrExtraNS string) []byte {
	t.Helper()
	const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`
	const documentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `><w:body>` +
		`<w:p><w:r><w:t xml:space="preserve">main body</w:t></w:r></w:p>` +
		`<w:sectPr><w:headerReference w:type="default" r:id="rId10"/></w:sectPr>` +
		`</w:body></w:document>`
	const documentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/></Relationships>`
	headerXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:hdr ` + fixtureWNS + hdrExtraNS + `>` +
		`<w:p>` + firstParaPPr + `<w:r><w:t xml:space="preserve">HEADER</w:t></w:r></w:p>` +
		`<w:p><w:hyperlink r:id="rId1" w:history="1"><w:r><w:t xml:space="preserve">LINK</w:t></w:r></w:hyperlink></w:p>` +
		`</w:hdr>`
	const headerRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com/" TargetMode="External"/></Relationships>`

	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  fixtureRootRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": documentRels,
		"word/header1.xml":             headerXML,
		"word/_rels/header1.xml.rels":  headerRels,
	})
}

// headerParagraph returns the first paragraph of the fixture's header, as a
// live handle into the parsed header model.
func headerParagraph(t *testing.T, doc *Document) *Paragraph {
	t.Helper()
	hdrs := doc.Headers()
	if len(hdrs) != 1 {
		t.Fatalf("got %d headers, want 1", len(hdrs))
	}
	paras := hdrs[0].Paragraphs()
	if len(paras) < 2 {
		t.Fatalf("got %d header paragraphs, want 2", len(paras))
	}
	return paras[0]
}

// TestFeatureMutatorsFlagHeaderPart is the C406 class sweep: every public
// feature mutator that appends to or edits a CT_P/CT_R must flag the header or
// footer part that owns it, or the preserved raw bytes mask the edit on save.
// Each case mutates the header through a live handle and asserts the saved
// header part carries the result.
func TestFeatureMutatorsFlagHeaderPart(t *testing.T) {
	fixed := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	cases := []struct {
		name string
		// hdrPPr seeds the header's first paragraph with properties, so a
		// removal-shaped mutator has something to remove; hdrNS adds root
		// namespace declarations content in a foreign namespace needs.
		hdrPPr string
		hdrNS  string
		mutate func(t *testing.T, doc *Document)
		// want must appear in the saved header part, gone must not. Exactly one
		// is set per case.
		want string
		gone string
	}{
		{name: "AddInsertedRun", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddInsertedRunWithDate("A", "INSERTED", fixed)
		}, want: "INSERTED"},
		{name: "MarkInserted", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).Runs()[0].MarkInsertedWithDate("A", fixed)
		}, want: "<w:ins "},
		{name: "MarkDeleted", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).Runs()[0].MarkDeletedWithDate("A", fixed)
		}, want: "<w:del "},
		{name: "AddMoveFromRun", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddMoveFromRunWithDate("A", "m1", "MOVED", fixed)
		}, want: "moveFromRangeStart"},
		{name: "AddMoveToRun", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddMoveToRunWithDate("A", "m1", "MOVED", fixed)
		}, want: "moveToRangeStart"},
		{name: "AddField", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddField(FieldPage)
		}, want: "PAGE"},
		{name: "AddMergeField", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddMergeField("Recipient")
		}, want: "MERGEFIELD"},
		{name: "AddCitation", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddCitation("Smi20")
		}, want: "CITATION"},
		{name: "AddFormField", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddFormField(FormFieldOptions{Name: "FF"})
		}, want: "FORMTEXT"},
		{name: "AddContentControl", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddContentControl("tag1", "CCVALUE")
		}, want: "CCVALUE"},
		{name: "ParagraphAddComment", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddComment("A", "note")
		}, want: "commentRangeStart"},
		{name: "RunAddComment", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).Runs()[0].AddComment("A", "note")
		}, want: "commentRangeStart"},
		{name: "AddFootnote", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).Runs()[0].AddFootnote("fn")
		}, want: "footnoteReference"},
		{name: "AddEndnote", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).Runs()[0].AddEndnote("en")
		}, want: "endnoteReference"},
		{name: "AddBookmark", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddBookmark("Mark")
		}, want: "bookmarkStart"},
		{name: "AddSignatureLine", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).AddSignatureLine(SignatureLineOptions{Signer: "S"})
		}, want: "signatureline"},
		{name: "SetListStyle", mutate: func(t *testing.T, doc *Document) {
			headerParagraph(t, doc).SetListStyle(doc.AddBulletList(), 0)
		}, want: "<w:numPr>"},

		// The math zone mutators reach the paragraph model directly rather than
		// through mut(), so a header equation was written into the model and
		// then masked by the preserved header bytes.
		{name: "AddMath", hdrNS: " " + mathNSDecl, mutate: func(t *testing.T, doc *Document) {
			if err := headerParagraph(t, doc).AddMath(&omml.OMath{Items: []omml.MathItem{mathRun("E=mc2")}}); err != nil {
				t.Fatalf("AddMath: %v", err)
			}
		}, want: "<m:oMath>"},
		{name: "AddMathPara", hdrNS: " " + mathNSDecl, mutate: func(t *testing.T, doc *Document) {
			mp := &omml.OMathPara{OMath: []*omml.OMath{{Items: []omml.MathItem{mathRun("E=mc2")}}}}
			if err := headerParagraph(t, doc).AddMathPara(mp); err != nil {
				t.Fatalf("AddMathPara: %v", err)
			}
		}, want: "<m:oMathPara>"},

		// The two removal-shaped paragraph property mutators: unlike their
		// SetBorders/SetShading counterparts they do not funnel through
		// ensurePPr, so they cleared the model and left the preserved header
		// bytes — and the border/shading — in place.
		{name: "ClearBorders", hdrPPr: `<w:pPr><w:pBdr><w:top w:val="single" w:sz="4" w:color="FF0000"/></w:pBdr></w:pPr>`,
			mutate: func(t *testing.T, doc *Document) {
				headerParagraph(t, doc).ClearBorders()
			}, gone: "<w:pBdr>"},
		{name: "ClearShading", hdrPPr: `<w:pPr><w:shd w:val="clear" w:fill="FF0000"/></w:pPr>`,
			mutate: func(t *testing.T, doc *Document) {
				headerParagraph(t, doc).ClearShading()
			}, gone: "<w:shd "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := openFixture(t, featureTouchFixtureNS(t, tc.hdrPPr, tc.hdrNS))
			tc.mutate(t, doc)
			hdr := mustZipEntry(t, saveDoc(t, doc), "word/header1.xml")
			if tc.want != "" && !strings.Contains(hdr, tc.want) {
				t.Errorf("header1.xml does not carry %q after %s — the edit was masked by the preserved bytes:\n%s",
					tc.want, tc.name, hdr)
			}
			if tc.gone != "" && strings.Contains(hdr, tc.gone) {
				t.Errorf("header1.xml still carries %q after %s — the edit was masked by the preserved bytes:\n%s",
					tc.gone, tc.name, hdr)
			}
		})
	}
}

// TestRemoveListStyleFlagsHeaderPart covers the removal half of the C406 sweep:
// RemoveListStyle edits pPr directly rather than through ensurePPr, so it was
// the one numbering mutator with no touch. The header paragraph starts out
// carrying a numbering reference, so the removal is observable in the saved
// part.
func TestRemoveListStyleFlagsHeaderPart(t *testing.T) {
	const numPr = `<w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>`
	doc := openFixture(t, featureTouchFixtureWith(t, numPr))
	headerParagraph(t, doc).RemoveListStyle()
	hdr := mustZipEntry(t, saveDoc(t, doc), "word/header1.xml")
	if strings.Contains(hdr, "<w:numPr>") {
		t.Errorf("header1.xml still carries the numbering reference after RemoveListStyle:\n%s", hdr)
	}
}

// TestAddFootnoteOnHeaderHyperlinkRunKeepsReference is the audit's named
// today-reachable C406 case: a run from Hyperlink.Runs() in a preserved header.
// anchorNoteRef appends the reference into the run itself (the run is not a
// direct paragraph child), footnotes.xml *is* regenerated with the new note, so
// without the touch the package ends up with an orphan note and no reference.
func TestAddFootnoteOnHeaderHyperlinkRunKeepsReference(t *testing.T) {
	doc := openFixture(t, featureTouchFixture(t))
	links := doc.Hyperlinks()
	if len(links) != 1 {
		t.Fatalf("got %d hyperlinks, want 1", len(links))
	}
	if n := links[0].Runs()[0].AddFootnote("note text"); n == nil {
		t.Fatal("AddFootnote returned nil")
	}
	saved := saveDoc(t, doc)
	notes := mustZipEntry(t, saved, "word/footnotes.xml")
	if !strings.Contains(notes, "note text") {
		t.Fatal("footnotes.xml does not carry the new note")
	}
	hdr := mustZipEntry(t, saved, "word/header1.xml")
	if !strings.Contains(hdr, "footnoteReference") {
		t.Errorf("header1.xml has no footnoteReference: the note in footnotes.xml is an orphan\n%s", hdr)
	}
}
