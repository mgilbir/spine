package docxrender

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/testutil"
)

// TestRenderPartsInEveryEncoding started as the #493 regression: Word writes a
// UTF-8 byte order mark at the start of some parts (document.xml, theme1.xml),
// which the source check read as content outside the root, so strict rendering
// failed with "content outside XML root". With a BOM on every XML part, every
// part the renderer reads must render exactly as without one. The same package
// with every part in Windows-1252, and in UTF-16 of either byte order, which
// the models read, must render the same too.
func TestRenderPartsInEveryEncoding(t *testing.T) {
	numbering := numAbstract("0", "", numDecimal(0, "%1.", ""), numLvl(1, `<w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:lvlText w:val="%1.%2"/>`)) + numInstance("1", "0")
	hdr := wordTestHeader(wordTestPara("", wordTestRun("", "H"), hfField("PAGE", "1"), hfField("NUMPAGES", "1")))
	ftr := wordTestFooter(wordTestPara("", wordTestRun("", "F"), `<w:r><w:pgNum/></w:r>`))
	notes := fnPart("footnote", fnNote("footnote", "1", "note"), fnNote("footnote", "2", "a", "b"))
	ends := fnPart("endnote", fnNote("endnote", "1", "end"))
	seed := wordTestDoc(t, wordTestPara("", wordTestRun("", "AA"), fnRef("1"), enRef("1"))+numPara("1", 0, "AA")+numPara("1", 1, "AA")+hfSect(hfRef("header", "default", hfRIDHeader1)+hfRef("footer", "default", hfRIDFooter1), 0, 0),
		wordTestParts{styles: wordTestStyles, settings: wordTestWordSettings, theme: wordTestWordTheme, numbering: numbering,
			media: wordTestMedia(wordTestHalves(8, 4)).media,
			extra: map[string]wordTestExtra{"word/header1.xml": hdr, "word/footer1.xml": ftr, "word/footnotes.xml": notes, "word/endnotes.xml": ends}})
	plain, err := seed.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	withBOM, changed, err := testutil.WithBOM(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"word/document.xml", "word/theme/theme1.xml", "word/styles.xml", "word/settings.xml", "word/numbering.xml", "word/header1.xml", "word/footnotes.xml"} {
		if !slices.Contains(changed, part) {
			t.Fatalf("fixture has no %s to mark: %v", part, changed)
		}
	}
	svg := func(data []byte) []byte {
		t.Helper()
		doc, err := docx.OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = doc.Close() }()
		pages, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options) // strict
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		for n := 1; n <= pages.Count(); n++ {
			page, err := pages.Page(context.Background(), n)
			if err != nil {
				t.Fatal(err)
			}
			if err = page.WriteSVG(context.Background(), &out, 96); err != nil {
				t.Fatal(err)
			}
		}
		return out.Bytes()
	}
	want := svg(plain)
	if got := svg(withBOM); !bytes.Equal(got, want) {
		t.Error("a document with BOM-prefixed parts renders differently")
	}
	// Parts in a code page, which the models read, render the same too. The
	// renderer read them with a decoder that has no charset support and
	// failed with "CharsetReader is nil".
	encoded, _, err := testutil.WithWindows1252(plain)
	if err != nil {
		t.Fatal(err)
	}
	if got := svg(encoded); !bytes.Equal(got, want) {
		t.Error("a document with Windows-1252 parts renders differently")
	}
	for _, big := range []bool{false, true} {
		encoded, err := testutil.WithUTF16(plain, big)
		if err != nil {
			t.Fatal(err)
		}
		if got := svg(encoded); !bytes.Equal(got, want) {
			t.Errorf("a document with UTF-16 parts (big endian %v) renders differently", big)
		}
	}
}
