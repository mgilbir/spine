// Package dml tests for DrawingML theme types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_OfficeStyleSheet tests CT_OfficeStyleSheet type (a:theme)
func TestDML_CT_OfficeStyleSheet(t *testing.T) {
	var v Theme
	input := `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">
		<a:themeElements>
			<a:clrScheme name="Office">
				<a:dk1>
					<a:sysClr val="windowText" lastClr="000000"/>
				</a:dk1>
				<a:lt1>
					<a:sysClr val="window" lastClr="FFFFFF"/>
				</a:lt1>
			</a:clrScheme>
			<a:fontScheme name="Office">
				<a:majorFont>
					<a:latin typeface="Calibri Light"/>
				</a:majorFont>
				<a:minorFont>
					<a:latin typeface="Calibri"/>
				</a:minorFont>
			</a:fontScheme>
			<a:fmtScheme name="Office">
				<a:fillStyleLst>
					<a:solidFill>
						<a:schemeClr val="phClr"/>
					</a:solidFill>
				</a:fillStyleLst>
			</a:fmtScheme>
		</a:themeElements>
	</a:theme>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Name != "Office Theme" {
		t.Errorf("Name = %q, want 'Office Theme'", v.Name)
	}
	if v.ThemeElements == nil {
		t.Error("ThemeElements is nil")
	}
}

// TestDML_CT_BaseStyles tests CT_BaseStyles type (a:themeElements)
func TestDML_CT_BaseStyles(t *testing.T) {
	var v ThemeElements
	input := `<a:themeElements xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:clrScheme name="Office"/>
		<a:fontScheme name="Office"/>
		<a:fmtScheme name="Office"/>
	</a:themeElements>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.ClrScheme == nil {
		t.Error("ClrScheme is nil")
	}
	if v.FontScheme == nil {
		t.Error("FontScheme is nil")
	}
	if v.FmtScheme == nil {
		t.Error("FmtScheme is nil")
	}
}

// TestDML_CT_ColorScheme tests CT_ColorScheme type (a:clrScheme)
func TestDML_CT_ColorScheme(t *testing.T) {
	var v ClrScheme
	input := `<a:clrScheme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office">
		<a:dk1>
			<a:sysClr val="windowText" lastClr="000000"/>
		</a:dk1>
		<a:lt1>
			<a:sysClr val="window" lastClr="FFFFFF"/>
		</a:lt1>
		<a:dk2>
			<a:srgbClr val="44546A"/>
		</a:dk2>
		<a:lt2>
			<a:srgbClr val="E7E6E6"/>
		</a:lt2>
		<a:accent1>
			<a:srgbClr val="4472C4"/>
		</a:accent1>
		<a:accent2>
			<a:srgbClr val="ED7D31"/>
		</a:accent2>
		<a:hlink>
			<a:srgbClr val="0563C1"/>
		</a:hlink>
		<a:folHlink>
			<a:srgbClr val="954F72"/>
		</a:folHlink>
	</a:clrScheme>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Name != "Office" {
		t.Errorf("Name = %q, want Office", v.Name)
	}
	if v.Dk1 == nil {
		t.Error("Dk1 is nil")
	}
	if v.Lt1 == nil {
		t.Error("Lt1 is nil")
	}
	if v.Accent1 == nil {
		t.Error("Accent1 is nil")
	}
	if v.Hlink == nil {
		t.Error("Hlink is nil")
	}
}

// TestDML_CT_FontScheme tests CT_FontScheme type (a:fontScheme)
func TestDML_CT_FontScheme(t *testing.T) {
	var v FontScheme
	input := `<a:fontScheme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office">
		<a:majorFont>
			<a:latin typeface="Calibri Light" panose="020F0302020204030204"/>
			<a:ea typeface=""/>
			<a:cs typeface=""/>
			<a:font script="Jpan" typeface="游ゴシック Light"/>
			<a:font script="Hang" typeface="맑은 고딕"/>
		</a:majorFont>
		<a:minorFont>
			<a:latin typeface="Calibri" panose="020F0502020204030204"/>
			<a:ea typeface=""/>
			<a:cs typeface=""/>
		</a:minorFont>
	</a:fontScheme>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Name != "Office" {
		t.Errorf("Name = %q, want Office", v.Name)
	}
	if v.MajorFont == nil {
		t.Error("MajorFont is nil")
	}
	if v.MinorFont == nil {
		t.Error("MinorFont is nil")
	}
}

