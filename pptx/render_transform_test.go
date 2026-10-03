package pptx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

func TestRenderRotatedAndFlippedShapes(t *testing.T) {
	data, opts := renderInheritedText(t)
	slide := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return renderAddToTree(xml)(renderAnyTxBody.ReplaceAllLiteralString(s, ""))
		}}
	}
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// A 20 by 10px rectangle centred on (60,20); turned a quarter, it stands
	// 10 by 20px.
	rect := func(xfrm string) string {
		return `<p:sp><p:nvSpPr><p:cNvPr id="90" name="Bar"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm` + xfrm + `><a:off x="476250" y="142875"/><a:ext cx="190500" cy="95250"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></p:spPr></p:sp>`
	}
	got := renderSlidePNG(t, data, opts, slide(rect(` rot="5400000"`)))
	for at, want := range map[[2]int]color.NRGBA{{60, 12}: red, {60, 28}: red, {52, 20}: white, {68, 20}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("turned at %v: %+v, want %+v", at, px, want)
		}
	}
	// A right triangle flipped horizontally puts its right angle on the
	// other side.
	tri := strings.Replace(rect(` flipH="1"`), `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`, `<a:custGeom><a:pathLst><a:path w="2" h="1"><a:moveTo><a:pt x="0" y="0"/></a:moveTo><a:lnTo><a:pt x="0" y="1"/></a:lnTo><a:lnTo><a:pt x="2" y="1"/></a:lnTo><a:close/></a:path></a:pathLst></a:custGeom>`, 1)
	got = renderSlidePNG(t, data, opts, slide(tri))
	if px := renderPixel(t, got, 68, 16); px != red {
		t.Fatalf("flipped triangle right: %+v", px)
	}
	if px := renderPixel(t, got, 52, 16); px != white {
		t.Fatalf("flipped triangle left: %+v", px)
	}
}

func TestRenderFlippedPicture(t *testing.T) {
	p, s, _, opts := renderTextSlide(t)
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}
	pic := NewPicture()
	pic.SetImageData(encoded.Bytes(), "image/png")
	pic.SetPosition(dml.Pixels(60), dml.Pixels(40))
	pic.SetSize(dml.Pixels(10), dml.Pixels(10))
	if err := s.AddShape(pic); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	flip := map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		i := strings.Index(s, "<p:pic>")
		return s[:i] + strings.Replace(s[i:], `<a:xfrm>`, `<a:xfrm flipH="1">`, 1)
	}}
	got := renderSlidePNG(t, data, opts, flip)
	if px := renderPixel(t, got, 62, 45); px != (color.NRGBA{B: 255, A: 255}) {
		t.Fatalf("flipped picture left: %+v", px)
	}
}
