package pptxrender

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

func TestRenderInheritedAlternateContent(t *testing.T) {
	data, opts := renderInheritedText(t)
	ac := func(fallback string) func(string) string {
		return func(s string) string {
			x := `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" Requires="a14"><p:sp><a14:unknown/></p:sp></mc:Choice><mc:Fallback>` + fallback + `</mc:Fallback></mc:AlternateContent>`
			return strings.Replace(s, `</p:spTree>`, x+`</p:spTree>`, 1)
		}
	}
	noText := func(s string) string { return renderAnyTxBody.ReplaceAllLiteralString(s, "") }
	red, blue := color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}
	// The master's fallback square at (60,4), the layout's at (64,30).
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{
		"ppt/slides/slide1.xml": noText,
		renderMasterPart:        ac(renderSquare(60, 4, "FF0000")),
		renderLayoutPart:        ac(renderSquare(64, 30, "0000FF")),
	})
	if px := renderPixel(t, got, 65, 9); px != red {
		t.Fatalf("master fallback: %+v", px)
	}
	if px := renderPixel(t, got, 69, 35); px != blue {
		t.Fatalf("layout fallback: %+v", px)
	}
	// A fallback's placeholders on a layout are prompts.
	prompt := `<p:sp><p:nvSpPr><p:cNvPr id="7" name="Title"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="571500" y="38100"/><a:ext cx="95250" cy="95250"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></p:spPr></p:sp>`
	got = renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": noText, renderLayoutPart: ac(prompt)})
	if px := renderPixel(t, got, 65, 9); px == red {
		t.Fatal("layout prompt drawn")
	}
	// An unsupported fallback fails when drawn, strictly, and is left out
	// in best effort.
	bad := strings.Replace(renderSquare(60, 4, "FF0000"), `<p:spPr>`, `<p:spPr><a:unknownThing/>`, 1)
	rewrites := map[string]func(string) string{"ppt/slides/slide1.xml": noText, renderMasterPart: ac(bad)}
	if _, err := renderRewrittenPNG(t, data, opts, rewrites); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict bad fallback: %v", err)
	}
	var warnings []string
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	got = renderSlidePNG(t, data, opts, rewrites)
	if px := renderPixel(t, got, 65, 9); px == red || len(warnings) != 1 {
		t.Fatalf("lenient bad fallback: %+v %q", px, warnings)
	}
}

func TestRenderSlideRootAlternateContent(t *testing.T) {
	data, opts := renderInheritedText(t)
	// PowerPoint wraps a 2010 transition in alternate content at the slide
	// root, after the color map override, with a plain transition as its
	// fallback.
	root := func(fallback string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			ac := `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" Requires="p14"><p:transition spd="slow" p14:dur="1250"><p14:vortex dir="r"/></p:transition></mc:Choice><mc:Fallback>` + fallback + `</mc:Fallback></mc:AlternateContent>`
			if !strings.Contains(s, `</p:clrMapOvr>`) {
				t.Fatalf("no color map override in %s", s)
			}
			return strings.Replace(s, `</p:clrMapOvr>`, `</p:clrMapOvr>`+ac, 1)
		}}
	}
	// Strict mode refuses it, as it does a plain transition.
	if _, err := renderRewrittenPNG(t, data, opts, root(`<p:transition spd="slow"><p:fade/></p:transition>`)); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict transition: %v", err)
	}
	var warnings []string
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	renderSlidePNG(t, data, opts, root(`<p:transition spd="slow"><p:fade/></p:transition>`))
	if len(warnings) != 0 {
		t.Fatalf("transition: %q", warnings)
	}
	// Anything else in the fallback is reported.
	warnings = nil
	renderSlidePNG(t, data, opts, root(`<p:extLst/>`))
	if len(warnings) == 0 {
		t.Fatal("unknown root fallback not reported")
	}
}
