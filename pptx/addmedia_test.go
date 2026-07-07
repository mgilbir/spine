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

// C164: media with no bytes cannot become a valid package part; the save must
// fail instead of silently writing an empty part.
func TestAddMedia_RejectsEmptyData(t *testing.T) {
	cases := []struct {
		name string
		add  func(*Slide)
	}{
		{"video nil data", func(s *Slide) { s.AddVideo(nil, "video/mp4") }},
		{"video empty data empty type", func(s *Slide) { s.AddVideo(nil, "") }},
		{"audio empty data", func(s *Slide) { s.AddAudio([]byte{}, "audio/mpeg") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Create()
			tc.add(p.AddSlide())
			if _, err := p.SaveBytes(); err == nil {
				t.Error("SaveBytes succeeded for media with no data")
			}
		})
	}
}

// C164: an empty content type used to produce /ppt/media/mediaN.bin with no
// [Content_Types].xml entry — an OPC-invalid package. The type is now sniffed
// from the data's magic bytes, and unrecognizable data fails the save.
func TestAddMedia_SniffsContentType(t *testing.T) {
	mp4 := append([]byte{0, 0, 0, 24}, []byte("ftypisom....more")...)
	mov := append([]byte{0, 0, 0, 24}, []byte("ftypqt  ....more")...)
	wav := []byte("RIFF\x24\x00\x00\x00WAVEfmt data")
	mp3 := []byte("ID3\x04\x00\x00\x00\x00\x00\x00rest-of-frame")

	cases := []struct {
		name, part, contentType string
		data                    []byte
		audio                   bool
	}{
		{"mp4", "ppt/media/media1.mp4", "video/mp4", mp4, false},
		{"mov", "ppt/media/media1.mov", "video/quicktime", mov, false},
		{"wav", "ppt/media/media1.wav", "audio/wav", wav, true},
		{"mp3", "ppt/media/media1.mp3", "audio/mpeg", mp3, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Create()
			s := p.AddSlide()
			if tc.audio {
				s.AddAudio(tc.data, "")
			} else {
				s.AddVideo(tc.data, "")
			}
			data, err := p.SaveBytes()
			if err != nil {
				t.Fatal(err)
			}
			if !zipNames(t, data)[tc.part] {
				t.Errorf("sniffed media part %s was not written", tc.part)
			}
			if ct := string(zipPart(t, data, "[Content_Types].xml")); !strings.Contains(ct, tc.contentType) {
				t.Errorf("[Content_Types].xml does not declare %q", tc.contentType)
			}
		})
	}
}

// C164: data that is neither typed by the caller nor recognizable fails the
// save instead of producing an unregistered .bin part.
func TestAddMedia_UnknownContentTypeErrors(t *testing.T) {
	p := Create()
	p.AddSlide().AddVideo([]byte("no-magic-bytes-here-at-all"), "")
	if _, err := p.SaveBytes(); err == nil {
		t.Error("SaveBytes succeeded for media with an unrecognizable content type")
	}

	// The failure must hold when an early sync (pre-save ReplaceText) runs
	// first: no unregistered media part may be created by the sync.
	q := Create()
	s := q.AddSlide()
	s.AddTextBox().TextFrame().SetText("{{a}}")
	s.AddVideo([]byte("no-magic-bytes-here-at-all"), "")
	s.ReplaceText(map[string]string{"{{a}}": "x"})
	if _, err := q.SaveBytes(); err == nil {
		t.Error("SaveBytes succeeded after early sync of media with unknown content type")
	}
}

// sniffMediaContentType recognizes the common containers and rejects noise.
func TestSniffMediaContentType(t *testing.T) {
	cases := []struct {
		name, want string
		data       []byte
	}{
		{"mp4 isom", "video/mp4", append([]byte{0, 0, 0, 24}, []byte("ftypisom....")...)},
		{"m4v", "video/mp4", append([]byte{0, 0, 0, 24}, []byte("ftypM4V ....")...)},
		{"quicktime", "video/quicktime", append([]byte{0, 0, 0, 24}, []byte("ftypqt  ....")...)},
		{"m4a", "audio/mp4", append([]byte{0, 0, 0, 24}, []byte("ftypM4A ....")...)},
		{"wav", "audio/wav", []byte("RIFF\x00\x00\x00\x00WAVEfmt ")},
		{"avi", "video/x-msvideo", []byte("RIFF\x00\x00\x00\x00AVI LIST")},
		{"mp3 id3", "audio/mpeg", []byte("ID3\x04\x00\x00\x00\x00\x00\x00\x00\x00")},
		{"mp3 frame sync", "audio/mpeg", []byte{0xFF, 0xFB, 0x90, 0x00, 1, 2, 3, 4, 5, 6, 7, 8}},
		{"ogg", "audio/ogg", []byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00")},
		{"webm", "video/webm", []byte{0x1A, 0x45, 0xDF, 0xA3, 1, 2, 3, 4, 5, 6, 7, 8}},
		{"garbage", "", []byte("hello world, not media at all")},
		{"too short", "", []byte("tiny")},
		{"empty", "", nil},
	}
	for _, tc := range cases {
		if got := sniffMediaContentType(tc.data); got != tc.want {
			t.Errorf("%s: sniffMediaContentType = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// C168: identical bytes under a different declared content type must not be
// deduplicated into one part — each content type gets its own media part, and
// identical bytes with the same type still share one.
func TestMediaDedup_RequiresMatchingContentType(t *testing.T) {
	same := []byte("identical-media-bytes")
	p := Create()
	p.AddSlide().AddVideo(same, "video/mp4")
	p.AddSlide().AddVideo(same, "video/quicktime")
	p.AddSlide().AddVideo(same, "video/mp4") // dedups with the first part

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, data)
	if !names["ppt/media/media1.mp4"] {
		t.Error("mp4 media part was not written")
	}
	if !names["ppt/media/media2.mov"] {
		t.Error("quicktime media part was not written (dedup ignored the content type)")
	}
	for name := range names {
		if strings.HasPrefix(name, "ppt/media/media") &&
			name != "ppt/media/media1.mp4" && name != "ppt/media/media2.mov" {
			t.Errorf("unexpected extra media part %s (same-type dedup broken)", name)
		}
	}
	ct := string(zipPart(t, data, "[Content_Types].xml"))
	for _, want := range []string{"video/mp4", "video/quicktime"} {
		if !strings.Contains(ct, want) {
			t.Errorf("[Content_Types].xml does not declare %q", want)
		}
	}
}
