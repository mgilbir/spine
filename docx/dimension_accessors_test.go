package docx

import (
	"math"
	"testing"
)

// sizedShape is the accessor quartet every geometry-bearing docx handle
// exposes: the OOXML-native EMU pair and the derived point pair. The tests
// below drive every implementation through one table, so a handle added later
// only has to be listed in sizedSubjects to inherit the transposition checks.
type sizedShape interface {
	Width() float64
	Height() float64
	WidthEMU() int64
	HeightEMU() int64
}

// Compile-time roster of the types under test. A new sized handle that forgets
// to appear in sizedSubjects still has to satisfy this list to compile the file.
var (
	_ sizedShape = (*TextBox)(nil)
	_ sizedShape = (*InlineImage)(nil)
)

// Deliberately unequal, and deliberately not a multiple of one another: a
// handle that returns the height for the width (or the width for the height)
// must produce a different number, and so must one that swaps the EMU pair on
// the way in or out of the part. Equal dimensions would make every assertion
// below pass under exactly the bug they exist to catch.
const (
	wantWidthEMU  int64 = 2743200 // 3.00in = 216pt
	wantHeightEMU int64 = 1028700 // 1.125in = 81pt
)

// sizedSubject builds one handle and reports the geometry it was authored with.
type sizedSubject struct {
	name string
	// build returns the handle as first authored (before any save).
	build func(t *testing.T) (*Document, sizedShape)
	// reread returns the handle for the same shape after a save/reopen cycle,
	// so the reader's geometry scan is checked as well as the writer's.
	reread func(t *testing.T, doc *Document) sizedShape
}

func sizedSubjects() []sizedSubject {
	return []sizedSubject{
		{
			name: "TextBox",
			build: func(t *testing.T) (*Document, sizedShape) {
				t.Helper()
				doc := Create()
				tb := doc.AddParagraph().AddTextBox("boxed", TextBoxOptions{
					WidthEMU:  wantWidthEMU,
					HeightEMU: wantHeightEMU,
				})
				return doc, tb
			},
			reread: func(t *testing.T, doc *Document) sizedShape {
				t.Helper()
				boxes := saveAndReopen(t, doc).TextBoxes()
				if len(boxes) != 1 {
					t.Fatalf("got %d text boxes after reopen, want 1", len(boxes))
				}
				return boxes[0]
			},
		},
		{
			name: "Shape",
			build: func(t *testing.T) (*Document, sizedShape) {
				t.Helper()
				doc := Create()
				// Captioned: Document.TextBoxes only reports drawings that carry
				// a w:txbxContent, so a caption-less shape has nothing to reread.
				tb := doc.AddShape("captioned", TextBoxOptions{
					Shape:     ShapeEllipse,
					WidthEMU:  wantWidthEMU,
					HeightEMU: wantHeightEMU,
				})
				return doc, tb
			},
			reread: func(t *testing.T, doc *Document) sizedShape {
				t.Helper()
				boxes := saveAndReopen(t, doc).TextBoxes()
				if len(boxes) != 1 {
					t.Fatalf("got %d shapes after reopen, want 1", len(boxes))
				}
				return boxes[0]
			},
		},
		{
			// The mc:AlternateContent read path is a separate geometry scan
			// from the plain w:drawing one.
			name: "TextBoxVMLFallback",
			build: func(t *testing.T) (*Document, sizedShape) {
				t.Helper()
				doc := Create()
				tb := doc.AddParagraph().AddTextBox("fallback", TextBoxOptions{
					WidthEMU:    wantWidthEMU,
					HeightEMU:   wantHeightEMU,
					VMLFallback: true,
				})
				return doc, tb
			},
			reread: func(t *testing.T, doc *Document) sizedShape {
				t.Helper()
				boxes := saveAndReopen(t, doc).TextBoxes()
				if len(boxes) != 1 {
					t.Fatalf("got %d text boxes after reopen, want 1", len(boxes))
				}
				return boxes[0]
			},
		},
		{
			name: "InlineImage",
			build: func(t *testing.T) (*Document, sizedShape) {
				t.Helper()
				doc := Create()
				img, err := doc.AddParagraph().AddRun().AddImageFromBytes(tinyPNG, "image/png")
				if err != nil {
					t.Fatalf("AddImageFromBytes: %v", err)
				}
				img.SetSize(emuToPoints(wantWidthEMU), emuToPoints(wantHeightEMU))
				return doc, img
			},
			reread: func(t *testing.T, doc *Document) sizedShape {
				t.Helper()
				imgs := saveAndReopen(t, doc).Images()
				if len(imgs) != 1 {
					t.Fatalf("got %d images after reopen, want 1", len(imgs))
				}
				return imgs[0]
			},
		},
		{
			name: "FloatingImage",
			build: func(t *testing.T) (*Document, sizedShape) {
				t.Helper()
				doc := Create()
				img, err := doc.AddParagraph().AddRun().AddFloatingImageFromBytes(
					tinyPNG, "image/png", Anchor{RelativeToPage: true, X: 12, Y: 34})
				if err != nil {
					t.Fatalf("AddFloatingImageFromBytes: %v", err)
				}
				img.SetSize(emuToPoints(wantWidthEMU), emuToPoints(wantHeightEMU))
				return doc, img
			},
			reread: func(t *testing.T, doc *Document) sizedShape {
				t.Helper()
				imgs := saveAndReopen(t, doc).Images()
				if len(imgs) != 1 {
					t.Fatalf("got %d images after reopen, want 1", len(imgs))
				}
				return imgs[0]
			},
		},
	}
}

