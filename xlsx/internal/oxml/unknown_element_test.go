package oxml

import (
	"encoding/xml"
	"strings"
	"testing"
)

// TestEncodeUnknownElement_EscapesAttributes verifies that captured unknown
// elements re-emit attribute values with XML metacharacters escaped, rather
// than producing malformed workbook XML (C77).
func TestEncodeUnknownElement_EscapesAttributes(t *testing.T) {
	start := xml.StartElement{
		Name: xml.Name{Space: "", Local: "custom"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "a"}, Value: `x & y`},
			{Name: xml.Name{Local: "b"}, Value: `"quoted" <tag>`},
		},
	}
	got := string(encodeUnknownElement(start, nil, map[string]string{}))

	if strings.Contains(got, `a="x & y"`) {
		t.Errorf("raw ampersand not escaped: %q", got)
	}
	if !strings.Contains(got, `a="x &amp; y"`) {
		t.Errorf("expected escaped ampersand, got: %q", got)
	}
	if !strings.Contains(got, `b="&quot;quoted&quot; &lt;tag&gt;"`) {
		t.Errorf("expected escaped quotes/angles, got: %q", got)
	}
}

// TestEncodeUnknownElement_InlineNamespaceDecl verifies that a prefix declared
// on the unknown element itself (not at the root) is used when re-encoding the
// element and its attributes, instead of being stripped and re-namespacing the
// content into the default namespace (C201).
func TestEncodeUnknownElement_InlineNamespaceDecl(t *testing.T) {
	start := xml.StartElement{
		Name: xml.Name{Space: "urn:foo", Local: "custom"},
		Attr: []xml.Attr{
			{Name: xml.Name{Space: "xmlns", Local: "foo"}, Value: "urn:foo"},
			{Name: xml.Name{Space: "urn:foo", Local: "val"}, Value: "1"},
		},
	}
	rootMap := map[string]string{"urn:other": "oth"}
	got := string(encodeUnknownElement(start, []byte("<foo:inner/>"), rootMap))

	want := `<foo:custom xmlns:foo="urn:foo" foo:val="1"><foo:inner/></foo:custom>`
	if got != want {
		t.Errorf("encodeUnknownElement = %q, want %q", got, want)
	}
	// The caller's root map must not be polluted for sibling elements.
	if len(rootMap) != 1 || rootMap["urn:other"] != "oth" {
		t.Errorf("root prefix map mutated: %v", rootMap)
	}
}
