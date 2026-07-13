package xlsx

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func zipNames(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	return names
}

func zipEntry(t *testing.T, data []byte, name string) string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if err := rc.Close(); err != nil {
				t.Fatalf("close %s: %v", name, err)
			}
			return buf.String()
		}
	}
	t.Fatalf("zip entry %q not found", name)
	return ""
}

func TestAddImageEmitsDrawingParts(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if _, err := sheet.Cell("A1"); err != nil {
		t.Fatalf("seed cell: %v", err)
	}

	pngData := testPNG(t, 20, 10)
	if err := sheet.AddImage("B2", pngData, ImageOptions{WidthPx: 200, HeightPx: 100}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}

	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	data := buf.Bytes()

	names := zipNames(t, data)
	for _, want := range []string{
		"xl/media/image1.png",
		"xl/drawings/drawing1.xml",
		"xl/worksheets/_rels/sheet1.xml.rels",
		"xl/drawings/_rels/drawing1.xml.rels",
	} {
		if !names[want] {
			t.Errorf("missing package part %q", want)
		}
	}

	// Content types must declare the drawing part and the png default.
	ct := zipEntry(t, data, "[Content_Types].xml")
	if !strings.Contains(ct, "drawing+xml") {
		t.Errorf("content types missing drawing override:\n%s", ct)
	}
	if !strings.Contains(ct, `Extension="png"`) {
		t.Errorf("content types missing png default:\n%s", ct)
	}

	// Worksheet references the drawing.
	ws := zipEntry(t, data, "xl/worksheets/sheet1.xml")
	if !strings.Contains(ws, "<drawing") || !strings.Contains(ws, `r:id="rId1"`) {
		t.Errorf("worksheet missing <drawing r:id>:\n%s", ws)
	}

	// Drawing anchors the image at B2 (col 1, row 1, 0-based) and embeds rId1.
	drawing := zipEntry(t, data, "xl/drawings/drawing1.xml")
	for _, want := range []string{
		"xdr:oneCellAnchor",
		"<xdr:col>1</xdr:col>",
		"<xdr:row>1</xdr:row>",
		`cx="1905000"`, // 200px * 9525
		`cy="952500"`,  // 100px * 9525
		`r:embed="rId1"`,
	} {
		if !strings.Contains(drawing, want) {
			t.Errorf("drawing1.xml missing %q:\n%s", want, drawing)
		}
	}

	// Drawing rels point at the media image.
	rels := zipEntry(t, data, "xl/drawings/_rels/drawing1.xml.rels")
	if !strings.Contains(rels, "../media/image1.png") {
		t.Errorf("drawing rels missing media target:\n%s", rels)
	}

	// The saved workbook must still be openable.
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen workbook with image: %v", err)
	}
}

func TestAddImageDefaultsToIntrinsicSize(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	pngData := testPNG(t, 40, 25)
	if err := sheet.AddImage("A1", pngData, ImageOptions{}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	data := buf.Bytes()
	drawing := zipEntry(t, data, "xl/drawings/drawing1.xml")
	if !strings.Contains(drawing, `cx="381000"`) { // 40px * 9525
		t.Errorf("expected intrinsic width 40px in EMU, got:\n%s", drawing)
	}
	if !strings.Contains(drawing, `cy="238125"`) { // 25px * 9525
		t.Errorf("expected intrinsic height 25px in EMU, got:\n%s", drawing)
	}
}

func TestAddImageRejectsUnknownFormat(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.AddImage("A1", []byte("not an image"), ImageOptions{}); err == nil {
		t.Fatal("expected error for unsupported image format")
	}
}

func TestAddImageGIF(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 16, 16))
	var buf bytes.Buffer
	if err := gif.Encode(&buf, src, nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.AddImage("A1", buf.Bytes(), ImageOptions{}); err != nil {
		t.Fatalf("AddImage gif: %v", err)
	}
	out, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	if !zipNames(t, out.Bytes())["xl/media/image1.gif"] {
		t.Error("expected xl/media/image1.gif media part")
	}
}

