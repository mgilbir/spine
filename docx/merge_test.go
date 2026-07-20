package docx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

func assertZipHasPrefix(t *testing.T, data []byte, prefix string) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, prefix) {
			return
		}
	}
	t.Fatalf("no zip entry with prefix %q", prefix)
}

func TestAppendBodyContent(t *testing.T) {
	dst := Create()
	dst.AddParagraphWithText("Dst first")
	dst.AddParagraphWithText("Dst second")

	src := Create()
	src.AddParagraphWithText("Src alpha")
	src.AddTable(2, 2)
	src.AddParagraphWithText("Src beta")

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}

	body := re.Body()
	for _, want := range []string{"Dst first", "Dst second", "Src alpha", "Src beta"} {
		if !strings.Contains(body, want) {
			t.Errorf("merged body missing %q; got:\n%s", want, body)
		}
	}
	// Order: dst content precedes src content.
	if i, j := strings.Index(body, "Dst second"), strings.Index(body, "Src alpha"); i < 0 || j < 0 || i > j {
		t.Errorf("append order wrong: %q", body)
	}
	if len(re.Tables()) != 1 {
		t.Errorf("expected 1 table after append, got %d", len(re.Tables()))
	}
}

func TestAppendWithImage(t *testing.T) {
	dst := Create()
	dst.AddParagraphWithText("Cover")

	src := Create()
	p := src.AddParagraph()
	r := p.AddRun()
	if _, err := r.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("AddImageFromBytes: %v", err)
	}

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if len(dst.imageParts) != 1 {
		t.Fatalf("expected 1 image part carried, got %d", len(dst.imageParts))
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	assertZipHasPrefix(t, data, "word/media/")
}

func TestAppendWithNumbering(t *testing.T) {
	dst := Create()
	dst.AddParagraphWithText("Intro")

	src := Create()
	def := src.Numbering().AddDefinition()
	def.SetLevel(0, NumberFormatDecimal, "%1.")
	list := def.ListStyle()
	src.AddParagraphWithText("Item one").SetListStyle(list, 0)

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	if !strings.Contains(re.Body(), "Item one") {
		t.Errorf("numbered item text missing")
	}
	assertZipHasPrefix(t, data, "word/numbering.xml")
}

func TestAppendNil(t *testing.T) {
	dst := Create()
	dst.AddParagraphWithText("x")
	if err := dst.Append(nil); err != ErrNilDocument {
		t.Fatalf("err = %v, want ErrNilDocument", err)
	}
}
