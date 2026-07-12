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

// TestWriteElement_CarriageReturnInText verifies that a literal \r in text
// content is written as &#xD; (C177). XML §2.11 end-of-line handling makes
// every conforming parser normalize a raw \r (or \r\n) in element content to
// \n, so only the character reference survives a reparse.
func TestWriteElement_CarriageReturnInText(t *testing.T) {
	b := NewSpreadsheetMLBuilder()
	b.WriteElement(NSSpreadsheetML, "t", "line1\rline2\r\nline3\nline4")
	got := b.String()

	if !strings.Contains(got, "line1&#xD;line2&#xD;\nline3\nline4") {
		t.Errorf("expected \\r written as &#xD; with \\n and \\t untouched, got: %q", got)
	}
	if strings.Contains(got, "\r") {
		t.Errorf("raw \\r leaked into text content (parsers normalize it to \\n): %q", got)
	}
}

// TestStripInvalidXMLChars_NonCharAndInvalidUTF8 verifies rune-level XML 1.0
// Char-production filtering (C213): U+FFFE/U+FFFF and invalid UTF-8 sequences
// (including lone surrogates encoded as raw bytes) are stripped, while valid
// characters — including U+FFFD and supplementary-plane runes — survive.
func TestStripInvalidXMLChars_NonCharAndInvalidUTF8(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"U+FFFE", "a\uFFFEb", "ab"},
		{"U+FFFF", "a\uFFFFb", "ab"},
		{"invalid UTF-8 byte", "a\xffb", "ab"},
		{"truncated sequence", "a\xe2\x82b", "ab"},
		{"lone surrogate bytes", "a\xed\xa0\x80b", "ab"}, // U+D800 as raw UTF-8
		{"U+FFFD kept", "a�b", "a�b"},
		{"supplementary plane kept", "a\U0001F600b", "a\U0001F600b"},
		{"CJK kept", "漢字", "漢字"},
		{"C0 controls still stripped", "a\x00b\x01c", "abc"},
		{"legal whitespace kept", "a\tb\nc\rd", "a\tb\nc\rd"},
	}
	for _, c := range cases {
		if got := stripInvalidXMLChars(c.in); got != c.want {
			t.Errorf("%s: stripInvalidXMLChars(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestStripInvalidXMLChars_NoAllocOnCleanInput guards the zero-allocation
// fast path: already-valid input (ASCII or multi-byte) must be returned
// unchanged without allocating.
func TestStripInvalidXMLChars_NoAllocOnCleanInput(t *testing.T) {
	for _, s := range []string{
		"plain ascii with\ttabs and\nnewlines",
		"multi-byte 漢字 and \U0001F600 emoji",
	} {
		var got string
		allocs := testing.AllocsPerRun(100, func() {
			got = stripInvalidXMLChars(s)
		})
		if allocs != 0 {
			t.Errorf("stripInvalidXMLChars(%q) allocated %.0f times on clean input, want 0", s, allocs)
		}
		if got != s {
			t.Errorf("clean input modified: got %q, want %q", got, s)
		}
	}
}

// TestWriteElement_StripsNonCharsFromAttrAndText verifies that the rune-level
// filter is applied on both the attribute and text paths (C213).
func TestWriteElement_StripsNonCharsFromAttrAndText(t *testing.T) {
	b := NewSpreadsheetMLBuilder()
	b.WriteElement(NSSpreadsheetML, "t", "x\uFFFFy", Attr{Name: "v", Value: "p\uFFFEq\xffr"})
	got := b.String()
	if !strings.Contains(got, ">xy<") {
		t.Errorf("U+FFFF not stripped from text content: %q", got)
	}
	if !strings.Contains(got, `v="pqr"`) {
		t.Errorf("U+FFFE / invalid UTF-8 not stripped from attribute: %q", got)
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
