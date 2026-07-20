package docx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// buildDocxWithOLE rebuilds minimal.docx with an embedded OLE object part, its
// content-type override, a document relationship, and a w:object reference that
// declares the ProgID. It returns the package bytes.
func buildDocxWithOLE(t *testing.T, oleData []byte) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("read minimal.docx: %v", err)
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
				`<Override PartName="/word/embeddings/oleObject1.bin" ContentType="`+opc.ContentTypeOLEObject+`"/></Types>`, 1)
		case "word/_rels/document.xml.rels":
			s = strings.Replace(s, "</Relationships>",
				`<Relationship Id="rId100" Type="`+opc.RelTypeOLEObject+`" Target="embeddings/oleObject1.bin"/></Relationships>`, 1)
		case "word/document.xml":
			ref := `<w:p><w:r><w:object w:dxaOrig="1440" w:dyaOrig="1440">` +
				`<o:OLEObject Type="Embed" ProgID="Excel.Sheet.12" ShapeID="_x0000_i1025" DrawAspect="Content" ObjectID="_1" r:id="rId100"/>` +
				`</w:object></w:r></w:p>`
			s = strings.Replace(s, "</w:body>", ref+"</w:body>", 1)
		}
		write(f.Name, []byte(s))
	}
	write("word/embeddings/oleObject1.bin", oleData)

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestOLEObjectsExtract(t *testing.T) {
	oleData := []byte("\xd0\xcf\x11\xe0 fake OLE compound file payload")
	pkg := buildDocxWithOLE(t, oleData)

	doc, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}

	objs := doc.OLEObjects()
	if len(objs) != 1 {
		t.Fatalf("OLEObjects returned %d objects, want 1", len(objs))
	}
	got := objs[0]
	if got.Name != "/word/embeddings/oleObject1.bin" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.ContentType != opc.ContentTypeOLEObject {
		t.Errorf("ContentType = %q", got.ContentType)
	}
	if !bytes.Equal(got.Data, oleData) {
		t.Errorf("Data mismatch: got %q", got.Data)
	}
	if got.ProgID != "Excel.Sheet.12" {
		t.Errorf("ProgID = %q, want Excel.Sheet.12", got.ProgID)
	}
}

// TestAddOLEObjectRoundTrip embeds an OLE object in a new document, saves it,
// reopens it, and confirms the object is reported with its stored bytes and
// ProgID.
func TestAddOLEObjectRoundTrip(t *testing.T) {
	doc := Create()
	oleData := []byte("\xd0\xcf\x11\xe0 embedded workbook payload")
	obj, err := doc.AddOLEObject(oleData, "Excel.Sheet.12", OLEEmbedOptions{})
	if err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}
	if obj.Name != "/word/embeddings/oleObject1.bin" {
		t.Errorf("Name = %q, want /word/embeddings/oleObject1.bin", obj.Name)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, _ := zipEntry(t, saved, "word/document.xml")
	for _, want := range []string{
		"<w:object",
		`ProgID="Excel.Sheet.12"`,
		"<o:OLEObject",
		"<v:shape",
		"<v:imagedata",
	} {
		if !bytes.Contains(docXML, []byte(want)) {
			t.Errorf("saved document.xml missing %q", want)
		}
	}
	// The embedded part and its icon image were written.
	if _, ok := zipEntry(t, saved, "word/embeddings/oleObject1.bin"); !ok {
		t.Error("embedded OLE part not written")
	}

	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	objs := reopened.OLEObjects()
	if len(objs) != 1 {
		t.Fatalf("OLEObjects() = %d, want 1", len(objs))
	}
	if !bytes.Equal(objs[0].Data, oleData) {
		t.Errorf("Data mismatch: got %q", objs[0].Data)
	}
	if objs[0].ProgID != "Excel.Sheet.12" {
		t.Errorf("ProgID = %q, want Excel.Sheet.12", objs[0].ProgID)
	}
	if objs[0].ContentType != opc.ContentTypeOLEObject {
		t.Errorf("ContentType = %q", objs[0].ContentType)
	}
}

// TestAddOLEObjectIntoOpened embeds an OLE object into a document opened from a
// package, exercising the round-trip save path and embedding-number scan.
func TestAddOLEObjectIntoOpened(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := doc.AddOLEObject([]byte("payload"), "Word.Document.12", OLEEmbedOptions{DisplayAsIcon: true}); err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, _ := zipEntry(t, saved, "word/document.xml")
	if !bytes.Contains(docXML, []byte(`DrawAspect="Icon"`)) {
		t.Error("DisplayAsIcon should emit DrawAspect=\"Icon\"")
	}
	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	if objs := reopened.OLEObjects(); len(objs) != 1 || objs[0].ProgID != "Word.Document.12" {
		t.Fatalf("OLEObjects() = %+v, want one Word.Document.12 object", objs)
	}
}

func TestOLEObjectsNoneOnPlainDoc(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if objs := doc.OLEObjects(); len(objs) != 0 {
		t.Fatalf("plain document returned %d OLE objects", len(objs))
	}
}
