package docxrender

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

// The test page is 300 by 200 pixels with 20 pixel margins, text of 12 pixel
// lines whose baseline is 9.6 below the line top, and 6 pixel glyphs.

const (
	hfRIDHeader1 = "rIdTheader1_xml"
	hfRIDHeader2 = "rIdTheader2_xml"
	hfRIDHeader3 = "rIdTheader3_xml"
	hfRIDFooter1 = "rIdTfooter1_xml"
	hfRIDFooter2 = "rIdTfooter2_xml"
)

func hfRef(kind, typ, rid string) string {
	return `<w:` + kind + `Reference w:type="` + typ + `" r:id="` + rid + `"/>`
}

// hfSect is a section with the given header and footer distances in pixels.
func hfSect(extra string, hdr, ftr int) string {
	return fmt.Sprintf(`<w:sectPr>%s<w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300" w:header="%d" w:footer="%d" w:gutter="0"/></w:sectPr>`, extra, hdr*15, ftr*15)
}

func hfPara(text string) string { return wordTestPara("", wordTestRun("", text)) }

// hfPages is one paragraph per page.
func hfPages(texts ...string) string {
	var sb strings.Builder
	for i, t := range texts {
		ppr := ""
		if i > 0 {
			ppr = "<w:pageBreakBefore/>"
		}
		sb.WriteString(wordTestPara(ppr, wordTestRun("", t)))
	}
	return sb.String()
}

func hfField(instr, cached string) string {
	return `<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> ` + instr + ` </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r>` + wordTestRun("", cached) + `<w:r><w:fldChar w:fldCharType="end"/></w:r>`
}

func hfRender(t *testing.T, body string, settings string, extra map[string]wordTestExtra) (*Pages, *wordTestOpts) {
	t.Helper()
	o := newWordTestOpts(t, true)
	doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles, settings: settings, extra: extra})
	return wordTestPages(t, doc, o), o
}

func hfTexts(p *Pages, page int) []string {
	var out []string
	for _, l := range p.lines(page) {
		out = append(out, l.text)
	}
	return out
}

func hfLine(t *testing.T, p *Pages, page int, text string) wordTestLine {
	t.Helper()
	for _, l := range p.lines(page) {
		if l.text == text {
			return l
		}
	}
	t.Fatalf("page %d has no line %q: %v", page, text, hfTexts(p, page))
	return wordTestLine{}
}

func TestHeaderAndFooterPositions(t *testing.T) {
	body := hfPara("body") + hfSect(hfRef("header", "default", hfRIDHeader1)+hfRef("footer", "default", hfRIDFooter1), 0, 0)
	p, o := hfRender(t, body, "", map[string]wordTestExtra{
		"word/header1.xml": wordTestHeader(hfPara("head")),
		"word/footer1.xml": wordTestFooter(hfPara("foot")),
	})
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	h := hfLine(t, p, 1, "head")
	if !near(h.x, 20) || !near(h.y, 9.6) {
		t.Errorf("header at %v,%v", h.x, h.y)
	}
	f := hfLine(t, p, 1, "foot")
	if !near(f.x, 20) || !near(f.y, 200-12+9.6) {
		t.Errorf("footer at %v,%v", f.x, f.y)
	}
	if b := hfLine(t, p, 1, "body"); !near(b.y, 20+9.6) {
		t.Errorf("body at %v", b.y)
	}
}

func TestHeaderDistanceMovesTheHeader(t *testing.T) {
	body := hfPara("body") + hfSect(hfRef("header", "default", hfRIDHeader1)+hfRef("footer", "default", hfRIDFooter1), 3, 4)
	p, _ := hfRender(t, body, "", map[string]wordTestExtra{
		"word/header1.xml": wordTestHeader(hfPara("head")),
		"word/footer1.xml": wordTestFooter(hfPara("foot")),
	})
	if h := hfLine(t, p, 1, "head"); !near(h.y, 3+9.6) {
		t.Errorf("header at %v", h.y)
	}
	if f := hfLine(t, p, 1, "foot"); !near(f.y, 200-4-12+9.6) {
		t.Errorf("footer at %v", f.y)
	}
}

