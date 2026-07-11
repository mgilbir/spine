package docx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// reopen saves the document and opens the result, failing the test on error.
func reopen(t *testing.T, doc *Document) (*Document, []byte) {
	t.Helper()
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	d, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return d, saved
}

// zipEntryCount returns how many times a zip entry name appears in the package.
func zipEntryCount(t *testing.T, data []byte, name string) int {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, f := range zr.File {
		if f.Name == name {
			count++
		}
	}
	return count
}

// C155: AddImage on an opened document that already contains
// /word/media/image1.png must pick the next free name instead of colliding —
// previously the numbering restarted at image1 and Save failed with
// "opc: duplicate part". Exercised over multiple save→reopen→add cycles.
func TestOpenedDocument_AddImageAvoidsExistingPartNames(t *testing.T) {
	doc := Create()
	if _, err := doc.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatal(err)
	}

	for cycle, want := range []string{"word/media/image1.png", "word/media/image2.png", "word/media/image3.png"} {
		var saved []byte
		doc, saved = reopen(t, doc)
		for i, name := range []string{"word/media/image1.png", "word/media/image2.png", "word/media/image3.png"} {
			wantCount := 0
			if i <= cycle {
				wantCount = 1
			}
			if got := zipEntryCount(t, saved, name); got != wantCount {
				t.Errorf("cycle %d: %s appears %d times, want %d", cycle, name, got, wantCount)
			}
		}
		if _, ok := zipEntry(t, saved, want); !ok {
			t.Errorf("cycle %d: expected %s in package", cycle, want)
		}
		// Add the next image on the reopened document; before the fix this
		// collided with the existing image1.png and Save failed.
		if _, err := doc.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
			t.Fatal(err)
		}
	}

	_, saved := reopen(t, doc)
	rels, ok := zipEntry(t, saved, "word/_rels/document.xml.rels")
	if !ok {
		t.Fatal("document.xml.rels missing")
	}
	for _, target := range []string{"media/image1.png", "media/image2.png", "media/image3.png", "media/image4.png"} {
		if !strings.Contains(string(rels), `Target="`+target+`"`) {
			t.Errorf("relationship for %s missing", target)
		}
	}
}

// C155: image numbering shares one number space across extensions (Word
// convention: image1.png and image1.jpeg collide) and fills gaps without
// colliding with higher-numbered existing parts.
func TestNextImageNumber_SharedNumberSpaceAndGaps(t *testing.T) {
	doc := Create()
	// Simulate a package that already contains image3.png (gap at 1 and 2)
	// and image2.jpeg (different extension, same number space).
	doc.otherParts["/word/media/image3.png"] = &coxml.RawPart{ContentType: opc.ContentTypePNG}
	doc.otherParts["/word/media/image2.jpeg"] = &coxml.RawPart{ContentType: opc.ContentTypeJPEG}

	r := doc.AddParagraph().AddRun()
	for i, want := range []string{"/word/media/image1.png", "/word/media/image4.png", "/word/media/image5.png"} {
		// Distinct bytes per image: identical bytes would be deduplicated into
		// a single media part instead of exercising the numbering.
		data := append(minimalPNG(), byte(i))
		if _, err := r.AddImageFromBytes(data, opc.ContentTypePNG); err != nil {
			t.Fatal(err)
		}
		got := doc.imageParts[len(doc.imageParts)-1].partName
		if got != want {
			t.Errorf("image part name = %s, want %s", got, want)
		}
	}
}

