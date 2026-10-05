package pptxrender

import (
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

func TestRenderShapeEffects(t *testing.T) {
	p := pptx.CreateWithOptions(pptx.CreateOptions{Options: pptx.Options{SlideSize: pptx.SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(200), Height: dml.Pixels(160)})
	s := blankSlide(t, p)
	// A 60px blue square at (40,30).
	sh := pptx.NewAutoShape(pptx.PresetRect)
	sh.SetPosition(dml.Pixels(40), dml.Pixels(30))
	sh.SetSize(dml.Pixels(60), dml.Pixels(60))
	sh.SetFill(dml.NewSolidFill(dml.ColorBlue))
	sh.SetNoLine()
	if err := s.AddShape(sh); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	effect := func(e string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(x string) string {
			const ln = `<a:ln><a:noFill/></a:ln></p:spPr>`
			if !strings.Contains(x, ln) {
				t.Fatalf("no outline in %s", x)
			}
			return strings.Replace(x, ln, `<a:ln><a:noFill/></a:ln><a:effectLst>`+e+`</a:effectLst></p:spPr>`, 1)
		}}
	}
	white, blue := color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{B: 255, A: 255}
	var warnings []string
	opts := render.Options{Warn: func(err error) { warnings = append(warnings, err.Error()) }}
	draw := func(e string) []byte {
		warnings = nil
		return renderSlidePNG(t, data, opts, effect(e))
	}
	for _, tc := range []struct {
		name, xml string
		check     func(px func(x, y int) color.NRGBA) bool
	}{
		// A glow tints just outside the square, fading with distance.
		{"glow", `<a:glow rad="152400"><a:srgbClr val="FF0000"/></a:glow>`, func(px func(x, y int) color.NRGBA) bool {
			near, far := px(103, 60), px(130, 60)
			return near.R == 255 && near.G < 200 && far == white && px(70, 60) == blue
		}},
		// Soft edges fade the square's own edge, its middle untouched.
		{"soft edges", `<a:softEdge rad="95250"/>`, func(px func(x, y int) color.NRGBA) bool {
			edge := px(41, 60)
			return edge.R > 100 && edge.B == 255 && px(70, 60) == blue
		}},
		// An inner shadow cast down and right darkens the inside of the top
		// and left edges.
		{"inner shadow", `<a:innerShdw blurRad="57150" dist="57150" dir="2700000"><a:srgbClr val="000000"/></a:innerShdw>`, func(px func(x, y int) color.NRGBA) bool {
			return px(42, 60).B < 200 && px(98, 60) == blue && px(70, 70) == blue
		}},
		// A blurred shadow below and right, its edge soft.
		{"blurred shadow", `<a:outerShdw blurRad="76200" dist="95250" dir="2700000" algn="tl"><a:srgbClr val="000000"/></a:outerShdw>`, func(px func(x, y int) color.NRGBA) bool {
			// The shadow spans x 47 to 107; 8px of blur soften its edge.
			mid, edge := px(103, 70), px(110, 70)
			return mid.R < 140 && edge.R > mid.R && edge.R < 255 && px(30, 20) == white
		}},
		// A reflection below the square, fading downwards.
		{"reflection", `<a:reflection stA="80000" endA="0" endPos="100000" dist="0" dir="5400000" sy="-100000" algn="bl"/>`, func(px func(x, y int) color.NRGBA) bool {
			top, bottom := px(70, 92), px(70, 140)
			return top.R < 100 && bottom.R > top.R && px(70, 60) == blue
		}},
		// The square itself blurred over its edge.
		{"blur", `<a:blur rad="76200"/>`, func(px func(x, y int) color.NRGBA) bool {
			edge := px(40, 60)
			return edge.R > 50 && edge.R < 220 && px(70, 60) == blue
		}},
	} {
		got := draw(tc.xml)
		if !tc.check(func(x, y int) color.NRGBA { return renderPixel(t, got, x, y) }) {
			t.Errorf("%s: pixels %+v %+v %+v", tc.name, renderPixel(t, got, 41, 60), renderPixel(t, got, 103, 60), renderPixel(t, got, 70, 92))
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], "drawn approximately") {
			t.Errorf("%s warnings: %q", tc.name, warnings)
		}
		// Strict mode refuses an effect drawn approximately.
		if _, err := renderRewrittenPNG(t, data, render.Options{}, effect(tc.xml)); !errors.Is(err, render.ErrUnsupported) {
			t.Errorf("%s strict: %v", tc.name, err)
		}
	}
	// A sharp shadow is drawn exactly, as shapes are.
	draw(`<a:outerShdw dist="95250" dir="2700000"><a:srgbClr val="000000"/></a:outerShdw>`)
	if len(warnings) != 0 {
		t.Fatalf("sharp shadow: %q", warnings)
	}
	// Effects share a slide budget of rasterized pixels.
	colors := &renderColors{ctx: context.Background(), effectPixels: renderMaxSlideEffectPixels - 10}
	sq, err := renderRectOps(10, 10, 50, 50)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := colors.rasterize(sq, 0); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("effect budget: %v", err)
	}
}

// renderRectOps is a black square's drawing.
func renderRectOps(x, y, w, h float64) ([]layout.Op, error) {
	ux, _ := style.FromPx(x)
	uy, _ := style.FromPx(y)
	uw, _ := style.FromPx(w)
	uh, _ := style.FromPx(h)
	return []layout.Op{layout.FillRect{Rect: layout.Rect{X: ux, Y: uy, W: uw, H: uh}, Color: style.RGBA{A: 1}}}, nil
}
