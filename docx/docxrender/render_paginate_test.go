package docxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

// wordTestLines is the text of a paragraph that wraps to exactly n lines on
// wordTestPage: eight words fill a line of the 260 pixel text area, which holds
// 13 lines of 12 pixels.
func wordTestLines(n int) string { return strings.Repeat("word ", 8*n) }

func wordTestLinesWith(label string, n int) string {
	return label + " " + strings.Repeat("word ", 8*n-1)
}

func lineCounts(p *Pages) []int {
	var out []int
	for i := 1; i <= p.Count(); i++ {
		out = append(out, len(p.lines(i)))
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPaginationCutsAtLines(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun("", wordTestLines(30)))))
	if got := lineCounts(p); !equalInts(got, []int{13, 13, 4}) {
		t.Fatalf("lines per page %v", got)
	}
	// Every page starts at the top of the text area.
	for pg := 2; pg <= 3; pg++ {
		if y := p.lines(pg)[0].y; !near(y, wordTestTop+wordTestAscent12) {
			t.Errorf("page %d first baseline %v", pg, y)
		}
	}
}

func TestOrphanControlMovesAParagraphWithOneLineOfRoom(t *testing.T) {
	// 12 lines leave room for one line of the next paragraph, which would be an
	// orphan.
	body := wordTestBody(
		wordTestPara("", wordTestRun("", wordTestLinesWith("a", 12))),
		wordTestPara("", wordTestRun("", wordTestLinesWith("b", 4))),
	)
	p, _ := wordTestRender(t, body)
	if got := lineCounts(p); !equalInts(got, []int{12, 4}) {
		t.Fatalf("lines per page %v", got)
	}
}

func TestWidowControlLeavesTwoLinesOnTheNextPage(t *testing.T) {
	body := wordTestBody(wordTestPara("", wordTestRun("", wordTestLines(14))))
	p, _ := wordTestRender(t, body)
	if got := lineCounts(p); !equalInts(got, []int{12, 2}) {
		t.Fatalf("lines per page %v", got)
	}
	off := wordTestBody(wordTestPara(`<w:widowControl w:val="0"/>`, wordTestRun("", wordTestLines(14))))
	p, _ = wordTestRender(t, off)
	if got := lineCounts(p); !equalInts(got, []int{13, 1}) {
		t.Fatalf("without widow control %v", got)
	}
}

func TestKeepLinesMovesTheWholeParagraph(t *testing.T) {
	body := wordTestBody(
		wordTestPara("", wordTestRun("", wordTestLinesWith("a", 10))),
		wordTestPara(`<w:keepLines/>`, wordTestRun("", wordTestLinesWith("b", 5))),
	)
	p, _ := wordTestRender(t, body)
	if got := lineCounts(p); !equalInts(got, []int{10, 5}) {
		t.Fatalf("lines per page %v", got)
	}
	// Without it the paragraph splits across the pages.
	plain := wordTestBody(
		wordTestPara("", wordTestRun("", wordTestLinesWith("a", 10))),
		wordTestPara("", wordTestRun("", wordTestLinesWith("b", 5))),
	)
	p, _ = wordTestRender(t, plain)
	if got := lineCounts(p); !equalInts(got, []int{13, 2}) {
		t.Fatalf("plain split %v", got)
	}
}

func TestKeepNextKeepsAHeadingWithTheFirstLinesOfTheNextParagraph(t *testing.T) {
	// 12 lines then a heading: the heading fits on page 1 but the 4-line
	// paragraph after it would have one line there.
	body := wordTestBody(
		wordTestPara("", wordTestRun("", wordTestLinesWith("a", 12))),
		wordTestPara(`<w:keepNext/>`, wordTestRun("", "Heading")),
		wordTestPara("", wordTestRun("", wordTestLinesWith("b", 4))),
	)
	p, _ := wordTestRender(t, body)
	if got := lineCounts(p); !equalInts(got, []int{12, 5}) {
		t.Fatalf("lines per page %v", got)
	}
	if !strings.HasPrefix(p.lines(2)[0].text, "Heading") {
		t.Errorf("page 2 starts with %q", p.lines(2)[0].text)
	}
	// Without keepNext the heading stays and the paragraph moves (orphan control).
	plain := wordTestBody(
		wordTestPara("", wordTestRun("", wordTestLinesWith("a", 12))),
		wordTestPara("", wordTestRun("", "Heading")),
		wordTestPara("", wordTestRun("", wordTestLinesWith("b", 4))),
	)
	p, _ = wordTestRender(t, plain)
	if got := lineCounts(p); !equalInts(got, []int{13, 4}) {
		t.Fatalf("plain %v", got)
	}
}

