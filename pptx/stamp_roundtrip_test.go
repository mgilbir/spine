package pptx

import (
	"bytes"
	"testing"
	"time"
)

// TestOpenedDeckUnaffectedByStamping is the guard that keeps the option from
// eating the library's core promise. Stamping governs newly created decks only;
// an opened deck writes core.xml solely when its properties differ from those
// read at open, so a zero-modification round trip must stay byte-identical with
// stamping left at its default.
func TestOpenedDeckUnaffectedByStamping(t *testing.T) {
	src := newDeckWithSlide()
	src.SetStampModifiedOnSave(false)
	data, err := src.SaveBytes()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	first, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !first.StampModifiedOnSave() {
		t.Fatal("an opened deck should still report the default stamping setting")
	}
	a, err := first.SaveBytes()
	if err != nil {
		t.Fatalf("first save: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	second, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	b, err := second.SaveBytes()
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Error("an untouched opened deck did not round-trip byte-identically across a second boundary")
	}
}
