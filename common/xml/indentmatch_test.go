package xml

import "testing"

func TestEqualIgnoringIndentAcceptsFormattingOnlyDifferences(t *testing.T) {
	cases := []struct {
		name string
		a, b string
	}{
		{
			name: "identical",
			a:    `<?xml version="1.0"?><r><c a="1"/></r>`,
			b:    `<?xml version="1.0"?><r><c a="1"/></r>`,
		},
		{
			name: "tab indentation",
			a:    "<?xml version=\"1.0\"?>\n<r>\n\t<c a=\"1\"/>\n\t<d>\n\t\t<e/>\n\t</d>\n</r>\n",
			b:    `<?xml version="1.0"?><r><c a="1"/><d><e/></d></r>`,
		},
		{
			name: "indentation without newlines",
			// A wild producer that stripped newlines but kept the indent runs
			// (seen on 128b0b554cac5427): the gaps are still pure whitespace.
			a: `<r><c/>        <d/></r>`,
			b: `<r><c/><d/></r>`,
		},
		{
			name: "self-closing versus expanded empty element",
			a:    `<r><c a="1"></c><d/></r>`,
			b:    `<r><c a="1"/><d></d></r>`,
		},
		{
			name: "expanded empty element with space before the close",
			a:    `<r><c a="1" ></c></r>`,
			b:    `<r><c a="1" /></r>`,
		},
		{
			name: "numeric character reference versus the literal character",
			a:    `<r><c char="&#149;" bullet="&#x2022;"/></r>`,
			// Both spellings occur in wild slide masters: &#149; on
			// 7330e2df353e31cc, &#x2022; on 8343569175a72d8d.
			b: "<r><c char=\"\u0095\" bullet=\"\u2022\"/></r>",
		},
		{
			name: "character reference in character data",
			a:    `<r><t>caf&#233;</t></r>`,
			b:    "<r><t>café</t></r>",
		},
		{
			name: "trailing newline after the root",
			a:    "<r><c/></r>\n",
			b:    "<r><c/></r>",
		},
		{
			name: "comments and processing instructions are kept verbatim",
			a:    "<r>\n  <!-- note -->\n  <c/>\n</r>",
			b:    `<r><!-- note --><c/></r>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !EqualIgnoringIndent([]byte(tc.a), []byte(tc.b)) {
				t.Errorf("EqualIgnoringIndent(%q, %q) = false, want true", tc.a, tc.b)
			}
			if !EqualIgnoringIndent([]byte(tc.b), []byte(tc.a)) {
				t.Errorf("EqualIgnoringIndent is not symmetric for %q / %q", tc.a, tc.b)
			}
		})
	}
}

func TestEqualIgnoringIndentRejectsEverythingElse(t *testing.T) {
	cases := []struct {
		name string
		a, b string
	}{
		{
			name: "different attribute value",
			a:    `<r><c a="1"/></r>`,
			b:    `<r><c a="2"/></r>`,
		},
		{
			name: "different attribute order",
			a:    `<r><c a="1" b="2"/></r>`,
			b:    `<r><c b="2" a="1"/></r>`,
		},
		{
			name: "dropped element",
			a:    "<r>\n  <c/>\n  <d/>\n</r>",
			b:    `<r><c/></r>`,
		},
		{
			name: "added element",
			a:    `<r><c/></r>`,
			b:    "<r>\n  <c/>\n  <d/>\n</r>",
		},
		{
			name: "whitespace that is an element's whole content is character data",
			// <a:t> </a:t> holds a space; dropping it changes the document.
			a: `<r><t> </t></r>`,
			b: `<r><t></t></r>`,
		},
		{
			name: "whitespace inside character data",
			a:    `<r><t>a  b</t></r>`,
			b:    `<r><t>a b</t></r>`,
		},
		{
			name: "leading whitespace in character data",
			a:    `<r><t> a</t></r>`,
			b:    `<r><t>a</t></r>`,
		},
		{
			name: "attribute-value normalization is not a spelling choice",
			// A literal newline in an attribute normalizes to a space
			// (XML 3.3.3); the reference does not. They are different values.
			a: "<r><c a=\"x\ny\"/></r>",
			b: `<r><c a="x&#xA;y"/></r>`,
		},
		{
			name: "escaped ampersand is not the literal character",
			a:    `<r><t>a&#38;b</t></r>`,
			b:    `<r><t>a&amp;b</t></r>`,
		},
		{
			name: "escaped angle bracket is not the literal character",
			a:    `<r><t>a&#60;b</t></r>`,
			b:    `<r><t>a&lt;b</t></r>`,
		},
		{
			name: "quot reference is not interchangeable",
			a:    `<r><c a="&#34;"/></r>`,
			b:    `<r><c a="&quot;"/></r>`,
		},
		{
			name: "element renamed",
			a:    `<r><c/></r>`,
			b:    `<r><d/></r>`,
		},
		{
			name: "expanded pair with different attributes",
			a:    `<r><c a="1"></c></r>`,
			b:    `<r><c a="2"/></r>`,
		},
		{
			name: "expanded pair holding whitespace is not an empty element",
			a:    `<r><c> </c></r>`,
			b:    `<r><c/></r>`,
		},
		{
			name: "truncated markup is never comparable",
			a:    `<r><c a="1"`,
			b:    `<r><c a="1"/>`,
		},
		{
			name: "unterminated comment is never comparable",
			a:    `<r><!-- open</r>`,
			b:    `<r></r>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if EqualIgnoringIndent([]byte(tc.a), []byte(tc.b)) {
				t.Errorf("EqualIgnoringIndent(%q, %q) = true, want false", tc.a, tc.b)
			}
			if EqualIgnoringIndent([]byte(tc.b), []byte(tc.a)) {
				t.Errorf("EqualIgnoringIndent(%q, %q) = true, want false", tc.b, tc.a)
			}
		})
	}
}

// TestEqualIgnoringIndentIdenticalTruncatedInput pins that unscannable input is
// rejected on the strength of being unscannable, not merely of differing: two
// byte-identical inputs short-circuit before the scan, which is the only case
// where a malformed document compares equal to itself.
func TestEqualIgnoringIndentIdenticalTruncatedInput(t *testing.T) {
	bad := []byte(`<r><c a="1"`)
	if !EqualIgnoringIndent(bad, bad) {
		t.Error("byte-identical input compared unequal")
	}
	if EqualIgnoringIndent(bad, append([]byte(nil), append(bad, ' ')...)) {
		t.Error("unscannable input compared equal to a different unscannable input")
	}
}

func TestEqualIgnoringIndentCDATA(t *testing.T) {
	// CDATA is opaque: its content is compared verbatim, and it is never
	// mistaken for ignorable whitespace.
	if !EqualIgnoringIndent([]byte("<r>\n  <![CDATA[ x ]]>\n</r>"), []byte(`<r><![CDATA[ x ]]></r>`)) {
		t.Error("indentation around CDATA was treated as significant")
	}
	if EqualIgnoringIndent([]byte(`<r><![CDATA[ x ]]></r>`), []byte(`<r><![CDATA[x]]></r>`)) {
		t.Error("CDATA content difference was ignored")
	}
}

func TestDecodeCharRefRejectsMalformed(t *testing.T) {
	for _, s := range []string{
		"&#;", "&#x;", "&#zz;", "&#149", "&amp;", "&#xD800;", "&#x110000;",
		"&#0;", "&#xFFFE;", "&", "&#",
	} {
		if _, _, ok := decodeCharRef([]byte(s)); ok {
			t.Errorf("decodeCharRef(%q) accepted", s)
		}
	}
}
