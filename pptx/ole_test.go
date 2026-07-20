package pptx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// buildPPTXWithOLE rebuilds minimal.pptx with an embedded OLE object part, its
// content-type override, and a presentation relationship. minimal.pptx has no
// slides, so the object is referenced from the presentation part.
func buildPPTXWithOLE(t *testing.T, oleData []byte) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("read minimal.pptx: %v", err)
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
				`<Override PartName="/ppt/embeddings/oleObject1.bin" ContentType="`+opc.ContentTypeOLEObject+`"/></Types>`, 1)
		case "ppt/_rels/presentation.xml.rels":
			s = strings.Replace(s, "</Relationships>",
				`<Relationship Id="rId100" Type="`+opc.RelTypeOLEObject+`" Target="embeddings/oleObject1.bin"/></Relationships>`, 1)
		}
		write(f.Name, []byte(s))
	}
	write("ppt/embeddings/oleObject1.bin", oleData)

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestOLEObjectsExtract(t *testing.T) {
	oleData := []byte("\xd0\xcf\x11\xe0 fake OLE compound file payload")
	pkg := buildPPTXWithOLE(t, oleData)

	pres, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}

	objs := pres.OLEObjects()
	if len(objs) != 1 {
		t.Fatalf("OLEObjects returned %d objects, want 1", len(objs))
	}
	got := objs[0]
	if got.Name != "/ppt/embeddings/oleObject1.bin" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.ContentType != opc.ContentTypeOLEObject {
		t.Errorf("ContentType = %q", got.ContentType)
	}
	if !bytes.Equal(got.Data, oleData) {
		t.Errorf("Data mismatch: got %q", got.Data)
	}
}

func TestOLEObjectsNoneOnPlainPresentation(t *testing.T) {
	pres, err := Open("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if objs := pres.OLEObjects(); len(objs) != 0 {
		t.Fatalf("plain presentation returned %d OLE objects", len(objs))
	}
}
