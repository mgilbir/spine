package pptxrender

import (
	"errors"
	"image/color"
	"regexp"
	"testing"

	"github.com/mgilbir/spine/render"
)

var (
	renderShapeProperties = regexp.MustCompile(`<p:spPr>.*?</p:spPr>`)
	renderAnyTxBody       = regexp.MustCompile(`<p:txBody>.*</p:txBody>`)
)

// renderShape replaces the 48px square shape at (4,4) with the given geometry
// and properties, without text unless body is set.
func renderShape(spPr, body string) func(string) string {
	return func(s string) string {
		s = renderShapeProperties.ReplaceAllLiteralString(s, `<p:spPr><a:xfrm><a:off x="38100" y="38100"/><a:ext cx="457200" cy="457200"/></a:xfrm>`+spPr+`</p:spPr>`)
		return renderAnyTxBody.ReplaceAllLiteralString(s, body)
	}
}

const (
	renderRed     = `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`
	renderNoFill  = `<a:noFill/>`
	renderOutline = `<a:ln w="38100">` + renderRed // 4px
)

func TestRenderPresetFillsAndOutlines(t *testing.T) {
	data, opts := renderInheritedText(t)
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for _, tc := range []struct {
		name, spPr string
		probes     map[[2]int]color.NRGBA
	}{
		// adj 50000 rounds a square into a circle of radius 24 about (28,28).
		{"round rectangle", `<a:prstGeom prst="roundRect"><a:avLst><a:gd name="adj" fmla="val 50000"/></a:avLst></a:prstGeom>` + renderRed,
			map[[2]int]color.NRGBA{{6, 6}: white, {28, 28}: red, {28, 5}: red}},
		// The default adjustment is 16667: an 8px radius.
		{"default radius", `<a:prstGeom prst="roundRect"><a:avLst/></a:prstGeom>` + renderRed,
			map[[2]int]color.NRGBA{{5, 5}: white, {12, 5}: red}},
		// A centered 4px outline spans x 2 to 6 at the left edge.
		{"mitered outline", `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` + renderNoFill + renderOutline + `<a:miter lim="800000"/></a:ln>`,
			map[[2]int]color.NRGBA{{1, 28}: white, {3, 28}: red, {28, 28}: white, {2, 2}: red}},
		{"inset outline", `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` + renderNoFill + `<a:ln w="38100" algn="in">` + renderRed + `<a:bevel/></a:ln>`,
			map[[2]int]color.NRGBA{{3, 28}: white, {5, 28}: red, {28, 28}: white}},
		{"circle outline", `<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>` + renderNoFill + renderOutline + `</a:ln>`,
			map[[2]int]color.NRGBA{{28, 3}: red, {28, 28}: white, {28, 0}: white}},
		{"rounded outline", `<a:prstGeom prst="roundRect"><a:avLst/></a:prstGeom>` + renderNoFill + renderOutline + `<a:prstDash val="solid"/></a:ln>`,
			map[[2]int]color.NRGBA{{28, 3}: red, {28, 28}: white}},
	} {
		got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderShape(tc.spPr, "")})
		for at, want := range tc.probes {
			if px := renderPixel(t, got, at[0], at[1]); px != want {
				t.Fatalf("%s at %v: %+v, want %+v", tc.name, at, px, want)
			}
		}
	}
	// A round join rounds the outline's outer corner; a miter keeps it sharp.
	round := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderShape(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`+renderNoFill+renderOutline+`<a:round/></a:ln>`, "")})
	if px := renderPixel(t, round, 2, 2); px == red {
		t.Fatal("round join left a sharp corner")
	}
}

func TestRenderTextUsesPresetTextRectangle(t *testing.T) {
	data, opts := renderInheritedText(t)
	// An ellipse's text rectangle is inset by 24 * (1 - cos 45°), about 7px,
	// so left-aligned text starts near x 11 rather than 4.
	body := `<p:txBody><a:bodyPr lIns="0" tIns="0" rIns="0" bIns="0" anchor="ctr"/><a:lstStyle/><a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r></a:p></p:txBody>`
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderShape(`<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>`+renderNoFill, body)})
	if px := renderPixel(t, got, 8, 28); px.R < 255 {
		t.Fatalf("text left of the text rectangle: %+v", px)
	}
	if px := renderPixel(t, got, 18, 28); px.R > 128 {
		t.Fatalf("text missing from the text rectangle: %+v", px)
	}
}

func TestRenderRejectsUnsupportedOutlines(t *testing.T) {
	data, opts := renderInheritedText(t)
	rect := `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` + renderNoFill
	for name, spPr := range map[string]string{
		"unspecified join on a sharp corner": rect + renderOutline + `</a:ln>`,
		"compound":                           rect + `<a:ln w="38100" cmpd="dbl">` + renderRed + `<a:miter/></a:ln>`,
		"no width":                           rect + `<a:ln>` + renderRed + `<a:miter/></a:ln>`,
		"two joins":                          rect + renderOutline + `<a:round/><a:miter/></a:ln>`,
		"elliptical outline":                 `<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>` + renderNoFill + renderOutline + `</a:ln>`,
		"adjustment formula":                 `<a:prstGeom prst="roundRect"><a:avLst><a:gd name="adj" fmla="*/ 1 2 3"/></a:avLst></a:prstGeom>` + renderRed,
		"unknown adjustment":                 `<a:prstGeom prst="roundRect"><a:avLst><a:gd name="adj2" fmla="val 1"/></a:avLst></a:prstGeom>` + renderRed,
		"unknown preset":                     `<a:prstGeom prst="notAShape"><a:avLst/></a:prstGeom>` + renderRed,
	} {
		rewrite := renderShape(spPr, "")
		if name == "elliptical outline" {
			rewrite = func(s string) string {
				return regexp.MustCompile(`cy="457200"`).ReplaceAllLiteralString(renderShape(spPr, "")(s), `cy="228600"`)
			}
		}
		if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": rewrite}); !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestRenderDashedOutlines(t *testing.T) {
	data, opts := renderInheritedText(t)
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	rect := `<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` + renderNoFill
	dash := renderOutline + `<a:prstDash val="dash"/>`
	for _, tc := range []struct {
		name, spPr string
		probes     map[[2]int]color.NRGBA
	}{
		// A 4px "dash" is 16px on and 12px off from the top-left corner,
		// clockwise: the top edge has dashes at x 4-20 and 32-48. The dash
		// over s 84-100 turns the bottom-right corner at (52,52), whose
		// mitered outside reaches 54 and whose inside meets at (50,50).
		{"mitered", rect + dash + `<a:miter lim="800000"/></a:ln>`,
			map[[2]int]color.NRGBA{{12, 4}: red, {26, 4}: white, {40, 4}: red, {53, 53}: red, {51, 51}: red, {49, 49}: white, {3, 3}: white}},
		{"inset", rect + `<a:ln w="38100" algn="in">` + renderRed + `<a:prstDash val="dash"/><a:miter/></a:ln>`,
			map[[2]int]color.NRGBA{{12, 6}: red, {12, 3}: white, {26, 6}: white}},
		// A circle starts at its left and runs clockwise: the top lies in the
		// second dash, and 232° in the first gap.
		{"circle", `<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>` + renderNoFill + dash + `</a:ln>`,
			map[[2]int]color.NRGBA{{28, 3}: red, {13, 9}: white, {28, 28}: white}},
		// dashDot's 44px period ends its last dash exactly at the start, where
		// it overlaps the first dash; the overlap stays painted.
		{"restart", rect + renderOutline + `<a:prstDash val="dashDot"/><a:round/></a:ln>`,
			map[[2]int]color.NRGBA{{4, 4}: red, {5, 5}: red}},
		{"rounded", `<a:prstGeom prst="roundRect"><a:avLst/></a:prstGeom>` + renderNoFill + `<a:ln w="38100" cap="flat">` + renderRed + `<a:prstDash val="sysDot"/></a:ln>`,
			map[[2]int]color.NRGBA{{28, 28}: white}},
	} {
		got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderShape(tc.spPr, "")})
		for at, want := range tc.probes {
			if px := renderPixel(t, got, at[0], at[1]); px != want {
				t.Fatalf("%s at %v: %+v, want %+v", tc.name, at, px, want)
			}
		}
	}
	round := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderShape(rect+dash+`<a:round/></a:ln>`, "")})
	if px := renderPixel(t, round, 53, 53); px == red {
		t.Fatal("round join left a sharp dash corner")
	}
	for name, spPr := range map[string]string{
		"round caps":  rect + `<a:ln w="38100" cap="rnd">` + renderRed + `<a:prstDash val="dash"/><a:miter/></a:ln>`,
		"custom dash": rect + renderOutline + `<a:custDash><a:ds d="100000" sp="100000"/></a:custDash><a:miter/></a:ln>`,
		"no join":     rect + dash + `</a:ln>`,
	} {
		if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderShape(spPr, "")}); !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
