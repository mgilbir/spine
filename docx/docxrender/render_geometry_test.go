package docxrender

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// The fixture's baseline sits 0.8 em below the top of a line: 9.6 pixels at 12.
// The text area of wordTestPage starts at (20, 20).
const (
	wordTestAscent12 = 9.6
	wordTestTop      = 20.0
	wordTestLeft     = 20.0
)

func near(a, b float64) bool { return math.Abs(a-b) < 0.05 }

func wordTestBody(paras ...string) string { return strings.Join(paras, "") + wordTestPage }

func wordTestRender(t *testing.T, body string, parts ...wordTestParts) (*Pages, *wordTestOpts) {
	t.Helper()
	if len(parts) == 0 {
		parts = []wordTestParts{{styles: wordTestStyles}}
	}
	o := newWordTestOpts(t, false)
	return wordTestPages(t, wordTestDoc(t, body, parts...), o), o
}

func TestLineSpacingModes(t *testing.T) {
	tests := []struct {
		name, spacing string
		step          float64
		// baseline is the first baseline below the top of its line: Word
		// puts a multiple's extra space below the text, an at-least
		// height's above it, and an exact height's baseline at 0.8 of it.
		baseline float64
	}{
		{"single", `<w:spacing w:line="240" w:lineRule="auto"/>`, 12, 9.6},
		{"one and a half", `<w:spacing w:line="360" w:lineRule="auto"/>`, 18, 9.6},
		{"double", `<w:spacing w:line="480" w:lineRule="auto"/>`, 24, 9.6},
		{"exact", `<w:spacing w:line="300" w:lineRule="exact"/>`, 20, 16},
		{"atLeast above", `<w:spacing w:line="300" w:lineRule="atLeast"/>`, 20, 17.6},
		{"atLeast below", `<w:spacing w:line="120" w:lineRule="atLeast"/>`, 12, 9.6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := wordTestRender(t, wordTestBody(wordTestPara(tc.spacing, wordTestRun("", strings.Repeat("word ", 30)))))
			lines := p.lines(1)
			if len(lines) != 4 {
				t.Fatalf("%d lines", len(lines))
			}
			if !near(lines[0].y, wordTestTop+tc.baseline) {
				t.Errorf("first baseline %v, want %v", lines[0].y-wordTestTop, tc.baseline)
			}
			for i := 1; i < len(lines); i++ {
				if !near(lines[i].y-lines[i-1].y, tc.step) {
					t.Errorf("step %v, want %v", lines[i].y-lines[i-1].y, tc.step)
				}
			}
		})
	}
}

func TestParagraphSpacingTakesTheLargerAndFirstParagraphKeepsSpaceBefore(t *testing.T) {
	// Word separates paragraphs by the larger of the space after and the
	// space before, as its PDF output shows.
	tests := []struct {
		name          string
		first, second string
		gap           float64
	}{
		{"after larger", `<w:spacing w:before="150" w:after="300"/>`, `<w:spacing w:before="150" w:after="300"/>`, 20},
		{"before larger", `<w:spacing w:before="0" w:after="150"/>`, `<w:spacing w:before="450"/>`, 30},
		{"after only", `<w:spacing w:after="150"/>`, `<w:spacing w:before="0"/>`, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := wordTestRender(t, wordTestBody(
				wordTestPara(tc.first, wordTestRun("", "one")),
				wordTestPara(tc.second, wordTestRun("", "two")),
			))
			l := p.lines(1)
			if !near(l[1].y-l[0].y, 12+tc.gap) {
				t.Errorf("gap %v, want %v", l[1].y-l[0].y-12, tc.gap)
			}
		})
	}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara(`<w:spacing w:before="150" w:after="300"/>`, wordTestRun("", "one"))))
	// before 10px is kept at the top of the first page.
	if l := p.lines(1); !near(l[0].y, wordTestTop+10+wordTestAscent12) {
		t.Errorf("first baseline %v", l[0].y)
	}
}

func TestMixedSizesUnderLineSpacingArePlacedLineByLine(t *testing.T) {
	// Layout places each line by its own tallest text: a 32 px run beside a
	// 12 px one at 1.5 lines makes a 48 px line whose baseline is the big
	// run's ascent (0.8 em) below its top, as Word puts it, and the next
	// line, with only small text, is placed by that text.
	body := wordTestBody(wordTestPara(`<w:spacing w:line="360" w:lineRule="auto"/>`, wordTestRun("", "small ")+wordTestRun(`<w:sz w:val="48"/>`, "big")+`<w:r><w:br/></w:r>`+wordTestRun("", "next")))
	p, o := wordTestRender(t, body)
	if len(o.warnings) != 0 {
		t.Fatalf("warnings %v", o.warnings)
	}
	l := p.lines(1)
	if len(l) != 2 || !near(l[0].y, wordTestTop+25.6) || !near(l[1].y, wordTestTop+48+9.6) {
		t.Errorf("lines %+v", l)
	}
}

