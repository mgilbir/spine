package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// --- C409: shape ids seeded from the opened document ---

// TestShapeIDSeededAcrossReopen is the C409 regression: nextShapeID restarted at
// shapeIDBase+1 every session, so Create → AddTextBox → save → reopen →
// AddTextBox → save produced two <wp:docPr id="100001"> in one document. All the
// shape-authoring APIs share the counter.
func TestShapeIDSeededAcrossReopen(t *testing.T) {
	doc := Create()
	doc.AddTextBox("first", TextBoxOptions{})
	doc, saved := reopen(t, doc)
	if got := strings.Count(zipEntryString(t, saved, "word/document.xml"), `<wp:docPr id="100001"`); got != 1 {
		t.Fatalf("first box should carry id 100001, got %d occurrences", got)
	}

	doc.AddTextBox("second", TextBoxOptions{})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := zipEntryString(t, saved, "word/document.xml")
	if n := strings.Count(got, `<wp:docPr id="100001"`); n != 1 {
		t.Errorf(`<wp:docPr id="100001"> appears %d times, want 1:`+"\n%s", n, got)
	}
	if !strings.Contains(got, `<wp:docPr id="100002"`) {
		t.Errorf("second box should have been seeded past the first:\n%s", got)
	}
}

// TestShapeIDSeedSeesVMLFallbackBoxes covers a shape written with a VML
// fallback: its drawing lives inside an mc:AlternateContent, which the seeding
// scan must look inside.
func TestShapeIDSeedSeesVMLFallbackBoxes(t *testing.T) {
	doc := Create()
	doc.AddTextBox("first", TextBoxOptions{})
	doc, _ = reopen(t, doc)

	doc.AddTextBox("second", TextBoxOptions{})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := zipEntryString(t, saved, "word/document.xml")
	if n := strings.Count(got, `<wp:docPr id="100001"`); n != 1 {
		t.Errorf(`<wp:docPr id="100001"> appears %d times, want 1:`+"\n%s", n, got)
	}
}

