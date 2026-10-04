package pptxrender

import (
	"image/color"
	"strings"
	"testing"
)

func TestRenderGradients(t *testing.T) {
	data, opts := renderInheritedText(t)
	slide := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return renderAddToTree(xml)(renderAnyTxBody.ReplaceAllLiteralString(s, ""))
		}}
	}
	stops := `<a:gsLst><a:gs pos="0"><a:srgbClr val="FF0000"/></a:gs><a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs></a:gsLst>`
	square := func(shade string) string {
		return strings.Replace(renderSquare(60, 4, "FF0000"), `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`, `<a:gradFill>`+stops+shade+`</a:gradFill>`, 1)
	}
	red := func(c color.NRGBA) bool { return c.R > 200 && c.B < 60 }
	blue := func(c color.NRGBA) bool { return c.B > 200 && c.R < 60 }
	for _, tc := range []struct {
		name, shade string
		near, far   [2]int
		nearT, farT func(color.NRGBA) bool
	}{
		// Left to right, then top to bottom.
		{"linear", `<a:lin ang="0" scaled="0"/>`, [2]int{60, 9}, [2]int{69, 9}, red, blue},
		{"vertical", `<a:lin ang="5400000" scaled="1"/>`, [2]int{65, 4}, [2]int{65, 13}, red, blue},
		// A path gradient's first stop is at the edge, its last at the centre.
		{"circle", `<a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path>`, [2]int{60, 4}, [2]int{65, 9}, red, blue},
	} {
		got := renderSlidePNG(t, data, opts, slide(square(tc.shade)))
		if px := renderPixel(t, got, tc.near[0], tc.near[1]); !tc.nearT(px) {
			t.Fatalf("%s near: %+v", tc.name, px)
		}
		if px := renderPixel(t, got, tc.far[0], tc.far[1]); !tc.farT(px) {
			t.Fatalf("%s far: %+v", tc.name, px)
		}
	}
	// A rounded shape clips its gradient to its outline.
	rounded := strings.Replace(square(`<a:lin ang="0" scaled="0"/>`), `prst="rect"`, `prst="ellipse"`, 1)
	if px := renderPixel(t, renderSlidePNG(t, data, opts, slide(rounded)), 60, 4); px != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("ellipse corner: %+v", px)
	}
	// A theme gradient background style draws.
	bg := map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgPr><a:gradFill>`+stops+`<a:lin ang="0" scaled="0"/></a:gradFill><a:effectLst/></p:bgPr></p:bg><p:spTree>`, 1)
	}}
	got := renderSlidePNG(t, data, opts, bg)
	if px := renderPixel(t, got, 0, 40); !red(px) {
		t.Fatalf("background left: %+v", px)
	}
	if px := renderPixel(t, got, 79, 40); !blue(px) {
		t.Fatalf("background right: %+v", px)
	}
}