func TestContextualSpacingIgnoresSpaceBetweenSameStyle(t *testing.T) {
	styles := wordTestStyles + `<w:style w:type="paragraph" w:styleId="Tight"><w:name w:val="Tight"/><w:pPr><w:spacing w:before="150" w:after="150"/><w:contextualSpacing/></w:pPr></w:style>`
	p, _ := wordTestRender(t, wordTestBody(
		wordTestPara(`<w:pStyle w:val="Tight"/>`, wordTestRun("", "one")),
		wordTestPara(`<w:pStyle w:val="Tight"/>`, wordTestRun("", "two")),
		wordTestPara("", wordTestRun("", "three")),
	), wordTestParts{styles: styles})
	l := p.lines(1)
	if !near(l[1].y-l[0].y, 12) {
		t.Errorf("same style gap %v", l[1].y-l[0].y)
	}
	// The last Tight paragraph keeps its after space before a different style.
	if !near(l[2].y-l[1].y, 12+10) {
		t.Errorf("different style gap %v", l[2].y-l[1].y)
	}
}

func TestIndentsAndAlignment(t *testing.T) {
	txt := strings.Repeat("word ", 12)
	p, _ := wordTestRender(t, wordTestBody(
		wordTestPara(`<w:ind w:left="600" w:right="300" w:firstLine="300"/>`, wordTestRun("", txt)),
		wordTestPara(`<w:ind w:left="600" w:hanging="300"/>`, wordTestRun("", txt)),
		wordTestPara(`<w:jc w:val="right"/>`, wordTestRun("", "right")),
		wordTestPara(`<w:jc w:val="center"/>`, wordTestRun("", "mid")),
		wordTestPara(`<w:ind w:start="300" w:end="150"/><w:jc w:val="end"/>`, wordTestRun("", "end")),
	))
	l := p.lines(1)
	if !near(l[0].x, 20+40+20) || !near(l[1].x, 20+40) {
		t.Errorf("first-line indent: %v %v", l[0].x, l[1].x)
	}
	// 40 left + 20 right leaves 200 pixels: six words of 30 characters at 6 px
	// fit with the trailing space hanging.
	if got := strings.TrimSpace(l[1].text); got != strings.TrimSpace(strings.Repeat("word ", 6)) {
		t.Errorf("wrap at right indent: %q", got)
	}
	if !near(l[2].x, 20+40-20) || !near(l[3].x, 20+40) {
		t.Errorf("hanging: %v %v", l[2].x, l[3].x)
	}
	var right, mid, end wordTestLine
	for _, ln := range l {
		switch strings.TrimSpace(ln.text) {
		case "right":
			right = ln
		case "mid":
			mid = ln
		case "end":
			end = ln
		}
	}
	if !near(right.x, 20+260-5*6) {
		t.Errorf("right x %v", right.x)
	}
	if !near(mid.x, 20+130-9) {
		t.Errorf("center x %v", mid.x)
	}
	// start 20, end 10: the text ends 10 pixels inside the right margin.
	if !near(end.x, 20+260-10-3*6) {
		t.Errorf("end x %v", end.x)
	}
}

func TestJustificationStretchesAllButTheLastLine(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestPara(`<w:jc w:val="both"/>`, wordTestRun("", strings.Repeat("word ", 12)))))
	var rights []float64
	for _, tx := range p.texts(1) {
		if tx.text == "word" {
			rights = append(rights, tx.x+24)
		}
	}
	// The final "word" of the first line ends at the right edge of the text area.
	maxRight := 0.0
	for _, r := range rights[:8] {
		maxRight = math.Max(maxRight, r)
	}
	if !near(maxRight, 280) {
		t.Errorf("justified line ends at %v", maxRight)
	}
}

func TestEmptyParagraphHasTheHeightOfItsMark(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(
		wordTestPara(`<w:rPr><w:sz w:val="36"/></w:rPr>`),
		wordTestPara("", wordTestRun("", "after")),
	))
	l := p.lines(1)
	// 18 pt mark = 24 px line, then a 12 px line whose baseline is 9.6 below its top.
	if !near(l[len(l)-1].y, wordTestTop+24+wordTestAscent12) {
		t.Errorf("baseline %v", l[len(l)-1].y)
	}
}

