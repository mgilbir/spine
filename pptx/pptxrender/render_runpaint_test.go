package pptxrender

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

func TestRenderPaintsRunsAndHighlights(t *testing.T) {
	data, opts := renderInheritedText(t)
	slide := func(paragraphs string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, paragraphs)}
	}
	// 12pt glyphs are 16px squares from x 4: "A" spans 4-20, " " 20-28 and
	// the second "A" 28-44, with the glyph box from y 4 to 20.
	run := func(rPr, text string) string {
		return `<a:r><a:rPr sz="1200">` + rPr + `</a:rPr><a:t>` + text + `</a:t></a:r>`
	}
	red := `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`
	blue := `<a:solidFill><a:srgbClr val="0000FF"/></a:solidFill>`
	yellow := `<a:highlight><a:srgbClr val="FFFF00"/></a:highlight>`
	got := renderSlidePNG(t, data, opts, slide(`<a:p>`+run(red, "A")+run(blue+yellow, " A")+`</a:p>`))
	for _, tc := range []struct {
		x, y int
		want color.NRGBA
	}{
		{10, 10, color.NRGBA{R: 255, A: 255}},
		{24, 10, color.NRGBA{R: 255, G: 255, A: 255}}, // highlighted space
		{36, 10, color.NRGBA{B: 255, A: 255}},
		{24, 22, color.NRGBA{R: 255, G: 255, B: 255, A: 255}}, // below the glyph box
	} {
		if px := renderPixel(t, got, tc.x, tc.y); px != tc.want {
			t.Fatalf("%d,%d: %+v, want %+v", tc.x, tc.y, px, tc.want)
		}
	}
	// Splitting a uniformly painted paragraph into runs paints the same.
	whole := renderSlidePNG(t, data, opts, slide(`<a:p>`+run(red, "A A")+`</a:p>`))
	split := renderSlidePNG(t, data, opts, slide(`<a:p>`+run(red, "A")+run(red, " ")+run(red, "")+run(red, "A")+`</a:p>`))
	if !bytes.Equal(whole, split) {
		t.Fatal("run boundaries changed uniform painting")
	}
}

func TestRenderMergesTouchingHighlights(t *testing.T) {
	data, opts := renderInheritedText(t)
	yellow := `<a:highlight><a:srgbClr val="FFFF00"/></a:highlight>`
	body := renderBody(`<a:lstStyle/>`, `<a:p><a:r><a:rPr sz="1200">`+yellow+`</a:rPr><a:t>A</a:t></a:r><a:r><a:rPr sz="1200">`+yellow+`</a:rPr><a:t> A</a:t></a:r></a:p>`)
	data = rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(b []byte) []byte { return []byte(body(string(b))) })
	opened, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	page, err := PrepareSlide(context.Background(), opened.Slides()[0], opts)
	if err != nil {
		t.Fatal(err)
	}
	var svg bytes.Buffer
	if err = page.WriteSVG(context.Background(), &svg, 96); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.ToLower(svg.String()), "rgb(255,255,0)") + strings.Count(strings.ToLower(svg.String()), "#ffff00"); n != 1 {
		t.Fatalf("%d highlight rectangles:\n%s", n, svg.String())
	}
}

func TestRenderMixesRunSizesOnOneBaseline(t *testing.T) {
	data, opts := renderInheritedText(t)
	// A 24pt "A" (32px, ascent 25.6) then a 12pt "A" (16px, ascent 12.8)
	// share the baseline at y 29.6: the big glyph spans x 4-36 and y 4-29.6,
	// the small one x 36-52 and y 16.8-29.6.
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`,
		`<a:p><a:r><a:rPr sz="2400"/><a:t>A</a:t></a:r><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r></a:p>`)})
	for _, tc := range []struct {
		x, y int
		dark bool
	}{{20, 10, true}, {44, 10, false}, {44, 24, true}, {44, 32, false}, {20, 32, false}} {
		if dark := renderPixel(t, got, tc.x, tc.y).R < 128; dark != tc.dark {
			t.Fatalf("%d,%d: dark %v", tc.x, tc.y, dark)
		}
	}
	// The next paragraph starts below the taller line box.
	two := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`,
		`<a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r><a:r><a:rPr sz="2400"/><a:t> </a:t></a:r></a:p><a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r></a:p>`)})
	// Line one: 32px box, small glyph on baseline 29.6 (y 16.8-29.6); line
	// two from 36: glyph y 36-48.8.
	for _, tc := range []struct {
		y    int
		dark bool
	}{{10, false}, {24, true}, {33, false}, {42, true}} {
		if dark := renderPixel(t, two, 10, tc.y).R < 128; dark != tc.dark {
			t.Fatalf("row %d: dark %v", tc.y, dark)
		}
	}
}

func TestRenderEuropeanText(t *testing.T) {
	data, opts := renderInheritedText(t)
	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil }
	text := func(runs string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, `<a:p>`+runs+`</a:p>`)}
	}
	if _, err = renderRewrittenPNG(t, data, opts, text(`<a:r><a:rPr sz="800"/><a:t>Cliënt – “Ωμέγα”</a:t></a:r>`)); err != nil {
		t.Fatal(err)
	}
	// A combining mark in its own run shapes with the letter before it.
	if _, err = renderRewrittenPNG(t, data, opts, text(`<a:r><a:rPr sz="800"/><a:t>e</a:t></a:r><a:r><a:rPr sz="800"/><a:t>`+"\u0301"+`</a:t></a:r>`)); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("mark split from its letter: %v", err)
	}
	// Right-to-left letters the run's font lacks.
	if _, err = renderRewrittenPNG(t, data, opts, text(`<a:r><a:rPr sz="800"/><a:t>`+"\u05e9\u05dc\u05d5\u05dd"+`</a:t></a:r>`)); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("right-to-left text: %v", err)
	}
	// PowerPoint may draw ambiguous-width symbols with the East Asian font in
	// East Asian text.
	if _, err = renderRewrittenPNG(t, data, opts, text(`<a:r><a:rPr lang="en-US" sz="800"/><a:t>2×3</a:t></a:r>`)); err != nil {
		t.Fatalf("symbol: %v", err)
	}
	for name, rewrite := range map[string]map[string]func(string) string{
		"own language":       text(`<a:r><a:rPr lang="ja-JP" sz="800"/><a:t>2×3</a:t></a:r>`),
		"alternate language": text(`<a:r><a:rPr lang="en-US" altLang="zh-CN" sz="800"/><a:t>2×3</a:t></a:r>`),
		"inherited language": {"ppt/slides/slide1.xml": renderBody(`<a:lstStyle><a:lvl1pPr><a:defRPr lang="ko-KR"/></a:lvl1pPr></a:lstStyle>`, `<a:p><a:r><a:rPr sz="800"/><a:t>2×3</a:t></a:r></a:p>`)},
	} {
		if _, err = renderRewrittenPNG(t, data, opts, rewrite); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err = renderRewrittenPNG(t, data, opts, text(`<a:r><a:rPr lang="ja-JP" sz="800"/><a:t>ASCII</a:t></a:r>`)); err != nil {
		t.Fatalf("ASCII in an East Asian language: %v", err)
	}
}
