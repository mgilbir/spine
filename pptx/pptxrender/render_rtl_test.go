package pptxrender

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// renderRTLFonts serves fixture fonts that tell right-to-left letters apart
// by their ink: of a one-em letter, alef fills the box, bet its lower half
// and gimel its upper half. Arabic beh has the four forms the shaper picks
// between in a font with no rules of its own, each with its own ink: isolated
// full, initial lower, final upper, medial the middle band. The "Latin" font
// has the letters, the bullet and a space, and "Hebrew" and "Arabic" are the
// families of the complex-script slot.
func renderRTLFonts(t testing.TB) (render.Options, *[]string) {
	t.Helper()
	letter := func(r rune, ink [4]int) fonttest.Glyph {
		return fonttest.Glyph{Rune: r, Advance: 1000, HasShape: true, Ink: ink}
	}
	load := func(glyphs ...fonttest.Glyph) *shape.Face {
		face, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: append(glyphs, fonttest.Glyph{Rune: ' ', Advance: 500})}))
		if err != nil {
			t.Fatal(err)
		}
		return face
	}
	full, lower, upper, middle := [4]int{0, 0, 1000, 800}, [4]int{0, 0, 1000, 400}, [4]int{0, 400, 1000, 800}, [4]int{0, 200, 1000, 600}
	faces := map[string]*shape.Face{
		"Latin":  load(letter('A', full), letter('B', lower), letter('•', full), letter('日', lower), letter('本', lower), letter('語', lower), letter('。', lower), letter('1', full), letter('2', upper)),
		"Hebrew": load(letter('א', full), letter('ב', lower), letter('ג', upper), letter('ְ', middle)),
		"Arabic": load(letter('ب', full), letter('ﺑ', lower), letter('ﺒ', middle), letter('ﺐ', upper), letter('ﺏ', full)),
		"Cjk":    load(letter('日', lower), letter('本', lower), letter('語', lower), letter('。', lower)),
	}
	var asked []string
	return render.Options{Fonts: func(_ context.Context, r render.FontRequest) (*shape.Face, error) {
		asked = append(asked, r.Family)
		if r.Family == "Aptos" {
			r.Family = "Latin"
		}
		return faces[r.Family], nil
	}}, &asked
}

func renderRTLRun(text string) string {
	return `<a:r><a:rPr sz="1100"><a:latin typeface="Latin"/><a:cs typeface="Hebrew"/><a:ea typeface="Cjk"/></a:rPr><a:t>` + text + `</a:t></a:r>`
}

