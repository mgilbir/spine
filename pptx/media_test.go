package pptx

import (
	"testing"
)

func TestNewPicture(t *testing.T) {
	pic := NewPicture()
	if pic == nil {
		t.Fatal("NewPicture() returned nil")
	}

	if pic.ShapeType() != ShapeTypePicture {
		t.Errorf("ShapeType() = %v, want ShapeTypePicture", pic.ShapeType())
	}
}

func TestPicture_ImagePath(t *testing.T) {
	pic := NewPicture()
	pic.SetImagePath("/images/test.png")

	if pic.ImagePath() != "/images/test.png" {
		t.Errorf("ImagePath() = %q, want %q", pic.ImagePath(), "/images/test.png")
	}
}

func TestPicture_ImageData(t *testing.T) {
	pic := NewPicture()
	data := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	pic.SetImageData(data, "image/png")

	if string(pic.ImageData()) != string(data) {
		t.Error("ImageData() does not match")
	}
	if pic.ContentType() != "image/png" {
		t.Errorf("ContentType() = %q, want %q", pic.ContentType(), "image/png")
	}
}

func TestPicture_SetSVGData(t *testing.T) {
	pic := NewPicture()
	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"></svg>`)

	pic.SetSVGData(svgData)

	if string(pic.svgData) != string(svgData) {
		t.Fatal("svgData was not stored")
	}
	if pic.svgContentType != "image/svg+xml" {
		t.Fatalf("svgContentType = %q, want image/svg+xml", pic.svgContentType)
	}
	if string(pic.imageData) != string(minimalTransparentPNG) {
		t.Fatal("fallback PNG was not populated")
	}
	if pic.contentType != "image/png" {
		t.Fatalf("contentType = %q, want image/png", pic.contentType)
	}
	if pic.ShapeType() != ShapeTypePicture {
		t.Fatalf("ShapeType() = %v, want ShapeTypePicture", pic.ShapeType())
	}
}

func TestPicture_Description(t *testing.T) {
	pic := NewPicture()
	pic.SetDescription("A test image")

	if pic.Description() != "A test image" {
		t.Errorf("Description() = %q, want %q", pic.Description(), "A test image")
	}
}

func TestPicture_Crop(t *testing.T) {
	pic := NewPicture()

	pic.SetCropLeft(0.1)
	pic.SetCropTop(0.2)
	pic.SetCropRight(0.3)
	pic.SetCropBottom(0.4)

	if pic.CropLeft() != 0.1 {
		t.Errorf("CropLeft() = %f, want 0.1", pic.CropLeft())
	}
	if pic.CropTop() != 0.2 {
		t.Errorf("CropTop() = %f, want 0.2", pic.CropTop())
	}
	if pic.CropRight() != 0.3 {
		t.Errorf("CropRight() = %f, want 0.3", pic.CropRight())
	}
	if pic.CropBottom() != 0.4 {
		t.Errorf("CropBottom() = %f, want 0.4", pic.CropBottom())
	}
}

func TestPicture_SetCrop(t *testing.T) {
	pic := NewPicture()
	pic.SetCrop(0.1, 0.2, 0.3, 0.4)

	if pic.CropLeft() != 0.1 || pic.CropTop() != 0.2 ||
		pic.CropRight() != 0.3 || pic.CropBottom() != 0.4 {
		t.Error("SetCrop did not set all values correctly")
	}
}

func TestPicture_CropClamping(t *testing.T) {
	pic := NewPicture()

	pic.SetCropLeft(-0.5)
	if pic.CropLeft() != 0 {
		t.Errorf("Negative crop should clamp to 0, got %f", pic.CropLeft())
	}

	pic.SetCropLeft(1.5)
	if pic.CropLeft() != 1 {
		t.Errorf("Crop > 1 should clamp to 1, got %f", pic.CropLeft())
	}
}
