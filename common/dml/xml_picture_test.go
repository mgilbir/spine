// Package dml tests for DrawingML picture types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_Blip tests CT_Blip type (a:blip) with effects
func TestDML_CT_Blip(t *testing.T) {
	var v Blip
	input := `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		r:embed="rId1" cstate="print">
		<a:alphaModFix amt="50000"/>
		<a:lum bright="20000" contrast="10000"/>
	</a:blip>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Embed != "rId1" {
		t.Errorf("Embed = %q, want rId1", v.Embed)
	}
	if v.Cstate != "print" {
		t.Errorf("Cstate = %q, want print", v.Cstate)
	}
	if v.AlphaModFix == nil {
		t.Error("AlphaModFix is nil")
	}
	if v.Lum == nil {
		t.Error("Lum is nil")
	}
}

// TestDML_CT_Blip_WithEffects tests CT_Blip with multiple effects
func TestDML_CT_Blip_WithEffects(t *testing.T) {
	var v Blip
	input := `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		r:embed="rId2">
		<a:grayscl/>
		<a:biLevel thresh="50000"/>
		<a:blur rad="25400"/>
	</a:blip>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Grayscl == nil {
		t.Error("Grayscl is nil")
	}
	if v.BiLevel == nil {
		t.Error("BiLevel is nil")
	}
	if v.Blur == nil {
		t.Error("Blur is nil")
	}
}

// TestDML_CT_Blip_LinkedPicture tests CT_Blip with link instead of embed
func TestDML_CT_Blip_LinkedPicture(t *testing.T) {
	var v Blip
	input := `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		r:link="rId3"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Link != "rId3" {
		t.Errorf("Link = %q, want rId3", v.Link)
	}
	if v.Embed != "" {
		t.Errorf("Embed should be empty, got %q", v.Embed)
	}
}

// TestDML_CT_BlipFill tests CT_BlipFillProperties type (a:blipFill)
func TestDML_CT_BlipFill(t *testing.T) {
	var v BlipFill
	input := `<a:blipFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		dpi="96" rotWithShape="1">
		<a:blip r:embed="rId1"/>
		<a:srcRect l="10000" t="10000" r="10000" b="10000"/>
		<a:stretch>
			<a:fillRect/>
		</a:stretch>
	</a:blipFill>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Dpi == nil || *v.Dpi != 96 {
		t.Errorf("Dpi = %v, want 96", v.Dpi)
	}
	if v.RotWithShape == nil || !*v.RotWithShape {
		t.Error("RotWithShape should be true")
	}
	if v.Blip == nil {
		t.Error("Blip is nil")
	}
	if v.SrcRect == nil {
		t.Error("SrcRect is nil")
	}
	if v.Stretch == nil {
		t.Error("Stretch is nil")
	}
}

// TestDML_CT_BlipFill_Tiled tests CT_BlipFillProperties with tile mode
func TestDML_CT_BlipFill_Tiled(t *testing.T) {
	var v BlipFill
	input := `<a:blipFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
		<a:blip r:embed="rId1"/>
		<a:tile tx="0" ty="0" sx="100000" sy="100000" flip="none" algn="tl"/>
	</a:blipFill>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Tile == nil {
		t.Error("Tile is nil")
	}
	if v.Tile != nil && v.Tile.Algn != "tl" {
		t.Errorf("Tile.Algn = %q, want tl", v.Tile.Algn)
	}
}

// TestDML_SrcRect tests source rectangle for cropping
func TestDML_SrcRect(t *testing.T) {
	var v SrcRect
	input := `<a:srcRect xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		l="5000" t="10000" r="5000" b="10000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.L != 5000 {
		t.Errorf("L = %d, want 5000", v.L)
	}
	if v.T != 10000 {
		t.Errorf("T = %d, want 10000", v.T)
	}
}

// TestDML_Stretch tests stretch info
func TestDML_Stretch(t *testing.T) {
	var v Stretch
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

// TestDML_Tile tests tile info
func TestDML_Tile(t *testing.T) {
	var v Tile
	input := `<a:tile xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		tx="914400" ty="914400" sx="50000" sy="50000" flip="xy" algn="ctr"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Tx != 914400 {
		t.Errorf("Tx = %d, want 914400", v.Tx)
	}
	if v.Sx != 50000 {
		t.Errorf("Sx = %d, want 50000", v.Sx)
	}
	if v.Flip != "xy" {
		t.Errorf("Flip = %q, want xy", v.Flip)
	}
	if v.Algn != "ctr" {
		t.Errorf("Algn = %q, want ctr", v.Algn)
	}
}

// TestDML_CropRect tests crop rectangle
func TestDML_CropRect(t *testing.T) {
	var v CropRect
	input := `<cropRect l="15000" t="20000" r="15000" b="20000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.L != 15000 {
		t.Errorf("L = %d, want 15000", v.L)
	}
	if v.T != 20000 {
		t.Errorf("T = %d, want 20000", v.T)
	}
}
