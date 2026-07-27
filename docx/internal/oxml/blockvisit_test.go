package oxml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

func parseBody(t *testing.T, src string) *CT_Body {
	t.Helper()
	body := &CT_Body{}
	if err := xmlb.UnmarshalWithSource([]byte(src), body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return body
}

func paraTexts(ps []*CT_P) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Text())
	}
	return out
}

// C412: collectTableParagraphs walked only tbl.Tr → tr.Tc, so paragraphs inside
// a block SDT wrapping rows (tbl.SdtBlock) or wrapping a cell (tr.SdtCell) were
// invisible to AllParagraphs — and therefore to ReplaceText, the hyperlink and
// image readers, MaxRevisionID and allBookmarkParagraphs. Its siblings
// tblHasMath and collectTableSdt already descended both, which is what made
// this a divergence rather than a decision.
func TestCollectTableParagraphsDescendsSdtWrappers(t *testing.T) {
	src := `<w:body` + wnsDecl + `><w:tbl>` +
		`<w:sdt><w:sdtContent><w:tr><w:tc><w:p><w:r><w:t>in-sdt-row</w:t></w:r></w:p></w:tc></w:tr></w:sdtContent></w:sdt>` +
		`<w:tr>` +
		`<w:sdt><w:sdtContent><w:tc><w:p><w:r><w:t>in-sdt-cell</w:t></w:r></w:p></w:tc></w:sdtContent></w:sdt>` +
		`<w:tc><w:p><w:r><w:t>plain-cell</w:t></w:r></w:p></w:tc>` +
		`</w:tr>` +
		`</w:tbl></w:body>`
	body := parseBody(t, src)
	got := strings.Join(paraTexts(body.AllParagraphs()), "|")
	for _, want := range []string{"in-sdt-row", "in-sdt-cell", "plain-cell"} {
		if !strings.Contains(got, want) {
			t.Errorf("AllParagraphs missed %q; got %q", want, got)
		}
	}
}

// C508: contentParagraphs for an SDT wrapping cells or rows read only tc.P, so
// a nested table or a nested SDT inside such a cell was invisible.
func TestSdtCellContentParagraphsDescendNested(t *testing.T) {
	src := `<w:body` + wnsDecl + `><w:tbl><w:tr>` +
		`<w:sdt><w:sdtContent><w:tc>` +
		`<w:p><w:r><w:t>outer</w:t></w:r></w:p>` +
		`<w:tbl><w:tr><w:tc><w:p><w:r><w:t>nested-tbl</w:t></w:r></w:p></w:tc></w:tr></w:tbl>` +
		`<w:sdt><w:sdtContent><w:p><w:r><w:t>nested-sdt</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
		`</w:tc></w:sdtContent></w:sdt>` +
		`</w:tr></w:tbl></w:body>`
	body := parseBody(t, src)
	got := strings.Join(paraTexts(body.AllParagraphs()), "|")
	for _, want := range []string{"outer", "nested-tbl", "nested-sdt"} {
		if !strings.Contains(got, want) {
			t.Errorf("AllParagraphs missed %q; got %q", want, got)
		}
	}
}

// C413: maxTableRevisionID read trPr/tcPr change records but never the ids of
// the row-level w:ins/w:del wrappers, nor descended SDT wrappers, so a new
// revision could be allocated an id already in use.
func TestMaxRevisionIDSeesRowLevelTrackChanges(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "row-level ins wrapper",
			src: `<w:body` + wnsDecl + `><w:tbl><w:tr>` +
				`<w:ins w:id="42" w:author="a"/>` +
				`<w:tc><w:p/></w:tc></w:tr></w:tbl></w:body>`,
		},
		{
			name: "row-level del wrapper",
			src: `<w:body` + wnsDecl + `><w:tbl><w:tr>` +
				`<w:del w:id="42" w:author="a"/>` +
				`<w:tc><w:p/></w:tc></w:tr></w:tbl></w:body>`,
		},
		{
			name: "trPrChange inside an sdt-wrapped row",
			src: `<w:body` + wnsDecl + `><w:tbl><w:sdt><w:sdtContent><w:tr>` +
				`<w:trPr><w:trPrChange w:id="42" w:author="a"><w:trPr/></w:trPrChange></w:trPr>` +
				`<w:tc><w:p/></w:tc></w:tr></w:sdtContent></w:sdt></w:tbl></w:body>`,
		},
		{
			name: "tcPrChange inside an sdt-wrapped cell",
			src: `<w:body` + wnsDecl + `><w:tbl><w:tr><w:sdt><w:sdtContent><w:tc>` +
				`<w:tcPr><w:tcPrChange w:id="42" w:author="a"><w:tcPr/></w:tcPrChange></w:tcPr>` +
				`<w:p/></w:tc></w:sdtContent></w:sdt></w:tr></w:tbl></w:body>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := parseBody(t, tc.src)
			if got := MaxRevisionID(body); got != 42 {
				t.Errorf("MaxRevisionID() = %d, want 42 (a new revision would collide)", got)
			}
		})
	}
}
