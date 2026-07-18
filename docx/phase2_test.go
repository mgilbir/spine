package docx

import (
	"bytes"
	"strings"
	"testing"
)

// tinyPNG is a 1x1 transparent PNG used across the image tests.
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x60, 0x00, 0x02, 0x00,
	0x00, 0x05, 0x00, 0x01, 0xe2, 0x26, 0x05, 0x9b,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44,
	0xae, 0x42, 0x60, 0x82,
}

func reopenBytes(t *testing.T, data []byte) *Document {
	t.Helper()
	d, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return d
}

// --- Hyperlinks ---

// fixtureHyperlinkDocx builds a package whose document.xml contains an external
// and an internal hyperlink, with the external one wired to a rel.
func fixtureHyperlinkDocx(t *testing.T) []byte {
	t.Helper()
	docRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`<Relationship Id="rId100" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com/" TargetMode="External"/>` +
		`</Relationships>`
	body := `<w:body><w:p>` +
		`<w:hyperlink r:id="rId100" w:tooltip="tip"><w:r><w:t>site</w:t></w:r></w:hyperlink>` +
		`<w:hyperlink w:anchor="place"><w:r><w:t>jump</w:t></w:r></w:hyperlink>` +
		`</w:p></w:body>`
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          fixtureContentTypes,
		"_rels/.rels":                  fixtureRootRels,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + docRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `>` + body + `</w:document>`,
	})
}

func TestHyperlinks_ReadFromFixture(t *testing.T) {
	d := reopenBytes(t, fixtureHyperlinkDocx(t))
	hls := d.Hyperlinks()
	if len(hls) != 2 {
		t.Fatalf("Hyperlinks() = %d, want 2", len(hls))
	}
	if got := hls[0].URL(); got != "https://example.com/" {
		t.Errorf("external URL = %q", got)
	}
	if got := hls[0].Anchor(); got != "" {
		t.Errorf("external Anchor = %q, want empty", got)
	}
	if got := hls[0].Tooltip(); got != "tip" {
		t.Errorf("Tooltip = %q", got)
	}
	if got := hls[1].URL(); got != "" {
		t.Errorf("internal URL = %q, want empty", got)
	}
	if got := hls[1].Anchor(); got != "place" {
		t.Errorf("internal Anchor = %q", got)
	}
	// Paragraph.Hyperlinks and Run.Hyperlink reachability.
	p := d.Paragraphs()[0]
	if len(p.Hyperlinks()) != 2 {
		t.Errorf("Paragraph.Hyperlinks() = %d, want 2", len(p.Hyperlinks()))
	}
	back := p.Hyperlinks()[0].Runs()[0].Hyperlink()
	if back == nil || back.URL() != "https://example.com/" {
		t.Errorf("Run.Hyperlink() did not resolve back to the hyperlink")
	}
}

func TestHyperlinks_CreateRoundTrip(t *testing.T) {
	d := Create()
	p := d.AddParagraph()
	p.AddRun().SetText("pre ")
	h := p.AddHyperlink("site", "https://example.com/")
	h.SetTooltip("tip")
	d.AddParagraph().AddInternalHyperlink("jump", "place")

	rd := reopenBytes(t, saveBytes(t, d))
	hls := rd.Hyperlinks()
	if len(hls) != 2 {
		t.Fatalf("Hyperlinks() = %d, want 2", len(hls))
	}
	if hls[0].URL() != "https://example.com/" || hls[0].Tooltip() != "tip" {
		t.Errorf("external hyperlink round-trip: url=%q tip=%q", hls[0].URL(), hls[0].Tooltip())
	}
	if hls[1].Anchor() != "place" {
		t.Errorf("internal anchor round-trip: %q", hls[1].Anchor())
	}
	if got := rd.Paragraphs()[0].Text(); got != "pre site" {
		t.Errorf("Paragraph.Text = %q, want %q", got, "pre site")
	}
}

// TestHyperlink_ChildOrder verifies a hyperlink appended to a paragraph with
// existing runs keeps order and serializes exactly once.
func TestHyperlink_ChildOrder(t *testing.T) {
	d := Create()
	p := d.AddParagraph()
	p.AddRun().SetText("a")
	p.AddHyperlink("link", "https://x/")
	p.AddRun().SetText("b")

	xml := docXML(t, saveBytes(t, d))
	if n := strings.Count(xml, "<w:hyperlink"); n != 1 {
		t.Fatalf("hyperlink serialized %d times, want 1", n)
	}
	// order: run "a", hyperlink, run "b"
	ia := strings.Index(xml, ">a<")
	ih := strings.Index(xml, "<w:hyperlink")
	ib := strings.Index(xml, ">b<")
	if ia >= ih || ih >= ib {
		t.Errorf("child order wrong: a=%d hyperlink=%d b=%d", ia, ih, ib)
	}
}

