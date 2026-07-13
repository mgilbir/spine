package docx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// buildFixtureDocx builds an in-memory docx from part name -> content.
func buildFixtureDocx(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range parts {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const fixtureContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`

const fixtureRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`

// fixtureWithDocument builds a minimal docx whose document.xml root carries
// the given attributes (namespace declarations) and body content.
func fixtureWithDocument(t *testing.T, rootAttrs, body string) []byte {
	t.Helper()
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureContentTypes,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + rootAttrs + `>` + body + `</w:document>`,
	})
}

const fixtureWNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

// openSave opens a fixture, saves it, and returns the regenerated
// document.xml content.
func openSave(t *testing.T, fixture []byte) string {
	t.Helper()
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// The result must reopen cleanly.
	if _, err := OpenReader(bytes.NewReader(saved), int64(len(saved))); err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
	data, ok := zipEntry(t, saved, "word/document.xml")
	if !ok {
		t.Fatal("word/document.xml missing from saved package")
	}
	return string(data)
}

// C171: run-level children with no unmarshal/marshal path were silently
// deleted on every regenerated document.xml. The worst case is w:delText —
// tracked-deletion text — whose loss is irreversible: after a save, rejecting
// the change in Word can no longer restore the text.
func TestRunLevelContentPreservedOnSave(t *testing.T) {
	body := `<w:body>` +
		// tracked deletion with delText
		`<w:p><w:del w:id="2" w:author="a"><w:r><w:delText xml:space="preserve">GONE</w:delText></w:r></w:del></w:p>` +
		// VML picture with inline namespace declaration
		`<w:p><w:r><w:pict><v:shape xmlns:v="urn:schemas-microsoft-com:vml" id="s1" style="width:10pt"><v:textbox/></v:shape></w:pict></w:r></w:p>` +
		// embedded OLE object with w-namespaced attributes
		`<w:p><w:r><w:object w:dxaOrig="100" w:dyaOrig="100"><o:OLEObject xmlns:o="urn:schemas-microsoft-com:office:office" ProgID="Excel.Sheet"/></w:object></w:r></w:p>` +
		// comment anchor plus range markers
		`<w:p><w:commentRangeStart w:id="1"/><w:r><w:t>T</w:t></w:r><w:commentRangeEnd w:id="1"/><w:r><w:commentReference w:id="1"/></w:r></w:p>` +
		// positional tab
		`<w:p><w:r><w:ptab w:alignment="left" w:relativeTo="margin" w:leader="none"/></w:r></w:p>` +
		`<w:sectPr/></w:body>`

	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openSave(t, fixture)

	for _, want := range []string{
		`<w:delText xml:space="preserve">GONE</w:delText>`,
		`<w:pict>`,
		`<v:shape xmlns:v="urn:schemas-microsoft-com:vml" id="s1" style="width:10pt"><v:textbox/></v:shape>`,
		`<w:object w:dxaOrig="100" w:dyaOrig="100">`,
		`<o:OLEObject xmlns:o="urn:schemas-microsoft-com:office:office" ProgID="Excel.Sheet"/>`,
		`<w:commentReference w:id="1"/>`,
		`<w:ptab w:alignment="left" w:relativeTo="margin" w:leader="none"/>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("saved document.xml lost run-level content: %s", want)
		}
	}

	// delText must stay inside its tracked deletion.
	if del := strings.Index(doc, "<w:del "); del < 0 || !strings.Contains(doc[del:], "GONE") {
		t.Error("delText not preserved inside w:del")
	}
}

// Corpus class F1: mc:AlternateContent inside w:r (the standard Word layout
// for drawings with VML fallbacks and textboxes) was silently deleted on every
// regenerated document.xml — up to 1.58 MB of markup lost in one corpus file.
// It must round-trip verbatim, including inline xmlns declarations on
// mc:Choice/mc:Fallback and a fallback without an xmlns="" reset.
func TestRunAlternateContentPreservedOnSave(t *testing.T) {
	ac := `<mc:AlternateContent>` +
		`<mc:Choice Requires="wps"><w:drawing><wp:anchor xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" behindDoc="0"/></w:drawing></mc:Choice>` +
		`<mc:Fallback xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main"><w:pict><v:rect xmlns:v="urn:schemas-microsoft-com:vml" id="r1"/></w:pict></mc:Fallback>` +
		`</mc:AlternateContent>`
	// Word also wraps w:rFonts in an AlternateContent INSIDE w:rPr for
	// markup-compat font fallbacks (e.g. emoji fonts via w16se).
	rprAC := `<mc:AlternateContent>` +
		`<mc:Choice Requires="w16se"><w:rFonts w:ascii="Segoe UI Emoji"/></mc:Choice>` +
		`<mc:Fallback><w:rFonts w:ascii="Segoe UI Symbol"/></mc:Fallback>` +
		`</mc:AlternateContent>`
	body := `<w:body>` +
		`<w:p><w:r><w:rPr>` + rprAC + `<w:noProof/></w:rPr>` + ac + `<w:t>tail</w:t></w:r></w:p>` +
		`<w:sectPr/></w:body>`

	rootAttrs := fixtureWNS + ` xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"`
	fixture := fixtureWithDocument(t, rootAttrs, body)
	doc := openSave(t, fixture)

	if !strings.Contains(doc, ac) {
		t.Errorf("saved document.xml lost or rewrote run-level mc:AlternateContent;\nwant %s\ngot document: %s", ac, doc)
	}
	if !strings.Contains(doc, `<w:rPr>`+rprAC+`<w:noProof/></w:rPr>`) {
		t.Errorf("saved document.xml lost or rewrote rPr-level mc:AlternateContent;\nwant %s\ngot document: %s", rprAC, doc)
	}
	// The AlternateContent must stay in position: after rPr, before the text.
	if i, j := strings.Index(doc, "</w:rPr><mc:AlternateContent>"), strings.Index(doc, ">tail<"); i < 0 || j < 0 || i >= j {
		t.Errorf("run child order not preserved: ac=%d tail=%d", i, j)
	}
	if strings.Count(doc, "<mc:AlternateContent>") != 2 {
		t.Error("AlternateContent duplicated or dropped")
	}
}

// C171: preserved run children must survive additional save cycles and
// interact correctly with the childOrder bookkeeping when the document is
// mutated (a tracked append must not drop them, and they must not duplicate).
func TestRunLevelContentSurvivesMutationAndResave(t *testing.T) {
	body := `<w:body>` +
		`<w:p><w:r><w:t>before</w:t><w:ptab w:alignment="left" w:relativeTo="margin" w:leader="none"/><w:t>after</w:t></w:r></w:p>` +
		`<w:p><w:del w:id="2" w:author="a"><w:r><w:delText>GONE</w:delText></w:r></w:del></w:p>` +
		`<w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	doc.AddParagraphWithText("MUTATED")
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// Second cycle: reopen and save unmodified.
	doc2, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	saved2, err := doc2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	data, ok := zipEntry(t, saved2, "word/document.xml")
	if !ok {
		t.Fatal("word/document.xml missing")
	}
	out := string(data)
	for _, want := range []string{"MUTATED", "GONE", "<w:ptab "} {
		if !strings.Contains(out, want) {
			t.Errorf("content lost after mutate+resave cycles: %s", want)
		}
	}
	// The ptab must still sit between its two text elements.
	if i, j, k := strings.Index(out, ">before<"), strings.Index(out, "<w:ptab "), strings.Index(out, ">after<"); i >= j || j >= k {
		t.Errorf("run child order not preserved: before=%d ptab=%d after=%d", i, j, k)
	}
	if strings.Count(out, "<w:ptab ") != 1 {
		t.Error("ptab duplicated across save cycles")
	}
}

const mathNSDecl = `xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"`

// C173: oMath/oMathPara were re-emitted as unprefixed <oMath> because the WML
// builder never registered the math namespace — equations destroyed on every
// open+save. With the root-declared m: prefix (Word's standard layout) the
// equation must round-trip prefixed and bound.
func TestOMathRoundTripRootDeclared(t *testing.T) {
	body := `<w:body><w:p><m:oMath><m:r><m:t>x</m:t></m:r></m:oMath></w:p>` +
		`<w:p><m:oMathPara><m:oMath><m:r><m:t>y</m:t></m:r></m:oMath></m:oMathPara></w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS+" "+mathNSDecl, body)
	doc := openSave(t, fixture)

	for _, want := range []string{
		mathNSDecl, // root declaration preserved
		`<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath>`,
		`<m:oMathPara><m:oMath><m:r><m:t>y</m:t></m:r></m:oMath></m:oMathPara>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("saved document.xml lost math content: %s", want)
		}
	}
	if strings.Contains(doc, "<oMath") {
		t.Error("oMath re-emitted unprefixed")
	}
}