// TestShapeAndImageIDSpacesStayDisjoint checks the one scan that seeds both
// counters keeps them apart: an image must not be pushed into the shape range
// and a shape must not be pulled down into the image range.
func TestShapeAndImageIDSpacesStayDisjoint(t *testing.T) {
	doc := Create()
	doc.AddTextBox("box", TextBoxOptions{})
	if _, err := doc.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("add image: %v", err)
	}
	doc, _ = reopen(t, doc)

	doc.AddTextBox("box2", TextBoxOptions{})
	if _, err := doc.AddParagraph().AddRun().AddImageFromBytes(append(minimalPNG(), 1), opc.ContentTypePNG); err != nil {
		t.Fatalf("add image: %v", err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := zipEntryString(t, saved, "word/document.xml")
	for _, want := range []string{`<wp:docPr id="100001"`, `<wp:docPr id="100002"`, `<wp:docPr id="1"`, `<wp:docPr id="2"`} {
		if n := strings.Count(got, want); n != 1 {
			t.Errorf("%s appears %d times, want 1:\n%s", want, n, got)
		}
	}
}

// --- C408: bookmark ids and enumeration across headers/footers ---

// TestHeaderBookmarkIDsDoNotCollide is the C408 regression: nextBookmarkID
// scanned body paragraphs only, so two AddBookmark calls on paragraphs of two
// session-added headers each got bodyMax+1 — the same id, aliasing the ranges.
func TestHeaderBookmarkIDsDoNotCollide(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	h1 := doc.AddHeader(HeaderDefault)
	h2 := doc.AddHeader(HeaderFirst)
	b1 := h1.AddParagraphWithText("in header one").AddBookmark("one")
	b2 := h2.AddParagraphWithText("in header two").AddBookmark("two")

	if b1.id == b2.id {
		t.Fatalf("both header bookmarks got id %q", b1.id)
	}
	// A body bookmark added afterwards must clear both.
	b3 := doc.AddParagraphWithText("more body").AddBookmark("three")
	if b3.id == b1.id || b3.id == b2.id {
		t.Errorf("body bookmark id %q collides with a header bookmark", b3.id)
	}
}

// TestBookmarksIncludeHeadersAndFooters checks the enumeration and the text
// resolution the godoc promises for "every bookmark in the document".
func TestBookmarksIncludeHeadersAndFooters(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("BODYTEXT").AddBookmark("body")
	doc.AddHeader(HeaderDefault).AddParagraphWithText("HEADERTEXT").AddBookmark("hdr")
	doc.AddFooter(FooterDefault).AddParagraphWithText("FOOTERTEXT").AddBookmark("ftr")

	doc, _ = reopen(t, doc)
	names := map[string]string{}
	for _, b := range doc.Bookmarks() {
		names[b.Name()] = b.Text()
	}
	for name, wantText := range map[string]string{
		"body": "BODYTEXT",
		"hdr":  "HEADERTEXT",
		"ftr":  "FOOTERTEXT",
	} {
		got, ok := names[name]
		if !ok {
			t.Errorf("bookmark %q missing from Bookmarks()", name)
			continue
		}
		if got != wantText {
			t.Errorf("bookmark %q text = %q, want %q", name, got, wantText)
		}
	}
}

// --- C402: core-property edits on a package with no core part ---

// noCorePropsFixture builds a package whose root .rels declares no
// core-properties relationship at all.
func noCorePropsFixture(t *testing.T) []byte {
	t.Helper()
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureContentTypes,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>hi</w:t></w:r></w:p></w:body></w:document>`,
	})
}

// TestPropertiesEditOnPackageWithoutCorePart is the C402 regression: with no
// core-properties part at open the guard short-circuited on hasCoreProps and
// every edit to the public Properties field was silently dropped, while
// Create+Save writes the same field unconditionally.
func TestPropertiesEditOnPackageWithoutCorePart(t *testing.T) {
	fixture := noCorePropsFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	doc.Properties.Title = "SET BY THE CALLER"
	doc.Properties.Creator = "spine"
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	core, ok := zipEntry(t, saved, "docProps/core.xml")
	if !ok {
		t.Fatal("docProps/core.xml was not created")
	}
	if !strings.Contains(string(core), "SET BY THE CALLER") {
		t.Errorf("core.xml does not carry the edit:\n%s", core)
	}
	rootRels := zipEntryString(t, saved, "_rels/.rels")
	if !strings.Contains(rootRels, "docProps/core.xml") {
		t.Errorf("the core-properties package relationship was not injected:\n%s", rootRels)
	}
	ct := zipEntryString(t, saved, "[Content_Types].xml")
	if !strings.Contains(ct, "/docProps/core.xml") {
		t.Errorf("core.xml has no content-type override:\n%s", ct)
	}

	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if re.Properties.Title != "SET BY THE CALLER" {
		t.Errorf("reopened Title = %q, want %q", re.Properties.Title, "SET BY THE CALLER")
	}
	if re.Properties.Creator != "spine" {
		t.Errorf("reopened Creator = %q", re.Properties.Creator)
	}
}

// TestNoCorePartAndNoEditStaysByteIdentical guards the other direction: the
// zero-baseline snapshot must not read an untouched save as an edit.
func TestNoCorePartAndNoEditStaysByteIdentical(t *testing.T) {
	fixture := noCorePropsFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := zipEntry(t, saved, "docProps/core.xml"); ok {
		t.Error("an untouched save invented a core-properties part")
	}
	if got := zipEntryString(t, saved, "_rels/.rels"); got != fixtureRootRels {
		t.Errorf("root .rels changed on an untouched save:\n%s", got)
	}
}

// --- C410: an undecodable main-part .rels must pass through ---

// TestMalformedMainRelsPassesThrough is the C410 regression: an unparsable
// word/_rels/document.xml.rels left relationships[mainPart] unset, the
// preserved-parts loop skipped that exact name, and writeDocumentRelationships
// returned early — so the part was simply not written and every image,
// hyperlink, header and styles reference in the document was severed.
func TestMalformedMainRelsPassesThrough(t *testing.T) {
	// A .rels Go's XML decoder rejects: an HTML entity reference in a target,
	// which real producers do emit and which is not a defined XML entity.
	const badRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="http://example.com/a&nbsp;b" TargetMode="External"/></Relationships>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureContentTypes,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>hi</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": badRels,
		"word/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:styles ` + fixtureWNS + `></w:styles>`,
	})
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !doc.unparsedRels["/word/document.xml"] {
		t.Fatal("the fixture .rels parsed cleanly, so the swallow path this test guards is not exercised")
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok := zipEntry(t, saved, "word/_rels/document.xml.rels")
	if !ok {
		t.Fatal("word/_rels/document.xml.rels was dropped from the saved package")
	}
	if string(got) != badRels {
		t.Errorf("main .rels was not passed through verbatim:\ngot:  %s\nwant: %s", got, badRels)
	}
}

