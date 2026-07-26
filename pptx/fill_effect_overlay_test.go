package pptx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// firstAutoShape returns the first AutoShape on the reopened slide.
func firstAutoShape(t *testing.T, pres *Presentation) *AutoShape {
	t.Helper()
	for _, sh := range pres.Slides()[0].Shapes() {
		if as, ok := sh.(*AutoShape); ok {
			return as
		}
	}
	t.Fatal("no auto shape materialized from the reopened deck")
	return nil
}

// C263: giving a solid fill to a shape parsed with <a:noFill/> must replace the
// fill atomically — the saved spPr has a solidFill and no noFill (a fill is an
// exclusive choice; both would be schema-invalid). Repeated for gradFill and
// pattFill parses.
func TestSetFillClearsCompetingFillChoice(t *testing.T) {
	cases := []struct {
		name       string
		parsedFill string
		absent     string
	}{
		{"noFill", `<a:noFill/>`, "<a:noFill"},
		{"gradFill", `<a:gradFill><a:gsLst><a:gs pos="0"><a:srgbClr val="FF0000"/></a:gs>` +
			`<a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs></a:gsLst></a:gradFill>`, "<a:gradFill"},
		{"pattFill", `<a:pattFill prst="cross"><a:fgClr><a:srgbClr val="000000"/></a:fgClr>` +
			`<a:bgClr><a:srgbClr val="FFFFFF"/></a:bgClr></a:pattFill>`, "<a:pattFill"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shape := `<p:sp><p:nvSpPr><p:cNvPr id="2" name="Shape 1"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>` +
				`<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="100" cy="100"/></a:xfrm>` +
				`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` + tc.parsedFill + `</p:spPr>` +
				`<p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody></p:sp>`
			data := buildPPTXWithSpTreeBody(t, shape)

			pres := openDeck(t, data)
			firstAutoShape(t, pres).SetFill(dml.NewSolidFill(dml.NewRGB(0x33, 0x66, 0x99).ToColor()))

			out, err := pres.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))
			if !strings.Contains(xml, "<a:solidFill") {
				t.Errorf("solid fill not written:\n%s", xml)
			}
			if strings.Contains(xml, tc.absent) {
				t.Errorf("competing %s not cleared (two fills is schema-invalid):\n%s", tc.absent, xml)
			}
		})
	}
}

// C308: setting a glow on a shape parsed with an outer shadow must merge into
// the effect list, not replace it — the saved effectLst has both the shadow and
// the glow.
func TestSetEffectMergesWithParsedEffectList(t *testing.T) {
	shape := `<p:sp><p:nvSpPr><p:cNvPr id="2" name="Shape 1"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="100" cy="100"/></a:xfrm>` +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` +
		`<a:effectLst><a:outerShdw blurRad="40000" dist="20000" dir="5400000">` +
		`<a:srgbClr val="000000"/></a:outerShdw></a:effectLst></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody></p:sp>`
	data := buildPPTXWithSpTreeBody(t, shape)

	pres := openDeck(t, data)
	firstAutoShape(t, pres).SetGlow(Glow{Color: dml.NewRGB(0xFF, 0xAA, 0x00).ToColor(), Radius: 5})

	out, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "<a:outerShdw") {
		t.Errorf("parsed outer shadow dropped when a glow was added:\n%s", xml)
	}
	if !strings.Contains(xml, "<a:glow") {
		t.Errorf("glow not written:\n%s", xml)
	}
}
