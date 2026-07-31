package pptx

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Parse and re-emit a slide, with no package around it.
//
// The part-level slide targets each build a deck, replace a part, open, save
// and open again — about 11 executions a second, against tens of thousands
// here. The heavy targets carry the semantic oracles; this one carries the
// parser, at the rate a fuzzer needs to reach past the seeds.
//
// The oracle is idempotence from the second pass: the first may legitimately
// normalize what it read, and everything after has to agree.
func FuzzPptxSlidePart(f *testing.F) {
	const open = `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`
	f.Add([]byte(open + `<p:cSld><p:spTree/></p:cSld></p:sld>`))
	f.Add([]byte{})
	f.Add([]byte("<p:sld"))
	f.Add([]byte(open + `<p:cSld><p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Box"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>` +
		`<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="100" cy="100"/></a:xfrm></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>text</a:t></a:r></a:p></p:txBody></p:sp>` +
		`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`))
	// Text needing escaping, and a group nested into itself.
	f.Add([]byte(open + `<p:cSld><p:spTree><p:grpSp><p:grpSp><p:sp><p:txBody><a:bodyPr/>` +
		`<a:p><a:r><a:t>a &amp; b &lt;c&gt; ]]&gt;</a:t></a:r></a:p></p:txBody></p:sp></p:grpSp></p:grpSp>` +
		`</p:spTree></p:cSld></p:sld>`))
	// An extension list, which is replayed rather than modeled.
	f.Add([]byte(open + `<p:cSld><p:spTree/></p:cSld><p:extLst><p:ext uri="{ABC}">` +
		`<p14:creationId xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" val="1"/>` +
		`</p:ext></p:extLst></p:sld>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if !fuzzseed.NamesAreValid(data) {
			// A name the source made up is replayed as given; see
			// fuzzseed.NamesAreValid.
			return
		}
		var sld oxml.Slide
		if err := xmlb.UnmarshalWithSource(data, &sld); err != nil {
			return
		}
		first, ok := marshalSlidePart(t, &sld)
		if !ok {
			return
		}
		second, ok := reparseSlide(t, first)
		if !ok {
			return
		}
		third, ok := reparseSlide(t, second)
		if !ok {
			return
		}
		if string(second) != string(third) {
			t.Fatalf("saving a slide is not idempotent: the third pass differs from the second\nsecond: %s\nthird:  %s",
				second, third)
		}
	})
}

func marshalSlidePart(t *testing.T, s *oxml.Slide) ([]byte, bool) {
	t.Helper()
	b := xmlb.NewPresentationMLBuilder()
	b.SetCollapseEmptyElements(true)
	b.WriteHeader()
	s.MarshalRootToBuilder(b)
	if err := b.Finish(); err != nil {
		return nil, false
	}
	return b.Bytes(), true
}

func reparseSlide(t *testing.T, part []byte) ([]byte, bool) {
	t.Helper()
	var s oxml.Slide
	if err := xmlb.UnmarshalWithSource(part, &s); err != nil {
		t.Fatalf("this library wrote a slide it cannot read back: %v\n%s", err, part)
	}
	return marshalSlidePart(t, &s)
}
