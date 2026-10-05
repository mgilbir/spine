package docxrender

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/render"
)

// Numbering tests. The page is 300 by 200 pixels with 20 pixel margins; text
// is 12 pixels with 6 pixel glyphs; 15 twips are a pixel, so a 720 twip indent
// is 48 pixels and a 360 twip hanging indent is 24.

// numLvl is a w:lvl.
func numLvl(ilvl int, body string) string {
	return `<w:lvl w:ilvl="` + strconv.Itoa(ilvl) + `">` + body + `</w:lvl>`
}

// numDecimal is a decimal level with the usual hanging indent: text at 48
// pixels, marker at 24.
func numDecimal(ilvl int, text string, extra string) string {
	return numLvl(ilvl, `<w:start w:val="1"/><w:numFmt w:val="decimal"/>`+extra+`<w:lvlText w:val="`+text+`"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>`)
}

func numAbstract(id string, extra string, lvls ...string) string {
	return `<w:abstractNum w:abstractNumId="` + id + `">` + extra + strings.Join(lvls, "") + `</w:abstractNum>`
}

func numInstance(id, abstract string, overrides ...string) string {
	return `<w:num w:numId="` + id + `"><w:abstractNumId w:val="` + abstract + `"/>` + strings.Join(overrides, "") + `</w:num>`
}

func numPr(id string, ilvl int) string {
	return `<w:numPr><w:ilvl w:val="` + strconv.Itoa(ilvl) + `"/><w:numId w:val="` + id + `"/></w:numPr>`
}

func numPara(id string, ilvl int, text string) string {
	return wordTestPara(numPr(id, ilvl), wordTestRun("", text))
}

// numRender prepares a document with the numbering part and returns the page
// lines of page 1.
func numRender(t *testing.T, body, numbering string, lenient bool) (*Pages, *wordTestOpts) {
	t.Helper()
	return numRenderStyles(t, body, wordTestStyles, numbering, lenient)
}

func numRenderStyles(t *testing.T, body, styles, numbering string, lenient bool) (*Pages, *wordTestOpts) {
	t.Helper()
	o := newWordTestOpts(t, lenient)
	doc := wordTestDoc(t, body+wordTestPage, wordTestParts{styles: styles, numbering: numbering})
	return wordTestPages(t, doc, o), o
}

func numLines(p *Pages, page int) []string {
	var out []string
	for _, l := range p.lines(page) {
		out = append(out, l.text)
	}
	return out
}

func equalLines(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("lines = %q, want %q", got, want)
	}
}

func TestNumberedParagraphsHangAtTheIndent(t *testing.T) {
	numbering := numAbstract("0", "", numDecimal(0, "%1.", "")) + numInstance("1", "0")
	p, o := numRender(t, numPara("1", 0, "one")+numPara("1", 0, "two")+wordTestPara("", wordTestRun("", "plain")), numbering, false)
	equalLines(t, numLines(p, 1), []string{"1.one", "2.two", "plain"})
	if len(o.warnings) != 0 {
		t.Errorf("warnings: %v", o.warnings)
	}
	// The marker sits at the hanging position, 20+24, and the text at the
	// left indent, 20+48.
	if x := p.startOf(1, "1.", 0); x != 44 {
		t.Errorf("marker x = %v, want 44", x)
	}
	if x := p.startOf(1, "one", 0); x != 68 {
		t.Errorf("text x = %v, want 68", x)
	}
	if x := p.startOf(1, "plain", 0); x != 20 {
		t.Errorf("plain x = %v, want 20", x)
	}
}

func TestNumberingMultiLevelCounters(t *testing.T) {
	numbering := numAbstract("0", "",
		numDecimal(0, "%1.", ""),
		numLvl(1, `<w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%2)"/><w:pPr><w:ind w:left="1440" w:hanging="360"/></w:pPr>`),
		numLvl(2, `<w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:lvlText w:val="%1.%2.%3"/>`),
	) + numInstance("1", "0")
	body := numPara("1", 0, "A") + numPara("1", 1, "B") + numPara("1", 1, "C") + numPara("1", 2, "D") + numPara("1", 2, "E") +
		numPara("1", 0, "F") + numPara("1", 1, "G") + numPara("1", 2, "H")
	p, _ := numRender(t, body, numbering, false)
	equalLines(t, numLines(p, 1), []string{"1.A", "a)B", "b)C", "1.b.iD", "1.b.iiE", "2.F", "a)G", "2.a.iH"})
}

