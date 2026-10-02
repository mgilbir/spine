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

func TestRenderShapeStyles(t *testing.T) {
	data, opts := renderInheritedText(t)
	slide := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return renderAddToTree(xml)(renderAnyTxBody.ReplaceAllLiteralString(s, ""))
		}}
	}
	white, red, accent := color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 0x15, G: 0x60, B: 0x82, A: 255}
	// A square without a fill of its own takes the theme's first fill style,
	// a solid fill in the reference's color.
	styled := strings.Replace(renderSquare(60, 4, "FF0000"), `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></p:spPr>`,
		`</p:spPr><p:style><a:lnRef idx="0"><a:schemeClr val="accent1"/></a:lnRef><a:fillRef idx="1"><a:schemeClr val="accent1"/></a:fillRef><a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef><a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef></p:style>`, 1)
	if px := renderPixel(t, renderSlidePNG(t, data, opts, slide(styled)), 65, 9); px != accent {
		t.Fatalf("style fill: %+v", px)
	}
	// Its own fill wins.
	own := strings.Replace(styled, `</p:spPr>`, `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></p:spPr>`, 1)
	if px := renderPixel(t, renderSlidePNG(t, data, opts, slide(own)), 65, 9); px != red {
		t.Fatalf("own fill: %+v", px)
	}
	// The font reference colors text that sets no color.
	font := map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		s = renderBody(`<a:lstStyle/>`, `<a:p>`+renderPlainRun+`</a:p>`)(s)
		return strings.Replace(s, `</p:spPr><p:txBody>`, `</p:spPr><p:style><a:lnRef idx="0"><a:schemeClr val="accent1"/></a:lnRef><a:fillRef idx="0"><a:schemeClr val="accent1"/></a:fillRef><a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef><a:fontRef idx="minor"><a:srgbClr val="FF0000"/></a:fontRef></p:style><p:txBody>`, 1)
	}}
	reddish := func(r, g, b uint8) bool { return r > 128 && g < 100 && b < 100 }
	if !renderInk(t, renderSlidePNG(t, data, opts, font), 0, 0, 96, 72, reddish) {
		t.Fatal("font reference color")
	}
	// A line shape runs between the corners its flips choose: flipped
	// horizontally from (50,10) to (10,50).
	line := `<p:sp><p:nvSpPr><p:cNvPr id="91" name="Line"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm flipH="1"><a:off x="95250" y="95250"/><a:ext cx="381000" cy="381000"/></a:xfrm><a:prstGeom prst="line"><a:avLst/></a:prstGeom>` + renderRedLine + `</p:spPr></p:sp>`
	got := renderSlidePNG(t, data, opts, slide(line))
	for at, want := range map[[2]int]color.NRGBA{{40, 20}: red, {20, 20}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("line at %v: %+v, want %+v", at, px, want)
		}
	}
}

func TestRenderPictureCrop(t *testing.T) {
	p, s, _, opts := renderTextSlide(t)
	// The left half is green, the right half blue; the crop keeps the left.
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
	crop := func(rect string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return strings.Replace(s, `<a:stretch>`, rect+`<a:stretch>`, 1)
		}}
	}
	green := color.NRGBA{G: 255, A: 255}
	got := renderSlidePNG(t, data, opts, crop(`<a:srcRect r="50000"/>`))
	if px := renderPixel(t, got, 68, 45); px != green {
		t.Fatalf("cropped picture: %+v", px)
	}
}

// TestRenderEditorMarkup checks that markup only editors or black-and-white
// output read paints as without it.
func TestRenderEditorMarkup(t *testing.T) {
	data, opts := renderInheritedText(t)
	plain := renderSquare(60, 4, "FF0000")
	marked := strings.Replace(strings.Replace(plain, `<p:spPr>`, `<p:spPr bwMode="auto">`, 1), `<p:nvPr/>`, `<p:nvPr userDrawn="1"/>`, 1)
	slide := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{renderLayoutPart: renderAddToTree(xml)}
	}
	if !bytes.Equal(renderSlidePNG(t, data, opts, slide(plain)), renderSlidePNG(t, data, opts, slide(marked))) {
		t.Fatal("editor markup changed the page")
	}
}
