package pptx

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

func TestRenderBestEffortApproximates(t *testing.T) {
	data, opts := renderInheritedText(t)
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	slide := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return renderAddToTree(xml)(renderAnyTxBody.ReplaceAllLiteralString(s, ""))
		}}
	}
	// A blurred shadow 4px to the right of the 10px square at (60,4).
	shadow := strings.Replace(renderSquare(60, 4, "FF0000"), `</p:spPr>`, `<a:effectLst><a:outerShdw blurRad="9525" dist="38100" dir="0"><a:srgbClr val="000000"/></a:outerShdw></a:effectLst></p:spPr>`, 1)
	// A line with a triangle at its end, from (10,10) to (50,10).
	arrow := renderConnectorXML("", renderAcross, strings.Replace(renderRedLine, `</a:ln>`, `<a:tailEnd type="triangle" w="lg" len="lg"/></a:ln>`, 1), "")
	for _, tc := range []struct {
		name, xml, warning string
		probes             map[[2]int]color.NRGBA
	}{
		{"shadow", shadow, "shadow drawn approximately", map[[2]int]color.NRGBA{{65, 9}: red, {72, 9}: {A: 255}, {60, 20}: white}},
		// A large head is five 4px line widths long and wide, so it spans
		// y 0-20 at its base, 20px before the tip.
		{"arrowhead", arrow, "arrowhead triangle", map[[2]int]color.NRGBA{{45, 10}: red, {31, 3}: red, {31, 17}: red, {20, 3}: white}},
	} {
		if _, err := renderRewrittenPNG(t, data, opts, slide(tc.xml)); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("%s strict: %v", tc.name, err)
		}
		var warnings []string
		lenient := opts
		lenient.Warn = func(err error) {
			if !errors.Is(err, render.ErrApproximated) {
				t.Errorf("%s: %v does not mark an approximation", tc.name, err)
			}
			warnings = append(warnings, err.Error())
		}
		got := renderSlidePNG(t, data, lenient, slide(tc.xml))
		if len(warnings) != 1 || !strings.Contains(warnings[0], tc.warning) || !strings.Contains(warnings[0], "drawn approximately") {
			t.Fatalf("%s warnings: %q", tc.name, warnings)
		}
		for at, want := range tc.probes {
			if px := renderPixel(t, got, at[0], at[1]); px != want {
				t.Fatalf("%s at %v: %+v, want %+v", tc.name, at, px, want)
			}
		}
	}
}

func TestRenderBestEffortText(t *testing.T) {
	data, opts := renderInheritedText(t)
	body := func(paragraphs string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, paragraphs)}
	}
	for name, tc := range map[string][2]string{
		"superscript":       {`<a:p><a:r><a:rPr lang="en-US" baseline="30000"/><a:t>A</a:t></a:r></a:p>`, "superscript or subscript size approximated"},
		"overlong word":     {`<a:p><a:r><a:rPr lang="en-US"/><a:t>AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA</a:t></a:r></a:p>`, "text wider than its box"},
	} {
		if _, err := renderRewrittenPNG(t, data, opts, body(tc[0])); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("%s strict: %v", name, err)
		}
		var warnings []string
		lenient := opts
		lenient.Warn = func(err error) { warnings = append(warnings, err.Error()) }
		got := renderSlidePNG(t, data, lenient, body(tc[0]))
		if len(warnings) != 1 || !strings.Contains(warnings[0], tc[1]) {
			t.Fatalf("%s warnings: %q", name, warnings)
		}
		if !renderInk(t, got, 0, 0, 96, 72, func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }) {
			t.Fatalf("%s: text not drawn", name)
		}
	}
}
