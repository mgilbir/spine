package oxml

import "testing"

const goodComments = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
	`<authors><author>Ada</author></authors>` +
	`<commentList><comment ref="A1" authorId="0"><text><t>hello</t></text></comment></commentList>` +
	`</comments>`

// A part read off a package must be well-formed to its end and must bind every
// prefix it uses. xlsx parsed its parts through encoding/xml, which enforces
// neither, so xlsx alone was outside a rule docx and pptx had been held to —
// not by decision, but because nobody migrated it.
//
// The consequence is not that a bad part is rejected. It is that a bad part was
// *accepted*, modeled, and then re-serialized on save from a model built out of
// content the parser had silently reinterpreted.
func TestCommentsPartRejectsWhatItCannotReadBack(t *testing.T) {
	if _, err := ParseComments([]byte(goodComments)); err != nil {
		t.Fatalf("a well-formed comments part no longer parses: %v", err)
	}

	for _, tc := range []struct {
		name string
		data string
	}{
		{
			// Content after the root element. encoding/xml stops at the root's
			// end tag and never looks further.
			name: "trailing content",
			data: goodComments + `</comments>`,
		},
		{
			// A prefix nothing declares. Go resolves it to the prefix itself
			// rather than failing, so the name silently means something else.
			name: "unbound prefix",
			data: `<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
				`<commentList><ghost:comment ref="A1" authorId="0"/></commentList></comments>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseComments([]byte(tc.data)); err == nil {
				t.Error("parsed a part this library could not write back unchanged")
			}
		})
	}
}

// The same standard, on the other two comment part kinds in this package.
func TestThreadedAndPersonPartsUseTheCheckedEntryPoint(t *testing.T) {
	trailing := `<threadedComments xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments"/>` +
		`<extra/>`
	if _, err := ParseThreadedComments([]byte(trailing)); err == nil {
		t.Error("threaded comments accepted content after the root element")
	}
	trailingPersons := `<personList xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments"/>` +
		`<extra/>`
	if _, err := ParsePersonList([]byte(trailingPersons)); err == nil {
		t.Error("person list accepted content after the root element")
	}
}