// C173: when the original document did not declare the math namespace at the
// root (it was declared inline on the oMath element), the regenerated root
// must gain the declaration so the prefixed re-emission stays bound.
func TestOMathRoundTripInlineDeclared(t *testing.T) {
	body := `<w:body><w:p><m:oMath ` + mathNSDecl + `><m:r><m:t>x</m:t></m:r></m:oMath></w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openSave(t, fixture)

	if !strings.Contains(doc, `<m:oMath>`) && !strings.Contains(doc, `<m:oMath `) {
		t.Error("oMath not re-emitted with m: prefix")
	}
	if !strings.Contains(doc, mathNSDecl) {
		t.Error("math namespace not declared anywhere in output; m: prefix unbound")
	}
	if !strings.Contains(doc, `<m:r><m:t>x</m:t></m:r>`) {
		t.Error("math content lost")
	}
}

// C173: a document without math must not gain a math namespace declaration —
// the regenerated root declarations stay exactly as captured.
func TestNoMathNoNamespaceDeclAdded(t *testing.T) {
	body := `<w:body><w:p><w:r><w:t>plain</w:t></w:r></w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openSave(t, fixture)
	if strings.Contains(doc, "officeDocument/2006/math") {
		t.Error("math namespace declared in a document without math")
	}
}

