// Package dml tests for DrawingML color types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_SRgbColor tests CT_SRgbColor type (a:srgbClr)
func TestDML_CT_SRgbColor(t *testing.T) {
	input := `<a:srgbClr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="FF0000"/>`
	var v SrgbClr
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val != "FF0000" {
		t.Errorf("Val = %q, want FF0000", v.Val)
	}
}

// TestDML_CT_SchemeColor tests CT_SchemeColor type (a:schemeClr)
func TestDML_CT_SchemeColor(t *testing.T) {
	input := `<a:schemeClr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="accent1"/>`
	var v SchemeClrTransform
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val != "accent1" {
		t.Errorf("Val = %q, want accent1", v.Val)
	}
}

// TestDML_CT_SystemColor tests CT_SystemColor type (a:sysClr)
func TestDML_CT_SystemColor(t *testing.T) {
	var v SystemClr
	input := `<a:sysClr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="windowText" lastClr="000000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val != "windowText" {
		t.Errorf("Val = %q, want windowText", v.Val)
	}
	if v.LastClr != "000000" {
		t.Errorf("LastClr = %q, want 000000", v.LastClr)
	}
}

// TestDML_CT_HslColor tests CT_HslColor type (a:hslClr)
func TestDML_CT_HslColor(t *testing.T) {
	var v HslClr
	input := `<a:hslClr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" hue="0" sat="100000" lum="50000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Hue != 0 {
		t.Errorf("Hue = %d, want 0", v.Hue)
	}
	if v.Sat.Int32() != 100000 {
		t.Errorf("Sat = %d, want 100000", v.Sat.Int32())
	}
	if v.Lum.Int32() != 50000 {
		t.Errorf("Lum = %d, want 50000", v.Lum.Int32())
	}
}

// TestDML_CT_PresetColor tests CT_PresetColor type (a:prstClr)
func TestDML_CT_PresetColor(t *testing.T) {
	var v PrstClr
	input := `<a:prstClr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="red"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val != "red" {
		t.Errorf("Val = %q, want red", v.Val)
	}
}

// TestDML_CT_ScRgbColor tests CT_ScRgbColor type (a:scrgbClr)
func TestDML_CT_ScRgbColor(t *testing.T) {
	var v ScRgbClr
	input := `<a:scrgbClr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" r="50000" g="50000" b="50000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.R.Int32() != 50000 {
		t.Errorf("R = %d, want 50000", v.R.Int32())
	}
}

// TestDML_CT_SchemeColorWithTransforms tests CT_SchemeColor with color transforms
func TestDML_CT_SchemeColorWithTransforms(t *testing.T) {
	var v SchemeClrTransform
	input := `<a:schemeClr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="phClr">
		<a:tint val="50000"/>
		<a:shade val="75000"/>
		<a:satMod val="300000"/>
		<a:alpha val="50000"/>
	</a:schemeClr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val != "phClr" {
		t.Errorf("Val = %q, want phClr", v.Val)
	}
	if len(v.Tint) == 0 || v.Tint[0].Val.Int32() != 50000 {
		t.Error("Tint not properly parsed")
	}
	if len(v.Shade) == 0 || v.Shade[0].Val.Int32() != 75000 {
		t.Error("Shade not properly parsed")
	}
	if len(v.SatMod) == 0 || v.SatMod[0].Val.Int32() != 300000 {
		t.Error("SatMod not properly parsed")
	}
	if len(v.Alpha) == 0 || v.Alpha[0].Val.Int32() != 50000 {
		t.Error("Alpha not properly parsed")
	}
}

// TestDML_CT_Tint tests tint color transform
func TestDML_CT_Tint(t *testing.T) {
	var v ColorTransform
	input := `<a:tint xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="50000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val.Int32() != 50000 {
		t.Errorf("Val = %d, want 50000", v.Val.Int32())
	}
}
