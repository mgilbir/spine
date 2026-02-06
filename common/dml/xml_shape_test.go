// Package dml tests for DrawingML shape property types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_ShapeProperties tests CT_ShapeProperties type (a:spPr)
func TestDML_CT_ShapeProperties(t *testing.T) {
	var v SpPr
	input := `<a:spPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" bwMode="auto">
		<a:xfrm>
			<a:off x="914400" y="914400"/>
			<a:ext cx="1828800" cy="914400"/>
		</a:xfrm>
		<a:prstGeom prst="rect">
			<a:avLst/>
		</a:prstGeom>
		<a:solidFill>
			<a:srgbClr val="FF0000"/>
		</a:solidFill>
		<a:ln w="12700">
			<a:solidFill>
				<a:srgbClr val="000000"/>
			</a:solidFill>
		</a:ln>
	</a:spPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.BwMode != "auto" {
		t.Errorf("BwMode = %q, want auto", v.BwMode)
	}
	if v.Xfrm == nil {
		t.Error("Xfrm is nil")
	}
	if v.PrstGeom == nil {
		t.Error("PrstGeom is nil")
	}
	if v.SolidFill == nil {
		t.Error("SolidFill is nil")
	}
	if v.Ln == nil {
		t.Error("Ln is nil")
	}
}

// TestDML_CT_GroupShapeProperties tests CT_GroupShapeProperties type (a:grpSpPr)
func TestDML_CT_GroupShapeProperties(t *testing.T) {
	var v GrpSpPr
	input := `<a:grpSpPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:xfrm>
			<a:off x="0" y="0"/>
			<a:ext cx="9144000" cy="6858000"/>
			<a:chOff x="0" y="0"/>
			<a:chExt cx="9144000" cy="6858000"/>
		</a:xfrm>
		<a:solidFill>
			<a:schemeClr val="bg1"/>
		</a:solidFill>
	</a:grpSpPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Xfrm == nil {
		t.Error("Xfrm is nil")
	}
	if v.SolidFill == nil {
		t.Error("SolidFill is nil")
	}
}

// TestDML_CT_ShapeStyle tests CT_ShapeStyle type (a:style)
func TestDML_CT_ShapeStyle(t *testing.T) {
	var v Style
	input := `<a:style xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:lnRef idx="2">
			<a:schemeClr val="accent1"/>
		</a:lnRef>
		<a:fillRef idx="1">
			<a:schemeClr val="accent1"/>
		</a:fillRef>
		<a:effectRef idx="0">
			<a:schemeClr val="accent1"/>
		</a:effectRef>
		<a:fontRef idx="minor">
			<a:schemeClr val="tx1"/>
		</a:fontRef>
	</a:style>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.LnRef == nil {
		t.Error("LnRef is nil")
	}
	if v.FillRef == nil {
		t.Error("FillRef is nil")
	}
	if v.EffectRef == nil {
		t.Error("EffectRef is nil")
	}
	if v.FontRef == nil {
		t.Error("FontRef is nil")
	}
}

// TestDML_CT_NonVisualDrawingProps tests CT_NonVisualDrawingProps type (a:cNvPr)
func TestDML_CT_NonVisualDrawingProps(t *testing.T) {
	var v CNvPr
	input := `<a:cNvPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		id="2" name="Shape 1" descr="A description" title="Shape Title" hidden="0"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Id != 2 {
		t.Errorf("Id = %d, want 2", v.Id)
	}
	if v.Name != "Shape 1" {
		t.Errorf("Name = %q, want 'Shape 1'", v.Name)
	}
	if v.Descr != "A description" {
		t.Errorf("Descr = %q, want 'A description'", v.Descr)
	}
}

