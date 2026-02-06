// Package dml tests for DrawingML geometry types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_PresetGeometry2D tests CT_PresetGeometry2D type (a:prstGeom)
func TestDML_CT_PresetGeometry2D(t *testing.T) {
	var v PrstGeom
	input := `<a:prstGeom xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" prst="rect">
		<a:avLst/>
	</a:prstGeom>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Prst != "rect" {
		t.Errorf("Prst = %q, want rect", v.Prst)
	}
}

// TestDML_CT_CustomGeometry2D tests CT_CustomGeometry2D type (a:custGeom)
func TestDML_CT_CustomGeometry2D(t *testing.T) {
	var v CustGeom
	input := `<a:custGeom xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:avLst/>
		<a:gdLst/>
		<a:ahLst/>
		<a:cxnLst/>
		<a:rect l="l" t="t" r="r" b="b"/>
		<a:pathLst>
			<a:path w="100" h="100">
				<a:moveTo>
					<a:pt x="0" y="0"/>
				</a:moveTo>
				<a:lnTo>
					<a:pt x="100" y="100"/>
				</a:lnTo>
				<a:close/>
			</a:path>
		</a:pathLst>
	</a:custGeom>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.PathLst == nil {
		t.Error("PathLst is nil")
	}
}

// TestDML_CT_GeomGuideList tests CT_GeomGuideList type (a:avLst, a:gdLst)
func TestDML_CT_GeomGuideList(t *testing.T) {
	var v AvLst
	input := `<a:avLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:gd name="adj1" fmla="val 25000"/>
		<a:gd name="adj2" fmla="val 50000"/>
	</a:avLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Gd) != 2 {
		t.Errorf("Gd length = %d, want 2", len(v.Gd))
	}
}

// TestDML_CT_GeomGuide tests CT_GeomGuide type (a:gd)
func TestDML_CT_GeomGuide(t *testing.T) {
	var v Gd
	input := `<a:gd xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="adj1" fmla="val 25000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Name != "adj1" {
		t.Errorf("Name = %q, want adj1", v.Name)
	}
	if v.Fmla != "val 25000" {
		t.Errorf("Fmla = %q, want val 25000", v.Fmla)
	}
}

// TestDML_CT_Path2DList tests CT_Path2DList type (a:pathLst)
func TestDML_CT_Path2DList(t *testing.T) {
	var v PathLst
	input := `<a:pathLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:path w="100" h="100">
			<a:moveTo>
				<a:pt x="0" y="0"/>
			</a:moveTo>
		</a:path>
	</a:pathLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Path) != 1 {
		t.Errorf("Path length = %d, want 1", len(v.Path))
	}
}

// TestDML_CT_Path2D tests CT_Path2D type (a:path)
func TestDML_CT_Path2D(t *testing.T) {
	var v PathXML2D
	input := `<a:path xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		w="914400" h="914400" fill="norm" stroke="1" extrusionOk="0">
		<a:moveTo>
			<a:pt x="0" y="0"/>
		</a:moveTo>
		<a:lnTo>
			<a:pt x="914400" y="0"/>
		</a:lnTo>
		<a:lnTo>
			<a:pt x="914400" y="914400"/>
		</a:lnTo>
		<a:close/>
	</a:path>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.W != 914400 {
		t.Errorf("W = %d, want 914400", v.W)
	}
	if v.H != 914400 {
		t.Errorf("H = %d, want 914400", v.H)
	}
}

// TestDML_CT_Path2DMoveTo tests CT_Path2DMoveTo type (a:moveTo)
func TestDML_CT_Path2DMoveTo(t *testing.T) {
	var v MoveToXML
	input := `<a:moveTo xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:pt x="100" y="200"/>
	</a:moveTo>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Pt == nil {
		t.Error("Pt is nil")
	}
}

// TestDML_CT_Path2DLineTo tests CT_Path2DLineTo type (a:lnTo)
func TestDML_CT_Path2DLineTo(t *testing.T) {
	var v LnToXML
	input := `<a:lnTo xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:pt x="500" y="600"/>
	</a:lnTo>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Pt == nil {
		t.Error("Pt is nil")
	}
}

