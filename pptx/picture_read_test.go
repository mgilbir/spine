package pptx

import (
	"bytes"
	"os"
	"testing"
)

// TestSlidePictures_CreateRoundTrip adds a picture on the create path and reads
// it back through Slide.Pictures after SaveBytes/OpenReader, checking alt text,
// content type, bytes, and frame size.
func TestSlidePictures_CreateRoundTrip(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	pic, err := s.AddPictureFromBytes(minimalTransparentPNG, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	pic.SetDescription("a caption")

	rp := saveReopen(t, p)
	rs, _ := rp.Slide(0)
	pics := rs.Pictures()
	if len(pics) != 1 {
		t.Fatalf("Pictures() = %d, want 1", len(pics))
	}
	got := pics[0]
	if got.AltText() != "a caption" {
		t.Errorf("AltText = %q, want %q", got.AltText(), "a caption")
	}
	if got.ContentType() != "image/png" {
		t.Errorf("ContentType = %q, want image/png", got.ContentType())
	}
	if !bytes.Equal(got.Data(), minimalTransparentPNG) {
		t.Errorf("Data length = %d, want %d", len(got.Data()), len(minimalTransparentPNG))
	}
	if w, h := got.Size(); w <= 0 || h <= 0 {
		t.Errorf("Size = %dx%d, want positive EMU dimensions", w, h)
	}
}

// TestPresentationPictures_AcrossSlides confirms Presentation.Pictures composes
// per-slide readers across slides.
func TestPresentationPictures_AcrossSlides(t *testing.T) {
	p := Create()
	for i := 0; i < 3; i++ {
		s := p.AddSlide()
		if _, err := s.AddPictureFromBytes(minimalTransparentPNG, "image/png"); err != nil {
			t.Fatal(err)
		}
	}
	rp := saveReopen(t, p)
	if got := len(rp.Pictures()); got != 3 {
		t.Errorf("Presentation.Pictures() = %d, want 3", got)
	}
}

// TestPicturesExcludeMedia confirms a video's poster p:pic is not reported as a
// picture by Slide.Pictures.
func TestPicturesExcludeMedia(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	if _, err := s.AddPictureFromBytes(minimalTransparentPNG, "image/png"); err != nil {
		t.Fatal(err)
	}
	// A minimal MP4 (ftyp box) so the media validates and embeds.
	mp4 := append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42")...)
	mp4 = append(mp4, make([]byte, 16)...)
	s.AddVideo(mp4, "video/mp4")

	rp := saveReopen(t, p)
	rs, _ := rp.Slide(0)
	pics := rs.Pictures()
	if len(pics) != 1 {
		t.Fatalf("Pictures() = %d, want 1 (video poster must be excluded)", len(pics))
	}
	if pics[0].isMedia {
		t.Error("the genuine picture was flagged as media")
	}
}

// TestReadPicturesFromFixture reads pictures from a real fixture and confirms
// their bytes and content types resolve from the embedded media parts.
func TestReadPicturesFromFixture(t *testing.T) {
	const path = "testdata/external/big_data.pptx"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("fixture not present:", path)
	}
	p, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	pics := p.Pictures()
	if len(pics) == 0 {
		t.Fatal("no pictures read from a fixture known to contain them")
	}
	var withData int
	for _, pic := range pics {
		if len(pic.Data()) > 0 && pic.ContentType() != "" {
			withData++
		}
	}
	if withData == 0 {
		t.Error("no picture resolved both its bytes and content type")
	}
}
