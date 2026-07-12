// Package dml tests for DrawingML extension types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TestDML_CT_OfficeArtExtensionList tests CT_OfficeArtExtensionList type (a:extLst)
func TestDML_CT_OfficeArtExtensionList(t *testing.T) {
	var v ExtLst
	input := `<a:extLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:ext uri="{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}">
			<a16:creationId xmlns:a16="http://schemas.microsoft.com/office/drawing/2014/main" id="{00000000-0008-0000-0000-000002000000}"/>
		</a:ext>
	</a:extLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Ext) != 1 {
		t.Errorf("Ext length = %d, want 1", len(v.Ext))
	}
	if v.Ext[0].CreationId == nil {
		t.Fatal("CreationId should not be nil")
	}
	if v.Ext[0].CreationId.Id != "{00000000-0008-0000-0000-000002000000}" {
		t.Errorf("CreationId.Id = %q", v.Ext[0].CreationId.Id)
	}
}

// TestDML_CT_OfficeArtExtension_CreationId tests typed dispatch for creationId
func TestDML_CT_OfficeArtExtension_CreationId(t *testing.T) {
	var v Ext
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}">
		<a16:creationId xmlns:a16="http://schemas.microsoft.com/office/drawing/2014/main" id="{ABCD1234-5678-9012-3456-789012345678}"/>
	</a:ext>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.URI != "{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}" {
		t.Errorf("URI = %q", v.URI)
	}
	if v.CreationId == nil {
		t.Fatal("CreationId should not be nil")
	}
	if v.CreationId.Id != "{ABCD1234-5678-9012-3456-789012345678}" {
		t.Errorf("CreationId.Id = %q", v.CreationId.Id)
	}
}

// TestDML_CT_OfficeArtExtension_UseLocalDpi tests typed dispatch for useLocalDpi
func TestDML_CT_OfficeArtExtension_UseLocalDpi(t *testing.T) {
	var v Ext
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="{28A0092B-C50C-407E-A947-70E740481C1C}">
		<a14:useLocalDpi xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" val="0"/>
	</a:ext>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.UseLocalDpi == nil {
		t.Fatal("UseLocalDpi should not be nil")
	}
	if v.UseLocalDpi.Val == nil || *v.UseLocalDpi.Val != false {
		t.Errorf("UseLocalDpi.Val = %v, want false", v.UseLocalDpi.Val)
	}
}

// TestDML_CT_OfficeArtExtension_UseLocalDpi_BooleanLexical verifies that the
// xsd:boolean lexical form val="true" parses, which the previous *int32 model
// rejected with a strconv error (C94).
func TestDML_CT_OfficeArtExtension_UseLocalDpi_BooleanLexical(t *testing.T) {
	var v Ext
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="{28A0092B-C50C-407E-A947-70E740481C1C}">
		<a14:useLocalDpi xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" val="true"/>
	</a:ext>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.UseLocalDpi == nil || v.UseLocalDpi.Val == nil || *v.UseLocalDpi.Val != true {
		t.Errorf("UseLocalDpi.Val = %v, want true", v.UseLocalDpi)
	}
}

// TestDML_CT_OfficeArtExtension_HiddenFill tests typed dispatch for hiddenFill
func TestDML_CT_OfficeArtExtension_HiddenFill(t *testing.T) {
	var v Ext
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="{909E8E84-426E-40DD-AFC4-6F175D3DCCD1}">
		<a14:hiddenFill xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main">
			<a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>
		</a14:hiddenFill>
	</a:ext>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.HiddenFill == nil {
		t.Fatal("HiddenFill should not be nil")
	}
	if v.HiddenFill.SolidFill == nil {
		t.Fatal("HiddenFill.SolidFill should not be nil")
	}
	if v.HiddenFill.SolidFill.SrgbClr == nil || v.HiddenFill.SolidFill.SrgbClr.Val != "FFFFFF" {
		t.Error("HiddenFill.SolidFill.SrgbClr should be FFFFFF")
	}
}

// TestDML_CT_OfficeArtExtension_HiddenLine tests typed dispatch for hiddenLine
func TestDML_CT_OfficeArtExtension_HiddenLine(t *testing.T) {
	var v Ext
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="{91240B29-F687-4F45-9708-019B960494DF}">
		<a14:hiddenLine xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main" w="9525">
			<a:solidFill><a:srgbClr val="000000"/></a:solidFill>
			<a:miter lim="800000"/>
			<a:headEnd/>
			<a:tailEnd/>
		</a14:hiddenLine>
	</a:ext>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.HiddenLine == nil {
		t.Fatal("HiddenLine should not be nil")
	}
	if v.HiddenLine.W == nil || *v.HiddenLine.W != 9525 {
		t.Error("HiddenLine.W should be 9525")
	}
	if v.HiddenLine.SolidFill == nil {
		t.Error("HiddenLine.SolidFill should not be nil")
	}
	if v.HiddenLine.Miter == nil {
		t.Error("HiddenLine.Miter should not be nil")
	}
	if v.HiddenLine.HeadEnd == nil {
		t.Error("HiddenLine.HeadEnd should not be nil")
	}
	if v.HiddenLine.TailEnd == nil {
		t.Error("HiddenLine.TailEnd should not be nil")
	}
}

// TestDML_CT_OfficeArtExtension_Unknown tests fallback for unknown extensions
func TestDML_CT_OfficeArtExtension_Unknown(t *testing.T) {
	var v Ext
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="{UNKNOWN-URI}">
		<someContent>inner xml content</someContent>
	</a:ext>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.URI != "{UNKNOWN-URI}" {
		t.Errorf("URI = %q", v.URI)
	}
	if len(v.RawContent) == 0 {
		t.Error("RawContent should not be empty for unknown extensions")
	}
}

// TestDML_CT_OfficeArtExtension_Empty tests empty extension
func TestDML_CT_OfficeArtExtension_Empty(t *testing.T) {
	var v Ext
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="{28A0092B-C50C-407E-A947-70E740481C1C}"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.URI != "{28A0092B-C50C-407E-A947-70E740481C1C}" {
		t.Errorf("URI = %q", v.URI)
	}
}

// TestDML_CreationId tests CreationId extension element
func TestDML_CreationId(t *testing.T) {
	var v CreationId
	input := `<a16:creationId xmlns:a16="http://schemas.microsoft.com/office/drawing/2014/main"
		id="{00000000-0008-0000-0000-000002000000}"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Id != "{00000000-0008-0000-0000-000002000000}" {
		t.Errorf("Id = %q, want {00000000-0008-0000-0000-000002000000}", v.Id)
	}
}

