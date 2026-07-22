package pptx

import (
	"bytes"
	"testing"
)

// buildDeckWithSlide creates a one-slide deck carrying some text and returns its
// saved bytes.
func buildDeckWithSlide(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	tb.TextFrame().SetText("hello lazy slide")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return data
}

// Opening a deck must not eagerly parse each slide; a slide is parsed lazily on
// first access. A slide round-tripped without being touched keeps its model nil
// and writes its original bytes verbatim.
func TestSlideParsedLazily(t *testing.T) {
	orig := buildDeckWithSlide(t)

	p, err := OpenReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	slides := p.Slides()
	if len(slides) != 1 {
		t.Fatalf("Slides() = %d, want 1", len(slides))
	}
	if slides[0].sxModel != nil {
		t.Errorf("slide model built eagerly at open; want lazy (nil until accessed)")
	}

	// A clean round-trip without touching any slide passes the slide part
	// through verbatim and never materializes the model.
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if p.Slides()[0].sxModel != nil {
		t.Errorf("clean round-trip materialized the slide model; want passthrough")
	}
	origSlide := zipPart(t, orig, "ppt/slides/slide1.xml")
	outSlide := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Equal(origSlide, outSlide) {
		t.Errorf("clean round-trip did not pass slide1.xml through byte-for-byte\norig: %q\nout:  %q", origSlide, outSlide)
	}

	// Accessing a slide materializes its model and yields the real content.
	p2, err := OpenReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s2 := p2.Slides()[0]
	if got := s2.Text(); got != "hello lazy slide" {
		t.Errorf("Text() = %q, want %q", got, "hello lazy slide")
	}
	if s2.sxModel == nil {
		t.Errorf("slide model still nil after Text(); lazy parse did not run")
	}
}
