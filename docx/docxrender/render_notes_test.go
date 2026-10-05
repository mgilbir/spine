package docxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/render"
)

// fnNote is a footnote or endnote part element holding one paragraph per text:
// the first starts with the note's own mark.
func fnNote(kind, id string, texts ...string) string {
	var sb strings.Builder
	sb.WriteString(`<w:` + kind + ` w:id="` + id + `">`)
	for i, t := range texts {
		sb.WriteString("<w:p>")
		if i == 0 {
			sb.WriteString(`<w:r><w:` + kind + `Ref/></w:r>`)
		}
		sb.WriteString(wordTestRun("", t) + "</w:p>")
	}
	sb.WriteString(`</w:` + kind + `>`)
	return sb.String()
}

// fnSeparators are the separator notes, one 12 pixel line each.
func fnSeparators(kind string) string {
	return `<w:` + kind + ` w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:` + kind + `>` +
		`<w:` + kind + ` w:type="continuationSeparator" w:id="0"><w:p><w:r><w:continuationSeparator/></w:r></w:p></w:` + kind + `>`
}

func fnPart(kind string, notes ...string) wordTestExtra {
	ct, rel := wordTestCTFootnotes, wordTestRelFootnotes
	if kind == "endnote" {
		ct, rel = wordTestCTEndnotes, wordTestRelEndnotes
	}
	return wordTestExtra{`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:` + kind + `s ` + wordTestNS + `>` + fnSeparators(kind) + strings.Join(notes, "") + `</w:` + kind + `s>`, ct, rel}
}

func fnRef(id string) string {
	return `<w:r><w:rPr><w:vertAlign w:val="superscript"/></w:rPr><w:footnoteReference w:id="` + id + `"/></w:r>`
}

func enRef(id string) string {
	return `<w:r><w:rPr><w:vertAlign w:val="superscript"/></w:rPr><w:endnoteReference w:id="` + id + `"/></w:r>`
}

func fnRender(t *testing.T, body, settings string, notes ...wordTestExtra) (*Pages, *wordTestOpts) {
	t.Helper()
	extra := map[string]wordTestExtra{}
	for _, n := range notes {
		if n.relType == wordTestRelEndnotes {
			extra["word/endnotes.xml"] = n
		} else {
			extra["word/footnotes.xml"] = n
		}
	}
	return hfRender(t, body, settings, extra)
}

// fnRules are the separators drawn on a page: the filled 1 pixel rules, as x,
// y and width.
func fnRules(p *Pages, page int) [][3]float64 {
	var out [][3]float64
	for _, op := range p.pages[page-1].ops {
		if v, ok := op.(layout.FillRect); ok && v.Rect.H.Px() == 1 && v.Rect.W.Px() < 300 {
			out = append(out, [3]float64{v.Rect.X.Px(), v.Rect.Y.Px(), v.Rect.W.Px()})
		}
	}
	return out
}

// fnBody counts the lines of a page that are body text (they start with "w").
func fnBody(p *Pages, page int) int {
	n := 0
	for _, l := range p.lines(page) {
		if strings.HasPrefix(l.text, "w") {
			n++
		}
	}
	return n
}

// fnPara is a paragraph of n lines whose first line holds the references.
func fnPara(n int, refs ...string) string {
	return wordTestPara("", wordTestRun("", "w "), strings.Join(refs, ""), wordTestRun("", wordTestLines(n)))
}

func TestFootnoteIsDrawnAtTheBottomOfThePage(t *testing.T) {
	body := wordTestPara("", wordTestRun("", "text"), fnRef("1")) + wordTestPage
	p, o := fnRender(t, body, "", fnPart("footnote", fnNote("footnote", "1", "the note")))
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	// The note is the last line of the text area, its mark first; the
	// separator is the line above it, 144 pt long.
	note := hfLine(t, p, 1, "1the note")
	if !near(note.x, 20) || !near(note.y, 180-12+9.6) {
		t.Errorf("note at %v,%v", note.x, note.y)
	}
	rules := fnRules(p, 1)
	if len(rules) != 1 || !near(rules[0][0], 20) || !near(rules[0][2], 192) || rules[0][1] < 180-24 || rules[0][1] > 180-12 {
		t.Errorf("rules %v", rules)
	}
	// The mark is drawn in the text with the run's formatting.
	mark := 0
	for _, tx := range p.texts(1) {
		if tx.text == "1" && tx.size < 12 {
			mark++
		}
	}
	if mark != 1 {
		t.Errorf("%d superscript marks", mark)
	}
}