// C180: an image added to a header paragraph must be related from the header
// part's own rels (word/_rels/headerN.xml.rels), not document.xml.rels — an
// r:embed only resolves through the rels of the part it appears in.
func TestHeaderImageRelationshipIsPartScoped(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	h := doc.AddHeader(HeaderDefault)
	if _, err := h.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatal(err)
	}
	_, saved := reopen(t, doc)
	assertHdrFtrImageResolves(t, saved, "word/header1.xml")
}

// C180: footer counterpart.
func TestFooterImageRelationshipIsPartScoped(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	f := doc.AddFooter(FooterDefault)
	if _, err := f.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatal(err)
	}
	_, saved := reopen(t, doc)
	assertHdrFtrImageResolves(t, saved, "word/footer1.xml")
}

// C180: same behavior on the round-trip save path (document opened from a
// file).
func TestHeaderImageOnOpenedDocument(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	h := doc.AddHeader(HeaderDefault)
	if _, err := h.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatal(err)
	}
	_, saved := reopen(t, doc)
	assertHdrFtrImageResolves(t, saved, "word/header1.xml")
}

// assertHdrFtrImageResolves checks that the header/footer part references the
// image via r:embed, that the part has its own rels file resolving that ID to
// the media part, and that document.xml.rels does not carry the image rel.
func assertHdrFtrImageResolves(t *testing.T, saved []byte, partName string) {
	t.Helper()
	part, ok := zipEntry(t, saved, partName)
	if !ok {
		t.Fatalf("%s missing", partName)
	}
	embed := extractAttr(t, string(part), `r:embed="`)

	relsName := strings.Replace(partName, "word/", "word/_rels/", 1) + ".rels"
	rels, ok := zipEntry(t, saved, relsName)
	if !ok {
		t.Fatalf("%s missing: image relationship is dangling", relsName)
	}
	if !strings.Contains(string(rels), `Id="`+embed+`"`) {
		t.Errorf("%s does not resolve r:embed %s", relsName, embed)
	}
	if !strings.Contains(string(rels), `Target="media/image1.png"`) {
		t.Errorf("%s does not target the image part", relsName)
	}

	if _, ok := zipEntry(t, saved, "word/media/image1.png"); !ok {
		t.Error("image media part missing")
	}

	if docRels, ok := zipEntry(t, saved, "word/_rels/document.xml.rels"); ok {
		if strings.Contains(string(docRels), "media/image1.png") {
			t.Error("image relationship leaked into document.xml.rels")
		}
	}
}

