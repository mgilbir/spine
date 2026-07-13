// Package dml tests for DrawingML text types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_TextBody tests CT_TextBody type (a:txBody)
func TestDML_CT_TextBody(t *testing.T) {
	var v TxBody
	input := `<a:txBody xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:bodyPr wrap="square" anchor="ctr"/>
		<a:lstStyle/>
		<a:p>
			<a:r>
				<a:t>Hello World</a:t>
			</a:r>
		</a:p>
	</a:txBody>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.BodyPr == nil {
		t.Error("BodyPr is nil")
	}
	if len(v.P) != 1 {
		t.Errorf("P length = %d, want 1", len(v.P))
	}
}

// TestDML_CT_TextBodyProperties tests CT_TextBodyProperties type (a:bodyPr)
func TestDML_CT_TextBodyProperties(t *testing.T) {
	var v BodyPr
	input := `<a:bodyPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		rot="0" spcFirstLastPara="1" vertOverflow="overflow" horzOverflow="overflow"
		vert="horz" wrap="square" lIns="91440" tIns="45720" rIns="91440" bIns="45720"
		numCol="1" spcCol="0" rtlCol="0" fromWordArt="0" anchor="ctr" anchorCtr="0"
		forceAA="0" upright="0" compatLnSpc="1">
		<a:noAutofit/>
	</a:bodyPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Wrap != "square" {
		t.Errorf("Wrap = %q, want square", v.Wrap)
	}
	if v.Anchor != "ctr" {
		t.Errorf("Anchor = %q, want ctr", v.Anchor)
	}
	if v.LIns == nil || *v.LIns != 91440 {
		t.Errorf("LIns = %v, want 91440", v.LIns)
	}
	if v.NoAutofit == nil {
		t.Error("NoAutofit is nil")
	}
}

// TestDML_CT_TextNormalAutofit tests CT_TextNormalAutofit type (a:normAutofit)
func TestDML_CT_TextNormalAutofit(t *testing.T) {
	var v NormAutofit
	input := `<a:normAutofit xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" fontScale="90000" lnSpcReduction="10000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.FontScale.Int32() != 90000 {
		t.Errorf("FontScale = %d, want 90000", v.FontScale.Int32())
	}
	if v.LnSpcReduction.Int32() != 10000 {
		t.Errorf("LnSpcReduction = %d, want 10000", v.LnSpcReduction.Int32())
	}
}

// TestDML_CT_TextListStyle tests CT_TextListStyle type (a:lstStyle)
func TestDML_CT_TextListStyle(t *testing.T) {
	var v LstStyle
	input := `<a:lstStyle xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:lvl1pPr algn="l" defTabSz="914400">
			<a:defRPr sz="1800"/>
		</a:lvl1pPr>
		<a:lvl2pPr algn="l" defTabSz="914400">
			<a:defRPr sz="1600"/>
		</a:lvl2pPr>
	</a:lstStyle>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Lvl1pPr == nil {
		t.Error("Lvl1pPr is nil")
	}
	if v.Lvl2pPr == nil {
		t.Error("Lvl2pPr is nil")
	}
}

