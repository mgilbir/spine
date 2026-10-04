package pptxrender

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
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
	pic := pptx.NewPicture()
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

func TestRenderTurnedText(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	black, white := color.NRGBA{A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	turn := func(old, new string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			i := strings.Index(s, "<p:sp>")
			if i < 0 || !strings.Contains(s[i:], old) {
				t.Fatalf("no %s in %s", old, s)
			}
			return s[:i] + strings.Replace(s[i:], old, new, 1)
		}}
	}
	// The shape spans (4,4) to (52,52). Its first line, "AA ", inks x 4 to
	// 36 and y 4 to 16.8; turned a quarter about (28,28), x 39.2 to 52 and
	// y 4 to 36.
	quarter := map[[2]int]color.NRGBA{{10, 10}: white, {45, 10}: black, {45, 30}: black, {30, 45}: white}
	// Turned a half, by a rotation or by a vertical flip, the line inks
	// x 20 to 52 and y 39.2 to 52, upside down.
	half := map[[2]int]color.NRGBA{{10, 10}: white, {30, 45}: black, {48, 48}: black, {10, 45}: white}
	for _, tc := range []struct {
		name, old, new string
		want           map[[2]int]color.NRGBA
	}{
		{"rotated shape", `<a:xfrm>`, `<a:xfrm rot="5400000">`, quarter},
		{"flipped shape", `<a:xfrm>`, `<a:xfrm flipV="1">`, half},
		{"rotated body", `<a:bodyPr`, `<a:bodyPr rot="10800000"`, half},
		{"turned back", `<a:xfrm>`, `<a:xfrm rot="10800000" flipV="1">`, map[[2]int]color.NRGBA{{10, 10}: black, {45, 45}: white}},
		// Vertical text turns its lines a quarter in the square shape.
		{"vertical", `<a:bodyPr`, `<a:bodyPr vert="vert"`, quarter},
		{"East Asian vertical", `<a:bodyPr`, `<a:bodyPr vert="eaVert"`, quarter},
		// Turned the other way, the first line runs up the left side.
		{"vertical 270", `<a:bodyPr`, `<a:bodyPr vert="vert270"`, map[[2]int]color.NRGBA{{10, 45}: black, {10, 25}: black, {45, 10}: white, {10, 10}: white}},
		// Upright text ignores its shape's turn.
		{"upright", `<a:xfrm>`, `<a:xfrm rot="5400000">`, nil},
	} {
		var warnings []string
		opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
		rewrite := turn(tc.old, tc.new)
		if tc.name == "upright" {
			body := turn(`<a:bodyPr`, `<a:bodyPr upright="1"`)["ppt/slides/slide1.xml"]
			xfrm := rewrite["ppt/slides/slide1.xml"]
			rewrite = map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string { return body(xfrm(s)) }}
			tc.want = map[[2]int]color.NRGBA{{10, 10}: black, {45, 10}: white}
		}
		got := renderSlidePNG(t, data, opts, rewrite)
		if len(warnings) != 0 {
			t.Fatalf("%s: %q", tc.name, warnings)
		}
		for at, want := range tc.want {
			if px := renderPixel(t, got, at[0], at[1]); px != want {
				t.Fatalf("%s at %v: %+v, want %+v", tc.name, at, px, want)
			}
		}
	}
}

func TestRenderVerticalTextSwapsItsBox(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// The shape is 48 by 24px at (4,4), holding "A". Set vertically, the
	// line runs down a 24px wide, 48px tall box about the shape's centre
	// (28,16), whose top edge is the shape's right side: the A, 16px wide
	// and 12.8px tall, inks x 39.2 to 52 and y 4 to 20.
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		i := strings.Index(s, "<p:sp>")
		rest := s[i:]
		rest = regexp.MustCompile(`<a:ext cx="\d+" cy="\d+"/>`).ReplaceAllLiteralString(rest, `<a:ext cx="457200" cy="228600"/>`)
		rest = strings.Replace(rest, "AA AA", "A", 1)
		rest = strings.Replace(rest, `<a:bodyPr`, `<a:bodyPr vert="vert"`, 1)
		return s[:i] + rest
	}})
	black, white := color.NRGBA{A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for at, want := range map[[2]int]color.NRGBA{{45, 10}: black, {45, 18}: black, {10, 10}: white, {45, 24}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("at %v: %+v, want %+v", at, px, want)
		}
	}
}