func TestNumberingStartAndFormats(t *testing.T) {
	numbering := numAbstract("0", "",
		numLvl(0, `<w:start w:val="3"/><w:numFmt w:val="upperRoman"/><w:lvlText w:val="%1"/>`),
		numLvl(1, `<w:start w:val="0"/><w:numFmt w:val="decimalZero"/><w:lvlText w:val="%2"/>`),
		numLvl(2, `<w:start w:val="26"/><w:numFmt w:val="upperLetter"/><w:lvlText w:val="%3"/>`),
	) + numInstance("1", "0")
	body := numPara("1", 0, "x") + numPara("1", 0, "x") + numPara("1", 1, "x") + numPara("1", 1, "x") + numPara("1", 2, "x") + numPara("1", 2, "x") + numPara("1", 2, "x")
	p, o := numRender(t, body, numbering, false)
	// Each marker is followed by its tab.
	equalLines(t, numLines(p, 1), []string{"IIIx", "IVx", "00x", "01x", "Zx", "AAx", "BBx"})
	if len(o.warnings) != 0 {
		t.Errorf("warnings: %v", o.warnings)
	}
}

func TestWordFormatNumber(t *testing.T) {
	tests := []struct {
		format string
		v      int
		want   string
		exact  bool
	}{
		{"decimal", 7, "7", true},
		{"decimalZero", 7, "07", true},
		{"decimalZero", 12, "12", true},
		{"lowerRoman", 4, "iv", true},
		{"upperRoman", 1994, "MCMXCIV", true},
		{"upperRoman", 3999, "MMMCMXCIX", true},
		{"upperRoman", 4000, "4000", false},
		{"lowerRoman", 0, "0", false},
		{"lowerLetter", 1, "a", true},
		{"lowerLetter", 26, "z", true},
		{"lowerLetter", 27, "aa", true},
		{"lowerLetter", 53, "aaa", true},
		{"upperLetter", 28, "BB", true},
		{"upperLetter", 0, "0", false},
		{"chicago", 1, "*", true},
		{"chicago", 5, "**", true},
		{"decimalEnclosedParen", 3, "(3)", true},
		{"decimalEnclosedCircle", 3, "③", true},
		{"decimalEnclosedCircle", 21, "21", false},
		{"decimalFullWidth", 12, "１２", true},
		{"ordinal", 1, "1st", false},
		{"ordinal", 12, "12th", false},
		{"ordinal", 23, "23rd", false},
		{"cardinalText", 0, "Zero", false},
		{"cardinalText", 21, "Twenty-one", false},
		{"cardinalText", 100, "One hundred", false},
		{"cardinalText", 342, "Three hundred and forty-two", false},
		{"cardinalText", 32767, "Thirty-two thousand seven hundred and sixty-seven", false},
		{"ordinalText", 3, "Third", false},
		{"ordinalText", 20, "Twentieth", false},
		{"ordinalText", 21, "Twenty-first", false},
		{"ordinalText", 1000, "One thousandth", false},
		{"none", 5, "", true},
		{"japaneseCounting", 5, "5", false},
	}
	for _, tt := range tests {
		got, exact := wordFormatNumber(tt.format, tt.v)
		if got != tt.want || exact != tt.exact {
			t.Errorf("%s %d = %q (exact %v), want %q (exact %v)", tt.format, tt.v, got, exact, tt.want, tt.exact)
		}
	}
}

func TestNumberingContinuesAcrossContentAndSections(t *testing.T) {
	numbering := numAbstract("0", "", numDecimal(0, "%1.", "")) + numInstance("1", "0")
	body := numPara("1", 0, "a") + wordTestPara("", wordTestRun("", "between")) +
		`<w:p><w:pPr>` + numPr("1", 0) + `<w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300"/></w:sectPr></w:pPr>` + wordTestRun("", "b") + `</w:p>` +
		numPara("1", 0, "c")
	p, _ := numRender(t, body, numbering, false)
	equalLines(t, numLines(p, 1), []string{"1.a", "between", "2.b"})
	equalLines(t, numLines(p, 2), []string{"3.c"})
}

