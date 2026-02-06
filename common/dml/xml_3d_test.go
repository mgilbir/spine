// Package dml tests for DrawingML 3D types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_Scene3D tests CT_Scene3D type (a:scene3d)
func TestDML_CT_Scene3D(t *testing.T) {
	var v Scene3d
	input := `<a:scene3d xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:camera prst="orthographicFront">
			<a:rot lat="0" lon="0" rev="0"/>
		</a:camera>
		<a:lightRig rig="threePt" dir="t">
			<a:rot lat="0" lon="0" rev="1200000"/>
		</a:lightRig>
	</a:scene3d>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Camera == nil {
		t.Error("Camera is nil")
	}
	if v.LightRig == nil {
		t.Error("LightRig is nil")
	}
}

// TestDML_CT_Camera tests CT_Camera type (a:camera)
func TestDML_CT_Camera(t *testing.T) {
	var v Camera
	input := `<a:camera xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		prst="perspectiveFront" fov="3600000" zoom="100000">
		<a:rot lat="0" lon="0" rev="0"/>
	</a:camera>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Prst != "perspectiveFront" {
		t.Errorf("Prst = %q, want perspectiveFront", v.Prst)
	}
	if v.Fov != 3600000 {
		t.Errorf("Fov = %d, want 3600000", v.Fov)
	}
	if v.Zoom != 100000 {
		t.Errorf("Zoom = %d, want 100000", v.Zoom)
	}
	if v.Rot == nil {
		t.Error("Rot is nil")
	}
}

// TestDML_CT_LightRig tests CT_LightRig type (a:lightRig)
func TestDML_CT_LightRig(t *testing.T) {
	var v LightRig
	input := `<a:lightRig xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rig="threePt" dir="t">
		<a:rot lat="0" lon="0" rev="1200000"/>
	</a:lightRig>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Rig != "threePt" {
		t.Errorf("Rig = %q, want threePt", v.Rig)
	}
	if v.Dir != "t" {
		t.Errorf("Dir = %q, want t", v.Dir)
	}
	if v.Rot == nil {
		t.Error("Rot is nil")
	}
}

// TestDML_CT_SphereCoords tests CT_SphereCoords type (a:rot)
func TestDML_CT_SphereCoords(t *testing.T) {
	var v Rot3d
	input := `<a:rot xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" lat="0" lon="0" rev="5400000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Lat != 0 {
		t.Errorf("Lat = %d, want 0", v.Lat)
	}
	if v.Lon != 0 {
		t.Errorf("Lon = %d, want 0", v.Lon)
	}
	if v.Rev != 5400000 {
		t.Errorf("Rev = %d, want 5400000", v.Rev)
	}
}

// TestDML_CT_Backdrop tests CT_Backdrop type (a:backdrop)
func TestDML_CT_Backdrop(t *testing.T) {
	var v Backdrop
	input := `<a:backdrop xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:anchor x="0" y="0" z="0"/>
		<a:norm dx="0" dy="0" dz="100000"/>
		<a:up dx="0" dy="100000" dz="0"/>
	</a:backdrop>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Anchor == nil {
		t.Error("Anchor is nil")
	}
	if v.Norm == nil {
		t.Error("Norm is nil")
	}
	if v.Up == nil {
		t.Error("Up is nil")
	}
}

// TestDML_CT_Point3D tests CT_Point3D type (a:anchor)
func TestDML_CT_Point3D(t *testing.T) {
	var v Point3d
	input := `<a:anchor xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" x="914400" y="457200" z="228600"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.X != 914400 {
		t.Errorf("X = %d, want 914400", v.X)
	}
	if v.Y != 457200 {
		t.Errorf("Y = %d, want 457200", v.Y)
	}
	if v.Z != 228600 {
		t.Errorf("Z = %d, want 228600", v.Z)
	}
}

// TestDML_CT_Vector3D tests CT_Vector3D type (a:norm, a:up)
func TestDML_CT_Vector3D(t *testing.T) {
	var v Vector3d
	input := `<a:norm xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" dx="0" dy="0" dz="100000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Dx != 0 {
		t.Errorf("Dx = %d, want 0", v.Dx)
	}
	if v.Dy != 0 {
		t.Errorf("Dy = %d, want 0", v.Dy)
	}
	if v.Dz != 100000 {
		t.Errorf("Dz = %d, want 100000", v.Dz)
	}
}

// TestDML_CT_Shape3D tests CT_Shape3D type (a:sp3d)
func TestDML_CT_Shape3D(t *testing.T) {
	var v Sp3d
	input := `<a:sp3d xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		z="0" extrusionH="76200" contourW="12700" prstMaterial="matte">
		<a:bevelT w="63500" h="25400" prst="relaxedInset"/>
		<a:bevelB w="63500" h="25400"/>
		<a:extrusionClr>
			<a:srgbClr val="000000"/>
		</a:extrusionClr>
		<a:contourClr>
			<a:srgbClr val="000000"/>
		</a:contourClr>
	</a:sp3d>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.ExtrusionH != 76200 {
		t.Errorf("ExtrusionH = %d, want 76200", v.ExtrusionH)
	}
	if v.ContourW != 12700 {
		t.Errorf("ContourW = %d, want 12700", v.ContourW)
	}
	if v.PrstMaterial != "matte" {
		t.Errorf("PrstMaterial = %q, want matte", v.PrstMaterial)
	}
	if v.BevelT == nil {
		t.Error("BevelT is nil")
	}
	if v.BevelB == nil {
		t.Error("BevelB is nil")
	}
}

// TestDML_CT_Bevel tests CT_Bevel type (a:bevelT, a:bevelB)
func TestDML_CT_Bevel(t *testing.T) {
	var v Bevel3d
	input := `<a:bevelT xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" w="63500" h="25400" prst="coolSlant"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.W != 63500 {
		t.Errorf("W = %d, want 63500", v.W)
	}
	if v.H != 25400 {
		t.Errorf("H = %d, want 25400", v.H)
	}
	if v.Prst != "coolSlant" {
		t.Errorf("Prst = %q, want coolSlant", v.Prst)
	}
}
