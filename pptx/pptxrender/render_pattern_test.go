package pptxrender

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/bits"
	"strings"
	"testing"

	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

func TestRenderPatterns(t *testing.T) {
	// Every preset of ST_PresetPatternVal is in the table, none blank or solid.
	presets := "pct5 pct10 pct20 pct25 pct30 pct40 pct50 pct60 pct70 pct75 pct80 pct90 horz vert ltHorz ltVert dkHorz dkVert narHorz narVert dashHorz dashVert cross dnDiag upDiag ltDnDiag ltUpDiag dkDnDiag dkUpDiag wdDnDiag wdUpDiag dashDnDiag dashUpDiag diagCross smCheck lgCheck smGrid lgGrid dotGrid smConfetti lgConfetti horzBrick diagBrick solidDmnd openDmnd dotDmnd plaid sphere weave divot shingle wave trellis zigZag"
	names := strings.Fields(presets)
	if len(names) != 54 || len(renderPatterns) != 54 {
		t.Fatalf("%d presets, %d bitmaps", len(names), len(renderPatterns))
	}
	set := func(rows [8]uint8) (n int) {
		for _, r := range rows {
			n += bits.OnesCount8(r)
		}
		return n
	}
	for _, p := range names {
		rows, ok := renderPatterns[p]
		if n := set(rows); !ok || n == 0 || n == 64 {
			t.Errorf("%s: %v, %d set", p, ok, n)
		}
	}
	// Percentages grow in density, and the line patterns are what they say.
	last := 0
	for _, p := range []string{"pct5", "pct10", "pct20", "pct25", "pct30", "pct40", "pct50", "pct60", "pct70", "pct75", "pct80", "pct90"} {
		if n := set(renderPatterns[p]); n <= last {
			t.Errorf("%s: %d set after %d", p, n, last)
		} else {
			last = n
		}
	}
	for p, want := range map[string]int{"pct50": 32, "horz": 8, "dkHorz": 32, "ltVert": 16, "narHorz": 32, "dnDiag": 8, "diagCross": 16, "solidDmnd": 25} {
		if n := set(renderPatterns[p]); n != want {
			t.Errorf("%s: %d set, want %d", p, n, want)
		}
	}
	// Diagonals mirror one another.
	for y, r := range renderPatterns["dnDiag"] {
		if bits.Reverse8(r) != renderPatterns["upDiag"][y] {
			t.Errorf("row %d: upDiag is not dnDiag mirrored", y)
		}
	}
}

