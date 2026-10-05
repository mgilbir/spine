package docxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/render"
)

func TestLibraryBuiltHeadersFootersAndNotes(t *testing.T) {
	d := docx.Create()
	sec := d.DefaultSection()
	sec.SetPageSize(595, 842)
	sec.SetMargins(docx.PageMargins{Top: 72, Bottom: 72, Left: 72, Right: 72})
	d.AddHeader(docx.HeaderDefault).AddParagraphWithText("The header")
	foot := d.AddFooter(docx.FooterDefault).AddParagraph()
	foot.AddText("Page ")
	foot.AddField(docx.FieldPage)
	foot.AddText(" of ")
	foot.AddField(docx.FieldNumPages)
	d.AddParagraphWithText("Body text with a note").AddRun().AddFootnote("The footnote text")
	d.AddParagraphWithText("And an endnote").AddRun().AddEndnote("The endnote text")
	face, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Limits: render.Limits{MaxShapeWork: 1 << 30}, Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }}
	pages, err := Prepare(context.Background(), d, opts)
	if err != nil {
		t.Fatal(err)
	}
	if pages.Count() != 1 {
		t.Fatalf("%d pages", pages.Count())
	}
	var all strings.Builder
	for _, tx := range pages.texts(1) {
		all.WriteString(tx.text)
	}
	for _, want := range []string{"The header", "Page 1 of 1", "The footnote text", "The endnote text"} {
		if !strings.Contains(all.String(), want) {
			t.Errorf("%q missing from %q", want, all.String())
		}
	}
}

func TestPageFieldsInTheBodyAreChecked(t *testing.T) {
	two := func(cached string) string {
		return wordTestPara("", wordTestRun("", "x "), hfField("PAGE", cached)) +
			wordTestPara("<w:pageBreakBefore/>", wordTestRun("", "y "), hfField("NUMPAGES", "2")) + wordTestPage
	}
	t.Run("saved results that match the page are exact", func(t *testing.T) {
		p, o := hfRender(t, two("1"), "", nil)
		if len(o.warnings) != 0 || p.Count() != 2 {
			t.Fatalf("%d pages, warnings %v", p.Count(), o.warnings)
		}
	})
	t.Run("a stale result is reported", func(t *testing.T) {
		doc := wordTestDoc(t, two("7"), wordTestParts{styles: wordTestStyles})
		if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("strict: %v", err)
		}
		o := newWordTestOpts(t, true)
		p := wordTestPages(t, doc, o)
		if len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrApproximated) {
			t.Errorf("warnings %v", o.warnings)
		}
		// The saved result is what is drawn.
		hfLine(t, p, 1, "x 7")
	})
	t.Run("a result for another page count", func(t *testing.T) {
		body := wordTestPara("", wordTestRun("", "x "), hfField("NUMPAGES", "5")) + wordTestPage
		doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles})
		if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("strict: %v", err)
		}
	})
}
