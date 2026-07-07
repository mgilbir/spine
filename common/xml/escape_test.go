package xml

import (
	"strings"
	"testing"
)

// TestWriteElement_StripsInvalidControlChars verifies that XML-1.0-illegal
// control characters are dropped from both attribute values and text content
// rather than emitted verbatim (which would produce non-well-formed XML).
func TestWriteElement_StripsInvalidControlChars(t *testing.T) {
	b := NewSpreadsheetMLBuilder()
	// U+0000..U+0008, U+000B, U+000C, U+000E..U+001F are illegal; tab, newline,
	// and carriage return are legal and must survive.
	text := "a\x00b\x01c\x08d\x0be\x0cf\x1fg\they\nz"
	b.WriteElement(NSSpreadsheetML, "t", text, Attr{Name: "v", Value: "x\x00y\x1fz\ttab"})
	got := b.String()

	for _, illegal := range []string{"\x00", "\x01", "\x08", "\x0b", "\x0c", "\x1f"} {
		if strings.Contains(got, illegal) {
			t.Errorf("output still contains illegal control char %q: %q", illegal, got)
		}
	}
	// Legal whitespace preserved: text keeps literal tab/newline; the attribute
	// value renders tab as a character reference.
	if !strings.Contains(got, "abcdefg") {
		t.Errorf("expected stripped text to read %q, got: %q", "abcdefg...", got)
	}
	if !strings.Contains(got, `v="xyz&#x9;tab"`) {
		t.Errorf("expected attr with tab char-ref and control chars stripped, got: %q", got)
	}
}

// TestEscapeAttrValue covers the exported helper used to re-emit captured
// unknown-element attributes (xlsx C77).
func TestEscapeAttrValue(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`plain`, `plain`},
		{`a&b`, `a&amp;b`},
		{`a<b>c`, `a&lt;b&gt;c`},
		{`say "hi"`, `say &quot;hi&quot;`},
		{"tab\there", "tab&#x9;here"},
		{"null\x00byte", "nullbyte"},
	}
	for _, c := range cases {
		if got := EscapeAttrValue(c.in); got != c.want {
			t.Errorf("EscapeAttrValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestStartElementWithNS_EscapesNamespaceURI verifies that a namespace URI
// containing XML metacharacters is escaped rather than written raw (C103).
func TestStartElementWithNS_EscapesNamespaceURI(t *testing.T) {
	b := NewBuilder()
	b.StartElementWithNS("", "root", []NSDecl{{Prefix: "x", URI: `http://ex/?a=1&b=2`}})
	b.EndElement("", "root")
	got := b.String()
	if strings.Contains(got, `a=1&b=2`) {
		t.Errorf("namespace URL written with raw & (malformed XML): %q", got)
	}
	if !strings.Contains(got, `a=1&amp;b=2`) {
		t.Errorf("expected escaped ampersand in namespace URL, got: %q", got)
	}
}
