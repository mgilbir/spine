package pptx

import (
	"bytes"
	"testing"
	"testing/synctest"
	"time"
)

// TestSaveBytesIsIdempotent pins the property the furniture flake was really
// about: saving the same deck twice must produce the same bytes. saveNew used
// to stamp Properties.Modified with time.Now() on every save, so two saves
// either side of a second boundary differed for an unchanged deck.
//
// The second boundary is crossed under a synctest bubble's fake clock, so it
// costs no real time. The sleep is not decoration: a per-save stamp within one
// second of the first save serializes to the same RFC3339 value and would slip
// through unnoticed — which is precisely how this bug survived three audits,
// showing up only as a ~1-in-300 TestFurnitureDeterministic flake.
func TestSaveBytesIsIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newDeckWithSlide()
		p.SetSlideFooter("Confidential")
		first, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("first SaveBytes: %v", err)
		}
		// Cross a second boundary: under the old behaviour this guaranteed a diff.
		time.Sleep(time.Second)
		second, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("second SaveBytes: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Error("SaveBytes is not idempotent: saving an unchanged deck twice produced different bytes")
		}
	})
}
