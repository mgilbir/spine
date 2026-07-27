package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// TestSetTextReclaimsHyperlinkRel is the C407 regression: SetText removes the
// w:hyperlink element but the External relationship it referenced stayed in
// document.xml.rels forever, and there is no RemoveHyperlink to reclaim it — so
// a template-fill workflow accreted one dead relationship per link.
func TestSetTextReclaimsHyperlinkRel(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddHyperlink("click me", "https://example.com/")
	p.AddHyperlink("and me", "https://example.org/")
	if n := len(doc.relationships[doc.mainPart()]); n != 2 {
		t.Fatalf("baseline relationship count = %d, want 2", n)
	}

	p.SetText("plain replacement")

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	rels := zipEntryString(t, saved, "word/_rels/document.xml.rels")
	for _, gone := range []string{"example.com", "example.org"} {
		if strings.Contains(rels, gone) {
			t.Errorf("relationship to %s survived the paragraph that referenced it:\n%s", gone, rels)
		}
	}
	docXML := zipEntryString(t, saved, "word/document.xml")
	if strings.Contains(docXML, "<w:hyperlink") {
		t.Errorf("hyperlink element survived SetText:\n%s", docXML)
	}
	if !strings.Contains(docXML, "plain replacement") {
		t.Errorf("replacement text missing:\n%s", docXML)
	}
}

// TestSetTextKeepsRelsStillReferencedElsewhere is the guard on the sweep: a
// relationship another paragraph still uses must not be collected.
func TestSetTextKeepsRelsStillReferencedElsewhere(t *testing.T) {
	doc := Create()
	keep := doc.AddParagraph()
	keep.AddHyperlink("keep me", "https://keep.example/")
	drop := doc.AddParagraph()
	drop.AddHyperlink("drop me", "https://drop.example/")

	drop.SetText("gone")

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	rels := zipEntryString(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, "keep.example") {
		t.Errorf("a still-referenced relationship was swept:\n%s", rels)
	}
	if strings.Contains(rels, "drop.example") {
		t.Errorf("the unreferenced relationship survived:\n%s", rels)
	}
}

// TestRunClearReclaimsImageRelAndMedia covers the image half of C407: the rel
// and, for a media part added in this session that nothing else points at, the
// part itself.
func TestRunClearReclaimsImageRelAndMedia(t *testing.T) {
	doc := Create()
	r := doc.AddParagraph().AddRun()
	if _, err := r.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("add image: %v", err)
	}
	if len(doc.imageParts) != 1 {
		t.Fatalf("baseline image part count = %d, want 1", len(doc.imageParts))
	}

	r.Clear()

	if len(doc.imageParts) != 0 {
		t.Errorf("media part count = %d after clearing its only reference, want 0", len(doc.imageParts))
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := zipEntry(t, saved, "word/media/image1.png"); ok {
		t.Error("orphaned media part written to the package")
	}
	rels := zipEntryString(t, saved, "word/_rels/document.xml.rels")
	if strings.Contains(rels, "media/image1.png") {
		t.Errorf("image relationship survived the run that referenced it:\n%s", rels)
	}
}

// TestRunClearKeepsSharedMedia guards the dedup case: the same bytes placed
// twice share one media part but hold two relationships, so clearing one
// placement must keep the part.
func TestRunClearKeepsSharedMedia(t *testing.T) {
	doc := Create()
	r1 := doc.AddParagraph().AddRun()
	r2 := doc.AddParagraph().AddRun()
	if _, err := r1.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("add image: %v", err)
	}
	if _, err := r2.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("add image: %v", err)
	}

	r1.Clear()

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := zipEntry(t, saved, "word/media/image1.png"); !ok {
		t.Error("shared media part removed while a second placement still references it")
	}
	rels := zipEntryString(t, saved, "word/_rels/document.xml.rels")
	if n := strings.Count(rels, "media/image1.png"); n != 1 {
		t.Errorf("media relationship count = %d, want 1:\n%s", n, rels)
	}
}