// TestDML_CT_TextParagraph tests CT_TextParagraph type (a:p)
func TestDML_CT_TextParagraph(t *testing.T) {
	var v P
	input := `<a:p xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:pPr algn="ctr"/>
		<a:r>
			<a:rPr lang="en-US" b="1"/>
			<a:t>Bold Text</a:t>
		</a:r>
		<a:endParaRPr lang="en-US"/>
	</a:p>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.PPr == nil {
		t.Error("PPr is nil")
	}
	if len(v.R) != 1 {
		t.Errorf("R length = %d, want 1", len(v.R))
	}
	if v.EndParaRPr == nil {
		t.Error("EndParaRPr is nil")
	}
}

// TestDML_CT_TextParagraphProperties tests CT_TextParagraphProperties type (a:pPr)
func TestDML_CT_TextParagraphProperties(t *testing.T) {
	var v PPr
	input := `<a:pPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		marL="0" marR="0" lvl="0" indent="0" algn="l" defTabSz="914400"
		rtl="0" eaLnBrk="1" fontAlgn="auto" latinLnBrk="0" hangingPunct="1">
		<a:lnSpc>
			<a:spcPct val="100000"/>
		</a:lnSpc>
		<a:spcBef>
			<a:spcPts val="0"/>
		</a:spcBef>
		<a:buNone/>
	</a:pPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Algn != "l" {
		t.Errorf("Algn = %q, want l", v.Algn)
	}
	if v.DefTabSz == nil || *v.DefTabSz != 914400 {
		t.Errorf("DefTabSz = %v, want 914400", v.DefTabSz)
	}
	if v.LnSpc == nil {
		t.Error("LnSpc is nil")
	}
	if v.BuNone == nil {
		t.Error("BuNone is nil")
	}
}

// TestDML_CT_RegularTextRun tests CT_RegularTextRun type (a:r)
func TestDML_CT_RegularTextRun(t *testing.T) {
	var v R
	input := `<a:r xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:rPr lang="en-US" sz="1800" b="1" i="0" u="none"/>
		<a:t>Sample Text</a:t>
	</a:r>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.T != "Sample Text" {
		t.Errorf("T = %q, want Sample Text", v.T)
	}
	if v.RPr == nil {
		t.Error("RPr is nil")
	}
}

// TestDML_CT_TextCharacterProperties tests CT_TextCharacterProperties type (a:rPr)
func TestDML_CT_TextCharacterProperties(t *testing.T) {
	var v RPr
	input := `<a:rPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		lang="en-US" altLang="ja-JP" sz="1800" b="1" i="1" u="sng" strike="noStrike"
		kern="1200" cap="none" spc="0" normalizeH="0" baseline="0" noProof="0"
		dirty="0" err="0" smtClean="0">
		<a:solidFill>
			<a:srgbClr val="FF0000"/>
		</a:solidFill>
		<a:latin typeface="Arial"/>
		<a:ea typeface="+mn-ea"/>
		<a:cs typeface="+mn-cs"/>
	</a:rPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Lang != "en-US" {
		t.Errorf("Lang = %q, want en-US", v.Lang)
	}
	if v.Sz != 1800 {
		t.Errorf("Sz = %d, want 1800", v.Sz)
	}
	if v.B == nil || !*v.B {
		t.Error("B should be true")
	}
	if v.I == nil || !*v.I {
		t.Error("I should be true")
	}
	if v.Latin == nil {
		t.Error("Latin is nil")
	}
}

// TestDML_CT_TextFont tests CT_TextFont type (a:latin, a:ea, a:cs)
func TestDML_CT_TextFont(t *testing.T) {
	var v TextFont
	input := `<a:latin xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		typeface="Calibri" panose="020F0502020204030204" pitchFamily="34" charset="0"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Typeface != "Calibri" {
		t.Errorf("Typeface = %q, want Calibri", v.Typeface)
	}
	if v.PitchFamily == nil || *v.PitchFamily != 34 {
		t.Errorf("PitchFamily = %v, want 34", v.PitchFamily)
	}
}

// TestDML_CT_TextLineBreak tests CT_TextLineBreak type (a:br)
func TestDML_CT_TextLineBreak(t *testing.T) {
	var v Br
	input := `<a:br xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:rPr lang="en-US" sz="1800"/>
	</a:br>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.RPr == nil {
		t.Error("RPr is nil")
	}
}

// TestDML_CT_TextField tests CT_TextField type (a:fld)
func TestDML_CT_TextField(t *testing.T) {
	var v Fld
	input := `<a:fld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" type="slidenum">
		<a:rPr lang="en-US"/>
		<a:t>1</a:t>
	</a:fld>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Type != "slidenum" {
		t.Errorf("Type = %q, want slidenum", v.Type)
	}
	if v.T != "1" {
		t.Errorf("T = %q, want 1", v.T)
	}
}

// TestDML_CT_TextSpacingPercent tests CT_TextSpacingPercent type (a:spcPct)
func TestDML_CT_TextSpacingPercent(t *testing.T) {
	var v SpcPct
	input := `<a:spcPct xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="100000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val.Int32() != 100000 {
		t.Errorf("Val = %d, want 100000", v.Val.Int32())
	}
}

