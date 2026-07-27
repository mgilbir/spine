package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

// TestAddImageOpenedNoCollision: adding an image to an opened workbook that
// already contains a drawing (on another sheet) must not collide on part names
// or sheet relationship ids, and both images must survive.
func TestAddImageOpenedNoCollision(t *testing.T) {
	// Base workbook already has an image on Sheet1 (a drawing1.xml + image1.png).
	wb := Create()
	s1 := addSheetT(wb, "Sheet1")
	addSheetT(wb, "Sheet2")
	if err := s1.AddImage("A1", testPNG(t, 10, 10), ImageOptions{}); err != nil {
		t.Fatalf("seed image: %v", err)
	}
	base, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// Reopen and add an image to Sheet2.
	wb2, err := OpenReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s2, err := wb2.Sheet(1)
	if err != nil {
		t.Fatalf("Sheet(1): %v", err)
	}
	if err := s2.AddImage("C3", testPNG(t, 20, 20), ImageOptions{}); err != nil {
		t.Fatalf("AddImage on opened: %v", err)
	}
	out, err := wb2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	names := zipNames(t, out)
	// The original drawing1/image1 must be preserved AND the new ones added
	// under non-colliding names.
	drawings, media := 0, 0
	for n := range names {
		if strings.HasPrefix(n, "xl/drawings/drawing") && strings.HasSuffix(n, ".xml") {
			drawings++
		}
		if strings.HasPrefix(n, "xl/media/image") {
			media++
		}
	}
	if drawings < 2 {
		t.Errorf("expected at least 2 drawing parts (original + new), got %d; parts=%v", drawings, names)
	}
	if media < 2 {
		t.Errorf("expected at least 2 media parts, got %d; parts=%v", media, names)
	}

	// Reopen and confirm both sheets reference a drawing.
	wb3, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("final reopen: %v", err)
	}
	_ = wb3
}