func TestNumberingStartOverrideAndSharedAbstract(t *testing.T) {
	numbering := numAbstract("0", "", numDecimal(0, "%1.", "")) +
		numInstance("1", "0") + numInstance("2", "0") +
		numInstance("3", "0", `<w:lvlOverride w:ilvl="0"><w:startOverride w:val="5"/></w:lvlOverride>`)
	body := numPara("1", 0, "a") + numPara("1", 0, "b") +
		// A second num of the same abstract numbering continues the count.
		numPara("2", 0, "c") +
		// A start override restarts at its first paragraph only.
		numPara("3", 0, "d") + numPara("3", 0, "e") +
		numPara("1", 0, "f")
	p, _ := numRender(t, body, numbering, false)
	equalLines(t, numLines(p, 1), []string{"1.a", "2.b", "3.c", "5.d", "6.e", "7.f"})
}

func TestNumberingLevelOverrideReplacesTheLevel(t *testing.T) {
	numbering := numAbstract("0", "", numDecimal(0, "%1.", "")) + numInstance("1", "0") +
		numInstance("2", "0", `<w:lvlOverride w:ilvl="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="upperLetter"/><w:lvlText w:val="(%1)"/></w:lvl></w:lvlOverride>`)
	p, _ := numRender(t, numPara("1", 0, "a")+numPara("2", 0, "b"), numbering, false)
	equalLines(t, numLines(p, 1), []string{"1.a", "(B)b"})
}

func TestNumberingRestartRules(t *testing.T) {
	level := func(ilvl int, restart string) string {
		return numLvl(ilvl, `<w:start w:val="1"/><w:numFmt w:val="decimal"/>`+restart+`<w:lvlText w:val="%`+strconv.Itoa(ilvl+1)+`"/>`)
	}
	tests := []struct {
		name    string
		restart string
		want    []string
	}{
		{"default", "", []string{"1x", "1x", "2x", "2x", "1x", "2x"}},
		{"never", `<w:lvlRestart w:val="0"/>`, []string{"1x", "1x", "2x", "2x", "3x", "4x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			numbering := numAbstract("0", "", level(0, ""), level(1, tt.restart)) + numInstance("1", "0")
			// Level 0 paragraphs mark the groups: 1, [1 2], 2, [1 2]...
			body := numPara("1", 0, "x") + numPara("1", 1, "x") + numPara("1", 1, "x") + numPara("1", 0, "x") + numPara("1", 1, "x") + numPara("1", 1, "x")
			p, _ := numRender(t, body, numbering, false)
			got := numLines(p, 1)
			// Keep the level-1 counters of paragraphs 2, 3, 5 and 6.
			var level1 []string
			for _, i := range []int{1, 2, 4, 5} {
				level1 = append(level1, got[i])
			}
			want := map[string][]string{
				"default": {"1x", "2x", "1x", "2x"},
				"never":   {"1x", "2x", "3x", "4x"},
			}[tt.name]
			equalLines(t, level1, want)
		})
	}
}

func TestNumberingRestartAfterNamedLevel(t *testing.T) {
	level := func(ilvl int, restart string) string {
		return numLvl(ilvl, `<w:start w:val="1"/><w:numFmt w:val="decimal"/>`+restart+`<w:lvlText w:val="%`+strconv.Itoa(ilvl+1)+`"/>`)
	}
	// Level 3 restarts only after level 1: a level 2 paragraph leaves it.
	numbering := numAbstract("0", "", level(0, ""), level(1, ""), level(2, `<w:lvlRestart w:val="1"/>`)) + numInstance("1", "0")
	body := numPara("1", 0, "x") + numPara("1", 1, "x") + numPara("1", 2, "x") + numPara("1", 1, "x") + numPara("1", 2, "x") + numPara("1", 0, "x") + numPara("1", 1, "x") + numPara("1", 2, "x")
	p, _ := numRender(t, body, numbering, false)
	got := numLines(p, 1)
	equalLines(t, []string{got[2], got[4], got[7]}, []string{"1x", "2x", "1x"})
}