// --- Bookmarks ---

func TestBookmarks_CreateRoundTripAndCompose(t *testing.T) {
	d := Create()
	p := d.AddParagraph()
	p.AddRun().SetText("Chapter One")
	p.AddBookmark("chap1")

	p2 := d.AddParagraph()
	r1 := p2.AddRun()
	r1.SetText("start ")
	r2 := p2.AddRun()
	r2.SetText("middle")
	p2.AddRun().SetText(" end")
	if got := d.AddBookmarkOnRange("mid", r1, r2); got == nil {
		t.Fatal("AddBookmarkOnRange returned nil")
	}
	// compose with an internal hyperlink
	d.AddParagraph().AddInternalHyperlink("go", "chap1")

	rd := reopenBytes(t, saveBytes(t, d))
	bms := rd.Bookmarks()
	if len(bms) != 2 {
		t.Fatalf("Bookmarks() = %d, want 2", len(bms))
	}
	byName := map[string]string{}
	for _, b := range bms {
		byName[b.Name()] = b.Text()
	}
	if byName["chap1"] != "Chapter One" {
		t.Errorf("chap1 text = %q", byName["chap1"])
	}
	if byName["mid"] != "start middle" {
		t.Errorf("mid range text = %q, want %q", byName["mid"], "start middle")
	}
	// the internal hyperlink anchor matches the bookmark name
	found := false
	for _, h := range rd.Hyperlinks() {
		if h.Anchor() == "chap1" {
			found = true
		}
	}
	if !found {
		t.Error("internal hyperlink does not compose with bookmark chap1")
	}
}

func TestBookmark_ChildOrder(t *testing.T) {
	d := Create()
	p := d.AddParagraph()
	p.AddRun().SetText("x")
	p.AddBookmark("b")
	xml := docXML(t, saveBytes(t, d))
	if n := strings.Count(xml, "<w:bookmarkStart"); n != 1 {
		t.Fatalf("bookmarkStart count = %d, want 1", n)
	}
	if n := strings.Count(xml, "<w:bookmarkEnd"); n != 1 {
		t.Fatalf("bookmarkEnd count = %d, want 1", n)
	}
	if strings.Index(xml, "<w:bookmarkStart") > strings.Index(xml, ">x<") {
		t.Error("bookmarkStart should precede the run")
	}
	if strings.Index(xml, "<w:bookmarkEnd") < strings.Index(xml, ">x<") {
		t.Error("bookmarkEnd should follow the run")
	}
}

// --- Footnotes / Endnotes ---

func TestNotes_CreateRoundTrip(t *testing.T) {
	d := Create()
	r := d.AddParagraphWithText("body").Runs()[0]
	r.AddFootnote("the footnote")
	r2 := d.AddParagraphWithText("more").Runs()[0]
	r2.AddEndnote("the endnote")

	data := saveBytes(t, d)
	// The parts, their overrides, and rels must be present.
	if _, ok := zipEntry(t, data, "word/footnotes.xml"); !ok {
		t.Fatal("footnotes.xml missing")
	}
	if _, ok := zipEntry(t, data, "word/endnotes.xml"); !ok {
		t.Fatal("endnotes.xml missing")
	}
	rd := reopenBytes(t, data)
	fns := rd.Footnotes()
	if len(fns) != 1 {
		t.Fatalf("Footnotes() = %d, want 1 (separators excluded)", len(fns))
	}
	if fns[0].ID() != "1" || fns[0].Text() != " the footnote" {
		t.Errorf("footnote id=%q text=%q", fns[0].ID(), fns[0].Text())
	}
	ens := rd.Endnotes()
	if len(ens) != 1 || ens[0].Text() != " the endnote" {
		t.Errorf("endnotes = %v", ens)
	}
	// The separator notes must have been written (Word requires them).
	fx := zipEntryString(t, data, "word/footnotes.xml")
	if !strings.Contains(fx, `w:type="separator"`) || !strings.Contains(fx, `w:type="continuationSeparator"`) {
		t.Error("standard separator notes missing from footnotes.xml")
	}
}