func TestKeepNextChainLongerThanAPageIsIgnored(t *testing.T) {
	var paras []string
	for i := 0; i < 20; i++ {
		paras = append(paras, wordTestPara(`<w:keepNext/>`, wordTestRun("", "line")))
	}
	paras = append(paras, wordTestPara("", wordTestRun("", "end")))
	p, _ := wordTestRender(t, wordTestBody(paras...))
	if got := lineCounts(p); !equalInts(got, []int{13, 8}) {
		t.Fatalf("lines per page %v", got)
	}
}

func TestPageBreakBeforeAndExplicitBreaks(t *testing.T) {
	body := wordTestBody(
		wordTestPara("", wordTestRun("", "one")),
		wordTestPara(`<w:pageBreakBefore/>`, wordTestRun("", "two")),
		wordTestPara("", wordTestRun("", "three before "), `<w:r><w:br w:type="page"/></w:r>`, wordTestRun("", "three after")),
		wordTestPara(`<w:spacing w:before="300"/>`, wordTestRun("", "four")),
	)
	p, _ := wordTestRender(t, body)
	if p.Count() != 3 {
		t.Fatalf("%d pages", p.Count())
	}
	if got := lineCounts(p); !equalInts(got, []int{1, 2, 2}) {
		t.Fatalf("lines per page %v", got)
	}
	if !strings.HasPrefix(p.lines(2)[1].text, "three before") || !strings.HasPrefix(p.lines(3)[0].text, "three after") {
		t.Errorf("break position: %+v %+v", p.lines(2), p.lines(3))
	}
	// After a manual break the next paragraph keeps its space before, and it is
	// not the first line of the paragraph, so it takes no first-line indent.
	if !near(p.lines(3)[1].y-p.lines(3)[0].y, 12+20) {
		t.Errorf("space before after a continuation: %v", p.lines(3))
	}
}

func TestContinuationAfterBreakTakesNoFirstLineIndent(t *testing.T) {
	body := wordTestBody(wordTestPara(`<w:ind w:firstLine="300"/>`, wordTestRun("", "before"), `<w:r><w:br w:type="page"/></w:r>`, wordTestRun("", "after")))
	p, _ := wordTestRender(t, body)
	if !near(p.lines(1)[0].x, 40) || !near(p.lines(2)[0].x, 20) {
		t.Errorf("indents %v %v", p.lines(1)[0].x, p.lines(2)[0].x)
	}
}

func TestManualBreaksSuppressSpaceBeforeAtPageTop(t *testing.T) {
	// Word's PDF output: a paragraph with space before that a manual break
	// puts at the top of a page starts at the margin.
	sp := `<w:spacing w:before="300"/>`
	for name, tc := range map[string]struct {
		body  string
		first int
	}{
		// The empty line holding the break stays on page 1.
		"break alone in a paragraph": {wordTestBody(wordTestPara("", wordTestRun("", "one")), wordTestPara("", `<w:r><w:br w:type="page"/></w:r>`), wordTestPara(sp, wordTestRun("", "next"))), 2},
		"break ending a paragraph":   {wordTestBody(wordTestPara("", wordTestRun("", "one"), `<w:r><w:br w:type="page"/></w:r>`), wordTestPara(sp, wordTestRun("", "next"))), 1},
		"pageBreakBefore":            {wordTestBody(wordTestPara("", wordTestRun("", "one")), wordTestPara(sp+`<w:pageBreakBefore/>`, wordTestRun("", "next"))), 1},
	} {
		p, _ := wordTestRender(t, tc.body)
		if got := lineCounts(p); !equalInts(got, []int{tc.first, 1}) {
			t.Errorf("%s: lines per page %v", name, got)
			continue
		}
		if l := p.lines(2)[0]; l.text != "next" || !near(l.y, wordTestTop+wordTestAscent12) {
			t.Errorf("%s: page 2 starts %+v", name, l)
		}
	}
}

func TestSectionBreakKeepsTheLargerSpacing(t *testing.T) {
	// After a section break the first paragraph keeps the part of its space
	// before beyond the previous paragraph's space after (Word's larger-of
	// rule, across the page): 30 px before less 10 px after leaves 20.
	sect := `<w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300" w:header="0" w:footer="0" w:gutter="0"/></w:sectPr>`
	body := wordTestBody(
		wordTestPara(`<w:spacing w:after="150"/>`+sect, wordTestRun("", "one")),
		wordTestPara(`<w:spacing w:before="450"/>`, wordTestRun("", "next")),
	)
	p, _ := wordTestRender(t, body)
	if got := lineCounts(p); !equalInts(got, []int{1, 1}) {
		t.Fatalf("lines per page %v", got)
	}
	if y := p.lines(2)[0].y; !near(y, wordTestTop+20+wordTestAscent12) {
		t.Errorf("first baseline %v", y)
	}
}

