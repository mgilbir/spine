package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

const tinySVGSheet = `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="24" viewBox="0 0 48 24"><rect width="48" height="24" fill="blue"/></svg>`

func TestAddImageTwoCellAnchor(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	png := testPNG(t, 20, 10)
	if err := sheet.AddImage("B2", png, ImageOptions{ToCell: "D10"}); err != nil {
		t.Fatalf("AddImage two-cell: %v", err)
	}
	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	drawing := zipEntry(t, buf.Bytes(), "xl/drawings/drawing1.xml")

	if !strings.Contains(drawing, "<xdr:twoCellAnchor") {
		t.Errorf("expected twoCellAnchor:\n%s", drawing)
	}
	// from B2 -> col1,row1 ; to D10 -> col3,row9 (0-based)
	if !strings.Contains(drawing, "<xdr:from><xdr:col>1</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row>") {
		t.Errorf("wrong from anchor:\n%s", drawing)
	}
	if !strings.Contains(drawing, "<xdr:to><xdr:col>3</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>9</xdr:row>") {
		t.Errorf("wrong to anchor:\n%s", drawing)
	}
	if strings.Contains(drawing, "oneCellAnchor") {
		t.Error("two-cell image should not emit a oneCellAnchor")
	}
}

func TestAddImageToCellValidation(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	png := testPNG(t, 20, 10)
	// ToCell above/left of the anchor is invalid.
	if err := sheet.AddImage("D10", png, ImageOptions{ToCell: "B2"}); err == nil {
		t.Error("expected error for ToCell left of and above the anchor")
	}
	// Malformed ToCell.
	if err := sheet.AddImage("A1", png, ImageOptions{ToCell: "not-a-cell"}); err == nil {
		t.Error("expected error for malformed ToCell")
	}
}

func TestAddImagePreserveAspect(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	// Intrinsic 40x10 (4:1). Provide width 200 only, preserve aspect -> height 50.
	png := testPNG(t, 40, 10)
	if err := sheet.AddImage("A1", png, ImageOptions{WidthPx: 200, PreserveAspect: true}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	drawing := zipEntry(t, buf.Bytes(), "xl/drawings/drawing1.xml")
	// 200px = 1905000 EMU, 50px = 476250 EMU
	if !strings.Contains(drawing, `cx="1905000" cy="476250"`) {
		t.Errorf("aspect-preserved size wrong (want 200x50 px):\n%s", drawing)
	}
}

func TestAddImageNoPreserveAspectUsesIntrinsic(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	png := testPNG(t, 40, 10)
	// Width only, no PreserveAspect -> height uses intrinsic 10px.
	if err := sheet.AddImage("A1", png, ImageOptions{WidthPx: 200}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	buf, _ := wb.WriteToBuffer()
	drawing := zipEntry(t, buf.Bytes(), "xl/drawings/drawing1.xml")
	// 200px x 10px = 1905000 x 95250
	if !strings.Contains(drawing, `cx="1905000" cy="95250"`) {
		t.Errorf("non-aspect size wrong (want 200x10 px):\n%s", drawing)
	}
}

func TestAddImageSVG(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.AddImage("A1", []byte(tinySVGSheet), ImageOptions{}); err != nil {
		t.Fatalf("AddImage svg: %v", err)
	}
	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	data := buf.Bytes()
	names := zipNames(t, data)

	// Two media parts: raster fallback + SVG.
	if !names["xl/media/image1.png"] {
		t.Error("missing raster fallback part")
	}
	if !names["xl/media/image2.svg"] {
		t.Errorf("missing svg media part; parts=%v", names)
	}

	// The drawing must carry the svgBlip extension; the drawing rels must
	// reference both media parts.
	drawing := zipEntry(t, data, "xl/drawings/drawing1.xml")
	if !strings.Contains(drawing, "asvg:svgBlip") || !strings.Contains(drawing, svgBlipExtURI) {
		t.Errorf("drawing missing svgBlip extension:\n%s", drawing)
	}
	rels := zipEntry(t, data, "xl/drawings/_rels/drawing1.xml.rels")
	if !strings.Contains(rels, "../media/image1.png") || !strings.Contains(rels, "../media/image2.svg") {
		t.Errorf("drawing rels missing a media target:\n%s", rels)
	}
	// Content types must declare svg.
	ct := zipEntry(t, data, "[Content_Types].xml")
	if !strings.Contains(ct, "svg") {
		t.Errorf("[Content_Types].xml missing svg:\n%s", ct)
	}

	// Intrinsic size from the viewBox (48x24) at 96 DPI.
	if !strings.Contains(drawing, `cx="457200" cy="228600"`) {
		t.Errorf("svg intrinsic size wrong (want 48x24 px):\n%s", drawing)
	}

	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}

// TestSVGDetection covers the isSVG sniffer edge cases.
func TestSVGDetection(t *testing.T) {
	cases := []struct {
		name string
		data string
		want bool
	}{
		{"plain", `<svg xmlns="..."></svg>`, true},
		{"xml decl", `<?xml version="1.0"?><svg></svg>`, true},
		{"doctype", `<!DOCTYPE svg><svg></svg>`, true},
		{"leading space", "   \n<svg></svg>", true},
		{"uppercase", `<SVG></SVG>`, true},
		{"not svg", `<html></html>`, false},
		{"png-ish", "\x89PNG\r\n\x1a\n", false},
	}
	for _, tc := range cases {
		if got := isSVG([]byte(tc.data)); got != tc.want {
			t.Errorf("isSVG(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
