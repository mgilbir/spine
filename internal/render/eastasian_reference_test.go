package render

import (
	"context"
	"slices"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
)

// TestRichLinesEastAsianMatchPowerPoint checks the line breaks PowerPoint's
// own export gave for one Japanese sentence at 14 pt in boxes 1.3 to 1.3875
// in wide (0.9 pt apart, 0.1 in insets), with hanging punctuation on and off.
// The face is a stand-in with full-width advances: the breaks depend on the
// advances, not on the glyph shapes.
func TestRichLinesEastAsianMatchPowerPoint(t *testing.T) {
	const sentence = "日本語の句読点の処理を確かめる文章です。句点が行末にくると、ぶら下げになるかどうかを見ます。"
	seen := map[rune]bool{}
	var glyphs []fonttest.Glyph
	for _, c := range sentence {
		if !seen[c] {
			seen[c] = true
			glyphs = append(glyphs, fonttest.Glyph{Rune: c, Advance: 1000, HasShape: true})
		}
	}
	face, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: glyphs}))
	if err != nil {
		t.Fatal(err)
	}
	narrow := []string{"日本語の句", "読点の処理", "を確かめる", "文章です。", "句点が行末", "にくると、", "ぶら下げに", "なるかどう"}
	wide := []string{"日本語の句読", "点の処理を確", "かめる文章で", "す。句点が行", "末にくると、", "ぶら下げにな", "るかどうかを", "見ます。"}
	for i := 0; i < 8; i++ {
		inches := 1.3 + float64(i)*0.0125
		width := inches*72 - 14.4
		for _, hang := range []bool{true, false} {
			want := wide
			if i < 6 {
				want = append(slices.Clone(narrow), "かを見ま", "す。")
				if hang {
					want = append(slices.Clone(narrow), "かを見ます。")
				}
			}
			layout, _ := NewTextLayout(Limits{})
			got, err := layout.RichLinesWith(context.Background(), []Span{{Face: face, Size: unit(14), Text: sentence}}, unit(width), RepertoireEastAsian, RichOptions{HangPunct: hang})
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for _, l := range got {
				s := ""
				for _, sg := range l.Segments {
					s += sg.Text
				}
				lines = append(lines, s)
			}
			if !slices.Equal(lines, want) {
				t.Errorf("width %.4f in, hanging %v: %q, want %q", inches, hang, lines, want)
			}
		}
	}
}
