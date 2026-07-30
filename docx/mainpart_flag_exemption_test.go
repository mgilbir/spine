package docx

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestZeroModificationRaisesNoFlag is the other side of the T2 guard. Making a
// mutator flag its part fixes a lost edit; making a *read* flag one breaks
// byte-identity, because a flagged part is regenerated instead of round-tripping
// its preserved bytes. Centralising the flag into funnels (Style.ensureRPr,
// ListLevel.ensureInd, InlineImage.applyEdit, ensureCommentEx) is exactly the
// change that could put a flag on a read path, so this pins that opening a
// document and walking every reader raises no flag at all.
func TestZeroModificationRaisesNoFlag(t *testing.T) {
	for _, path := range []string{"testdata/minimal.docx", "testdata/chart.docx", "testdata/svg_test.docx"} {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			doc := openFixture(t, raw)

			// Every read accessor that touches a flag-gated model, including the
			// ones whose write funnels now raise flags.
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
			for _, h := range doc.Headers() {
				h.Paragraphs()
			}
			for _, f := range doc.Footers() {
				f.Paragraphs()
			}

			flags := map[string]bool{
				"stylesModified": doc.stylesModified, "numberingModified": doc.numberingModified,
				"settingsModified": doc.settingsModified, "commentsModified": doc.commentsModified,
				"commentsExtModified": doc.commentsExtModified, "peopleModified": doc.peopleModified,
				"footnotesModified": doc.footnotesModified, "endnotesModified": doc.endnotesModified,
				"sourcesModified": doc.sourcesModified,
			}
			for name, set := range flags {
				if set {
					t.Errorf("reading %s raised %s: the part will be regenerated instead of "+
						"round-tripping its preserved bytes", path, name)
				}
			}
			if len(doc.modifiedHdrFtrParts) != 0 {
				t.Errorf("reading %s flagged header/footer parts %v for regeneration",
					path, doc.modifiedHdrFtrParts)
			}
			// The same claim for the main document part, which has no flag of
			// its own: reading it materializes docModel and so makes the save
			// regenerate the part, but it must not count as an edit or every
			// read would stamp dcterms:modified (see modified.go).
			if doc.modelEdits != 0 {
				t.Errorf("reading %s recorded %d content edits: every read would stamp "+
					"dcterms:modified and no save would be reproducible",
					path, doc.modelEdits)
			}

			// And the save is stable: two zero-modification saves agree byte for
			// byte, so no read introduced regeneration drift.
			if a, b := saveDoc(t, doc), saveDoc(t, openFixture(t, raw)); !bytes.Equal(a, b) {
				t.Errorf("zero-modification saves of %s differ (%d vs %d bytes)", path, len(a), len(b))
			}
		})
	}
}

// The main document part carries no *Modified flag: Section, Table, TableRow,
// TableCell and ContentControl all hold w:sectPr / w:tbl / w:sdt nodes that
// live in document.xml, and nothing has to be raised for an edit through them
// to be written out. (TestMutationsFlagTheirPart does track the part now, but
// for the document's modification time, not for persistence.)
//
// The claim that makes that safe is that it needs no flag — the part is
// regenerated whenever d.docModel is non-nil, and every constructor of those
// handles reaches the body through d.doc(), which materializes docModel as a
// side effect of the read. So obtaining the handle is already the flag.
//
// That is an argument, and arguments rot. This test checks it instead: each
// case mutates an opened document through one of those handles, saves, and
// asserts the edit is in the written part. If document.xml ever became
// preserve-by-default the way headers are, these fail rather than silently
// dropping every table and section edit in the library.
func TestMainPartHandleEditsPersist(t *testing.T) {
	body := `<w:body>` +
		`<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="4680"/></w:tblGrid>` +
		`<w:tr><w:tc><w:tcPr/><w:p><w:r><w:t>CELL</w:t></w:r></w:p></w:tc></w:tr></w:tbl>` +
		`<w:sdt><w:sdtPr><w:tag w:val="t1"/></w:sdtPr>` +
		`<w:sdtContent><w:p><w:r><w:t>SDT</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
		`<w:p><w:r><w:t>BODY</w:t></w:r></w:p>` +
		`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>` +
		`</w:body>`

	cases := []struct {
		name   string
		mutate func(t *testing.T, doc *Document)
		want   string
	}{
		{"Section.SetPageSize", func(t *testing.T, doc *Document) {
			doc.DefaultSection().SetPageSize(200, 400)
		}, `w:w="4000"`},
		{"Section.SetTitlePage", func(t *testing.T, doc *Document) {
			doc.DefaultSection().SetTitlePage(true)
		}, "<w:titlePg"},
		{"Table.SetStyle", func(t *testing.T, doc *Document) {
			doc.Tables()[0].SetStyle("TableGrid")
		}, `w:val="TableGrid"`},
		{"Table.AddRow", func(t *testing.T, doc *Document) {
			doc.Tables()[0].AddRow().AddCell().AddParagraph().AddText("ADDED")
		}, "ADDED"},
		{"TableRow.SetHeaderRow", func(t *testing.T, doc *Document) {
			doc.Tables()[0].Rows()[0].SetHeaderRow(true)
		}, "<w:tblHeader"},
		{"TableCell.SetShading", func(t *testing.T, doc *Document) {
			doc.Tables()[0].Rows()[0].Cells()[0].SetShading("FF0000")
		}, `w:fill="FF0000"`},
		{"ContentControl.SetValue", func(t *testing.T, doc *Document) {
			doc.ContentControls()[0].SetValue("NEWVALUE")
		}, "NEWVALUE"},
		{"ContentControl.SetAlias", func(t *testing.T, doc *Document) {
			doc.ContentControls()[0].SetAlias("Alias1")
		}, "Alias1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, body))
			tc.mutate(t, doc)
			main := mustZipEntry(t, saveDoc(t, doc), "word/document.xml")
			if !strings.Contains(main, tc.want) {
				t.Errorf("document.xml does not carry %q after %s — the main part is no longer "+
					"regenerated unconditionally, so these handles now need a modification flag "+
					"and the mutationFlagExempt rationale is stale:\n%s", tc.want, tc.name, main)
			}
		})
	}
}