func TestNumberingLegalStyle(t *testing.T) {
	numbering := numAbstract("0", "",
		numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="upperRoman"/><w:lvlText w:val="%1."/>`),
		numLvl(1, `<w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:isLgl/><w:lvlText w:val="%1.%2"/>`),
		numLvl(2, `<w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%1.%2"/>`),
	) + numInstance("1", "0")
	body := numPara("1", 0, "x") + numPara("1", 0, "x") + numPara("1", 1, "x") + numPara("1", 2, "x")
	p, _ := numRender(t, body, numbering, false)
	// Only level 1 is legal: its placeholders are decimal.
	equalLines(t, numLines(p, 1), []string{"I.x", "II.x", "2.1x", "II.ax"})
}

func TestNumberingSuffixes(t *testing.T) {
	lvl := func(suff, jc string) string {
		return numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="decimal"/><w:suff w:val="`+suff+`"/><w:lvlText w:val="%1."/><w:lvlJc w:val="`+jc+`"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>`)
	}
	tests := []struct {
		name, suff, jc string
		markerX, textX float64
	}{
		{"tab", "tab", "left", 44, 68},
		{"space", "space", "left", 44, 44 + 12 + 6},
		{"nothing", "nothing", "left", 44, 44 + 12},
		// The marker ends where it would have started: 44 less its 12 pixels
		// of width, and the tab goes to the indent.
		{"right", "tab", "right", 44 - 12, 68},
		{"center", "tab", "center", 44 - 6, 68},
		{"right with space", "space", "right", 44 - 12, 44 + 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			numbering := numAbstract("0", "", lvl(tt.suff, tt.jc)) + numInstance("1", "0")
			p, o := numRender(t, numPara("1", 0, "text"), numbering, false)
			if x := p.startOf(1, "1.", 0); x != tt.markerX {
				t.Errorf("marker x = %v, want %v", x, tt.markerX)
			}
			if x := p.startOf(1, "text", 0); x != tt.textX {
				t.Errorf("text x = %v, want %v", x, tt.textX)
			}
			if len(o.warnings) != 0 {
				t.Errorf("warnings: %v", o.warnings)
			}
		})
	}
}

func TestNumberTabPassesTheIndentWhenTheMarkerIsWide(t *testing.T) {
	// The marker "10.0.0." is 42 pixels wide, ending at 44+42=86 past the
	// 68 pixel indent, so the tab goes to the next default stop at 96 (the
	// stops are 48 pixels apart from the margin: 20+96=116).
	numbering := numAbstract("0", "", numLvl(0, `<w:start w:val="10"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1.0.0."/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>`)) + numInstance("1", "0")
	p, _ := numRender(t, numPara("1", 0, "text"), numbering, false)
	if x := p.startOf(1, "text", 0); x != 116 {
		t.Errorf("text x = %v, want 116", x)
	}
}

func TestNumberingCustomTabStopInLevel(t *testing.T) {
	numbering := numAbstract("0", "", numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:pPr><w:tabs><w:tab w:val="num" w:pos="1500"/></w:tabs><w:ind w:left="0" w:firstLine="0"/></w:pPr>`)) + numInstance("1", "0")
	p, _ := numRender(t, numPara("1", 0, "text"), numbering, false)
	// 1500 twips is 100 pixels from the margin.
	if x := p.startOf(1, "text", 0); x != 120 {
		t.Errorf("text x = %v, want 120", x)
	}
}

func TestNumberedParagraphThatWrapsIsExact(t *testing.T) {
	numbering := numAbstract("0", "", numDecimal(0, "%1.", "")) + numInstance("1", "0")
	long := strings.Repeat("word ", 30)
	p, o := numRender(t, numPara("1", 0, long), numbering, true)
	if len(o.warnings) != 0 {
		t.Errorf("warnings: %v", o.warnings)
	}
	lines := p.lines(1)
	if len(lines) < 3 {
		t.Fatalf("lines = %v", lines)
	}
	// Continuation lines start at the left indent.
	if lines[1].x != 68 {
		t.Errorf("second line x = %v, want 68", lines[1].x)
	}
}

func TestNumberingMarkerFormatting(t *testing.T) {
	numbering := numAbstract("0", "", numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr><w:rPr><w:sz w:val="36"/><w:color w:val="FF0000"/></w:rPr>`)) + numInstance("1", "0")
	p, _ := numRender(t, numPara("1", 0, "text"), numbering, false)
	var marker, text wordTestText
	for _, tx := range p.texts(1) {
		switch tx.text {
		case "1.":
			marker = tx
		case "text":
			text = tx
		}
	}
	if marker.size != 24 || text.size != 12 {
		t.Errorf("sizes marker %v text %v, want 24 and 12", marker.size, text.size)
	}
}