// checkSized asserts the four accessors independently: each EMU accessor
// against its own authored value, and each point accessor against a point value
// computed here from first principles (72pt per 914400 EMU) rather than through
// the package's own converter, so a converter that reads the wrong field is not
// able to hide behind itself.
func checkSized(t *testing.T, stage string, s sizedShape) {
	t.Helper()
	if got := s.WidthEMU(); got != wantWidthEMU {
		t.Errorf("%s: WidthEMU() = %d, want %d (height is %d — transposed?)",
			stage, got, wantWidthEMU, wantHeightEMU)
	}
	if got := s.HeightEMU(); got != wantHeightEMU {
		t.Errorf("%s: HeightEMU() = %d, want %d (width is %d — transposed?)",
			stage, got, wantHeightEMU, wantWidthEMU)
	}
	wantWidthPt := float64(wantWidthEMU) * 72 / 914400
	wantHeightPt := float64(wantHeightEMU) * 72 / 914400
	if got := s.Width(); math.Abs(got-wantWidthPt) > 1e-9 {
		t.Errorf("%s: Width() = %v pt, want %v pt (height is %v pt — transposed?)",
			stage, got, wantWidthPt, wantHeightPt)
	}
	if got := s.Height(); math.Abs(got-wantHeightPt) > 1e-9 {
		t.Errorf("%s: Height() = %v pt, want %v pt (width is %v pt — transposed?)",
			stage, got, wantHeightPt, wantWidthPt)
	}
	// The unit relationship must hold per axis, which fails the moment one of
	// the two forms reads the other axis' field.
	if got, want := s.Width(), emuToPoints(s.WidthEMU()); math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: Width() = %v pt but WidthEMU() converts to %v pt", stage, got, want)
	}
	if got, want := s.Height(), emuToPoints(s.HeightEMU()); math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: Height() = %v pt but HeightEMU() converts to %v pt", stage, got, want)
	}
}

// TestSizedShape_AccessorsAreNotTransposed drives every width/height accessor
// pair with deliberately unequal dimensions. Only one of the four accessors on
// each type was exercised anywhere before this, so a handle that returned the
// width from HeightEMU (or the height from Width) round-tripped silently.
func TestSizedShape_AccessorsAreNotTransposed(t *testing.T) {
	if wantWidthEMU == wantHeightEMU {
		t.Fatal("the fixture dimensions must differ or this test cannot fail for its own bug")
	}
	for _, subj := range sizedSubjects() {
		t.Run(subj.name, func(t *testing.T) {
			doc, shape := subj.build(t)
			checkSized(t, "authored", shape)
			checkSized(t, "reopened", subj.reread(t, doc))
		})
	}
}

// TestEMUToPoints pins the conversion itself, since every point accessor above
// is derived from it and a wrong constant would move all of them together.
func TestEMUToPoints(t *testing.T) {
	cases := []struct {
		emu  int64
		want float64
	}{
		{0, 0},
		{914400, 72},   // one inch
		{12700, 1},     // one point
		{-914400, -72}, // negative offsets are legal in a drawing
		{457200, 36},   // half an inch
		{1028700, 81},  // the fixture height
		{2743200, 216}, // the fixture width
	}
	for _, c := range cases {
		if got := emuToPoints(c.emu); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("emuToPoints(%d) = %v, want %v", c.emu, got, c.want)
		}
	}
}
