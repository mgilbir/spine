package pptx

import (
	"archive/zip"
	"bytes"
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
