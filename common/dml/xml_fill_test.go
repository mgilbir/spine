// Package dml tests for DrawingML fill types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_SolidColorFillProperties tests CT_SolidColorFillProperties type (a:solidFill)
func TestDML_CT_SolidColorFillProperties(t *testing.T) {
	input := `<a:solidFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:srgbClr val="FF0000"/>
	</a:solidFill>`
	var v SolidFill
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.SrgbClr == nil || v.SrgbClr.Val != "FF0000" {
		t.Error("SrgbClr not properly parsed")
	}
}

// TestDML_CT_GradientFillProperties tests CT_GradientFillProperties type (a:gradFill)
func TestDML_CT_GradientFillProperties(t *testing.T) {
	var v GradFill
	input := `<a:gradFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rotWithShape="1">
		<a:gsLst>
			<a:gs pos="0">
				<a:schemeClr val="phClr">
					<a:tint val="50000"/>
				</a:schemeClr>
			</a:gs>
			<a:gs pos="100000">
				<a:schemeClr val="phClr">
					<a:shade val="50000"/>
				</a:schemeClr>
			</a:gs>
		</a:gsLst>
		<a:lin ang="5400000" scaled="1"/>
	</a:gradFill>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.RotWithShape {
		t.Error("RotWithShape should be true")
	}
	if v.GsLst == nil || len(v.GsLst.Gs) != 2 {
		t.Error("GsLst not properly parsed")
	}
	if v.Lin == nil || v.Lin.Ang != 5400000 {
		t.Error("Lin not properly parsed")
	}
}

// TestDML_CT_GradientStopList tests CT_GradientStopList type (a:gsLst)
func TestDML_CT_GradientStopList(t *testing.T) {
	var v GsLst
	input := `<a:gsLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:gs pos="0">
			<a:srgbClr val="FF0000"/>
		</a:gs>
		<a:gs pos="50000">
			<a:srgbClr val="00FF00"/>
		</a:gs>
		<a:gs pos="100000">
			<a:srgbClr val="0000FF"/>
		</a:gs>
	</a:gsLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Gs) != 3 {
		t.Errorf("Gs length = %d, want 3", len(v.Gs))
	}
}

// TestDML_CT_GradientStop tests CT_GradientStop type (a:gs)
func TestDML_CT_GradientStop(t *testing.T) {
	var v Gs
	input := `<a:gs xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" pos="50000">
		<a:srgbClr val="FF0000"/>
	</a:gs>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Pos != 50000 {
		t.Errorf("Pos = %d, want 50000", v.Pos)
	}
	if v.SrgbClr == nil || v.SrgbClr.Val != "FF0000" {
		t.Error("SrgbClr not properly parsed")
	}
}

// TestDML_CT_LinearShadeProperties tests CT_LinearShadeProperties type (a:lin)
func TestDML_CT_LinearShadeProperties(t *testing.T) {
	var v Lin
	input := `<a:lin xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ang="5400000" scaled="1"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Ang != 5400000 {
		t.Errorf("Ang = %d, want 5400000", v.Ang)
	}
	if !v.Scaled {
		t.Error("Scaled should be true")
	}
}

// TestDML_CT_PathShadeProperties tests CT_PathShadeProperties type (a:path)
func TestDML_CT_PathShadeProperties(t *testing.T) {
	var v PathXML
	input := `<a:path xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" path="circle">
		<a:fillToRect l="50000" t="50000" r="50000" b="50000"/>
	</a:path>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Path != "circle" {
		t.Errorf("Path = %q, want circle", v.Path)
	}
	if v.FillToRect == nil || v.FillToRect.L != 50000 {
		t.Error("FillToRect not properly parsed")
	}
}

// TestDML_CT_PatternFillProperties tests CT_PatternFillProperties type (a:pattFill)
func TestDML_CT_PatternFillProperties(t *testing.T) {
	var v PattFill
	input := `<a:pattFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" prst="ltDnDiag">
		<a:fgClr>
			<a:srgbClr val="000000"/>
		</a:fgClr>
		<a:bgClr>
			<a:srgbClr val="FFFFFF"/>
		</a:bgClr>
	</a:pattFill>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Prst != "ltDnDiag" {
		t.Errorf("Prst = %q, want ltDnDiag", v.Prst)
	}
	if v.FgClr == nil {
		t.Error("FgClr is nil")
	}
	if v.BgClr == nil {
		t.Error("BgClr is nil")
	}
}

// TestDML_CT_BlipFillProperties tests CT_BlipFillProperties type (a:blipFill)
func TestDML_CT_BlipFillProperties(t *testing.T) {
	input := `<a:blipFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" dpi="96" rotWithShape="1">
		<a:blip r:embed="rId1"/>
		<a:srcRect l="10000" t="10000" r="10000" b="10000"/>
		<a:stretch>
			<a:fillRect/>
		</a:stretch>
	</a:blipFill>`
	var v BlipFillXML
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Dpi != 96 {
		t.Errorf("Dpi = %d, want 96", v.Dpi)
	}
	if !v.RotWithShape {
		t.Error("RotWithShape should be true")
	}
	if v.Blip == nil {
		t.Error("Blip is nil")
	}
	if v.SrcRect == nil || v.SrcRect.L != 10000 {
		t.Error("SrcRect not properly parsed")
	}
	if v.Stretch == nil {
		t.Error("Stretch is nil")
	}
}

// TestDML_CT_TileInfoProperties tests CT_TileInfoProperties type (a:tile)
func TestDML_CT_TileInfoProperties(t *testing.T) {
	var v TileXML
	input := `<a:tile xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		tx="0" ty="0" sx="100000" sy="100000" flip="none" algn="tl"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Sx != 100000 {
		t.Errorf("Sx = %d, want 100000", v.Sx)
	}
	if v.Algn != "tl" {
		t.Errorf("Algn = %q, want tl", v.Algn)
	}
}

// TestDML_CT_StretchInfoProperties tests CT_StretchInfoProperties type (a:stretch)
func TestDML_CT_StretchInfoProperties(t *testing.T) {
	var v StretchXML
	input := `<a:stretch xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:fillRect l="0" t="0" r="0" b="0"/>
	</a:stretch>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.FillRect == nil {
		t.Error("FillRect is nil")
	}
}

// TestDML_CT_NoFillProperties tests CT_NoFillProperties type (a:noFill)
func TestDML_CT_NoFillProperties(t *testing.T) {
	var v NoFillXML
	input := `<a:noFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// NoFill is an empty element
}

// TestDML_CT_GroupFillProperties tests CT_GroupFillProperties type (a:grpFill)
func TestDML_CT_GroupFillProperties(t *testing.T) {
	var v GrpFill
	input := `<a:grpFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// GroupFill is an empty element
}

// TestDML_CT_RelativeRect tests CT_RelativeRect type
func TestDML_CT_RelativeRect(t *testing.T) {
	var v RelRect
	input := `<a:fillRect xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" l="10000" t="20000" r="30000" b="40000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.L != 10000 {
		t.Errorf("L = %d, want 10000", v.L)
	}
	if v.T != 20000 {
		t.Errorf("T = %d, want 20000", v.T)
	}
	if v.R != 30000 {
		t.Errorf("R = %d, want 30000", v.R)
	}
	if v.B != 40000 {
		t.Errorf("B = %d, want 40000", v.B)
	}
}
