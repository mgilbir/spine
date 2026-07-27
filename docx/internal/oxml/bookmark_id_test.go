package oxml

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C507: MaxBookmarkID scanned only paragraph-level w:bookmarkStart. Word's
// table column bookmarks (w:colFirst/w:colLast) are direct children of a row,
// a cell, a table or the body, so they were never counted and the next
// allocated bookmark reused an id already in the document.
func TestMaxBookmarkIDSeesNonParagraphBookmarks(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "row-level column bookmark",
			src: `<w:body` + wnsDecl + `><w:tbl><w:tr>` +
				`<w:bookmarkStart w:id="9" w:name="cols" w:colFirst="0" w:colLast="1"/>` +
				`<w:tc><w:p/></w:tc><w:bookmarkEnd w:id="9"/></w:tr></w:tbl></w:body>`,
		},
		{
			name: "cell-level bookmark",
			src: `<w:body` + wnsDecl + `><w:tbl><w:tr><w:tc>` +
				`<w:bookmarkStart w:id="9" w:name="cell"/><w:p/><w:bookmarkEnd w:id="9"/>` +
				`</w:tc></w:tr></w:tbl></w:body>`,
		},
		{
			name: "table-level bookmark",
			src: `<w:body` + wnsDecl + `><w:tbl>` +
				`<w:bookmarkStart w:id="9" w:name="tbl"/>` +
				`<w:tr><w:tc><w:p/></w:tc></w:tr><w:bookmarkEnd w:id="9"/></w:tbl></w:body>`,
		},
		{
			name: "body-level bookmark",
			src: `<w:body` + wnsDecl + `><w:bookmarkStart w:id="9" w:name="body"/>` +
				`<w:p/><w:bookmarkEnd w:id="9"/></w:body>`,
		},
		{
			name: "paragraph-level bookmark (already covered)",
			src: `<w:body` + wnsDecl + `><w:p>` +
				`<w:bookmarkStart w:id="9" w:name="para"/><w:bookmarkEnd w:id="9"/>` +
				`</w:p></w:body>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := &CT_Body{}
			if err := xmlb.UnmarshalWithSource([]byte(tc.src), body); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := MaxBookmarkID(body); got != 9 {
				t.Errorf("MaxBookmarkID = %d, want 9 (a new bookmark would reuse the id)", got)
			}
		})
	}
}
