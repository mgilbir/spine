package render

import (
	"context"
	"errors"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
)

func TestComplexScriptRepertoire(t *testing.T) {
	for _, c := range "אבגשלום،ابتثمرحبا٣ﻻﭐְًܐހߊיִﷺﺍ\u200c\u200d\u200e\u200f\u061c" {
		if !RepertoireBidi.Allows(c) {
			t.Errorf("%U not allowed", c)
		}
		if RepertoireEastAsian.Allows(c) || RepertoireEuropean.Allows(c) || RepertoireASCII.Allows(c) {
			t.Errorf("%U allowed outside the right-to-left repertoire", c)
		}
	}
	for _, c := range []rune{0x0600, 0x06dd, 0x070f, 0x08e2, 0xfeff, 0xfdd0, 0x05ff, 0x2066, 0x202a, 0x202e, 0x200b, 0x0e01, 0x0915} {
		if RepertoireBidi.Allows(c) {
			t.Errorf("%U allowed", c)
		}
	}
	for _, c := range "日áé" {
		if !RepertoireBidi.Allows(c) {
			t.Errorf("%U not allowed", c)
		}
	}
	if IsComplex('a') || IsComplex('日') || !IsComplex('א') || IsFormat('a') || !IsFormat(0x200d) {
		t.Error("classification")
	}
}

// bidiFixtureFace has Hebrew letters of different widths, Arabic beh in its
// four forms, Latin letters and brackets that tell a mirrored glyph from its
// mirror.
func bidiFixtureFace(t *testing.T) *shape.Face {
	t.Helper()
	var glyphs []fonttest.Glyph
	for r, adv := range map[rune]int{
		'א': 1000, 'ב': 2000, 'ג': 3000, ' ': 500, 'A': 1000, 'B': 1000, '1': 1000, '2': 1000, '.': 500,
		'(': 400, ')': 600, 'ب': 500, 'ﺏ': 500, 'ﺐ': 900, 'ﺑ': 700, 'ﺒ': 800,
	} {
		glyphs = append(glyphs, fonttest.Glyph{Rune: r, Advance: adv, HasShape: r != ' '})
	}
	face, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: glyphs}))
	if err != nil {
		t.Fatal(err)
	}
	return face
}

