package docx

import (
	"bytes"
	"strings"
	"testing"
)

// anchoredImageFixture builds a docx whose body carries a floating (wp:anchor)
// image with everything the InlineImage model cannot express: an explicit
// docPr id, page-relative position offsets, a square wrap, a stacking order,
// and a rotated spPr. Editing it must leave all of that alone (C372).
func anchoredImageFixture(t *testing.T) []byte {
	t.Helper()
	const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`
	const drawingNS = `xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	const documentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `><w:body>` +
		`<w:p><w:r><w:drawing>` +
		`<wp:anchor ` + drawingNS + ` distT="0" distB="0" distL="114300" distR="114300" simplePos="0" relativeHeight="251659264" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">` +
		`<wp:simplePos x="0" y="0"/>` +
		`<wp:positionH relativeFrom="page"><wp:posOffset>999999</wp:posOffset></wp:positionH>` +
		`<wp:positionV relativeFrom="page"><wp:posOffset>888888</wp:posOffset></wp:positionV>` +
		`<wp:extent cx="914400" cy="914400"/>` +
		`<wp:effectExtent l="0" t="0" r="0" b="0"/>` +
		`<wp:wrapSquare wrapText="bothSides"/>` +
		`<wp:docPr name="Logo" id="42" descr="old alt"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:pic><pic:nvPicPr><pic:cNvPr id="42" name="Logo"/><pic:cNvPicPr/></pic:nvPicPr>` +
		`<pic:blipFill><a:blip r:embed="rId2"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
		`<pic:spPr><a:xfrm rot="600000"><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>` +
		`</pic:pic></a:graphicData></a:graphic>` +
		`</wp:anchor>` +
		`</w:drawing></w:r></w:p>` +
		`</w:body></w:document>`
	const documentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`

	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  fixtureRootRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": documentRels,
		"word/media/image1.png":        string(minimalPNG()),
	})
}

