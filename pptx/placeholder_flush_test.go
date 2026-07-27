package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// C309: on a placeholder loaded from a file, SetOrientation, SetIndex, and
// SetPlaceholderSize must reach the XML on save. Previously SetOrientation was
// a total no-op (placeholderToOxml never wrote orient) and none of the three
// marked the shape dirty or was flushed by updateShapeNode, so all three edits
// were dropped for a materialized placeholder.
func TestPlaceholder_LoadedSettersFlush(t *testing.T) {
	// Build a deck carrying a placeholder, then reopen so the placeholder is a
	// loaded/materialized shape (the dirty-sync path, not the from-scratch
	// marshal path).
	p := Create()
	slide := p.AddSlide()
	ph := NewPlaceholderShape(PlaceholderBody)
	ph.SetPosition(dml.Inches(1), dml.Inches(1))
	ph.SetSize(dml.Inches(4), dml.Inches(3))
	ph.SetIndex(1)
	if err := slide.AddShape(ph); err != nil {
		t.Fatalf("AddShape: %v", err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	loaded := openBytes(t, data)
	lph := loaded.Slides()[0].Placeholders()
	if len(lph) != 1 {
		t.Fatalf("loaded placeholders = %d, want 1", len(lph))
	}
	target := lph[0]

	target.SetOrientation(PlaceholderOrientationVertical)
	target.SetIndex(7)
	target.SetPlaceholderSize(PlaceholderSizeHalf)

	data2, err := loaded.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	reopened := openBytes(t, data2)
	got := reopened.Slides()[0].Placeholders()
	if len(got) != 1 {
		t.Fatalf("reopened placeholders = %d, want 1", len(got))
	}
	g := got[0]
	if g.Orientation() != PlaceholderOrientationVertical {
		t.Errorf("Orientation() = %q, want %q (SetOrientation dropped)", g.Orientation(), PlaceholderOrientationVertical)
	}
	if g.Index() != 7 {
		t.Errorf("Index() = %d, want 7 (SetIndex dropped)", g.Index())
	}
	if g.PlaceholderSize() != PlaceholderSizeHalf {
		t.Errorf("PlaceholderSize() = %q, want %q (SetPlaceholderSize dropped)", g.PlaceholderSize(), PlaceholderSizeHalf)
	}
}