// TestDML_CT_FontCollection tests CT_FontCollection type (a:majorFont, a:minorFont)
func TestDML_CT_FontCollection(t *testing.T) {
	var v FontCollection
	input := `<a:majorFont xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:latin typeface="Calibri Light" panose="020F0302020204030204"/>
		<a:ea typeface="+mn-ea"/>
		<a:cs typeface="+mn-cs"/>
		<a:font script="Jpan" typeface="游ゴシック Light"/>
		<a:font script="Hang" typeface="맑은 고딕"/>
	</a:majorFont>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Latin == nil {
		t.Error("Latin is nil")
	}
	if len(v.Font) != 2 {
		t.Errorf("Font length = %d, want 2", len(v.Font))
	}
}

// TestDML_CT_SupplementalFont tests CT_SupplementalFont type (a:font)
func TestDML_CT_SupplementalFont(t *testing.T) {
	var v SupplementalFont
	input := `<a:font xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" script="Jpan" typeface="游ゴシック Light"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Script != "Jpan" {
		t.Errorf("Script = %q, want Jpan", v.Script)
	}
	if v.Typeface != "游ゴシック Light" {
		t.Errorf("Typeface = %q, want '游ゴシック Light'", v.Typeface)
	}
}

// TestDML_CT_StyleMatrix tests CT_StyleMatrix type (a:fmtScheme)
func TestDML_CT_StyleMatrix(t *testing.T) {
	var v FmtScheme
	input := `<a:fmtScheme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office">
		<a:fillStyleLst>
			<a:solidFill>
				<a:schemeClr val="phClr"/>
			</a:solidFill>
			<a:gradFill rotWithShape="1">
				<a:gsLst>
					<a:gs pos="0">
						<a:schemeClr val="phClr"/>
					</a:gs>
				</a:gsLst>
			</a:gradFill>
		</a:fillStyleLst>
		<a:lnStyleLst>
			<a:ln w="6350" cap="flat" cmpd="sng" algn="ctr">
				<a:solidFill>
					<a:schemeClr val="phClr"/>
				</a:solidFill>
			</a:ln>
		</a:lnStyleLst>
		<a:effectStyleLst>
			<a:effectStyle>
				<a:effectLst/>
			</a:effectStyle>
		</a:effectStyleLst>
		<a:bgFillStyleLst>
			<a:solidFill>
				<a:schemeClr val="phClr"/>
			</a:solidFill>
		</a:bgFillStyleLst>
	</a:fmtScheme>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Name != "Office" {
		t.Errorf("Name = %q, want Office", v.Name)
	}
	if v.FillStyleLst == nil {
		t.Error("FillStyleLst is nil")
	}
	if v.LnStyleLst == nil {
		t.Error("LnStyleLst is nil")
	}
	if v.EffectStyleLst == nil {
		t.Error("EffectStyleLst is nil")
	}
	if v.BgFillStyleLst == nil {
		t.Error("BgFillStyleLst is nil")
	}
}

// TestDML_CT_FillStyleList tests CT_FillStyleList type (a:fillStyleLst)
func TestDML_CT_FillStyleList(t *testing.T) {
	var v FillStyleLst
	input := `<a:fillStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:solidFill>
			<a:schemeClr val="phClr"/>
		</a:solidFill>
		<a:gradFill rotWithShape="1">
			<a:gsLst>
				<a:gs pos="0">
					<a:schemeClr val="phClr"/>
				</a:gs>
			</a:gsLst>
		</a:gradFill>
	</a:fillStyleLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.SolidFill) != 1 {
		t.Errorf("SolidFill length = %d, want 1", len(v.SolidFill))
	}
	if len(v.GradFill) != 1 {
		t.Errorf("GradFill length = %d, want 1", len(v.GradFill))
	}
}