// extractAttr returns the value of the first occurrence of `marker` (an
// attribute prefix ending in `="`).
func extractAttr(t *testing.T, s, marker string) string {
	t.Helper()
	i := strings.Index(s, marker)
	if i < 0 {
		t.Fatalf("marker %q not found", marker)
	}
	rest := s[i+len(marker):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		t.Fatalf("unterminated attribute at %q", marker)
	}
	return rest[:j]
}

// C180: an image used in both the body and a header is written once; each
// placement gets its own relationship in its own part's rels.
func TestBodyAndHeaderImageShareMediaPart(t *testing.T) {
	doc := Create()
	if _, err := doc.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatal(err)
	}
	h := doc.AddHeader(HeaderDefault)
	if _, err := h.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatal(err)
	}
	_, saved := reopen(t, doc)

	if got := zipEntryCount(t, saved, "word/media/image1.png"); got != 1 {
		t.Errorf("image1.png appears %d times, want 1", got)
	}
	if _, ok := zipEntry(t, saved, "word/media/image2.png"); ok {
		t.Error("identical image duplicated as image2.png")
	}

	docRels, ok := zipEntry(t, saved, "word/_rels/document.xml.rels")
	if !ok || !strings.Contains(string(docRels), `Target="media/image1.png"`) {
		t.Error("body image relationship missing from document.xml.rels")
	}
	hdrRels, ok := zipEntry(t, saved, "word/_rels/header1.xml.rels")
	if !ok || !strings.Contains(string(hdrRels), `Target="media/image1.png"`) {
		t.Error("header image relationship missing from header1.xml.rels")
	}
}

// C226: repeated AddHeader of the same type must replace the reference (ECMA
// allows at most one per type per sectPr) — latest content wins, and the
// orphaned session part and relationship are dropped.
func TestAddHeaderSameTypeReplaces(t *testing.T) {
	doc := Create()
	doc.AddHeader(HeaderDefault).AddParagraphWithText("OLD")
	doc.AddHeader(HeaderDefault).AddParagraphWithText("NEW")

	if got := len(doc.document.Body.SectPr.HeaderReference); got != 1 {
		t.Fatalf("headerReference count = %d, want 1", got)
	}

	_, saved := reopen(t, doc)

	if got := zipEntryCount(t, saved, "word/header1.xml"); got != 1 {
		t.Fatalf("header1.xml appears %d times, want 1", got)
	}
	if _, ok := zipEntry(t, saved, "word/header2.xml"); ok {
		t.Error("orphan header2.xml written")
	}
	hdr, _ := zipEntry(t, saved, "word/header1.xml")
	if !strings.Contains(string(hdr), "NEW") || strings.Contains(string(hdr), "OLD") {
		t.Errorf("latest header content must win, got: %s", hdr)
	}
	docXML, _ := zipEntry(t, saved, "word/document.xml")
	if strings.Count(string(docXML), "<w:headerReference") != 1 {
		t.Error("duplicate same-type headerReference in sectPr")
	}
	rels, _ := zipEntry(t, saved, "word/_rels/document.xml.rels")
	if strings.Count(string(rels), "header1.xml") != 1 {
		t.Error("orphan header relationship left in document.xml.rels")
	}
}

