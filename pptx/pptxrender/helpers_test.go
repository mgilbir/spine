package pptxrender

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/mgilbir/spine/pptx"
)

// zipParts reads every entry of an OPC package into a name->bytes map. Names are
// normalized to the leading-slash part-name form (e.g. "/ppt/...").
func zipParts(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		out["/"+f.Name] = b
	}
	return out
}

// rewriteZipPart returns data with the named entry passed through rewrite.
func rewriteZipPart(t *testing.T, data []byte, name string, rewrite func([]byte) []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, file := range reader.File {
		src, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == name {
			content = rewrite(content)
		}
		dst, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// blankSlide adds a slide on p's blank layout.
func blankSlide(t testing.TB, p *pptx.Presentation) *pptx.Slide {
	t.Helper()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	return p.AddSlideFromLayout(layout)
}