// TestDML_CT_LineStyleList tests CT_LineStyleList type (a:lnStyleLst)
func TestDML_CT_LineStyleList(t *testing.T) {
	var v LnStyleLst
	input := `<a:lnStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:ln w="6350" cap="flat">
			<a:solidFill>
				<a:schemeClr val="phClr"/>
			</a:solidFill>
		</a:ln>
		<a:ln w="12700" cap="flat">
			<a:solidFill>
				<a:schemeClr val="phClr"/>
			</a:solidFill>
		</a:ln>
	</a:lnStyleLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Ln) != 2 {
		t.Errorf("Ln length = %d, want 2", len(v.Ln))
	}
}

// TestDML_CT_EffectStyleList tests CT_EffectStyleList type (a:effectStyleLst)
func TestDML_CT_EffectStyleList(t *testing.T) {
	var v EffectStyleLst
	input := `<a:effectStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectStyle>
			<a:effectLst/>
		</a:effectStyle>
		<a:effectStyle>
			<a:effectLst>
				<a:outerShdw blurRad="40000" dist="23000" dir="5400000" rotWithShape="0">
					<a:srgbClr val="000000">
						<a:alpha val="35000"/>
					</a:srgbClr>
				</a:outerShdw>
			</a:effectLst>
		</a:effectStyle>
	</a:effectStyleLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.EffectStyle) != 2 {
		t.Errorf("EffectStyle length = %d, want 2", len(v.EffectStyle))
	}
}

// TestDML_CT_EffectStyleItem tests CT_EffectStyleItem type (a:effectStyle)
func TestDML_CT_EffectStyleItem(t *testing.T) {
	var v EffectStyle
	input := `<a:effectStyle xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectLst>
			<a:outerShdw blurRad="40000" dist="23000" dir="5400000">
				<a:srgbClr val="000000"/>
			</a:outerShdw>
		</a:effectLst>
		<a:scene3d>
			<a:camera prst="orthographicFront"/>
			<a:lightRig rig="threePt" dir="t"/>
		</a:scene3d>
		<a:sp3d prstMaterial="matte"/>
	</a:effectStyle>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.EffectLst == nil {
		t.Error("EffectLst is nil")
	}
	if v.Scene3d == nil {
		t.Error("Scene3d is nil")
	}
	if v.Sp3d == nil {
		t.Error("Sp3d is nil")
	}
}

// TestDML_CT_ColorMapping tests CT_ColorMapping type (a:clrMap)
func TestDML_CT_ColorMapping(t *testing.T) {
	var v ClrMap
	input := `<a:clrMap xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2"
		accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4"
		accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Bg1 != "lt1" {
		t.Errorf("Bg1 = %q, want lt1", v.Bg1)
	}
	if v.Tx1 != "dk1" {
		t.Errorf("Tx1 = %q, want dk1", v.Tx1)
	}
	if v.Accent1 != "accent1" {
		t.Errorf("Accent1 = %q, want accent1", v.Accent1)
	}
}

// TestDML_CT_ColorMappingOverride tests CT_ColorMappingOverride type (a:clrMapOvr)
func TestDML_CT_ColorMappingOverride(t *testing.T) {
	var v ClrMapOvr
	input := `<a:clrMapOvr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:masterClrMapping/>
	</a:clrMapOvr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.MasterClrMapping == nil {
		t.Error("MasterClrMapping is nil")
	}
}

// TestDML_CT_ObjectStyleDefaults tests CT_ObjectStyleDefaults type (a:objectDefaults)
func TestDML_CT_ObjectStyleDefaults(t *testing.T) {
	var v ObjectDefaults
	input := `<a:objectDefaults xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:spDef>
			<a:spPr>
				<a:solidFill>
					<a:schemeClr val="accent1"/>
				</a:solidFill>
			</a:spPr>
			<a:bodyPr/>
			<a:lstStyle/>
		</a:spDef>
	</a:objectDefaults>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.SpDef == nil {
		t.Error("SpDef is nil")
	}
}

// TestDML_CT_DefaultShapeDefinition tests CT_DefaultShapeDefinition type (a:spDef)
func TestDML_CT_DefaultShapeDefinition(t *testing.T) {
	var v DefaultShapeDefinition
	input := `<a:spDef xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:spPr>
			<a:solidFill>
				<a:schemeClr val="accent1"/>
			</a:solidFill>
			<a:ln w="12700">
				<a:solidFill>
					<a:schemeClr val="tx1"/>
				</a:solidFill>
			</a:ln>
		</a:spPr>
		<a:bodyPr wrap="square" anchor="ctr"/>
		<a:lstStyle>
			<a:defPPr algn="ctr"/>
		</a:lstStyle>
		<a:style>
			<a:lnRef idx="2">
				<a:schemeClr val="accent1"/>
			</a:lnRef>
		</a:style>
	</a:spDef>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.SpPr == nil {
		t.Error("SpPr is nil")
	}
	if v.BodyPr == nil {
		t.Error("BodyPr is nil")
	}
	if v.LstStyle == nil {
		t.Error("LstStyle is nil")
	}
	if v.Style == nil {
		t.Error("Style is nil")
	}
}
