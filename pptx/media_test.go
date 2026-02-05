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

func TestNewVideo(t *testing.T) {
	video := NewVideo()
	if video == nil {
		t.Fatal("NewVideo() returned nil")
	}
}

func TestVideo_VideoPath(t *testing.T) {
	video := NewVideo()
	video.SetVideoPath("/videos/test.mp4")

	if video.VideoPath() != "/videos/test.mp4" {
		t.Errorf("VideoPath() = %q, want %q", video.VideoPath(), "/videos/test.mp4")
	}
}

func TestVideo_VideoData(t *testing.T) {
	video := NewVideo()
	data := []byte{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70} // MP4 magic bytes
	video.SetVideoData(data, "video/mp4")

	if string(video.VideoData()) != string(data) {
		t.Error("VideoData() does not match")
	}
	if video.ContentType() != "video/mp4" {
		t.Errorf("ContentType() = %q, want %q", video.ContentType(), "video/mp4")
	}
}

func TestVideo_PosterFrame(t *testing.T) {
	video := NewVideo()
	pic := NewPicture()

	video.SetPosterFrame(pic)
	if video.PosterFrame() != pic {
		t.Error("PosterFrame() does not match set value")
	}
}

func TestNewAudio(t *testing.T) {
	audio := NewAudio()
	if audio == nil {
		t.Fatal("NewAudio() returned nil")
	}
}

func TestAudio_AudioPath(t *testing.T) {
	audio := NewAudio()
	audio.SetAudioPath("/audio/test.mp3")

	if audio.AudioPath() != "/audio/test.mp3" {
		t.Errorf("AudioPath() = %q, want %q", audio.AudioPath(), "/audio/test.mp3")
	}
}

func TestAudio_AudioData(t *testing.T) {
	audio := NewAudio()
	data := []byte{0xFF, 0xFB} // MP3 magic bytes
	audio.SetAudioData(data, "audio/mpeg")

	if string(audio.AudioData()) != string(data) {
		t.Error("AudioData() does not match")
	}
	if audio.ContentType() != "audio/mpeg" {
		t.Errorf("ContentType() = %q, want %q", audio.ContentType(), "audio/mpeg")
	}
}

func TestAudio_Icon(t *testing.T) {
	audio := NewAudio()
	pic := NewPicture()

	audio.SetIcon(pic)
	if audio.Icon() != pic {
		t.Error("Icon() does not match set value")
	}
}

func TestNewOLEObject(t *testing.T) {
	ole := NewOLEObject()
	if ole == nil {
		t.Fatal("NewOLEObject() returned nil")
	}
}

func TestOLEObject_ProgID(t *testing.T) {
	ole := NewOLEObject()
	ole.SetProgID("Excel.Sheet.12")

	if ole.ProgID() != "Excel.Sheet.12" {
		t.Errorf("ProgID() = %q, want %q", ole.ProgID(), "Excel.Sheet.12")
	}
}

func TestOLEObject_Data(t *testing.T) {
	ole := NewOLEObject()
	data := []byte{0xD0, 0xCF, 0x11, 0xE0} // OLE magic bytes
	ole.SetData(data)

	if string(ole.Data()) != string(data) {
		t.Error("Data() does not match")
	}
}

func TestOLEObject_Icon(t *testing.T) {
	ole := NewOLEObject()
	pic := NewPicture()

	ole.SetIcon(pic)
	if ole.Icon() != pic {
		t.Error("Icon() does not match set value")
	}
}
