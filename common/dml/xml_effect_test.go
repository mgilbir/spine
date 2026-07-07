// Package dml tests for DrawingML effect types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
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
	if v.StA == nil || *v.StA != 50000 {
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
	if v.Amt != 50000 {
		t.Errorf("Amt = %d, want 50000", v.Amt)
	}
}

// TestDML_CT_BiLevelEffect tests CT_BiLevelEffect type (a:biLevel)
func TestDML_CT_BiLevelEffect(t *testing.T) {
	var v BiLevelXML
	input := `<a:biLevel xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" thresh="50000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Thresh != 50000 {
		t.Errorf("Thresh = %d, want 50000", v.Thresh)
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
	if v.Sat != 100000 {
		t.Errorf("Sat = %d, want 100000", v.Sat)
	}
}

// TestDML_CT_LuminanceEffect tests CT_LuminanceEffect type (a:lum)
func TestDML_CT_LuminanceEffect(t *testing.T) {
	var v LumXML
	input := `<a:lum xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" bright="20000" contrast="0"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Bright != 20000 {
		t.Errorf("Bright = %d, want 20000", v.Bright)
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