func TestFootnoteReducesTheTextArea(t *testing.T) {
	// A one line note and its separator take two lines of the 13: the first
	// page holds 11 lines of the paragraph, and the rest flows on.
	body := fnPara(15, fnRef("1")) + wordTestPage
	p, _ := fnRender(t, body, "", fnPart("footnote", fnNote("footnote", "1", "the note")))
	if got := fnBody(p, 1); got != 11 {
		t.Errorf("%d lines on page 1", got)
	}
	if got := fnBody(p, 2); got != 4 {
		t.Errorf("%d lines on page 2", got)
	}
	if rules := fnRules(p, 2); len(rules) != 0 {
		t.Errorf("page 2 has a separator: %v", rules)
	}
}

func TestFootnoteGoesToThePageOfItsReference(t *testing.T) {
	body := fnPara(13) + fnPara(1, fnRef("1")) + wordTestPage
	p, _ := fnRender(t, body, "", fnPart("footnote", fnNote("footnote", "1", "the note")))
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	if got := hfTexts(p, 1); len(got) != 13 {
		t.Errorf("page 1: %v", got)
	}
	hfLine(t, p, 2, "1the note")
}

func TestFootnoteMovesALineThatDoesNotFitWithIt(t *testing.T) {
	// The reference is on the 13th line; with the note's 24 pixels the text
	// area holds 11 lines, so the line moves to the next page with its note
	// while the 12 lines before it stay.
	p := wordTestPara("", wordTestRun("", wordTestLines(12)), wordTestRun("", "w "), fnRef("1"), wordTestRun("", wordTestLines(2)))
	body := p + wordTestPage
	got, _ := fnRender(t, body, "", fnPart("footnote", fnNote("footnote", "1", "the note")))
	if n := fnBody(got, 1); n != 12 {
		t.Errorf("%d lines on page 1: %v", n, hfTexts(got, 1))
	}
	hfLine(t, got, 2, "1the note")
}

func TestSeveralNotesOnAPageKeepTheirOrder(t *testing.T) {
	body := fnPara(3, fnRef("1"), fnRef("2")) + wordTestPage
	p, _ := fnRender(t, body, "", fnPart("footnote", fnNote("footnote", "2", "second"), fnNote("footnote", "1", "first")))
	l := p.lines(1)
	if len(l) < 5 {
		t.Fatalf("lines %v", l)
	}
	if l[len(l)-2].text != "1first" || l[len(l)-1].text != "2second" || !near(l[len(l)-1].y, 180-12+9.6) {
		t.Errorf("notes %v", l[len(l)-2:])
	}
	// 3 body lines, the separator and two notes.
	if rules := fnRules(p, 1); len(rules) != 1 || rules[0][1] < 180-36 {
		t.Errorf("rules %v", rules)
	}
}

func TestLongFootnoteContinuesOnTheNextPage(t *testing.T) {
	var lines []string
	for i := 0; i < 16; i++ {
		lines = append(lines, "n"+strings.Repeat("x", i))
	}
	body := fnPara(1, fnRef("1")) + wordTestPara("", wordTestRun("", wordTestLines(30))) + wordTestPage
	p, o := fnRender(t, body, "", fnPart("footnote", fnNote("footnote", "1", lines...)))
	// Sixteen one line paragraphs of the note, 12 pixels each: the page has 13
	// lines, the separator takes one and the body line one, so 11 lines of
	// the note stay and the other five continue under the continuation
	// separator.
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	var first, second []string
	for _, l := range p.lines(1) {
		if strings.HasPrefix(l.text, "n") || strings.HasPrefix(l.text, "1n") {
			first = append(first, l.text)
		}
	}
	for _, l := range p.lines(2) {
		if strings.HasPrefix(l.text, "n") {
			second = append(second, l.text)
		}
	}
	if len(first) != 11 || len(second) == 0 {
		t.Fatalf("%d lines of the note on page 1, %d on page 2", len(first), len(second))
	}
	if len(first)+len(second) != 16 {
		t.Errorf("%d lines of the note drawn", len(first)+len(second))
	}
	r1, r2 := fnRules(p, 1), fnRules(p, 2)
	if len(r1) != 1 || !near(r1[0][2], 192) {
		t.Errorf("page 1 rules %v", r1)
	}
	if len(r2) != 1 || !near(r2[0][2], 260) {
		t.Errorf("page 2 rules %v (the continuation separator spans the text)", r2)
	}
	// The continued text ends at the bottom of the text area.
	if l := p.lines(2); !near(l[len(l)-1].y, 180-12+9.6) {
		t.Errorf("page 2 ends at %v", l[len(l)-1].y)
	}
}

