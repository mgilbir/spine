package xml

import (
	"encoding/xml"
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

func TestUTF8(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
		err            bool
	}{
		{"utf-8 untouched", `<?xml version="1.0" encoding="UTF-8"?><r>é</r>`, `<?xml version="1.0" encoding="UTF-8"?><r>é</r>`, false},
		{"no declaration", `<r>é</r>`, `<r>é</r>`, false},
		{"bom kept", "\xef\xbb\xbf<?xml version=\"1.0\"?><r/>", "\xef\xbb\xbf<?xml version=\"1.0\"?><r/>", false},
		{"windows-1252", "<?xml version=\"1.0\" encoding=\"windows-1252\" standalone=\"yes\"?>\r\n<r a=\"\x80\">John\x92s</r>\r\n", "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\r\n<r a=\"€\">John’s</r>\r\n", false},
		{"latin-1, single quotes, spaced", "<?xml version='1.0' encoding = 'ISO-8859-1'?><r>caf\xe9</r>", "<?xml version='1.0' encoding = 'UTF-8'?><r>café</r>", false},
		{"bom contradicts declaration", "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"windows-1252\"?><r/>", "", true},
		{"unsupported", `<?xml version="1.0" encoding="shift_jis"?><r/>`, "", true},
	} {
		got, err := UTF8([]byte(tc.in))
		if tc.err {
			if err == nil {
				t.Errorf("%s: no error", tc.name)
			}
			continue
		}
		if err != nil || string(got) != tc.want {
			t.Errorf("%s: %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
	// A part that needs no transcoding is returned without a copy.
	in := []byte(`<?xml version="1.0" encoding="us-ascii"?><r/>`)
	if got, _ := UTF8(in); &got[0] != &in[0] {
		t.Error("UTF-8 input copied")
	}
}

// TestUnmarshalWithSourceCapturesTranscoded pins that a part in a code page
// gets the same offset-based capture as a UTF-8 one: the decoder reads the
// part's UTF-8 form, so its offsets index the registered source. The capture
// used to be switched off for such parts, and replayed elements fell back to
// canonical form.
func TestUnmarshalWithSourceCapturesTranscoded(t *testing.T) {
	var v transcodedProbe
	if err := UnmarshalWithSource([]byte("<?xml version=\"1.0\" encoding=\"windows-1252\"?><r><a>\x92\x92\x92</a><b ></b></r>"), &v); err != nil {
		t.Fatal(err)
	}
	if v.B.Style != EmptyTagExpanded || v.A != "’’’" {
		t.Errorf("got %+v", v)
	}
}

type transcodedProbe struct {
	A string         `xml:"a"`
	B transcodedLeaf `xml:"b"`
}

type transcodedLeaf struct{ Style EmptyTagStyle }

func (p *transcodedLeaf) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	p.Style = CaptureEmptyTagStyle(d)
	return d.Skip()
}

func TestPreferSource(t *testing.T) {
	encoded := []byte("<?xml version=\"1.0\" encoding=\"windows-1252\"?><r>caf\xe9</r>")
	text, err := UTF8(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got := PreferSource(encoded, text); &got[0] != &encoded[0] {
		t.Error("a rebuild equal to the UTF-8 form did not keep the source")
	}
	edited := []byte(`<?xml version="1.0" encoding="UTF-8"?><r>café!</r>`)
	if got := PreferSource(encoded, edited); &got[0] != &edited[0] {
		t.Error("an edited rebuild lost to the source")
	}
	// A UTF-8 source is the caller's business: the rebuild is returned even
	// when equal, without comparing.
	plain := []byte(`<r>café</r>`)
	if got := PreferSource(plain, []byte(`<r>café</r>`)); &got[0] == &plain[0] {
		t.Error("a UTF-8 source was preferred")
	}
	if got := PreferSource(nil, plain); &got[0] != &plain[0] {
		t.Error("no source")
	}
}
