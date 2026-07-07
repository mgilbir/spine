package pptx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func zipNames(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(r.File))
	for _, f := range r.File {
		names[f.Name] = true
	}
	return names
}

// C141 (real feature): adding a video and audio to a slide embeds the media
// parts, declares their content types, writes the media/video/audio
// relationships, and produces the p:pic representation PowerPoint expects.
func TestAddVideoAndAudio_Embed(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	slide.AddVideo([]byte("fake-mp4-bytes"), "video/mp4")
	slide.AddAudio([]byte("fake-mp3-bytes"), "audio/mpeg")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	names := zipNames(t, data)
	if !names["ppt/media/media1.mp4"] {
		t.Error("video media part /ppt/media/media1.mp4 was not written")
	}
	if !names["ppt/media/media2.mp3"] {
		t.Error("audio media part /ppt/media/media2.mp3 was not written")
	}

	ct := string(zipPart(t, data, "[Content_Types].xml"))
	for _, want := range []string{"video/mp4", "audio/mpeg", "image/png"} {
		if !strings.Contains(ct, want) {
			t.Errorf("[Content_Types].xml does not declare %q", want)
		}
	}

	slideXML := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	for _, want := range []string{"<p:pic>", "videoFile", "audioFile", "p14:media", "ppaction://media"} {
		if !strings.Contains(slideXML, want) {
			t.Errorf("slide1.xml is missing %q\n%s", want, slideXML)
		}
	}

	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	for _, want := range []string{
		"officeDocument/2006/relationships/video",
		"officeDocument/2006/relationships/audio",
		"office/2007/relationships/media",
	} {
		if !strings.Contains(rels, want) {
			t.Errorf("slide rels missing relationship type %q\n%s", want, rels)
		}
	}

	// The result must reopen cleanly (exercises the p14:media extension parse).
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("saved deck with media does not reopen: %v", err)
	}
}