func TestFootnoteNumbering(t *testing.T) {
	two := fnPara(1, fnRef("1")) + fnPara(1, fnRef("2"))
	notes := fnPart("footnote", fnNote("footnote", "1", "a"), fnNote("footnote", "2", "b"))
	t.Run("format and start", func(t *testing.T) {
		p, _ := fnRender(t, two+wordTestPage, `<w:footnotePr><w:numFmt w:val="lowerRoman"/><w:numStart w:val="3"/></w:footnotePr>`, notes)
		hfLine(t, p, 1, "iiia")
		hfLine(t, p, 1, "ivb")
	})
	t.Run("section overrides the settings", func(t *testing.T) {
		sect := hfSect(`<w:footnotePr><w:numFmt w:val="upperLetter"/></w:footnotePr>`, 0, 0)
		p, _ := fnRender(t, two+sect, `<w:footnotePr><w:numFmt w:val="lowerRoman"/></w:footnotePr>`, notes)
		hfLine(t, p, 1, "Aa")
		hfLine(t, p, 1, "Bb")
	})
	t.Run("restart each section", func(t *testing.T) {
		first := `<w:p><w:pPr>` + hfSect(`<w:footnotePr><w:numRestart w:val="eachSect"/></w:footnotePr>`, 0, 0) + `</w:pPr>` + wordTestRun("", "w ") + fnRef("1") + `</w:p>`
		body := first + fnPara(1, fnRef("2")) + hfSect(`<w:footnotePr><w:numRestart w:val="eachSect"/></w:footnotePr>`, 0, 0)
		p, _ := fnRender(t, body, "", notes)
		hfLine(t, p, 1, "1a")
		hfLine(t, p, 2, "1b")
	})
	t.Run("continuous across sections", func(t *testing.T) {
		first := `<w:p><w:pPr>` + hfSect("", 0, 0) + `</w:pPr>` + wordTestRun("", "w ") + fnRef("1") + `</w:p>`
		body := first + fnPara(1, fnRef("2")) + hfSect("", 0, 0)
		p, _ := fnRender(t, body, "", notes)
		hfLine(t, p, 1, "1a")
		hfLine(t, p, 2, "2b")
	})
	t.Run("restart each page", func(t *testing.T) {
		body := fnPara(13, fnRef("1")) + fnPara(1, fnRef("2")) + wordTestPage
		p, o := fnRender(t, body, `<w:footnotePr><w:numRestart w:val="eachPage"/></w:footnotePr>`, notes)
		if len(o.warnings) != 0 {
			t.Fatalf("warnings %v", o.warnings)
		}
		if p.Count() < 2 {
			t.Fatalf("%d pages", p.Count())
		}
		hfLine(t, p, 1, "1a")
		hfLine(t, p, 2, "1b")
	})
	t.Run("restart each page keeps counting on a page", func(t *testing.T) {
		body := fnPara(1, fnRef("1")) + fnPara(1, fnRef("2")) + wordTestPage
		p, _ := fnRender(t, body, `<w:footnotePr><w:numRestart w:val="eachPage"/></w:footnotePr>`, notes)
		hfLine(t, p, 1, "1a")
		hfLine(t, p, 1, "2b")
	})
}

func TestEndnotesFollowTheText(t *testing.T) {
	body := fnPara(2, enRef("1")) + wordTestPage
	p, o := fnRender(t, body, "", fnPart("endnote", fnNote("endnote", "1", "the end")))
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	// Endnotes number in lower-case Roman; they follow the body text, after
	// the separator.
	l := p.lines(1)
	if l[len(l)-1].text != "ithe end" {
		t.Errorf("lines %v", l)
	}
	if rules := fnRules(p, 1); len(rules) != 1 || rules[0][1] < 20+24 {
		t.Errorf("rules %v", rules)
	}
	// Two body lines, the separator line, then the note.
	if d := l[len(l)-1].y - l[len(l)-2].y; !near(d, 24) {
		t.Errorf("note %v below the text", d)
	}
}

