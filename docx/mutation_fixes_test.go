package docx

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// parseHdrFtr unmarshals a header/footer part body for marshal-path tests.
func parseHdrFtr(t *testing.T, xmlStr string) *oxml.CT_HdrFtr {
	t.Helper()
	hdr := &oxml.CT_HdrFtr{}
	if err := xml.Unmarshal([]byte(xmlStr), hdr); err != nil {
		t.Fatal(err)
	}
	return hdr
}

// C63 residual: SetText replaced only the runs, leaving stale hyperlink/SDT
// text in the paragraph.
func TestSetTextClearsAllContentChildren(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:t>lead </w:t></w:r>` +
		`<w:hyperlink r:id="rId5"><w:r><w:t>LINKTEXT</w:t></w:r></w:hyperlink>` +
		`<w:sdt><w:sdtPr/><w:sdtContent><w:r><w:t>SDTTEXT</w:t></w:r></w:sdtContent></w:sdt>` +
		`<w:smartTag w:uri="urn:z" w:element="tag"><w:r><w:t>SMARTTEXT</w:t></w:r></w:smartTag>` +
		`</w:p></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openFixture(t, fixture)

	doc.Paragraphs()[0].SetText("replaced")

	saved := saveDoc(t, doc)
	out := mustZipEntry(t, saved, "word/document.xml")
	for _, stale := range []string{"LINKTEXT", "SDTTEXT", "SMARTTEXT", "hyperlink"} {
		if strings.Contains(out, stale) {
			t.Errorf("stale content %q survived SetText:\n%s", stale, out)
		}
	}
	if !strings.Contains(out, ">replaced</w:t>") {
		t.Fatalf("new text missing:\n%s", out)
	}
	if got := doc.Paragraphs()[0].Text(); got != "replaced" {
		t.Fatalf("Text() = %q, want %q", got, "replaced")
	}
}

// C25: with two images in one run, mutating the second image's handle
// rewrote the FIRST drawing (first-match), duplicating its rId and dropping
// the first image.
func TestTwoImagesInOneRunKeepDistinctDrawings(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()

	img1, err := run.AddImageFromBytes([]byte("PNGDATA-ONE"), opc.ContentTypePNG)
	if err != nil {
		t.Fatal(err)
	}
	img2, err := run.AddImageFromBytes([]byte("PNGDATA-TWO"), opc.ContentTypePNG)
	if err != nil {
		t.Fatal(err)
	}
	img1.SetSize(100, 100)
	img2.SetSize(50, 25)

	saved := saveDoc(t, doc)
	out := mustZipEntry(t, saved, "word/document.xml")

	if strings.Count(out, `r:embed="`+img1.relID+`"`) != 1 {
		t.Errorf("first image rId %s should appear exactly once:\n%s", img1.relID, out)
	}
	if strings.Count(out, `r:embed="`+img2.relID+`"`) != 1 {
		t.Errorf("second image rId %s should appear exactly once:\n%s", img2.relID, out)
	}
	// 50pt x 25pt = 635000 x 317500 EMU; 100pt = 1270000 EMU.
	if !strings.Contains(out, `cx="1270000" cy="1270000"`) {
		t.Errorf("first image lost its size:\n%s", out)
	}
	if !strings.Contains(out, `cx="635000" cy="317500"`) {
		t.Errorf("second image lost its size:\n%s", out)
	}
}

// C65: AddImage in a table cell errored with "run is not attached to a
// document" because the document backref was never propagated through
// table -> row -> cell -> paragraph.
func TestTableCellAddImage(t *testing.T) {
	doc := Create()
	table := doc.AddTable(1, 1)
	cell := table.Rows()[0].Cells()[0]
	run := cell.AddParagraph().AddRun()

	img, err := run.AddImageFromBytes([]byte("PNGDATA-CELL"), opc.ContentTypePNG)
	if err != nil {
		t.Fatalf("AddImage in table cell: %v", err)
	}
	img.SetSize(10, 10)

	saved := saveDoc(t, doc)
	if _, ok := zipEntry(t, saved, "word/media/image1.png"); !ok {
		t.Fatal("media part missing")
	}
	out := mustZipEntry(t, saved, "word/document.xml")
	if !strings.Contains(out, `r:embed="`+img.relID+`"`) {
		t.Fatalf("cell drawing missing embed:\n%s", out)
	}
	rels := mustZipEntry(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, `Target="media/image1.png"`) {
		t.Fatalf("image relationship missing:\n%s", rels)
	}

	// The pre-existing seed cells (via Tables()/Cells()/Paragraphs()) must
	// also carry the backref.
	run2 := doc.Tables()[0].Rows()[0].Cells()[0].Paragraphs()[0].AddRun()
	if _, err := run2.AddImageFromBytes([]byte("PNGDATA-CELL2"), opc.ContentTypePNG); err != nil {
		t.Fatalf("AddImage in seed cell paragraph: %v", err)
	}
}

