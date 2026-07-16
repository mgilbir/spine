package xml

import (
	"io"
	"strings"
	"testing"
)

func TestCharsetReaderTranscoding(t *testing.T) {
	tests := []struct {
		name    string
		charset string
		in      string
		want    string
		wantErr bool
	}{
		{"utf8 passthrough", "utf-8", "hello", "hello", false},
		{"empty passthrough", "", "hello", "hello", false},
		{"us-ascii passthrough", "us-ascii", "hello", "hello", false},
		{"ascii passthrough", "ascii", "plain", "plain", false},
		// ISO-8859-1: 0xE9 -> é (U+00E9), 0xFC -> ü (U+00FC)
		{"latin1 accents", "iso-8859-1", "caf\xe9 \xfc", "café ü", false},
		{"latin1 alias l1", "l1", "\xe9", "é", false},
		// Windows-1252 curly quote 0x92 -> U+2019, é still 0xE9
		{"win1252 curly quote", "windows-1252", "John\x92s caf\xe9", "John’s café", false},
		// euro 0x80 -> U+20AC
		{"win1252 euro", "cp1252", "\x80", "€", false},
		// undefined 0x81 -> replacement char
		{"win1252 undefined", "windows-1252", "\x81", "�", false},
		// win1252 low bytes still ASCII
		{"win1252 ascii", "win-1252", "abc", "abc", false},
		{"unknown charset", "shift_jis", "x", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := CharsetReader(tt.charset, strings.NewReader(tt.in))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for charset %q, got nil", tt.charset)
				}
				return
			}
			if err != nil {
				t.Fatalf("CharsetReader(%q): %v", tt.charset, err)
			}
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("charset %q: got %q, want %q", tt.charset, got, tt.want)
			}
		})
	}
}

// TestCharsetReaderSmallBuffer exercises the pending-byte path where a
// multi-byte UTF-8 rune straddles the caller's Read buffer boundary.
func TestCharsetReaderSmallBuffer(t *testing.T) {
	// "é" is two UTF-8 bytes; read it one byte at a time.
	r, err := CharsetReader("iso-8859-1", strings.NewReader("\xe9\xe9"))
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if string(out) != "éé" {
		t.Errorf("got %q, want %q", out, "éé")
	}
}

// TestUnmarshalNonUTF8 verifies the package-level Unmarshal honors a non-UTF-8
// prolog declaration instead of rejecting it.
func TestUnmarshalNonUTF8(t *testing.T) {
	type doc struct {
		V string `xml:"v"`
	}
	cases := []struct {
		name string
		data string
		want string
	}{
		{
			"us-ascii",
			`<?xml version="1.0" encoding="us-ascii"?><doc><v>hello</v></doc>`,
			"hello",
		},
		{
			"windows-1252",
			"<?xml version=\"1.0\" encoding=\"Windows-1252\"?><doc><v>John\x92s</v></doc>",
			"John’s",
		},
		{
			"iso-8859-1",
			"<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><doc><v>caf\xe9</v></doc>",
			"café",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d doc
			if err := Unmarshal([]byte(tc.data), &d); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if d.V != tc.want {
				t.Errorf("got %q, want %q", d.V, tc.want)
			}
		})
	}
}
