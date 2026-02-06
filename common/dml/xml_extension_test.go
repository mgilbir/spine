// Package dml tests for DrawingML extension types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
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
}

// TestDML_CT_OfficeArtExtension tests CT_OfficeArtExtension type (a:ext)
func TestDML_CT_OfficeArtExtension(t *testing.T) {
	var v Ext
	input := `<a:ext xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}">
		<someContent>inner xml content</someContent>
	</a:ext>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.URI != "{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}" {
		t.Errorf("URI = %q, want {FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}", v.URI)
	}
	if len(v.InnerXML) == 0 {
		t.Error("InnerXML should not be empty")
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
		t.Errorf("URI = %q, want {28A0092B-C50C-407E-A947-70E740481C1C}", v.URI)
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

// TestDML_CompatExt tests compatibility extension
func TestDML_CompatExt(t *testing.T) {
	var v CompatExt
	input := `<compatExt spId="_x0000_s1025"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.SpId != "_x0000_s1025" {
		t.Errorf("SpId = %q, want _x0000_s1025", v.SpId)
	}
}
