package xml

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
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
// original source bytes. Only the genuinely single-byte encodings wrap the
// reader in a transcoder, whose offsets index the transcoded stream; parse a
// part through [UTF8] when anything is sliced from it at those offsets.
func CharsetReader(charset string, in io.Reader) (io.Reader, error) {
	table, err := charmap(charset)
	if err != nil || table == nil {
		// ASCII is a subset of UTF-8: pass the bytes through untouched so
		// input offsets continue to index the original source.
		return in, err
	}
	return newCharmapReader(in, table), nil
}

// charmap returns the byte-to-rune table of a single-byte charset, nil for the
// ASCII-compatible encodings that need no transcoding, or an error for a
// charset the library does not read.
func charmap(charset string) (*[256]rune, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii", "usascii", "ansi_x3.4-1968":
		return nil, nil
	case "iso-8859-1", "iso8859-1", "iso_8859-1", "latin1", "latin-1", "l1",
		"8859-1", "cp819", "iso-ir-100", "csisolatin1":
		return &latin1Table, nil
	case "windows-1252", "cp1252", "win-1252", "win1252", "1252", "x-cp1252":
		return &windows1252Table, nil
	default:
		return nil, fmt.Errorf("xml: unsupported charset %q", charset)
	}
}

// UTF8 returns a part's bytes as the UTF-8 document they encode. A part in
// UTF-16 (which OPC requires readers to accept alongside UTF-8) or one that
// declares a single-byte code page is transcoded, and its declaration's
// encoding rewritten to UTF-8 so the result describes itself; anything else is
// returned as is, without a copy.
//
// Parse a part through this, and slice any raw spans out of its result. The
// decoder then reads the bytes those spans come from, so its input offsets
// index them, and everything captured for replay (the prolog, verbatim
// children, separators) is already UTF-8 — the encoding every regenerated part
// is written in. Capturing from the source bytes instead replayed code-page
// bytes, or a declaration naming the code page, into UTF-8 output, and text
// read back as mojibake. The source bytes stay what an untouched part is
// saved as.
//
// UTF-16 is recognised by its byte order mark or, without one, by the "<?"
// that must then open the part (XML 1.0 Appendix F). An encoding the bytes
// contradict is an error (§4.3.3): a UTF-8 byte order mark on a part declaring
// another encoding, a UTF-16 part declaring anything but UTF-16, or one that
// declares UTF-16 without being it. So is a charset the library does not
// read, UTF-32 among them, and UTF-16 that does not decode.
func UTF8(data []byte) ([]byte, error) {
	if order, skip, ok, err := utf16Order(data); err != nil {
		return nil, err
	} else if ok {
		return utf16ToUTF8(data[skip:], order)
	}
	charset := declaredCharset(data)
	if isUTF16Name(charset) {
		return nil, fmt.Errorf("xml: part declares encoding %q but is not UTF-16", charset)
	}
	table, err := charmap(charset)
	if err != nil {
		return nil, err
	}
	if table == nil {
		return data, nil
	}
	if bytes.HasPrefix(data, utf8BOM) {
		return nil, fmt.Errorf("xml: UTF-8 byte order mark on a part declaring encoding %q", charset)
	}
	out := make([]byte, 0, len(data)+len(data)/4)
	for _, c := range data {
		out = utf8.AppendRune(out, table[c])
	}
	return rewriteDeclEncoding(out), nil
}

// PreferSource returns a part's source bytes when they are in an encoding
// other than UTF-8 and regenerated is exactly their UTF-8 form, and
// regenerated otherwise. A part the library rebuilds because it was read, not
// because it was edited, then keeps the bytes it was written in; a UTF-8
// source is left to the caller's own fidelity, at no cost.
func PreferSource(source, regenerated []byte) []byte {
	if len(source) == 0 {
		return regenerated
	}
	text, err := UTF8(source)
	if err != nil || len(text) > 0 && &text[0] == &source[0] {
		return regenerated
	}
	if bytes.Equal(text, regenerated) {
		return source
	}
	return regenerated
}

