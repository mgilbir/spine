package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

func zipEntry(t *testing.T, data []byte, name string) ([]byte, bool) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close() //nolint:errcheck
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return b, true
		}
	}
	return nil, false
}

// C3: a part added to a document opened from a file (here an image) must have
// its part bytes, content type, and relationship all written — previously the
// round-trip save wrote the relationship and reference but not the part itself,
// producing a package with a dangling reference.
func TestOpenedDocument_AddImageWritesPart(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	r := doc.AddParagraph().AddRun()
	if _, err := r.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := zipEntry(t, saved, "word/media/image1.png"); !ok {
		t.Error("image media part was not written to the package")
	}
	ct, ok := zipEntry(t, saved, "[Content_Types].xml")
	if !ok {
		t.Fatal("[Content_Types].xml missing")
	}
	if !strings.Contains(string(ct), "image/png") {
		t.Error("png content type not declared in [Content_Types].xml")
	}
	rels, ok := zipEntry(t, saved, "word/_rels/document.xml.rels")
	if !ok {
		t.Fatal("document.xml.rels missing")
	}
	if !strings.Contains(string(rels), "media/image1.png") {
		t.Error("image relationship not written")
	}

	// The result must still open cleanly.
	if _, err := OpenReader(bytes.NewReader(saved), int64(len(saved))); err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
}
