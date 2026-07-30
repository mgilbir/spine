package pptx

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"
)

// slideXML returns the first slide's XML from a saved deck.
func slideXML(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open slide1: %v", err)
			}
			defer func() { _ = rc.Close() }()
			b, _ := io.ReadAll(rc)
			return string(b)
		}
	}
	t.Fatal("slide1.xml not found")
	return ""
}

func newDeckWithSlide() *Presentation {
	p := Create()
	p.AddSlide()
	return p
}

func TestSetSlideFooter(t *testing.T) {
	p := newDeckWithSlide()
	p.SetSlideFooter("© 2026 — Confidential")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := slideXML(t, data)

	if !strings.Contains(xml, `type="ftr"`) {
		t.Errorf("no footer placeholder in slide:\n%s", xml)
	}
	if !strings.Contains(xml, "© 2026 — Confidential") {
		t.Error("footer text missing")
	}
	// Geometry must be inherited (empty spPr, no xfrm on the ftr placeholder).
	ftr := ftrShape(t, xml)
	if strings.Contains(ftr, "<a:off") || strings.Contains(ftr, "<a:xfrm") {
		t.Errorf("footer placeholder should inherit geometry (no xfrm):\n%s", ftr)
	}

	// Reopen must round-trip.
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}

// ftrShape extracts the <p:sp> containing the ftr placeholder.
func ftrShape(t *testing.T, xml string) string {
	t.Helper()
	i := strings.Index(xml, `type="ftr"`)
	if i < 0 {
		t.Fatal("no ftr placeholder")
	}
	start := strings.LastIndex(xml[:i], "<p:sp>")
	end := strings.Index(xml[i:], "</p:sp>")
	return xml[start : i+end]
}

func TestSetSlideFooterIdempotent(t *testing.T) {
	p := newDeckWithSlide()
	p.SetSlideFooter("first")
	p.SetSlideFooter("second")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := slideXML(t, data)
	if n := strings.Count(xml, `type="ftr"`); n != 1 {
		t.Errorf("expected exactly one footer placeholder, got %d", n)
	}
	if strings.Contains(xml, "first") {
		t.Error("stale footer text present after update")
	}
	if !strings.Contains(xml, "second") {
		t.Error("updated footer text missing")
	}
}

func TestShowSlideNumbers(t *testing.T) {
	p := newDeckWithSlide()
	p.ShowSlideNumbers(true)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := slideXML(t, data)
	if !strings.Contains(xml, `type="sldNum"`) {
		t.Errorf("no slide-number placeholder:\n%s", xml)
	}
	if !strings.Contains(xml, `<a:fld`) || !strings.Contains(xml, `type="slidenum"`) {
		t.Error("slide-number field (a:fld type=slidenum) missing")
	}
	if !strings.Contains(xml, "‹#›") {
		t.Error("slide-number fallback glyph missing")
	}

	// Toggling off removes it.
	p.ShowSlideNumbers(false)
	data, err = p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after off: %v", err)
	}
	if strings.Contains(slideXML(t, data), `type="sldNum"`) {
		t.Error("slide-number placeholder not removed when toggled off")
	}
}

func TestSetSlideDateFixedAndAuto(t *testing.T) {
	p := newDeckWithSlide()
	p.SetSlideDate("2026-01-01")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := slideXML(t, data)
	if !strings.Contains(xml, `type="dt"`) || !strings.Contains(xml, "2026-01-01") {
		t.Errorf("fixed date placeholder/text missing:\n%s", xml)
	}
	// Fixed date is literal text, not an auto field.
	if strings.Contains(xml, `type="datetime"`) {
		t.Error("fixed date should not emit an auto datetime field")
	}

	// Switch the same deck to an auto date field.
	p.SetSlideDateAuto()
	data, err = p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes auto: %v", err)
	}
	xml = slideXML(t, data)
	if !strings.Contains(xml, `type="datetime"`) {
		t.Errorf("auto datetime field missing:\n%s", xml)
	}
	if n := strings.Count(xml, `type="dt"`); n != 1 {
		t.Errorf("expected one date placeholder after switch, got %d", n)
	}
}

func TestFurnitureDeterministic(t *testing.T) {
	// Create stamps Properties.Created/Modified with time.Now() at
	// second granularity, and each build makes a fresh deck. Two builds that
	// land either side of a second boundary therefore differ in
	// docProps/core.xml no matter how deterministic the furniture is — which is
	// what made this test fail roughly once in 300 runs and get written off as
	// "known flaky" rather than read. Pinning the stamps makes the comparison
	// answerable, and keeps core.xml inside it instead of excluding the one part
	// that was actually moving.
	stamp := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	build := func() []byte {
		p := newDeckWithSlide()
		p.Properties.Created = stamp
		p.Properties.Modified = stamp
		p.SetSlideFooter("Confidential")
		p.ShowSlideNumbers(true)
		p.SetSlideDateAuto()
		data, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes: %v", err)
		}
		return data
	}
	if !bytes.Equal(build(), build()) {
		t.Error("furniture output is not deterministic")
	}
}

func TestFurnitureAllSlides(t *testing.T) {
	p := Create()
	p.AddSlide()
	p.AddSlide()
	p.AddSlide()
	p.SetSlideFooter("F")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	count := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			if strings.Contains(string(b), `type="ftr"`) {
				count++
			}
		}
	}
	if count != 3 {
		t.Errorf("footer applied to %d slides, want 3", count)
	}
}

// TestAddPictureFromBytes embeds an image from bytes and defaults to native
// size, mirroring AddPicture but without a file.
func TestAddPictureFromBytes(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	pic, err := s.AddPictureFromBytes(tinyPNG10x8(), "image/png")
	if err != nil {
		t.Fatalf("AddPictureFromBytes: %v", err)
	}
	w, h := pic.Size()
	if w != 10*emuPerPixel || h != 8*emuPerPixel {
		t.Errorf("native size wrong: got %dx%d", w, h)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	// The image must be embedded (a media part + an embed reference).
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	var hasMedia, hasEmbed bool
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/media/") {
			hasMedia = true
		}
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			if strings.Contains(string(b), "embed=") {
				hasEmbed = true
			}
		}
	}
	if !hasMedia || !hasEmbed {
		t.Errorf("image not embedded: media=%v embed=%v", hasMedia, hasEmbed)
	}

	// Empty data is an error.
	if _, err := s.AddPictureFromBytes(nil, "image/png"); err == nil {
		t.Error("expected error for empty image data")
	}
}

// tinyPNG10x8 is a 10x8 white PNG for native-size assertions.
func tinyPNG10x8() []byte {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 10, 8))
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