func TestTallHeaderPushesTheBodyDown(t *testing.T) {
	// Three lines of header from the top edge reach 36 pixels, past the 20
	// pixel margin: the text area loses 16 pixels, which is one 12 pixel line.
	body := wordTestPara("", wordTestRun("", wordTestLines(30))) + hfSect(hfRef("header", "default", hfRIDHeader1), 0, 0)
	p, _ := hfRender(t, body, "", map[string]wordTestExtra{
		"word/header1.xml": wordTestHeader(hfPara("one"), hfPara("two"), hfPara("three")),
	})
	if b := p.lines(1); !near(b[3].y, 36+9.6) {
		t.Fatalf("body starts at %v: %v", b[3].y, hfTexts(p, 1))
	}
	// 200 - 36 - 20 = 144 pixels: 12 body lines (the paragraph has 30).
	got := 0
	for _, l := range p.lines(1) {
		if strings.HasPrefix(l.text, "word") {
			got++
		}
	}
	if got != 12 {
		t.Errorf("%d body lines on page 1", got)
	}
	if p.Count() != 3 {
		t.Errorf("%d pages", p.Count())
	}
}

func TestTallFooterPushesTheBodyUp(t *testing.T) {
	body := wordTestPara("", wordTestRun("", wordTestLines(30))) + hfSect(hfRef("footer", "default", hfRIDFooter1), 0, 0)
	p, _ := hfRender(t, body, "", map[string]wordTestExtra{
		"word/footer1.xml": wordTestFooter(hfPara("one"), hfPara("two"), hfPara("three")),
	})
	got := 0
	for _, l := range p.lines(1) {
		if strings.HasPrefix(l.text, "word") {
			got++
		}
	}
	// 200 - 20 - 36 = 144 pixels.
	if got != 12 {
		t.Errorf("%d body lines on page 1: %v", got, hfTexts(p, 1))
	}
	// The footer's last line ends at the bottom edge.
	if f := hfLine(t, p, 1, "three"); !near(f.y, 200-12+9.6) {
		t.Errorf("footer ends at %v", f.y)
	}
}

func TestFirstEvenAndDefaultHeaders(t *testing.T) {
	refs := hfRef("header", "default", hfRIDHeader1) + hfRef("header", "first", hfRIDHeader2) + hfRef("header", "even", hfRIDHeader3) + "<w:titlePg/>"
	body := hfPages("a", "b", "c", "d") + hfSect(refs, 0, 0)
	parts := map[string]wordTestExtra{
		"word/header1.xml": wordTestHeader(hfPara("default")),
		"word/header2.xml": wordTestHeader(hfPara("first")),
		"word/header3.xml": wordTestHeader(hfPara("even")),
	}
	p, _ := hfRender(t, body, `<w:evenAndOddHeaders/>`, parts)
	want := []string{"first", "even", "default", "even"}
	for i, w := range want {
		hfLine(t, p, i+1, w)
		if n := len(p.lines(i + 1)); n != 2 {
			t.Errorf("page %d has %d lines", i+1, n)
		}
	}
	// Without evenAndOddHeaders the even header is not used, and without titlePg
	// neither is the first.
	p, _ = hfRender(t, hfPages("a", "b", "c")+hfSect(hfRef("header", "default", hfRIDHeader1)+hfRef("header", "first", hfRIDHeader2)+hfRef("header", "even", hfRIDHeader3), 0, 0), "", parts)
	for i := 1; i <= 3; i++ {
		hfLine(t, p, i, "default")
	}
}

func TestMissingTypeInheritsAndUndefinedDrawsNothing(t *testing.T) {
	// Section 1 has a default header; section 2 has none and inherits it, and
	// its first page header (titlePg) is not defined anywhere: blank.
	sect1 := `<w:p><w:pPr>` + hfSect(hfRef("header", "default", hfRIDHeader1), 0, 0) + `</w:pPr>` + wordTestRun("", "s1") + `</w:p>`
	body := sect1 + hfPara("s2") + hfSect("<w:titlePg/>", 0, 0)
	p, _ := hfRender(t, body, "", map[string]wordTestExtra{"word/header1.xml": wordTestHeader(hfPara("head"))})
	hfLine(t, p, 1, "head")
	if got := hfTexts(p, 2); len(got) != 1 || got[0] != "s2" {
		t.Errorf("first page of section 2 has %v", got)
	}
	// Without titlePg the inherited default shows.
	body = sect1 + hfPara("s2") + hfSect("", 0, 0)
	p, _ = hfRender(t, body, "", map[string]wordTestExtra{"word/header1.xml": wordTestHeader(hfPara("head"))})
	hfLine(t, p, 2, "head")
}