func TestRenderRightToLeftParagraphs(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, asked := renderRTLFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	// 11pt is 14.67px: the box is 48px wide from x 4, so three letters take
	// 44px. The line's baseline is at y 15.7; upper ink is y 4 to 9.9 and
	// lower ink 9.9 to 15.7.
	slot := func(n int, left bool) (x0, x1 int) {
		x := 4
		if !left {
			x = 8
		}
		return x + n*14 + 2, x + n*14 + 12
	}
	// shape reads the three slots of one line as: A full, L lower, U upper.
	read := func(img []byte, left bool) string {
		var out strings.Builder
		for n := 0; n < 3; n++ {
			x0, x1 := slot(n, left)
			up, low := renderInk(t, img, x0, 5, x1, 8, black), renderInk(t, img, x0, 12, x1, 15, black)
			switch {
			case up && low:
				out.WriteByte('A')
			case low:
				out.WriteByte('L')
			case up:
				out.WriteByte('U')
			default:
				out.WriteByte('.')
			}
		}
		return out.String()
	}
	// Hebrew in a left-to-right paragraph runs right to left: alef, bet and
	// gimel are drawn gimel first, and the line starts at the left. The font
	// is the run's complex-script font, and no other is asked for.
	got := renderSlidePNG(t, data, opts, renderScriptSlide(renderRTLRun("אבג"), ""))
	if r := read(got, true); r != "UL"+"A" {
		t.Fatalf("hebrew: %s", r)
	}
	if len(*asked) != 1 || (*asked)[0] != "Hebrew" {
		t.Fatalf("fonts asked for: %q", *asked)
	}
	// Alignment is physical, as PowerPoint draws it, in a right-to-left
	// paragraph too: with no `algn` and with `l` the line is at the left edge,
	// `r` puts it at the right and `ctr` between.
	for pPr, want := range map[string][3]int{
		`<a:pPr rtl="1"/>`:            {4, 32, 0},
		`<a:pPr algn="l" rtl="1"/>`:   {4, 32, 0},
		`<a:pPr algn="ctr" rtl="1"/>`: {13, 42, 0},
		`<a:pPr algn="r" rtl="1"/>`:   {23, 51, 0},
		`<a:pPr algn="l" rtl="0"/>`:   {4, 32, 0},
		`<a:pPr algn="r" rtl="0"/>`:   {23, 51, 0},
		`<a:pPr algn="ctr" rtl="0"/>`: {13, 42, 0},
	} {
		img := renderSlidePNG(t, data, opts, renderScriptSlide(renderRTLRun("אב"), pPr))
		if l, _, r, _ := renderInkBounds(t, img); l != want[0] || r != want[1] {
			t.Errorf("%s: ink spans x %d to %d, want %d to %d", pPr, l, r, want[0], want[1])
		}
	}
	rtl := renderSlidePNG(t, data, opts, renderScriptSlide(renderRTLRun("אבג"), `<a:pPr rtl="1"/>`))
	if r := read(rtl, true); r != "ULA" {
		t.Fatalf("rtl paragraph: %s", r)
	}
	if got := renderSlidePNG(t, data, opts, renderScriptSlide(renderRTLRun("אבג"), `<a:pPr algn="r" rtl="1"/>`)); read(got, false) != "ULA" {
		t.Fatal("right alignment")
	}
	// Latin text in a right-to-left paragraph reads left to right, and sits
	// at the left, where an absent `algn` puts it.
	latin := renderSlidePNG(t, data, opts, renderScriptSlide(renderRTLRun("AB"), `<a:pPr rtl="1"/>`))
	if !renderInk(t, latin, 5, 5, 18, 8, black) || renderInk(t, latin, 36, 5, 52, 15, black) || !renderInk(t, latin, 20, 12, 32, 15, black) || renderInk(t, latin, 20, 5, 32, 8, black) {
		t.Fatal("Latin text in a right-to-left paragraph")
	}
	// Without the paragraph's direction the same Latin text is at the left.
	if left := renderSlidePNG(t, data, opts, renderScriptSlide(renderRTLRun("AB"), "")); !renderInk(t, left, 5, 5, 18, 8, black) {
		t.Fatal("Latin text")
	}
	// Centered text is centered in either direction.
	ctr := renderSlidePNG(t, data, opts, renderScriptSlide(renderRTLRun("אב"), `<a:pPr algn="ctr" rtl="1"/>`))
	if !renderInk(t, ctr, 14, 5, 20, 8, black) && !renderInk(t, ctr, 14, 12, 20, 15, black) {
		t.Fatal("centered")
	}
	// Mixed directions: the Hebrew stays in order inside the Latin, and a
	// space ending a line takes the paragraph's direction.
	mixed := renderSlidePNG(t, data, opts, renderScriptSlide(renderRTLRun("Aאב "), ""))
	if !renderInk(t, mixed, 5, 5, 18, 8, black) || !renderInk(t, mixed, 20, 12, 32, 15, black) || renderInk(t, mixed, 20, 5, 32, 8, black) || !renderInk(t, mixed, 34, 5, 46, 8, black) {
		t.Fatal("mixed directions")
	}
}

func TestRenderRightToLeftArabicShaping(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, asked := renderRTLFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	run := func(text string) map[string]func(string) string {
		return renderScriptSlide(`<a:r><a:rPr lang="ar-SA" sz="1100"><a:latin typeface="Latin"/><a:cs typeface="Arabic"/></a:rPr><a:t>`+text+`</a:t></a:r>`, "")
	}
	// Beh before beh takes its initial form, then its final one; the
	// letters are drawn final first.
	got := renderSlidePNG(t, data, opts, run("بب"))
	if !renderInk(t, got, 6, 5, 16, 8, black) || renderInk(t, got, 6, 12, 16, 15, black) || !renderInk(t, got, 20, 12, 30, 15, black) || renderInk(t, got, 20, 5, 30, 8, black) {
		t.Fatal("joined forms")
	}
	if len(*asked) != 1 || (*asked)[0] != "Arabic" {
		t.Fatalf("fonts asked for: %q", *asked)
	}
	// Runs of different color inside a word shape together.
	*asked = nil
	split := renderScriptSlide(`<a:r><a:rPr sz="1100"><a:latin typeface="Latin"/><a:cs typeface="Arabic"/></a:rPr><a:t>`+"ب"+`</a:t></a:r><a:r><a:rPr sz="1100"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:latin typeface="Latin"/><a:cs typeface="Arabic"/></a:rPr><a:t>`+"ب"+`</a:t></a:r>`, "")
	img := renderSlidePNG(t, data, opts, split)
	red := func(r, g, b uint8) bool { return r > 128 && g < 100 && b < 100 }
	// The first letter is initial (lower ink) and the second final (upper
	// ink, red), drawn second letter first.
	if !renderInk(t, img, 6, 5, 16, 8, red) || !renderInk(t, img, 20, 12, 30, 15, black) {
		t.Fatal("joined forms across a change of color")
	}
}

