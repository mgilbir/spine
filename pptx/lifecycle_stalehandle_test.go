package pptx

import (
	"strings"
	"testing"
)

// C302: a stale slide handle (already removed) must not silently act on whichever
// slide now occupies its old index. A second Delete errors, and Duplicate no-ops.
func TestStaleSlideHandle_RejectedNotReused(t *testing.T) {
	p := Create()
	for _, name := range []string{"S1", "S2", "S3"} {
		p.AddSlide().SetName(name)
	}

	s1, err := p.Slide(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Delete(); err != nil {
		t.Fatalf("first Delete: %v", err)
	}

	// A second Delete on the stale handle must error and not remove S2.
	if err := s1.Delete(); err == nil {
		t.Error("second Delete on a stale handle succeeded, want an error")
	}
	// Duplicate on the stale handle must no-op (return nil) and not add a slide.
	if dup := s1.Duplicate(); dup != nil {
		t.Error("Duplicate on a stale handle returned a slide, want nil")
	}

	var names []string
	for _, s := range p.Slides() {
		names = append(names, s.Name())
	}
	if strings.Join(names, ",") != "S2,S3" {
		t.Errorf("after double Delete, remaining slides = %v, want [S2 S3]", names)
	}
}
