package render

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
)

func TestPlainParagraphShapingMatchesWholeLines(t *testing.T) {
	f, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	l, _ := NewTextLayout(Limits{})
	lines, err := l.PlainLines(context.Background(), f, "office AVATAR office AVATAR", unit(16), unit(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) < 2 {
		t.Fatal("fixture did not wrap")
	}
	for _, line := range lines {
		want, missing := f.Clone().ShapeGlyphs(line.Text)
		if missing != 0 || len(want) != len(line.Glyphs) {
			t.Fatal("shaping mismatch")
		}
		for i, glyph := range want {
			if glyph != line.Glyphs[i] {
				t.Fatalf("glyph %d: %+v != %+v", i, glyph, line.Glyphs[i])
			}
		}
	}
	// The same layout consumes the previously spent measurement work.
	used := l.budget.shapeWork
	limited, _ := NewTextLayout(Limits{MaxShapeWork: used})
	if _, err = limited.PlainLines(context.Background(), f, "office AVATAR office AVATAR", unit(16), unit(100)); err != nil {
		t.Fatal(err)
	}
	if _, err = limited.PlainLines(context.Background(), f, "office", unit(16), unit(100)); !errors.Is(err, ErrLimit) {
		t.Fatalf("aggregate work: %v", err)
	}
}

func TestPlainParagraphUsesFormatFeatureDefaults(t *testing.T) {
	f, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	l, _ := NewTextLayout(Limits{})
	features := shape.Features{TagsOff: "liga,clig,kern,calt"}
	lines, err := l.PlainLinesWithFeatures(context.Background(), f, "office", unit(16), unit(100), features)
	if err != nil {
		t.Fatal(err)
	}
	want, missing := f.Clone().ShapeGlyphsMerged("office", "", "", "", "", false, features)
	if missing != 0 || len(lines) != 1 || len(lines[0].Glyphs) != len(want) {
		t.Fatal("feature shaping mismatch")
	}
	for i, g := range want {
		if g != lines[0].Glyphs[i] {
			t.Fatalf("glyph %d mismatch", i)
		}
	}
	defaultGlyphs, _ := f.Clone().ShapeGlyphs("office")
	if len(defaultGlyphs) >= len(want) {
		t.Fatal("fixture did not exercise disabled ligatures")
	}
}

func paragraphFace(t testing.TB) *shape.Face {
	t.Helper()
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: []fonttest.Glyph{
		{Rune: 'A', Advance: 1000, HasShape: true, Ink: [4]int{0, 0, 1000, 1000}}, {Rune: ' ', Advance: 500},
	}}))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPlainParagraphWrapAndOwnership(t *testing.T) {
	f := paragraphFace(t)
	l, err := NewTextLayout(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	lines, err := l.PlainLines(context.Background(), f, "AA AA AA", unit(10), unit(30))
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, line := range lines {
		texts = append(texts, line.Text)
		if line.Width > unit(30) {
			t.Fatal("overflow")
		}
	}
	if strings.Join(texts, "") != "AA AA AA" || len(lines) != 3 {
		t.Fatalf("lines: %q", texts)
	}
	if len(f.Used()) != 0 {
		t.Fatal("source font mutated")
	}
	if lines[0].Face == f {
		t.Fatal("retained source font")
	}
	if lines[0].Width != unit(25) {
		t.Fatalf("first width: %v", lines[0].Width)
	}
}

func TestPlainParagraphBudgetsAndFailures(t *testing.T) {
	f := paragraphFace(t)
	for _, tc := range []struct {
		limits Limits
		text   string
		width  int
		want   error
	}{
		{Limits{MaxShapeWork: 1}, "AA", 30, ErrLimit},
		{Limits{MaxGlyphs: 1}, "AA", 30, ErrLimit},
		{Limits{MaxTextBytes: 1}, "AA", 30, ErrLimit},
		{Limits{MaxRunBytes: 1}, "AA", 30, ErrLimit},
		{Limits{}, "AAA", 10, ErrUnsupported},
		{Limits{}, "B", 30, ErrUnsupported},
		{Limits{}, "A\tA", 30, ErrUnsupported},
	} {
		l, e := NewTextLayout(tc.limits)
		if e != nil {
			t.Fatal(e)
		}
		got, e := l.PlainLines(context.Background(), f, tc.text, unit(10), unit(float64(tc.width)))
		if got != nil || !errors.Is(e, tc.want) {
			t.Fatalf("%+v: %v %v", tc, got, e)
		}
	}
	l, _ := NewTextLayout(Limits{MaxTextBytes: 3})
	if _, e := l.PlainLines(context.Background(), f, "AA", unit(10), unit(30)); e != nil {
		t.Fatal(e)
	}
	if _, e := l.PlainLines(context.Background(), f, "AA", unit(10), unit(30)); !errors.Is(e, ErrLimit) {
		t.Fatalf("cumulative bytes: %v", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := l.PlainLines(ctx, f, "A", unit(10), unit(30)); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestEuropeanParagraphRepertoire(t *testing.T) {
	f, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	l, _ := NewTextLayout(Limits{})
	text := "café naïve “Ωμέγα” Привет — 5 € … 5 − 3"
	lines, err := l.Lines(context.Background(), f, text, unit(16), unit(90), shape.Features{}, RepertoireEuropean)
	if err != nil {
		t.Fatal(err)
	}
	var joined strings.Builder
	for _, line := range lines {
		want, missing := f.Clone().ShapeGlyphs(line.Text)
		if missing != 0 || len(want) != len(line.Glyphs) {
			t.Fatalf("shaping mismatch in %q", line.Text)
		}
		joined.WriteString(line.Text)
	}
	if len(lines) < 2 || joined.String() != text {
		t.Fatalf("%d lines joined as %q", len(lines), joined.String())
	}
	// Arrows, operators and geometric shapes, in a font that has them.
	symbols, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{UnitsPerEm: 1000, Ascent: 800, Descent: -200, Glyphs: []fonttest.Glyph{{Rune: '→', Advance: 500, HasShape: true, Ink: [4]int{0, 0, 500, 500}}, {Rune: '≤', Advance: 500, HasShape: true, Ink: [4]int{0, 0, 500, 500}}, {Rune: '■', Advance: 500, HasShape: true, Ink: [4]int{0, 0, 500, 500}}, {Rune: ' ', Advance: 250}}}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.Lines(context.Background(), symbols, "→ ≤ ■", unit(16), unit(90), shape.Features{}, RepertoireEuropean); err != nil {
		t.Fatalf("symbols: %v", err)
	}
	if _, err = l.PlainLines(context.Background(), f, "café", unit(16), unit(90)); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("ASCII repertoire accepted Latin-1: %v", err)
	}
	for name, s := range map[string]string{
		"right-to-left":    "\u05e9\u05dc\u05d5\u05dd",
		"soft hyphen":      "co\u00adop",
		"zero width space": "a\u200bb",
		"line separator":   "a\u2028b",
		"fullwidth":        "\uff21",
		"ideograph":        "\u4e2d",
		"control":          "a\u0085b",
	} {
		if _, err = l.Lines(context.Background(), f, s, unit(16), unit(90), shape.Features{}, RepertoireEuropean); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestRichLinesMatchSingleSpanAndMixFaces(t *testing.T) {
	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	text := "office AVATAR office AVATAR café"
	plain, _ := NewTextLayout(Limits{})
	want, err := plain.Lines(context.Background(), noto, text, unit(16), unit(100), shape.Features{}, RepertoireEuropean)
	if err != nil {
		t.Fatal(err)
	}
	rich, _ := NewTextLayout(Limits{})
	got, err := rich.RichLines(context.Background(), []Span{{Face: noto, Size: unit(16), Text: text}}, unit(100), RepertoireEuropean)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("%d lines, want %d", len(got), len(want))
	}
	for i, line := range got {
		if len(line.Segments) != 1 || line.Segments[0].Text != want[i].Text || line.Width != want[i].Width || len(line.Segments[0].Glyphs) != len(want[i].Glyphs) {
			t.Fatalf("line %d: %+v, want %q", i, line, want[i].Text)
		}
		for k, g := range line.Segments[0].Glyphs {
			if g != want[i].Glyphs[k] {
				t.Fatalf("line %d glyph %d", i, k)
			}
		}
	}
	if rich.budget.shapeWork != plain.budget.shapeWork || rich.budget.glyphs != plain.budget.glyphs || rich.budget.textBytes != plain.budget.textBytes || rich.lines != plain.lines {
		t.Fatal("single span charged differently from Lines")
	}
	// Two sizes: a word split across spans stays unbreakable, each span
	// shapes alone, and segments sit end to end.
	mixed, _ := NewTextLayout(Limits{})
	lines, err := mixed.RichLines(context.Background(), []Span{{Face: noto, Size: unit(16), Text: "big wo"}, {Face: noto, Size: unit(8), Text: "rd small text"}}, unit(70), RepertoireEuropean)
	if err != nil {
		t.Fatal(err)
	}
	var joined strings.Builder
	for _, line := range lines {
		x := unit(0)
		for _, sg := range line.Segments {
			if sg.X != x {
				t.Fatalf("segment %q at %v, want %v", sg.Text, sg.X, x)
			}
			x += sg.Width
			joined.WriteString(sg.Text)
		}
		if line.Width > unit(70) {
			t.Fatalf("line width %v", line.Width)
		}
	}
	if joined.String() != "big word small text" || len(lines) < 2 {
		t.Fatalf("%d lines joined as %q", len(lines), joined.String())
	}
	for _, line := range lines {
		for _, sg := range line.Segments {
			if strings.HasPrefix(sg.Text, "rd") && sg.Offset != 0 {
				t.Fatalf("offset %d", sg.Offset)
			}
		}
		if line.Segments[0].Text == "rd " || line.Segments[0].Text == "rd" {
			t.Fatal("broke inside a word at a span boundary")
		}
	}
	empty, err := mixed.RichLines(context.Background(), []Span{{Face: noto, Size: unit(10)}}, unit(70), RepertoireEuropean)
	if err != nil || len(empty) != 1 || len(empty[0].Segments) != 1 || empty[0].Segments[0].Face == nil {
		t.Fatalf("empty paragraph: %+v %v", empty, err)
	}
}

func TestRichLinesTabs(t *testing.T) {
	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	layout, _ := NewTextLayout(Limits{})
	if _, err = layout.RichLines(context.Background(), []Span{{Face: noto, Size: unit(16), Text: "A\tB"}}, unit(400), RepertoireEuropean); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("tab without stops: %v", err)
	}
	lines, err := layout.RichLines(context.Background(), []Span{{Face: noto, Size: unit(16), Text: "A\tB\tC", TabStop: unit(100)}}, unit(400), RepertoireEuropean)
	if err != nil {
		t.Fatal(err)
	}
	// A, a tab to 100, B, a tab to 200, C.
	if len(lines) != 1 || len(lines[0].Segments) != 5 {
		t.Fatalf("lines: %+v", lines)
	}
	sg := lines[0].Segments
	if sg[1].Text != "\t" || len(sg[1].Glyphs) != 0 || sg[2].X != unit(100) || sg[4].X != unit(200) {
		t.Fatalf("tab stops: %v %v %v", sg[1].Text, sg[2].X, sg[4].X)
	}
}
