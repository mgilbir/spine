package docx

import (
	"bytes"
	"maps"
	"slices"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/internal/testutil"
)

// TestWindows1252PackageRoundTrip covers a package whose parts declare a
// single-byte code page, which wild files do and Word opens. The text reads
// correctly; an untouched save keeps every part's bytes; and an edited save,
// which regenerates the document, core and custom property parts, must still
// read back the same text. It did not: a regenerated part replayed the
// source's declaration (encoding="windows-1252") over UTF-8 bytes, so
// "Café" read back as "CafÃ©", and raw spans sliced from the source at
// offsets into the transcoded stream were misplaced and still in the code page.
func TestWindows1252PackageRoundTrip(t *testing.T) {
	d := Create()
	d.AddParagraphWithText("Café ‘quoted’ costs €5 – naïve")
	d.AddHeader(HeaderDefault).AddParagraphWithText("Tête")
	d.Properties.Title = "Résumé"
	if err := d.SetCustomProperty("Owner", "Zoë"); err != nil {
		t.Fatal(err)
	}
	created, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// An element the core-properties model does not know is kept by slicing
	// it from the source; non-ASCII text before it shifts the offsets of a
	// transcoded stream.
	core := string(fuzzseed.ZipEntry(created, "docProps/core.xml"))
	core = strings.Replace(core, "</cp:coreProperties>", `<x:note xmlns:x="urn:example">Ünïcode</x:note></cp:coreProperties>`, 1)
	// Likewise a custom property of a variant type the model does not know.
	custom := string(fuzzseed.ZipEntry(created, "docProps/custom.xml"))
	custom = strings.Replace(custom, "</Properties>", `<property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="3" name="Tags"><vt:vector size="1" baseType="lpwstr"><vt:lpwstr>Ünï</vt:lpwstr></vt:vector></property></Properties>`, 1)
	plain := fuzzseed.EditZip(created, [][2]string{{"docProps/core.xml", core}, {"docProps/custom.xml", custom}})
	src, nonASCII, err := testutil.WithWindows1252(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"word/document.xml", "word/header1.xml", "docProps/core.xml", "docProps/custom.xml"} {
		if !slices.Contains(nonASCII, part) {
			t.Fatalf("fixture has no non-ASCII %s: %v", part, nonASCII)
		}
	}

	check := func(label string, data []byte, extra ...string) {
		t.Helper()
		doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		defer func() { _ = doc.Close() }()
		var body []string
		for _, p := range doc.Paragraphs() {
			body = append(body, p.Text())
		}
		want := append([]string{"Café ‘quoted’ costs €5 – naïve"}, extra...)
		if !slices.Equal(body, want) {
			t.Errorf("%s: body %q, want %q", label, body, want)
		}
		if hs := doc.Headers(); len(hs) != 1 || len(hs[0].Paragraphs()) != 1 || hs[0].Paragraphs()[0].Text() != "Tête" {
			t.Errorf("%s: header lost", label)
		}
		if doc.Properties.Title != "Résumé" {
			t.Errorf("%s: title %q", label, doc.Properties.Title)
		}
		if got := doc.CustomProperties()["Owner"]; got != "Zoë" {
			t.Errorf("%s: custom property %q", label, got)
		}
		if out := fuzzseed.ZipEntry(data, "docProps/core.xml"); !bytes.Contains(must(t, out), []byte(`<x:note xmlns:x="urn:example">Ünïcode</x:note>`)) {
			t.Errorf("%s: unknown core child lost: %s", label, out)
		}
		if out := fuzzseed.ZipEntry(data, "docProps/custom.xml"); !bytes.Contains(must(t, out), []byte(`<vt:vector size="1" baseType="lpwstr"><vt:lpwstr>Ünï</vt:lpwstr></vt:vector>`)) {
			t.Errorf("%s: unmodeled custom property lost: %s", label, out)
		}
	}
	check("source", src)

	// An untouched save keeps every part's bytes, also after reading the body,
	// which makes the save rebuild it.
	for _, read := range []bool{false, true} {
		doc, err := OpenReader(bytes.NewReader(src), int64(len(src)))
		if err != nil {
			t.Fatal(err)
		}
		if read {
			_ = doc.Text()
		}
		untouched, err := doc.SaveBytes()
		_ = doc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if want, got := zipEntryMap(t, src), zipEntryMap(t, untouched); !maps.EqualFunc(want, got, bytes.Equal) {
			for name := range want {
				if !bytes.Equal(want[name], got[name]) {
					t.Errorf("untouched save (read=%v) changed %s", read, name)
				}
			}
		}
	}

	doc, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	doc.AddParagraphWithText("Ça va")
	doc.Properties.Subject = "Ü"
	if err = doc.SetCustomProperty("Reviewer", "Åsa"); err != nil {
		t.Fatal(err)
	}
	edited, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	check("edited", edited, "Ça va")
}

// must returns a part's UTF-8 form, failing the test when it has none.
func must(t *testing.T, part []byte) []byte {
	t.Helper()
	text, err := xmlb.UTF8(part)
	if err != nil {
		t.Fatal(err)
	}
	return text
}