func TestNumberingThroughStyles(t *testing.T) {
	styles := wordTestStyles +
		`<w:style w:type="paragraph" w:styleId="List"><w:name w:val="List"/><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Sub"><w:name w:val="Sub"/><w:basedOn w:val="List"/></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Head2"><w:name w:val="Head2"/><w:pPr><w:numPr><w:numId w:val="2"/></w:numPr></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Own"><w:name w:val="Own"/><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr><w:ind w:left="1500" w:hanging="300"/></w:pPr></w:style>`
	numbering := numAbstract("0", "", numDecimal(0, "%1.", ""), numDecimal(1, "%1.%2", "")) +
		numAbstract("1", "", numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="upperLetter"/><w:lvlText w:val="%1)"/>`), numLvl(1, `<w:start w:val="1"/><w:numFmt w:val="decimal"/><w:pStyle w:val="Head2"/><w:lvlText w:val="%1.%2"/>`)) +
		numInstance("1", "0") + numInstance("2", "1")
	para := func(style, extra, text string) string {
		return wordTestPara(`<w:pStyle w:val="`+style+`"/>`+extra, wordTestRun("", text))
	}
	body := para("List", "", "a") + para("Sub", "", "b") +
		para("List", numPr("1", 1), "c") +
		// numId 0 removes the style's numbering.
		para("List", `<w:numPr><w:numId w:val="0"/></w:numPr>`, "plain") +
		// A style that is linked to level 1 by the level's pStyle draws there.
		para("Head2", "", "link") + para("Head2", "", "link") +
		// The style's own indent wins over the level's.
		para("Own", "", "own") +
		// A direct indent wins over everything.
		para("List", `<w:ind w:left="300" w:firstLine="0"/>`, "direct")
	p, _ := numRenderStyles(t, body, styles, numbering, false)
	// Head2 comes through numId 2, level 1, so the abstract numbering of
	// numId 2 counts level 1 without a level 0 before it (which counts from its
	// start).
	equalLines(t, numLines(p, 1), []string{"1.a", "2.b", "2.1c", "plain", "A.1link", "A.2link", "3.own", "4.direct"})
	if x := p.startOf(1, "own", 0); x != 20+100 {
		t.Errorf("own text x = %v, want 120", x)
	}
	if x := p.startOf(1, "3.", 0); x != 20+100-20 {
		t.Errorf("own marker x = %v, want 100", x)
	}
	// 300 twips is 20 pixels; the level's hanging indent is replaced by the
	// firstLine of 0, so the marker is at the indent and the text follows the
	// next default stop.
	if x := p.startOf(1, "4.", 0); x != 40 {
		t.Errorf("direct marker x = %v, want 40", x)
	}
}

func TestNumberingStyleLink(t *testing.T) {
	styles := wordTestStyles +
		`<w:style w:type="numbering" w:styleId="ListStyle"><w:name w:val="ListStyle"/><w:pPr><w:numPr><w:numId w:val="2"/></w:numPr></w:pPr></w:style>`
	numbering := numAbstract("0", `<w:numStyleLink w:val="ListStyle"/>`) +
		numAbstract("1", `<w:styleLink w:val="ListStyle"/>`, numDecimal(0, "[%1]", "")) +
		numInstance("1", "0") + numInstance("2", "1")
	o := newWordTestOpts(t, false)
	doc := wordTestDoc(t, numPara("1", 0, "a")+numPara("2", 0, "b")+numPara("1", 0, "c")+wordTestPage, wordTestParts{styles: styles, numbering: numbering})
	p := wordTestPages(t, doc, o)
	equalLines(t, numLines(p, 1), []string{"[1]a", "[2]b", "[3]c"})
}

func TestNumberingStyleLinkLoopIsRefused(t *testing.T) {
	styles := wordTestStyles +
		`<w:style w:type="numbering" w:styleId="ListStyle"><w:name w:val="ListStyle"/><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr></w:style>`
	numbering := numAbstract("0", `<w:numStyleLink w:val="ListStyle"/>`) + numInstance("1", "0")
	doc := wordTestDoc(t, numPara("1", 0, "a")+wordTestPage, wordTestParts{styles: styles, numbering: numbering})
	_, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options)
	if !errors.Is(err, render.ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestBulletsAndFonts(t *testing.T) {
	bullet := func(text, font string) string {
		return numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="`+text+`"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr><w:rPr><w:rFonts w:ascii="`+font+`" w:hAnsi="`+font+`"/></w:rPr>`)
	}
	t.Run("text bullet", func(t *testing.T) {
		numbering := numAbstract("0", "", bullet("o", "Courier New")) + numInstance("1", "0")
		p, o := numRender(t, numPara("1", 0, "x")+numPara("1", 0, "y"), numbering, false)
		equalLines(t, numLines(p, 1), []string{"ox", "oy"})
		if len(o.warnings) != 0 {
			t.Errorf("warnings: %v", o.warnings)
		}
		// Bullets do not count: the font is requested, though.
		found := false
		for _, r := range o.requests {
			found = found || r.Family == "Courier New"
		}
		if !found {
			t.Errorf("requests %v lack the bullet font", o.requests)
		}
	})
	t.Run("symbol bullet strict", func(t *testing.T) {
		numbering := numAbstract("0", "", bullet("", "Symbol")) + numInstance("1", "0")
		doc := wordTestDoc(t, numPara("1", 0, "x")+wordTestPage, wordTestParts{styles: wordTestStyles, numbering: numbering})
		_, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options)
		if !errors.Is(err, render.ErrUnsupported) {
			t.Errorf("err = %v, want ErrUnsupported", err)
		}
	})
	t.Run("symbol bullet best effort", func(t *testing.T) {
		numbering := numAbstract("0", "", bullet("", "Symbol")) + numInstance("1", "0")
		p, o := numRender(t, numPara("1", 0, "x"), numbering, true)
		var approx bool
		for _, w := range o.warnings {
			approx = approx || (errors.Is(w, render.ErrApproximated) && strings.Contains(w.Error(), "symbol font bullet"))
		}
		if !approx {
			t.Errorf("warnings %v lack the symbol bullet approximation", o.warnings)
		}
		if x := p.startOf(1, "x", 0); x != 68 {
			t.Errorf("text x = %v, want 68", x)
		}
	})
	t.Run("symbol bullet drawn from a font that has it", func(t *testing.T) {
		// A host that resolves Symbol to a face with the private-use glyph
		// draws the bullet as Word does: exact, in strict mode too.
		numbering := numAbstract("0", "", bullet("", "Symbol")) + numInstance("1", "0")
		o := newWordTestOpts(t, false)
		face := wordTestFaceWith(t, 0xF0B7)
		base := o.Fonts
		o.Fonts = func(ctx context.Context, r render.FontRequest) (*shape.Face, error) {
			if r.Family == "Symbol" {
				return face, nil
			}
			return base(ctx, r)
		}
		doc := wordTestDoc(t, numPara("1", 0, "x")+wordTestPage, wordTestParts{styles: wordTestStyles, numbering: numbering})
		p := wordTestPages(t, doc, o)
		if x := p.startOf(1, "", 0); x != 44 {
			t.Errorf("bullet x = %v (texts %v), want 44", x, p.texts(1))
		}
	})
}

