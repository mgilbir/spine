package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

func hasCode(r validate.Report, code string, sev validate.Severity) bool {
	for _, e := range r {
		if e.Code == code && e.Severity == sev {
			return true
		}
	}
	return false
}

// A freshly created deck must validate clean (no error-severity findings).
func TestValidate_CreatedDeckClean(t *testing.T) {
	p := Create()
	p.AddSlide().AddTextBox().TextFrame().SetText("hi")
	if r := p.Validate(); r.HasErrors() {
		t.Fatalf("created deck reported validation errors: %v", r.Errors())
	}
}

// Duplicate cNvPr ids within a slide are an error (ST_DrawingElementId).
func TestValidate_DuplicateShapeID(t *testing.T) {
	sp1 := &oxml.Shape{NvSpPr: &oxml.NvSpPr{CNvPr: &dml.CNvPr{Id: 7, Name: "A"}}}
	sp2 := &oxml.Shape{NvSpPr: &oxml.NvSpPr{CNvPr: &dml.CNvPr{Id: 7, Name: "B"}}}
	tree := &oxml.ShapeTree{Sp: []*oxml.Shape{sp1, sp2}}
	slide := &Slide{
		partName: "/ppt/slides/slide1.xml",
		slideXML: &oxml.Slide{CSld: &oxml.CommonSlideData{SpTree: tree}},
	}
	p := &Presentation{slides: []*Slide{slide}}

	r := p.Validate()
	if !hasCode(r, codeShapeIDDup, validate.SeverityError) {
		t.Fatalf("expected shape-id-dup error, got: %v", r)
	}

	// Distinct ids: clean.
	sp2.NvSpPr.CNvPr.Id = 8
	if r := p.Validate(); hasCode(r, codeShapeIDDup, validate.SeverityError) {
		t.Fatalf("did not expect shape-id-dup after fixing ids, got: %v", r)
	}
}

// C139: created 4:3 and 16:9 decks have baked layouts/master whose placeholder
// extents fit within the slide, and span (roughly) the content width.
func TestC139_CreatedLayoutGeometryFitsSlide(t *testing.T) {
	cases := []struct {
		name string
		p    *Presentation
	}{
		{"4:3", Create()},
		{"16:9", CreateWidescreen()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, h := tc.p.slideDimensions()
			if w <= 0 || h <= 0 {
				t.Fatalf("bad slide dimensions %dx%d", w, h)
			}
			check := func(part string, phs []*PlaceholderShape) {
				for _, ph := range phs {
					x, y := ph.Position()
					cx, cy := ph.Size()
					if x < 0 || y < 0 || cx <= 0 || cy <= 0 {
						t.Errorf("%s: placeholder has non-positive geometry off=(%d,%d) ext=(%d,%d)", part, x, y, cx, cy)
					}
					if x+cx > w {
						t.Errorf("%s: placeholder overflows slide width: x+cx=%d > w=%d", part, x+cx, w)
					}
					if y+cy > h {
						t.Errorf("%s: placeholder overflows slide height: y+cy=%d > h=%d", part, y+cy, h)
					}
				}
			}
			for _, m := range tc.p.slideMasters {
				check("master", m.Placeholders())
			}
			for _, l := range tc.p.slideLayouts {
				check("layout "+string(l.Type()), l.Placeholders())
			}
			// At least one title placeholder should span most of the width
			// (coherence, not a hard-coded widescreen constant on a 4:3 slide).
			spanned := false
			for _, l := range tc.p.slideLayouts {
				for _, ph := range l.Placeholders() {
					if cx, _ := ph.Size(); cx*10 >= w*8 {
						spanned = true
					}
				}
			}
			if !spanned {
				t.Errorf("no layout placeholder spans ~80%% of slide width %d", w)
			}
		})
	}
}
