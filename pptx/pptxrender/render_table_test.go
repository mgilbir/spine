package pptxrender

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
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

// renderTableDeck returns a saved deck whose slide holds a 2x2 table at
// (10,10) px with 100px and 120px columns and 48px rows: cell (0,0) filled red,
// 4px black borders on every edge, and options serving Noto Sans for every
// font. edit adjusts the table before saving.
func renderTableDeck(t *testing.T, edit func(*pptx.Table)) ([]byte, render.Options) {
	t.Helper()
	p := pptx.CreateWithOptions(pptx.CreateOptions{Options: pptx.Options{SlideSize: pptx.SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(500), Height: dml.Pixels(300)})
	s := blankSlide(t, p)
	tbl := s.AddTable(2, 2)
	tbl.SetPosition(dml.Pixels(10), dml.Pixels(10))
	tbl.SetColWidth(0, dml.Pixels(100))
	tbl.SetColWidth(1, dml.Pixels(120))
	for r := 0; r < 2; r++ {
		for c := 0; c < 2; c++ {
			tbl.Cell(r, c).SetBorders(&pptx.TableBorder{Width: dml.Pixels(4), Color: dml.ColorBlack, Style: pptx.BorderStyleSingle})
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
	data, opts := renderTableDeck(t, func(tbl *pptx.Table) { tbl.Cell(1, 1).SetText("Hi") })
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
	tall, opts := renderTableDeck(t, func(tbl *pptx.Table) {
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
	for name, edit := range map[string]func(*pptx.Table){
		// A style that is neither built in nor read from the file.
		"style": func(tbl *pptx.Table) { tbl.SetStyleID("{00000000-0000-0000-0000-000000000000}") },
		"conflicting": func(tbl *pptx.Table) {
			tbl.Cell(0, 0).SetBorderRight(&pptx.TableBorder{Width: dml.Pixels(4), Color: dml.ColorRed, Style: pptx.BorderStyleSingle})
		},
		"differing at a corner": func(tbl *pptx.Table) {
			tbl.Cell(0, 0).SetBorderBottom(&pptx.TableBorder{Width: dml.Pixels(2), Color: dml.ColorBlack, Style: pptx.BorderStyleSingle})
			tbl.Cell(1, 0).SetBorderTop(&pptx.TableBorder{Width: dml.Pixels(2), Color: dml.ColorBlack, Style: pptx.BorderStyleSingle})
		},
		"dashed border": func(tbl *pptx.Table) {
			tbl.Cell(0, 0).SetBorderLeft(&pptx.TableBorder{Width: dml.Pixels(4), Color: dml.ColorBlack, Style: pptx.BorderStyleDashed})
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
	p := pptx.CreateWithOptions(pptx.CreateOptions{Options: pptx.Options{SlideSize: pptx.SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(500), Height: dml.Pixels(300)})
	s := blankSlide(t, p)
	s.AddTable(1, 1)
	if _, err := PrepareSlide(context.Background(), s, opts); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("unsaved table: %v", err)
	}
	opened, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	slide := opened.Slides()[0]
	if _, err = PrepareSlide(context.Background(), slide, opts); err != nil {
		t.Fatalf("opened table: %v", err)
	}
	slide.Shapes()[0].(*pptx.Table).Cell(0, 0).SetText("edited")
	if _, err = PrepareSlide(context.Background(), slide, opts); !errors.Is(err, render.ErrUnsupported) {
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

func TestRenderInheritedAndGroupedTables(t *testing.T) {
	data, opts := renderTableDeck(t, nil)
	frame := regexp.MustCompile(`(?s)<p:graphicFrame>.*</p:graphicFrame>`)
	var table string
	slide := func(f func(rest string) string) func(string) string {
		return func(s string) string {
			table = frame.FindString(s)
			if table == "" {
				t.Fatalf("no table in %s", s)
			}
			return f(frame.ReplaceAllLiteralString(s, ""))
		}
	}
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// group wraps the table in a group moved 40px right, whose child space
	// is scale times its extent.
	group := func(scale int) string {
		ext := `<a:ext cx="3000000" cy="2000000"/>`
		ch := `<a:chExt cx="` + []string{"", "3000000", "6000000"}[scale] + `" cy="` + []string{"", "2000000", "4000000"}[scale] + `"/>`
		return `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="95" name="Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="381000" y="0"/>` + ext + `<a:chOff x="0" y="0"/>` + ch + `</a:xfrm></p:grpSpPr>` + table + `</p:grpSp>`
	}
	for _, tc := range []struct {
		name     string
		rewrites map[string]func(string) string
		at       [2]int
		warning  string
	}{
		{"master", map[string]func(string) string{
			"ppt/slides/slide1.xml": slide(func(s string) string { return s }),
			renderMasterPart:        func(s string) string { return renderAddToTree(table)(s) },
		}, [2]int{50, 30}, ""},
		{"group", map[string]func(string) string{
			"ppt/slides/slide1.xml": slide(func(s string) string { return renderAddToTree(group(1))(s) }),
		}, [2]int{90, 30}, ""},
		{"scaled group", map[string]func(string) string{
			"ppt/slides/slide1.xml": slide(func(s string) string { return renderAddToTree(group(2))(s) }),
		}, [2]int{50, 20}, "table in a scaled group drawn at its own size"},
	} {
		var warnings []string
		o := opts
		o.Warn = func(err error) { warnings = append(warnings, err.Error()) }
		// The slide is rewritten first, so the master sees the table.
		data := renderApply(t, data, map[string]func(string) string{"ppt/slides/slide1.xml": tc.rewrites["ppt/slides/slide1.xml"]})
		delete(tc.rewrites, "ppt/slides/slide1.xml")
		got := renderSlidePNG(t, data, o, tc.rewrites)
		if (tc.warning == "" && len(warnings) != 0) || (tc.warning != "" && (len(warnings) != 1 || !strings.Contains(warnings[0], tc.warning))) {
			t.Fatalf("%s warnings: %q", tc.name, warnings)
		}
		if px := renderPixel(t, got, tc.at[0], tc.at[1]); px != red {
			t.Fatalf("%s at %v: %+v", tc.name, tc.at, px)
		}
		if tc.name != "master" {
			if px := renderPixel(t, got, 15, 30); px != white {
				t.Fatalf("%s left of the group: %+v", tc.name, px)
			}
		}
	}
}