func TestPictureBulletIsApproximated(t *testing.T) {
	numbering := `<w:numPicBullet w:numPicBulletId="0"><w:pict/></w:numPicBullet>` +
		numAbstract("0", "", numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val=""/><w:lvlPicBulletId w:val="0"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>`)) + numInstance("1", "0")
	doc := wordTestDoc(t, numPara("1", 0, "x")+wordTestPage, wordTestParts{styles: wordTestStyles, numbering: numbering})
	if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
		t.Errorf("strict err = %v, want ErrUnsupported", err)
	}
	o := newWordTestOpts(t, true)
	wordTestPages(t, doc, o)
	var found bool
	for _, w := range o.warnings {
		found = found || (errors.Is(w, render.ErrApproximated) && strings.Contains(w.Error(), "picture bullet"))
	}
	if !found {
		t.Errorf("warnings %v lack the picture bullet", o.warnings)
	}
}

func TestNumberFormatsOutsideTheProfile(t *testing.T) {
	numbering := numAbstract("0", "", numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="japaneseCounting"/><w:lvlText w:val="%1."/>`)) + numInstance("1", "0")
	doc := wordTestDoc(t, numPara("1", 0, "x")+wordTestPage, wordTestParts{styles: wordTestStyles, numbering: numbering})
	if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
		t.Errorf("strict err = %v, want ErrUnsupported", err)
	}
	o := newWordTestOpts(t, true)
	p := wordTestPages(t, doc, o)
	equalLines(t, numLines(p, 1), []string{"1.x"})
	var found bool
	for _, w := range o.warnings {
		found = found || (errors.Is(w, render.ErrApproximated) && strings.Contains(w.Error(), "number format japaneseCounting"))
	}
	if !found {
		t.Errorf("warnings %v lack the format", o.warnings)
	}
}

