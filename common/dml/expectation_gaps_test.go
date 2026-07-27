package dml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TestColorUnknownChildPreserved covers the §5 expectation gap that
// unmarshalClrColor silently d.Skip-ped anything not in the transform map,
// violating the package's own rule that typed dispatch must never be lossier
// than raw capture.
func TestColorUnknownChildPreserved(t *testing.T) {
	src := `<a:wrap xmlns:a="` + NsDrawingML + `" xmlns:x="urn:x">` +
		`<a:srgbClr val="FF0000"><a:shade val="50000"/><x:futureXf val="7"/><a:alpha val="30000"/></a:srgbClr>` +
		`</a:wrap>`
	var w struct {
		SrgbClr *SrgbClr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr"`
	}
	if err := xmlb.UnmarshalWithSource([]byte(src), &w); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if w.SrgbClr == nil {
		t.Fatal("srgbClr not decoded")
	}
	out := marshalElement(t, "srgbClr", w.SrgbClr)
	if !strings.Contains(out, "futureXf") {
		t.Errorf("unmodeled color child dropped: %s", out)
	}
	// It must be replayed at its captured position, between shade and alpha.
	iShade := strings.Index(out, "shade")
	iFuture := strings.Index(out, "futureXf")
	iAlpha := strings.Index(out, "alpha")
	if iShade > iFuture || iFuture > iAlpha {
		t.Errorf("unmodeled child moved out of document order: %s", out)
	}
}

// TestThemeSlotColorResolvesMoreKinds covers the §5 expectation gap that
// themeSlotColor collapsed every kind but srgbClr/sysClr to the zero Color,
// which callers cannot tell from opaque black.
func TestThemeSlotColorResolvesMoreKinds(t *testing.T) {
	// scrgbClr: percentage channels resolve to an RGB value.
	sc := &ColorChoice{ScrgbClr: &ScRgbClr{
		R: NewPercentage(100000), G: NewPercentage(0), B: NewPercentage(50000),
	}}
	got := themeSlotColor(sc)
	if got.Type != ColorTypeRGB || got.RGB.String() != "FF0080" {
		t.Errorf("scrgbClr slot = %+v (%s), want RGB FF0080", got, got.RGB.String())
	}

	// schemeClr: an alias slot resolves to the theme color it names, not black.
	scheme := &ColorChoice{SchemeClr: &SchemeClrTransform{Val: "accent3"}}
	got = themeSlotColor(scheme)
	if got.Type != ColorTypeTheme || got.Theme != ThemeColorAccent3 {
		t.Errorf("schemeClr slot = %+v, want theme accent3", got)
	}

	// srgbClr and sysClr keep working exactly as before.
	if got := themeSlotColor(&ColorChoice{SrgbClr: &SrgbClr{Val: "112233"}}); got.RGB.String() != "112233" {
		t.Errorf("srgbClr slot = %s, want 112233", got.RGB.String())
	}
	if got := themeSlotColor(&ColorChoice{SysClr: &SystemClr{Val: "windowText", LastClr: "010203"}}); got.RGB.String() != "010203" {
		t.Errorf("sysClr slot = %s, want 010203", got.RGB.String())
	}
}

// TestWspUsesWordprocessingShapeNamespace covers C486: Wsp claimed
// dml-wordprocessingDrawing.xsd but a real wps:wsp — including the one this
// library's own docx builders emit — is in the Microsoft 2010
// wordprocessingShape namespace, so it parsed into an empty struct.
func TestWspUsesWordprocessingShapeNamespace(t *testing.T) {
	src := `<wps:wsp xmlns:wps="` + xmlb.NSWordprocessingShape + `" xmlns:a="` + NsDrawingML + `">` +
		`<wps:cNvPr id="4" name="Shape"/>` +
		`<wps:cNvSpPr/>` +
		`<wps:spPr><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></wps:spPr>` +
		`<wps:bodyPr rot="0"/>` +
		`</wps:wsp>`
	var w Wsp
	if err := xmlb.Unmarshal([]byte(src), &w); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if w.CNvPr == nil {
		t.Fatal("wps:cNvPr not decoded (C486)")
	}
	if w.CNvPr.Id != 4 || w.CNvPr.Name != "Shape" {
		t.Errorf("cNvPr = %+v, want id 4 name Shape", w.CNvPr)
	}
	if w.CNvSpPr == nil {
		t.Error("wps:cNvSpPr not decoded")
	}
	if w.SpPr == nil || w.SpPr.PrstGeom == nil || w.SpPr.PrstGeom.Prst != "rect" {
		t.Errorf("wps:spPr not decoded: %+v", w.SpPr)
	}
	if w.BodyPr == nil {
		t.Error("wps:bodyPr not decoded")
	}
}
