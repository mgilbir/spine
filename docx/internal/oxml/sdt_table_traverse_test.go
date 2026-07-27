package oxml

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C330: contentParagraphs had no bodyChildTbl case and its untracked path
// ignored sc.Tbl, so paragraphs inside a table wrapped by a block-level SDT
// never surfaced through AllParagraphs()/Paragraphs() — ReplaceText and the
// revision walkers missed them — and MaxRevisionID (which walked only
// body.Tbl) could hand out a revision id that already existed inside the
// SDT-wrapped table. Both traversals must descend into SDT-wrapped tables.
func TestSdtWrappedTableParagraphsAndRevisions(t *testing.T) {
	const nsW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	src := `<w:document xmlns:w="` + nsW + `"><w:body>` +
		`<w:p><w:r><w:t>top</w:t></w:r></w:p>` +
		`<w:sdt><w:sdtContent><w:tbl>` +
		`<w:tr><w:trPr><w:ins w:id="9" w:author="a"/></w:trPr><w:tc>` +
		`<w:p><w:ins w:id="7" w:author="a"><w:r><w:t>inSdtTable</w:t></w:r></w:ins></w:p>` +
		`</w:tc></w:tr>` +
		`</w:tbl></w:sdtContent></w:sdt>` +
		`<w:sectPr/></w:body></w:document>`

	doc := &CT_Document{}
	if err := xmlb.UnmarshalWithSource([]byte(src), doc); err != nil {
		t.Fatal(err)
	}
	if doc.Body == nil {
		t.Fatal("body did not parse")
	}

	// AllParagraphs must reach the paragraph inside the SDT-wrapped table.
	var found bool
	for _, p := range doc.Body.AllParagraphs() {
		if p.Text() == "inSdtTable" {
			found = true
		}
	}
	if !found {
		t.Errorf("AllParagraphs() did not surface the paragraph inside the SDT-wrapped table; got %d paragraphs", len(doc.Body.AllParagraphs()))
	}

	// MaxRevisionID must see both the content insertion (id 7) and the structural
	// row insertion (id 9) inside that table, so a newly authored revision does
	// not collide with either. Id 9 exercises maxSdtBlockTableRevisionID.
	if got := MaxRevisionID(doc.Body); got != 9 {
		t.Errorf("MaxRevisionID = %d, want 9 (revision inside SDT-wrapped table not seen)", got)
	}
}