func TestEndnotesAtTheEndOfASection(t *testing.T) {
	sect := `<w:endnotePr><w:pos w:val="sectEnd"/></w:endnotePr>`
	first := `<w:p><w:pPr>` + hfSect(sect, 0, 0) + `</w:pPr>` + wordTestRun("", "w ") + enRef("1") + `</w:p>`
	body := first + fnPara(1, enRef("2")) + hfSect(sect, 0, 0)
	p, _ := fnRender(t, body, "", fnPart("endnote", fnNote("endnote", "1", "one"), fnNote("endnote", "2", "two")))
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	hfLine(t, p, 1, "ione")
	hfLine(t, p, 2, "iitwo")
}

func TestNotesInStrictAndBestEffort(t *testing.T) {
	body := fnPara(1, fnRef("1")) + wordTestPage
	notes := fnPart("footnote", fnNote("footnote", "1", "a"))
	t.Run("a number format that is not drawn", func(t *testing.T) {
		settings := `<w:footnotePr><w:numFmt w:val="cardinalText"/></w:footnotePr>`
		doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles, settings: settings, extra: map[string]wordTestExtra{"word/footnotes.xml": notes}})
		if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("strict: %v", err)
		}
		o := newWordTestOpts(t, true)
		wordTestPages(t, doc, o)
		if len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrApproximated) {
			t.Errorf("warnings %v", o.warnings)
		}
	})
	t.Run("a footnote the document does not have", func(t *testing.T) {
		doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles, extra: map[string]wordTestExtra{"word/footnotes.xml": fnPart("footnote")}})
		if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("strict: %v", err)
		}
		o := newWordTestOpts(t, true)
		p := wordTestPages(t, doc, o)
		if len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrUnsupported) {
			t.Errorf("warnings %v", o.warnings)
		}
		if got := fnBody(p, 1); got != 1 {
			t.Errorf("%d lines", got)
		}
	})
	t.Run("footnotes beneath the text", func(t *testing.T) {
		settings := `<w:footnotePr><w:pos w:val="beneathText"/></w:footnotePr>`
		doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles, settings: settings, extra: map[string]wordTestExtra{"word/footnotes.xml": notes}})
		if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("strict: %v", err)
		}
	})
	t.Run("an invalid restart", func(t *testing.T) {
		settings := `<w:footnotePr><w:numRestart w:val="sometimes"/></w:footnotePr>`
		doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles, settings: settings, extra: map[string]wordTestExtra{"word/footnotes.xml": notes}})
		if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, true).Options); !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("err %v", err)
		}
	})
}

func TestNotesSitOnTopOfTheFooter(t *testing.T) {
	// A footer of two lines reaches 24 pixels from the bottom, past the 20
	// pixel margin: the text area ends at 176 and the note ends there.
	body := wordTestPara("", wordTestRun("", "text"), fnRef("1")) + hfSect(hfRef("footer", "default", hfRIDFooter1), 0, 0)
	p, o := hfRender(t, body, "", map[string]wordTestExtra{
		"word/footnotes.xml": fnPart("footnote", fnNote("footnote", "1", "the note")),
		"word/footer1.xml":   wordTestFooter(hfPara("f1"), hfPara("f2")),
	})
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	if n := hfLine(t, p, 1, "1the note"); !near(n.y, 176-12+9.6) {
		t.Errorf("note at %v", n.y)
	}
	if f := hfLine(t, p, 1, "f2"); !near(f.y, 200-12+9.6) {
		t.Errorf("footer at %v", f.y)
	}
}

func TestFootnoteSeparatorStopsAtANarrowTextArea(t *testing.T) {
	// 120 pt of text width is less than the separator's 144 pt.
	body := wordTestPara("", wordTestRun("", "text"), fnRef("1")) + wordTestSect(3000, 3000, 300, 300, 300, 300, "")
	p, _ := fnRender(t, body, "", fnPart("footnote", fnNote("footnote", "1", "the note")))
	if rules := fnRules(p, 1); len(rules) != 1 || !near(rules[0][2], 160) {
		t.Errorf("rules %v", rules)
	}
}