// asciiScanForm returns bytes a scan that reads markup as ASCII can work on:
// a UTF-16 part's UTF-8 form, since in UTF-16 every '<', '/' and '>' comes with
// a zero byte; anything else as is (a code page is ASCII in its markup). A
// UTF-16 part that does not decode is returned as is, and no parse reads it.
func asciiScanForm(data []byte) []byte {
	if _, _, utf16, _ := utf16Order(data); utf16 {
		if text, err := UTF8(data); err == nil {
			return text
		}
	}
	return data
}

// rewriteDeclEncoding returns text, a transcoded part, with its declaration's
// encoding rewritten to UTF-8. The declaration is ASCII, so it is found in the
// transcoded text where it was in the source.
func rewriteDeclEncoding(text []byte) []byte {
	if !bytes.HasPrefix(text, []byte("<?xml")) {
		return text
	}
	end := bytes.Index(text, []byte("?>"))
	if end < 0 {
		return text
	}
	end += len("?>")
	return append([]byte(declEncodingUTF8(string(text[:end]))), text[end:]...)
}

// utf16Order reports whether data is UTF-16 and in which byte order, and how
// many bytes its byte order mark takes. UTF-32, whose marks begin like
// UTF-16's, is an error rather than a misreading.
func utf16Order(data []byte) (order binary.ByteOrder, skip int, ok bool, err error) {
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE, 0, 0}), bytes.HasPrefix(data, []byte{0, 0, 0xFE, 0xFF}),
		bytes.HasPrefix(data, []byte{'<', 0, 0, 0}), bytes.HasPrefix(data, []byte{0, 0, 0, '<'}):
		return nil, 0, false, fmt.Errorf("xml: unsupported encoding UTF-32")
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return binary.LittleEndian, 2, true, nil
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return binary.BigEndian, 2, true, nil
	case bytes.HasPrefix(data, []byte{'<', 0, '?', 0}):
		return binary.LittleEndian, 0, true, nil
	case bytes.HasPrefix(data, []byte{0, '<', 0, '?'}):
		return binary.BigEndian, 0, true, nil
	}
	return nil, 0, false, nil
}

// utf16ToUTF8 decodes UTF-16 code units in the given byte order into UTF-8
// and rewrites the declaration, which must name UTF-16 if it names anything.
func utf16ToUTF8(data []byte, order binary.ByteOrder) ([]byte, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("xml: UTF-16 part has an odd number of bytes")
	}
	// Markup is mostly ASCII, which halves; anything wider grows in append.
	out := make([]byte, 0, len(data)/2+16)
	for i := 0; i < len(data); i += 2 {
		u := order.Uint16(data[i:])
		switch {
		case u < 0xD800 || u > 0xDFFF:
			out = utf8.AppendRune(out, rune(u))
		case u < 0xDC00 && i+3 < len(data):
			lo := order.Uint16(data[i+2:])
			if lo < 0xDC00 || lo > 0xDFFF {
				return nil, fmt.Errorf("xml: UTF-16 part has an unpaired surrogate at byte %d", i)
			}
			out = utf8.AppendRune(out, utf16.DecodeRune(rune(u), rune(lo)))
			i += 2
		default:
			return nil, fmt.Errorf("xml: UTF-16 part has an unpaired surrogate at byte %d", i)
		}
	}
	if charset := declaredCharset(out); charset != "" && !isUTF16Name(charset) {
		return nil, fmt.Errorf("xml: UTF-16 part declares encoding %q", charset)
	}
	return rewriteDeclEncoding(out), nil
}

// isUTF16Name reports whether charset names UTF-16.
func isUTF16Name(charset string) bool {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "utf-16", "utf16", "utf-16le", "utf-16be", "unicode", "ucs-2", "iso-10646-ucs-2", "csunicode":
		return true
	}
	return false
}

