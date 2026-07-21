package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func mainPartBytes(t *testing.T, pkg []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			return b
		}
	}
	t.Fatal("word/document.xml not found")
	return nil
}

// Opening a document must not eagerly build the body model; it is parsed lazily
// on first access, so a document round-tripped without touching its body holds
// no body model and writes its original main-part bytes verbatim.
func TestBodyParsedLazily(t *testing.T) {
	d := Create()
	d.AddParagraphWithText("hello world")
	orig, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	r, err := OpenReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r.docModel != nil {
		t.Errorf("body model built eagerly at open; want lazy (nil until accessed)")
	}

	// A clean round-trip without touching the body passes the original main
	// part through verbatim.
	out, err := r.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if r.docModel != nil {
		t.Errorf("clean round-trip materialized the body model; want passthrough")
	}
	if !bytes.Equal(mainPartBytes(t, orig), mainPartBytes(t, out)) {
		t.Errorf("clean round-trip did not pass the main part through byte-for-byte")
	}

	// Accessing the body materializes the model and yields the real content.
	r2, _ := OpenReader(bytes.NewReader(orig), int64(len(orig)))
	if got := r2.Text(); got != "hello world" {
		t.Errorf("Text() = %q, want %q", got, "hello world")
	}
	if r2.docModel == nil {
		t.Errorf("body model still nil after Text(); lazy parse did not run")
	}
}
