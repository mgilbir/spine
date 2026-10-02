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
