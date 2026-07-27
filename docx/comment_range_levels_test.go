package docx

import (
	"strings"
	"testing"
)

// C371: Word anchors a comment on a whole row or a whole cell by placing the
// w:commentRangeStart/End markers as direct children of w:tr or w:tc (they are
// EG_RunLevelElts, valid at body, table, row and cell level). None of the four
// raw-child whitelists carried them, so a regenerated document.xml kept the
// comment definition but destroyed its anchor range.
func TestCommentRangeMarkersSurviveAtEveryBlockLevel(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "row level",
			body: `<w:body><w:tbl><w:tr>` +
				`<w:commentRangeStart w:id="1"/>` +
				`<w:tc><w:p><w:r><w:t>cell</w:t></w:r></w:p></w:tc>` +
				`<w:commentRangeEnd w:id="1"/>` +
				`</w:tr></w:tbl><w:p/></w:body>`,
		},
		{
			name: "cell level",
			body: `<w:body><w:tbl><w:tr><w:tc>` +
				`<w:commentRangeStart w:id="1"/>` +
				`<w:p><w:r><w:t>cell</w:t></w:r></w:p>` +
				`<w:commentRangeEnd w:id="1"/>` +
				`</w:tc></w:tr></w:tbl><w:p/></w:body>`,
		},
		{
			name: "body level",
			body: `<w:body>` +
				`<w:commentRangeStart w:id="1"/>` +
				`<w:p><w:r><w:t>text</w:t></w:r></w:p>` +
				`<w:commentRangeEnd w:id="1"/>` +
				`</w:body>`,
		},
		{
			name: "table level",
			body: `<w:body><w:tbl>` +
				`<w:commentRangeStart w:id="1"/>` +
				`<w:tr><w:tc><w:p><w:r><w:t>cell</w:t></w:r></w:p></w:tc></w:tr>` +
				`<w:commentRangeEnd w:id="1"/>` +
				`</w:tbl><w:p/></w:body>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := fixtureWithDocument(t, fixtureWNS, tc.body)
			doc := openFixture(t, fixture)
			// A realistic mutation flips document.xml to regenerate from the
			// model; before the fix that is where the anchor was deleted.
			doc.AddParagraphWithText("appended")
			saved := saveDoc(t, doc)

			out := mustZipEntry(t, saved, "word/document.xml")
			for _, want := range []string{`<w:commentRangeStart w:id="1"/>`, `<w:commentRangeEnd w:id="1"/>`} {
				if !strings.Contains(out, want) {
					t.Errorf("comment anchor %s destroyed on save:\n%s", want, out)
				}
			}
		})
	}
}