// TestDML_CT_TextSpacingPoint tests CT_TextSpacingPoint type (a:spcPts)
func TestDML_CT_TextSpacingPoint(t *testing.T) {
	var v SpcPts
	input := `<a:spcPts xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="600"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val != 600 {
		t.Errorf("Val = %d, want 600", v.Val)
	}
}

// TestDML_CT_TextAutonumberBullet tests CT_TextAutonumberBullet type (a:buAutoNum)
func TestDML_CT_TextAutonumberBullet(t *testing.T) {
	var v BuAutoNum
	input := `<a:buAutoNum xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" type="arabicPeriod" startAt="1"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Type != "arabicPeriod" {
		t.Errorf("Type = %q, want arabicPeriod", v.Type)
	}
	if v.StartAt != 1 {
		t.Errorf("StartAt = %d, want 1", v.StartAt)
	}
}

// TestDML_CT_TextCharBullet tests CT_TextCharBullet type (a:buChar)
func TestDML_CT_TextCharBullet(t *testing.T) {
	var v BuChar
	input := `<a:buChar xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" char="•"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Char != "•" {
		t.Errorf("Char = %q, want •", v.Char)
	}
}

// TestDML_CT_TextBulletSizePercent tests CT_TextBulletSizePercent type (a:buSzPct)
func TestDML_CT_TextBulletSizePercent(t *testing.T) {
	var v BuSzPct
	input := `<a:buSzPct xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="100000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val.Int32() != 100000 {
		t.Errorf("Val = %d, want 100000", v.Val.Int32())
	}
}

// TestDML_CT_TextBulletSizePoint tests CT_TextBulletSizePoint type (a:buSzPts)
func TestDML_CT_TextBulletSizePoint(t *testing.T) {
	var v BuSzPts
	input := `<a:buSzPts xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="1800"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Val != 1800 {
		t.Errorf("Val = %d, want 1800", v.Val)
	}
}

// TestDML_CT_TextTabStop tests CT_TextTabStop type (a:tab)
func TestDML_CT_TextTabStop(t *testing.T) {
	var v Tab
	input := `<a:tab xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" pos="914400" algn="l"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Pos != 914400 {
		t.Errorf("Pos = %d, want 914400", v.Pos)
	}
	if v.Algn != "l" {
		t.Errorf("Algn = %q, want l", v.Algn)
	}
}

// TestDML_CT_Hyperlink tests CT_Hyperlink type (a:hlinkClick)
func TestDML_CT_Hyperlink(t *testing.T) {
	var v HlinkXML
	input := `<a:hlinkClick xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		r:id="rId1" action="ppaction://hlinksldjump" tooltip="Go to slide 2"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Id == nil || *v.Id != "rId1" {
		t.Errorf("Id = %v, want rId1", v.Id)
	}
	if v.Action != "ppaction://hlinksldjump" {
		t.Errorf("Action = %q, want ppaction://hlinksldjump", v.Action)
	}
	if v.Tooltip != "Go to slide 2" {
		t.Errorf("Tooltip = %q, want 'Go to slide 2'", v.Tooltip)
	}
}

// TestDML_CT_PresetTextShape tests CT_PresetTextShape type (a:prstTxWarp)
func TestDML_CT_PresetTextShape(t *testing.T) {
	var v PrstTxWarp
	input := `<a:prstTxWarp xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" prst="textNoShape">
		<a:avLst/>
	</a:prstTxWarp>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Prst != "textNoShape" {
		t.Errorf("Prst = %q, want textNoShape", v.Prst)
	}
}
