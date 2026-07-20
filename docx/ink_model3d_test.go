package docx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

const docxInkPartXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<inkml:ink xmlns:inkml="http://www.w3.org/2003/InkML">` +
	`<inkml:trace>0 0 10</inkml:trace></inkml:ink>`

// buildDocxWithInkAndModel rebuilds minimal.docx with an ink (InkML) part and a
// 3D model (glb) part referenced from the main document part, plus the matching
// content types, relationships, and a w:contentPart body reference.
func buildDocxWithInkAndModel(t *testing.T, glbData []byte) []byte {
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
				`<Default Extension="glb" ContentType="`+opc.ContentTypeModel3D+`"/>`+
					`<Override PartName="/word/ink/ink1.xml" ContentType="`+opc.ContentTypeInk+`"/></Types>`, 1)
		case "word/_rels/document.xml.rels":
			s = strings.Replace(s, "</Relationships>",
				`<Relationship Id="rId2" Type="`+opc.RelTypeCustomXML+`" Target="ink/ink1.xml"/>`+
					`<Relationship Id="rId3" Type="http://schemas.microsoft.com/office/2017/06/relationships/model3d" Target="media/model1.glb"/></Relationships>`, 1)
		case "word/document.xml":
			s = strings.Replace(s, "</w:body>",
				`<w:p><w:contentPart r:id="rId2"/></w:p></w:body>`, 1)
		}
		write(f.Name, []byte(s))
	}
	write("word/ink/ink1.xml", []byte(docxInkPartXML))
	write("word/media/model1.glb", glbData)

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestDocumentInkAnnotationsExtract(t *testing.T) {
	pkg := buildDocxWithInkAndModel(t, []byte("glTF\x02\x00\x00\x00 fake glb"))
	doc, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	inks := doc.InkAnnotations()
	if len(inks) != 1 {
		t.Fatalf("InkAnnotations = %d, want 1", len(inks))
	}
	got := inks[0]
	if got.PartName != "/word/ink/ink1.xml" {
		t.Errorf("PartName = %q", got.PartName)
	}
	if got.ContentType != opc.ContentTypeInk {
		t.Errorf("ContentType = %q", got.ContentType)
	}
	if got.RelID != "rId2" {
		t.Errorf("RelID = %q", got.RelID)
	}
	if got.Owner != "/word/document.xml" {
		t.Errorf("Owner = %q", got.Owner)
	}
	if string(got.Data) != docxInkPartXML {
		t.Errorf("Data mismatch: got %q", got.Data)
	}
}

func TestDocumentModel3DExtract(t *testing.T) {
	glb := []byte("glTF\x02\x00\x00\x00 fake glb payload")
	pkg := buildDocxWithInkAndModel(t, glb)
	doc, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	models := doc.Model3D()
	if len(models) != 1 {
		t.Fatalf("Model3D = %d, want 1", len(models))
	}
	got := models[0]
	if got.PartName != "/word/media/model1.glb" {
		t.Errorf("PartName = %q", got.PartName)
	}
	if got.ContentType != opc.ContentTypeModel3D {
		t.Errorf("ContentType = %q", got.ContentType)
	}
	if got.RelID != "rId3" {
		t.Errorf("RelID = %q", got.RelID)
	}
	if !bytes.Equal(got.Data, glb) {
		t.Errorf("Data mismatch: got %q", got.Data)
	}
}

// TestDocxInkAndModel3DRoundTrip confirms the ink and 3D model parts (and their
// relationships) survive a save/reopen unchanged, so extraction still works
// after a round trip and the part bytes are byte-identical.
//
// Note: WordprocessingML stores the ink reference as a run-level w:contentPart
// in the document body; the docx paragraph marshaler does not yet preserve that
// element, so the in-body reference is dropped on save even though the ink part
// and its relationship are preserved (authoring/body-reference preservation is
// deferred). Ink extraction is driven off the relationship, which survives.
func TestDocxInkAndModel3DRoundTrip(t *testing.T) {
	glb := []byte("glTF\x02\x00\x00\x00 fake glb payload for round trip")
	pkg := buildDocxWithInkAndModel(t, glb)
	doc, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	out, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen zip: %v", err)
	}
	if got := readDocxZipEntry(t, zr, "word/ink/ink1.xml"); string(got) != docxInkPartXML {
		t.Errorf("ink part bytes changed:\n%s", got)
	}
	if got := readDocxZipEntry(t, zr, "word/media/model1.glb"); !bytes.Equal(got, glb) {
		t.Errorf("model part bytes changed: got %q", got)
	}

	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if inks := re.InkAnnotations(); len(inks) != 1 || string(inks[0].Data) != docxInkPartXML {
		t.Errorf("ink not re-extractable after round trip: %+v", inks)
	}
	if ms := re.Model3D(); len(ms) != 1 || !bytes.Equal(ms[0].Data, glb) {
		t.Errorf("model not re-extractable after round trip: %+v", ms)
	}
}

func TestDocxInkAndModel3DNoneOnPlainDoc(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := doc.InkAnnotations(); len(got) != 0 {
		t.Errorf("plain document returned %d ink annotations", len(got))
	}
	if got := doc.Model3D(); len(got) != 0 {
		t.Errorf("plain document returned %d 3D models", len(got))
	}
}

func readDocxZipEntry(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer func() { _ = rc.Close() }()
			var b bytes.Buffer
			if _, err := b.ReadFrom(rc); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return b.Bytes()
		}
	}
	t.Fatalf("entry %s not found", name)
	return nil
}
