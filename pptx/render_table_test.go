package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderTableDeck returns a saved deck whose slide holds a 2x2 table at
// (10,10) px with 100px and 120px columns and 48px rows: cell (0,0) filled red,
// 4px black borders on every edge, and options serving Noto Sans for every
// font. edit adjusts the table before saving.
func renderTableDeck(t *testing.T, edit func(*Table)) ([]byte, render.Options) {
	t.Helper()
	p := CreateWithOptions(CreateOptions{Options: Options{SlideSize: SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(500), Height: dml.Pixels(300)})
	s := p.AddSlideWithLayout(p.GetLayoutByType(LayoutBlank))
	tbl := s.AddTable(2, 2)
	tbl.SetPosition(dml.Pixels(10), dml.Pixels(10))
	tbl.SetColWidth(0, dml.Pixels(100))
	tbl.SetColWidth(1, dml.Pixels(120))
	for r := 0; r < 2; r++ {
		for c := 0; c < 2; c++ {
			tbl.Cell(r, c).SetBorders(&TableBorder{Width: dml.Pixels(4), Color: dml.ColorBlack, Style: BorderStyleSingle})
		}
	}
	tbl.Cell(0, 0).SetFill(dml.ColorRed)
	if edit != nil {
		edit(tbl)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	return data, render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil }}
}

func TestRenderUnstyledTable(t *testing.T) {
	data, opts := renderTableDeck(t, func(tbl *Table) { tbl.Cell(1, 1).SetText("Hi") })
	got, err := renderRewrittenPNG(t, data, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	red, black := color.NRGBA{R: 255, A: 255}, color.NRGBA{A: 255}
	for at, want := range map[[2]int]color.NRGBA{
		{50, 30}:  red,   // cell (0,0)
		{110, 30}: black, // grid line x 110, 4px wide
		{50, 58}:  black, // grid line y 58
		{8, 8}:    black, // the outer corner is closed
		{180, 30}: {R: 255, G: 255, B: 255, A: 255},
	} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("%v: %+v, want %+v", at, px, want)
		}
	}
	// Cell (1,1) spans x 110-230 and y 58-106; its text starts past the
	// 0.1" left margin.
	dark := false
	for x := 118; x < 150 && !dark; x++ {
		for y := 60; y < 104 && !dark; y++ {
			dark = renderPixel(t, got, x, y).R < 100
		}
	}
	if !dark {
		t.Fatal("cell text missing")
	}
	// A row grows to hold its text, moving the grid line below it.
	tall, opts := renderTableDeck(t, func(tbl *Table) {
		tbl.Cell(0, 1).SetText(strings.Repeat("word ", 30))
	})
	// Noto Sans charges about a million shaping units per byte; see the
	// spine-render budget.
	opts.Limits.MaxShapeWork = 16 << 30
	grown, err := renderRewrittenPNG(t, tall, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if px := renderPixel(t, grown, 50, 58); px != red {
		t.Fatalf("row did not grow: %+v", px)
	}
}

func TestRenderRejectsUnsupportedTables(t *testing.T) {
	for name, edit := range map[string]func(*Table){
		"style": func(tbl *Table) { tbl.SetStyleID("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}") },
		"conflicting": func(tbl *Table) {
			tbl.Cell(0, 0).SetBorderRight(&TableBorder{Width: dml.Pixels(4), Color: dml.ColorRed, Style: BorderStyleSingle})
		},
		"differing at a corner": func(tbl *Table) {
			tbl.Cell(0, 0).SetBorderBottom(&TableBorder{Width: dml.Pixels(2), Color: dml.ColorBlack, Style: BorderStyleSingle})
			tbl.Cell(1, 0).SetBorderTop(&TableBorder{Width: dml.Pixels(2), Color: dml.ColorBlack, Style: BorderStyleSingle})
		},
		"dashed border": func(tbl *Table) {
			tbl.Cell(0, 0).SetBorderLeft(&TableBorder{Width: dml.Pixels(4), Color: dml.ColorBlack, Style: BorderStyleDashed})
		},
	} {
		data, opts := renderTableDeck(t, edit)
		if _, err := renderRewrittenPNG(t, data, opts, nil); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	data, opts := renderTableDeck(t, nil)
	merged := func(s string) string {
		s = strings.Replace(s, `<a:tc>`, `<a:tc gridSpan="2">`, 1)
		return strings.Replace(s, `</a:tc><a:tc>`, `</a:tc><a:tc hMerge="1">`, 1)
	}
	// The first row's cells merge: its red fill spans both columns, with no
	// border between them; the second row keeps its grid line.
	got, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": merged})
	if err != nil {
		t.Fatalf("merged: %v", err)
	}
	red, black := color.NRGBA{R: 255, A: 255}, color.NRGBA{A: 255}
	for at, want := range map[[2]int]color.NRGBA{{110, 30}: red, {180, 30}: red, {110, 80}: black} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("merged at %v: %+v, want %+v", at, px, want)
		}
	}
	// The first column's cells merge down: its fill reaches the second row.
	down := func(s string) string {
		s = strings.Replace(s, `<a:tc>`, `<a:tc rowSpan="2">`, 1)
		i := strings.Index(s, `</a:tr>`)
		return s[:i] + strings.Replace(s[i:], `<a:tc>`, `<a:tc vMerge="1">`, 1)
	}
	got, err = renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": down})
	if err != nil {
		t.Fatalf("merged down: %v", err)
	}
	for at, want := range map[[2]int]color.NRGBA{{50, 58}: red, {50, 80}: red, {180, 58}: black} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("merged down at %v: %+v, want %+v", at, px, want)
		}
	}
	// A cell marked merged outside any merge is invalid.
	stray := func(s string) string { return strings.Replace(s, `<a:tc>`, `<a:tc hMerge="1">`, 1) }
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": stray}); !errors.Is(err, render.ErrInvalid) {
		t.Fatalf("stray merge: %v", err)
	}
	// Tables are drawn from their saved form only.
	p := CreateWithOptions(CreateOptions{Options: Options{SlideSize: SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(500), Height: dml.Pixels(300)})
	s := p.AddSlideWithLayout(p.GetLayoutByType(LayoutBlank))
	s.AddTable(1, 1)
	if _, err := s.PrepareRender(context.Background(), opts); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("unsaved table: %v", err)
	}
	opened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	slide := opened.Slides()[0]
	if _, err = slide.PrepareRender(context.Background(), opts); err != nil {
		t.Fatalf("opened table: %v", err)
	}
	slide.Shapes()[0].(*Table).Cell(0, 0).SetText("edited")
	if _, err = slide.PrepareRender(context.Background(), opts); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("edited table: %v", err)
	}
}

func TestRenderBoundsHostileTableGeometry(t *testing.T) {
	data, opts := renderTableDeck(t, nil)
	wide := func(s string) string {
		return regexp.MustCompile(`<a:gridCol w="\d+"`).ReplaceAllString(s, `<a:gridCol w="9223372036854775807"`)
	}
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": wide}); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("overflowing width: %v", err)
	}
}
