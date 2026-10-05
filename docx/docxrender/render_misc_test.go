package docxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

func TestGutterWidensTheLeftMargin(t *testing.T) {
	sect := `<w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300" w:header="0" w:footer="0" w:gutter="150"/></w:sectPr>`
	p, _ := wordTestRender(t, wordTestPara("", wordTestRun("", "gutter"))+sect)
	if x := p.lines(1)[0].x; !near(x, 30) {
		t.Errorf("text starts at %v", x)
	}
}

func TestAutomaticSpacingIsFourteenPointsAndCollapses(t *testing.T) {
	auto := `<w:spacing w:beforeAutospacing="1" w:afterAutospacing="1"/>`
	p, _ := wordTestRender(t, wordTestBody(
		wordTestPara(auto, wordTestRun("", "one")),
		wordTestPara(auto, wordTestRun("", "two")),
	))
	l := p.lines(1)
	// 14 pt is 18.67 px, once between the two paragraphs.
	if !near(l[0].y, wordTestTop+14*96.0/72+wordTestAscent12) {
		t.Errorf("space before %v", l[0].y)
	}
	if !near(l[1].y-l[0].y, 12+14*96.0/72) {
		t.Errorf("gap %v", l[1].y-l[0].y)
	}
}

func TestComplexScriptRunsUseTheirOwnSizeAndFont(t *testing.T) {
	body := wordTestBody(wordTestPara("",
		wordTestRun(`<w:rFonts w:ascii="Lat" w:cs="Cmplx"/><w:sz w:val="18"/><w:szCs w:val="24"/><w:b/>`, "plain"),
		wordTestRun(`<w:rFonts w:ascii="Lat" w:cs="Cmplx"/><w:sz w:val="18"/><w:szCs w:val="24"/><w:b/><w:cs/>`, "script"),
	))
	p, o := wordTestRender(t, body)
	a, _ := p.textOf(1, "plain")
	b, _ := p.textOf(1, "script")
	if a.Size.Px() != 12 || b.Size.Px() != 16 {
		t.Errorf("sizes %v %v", a.Size.Px(), b.Size.Px())
	}
	for _, r := range o.requests {
		if r.Family == "Cmplx" && r.Bold {
			t.Errorf("complex script text is bold without bCs: %+v", r)
		}
	}
}

func TestColourSchemeMappingRedirectsThemeColours(t *testing.T) {
	theme := `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements><a:clrScheme name="x"><a:dk1><a:srgbClr val="111111"/></a:dk1><a:lt1><a:srgbClr val="EEEEEE"/></a:lt1><a:dk2><a:srgbClr val="222222"/></a:dk2><a:lt2><a:srgbClr val="DDDDDD"/></a:lt2></a:clrScheme></a:themeElements></a:theme>`
	body := wordTestBody(wordTestPara("", wordTestRun(`<w:color w:val="000000" w:themeColor="text1"/>`, "mapped")))
	for _, tc := range []struct{ mapping, want string }{
		{"", "111111"},
		{`<w:clrSchemeMapping w:bg1="dark1" w:tx1="light1" w:bg2="dark2" w:tx2="light2"/>`, "eeeeee"},
	} {
		p, _ := wordTestRender(t, body, wordTestParts{styles: wordTestStyles, theme: theme, settings: tc.mapping})
		v, _ := p.textOf(1, "mapped")
		if rgbOf(v.Color) != tc.want {
			t.Errorf("mapping %q: %s, want %s", tc.mapping, rgbOf(v.Color), tc.want)
		}
	}
}

func TestInvalidPropertyValuesAreInvalidNotUnsupported(t *testing.T) {
	for _, ppr := range []string{`<w:ind w:left="abc"/>`, `<w:jc w:val="zigzag"/>`, `<w:spacing w:line="x"/>`, `<w:keepNext w:val="maybe"/>`, `<w:spacing w:line="240" w:lineRule="sideways"/>`} {
		o := newWordTestOpts(t, false)
		doc := wordTestDoc(t, wordTestBody(wordTestPara(ppr, wordTestRun("", "x"))), wordTestParts{styles: wordTestStyles})
		if _, err := Prepare(context.Background(), doc, o.Options); !errors.Is(err, render.ErrInvalid) {
			t.Errorf("%s: %v", ppr, err)
		}
	}
	for _, rpr := range []string{`<w:sz w:val="x"/>`, `<w:sz w:val="0"/>`, `<w:color w:val="nothex"/>`, `<w:highlight w:val="plaid"/>`, `<w:vertAlign w:val="up"/>`, `<w:b w:val="2"/>`} {
		o := newWordTestOpts(t, false)
		doc := wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun(rpr, "x"))), wordTestParts{styles: wordTestStyles})
		if _, err := Prepare(context.Background(), doc, o.Options); !errors.Is(err, render.ErrInvalid) {
			t.Errorf("%s: %v", rpr, err)
		}
	}
}

func TestPageBreakInAnEmptyParagraph(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(
		wordTestPara("", `<w:r><w:br w:type="page"/></w:r>`),
		wordTestPara("", wordTestRun("", "second")),
	))
	// The line holding the break stays on page 1; the paragraph's mark is an
	// empty line on page 2, above the next paragraph.
	if got := lineCounts(p); !equalInts(got, []int{1, 2}) {
		t.Fatalf("lines per page %v", got)
	}
	l := p.lines(2)
	if !near(l[1].y-l[0].y, 12) || !strings.HasPrefix(l[1].text, "second") {
		t.Errorf("page 2 %+v", l)
	}
}

func TestBreaksInsideFieldInstructionsAreIgnored(t *testing.T) {
	body := wordTestBody(wordTestPara("",
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>`,
		`<w:r><w:br w:type="page"/><w:t>instruction</w:t></w:r>`,
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>`,
		wordTestRun("", "result"),
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>`,
	))
	p, _ := wordTestRender(t, body)
	if p.Count() != 1 {
		t.Fatalf("%d pages", p.Count())
	}
	if got := p.lines(1)[0].text; got != "result" {
		t.Errorf("drawn %q", got)
	}
}

func TestPageDecoratorsSeam(t *testing.T) {
	old := wordPageDecorators
	defer func() { wordPageDecorators = old }()
	called := 0
	wordPageDecorators = append(wordPageDecorators, func(_ *wordRenderer, secs []*wordLaidSection, pages []*wordPage) error {
		for _, p := range pages {
			if p.sec == nil || len(secs) != 1 {
				t.Errorf("page without section")
			}
			called++
		}
		return nil
	})
	wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun("", wordTestLines(20)))))
	if called != 2 {
		t.Errorf("decorated %d pages", called)
	}
}