// C226: footer counterpart.
func TestAddFooterSameTypeReplaces(t *testing.T) {
	doc := Create()
	doc.AddFooter(FooterDefault).AddParagraphWithText("OLD")
	doc.AddFooter(FooterDefault).AddParagraphWithText("NEW")

	if got := len(doc.document.Body.SectPr.FooterReference); got != 1 {
		t.Fatalf("footerReference count = %d, want 1", got)
	}

	_, saved := reopen(t, doc)
	if got := zipEntryCount(t, saved, "word/footer1.xml"); got != 1 {
		t.Fatalf("footer1.xml appears %d times, want 1", got)
	}
	if _, ok := zipEntry(t, saved, "word/footer2.xml"); ok {
		t.Error("orphan footer2.xml written")
	}
	ftr, _ := zipEntry(t, saved, "word/footer1.xml")
	if !strings.Contains(string(ftr), "NEW") || strings.Contains(string(ftr), "OLD") {
		t.Errorf("latest footer content must win, got: %s", ftr)
	}
}

// C226: different types still coexist — replacement is per type.
func TestAddHeaderDifferentTypesCoexist(t *testing.T) {
	doc := Create()
	doc.AddHeader(HeaderDefault).AddParagraphWithText("D")
	doc.AddHeader(HeaderFirst).AddParagraphWithText("F")
	if got := len(doc.document.Body.SectPr.HeaderReference); got != 2 {
		t.Fatalf("headerReference count = %d, want 2", got)
	}
	_, saved := reopen(t, doc)
	for _, name := range []string{"word/header1.xml", "word/header2.xml"} {
		if got := zipEntryCount(t, saved, name); got != 1 {
			t.Errorf("%s appears %d times, want 1", name, got)
		}
	}
}

// Corpus class F2 (docx side): w:cnfStyle was bound to CT_String, so its
// twelve explicit conditional-formatting booleans were dropped from trPr,
// tcPr, and pPr on save; w:shd lacked themeFillTint/themeFillShade. Word
// writes explicit zeros, so every attribute must survive verbatim.
func TestCnfStyleAndShdThemeFillRoundTrip(t *testing.T) {
	const cnf = `<w:cnfStyle w:val="000010000000" w:firstRow="0" w:lastRow="0" w:firstColumn="0" w:lastColumn="0" w:oddVBand="1" w:evenVBand="0" w:oddHBand="0" w:evenHBand="0" w:firstRowFirstColumn="0" w:firstRowLastColumn="0" w:lastRowFirstColumn="0" w:lastRowLastColumn="0"/>`
	const shd = `<w:shd w:val="clear" w:color="auto" w:fill="EDEDED" w:themeFill="accent3" w:themeFillTint="33" w:themeFillShade="99"/>`
	body := `<w:body><w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/></w:tblPr>` +
		`<w:tblGrid><w:gridCol w:w="4675"/></w:tblGrid>` +
		`<w:tr><w:trPr>` + cnf + `</w:trPr>` +
		`<w:tc><w:tcPr>` + cnf + shd + `</w:tcPr>` +
		`<w:p><w:pPr>` + cnf + `</w:pPr><w:r><w:t>cell</w:t></w:r></w:p>` +
		`</w:tc></w:tr></w:tbl><w:p/><w:sectPr/></w:body>`

	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openSave(t, fixture)

	if got := strings.Count(doc, cnf); got != 3 {
		t.Errorf("cnfStyle preserved %d/3 times (trPr, tcPr, pPr):\n%s", got, doc)
	}
	if !strings.Contains(doc, shd) {
		t.Errorf("shd themeFillTint/themeFillShade dropped:\n%s", doc)
	}
}
