package pptxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// TestRenderRightToLeftTabs places tabs as PowerPoint does in a right-to-left
// paragraph, as measured in its own export: stops are measured from the
// line's start at its right end, a left stop puts the left edge of the text
// after the tab at the stop and a right stop its right edge, and the line is
// then placed by the paragraph's physical alignment.
func TestRenderRightToLeftTabs(t *testing.T) {
	data, _ := renderInheritedText(t)
	opts, _ := renderRTLFonts(t)
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	// 11pt is 14.67px, and the box's text starts at x 4 and is 48px wide.
	// Alef then bet are read right to left: bet is drawn at the left.
	slide := func(algn, stop string) map[string]func(string) string {
		return renderScriptSlide(renderRTLRun("א\tב"), `<a:pPr rtl="1" defTabSz="190500"><a:tabLst><a:tab pos="`+stop+`" algn="`+algn+`"/></a:tabLst></a:pPr>`)
	}
	for _, tc := range []struct {
		algn, stop string
		right      int
	}{
		// Bet's left edge is 36px from the line's right end, so the line is
		// 36px wide from x 4 and alef ends at x 40.
		{"l", "342900", 40},
		// Bet's right edge is 30px from the right end: the line is 44.7px.
		{"r", "285750", 49},
	} {
		got := renderSlidePNG(t, data, opts, slide(tc.algn, tc.stop))
		l, _, r, _ := renderInkBounds(t, got)
		if l < 3 || l > 5 || r < tc.right-1 || r > tc.right+1 {
			t.Errorf("%s stop: ink spans x %d to %d, want 4 to %d", tc.algn, l, r, tc.right)
		}
		// Bet (lower ink) at the left, a gap, then alef (full ink).
		if !renderInk(t, got, 5, 12, 17, 15, black) || renderInk(t, got, 5, 5, 17, 8, black) || renderInk(t, got, 20, 5, 24, 15, black) || !renderInk(t, got, r-10, 5, r-2, 8, black) {
			t.Errorf("%s stop: letters out of place", tc.algn)
		}
	}
	// Stops that were not measured fail in strict mode, and best effort draws
	// them and says so.
	for name, rewrite := range map[string]map[string]func(string) string{
		"centred stop": slide("ctr", "342900"),
		"no stop":      renderScriptSlide(renderRTLRun("א\tב"), `<a:pPr rtl="1" defTabSz="190500"/>`),
	} {
		if _, err := renderRewrittenPNG(t, data, opts, rewrite); !errors.Is(err, render.ErrUnsupported) {
			t.Errorf("%s: %v", name, err)
		}
		var warnings []string
		best := opts
		best.Warn = func(err error) { warnings = append(warnings, err.Error()) }
		if _, err := renderRewrittenPNG(t, data, best, rewrite); err != nil || !strings.Contains(strings.Join(warnings, "\n"), "right-to-left tab") {
			t.Errorf("%s in best effort: %v %q", name, err, warnings)
		}
	}
}

// TestRenderComplexScriptDigitsFont checks which font draws ASCII digits and
// spaces, as measured in PowerPoint's own export: in a run of a language
// written in a complex script they use the complex-script font, and
// elsewhere the Latin one.
func TestRenderComplexScriptDigitsFont(t *testing.T) {
	data, _ := renderInheritedText(t)
	// The Latin font's digits are full boxes and the complex-script font's
	// lower halves.
	load := func(space int, glyphs ...fonttest.Glyph) *shape.Face {
		face, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: append(glyphs, fonttest.Glyph{Rune: ' ', Advance: space})}))
		if err != nil {
			t.Fatal(err)
		}
		return face
	}
	glyph := func(r rune, ink [4]int) fonttest.Glyph {
		return fonttest.Glyph{Rune: r, Advance: 1000, HasShape: true, Ink: ink}
	}
	full, lower := [4]int{0, 0, 1000, 800}, [4]int{0, 0, 1000, 400}
	faces := map[string]*shape.Face{
		"Latin":  load(500, glyph('1', full), glyph('A', full), glyph('.', full)),
		"Hebrew": load(1000, glyph('1', lower), glyph('א', full), glyph('.', lower)),
	}
	opts := render.Options{Fonts: func(_ context.Context, r render.FontRequest) (*shape.Face, error) {
		if r.Family == "Aptos" {
			r.Family = "Latin"
		}
		return faces[r.Family], nil
	}}
	black := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	run := func(lang, text string) map[string]func(string) string {
		return renderScriptSlide(`<a:r><a:rPr lang="`+lang+`" sz="1100"><a:latin typeface="Latin"/><a:cs typeface="Hebrew"/></a:rPr><a:t>`+text+`</a:t></a:r>`, "")
	}
	// 11pt is 14.67px: upper ink is y 4 to 9.9 and lower ink 9.9 to 15.7.
	for lang, upper := range map[string]bool{"he-IL": false, "ar-EG": false, "en-US": true} {
		got, err := renderRewrittenPNG(t, data, opts, run(lang, "1"))
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if !renderInk(t, got, 5, 12, 17, 15, black) || renderInk(t, got, 5, 5, 17, 8, black) == !upper {
			t.Errorf("%s: digit in the wrong font", lang)
		}
	}
	// A space is in the same font as the digits: in the complex-script font
	// it is as wide as a letter, so the second digit ends at x 48; in the
	// Latin font it is half as wide and the digit ends at x 41.
	for lang, wide := range map[string]bool{"he-IL": true, "en-US": false} {
		got, err := renderRewrittenPNG(t, data, opts, run(lang, "1 1"))
		if err != nil || renderInk(t, got, 44, 12, 47, 15, black) != wide {
			t.Errorf("%s: digits and space: %v", lang, err)
		}
	}
	// ASCII punctuation and symbols follow the digits: every mark of
	// ". , : ; ! ? ( ) [ ] { } - _ / \\ | ~ \" ' ` @ # $ % ^ & * + = < >" is drawn
	// in the complex-script font in he-IL and ar-SA runs and in the Latin
	// font in en-US, in PowerPoint's own export.
	for lang, upper := range map[string]bool{"he-IL": false, "en-US": true} {
		got, err := renderRewrittenPNG(t, data, opts, run(lang, "1."))
		if err != nil {
			t.Fatalf("%s punctuation: %v", lang, err)
		}
		if !renderInk(t, got, 5, 12, 17, 15, black) || renderInk(t, got, 5, 5, 17, 8, black) == !upper {
			t.Errorf("%s: digit beside punctuation in the wrong font", lang)
		}
		if got, err = renderRewrittenPNG(t, data, opts, run(lang, ".")); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if !renderInk(t, got, 5, 12, 17, 15, black) || renderInk(t, got, 5, 5, 17, 8, black) == !upper {
			t.Errorf("%s: punctuation in the wrong font", lang)
		}
	}
}