// C155: AddHeader/AddFooter on an opened document that already contains
// header1.xml/footer1.xml must pick the next free name; the save must succeed
// and both the old and the new parts must be present. Exercised over multiple
// save→reopen→add cycles.
func TestOpenedDocument_AddHeaderFooterAvoidsExistingPartNames(t *testing.T) {
	doc := Create()
	doc.AddHeader(HeaderDefault).AddParagraphWithText("H1")
	doc.AddFooter(FooterDefault).AddParagraphWithText("F1")

	doc, saved := reopen(t, doc)
	for _, name := range []string{"word/header1.xml", "word/footer1.xml"} {
		if got := zipEntryCount(t, saved, name); got != 1 {
			t.Fatalf("%s appears %d times, want 1", name, got)
		}
	}

	// Before the fix this reused header1.xml/footer1.xml and Save failed with
	// "opc: duplicate part".
	doc.AddHeader(HeaderFirst).AddParagraphWithText("H2")
	doc.AddFooter(FooterFirst).AddParagraphWithText("F2")

	doc, saved = reopen(t, doc)
	for _, name := range []string{"word/header1.xml", "word/header2.xml", "word/footer1.xml", "word/footer2.xml"} {
		if got := zipEntryCount(t, saved, name); got != 1 {
			t.Errorf("%s appears %d times, want 1", name, got)
		}
	}
	if h1, ok := zipEntry(t, saved, "word/header1.xml"); !ok || !strings.Contains(string(h1), "H1") {
		t.Error("original header1.xml content lost")
	}
	if h2, ok := zipEntry(t, saved, "word/header2.xml"); !ok || !strings.Contains(string(h2), "H2") {
		t.Error("new header2.xml content missing")
	}

	// Third cycle: one more header on top of the two parsed ones.
	doc.AddHeader(HeaderEven).AddParagraphWithText("H3")
	_, saved = reopen(t, doc)
	for _, name := range []string{"word/header1.xml", "word/header2.xml", "word/header3.xml"} {
		if got := zipEntryCount(t, saved, name); got != 1 {
			t.Errorf("%s appears %d times, want 1", name, got)
		}
	}
}

// C155: header naming fills gaps and never collides with a higher-numbered
// existing part.
func TestAddHeader_NamingWithGaps(t *testing.T) {
	doc := Create()
	// Simulate a package that already contains header2.xml only.
	doc.headers["/word/header2.xml"] = &headerPart{contentType: opc.ContentTypeDocHeader}

	// Distinct types: a repeated same-type AddHeader replaces the previous
	// header (C226) instead of adding a second part.
	doc.AddHeader(HeaderDefault)
	doc.AddHeader(HeaderFirst)
	got := make([]string, 0, len(doc.newHeaderParts))
	for _, hp := range doc.newHeaderParts {
		got = append(got, hp.partName)
	}
	want := []string{"/word/header1.xml", "/word/header3.xml"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("new header part names = %v, want %v", got, want)
	}
}

// C161: AddHeader/AddFooter must never clobber a parsed header/footer in the
// in-memory maps — the derived part name is always a fresh key.
func TestAddHeaderFooter_DoesNotClobberParsedParts(t *testing.T) {
	doc := Create()
	doc.AddHeader(HeaderDefault).AddParagraphWithText("ORIGINAL-HEADER")
	doc.AddFooter(FooterDefault).AddParagraphWithText("ORIGINAL-FOOTER")

	doc, _ = reopen(t, doc)
	parsedHdr, ok := doc.headers["/word/header1.xml"]
	if !ok {
		t.Fatal("parsed header1.xml not in headers map")
	}
	parsedFtr, ok := doc.footers["/word/footer1.xml"]
	if !ok {
		t.Fatal("parsed footer1.xml not in footers map")
	}

	doc.AddHeader(HeaderDefault)
	doc.AddFooter(FooterDefault)

	if got := doc.headers["/word/header1.xml"]; got != parsedHdr {
		t.Error("AddHeader clobbered the parsed header1.xml entry")
	}
	if got := doc.footers["/word/footer1.xml"]; got != parsedFtr {
		t.Error("AddFooter clobbered the parsed footer1.xml entry")
	}
	if len(doc.headers) != 2 {
		t.Errorf("headers map has %d entries, want 2", len(doc.headers))
	}
	if len(doc.footers) != 2 {
		t.Errorf("footers map has %d entries, want 2", len(doc.footers))
	}
}
