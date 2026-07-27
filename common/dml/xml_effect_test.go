// Package dml tests for DrawingML effect types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TestDML_CT_EffectList tests CT_EffectList type (a:effectLst)
func TestDML_CT_EffectList(t *testing.T) {
	var v EffectLst
	input := `<a:effectLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:outerShdw blurRad="50800" dist="38100" dir="2700000" algn="tl" rotWithShape="0">
			<a:srgbClr val="000000">
				<a:alpha val="43000"/>
			</a:srgbClr>
		</a:outerShdw>
	</a:effectLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.OuterShdw == nil {
		t.Error("OuterShdw is nil")
	}
}

// TestDML_CT_OuterShadowEffect tests CT_OuterShadowEffect type (a:outerShdw)
func TestDML_CT_OuterShadowEffect(t *testing.T) {
	var v OuterShdw
	input := `<a:outerShdw xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		blurRad="50800" dist="38100" dir="2700000" sx="100000" sy="100000"
		kx="0" ky="0" algn="tl" rotWithShape="0">
		<a:srgbClr val="000000"/>
	</a:outerShdw>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.BlurRad == nil || *v.BlurRad != 50800 {
		t.Errorf("BlurRad = %v, want 50800", v.BlurRad)
	}
	if v.Dist == nil || *v.Dist != 38100 {
		t.Errorf("Dist = %v, want 38100", v.Dist)
	}
	if v.Dir == nil || *v.Dir != 2700000 {
		t.Errorf("Dir = %v, want 2700000", v.Dir)
	}
}

// TestDML_CT_InnerShadowEffect tests CT_InnerShadowEffect type (a:innerShdw)
func TestDML_CT_InnerShadowEffect(t *testing.T) {
	var v InnerShdw
	input := `<a:innerShdw xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		blurRad="63500" dist="50800" dir="2700000">
		<a:srgbClr val="000000">
			<a:alpha val="50000"/>
		</a:srgbClr>
	</a:innerShdw>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.BlurRad == nil || *v.BlurRad != 63500 {
		t.Errorf("BlurRad = %v, want 63500", v.BlurRad)
	}
}

// TestDML_CT_ReflectionEffect tests CT_ReflectionEffect type (a:reflection)
func TestDML_CT_ReflectionEffect(t *testing.T) {
	var v ReflectionXML
	input := `<a:reflection xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		blurRad="6350" stA="50000" endA="300" endPos="55000"
		dist="50800" dir="5400000" fadeDir="5400000"
		sx="100000" sy="-100000" kx="0" ky="0" algn="bl" rotWithShape="0"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.BlurRad == nil || *v.BlurRad != 6350 {
		t.Errorf("BlurRad = %v, want 6350", v.BlurRad)
	}
	if v.StA == nil || v.StA.Int32() != 50000 {
		t.Errorf("StA = %v, want 50000", v.StA)
	}
}

// TestDML_CT_GlowEffect tests CT_GlowEffect type (a:glow)
func TestDML_CT_GlowEffect(t *testing.T) {
	var v GlowXML
	input := `<a:glow xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rad="101600">
		<a:schemeClr val="accent1">
			<a:satMod val="175000"/>
			<a:alpha val="40000"/>
		</a:schemeClr>
	</a:glow>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Rad != 101600 {
		t.Errorf("Rad = %d, want 101600", v.Rad)
	}
}

// TestDML_CT_SoftEdgesEffect tests CT_SoftEdgesEffect type (a:softEdge)
func TestDML_CT_SoftEdgesEffect(t *testing.T) {
	var v SoftEdgeXML
	input := `<a:softEdge xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rad="12700"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Rad != 12700 {
		t.Errorf("Rad = %d, want 12700", v.Rad)
	}
}

