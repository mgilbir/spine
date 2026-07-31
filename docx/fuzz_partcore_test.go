package docx

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/internal/fuzzseed"
)

// Parse and re-emit one part, with no package around it.
//
// The part-level targets in this package each build a zip, open it, save it and
// open it again, which costs so much per execution that a fuzzer barely moves:
// FuzzPptxNotesSlideXML manages about 11 executions a second, against 25,000 for
// a target that only parses. At the nightly budget that is a thousand executions
// — enough to run the seeds and hardly mutate them — so the XML surface those
// targets nominally cover is explored by almost nothing.
//
// This target covers the same parser at the speed the fuzzer needs. The oracle
// is a fixed point one step in: the first marshal may legitimately normalize
// what it read, but everything after that has to agree, or the model and the
// writer disagree about what the document says. A parser that drops a child on
// the second pass, or a writer that re-escapes what it already escaped, shows
// up here as a mismatch rather than as a document that quietly changes every
// time it is saved.
func FuzzDocxDocumentPart(f *testing.F) {
	const open = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`
	f.Add([]byte(open + `<w:body><w:p><w:r><w:t>hello</w:t></w:r></w:p></w:body></w:document>`))
	f.Add([]byte{})
	f.Add([]byte("<w:document"))
	f.Add([]byte(open + `<w:body/></w:document>`))
	f.Add([]byte(open + `<w:body><w:p><w:pPr><w:jc w:val="center"/></w:pPr>` +
		`<w:r><w:rPr><w:b/><w:sz w:val="28"/></w:rPr><w:t xml:space="preserve"> spaced </w:t></w:r></w:p>` +
		`<w:tbl><w:tr><w:tc><w:p/></w:tc></w:tr></w:tbl>` +
		`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/></w:sectPr></w:body></w:document>`))
	// Text carrying everything XML has to escape, which is where a writer that
	// re-escapes shows up as growth on every pass.
	f.Add([]byte(open + `<w:body><w:p><w:r><w:t>a &amp; b &lt;c&gt; ]]&gt; &#xD;</w:t></w:r></w:p></w:body></w:document>`))
	// Children the model does not type, which have to survive verbatim.
	f.Add([]byte(open + `<w:background w:color="FF0000"/><w:body><w:customXml w:element="e"><w:p/></w:customXml></w:body></w:document>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if !fuzzseed.NamesAreValid(data) {
			// Faithfully replaying a name that was never a name is correct: the
			// source is what is broken, and this library's contract is to
			// reproduce what it was given. Go's decoder accepts such a name on
			// the way in and rejects it on the way back, so asserting a fixed
			// point over these inputs would demand that the library silently
			// repair its input instead.
			return
		}
		var doc oxml.CT_Document
		if err := xmlb.UnmarshalWithSource(data, &doc); err != nil {
			return // not this part; nothing to say about it
		}
		first, err := marshalDocumentXML(&doc)
		if err != nil {
			return // refusing to write is a legitimate outcome
		}

		second, err := reparseAndMarshal(t, first)
		if err != nil {
			return
		}
		// The comparison starts at the second pass, not the first. The first
		// may legitimately normalize what it read — binding a namespace the
		// source left unbound is a repair this library makes deliberately, and
		// it changes how the content parses next time round. What must not
		// happen is drift *after* that: a document that keeps changing on every
		// save is one whose bytes never settle.
		third, err := reparseAndMarshal(t, second)
		if err != nil {
			return
		}
		if string(second) != string(third) {
			t.Fatalf("saving is not idempotent: the third pass differs from the second\nsecond: %s\nthird:  %s",
				second, third)
		}
	})
}

// reparseAndMarshal reads a part this library wrote and writes it again,
// failing the test when its own output does not parse.
func reparseAndMarshal(t *testing.T, part []byte) ([]byte, error) {
	t.Helper()
	var doc oxml.CT_Document
	if err := xmlb.UnmarshalWithSource(part, &doc); err != nil {
		t.Fatalf("this library wrote a document part it cannot read back: %v\n%s", err, part)
	}
	out, err := marshalDocumentXML(&doc)
	if err != nil {
		// Refusing to write is a legitimate outcome; writing something
		// unreadable is not.
		return nil, err
	}
	return out, nil
}
