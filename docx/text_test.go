package docx

import (
	"strings"
	"testing"
)

func TestDocumentText(t *testing.T) {
	doc := Create()

	doc.AddParagraphWithText("Hello world")

	tbl := doc.AddTable(2, 2)
	cells := [][]string{{"A1", "B1"}, {"A2", "B2"}}
	rows := tbl.Rows()
	for i, row := range rows {
		for j, c := range row.Cells() {
			c.Paragraphs()[0].AddRun().SetText(cells[i][j])
		}
	}

	doc.AddParagraphWithText("After table")

	hdr := doc.AddHeader(HeaderDefault)
	hdr.AddParagraphWithText("Header text")
	ftr := doc.AddFooter(FooterDefault)
	ftr.AddParagraphWithText("Footer text")

	anchor := doc.AddParagraph()
	r := anchor.AddRun()
	r.SetText("Anchor")
	r.AddFootnote("Footnote text")

	got := doc.Text()

	// The body must appear in document order: paragraph, then the table with
	// tab-separated cells and newline-separated rows, then the next paragraph.
	wantBody := "Hello world\nA1\tB1\nA2\tB2\nAfter table\nAnchor"
	if !strings.HasPrefix(got, wantBody) {
		t.Fatalf("body text mismatch:\n got: %q\nwant prefix: %q", got, wantBody)
	}

	for _, want := range []string{"Header text", "Footer text", "Footnote text"} {
		if !strings.Contains(got, want) {
			t.Errorf("Text() missing %q; got:\n%s", want, got)
		}
	}
}

func TestDocumentTextHyperlinkAndContentControl(t *testing.T) {
	doc := Create()

	p := doc.AddParagraphWithText("Visit ")
	p.AddHyperlink("Anthropic", "https://www.anthropic.com")

	doc.AddContentControl("tag", "Controlled value")

	got := doc.Text()

	for _, want := range []string{"Visit ", "Anthropic", "Controlled value"} {
		if !strings.Contains(got, want) {
			t.Errorf("Text() missing %q; got:\n%s", want, got)
		}
	}
}

func TestDocumentTextEmpty(t *testing.T) {
	doc := Create()
	if got := doc.Text(); got != "" {
		t.Errorf("empty document Text() = %q, want \"\"", got)
	}
}