func TestSectionOwnHeaderReplacesTheInherited(t *testing.T) {
	sect1 := `<w:p><w:pPr>` + hfSect(hfRef("header", "default", hfRIDHeader1), 0, 0) + `</w:pPr>` + wordTestRun("", "s1") + `</w:p>`
	body := sect1 + hfPara("s2") + hfSect(hfRef("header", "default", hfRIDHeader2), 0, 0)
	p, _ := hfRender(t, body, "", map[string]wordTestExtra{
		"word/header1.xml": wordTestHeader(hfPara("one")),
		"word/header2.xml": wordTestHeader(hfPara("two")),
	})
	hfLine(t, p, 1, "one")
	hfLine(t, p, 2, "two")
}

func TestPageFieldsShowRealNumbers(t *testing.T) {
	foot := wordTestPara("", wordTestRun("", "Page "), hfField("PAGE", "9"), wordTestRun("", " of "), hfField("NUMPAGES", "9"))
	body := hfPages("a", "b", "c") + hfSect(hfRef("footer", "default", hfRIDFooter1), 0, 0)
	p, o := hfRender(t, body, "", map[string]wordTestExtra{"word/footer1.xml": wordTestFooter(foot)})
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	for i, want := range []string{"Page 1 of 3", "Page 2 of 3", "Page 3 of 3"} {
		hfLine(t, p, i+1, want)
	}
}

func TestPageNumberRestartAndFormat(t *testing.T) {
	foot := wordTestPara("", hfField("PAGE", "1"), `<w:r><w:pgNum/></w:r>`, hfField(`PAGE \* roman`, "1"))
	sect1 := hfPara("a") + `<w:p><w:pPr><w:pageBreakBefore/>` + hfSect(hfRef("footer", "default", hfRIDFooter1)+`<w:pgNumType w:fmt="upperRoman" w:start="3"/>`, 0, 0) + `</w:pPr>` + wordTestRun("", "b") + `</w:p>`
	body := sect1 + hfPara("c") + hfSect(`<w:pgNumType w:start="1"/>`, 0, 0)
	p, o := hfRender(t, body, "", map[string]wordTestExtra{"word/footer1.xml": wordTestFooter(foot)})
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	// Pages 1 and 2 count from III in capitals; the second section restarts at
	// 1 in decimal, and the \* roman switch overrides the format.
	hfLine(t, p, 1, "IIIIIIiii")
	hfLine(t, p, 2, "IVIViv")
	hfLine(t, p, 3, "11i")
}

func TestSectionPagesAndSection(t *testing.T) {
	foot := wordTestPara("", hfField("SECTION", "0"), wordTestRun("", ":"), hfField("SECTIONPAGES", "0"), wordTestRun("", "/"), hfField("NUMPAGES", "0"))
	sect1 := `<w:p><w:pPr>` + hfSect(hfRef("footer", "default", hfRIDFooter1), 0, 0) + `</w:pPr>` + wordTestRun("", "a") + `</w:p>`
	body := sect1 + hfPages("b", "c", "d") + hfSect("", 0, 0)
	p, _ := hfRender(t, body, "", map[string]wordTestExtra{"word/footer1.xml": wordTestFooter(foot)})
	for i, want := range []string{"1:1/4", "2:3/4", "2:3/4", "2:3/4"} {
		hfLine(t, p, i+1, want)
	}
}

func TestFooterWithTabsAndFields(t *testing.T) {
	foot := wordTestPara(`<w:tabs><w:tab w:val="right" w:pos="3900"/></w:tabs>`, `<w:r><w:t>left</w:t><w:tab/></w:r>`, hfField("PAGE", "9"))
	body := hfPages("a", "b") + hfSect(hfRef("footer", "default", hfRIDFooter1), 0, 0)
	p, _ := hfRender(t, body, "", map[string]wordTestExtra{"word/footer1.xml": wordTestFooter(foot)})
	for page := 1; page <= 2; page++ {
		x := -1.0
		for _, tx := range p.texts(page) {
			if tx.text == fmt.Sprint(page) {
				x = tx.x
			}
		}
		// The right tab is at 260 from the margin: the digit ends there.
		if !near(x, 20+260-6) {
			t.Errorf("page %d number at %v", page, x)
		}
	}
}

