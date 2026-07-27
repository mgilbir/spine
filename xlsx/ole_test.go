package xlsx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// buildXLSXWithOLE rebuilds minimal.xlsx with an embedded OLE object part, its
// content-type override, a worksheet relationship, and an oleObject reference
// that declares the progId.
func buildXLSXWithOLE(t *testing.T, oleData []byte) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name string, data []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := content.String()

		switch f.Name {
		case "[Content_Types].xml":
			s = strings.Replace(s, "</Types>",
				`<Override PartName="/xl/embeddings/oleObject1.bin" ContentType="`+opc.ContentTypeOLEObject+`"/></Types>`, 1)
		case "xl/worksheets/sheet1.xml":
			ref := `<oleObjects><oleObject progId="Word.Document.12" shapeId="1025" r:id="rId100"/></oleObjects>`
			s = strings.Replace(s, "</worksheet>", ref+"</worksheet>", 1)
		}
		write(f.Name, []byte(s))
	}
	write("xl/worksheets/_rels/sheet1.xml.rels", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId100" Type="`+opc.RelTypeOLEObject+`" Target="../embeddings/oleObject1.bin"/>`+
			`</Relationships>`))
	write("xl/embeddings/oleObject1.bin", oleData)

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestOLEObjectsExtract(t *testing.T) {
	oleData := []byte("\xd0\xcf\x11\xe0 fake OLE compound file payload")
	pkg := buildXLSXWithOLE(t, oleData)

	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}

	objs := wb.OLEObjects()
	if len(objs) != 1 {
		t.Fatalf("OLEObjects returned %d objects, want 1", len(objs))
	}
	got := objs[0]
	if got.Name != "/xl/embeddings/oleObject1.bin" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.ContentType != opc.ContentTypeOLEObject {
		t.Errorf("ContentType = %q", got.ContentType)
	}
	if !bytes.Equal(got.Data, oleData) {
		t.Errorf("Data mismatch: got %q", got.Data)
	}
	if got.ProgID != "Word.Document.12" {
		t.Errorf("ProgID = %q, want Word.Document.12", got.ProgID)
	}
}

func TestOLEObjectsNoneOnPlainWorkbook(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if objs := wb.OLEObjects(); len(objs) != 0 {
		t.Fatalf("plain workbook returned %d OLE objects", len(objs))
	}
}

// AddOLEObject embeds an object on a created sheet: it writes the embedding
// part, the oleObjects reference, a legacy VML drawing, and the relationships,
// and the object is re-extractable after a round trip.
func TestAddOLEObjectRoundTrip(t *testing.T) {
	w := Create()
	s := addSheetT(w, "Sheet1")
	if _, err := s.Cell("A1"); err != nil {
		t.Fatal(err)
	}
	oleData := []byte("\xd0\xcf\x11\xe0 embedded payload")
	if err := s.AddOLEObject(OLEObjectSpec{Data: oleData, ProgID: "Word.Document.12", Anchor: "B2"}); err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	embed := readZipEntry(t, out, "xl/embeddings/oleObject1.bin")
	if !bytes.Equal(embed, oleData) {
		t.Errorf("embedded payload mismatch: %q", embed)
	}

	sheet1 := string(readZipEntry(t, out, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheet1, `<oleObjects>`) || !strings.Contains(sheet1, `progId="Word.Document.12"`) {
		t.Errorf("worksheet missing oleObjects reference:\n%s", sheet1)
	}
	if !strings.Contains(sheet1, `shapeId="1025"`) {
		t.Errorf("worksheet oleObject missing shapeId:\n%s", sheet1)
	}
	if !strings.Contains(sheet1, `<legacyDrawing`) {
		t.Errorf("worksheet missing legacyDrawing:\n%s", sheet1)
	}

	vml := string(readZipEntry(t, out, "xl/drawings/vmlDrawing1.vml"))
	if !strings.Contains(vml, `ObjectType="Pict"`) || !strings.Contains(vml, `_x0000_s1025`) {
		t.Errorf("VML drawing missing OLE shape:\n%s", vml)
	}

	ct := string(readZipEntry(t, out, "[Content_Types].xml"))
	if !strings.Contains(ct, opc.ContentTypeOLEObject) {
		t.Errorf("content types missing OLE object type:\n%s", ct)
	}

	rw, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	objs := rw.OLEObjects()
	if len(objs) != 1 {
		t.Fatalf("OLEObjects() = %d, want 1", len(objs))
	}
	if objs[0].ProgID != "Word.Document.12" || !bytes.Equal(objs[0].Data, oleData) {
		t.Errorf("re-extracted object = %+v", objs[0])
	}
}

// AddOLEObject with a preview image writes the media part and the VML's imagedata
// relationship.
func TestAddOLEObjectWithPreview(t *testing.T) {
	w := Create()
	s := addSheetT(w, "Sheet1")
	if _, err := s.Cell("A1"); err != nil {
		t.Fatal(err)
	}
	preview := []byte("\x89PNG\r\n\x1a\n fake png")
	err := s.AddOLEObject(OLEObjectSpec{
		Data:               []byte("\xd0\xcf\x11\xe0 payload"),
		Preview:            preview,
		PreviewContentType: opc.ContentTypePNG,
		PreviewExt:         "png",
	})
	if err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := readZipEntry(t, out, "xl/media/image1.png"); !bytes.Equal(got, preview) {
		t.Errorf("preview image mismatch: %q", got)
	}
	vmlRels := string(readZipEntry(t, out, "xl/drawings/_rels/vmlDrawing1.vml.rels"))
	if !strings.Contains(vmlRels, "image1.png") {
		t.Errorf("VML rels missing preview image:\n%s", vmlRels)
	}
	vml := string(readZipEntry(t, out, "xl/drawings/vmlDrawing1.vml"))
	if !strings.Contains(vml, "<v:imagedata") {
		t.Errorf("VML missing imagedata:\n%s", vml)
	}
}

// AddOLEObject rejects an empty payload.
func TestAddOLEObjectValidation(t *testing.T) {
	w := Create()
	s := addSheetT(w, "Sheet1")
	if err := s.AddOLEObject(OLEObjectSpec{}); err == nil {
		t.Error("empty OLE data accepted")
	}
}

// TestReadCommentsThenAddOLE is the C284 regression: reading Comments() (or
// Cell.Comment()) on a comment-free sheet lazily creates the sheet's comment
// model, which must not permanently block AddOLEObject. A read-only inspection
// must leave the writer available.
func TestReadCommentsThenAddOLE(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s, _ := wb.Sheet(0)

	// A read-only inspection of a comment-free sheet.
	if got := s.Comments(); len(got) != 0 {
		t.Fatalf("comment-free sheet reported %d comments", len(got))
	}
	c, err := s.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Comment()

	// AddOLEObject must still succeed: the sheet has no real comments.
	if err := s.AddOLEObject(OLEObjectSpec{Data: []byte("\xd0\xcf\x11\xe0 payload")}); err != nil {
		t.Fatalf("AddOLEObject after read-only Comments(): %v", err)
	}
	if _, err := wb.SaveBytes(); err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
}
