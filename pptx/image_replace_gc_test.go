package pptx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// pngOfColor encodes a 1x1 PNG of the given color, giving distinct, decodable
// image bytes for image-replacement tests.
func pngOfColor(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, c)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// mediaPartNames returns the sorted set of /ppt/media parts in a saved deck.
func mediaPartCount(t *testing.T, data []byte) int {
	t.Helper()
	n := 0
	for name := range zipNames(t, data) {
		if strings.HasPrefix(name, "ppt/media/") {
			n++
		}
	}
	return n
}

// C314: replacing a picture's image must garbage-collect the outgoing image's
// relationship and media part — otherwise bulk template replacement accretes a
// dead image on every swap.
func TestReplacePictureImage_GCsOldImage(t *testing.T) {
	imgA := pngOfColor(t, color.RGBA{R: 255, A: 255})
	imgB := pngOfColor(t, color.RGBA{B: 255, A: 255})

	p := Create()
	s := p.AddSlide()
	if _, err := s.AddPictureFromBytes(imgA, "image/png"); err != nil {
		t.Fatal(err)
	}
	data0, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if got := mediaPartCount(t, data0); got != 1 {
		t.Fatalf("setup: media part count = %d, want 1", got)
	}

	p = openBytes(t, data0)
	pics := p.Slides()[0].Pictures()
	if len(pics) != 1 {
		t.Fatalf("loaded slide has %d pictures, want 1", len(pics))
	}
	pics[0].SetImageData(imgB, "image/png")
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	if got := mediaPartCount(t, data1); got != 1 {
		t.Errorf("after replace: media part count = %d, want 1 (old image leaked)", got)
	}
	// The surviving media part must be the new image, not the old one.
	foundNew := false
	for name := range zipNames(t, data1) {
		if strings.HasPrefix(name, "ppt/media/") {
			part := zipPart(t, data1, name)
			if bytes.Equal(part, imgA) {
				t.Errorf("old image bytes still present in %s", name)
			}
			if bytes.Equal(part, imgB) {
				foundNew = true
			}
		}
	}
	if !foundNew {
		t.Error("replacement image bytes were not written")
	}
}

// C314: a media part still referenced by another picture on the slide must NOT
// be collected when a sibling picture's image is replaced (gcSlideRels re-checks
// the slide XML before dropping anything).
func TestReplacePictureImage_KeepsSharedImage(t *testing.T) {
	shared := pngOfColor(t, color.RGBA{G: 255, A: 255})
	imgB := pngOfColor(t, color.RGBA{B: 255, A: 255})

	p := Create()
	s := p.AddSlide()
	if _, err := s.AddPictureFromBytes(shared, "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddPictureFromBytes(shared, "image/png"); err != nil {
		t.Fatal(err)
	}
	data0, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// Both pictures dedupe to one media part.
	if got := mediaPartCount(t, data0); got != 1 {
		t.Fatalf("setup: media part count = %d, want 1", got)
	}

	p = openBytes(t, data0)
	pics := p.Slides()[0].Pictures()
	if len(pics) != 2 {
		t.Fatalf("loaded slide has %d pictures, want 2", len(pics))
	}
	pics[0].SetImageData(imgB, "image/png")
	data1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// New image added; shared image kept for the second picture.
	if got := mediaPartCount(t, data1); got != 2 {
		t.Errorf("after replace: media part count = %d, want 2", got)
	}
	foundShared, foundNew := false, false
	for name := range zipNames(t, data1) {
		if strings.HasPrefix(name, "ppt/media/") {
			part := zipPart(t, data1, name)
			if bytes.Equal(part, shared) {
				foundShared = true
			}
			if bytes.Equal(part, imgB) {
				foundNew = true
			}
		}
	}
	if !foundShared {
		t.Error("shared image referenced by the second picture was wrongly collected")
	}
	if !foundNew {
		t.Error("replacement image bytes were not written")
	}
}

// C314: calling SetBackgroundImage twice on a slide must not leak the first
// background image's rel + media part.
func TestSlideSetBackgroundImageTwice_GCsOldImage(t *testing.T) {
	imgA := pngOfColor(t, color.RGBA{R: 255, A: 255})
	imgB := pngOfColor(t, color.RGBA{B: 255, A: 255})

	p := Create()
	s := p.AddSlide()
	if err := s.SetBackgroundImage(imgA, "image/png"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBackgroundImage(imgB, "image/png"); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	if got := mediaPartCount(t, data); got != 1 {
		t.Errorf("after second SetBackgroundImage: media part count = %d, want 1 (first image leaked)", got)
	}
	for name := range zipNames(t, data) {
		if strings.HasPrefix(name, "ppt/media/") && bytes.Equal(zipPart(t, data, name), imgA) {
			t.Errorf("first background image bytes still present in %s", name)
		}
	}
}

// TestEmbedImagePart_DedupHonorsContentType confirms two media parts with
// identical bytes but different content types do not collapse to one, while
// identical bytes with the same content type reuse the existing part (C354).
func TestEmbedImagePart_DedupHonorsContentType(t *testing.T) {
	p := Create()
	data := []byte("shared-bytes")

	png1 := p.embedImagePart(data, "image/png")
	png2 := p.embedImagePart(data, "image/png")
	if png1 != png2 {
		t.Errorf("same bytes + same content type should reuse the part: %q != %q", png1, png2)
	}

	jpeg := p.embedImagePart(data, "image/jpeg")
	if jpeg == png1 {
		t.Errorf("same bytes + different content type must not collapse: both %q", jpeg)
	}
	if p.otherParts[jpeg].ContentType != "image/jpeg" {
		t.Errorf("jpeg part content type = %q, want image/jpeg", p.otherParts[jpeg].ContentType)
	}
}
