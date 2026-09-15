package pptx

import (
	"bytes"
	"sync"
	"testing"
)

// TestConcurrentReadsOfADeck pins that reading a presentation from several
// goroutines is safe. Every accessor below fills a cache on first use — the
// slide parse, the shape list it materializes, the notes model and the modern
// author list — so the goroutines race to be first to touch each one, which is
// the case that matters. Pre-warming them here would hide the thing under test.
//
// It does not make a presentation safe to modify concurrently, which it is not.
//
// The assertion is the race detector: this test only means something under
// -race, a required gate (make test-race).
func TestConcurrentReadsOfADeck(t *testing.T) {
	p := Create()
	for i := 0; i < 12; i++ {
		s := p.AddSlide()
		tb := s.AddTextBox()
		tb.SetText("slide text")
		s.SetNotes("speaker notes")
		s.AddComment("author", "a comment")
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	slides := reopened.Slides()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reopened.SlideMasters()
			_ = reopened.SlideLayouts()
			for _, s := range slides {
				_ = s.Shapes()
				_ = s.Notes()
				_ = s.Comments()
				_ = s.Index()
			}
		}()
	}
	wg.Wait()

	if got := len(slides); got != 12 {
		t.Fatalf("slide count = %d, want 12", got)
	}
	if got := slides[3].Notes(); got != "speaker notes" {
		t.Errorf("notes = %q, want %q", got, "speaker notes")
	}
}
