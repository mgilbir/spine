package pptx

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

func TestRenderPatternBits(t *testing.T) {
	seen := map[int]bool{}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			seen[renderBayer[y][x]] = true
		}
	}
	if len(seen) != 64 {
		t.Fatalf("dither ranks: %d distinct", len(seen))
	}
	count := func(b [8][8]bool) int {
		n := 0
		for _, row := range b {
			for _, v := range row {
				if v {
					n++
				}
			}
		}
		return n
	}
	// Every preset of ST_PresetPatternVal draws, none blank or solid.
	presets := "pct5 pct10 pct20 pct25 pct30 pct40 pct50 pct60 pct70 pct75 pct80 pct90 horz vert ltHorz ltVert dkHorz dkVert narHorz narVert dashHorz dashVert cross dnDiag upDiag ltDnDiag ltUpDiag dkDnDiag dkUpDiag wdDnDiag wdUpDiag dashDnDiag dashUpDiag diagCross smCheck lgCheck smGrid lgGrid dotGrid smConfetti lgConfetti horzBrick diagBrick solidDmnd openDmnd dotDmnd plaid sphere weave divot shingle wave trellis zigZag"
	for _, p := range strings.Fields(presets) {
		bits, ok := renderPatternBits(p)
		if n := count(bits); !ok || n == 0 || n == 64 {
			t.Errorf("%s: %v, %d set", p, ok, n)
		}
	}
	if len(strings.Fields(presets)) != 54 {
		t.Fatal("preset list")
	}
	// Percentages are their density, and each spreads its dots: no two
	// of pct25's set pixels touch.
	for p, want := range map[string]int{"pct5": 3, "pct25": 16, "pct50": 32, "pct90": 58} {
		if bits, _ := renderPatternBits(p); count(bits) != want {
			t.Errorf("%s: %d set", p, count(bits))
		}
	}
	bits, _ := renderPatternBits("pct25")
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if bits[y][x] && (bits[y][(x+1)%8] || bits[(y+1)%8][x]) {
				t.Fatalf("pct25 clusters at %d,%d", x, y)
			}
		}
	}
	if _, ok := renderPatternBits("plaidish"); ok {
		t.Fatal("unknown preset drawn")
	}
}

func TestRenderPatternFills(t *testing.T) {
	data, opts := renderInheritedText(t)
	square := func(fill string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
			// A 16px square at (60,40).
			return renderAddToTree(`<p:sp><p:nvSpPr><p:cNvPr id="90" name="Patterned"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="571500" y="381000"/><a:ext cx="152400" cy="152400"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` + fill + `</p:spPr></p:sp>`)(s)
		}}
	}
	red, blue := color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}
	vert := square(`<a:pattFill prst="ltVert"><a:fgClr><a:srgbClr val="FF0000"/></a:fgClr><a:bgClr><a:srgbClr val="0000FF"/></a:bgClr></a:pattFill>`)
	// Strict mode refuses a pattern drawn from its description.
	if _, err := renderRewrittenPNG(t, data, opts, vert); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict pattern: %v", err)
	}
	var warnings []string
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	got := renderSlidePNG(t, data, opts, vert)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "pattern ltVert drawn from its description") {
		t.Fatalf("warnings: %q", warnings)
	}
	// Light vertical lines every fourth pixel from the box's left edge.
	for x, want := range map[int]color.NRGBA{60: red, 61: blue, 63: blue, 64: red, 68: red, 70: blue} {
		if px := renderPixel(t, got, x, 45); px != want {
			t.Fatalf("at %d: %+v, want %+v", x, px, want)
		}
	}
	// Black on white without colors.
	plain := renderSlidePNG(t, data, opts, square(`<a:pattFill prst="horz"/>`))
	if px := renderPixel(t, plain, 65, 40); px != (color.NRGBA{A: 255}) {
		t.Fatalf("default foreground: %+v", px)
	}
	if px := renderPixel(t, plain, 65, 41); px != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("default background: %+v", px)
	}
	// A pattern background covers the slide.
	warnings = nil
	bg := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
		return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgPr><a:pattFill prst="smCheck"><a:fgClr><a:srgbClr val="FF0000"/></a:fgClr><a:bgClr><a:srgbClr val="0000FF"/></a:bgClr></a:pattFill><a:effectLst/></p:bgPr></p:bg><p:spTree>`, 1)
	}})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "pptx: background: render: drawn approximately: render: unsupported operation: pattern smCheck") {
		t.Fatalf("background warnings: %q", warnings)
	}
	for at, want := range map[[2]int]color.NRGBA{{0, 0}: red, {2, 0}: blue, {2, 2}: red, {41, 30}: blue} {
		if px := renderPixel(t, bg, at[0], at[1]); px != want {
			t.Fatalf("background at %v: %+v, want %+v", at, px, want)
		}
	}
}
