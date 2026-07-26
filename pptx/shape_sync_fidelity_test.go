package pptx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
)

// buildPPTXWithSpTreeBody rewrites test.pptx's slide1.xml, replacing the shapes
// between the group's grpSpPr and </p:spTree> with the given fragment, so a
// test can load a slide whose shapes carry parsed content the public API cannot
// author (noFill, a:br/a:fld, a body without insets, a centered algn).
func buildPPTXWithSpTreeBody(t *testing.T, shapesXML string) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/test.pptx")
	if err != nil {
		t.Fatalf("read test.pptx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := content.String()
		if f.Name == "ppt/slides/slide1.xml" {
			openIdx := strings.Index(s, "</p:grpSpPr>")
			closeIdx := strings.Index(s, "</p:spTree>")
			if openIdx < 0 || closeIdx < 0 {
				t.Fatalf("slide1.xml has no spTree to rewrite")
			}
			openIdx += len("</p:grpSpPr>")
			s = s[:openIdx] + shapesXML + s[closeIdx:]
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("write %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func openDeck(t *testing.T, data []byte) *Presentation {
	t.Helper()
	pres, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	return pres
}

// firstTextBox returns the first TextBox on the reopened slide.
func firstTextBox(t *testing.T, pres *Presentation) *TextBox {
	t.Helper()
	for _, sh := range pres.Slides()[0].Shapes() {
		if tb, ok := sh.(*TextBox); ok {
			return tb
		}
	}
	t.Fatal("no text box materialized from the reopened deck")
	return nil
}

// C244: editing one run on a loaded paragraph that mixes runs with a:br, a:fld
// and endParaRPr must not rebuild the whole a:txBody from the lossy domain
// model — the line break, field and endParaRPr must survive and the two runs
// stay separate.
func TestEditRunPreservesBrFldEndParaRPr(t *testing.T) {
	shape := `<p:sp><p:nvSpPr><p:cNvPr id="2" name="TextBox 1"/>` +
		`<p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>` +
		`<p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>` +
		`<a:p>` +
		`<a:r><a:rPr lang="en-US"/><a:t>Line1</a:t></a:r>` +
		`<a:br/>` +
		`<a:r><a:rPr lang="en-US"/><a:t>Line2</a:t></a:r>` +
		`<a:fld id="{4A9C4C6E-5B2A-4C1D-9E8F-1A2B3C4D5E6F}" type="slidenum"><a:t>3</a:t></a:fld>` +
		`<a:endParaRPr lang="en-US" b="0"/>` +
		`</a:p></p:txBody></p:sp>`
	data := buildPPTXWithSpTreeBody(t, shape)

	pres := openDeck(t, data)
	tb := firstTextBox(t, pres)
	tb.TextFrame().Paragraphs()[0].Runs()[0].SetBold(true)

	out, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))

	for _, want := range []string{"<a:br", "<a:fld", "endParaRPr", "Line1", "Line2", `b="1"`} {
		if !strings.Contains(xml, want) {
			t.Errorf("saved slide lost %q:\n%s", want, xml)
		}
	}
	// The two lines must not merge: the a:br must still sit between Line1 and
	// Line2, and both run texts must be present.
	if got := strings.Count(xml, "<a:t>Line"); got != 2 {
		t.Errorf("expected two separate runs, got %d run texts:\n%s", got, xml)
	}
	if i1, iBr := strings.Index(xml, "Line1"), strings.Index(xml, "<a:br"); i1 < 0 || iBr < i1 {
		t.Errorf("a:br no longer separates the runs:\n%s", xml)
	}
}
