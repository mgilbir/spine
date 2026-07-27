package xml

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// CharsetReader is the [xml.Decoder.CharsetReader] the library installs on
// every decoder it creates for parsing a part. Conformant OOXML is UTF-8 (or
// UTF-16), but wild files harvested from the web declare us-ascii, ISO-8859-1
// or Windows-1252 in their XML prolog; Office opens them, so the library must
// too. Without a CharsetReader Go's decoder rejects any non-UTF-8 declaration
// outright ("encoding %q declared but Decoder.CharsetReader is nil").
//
// It is parse-only: it transcodes bytes into the model on read. Preserved raw
// parts re-emit their original bytes untouched, so a non-UTF-8 part still
// round-trips byte-identically on a zero-mod save; only regenerated parts
// normalize to UTF-8, which is already the library's behavior.
//
// For the ASCII-compatible encodings (utf-8/us-ascii/empty) it returns the
// input reader unchanged, so the decoder's InputOffset keeps indexing the
// original source bytes and the fidelity-capture helpers are unaffected. Only
// the genuinely single-byte encodings wrap the reader in a transcoder, and
// such files are handled by preserved raw bytes rather than offset capture.
func CharsetReader(charset string, in io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii", "usascii", "ansi_x3.4-1968":
		// ASCII is a subset of UTF-8: pass the bytes through untouched so
		// input offsets continue to index the original source.
		return in, nil
	case "iso-8859-1", "iso8859-1", "iso_8859-1", "latin1", "latin-1", "l1",
		"8859-1", "cp819", "iso-ir-100", "csisolatin1":
		return newCharmapReader(in, &latin1Table), nil
	case "windows-1252", "cp1252", "win-1252", "win1252", "1252", "x-cp1252":
		return newCharmapReader(in, &windows1252Table), nil
	default:
		return nil, fmt.Errorf("xml: unsupported charset %q", charset)
	}
}

// charsetTranscodes reports whether CharsetReader will wrap a stream declaring
// charset in a transcoder (rather than returning it unchanged). The ASCII-
// compatible encodings pass through untouched; every other supported encoding
// is a single-byte code page that gets transcoded, which shifts the decoder's
// InputOffset off the original source bytes.
func charsetTranscodes(charset string) bool {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii", "usascii", "ansi_x3.4-1968":
		return false
	default:
		return true
	}
}

// OffsetCaptureSafe reports whether an [xml.Decoder]'s input offsets, taken
// while decoding data through [CharsetReader], still index data itself.
//
// A part declaring a single-byte code page is transcoded to UTF-8 on read, so
// the decoder's offsets index the transcoded stream and any capture that
// slices the original bytes at those offsets would replay garbage. Callers
// that hand raw part bytes to an offset-based capture (a RawSource field, a
// verbatim child slice) must gate the assignment on this predicate;
// [UnmarshalWithSource] applies it to its own source registration.
func OffsetCaptureSafe(data []byte) bool {
	return !charsetTranscodes(declaredCharset(data))
}

// declaredCharset returns the encoding named in data's XML declaration, or ""
// when the part has no declaration or no encoding pseudo-attribute. It reads
// only the declaration bytes; the value is not validated against the supported
// set (charsetTranscodes handles that).
func declaredCharset(data []byte) string {
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		data = data[3:]
	}
	if !bytes.HasPrefix(data, []byte("<?xml")) {
		return ""
	}
	end := bytes.Index(data, []byte("?>"))
	if end < 0 {
		return ""
	}
	decl := data[:end]
	idx := bytes.Index(decl, []byte("encoding"))
	if idx < 0 {
		return ""
	}
	rest := decl[idx+len("encoding"):]
	skipSpace := func() {
		for len(rest) > 0 && isXMLSpace(rest[0]) {
			rest = rest[1:]
		}
	}
	skipSpace()
	if len(rest) == 0 || rest[0] != '=' {
		return ""
	}
	rest = rest[1:]
	skipSpace()
	if len(rest) == 0 || (rest[0] != '"' && rest[0] != '\'') {
		return ""
	}
	quote := rest[0]
	rest = rest[1:]
	if q := bytes.IndexByte(rest, quote); q >= 0 {
		return string(rest[:q])
	}
	return ""
}

// NewDecoder returns an [xml.Decoder] with the library's CharsetReader
// installed, so parts declaring a non-UTF-8 (but supported) charset decode
// instead of failing.
func NewDecoder(r io.Reader) *xml.Decoder {
	d := xml.NewDecoder(r)
	d.CharsetReader = CharsetReader
	return d
}

// Unmarshal decodes data into v like [xml.Unmarshal], with the library's
// CharsetReader installed so a non-UTF-8 charset declaration in the prolog is
// honored rather than rejected.
func Unmarshal(data []byte, v any) error {
	return NewDecoder(bytes.NewReader(data)).Decode(v)
}

// latin1Table maps each ISO-8859-1 byte to its Unicode code point: the
// identity map, since ISO-8859-1 code points equal U+0000..U+00FF.
var latin1Table = func() [256]rune {
	var t [256]rune
	for i := range t {
		t[i] = rune(i)
	}
	return t
}()

// windows1252Table is ISO-8859-1 with the 0x80-0x9F range remapped to the
// Windows-1252 code points (curly quotes, dashes, the euro sign, etc.). The
// five bytes undefined in the standard (0x81/0x8D/0x8F/0x90/0x9D) map to
// U+FFFD. Verified against the Unicode CP1252 mapping.
var windows1252Table = func() [256]rune {
	t := latin1Table
	high := [32]rune{
		0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, // 0x80-0x87
		0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD, // 0x88-0x8F
		0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, // 0x90-0x97
		0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178, // 0x98-0x9F
	}
	for i, r := range high {
		t[0x80+i] = r
	}
	return t
}()

// charmapReader transcodes a single-byte encoding into UTF-8 on the fly using
// a 256-entry byte→rune table. It buffers at most a few pending UTF-8 bytes
// when a rune straddles the caller's buffer boundary.
type charmapReader struct {
	br    *bufio.Reader
	table *[256]rune
	pend  []byte
}

func newCharmapReader(in io.Reader, table *[256]rune) *charmapReader {
	return &charmapReader{br: bufio.NewReader(in), table: table}
}

func (r *charmapReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	total := 0
	for total < len(p) {
		if len(r.pend) > 0 {
			c := copy(p[total:], r.pend)
			r.pend = r.pend[c:]
			total += c
			continue
		}
		b, err := r.br.ReadByte()
		if err != nil {
			if total > 0 {
				return total, nil
			}
			return 0, err
		}
		var buf [utf8.UTFMax]byte
		n := utf8.EncodeRune(buf[:], r.table[b])
		c := copy(p[total:], buf[:n])
		total += c
		if c < n {
			r.pend = append([]byte(nil), buf[c:n]...)
		}
	}
	return total, nil
}
