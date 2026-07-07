package pptx

import (
	"bytes"
	"fmt"
	"testing"
)

// C136: a run created with default formatting does not emit an explicit font
// size, so it inherits from its placeholder/layout.
func TestRunToOxml_NoDefaultFontSize(t *testing.T) {
	r := NewRun()
	r.SetText("hi")
	ar := runToOxml(r)
	if ar.RPr != nil && ar.RPr.Sz != 0 {
		t.Errorf("default run emitted sz=%d, want inherit (no size)", ar.RPr.Sz)
	}
}

// C138: duplicating a slide allocates the next part name without skipping
// (burning) a slide number.
func TestDuplicate_DoesNotBurnSlideNumber(t *testing.T) {
	p := Create()
	for p.SlideCount() < 1 {
		p.AddSlide()
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	before := loaded.SlideCount()
	dup := loaded.Slides()[0].Duplicate()

	want := fmt.Sprintf("/ppt/slides/slide%d.xml", before+1)
	if dup.partName != want {
		t.Errorf("Duplicate part name = %q, want %q (a burned number would skip ahead)", dup.partName, want)
	}
}