func TestUndefinedListsDrawWithoutNumbering(t *testing.T) {
	// Word draws a paragraph whose list is not defined without a marker.
	for name, numbering := range map[string]string{
		"no part":               "",
		"no num":                numAbstract("0", "", numDecimal(0, "%1.", "")),
		"no abstract":           numInstance("1", "9"),
		"level not defined":     numAbstract("0", "", numDecimal(1, "%2.", "")) + numInstance("1", "0"),
		"numbering part absent": "",
	} {
		t.Run(name, func(t *testing.T) {
			p, o := numRender(t, numPara("1", 0, "x"), numbering, false)
			equalLines(t, numLines(p, 1), []string{"x"})
			if len(o.warnings) != 0 {
				t.Errorf("warnings: %v", o.warnings)
			}
		})
	}
}

func TestNumberingInvalidSources(t *testing.T) {
	tests := []struct {
		name      string
		numbering string
		body      string
		want      error
	}{
		{"ilvl beyond nine levels", numAbstract("0", "", numDecimal(0, "%1.", "")) + numInstance("1", "0"), numPara("1", 9, "x"), render.ErrInvalid},
		{"level out of range", numAbstract("0", "", numLvl(9, `<w:start w:val="1"/>`)) + numInstance("1", "0"), numPara("1", 0, "x"), render.ErrInvalid},
		{"duplicate level", numAbstract("0", "", numDecimal(0, "%1.", ""), numDecimal(0, "%1.", "")) + numInstance("1", "0"), numPara("1", 0, "x"), render.ErrInvalid},
		{"duplicate abstract", numAbstract("0", "") + numAbstract("0", "") + numInstance("1", "0"), numPara("1", 0, "x"), render.ErrInvalid},
		{"duplicate num", numAbstract("0", "") + numInstance("1", "0") + numInstance("1", "0"), numPara("1", 0, "x"), render.ErrInvalid},
		{"negative start", numAbstract("0", "", numLvl(0, `<w:start w:val="-1"/>`)) + numInstance("1", "0"), numPara("1", 0, "x"), render.ErrInvalid},
		{"huge start", numAbstract("0", "", numLvl(0, `<w:start w:val="40000"/>`)) + numInstance("1", "0"), numPara("1", 0, "x"), render.ErrInvalid},
		{"bad suffix", numAbstract("0", "", numLvl(0, `<w:suff w:val="always"/>`)) + numInstance("1", "0"), numPara("1", 0, "x"), render.ErrInvalid},
		{"lvlText too long", numAbstract("0", "", numLvl(0, `<w:lvlText w:val="`+strings.Repeat("a", 300)+`"/>`)) + numInstance("1", "0"), numPara("1", 0, "x"), render.ErrInvalid},
		{"counter limit", numAbstract("0", "", numLvl(0, `<w:start w:val="32767"/><w:lvlText w:val="%1"/>`)) + numInstance("1", "0"), numPara("1", 0, "x") + numPara("1", 0, "y"), render.ErrLimit},
		{"override level mismatch", numAbstract("0", "", numDecimal(0, "%1.", "")) + numInstance("1", "0", `<w:lvlOverride w:ilvl="0"><w:lvl w:ilvl="1"/></w:lvlOverride>`), numPara("1", 0, "x"), render.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, lenient := range []bool{false, true} {
				o := newWordTestOpts(t, lenient)
				doc := wordTestDoc(t, tt.body+wordTestPage, wordTestParts{styles: wordTestStyles, numbering: tt.numbering})
				if _, err := Prepare(context.Background(), doc, o.Options); !errors.Is(err, tt.want) {
					t.Errorf("lenient %v: err = %v, want %v", lenient, err, tt.want)
				}
			}
		})
	}
}