// --- C488: AddSectionBreak carries the section setup across ---

// TestAddSectionBreakClonesPreviousSection is the C488 regression: the new final
// section was left with an empty sectPr, so everything after the break reverted
// to Word's defaults — no headers, Letter portrait, default margins.
func TestAddSectionBreakClonesPreviousSection(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("before the break")
	sec := doc.DefaultSection()
	sec.SetPageSize(PageSizeA4())
	sec.SetOrientation(OrientationLandscape)
	sec.SetMargins(PageMargins{Top: 20, Bottom: 20, Left: 30, Right: 30, Header: 10, Footer: 10})
	doc.AddHeader(HeaderDefault).AddParagraphWithText("PAGE FURNITURE")

	newSec := doc.AddSectionBreak()

	wantW, wantH := sec.PageSize()
	if gotW, gotH := newSec.PageSize(); gotW != wantW || gotH != wantH {
		t.Errorf("new section page size = %vx%v, want %vx%v", gotW, gotH, wantW, wantH)
	}
	if newSec.Orientation() != OrientationLandscape {
		t.Error("new section lost the landscape orientation")
	}
	gotM, ok := newSec.MarginsOK()
	if !ok {
		t.Fatal("new section declares no margins")
	}
	if wantM, _ := sec.MarginsOK(); gotM != wantM {
		t.Errorf("new section margins = %+v, want %+v", gotM, wantM)
	}

	doc, saved := reopen(t, doc)
	docXML := zipEntryString(t, saved, "word/document.xml")
	if n := strings.Count(docXML, "<w:headerReference"); n != 2 {
		t.Errorf("headerReference count = %d, want 2 (one per section):\n%s", n, docXML)
	}
	if n := strings.Count(docXML, `<w:pgSz`); n != 2 {
		t.Errorf("pgSz count = %d, want 2:\n%s", n, docXML)
	}
	// The two sections must be independent: adjusting one must not move the
	// other, which a shallow copy sharing child pointers would.
	final := doc.DefaultSection()
	final.SetPageSize(PageSizeLegal())
	saved2, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	after := zipEntryString(t, saved2, "word/document.xml")
	if n := strings.Count(after, `w:w="12240" w:h="20160"`); n != 1 {
		t.Errorf("the resized final section should be the only Legal one:\n%s", after)
	}
	if n := strings.Count(after, `w:w="16838" w:h="11906"`); n != 1 {
		t.Errorf("resizing the final section changed the section before the break too:\n%s", after)
	}
}

// --- C493: SetMargins writes all six values ---

func TestSetMarginsExpressesZeroHeaderDistance(t *testing.T) {
	doc := Create()
	sec := doc.DefaultSection()
	sec.SetMargins(PageMargins{Top: 72, Bottom: 72, Left: 72, Right: 72, Header: 36, Footer: 36})
	if m, _ := sec.MarginsOK(); m.Header != 36 {
		t.Fatalf("header distance = %v, want 36", m.Header)
	}
	sec.SetMargins(PageMargins{Top: 72, Bottom: 72, Left: 72, Right: 72})
	m, ok := sec.MarginsOK()
	if !ok {
		t.Fatal("margins not declared")
	}
	if m.Header != 0 || m.Footer != 0 {
		t.Errorf("header/footer distance = %v/%v, want 0/0 — a zero must be expressible", m.Header, m.Footer)
	}
}