// renderPatternPNG draws a slide rewritten by rewrites at the given DPI.
func renderPatternPNG(t *testing.T, data []byte, opts render.Options, rewrites map[string]func(string) string, dpi float64) image.Image {
	t.Helper()
	rewritten := renderApply(t, data, rewrites)
	opened, err := pptx.OpenReader(bytes.NewReader(rewritten), int64(len(rewritten)))
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
	var out bytes.Buffer
	if err = page.WritePNG(context.Background(), &out, dpi); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestRenderPatternFills(t *testing.T) {
	data, opts := renderInheritedText(t)
	shape := func(id int, x, y, size int, fill string) string {
		return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Patterned%d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>%s</p:spPr></p:sp>`, id, id, x, y, size, size, fill)
	}
	slide := func(shapes ...string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
			for _, sh := range shapes {
				s = renderAddToTree(sh)(s)
			}
			return s
		}}
	}
	patt := func(prst string) string {
		return `<a:pattFill prst="` + prst + `"><a:fgClr><a:srgbClr val="FF0000"/></a:fgClr><a:bgClr><a:srgbClr val="0000FF"/></a:bgClr></a:pattFill>`
	}
	red, blue := color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}
	at := func(img image.Image, x, y int) color.NRGBA {
		return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	}
	// near allows the layout's rounding of where the image is placed, to 1/64 CSS pixel.
	near := func(a, b color.NRGBA) bool {
		d := func(x, y uint8) bool { return x-y < 32 || y-x < 32 }
		return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && a.A == b.A
	}
	// A pattern pixel is a point, 4 device pixels at 288 DPI, and the tiling
	// starts at the slide's origin: device pixel (X, Y) shows bitmap pixel
	// (X/4 mod 8, Y/4 mod 8). check compares the inside of a box, in EMU,
	// pixel by pixel.
	check := func(t *testing.T, img image.Image, prst string, x, y, size int) {
		t.Helper()
		rows := renderPatterns[prst]
		dev := func(emu int) int { return int(math.Round(float64(emu) / 9525 * 3)) }
		for Y := dev(y) + 2; Y < dev(y+size)-2; Y++ {
			for X := dev(x) + 2; X < dev(x+size)-2; X++ {
				want := blue
				if rows[(Y/4)%8]&(0x80>>((X/4)%8)) != 0 {
					want = red
				}
				if got := at(img, X, Y); !near(got, want) {
					t.Fatalf("%s at %d,%d: %+v, want %+v", prst, X, Y, got, want)
				}
			}
		}
	}
	// Strict mode draws a pattern, exactly: no warning option, no error.
	opts.Warn = nil
	for _, prst := range []string{"pct50", "ltVert", "dnDiag", "wave", "lgConfetti", "sphere"} {
		img := renderPatternPNG(t, data, opts, slide(shape(90, 571500, 381000, 152400, patt(prst))), 288)
		check(t, img, prst, 571500, 381000, 152400)
	}
	// The phase comes from the slide, not the shape: shapes at other
	// offsets show the one pattern, as one surface would.
	img := renderPatternPNG(t, data, opts, slide(
		shape(90, 571500, 381000, 152400, patt("smCheck")),
		shape(91, 300000, 400000, 150000, patt("smCheck")),
		shape(92, 234567, 150001, 127000, patt("smCheck")),
	), 288)
	check(t, img, "smCheck", 571500, 381000, 152400)
	check(t, img, "smCheck", 300000, 400000, 150000)
	check(t, img, "smCheck", 234567, 150001, 127000)
	// At 96 DPI a point is 1.33 pixels: the pixel at css x shows the bitmap
	// pixel at floor(0.75 x).
	img96 := renderPatternPNG(t, data, opts, slide(shape(90, 571500, 381000, 152400, patt("ltVert"))), 96)
	for x := 61; x < 74; x++ {
		if math.Floor(0.75*float64(x)) != math.Floor(0.75*float64(x+1)) {
			// Straddles two pattern pixels, so is a blend of them.
			continue
		}
		want := blue
		if renderPatterns["ltVert"][0]&(0x80>>(int(math.Floor(0.75*float64(x)))%8)) != 0 {
			want = red
		}
		if got := at(img96, x, 45); got != want {
			t.Fatalf("96 DPI at %d: %+v, want %+v", x, got, want)
		}
	}
	// Black on white without colors. The box starts at y 40 css pixels, point
	// 30, row 6 of the bitmap, so horz's set row is at point 32: device
	// pixels 128 to 131.
	plain := renderPatternPNG(t, data, opts, slide(shape(90, 571500, 381000, 152400, `<a:pattFill prst="horz"/>`)), 288)
	if px := at(plain, 200, 129); px != (color.NRGBA{A: 255}) {
		t.Fatalf("default foreground: %+v", px)
	}
	if px := at(plain, 200, 140); px != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("default background: %+v", px)
	}
	// A pattern background covers the slide, from its origin, and is exact.
	var warnings []string
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	bg := renderPatternPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
		return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgPr>`+patt("smCheck")+`<a:effectLst/></p:bgPr></p:bg><p:spTree>`, 1)
	}}, 96)
	if len(warnings) != 0 {
		t.Fatalf("background warnings: %q", warnings)
	}
	// A pixel inside one pattern pixel, 0.75 per CSS pixel, shows it; smCheck's
	// rows 0 and 3 are 0x99.
	for pt, want := range map[[2]int]color.NRGBA{{0, 0}: red, {3, 0}: blue, {4, 0}: red, {7, 0}: blue, {8, 0}: blue, {11, 0}: red, {0, 4}: red, {4, 4}: red, {3, 4}: blue} {
		if got := at(bg, pt[0], pt[1]); got != want {
			t.Fatalf("background at %v: %+v, want %+v", pt, got, want)
		}
	}
	// PowerPoint never turns or mirrors a pattern: a rotated or flipped
	// shape, or one in a rotated group, shows the slide's upright pattern
	// under it, with no warning and in strict mode too. disk compares a disk
	// well inside the shape, around its centre.
	disk := func(t *testing.T, img image.Image, prst string, cx, cy, radius int) {
		t.Helper()
		rows := renderPatterns[prst]
		dev := func(emu int) float64 { return float64(emu) / 9525 * 3 }
		r := dev(radius) - 4
		n := 0
		for Y := int(dev(cy) - r); Y <= int(dev(cy)+r); Y++ {
			for X := int(dev(cx) - r); X <= int(dev(cx)+r); X++ {
				if math.Hypot(float64(X)+.5-dev(cx), float64(Y)+.5-dev(cy)) > r {
					continue
				}
				n++
				want := blue
				if rows[(Y/4)%8]&(0x80>>((X/4)%8)) != 0 {
					want = red
				}
				if got := at(img, X, Y); !near(got, want) {
					t.Fatalf("%s turned at %d,%d: %+v, want %+v", prst, X, Y, got, want)
				}
			}
		}
		if n < 1000 {
			t.Fatalf("%s: only %d pixels compared", prst, n)
		}
	}
	const size = 152400
	for _, xfrm := range []string{` rot="1800000"`, ` rot="5400000"`, ` flipH="1"`, ` flipV="1"`, ` rot="2700000" flipH="1"`} {
		for _, prst := range []string{"dnDiag", "lgConfetti", "dashHorz"} {
			turned := slide(strings.Replace(shape(90, 571500, 381000, size, patt(prst)), `<a:xfrm>`, `<a:xfrm`+xfrm+`>`, 1))
			img := renderPatternPNG(t, data, opts, turned, 288)
			disk(t, img, prst, 571500+size/2, 381000+size/2, size/2)
		}
	}
	warnings = nil
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	rotated := slide(strings.Replace(shape(90, 571500, 381000, size, patt("ltVert")), `<a:xfrm>`, `<a:xfrm rot="1800000">`, 1))
	renderSlidePNG(t, data, opts, rotated)
	if len(warnings) != 0 {
		t.Fatalf("rotated warnings: %q", warnings)
	}
	opts.Warn = nil
	// A rotated group moves its shapes, not their patterns, and a shape
	// turned inside a turned group is placed by both.
	inner := shape(90, 571500, 381000, size, patt("dnDiag"))
	group := func(rot string, child string) string {
		return `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="95" name="Turned"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm rot="` + rot + `"><a:off x="500000" y="300000"/><a:ext cx="300000" cy="300000"/><a:chOff x="500000" y="300000"/><a:chExt cx="300000" cy="300000"/></a:xfrm></p:grpSpPr>` + child + `</p:grpSp>`
	}
	// The group turns about (650000, 450000), so the child's centre
	// (647700, 457200) lands at (642800, 447700) after a quarter turn.
	img = renderPatternPNG(t, data, opts, slide(group("5400000", inner)), 288)
	disk(t, img, "dnDiag", 642800, 447700, size/2)
	// A turn of 30 degrees takes the centre's offset (-2300, 7200) to
	// (-2300 cos - 7200 sin, -2300 sin + 7200 cos).
	c, s := math.Cos(math.Pi/6), math.Sin(math.Pi/6)
	img = renderPatternPNG(t, data, opts, slide(group("1800000", strings.Replace(inner, `<a:xfrm>`, `<a:xfrm rot="2700000" flipV="1">`, 1))), 288)
	disk(t, img, "dnDiag", 650000+int(math.Round(-2300*c-7200*s)), 450000+int(math.Round(-2300*s+7200*c)), size/2)
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	renderSlidePNG(t, data, opts, slide(group("1800000", inner)))
	if len(warnings) != 0 {
		t.Fatalf("rotated group warnings: %q", warnings)
	}
}