func TestRenderRightToLeftFontsAndFailures(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, _ := renderRTLFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	run := func(lang, text string) map[string]func(string) string {
		return renderScriptSlide(`<a:r><a:rPr lang="`+lang+`" sz="1100"><a:latin typeface="Latin"/></a:rPr><a:t>`+text+`</a:t></a:r>`, "")
	}
	// The default style names the theme's complex-script font, which the
	// theme leaves empty.
	if _, err := renderRewrittenPNG(t, data, opts, run("he-IL", "אב")); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("no complex-script font: %v", err)
	}
	var warnings []string
	best := opts
	best.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	if _, err := renderRewrittenPNG(t, data, best, run("he-IL", "אב")); err != nil || !strings.Contains(strings.Join(warnings, "\n"), "complex-script text without a complex-script font") {
		t.Fatalf("best effort: %v %q", err, warnings)
	}
	// The theme's font list gives a font by the run's language, or by the
	// script of the text where the language names none.
	list := regexp.MustCompile(`(?s)(<a:minorFont>.*?)(</a:minorFont>)`)
	themed := renderApply(t, data, map[string]func(string) string{"ppt/theme/theme1.xml": func(s string) string {
		return list.ReplaceAllString(s, `${1}<a:font script="Hebr" typeface="Hebrew"/><a:font script="Arab" typeface="Arabic"/>${2}`)
	}})
	for name, tc := range map[string]struct{ lang, text string }{
		"hebrew by language": {"he-IL", "א"}, "hebrew by script": {"en-US", "א"},
		"arabic by language": {"ar-EG", "ب"}, "arabic by script": {"en-US", "ب"},
	} {
		got, err := renderRewrittenPNG(t, themed, opts, run(tc.lang, tc.text))
		if err != nil || !renderInk(t, got, 6, 5, 16, 15, black) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// A character of a script the list does not name has no font.
	if _, err := renderRewrittenPNG(t, themed, opts, run("en-US", "ܐ")); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("syriac: %v", err)
	}

	// Tabs, a first-line indent without a bullet and Latin breaking are not
	// drawn in right-to-left paragraphs either.
	for name, rewrite := range map[string]map[string]func(string) string{
		"tab in a right-to-left paragraph": renderScriptSlide(renderRTLRun("A\tB"), `<a:pPr rtl="1"/>`),
		"tab beside Hebrew":                renderScriptSlide(renderRTLRun("א\tB"), ""),
		"run direction":                    renderScriptSlide(`<a:r><a:rPr sz="1100"><a:latin typeface="Latin"/><a:rtl/></a:rPr><a:t>A</a:t></a:r>`, ""),
		"Latin line breaking":              renderScriptSlide(renderRTLRun("A"), `<a:pPr latinLnBrk="1"/>`),
	} {
		if _, err := renderRewrittenPNG(t, data, opts, rewrite); !errors.Is(err, render.ErrUnsupported) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Left-to-right paragraphs of Latin text with tabs still draw.
	if _, err := renderRewrittenPNG(t, data, opts, renderScriptSlide(renderRTLRun("A\tB"), `<a:pPr defTabSz="228600"/>`)); err != nil {
		t.Fatalf("Latin tab: %v", err)
	}
	// Direction marks and joiners set among right-to-left text draw nothing
	// and keep the text's font; other format characters still fail.
	if _, err := renderRewrittenPNG(t, data, opts, renderScriptSlide(renderRTLRun("א\u200fב\u200c"), "")); err != nil {
		t.Fatalf("marks: %v", err)
	}
	if _, err := renderRewrittenPNG(t, data, opts, renderScriptSlide(renderRTLRun("א\u202eב"), "")); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("override: %v", err)
	}
	// A right-to-left paragraph does not hang East Asian punctuation, which
	// wraps with its character as without hanging.
	hanging := renderScriptSlide(renderRTLRun("日本語。"), `<a:pPr rtl="1"/>`)
	if got, err := renderRewrittenPNG(t, data, opts, hanging); err != nil || !renderInk(t, got, 4, 25, 52, 30, black) {
		t.Fatalf("East Asian punctuation in a right-to-left paragraph: %v", err)
	}
}