func TestParagraphMarkSizesOnlyALineItIsAloneOn(t *testing.T) {
	// Word's PDF output: a 36 pt mark beside 11 pt text leaves the line at
	// the text's height, and an 8 pt mark does not shrink 24 pt text. Here a
	// 24 px mark beside a 12 px run leaves a 12 px line.
	p, _ := wordTestRender(t, wordTestBody(
		wordTestPara("", wordTestRun("", strings.Repeat("word ", 30))),
		wordTestPara(`<w:rPr><w:sz w:val="36"/></w:rPr>`, wordTestRun("", "x")),
		wordTestPara("", wordTestRun("", "y")),
	))
	l := p.lines(1)
	if last := l[len(l)-1]; !near(last.y, wordTestTop+4*12+12+wordTestAscent12) {
		t.Errorf("last baseline %v, lines %+v", last.y, l)
	}
	// After a line break that ends the paragraph the mark is alone on its
	// line, which it sizes: 12, then 24, then "y".
	p, _ = wordTestRender(t, wordTestBody(
		wordTestPara(`<w:rPr><w:sz w:val="36"/></w:rPr>`, wordTestRun("", "x")+`<w:r><w:br/></w:r>`),
		wordTestPara("", wordTestRun("", "y")),
	))
	if l = p.lines(1); !near(l[len(l)-1].y, wordTestTop+12+24+wordTestAscent12) {
		t.Errorf("after a trailing break: lines %+v", l)
	}
}

func TestSoftHyphenAndNoBreakHyphen(t *testing.T) {
	body := wordTestBody(wordTestPara("", `<w:r><w:t>aa</w:t><w:softHyphen/><w:t>bb</w:t><w:noBreakHyphen/><w:t>cc</w:t></w:r>`))
	p, _ := wordTestRender(t, body)
	got := ""
	for _, tx := range p.texts(1) {
		got += tx.text
	}
	if !strings.Contains(got, "‑") {
		t.Errorf("no-break hyphen not drawn: %q", got)
	}
}

// notoOpts renders with Noto Sans, whose Windows metrics (usWinAscent 1124,
// usWinDescent 395 per 1000) are not its line metrics (1069, 293).
func notoOpts(t *testing.T, lenient bool) *wordTestOpts {
	t.Helper()
	face, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	o := newWordTestOpts(t, lenient)
	o.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }
	return o
}

func TestWindowsMetricsSetWordLines(t *testing.T) {
	// Word sets a single line of 12 px Noto Sans 1.519 em tall (18.228 px)
	// with the baseline 1.124 em (13.488 px) below its top.
	body := wordTestBody(wordTestPara("", wordTestRun("", strings.Repeat("word ", 30))))
	o := notoOpts(t, false)
	p := wordTestPages(t, wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), o)
	l := p.lines(1)
	if len(l) < 2 || math.Abs(l[0].y-(wordTestTop+13.488)) > 0.05 || math.Abs(l[1].y-l[0].y-18.228) > 0.05 {
		t.Errorf("lines %+v", l)
	}
	// At 1.5 lines the baseline stays at the Windows ascent, the extra space
	// below, for every size of the face.
	body = wordTestBody(wordTestPara(`<w:spacing w:line="360" w:lineRule="auto"/>`, wordTestRun("", "a ")+wordTestRun(`<w:sz w:val="36"/>`, "b")))
	p = wordTestPages(t, wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), notoOpts(t, false))
	if l = p.lines(1); len(l) != 1 || math.Abs(l[0].y-(wordTestTop+1.124*24)) > 0.05 {
		t.Errorf("1.5 lines %+v", l)
	}
}

func TestWindowsMetricsLayoutCannotFollowAreApproximated(t *testing.T) {
	// An at-least line puts the lowest text at the bottom by its line
	// metrics, whose descent is not Word's; a fraction placement does not
	// hold for a picture on the line.
	for name, body := range map[string]string{
		"at least":       wordTestPara(`<w:spacing w:line="600" w:lineRule="atLeast"/>`, wordTestRun("", "x")),
		"inline picture": wordTestPara(`<w:spacing w:line="360" w:lineRule="auto"/>`, wordTestRun("", "x"), wordTestPic{w: 20, h: 30}.inline()),
	} {
		doc := wordTestDoc(t, wordTestBody(body), wordTestMedia(wordTestSolid(4, 4, wordTestGreen)))
		if _, err := Prepare(context.Background(), doc, notoOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
			t.Errorf("%s: strict %v", name, err)
		}
		o := notoOpts(t, true)
		wordTestPages(t, doc, o)
		if len(o.warnings) == 0 || !strings.Contains(o.warnings[0].Error(), "line placement") {
			t.Errorf("%s: warnings %v", name, o.warnings)
		}
	}
}