// TestDML_CT_BlurEffect tests CT_BlurEffect type (a:blur)
func TestDML_CT_BlurEffect(t *testing.T) {
	var v BlurXML
	input := `<a:blur xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rad="50800" grow="1"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Rad != 50800 {
		t.Errorf("Rad = %d, want 50800", v.Rad)
	}
	if v.Grow == nil || !*v.Grow {
		t.Error("Grow should be true")
	}
}

// TestDML_CT_PresetShadowEffect tests CT_PresetShadowEffect type (a:prstShdw)
func TestDML_CT_PresetShadowEffect(t *testing.T) {
	var v PrstShdw
	input := `<a:prstShdw xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		prst="shdw14" dist="35921" dir="2700000">
		<a:srgbClr val="000000">
			<a:alpha val="50000"/>
		</a:srgbClr>
	</a:prstShdw>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Prst != "shdw14" {
		t.Errorf("Prst = %q, want shdw14", v.Prst)
	}
	if v.Dist != 35921 {
		t.Errorf("Dist = %d, want 35921", v.Dist)
	}
}

// TestDML_CT_AlphaModulateFixedEffect tests CT_AlphaModulateFixedEffect type (a:alphaModFix)
func TestDML_CT_AlphaModulateFixedEffect(t *testing.T) {
	var v AlphaModFix
	input := `<a:alphaModFix xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" amt="50000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Amt == nil || v.Amt.Int32() != 50000 {
		t.Errorf("Amt = %v, want 50000", v.Amt)
	}
}

// TestDML_CT_AlphaModulateFixedEffect_ZeroRoundTrip pins the fix for a
// Common Crawl fidelity failure (pptx slideLayouts with a:alphaModFix amt="0"):
// the amt attribute defaults to 100000 (100%) in the XSD, so an explicit
// amt="0" (0% alpha) means something entirely different from an absent amt and
// must survive a round-trip. A non-pointer Percentage with omitempty dropped
// the strict integer "0" (Val==0, orig==""), silently reinterpreting 0% as the
// 100% default; Amt is therefore a pointer so nil stays omitted while an
// explicit zero is emitted.
func TestDML_CT_AlphaModulateFixedEffect_ZeroRoundTrip(t *testing.T) {
	var v AlphaModFix
	if err := xml.Unmarshal([]byte(`<a:alphaModFix xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" amt="0"/>`), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Amt == nil {
		t.Fatal("explicit amt=\"0\" lost on unmarshal (Amt is nil)")
	}
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, "a")
	b.MarshalElement(NsDrawingML, "alphaModFix", &v)
	if err := b.Err(); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if out := b.String(); !strings.Contains(out, `amt="0"`) {
		t.Errorf("explicit amt=\"0\" dropped on marshal: %s", out)
	}

	// An absent amt must stay omitted (nil pointer), not emit amt="0".
	b2 := xmlb.NewBuilder()
	b2.RegisterNamespace(NsDrawingML, "a")
	b2.MarshalElement(NsDrawingML, "alphaModFix", &AlphaModFix{})
	if out := b2.String(); strings.Contains(out, "amt=") {
		t.Errorf("absent amt emitted: %s", out)
	}
}

// TestDML_EffectContainer_PreservesUnmodeledChildren pins C340 (KNOWN C146):
// CT_EffectContainer (a:cont / a:effectDag) modeled only a:blur, so a blip's
// <a:alphaMod><a:cont>…<a:alphaModFix/>…</a:cont></a:alphaMod> re-marshaled with
// the alphaModFix (and every other unmodeled effect) deleted — and because the
// container is reached through TYPED dispatch it bypassed the raw-capture
// fallback, contradicting BlipEffect's "typed dispatch must never be lossier
// than raw capture". The container now captures unmodeled children raw and
// replays them in document order.
func TestDML_EffectContainer_PreservesUnmodeledChildren(t *testing.T) {
	var am AlphaMod
	input := `<a:alphaMod xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<a:cont type="sib"><a:alphaModFix amt="50000"/><a:blur rad="100"/></a:cont>` +
		`</a:alphaMod>`
	if err := xml.Unmarshal([]byte(input), &am); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if am.Cont == nil {
		t.Fatal("Cont is nil")
	}
	if len(am.Cont.Children) != 2 {
		t.Fatalf("Children = %d, want 2 (alphaModFix raw + blur typed)", len(am.Cont.Children))
	}

	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, "a")
	b.MarshalElement(NsDrawingML, "alphaMod", &am)
	if err := b.Err(); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, `<a:alphaModFix amt="50000"`) {
		t.Errorf("alphaModFix deleted on re-marshal: %s", out)
	}
	if !strings.Contains(out, `<a:blur rad="100"`) {
		t.Errorf("blur dropped on re-marshal: %s", out)
	}
	if !strings.Contains(out, `type="sib"`) {
		t.Errorf("cont type attr dropped: %s", out)
	}
}

