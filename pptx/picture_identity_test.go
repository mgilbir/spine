package pptx

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// C137 (dedup): adding the same image twice stores a single media part.
func TestAddPicture_DedupsIdenticalMedia(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "img.png")
	if err := os.WriteFile(imgPath, minimalTransparentPNG, 0o644); err != nil {
		t.Fatal(err)
	}

	p := Create()
	s := p.AddSlide()
	if _, err := s.AddPicture(imgPath); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddPicture(imgPath); err != nil {
		t.Fatal(err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	media := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/media/") {
			media++
		}
	}
	if media != 1 {
		t.Errorf("expected 1 deduped media part for identical images, got %d", media)
	}
}

// C137 (correct match): replacing one of two pictures that share a blip embed
// updates the right picture node, not the first one found.
func TestReplaceImage_PicksCorrectSharedPicture(t *testing.T) {
	sld := &oxml.Slide{CSld: &oxml.CommonSlideData{SpTree: &oxml.ShapeTree{
		Pic: []*oxml.Picture{
			{
				NvPicPr:  &oxml.NvPicPr{CNvPr: &dml.CNvPr{Id: 2, Name: "A"}},
				BlipFill: &dml.BlipFill{Blip: &dml.Blip{Embed: "rIdShared"}},
				SpPr:     &dml.SpPr{},
			},
			{
				NvPicPr:  &oxml.NvPicPr{CNvPr: &dml.CNvPr{Id: 3, Name: "B"}},
				BlipFill: &dml.BlipFill{Blip: &dml.Blip{Embed: "rIdShared"}},
				SpPr:     &dml.SpPr{},
			},
		},
	}}}

	p := Create()
	s := p.AddSlide()
	s.sxModel = sld
	s.partName = "/ppt/slides/slide1.xml"
	s.materializeShapes()

	var pics []*Picture
	for _, sh := range s.shapeCache {
		if pic, ok := sh.(*Picture); ok {
			pics = append(pics, pic)
		}
	}
	if len(pics) != 2 {
		t.Fatalf("expected 2 materialized pictures, got %d", len(pics))
	}

	// Replace the image on the SECOND picture (cNvPr id 3).
	pics[1].SetImageData([]byte("NEW-IMAGE-BYTES"), "image/png")
	if err := s.processPendingImages(); err != nil {
		t.Fatal(err)
	}

	if got := sld.CSld.SpTree.Pic[0].BlipFill.Blip.Embed; got != "rIdShared" {
		t.Errorf("first picture (id 2) was changed to %q; the wrong node was replaced", got)
	}
	if got := sld.CSld.SpTree.Pic[1].BlipFill.Blip.Embed; got == "rIdShared" || got == "" {
		t.Errorf("second picture (id 3) embed not updated: %q", got)
	}
}
