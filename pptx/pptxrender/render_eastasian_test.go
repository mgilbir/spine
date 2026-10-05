package pptxrender

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// renderScriptFace builds a 1000-unit font whose glyphs for the given
// characters are one em wide, with ink from the baseline up to height units,
// so that the height tells fonts apart.
func renderScriptFace(t testing.TB, height int, chars string) *shape.Face {
	t.Helper()
	glyphs := []fonttest.Glyph{{Rune: ' ', Advance: 500}}
	for _, c := range chars {
		glyphs = append(glyphs, fonttest.Glyph{Rune: c, Advance: 1000, HasShape: true, Ink: [4]int{0, 0, 1000, height}})
	}
	face, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: glyphs}))
	if err != nil {
		t.Fatal(err)
	}
	return face
}

// renderScriptFonts serves fixture fonts by family and records the families
// asked for. Latin ink is 800 units high, the East Asian fonts' 400, 250, 550
// and 700.
func renderScriptFonts(t testing.TB) (render.Options, *[]string) {
	t.Helper()
	const cjk = "日本語、。「」ＡＢあア한"
	faces := map[string]*shape.Face{
		"Latin":    renderScriptFace(t, 800, "AB23×é"),
		"Cjk":      renderScriptFace(t, 400, cjk),
		"CjkJa":    renderScriptFace(t, 250, cjk),
		"CjkZh":    renderScriptFace(t, 550, cjk),
		"CjkTheme": renderScriptFace(t, 700, cjk),
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

func renderScriptSlide(runs, pPr string) map[string]func(string) string {
	return map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, `<a:p>`+pPr+runs+`</a:p>`)}
}

func TestRenderEastAsianFontsByClass(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, asked := renderScriptFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	// 11pt is 14.67px: the line's baseline is at y 15.7, Latin ink fills y 4
	// to 15.7 and East Asian ink the 5.9px above the baseline.
	const run = `<a:r><a:rPr lang="ja-JP" sz="1100"><a:latin typeface="Latin"/><a:ea typeface="Cjk"/></a:rPr><a:t>A日</a:t></a:r>`
	got := renderSlidePNG(t, data, opts, renderScriptSlide(run, ""))
	if !renderInk(t, got, 6, 5, 16, 8, black) || renderInk(t, got, 20, 5, 32, 8, black) || !renderInk(t, got, 20, 12, 32, 15, black) {
		t.Fatal("A and 日 not drawn in their fonts")
	}
	if got := *asked; len(got) != 2 || got[0] != "Latin" || got[1] != "Cjk" {
		t.Fatalf("fonts asked for: %q", got)
	}
	// Fullwidth forms and CJK punctuation are East Asian characters, in any
	// language, and set in the East Asian font.
	*asked = nil
	got = renderSlidePNG(t, data, opts, renderScriptSlide(`<a:r><a:rPr lang="en-US" sz="1100"><a:latin typeface="Latin"/><a:ea typeface="Cjk"/></a:rPr><a:t>Ａ、</a:t></a:r>`, ""))
	if renderInk(t, got, 6, 5, 32, 8, black) || !renderInk(t, got, 6, 12, 32, 15, black) {
		t.Fatal("fullwidth letter and ideographic comma not in the East Asian font")
	}
	if len(*asked) != 1 || (*asked)[0] != "Cjk" {
		t.Fatalf("fonts asked for: %q", *asked)
	}
	// Latin text never asks for an East Asian font, however the slot is set.
	*asked = nil
	renderSlidePNG(t, data, opts, renderScriptSlide(`<a:r><a:rPr sz="1100"><a:latin typeface="Latin"/><a:ea typeface="Missing"/></a:rPr><a:t>AB</a:t></a:r>`, ""))
	if len(*asked) != 1 || (*asked)[0] != "Latin" {
		t.Fatalf("fonts asked for: %q", *asked)
	}
	// Runs split by color inside East Asian text keep their characters.
	got = renderSlidePNG(t, data, opts, renderScriptSlide(`<a:r><a:rPr sz="1100"><a:latin typeface="Latin"/><a:ea typeface="Cjk"/></a:rPr><a:t>日</a:t></a:r><a:r><a:rPr sz="1100"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:latin typeface="Latin"/><a:ea typeface="Cjk"/></a:rPr><a:t>本語</a:t></a:r>`, ""))
	red := func(r, g, b uint8) bool { return r > 128 && g < 100 && b < 100 }
	if !renderInk(t, got, 6, 12, 16, 15, black) || !renderInk(t, got, 20, 12, 46, 15, red) || renderInk(t, got, 20, 12, 32, 15, black) {
		t.Fatal("colored runs of East Asian text")
	}
}