// TestDML_EffectDag_PreservesUnmodeledChildren pins the same C340 gap on
// a:effectDag: an unmodeled effect child (a:outerShdw) must survive re-marshal.
func TestDML_EffectDag_PreservesUnmodeledChildren(t *testing.T) {
	var ed EffectDag
	input := `<a:effectDag xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" type="tree">` +
		`<a:cont><a:outerShdw blurRad="40000"><a:srgbClr val="000000"/></a:outerShdw></a:cont>` +
		`</a:effectDag>`
	if err := xml.Unmarshal([]byte(input), &ed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, "a")
	b.MarshalElement(NsDrawingML, "effectDag", &ed)
	if err := b.Err(); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, `<a:outerShdw blurRad="40000"`) {
		t.Errorf("outerShdw deleted on re-marshal: %s", out)
	}
	if !strings.Contains(out, `<a:srgbClr val="000000"`) {
		t.Errorf("nested color dropped: %s", out)
	}
}

// TestDML_CT_BiLevelEffect tests CT_BiLevelEffect type (a:biLevel)
func TestDML_CT_BiLevelEffect(t *testing.T) {
	var v BiLevelXML
	input := `<a:biLevel xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" thresh="50000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Thresh.Int32() != 50000 {
		t.Errorf("Thresh = %d, want 50000", v.Thresh.Int32())
	}
}

// TestDML_CT_GrayscaleEffect tests CT_GrayscaleEffect type (a:grayscl)
func TestDML_CT_GrayscaleEffect(t *testing.T) {
	var v GrayscaleXML
	input := `<a:grayscl xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// Grayscale is an empty element
}

// TestDML_CT_HSLEffect tests CT_HSLEffect type (a:hsl)
func TestDML_CT_HSLEffect(t *testing.T) {
	var v HslXML
	input := `<a:hsl xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" hue="0" sat="100000" lum="0"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Sat.Int32() != 100000 {
		t.Errorf("Sat = %d, want 100000", v.Sat.Int32())
	}
}

// TestDML_CT_LuminanceEffect tests CT_LuminanceEffect type (a:lum)
func TestDML_CT_LuminanceEffect(t *testing.T) {
	var v LumXML
	input := `<a:lum xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" bright="20000" contrast="0"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Bright.Int32() != 20000 {
		t.Errorf("Bright = %d, want 20000", v.Bright.Int32())
	}
}

// TestDML_CT_AlphaBiLevelEffect tests CT_AlphaBiLevelEffect type (a:alphaBiLevel)
func TestDML_CT_AlphaBiLevelEffect(t *testing.T) {
	var v AlphaBiLevel
	input := `<a:alphaBiLevel xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" thresh="50000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Thresh != 50000 {
		t.Errorf("Thresh = %d, want 50000", v.Thresh)
	}
}

// TestDML_CT_AlphaCeilingEffect tests CT_AlphaCeilingEffect type (a:alphaCeiling)
func TestDML_CT_AlphaCeilingEffect(t *testing.T) {
	var v AlphaCeiling
	input := `<a:alphaCeiling xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// AlphaCeiling is an empty element
}

// TestDML_CT_AlphaFloorEffect tests CT_AlphaFloorEffect type (a:alphaFloor)
func TestDML_CT_AlphaFloorEffect(t *testing.T) {
	var v AlphaFloor
	input := `<a:alphaFloor xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// AlphaFloor is an empty element
}

// TestDML_CT_AlphaReplaceEffect tests CT_AlphaReplaceEffect type (a:alphaRepl)
func TestDML_CT_AlphaReplaceEffect(t *testing.T) {
	var v AlphaRepl
	// Use 'dml' as prefix to avoid conflict with the 'a' attribute
	input := `<dml:alphaRepl xmlns:dml="http://schemas.openxmlformats.org/drawingml/2006/main" a="50000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.A != 50000 {
		t.Errorf("A = %d, want 50000", v.A)
	}
}
