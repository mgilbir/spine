package pptx

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/presetgeom"
	"github.com/mgilbir/spine/render"
)

func TestRenderPresetGeometries(t *testing.T) {
	data, opts := renderInheritedText(t)
	slide := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return renderAddToTree(xml)(renderAnyTxBody.ReplaceAllLiteralString(s, ""))
		}}
	}
	// A 20px square at (50,10) with a preset geometry.
	shape := func(prst, ln string) string {
		return `<p:sp><p:nvSpPr><p:cNvPr id="90" name="Preset"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="476250" y="95250"/><a:ext cx="190500" cy="190500"/></a:xfrm>` + prst + `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>` + ln + `</p:spPr></p:sp>`
	}
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for _, tc := range []struct {
		name, prst string
		probes     map[[2]int]color.NRGBA
	}{
		// An isosceles triangle with its apex at the top middle.
		{"triangle", `<a:prstGeom prst="triangle"><a:avLst/></a:prstGeom>`, map[[2]int]color.NRGBA{{60, 27}: red, {51, 11}: white, {68, 11}: white}},
		// The default right arrow's shaft spans the middle half of its
		// height; its head, from x 60, narrows to the tip at (70,20).
		{"right arrow", `<a:prstGeom prst="rightArrow"><a:avLst/></a:prstGeom>`, map[[2]int]color.NRGBA{{52, 20}: red, {52, 11}: white, {63, 17}: red, {65, 12}: white}},
		// A thinner shaft through the adjustment.
		{"adjusted arrow", `<a:prstGeom prst="rightArrow"><a:avLst><a:gd name="adj1" fmla="val 20000"/></a:avLst></a:prstGeom>`, map[[2]int]color.NRGBA{{52, 20}: red, {52, 16}: white}},
		{"diamond", `<a:prstGeom prst="diamond"><a:avLst/></a:prstGeom>`, map[[2]int]color.NRGBA{{60, 20}: red, {51, 11}: white}},
	} {
		got := renderSlidePNG(t, data, opts, slide(shape(tc.prst, "")))
		for at, want := range tc.probes {
			if px := renderPixel(t, got, at[0], at[1]); px != want {
				t.Fatalf("%s at %v: %+v, want %+v", tc.name, at, px, want)
			}
		}
	}
	// An outlined preset of several segments is approximate.
	outlined := slide(shape(`<a:prstGeom prst="triangle"><a:avLst/></a:prstGeom>`, `<a:ln w="9525"><a:solidFill><a:srgbClr val="0000FF"/></a:solidFill></a:ln>`))
	if _, err := renderRewrittenPNG(t, data, opts, outlined); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict outline: %v", err)
	}
	// A bent connector draws its three segments in best effort.
	opts.Warn = func(error) {}
	bent := strings.Replace(renderConnectorXML("", renderDiagonal, renderRedLine, ""), "straightConnector1", "bentConnector3", 1)
	got := renderSlidePNG(t, data, opts, slide(bent))
	// From (10,10) right to x 30, down to y 50, right to x 50.
	for at, want := range map[[2]int]color.NRGBA{{20, 10}: red, {30, 30}: red, {40, 50}: red, {20, 30}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("bent connector at %v: %+v, want %+v", at, px, want)
		}
	}
}

// TestRenderAllPresetsEvaluate checks that every standard preset's guides and
// paths evaluate, at a wide and a tall size.
func TestRenderAllPresetsEvaluate(t *testing.T) {
	for _, name := range presetgeom.Names() {
		def, _ := presetgeom.Lookup(name)
		for _, size := range [][2]dml.EMU{{952500, 476250}, {95250, 381000}} {
			paths, _, err := renderCustomGeometry(def, 0, 0, size[0], size[1], 1<<16)
			if err != nil {
				t.Fatalf("%s %v: %v", name, size, err)
			}
			if len(paths) == 0 {
				t.Fatalf("%s: no paths", name)
			}
		}
	}
}
