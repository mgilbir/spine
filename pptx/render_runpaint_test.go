package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
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
	opened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	page, err := opened.Slides()[0].PrepareRender(context.Background(), opts)
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

func TestRenderRejectsRunsThatShapeApart(t *testing.T) {
	data, opts := renderInheritedText(t)
	for name, paragraphs := range map[string]string{
		"sizes":  `<a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r><a:r><a:rPr sz="1400"/><a:t>A</a:t></a:r></a:p>`,
		"fonts":  `<a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r><a:r><a:rPr sz="1200"><a:latin typeface="Other"/></a:rPr><a:t>A</a:t></a:r></a:p>`,
		"kerned": `<a:p><a:r><a:rPr sz="1200" kern="0"/><a:t>A</a:t></a:r><a:r><a:rPr sz="1200" kern="1200"/><a:t>A</a:t></a:r></a:p>`,
	} {
		_, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, paragraphs)})
		if !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("%s: %v", name, err)
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
	if _, err = renderRewrittenPNG(t, data, opts, text(`<a:r><a:rPr sz="800"/><a:t>`+"\u05e9\u05dc\u05d5\u05dd"+`</a:t></a:r>`)); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("right-to-left text: %v", err)
	}
}
