package docxrender

import (
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

func TestCustomMarkAndUnreferencedNotes(t *testing.T) {
	custom := `<w:r><w:footnoteReference w:id="7" w:customMarkFollows="1"/><w:t>*</w:t></w:r>`
	body := wordTestPara("", wordTestRun("", "a "), custom, wordTestRun("", " b "), fnRef("1")) + wordTestPage
	p, o := fnRender(t, body, "", fnPart("footnote",
		`<w:footnote w:id="7"><w:p><w:r><w:t>* custom note</w:t></w:r></w:p></w:footnote>`,
		fnNote("footnote", "1", "plain"),
		fnNote("footnote", "9", "never referenced")))
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	// The custom mark takes no number: the plain note is the first.
	hfLine(t, p, 1, "* custom note")
	hfLine(t, p, 1, "1plain")
	for _, tx := range p.texts(1) {
		if strings.Contains(tx.text, "never") {
			t.Errorf("drew %q", tx.text)
		}
	}
	// Two notes and the separator under the text.
	l := p.lines(1)
	if n := len(l); n < 3 || !near(l[n-1].y, 180-12+9.6) || !near(l[n-2].y, 180-24+9.6) {
		t.Errorf("lines %v", l)
	}
}

func TestNoteReferencesInHeadersAreLeftOut(t *testing.T) {
	body := hfPara("body") + hfSect(hfRef("header", "default", hfRIDHeader1), 0, 0)
	head := wordTestHeader(wordTestPara("", wordTestRun("", "head"), fnRef("1")))
	o := newWordTestOpts(t, false)
	doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles, extra: map[string]wordTestExtra{"word/header1.xml": head, "word/footnotes.xml": fnPart("footnote", fnNote("footnote", "1", "a"))}})
	if _, err := Prepare(t.Context(), doc, o.Options); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
}

func TestContinuationNoticeIsReported(t *testing.T) {
	var lines []string
	for i := 0; i < 16; i++ {
		lines = append(lines, "n"+strings.Repeat("x", i))
	}
	notice := `<w:footnote w:type="continuationNotice" w:id="1"><w:p><w:r><w:t>continued</w:t></w:r></w:p></w:footnote>`
	body := fnPara(1, fnRef("2")) + wordTestPara("", wordTestRun("", wordTestLines(30))) + wordTestPage
	_, o := fnRender(t, body, "", fnPart("footnote", notice, fnNote("footnote", "2", lines...)))
	found := false
	for _, w := range o.warnings {
		found = found || (errors.Is(w, render.ErrApproximated) && strings.Contains(w.Error(), "continuation notice"))
	}
	if !found {
		t.Errorf("warnings %v", o.warnings)
	}
	// A notice only matters where a note continues.
	_, o = fnRender(t, fnPara(1, fnRef("2"))+wordTestPage, "", fnPart("footnote", notice, fnNote("footnote", "2", "short")))
	if len(o.warnings) != 0 {
		t.Errorf("warnings %v", o.warnings)
	}
}

func TestNotesDoNotLoopOnHugeNotes(t *testing.T) {
	// A note of many pages: pagination terminates, taking at most half a page
	// of continued text per page, and reports it.
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "n")
	}
	body := fnPara(2, fnRef("1")) + wordTestPage
	p, o := fnRender(t, body, "", fnPart("footnote", fnNote("footnote", "1", lines...)))
	if p.Count() < 10 || p.Count() > 60 {
		t.Errorf("%d pages", p.Count())
	}
	found := false
	for _, w := range o.warnings {
		found = found || (errors.Is(w, render.ErrApproximated) && strings.Contains(w.Error(), "half a page"))
	}
	if !found {
		t.Errorf("warnings %v", o.warnings)
	}
	total := 0
	for pg := 1; pg <= p.Count(); pg++ {
		for _, l := range p.lines(pg) {
			if strings.HasPrefix(l.text, "n") || strings.HasPrefix(l.text, "1n") {
				total++
			}
		}
	}
	if total != 200 {
		t.Errorf("%d lines of the note drawn", total)
	}
}