// TestDML_CT_NonVisualDrawingShapeProps tests CT_NonVisualDrawingShapeProps type (a:cNvSpPr)
func TestDML_CT_NonVisualDrawingShapeProps(t *testing.T) {
	var v CNvSpPr
	input := `<a:cNvSpPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" txBox="1">
		<a:spLocks noGrp="1" noChangeArrowheads="1"/>
	</a:cNvSpPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.TxBox {
		t.Error("TxBox should be true")
	}
	if v.SpLocks == nil {
		t.Error("SpLocks is nil")
	}
}

// TestDML_CT_ShapeLocking tests CT_ShapeLocking type (a:spLocks)
func TestDML_CT_ShapeLocking(t *testing.T) {
	var v SpLocks
	input := `<a:spLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		noGrp="1" noSelect="0" noRot="1" noChangeAspect="1"
		noMove="0" noResize="0" noTextEdit="1"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.NoGrp {
		t.Error("NoGrp should be true")
	}
	if !v.NoRot {
		t.Error("NoRot should be true")
	}
	if !v.NoChangeAspect {
		t.Error("NoChangeAspect should be true")
	}
	if !v.NoTextEdit {
		t.Error("NoTextEdit should be true")
	}
}

// TestDML_CT_PictureLocking tests CT_PictureLocking type (a:picLocks)
func TestDML_CT_PictureLocking(t *testing.T) {
	var v PicLocks
	input := `<a:picLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		noChangeAspect="1" noCrop="1"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.NoChangeAspect {
		t.Error("NoChangeAspect should be true")
	}
	if !v.NoCrop {
		t.Error("NoCrop should be true")
	}
}

// TestDML_CT_GroupLocking tests CT_GroupLocking type (a:grpSpLocks)
func TestDML_CT_GroupLocking(t *testing.T) {
	var v GrpSpLocks
	input := `<a:grpSpLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		noGrp="1" noUngrp="1" noSelect="0"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.NoGrp {
		t.Error("NoGrp should be true")
	}
	if !v.NoUngrp {
		t.Error("NoUngrp should be true")
	}
}

// TestDML_CT_ConnectorLocking tests CT_ConnectorLocking type (a:cxnSpLocks)
func TestDML_CT_ConnectorLocking(t *testing.T) {
	var v CxnSpLocks
	input := `<a:cxnSpLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		noGrp="1" noChangeShapeType="1"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.NoGrp {
		t.Error("NoGrp should be true")
	}
	if !v.NoChangeShapeType {
		t.Error("NoChangeShapeType should be true")
	}
}

// TestDML_CT_Connection tests CT_Connection type (a:stCxn, a:endCxn)
func TestDML_CT_Connection(t *testing.T) {
	var v Cxn
	input := `<a:stCxn xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" id="3" idx="2"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Id != 3 {
		t.Errorf("Id = %d, want 3", v.Id)
	}
	if v.Idx != 2 {
		t.Errorf("Idx = %d, want 2", v.Idx)
	}
}

// TestDML_CT_NonVisualConnectorProperties tests CT_NonVisualConnectorProperties type (a:cNvCxnSpPr)
func TestDML_CT_NonVisualConnectorProperties(t *testing.T) {
	var v CNvCxnSpPr
	input := `<a:cNvCxnSpPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:cxnSpLocks noChangeShapeType="1"/>
		<a:stCxn id="2" idx="0"/>
		<a:endCxn id="3" idx="2"/>
	</a:cNvCxnSpPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.CxnSpLocks == nil {
		t.Error("CxnSpLocks is nil")
	}
	if v.StCxn == nil {
		t.Error("StCxn is nil")
	}
	if v.EndCxn == nil {
		t.Error("EndCxn is nil")
	}
}

// TestDML_CT_NonVisualPictureProperties tests CT_NonVisualPictureProperties type (a:cNvPicPr)
func TestDML_CT_NonVisualPictureProperties(t *testing.T) {
	var v CNvPicPr
	input := `<a:cNvPicPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" preferRelativeResize="1">
		<a:picLocks noChangeAspect="1"/>
	</a:cNvPicPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.PreferRelativeResize {
		t.Error("PreferRelativeResize should be true")
	}
	if v.PicLocks == nil {
		t.Error("PicLocks is nil")
	}
}

// TestDML_CT_NonVisualGroupDrawingShapeProps tests CT_NonVisualGroupDrawingShapeProps type (a:cNvGrpSpPr)
func TestDML_CT_NonVisualGroupDrawingShapeProps(t *testing.T) {
	var v CNvGrpSpPr
	input := `<a:cNvGrpSpPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:grpSpLocks noGrp="1"/>
	</a:cNvGrpSpPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.GrpSpLocks == nil {
		t.Error("GrpSpLocks is nil")
	}
}
