package pptx

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// C79: AddPicture reads the image file, embeds it as a media part, and wires the
// blip embed reference; a nonexistent path returns an error.
func TestAddPicture_EmbedsImageAndErrors(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "img.png")
	if err := os.WriteFile(imgPath, minimalTransparentPNG, 0o644); err != nil {
		t.Fatal(err)
	}

	p := Create()
	s := p.AddSlide()
	pic, err := s.AddPicture(imgPath)
	if err != nil {
		t.Fatalf("AddPicture: %v", err)
	}
	pic.SetSize(dml.Inches(2), dml.Inches(2))

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var hasMedia, hasEmbed bool
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/media/") {
			hasMedia = true
		}
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			if strings.Contains(string(b), "embed=") {
				hasEmbed = true
			}
		}
	}
	if !hasMedia {
		t.Error("no media part written for the added picture")
	}
	if !hasEmbed {
		t.Error("slide blip has no embed reference to the media part")
	}

	// A nonexistent path must return an error, not a silent stub.
	if _, err := s.AddPicture(filepath.Join(dir, "does-not-exist.png")); err == nil {
		t.Error("AddPicture on a nonexistent file returned nil error")
	}
}

// C166: AddPicture defaults the frame to the image's intrinsic size at 96 DPI
// instead of an invisible 0x0; unsupported formats fall back to 4x3 inches.
func TestAddPictureDefaultsToNativeSize(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "img.png")

	img := image.NewRGBA(image.Rect(0, 0, 10, 8))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imgPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	p := Create()
	s := p.AddSlide()
	pic, err := s.AddPicture(imgPath)
	if err != nil {
		t.Fatalf("AddPicture: %v", err)
	}
	w, h := pic.Size()
	if w != 10*emuPerPixel || h != 8*emuPerPixel {
		t.Errorf("default picture size = %dx%d EMU, want %dx%d (10x8 px at 96 DPI)",
			w, h, 10*emuPerPixel, 8*emuPerPixel)
	}

	// The native size must reach the saved slide XML inside the p:pic element.
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	slideXML := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	picStart := strings.Index(slideXML, "<p:pic>")
	if picStart < 0 {
		t.Fatalf("no p:pic in slide XML:\n%s", slideXML)
	}
	if !strings.Contains(slideXML[picStart:], `<a:ext cx="95250" cy="76200"/>`) {
		t.Errorf("picture frame does not carry the native size:\n%s", slideXML[picStart:])
	}

	// The caller can still override the default.
	pic.SetSize(dml.Inches(1), dml.Inches(1))
	if w, h := pic.Size(); w != dml.Inches(1) || h != dml.Inches(1) {
		t.Errorf("SetSize after AddPicture = %dx%d, want %dx%d", w, h, dml.Inches(1), dml.Inches(1))
	}

	// A format image.DecodeConfig cannot parse falls back to 4x3 inches.
	bmpPath := filepath.Join(dir, "img.bmp")
	if err := os.WriteFile(bmpPath, []byte("BM not a decodable image"), 0o644); err != nil {
		t.Fatal(err)
	}
	fallback, err := s.AddPicture(bmpPath)
	if err != nil {
		t.Fatalf("AddPicture (bmp): %v", err)
	}
	if w, h := fallback.Size(); w != dml.Inches(4) || h != dml.Inches(3) {
		t.Errorf("fallback picture size = %dx%d, want 4x3 inches (%dx%d)",
			w, h, dml.Inches(4), dml.Inches(3))
	}
}
