package pptx

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

func TestRenderAlternateContentFallback(t *testing.T) {
	data, opts := renderInheritedText(t)
	alternate := func(fallback string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
			// The choice needs an extension this renderer does not draw.
			ac := `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" Requires="a14"><p:sp><a14:unknown/></p:sp></mc:Choice><mc:Fallback>` + fallback + `</mc:Fallback></mc:AlternateContent>`
			return strings.Replace(s, `</p:spTree>`, ac+`</p:spTree>`, 1)
		}}
	}
	red := color.NRGBA{R: 255, A: 255}
	got := renderSlidePNG(t, data, opts, alternate(renderSquare(60, 4, "FF0000")))
	if px := renderPixel(t, got, 65, 9); px != red {
		t.Fatalf("fallback: %+v", px)
	}
	// An unsupported fallback fails strictly and is left out in best effort.
	bad := strings.Replace(renderSquare(60, 4, "FF0000"), `<a:prstGeom prst="rect">`, `<a:prstGeom prst="notAShape">`, 1)
	bad = strings.Replace(bad, `<p:spPr>`, `<p:spPr><a:unknownThing/>`, 1)
	if _, err := renderRewrittenPNG(t, data, opts, alternate(bad)); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict bad fallback: %v", err)
	}
	var warnings []string
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	got = renderSlidePNG(t, data, opts, alternate(bad))
	if px := renderPixel(t, got, 65, 9); px == red || len(warnings) == 0 {
		t.Fatalf("lenient bad fallback: %+v %q", px, warnings)
	}
}
