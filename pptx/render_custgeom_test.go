package pptx

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

func TestRenderCustomGeometry(t *testing.T) {
	data, opts := renderInheritedText(t)
	slide := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return renderAddToTree(xml)(renderAnyTxBody.ReplaceAllLiteralString(s, ""))
		}}
	}
	// A 20px square shape at (50,10) whose geometry is in a 100-unit space.
	shape := func(geometry, ln string) string {
		return `<p:sp><p:nvSpPr><p:cNvPr id="90" name="Custom"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="476250" y="95250"/><a:ext cx="190500" cy="190500"/></a:xfrm><a:custGeom><a:avLst/><a:gdLst><a:gd name="mid" fmla="*/ w 1 2"/></a:gdLst><a:ahLst/><a:cxnLst/><a:rect l="l" t="t" r="r" b="b"/><a:pathLst>` +
			geometry + `</a:pathLst></a:custGeom><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>` + ln + `</p:spPr></p:sp>`
	}
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// A triangle with its apex at the top middle.
	triangle := `<a:path w="100" h="100"><a:moveTo><a:pt x="50" y="0"/></a:moveTo><a:lnTo><a:pt x="100" y="100"/></a:lnTo><a:lnTo><a:pt x="0" y="100"/></a:lnTo><a:close/></a:path>`
	got := renderSlidePNG(t, data, opts, slide(shape(triangle, "")))
	for at, want := range map[[2]int]color.NRGBA{{60, 27}: red, {51, 11}: white, {68, 11}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("triangle at %v: %+v, want %+v", at, px, want)
		}
	}
	// A half disc from an arc: from the left middle, sweeping 180° clockwise
	// through the top, closed along the diameter.
	half := `<a:path w="100" h="100"><a:moveTo><a:pt x="0" y="50"/></a:moveTo><a:arcTo wR="50" hR="50" stAng="10800000" swAng="10800000"/><a:close/></a:path>`
	got = renderSlidePNG(t, data, opts, slide(shape(half, "")))
	for at, want := range map[[2]int]color.NRGBA{{60, 14}: red, {60, 25}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("half disc at %v: %+v, want %+v", at, px, want)
		}
	}
	// A cubic curve and guide-based points.
	curve := `<a:path><a:moveTo><a:pt x="l" y="b"/></a:moveTo><a:cubicBezTo><a:pt x="l" y="t"/><a:pt x="r" y="t"/><a:pt x="r" y="b"/></a:cubicBezTo><a:close/></a:path>`
	if px := renderPixel(t, renderSlidePNG(t, data, opts, slide(shape(curve, ""))), 60, 26); px != red {
		t.Fatalf("curve: %+v", px)
	}
	// A multi-segment outline fails strictly and is drawn approximately.
	outlined := slide(shape(strings.Replace(triangle, `<a:path w="100" h="100">`, `<a:path w="100" h="100" fill="none">`, 1), `<a:ln w="19050"><a:solidFill><a:srgbClr val="0000FF"/></a:solidFill></a:ln>`))
	if _, err := renderRewrittenPNG(t, data, opts, outlined); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict outline: %v", err)
	}
	opts.Warn = func(error) {}
	got = renderSlidePNG(t, data, opts, outlined)
	if px := renderPixel(t, got, 50, 29); px.B < 200 || px.R > 100 {
		t.Fatalf("outline corner: %+v", px)
	}
	if px := renderPixel(t, got, 60, 25); px != white {
		t.Fatalf("unfilled path: %+v", px)
	}
}