func TestNumberingTextBudget(t *testing.T) {
	// A long list of letters-formatted markers is bounded by the text budget
	// rather than growing without limit.
	numbering := numAbstract("0", "", numLvl(0, `<w:start w:val="32000"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="`+strings.Repeat("%1", 100)+`"/>`)) + numInstance("1", "0")
	body := strings.Repeat(numPara("1", 0, "x"), 20)
	o := newWordTestOpts(t, false)
	o.Limits.MaxTextBytes = 4096
	doc := wordTestDoc(t, body+wordTestPage, wordTestParts{styles: wordTestStyles, numbering: numbering})
	if _, err := Prepare(context.Background(), doc, o.Options); !errors.Is(err, render.ErrLimit) {
		t.Errorf("err = %v, want ErrLimit", err)
	}
}

func TestNumberingFromDocumentAPI(t *testing.T) {
	// Lists made with the document API, with unsaved edits, are drawn.
	doc := docx.Create()
	list := doc.Numbering().AddDefinition()
	list.SetLevel(0, docx.NumberFormatDecimal, "%1.")
	first, second := list.ListStyle(), list.RestartedListStyle(0, 1)
	for _, step := range []struct {
		style *docx.ListStyle
		text  string
	}{{first, "a"}, {first, "b"}, {second, "c"}} {
		para := doc.AddParagraph()
		para.AddRun().SetText(step.text)
		para.SetListStyle(step.style, 0)
	}
	// A new document has no page size, which best effort draws on Letter.
	o := newWordTestOpts(t, true)
	pages, err := Prepare(context.Background(), doc, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range pages.lines(1) {
		got = append(got, l.text)
	}
	equalLines(t, got, []string{"1.a", "2.b", "1.c"})
}

// wordTestFaceWith is the fixture face with glyphs for extra characters.
func wordTestFaceWith(t testing.TB, extra ...rune) *shape.Face {
	t.Helper()
	var glyphs []fonttest.Glyph
	for c := rune(0x21); c <= 0x7E; c++ {
		glyphs = append(glyphs, fonttest.Glyph{Rune: c, Advance: 1000, HasShape: true, Ink: [4]int{100, 0, 900, 1400}})
	}
	glyphs = append(glyphs, fonttest.Glyph{Rune: ' ', Advance: 1000})
	for _, c := range extra {
		glyphs = append(glyphs, fonttest.Glyph{Rune: c, Advance: 1000, HasShape: true, Ink: [4]int{100, 0, 900, 1400}})
	}
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{UnitsPerEm: 2000, Ascent: 1600, Descent: -400, Glyphs: glyphs}))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestNumberingWithoutMarkerStillHangs(t *testing.T) {
	// A level without a number holds the text at the indent, which is how
	// continuation paragraphs line up under a numbered one.
	numbering := numAbstract("0", "", numLvl(0, `<w:start w:val="1"/><w:numFmt w:val="none"/><w:lvlText w:val=""/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>`)) + numInstance("1", "0")
	p, o := numRender(t, numPara("1", 0, "text"), numbering, false)
	equalLines(t, numLines(p, 1), []string{"text"})
	if x := p.startOf(1, "text", 0); x != 68 {
		t.Errorf("text x = %v, want 68", x)
	}
	if len(o.warnings) != 0 {
		t.Errorf("warnings: %v", o.warnings)
	}
}

func TestNumberedParagraphWithLaterTabs(t *testing.T) {
	// A tab in the text is measured after the marker, from the same pen.
	numbering := numAbstract("0", "", numDecimal(0, "%1.", "")) + numInstance("1", "0")
	body := wordTestPara(numPr("1", 0)+tabStops(tabStop("left", 150, "")), `<w:r><w:t>ab</w:t><w:tab/><w:t>cd</w:t></w:r>`)
	p, _ := numRender(t, body, numbering, false)
	if x := p.startOf(1, "ab", 0); x != 68 {
		t.Errorf("text x = %v, want 68", x)
	}
	if x := p.startOf(1, "cd", 0); x != 20+150 {
		t.Errorf("tabbed text x = %v, want 170", x)
	}
}
