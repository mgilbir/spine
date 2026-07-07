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
