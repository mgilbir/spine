package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// hdrFtrHandleFixture builds a docx whose default section references a header
// part carrying both an external hyperlink and an inline image. The handles
// handed out by Document.Hyperlinks()/Document.Images() point into this parsed
// header model, so mutating them must flag the header part for regeneration on
// save (C266) — otherwise the preserved raw header bytes mask the edit.
func hdrFtrHandleFixture(t *testing.T) []byte {
	t.Helper()
	const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`
	const documentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `><w:body>` +
		`<w:p><w:r><w:t xml:space="preserve">main body</w:t></w:r></w:p>` +
		`<w:sectPr><w:headerReference w:type="default" r:id="rId10"/></w:sectPr>` +
		`</w:body></w:document>`
	const documentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/></Relationships>`
	const drawingNS = `xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	const headerXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:hdr ` + fixtureWNS + `>` +
		`<w:p><w:hyperlink r:id="rId1" w:history="1"><w:r><w:t xml:space="preserve">OLD LINK</w:t></w:r></w:hyperlink></w:p>` +
		`<w:p><w:r><w:drawing>` +
		`<wp:inline ` + drawingNS + `>` +
		`<wp:extent cx="914400" cy="914400"/>` +
		`<wp:docPr id="1" name="Picture 1" descr="old alt"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:pic><pic:nvPicPr><pic:cNvPr id="1" name="Picture 1"/><pic:cNvPicPr/></pic:nvPicPr>` +
		`<pic:blipFill><a:blip r:embed="rId2"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>` +
		`</pic:pic></a:graphicData></a:graphic>` +
		`</wp:inline>` +
		`</w:drawing></w:r></w:p>` +
		`</w:hdr>`
	const headerRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com/" TargetMode="External"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`

	parts := map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  fixtureRootRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": documentRels,
		"word/header1.xml":             headerXML,
		"word/_rels/header1.xml.rels":  headerRels,
		"word/media/image1.png":        string(minimalPNG()),
	}
	return buildFixtureDocx(t, parts)
}

// TestHeaderHandleEditsPersist verifies that edits made through the live
// Paragraph/Run/Hyperlink/InlineImage handles handed out for a reopened
// header — a run SetText, a hyperlink SetTooltip, and an image SetAltText —
// are written back to the header part instead of being masked by the preserved
// original bytes (C266).
func TestHeaderHandleEditsPersist(t *testing.T) {
	fixture := hdrFtrHandleFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	imgs := doc.Images()
	if len(imgs) != 1 {
		t.Fatalf("want 1 header image, got %d", len(imgs))
	}
	if got := imgs[0].AltText(); got != "old alt" {
		t.Fatalf("pre-edit alt text = %q, want %q", got, "old alt")
	}
	imgs[0].SetAltText("NEW ALT 2026")

	links := doc.Hyperlinks()
	if len(links) != 1 {
		t.Fatalf("want 1 header hyperlink, got %d", len(links))
	}
	links[0].SetTooltip("NEW TIP")
	runs := links[0].Runs()
	if len(runs) != 1 {
		t.Fatalf("want 1 hyperlink run, got %d", len(runs))
	}
	runs[0].SetText("NEW LINK TEXT")

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	hdr, ok := zipEntry(t, saved, "word/header1.xml")
	if !ok {
		t.Fatal("word/header1.xml missing from saved package")
	}
	s := string(hdr)
	if !strings.Contains(s, `descr="NEW ALT 2026"`) {
		t.Errorf("image alt-text edit not written to header:\n%s", s)
	}
	if !strings.Contains(s, `w:tooltip="NEW TIP"`) {
		t.Errorf("hyperlink tooltip edit not written to header:\n%s", s)
	}
	if !strings.Contains(s, "NEW LINK TEXT") {
		t.Errorf("run text edit not written to header:\n%s", s)
	}

	// The result must reopen cleanly and read the edits back.
	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := re.Images()[0].AltText(); got != "NEW ALT 2026" {
		t.Errorf("reopened alt text = %q, want %q", got, "NEW ALT 2026")
	}
	if got := re.Hyperlinks()[0].Tooltip(); got != "NEW TIP" {
		t.Errorf("reopened tooltip = %q, want %q", got, "NEW TIP")
	}
}

// TestHeaderRunAddImageNoOrphan verifies that adding an image to a run in a
// reopened header writes the new media part AND its relationship into the
// header's rels — the preserved header bytes must not mask the new drawing,
// which would leave the media part orphaned with a dangling in-memory rel
// (C266).
func TestHeaderRunAddImageNoOrphan(t *testing.T) {
	fixture := hdrFtrHandleFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// A live run in the reopened header (the hyperlink's display run).
	run := doc.Hyperlinks()[0].Runs()[0]
	img, err := run.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG)
	if err != nil {
		t.Fatalf("AddImageFromBytes: %v", err)
	}
	// image1.png already exists in the package, so the new part is image2.png.
	if got := img.PartName(); got != "/word/media/image2.png" {
		t.Fatalf("new image part = %q, want /word/media/image2.png", got)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, ok := zipEntry(t, saved, "word/media/image2.png"); !ok {
		t.Fatal("new media part image2.png not written")
	}
	rels, ok := zipEntry(t, saved, "word/_rels/header1.xml.rels")
	if !ok {
		t.Fatal("header rels missing from saved package")
	}
	if !strings.Contains(string(rels), "media/image2.png") {
		t.Errorf("header rels does not reference the new image (orphan media part):\n%s", string(rels))
	}
	hdr, _ := zipEntry(t, saved, "word/header1.xml")
	if !bytes.Contains(hdr, []byte("<w:drawing")) {
		t.Errorf("new drawing not written to header:\n%s", string(hdr))
	}

	// The saved package must reopen without a dangling-relationship error.
	if _, err := OpenReader(bytes.NewReader(saved), int64(len(saved))); err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
}