func TestMarginsOKDistinguishesUnsetFromZero(t *testing.T) {
	doc := Create()
	sec := doc.DefaultSection()
	if _, ok := sec.MarginsOK(); ok {
		t.Error("a section with no w:pgMar must report ok=false")
	}
	sec.SetMargins(PageMargins{})
	if _, ok := sec.MarginsOK(); !ok {
		t.Error("a section with an all-zero w:pgMar must report ok=true")
	}
}

// --- C505 / expectation gap: header and footer accessors ---

// TestHeadersAccessorEditsPreservedHeader covers the missing-accessor gap: a
// header parsed from an opened package had no editable handle at all, so the
// header-side mutators were unreachable.
func TestHeadersAccessorEditsPreservedHeader(t *testing.T) {
	fixture := hdrFtrHandleFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	hdrs := doc.Headers()
	if len(hdrs) != 1 {
		t.Fatalf("Headers() returned %d handles, want 1", len(hdrs))
	}
	if got := hdrs[0].PartName(); got != "/word/header1.xml" {
		t.Errorf("PartName = %q", got)
	}
	if n := len(hdrs[0].Paragraphs()); n != 2 {
		t.Errorf("header has %d paragraphs, want 2", n)
	}
	sec, ok := doc.Header(doc.DefaultSection(), HeaderDefault)
	if !ok {
		t.Fatal("Header(section, HeaderDefault) did not resolve")
	}
	if sec.PartName() != hdrs[0].PartName() {
		t.Errorf("section header = %q, document header = %q", sec.PartName(), hdrs[0].PartName())
	}

	sec.AddParagraphWithText("ADDED THROUGH THE ACCESSOR")
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := zipEntryString(t, saved, "word/header1.xml"); !strings.Contains(got, "ADDED THROUGH THE ACCESSOR") {
		t.Errorf("the edit was masked by the preserved header bytes:\n%s", got)
	}
}

// --- C490: TextBoxes and validation descend header tables ---

// TestTextBoxesFindsBoxInHeaderTable is the C490 regression: the header/footer
// side of TextBoxes walked only top-level paragraphs, so a box inside a header
// table — a very common layout — was invisible despite the godoc promising
// "including boxes nested in tables, headers, and footers".
func TestTextBoxesFindsBoxInHeaderTable(t *testing.T) {
	doc := Create()
	hdr := doc.AddHeader(HeaderDefault)
	// Author the box in a header paragraph, then move that paragraph into a
	// single-cell table in the same header so the box is table-nested.
	p := hdr.AddParagraph()
	p.AddTextBox("BOXED IN A HEADER TABLE", TextBoxOptions{})
	doc, _ = reopen(t, doc)

	if len(doc.TextBoxes()) != 1 {
		t.Fatalf("baseline: want 1 text box, got %d", len(doc.TextBoxes()))
	}

	// Now the nested case, built directly as a fixture.
	const drawing = `<w:tbl><w:tr><w:tc><w:p><w:r><w:drawing>` +
		`<wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<wp:extent cx="914400" cy="914400"/><wp:docPr id="7" name="Text Box 7"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<wps:wsp><wps:cNvPr id="7" name="Text Box 7"/><wps:spPr/>` +
		`<wps:txbx><w:txbxContent><w:p><w:r><w:t>NESTED BOX</w:t></w:r></w:p></w:txbxContent></wps:txbx>` +
		`<wps:bodyPr/></wps:wsp></a:graphicData></a:graphic>` +
		`</wp:inline></w:drawing></w:r></w:p></w:tc></w:tr></w:tbl>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`,
		"_rels/.rels": fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/><w:sectPr><w:headerReference w:type="default" r:id="rId10"/></w:sectPr></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/></Relationships>`,
		"word/header1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:hdr ` + fixtureWNS + `>` + drawing + `</w:hdr>`,
	})
	nested, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	boxes := nested.TextBoxes()
	if len(boxes) != 1 {
		t.Fatalf("want 1 text box nested in a header table, got %d", len(boxes))
	}
	if got := boxes[0].Text(); got != "NESTED BOX" {
		t.Errorf("box text = %q", got)
	}
}
