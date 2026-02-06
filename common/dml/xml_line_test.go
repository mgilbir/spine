// Package dml tests for DrawingML line/stroke types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_LineProperties tests CT_LineProperties type (a:ln)
func TestDML_CT_LineProperties(t *testing.T) {
	var v Ln
	input := `<a:ln xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		w="12700" cap="flat" cmpd="sng" algn="ctr">
		<a:solidFill>
			<a:srgbClr val="000000"/>
		</a:solidFill>
		<a:prstDash val="solid"/>
		<a:round/>
		<a:headEnd type="none"/>
		<a:tailEnd type="triangle" w="med" len="med"/>
	</a:ln>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.W != 12700 {
		t.Errorf("W = %d, want 12700", v.W)
	}
	if v.Cap != "flat" {
		t.Errorf("Cap = %q, want flat", v.Cap)
	}
	if v.SolidFill == nil {
		t.Error("SolidFill is nil")
	}
	if v.PrstDash == nil {
		t.Error("PrstDash is nil")
	}
	if v.Round == nil {
		t.Error("Round is nil")
	}
	if v.TailEnd == nil {
		t.Error("TailEnd is nil")
	}
}

// TestDML_CT_PresetLineDashProperties tests CT_PresetLineDashProperties type (a:prstDash)
func TestDML_CT_PresetLineDashProperties(t *testing.T) {
	var v PrstDash
	input := `<a:prstDash xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="dash"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val != "dash" {
		t.Errorf("Val = %q, want dash", v.Val)
	}
}

// TestDML_CT_DashStopList tests CT_DashStopList type (a:custDash)
func TestDML_CT_DashStopList(t *testing.T) {
	var v CustDash
	input := `<a:custDash xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:ds d="300000" sp="100000"/>
		<a:ds d="100000" sp="100000"/>
	</a:custDash>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Ds) != 2 {
		t.Errorf("Ds length = %d, want 2", len(v.Ds))
	}
}

// TestDML_CT_DashStop tests CT_DashStop type (a:ds)
func TestDML_CT_DashStop(t *testing.T) {
	var v Ds
	input := `<a:ds xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" d="300000" sp="100000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.D != 300000 {
		t.Errorf("D = %d, want 300000", v.D)
	}
	if v.Sp != 100000 {
		t.Errorf("Sp = %d, want 100000", v.Sp)
	}
}

// TestDML_CT_LineJoinMiterProperties tests CT_LineJoinMiterProperties type (a:miter)
func TestDML_CT_LineJoinMiterProperties(t *testing.T) {
	var v Miter
	input := `<a:miter xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" lim="800000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Lim != 800000 {
		t.Errorf("Lim = %d, want 800000", v.Lim)
	}
}

// TestDML_CT_LineEndProperties tests CT_LineEndProperties type (a:headEnd, a:tailEnd)
func TestDML_CT_LineEndProperties(t *testing.T) {
	var v LineEnd
	input := `<a:tailEnd xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" type="arrow" w="lg" len="lg"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Type != "arrow" {
		t.Errorf("Type = %q, want arrow", v.Type)
	}
	if v.W != "lg" {
		t.Errorf("W = %q, want lg", v.W)
	}
	if v.Len != "lg" {
		t.Errorf("Len = %q, want lg", v.Len)
	}
}

// TestDML_CT_StyleMatrixReference_LnRef tests CT_StyleMatrixReference type (a:lnRef)
func TestDML_CT_StyleMatrixReference_LnRef(t *testing.T) {
	var v LnRef
	input := `<a:lnRef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" idx="2">
		<a:schemeClr val="accent1"/>
	</a:lnRef>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Idx != 2 {
		t.Errorf("Idx = %d, want 2", v.Idx)
	}
	if v.SchemeClr == nil {
		t.Error("SchemeClr is nil")
	}
}

// TestDML_CT_StyleMatrixReference_FillRef tests CT_StyleMatrixReference type (a:fillRef)
func TestDML_CT_StyleMatrixReference_FillRef(t *testing.T) {
	var v FillRef
	input := `<a:fillRef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" idx="1">
		<a:schemeClr val="accent1"/>
	</a:fillRef>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Idx != 1 {
		t.Errorf("Idx = %d, want 1", v.Idx)
	}
}

// TestDML_CT_StyleMatrixReference_EffectRef tests CT_StyleMatrixReference type (a:effectRef)
func TestDML_CT_StyleMatrixReference_EffectRef(t *testing.T) {
	var v StyleEffectRef
	input := `<a:effectRef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" idx="0">
		<a:schemeClr val="accent1"/>
	</a:effectRef>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Idx != 0 {
		t.Errorf("Idx = %d, want 0", v.Idx)
	}
}

// TestDML_CT_FontReference tests CT_FontReference type (a:fontRef)
func TestDML_CT_FontReference(t *testing.T) {
	var v FontRef
	input := `<a:fontRef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" idx="minor">
		<a:schemeClr val="tx1"/>
	</a:fontRef>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Idx != "minor" {
		t.Errorf("Idx = %q, want minor", v.Idx)
	}
	if v.SchemeClr == nil {
		t.Error("SchemeClr is nil")
	}
}

// TestDML_CT_LineJoinRound tests CT_LineJoinRound type (a:round)
func TestDML_CT_LineJoinRound(t *testing.T) {
	var v Round
	input := `<a:round xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// Round is an empty element
}

// TestDML_CT_LineJoinBevel tests CT_LineJoinBevel type (a:bevel)
func TestDML_CT_LineJoinBevel(t *testing.T) {
	var v Bevel
	input := `<a:bevel xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// Bevel is an empty element
}
