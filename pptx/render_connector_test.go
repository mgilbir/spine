package pptx

import (
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

// renderConnectorXML is a connector at (10,10) px with extent (cx,cy) px.
func renderConnectorXML(xfrm, ext, ln, style string) string {
	return `<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="9" name="Connector"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr><p:spPr><a:xfrm` + xfrm + `><a:off x="95250" y="95250"/>` + ext + `</a:xfrm><a:prstGeom prst="straightConnector1"><a:avLst/></a:prstGeom>` + ln + `</p:spPr>` + style + `</p:cxnSp>`
}

const (
	renderAcross   = `<a:ext cx="381000" cy="0"/>`      // 40px to the right
	renderDiagonal = `<a:ext cx="381000" cy="381000"/>` // 40px right and down
	renderRedLine  = `<a:ln w="38100"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:ln>`
	renderLnStyle  = `<p:style><a:lnRef idx="3"><a:schemeClr val="accent1"/></a:lnRef><a:fillRef idx="0"><a:schemeClr val="accent1"/></a:fillRef><a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef><a:fontRef idx="minor"><a:schemeClr val="tx1"/></a:fontRef></p:style>`
)

func TestRenderConnectors(t *testing.T) {
	data, opts := renderInheritedText(t)
	connector := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
			return strings.Replace(s, `</p:spTree>`, xml+`</p:spTree>`, 1)
		}}
	}
	white, red := color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{R: 255, A: 255}
	for _, tc := range []struct {
		name   string
		xml    string
		probes map[[2]int]color.NRGBA
	}{
		// The theme's third line style is 2.67px wide in the accent1 color.
		{"styled", renderConnectorXML("", renderAcross, `<a:ln/>`, renderLnStyle),
			map[[2]int]color.NRGBA{{30, 9}: {R: 0x15, G: 0x60, B: 0x82, A: 255}, {30, 13}: white}},
		{"own line wins", renderConnectorXML("", renderAcross, renderRedLine, renderLnStyle),
			map[[2]int]color.NRGBA{{30, 10}: red, {30, 11}: red}},
		// Flipped horizontally it runs from (50,10) to (10,50).
		{"flipped", renderConnectorXML(` flipH="1"`, renderDiagonal, renderRedLine, ""),
			map[[2]int]color.NRGBA{{40, 20}: red, {20, 20}: white}},
		{"dashed", renderConnectorXML("", renderAcross, strings.Replace(renderRedLine, `</a:ln>`, `<a:prstDash val="dash"/></a:ln>`, 1), ""),
			map[[2]int]color.NRGBA{{18, 10}: red, {32, 10}: white}},
		{"flat cap", renderConnectorXML("", renderAcross, renderRedLine, ""),
			map[[2]int]color.NRGBA{{8, 9}: white}},
	} {
		got := renderSlidePNG(t, data, opts, connector(tc.xml))
		for at, want := range tc.probes {
			if px := renderPixel(t, got, at[0], at[1]); px != want {
				t.Fatalf("%s at %v: %+v, want %+v", tc.name, at, px, want)
			}
		}
	}
	round := renderSlidePNG(t, data, opts, connector(renderConnectorXML("", renderAcross, strings.Replace(renderRedLine, `<a:ln `, `<a:ln cap="rnd" `, 1), "")))
	if px := renderPixel(t, round, 8, 9); px == white {
		t.Fatal("round cap missing")
	}
	for name, xml := range map[string]string{
		"arrowhead":      renderConnectorXML("", renderAcross, strings.Replace(renderRedLine, `</a:ln>`, `<a:tailEnd type="triangle"/></a:ln>`, 1), ""),
		"bent":           strings.Replace(renderConnectorXML("", renderAcross, renderRedLine, ""), "straightConnector1", "bentConnector3", 1),
		"rotated":        renderConnectorXML(` rot="5400000"`, renderAcross, renderRedLine, ""),
		"shadow style":   renderConnectorXML("", renderAcross, `<a:ln/>`, strings.Replace(renderLnStyle, `<a:effectRef idx="0">`, `<a:effectRef idx="3">`, 1)),
		"missing style":  renderConnectorXML("", renderAcross, `<a:ln/>`, strings.Replace(renderLnStyle, `<a:lnRef idx="3">`, `<a:lnRef idx="9">`, 1)),
		"dash round cap": renderConnectorXML("", renderAcross, strings.Replace(strings.Replace(renderRedLine, `<a:ln `, `<a:ln cap="rnd" `, 1), `</a:ln>`, `<a:prstDash val="dash"/></a:ln>`, 1), ""),
	} {
		if _, err := renderRewrittenPNG(t, data, opts, connector(xml)); !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Connectors are drawn from their saved form only.
	_, s, _, _ := renderTextSlide(t)
	s.AddConnector(ConnectorStraight)
	if _, err := s.PrepareRender(context.Background(), opts); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("unsaved connector: %v", err)
	}
}
