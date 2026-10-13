package xml

import "testing"

// TestPartEntryPointsCheckTheProlog pins what both part-level entry points
// accept around and inside the root element. Decode skips what precedes the
// root, and an unmarshal ignores processing instructions, so without these
// checks each "reject" case opened, and a preserved part was saved verbatim.
func TestPartEntryPointsCheckTheProlog(t *testing.T) {
	const decl, bom = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`, "\xEF\xBB\xBF"
	type doc struct {
		V string `xml:"v"`
	}
	for _, tc := range []struct {
		name, in string
		ok       bool
	}{
		{"bare", `<r><v>x</v></r>`, true},
		{"declaration", decl + "\r\n<r/>", true},
		{"byte order mark and declaration", bom + decl + "<r/>", true},
		{"byte order mark alone", bom + "<r/>", true},
		{"misc before and after", decl + "\n<!-- c -->\n<?xml-stylesheet href=\"s.xsl\"?>\n<r/>\n<!-- c -->\n", true},
		{"declaration in a comment", `<r><!-- <?xml version="1.0"?> --><v>x</v></r>`, true},
		{"declaration in CDATA", `<r><v><![CDATA[<?xml version="1.0"?>]]></v></r>`, true},
		{"xml-prefixed target inside", `<r><?xml-stylesheet href="s.xsl"?></r>`, true},
		{"text before the root", "junk<r/>", false},
		{"no-break space before the root", " <r/>", false},
		{"no-break space after the root", "<r/> ", false},
		{"line separator after the root", "<r/> ", false},
		{"byte order mark after the declaration", decl + bom + "<r/>", false},
		{"two byte order marks", bom + bom + "<r/>", false},
		{"white space before the declaration", " " + decl + "<r/>", false},
		{"comment before the declaration", "<!-- c -->" + decl + "<r/>", false},
		{"declaration inside the root", `<r>` + decl + `</r>`, false},
		{"declaration after the root", `<r/>` + decl, false},
		{"upper-case declaration inside", `<r><?XML version="1.0"?></r>`, false},
		{"mixed-case declaration at the start", `<?Xml version="1.0"?><r/>`, false},
	} {
		for _, entry := range []struct {
			name string
			fn   func([]byte, any) error
		}{{"Unmarshal", Unmarshal}, {"UnmarshalWithSource", func(b []byte, v any) error { return UnmarshalWithSource(b, v) }}} {
			var v doc
			err := entry.fn([]byte(tc.in), &v)
			if tc.ok && err != nil {
				t.Errorf("%s: %s: rejected: %v", entry.name, tc.name, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("%s: %s: accepted", entry.name, tc.name)
			}
		}
	}
}

func TestIsSpaceAndIsBlank(t *testing.T) {
	for _, tc := range []struct {
		in           string
		space, blank bool
	}{
		{"", true, true},
		{" \t\r\n", true, true},
		{" ", false, false},
		{"　", false, false},
		{"\u0085", false, false},
		{"\xEF\xBB\xBF", false, true},
		{"\xEF\xBB\xBF \r\n", false, true},
		{"\xFF\xFE", false, true},
		{"\xFF\xFE \x00\n\x00", false, true},
		{"\xEF\xBB\xBF\xEF\xBB\xBF", false, false},
		{"<r/>", false, false},
	} {
		if got := IsSpace([]byte(tc.in)); got != tc.space {
			t.Errorf("IsSpace(%q) = %v", tc.in, got)
		}
		if got := IsBlank([]byte(tc.in)); got != tc.blank {
			t.Errorf("IsBlank(%q) = %v", tc.in, got)
		}
	}
}