// C227: AddSectionBreak attached the old sectPr to the last element of
// body.P even when the document ended with a table, moving the section
// boundary before the table.
func TestAddSectionBreakAfterTable(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("first")
	tbl := doc.AddTable(1, 1)
	tbl.Rows()[0].Cells()[0].AddParagraph().AddRun().SetText("cell")

	doc.AddSectionBreak()

	saved := saveDoc(t, doc)
	out := mustZipEntry(t, saved, "word/document.xml")

	// The sectPr-carrying paragraph must come after the table, not attach to
	// the paragraph that precedes it.
	tblEnd := strings.Index(out, "</w:tbl>")
	sectPrPos := strings.Index(out, "<w:sectPr")
	if tblEnd == -1 || sectPrPos == -1 {
		t.Fatalf("missing table or sectPr:\n%s", out)
	}
	if sectPrPos < tblEnd {
		t.Fatalf("section break landed before the end of the table:\n%s", out)
	}
	if !strings.Contains(out, "</w:tbl><w:p><w:pPr><w:sectPr") {
		t.Fatalf("expected a new sectPr paragraph directly after the table:\n%s", out)
	}
}

// C228: Body()/Paragraphs() omitted paragraphs inside body-level SDTs.
func TestParagraphsIncludeSdtWrappedContent(t *testing.T) {
	body := `<w:body>` +
		`<w:p><w:r><w:t>plain</w:t></w:r></w:p>` +
		`<w:sdt><w:sdtPr/><w:sdtContent><w:p><w:r><w:t>inside sdt</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
		`<w:p><w:r><w:t>after</w:t></w:r></w:p>` +
		`</w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openFixture(t, fixture)

	paras := doc.Paragraphs()
	if len(paras) != 3 {
		t.Fatalf("Paragraphs() = %d paragraphs, want 3 (SDT content omitted?)", len(paras))
	}
	if got := paras[1].Text(); got != "inside sdt" {
		t.Fatalf("SDT paragraph out of order or missing: %q", got)
	}
	if body := doc.Body(); !strings.Contains(body, "inside sdt") {
		t.Fatalf("Body() omits SDT text: %q", body)
	}
}

// C228: header/footer content marshal ignored childOrder and SdtBlock: a
// header containing an SDT lost it when the part was regenerated.
func TestHeaderWithSdtRoundTrips(t *testing.T) {
	hdrXML := `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:p><w:r><w:t>before</w:t></w:r></w:p>` +
		`<w:sdt><w:sdtPr/><w:sdtContent><w:p><w:r><w:t>HDRSDT</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
		`<w:p><w:r><w:t>after</w:t></w:r></w:p>` +
		`</w:hdr>`
	hdr := parseHdrFtr(t, hdrXML)
	data, err := marshalHdrFtrXML(hdr, "hdr")
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "HDRSDT") {
		t.Fatalf("SDT content dropped from header:\n%s", out)
	}
	before := strings.Index(out, "before")
	sdt := strings.Index(out, "HDRSDT")
	after := strings.Index(out, "after")
	if before >= sdt || sdt >= after {
		t.Fatalf("header children out of order:\n%s", out)
	}
}

// C229: nextImageNumber matched /word/media/image prefixes case-sensitively
// while OPC part names are case-insensitive, so /word/media/IMAGE1.PNG
// collided with the generated image1.png at save time.
func TestNextImageNumberCaseInsensitive(t *testing.T) {
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="PNG" ContentType="image/png"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels": fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/media/IMAGE1.PNG": "fake-png-bytes",
	})
	doc := openFixture(t, fixture)

	run := doc.Paragraphs()[0].AddRun()
	if _, err := run.AddImageFromBytes([]byte("PNGDATA-NEW"), opc.ContentTypePNG); err != nil {
		t.Fatal(err)
	}
	if got := doc.imageParts[0].partName; got != "/word/media/image2.png" {
		t.Fatalf("image number collides with IMAGE1.PNG: got %s", got)
	}
	saved := saveDoc(t, doc)
	if _, ok := zipEntry(t, saved, "word/media/image2.png"); !ok {
		t.Fatal("new media part missing from saved package")
	}
	if _, ok := zipEntry(t, saved, "word/media/IMAGE1.PNG"); !ok {
		t.Fatal("original media part lost")
	}
}
