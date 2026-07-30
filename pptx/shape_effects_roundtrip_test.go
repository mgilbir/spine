package pptx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// A shape's SpPr on the domain handle is an *overlay*: it starts empty for a
// materialized shape and is merged onto the parsed node at save (mergeSpPr), so
// setting one effect leaves the others alone.
//
// The effect getters read that overlay. Until the materializer carried the
// parsed effects into it, they therefore answered nil for every shape read from
// a file — a shape carrying a red glow reported Glow() == nil while the glow sat
// in the XML two lines away. That is a silent wrong answer rather than a missing
// feature, and it inverts the result of the most natural use:
//
//	if s.Glow() == nil { s.SetGlow(...) }   // fired on shapes that already had one
//
// The coverage sweep found it because the getters had never executed against a
// reopened shape at all: every existing test set an effect and read it back on
// the same in-memory handle, where the overlay is populated by the setter.
//
// So these tests go through save → reopen deliberately. An assertion on the
// authoring handle passes with or without the fix and proves nothing.

// effectFixture builds a one-slide deck whose single AutoShape carries all four
// effects, with distinct values so a getter reading a neighbour's slot fails.
func effectFixture(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s := p.AddSlide()
	sh := NewAutoShape("rect")
	sh.SetBounds(dml.Rect{X: 914400, Y: 914400, Width: 1828800, Height: 914400})
	sh.SetGlow(Glow{Radius: 5, Color: dml.Color{Type: dml.ColorTypeRGB, RGB: dml.RGB{R: 0xFF}}})
	sh.SetSoftEdge(SoftEdge{Radius: 2})
	sh.SetReflection(Reflection{BlurRadius: 3})
	sh.SetBevel(Bevel{Preset: "circle", Width: 4, Height: 6})
	if err := s.AddShape(sh); err != nil {
		t.Fatalf("AddShape: %v", err)
	}
	b, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return b
}

// reopenAutoShape returns the first AutoShape on the first slide of a saved deck.
func reopenAutoShape(t *testing.T, b []byte) *AutoShape {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s, err := p.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	for _, shape := range s.Shapes() {
		if as, ok := shape.(*AutoShape); ok {
			return as
		}
	}
	t.Fatal("no AutoShape on the reopened slide")
	return nil
}

func TestEffectGettersSeeEffectsOnAReopenedShape(t *testing.T) {
	as := reopenAutoShape(t, effectFixture(t))

	g := as.Glow()
	if g == nil {
		t.Error("Glow() = nil on a shape whose XML carries a:glow")
	} else {
		if g.Radius != 5 {
			t.Errorf("Glow().Radius = %v, want 5", g.Radius)
		}
		if got := g.Color.RGB.String(); got != "FF0000" {
			t.Errorf("Glow().Color = %s, want FF0000", got)
		}
	}

	if se := as.SoftEdge(); se == nil {
		t.Error("SoftEdge() = nil on a shape whose XML carries a:softEdge")
	} else if se.Radius != 2 {
		// Distinct from the glow radius on purpose: a getter reading the wrong
		// effect's slot would otherwise agree by coincidence.
		t.Errorf("SoftEdge().Radius = %v, want 2", se.Radius)
	}

	if r := as.Reflection(); r == nil {
		t.Error("Reflection() = nil on a shape whose XML carries a:reflection")
	} else if r.BlurRadius != 3 {
		t.Errorf("Reflection().BlurRadius = %v, want 3", r.BlurRadius)
	}

	if b := as.Bevel(); b == nil {
		t.Error("Bevel() = nil on a shape whose XML carries a:sp3d/a:bevelT")
	} else {
		if b.Preset != "circle" {
			t.Errorf("Bevel().Preset = %q, want circle", b.Preset)
		}
		// Width and height differ so a getter returning one for the other fails.
		if b.Width != 4 || b.Height != 6 {
			t.Errorf("Bevel() = %vx%v, want 4x6", b.Width, b.Height)
		}
	}
}

// TestReadingEffectsDoesNotRegenerateTheShape is the other half. Carrying the
// parsed effects into the overlay must not make the shape look edited: the
// merge at save writes those values back over themselves, so the bytes are
// unchanged, and nothing may set dirty. This is the same "reading is not
// editing" rule the modification-stamp work rests on, applied one layer down.
func TestReadingEffectsDoesNotRegenerateTheShape(t *testing.T) {
	src := effectFixture(t)

	untouched, err := reopenAndSave(t, src, nil)
	if err != nil {
		t.Fatalf("untouched save: %v", err)
	}
	read, err := reopenAndSave(t, src, func(as *AutoShape) {
		_, _, _, _ = as.Glow(), as.SoftEdge(), as.Reflection(), as.Bevel()
	})
	if err != nil {
		t.Fatalf("save after reading: %v", err)
	}
	if !bytes.Equal(untouched, read) {
		t.Errorf("reading the effect getters changed the saved bytes (%d vs %d)",
			len(untouched), len(read))
	}
}

// TestSettingOneEffectKeepsTheParsedOnes pins the property the overlay exists
// for, now that the overlay is pre-populated: replacing one effect on a reopened
// shape must not drop the three it did not touch.
func TestSettingOneEffectKeepsTheParsedOnes(t *testing.T) {
	out, err := reopenAndSave(t, effectFixture(t), func(as *AutoShape) {
		as.SetSoftEdge(SoftEdge{Radius: 9})
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	as := reopenAutoShape(t, out)
	if se := as.SoftEdge(); se == nil || se.Radius != 9 {
		t.Errorf("SoftEdge() = %v, want the updated radius 9", se)
	}
	if as.Glow() == nil {
		t.Error("setting the soft edge dropped the glow")
	}
	if as.Reflection() == nil {
		t.Error("setting the soft edge dropped the reflection")
	}
	if as.Bevel() == nil {
		t.Error("setting the soft edge dropped the bevel")
	}
}

// reopenAndSave opens a deck, optionally applies fn to its first AutoShape, and
// saves it back.
func reopenAndSave(t *testing.T, src []byte, fn func(*AutoShape)) ([]byte, error) {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if fn != nil {
		s, err := p.Slide(0)
		if err != nil {
			t.Fatalf("Slide(0): %v", err)
		}
		for _, shape := range s.Shapes() {
			if as, ok := shape.(*AutoShape); ok {
				fn(as)
				break
			}
		}
	}
	return p.SaveBytes()
}
