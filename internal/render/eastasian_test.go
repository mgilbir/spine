package render

import (
	"context"
	"errors"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
)

func TestEastAsianRepertoire(t *testing.T) {
	for _, c := range "日本語あアカ한글ㄅ、。「」（）　ＡＢ１ｱ㈱㍿豈\U00020000" {
		if !IsEastAsian(c) || !RepertoireEastAsian.Allows(c) {
			t.Errorf("%U not East Asian text", c)
		}
		if RepertoireEuropean.Allows(c) || RepertoireASCII.Allows(c) {
			t.Errorf("%U allowed outside the East Asian repertoire", c)
		}
	}
	// Symbols of ambiguous width and European letters stay European, and
	// controls and format characters stay out of every repertoire.
	for _, c := range "aé×“→Ω" {
		if IsEastAsian(c) || !RepertoireEastAsian.Allows(c) {
			t.Errorf("%U", c)
		}
	}
	for _, c := range []rune{0x85, 0x200b, 0x2028, 0xad, 0xfeff, 0x202e, 0xfe0f, 0x5e9, 0x627} {
		if RepertoireEastAsian.Allows(c) {
			t.Errorf("%U allowed", c)
		}
	}
}

func TestRichLinesEastAsianBreakingAndHanging(t *testing.T) {
	var glyphs []fonttest.Glyph
	for _, c := range "日本語、。「」A" {
		glyphs = append(glyphs, fonttest.Glyph{Rune: c, Advance: 1000, HasShape: true})
	}
	face, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: glyphs}))
	if err != nil {
		t.Fatal(err)
	}
	lines := func(text string, width float64, hang bool) []RichLine {
		t.Helper()
		layout, _ := NewTextLayout(Limits{})
		got, err := layout.RichLinesWith(context.Background(), []Span{{Face: face, Size: unit(10), Text: text}}, unit(width), RepertoireEastAsian, RichOptions{HangPunct: hang})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	text := func(l RichLine) string {
		s := ""
		for _, sg := range l.Segments {
			s += sg.Text
		}
		return s
	}
	// Ideographs break between characters.
	got := lines("日本語日本", 35, false)
	if len(got) != 2 || text(got[0]) != "日本語" || text(got[1]) != "日本" || got[0].Width != unit(30) {
		t.Fatalf("ideographs: %+v", got)
	}
	// A closing mark cannot begin a line, nor an opening one end it.
	got = lines("日本語。", 35, false)
	if len(got) != 2 || text(got[0]) != "日本" || text(got[1]) != "語。" {
		t.Fatalf("closing mark: %+v", got)
	}
	got = lines("日本「語", 35, false)
	if len(got) != 2 || text(got[0]) != "日本" || text(got[1]) != "「語" {
		t.Fatalf("opening mark: %+v", got)
	}
	// A stop that would not fit hangs past the line.
	got = lines("日本語。日", 35, true)
	if len(got) != 2 || text(got[0]) != "日本語。" || text(got[1]) != "日" || got[0].Width != unit(30) || got[0].Hang != unit(10) || got[1].Hang != 0 {
		t.Fatalf("hanging stop: %+v", got)
	}
	// One that fits stays in the line, and only stops and commas hang.
	got = lines("日、日", 35, true)
	if len(got) != 1 || got[0].Width != unit(30) || got[0].Hang != 0 {
		t.Fatalf("stop that fits: %+v", got)
	}
	got = lines("日本語」", 35, true)
	if len(got) != 2 || text(got[1]) != "語」" {
		t.Fatalf("closing quote: %+v", got)
	}
	// A hung stop still counts toward the budgets and the segment text.
	if got = lines("日本語。", 35, true); len(got) != 1 || len(got[0].Segments) != 1 || got[0].Segments[0].Text != "日本語。" || len(got[0].Segments[0].Glyphs) != 4 {
		t.Fatalf("hung stop segment: %+v", got)
	}
	// Characters outside the repertoire still fail.
	layout, _ := NewTextLayout(Limits{})
	if _, err = layout.RichLinesWith(context.Background(), []Span{{Face: face, Size: unit(10), Text: "日ש"}}, unit(100), RepertoireEastAsian, RichOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("right-to-left letter: %v", err)
	}
	if _, err = layout.RichLines(context.Background(), []Span{{Face: face, Size: unit(10), Text: "日"}}, unit(100), RepertoireEuropean); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("ideograph in the European repertoire: %v", err)
	}
}