func TestAddImageSingleAxisUsesIntrinsicForOther(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	// Source is 40x25; setting only WidthPx must keep the intrinsic height.
	if err := sheet.AddImage("A1", testPNG(t, 40, 25), ImageOptions{WidthPx: 300}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	out, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	drawing := zipEntry(t, out.Bytes(), "xl/drawings/drawing1.xml")
	if !strings.Contains(drawing, `cx="2857500"`) { // 300px * 9525
		t.Errorf("expected explicit width 300px, got:\n%s", drawing)
	}
	if !strings.Contains(drawing, `cy="238125"`) { // intrinsic 25px * 9525
		t.Errorf("expected intrinsic height 25px, got:\n%s", drawing)
	}
}

func TestAddImageRejectsBadInputs(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	png := testPNG(t, 8, 8)

	cases := []struct {
		name    string
		cellRef string
		data    []byte
		opts    ImageOptions
	}{
		{"empty data", "A1", nil, ImageOptions{}},
		{"bad cell ref", "not-a-ref", png, ImageOptions{}},
		{"row out of range", "A1048577", png, ImageOptions{}},
		{"width over cap", "A1", png, ImageOptions{WidthPx: maxImagePixels + 1, HeightPx: 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := sheet.AddImage(tc.cellRef, tc.data, tc.opts); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

// TestAddImageMultipleSheetsAndImages exercises the relationship-id and
// part-name allocation across several images on several sheets — the case most
// likely to produce collisions. media parts are workbook-global (image1..N)
// while drawing relationship ids are drawing-local (rId1 per drawing).
func TestAddImageMultipleSheetsAndImages(t *testing.T) {
	wb := Create()
	s1 := wb.AddSheet("One")
	if err := s1.AddImage("A1", testPNG(t, 10, 10), ImageOptions{}); err != nil {
		t.Fatalf("s1 img1: %v", err)
	}
	if err := s1.AddImage("C3", testPNG(t, 12, 12), ImageOptions{}); err != nil {
		t.Fatalf("s1 img2: %v", err)
	}
	s2 := wb.AddSheet("Two")
	if err := s2.AddImage("B2", testPNG(t, 14, 14), ImageOptions{}); err != nil {
		t.Fatalf("s2 img1: %v", err)
	}

	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	data := buf.Bytes()

	names := zipNames(t, data)
	for _, want := range []string{
		"xl/media/image1.png", "xl/media/image2.png", "xl/media/image3.png",
		"xl/drawings/drawing1.xml", "xl/drawings/drawing2.xml",
		"xl/drawings/_rels/drawing1.xml.rels", "xl/drawings/_rels/drawing2.xml.rels",
		"xl/worksheets/_rels/sheet1.xml.rels", "xl/worksheets/_rels/sheet2.xml.rels",
	} {
		if !names[want] {
			t.Errorf("missing part %q", want)
		}
	}

	// drawing1 holds the two sheet-One images: drawing-local rId1->image1, rId2->image2.
	d1rels := zipEntry(t, data, "xl/drawings/_rels/drawing1.xml.rels")
	if !strings.Contains(d1rels, "../media/image1.png") || !strings.Contains(d1rels, "../media/image2.png") {
		t.Errorf("drawing1 rels wrong:\n%s", d1rels)
	}
	// drawing2 (sheet Two) reuses drawing-local rId1 but must target image3.
	d2rels := zipEntry(t, data, "xl/drawings/_rels/drawing2.xml.rels")
	if !strings.Contains(d2rels, "../media/image3.png") {
		t.Errorf("drawing2 rels wrong:\n%s", d2rels)
	}
	if strings.Contains(d2rels, "image1.png") || strings.Contains(d2rels, "image2.png") {
		t.Errorf("drawing2 rels leaked another drawing's media:\n%s", d2rels)
	}

	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen multi-image workbook: %v", err)
	}
}

// TestAddImageJPEG covers the JPEG sniff/extension branch (PNG is covered
// elsewhere).
func TestAddImageJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.AddImage("A1", buf.Bytes(), ImageOptions{}); err != nil {
		t.Fatalf("AddImage jpeg: %v", err)
	}
	out, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	if !zipNames(t, out.Bytes())["xl/media/image1.jpeg"] {
		t.Error("expected xl/media/image1.jpeg media part")
	}
}

// TestAddImageOnOpenedWorkbook embeds an image into a workbook opened from
// bytes: the drawing, media, and relationship parts must be added alongside
// the preserved content and the package must reopen cleanly.
func TestAddImageOnOpenedWorkbook(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")
	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	base := buf.Bytes()

	reopened, err := OpenReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	sheet, err := reopened.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.AddImage("B2", testPNG(t, 20, 10), ImageOptions{WidthPx: 200, HeightPx: 100}); err != nil {
		t.Fatalf("AddImage on opened workbook: %v", err)
	}

	out, err := reopened.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	names := zipNames(t, out)
	for _, want := range []string{
		"xl/media/image1.png",
		"xl/drawings/drawing1.xml",
	} {
		if !names[want] {
			t.Errorf("missing part %q; parts=%v", want, names)
		}
	}
	// The sheet part name is preserved; its rels must gain a drawing rel.
	sheetName := strings.TrimPrefix(sheet.partName, "/")
	relsName := ""
	for n := range names {
		if strings.HasSuffix(n, "_rels/"+strings.TrimPrefix(sheetName, "xl/worksheets/")+".rels") {
			relsName = n
		}
	}
	if relsName == "" {
		t.Fatalf("sheet rels part not found; parts=%v", names)
	}
	rels := zipEntry(t, out, relsName)
	if !strings.Contains(rels, "drawing") {
		t.Errorf("sheet rels missing drawing relationship:\n%s", rels)
	}
	worksheet := zipEntry(t, out, sheetName)
	if !strings.Contains(worksheet, "<drawing") {
		t.Errorf("worksheet missing <drawing> element:\n%s", worksheet)
	}
	// Content types must declare the drawing part and the png extension.
	ct := zipEntry(t, out, "[Content_Types].xml")
	if !strings.Contains(ct, "drawing+xml") {
		t.Errorf("[Content_Types].xml missing drawing type:\n%s", ct)
	}

	if _, err := OpenReader(bytes.NewReader(out), int64(len(out))); err != nil {
		t.Fatalf("reopen after adding image: %v", err)
	}
}

// C200 (updated for opened-workbook image support): AddImage after Close()
// must not silently drop the image. The reader is nil after Close, but the
// preserved parts are durable and the round-trip save path writes images
// added to opened workbooks, so the image must land in the output.
func TestAddImageOpenedWorkbookAfterClose(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")
	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	data := buf.Bytes()

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sheet, err := reopened.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.AddImage("A1", testPNG(t, 10, 10), ImageOptions{}); err != nil {
		t.Fatalf("AddImage after Open→Close: %v", err)
	}

	// The image must not be silently dropped on save.
	out, err := reopened.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	var haveDrawing, haveMedia bool
	for name := range zipNames(t, out) {
		if strings.Contains(name, "drawings/drawing") {
			haveDrawing = true
		}
		if strings.Contains(name, "media/") {
			haveMedia = true
		}
	}
	if !haveDrawing || !haveMedia {
		t.Fatalf("image silently dropped on save after Close: drawing=%v media=%v", haveDrawing, haveMedia)
	}
	if _, err := OpenReader(bytes.NewReader(out), int64(len(out))); err != nil {
		t.Fatalf("reopen after save: %v", err)
	}
}