func TestFootnoteRef_ChildOrder(t *testing.T) {
	d := Create()
	p := d.AddParagraph()
	a := p.AddRun()
	a.SetText("a")
	a.AddFootnote("note")
	p.AddRun().SetText("b")
	xml := docXML(t, saveBytes(t, d))
	if n := strings.Count(xml, "<w:footnoteReference"); n != 1 {
		t.Fatalf("footnoteReference count = %d, want 1", n)
	}
	// reference sits between run "a" and run "b"
	ia := strings.Index(xml, ">a<")
	ir := strings.Index(xml, "<w:footnoteReference")
	ib := strings.Index(xml, ">b<")
	if ia >= ir || ir >= ib {
		t.Errorf("footnote ref order wrong: a=%d ref=%d b=%d", ia, ir, ib)
	}
}

// TestNotes_ByteIdentityUnmodified verifies an opened footnotes part is
// preserved byte-for-byte on a zero-modification save.
func TestNotes_ByteIdentityUnmodified(t *testing.T) {
	footnotes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:footnotes ` + fixtureWNS + `>` +
		`<w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>` +
		`<w:footnote w:id="1"><w:p><w:r><w:t>existing note</w:t></w:r></w:p></w:footnote>` +
		`</w:footnotes>`
	ct := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/word/footnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"/>` +
		`</Types>`
	docRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>` +
		`</Relationships>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          ct,
		"_rels/.rels":                  fixtureRootRels,
		"word/_rels/document.xml.rels": docRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>hi</w:t></w:r></w:p></w:body></w:document>`,
		"word/footnotes.xml": footnotes,
	})
	d := reopenBytes(t, fixture)
	// The reader can still see the existing note.
	if fns := d.Footnotes(); len(fns) != 1 || fns[0].Text() != "existing note" {
		t.Fatalf("read existing footnote failed: %v", fns)
	}
	saved := saveBytes(t, d)
	got := zipEntryString(t, saved, "word/footnotes.xml")
	if got != footnotes {
		t.Errorf("footnotes.xml not byte-identical after zero-mod save:\n got: %q\nwant: %q", got, footnotes)
	}
}

// --- Images ---

func TestImages_CreateRoundTrip(t *testing.T) {
	d := Create()
	r := d.AddParagraph().AddRun()
	img, err := r.AddImageFromBytes(tinyPNG, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	img.SetSize(48, 24)
	img.SetAltText("logo")
	fr := d.AddParagraph().AddRun()
	fimg, err := fr.AddFloatingImageFromBytes(tinyPNG, "image/png", Anchor{RelativeToPage: true, X: 5, Y: 6})
	if err != nil {
		t.Fatal(err)
	}
	fimg.SetAltText("float")

	rd := reopenBytes(t, saveBytes(t, d))
	imgs := rd.Images()
	if len(imgs) != 2 {
		t.Fatalf("Images() = %d, want 2", len(imgs))
	}
	if imgs[0].AltText() != "logo" {
		t.Errorf("AltText = %q", imgs[0].AltText())
	}
	if imgs[0].Width() != 48 || imgs[0].Height() != 24 {
		t.Errorf("size = %vx%v, want 48x24", imgs[0].Width(), imgs[0].Height())
	}
	if imgs[0].ContentType() != "image/png" {
		t.Errorf("content type = %q", imgs[0].ContentType())
	}
	if !bytes.Equal(imgs[0].Data(), tinyPNG) {
		t.Error("Data() mismatch")
	}
	if imgs[0].PartName() == "" {
		t.Error("PartName empty")
	}
	if imgs[0].Floating() {
		t.Error("first image should be inline")
	}
	if !imgs[1].Floating() || imgs[1].AltText() != "float" {
		t.Errorf("second image should be floating with alt %q", imgs[1].AltText())
	}
}

// --- helpers ---

func saveBytes(t *testing.T, d *Document) []byte {
	t.Helper()
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return data
}

func docXML(t *testing.T, data []byte) string {
	t.Helper()
	return zipEntryString(t, data, "word/document.xml")
}

func zipEntryString(t *testing.T, data []byte, name string) string {
	t.Helper()
	b, ok := zipEntry(t, data, name)
	if !ok {
		t.Fatalf("%s missing from package", name)
	}
	return string(b)
}