func TestRichLinesBidirectional(t *testing.T) {
	face := bidiFixtureFace(t)
	gid := func(r rune) int {
		id, ok := face.GlyphID(r)
		if !ok {
			t.Fatalf("no glyph for %q", r)
		}
		return id
	}
	lines := func(text string, width float64, rtl bool) []RichLine {
		t.Helper()
		layout, _ := NewTextLayout(Limits{})
		got, err := layout.RichLinesWith(context.Background(), []Span{{Face: face, Size: unit(10), Text: text}}, unit(width), RepertoireBidi, RichOptions{RTL: rtl})
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		return got
	}
	// Segments in the order they are drawn, with their text and level.
	type seg struct {
		text  string
		level int
	}
	order := func(l RichLine) []seg {
		var out []seg
		x := unit(0)
		for _, sg := range l.Segments {
			if sg.X != x {
				t.Fatalf("segment %q at %v, want %v", sg.Text, sg.X, x)
			}
			x += sg.Width
			out = append(out, seg{sg.Text, sg.Level})
		}
		return out
	}
	equal := func(got, want []seg) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	// Hebrew in a left-to-right paragraph is one right-to-left segment whose
	// glyphs are in drawing order, last letter first, clusters falling.
	got := lines("אבג", 100, false)
	if len(got) != 1 || len(got[0].Segments) != 1 {
		t.Fatalf("%+v", got)
	}
	sg := got[0].Segments[0]
	if !sg.RTL() || sg.Text != "אבג" || len(sg.Glyphs) != 3 || sg.Glyphs[0].GID != gid('ג') || sg.Glyphs[2].GID != gid('א') || sg.Glyphs[0].Cluster != 4 || sg.Glyphs[2].Cluster != 0 || sg.Width != unit(60) {
		t.Fatalf("hebrew segment: %+v", sg)
	}
	// A Latin word in it stays left to right, and the line runs left to right
	// around the right-to-left stretch.
	if got = lines("A אב B", 100, false); len(got) != 1 || !equal(order(got[0]), []seg{{"A ", 0}, {"אב", 1}, {" B", 0}}) {
		t.Fatalf("mixed: %v", order(got[0]))
	}
	// In a right-to-left paragraph the line starts at the right: the Latin
	// word comes last, and the digits keep their order inside the Hebrew.
	if got = lines("A אב", 100, true); len(got) != 1 || !equal(order(got[0]), []seg{{" אב", 1}, {"A", 2}}) {
		t.Fatalf("right-to-left paragraph: %v", order(got[0]))
	}
	if got = lines("א 12 ב", 100, true); len(got) != 1 || !equal(order(got[0]), []seg{{" ב", 1}, {"12", 2}, {"א ", 1}}) {
		t.Fatalf("numbers: %v", order(got[0]))
	}
	// White space ending a line takes the paragraph's direction.
	if got = lines("אב ", 100, false); len(got) != 1 || !equal(order(got[0]), []seg{{"אב", 1}, {" ", 0}}) {
		t.Fatalf("trailing space: %v", order(got[0]))
	}
	// Brackets facing a right-to-left run are mirrored: in the drawing order
	// the opening glyph comes first, standing for the closing character.
	got = lines("(א)", 100, true)
	if len(got) != 1 || len(got[0].Segments) != 1 {
		t.Fatalf("brackets: %+v", got)
	}
	if g := got[0].Segments[0].Glyphs; len(g) != 3 || g[0].GID != gid('(') || g[1].GID != gid('א') || g[2].GID != gid(')') {
		t.Fatalf("brackets not mirrored: %+v", g)
	}
	// Breaking is in logical order, and each line is ordered on its own.
	got = lines("אב אב א", 35, true)
	if len(got) != 3 || got[0].Segments[0].Text != "אב " {
		t.Fatalf("wrapped: %+v", got)
	}
	var joined string
	for _, l := range got {
		for _, sg := range l.Segments {
			joined += sg.Text
			if l.Width > unit(35) {
				t.Fatalf("line width %v", l.Width)
			}
		}
	}
	if joined != "אב אב א" {
		t.Fatalf("joined %q", joined)
	}
	// Arabic letters take their joined forms: initial, then final, drawn
	// final first.
	got = lines("بب", 100, true)
	if len(got) != 1 || len(got[0].Segments) != 1 {
		t.Fatalf("arabic: %+v", got)
	}
	if g := got[0].Segments[0].Glyphs; len(g) != 2 || g[0].GID != gid('ﺐ') || g[1].GID != gid('ﺑ') || got[0].Width != unit(16) {
		t.Fatalf("arabic joining: %+v", g)
	}
	// Left-to-right text is not touched: no levels, and tabs still work.
	if got = lines("A B", 100, false); len(got[0].Segments) != 1 || got[0].Segments[0].Level != 0 || got[0].Segments[0].RTL() {
		t.Fatalf("left to right: %+v", got)
	}
	layout, _ := NewTextLayout(Limits{})
	spans := []Span{{Face: face, Size: unit(10), Text: "א\tב", TabStop: unit(50)}}
	if _, err := layout.RichLinesWith(context.Background(), spans, unit(100), RepertoireBidi, RichOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("tab in right-to-left text: %v", err)
	}
	spans[0].Text = "A\tB"
	if _, err := layout.RichLinesWith(context.Background(), spans, unit(100), RepertoireBidi, RichOptions{}); err != nil {
		t.Fatalf("tab in left-to-right text: %v", err)
	}
	// A repertoire without the right-to-left scripts still refuses them.
	spans[0].Text = "א"
	if _, err := layout.RichLinesWith(context.Background(), spans, unit(100), RepertoireEastAsian, RichOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("hebrew in the East Asian repertoire: %v", err)
	}
}
