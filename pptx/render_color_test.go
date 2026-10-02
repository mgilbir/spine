package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

func renderPixel(t *testing.T, data []byte, x, y int) color.NRGBA {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func TestRenderThemeColors(t *testing.T) {
	p, _, _ := renderTestSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	fill := func(xml string) func(string) string {
		return func(s string) string {
			return strings.Replace(s, `<a:srgbClr val="FF0000"/>`, xml, 1)
		}
	}
	background := func(bg string) func(string) string {
		return func(s string) string { return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld>`+bg+`<p:spTree>`, 1) }
	}
	const swapped = `<p:clrMapOvr><a:overrideClrMapping bg1="dk1" tx1="lt1" bg2="dk2" tx2="lt2" accent1="accent2" accent2="accent1" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:clrMapOvr>`
	overrideMap := func(s string) string {
		return strings.Replace(s, `<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>`, swapped, 1)
	}
	for _, tc := range []struct {
		name     string
		rewrites map[string]func(string) string
		x, y     int
		want     color.NRGBA
	}{
		// Office 2023 accent1 and its "Darker 25%" and "Lighter 80%" swatches.
		{"scheme", map[string]func(string) string{"ppt/slides/slide1.xml": fill(`<a:schemeClr val="accent1"/>`)}, 2, 2, color.NRGBA{R: 0x15, G: 0x60, B: 0x82, A: 255}},
		{"darker", map[string]func(string) string{"ppt/slides/slide1.xml": fill(`<a:schemeClr val="accent1"><a:lumMod val="75000"/></a:schemeClr>`)}, 2, 2, color.NRGBA{R: 0x10, G: 0x48, B: 0x62, A: 255}},
		{"lighter", map[string]func(string) string{"ppt/slides/slide1.xml": fill(`<a:schemeClr val="accent1"><a:lumMod val="20000"/><a:lumOff val="80000"/></a:schemeClr>`)}, 2, 2, color.NRGBA{R: 0xC1, G: 0xE4, B: 0xF5, A: 255}},
		{"rgb luminance", map[string]func(string) string{"ppt/slides/slide1.xml": fill(`<a:srgbClr val="000000"><a:lumOff val="50000"/></a:srgbClr>`)}, 2, 2, color.NRGBA{R: 128, G: 128, B: 128, A: 255}},
		{"system", map[string]func(string) string{"ppt/slides/slide1.xml": fill(`<a:sysClr val="windowText" lastClr="123456"/>`)}, 2, 2, color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 255}},
		{"slide map override", map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string { return overrideMap(fill(`<a:schemeClr val="accent1"/>`)(s)) }}, 2, 2, color.NRGBA{R: 0xE9, G: 0x71, B: 0x32, A: 255}},
		{"background style", map[string]func(string) string{"ppt/slides/slide1.xml": background(`<p:bg><p:bgRef idx="1001"><a:schemeClr val="accent2"/></p:bgRef></p:bg>`)}, 0, 0, color.NRGBA{R: 0xE9, G: 0x71, B: 0x32, A: 255}},
		{"background without style", map[string]func(string) string{"ppt/slides/slide1.xml": background(`<p:bg><p:bgRef idx="0"><a:schemeClr val="accent2"/></p:bgRef></p:bg>`)}, 0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{"background reference transform", map[string]func(string) string{"ppt/slides/slide1.xml": background(`<p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"><a:lumMod val="50000"/></a:schemeClr></p:bgRef></p:bg>`)}, 0, 0, color.NRGBA{R: 128, G: 128, B: 128, A: 255}},
		// An inherited background resolves through the slide's color map.
		{"inherited background uses slide map", map[string]func(string) string{
			"ppt/slides/slide1.xml": overrideMap,
			"ppt/slideMasters/slideMaster1.xml": func(s string) string {
				return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>`, 1)
			},
		}, 0, 0, color.NRGBA{A: 255}},
	} {
		got, err := renderRewrittenPNG(t, data, render.Options{}, tc.rewrites)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		px := renderPixel(t, got, tc.x, tc.y)
		// PowerPoint's HSL quantization is not documented; allow one 8-bit
		// step against its swatch after a luminance offset.
		slack := uint8(0)
		if tc.name == "lighter" {
			slack = 1
		}
		near := func(a, b uint8) bool { return a-b <= slack || b-a <= slack }
		if !near(px.R, tc.want.R) || !near(px.G, tc.want.G) || !near(px.B, tc.want.B) || px.A != tc.want.A {
			t.Fatalf("%s: %+v, want %+v", tc.name, px, tc.want)
		}
	}
}

func TestRenderRejectsUnsupportedThemeColors(t *testing.T) {
	p, _, _ := renderTestSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	for name, rewrite := range map[string]func(string) string{
		"other transform": func(s string) string {
			return strings.Replace(s, `<a:srgbClr val="FF0000"/>`, `<a:srgbClr val="FF0000"><a:tint val="50000"/></a:srgbClr>`, 1)
		},
		"repeated transform": func(s string) string {
			return strings.Replace(s, `<a:srgbClr val="FF0000"/>`, `<a:srgbClr val="FF0000"><a:lumMod val="50000"/><a:lumMod val="50000"/></a:srgbClr>`, 1)
		},
		"placeholder color": func(s string) string {
			return strings.Replace(s, `<a:srgbClr val="FF0000"/>`, `<a:schemeClr val="phClr"/>`, 1)
		},
		"unknown scheme": func(s string) string {
			return strings.Replace(s, `<a:srgbClr val="FF0000"/>`, `<a:schemeClr val="accent9"/>`, 1)
		},
		"no system value": func(s string) string {
			return strings.Replace(s, `<a:srgbClr val="FF0000"/>`, `<a:sysClr val="windowText"/>`, 1)
		},
		"gradient background style": func(s string) string {
			return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgRef idx="1003"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>`, 1)
		},
		"background style out of range": func(s string) string {
			return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgRef idx="1009"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>`, 1)
		},
		"background reserved index": func(s string) string {
			return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgRef idx="1000"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>`, 1)
		},
		"incomplete color map": func(s string) string {
			return strings.Replace(strings.Replace(s, `<a:srgbClr val="FF0000"/>`, `<a:schemeClr val="accent1"/>`, 1), `<a:masterClrMapping/>`, `<a:overrideClrMapping bg1="lt1"/>`, 1)
		},
	} {
		_, err := renderRewrittenPNG(t, data, render.Options{}, map[string]func(string) string{"ppt/slides/slide1.xml": rewrite})
		if !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestRenderThemeUsesUnsavedEditsAndSourceBudget(t *testing.T) {
	p, _, sh := renderTestSlide(t)
	sh.SetFill(dml.NewSolidFill(dml.ThemeColorAccent1.ToColor()))
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
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
	opened.Theme().ColorScheme().SetAccent1(dml.ColorBlue)
	page, err := slide.PrepareRender(context.Background(), render.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = page.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	if px := renderPixel(t, out.Bytes(), 2, 2); px != (color.NRGBA{B: 255, A: 255}) {
		t.Fatalf("unsaved theme edit: %+v", px)
	}
	parts := zipParts(t, data)
	sources := len(parts["/"+strings.TrimPrefix(slide.partName, "/")]) + len(slide.layout.layoutXML.SourceXML) + len(slide.layout.master.masterXML.SourceXML)
	if _, err = slide.PrepareRender(context.Background(), render.Options{MaxSourceBytes: int64(sources)}); !errors.Is(err, render.ErrLimit) {
		t.Fatalf("theme outside source budget: %v", err)
	}
}
