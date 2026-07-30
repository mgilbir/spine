package pptx

import (
	"bytes"
	"testing"
	"time"
)

// TestSaveBytesIsIdempotent pins what the furniture flake was really about:
// with the save-time stamp turned off, saving the same deck twice must produce
// the same bytes. The stamp is on by default, so this is the guarantee callers
// opt into when they need reproducible output.
func TestSaveBytesIsIdempotent(t *testing.T) {
	p := newDeckWithSlide()
	p.SetStampModifiedOnSave(false)
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

// TestStampModifiedOnSaveIsDefaultOn pins the other half of the contract: left
// alone, a save records when it happened. Without this the option could be
// silently inverted and only the determinism test would notice — which is to
// say, nothing would.
func TestStampModifiedOnSaveIsDefaultOn(t *testing.T) {
	p := newDeckWithSlide()
	if !p.StampModifiedOnSave() {
		t.Fatal("StampModifiedOnSave() = false, want true by default")
	}
	before := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	p.Properties.Modified = before
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if !p.Properties.Modified.After(before) {
		t.Errorf("Properties.Modified = %v, want it advanced past %v", p.Properties.Modified, before)
	}

	p.SetStampModifiedOnSave(false)
	p.Properties.Modified = before
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if !p.Properties.Modified.Equal(before) {
		t.Errorf("with stamping disabled Properties.Modified = %v, want it left at %v", p.Properties.Modified, before)
	}
}