func TestRenderRightToLeftBulletsMarginsAndJustify(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, _ := renderRTLFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	// The shape is widened to 72px, x 4 to 76, so that a 0.5" (48px) margin
	// leaves room: the lines span x 4 to 28, the text is two 7pt letters of
	// 9.33px, and the 0.25" (24px) hanging indent puts the bullet's start 24px
	// right of the text's start, so that the bullet sits right of its own line's
	// text, with the hanging gap between them, wherever the text is.
	wide := func(lstStyle string) map[string]func(string) string {
		body := renderBody(lstStyle, `<a:p>`+`<a:r><a:rPr sz="700"><a:latin typeface="Latin"/><a:cs typeface="Hebrew"/></a:rPr><a:t>אב</a:t></a:r>`+`</a:p>`)
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return strings.Replace(body(s), `cx="457200"`, `cx="685800"`, 1)
		}}
	}
	// spans lists the runs of columns with ink, as x to x.
	spans := func(img []byte) [][2]int {
		var out [][2]int
		for x := 0; x < 80; x++ {
			if !renderInk(t, img, x, 0, x+1, 30, black) {
				continue
			}
			if n := len(out); n > 0 && out[n-1][1] == x-1 {
				out[n-1][1] = x
			} else {
				out = append(out, [2]int{x, x})
			}
		}
		return out
	}
	list := `<a:lstStyle><a:lvl1pPr marL="457200" indent="-228600" rtl="1" %s><a:buFont typeface="Latin"/><a:buChar char="•"/></a:lvl1pPr></a:lstStyle>`
	for algn, want := range map[string][][2]int{
		// The text against the left edge, the bullet 24px right of its end.
		"":           {{4, 22}, {37, 46}},
		`algn="l"`:   {{4, 22}, {37, 46}},
		`algn="ctr"`: {{7, 24}, {40, 48}},
		// The text against the right margin, 48px in, the bullet 24px in.
		`algn="r"`: {{9, 27}, {43, 51}},
	} {
		got := spans(renderSlidePNG(t, data, opts, wide(fmt.Sprintf(list, algn))))
		if !slices.Equal(got, want) {
			t.Errorf("bullet with %q: ink at %v, want %v", algn, got, want)
		}
	}
	// Margins are from the start: marL is the right margin, and marR the left.
	margin := `<a:lstStyle><a:lvl1pPr marL="228600" marR="96000" algn="%s" rtl="1"/></a:lstStyle>`
	for algn, want := range map[string][][2]int{"l": {{14, 32}}, "r": {{34, 52}}} {
		got := spans(renderSlidePNG(t, data, opts, wide(fmt.Sprintf(margin, algn))))
		if len(got) != 1 || got[0][0] < want[0][0]-1 || got[0][1] > want[0][1]+1 {
			t.Errorf("margins with algn %q: ink at %v, want about %v", algn, got, want)
		}
	}

	// Justified lines stretch their spaces across the box from the start; the
	// space ending the line hangs at its end. 7pt letters are 9.33px, and the
	// first line, "אב אב ", is 42px of text.
	justified := renderScriptSlide(`<a:r><a:rPr sz="700"><a:latin typeface="Latin"/><a:cs typeface="Hebrew"/></a:rPr><a:t>אב אב אב</a:t></a:r>`, `<a:pPr algn="just" rtl="1"/>`)
	got := renderSlidePNG(t, data, opts, justified)
	left, _, right, _ := renderInkBounds(t, got)
	if left < 4 || left > 5 || right < 50 || right > 52 {
		t.Fatalf("justified line spans x %d to %d", left, right)
	}
	// The last line sits at the right, the logical start, as PowerPoint draws
	// it, and not at the left as left alignment would put it.
	if !renderInk(t, got, 35, 17, 52, 26, black) || renderInk(t, got, 4, 17, 30, 26, black) {
		t.Fatal("last line of a justified right-to-left paragraph")
	}
	// Low kashida elongates Arabic, which this profile does not.
	kashida := renderScriptSlide(`<a:r><a:rPr sz="700"><a:latin typeface="Latin"/><a:cs typeface="Arabic"/></a:rPr><a:t>`+"بب بب بب"+`</a:t></a:r>`, `<a:pPr algn="justLow" rtl="1"/>`)
	if _, err := renderRewrittenPNG(t, data, opts, kashida); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("low kashida: %v", err)
	}
	var warnings []string
	best := opts
	best.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	if _, err := renderRewrittenPNG(t, data, best, kashida); err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "kashida") {
		t.Fatalf("best effort: %v %q", err, warnings)
	}
	// Hebrew has no kashida.
	if _, err := renderRewrittenPNG(t, data, opts, renderScriptSlide(renderRTLRun("אב אב"), `<a:pPr algn="justLow"/>`)); err != nil {
		t.Fatalf("justLow without Arabic: %v", err)
	}
}