// TestParsedAnchoredImageSetSizePreservesDrawing is the C372 regression: an
// edit through a live image handle must patch the drawing, not rebuild it.
// Before the fix SetSize replaced the whole wp:anchor with the canonical
// inline/anchor template, so the saved drawing carried <wp:docPr id="0">
// (ECMA-invalid, and duplicated across every edited image), position (0,0)
// column/paragraph-relative, wrapNone instead of wrapSquare, no rotation, and a
// reset relativeHeight.
func TestParsedAnchoredImageSetSizePreservesDrawing(t *testing.T) {
	fixture := anchoredImageFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	imgs := doc.Images()
	if len(imgs) != 1 {
		t.Fatalf("want 1 image, got %d", len(imgs))
	}
	if !imgs[0].Floating() {
		t.Error("image should be read back as floating")
	}
	if got := imgs[0].AltText(); got != "old alt" {
		t.Errorf("pre-edit alt text = %q, want %q", got, "old alt")
	}
	if got := imgs[0].WidthEMU(); got != 914400 {
		t.Errorf("pre-edit width = %d EMU, want 914400", got)
	}

	imgs[0].SetSize(100, 100)

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := zipEntryString(t, saved, "word/document.xml")

	// Everything the InlineImage model cannot express must be untouched.
	for _, want := range []string{
		`id="42"`,
		`<wp:posOffset>999999</wp:posOffset>`,
		`<wp:posOffset>888888</wp:posOffset>`,
		`relativeFrom="page"`,
		`<wp:wrapSquare wrapText="bothSides"/>`,
		`relativeHeight="251659264"`,
		`rot="600000"`,
		`descr="old alt"`,
		`<pic:cNvPr id="42" name="Logo"/>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("SetSize dropped %s from the drawing:\n%s", want, got)
		}
	}
	// ...and the two things it does express must have changed, in both the
	// wrapper extent and the picture's own xfrm.
	if strings.Contains(got, `id="0"`) {
		t.Errorf(`SetSize emitted the ECMA-invalid docPr id="0":`+"\n%s", got)
	}
	if strings.Contains(got, "wrapNone") {
		t.Errorf("SetSize replaced wrapSquare with wrapNone:\n%s", got)
	}
	if n := strings.Count(got, `cx="1270000"`); n != 2 {
		t.Errorf("want the resized cx in both wp:extent and a:ext, got %d occurrences:\n%s", n, got)
	}
	if strings.Contains(got, `cx="914400"`) {
		t.Errorf("SetSize left an original extent behind:\n%s", got)
	}

	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	reimgs := re.Images()
	if len(reimgs) != 1 {
		t.Fatalf("reopen: want 1 image, got %d", len(reimgs))
	}
	if w := reimgs[0].WidthEMU(); w != 1270000 {
		t.Errorf("reopened width = %d EMU, want 1270000", w)
	}
	if !reimgs[0].Floating() {
		t.Error("reopened image should still be floating")
	}
}

// TestParsedImageSetAltTextPatchesOnlyDescr checks the alt-text setter patches
// the docPr descr attribute in place, adding it when the source had none and
// leaving the rest of the tag (including its attribute order) verbatim.
func TestParsedImageSetAltTextPatchesOnlyDescr(t *testing.T) {
	fixture := anchoredImageFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	doc.Images()[0].SetAltText(`quote " and <angle>`)
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := zipEntryString(t, saved, "word/document.xml")
	if !strings.Contains(got, `<wp:docPr name="Logo" id="42" descr="quote &quot; and &lt;angle&gt;"/>`) {
		t.Errorf("descr not patched in place:\n%s", got)
	}
	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if alt := re.Images()[0].AltText(); alt != `quote " and <angle>` {
		t.Errorf("reopened alt = %q", alt)
	}
}

// TestParsedImageAltTextInsertedWhenAbsent covers a docPr with no descr at all.
func TestParsedImageAltTextInsertedWhenAbsent(t *testing.T) {
	fixture := hdrFtrHandleFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Clear it first (descr="" ), then set it again: both directions must work.
	doc.Images()[0].SetAltText("")
	doc.Images()[0].SetAltText("set later")
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := zipEntryString(t, saved, "word/header1.xml")
	if !strings.Contains(got, `descr="set later"`) {
		t.Errorf("descr not set:\n%s", got)
	}
	if strings.Count(got, "descr=") != 1 {
		t.Errorf("descr duplicated:\n%s", got)
	}
}

// TestDocPrIDParsingIsAttributeOrderIndependent is the C491 regression: a
// producer that writes name before id (Word does exactly this) previously
// defeated the docPr-id seed, so a later AddImage handed out an id the document
// already used.
func TestDocPrIDParsingIsAttributeOrderIndependent(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"id first", `<wp:docPr id="5" name="Picture 5"/>`, 5},
		{"name first", `<wp:docPr name="Picture 5" id="5"/>`, 5},
		{"single quotes", `<wp:docPr name='x' id='7'/>`, 7},
		{"extra spaces", `<wp:docPr  name = "x"   id = "9" />`, 9},
		{"other prefix", `<w14:docPr name="x" id="11"/>`, 11},
		{"no id", `<wp:docPr name="x"/>`, 0},
		{"gt inside a value", `<wp:docPr name="a &gt; b" id="13"/>`, 13},
		{"commented out", `<!--<wp:docPr id="99"/>--><wp:docPr id="3"/>`, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := docPrID([]byte(tc.raw)); got != tc.want {
				t.Errorf("docPrID(%s) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}

// TestAddImageSeedsPastNameFirstDocPrID drives C491 through the public API: the
// fixture's only drawing writes name before id, so the seed must still see 42.
func TestAddImageSeedsPastNameFirstDocPrID(t *testing.T) {
	fixture := anchoredImageFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := doc.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), "image/png"); err != nil {
		t.Fatalf("add image: %v", err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := zipEntryString(t, saved, "word/document.xml")
	if !strings.Contains(got, `<wp:docPr id="43"`) {
		t.Errorf("new image should have been seeded past id 42:\n%s", got)
	}
	if strings.Count(got, `id="42"`) != 2 { // wp:docPr and pic:cNvPr of the original
		t.Errorf("original id 42 was reused or lost:\n%s", got)
	}
}