// TestSetTextOnHeaderParagraphSweepsHeaderRels checks the sweep is part-scoped:
// a header hyperlink's relationship lives in the header's own .rels.
func TestSetTextOnHeaderParagraphSweepsHeaderRels(t *testing.T) {
	fixture := hdrFtrHandleFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	links := doc.Hyperlinks()
	if len(links) != 1 {
		t.Fatalf("want 1 header hyperlink, got %d", len(links))
	}
	links[0].para.SetText("replaced")

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	rels := zipEntryString(t, saved, "word/_rels/header1.xml.rels")
	if strings.Contains(rels, "example.com") {
		t.Errorf("header hyperlink relationship survived:\n%s", rels)
	}
	// The image relationship in the same part is untouched: a different
	// paragraph still references it.
	if !strings.Contains(rels, "media/image1.png") {
		t.Errorf("the header image relationship was swept too:\n%s", rels)
	}
}

// TestSetTextDoesNotSweepSectionFurniture guards against the sweep reaching
// relationships that are not body references at all.
func TestSetTextDoesNotSweepSectionFurniture(t *testing.T) {
	doc := Create()
	doc.AddHeader(HeaderDefault).AddParagraphWithText("HEADER")
	p := doc.AddParagraph()
	p.AddHyperlink("link", "https://example.com/")

	p.SetText("plain")

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	rels := zipEntryString(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, "header1.xml") {
		t.Errorf("the header relationship was swept:\n%s", rels)
	}
	if strings.Contains(rels, "example.com") {
		t.Errorf("the hyperlink relationship survived:\n%s", rels)
	}
}

// --- C492: replacing a preserved header releases it ---

// TestAddHeaderOverPreservedHeaderReleasesIt is the C492 regression: AddHeader
// over a header that came from the opened package repointed the reference but
// dropSessionHeader only ever dropped session parts, so the old part and its
// now-unreferenced relationship stayed in the package permanently.
func TestAddHeaderOverPreservedHeaderReleasesIt(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.AddHeader(HeaderDefault).AddParagraphWithText("ORIGINAL")
	doc, _ = reopen(t, doc)

	doc.AddHeader(HeaderDefault).AddParagraphWithText("REPLACEMENT")
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, ok := zipEntry(t, saved, "word/header1.xml"); ok {
		t.Error("the replaced header part is still in the package")
	}
	if _, ok := zipEntry(t, saved, "word/_rels/header1.xml.rels"); ok {
		t.Error("the replaced header's .rels is still in the package")
	}
	hdr2, ok := zipEntry(t, saved, "word/header2.xml")
	if !ok {
		t.Fatal("the replacement header was not written")
	}
	if !strings.Contains(string(hdr2), "REPLACEMENT") {
		t.Errorf("replacement content missing:\n%s", hdr2)
	}
	rels := zipEntryString(t, saved, "word/_rels/document.xml.rels")
	if strings.Contains(rels, "header1.xml") {
		t.Errorf("the orphaned header relationship survived:\n%s", rels)
	}
	if !strings.Contains(rels, "header2.xml") {
		t.Errorf("the replacement header relationship is missing:\n%s", rels)
	}

	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	hdrs := re.Headers()
	if len(hdrs) != 1 {
		t.Fatalf("reopened document has %d headers, want 1", len(hdrs))
	}
}

// TestAddHeaderKeepsHeaderSharedWithAnotherSection guards the release: a header
// a paragraph-level section still references must survive.
func TestAddHeaderKeepsHeaderSharedWithAnotherSection(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("first section")
	doc.AddHeader(HeaderDefault).AddParagraphWithText("SHARED")
	// A section break copies the section — including its header reference — so
	// two sections now point at the same header part.
	doc.AddSectionBreak()
	doc.AddParagraphWithText("second section")
	doc, saved := reopen(t, doc)
	if n := strings.Count(zipEntryString(t, saved, "word/document.xml"), "<w:headerReference"); n != 2 {
		t.Fatalf("baseline headerReference count = %d, want 2", n)
	}

	doc.AddHeader(HeaderDefault).AddParagraphWithText("ONLY FOR THE LAST SECTION")
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := zipEntry(t, saved, "word/header1.xml"); !ok {
		t.Error("a header another section still references was released")
	}
	rels := zipEntryString(t, saved, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, "header1.xml") {
		t.Errorf("the shared header relationship was dropped:\n%s", rels)
	}
}
