package pptx

import (
	"bytes"
	"testing"
	"time"
)

// TestSaveBytesIsIdempotent pins the property the furniture flake was really
// about: saving the same deck twice must produce the same bytes. saveNew used
// to stamp Properties.Modified with time.Now() on every save, so two saves
// either side of a second boundary differed for an unchanged deck.
func TestSaveBytesIsIdempotent(t *testing.T) {
	p := newDeckWithSlide()
	p.SetSlideFooter("Confidential")
	first, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	// Cross a second boundary: under the old behaviour this guaranteed a diff.
	time.Sleep(1100 * time.Millisecond)
	second, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("SaveBytes is not idempotent: saving an unchanged deck twice produced different bytes")
	}
}
