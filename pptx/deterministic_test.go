package pptx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Parts written by iterating a map (media, relationships, ...) must land in a
// deterministic order; otherwise the same input produces byte-different packages
// on different process runs, since Go randomizes map iteration per process. We
// assert the media parts (which come from the otherParts map) are written in
// sorted order — a stable, deterministic ordering.
func TestSaveBytes_DeterministicPartOrder(t *testing.T) {
	dir := t.TempDir()
	p := Create()
	s := p.AddSlide()

	// Several distinct images become distinct /ppt/media/ parts (image1.png..
	// image12.png). With >=10 parts, lexical order (image1, image10, image11,
	// image12, image2, ...) differs from insertion order, so a map-ordered write
	// would not be sorted — making the assertion below reliably catch a
	// regression, not just occasionally.
	for i := 0; i < 12; i++ {
		path := filepath.Join(dir, fmt.Sprintf("img%02d.png", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("fake-png-%d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddPicture(path); err != nil {
			t.Fatal(err)
		}
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var media []string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/media/") {
			media = append(media, f.Name)
		}
	}
	if len(media) < 3 {
		t.Fatalf("expected several media parts, got %v", media)
	}
	if !sort.StringsAreSorted(media) {
		t.Errorf("media parts not written in deterministic (sorted) order: %v", media)
	}
}
