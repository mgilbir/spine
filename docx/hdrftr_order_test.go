package docx

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
)

// C497 and its class: every document-wide reader that visits the header and
// footer models must do so in part-name order. Iterating the maps directly
// makes a godoc'd "in document order" result depend on Go's randomized map
// iteration, so the same document yields different orderings run to run.
//
// Go randomizes map iteration per range statement, so a handful of repeats over
// three parts reliably catches an unsorted walk.
const hdrFtrOrderRepeats = 40

// multiHeaderDoc builds a saved document with three headers and three footers,
// each carrying one paragraph whose content identifies its part.
func multiHeaderDoc(t *testing.T, fill func(p *Paragraph, tag string)) []byte {
	t.Helper()
	doc := Create()
	doc.AddParagraphWithText("body")
	for i, ht := range []HeaderType{HeaderDefault, HeaderFirst, HeaderEven} {
		h := doc.AddHeader(ht)
		fill(h.AddParagraph(), fmt.Sprintf("h%d", i))
	}
	for i, ft := range []FooterType{FooterDefault, FooterFirst, FooterEven} {
		f := doc.AddFooter(ft)
		fill(f.AddParagraph(), fmt.Sprintf("f%d", i))
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return data
}

// assertStableOrder reopens the saved document hdrFtrOrderRepeats times and
// fails when read returns a different sequence on any of them.
func assertStableOrder(t *testing.T, saved []byte, what string, read func(*Document) []string) {
	t.Helper()
	var first []string
	for i := 0; i < hdrFtrOrderRepeats; i++ {
		doc, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
		if err != nil {
			t.Fatalf("OpenReader: %v", err)
		}
		got := read(doc)
		if i == 0 {
			first = got
			if len(first) < 6 {
				t.Fatalf("%s returned %d entries (%v), want at least 6", what, len(first), first)
			}
			continue
		}
		if strings.Join(got, "|") != strings.Join(first, "|") {
			t.Fatalf("%s order is not deterministic:\n run 0: %v\n run %d: %v", what, first, i, got)
		}
	}
}

// TestHyperlinksHeaderOrderDeterministic covers Document.Hyperlinks.
func TestHyperlinksHeaderOrderDeterministic(t *testing.T) {
	saved := multiHeaderDoc(t, func(p *Paragraph, tag string) {
		p.AddHyperlink("link-"+tag, "https://example.com/"+tag)
	})
	assertStableOrder(t, saved, "Hyperlinks()", func(d *Document) []string {
		var out []string
		for _, h := range d.Hyperlinks() {
			out = append(out, h.Text())
		}
		return out
	})
}

// TestImagesHeaderOrderDeterministic covers Document.Images.
func TestImagesHeaderOrderDeterministic(t *testing.T) {
	saved := multiHeaderDoc(t, func(p *Paragraph, tag string) {
		img, err := p.AddRun().AddImageFromBytes(minimalPNG(), "image/png")
		if err != nil {
			t.Fatalf("AddImageFromBytes: %v", err)
		}
		img.SetAltText("img-" + tag)
	})
	assertStableOrder(t, saved, "Images()", func(d *Document) []string {
		var out []string
		for _, img := range d.Images() {
			out = append(out, img.AltText())
		}
		return out
	})
}

// TestChartsHeaderOrderDeterministic is the audit's named instance: Charts()
// promises "in document order" and iterated the header and footer maps.
func TestChartsHeaderOrderDeterministic(t *testing.T) {
	saved := multiHeaderDoc(t, func(p *Paragraph, tag string) {
		c := chart.NewBar().SetTitle("chart-" + tag).SetCategories([]string{"a"})
		c.AddSeries(tag, []float64{1})
		if err := p.AddChart(c, 2000000, 1000000); err != nil {
			t.Fatalf("AddChart: %v", err)
		}
	})
	assertStableOrder(t, saved, "Charts()", func(d *Document) []string {
		var out []string
		for _, c := range d.Charts() {
			out = append(out, c.Title())
		}
		return out
	})
}

// TestMergeFieldsHeaderOrderDeterministic covers Document.MergeFields, whose
// first-appearance ordering was equally map-dependent.
func TestMergeFieldsHeaderOrderDeterministic(t *testing.T) {
	saved := multiHeaderDoc(t, func(p *Paragraph, tag string) {
		p.AddMergeField("F" + tag)
	})
	assertStableOrder(t, saved, "MergeFields()", func(d *Document) []string {
		return d.MergeFields()
	})
}

// TestWatermarkHeaderOrderDeterministic covers Document.Watermark, which
// returns the *first* watermark it finds, so map order decided *which* of a
// document's watermarks was reported. Word writes one watermark per header, but
// nothing stops the three headers of a section carrying different ones.
func TestWatermarkHeaderOrderDeterministic(t *testing.T) {
	saved := multiWatermarkFixture(t)
	var first string
	for i := 0; i < hdrFtrOrderRepeats; i++ {
		rd, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
		if err != nil {
			t.Fatalf("OpenReader: %v", err)
		}
		wm := rd.Watermark()
		if wm == nil {
			t.Fatalf("Watermark() = nil on run %d", i)
		}
		if i == 0 {
			first = wm.Text
			continue
		}
		if wm.Text != first {
			t.Fatalf("Watermark() is not deterministic: run 0 = %q, run %d = %q", first, i, wm.Text)
		}
	}
	if first != "WM-A" {
		t.Errorf("Watermark() = %q, want the one in the first header by part name (%q)", first, "WM-A")
	}
}

// multiWatermarkFixture builds a docx whose default section references three
// header parts, each carrying a different text watermark.
func multiWatermarkFixture(t *testing.T) []byte {
	t.Helper()
	const ctHeader = `application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml`
	var overrides, rels, refs strings.Builder
	parts := map[string]string{"_rels/.rels": fixtureRootRels}
	names := []string{"A", "B", "C"}
	types := []string{"default", "first", "even"}
	for i, n := range names {
		part := "word/header" + string(rune('1'+i)) + ".xml"
		overrides.WriteString(`<Override PartName="/` + part + `" ContentType="` + ctHeader + `"/>`)
		rels.WriteString(`<Relationship Id="rId1` + string(rune('0'+i)) +
			`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header` +
			string(rune('1'+i)) + `.xml"/>`)
		refs.WriteString(`<w:headerReference w:type="` + types[i] + `" r:id="rId1` + string(rune('0'+i)) + `"/>`)
		parts[part] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:hdr ` + fixtureWNS + `><w:p><w:r><w:pict ` +
			`xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">` +
			`<v:shape id="PowerPlusWaterMarkObject` + string(rune('0'+i)) + `" o:spid="_x0000_s202` +
			string(rune('6'+i)) + `" type="#_x0000_t136" style="position:absolute">` +
			`<v:textpath style="font-family:&quot;Calibri&quot;" string="WM-` + n + `"/>` +
			`</v:shape></w:pict></w:r></w:p></w:hdr>`
	}
	parts["[Content_Types].xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		overrides.String() + `</Types>`
	parts["word/document.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>body</w:t></w:r></w:p>` +
		`<w:sectPr>` + refs.String() + `</w:sectPr></w:body></w:document>`
	parts["word/_rels/document.xml.rels"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + rels.String() + `</Relationships>`
	return buildFixtureDocx(t, parts)
}
