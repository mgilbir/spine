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