func TestSoftBreakSuppressesSpaceBeforeAtPageTop(t *testing.T) {
	body := wordTestBody(
		wordTestPara("", wordTestRun("", wordTestLinesWith("a", 13))),
		wordTestPara(`<w:spacing w:before="300"/>`, wordTestRun("", "next")),
	)
	p, _ := wordTestRender(t, body)
	if got := lineCounts(p); !equalInts(got, []int{13, 1}) {
		t.Fatalf("lines per page %v", got)
	}
	if y := p.lines(2)[0].y; !near(y, wordTestTop+wordTestAscent12) {
		t.Errorf("space before kept after a soft break: %v", y)
	}
}

func TestSectionBreaks(t *testing.T) {
	first := wordTestPara(`<w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300" w:header="0" w:footer="0" w:gutter="0"/></w:sectPr>`, wordTestRun("", "first"))
	t.Run("next page with its own geometry", func(t *testing.T) {
		second := wordTestSect(3000, 4500, 450, 150, 300, 600, "")
		p, _ := wordTestRender(t, first+wordTestPara("", wordTestRun("", "second"))+second)
		if p.Count() != 2 {
			t.Fatalf("%d pages", p.Count())
		}
		w1, h1 := p.pages[0].w, p.pages[0].h
		w2, h2 := p.pages[1].w, p.pages[1].h
		if w1 != 300 || h1 != 200 || w2 != 200 || h2 != 300 {
			t.Errorf("sizes %vx%v %vx%v", w1, h1, w2, h2)
		}
		l := p.lines(2)[0]
		if !near(l.x, 40) || !near(l.y, 30+wordTestAscent12) {
			t.Errorf("second section origin %+v", l)
		}
	})
	t.Run("continuous shares the page", func(t *testing.T) {
		second := wordTestSect(4500, 3000, 300, 300, 300, 300, `<w:type w:val="continuous"/>`)
		p, _ := wordTestRender(t, first+wordTestPara("", wordTestRun("", "second"))+second)
		if p.Count() != 1 {
			t.Fatalf("%d pages", p.Count())
		}
		l := p.lines(1)
		if len(l) != 2 || !near(l[1].y-l[0].y, 12) {
			t.Errorf("lines %+v", l)
		}
	})
	t.Run("odd page adds a blank page", func(t *testing.T) {
		second := wordTestSect(4500, 3000, 300, 300, 300, 300, `<w:type w:val="oddPage"/>`)
		p, _ := wordTestRender(t, first+wordTestPara("", wordTestRun("", "second"))+second)
		if p.Count() != 3 || len(p.lines(2)) != 0 || len(p.lines(3)) != 1 {
			t.Errorf("pages %d, %v", p.Count(), lineCounts(p))
		}
	})
	t.Run("even page after one page", func(t *testing.T) {
		second := wordTestSect(4500, 3000, 300, 300, 300, 300, `<w:type w:val="evenPage"/>`)
		p, _ := wordTestRender(t, first+wordTestPara("", wordTestRun("", "second"))+second)
		if p.Count() != 2 || len(p.lines(2)) != 1 {
			t.Errorf("pages %d, %v", p.Count(), lineCounts(p))
		}
	})
}

func TestEmptyDocumentHasOneBlankPage(t *testing.T) {
	p, _ := wordTestRender(t, wordTestPage)
	if p.Count() != 1 || len(p.texts(1)) != 0 {
		t.Fatalf("%d pages", p.Count())
	}
}

func TestPageOutOfRange(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun("", "one"))))
	for _, n := range []int{0, -1, 2} {
		page, err := p.Page(context.Background(), n)
		if page != nil || !errors.Is(err, render.ErrInvalid) {
			t.Errorf("page %d: %v %v", n, page, err)
		}
		if n == 2 && !errors.Is(err, ErrPageOutOfRange) {
			t.Errorf("page 2: %v", err)
		}
	}
	if _, err := p.Page(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestPageLimit(t *testing.T) {
	o := newWordTestOpts(t, false)
	o.Limits.MaxOperations = 2
	doc := wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun("", wordTestLines(40)))), wordTestParts{styles: wordTestStyles})
	if _, err := Prepare(context.Background(), doc, o.Options); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("limit: %v", err)
	}
}
