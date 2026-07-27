package docx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// minimalPNG returns a minimal valid 1x1 red PNG image.
func minimalPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde,
		0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, // IDAT chunk
		0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00,
		0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc, 0x33,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, // IEND chunk
		0xae, 0x42, 0x60, 0x82,
	}
}

// taggedPNG returns minimalPNG with tag appended after the IEND chunk: a real
// PNG as far as any signature sniff or decoder is concerned, but byte-distinct
// per tag. Tests that need two *different* images used to pass ad-hoc strings
// like []byte("PNGDATA-ONE"), which C441's add-time validation now rejects (a
// mislabelled blob is exactly what the finding is about); this keeps them
// distinct without pretending garbage is an image.
func taggedPNG(tag string) []byte {
	return append(minimalPNG(), tag...)
}

func TestRunAddImageFromBytes(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddRun()

	img, err := r.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG)
	if err != nil {
		t.Fatal(err)
	}

	if img.relID == "" {
		t.Error("expected relID to be set")
	}
	if img.drawingID == 0 {
		t.Error("expected drawingID to be set")
	}

	// Should have added a drawing to the run
	if len(r.r.Drawing) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(r.r.Drawing))
	}

	// Drawing should have raw XML content
	if len(r.r.Drawing[0].RawContent) == 0 {
		t.Error("expected drawing RawContent to be non-empty")
	}

	// Should have an image part
	if len(doc.imageParts) != 1 {
		t.Fatalf("expected 1 image part, got %d", len(doc.imageParts))
	}
	if doc.imageParts[0].contentType != opc.ContentTypePNG {
		t.Errorf("contentType = %s, want image/png", doc.imageParts[0].contentType)
	}
}

func TestInlineImageSetSize(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddRun()

	img, err := r.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG)
	if err != nil {
		t.Fatal(err)
	}

	img.SetSize(72, 72) // 1 inch square

	// 72 points = 1 inch = 914400 EMUs
	if img.widthEMU != 914400 {
		t.Errorf("widthEMU = %d, want 914400", img.widthEMU)
	}
	if img.heightEMU != 914400 {
		t.Errorf("heightEMU = %d, want 914400", img.heightEMU)
	}
}

func TestImageSaveAndReopen(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("Here is an image:")
	p2 := doc.AddParagraph()
	r := p2.AddRun()

	img, err := r.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG)
	if err != nil {
		t.Fatal(err)
	}
	img.SetSize(72, 72)
	img.SetAltText("Test image")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "image.docx")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Reopen and verify paragraphs
	doc2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer doc2.Close() //nolint:errcheck

	paras := doc2.Paragraphs()
	if len(paras) < 2 {
		t.Fatalf("expected at least 2 paragraphs, got %d", len(paras))
	}
}

func TestRunAddImage(t *testing.T) {
	// Write a temp PNG file
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(pngPath, minimalPNG(), 0644); err != nil {
		t.Fatal(err)
	}

	doc := Create()
	p := doc.AddParagraph()
	r := p.AddRun()

	img, err := r.AddImage(pngPath)
	if err != nil {
		t.Fatal(err)
	}

	if img.relID == "" {
		t.Error("expected relID to be set")
	}
	if len(doc.imageParts) != 1 {
		t.Fatalf("expected 1 image part, got %d", len(doc.imageParts))
	}

	// Save and verify
	outPath := filepath.Join(tmpDir, "from_file.docx")
	if err := doc.Save(outPath); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
}

func TestUnsupportedImageFormat(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddRun()

	_, err := r.AddImageFromBytes([]byte{0, 0, 0}, "image/bmp")
	if err == nil {
		t.Error("expected error for unsupported content type")
	}
}

func TestContentTypeForExt(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".png", "image/png"},
		{".jpg", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".gif", "image/gif"},
		{".bmp", ""},
	}
	for _, tc := range tests {
		got := contentTypeForExt(tc.ext)
		if got != tc.want {
			t.Errorf("contentTypeForExt(%s) = %s, want %s", tc.ext, got, tc.want)
		}
	}
}
