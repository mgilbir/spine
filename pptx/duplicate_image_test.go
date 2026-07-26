package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

// C256: pending image data is embedded only at marshal time
// (processPendingImages). Duplicate snapshotted the slide XML after
// syncShapesToXML (a blip with an empty embed) but before that embed, so the
// duplicate's picture had <a:blip/> with no r:embed and no image relationship.
// Duplicate now flushes pending images before the snapshot, like it already does
// for auto-play timing (C193).
func TestDuplicate_PendingImageEmbedsOnBothSlides(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	if _, err := s.AddPictureFromBytes(minimalTransparentPNG, "image/png"); err != nil {
		t.Fatalf("AddPictureFromBytes: %v", err)
	}

	dup := s.Duplicate()
	if dup == nil {
		t.Fatal("Duplicate returned nil")
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		files[f.Name] = b
	}

	slideRe := regexp.MustCompile(`^ppt/slides/slide\d+\.xml$`)
	embedRe := regexp.MustCompile(`embed="(rId\d+)"`)
	var slides []string
	for name := range files {
		if slideRe.MatchString(name) {
			slides = append(slides, name)
		}
	}
	if len(slides) != 2 {
		t.Fatalf("expected 2 slide parts, got %d: %v", len(slides), slides)
	}

	for _, slideName := range slides {
		m := embedRe.FindSubmatch(files[slideName])
		if m == nil {
			t.Errorf("%s: blip has no r:embed (picture would render blank):\n%s", slideName, files[slideName])
			continue
		}
		embedID := string(m[1])

		// The embed id must resolve to an image relationship in the slide's .rels.
		relsName := strings.Replace(slideName, "ppt/slides/", "ppt/slides/_rels/", 1) + ".rels"
		rels, ok := files[relsName]
		if !ok {
			t.Errorf("%s: no rels part for slide", slideName)
			continue
		}
		if !bytes.Contains(rels, []byte(`Id="`+embedID+`"`)) {
			t.Errorf("%s: embed %q not found in %s:\n%s", slideName, embedID, relsName, rels)
		}
		if !bytes.Contains(rels, []byte("/media/")) {
			t.Errorf("%s: rels does not reference a media part:\n%s", slideName, rels)
		}
	}

	// At least one media part is present (the two pictures dedupe to one part).
	hasMedia := false
	for name := range files {
		if strings.HasPrefix(name, "ppt/media/") {
			hasMedia = true
			break
		}
	}
	if !hasMedia {
		t.Error("no media part written for the duplicated picture")
	}
}