// TestDML_CT_Path2DArcTo tests CT_Path2DArcTo type (a:arcTo)
func TestDML_CT_Path2DArcTo(t *testing.T) {
	var v ArcToXML
	input := `<a:arcTo xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		wR="50000" hR="50000" stAng="0" swAng="5400000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.WR != 50000 {
		t.Errorf("WR = %d, want 50000", v.WR)
	}
	if v.SwAng != 5400000 {
		t.Errorf("SwAng = %d, want 5400000", v.SwAng)
	}
}

// TestDML_CT_Path2DQuadBezierTo tests CT_Path2DQuadBezierTo type (a:quadBezTo)
func TestDML_CT_Path2DQuadBezierTo(t *testing.T) {
	var v QuadBezToXML
	input := `<a:quadBezTo xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:pt x="100" y="100"/>
		<a:pt x="200" y="200"/>
	</a:quadBezTo>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Pt) != 2 {
		t.Errorf("Pt length = %d, want 2", len(v.Pt))
	}
}

// TestDML_CT_Path2DCubicBezierTo tests CT_Path2DCubicBezierTo type (a:cubicBezTo)
func TestDML_CT_Path2DCubicBezierTo(t *testing.T) {
	var v CubicBezToXML
	input := `<a:cubicBezTo xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:pt x="100" y="100"/>
		<a:pt x="200" y="200"/>
		<a:pt x="300" y="300"/>
	</a:cubicBezTo>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Pt) != 3 {
		t.Errorf("Pt length = %d, want 3", len(v.Pt))
	}
}

// TestDML_CT_Path2DClose tests CT_Path2DClose type (a:close)
func TestDML_CT_Path2DClose(t *testing.T) {
	var v CloseXML
	input := `<a:close xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// Close is an empty element
}

// TestDML_CT_AdjPoint2D tests CT_AdjPoint2D type (a:pt)
func TestDML_CT_AdjPoint2D(t *testing.T) {
	var v PtXML
	input := `<a:pt xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" x="914400" y="457200"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.X != "914400" {
		t.Errorf("X = %q, want 914400", v.X)
	}
	if v.Y != "457200" {
		t.Errorf("Y = %q, want 457200", v.Y)
	}
}

// TestDML_CT_GeomRect tests CT_GeomRect type (a:rect)
func TestDML_CT_GeomRect(t *testing.T) {
	var v RectXML
	input := `<a:rect xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" l="l" t="t" r="r" b="b"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.L != "l" {
		t.Errorf("L = %q, want l", v.L)
	}
	if v.R != "r" {
		t.Errorf("R = %q, want r", v.R)
	}
}

// TestDML_CT_Transform2D tests CT_Transform2D type (a:xfrm)
func TestDML_CT_Transform2D(t *testing.T) {
	var v Xfrm
	input := `<a:xfrm xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rot="5400000" flipH="1" flipV="0">
		<a:off x="914400" y="914400"/>
		<a:ext cx="1828800" cy="914400"/>
	</a:xfrm>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Rot != 5400000 {
		t.Errorf("Rot = %d, want 5400000", v.Rot)
	}
	if !v.FlipH {
		t.Error("FlipH should be true")
	}
	if v.Off == nil {
		t.Error("Off is nil")
	}
	if v.Ext == nil {
		t.Error("Ext is nil")
	}
}

// TestDML_CT_Point2D tests CT_Point2D type (a:off)
func TestDML_CT_Point2D(t *testing.T) {
	var v OffXML
	input := `<a:off xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" x="914400" y="457200"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.X != 914400 {
		t.Errorf("X = %d, want 914400", v.X)
	}
	if v.Y != 457200 {
		t.Errorf("Y = %d, want 457200", v.Y)
	}
}

// TestDML_CT_PositiveSize2D tests CT_PositiveSize2D type (a:ext)
func TestDML_CT_PositiveSize2D(t *testing.T) {
	var v ExtXML
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" cx="1828800" cy="914400"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Cx != 1828800 {
		t.Errorf("Cx = %d, want 1828800", v.Cx)
	}
	if v.Cy != 914400 {
		t.Errorf("Cy = %d, want 914400", v.Cy)
	}
}

// TestDML_CT_GroupTransform2D tests CT_GroupTransform2D type (a:xfrm for groups)
func TestDML_CT_GroupTransform2D(t *testing.T) {
	var v GrpXfrm
	input := `<a:xfrm xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rot="0" flipH="0" flipV="0">
		<a:off x="0" y="0"/>
		<a:ext cx="9144000" cy="6858000"/>
		<a:chOff x="0" y="0"/>
		<a:chExt cx="9144000" cy="6858000"/>
	</a:xfrm>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.ChOff == nil {
		t.Error("ChOff is nil")
	}
	if v.ChExt == nil {
		t.Error("ChExt is nil")
	}
}