// utf8BOM is the UTF-8 encoding of U+FEFF.
var utf8BOM = []byte("\xef\xbb\xbf")

// declEncodingUTF8 rewrites the encoding pseudo-attribute of an XML
// declaration to UTF-8, preserving everything else about the producer's
// declaration (version, standalone, spacing, quote style). Declarations
// without an encoding pseudo-attribute are returned unchanged.
func declEncodingUTF8(decl string) string {
	i := strings.Index(decl, "encoding")
	if i < 0 {
		return decl
	}
	j := i + len("encoding")
	skipSpace := func() {
		for j < len(decl) && isXMLSpace(decl[j]) {
			j++
		}
	}
	skipSpace()
	if j >= len(decl) || decl[j] != '=' {
		return decl
	}
	j++
	skipSpace()
	if j >= len(decl) || (decl[j] != '"' && decl[j] != '\'') {
		return decl
	}
	quote := decl[j]
	j++
	end := strings.IndexByte(decl[j:], quote)
	if end < 0 {
		return decl
	}
	return decl[:j] + "UTF-8" + decl[j+end:]
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
// slices the original bytes at those offsets would replay garbage. The
// library's parsers no longer need the gate: they decode the part's [UTF8]
// form and slice that, which keeps the capture for such parts too. A UTF-16
// part is never safe: only its UTF-8 form decodes at all.
func OffsetCaptureSafe(data []byte) bool {
	if _, _, utf16, err := utf16Order(data); utf16 || err != nil {
		return false
	}
	return !charsetTranscodes(declaredCharset(data))
}

// declaredCharset returns the encoding named in data's XML declaration, or ""
// when the part has no declaration or no encoding pseudo-attribute. It reads
// only the declaration bytes; the value is not validated against the supported
// set (charsetTranscodes handles that).
func declaredCharset(data []byte) string {
	data = bytes.TrimPrefix(data, utf8BOM)
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

// NewPartDecoder returns a decoder over a part's UTF-8 form ([UTF8]) and the
// bytes it reads. A token loop over a part read off a package uses it, so that
// a part in a code page decodes; slice any raw span out of the returned bytes,
// never the argument, since the decoder's offsets index them. Splicing into
// them, too, gives a part that is UTF-8 throughout and says so.
func NewPartDecoder(data []byte) (*xml.Decoder, []byte, error) {
	data, err := UTF8(data)
	if err != nil {
		return nil, nil, err
	}
	return NewDecoder(bytes.NewReader(data)), data, nil
}

// NewDecoder returns an [xml.Decoder] with the library's CharsetReader
// installed, so a stream declaring a supported code page decodes instead of
// failing. It transcodes as it reads, so it cannot read UTF-16, whose
// declaration it never reaches: read a whole part through [NewPartDecoder].
func NewDecoder(r io.Reader) *xml.Decoder {
	d := xml.NewDecoder(r)
	d.CharsetReader = CharsetReader
	return d
}

// Unmarshal decodes data into v like [xml.Unmarshal], with the library's
// CharsetReader installed so a non-UTF-8 charset declaration in the prolog is
// honored rather than rejected.
//
// Unlike [xml.Unmarshal] it also requires the document to be well-formed to its
// end and to bind every prefix it uses — see [CheckDocumentEnd] and
// [CheckNamespacePrefixes] for why the difference matters.
//
// The two checks are applied together, and by both part-level entry points.
// They were not, once: this one took only CheckDocumentEnd while
// UnmarshalWithSource took both, and a part parsed through the weaker entry
// point still reached the writer carrying a prefix nothing declared
// (FuzzPptxModernCommentXML). Two entry points enforcing different rules is one
// rule and one hole.
func Unmarshal(data []byte, v any) error {
	data, err := UTF8(data)
	if err != nil {
		return err
	}
	d := NewDecoder(bytes.NewReader(data))
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := CheckDocumentEnd(d); err != nil {
		return err
	}
	return CheckNamespacePrefixes(data)
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