// C189: unmodeled a14:imgEffect children (artistic effects) are captured as
// raw content and re-emitted through the production Builder, so the typed
// imgProps dispatch never loses what the unknown-URI raw fallback preserves.
func TestA14ImgProps_ArtisticEffectRoundTrip(t *testing.T) {
	input := `<extLst xmlns="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:a14="http://schemas.microsoft.com/office/drawing/2010/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<ext uri="{BEBA8EAE-BF5A-486C-A8C5-ECC9F3942E4B}"><a14:imgProps><a14:imgLayer r:embed="rId2">` +
		`<a14:imgEffect><a14:artisticChalkSketch pressure="75000"/></a14:imgEffect>` +
		`<a14:imgEffect><a14:saturation sat="400000"/></a14:imgEffect>` +
		`</a14:imgLayer></a14:imgProps></ext></extLst>`

	var el ExtLst
	if err := xml.Unmarshal([]byte(input), &el); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(el.Ext) != 1 || el.Ext[0].ImgProps == nil || el.Ext[0].ImgProps.ImgLayer == nil {
		t.Fatalf("imgProps not parsed: %+v", el.Ext)
	}
	effects := el.Ext[0].ImgProps.ImgLayer.ImgEffects
	if len(effects) != 2 {
		t.Fatalf("imgEffect count = %d, want 2", len(effects))
	}
	if len(effects[0].Raw) == 0 {
		t.Fatalf("artistic effect not captured as raw content: %+v", effects[0])
	}
	if effects[1].Saturation == nil || effects[1].Saturation.Sat == nil || *effects[1].Saturation.Sat != 400000 {
		t.Errorf("typed saturation effect lost: %+v", effects[1])
	}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(nsA, "extLst", &el)
	out := b.String()
	if !strings.Contains(out, `<a14:artisticChalkSketch pressure="75000"/>`) {
		t.Errorf("artistic effect lost on Builder re-marshal: %s", out)
	}
	if !strings.Contains(out, `<a14:saturation sat="400000"/>`) {
		t.Errorf("typed saturation effect lost on Builder re-marshal: %s", out)
	}
	if strings.Contains(out, "<a14:imgEffect></a14:imgEffect>") || strings.Contains(out, "<a14:imgEffect/>") {
		t.Errorf("empty imgEffect emitted: %s", out)
	}
}