func TestRenderEastAsianFontResolution(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, _ := renderScriptFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	run := func(lang string) map[string]func(string) string {
		return renderScriptSlide(`<a:r><a:rPr lang="`+lang+`" sz="1100"><a:latin typeface="Latin"/></a:rPr><a:t>日</a:t></a:r>`, "")
	}
	// The default text style names the theme's East Asian font, which the
	// theme leaves empty: without a font list the text has no font.
	if _, err := renderRewrittenPNG(t, data, opts, run("ja-JP")); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("no East Asian font: %v", err)
	}
	var warnings []string
	best := opts
	best.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	got, err := renderRewrittenPNG(t, data, best, run("ja-JP"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "\n"), "East Asian text without an East Asian font") {
		t.Fatalf("warnings: %q", warnings)
	}
	if renderInk(t, got, 0, 0, 80, 60, black) {
		t.Fatal("character drawn in a font without it")
	}
	// The theme's font list names a font by script, chosen by the run's
	// language.
	list := regexp.MustCompile(`(?s)(<a:minorFont>.*?)(</a:minorFont>)`)
	withList := renderApply(t, data, map[string]func(string) string{"ppt/theme/theme1.xml": func(s string) string {
		return list.ReplaceAllString(s, `${1}<a:font script="Jpan" typeface="CjkJa"/><a:font script="Hans" typeface="CjkZh"/>${2}`)
	}})
	for lang, ink := range map[string]int{"ja-JP": 250, "zh-CN": 550, "zh-Hans": 550} {
		got, err := renderRewrittenPNG(t, withList, opts, run(lang))
		if err != nil {
			t.Fatal(lang, err)
		}
		// The ink is ink/1000 of the 14.67px em above the baseline at 15.7.
		top := 15 - ink*14/1000
		if !renderInk(t, got, 6, top+2, 16, top+3, black) || renderInk(t, got, 6, top-3, 16, top-1, black) {
			t.Fatalf("%s: wrong font", lang)
		}
	}
	if _, err := renderRewrittenPNG(t, withList, opts, run("zh-TW")); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("language without a listed font: %v", err)
	}
	// A theme font that names a typeface takes precedence over the list.
	named := renderApply(t, withList, map[string]func(string) string{"ppt/theme/theme1.xml": func(s string) string {
		return strings.Replace(s, `<a:ea typeface=""/>`, `<a:ea typeface="CjkTheme"/>`, 2)
	}})
	got, err = renderRewrittenPNG(t, named, opts, run("ja-JP"))
	if err != nil {
		t.Fatal(err)
	}
	if !renderInk(t, got, 6, 7, 16, 8, black) {
		t.Fatal("theme East Asian font not used")
	}
	// A run's own font overrides the theme, and a major theme reference
	// reads the major font.
	got, err = renderRewrittenPNG(t, named, opts, renderScriptSlide(`<a:r><a:rPr lang="ja-JP" sz="1100"><a:latin typeface="Latin"/><a:ea typeface="+mj-ea"/></a:rPr><a:t>日</a:t></a:r>`, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !renderInk(t, got, 6, 7, 16, 8, black) {
		t.Fatal("major East Asian font not used")
	}
	if _, err = renderRewrittenPNG(t, named, opts, renderScriptSlide(`<a:r><a:rPr sz="1100"><a:latin typeface="Latin"/><a:ea typeface="+bogus-ea"/></a:rPr><a:t>日</a:t></a:r>`, "")); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("unknown font reference: %v", err)
	}
	// ... which matters only to East Asian text.
	if _, err = renderRewrittenPNG(t, named, opts, renderScriptSlide(`<a:r><a:rPr sz="1100"><a:latin typeface="Latin"/><a:ea typeface="+bogus-ea"/></a:rPr><a:t>A</a:t></a:r>`, "")); err != nil {
		t.Fatal(err)
	}
}