func TestHeaderHeightFollowsThePageCount(t *testing.T) {
	// The header is 42 letters and NUMPAGES: 43 characters fit a line and 44
	// wrap. From the distance 6 two lines reach 30 pixels, past the margin, and
	// the text area loses 10 pixels, a line: so the page count, which decides
	// whether the header wraps, depends on the header.
	head := wordTestPara("", wordTestRun("", strings.Repeat("a", 42)), hfField("NUMPAGES", "0"))
	body := wordTestPara("", wordTestRun("", wordTestLines(118))) + hfSect(hfRef("header", "default", hfRIDHeader1), 6, 0)
	p, o := hfRender(t, body, "", map[string]wordTestExtra{"word/header1.xml": wordTestHeader(head)})
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	if p.Count() != 10 {
		t.Fatalf("%d pages", p.Count())
	}
	if n := len(p.lines(1)); n != 2+12 {
		t.Errorf("%d lines on page 1: %v", n, hfTexts(p, 1))
	}
	if got := hfTexts(p, 1)[0]; got != strings.Repeat("a", 42)+"1" {
		t.Errorf("first header line %q", got)
	}
}

func TestHeaderSettings(t *testing.T) {
	t.Run("strict refuses an approximated page number format", func(t *testing.T) {
		foot := wordTestPara("", hfField("PAGE", "1"))
		body := hfPara("a") + hfSect(hfRef("footer", "default", hfRIDFooter1)+`<w:pgNumType w:fmt="cardinalText"/>`, 0, 0)
		doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles, extra: map[string]wordTestExtra{"word/footer1.xml": wordTestFooter(foot)}})
		_, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options)
		if !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("err %v", err)
		}
		o := newWordTestOpts(t, true)
		p := wordTestPages(t, doc, o)
		if len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrApproximated) {
			t.Errorf("warnings %v", o.warnings)
		}
		hfLine(t, p, 1, "1")
	})
	t.Run("a header taller than the page", func(t *testing.T) {
		var paras []string
		for i := 0; i < 20; i++ {
			paras = append(paras, hfPara("h"))
		}
		body := hfPara("a") + hfSect(hfRef("header", "default", hfRIDHeader1), 0, 0)
		o := newWordTestOpts(t, true)
		doc := wordTestDoc(t, body, wordTestParts{styles: wordTestStyles, extra: map[string]wordTestExtra{"word/header1.xml": wordTestHeader(paras...)}})
		p := wordTestPages(t, doc, o)
		if p.Count() != 1 || len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrApproximated) {
			t.Errorf("%d pages, warnings %v", p.Count(), o.warnings)
		}
	})
	t.Run("a missing part", func(t *testing.T) {
		body := hfPara("a") + hfSect(hfRef("header", "default", "rId77"), 0, 0)
		o := newWordTestOpts(t, true)
		p := wordTestPages(t, wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), o)
		if len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrUnsupported) {
			t.Errorf("warnings %v", o.warnings)
		}
		hfLine(t, p, 1, "a")
	})
	t.Run("a bad reference", func(t *testing.T) {
		body := hfPara("a") + hfSect(`<w:headerReference w:type="sideways" r:id="rId1"/>`, 0, 0)
		_, err := Prepare(context.Background(), wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), newWordTestOpts(t, true).Options)
		if !errors.Is(err, render.ErrInvalid) {
			t.Errorf("err %v", err)
		}
	})
}

func TestOddSectionBreakCountsDisplayedPages(t *testing.T) {
	// The first section restarts at 2, so its page is even and the next, odd
	// page is the very next one: no blank page is needed, though the physical
	// page number of the next page is even.
	foot := wordTestPara("", hfField("PAGE", "0"))
	sect1 := `<w:p><w:pPr>` + hfSect(hfRef("footer", "default", hfRIDFooter1)+`<w:pgNumType w:start="2"/>`, 0, 0) + `</w:pPr>` + wordTestRun("", "a") + `</w:p>`
	body := sect1 + hfPara("b") + hfSect(`<w:type w:val="oddPage"/>`, 0, 0)
	p, _ := hfRender(t, body, "", map[string]wordTestExtra{"word/footer1.xml": wordTestFooter(foot)})
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	for i, want := range []string{"2", "3"} {
		if got := hfTexts(p, i+1); got[len(got)-1] != want {
			t.Errorf("page %d: %v", i+1, got)
		}
	}
}