func TestRenderEastAsianLineBreaking(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, _ := renderScriptFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	rpr := `<a:rPr sz="1100"><a:latin typeface="Latin"/><a:ea typeface="Cjk"/></a:rPr>`
	slide := func(text, pPr string) map[string]func(string) string {
		return renderScriptSlide(`<a:r>`+rpr+`<a:t>`+text+`</a:t></a:r>`, pPr)
	}
	// 14.67px characters, three to the 48px line: ideographs break between
	// characters without a space. The second line's ink is at y 24.5 to 30.4.
	got := renderSlidePNG(t, data, opts, slide("日本語日本", ""))
	if !renderInk(t, got, 4, 10, 48, 15, black) || renderInk(t, got, 49, 10, 80, 15, black) || !renderInk(t, got, 4, 25, 32, 30, black) || renderInk(t, got, 33, 25, 80, 30, black) {
		t.Fatal("ideographs not wrapped between characters")
	}
	// An ideographic full stop may not begin a line, and by default hangs
	// past the end of the one before.
	hung := renderSlidePNG(t, data, opts, slide("日本語。", ""))
	if !renderInk(t, hung, 50, 10, 62, 15, black) || renderInk(t, hung, 4, 25, 80, 30, black) {
		t.Fatal("stop did not hang")
	}
	// Without hanging punctuation it takes its character to the next line.
	wrapped := renderSlidePNG(t, data, opts, slide("日本語。", `<a:pPr hangingPunct="0"/>`))
	if renderInk(t, wrapped, 49, 10, 80, 15, black) || !renderInk(t, wrapped, 4, 25, 32, 30, black) || renderInk(t, wrapped, 4, 10, 80, 15, black) == false {
		t.Fatal("stop not wrapped with its character")
	}
	// A stop that fits does not hang, and a line that ends with one is
	// aligned without it.
	right := renderSlidePNG(t, data, opts, slide("日。", `<a:pPr algn="r"/>`))
	if !renderInk(t, right, 33, 10, 52, 15, black) {
		t.Fatal("stop that fits did not stay in the line")
	}
	right = renderSlidePNG(t, data, opts, slide("日本語。", `<a:pPr algn="r"/>`))
	if renderInk(t, right, 4, 10, 8, 15, black) || !renderInk(t, right, 8, 10, 12, 15, black) || !renderInk(t, right, 60, 10, 66, 15, black) {
		t.Fatal("right-aligned line did not hang its stop past the margin")
	}
	// Without East Asian line breaking PowerPoint's wrapping is not known.
	off := slide("日本語日", `<a:pPr eaLnBrk="0"/>`)
	if _, err := renderRewrittenPNG(t, data, opts, off); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("eaLnBrk off: %v", err)
	}
	var warnings []string
	best := opts
	best.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	if _, err := renderRewrittenPNG(t, data, best, off); err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "East Asian line breaking") {
		t.Fatalf("best effort: %v %q", err, warnings)
	}
	// ... but it changes nothing for text without East Asian characters.
	if _, err := renderRewrittenPNG(t, data, opts, renderScriptSlide(`<a:r>`+rpr+`<a:t>AB</a:t></a:r>`, `<a:pPr eaLnBrk="0"/>`)); err != nil {
		t.Fatal(err)
	}
}

func TestRenderEastAsianJustifyAndVertical(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, _ := renderScriptFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	rpr := `<a:rPr sz="1100"><a:latin typeface="Latin"/><a:ea typeface="Cjk"/></a:rPr>`
	var warnings []string
	best := opts
	best.Warn = func(err error) { warnings = append(warnings, err.Error()) }

	// Justified East Asian text is spread between its characters, which
	// PowerPoint may do differently.
	justified := renderScriptSlide(`<a:r>`+rpr+`<a:t>日本語日本語</a:t></a:r>`, `<a:pPr algn="just"/>`)
	if _, err := renderRewrittenPNG(t, data, opts, justified); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("justified: %v", err)
	}
	got, err := renderRewrittenPNG(t, data, best, justified)
	if err != nil {
		t.Fatal(err)
	}
	// The first line spans the box, 4 to 52, and the last stays left.
	if len(warnings) == 0 || !renderInk(t, got, 49, 10, 52, 15, black) || renderInk(t, got, 49, 25, 52, 30, black) {
		t.Fatalf("justified lines: %q", warnings)
	}

	// Vertical text turns East Asian characters with their line, where
	// PowerPoint sets them upright; Latin text turns as it does.
	vertical := func(text string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = renderBody(`<a:lstStyle/>`, `<a:p><a:r>`+rpr+`<a:t>`+text+`</a:t></a:r></a:p>`)(s)
			return strings.Replace(s, `<a:bodyPr `, `<a:bodyPr vert="eaVert" `, 1)
		}}
	}
	if _, err = renderRewrittenPNG(t, data, opts, vertical("日")); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("East Asian vertical text: %v", err)
	}
	warnings = nil
	if _, err = renderRewrittenPNG(t, data, best, vertical("日")); err != nil || len(warnings) != 1 {
		t.Fatalf("best effort: %v %q", err, warnings)
	}
	if _, err = renderRewrittenPNG(t, data, opts, vertical("A")); err != nil {
		t.Fatalf("Latin in East Asian vertical text: %v", err)
	}
}

func TestRenderEastAsianAmbiguousSymbols(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, _ := renderScriptFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	run := renderScriptSlide(`<a:r><a:rPr lang="ja-JP" sz="1100"><a:latin typeface="Latin"/><a:ea typeface="Cjk"/></a:rPr><a:t>2×日</a:t></a:r>`, "")
	// PowerPoint may draw the multiplication sign with either font.
	if _, err := renderRewrittenPNG(t, data, opts, run); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("ambiguous symbol: %v", err)
	}
	var warnings []string
	best := opts
	best.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	got, err := renderRewrittenPNG(t, data, best, run)
	if err != nil || len(warnings) != 1 {
		t.Fatalf("best effort: %v %q", err, warnings)
	}
	// All three characters are drawn, the sign in the Latin font.
	if !renderInk(t, got, 20, 5, 32, 8, black) || !renderInk(t, got, 36, 12, 48, 15, black) {
		t.Fatal("symbol or ideograph missing")
	}
}
